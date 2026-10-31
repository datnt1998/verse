"use client";

import { useCallback, useEffect, useId, useMemo, useRef, useState, type ReactNode } from "react";
import { AlignCenter, AlignLeft, AlignRight, AlignVerticalJustifyCenter, AlignVerticalJustifyEnd, AlignVerticalJustifyStart, ChevronDown, Italic, Underline } from "lucide-react";
import type { SubtitleSegmentItem, TextItem, TimelineItem } from "@editor/types/timeline";
import type { TextFontWeight, TextHorizontalAlign, TextShadow, TextStroke, TextVerticalAlign } from "@editor/types/text";
import type { TextMotionEffectBase, TextMotionSpec } from "@editor/types/text-motion";
import { useTimelineStore } from "@editor/features/timeline/stores/timeline-store";
import { DEFAULT_TEXT_FONT_FAMILY, FONT_CATALOG } from "@editor/shared/typography/font-catalog";
import { loadFont } from "@editor/shared/typography/font-loader";
import {
  TEXT_MOTION_IN_PRESET_IDS,
  TEXT_MOTION_LOOP_PRESET_IDS,
  TEXT_MOTION_OUT_PRESET_IDS,
  createTextMotionEffect,
  type TextMotionInPresetId,
  type TextMotionLoopPresetId,
  type TextMotionOutPresetId,
} from "@editor/shared/typography/text-motion";
import { Textarea } from "@/components/ui/textarea";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { ColorField, EditorIconButton, EditorSelect, NumberField, PropertyRow, SectionHeader } from "@/editor/ui/controls";
import { announce } from "@/editor/ui/shell/announcer";
import { AnimatedNumber } from "./fields";
import type { FontCatalogEntry } from "@editor/shared/typography/font-catalog";

type StyledTextItem = TextItem | SubtitleSegmentItem;

const WEIGHT_LABELS: Record<TextFontWeight, string> = { normal: "Thường", medium: "Vừa", semibold: "Đậm vừa", bold: "Đậm" };
const WEIGHTS = Object.keys(WEIGHT_LABELS) as TextFontWeight[];

const IN_LABELS: Record<TextMotionInPresetId, string> = {
  typewriter: "Máy chữ",
  "fade-up": "Hiện dần lên",
  rise: "Trồi lên",
  cascade: "Xếp tầng",
  pop: "Bật lên",
  "blur-in": "Từ mờ thành nét",
  "slide-mask": "Trượt qua mặt nạ",
  "wave-in": "Sóng vào",
};
const OUT_LABELS: Record<TextMotionOutPresetId, string> = {
  "fade-down": "Mờ dần đi xuống",
  sink: "Chìm xuống",
  "pop-out": "Thu nhỏ biến mất",
  "blur-out": "Từ nét thành mờ",
  "typewriter-erase": "Xoá kiểu máy chữ",
};
const LOOP_LABELS: Record<TextMotionLoopPresetId, string> = { pulse: "Nhịp đập", wave: "Sóng", shimmer: "Lấp lánh", swing: "Đung đưa" };

const DEFAULT_SHADOW: TextShadow = { offsetX: 0, offsetY: 3, blur: 6, color: "#000000" };
const DEFAULT_STROKE: TextStroke = { width: 2, color: "#111827" };
const DEFAULT_BACKGROUND = "#000000";

const HORIZONTAL_ALIGNS: { value: TextHorizontalAlign; label: string; icon: typeof AlignLeft }[] = [
  { value: "left", label: "Căn trái", icon: AlignLeft },
  { value: "center", label: "Căn giữa", icon: AlignCenter },
  { value: "right", label: "Căn phải", icon: AlignRight },
];
const VERTICAL_ALIGNS: { value: TextVerticalAlign; label: string; icon: typeof AlignLeft }[] = [
  { value: "top", label: "Căn trên", icon: AlignVerticalJustifyStart },
  { value: "middle", label: "Căn giữa theo chiều dọc", icon: AlignVerticalJustifyCenter },
  { value: "bottom", label: "Căn dưới", icon: AlignVerticalJustifyEnd },
];

function update(item: StyledTextItem, updates: Partial<TimelineItem>) {
  useTimelineStore.getState().updateItem(item.id, updates);
}

/** Announces a continuously edited value once the user pauses, instead of on every drag tick. */
function useSettledAnnounce() {
  const timer = useRef<number | undefined>(undefined);
  useEffect(() => () => window.clearTimeout(timer.current), []);
  return useCallback((text: string) => {
    window.clearTimeout(timer.current);
    timer.current = window.setTimeout(() => announce(text), 400);
  }, []);
}

function TextContent({ item }: { item: TextItem }) {
  const [draft, setDraft] = useState<string | null>(null);
  const locked = Boolean(item.textSpans?.length);
  const commit = () => {
    if (draft !== null && draft !== item.text) {
      update(item, { text: draft });
      announce("Đã cập nhật nội dung chữ");
    }
    setDraft(null);
  };
  return (
    <div className="space-y-1">
      <label className="text-xs text-muted-foreground" htmlFor={`text-content-${item.id}`}>Nội dung</label>
      <Textarea
        id={`text-content-${item.id}`}
        value={draft ?? item.text}
        disabled={locked}
        rows={3}
        onChange={(e) => setDraft(e.target.value)}
        onBlur={commit}
        onKeyDown={(e) => {
          if (e.key === "Escape") { e.preventDefault(); setDraft(null); }
          else if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) { e.preventDefault(); commit(); }
        }}
        className="min-h-14 text-xs [@media(pointer:coarse)]:min-h-24"
        aria-describedby={`text-content-hint-${item.id}`}
      />
      <p id={`text-content-hint-${item.id}`} className="text-[11px] text-muted-foreground">
        {locked ? "Văn bản gồm nhiều đoạn, không sửa trực tiếp tại đây." : "Ctrl/⌘ + Enter để áp dụng, Esc để hoàn tác nội dung đang gõ."}
      </p>
    </div>
  );
}

const FONT_ROW_HEIGHT = 28;
const FONT_LIST_HEIGHT = 224;
const FONT_OVERSCAN = 4;
/** Same surface as the editor's 28px fields (EditorSelect). */
const FONT_TRIGGER_CLASS = "flex h-7 w-full min-w-0 items-center justify-between gap-2 rounded-md border border-input bg-transparent px-2 text-xs whitespace-nowrap shadow-xs outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 dark:bg-input/30 dark:hover:bg-input/50";

/** Font picker with keyboard filtering; renders only the visible rows so it opens almost instantly. */
function FontPicker({ value, onChange }: { value: string; onChange: (next: string) => void }) {
  const listId = useId().replace(/\W/g, "");
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [activeIndex, setActiveIndex] = useState(0);
  const [scrollTop, setScrollTop] = useState(0);
  const listRef = useRef<HTMLDivElement>(null);
  const list = useMemo(() => {
    const base: readonly FontCatalogEntry[] = FONT_CATALOG;
    const withUnknown = base.some((font) => font.value === value) ? base : [{ value, label: value, family: value, weights: [400] }, ...base];
    const needle = query.trim().toLowerCase();
    return needle ? withUnknown.filter((font) => font.label.toLowerCase().includes(needle)) : withUnknown;
  }, [query, value]);
  const clampedActive = Math.min(activeIndex, Math.max(0, list.length - 1));
  const ensureVisible = (index: number) => {
    const node = listRef.current;
    if (!node) return;
    const top = index * FONT_ROW_HEIGHT;
    const bottom = top + FONT_ROW_HEIGHT;
    if (top < node.scrollTop) node.scrollTop = top;
    else if (bottom > node.scrollTop + FONT_LIST_HEIGHT) node.scrollTop = bottom - FONT_LIST_HEIGHT;
  };
  const move = (delta: number) => {
    setActiveIndex((current) => {
      const lastIndex = Math.max(0, list.length - 1);
      const next = Math.max(0, Math.min(lastIndex, Math.min(current, lastIndex) + delta));
      ensureVisible(next);
      return next;
    });
  };
  const pick = (font: FontCatalogEntry) => {
    onChange(font.value);
    setOpen(false);
  };
  const first = Math.max(0, Math.floor(scrollTop / FONT_ROW_HEIGHT) - FONT_OVERSCAN);
  const last = Math.min(list.length - 1, Math.ceil((scrollTop + FONT_LIST_HEIGHT) / FONT_ROW_HEIGHT) + FONT_OVERSCAN);
  const rows: { font: FontCatalogEntry; index: number }[] = [];
  for (let index = first; index <= last; index += 1) rows.push({ font: list[index]!, index });
  // Load the visible rows' fonts for the preview; runs after open, so it never blocks showing the list.
  useEffect(() => {
    if (!open) return;
    for (const { font } of rows) loadFont(font.value);
  });
  return (
    <Popover open={open} onOpenChange={(next) => { setOpen(next); if (next) { setQuery(""); setActiveIndex(0); setScrollTop(0); } }}>
      <PopoverTrigger className={FONT_TRIGGER_CLASS} aria-label="Phông chữ" data-tooltip="Phông chữ">
        <span className="min-w-0 flex-1 truncate text-left">{value}</span>
        <ChevronDown aria-hidden className="size-3.5 shrink-0 text-muted-foreground" />
      </PopoverTrigger>
      <PopoverContent align="start" sideOffset={4} className="w-[var(--radix-popover-trigger-width)] min-w-44 p-1.5" onOpenAutoFocus={(event) => { event.preventDefault(); document.getElementById(`${listId}-search`)?.focus(); }}>
        <Input
          id={`${listId}-search`}
          autoFocus
          role="combobox"
          aria-expanded
          aria-controls={`${listId}-list`}
          aria-activedescendant={list[clampedActive] ? `${listId}-font-${clampedActive}` : undefined}
          aria-label="Tìm phông"
          placeholder="Tìm phông…"
          value={query}
          onChange={(event) => { setQuery(event.target.value); setActiveIndex(0); setScrollTop(0); if (listRef.current) listRef.current.scrollTop = 0; }}
          onKeyDown={(event) => {
            if (event.key === "ArrowDown") { event.preventDefault(); move(1); }
            else if (event.key === "ArrowUp") { event.preventDefault(); move(-1); }
            else if (event.key === "Enter") { event.preventDefault(); const font = list[clampedActive]; if (font) pick(font); }
          }}
          className="mb-1 h-7 min-w-0 border-ring/50 text-xs"
        />
        <div
          ref={listRef}
          role="listbox"
          id={`${listId}-list`}
          aria-label="Phông chữ"
          className="relative overflow-y-auto [scrollbar-width:thin]"
          style={{ height: FONT_LIST_HEIGHT }}
          onScroll={(event) => setScrollTop(event.currentTarget.scrollTop)}
        >
          <div style={{ height: list.length * FONT_ROW_HEIGHT }}>
            {rows.map(({ font, index }) => (
              <div
                key={font.value}
                id={`${listId}-font-${index}`}
                role="option"
                aria-selected={font.value === value}
                className={`absolute inset-x-0 flex cursor-pointer items-center truncate rounded-sm px-2 text-xs ${index === clampedActive ? "bg-accent text-accent-foreground" : ""} ${font.value === value ? "font-medium" : ""}`}
                style={{ top: index * FONT_ROW_HEIGHT, height: FONT_ROW_HEIGHT, fontFamily: `"${font.family}", sans-serif` }}
                onMouseEnter={() => setActiveIndex(index)}
                onClick={() => pick(font)}
              >
                {font.label}
              </div>
            ))}
          </div>
          {list.length === 0 && <p className="px-2 py-3 text-xs text-muted-foreground">Không tìm thấy phông.</p>}
        </div>
      </PopoverContent>
    </Popover>
  );
}

function FontControls({ item }: { item: StyledTextItem }) {
  const family = item.fontFamily ?? DEFAULT_TEXT_FONT_FAMILY;
  const weight = item.fontWeight ?? "normal";
  const settled = useSettledAnnounce();
  return (
    <>
      <PropertyRow label="Phông chữ">
        <FontPicker
          value={family}
          onChange={(next) => {
            loadFont(next);
            update(item, { fontFamily: next });
            announce(`Phông chữ: ${next}`);
          }}
        />
      </PropertyRow>
      <PropertyRow label="Độ đậm">
        <EditorSelect
          label="Độ đậm"
          value={weight}
          onValueChange={(next) => {
            if (!WEIGHTS.includes(next as TextFontWeight)) return;
            update(item, { fontWeight: next as TextFontWeight });
            announce(`Độ đậm: ${WEIGHT_LABELS[next as TextFontWeight]}`);
          }}
          options={WEIGHTS.map((value) => ({ value, label: WEIGHT_LABELS[value] }))}
        />
      </PropertyRow>
      <PropertyRow label="Kiểu chữ">
        <div className="flex gap-1">
          <EditorIconButton
            icon={Italic}
            label="Chữ nghiêng"
            aria-pressed={item.fontStyle === "italic"}
            onClick={() => {
              const next = item.fontStyle === "italic" ? "normal" : "italic";
              update(item, { fontStyle: next });
              announce(next === "italic" ? "Đã bật chữ nghiêng" : "Đã tắt chữ nghiêng");
            }}
          />
          <EditorIconButton
            icon={Underline}
            label="Gạch chân"
            aria-pressed={Boolean(item.underline)}
            onClick={() => {
              update(item, { underline: !item.underline });
              announce(item.underline ? "Đã tắt gạch chân" : "Đã bật gạch chân");
            }}
          />
        </div>
      </PropertyRow>
      <AnimatedNumber item={item} property="fontSize" label="Cỡ chữ" value={item.fontSize ?? 60} onChange={(fontSize) => { update(item, { fontSize }); settled(`Cỡ chữ: ${fontSize} px`); }} min={8} max={500} step={1} unit="px" />
      <AnimatedNumber item={item} property="lineHeight" label="Giãn dòng" value={item.lineHeight ?? 1.2} onChange={(lineHeight) => { update(item, { lineHeight }); settled(`Giãn dòng: ${lineHeight}`); }} min={0.5} max={3} step={0.05} unit="x" />
      <PropertyRow label="Giãn chữ">
        <NumberField label="Giãn chữ" value={item.letterSpacing ?? 0} onChange={(letterSpacing) => update(item, { letterSpacing })} onCommit={(letterSpacing) => announce(`Giãn chữ: ${letterSpacing} px`)} min={-50} max={100} step={0.5} unit="px" />
      </PropertyRow>
      <PropertyRow label="Màu chữ">
        <ColorField label="Màu chữ" value={item.color} onChange={(color) => { update(item, { color }); settled(`Màu chữ: ${color}`); }} />
      </PropertyRow>
    </>
  );
}

function AlignControls({ item }: { item: StyledTextItem }) {
  const horizontal = item.textAlign ?? "center";
  const vertical = item.verticalAlign ?? "middle";
  return (
    <>
      <PropertyRow label="Căn ngang">
        <ToggleGroup type="single" variant="outline" spacing={0} size="xs" aria-label="Căn lề ngang" value={horizontal} onValueChange={next => { if (next === "left" || next === "center" || next === "right") { update(item, { textAlign: next }); announce(HORIZONTAL_ALIGNS.find(align => align.value === next)?.label ?? ""); } }}>
          {HORIZONTAL_ALIGNS.map(({ value, label, icon: Icon }) => (
            <ToggleGroupItem key={value} value={value} aria-label={label} data-tooltip={label} className="px-1.5 [&_svg:not([class*='size-'])]:size-3.5">
              <Icon aria-hidden />
            </ToggleGroupItem>
          ))}
        </ToggleGroup>
      </PropertyRow>
      <PropertyRow label="Căn dọc">
        <ToggleGroup type="single" variant="outline" spacing={0} size="xs" aria-label="Căn lề dọc" value={vertical} onValueChange={next => { if (next === "top" || next === "middle" || next === "bottom") { update(item, { verticalAlign: next }); announce(VERTICAL_ALIGNS.find(align => align.value === next)?.label ?? ""); } }}>
          {VERTICAL_ALIGNS.map(({ value, label, icon: Icon }) => (
            <ToggleGroupItem key={value} value={value} aria-label={label} data-tooltip={label} className="px-1.5 [&_svg:not([class*='size-'])]:size-3.5">
              <Icon aria-hidden />
            </ToggleGroupItem>
          ))}
        </ToggleGroup>
      </PropertyRow>
    </>
  );
}

function ToggleRow({ label, checked, onToggle, children }: { label: string; checked: boolean; onToggle: () => void; children?: ReactNode }) {
  return (
    <>
      <PropertyRow label={label}>
        <Switch
          aria-label={label}
          checked={checked}
          onCheckedChange={onToggle}
        />
      </PropertyRow>
      {checked && children}
    </>
  );
}

function StrokeControls({ item }: { item: StyledTextItem }) {
  const stroke = item.stroke;
  const settled = useSettledAnnounce();
  return (
    <ToggleRow
      label="Viền chữ"
      checked={Boolean(stroke && stroke.width > 0)}
      onToggle={() => {
        const on = Boolean(stroke && stroke.width > 0);
        update(item, { stroke: on ? { color: stroke?.color ?? DEFAULT_STROKE.color, width: 0 } : { color: stroke?.color ?? DEFAULT_STROKE.color, width: DEFAULT_STROKE.width } });
        announce(on ? "Đã tắt viền chữ" : "Đã bật viền chữ");
      }}
    >
      <AnimatedNumber item={item} property="strokeWidth" label="Độ dày viền" value={stroke?.width ?? 0} onChange={(width) => { update(item, { stroke: { color: stroke?.color ?? DEFAULT_STROKE.color, width } }); settled(`Độ dày viền: ${width} px`); }} min={0} max={24} step={0.5} unit="px" />
      <PropertyRow label="Màu viền">
        <ColorField label="Màu viền" value={stroke?.color ?? DEFAULT_STROKE.color} onChange={(color) => { update(item, { stroke: { width: stroke?.width ?? DEFAULT_STROKE.width, color } }); settled(`Màu viền: ${color}`); }} />
      </PropertyRow>
    </ToggleRow>
  );
}

function ShadowControls({ item }: { item: StyledTextItem }) {
  const shadow = item.textShadow;
  const settled = useSettledAnnounce();
  const patch = (next: Partial<TextShadow>, text: string) => { update(item, { textShadow: { ...(shadow ?? DEFAULT_SHADOW), ...next } }); settled(text); };
  return (
    <ToggleRow
      label="Bóng chữ"
      checked={Boolean(shadow)}
      onToggle={() => {
        update(item, { textShadow: shadow ? undefined : DEFAULT_SHADOW });
        announce(shadow ? "Đã tắt bóng chữ" : "Đã bật bóng chữ");
      }}
    >
      <AnimatedNumber item={item} property="textShadowOffsetX" label="Bóng lệch ngang" value={shadow?.offsetX ?? 0} onChange={(offsetX) => patch({ offsetX }, `Bóng lệch ngang: ${offsetX} px`)} min={-100} max={100} step={1} unit="px" />
      <AnimatedNumber item={item} property="textShadowOffsetY" label="Bóng lệch dọc" value={shadow?.offsetY ?? 0} onChange={(offsetY) => patch({ offsetY }, `Bóng lệch dọc: ${offsetY} px`)} min={-100} max={100} step={1} unit="px" />
      <AnimatedNumber item={item} property="textShadowBlur" label="Độ mờ bóng" value={shadow?.blur ?? 0} onChange={(blur) => patch({ blur }, `Độ mờ bóng: ${blur} px`)} min={0} max={160} step={1} unit="px" />
      <PropertyRow label="Màu bóng">
        <ColorField label="Màu bóng" value={shadow?.color ?? DEFAULT_SHADOW.color} onChange={(color) => patch({ color }, `Màu bóng: ${color}`)} />
      </PropertyRow>
    </ToggleRow>
  );
}

function BackgroundControls({ item }: { item: StyledTextItem }) {
  const settled = useSettledAnnounce();
  return (
    <ToggleRow
      label="Nền chữ"
      checked={Boolean(item.backgroundColor)}
      onToggle={() => {
        update(item, { backgroundColor: item.backgroundColor ? undefined : DEFAULT_BACKGROUND });
        announce(item.backgroundColor ? "Đã tắt nền chữ" : "Đã bật nền chữ");
      }}
    >
      <PropertyRow label="Màu nền">
        <ColorField label="Màu nền" value={item.backgroundColor ?? DEFAULT_BACKGROUND} onChange={(backgroundColor) => { update(item, { backgroundColor }); settled(`Màu nền: ${backgroundColor}`); }} />
      </PropertyRow>
      <AnimatedNumber item={item} property="backgroundRadius" label="Bo góc nền" value={item.backgroundRadius ?? 0} onChange={(backgroundRadius) => { update(item, { backgroundRadius }); settled(`Bo góc nền: ${backgroundRadius} px`); }} min={0} max={200} step={1} unit="px" />
      <AnimatedNumber item={item} property="textPadding" label="Đệm quanh chữ" value={item.textPadding ?? 16} onChange={(textPadding) => { update(item, { textPadding }); settled(`Đệm quanh chữ: ${textPadding} px`); }} min={0} max={160} step={1} unit="px" />
    </ToggleRow>
  );
}

function pruneMotion(spec: TextMotionSpec): TextMotionSpec | undefined {
  const next: TextMotionSpec = {};
  if (spec.in) next.in = spec.in;
  if (spec.out) next.out = spec.out;
  if (spec.loop) next.loop = spec.loop;
  return next.in || next.out || next.loop ? next : undefined;
}

type MotionSlotProps = {
  label: string;
  options: { id: string; label: string }[];
  effect: TextMotionEffectBase & { presetId: string } | undefined;
  onPick: (id: string) => void;
  onParams: (patch: Partial<TextMotionEffectBase>) => void;
  loop?: boolean;
};

function MotionSlot({ label, options, effect, onPick, onParams, loop }: MotionSlotProps) {
  return (
    <div className="space-y-2">
      <PropertyRow label={label}>
        <EditorSelect
          label={label}
          value={effect?.presetId ?? "none"}
          onValueChange={(id) => { onPick(id === "none" ? "" : id); announce(id === "none" ? `Đã tắt ${label.toLowerCase()}` : `${label}: ${options.find((o) => o.id === id)?.label ?? id}`); }}
          options={[{ value: "none", label: "Không" }, ...options.map((option) => ({ value: option.id, label: option.label }))]}
        />
      </PropertyRow>
      {effect && (
        <>
          <PropertyRow label={loop ? "Chu kỳ" : "Thời lượng"}>
            <NumberField label={`${label}: ${loop ? "chu kỳ" : "thời lượng"}`} value={effect.durationFrames} onChange={(durationFrames) => onParams({ durationFrames })} onCommit={(v) => announce(`${label}: ${v} khung`)} min={1} max={600} step={1} unit="f" />
          </PropertyRow>
          <PropertyRow label="Trễ giữa đơn vị">
            <NumberField label={`${label}: trễ giữa đơn vị`} value={effect.staggerFrames} onChange={(staggerFrames) => onParams({ staggerFrames })} onCommit={(v) => announce(`${label}: trễ ${v} khung`)} min={0} max={60} step={1} unit="f" />
          </PropertyRow>
          <PropertyRow label="Cường độ">
            <NumberField label={`${label}: cường độ`} value={effect.intensity} onChange={(intensity) => onParams({ intensity })} onCommit={(v) => announce(`${label}: cường độ ${v}`)} min={0} max={2} step={0.1} unit="x" />
          </PropertyRow>
        </>
      )}
    </div>
  );
}

function MotionControls({ item }: { item: TextItem }) {
  const motion: TextMotionSpec = item.textMotion ?? {};
  const set = (next: TextMotionSpec) => update(item, { textMotion: pruneMotion(next) });
  return (
    <SectionHeader title="Hoạt ảnh chữ" defaultOpen={Boolean(item.textMotion)}>
      <MotionSlot
        label="Hiện vào"
        options={TEXT_MOTION_IN_PRESET_IDS.map((id) => ({ id, label: IN_LABELS[id] }))}
        effect={motion.in}
        onPick={(id) => { const presetId = TEXT_MOTION_IN_PRESET_IDS.find((v) => v === id); set({ ...motion, in: presetId ? createTextMotionEffect(presetId) : undefined }); }}
        onParams={(patch) => set({ ...motion, in: motion.in && { ...motion.in, ...patch } })}
      />
      <MotionSlot
        label="Biến mất"
        options={TEXT_MOTION_OUT_PRESET_IDS.map((id) => ({ id, label: OUT_LABELS[id] }))}
        effect={motion.out}
        onPick={(id) => { const presetId = TEXT_MOTION_OUT_PRESET_IDS.find((v) => v === id); set({ ...motion, out: presetId ? createTextMotionEffect(presetId) : undefined }); }}
        onParams={(patch) => set({ ...motion, out: motion.out && { ...motion.out, ...patch } })}
      />
      <MotionSlot
        label="Lặp liên tục"
        loop
        options={TEXT_MOTION_LOOP_PRESET_IDS.map((id) => ({ id, label: LOOP_LABELS[id] }))}
        effect={motion.loop}
        onPick={(id) => { const presetId = TEXT_MOTION_LOOP_PRESET_IDS.find((v) => v === id); set({ ...motion, loop: presetId ? createTextMotionEffect(presetId) : undefined }); }}
        onParams={(patch) => set({ ...motion, loop: motion.loop && { ...motion.loop, ...patch } })}
      />
    </SectionHeader>
  );
}

function isStyledText(item: TimelineItem): item is StyledTextItem {
  return item.type === "text" || item.type === "subtitle";
}

export function TextSection({ item }: { item: TimelineItem }) {
  if (!isStyledText(item)) return null;
  return (
    <>
      <SectionHeader title={item.type === "subtitle" ? "Kiểu phụ đề" : "Chữ"}>
        {item.type === "text" && <TextContent item={item} />}
        <FontControls item={item} />
        <AlignControls item={item} />
        <StrokeControls item={item} />
        <ShadowControls item={item} />
        <BackgroundControls item={item} />
      </SectionHeader>
      {item.type === "text" && <MotionControls item={item} />}
    </>
  );
}
