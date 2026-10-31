"use client";

import type { TimelineItem } from "@editor/types/timeline";
import type { CropSettings, TransformProperties } from "@editor/types/transform";
import { useTimelineStore } from "@editor/features/timeline/stores/timeline-store";
import { useProjectStore } from "@editor/features/projects/stores/project-store";
import { computeInitialTransform } from "@editor/features/timeline/utils/transform-init";
import { Switch } from "@/components/ui/switch";
import { EditorSelect, PropertyRow, SectionHeader } from "@/editor/ui/controls";
import { AnimatedNumber } from "./fields";
import { announce } from "@/editor/ui/shell/announcer";

export function TransformSection({ item }: { item: TimelineItem }) {
  const metadata = useProjectStore(s => s.currentProject?.metadata);
  const canvasWidth = metadata?.width ?? 1920;
  const canvasHeight = metadata?.height ?? 1080;
  const sourceWidth = "sourceWidth" in item ? item.sourceWidth ?? canvasWidth : canvasWidth;
  const sourceHeight = "sourceHeight" in item ? item.sourceHeight ?? canvasHeight : canvasHeight;
  const fitted = computeInitialTransform(sourceWidth, sourceHeight, canvasWidth, canvasHeight);
  const transform = { ...fitted, ...item.transform };
  const writeTransform = (patch: Partial<TransformProperties>) => useTimelineStore.getState().updateItemTransform(item.id, patch);
  const writeCrop = (edge: keyof CropSettings, value: number) => { useTimelineStore.getState().updateItem(item.id, { crop: { ...item.crop, [edge]: value } }); };
  const cropEdges = [["left", "cropLeft", "Crop trái"], ["right", "cropRight", "Crop phải"], ["top", "cropTop", "Crop trên"], ["bottom", "cropBottom", "Crop dưới"]] as const;
  return (
    <SectionHeader title="Vị trí và kích thước">
      <AnimatedNumber item={item} property="x" label="Vị trí X" value={transform.x ?? 0} onChange={x => writeTransform({ x })} unit="px" />
      <AnimatedNumber item={item} property="y" label="Vị trí Y" value={transform.y ?? 0} onChange={y => writeTransform({ y })} unit="px" />
      <AnimatedNumber item={item} property="width" label="Tỉ lệ X" value={transform.width ?? canvasWidth} displayFactor={100 / (fitted.width ?? canvasWidth)} onChange={width => writeTransform({ width, ...(item.transform?.aspectRatioLocked !== false ? { height: width * (transform.height ?? canvasHeight) / (transform.width ?? canvasWidth) } : {}) })} min={0.1} unit="%" />
      <AnimatedNumber item={item} property="height" label="Tỉ lệ Y" value={transform.height ?? canvasHeight} displayFactor={100 / (fitted.height ?? canvasHeight)} onChange={height => writeTransform({ height, ...(item.transform?.aspectRatioLocked !== false ? { width: height * (transform.width ?? canvasWidth) / (transform.height ?? canvasHeight) } : {}) })} min={0.1} unit="%" />
      <PropertyRow label="Giữ tỉ lệ">
        <Switch aria-label="Giữ tỉ lệ kích thước" checked={item.transform?.aspectRatioLocked !== false} onCheckedChange={aspectRatioLocked => { writeTransform({ aspectRatioLocked }); announce(`Giữ tỉ lệ: ${aspectRatioLocked ? "bật" : "tắt"}`); }} />
      </PropertyRow>
      <AnimatedNumber item={item} property="rotation" label="Xoay" value={transform.rotation ?? 0} onChange={rotation => writeTransform({ rotation })} step={1} unit="°" />
      <AnimatedNumber item={item} property="opacity" label="Độ mờ" value={transform.opacity ?? 1} displayFactor={100} onChange={opacity => writeTransform({ opacity })} min={0} max={100} step={1} unit="%" />
      {(item.type === "video" || item.type === "image") && <>
        <PropertyRow label="Vừa khung">
          <EditorSelect label="Chế độ vừa khung" value="" placeholder="Chọn chế độ" onValueChange={mode => {
            const scale = mode === "cover" ? Math.max(canvasWidth / sourceWidth, canvasHeight / sourceHeight) : mode === "original" ? 1 : Math.min(canvasWidth / sourceWidth, canvasHeight / sourceHeight);
            writeTransform({ width: Math.round(sourceWidth * scale), height: Math.round(sourceHeight * scale), x: 0, y: 0 });
            announce("Đã đổi chế độ vừa khung");
          }} options={[{ value: "contain", label: "Vừa toàn ảnh" }, { value: "cover", label: "Phủ khung" }, { value: "original", label: "Kích thước gốc" }]} />
        </PropertyRow>
        {cropEdges.map(([edge, property, label]) => <AnimatedNumber key={edge} item={item} property={property} label={label} value={item.crop?.[edge] ?? 0} displayFactor={100} onChange={v => writeCrop(edge, v)} min={0} max={(1 - (item.crop?.[edge === "left" ? "right" : edge === "right" ? "left" : edge === "top" ? "bottom" : "top"] ?? 0) - 0.001) * 100} step={1} unit="%" />)}
      </>}
    </SectionHeader>
  );
}
