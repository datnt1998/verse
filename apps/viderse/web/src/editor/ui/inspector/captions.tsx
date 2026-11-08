"use client";

import { useRef, useState, type KeyboardEvent } from "react";
import { toast } from "@/editor/ui/toast";
import { ChevronLeft, ChevronRight, FileUp, Trash2 } from "lucide-react";
import type { SubtitleSegmentItem, TextItem, TimelineItem } from "@editor/types/timeline";
import { executeCommands } from "@editor/commands";
import type { SerializedCommand } from "@editor/commands";
import { useTimelineStore } from "@editor/features/timeline/stores/timeline-store";
import { parseCaptionTiming, type ParsedCaptionTiming } from "@editor/features/captions/parsers";
import { activeCaptionWordIndex } from "@editor/features/captions/word-timing";
import { CAPTION_ANIMATION_PRESETS } from "@editor/donor/openreel/text/text-animation-presets";
import { usePlaybackStore } from "@editor/shared/state/playback";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { EditorIconButton, EditorSelect, NumberField, PropertyRow, SectionHeader } from "@/editor/ui/controls";
import { announce } from "@/editor/ui/shell/announcer";
import { formatTimecode, parseTimecode } from "./timecode";


type CaptionItem = TextItem | SubtitleSegmentItem;

const PAGE_SIZE = 20;
const ACCEPTED_FILES = ".srt,.vtt,.json,.json3";
const GENERIC_ERROR = "Không thể áp dụng thay đổi.";
const VIETNAMESE_TEXT = /[À-ỹ]/u;
const inputClass = "h-7 min-w-0 rounded-md border-ring/50 bg-background text-xs [@media(pointer:coarse)]:h-11";

function fail(message: string) {
  toast.error(message);
  announce(message, "assertive");
}

/** Runs public commands; engine issues already written in Vietnamese are surfaced, others become a generic message. */
function run(commands: SerializedCommand[], success: string): boolean {
  const result = executeCommands(commands);
  if (!result.ok) {
    fail(result.issues.find((issue) => VIETNAMESE_TEXT.test(issue.message))?.message ?? GENERIC_ERROR);
    return false;
  }
  announce(success);
  return true;
}

function detectFormat(fileName: string): "srt" | "vtt" | "json3" | null {
  const extension = fileName.split(".").pop()?.toLowerCase();
  if (extension === "srt" || extension === "vtt") return extension;
  return extension === "json" || extension === "json3" ? "json3" : null;
}

function DraftInput({ label, value, onCommit, className, numeric }: { label: string; value: string; onCommit: (value: string) => void; className?: string; numeric?: boolean }) {
  const [draft, setDraft] = useState<string | null>(null);
  const commit = () => {
    if (draft !== null && draft !== value) onCommit(draft);
    setDraft(null);
  };
  const onKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === "Enter") { event.preventDefault(); commit(); }
    else if (event.key === "Escape") { event.preventDefault(); setDraft(null); }
  };
  return <Input aria-label={label} value={draft ?? value} onChange={(e) => setDraft(e.target.value)} onBlur={commit} onKeyDown={onKeyDown} className={`${inputClass} ${numeric ? "tabular-nums" : ""} ${className ?? ""}`} />;
}

function WordRow({ item, index, fps, active }: { item: CaptionItem; index: number; fps: number; active: boolean }) {
  const word = item.words?.[index];
  if (!word) return null;
  const position = `Từ ${index + 1}`;
  const update = (patch: { text?: string; startFrame?: number; endFrame?: number }, success: string) =>
    run([{ type: "caption.updateWord", params: { itemId: item.id, index, word: patch } }], success);
  const setFrame = (key: "startFrame" | "endFrame", value: number, name: string) => {
    const frame = Math.round(value);
    if (frame !== word[key]) update(key === "startFrame" ? { startFrame: frame } : { endFrame: frame }, `${position}: ${name} khung ${frame}`);
  };
  const setTimecode = (key: "startFrame" | "endFrame", text: string, name: string) => {
    const frame = parseTimecode(text, fps);
    if (frame === null) return fail("Timecode không hợp lệ. Nhập theo dạng phút:giây:khung, ví dụ 00:03:12.");
    if (frame < 0 || frame > item.durationInFrames) return fail(`Timecode nằm ngoài thời lượng mục (${formatTimecode(item.durationInFrames, fps)}).`);
    setFrame(key, frame, name);
  };
  const remove = () => {
    const words = (item.words ?? []).filter((_, at) => at !== index);
    run([{ type: "caption.setWords", params: { itemId: item.id, words, estimated: item.wordsEstimated ?? false } }], `Đã xoá ${position.toLowerCase()}: ${word.text}`);
  };
  return (
    <li aria-current={active ? "true" : undefined} className={`space-y-1.5 rounded-md border p-2 ${active ? "border-primary bg-primary/10" : "border-border"}`}>
      <div className="flex items-center gap-1.5">
        <span className="w-6 shrink-0 text-right text-[11px] tabular-nums text-muted-foreground">{index + 1}</span>
        <DraftInput
          label={`${position}: nội dung`}
          value={word.text}
          onCommit={(text) => (text.trim() ? update({ text: text.trim() }, `${position}: đã đổi nội dung thành ${text.trim()}`) : fail("Từ không được để trống."))}
          className="flex-1"
        />
        <EditorIconButton icon={Trash2} label={`Xoá ${position.toLowerCase()}`} onClick={remove} />
      </div>
      <div className="grid grid-cols-2 gap-2 pl-7">
        <div className="space-y-1">
          <NumberField label={`${position}: khung bắt đầu`} value={word.startFrame} min={0} max={word.endFrame - 1} step={1} unit="f" onChange={(value) => setFrame("startFrame", value, "bắt đầu")} />
          <DraftInput label={`${position}: timecode bắt đầu`} value={formatTimecode(word.startFrame, fps)} onCommit={(text) => setTimecode("startFrame", text, "bắt đầu")} numeric />
        </div>
        <div className="space-y-1">
          <NumberField label={`${position}: khung kết thúc`} value={word.endFrame} min={word.startFrame + 1} max={item.durationInFrames} step={1} unit="f" onChange={(value) => setFrame("endFrame", value, "kết thúc")} />
          <DraftInput label={`${position}: timecode kết thúc`} value={formatTimecode(word.endFrame, fps)} onCommit={(text) => setTimecode("endFrame", text, "kết thúc")} numeric />
        </div>
      </div>
    </li>
  );
}

function WordList({ item, fps }: { item: CaptionItem; fps: number }) {
  const words = item.words ?? [];
  const activeIndex = usePlaybackStore((state) => activeCaptionWordIndex(words, state.currentFrame - item.from));
  const [requestedPage, setPage] = useState(0);
  const pageCount = Math.max(1, Math.ceil(words.length / PAGE_SIZE));
  const page = Math.min(requestedPage, pageCount - 1);
  const first = page * PAGE_SIZE;
  const goto = (next: number) => {
    setPage(next);
    announce(`Trang ${next + 1} trên ${pageCount}`);
  };
  if (!words.length) return <p className="text-xs text-muted-foreground">Chưa có dữ liệu từ. Nhập tệp .srt, .vtt hoặc .json3 để tạo danh sách từ.</p>;
  return (
    <div className="space-y-2">
      <div className="flex items-center gap-1">
        <EditorIconButton icon={ChevronLeft} label="Trang trước" disabled={page === 0} onClick={() => goto(page - 1)} />
        <span className="flex-1 text-center text-[11px] tabular-nums text-muted-foreground">Từ {first + 1}–{Math.min(words.length, first + PAGE_SIZE)} / {words.length}</span>
        <EditorIconButton icon={ChevronRight} label="Trang sau" disabled={page >= pageCount - 1} onClick={() => goto(page + 1)} />
      </div>
      {activeIndex >= 0 && Math.floor(activeIndex / PAGE_SIZE) !== page && (
        <Button type="button" variant="outline" size="sm" className="h-7 w-full text-xs [@media(pointer:coarse)]:h-11" onClick={() => goto(Math.floor(activeIndex / PAGE_SIZE))}>
          Đến từ đang phát ({activeIndex + 1})
        </Button>
      )}
      <ul aria-label="Danh sách từ" className="space-y-1.5">
        {words.slice(first, first + PAGE_SIZE).map((_, offset) => (
          <WordRow key={`${item.id}:${first + offset}`} item={item} index={first + offset} fps={fps} active={first + offset === activeIndex} />
        ))}
      </ul>
    </div>
  );
}

function ImportButton({ item, fps }: { item: CaptionItem; fps: number }) {
  const input = useRef<HTMLInputElement>(null);
  const [busy, setBusy] = useState(false);
  const importFile = async (file: File) => {
    const format = detectFormat(file.name);
    if (!format) return fail("Định dạng không được hỗ trợ. Chọn tệp .srt, .vtt hoặc .json3.");
    let parsed: ParsedCaptionTiming;
    try {
      parsed = parseCaptionTiming(await file.text(), format, fps);
    } catch (error) {
      return fail(error instanceof SyntaxError ? "Tệp không phải JSON hợp lệ." : "Không thể đọc tệp phụ đề. Kiểm tra lại định dạng tệp.");
    }
    if (!parsed.words.length) return fail("Tệp không chứa từ hoặc dòng phụ đề nào.");
    if (parsed.durationInFrames > item.durationInFrames) {
      return fail(`Phụ đề dài ${parsed.durationInFrames} khung, vượt thời lượng mục (${item.durationInFrames} khung). Hãy kéo dài mục hoặc dùng tệp ngắn hơn.`);
    }
    run([{ type: "caption.importTimed", params: { itemId: item.id, data: parsed } }], `Đã nhập ${parsed.words.length} từ từ tệp ${file.name}${parsed.estimated ? ", thời gian từng từ được ước lượng" : ""}`);
  };
  return (
    <>
      <input
        ref={input}
        type="file"
        accept={ACCEPTED_FILES}
        hidden
        onChange={async (e) => {
          const file = e.target.files?.[0];
          e.target.value = "";
          if (!file) return;
          setBusy(true);
          try { await importFile(file); } finally { setBusy(false); }
        }}
      />
      <Button type="button" variant="outline" size="sm" disabled={busy} className="h-7 w-full gap-1.5 text-xs [@media(pointer:coarse)]:h-11" onClick={() => input.current?.click()}>
        <FileUp aria-hidden className="size-3.5" />
        {busy ? "Đang đọc tệp…" : "Nhập tệp SRT / VTT / json3"}
      </Button>
    </>
  );
}

function CaptionsEditor({ item }: { item: CaptionItem }) {
  const fps = useTimelineStore((state) => state.fps);
  const preset = item.captionPreset ?? "none";
  const wordCount = item.words?.length ?? 0;
  return (
    <div data-tour="editor-caption-editor">
      <SectionHeader title="Phụ đề theo từ" action={item.wordsEstimated && wordCount > 0 ? <Badge variant="outline" title="Thời gian từng từ được chia theo độ dài chữ vì tệp không có mốc từng từ." className="text-[10px]">Ước lượng</Badge> : undefined}>
        <PropertyRow label="Hiệu ứng từ">
          <EditorSelect
            aria-label="Hiệu ứng từng từ"
            value={preset}
            onValueChange={(id) => {
              const next = CAPTION_ANIMATION_PRESETS.find((option) => option.id === id);
              if (next) run([{ type: "caption.setPreset", params: { itemId: item.id, preset: next.id } }], `Hiệu ứng phụ đề: ${next.name}`);
            }}
            options={CAPTION_ANIMATION_PRESETS.map((option) => ({ value: option.id, label: option.name }))}
          />
        </PropertyRow>
        <ImportButton item={item} fps={fps} />
        {item.wordsEstimated && wordCount > 0 && <p className="text-[11px] text-muted-foreground">Mốc thời gian từng từ là ước lượng. Chỉnh từng từ bên dưới nếu cần khớp lời nói.</p>}
        <WordList item={item} fps={fps} />
      </SectionHeader>
    </div>
  );
}

export function CaptionsSection({ item }: { item: TimelineItem }) {
  if (item.type !== "text" && item.type !== "subtitle") return null;
  return <CaptionsEditor item={item} />;
}
