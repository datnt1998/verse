#!/usr/bin/env python3
"""W0 — đo tìm kiếm trên truyện thật (chỉ thư viện chuẩn Python).

Cách dùng:
  python3 run.py                  # chạy đủ 7 retriever, in bảng kết quả
  python3 run.py --no-ollama      # chỉ 3 retriever theo từ (không cần Ollama)
  python3 run.py --dump-chunks    # in id + 80 ký tự đầu mỗi chunk
  python3 run.py --show c03-007   # in toàn văn một chunk
  python3 run.py --story <dir>    # thư mục output/novel khác
"""
import argparse
import hashlib
import json
import math
import os
import re
import subprocess
import sys
import time
import unicodedata
import urllib.request
from collections import Counter

DEFAULT_STORY = "/Users/dat/Projects/truyen/dit-dang-audio/kich_ban/01-tra-xanh-cuop-kich-ban/output/novel"
OLLAMA_URL = "http://127.0.0.1:11434/api/embed"
CACHE_DIR = "/tmp/nao-w0-cache"
CHUNK_TARGET = 500      # đóng gói đoạn tới ~500 ký tự
PARA_SOLO = 900         # đoạn dài hơn 900 tự thành một chunk
K1, B, RRF_K = 1.2, 0.75, 60
TOP = 10
QWEN3_PREFIX = ("Instruct: Given a Vietnamese question about a story, "
                "retrieve the passage that answers it\nQuery: ")
LETTER = r"[^\W\d_]"    # một ký tự chữ (unicode, giữ dấu)
TOKEN_RE = re.compile(LETTER + "+", re.UNICODE)


def nfc(s):
    return unicodedata.normalize("NFC", s)


def tokens(text):
    return TOKEN_RE.findall(nfc(text).lower())


def fold_token(t):
    """Bỏ dấu một âm tiết: NFD, đ→d, xoá dấu tổ hợp."""
    t = t.replace("đ", "d")
    return "".join(c for c in unicodedata.normalize("NFD", t)
                   if unicodedata.category(c) != "Mn")



# ---------------------------------------------------------------- chunks
def chunk_story(story_dir):
    """Đọc chapters/01..08.md, gói đoạn văn thành chunk ~CHUNK_TARGET ký tự."""
    chunks = []  # [(id, text)]
    for ch in range(1, 9):
        path = os.path.join(story_dir, "chapters", "%02d.md" % ch)
        with open(path, encoding="utf-8") as f:
            text = nfc(f.read())
        paras = [p.strip() for p in re.split(r"\n\s*\n", text) if p.strip()]
        buf, buflen, idx = [], 0, 0
        for p in paras:
            if len(p) > PARA_SOLO:
                if buf:
                    idx += 1
                    chunks.append(("c%02d-%03d" % (ch, idx), "\n\n".join(buf)))
                    buf, buflen = [], 0
                idx += 1
                chunks.append(("c%02d-%03d" % (ch, idx), p))
            elif buf and buflen + len(p) + 2 > CHUNK_TARGET:
                idx += 1
                chunks.append(("c%02d-%03d" % (ch, idx), "\n\n".join(buf)))
                buf, buflen = [p], len(p)
            else:
                buf.append(p)
                buflen += len(p) + 2
        if buf:
            idx += 1
            chunks.append(("c%02d-%03d" % (ch, idx), "\n\n".join(buf)))
    return chunks


# ---------------------------------------------------------------- entities
def load_entities(story_dir):
    """Trả về [(tên chuẩn, [tên+biệt danh đã lower])] từ characters.json."""
    path = os.path.join(story_dir, "characters.json")
    with open(path, encoding="utf-8") as f:
        chars = json.load(f)
    table = []
    for c in chars:
        # biệt danh một âm tiết ("tôi") quá mơ hồ để khớp ranh giới: bỏ
        names = [nfc(c["name"]).lower()]
        names += [nfc(a).lower() for a in c.get("aliases", [])
                  if len(tokens(a)) > 1]
        table.append((c["name"], names))
    return table


def name_hits(text, names):
    """Danh sách tên/biệt danh xuất hiện trong text (khớp theo ranh giới chữ)."""
    t = nfc(text).lower()
    hits = []
    for n in names:
        if re.search(r"(?<!%s)%s(?!%s)" % (LETTER, re.escape(n), LETTER), t):
            hits.append(n)
    return hits


# ---------------------------------------------------------------- bm25
class BM25:
    def __init__(self, doc_tokens):
        self.N = len(doc_tokens)
        self.dl = [len(d) for d in doc_tokens]
        self.avgdl = sum(self.dl) / self.N if self.N else 1.0
        self.tf = [Counter(d) for d in doc_tokens]
        df = Counter()
        for d in doc_tokens:
            for t in set(d):
                df[t] += 1
        self.idf = {t: math.log(1.0 + (self.N - f + 0.5) / (f + 0.5))
                    for t, f in df.items()}

    def scores(self, q_tokens):
        out = []
        for i in range(self.N):
            s = 0.0
            tf = self.tf[i]
            dl = self.dl[i]
            for t in q_tokens:
                f = tf.get(t)
                if not f:
                    continue
                s += self.idf[t] * f * (K1 + 1.0) / (
                    f + K1 * (1.0 - B + B * dl / self.avgdl))
            out.append(s)
        return out

    def rank(self, q_tokens):
        sc = self.scores(q_tokens)
        order = sorted(range(self.N), key=lambda i: (-sc[i], i))
        return order, sc


def rrf(rank_lists):
    """Reciprocal Rank Fusion, k=RRF_K. rank_lists: [ [doc_index,...], ... ]"""
    score = Counter()
    for ranks in rank_lists:
        for r, doc in enumerate(ranks):
            score[doc] += 1.0 / (RRF_K + r + 1)
    return [d for d, _ in sorted(score.items(), key=lambda kv: (-kv[1], kv[0]))]


# ---------------------------------------------------------------- ollama
def _post(url, payload):
    req = urllib.request.Request(
        url, data=json.dumps(payload).encode("utf-8"),
        headers={"Content-Type": "application/json"})
    with urllib.request.urlopen(req, timeout=600) as r:
        return json.loads(r.read().decode("utf-8"))


def _cache_name(model):
    return re.sub(r"[^A-Za-z0-9._-]", "_", model)


def embed(model, texts):
    """Embedding theo batch <=32, đệm vào CACHE_DIR theo sha256(text)."""
    cdir = os.path.join(CACHE_DIR, _cache_name(model))
    os.makedirs(cdir, exist_ok=True)
    keys = [hashlib.sha256(t.encode("utf-8")).hexdigest() for t in texts]
    out = [None] * len(texts)
    missing = []
    for i, k in enumerate(keys):
        p = os.path.join(cdir, k + ".json")
        if os.path.exists(p):
            with open(p) as f:
                out[i] = json.load(f)
        else:
            missing.append(i)
    for b in range(0, len(missing), 32):
        idxs = missing[b:b + 32]
        resp = _post(OLLAMA_URL, {"model": model,
                                  "input": [texts[i] for i in idxs]})
        for i, vec in zip(idxs, resp["embeddings"]):
            out[i] = vec
            with open(os.path.join(cdir, keys[i] + ".json"), "w") as f:
                json.dump(vec, f)
    return out


def cosine(a, b):
    dot = na = nb = 0.0
    for x, y in zip(a, b):
        dot += x * y
        na += x * x
        nb += y * y
    return dot / (math.sqrt(na) * math.sqrt(nb) + 1e-12)


def model_size(model):
    try:
        out = subprocess.run(["ollama", "list"], capture_output=True,
                             text=True, timeout=30).stdout
        for line in out.splitlines():
            parts = line.split()
            if parts and (parts[0] == model or parts[0].startswith(model + ":")):
                return parts[2] + " " + parts[3]
    except Exception:
        pass
    return "?"


# ---------------------------------------------------------------- eval
def load_questions(path):
    qs = []
    with open(path, encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if line:
                qs.append(json.loads(line))
    return qs


def metrics(rankings, questions):
    """rankings: {qid: [chunk_id,...]} -> (r1, r5, mrr) và theo kind."""
    def score(qs):
        r1 = r5 = mrr = 0.0
        for q in qs:
            gold = set(q["gold"])
            top = rankings[q["id"]]
            if gold & set(top[:1]):
                r1 += 1
            if gold & set(top[:5]):
                r5 += 1
            for i, cid in enumerate(top[:10]):
                if cid in gold:
                    mrr += 1.0 / (i + 1)
                    break
        n = len(qs)
        return r1 / n, r5 / n, mrr / n
    overall = score(questions)
    by_kind = {}
    for kind in ("lexical", "paraphrase", "entity"):
        sub = [q for q in questions if q["kind"] == kind]
        if sub:
            by_kind[kind] = score(sub)
    return overall, by_kind


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--story", default=DEFAULT_STORY)
    ap.add_argument("--questions",
                    default=os.path.join(os.path.dirname(os.path.abspath(__file__)),
                                         "questions.jsonl"))
    ap.add_argument("--dump-chunks", action="store_true")
    ap.add_argument("--show", metavar="CHUNK_ID")
    ap.add_argument("--no-ollama", action="store_true")
    args = ap.parse_args()

    chunks = chunk_story(args.story)
    ids = [c[0] for c in chunks]
    texts = [c[1] for c in chunks]
    id2i = {cid: i for i, cid in enumerate(ids)}

    if args.dump_chunks:
        for cid, t in chunks:
            print("%s  %s" % (cid, " ".join(t.split())[:80]))
        print("# %d chunks" % len(chunks), file=sys.stderr)
        return
    if args.show:
        print(texts[id2i[args.show]])
        return

    questions = load_questions(args.questions)
    for q in questions:
        for g in q["gold"]:
            if g not in id2i:
                sys.exit("gold không tồn tại: %s (%s)" % (g, q["id"]))

    entities = load_entities(args.story)
    ent_names = [names for _, names in entities]

    # --- chuẩn bị chỉ mục theo từ ---
    doc_tok = [tokens(t) for t in texts]
    doc_tok_fold = [d + [fold_token(t) for t in d] for d in doc_tok]
    bm = BM25(doc_tok)
    bm_fold = BM25(doc_tok_fold)
    # nhân vật xuất hiện trong từng chunk (tiền tính)
    chunk_ent = [set() for _ in chunks]
    for i, t in enumerate(texts):
        for ci, names in enumerate(ent_names):
            if name_hits(t, names):
                chunk_ent[i].add(ci)

    # --- embedding (một lần cho toàn bộ chunk, đo thời gian) ---
    vec_chunks = {}
    embed_secs = {}
    if not args.no_ollama:
        for model in ("bge-m3", "qwen3-embedding:0.6b"):
            t0 = time.time()
            vec_chunks[model] = embed(model, texts)
            embed_secs[model] = time.time() - t0

    # --- các retriever ---
    def r_bm25(q):
        order, _ = bm.rank(tokens(q))
        return order

    def r_bm25_fold(q):
        qt = tokens(q)
        order, _ = bm_fold.rank(qt + [fold_token(t) for t in qt])
        return order

    def r_bm25_graph(q):
        order, _ = bm.rank(tokens(q))
        mentioned = set()
        for ci, names in enumerate(ent_names):
            if name_hits(q, names):
                mentioned.add(ci)
        # danh sách thực thể: chunk chứa nhân vật được nhắc trong câu hỏi,
        # xếp theo số thực thể khớp rồi theo điểm bm25
        if mentioned:
            esc = bm.scores(tokens(q))
            elist = [i for i in range(len(chunks)) if chunk_ent[i] & mentioned]
            elist.sort(key=lambda i: (-len(chunk_ent[i] & mentioned), -esc[i]))
            return rrf([order, elist])
        return order

    def r_vec(model, q):
        qv = embed(model, [QWEN3_PREFIX + q if model.startswith("qwen3") else q])[0]
        sc = [cosine(qv, v) for v in vec_chunks[model]]
        return sorted(range(len(chunks)), key=lambda i: (-sc[i], i))

    def r_hybrid(model, q):
        return rrf([r_bm25_graph(q), r_vec(model, q)])

    retrievers = [
        ("bm25", r_bm25),
        ("bm25_fold", r_bm25_fold),
        ("bm25_graph", r_bm25_graph),
    ]
    if not args.no_ollama:
        retrievers += [
            ("vec_bge_m3", lambda q: r_vec("bge-m3", q)),
            ("vec_qwen3", lambda q: r_vec("qwen3-embedding:0.6b", q)),
            ("hybrid_bge", lambda q: r_hybrid("bge-m3", q)),
            ("hybrid_qwen3", lambda q: r_hybrid("qwen3-embedding:0.6b", q)),
        ]

    details = {}
    print("\n## Tổng quan (n=%d câu, %d chunk)\n" % (len(questions), len(chunks)))
    print("| retriever | R@1 | R@5 | MRR@10 |")
    print("|---|---|---|---|")
    all_rankings = {}
    for name, fn in retrievers:
        rankings = {}
        for q in questions:
            top = fn(q["q"])[:TOP]
            rankings[q["id"]] = [ids[i] for i in top]
        all_rankings[name] = rankings
        o, bk = metrics(rankings, questions)
        details[name] = {"overall": o, "by_kind": bk, "rankings": rankings}
        print("| %s | %.3f | %.3f | %.3f |" % (name, *o))

    print("\n## R@5 theo loại câu hỏi\n")
    print("| retriever | lexical | paraphrase | entity |")
    print("|---|---|---|---|")
    for name, _ in retrievers:
        bk = details[name]["by_kind"]
        print("| %s | %.3f | %.3f | %.3f |" % (
            name, bk["lexical"][1], bk["paraphrase"][1], bk["entity"][1]))

    print("\n## Thời gian & kích thước\n")
    print("- Số chunk: %d — Số câu hỏi: %d" % (len(chunks), len(questions)))
    for model, secs in embed_secs.items():
        print("- Embedding %d chunk bằng %s: %.1f s — kích thước trên đĩa: %s"
              % (len(chunks), model, secs, model_size(model)))

    os.makedirs(CACHE_DIR, exist_ok=True)
    with open(os.path.join(CACHE_DIR, "last_run.json"), "w") as f:
        json.dump({"questions": questions, "rankings": all_rankings}, f,
                  ensure_ascii=False, indent=1)


if __name__ == "__main__":
    main()
