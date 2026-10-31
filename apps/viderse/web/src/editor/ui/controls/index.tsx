"use client";

import { useCallback, useEffect, useId, useRef, useState, type ComponentProps, type ReactNode, type Ref } from "react";
import { Diamond, ChevronDown, type LucideIcon } from "lucide-react";
import { cn } from "cn";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectGroup, SelectItem, SelectLabel, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Slider } from "@/components/ui/slider";
import { getCommandExecutor, withCommandMergeKey } from "@editor/commands";
import { defaultPrecision, isNumberEdit, roundForDisplay } from "./number-display";

/** Dense editor field (inputs and select triggers share it): 28px, 12px values. The 44px touch box
 * comes from `hit-area` on the trigger or on a label wrapper — pseudo-elements cannot grow an
 * <input>'s own hit box. `md:text-xs` overrides the Input primitive's `md:text-sm`. */
const fieldClass = "h-7 min-w-0 border-ring/50 px-2 text-xs md:text-xs tabular-nums disabled:opacity-100 disabled:text-muted-foreground";
/** Dialog field: same surface, 44px and 14px text like dialog inputs. */
const dialogFieldClass = "h-11 min-w-0 border-ring/50 px-3 text-sm md:text-sm";

/** Icon-only button. Its tooltip comes from the page's one `SharedTooltip` (`data-tooltip`), not a Radix tooltip per button. */
export function EditorIconButton({ icon: Icon, label, shortcut, className, ...props }: Omit<ComponentProps<typeof Button>, "children" | "size"> & { icon: LucideIcon; label: string; shortcut?: string }) {
  const title = shortcut ? `${label} (${shortcut})` : label;
  return <Button type="button" variant="ghost" data-hit-area data-tooltip={title} aria-label={title} className={cn("size-7 min-h-7 min-w-7 shrink-0 border border-transparent p-0 rounded-md text-muted-foreground focus-visible:ring-2 focus-visible:ring-ring aria-pressed:border-primary aria-pressed:bg-primary/10 aria-pressed:text-foreground disabled:opacity-100 [&_svg]:size-3.5", className)} {...props}><Icon aria-hidden /></Button>;
}

export function PropertyRow({ label, children, action }: { label: string; children: ReactNode; action?: ReactNode }) {
  // The action column is always reserved so every row's control starts and
  // ends at the same x, whether or not it has a keyframe/reset button.
  return (
    <div className="grid min-w-0 grid-cols-[minmax(70px,1fr)_minmax(0,1.15fr)_28px] items-center gap-2">
      <span className="break-words text-xs text-muted-foreground">{label}</span>
      <div className="min-w-0">{children}</div>
      <div>{action}</div>
    </div>
  );
}

export type EditorSelectOption = { value: string; label: string; disabled?: boolean };
export type EditorSelectGroup = { label: string; options: readonly EditorSelectOption[] };
export type EditorSelectProps = {
  /** Accessible name; omit only when `id` is referenced by a visible <label htmlFor>. */
  label?: string;
  id?: string;
  value: string;
  onValueChange: (value: string) => void;
  options?: readonly EditorSelectOption[];
  groups?: readonly EditorSelectGroup[];
  placeholder?: string;
  disabled?: boolean;
  /** `compact` matches inspector fields (28px, 44px on coarse pointers); `field` matches dialog inputs (44px). */
  size?: "compact" | "field";
  className?: string;
  ref?: Ref<HTMLButtonElement>;
};

/**
 * Themed replacement for native <select>, sharing the Input surface, border,
 * height and focus ring. The menu opens under the trigger, at least as wide as
 * it, aligned to the edge nearest the panel side so long labels grow inward
 * instead of being shifted by viewport collision. Option values must be
 * non-empty strings (Radix reserves "" for "no selection").
 */
export function EditorSelect({ label, id, value, onValueChange, options, groups, placeholder, disabled, size = "compact", className, ref }: EditorSelectProps) {
  const trigger = useRef<HTMLButtonElement | null>(null);
  const [open, setOpen] = useState(false);
  const [align, setAlign] = useState<"start" | "end">("start");
  // A stable ref: a new callback each render makes Radix detach/re-attach the trigger and re-render the Select twice.
  const setTriggerRef = useCallback((node: HTMLButtonElement | null) => {
    trigger.current = node;
    if (typeof ref === "function") ref(node);
    else if (ref) ref.current = node;
  }, [ref]);
  const onOpenChange = (next: boolean) => {
    const rect = trigger.current?.getBoundingClientRect();
    if (next && rect) setAlign(rect.left + rect.width / 2 > window.innerWidth / 2 ? "end" : "start");
    setOpen(next);
  };
  // Radix mounts every option even while the menu is closed, so each inspector re-render (a colour drag)
  // re-rendered every row of every select. Options mount only while open; the trigger shows the label itself.
  const selected = options?.find((option) => option.value === value) ?? groups?.flatMap((group) => group.options).find((option) => option.value === value);
  const itemClass = size === "compact" ? "min-h-7 py-1 text-xs [@media(pointer:coarse)]:min-h-11" : "min-h-11 text-sm";
  const renderOption = (option: EditorSelectOption) => (
    <SelectItem key={option.value} value={option.value} disabled={option.disabled} className={itemClass}>
      {option.label}
    </SelectItem>
  );
  return (
    <Select value={value} onValueChange={onValueChange} open={open} onOpenChange={onOpenChange} disabled={disabled}>
      <SelectTrigger ref={setTriggerRef} id={id} aria-label={label} className={cn("hit-area w-full", size === "compact" ? fieldClass : dialogFieldClass, className)}>
        <SelectValue placeholder={placeholder}>{selected?.label}</SelectValue>
      </SelectTrigger>
      <SelectContent position="popper" align={align} className="max-h-72">
        {open && <>
          {options?.map(renderOption)}
          {groups?.map((group) => (
            <SelectGroup key={group.label}>
              <SelectLabel className={size === "compact" ? "text-[11px]" : undefined}>{group.label}</SelectLabel>
              {group.options.map(renderOption)}
            </SelectGroup>
          ))}
        </>}
      </SelectContent>
    </Select>
  );
}

/** `precision`: decimals shown (default 2, 1 for degrees). Display only; stored values keep full precision. */
export type NumberFieldProps = { label: string; value: number; onChange: (value: number) => void; onCommit?: (value: number) => void; min?: number; max?: number; step?: number; unit?: string; disabled?: boolean; className?: string; precision?: number };
export function NumberField({ label, value, onChange, onCommit, min = -Infinity, max = Infinity, step = 1, unit, disabled, className, precision = defaultPrecision(unit) }: NumberFieldProps) {
  const [draft, setDraft] = useState<string | null>(null);
  const id = useId();
  const gestureCount = useRef(0);
  const origin = useRef(value);
  const drag = useRef<{ x: number; value: number; lastValue: number; moved: boolean; key: string } | null>(null);
  const normalize = (n: number) => Math.min(max, Math.max(min, Number(n.toFixed(5))));
  const round = (n: number) => roundForDisplay(n, precision);
  const shown = round(value);
  const commit = () => {
    if (draft !== null) {
      const parsed = Number(draft);
      if (draft.trim() && Number.isFinite(parsed)) {
        const next = normalize(parsed);
        if (isNumberEdit(next, value, precision)) withCommandMergeKey(null, () => onChange(next));
        onCommit?.(next);
        origin.current = next;
      }
    }
    setDraft(null);
    getCommandExecutor().closeMergeGroup();
  };
  return (
    <label htmlFor={id} className="hit-area relative block min-w-0">
      <Input type="text" inputMode="decimal" role="spinbutton" aria-label={label} aria-valuenow={shown} aria-valuemin={Number.isFinite(min) ? min : undefined} aria-valuemax={Number.isFinite(max) ? max : undefined} disabled={disabled} className={cn(fieldClass, unit && "pr-7", className)} value={draft ?? shown}
        onFocus={() => { origin.current = value; }}
        onChange={e => setDraft(e.target.value)}
        onBlur={commit}
        onKeyDown={e => {
          if (e.key === "Escape") {
            e.preventDefault();
            setDraft(null);
            if (value !== origin.current) withCommandMergeKey(null, () => onChange(origin.current));
            onCommit?.(origin.current);
            getCommandExecutor().closeMergeGroup();
          } else if (e.key === "Enter") {
            e.preventDefault();
            commit();
          } else if (e.key === "ArrowUp" || e.key === "ArrowDown") {
            e.preventDefault();
            const parsed = Number(draft ?? value);
            const next = normalize((Number.isFinite(parsed) ? parsed : value) + step * (e.shiftKey ? 10 : 1) * (e.key === "ArrowUp" ? 1 : -1));
            setDraft(null);
            withCommandMergeKey(null, () => onChange(next));
            onCommit?.(next);
            getCommandExecutor().closeMergeGroup();
          }
        }}
        onPointerDown={e => {
          if (e.button !== 0 || e.pointerType === "touch") return;
          drag.current = { x: e.clientX, value, lastValue: value, moved: false, key: `${id}:${++gestureCount.current}` };
          e.currentTarget.setPointerCapture(e.pointerId);
        }}
        onPointerMove={e => {
          const gesture = drag.current;
          if (!gesture || Math.abs(e.clientX - gesture.x) < 3) return;
          gesture.moved = true;
          const next = normalize(gesture.value + (e.clientX - gesture.x) * step * (e.shiftKey ? 10 : 1));
          gesture.lastValue = next;
          setDraft(String(round(next)));
          withCommandMergeKey(gesture.key, () => onChange(next));
        }}
        onPointerUp={() => {
          if (drag.current?.moved) {
            onCommit?.(drag.current.lastValue);
            setDraft(null);
            getCommandExecutor().closeMergeGroup();
          }
          drag.current = null;
        }}
        onPointerCancel={() => {
          const gesture = drag.current;
          if (gesture?.moved) withCommandMergeKey(gesture.key, () => onChange(gesture.value));
          drag.current = null;
          setDraft(null);
          getCommandExecutor().closeMergeGroup();
        }}
      />
      {unit && <span className="pointer-events-none absolute inset-y-0 right-2 flex items-center text-[10px] text-muted-foreground">{unit}</span>}
    </label>
  );
}

export function SliderField({ label, value, onChange, onCommit, min = 0, max = 100, step = 1, unit, disabled, precision }: NumberFieldProps) {
  const id = useId();
  const gestureCount = useRef(0);
  const gesture = useRef<string | null>(null);
  return (
    <div className="grid min-w-0 grid-cols-[minmax(0,1fr)_64px] items-center gap-2">
      <Slider aria-label={label} value={[value]} min={min} max={max} step={step} disabled={disabled}
        onPointerDownCapture={() => { gesture.current = `${id}:${++gestureCount.current}`; }}
        onValueChange={values => withCommandMergeKey(gesture.current, () => onChange(values[0]))}
        onValueCommit={values => { onCommit?.(values[0]); gesture.current = null; getCommandExecutor().closeMergeGroup(); }}
        onPointerCancel={() => { gesture.current = null; getCommandExecutor().closeMergeGroup(); }}
        className="min-h-7 data-[disabled]:opacity-100"
      />
      <NumberField label={label} value={value} onChange={onChange} onCommit={onCommit} min={min} max={max} step={step} unit={unit} disabled={disabled} precision={precision} />
    </div>
  );
}

export function KeyframeToggle({ label, active = false, onToggle, disabled }: { label: string; active?: boolean; onToggle: () => void; disabled?: boolean }) {
  return <EditorIconButton icon={Diamond} label={`${active ? "Xoá" : "Thêm"} điểm mốc ${label}`} shortcut="K" aria-pressed={active} onClick={onToggle} disabled={disabled} className={active ? "text-ed-keyframe aria-pressed:text-ed-keyframe [&_svg]:fill-current" : undefined} />;
}

/** Shortest idle gap between two live colour edits, so input and paint always get a turn. */
const COLOR_DRAG_MIN_GAP_MS = 50;
type ColorDrag = { color: string; timer: number | null; apply: (color: string) => void };
type ColorDragSlots = { pending: { current: ColorDrag | null }; readyAt: { current: number } };
function scheduleColorDrag(slots: ColorDragSlots) {
  const next = slots.pending.current;
  if (!next || next.timer !== null || !Number.isFinite(slots.readyAt.current)) return;
  next.timer = window.setTimeout(() => flushColorDrag(slots), Math.max(0, slots.readyAt.current - performance.now()));
}
function flushColorDrag(slots: ColorDragSlots) {
  const next = slots.pending.current;
  if (!next) return;
  if (next.timer !== null) clearTimeout(next.timer);
  slots.pending.current = null;
  const start = performance.now();
  slots.readyAt.current = Number.POSITIVE_INFINITY;
  next.apply(next.color);
  // The editor re-renders right after this task. Measure that and stay idle at least as long before the
  // next live edit: the swatch follows the picker at once, the preview as fast as the machine allows.
  window.setTimeout(() => {
    const now = performance.now();
    slots.readyAt.current = now + Math.max(COLOR_DRAG_MIN_GAP_MS, now - start);
    scheduleColorDrag(slots);
  }, 0);
}

export function ColorField({ label, value, onChange }: { label: string; value: string; onChange: (value: string) => void }) {
  const id = useId();
  const [draft, setDraft] = useState<string | null>(null);
  // The native picker fires `input` far faster than an edit re-renders the editor; the swatch shows the
  // latest colour immediately and only the newest pending colour is applied when the editor is ready.
  const [picking, setPicking] = useState<string | null>(null);
  const pending = useRef<ColorDrag | null>(null);
  const readyAt = useRef(0);
  useEffect(() => {
    const slots = { pending, readyAt };
    return () => flushColorDrag(slots);
  }, []);
  const pick = (color: string) => {
    setPicking(color);
    if (pending.current) { pending.current.color = color; return; }
    const apply = (next: string) => { withCommandMergeKey(`color:${id}`, () => onChange(next)); setPicking(null); };
    pending.current = { color, apply, timer: null };
    scheduleColorDrag({ pending, readyAt });
  };
  const commit = () => {
    if (draft !== null && draft !== value) withCommandMergeKey(null, () => onChange(draft));
    setDraft(null);
    getCommandExecutor().closeMergeGroup();
  };
  const swatch = picking ?? value;
  return (
    <div className="flex min-w-0 items-center gap-2">
      <label htmlFor={id} className="hit-area flex shrink-0">
        <input id={id} type="color" aria-label={label} value={/^#[0-9a-f]{6}$/i.test(swatch) ? swatch : "#ffffff"}
          onChange={e => pick(e.target.value)}
          onBlur={() => { flushColorDrag({ pending, readyAt }); getCommandExecutor().closeMergeGroup(); }}
          className="size-7 shrink-0 cursor-pointer rounded border border-ring/50 bg-background p-0.5"
        />
      </label>
      <label htmlFor={`${id}-hex`} className="hit-area min-w-0 flex-1">
        <Input id={`${id}-hex`} aria-label={`${label}, mã màu`} value={draft ?? swatch} onChange={e => setDraft(e.target.value)} onBlur={commit}
          onKeyDown={e => { if (e.key === "Enter") { e.preventDefault(); commit(); } else if (e.key === "Escape") { e.preventDefault(); setDraft(null); } }}
          className={cn(fieldClass, "font-mono")}
        />
      </label>
    </div>
  );
}

export function SectionHeader({ title, children, defaultOpen = true, action }: { title: string; children: ReactNode; defaultOpen?: boolean; action?: ReactNode }) {
  return <details open={defaultOpen} className="border-b border-border py-3"><summary className="hit-area flex min-h-7 cursor-pointer list-none items-center gap-2 text-xs font-semibold"><ChevronDown aria-hidden className="size-3.5" /><span className="flex-1">{title}</span>{action}</summary><div className="space-y-2 pt-3">{children}</div></details>;
}
