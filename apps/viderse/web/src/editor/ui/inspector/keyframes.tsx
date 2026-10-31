"use client";

import { Crosshair, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import type { TimelineItem } from "@editor/types/timeline";
import type { Keyframe, AnimatableProperty, BezierControlPoints, EasingType, VectorKeyframe, VectorAnimatableProperty } from "@editor/types/keyframe";
import { DEFAULT_BEZIER_POINTS, parseEffectAnimatableProperty } from "@editor/types/keyframe";
import { useKeyframesStore } from "@editor/features/timeline/stores/keyframes-store";
import { useTimelineStore } from "@editor/features/timeline/stores/timeline-store";
import { usePlaybackStore } from "@editor/shared/state/playback";
import { BEZIER_PRESETS, buildEasingConfig } from "@editor/features/keyframes/utils/easing-presets";
import { EditorIconButton, EditorSelect, NumberField, PropertyRow, SectionHeader } from "@/editor/ui/controls";
import { useShortcutAction } from "@/editor/ui/shortcuts";
import { announce } from "@/editor/ui/shell/announcer";

import { effectName, PARAM_LABELS } from "./effects";

const labels: Record<string, string> = { x: "Vị trí X", y: "Vị trí Y", width: "Rộng", height: "Cao", anchorX: "Điểm neo X", anchorY: "Điểm neo Y", rotation: "Xoay", opacity: "Độ mờ", volume: "Âm lượng", cornerRadius: "Bo góc", cropLeft: "Crop trái", cropRight: "Crop phải", cropTop: "Crop trên", cropBottom: "Crop dưới", cropSoftness: "Mềm crop", textStyleScale: "Tỉ lệ chữ", fontSize: "Cỡ chữ", lineHeight: "Giãn dòng", textPadding: "Đệm chữ", backgroundRadius: "Bo nền chữ", textShadowOffsetX: "Bóng X", textShadowOffsetY: "Bóng Y", textShadowBlur: "Mờ bóng", strokeWidth: "Độ dày viền", position: "Vị trí", scale: "Tỉ lệ", anchor: "Điểm neo" };
const eases: { id: EasingType; label: string }[] = [{ id: "linear", label: "Tuyến tính" }, { id: "ease-in", label: "Chậm đầu" }, { id: "ease-out", label: "Chậm cuối" }, { id: "ease-in-out", label: "Chậm hai đầu" }, { id: "hold", label: "Giữ giá trị" }, { id: "cubic-bezier", label: "Cubic tuỳ chỉnh" }];
const presetNames: Record<string, string> = { soft: "Mượt", "ease-out": "Chậm cuối", "ease-in": "Chậm đầu", "ease-in-out": "Chậm hai đầu", overshoot: "Vượt đích", snap: "Nhanh", "out-cubic": "Cubic cuối", "out-quart": "Quart cuối", "out-quint": "Quint cuối", "out-expo": "Expo cuối", "out-circ": "Circ cuối", "in-out-cubic": "Cubic hai đầu", "in-out-quart": "Quart hai đầu", "in-out-expo": "Expo hai đầu", "in-cubic": "Cubic đầu", "in-quart": "Quart đầu", "in-expo": "Expo đầu" };

function EasingFields({ keyframe, update }: { keyframe: Pick<Keyframe, "easing" | "easingConfig">; update: (patch: Partial<Omit<Keyframe, "id" | "value">>) => void }) {
  const bezier = keyframe.easingConfig?.bezier ?? DEFAULT_BEZIER_POINTS;
  return <><PropertyRow label="Nhịp"><EditorSelect label="Nhịp giữa hai điểm mốc" value={keyframe.easing} onValueChange={v => { const easing = v as EasingType; update({ easing, easingConfig: buildEasingConfig(easing, keyframe.easingConfig) }); }} options={eases.map(e => ({ value: e.id, label: e.label }))} /></PropertyRow>{keyframe.easing === "cubic-bezier" && <><PropertyRow label="Đường cong"><EditorSelect label="Mẫu đường cong" value="" placeholder="Chọn mẫu" onValueChange={v => { const preset = BEZIER_PRESETS.find(p => p.value === v); if (preset) update({ easingConfig: { type: "cubic-bezier", bezier: { ...preset.points } } }); }} options={BEZIER_PRESETS.map(p => ({ value: p.value, label: presetNames[p.value] ?? p.value }))} /></PropertyRow>{(["x1", "y1", "x2", "y2"] as const).map(component => <PropertyRow key={component} label={component.toUpperCase()}><NumberField label={`Cubic ${component}`} value={bezier[component]} min={component.startsWith("x") ? 0 : -2} max={component.startsWith("x") ? 1 : 3} step={0.01} onChange={n => update({ easingConfig: { type: "cubic-bezier", bezier: { ...bezier, [component]: n } as BezierControlPoints } })} /></PropertyRow>)}</>}</>;
}

function ScalarKey({ item, property, keyframe }: { item: TimelineItem; property: AnimatableProperty; keyframe: Keyframe }) {
  const update = (updates: Partial<Omit<Keyframe, "id">>) => useTimelineStore.getState().updateKeyframe(item.id, property, keyframe.id, updates);
  const effect = parseEffectAnimatableProperty(property);
  const label = effect ? `${effectName(effect.gpuEffectType)} · ${PARAM_LABELS[effect.paramKey] ?? "Tham số"}` : labels[property] ?? "Thuộc tính";
  return (
    <div className="space-y-2 rounded-md border border-border bg-background/50 p-2">
      <div className="flex items-center gap-2">
        <span className="min-w-0 flex-1 break-words text-xs font-medium tabular-nums">{label}</span>
        <EditorIconButton icon={Crosshair} label={`Nhảy tới điểm mốc ${label}`} onClick={() => { usePlaybackStore.getState().setCurrentFrame(item.from + keyframe.frame); announce(`Điểm mốc ${label}, khung ${keyframe.frame}`); }} />
        <EditorIconButton icon={Trash2} label="Xoá điểm mốc" onClick={() => useTimelineStore.getState().removeKeyframe(item.id, property, keyframe.id)} />
      </div>
      <PropertyRow label="Khung"><NumberField label="Khung của điểm mốc" value={keyframe.frame} min={0} max={item.durationInFrames - 1} onChange={frame => update({ frame: Math.round(frame) })} /></PropertyRow>
      <PropertyRow label="Giá trị"><NumberField label={`Giá trị điểm mốc ${label}`} value={keyframe.value} step={0.01} onChange={value => update({ value })} /></PropertyRow>
      <EasingFields keyframe={keyframe} update={update} />
    </div>
  );
}

function VectorKey({ item, property, keyframe }: { item: TimelineItem; property: VectorAnimatableProperty; keyframe: VectorKeyframe }) {
  const update = (updates: Partial<Omit<VectorKeyframe, "id">>) => useTimelineStore.getState().updateVectorKeyframe(item.id, property, keyframe.id, updates);
  return <div className="space-y-2 rounded-md border border-border bg-background/50 p-2"><div className="flex items-center gap-2"><span className="flex-1 text-xs font-medium">{labels[property]}</span><EditorIconButton icon={Crosshair} label={`Nhảy tới điểm mốc ${labels[property]}`} onClick={() => usePlaybackStore.getState().setCurrentFrame(item.from + keyframe.frame)} /><EditorIconButton icon={Trash2} label="Xoá điểm mốc" onClick={() => useTimelineStore.getState().removeVectorKeyframe(item.id, property, keyframe.id)} /></div><PropertyRow label="Khung"><NumberField label="Khung của điểm mốc" value={keyframe.frame} min={0} max={item.durationInFrames - 1} onChange={frame => update({ frame: Math.round(frame) })} /></PropertyRow>{(["x", "y"] as const).map(axis => <PropertyRow key={axis} label={axis.toUpperCase()}><NumberField label={`Giá trị điểm mốc ${axis}`} value={keyframe.value[axis]} step={0.01} onChange={n => update({ value: { ...keyframe.value, [axis]: n } })} /></PropertyRow>)}<EasingFields keyframe={keyframe} update={update} /></div>;
}

export function KeyframesSection({ item }: { item: TimelineItem }) {
  const data = useKeyframesStore(s => s.keyframesByItemId[item.id]);
  const currentFrame = usePlaybackStore(s => s.currentFrame);
  const scalar = data?.properties.flatMap(p => p.keyframes.map(keyframe => ({ property: p.property, keyframe }))) ?? [];
  const vector = data?.vectorProperties?.flatMap(p => p.keyframes.map(keyframe => ({ property: p.property, keyframe }))) ?? [];
  const frames = [...new Set([...scalar, ...vector].map(k => k.keyframe.frame))].sort((a, b) => a - b);
  const jump = (direction: -1 | 1) => { const local = currentFrame - item.from; const frame = direction < 0 ? frames.findLast(f => f < local) : frames.find(f => f > local); if (frame !== undefined) { usePlaybackStore.getState().setCurrentFrame(item.from + frame); announce(`Điểm mốc tại khung ${frame}`); } };
  useShortcutAction({ id: "keyframe-previous", label: "Điểm mốc trước", scope: "inspector", enabled: frames.length > 0, run: () => jump(-1) });
  useShortcutAction({ id: "keyframe-next", label: "Điểm mốc sau", scope: "inspector", enabled: frames.length > 0, run: () => jump(1) });
  return <SectionHeader title={`Điểm mốc · ${scalar.length + vector.length}`} defaultOpen={false}>{scalar.length + vector.length === 0 ? <p className="text-xs text-muted-foreground">Nhấn hình thoi cạnh thuộc tính để thêm điểm mốc.</p> : <><div className="flex gap-2"><Button type="button" variant="outline" size="sm" onClick={() => jump(-1)}>Trước</Button><Button type="button" variant="outline" size="sm" onClick={() => jump(1)}>Sau</Button></div>{scalar.map(k => <ScalarKey key={k.keyframe.id} item={item} {...k} />)}{vector.map(k => <VectorKey key={k.keyframe.id} item={item} {...k} />)}</>}</SectionHeader>;
}
