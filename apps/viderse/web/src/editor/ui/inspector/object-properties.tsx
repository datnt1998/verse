"use client";

import { useEffect, useMemo, useState } from "react";
import { RotateCcw } from "lucide-react";
import { Input } from "@/components/ui/input";
import { executeCommands } from "@editor/commands";
import type { CompositionItem, LottieItem, TimelineItem } from "@editor/types/timeline";
import { useCompositionsStore } from "@editor/features/timeline/stores/compositions-store";
import { normalizeHexColor } from "@editor/features/keyframes/utils/color-keyframes";
import { getCompositionControlSourceValue, sanitizeCompositionControlSchema } from "@editor/shared/utils/composition-controls";
import { resolveMediaUrl } from "@editor/features/timeline/deps/media-library-resolver";
import { extractLottieAnimation, extractLottieManifest, parseLottieMetadata } from "@editor/infrastructure/lottie/lottie-metadata";
import { extractLottieTextLayers } from "@editor/infrastructure/lottie/lottie-text";
import { extractLottieColorLayers } from "@editor/infrastructure/lottie/lottie-color";
import { extractLottieValueSlots } from "@editor/infrastructure/lottie/lottie-slots";
import { ColorField, EditorIconButton, EditorSelect, NumberField, PropertyRow, SectionHeader } from "@/editor/ui/controls";
import { toast } from "@/editor/ui/toast";
import { announce } from "@/editor/ui/shell/announcer";

/** Every persisted change uses the command executor, including removing an override. */
function patch(item: TimelineItem, updates: Record<string, unknown>, clearFields?: string[]): boolean {
  const result = executeCommands([{ type: "item.update", params: { id: item.id, updates, ...(clearFields?.length ? { clearFields } : {}) } }]);
  if (!result.ok) {
    const message = result.issues.find(issue => /[À-ỹ]/u.test(issue.message))?.message ?? "Không thể cập nhật thuộc tính đối tượng.";
    toast.error(message);
    announce(message, "assertive");
    return false;
  }
  return true;
}

/** Transient typing only; the command store remains the source of truth. */
function TextValue({ label, value, onCommit }: { label: string; value: string; onCommit: (value: string) => void }) {
  const [draft, setDraft] = useState<string | null>(null);
  const commit = () => {
    if (draft !== null && draft !== value) onCommit(draft);
    setDraft(null);
  };
  return <Input aria-label={label} value={draft ?? value} className="h-7 min-w-0 border-input px-2 text-xs md:text-xs"
    onChange={event => setDraft(event.target.value)} onBlur={commit}
    onKeyDown={event => {
      if (event.key === "Enter") { event.preventDefault(); commit(); }
      if (event.key === "Escape") { event.preventDefault(); setDraft(null); }
    }} />;
}

const BOOLEAN_OPTIONS = [{ value: "on", label: "Bật" }, { value: "off", label: "Tắt" }];
function BooleanValue({ label, value, disabled, onChange }: { label: string; value: boolean; disabled?: boolean; onChange: (value: boolean) => void }) {
  return <EditorSelect label={label} value={value ? "on" : "off"} disabled={disabled} options={BOOLEAN_OPTIONS} onValueChange={value => onChange(value === "on")} />;
}


function CompositionProperties({ item }: { item: CompositionItem }) {
  const composition = useCompositionsStore(state => state.compositionById[item.compositionId]);
  const schema = useMemo(() => composition ? sanitizeCompositionControlSchema(composition.compositionControls, composition.items) : undefined, [composition]);
  if (!composition) return <SectionHeader title="Nội dung lồng ghép"><p role="status" className="text-xs text-muted-foreground">Không tìm thấy nội dung nguồn của đối tượng.</p></SectionHeader>;
  if (!schema) return null;
  const change = (id: string, value?: string) => {
    const overrides = { ...item.compositionControlOverrides };
    if (value === undefined) delete overrides[id];
    else overrides[id] = value;
    patch(item, { compositionControlOverrides: overrides });
  };
  return <SectionHeader title="Nội dung lồng ghép">
    <p className="text-xs text-muted-foreground">Các thay đổi chỉ áp dụng cho đối tượng này, không sửa bản gốc.</p>
    {schema.controls.map(control => {
      const value = item.compositionControlOverrides?.[control.id] ?? getCompositionControlSourceValue(composition.items, control);
      return <PropertyRow key={control.id} label={control.name} action={<EditorIconButton icon={RotateCcw} label={`Khôi phục ${control.name}`} disabled={item.compositionControlOverrides?.[control.id] === undefined} onClick={() => change(control.id)} />}>
        {control.kind === "color" ? <ColorField label={control.name} value={value} onChange={value => change(control.id, value)} /> : <TextValue label={control.name} value={value} onCommit={value => change(control.id, value)} />}
      </PropertyRow>;
    })}
  </SectionHeader>;
}

type LottieSource = { key: string; bytes?: Uint8Array; error?: string };
function LottieProperties({ item }: { item: LottieItem }) {
  const sourceKey = `${item.mediaId ?? ""}\n${item.src}`;
  const [source, setSource] = useState<LottieSource | null>(null);
  useEffect(() => {
    const controller = new AbortController();
    const load = async () => {
      try {
        const src = item.mediaId ? await resolveMediaUrl(item.mediaId) : item.src;
        if (controller.signal.aborted) return;
        if (!src) throw new Error("missing source");
        const response = await fetch(src, { signal: controller.signal });
        if (!response.ok) throw new Error("source unavailable");
        const bytes = new Uint8Array(await response.arrayBuffer());
        if (!extractLottieAnimation(bytes)) throw new Error("invalid animation");
        if (!controller.signal.aborted) setSource({ key: sourceKey, bytes });
      } catch {
        if (!controller.signal.aborted) setSource({ key: sourceKey, error: "Không đọc được nội dung hoạt ảnh. Hãy kiểm tra tệp nguồn." });
      }
    };
    void load();
    return () => controller.abort();
  }, [sourceKey, item.mediaId, item.src]);
  const bytes = source?.key === sourceKey ? source.bytes : undefined;
  const manifest = useMemo(() => bytes ? extractLottieManifest(bytes) : null, [bytes]);
  const animation = useMemo(() => bytes ? extractLottieAnimation(bytes, false, item.animationId) : null, [bytes, item.animationId]);
  const editable = useMemo(() => ({ text: extractLottieTextLayers(animation), colors: extractLottieColorLayers(animation), slots: extractLottieValueSlots(animation) }), [animation]);
  const maxFrame = Math.max(0, Math.floor(item.totalFrames) - 1);
  const start = Math.max(0, Math.min(maxFrame, item.segmentStart ?? 0));
  const end = Math.max(start, Math.min(maxFrame, item.segmentEnd ?? maxFrame));
  const reset = (field: "textOverrides" | "colorOverrides" | "slotOverrides", key: string) => {
    const overrides: Record<string, string | number | [number, number]> = { ...item[field] };
    delete overrides[key];
    patch(item, { [field]: overrides });
  };
  const resetButton = (field: "textOverrides" | "colorOverrides" | "slotOverrides", key: string, label: string) => <EditorIconButton icon={RotateCcw} label={`Khôi phục ${label}`} disabled={item[field]?.[key] === undefined} onClick={() => reset(field, key)} />;
  const selectAnimation = (value: string) => {
    const entry = manifest?.animations[Number(value)];
    if (!bytes || !entry) return;
    const metadata = parseLottieMetadata(extractLottieAnimation(bytes, false, entry.id));
    if (!metadata || ![metadata.width, metadata.height, metadata.frameRate, metadata.totalFrames].every(value => Number.isFinite(value) && value > 0)) {
      toast.error("Không đọc được thông số hoạt ảnh đã chọn.");
      return;
    }
    patch(item, { animationId: entry.id, frameRate: metadata.frameRate, totalFrames: metadata.totalFrames, sourceWidth: metadata.width, sourceHeight: metadata.height }, ["segmentStart", "segmentEnd", "textOverrides", "colorOverrides", "slotOverrides", "themeId"]);
  };
  return <>
    <SectionHeader title="Phát hoạt ảnh">
      <PropertyRow label="Tốc độ"><NumberField label="Tốc độ hoạt ảnh" value={item.speed ?? 1} min={0.1} max={10} step={0.1} unit="×" onChange={speed => patch(item, { speed })} /></PropertyRow>
      <PropertyRow label="Phát ngược"><BooleanValue label="Phát ngược hoạt ảnh" value={item.reversed ?? false} onChange={reversed => patch(item, { reversed })} /></PropertyRow>
      <PropertyRow label="Lặp lại"><BooleanValue label="Lặp hoạt ảnh" value={item.loop ?? true} onChange={loop => patch(item, { loop })} /></PropertyRow>
      <PropertyRow label="Kiểu lặp"><EditorSelect label="Kiểu lặp hoạt ảnh" value={item.loopMode ?? "loop"} disabled={item.loop === false} options={[{ value: "loop", label: "Từ đầu" }, { value: "pingpong", label: "Qua lại" }]} onValueChange={loopMode => { if (loopMode === "loop" || loopMode === "pingpong") patch(item, { loopMode }); }} /></PropertyRow>
      <PropertyRow label="Khung đầu"><NumberField label="Khung đầu hoạt ảnh" value={start} min={0} max={end} step={1} precision={0} onChange={segmentStart => patch(item, { segmentStart: Math.round(segmentStart) })} /></PropertyRow>
      <PropertyRow label="Khung cuối"><NumberField label="Khung cuối hoạt ảnh" value={end} min={start} max={maxFrame} step={1} precision={0} onChange={segmentEnd => patch(item, { segmentEnd: Math.round(segmentEnd) })} /></PropertyRow>
      <p className="text-xs text-muted-foreground">Khung hình tính theo hoạt ảnh nguồn, bắt đầu từ 0. Không lặp sẽ giữ khung cuối; chọn cùng khung đầu và cuối để đứng hình.</p>
      {manifest && manifest.animations.length > 1 && <PropertyRow label="Hoạt ảnh"><EditorSelect label="Hoạt ảnh trong tệp" value={String(Math.max(0, manifest.animations.findIndex(entry => entry.id === item.animationId)))} options={manifest.animations.map((_, index) => ({ value: String(index), label: `Hoạt ảnh ${index + 1}` }))} onValueChange={selectAnimation} /></PropertyRow>}
      {!!manifest?.themes.length && <PropertyRow label="Bảng màu"><EditorSelect label="Bảng màu hoạt ảnh" value={item.themeId && manifest.themes.includes(item.themeId) ? String(manifest.themes.indexOf(item.themeId)) : "original"} options={[{ value: "original", label: "Bản gốc" }, ...manifest.themes.map((_, index) => ({ value: String(index), label: `Bảng màu ${index + 1}` }))]} onValueChange={value => {
        if (value === "original") patch(item, {}, ["themeId"]);
        else { const themeId = manifest.themes[Number(value)]; if (themeId) patch(item, { themeId }); }
      }} /></PropertyRow>}
    </SectionHeader>
    <SectionHeader title="Nội dung hoạt ảnh">
      {!bytes && <p role="status" className="text-xs text-muted-foreground">{source?.key === sourceKey && source.error ? source.error : "Đang đọc các thuộc tính có thể chỉnh…"}</p>}
      {bytes && !editable.text.length && !editable.colors.length && !editable.slots.length && <p className="text-xs text-muted-foreground">Tệp này không có nội dung chữ, màu đơn hoặc giá trị có thể chỉnh.</p>}
      {editable.text.map((layer, index) => {
        const label = `Nội dung ${index + 1}`;
        return <PropertyRow key={layer.key} label={label} action={resetButton("textOverrides", layer.key, label)}><TextValue label={label} value={item.textOverrides?.[layer.key] ?? layer.text} onCommit={value => patch(item, { textOverrides: { ...item.textOverrides, [layer.key]: value } })} /></PropertyRow>;
      })}
      {editable.colors.map((layer, index) => {
        const label = `Màu ${index + 1}`;
        return <PropertyRow key={layer.key} label={label} action={resetButton("colorOverrides", layer.key, label)}><ColorField label={label} value={item.colorOverrides?.[layer.key] ?? layer.color} onChange={value => {
          const color = normalizeHexColor(value);
          if (!color || color.length !== 7) { toast.error("Nhập màu đơn dạng #RRGGBB, không kèm độ trong suốt."); return; }
          patch(item, { colorOverrides: { ...item.colorOverrides, [layer.key]: color } });
        }} /></PropertyRow>;
      })}
      {editable.slots.map((slot, index) => {
        const label = `Giá trị ${index + 1}`;
        const override = item.slotOverrides?.[slot.id];
        const update = (value: number | [number, number]) => {
          if ((typeof value === "number" ? [value] : value).every(Number.isFinite)) patch(item, { slotOverrides: { ...item.slotOverrides, [slot.id]: value } });
        };
        if (slot.type === "scalar") {
          const value = typeof override === "number" && Number.isFinite(override) ? override : slot.value;
          return Number.isFinite(value) ? <PropertyRow key={slot.id} label={label} action={resetButton("slotOverrides", slot.id, label)}><NumberField label={label} value={value} step={0.1} onChange={update} /></PropertyRow> : null;
        }
        const value = Array.isArray(override) && override.length === 2 && override.every(Number.isFinite) ? override : slot.value;
        if (!value.every(Number.isFinite)) return null;
        return <PropertyRow key={slot.id} label={label} action={resetButton("slotOverrides", slot.id, label)}><div className="grid grid-cols-2 gap-1"><NumberField label={`${label}, chiều ngang`} value={value[0]} step={0.1} onChange={x => update([x, value[1]])} /><NumberField label={`${label}, chiều dọc`} value={value[1]} step={0.1} onChange={y => update([value[0], y])} /></div></PropertyRow>;
      })}
      {!!editable.colors.length && <p className="text-xs text-muted-foreground">Đổi màu đơn sẽ cố định màu đó nếu màu gốc có chuyển động. Màu chuyển sắc chỉ đổi được qua bảng màu do tệp cung cấp.</p>}
    </SectionHeader>
  </>;
}

/** Mount once beside Transform/Text/Audio/Color. No project mutation occurs on mount. */
export function ObjectPropertiesSection({ item }: { item: TimelineItem }) {
  return <>
    <SectionHeader title="Đối tượng"><PropertyRow label="Tên"><TextValue key={item.id} label="Tên đối tượng" value={item.label} onCommit={value => { const label = value.trim(); if (label) patch(item, { label }); }} /></PropertyRow></SectionHeader>
    {item.type === "lottie" && <LottieProperties key={item.id} item={item} />}
    {item.type === "composition" && <CompositionProperties item={item} />}
  </>;
}
