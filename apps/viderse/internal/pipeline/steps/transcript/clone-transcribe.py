#!/usr/bin/env python3
"""Speech-evidence adapter for the clone pipeline (app-managed env).

Two modes over a float32 stereo 44.1 kHz WAV (the step decodes it):

  vad  --bounded windows of `--window` content seconds + `--context` overlap:
         Silero VAD per window, intervals clipped to the window's core so no
         boundary speech is counted twice, then merged into absolute seconds.
  asr  --faster-whisper over explicit `start,end` regions (or the whole file),
         bounded slices with overlap; word timestamps on; overlapping region
         segments deduplicated by timestamp, first occurrence wins.

Progress is JSONL on stderr:
  {"event":"progress","done":<seconds>,"total":<seconds>}
  {"event":"done", ...} — also written to --report.
"""
import argparse
import hashlib
import importlib.metadata
import importlib.resources
import json
import math
import os
import resource
import struct
import sys

import numpy as np
import torch
import torchaudio

VAD_RATE = 16000


def emit(event):
    sys.stderr.write(json.dumps(event, sort_keys=True) + "\n")
    sys.stderr.flush()


def fail(message, code=2):
    emit({"event": "error", "message": str(message)})
    sys.exit(code)


def read_wav_f32(path):
    """Canonical-WAV reader: returns (memmap[N,2] float32, sample_rate)."""
    with open(path, "rb") as fh:
        riff = fh.read(12)
        if riff[0:4] != b"RIFF" or riff[8:12] != b"WAVE":
            fail("input is not a RIFF/WAVE file")
        fmt = None
        data_off = None
        data_size = None
        while True:
            head = fh.read(8)
            if len(head) < 8:
                break
            cid, size = head[0:4], struct.unpack("<I", head[4:8])[0]
            if cid == b"fmt ":
                fmt = fh.read(size)
            elif cid == b"data":
                data_off = fh.tell()
                data_size = size
                fh.seek(size, 1)
            else:
                fh.seek(size, 1)
            if size % 2:
                fh.seek(1, 1)
        if fmt is None or data_off is None:
            fail("WAVE file has no fmt/data chunk")
        audio_format, channels, rate, _br, _ba, bits = struct.unpack("<HHIIHH", fmt[:16])
        # ffmpeg emits WAVE_FORMAT_EXTENSIBLE for float: tag 0xFFFE with the
        # real subformat GUID at offset 24 (first 2 bytes = 0x0003 = IEEE float).
        if audio_format == 0xFFFE and len(fmt) >= 40:
            audio_format = struct.unpack("<H", fmt[24:26])[0]
        if audio_format != 3 or bits != 32 or channels != 2:
            fail(f"expected float32 stereo WAV, got format={audio_format} ch={channels} bits={bits}")
    audio = np.memmap(path, dtype="<f4", mode="r", offset=data_off,
                      shape=(data_size // 8, 2))
    return audio, rate


def to_mono_16k(audio, start_s, end_s):
    """Slice [start_s,end_s) of source audio → mono float32 at 16 kHz."""
    s = max(0, int(round(start_s * 44100)))
    e = min(audio.shape[0], int(round(end_s * 44100)))
    if e <= s:
        return np.zeros(0, dtype=np.float32)
    frame = torch.from_numpy(np.asarray(audio[s:e], dtype=np.float32).T.copy())
    mono = frame.mean(dim=0, keepdim=True)
    out = torchaudio.functional.resample(mono, 44100, VAD_RATE)
    return out[0].numpy()


def parse_ranges(spec):
    ranges = []
    for part in spec.split(";"):
        if not part.strip():
            continue
        a, b = part.split(",")
        ranges.append((float(a), float(b)))
    return ranges


def peak_rss_bytes():
    rss = resource.getrusage(resource.RUSAGE_SELF).ru_maxrss
    return rss if sys.platform == "darwin" else rss * 1024


VAD_PARAMS = {
    "threshold": 0.5,
    "min_speech_duration_ms": 250,
    "min_silence_duration_ms": 100,
    "speech_pad_ms": 30,
    "window_size_samples": 512,
    # Adjacent intervals closer than this merge into one speech interval.
    "merge_gap_ms": 32,
}


def run_vad(args):
    from silero_vad import get_speech_timestamps, load_silero_vad

    audio, _rate = read_wav_f32(args.input)
    duration_s = audio.shape[0] / 44100.0
    if args.ranges:
        regions = [(a, b) for a, b in parse_ranges(args.ranges) if b > a]
    else:
        regions = [(0.0, duration_s)]

    model = load_silero_vad()
    window = args.window
    context = args.context
    if window <= 0:
        fail("window must be positive")

    intervals = []
    done_s = 0.0
    total_s = sum(b - a for a, b in regions)
    for (r_start, r_end) in regions:
        # Bounded windows: `window` seconds of content + `context` lookahead.
        pos = r_start
        while pos < r_end:
            core_start = pos
            core_end = min(pos + window, r_end)
            read_start = max(r_start, core_start - context)
            read_end = min(r_end, core_end + context)
            pcm = to_mono_16k(audio, read_start, read_end)
            if pcm.shape[0] == 0:
                break
            stamps = get_speech_timestamps(
                torch.from_numpy(pcm), model, sampling_rate=VAD_RATE,
                **{k: v for k, v in VAD_PARAMS.items() if k != "merge_gap_ms"},
            )
            for st in stamps:
                a = read_start + st["start"] / VAD_RATE
                b = read_start + st["end"] / VAD_RATE
                # Clip to this window's core; a following window re-detects the
                # clipped tail from its own context overlap.
                a, b = max(a, core_start), min(b, core_end)
                if b - a >= VAD_PARAMS["min_speech_duration_ms"] / 1000.0:
                    intervals.append((a, b))
            pos = core_end
            done_s = min(total_s, done_s + min(window, r_end - core_start))
            emit({"event": "progress", "done": round(done_s, 3), "total": round(total_s, 3)})

    intervals.sort()
    merged = []
    gap = VAD_PARAMS["merge_gap_ms"] / 1000.0
    for a, b in intervals:
        if merged and a - merged[-1][1] <= gap:
            merged[-1] = (merged[-1][0], max(merged[-1][1], b))
        else:
            merged.append((a, b))

    jit = None
    try:
        model_path = str(importlib.resources.files("silero_vad.data").joinpath("silero_vad.jit"))
        if os.path.exists(model_path):
            with open(model_path, "rb") as fh:
                jit = {"file": "silero_vad.jit", "sha256": hashlib.sha256(fh.read()).hexdigest()}
    except Exception:
        jit = None

    report = {
        "event": "done",
        "mode": "vad",
        "model": {
            "name": "silero-vad",
            "version": importlib.metadata.version("silero-vad"),
            "checkpoint": jit,
        },
        "params": dict(VAD_PARAMS),
        "sampleRate": VAD_RATE,
        "windowSec": window,
        "contextSec": context,
        "durationSec": duration_s,
        "intervals": [
            {"startMs": int(round(a * 1000)), "endMs": int(round(b * 1000))}
            for a, b in merged
        ],
        "peakRssBytes": peak_rss_bytes(),
    }
    with open(args.report, "w") as fh:
        json.dump(report, fh)
    emit({"event": "done", "intervals": len(merged)})


def run_asr(args):
    from faster_whisper import WhisperModel

    audio, _rate = read_wav_f32(args.input)
    duration_s = audio.shape[0] / 44100.0
    if args.ranges:
        regions = [(a, b) for a, b in parse_ranges(args.ranges) if b > a]
    else:
        regions = [(0.0, duration_s)]
    bounded = sorted((max(0.0, a), min(b, duration_s)) for a, b in regions
                     if math.isfinite(a) and math.isfinite(b) and b > a and a < duration_s)
    regions = []
    for a, b in bounded:
        if regions and a <= regions[-1][1]:
            regions[-1] = (regions[-1][0], max(regions[-1][1], b))
        elif b > a:
            regions.append((a, b))

    threads = max(1, args.threads)
    model = WhisperModel(args.model_size, device="cpu", compute_type="int8",
                         cpu_threads=threads, num_workers=1,
                         download_root=os.environ.get("HF_HOME") or None)

    max_slice = args.slice_sec
    overlap = args.slice_overlap
    segments = []
    detected = []
    done_s = 0.0
    total_s = sum(b - a for a, b in regions)

    for (r_start, r_end) in regions:
        pos = r_start
        while pos < r_end:
            s_end = min(pos + max_slice, r_end)
            pcm = to_mono_16k(audio, pos, s_end)
            if pcm.shape[0] == 0:
                break
            it, info = model.transcribe(
                pcm,
                language=args.language or None,
                beam_size=max(1, args.beam_size),
                word_timestamps=True,
                condition_on_previous_text=False,
            )
            detected.append({"language": info.language, "probability": info.language_probability})
            for seg in it:
                # Times are slice-relative → absolute source seconds.
                seg_out = {
                    "startMs": int(round((pos + seg.start) * 1000)),
                    "endMs": int(round((pos + seg.end) * 1000)),
                    "text": seg.text.strip(),
                    "avgLogprob": seg.avg_logprob,
                    "words": [
                        {"startMs": int(round((pos + w.start) * 1000)),
                         "endMs": int(round((pos + w.end) * 1000)),
                         "text": w.word,
                         "probability": w.probability}
                        for w in (seg.words or [])
                    ],
                }
                if seg_out["text"]:
                    segments.append(seg_out)
            done_s = min(total_s, done_s + (s_end - pos))
            emit({"event": "progress", "done": round(done_s, 3), "total": round(total_s, 3)})
            if s_end >= r_end:
                break
            pos = s_end - overlap

    # Overlap dedupe by timestamp: drop a segment that starts inside the span
    # of an already-kept segment (earlier slice won that audio).
    segments.sort(key=lambda s: (s["startMs"], s["endMs"]))
    kept = []
    for seg in segments:
        if kept and seg["startMs"] < kept[-1]["endMs"] and seg["text"] == kept[-1]["text"]:
            continue
        if kept and seg["startMs"] < kept[-1]["startMs"]:
            continue
        kept.append(seg)

    lang = None
    if detected:
        best = max(detected, key=lambda d: d["probability"] or 0)
        lang = {"language": best["language"], "probability": best["probability"]}

    report = {
        "event": "done",
        "mode": "asr",
        "model": {
            "name": "faster-whisper",
            "version": importlib.metadata.version("faster-whisper"),
            "size": args.model_size,
            "computeType": "int8",
            "device": "cpu",
        },
        "languageHint": args.language or None,
        "detected": lang,
        "sliceSec": max_slice,
        "sliceOverlapSec": overlap,
        "spans": [{"startMs": round(a * 1000), "endMs": round(b * 1000)} for a, b in regions],
        "segments": kept,
        "peakRssBytes": peak_rss_bytes(),
    }
    with open(args.report, "w") as fh:
        json.dump(report, fh)
    emit({"event": "done", "segments": len(kept)})


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--mode", choices=["vad", "asr"], required=True)
    ap.add_argument("--input", required=True)
    ap.add_argument("--ranges", default="")  # "s,e;s,e" seconds
    ap.add_argument("--window", type=float, default=30.0)
    ap.add_argument("--context", type=float, default=2.0)
    ap.add_argument("--model-size", default="small")
    ap.add_argument("--beam-size", type=int, default=5)
    ap.add_argument("--language", default="")
    ap.add_argument("--slice-sec", type=float, default=600.0)
    ap.add_argument("--slice-overlap", type=float, default=5.0)
    ap.add_argument("--threads", type=int, default=4)
    ap.add_argument("--report", required=True)
    args = ap.parse_args()
    torch.set_num_threads(max(1, args.threads))
    if args.mode == "vad":
        run_vad(args)
    else:
        run_asr(args)


if __name__ == "__main__":
    main()
