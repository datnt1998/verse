"use client";

import { Trash2 } from "lucide-react";
import type { Transition, WipeDirection } from "@editor/types/transition";
import { GPU_TRANSITION_REGISTRY } from "@editor/infrastructure/gpu-transitions/registry";
import { useTimelineStore } from "@editor/features/timeline/stores/timeline-store";
import { EditorIconButton, EditorSelect, NumberField, PropertyRow, SectionHeader } from "@/editor/ui/controls";
import { announce } from "@/editor/ui/shell/announcer";


export const transitionNames: Record<string, string> = { dissolve: "Hoà tan", additiveDissolve: "Hoà tan sáng", blurDissolve: "Hoà tan mờ", dipToColorDissolve: "Qua màu", nonAdditiveDissolve: "Hoà tan tối", smoothCut: "Cắt mượt", sparkles: "Lấp lánh", glitch: "Nhiễu", pixelate: "Điểm ảnh", chromatic: "Tách màu", radialBlur: "Mờ tâm", fade: "Mờ dần", wipe: "Gạt", slide: "Trượt", flip: "Lật", clockWipe: "Gạt đồng hồ", iris: "Khẩu độ", liquidDistort: "Biến dạng nước", lensWarpZoom: "Zoom ống kính", lightLeakBurn: "Loé sáng", filmGateSlip: "Trượt phim" };

export function TransitionSection({ transition }: { transition: Transition }) {
  const definition = GPU_TRANSITION_REGISTRY.get(transition.presentation);
  return (
    <SectionHeader title="Chuyển cảnh">
      <PropertyRow label="Kiểu" action={<EditorIconButton icon={Trash2} label="Xoá chuyển cảnh" onClick={() => { useTimelineStore.getState().removeTransition(transition.id); announce("Đã xoá chuyển cảnh"); }} />}>
        <EditorSelect label="Kiểu chuyển cảnh" value={transition.presentation} onValueChange={presentation => {
          const next = GPU_TRANSITION_REGISTRY.get(presentation);
          useTimelineStore.getState().updateTransition(transition.id, { presentation, direction: next?.hasDirection ? transition.direction ?? "from-left" : undefined });
          announce(`Đã đổi chuyển cảnh sang ${transitionNames[presentation] ?? presentation}`);
        }} options={Array.from(GPU_TRANSITION_REGISTRY.values()).map(d => ({ value: d.id, label: transitionNames[d.id] ?? d.name }))} />
      </PropertyRow>
      <PropertyRow label="Thời lượng"><NumberField label="Thời lượng chuyển cảnh" value={transition.durationInFrames} min={1} step={1} unit="khung" onChange={durationInFrames => useTimelineStore.getState().updateTransition(transition.id, { durationInFrames: Math.round(durationInFrames) })} /></PropertyRow>
      {definition?.hasDirection && <PropertyRow label="Hướng"><EditorSelect label="Hướng chuyển cảnh" value={transition.direction ?? "from-left"} onValueChange={direction => useTimelineStore.getState().updateTransition(transition.id, { direction: direction as WipeDirection })} options={[{ value: "from-left", label: "Từ trái" }, { value: "from-right", label: "Từ phải" }, { value: "from-top", label: "Từ trên" }, { value: "from-bottom", label: "Từ dưới" }]} /></PropertyRow>}
      <PropertyRow label="Căn"><EditorSelect label="Vị trí chuyển cảnh" value={String(transition.alignment ?? 0.5)} onValueChange={v => useTimelineStore.getState().updateTransition(transition.id, { alignment: Number(v) })} options={[{ value: "0.5", label: "Giữa điểm cắt" }, { value: "0", label: "Đầu clip sau" }, { value: "1", label: "Cuối clip trước" }]} /></PropertyRow>
    </SectionHeader>
  );
}
