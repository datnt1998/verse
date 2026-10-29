# Phụ lục nhánh truyện audio một tập: "vả mặt, mỏ hỗn" (truyện Trung hiện đại Việt hóa)

## 0. Chuẩn chấm
- **Master Rules v3 là chuẩn chấm duy nhất.** Mức LỖI và CẢNH BÁO lấy theo nhãn trong Master và trong quy tắc thể loại đang dùng (bộ `rules-A`, `rules-B`, `rules-C`, hoặc module trong `phu-luc-the-loai-v3.md`).
- File này chỉ bổ sung định dạng riêng của nhánh truyện một tập, khung chương gợi ý, cách đo và script. Mọi thứ trong file này đều ở mức **THAM KHẢO**, không tính là trượt.
- Khi file này và Master khác nhau, làm theo Master.
- Khung 8 chương dưới đây dùng được cho Module A và Module D. Các module khác có khung riêng trong phụ lục thể loại; script vẫn dùng được, chỉ cần bỏ qua các dòng THAM KHẢO về số chương và độ dài, và đặt tham số `--thoai`, `--cau`, `--doan` theo module.

## 1. Định dạng nhánh
- Một quyển gồm 8 chương, kết thúc trọn vẹn ở chương 8. Phần hai, nếu có, là một quyển mới theo Master §19.
- Mỗi chương khoảng 1.400–1.600 tiếng. Tổng khoảng 11.200–12.800 tiếng, tức khoảng 50–58 phút audio.
- Một mâu thuẫn chính, một phản diện chính, và một tuyến tình cảm phụ nếu có.

## 2. Khung 8 chương, gắn với các mốc giữ chân của Master v3
| Chương | Vị trí | Nội dung | Mốc Master v3 |
|---|---|---|---|
| 1 | 0–12% | Bất công mở màn, nữ chính trả đũa nhỏ ngay trong cảnh đầu, gieo bí mật hoặc vật chứng; giới thiệu điểm thương | §4: mốc 30 giây, 2 phút, 5 phút; §10 |
| 2 | 12–25% | Vả mặt lớn đầu tiên trước đám đông | §6: leo thang khán giả |
| 3 | 25–37% | Leo thang, xen cảnh được cưng chiều hoặc hồi tưởng bất công | §12: chạm vào điểm yếu của nữ chính |
| 4 | 37–50% | Sự kiện công khai lớn; **cảnh lời hứa của tiêu đề** | §3: lời hứa nằm trong 35–60% |
| 5 | 50–62% | **Cú lật giữa truyện**; phản diện phản công bằng tung tin hoặc vu khống | §8: cú lật quanh mốc 50% |
| 6 | 62–75% | Chạm đáy, nhưng có ít nhất một chiến thắng nhỏ; nữ chính âm thầm chuẩn bị; chạm lại điểm thương | §8, §10: không cam chịu quá khoảng 450 tiếng liền |
| 7 | 75–88% | Cao trào: đối chất bằng thoại, bằng chứng đã cài từ trước, một bí mật lớn bị lộ | §13, §14 |
| 8 | 88–100% | Phản diện tự chuốc kết cục tương xứng có dư, nói ra vì sao mình thua; người hùa theo trả giá; phần kết không quá khoảng 900 tiếng; câu cuối gọi lại phần mở đầu | §11, §16, §19 |


## 3. Cách đo
- **Tiếng:** cụm chữ phân tách bằng khoảng trắng, không tính tiêu đề chương. Ước tính khoảng 220 tiếng bằng một phút audio.
- **Tỷ lệ thoại:** số tiếng nằm trong “ ” chia cho tổng số tiếng của chương.
- **Chạy script ở Phụ lục:**
  - Bản editor: `python3 kiem_tra_truyen.py ban_editor.txt`
  - Bản narration: `python3 kiem_tra_truyen.py ban_narration.txt --narration`
  - So mở đầu với các truyện cũ của kênh (Master §17): thêm `--so-voi truyen_cu_1.txt truyen_cu_2.txt ...`
  - Đổi khoảng thoại cho module khác: thêm `--thoai 30-45` (ví dụ Module C).
  - Đổi giới hạn câu và đoạn: thêm `--cau 30 --doan 70` (ví dụ Module G).
  - Đổi số chương và độ dài chương: thêm `--chuong 10 --tieng 1400-1600` (ví dụ bộ C). Mặc định 8 chương, 1.400–1.600 tiếng.
  - Script nhận cả tiêu đề dạng "Chương 1:" lẫn "第 1 章". Trong bản narration, các chương cách nhau hai dòng trống.
- **Script đo được:**
  - tỷ lệ thoại, mật độ từ thủ tục (§13, §14);
  - các mục TTS và narration sạch (§15);
  - quãng dài không có thoại (§5, đo gần đúng);
  - cụm từ lặp và sáo ngữ (§13);
  - độ trùng mở đầu với truyện cũ (§17, đo gần đúng).
- **Script in sẵn để kiểm tay:** 110 tiếng đầu (§4), đoạn cuối mỗi chương (§7), và 900 tiếng cuối truyện để soát §11 và §16.
- **Phải đọc bằng người:** các mục còn lại, theo bộ câu hỏi ở §20 của Master.

## 4. Báo cáo chấm khi nộp
1. **Thông tin đầu bản:** nhóm người nghe, "Điểm khác", hạt giống phần hai, ba phương án tiêu đề (Master §21).
2. **Bản đồ giữ chân:** theo mẫu ở Master §21.
3. **LỖI:** mỗi lỗi ghi số mục, dòng hoặc đoạn trích, và cách sửa.
4. **CẢNH BÁO:** mỗi mục ghi "đã sửa" hoặc "giữ vì…".
5. **Kết quả máy đo:** dán nguyên văn kết quả script cho cả bản editor và bản narration.

Không nộp khi kết quả script hoặc phần kiểm tay vẫn còn LỖI.

## Phụ lục: script kiểm tra
Lưu đoạn dưới thành file `kiem_tra_truyen.py`.

```python
#!/usr/bin/env python3
"""Kiểm tra truyện audio theo MASTER RULES v3 (nhánh truyện một tập).

Cách dùng:
  python3 kiem_tra_truyen.py ban_editor.txt
  python3 kiem_tra_truyen.py ban_narration.txt --narration
  python3 kiem_tra_truyen.py ban_narration.txt --narration --so-voi cu1.txt cu2.txt
  python3 kiem_tra_truyen.py ban_editor.txt --thoai 30-45
  python3 kiem_tra_truyen.py ban_editor.txt --cau 30 --doan 70
  python3 kiem_tra_truyen.py ban_editor.txt --chuong 10 --tieng 1400-1600

Ba mức kết quả:
  LỖI        quy tắc có nhãn [LỖI] trong Master v3, phải sửa trước khi nộp
  CẢNH BÁO   quy tắc có nhãn [CẢNH BÁO]: sửa hoặc ghi lý do
  THAM KHẢO  định dạng riêng của nhánh, không tính trượt
Script chỉ đo được phần máy đo được. Các mục còn lại kiểm bằng tay theo §20 của Master Rules v3.
"""
import re, sys, collections

HEAD = re.compile(r'(?m)^(?:Chương \d+:|第 ?\d+ ?章).*$')
PROC = ["xác nhận", "đối chiếu", "niêm phong", "phạm vi", "biên bản", "bảo toàn",
        "giám định", "lưu ký", "nghiệm thu", "giải ngân", "thủ tục", "văn bản"]
EN_RISK = ["live", "credit", "casting", "scandal", "team", "post", "comment",
           "trend", "view", "share", "follow", "hashtag", "story"]
CLICHE = ["cười lạnh", "sững sờ", "mặt cắt không còn giọt máu"]
STOP = set("tôi cô ta anh chị em của và là đã không có một những cho rồi thì mà với "
           "trong ra vào được để này đó nó lại cũng còn người".split())

N_CHUONG, T_MIN, T_MAX = 8, 1400, 1600

def wc(s): return len(s.split())
def count(words, text): return sum(len(re.findall(r'(?<!\w)' + re.escape(k) + r'(?!\w)', text)) for k in words)

def body_words(text):
    lines = [l for l in text.split("\n") if l.strip() and not HEAD.match(l)]
    return " ".join(lines).lower().split()

def grams5(words):
    toks = [re.sub(r'\W', '', w) for w in words]
    toks = [w for w in toks if w]
    return set(tuple(toks[i:i + 5]) for i in range(len(toks) - 4))

def parse_args(argv):
    path, narration, so_voi, lo, hi, cau, doan = argv[0], False, [], 0.40, 0.55, 40, 90
    global N_CHUONG, T_MIN, T_MAX
    i = 1
    while i < len(argv):
        a = argv[i]
        if a == "--narration": narration = True
        elif a == "--chuong":
            i += 1; N_CHUONG = int(argv[i])
        elif a == "--tieng":
            i += 1; T_MIN, T_MAX = (int(x) for x in argv[i].split("-"))
        elif a == "--cau":
            i += 1; cau = int(argv[i])
        elif a == "--doan":
            i += 1; doan = int(argv[i])
        elif a == "--thoai":
            i += 1; lo, hi = (int(x) / 100 for x in argv[i].split("-"))
        elif a == "--so-voi":
            i += 1
            while i < len(argv) and not argv[i].startswith("--"):
                so_voi.append(argv[i]); i += 1
            continue
        i += 1
    return path, narration, so_voi, lo, hi, cau, doan

def main(argv):
    path, narration, so_voi, lo, hi, cau, doan = parse_args(argv)
    t = open(path, encoding="utf-8").read()
    heads = HEAD.findall(t)
    mode = "narration" if narration or not heads else "editor"
    loi, canh, tk = [], [], []

    # --- Tách chương
    if heads:
        parts = HEAD.split(t)[1:]
        chs = list(zip([h.strip() for h in heads], parts))
    else:
        blocks = [b for b in re.split(r'\n\s*\n\s*\n', t) if b.strip()]
        chs = [(f"Phần {i}", b) for i, b in enumerate(blocks, 1)]

    print(f"Bản: {mode.upper()} | số chương nhận được: {len(chs)} | khoảng thoại {lo:.0%}–{hi:.0%}\n")
    total = 0
    for name, body in chs:
        w = wc(body); total += w
        r = sum(wc(x) for x in re.findall(r'“([^”]*)”', body)) / max(w, 1)
        p = count(PROC, body.lower())
        print(f"{name[:42]:42} | tiếng {w:5} | thoại {r:4.0%} | từ thủ tục {p:2}")
        if not lo <= r <= hi:
            canh.append(f"§13 Thoại {r:.0%} ngoài khoảng {lo:.0%}–{hi:.0%} ở '{name[:30]}'")
        if p > 10:
            canh.append(f"§14 Mật độ thủ tục cao ({p} từ) ở '{name[:30]}': đọc lại xem có rườm rà không")
        if not T_MIN <= w <= T_MAX:
            tk.append(f"Số tiếng {w} ngoài mục tiêu {T_MIN}–{T_MAX} ở '{name[:30]}'")
    if len(chs) != N_CHUONG: tk.append(f"Nhận được {len(chs)} chương, dự kiến {N_CHUONG}")
    tk.append(f"Tổng {total} tiếng (mục tiêu {N_CHUONG * T_MIN}–{N_CHUONG * T_MAX})")

    lines = t.split("\n")
    paras = [(i + 1, l) for i, l in enumerate(lines) if l.strip() and not HEAD.match(l)]

    # --- LỖI chỉ áp cho bản narration (Master v3 §15.5)
    if mode == "narration":
        cjk = re.findall(r'[　-〿一-鿿《》]', t)
        if cjk: loi.append(f"§15.5 Còn ký tự tiếng Trung/《》: {len(cjk)} ký tự")
        md = [n for n, l in paras if re.search(r'\*[^*\n]+\*|^#|^\s*[-+] |\|', l)]
        if md: loi.append(f"§15.5 Còn Markdown/bảng/gạch đầu dòng ở dòng {md[:10]}")
        if heads: loi.append("§15.5 Bản narration còn tiêu đề chương")
        dg = [n for n, l in paras if re.search(r'\d', l)]
        if dg: loi.append(f"§15.5 Còn chữ số (TTS dễ đọc sai) ở dòng {dg[:10]}")
        emo = [n for n, l in paras if re.search(r'[\U0001F300-\U0001FAFF☀-➿]', l)]
        if emo: loi.append(f"§15.5 Còn emoji hoặc ký hiệu trang trí ở dòng {emo[:10]}")
        tag = [n for n, l in paras if re.search(r'\[[^\]]{1,30}\]|\([^)]*(?:giọng|cảm xúc|thì thầm|hét)[^)]*\)', l)]
        if tag: loi.append(f"§15.5 Có thể còn thẻ điều khiển giọng/hướng dẫn cảm xúc ở dòng {tag[:10]}")
    else:
        if re.search(r'\*[^*\n]+\*', t):
            canh.append("§15.5 Có dấu * in nghiêng: nhớ bỏ khi xuất bản narration")

    # --- CẢNH BÁO đo được (Master v3 §13, §15)
    low = t.lower()
    en = collections.Counter(re.findall(r'(?<!\w)(' + '|'.join(EN_RISK) + r')(?!\w)', low))
    if en: canh.append(f"§15.3 Từ tiếng Anh TTS dễ đọc sai: {dict(en)}")
    nt = [n for n, l in paras if re.search(r'ngày thứ (?:hai|ba|tư|năm|sáu|bảy)(?!\w)', l.lower())]
    if nt: canh.append(f"§15.3 'ngày thứ' + số hai–bảy (dễ nghe thành ngày trong tuần) ở dòng {nt}")
    for c in CLICHE:
        k = low.count(c)
        if k > 2: canh.append(f"§13 '{c}' dùng {k} lần")
    ls = [n for n, l in paras for s in re.split(r'(?<=[.!?…])\s+', l) if wc(s) > cau]
    if ls: canh.append(f"§15.1 Câu dài hơn {cau} tiếng ở dòng {sorted(set(ls))[:10]}")
    lp = [n for n, l in paras if wc(l) > doan]
    if lp: canh.append(f"§15.1 Đoạn dài hơn {doan} tiếng ở dòng {lp[:10]}")
    lq = [n for n, l in paras for q in re.findall(r'“([^”]*)”', l) if wc(q) > 60]
    if lq: canh.append(f"§13 Câu thoại dài như diễn thuyết (>60 tiếng) ở dòng {lq[:10]}")
    run, runs = [], []
    for n, l in paras:
        s = l.strip()
        if s.startswith("“") and s.endswith("”") and s.count("“") == 1: run.append(n)
        else:
            if len(run) >= 7: runs.append(f"{run[0]}–{run[-1]}")
            run = []
    if len(run) >= 7: runs.append(f"{run[0]}–{run[-1]}")
    if runs: canh.append(f"§15.2 Chuỗi thoại ≥7 lượt không lời dẫn ở dòng {runs}")
    toks = re.findall(r'\w+', " ".join(l for _, l in paras).lower())
    grams = collections.Counter(tuple(toks[i:i + 5]) for i in range(len(toks) - 4))
    rep = [" ".join(g) for g, v in grams.items() if v >= 2 and sum(w not in STOP for w in g) >= 5]
    if rep: canh.append(f"§13 Cụm 5 tiếng lặp lại (kiểm tay xem có phải câu châm biếm/so sánh lặp): {rep[:12]}")

    # §5 Quy tắc một phút (đo gần đúng): quãng lời kể dài >250 tiếng không có câu thoại nào
    quiet, acc, start, prev = [], 0, None, None
    for n, l in paras:
        if "“" in l:
            if acc > 250: quiet.append(f"{start}–{prev} ({acc} tiếng)")
            acc, start = 0, None
        else:
            if start is None: start = n
            acc += wc(l); prev = n
    if acc > 250: quiet.append(f"{start}–{prev} ({acc} tiếng)")
    if quiet: canh.append(f"§5 Quãng >250 tiếng không có thoại (đọc tay xem có thông tin mới/xung đột không): {quiet}")

    # §17 Độ trùng mở đầu với truyện cũ (đo gần đúng trên 220 tiếng đầu)
    if so_voi:
        g_new = grams5(body_words(t)[:220])
        for old in so_voi:
            try:
                g_old = grams5(body_words(open(old, encoding="utf-8").read())[:220])
            except OSError:
                tk.append(f"Không mở được file so sánh {old}"); continue
            share = len(g_new & g_old) / max(len(g_new), 1)
            if share > 0.20:
                canh.append(f"§17 Mở đầu trùng {share:.0%} cụm 5 tiếng với {old}: viết lại cho khác")

    for title, items in (("LỖI", loi), ("CẢNH BÁO", canh), ("THAM KHẢO", tk)):
        print(f"\n{title} ({len(items)})")
        for x in items: print("  -", x)

    # In sẵn phần cần kiểm tay
    body0 = " ".join(l for _, l in paras if not l.lstrip().startswith("《")).split()
    print("\n--- KIỂM TAY §4: 110 tiếng đầu (≈30 giây)")
    print(" ".join(body0[:110]))
    print("\n--- KIỂM TAY §7: đoạn cuối mỗi chương (móc)")
    for name, body in chs:
        last = [l for l in body.split("\n") if l.strip()]
        if last: print(f"[{name[:30]}] {last[-1][:160]}")
    print("\n--- KIỂM TAY §11, §16: 900 tiếng cuối truyện")
    print("Soát: phản diện có câu tự nhận vì sao thua? Hậu quả có ngang hoặc hơn thứ đã gây ra? Câu cuối gọi lại mở đầu?")
    print(" ".join(body0[-900:]))
    print("\nKẾT QUẢ MÁY ĐO:", "KHÔNG CÓ LỖI" if not loi else f"CÒN {len(loi)} LỖI, phải sửa")
    print("Còn phải kiểm tay: §1.2, §2–§14 (§5 script chỉ đo gần đúng), §15.2, §15.3 đại từ, §15.4 nhạc nền, §16–§19, theo câu hỏi §20 của Master Rules v3.")
    return 1 if loi else 0

if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
```
