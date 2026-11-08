"use client";

import type { ShapeItem } from "@editor/types/timeline";
import type { ShapeAnimatableProperty } from "@editor/types/keyframe";
import { useTimelineStore } from "@editor/features/timeline/stores/timeline-store";
import { getShapeAnimatableBaseValue } from "@editor/features/keyframes/utils/shape-animatable-properties";
import { DEFAULT_SHAPE_GRADIENT_END_COLOR, resolveShapeLinearGradient } from "@editor/shared/graphics/shapes/linear-gradient";
import { Switch } from "@/components/ui/switch";
import { ColorField, EditorSelect, NumberField, PropertyRow, SectionHeader } from "@/editor/ui/controls";
import { announce } from "@/editor/ui/shell/announcer";
import { AnimatedNumber } from "./fields";

const DIRECTIONS = [
  { value: "up", label: "Lên" }, { value: "down", label: "Xuống" },
  { value: "left", label: "Trái" }, { value: "right", label: "Phải" },
] as const;
const CAPS = [
  { value: "butt", label: "Phẳng" }, { value: "round", label: "Tròn" }, { value: "square", label: "Vuông" },
] as const;
const JOINS = [
  { value: "miter", label: "Nhọn" }, { value: "round", label: "Tròn" }, { value: "bevel", label: "Vát" },
] as const;
const STROKE_ANIMATION_FIELDS: readonly { property: ShapeAnimatableProperty; label: string; max?: number; unit: string }[] = [
  { property: "trimPathStart", label: "Bắt đầu nét", max: 100, unit: "%" },
  { property: "trimPathEnd", label: "Kết thúc nét", max: 100, unit: "%" },
  { property: "trimPathOffset", label: "Dịch vòng nét", unit: "°" },
  { property: "taperStartWidth", label: "Độ rộng đầu nét", max: 200, unit: "%" },
  { property: "taperEndWidth", label: "Độ rộng cuối nét", max: 200, unit: "%" },
  { property: "taperStartLength", label: "Độ dài vuốt đầu", max: 100, unit: "%" },
  { property: "taperEndLength", label: "Độ dài vuốt cuối", max: 100, unit: "%" },
];

/** Shape geometry uses item.cornerRadius, never the transform's bounding-box clipping radius. */
export function ShapeSection({ item }: { item: ShapeItem }) {
  // Generated visualizers have their own geometry and appearance controls.
  if (item.waveformVisualizer) return null;

  const update = (patch: Partial<ShapeItem>) => useTimelineStore.getState().updateItem(item.id, patch);
  const toggle = (label: string, patch: Partial<ShapeItem>, enabled: boolean) => {
    update(patch);
    announce(`${label}: ${enabled ? "bật" : "tắt"}`);
  };
  const openPath = item.shapeType === "path" && item.pathClosed === false;
  const fillEnabled = item.fillEnabled ?? true;
  const strokeEnabled = item.strokeEnabled ?? ((item.strokeWidth ?? 0) > 0 && item.strokeColor !== undefined);
  const gradient = resolveShapeLinearGradient(item);
  const rounded = item.shapeType === "rectangle" || item.shapeType === "triangle" || item.shapeType === "star" || item.shapeType === "polygon";
  const hasGeometry = rounded || item.shapeType === "path";

  return <>
    {hasGeometry && <SectionHeader title="Hình dạng">
      {rounded && <PropertyRow label="Bo góc hình">
        <NumberField label="Bo góc hình" value={item.cornerRadius ?? 0} min={0} unit="px" onChange={cornerRadius => update({ cornerRadius })} />
      </PropertyRow>}
      {item.shapeType === "triangle" && <PropertyRow label="Hướng đỉnh">
        <EditorSelect label="Hướng đỉnh tam giác" value={item.direction ?? "up"} options={DIRECTIONS} onValueChange={value => {
          const direction = DIRECTIONS.find(option => option.value === value)?.value;
          if (direction) update({ direction });
        }} />
      </PropertyRow>}
      {(item.shapeType === "star" || item.shapeType === "polygon") && <PropertyRow label={item.shapeType === "star" ? "Số cánh" : "Số cạnh"}>
        <NumberField label={item.shapeType === "star" ? "Số cánh sao" : "Số cạnh đa giác"} value={item.points ?? (item.shapeType === "star" ? 5 : 6)} min={3} step={1} precision={0} onChange={points => update({ points: Math.round(points) })} />
      </PropertyRow>}
      {item.shapeType === "star" && <PropertyRow label="Bán kính trong">
        <NumberField label="Bán kính trong ngôi sao" value={(item.innerRadius ?? 0.5) * 100} min={5} max={95} unit="%" onChange={value => update({ innerRadius: value / 100 })} />
      </PropertyRow>}
      {item.shapeType === "path" && <PropertyRow label="Khép kín đường">
        <Switch aria-label="Khép kín đường vẽ" checked={item.pathClosed !== false} onCheckedChange={pathClosed => toggle("Khép kín đường vẽ", { pathClosed }, pathClosed)} />
      </PropertyRow>}
    </SectionHeader>}

    {!openPath && <SectionHeader title="Màu tô">
      <PropertyRow label="Bật màu tô">
        <Switch aria-label="Bật màu tô" checked={fillEnabled} onCheckedChange={enabled => toggle("Màu tô", { fillEnabled: enabled }, enabled)} />
      </PropertyRow>
      {fillEnabled && <>
        <PropertyRow label="Kiểu tô">
          <EditorSelect label="Kiểu màu tô" value={item.fillType ?? "solid"} options={[{ value: "solid", label: "Một màu" }, { value: "linear", label: "Chuyển màu thẳng" }]} onValueChange={fillType => {
            if (fillType === "solid" || fillType === "linear") update({ fillType });
          }} />
        </PropertyRow>
        {item.fillType === "linear" ? <>
          <PropertyRow label="Màu đầu">
            <ColorField label="Màu đầu chuyển sắc" value={gradient?.startColor ?? item.fillColor} onChange={gradientStartColor => update({ gradientStartColor })} />
          </PropertyRow>
          <PropertyRow label="Màu cuối">
            <ColorField label="Màu cuối chuyển sắc" value={gradient?.endColor ?? DEFAULT_SHAPE_GRADIENT_END_COLOR} onChange={gradientEndColor => update({ gradientEndColor })} />
          </PropertyRow>
          <PropertyRow label="Góc chuyển màu">
            <NumberField label="Góc chuyển màu" value={gradient?.angle ?? 0} unit="°" onChange={gradientAngle => update({ gradientAngle })} />
          </PropertyRow>
        </> : <PropertyRow label="Màu tô">
          <ColorField label="Màu tô hình" value={item.fillColor} onChange={fillColor => update({ fillColor })} />
        </PropertyRow>}
      </>}
    </SectionHeader>}

    <SectionHeader title="Đường viền">
      <PropertyRow label="Bật đường viền">
        <Switch aria-label="Bật đường viền" checked={strokeEnabled} onCheckedChange={enabled => toggle("Đường viền", {
          strokeEnabled: enabled,
          // A legacy fill-only shape has neither width nor colour. Enabling its
          // outline must create visible paint; disabling retains that styling.
          ...(enabled ? { strokeColor: item.strokeColor ?? "#000000", strokeWidth: (item.strokeWidth ?? 0) > 0 ? item.strokeWidth : 1 } : {}),
        }, enabled)} />
      </PropertyRow>
      {strokeEnabled && <>
        <PropertyRow label="Màu viền">
          <ColorField label="Màu đường viền" value={item.strokeColor ?? "#000000"} onChange={strokeColor => update({ strokeColor })} />
        </PropertyRow>
        <PropertyRow label="Độ dày viền">
          <NumberField label="Độ dày đường viền" value={item.strokeWidth ?? 0} min={0} unit="px" onChange={strokeWidth => update({ strokeWidth })} />
        </PropertyRow>
        <PropertyRow label="Đầu nét">
          <EditorSelect label="Kiểu đầu nét" value={item.strokeLineCap ?? "butt"} options={CAPS} onValueChange={value => {
            const strokeLineCap = CAPS.find(option => option.value === value)?.value;
            if (strokeLineCap) update({ strokeLineCap });
          }} />
        </PropertyRow>
        <PropertyRow label="Góc nối">
          <EditorSelect label="Kiểu góc nối" value={item.strokeLineJoin ?? "miter"} options={JOINS} onValueChange={value => {
            const strokeLineJoin = JOINS.find(option => option.value === value)?.value;
            if (strokeLineJoin) update({ strokeLineJoin });
          }} />
        </PropertyRow>
        {(item.strokeLineJoin ?? "miter") === "miter" && <PropertyRow label="Giới hạn góc nhọn">
          <NumberField label="Giới hạn góc nhọn" value={item.strokeMiterLimit ?? 4} min={1} onChange={strokeMiterLimit => update({ strokeMiterLimit })} />
        </PropertyRow>}
        {STROKE_ANIMATION_FIELDS.map(({ property, label, max, unit }) => <AnimatedNumber key={property}
          item={item} property={property} label={label} value={getShapeAnimatableBaseValue(item, property)}
          min={property === "trimPathOffset" ? undefined : 0} max={max} unit={unit}
          onChange={value => update({ [property]: value })}
        />)}
      </>}
    </SectionHeader>

    <SectionHeader title="Mặt nạ">
      <PropertyRow label="Dùng làm mặt nạ">
        <Switch aria-label="Dùng hình làm mặt nạ" checked={item.isMask ?? false} onCheckedChange={isMask => toggle("Dùng hình làm mặt nạ", { isMask }, isMask)} />
      </PropertyRow>
      {item.isMask && <>
        <PropertyRow label="Kiểu mặt nạ">
          <EditorSelect label="Kiểu mặt nạ" value={item.maskType ?? "clip"} options={[{ value: "clip", label: "Cạnh sắc" }, { value: "alpha", label: "Cạnh mềm" }]} onValueChange={maskType => {
            if (maskType === "clip" || maskType === "alpha") update({ maskType });
          }} />
        </PropertyRow>
        {item.maskType === "alpha" && <PropertyRow label="Độ mềm cạnh">
          <NumberField label="Độ mềm cạnh mặt nạ" value={item.maskFeather ?? 0} min={0} max={100} unit="px" onChange={maskFeather => update({ maskFeather })} />
        </PropertyRow>}
        <PropertyRow label="Độ phủ mặt nạ">
          <NumberField label="Độ phủ mặt nạ" value={item.maskOpacity ?? 100} min={0} max={100} unit="%" onChange={maskOpacity => update({ maskOpacity })} />
        </PropertyRow>
        <PropertyRow label="Đảo mặt nạ">
          <Switch aria-label="Đảo vùng mặt nạ" checked={item.maskInvert ?? false} onCheckedChange={maskInvert => toggle("Đảo vùng mặt nạ", { maskInvert }, maskInvert)} />
        </PropertyRow>
      </>}
    </SectionHeader>
  </>;
}
