"use client";

import { useRef, useState, type ChangeEvent } from "react";
import { toast } from "sonner";
import { Eye, EyeOff, FileUp, RotateCcw, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { EditorIconButton, NumberField, PropertyRow, SectionHeader } from "@/editor/ui/controls";
import { AnimatedNumber } from "@/editor/ui/inspector/fields";
import { announce } from "@/editor/ui/shell/announcer";
import { currentEffects, describeEffectError, removeEffectWithKeyframes, runEffectAction, setEffectParam } from "@/editor/ui/inspector/effects";
import { getGpuEffect, getGpuEffectDefaultParams } from "@editor/infrastructure/gpu-effects/registry";
import { encodeLutData, parseCubeLut, resampleCubeLut } from "@editor/infrastructure/gpu-effects/lut/cube-lut";
import { useTimelineStore } from "@editor/features/timeline/stores/timeline-store";
import { useItemsStore } from "@editor/features/timeline/stores/items-store";
import { buildEffectAnimatableProperty } from "@editor/types/keyframe";
import type { ItemEffect } from "@editor/types/effects";
import type { TimelineItem } from "@editor/types/timeline";
import { assertSupportedLutSize } from "./lut-input";

const LUT_TYPE = "gpu-lut";
/** The engine embeds the LUT in the effect params as base64 rgba8, at most 33³ (see gpu-lut). */
const LUT_EMBED_SIZE = 33;
const BASIC_TYPES = ["gpu-temperature", "gpu-exposure", "gpu-contrast", "gpu-saturation"];
const BASIC_CONTROLS = [
  { type: "gpu-temperature", param: "temperature", label: "Nhiệt độ màu" },
  { type: "gpu-temperature", param: "tint", label: "Sắc xanh – hồng" },
  { type: "gpu-exposure", param: "exposure", label: "Phơi sáng" },
  { type: "gpu-contrast", param: "amount", label: "Tương phản" },
  { type: "gpu-saturation", param: "amount", label: "Bão hoà" },
] as const;

function findEffect(effects: readonly ItemEffect[], gpuEffectType: string): ItemEffect | undefined {
  return effects.find(entry => entry.effect.gpuEffectType === gpuEffectType);
}

function describeLutError(error: unknown): string {
  const message = error instanceof Error ? error.message : "";
  if (message.includes("1D LUTs")) return "Bảng màu 1D chưa được hỗ trợ. Hãy dùng bảng màu 3D (.cube có dòng LUT_3D_SIZE).";
  if (message.includes("LUT_3D_SIZE must be")) return "Giá trị LUT_3D_SIZE phải là số nguyên từ 2 đến 129.";
  if (message.includes("missing LUT_3D_SIZE") || message.includes("before LUT_3D_SIZE")) return "File thiếu dòng LUT_3D_SIZE hoặc dữ liệu nằm trước dòng này.";
  if (message.includes("expected") || message.includes("too many")) return "Số dòng dữ liệu không khớp với LUT_3D_SIZE.";
  if (message.includes("non-numeric") || message.includes("must contain three") || message.includes("missing value")) return "Có dòng dữ liệu không phải ba số hợp lệ.";
  return message.startsWith("Invalid .cube") ? "Nội dung file không đúng định dạng .cube." : describeEffectError(error);
}

function BasicRow({ item, control }: { item: TimelineItem; control: (typeof BASIC_CONTROLS)[number] }) {
  const param = getGpuEffect(control.type)?.params[control.param];
  const entry = findEffect(item.effects ?? [], control.type);
  if (!param) return null;
  const min = param.min ?? -Infinity;
  const max = param.max ?? Infinity;
  const step = param.step ?? 0.01;
  if (!entry) return <PropertyRow label={control.label}><NumberField label={control.label} value={Number(param.default)} onChange={() => undefined} min={min} max={max} step={step} disabled /></PropertyRow>;
  const value = Number(entry.effect.params[control.param] ?? param.default);
  return <AnimatedNumber item={item} property={buildEffectAnimatableProperty(control.type, entry.id, control.param)} label={control.label} value={value} min={min} max={max} step={step}
    onChange={next => runEffectAction("Không thể chỉnh màu", () => setEffectParam(item.id, entry.id, control.param, next))} />;
}

function LutControls({ item }: { item: TimelineItem }) {
  const inputRef = useRef<HTMLInputElement>(null);
  const [busy, setBusy] = useState(false);
  const entry = findEffect(item.effects ?? [], LUT_TYPE);
  const params = entry?.effect.params;
  const lutName = typeof params?.lutFileName === "string" && params.lutFileName ? params.lutFileName : typeof params?.lutName === "string" && params.lutName ? params.lutName : null;
  const lutSize = Number(params?.lutSize ?? 0);
  const intensity = getGpuEffect(LUT_TYPE)?.params.intensity;

  const importFile = async (event: ChangeEvent<HTMLInputElement>) => {
    const input = event.currentTarget;
    const file = input.files?.[0];
    input.value = "";
    if (!file) return;
    setBusy(true);
    try {
      const source = await file.text();
      assertSupportedLutSize(source);
      const parsed = parseCubeLut(source);
      const lut = resampleCubeLut(parsed, LUT_EMBED_SIZE);
      const embedded = { lutName: lut.title || file.name, lutFileName: file.name, lutSize: String(lut.size), lutData: encodeLutData(lut.data) };
      const store = useTimelineStore.getState();
      const existing = findEffect(currentEffects(item.id), LUT_TYPE);
      if (!useItemsStore.getState().itemById[item.id]) throw new Error("Clip không còn trên timeline.");
      if (existing) store.updateEffect(item.id, existing.id, { effect: { ...existing.effect, params: { ...existing.effect.params, ...embedded } } });
      else store.addEffect(item.id, { type: "gpu-effect", gpuEffectType: LUT_TYPE, params: { ...getGpuEffectDefaultParams(LUT_TYPE), ...embedded } });
      toast.success("Đã áp dụng bảng màu", { description: embedded.lutName });
      announce(`Đã áp dụng bảng màu ${embedded.lutName}`);
    } catch (error) {
      toast.error("Không thể nhập file bảng màu", { description: describeLutError(error) });
    } finally {
      setBusy(false);
    }
  };

  const remove = () => {
    if (entry && runEffectAction("Không thể gỡ bảng màu", () => removeEffectWithKeyframes(item.id, entry))) announce("Đã gỡ bảng màu");
  };

  return <div className="space-y-2">
    <PropertyRow label="Bảng màu" action={entry && <EditorIconButton icon={X} label="Gỡ bảng màu khỏi clip" onClick={remove} />}>
      <span className="block truncate text-xs" title={lutName ?? undefined}>{lutName ? `${lutName}${lutSize >= 2 ? ` (${lutSize}³)` : ""}` : "Chưa chọn bảng màu"}</span>
    </PropertyRow>
    <input ref={inputRef} type="file" accept=".cube" tabIndex={-1} aria-hidden className="sr-only" onChange={importFile} />
    <Button type="button" variant="outline" size="sm" disabled={busy} onClick={() => inputRef.current?.click()} className="w-full gap-2">
      <FileUp aria-hidden className="size-3.5" />{busy ? "Đang nhập bảng màu…" : lutName ? "Đổi file bảng màu (.cube)" : "Nhập file bảng màu (.cube)"}
    </Button>
    {entry && intensity
      ? <AnimatedNumber item={item} property={buildEffectAnimatableProperty(LUT_TYPE, entry.id, "intensity")} label="Cường độ bảng màu" value={Number(entry.effect.params.intensity ?? intensity.default)} min={intensity.min ?? 0} max={intensity.max ?? 1} step={intensity.step ?? 0.01}
        onChange={next => runEffectAction("Không thể chỉnh cường độ bảng màu", () => setEffectParam(item.id, entry.id, "intensity", next))} />
      : <PropertyRow label="Cường độ bảng màu"><NumberField label="Cường độ bảng màu" value={1} onChange={() => undefined} min={0} max={1} step={0.01} disabled /></PropertyRow>}
  </div>;
}

export function ColorSection({ item }: { item: TimelineItem }) {
  if (item.type === "audio") return <SectionHeader title="Màu"><p className="text-xs text-muted-foreground">Clip âm thanh không có chỉnh màu.</p></SectionHeader>;
  const effects = item.effects ?? [];
  const basics = BASIC_TYPES.flatMap(type => findEffect(effects, type) ?? []);
  const missing = BASIC_TYPES.filter(type => !findEffect(effects, type));
  const allOn = basics.length > 0 && basics.every(entry => entry.enabled);
  const setBasics = (failure: string, map: (entry: ItemEffect) => ItemEffect): boolean => runEffectAction(failure, () => {
    const latest = currentEffects(item.id);
    useTimelineStore.getState().setItemEffects([{ itemId: item.id, effects: latest.map(entry => BASIC_TYPES.includes(entry.effect.gpuEffectType) ? map(entry) : entry) }]);
  });
  const enableMissing = () => {
    const ok = runEffectAction("Không thể bật chỉnh màu", () => useTimelineStore.getState().addEffects([{ itemId: item.id, effects: missing.map(type => ({ type: "gpu-effect" as const, gpuEffectType: type, params: getGpuEffectDefaultParams(type) })) }]));
    if (ok) announce("Đã bật chỉnh màu cơ bản");
  };
  const toggleAll = () => {
    if (setBasics("Không thể bật hoặc tắt chỉnh màu", entry => ({ ...entry, enabled: !allOn }))) announce(allOn ? "Đã tắt chỉnh màu cơ bản" : "Đã bật chỉnh màu cơ bản");
  };
  const reset = () => {
    if (setBasics("Không thể đặt lại chỉnh màu", entry => ({ ...entry, effect: { ...entry.effect, params: { ...entry.effect.params, ...getGpuEffectDefaultParams(entry.effect.gpuEffectType) } } }))) announce("Đã đặt lại chỉnh màu cơ bản");
  };
  return <SectionHeader title="Màu">
    <div className="flex items-center gap-1">
      <h3 className="flex-1 text-xs font-semibold">Chỉnh màu cơ bản</h3>
      <EditorIconButton icon={allOn ? Eye : EyeOff} label="Bật chỉnh màu cơ bản" aria-pressed={allOn} disabled={basics.length === 0} onClick={toggleAll} />
      <EditorIconButton icon={RotateCcw} label="Đặt lại chỉnh màu cơ bản" disabled={basics.length === 0} onClick={reset} />
    </div>
    {BASIC_CONTROLS.map(control => <BasicRow key={`${control.type}:${control.param}`} item={item} control={control} />)}
    {missing.length > 0 && <Button type="button" variant="outline" size="sm" onClick={enableMissing} className="w-full">{basics.length ? "Bổ sung điều chỉnh còn thiếu" : "Bật chỉnh màu cơ bản"}</Button>}
    <h3 className="pt-2 text-xs font-semibold">Bảng màu (.cube)</h3>
    <LutControls item={item} />
  </SectionHeader>;
}
