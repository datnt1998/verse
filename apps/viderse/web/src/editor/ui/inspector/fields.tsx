"use client";

import { useId, useRef } from "react";
import type { AnimatableProperty, VectorAnimatableProperty } from "@editor/types/keyframe";
import type { TimelineItem } from "@editor/types/timeline";
import { useTimelineStore } from "@editor/features/timeline/stores/timeline-store";
import { useKeyframesStore } from "@editor/features/timeline/stores/keyframes-store";
import { usePlaybackStore } from "@editor/shared/state/playback";
import { getPropertyKeyframes, interpolatePropertyValue } from "@editor/features/keyframes/utils/interpolation";
import { interpolateVectorPropertyValue } from "@editor/features/keyframes/utils/vector-interpolation";
import { getAnimatablePropertiesForItem } from "@editor/features/keyframes/utils/animatable-properties";
import { KeyframeToggle, NumberField, PropertyRow, SliderField, type NumberFieldProps } from "@/editor/ui/controls";
import { useShortcutAction } from "@/editor/ui/shortcuts";
import { announce } from "@/editor/ui/shell/announcer";

export function AnimatedNumber({ item, property, label, value, onChange, displayFactor = 1, slider = false, ...props }: Omit<NumberFieldProps, "onCommit"> & { item: TimelineItem; property: AnimatableProperty; displayFactor?: number; slider?: boolean }) {
  const fieldId = useId();
  const frame = usePlaybackStore(s => s.currentFrame);
  const animation = useKeyframesStore(s => s.keyframesByItemId[item.id]);
  const focused = useRef(false);
  const localFrame = Math.max(0, Math.min(item.durationInFrames - 1, frame - item.from));
  const keys = getPropertyKeyframes(animation, property);
  const vectorProperty: VectorAnimatableProperty | undefined = property === "x" || property === "y" ? "position" : property === "width" || property === "height" ? "scale" : property === "anchorX" || property === "anchorY" ? "anchor" : undefined;
  const vectorLane = animation?.vectorProperties?.find(p => p.property === vectorProperty);
  const axis = property === "x" || property === "width" || property === "anchorX" ? "x" : "y";
  const vectorKey = vectorLane?.keyframes.find(k => k.frame === localFrame);
  const vectorValue = vectorLane ? interpolateVectorPropertyValue(vectorLane.property, vectorLane.keyframes, localFrame, { x: value, y: value }, useTimelineStore.getState().fps) : undefined;
  const key = keys.find(k => k.frame === localFrame);
  const currentValue = vectorValue?.[axis] ?? interpolatePropertyValue(keys, localFrame, value);
  const supported = getAnimatablePropertiesForItem(item).includes(property);
  const Field = slider ? SliderField : NumberField;
  const toggle = () => {
    const store = useTimelineStore.getState();
    if (vectorLane) {
      if (vectorKey) store.removeVectorKeyframe(item.id, vectorLane.property, vectorKey.id);
      else store.upsertVectorKeyframe(item.id, vectorLane.property, { frame: localFrame, value: vectorValue ?? { x: value, y: value } });
    } else if (key) store.removeKeyframe(item.id, property, key.id);
    else store.addKeyframe(item.id, property, localFrame, currentValue);
    announce(`${key || vectorKey ? "Đã xoá" : "Đã thêm"} điểm mốc ${label} tại khung ${localFrame}`);
  };
  useShortcutAction({ id: `keyframe-${item.id}-${property}-${fieldId}`, label: `Điểm mốc ${label}`, scope: "inspector", bindings: ["KeyK"], enabled: () => focused.current && supported, run: toggle });
  return (
    <div onFocusCapture={() => { focused.current = true; }} onBlurCapture={e => { if (!e.currentTarget.contains(e.relatedTarget)) focused.current = false; }}>
      <PropertyRow label={label} action={supported && <KeyframeToggle label={label} active={Boolean(key || vectorKey)} onToggle={toggle} />}>
        <Field {...props} label={label} value={Number((currentValue * displayFactor).toFixed(5))} onChange={displayValue => {
          const next = displayValue / displayFactor;
          const store = useTimelineStore.getState();
          if (vectorLane && vectorValue) {
            const value = { ...vectorValue, [axis]: next };
            if (vectorKey) store.updateVectorKeyframe(item.id, vectorLane.property, vectorKey.id, { value });
            else store.upsertVectorKeyframe(item.id, vectorLane.property, { frame: localFrame, value });
          } else if (key) store.updateKeyframe(item.id, property, key.id, { value: next });
          else if (keys.length > 0) store.addKeyframe(item.id, property, localFrame, next);
          else onChange(next);
        }} onCommit={n => announce(`Đã đổi ${label}: ${Number(n.toFixed(3))}${props.unit ? ` ${props.unit}` : ""}`)} />
      </PropertyRow>
    </div>
  );
}
