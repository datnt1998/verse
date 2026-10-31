"use client";

import { memo, useState } from "react";
import { SlidersHorizontal } from "lucide-react";
import type { TimelineItem } from "@editor/types/timeline";
import { useSelectionStore } from "@editor/shared/state/selection";
import { useItemsStore } from "@editor/features/timeline/stores/items-store";
import { useTransitionsStore } from "@editor/features/timeline/stores/transitions-store";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { TransformSection } from "./transform";
import { KeyframesSection } from "./keyframes";
import { TransitionSection } from "./transitions";
import { CanvasSection } from "./canvas";
import { TextSection } from "./text";
import { CaptionsSection } from "./captions";
import { EffectsSection } from "./effects";
import { ColorSection } from "./color";
import { AudioSection, MixerSection } from "./audio";
import { WaveformSection } from "./waveform";
import { TemplateAnchorSection } from "./template-anchor";

/** Field groups a section can provably ignore. An edit gives the item a new object but keeps untouched
 * fields by reference, so a section skips its re-render when every changed field belongs to a group it
 * never draws. Unknown fields always re-render: a missing entry costs speed, never a stale panel. */
const TEXT_STYLE_FIELDS = new Set(["text", "textSpans", "color", "fontFamily", "fontSize", "fontWeight", "fontStyle", "textAlign", "verticalAlign", "lineHeight", "letterSpacing", "textPadding", "backgroundColor", "backgroundRadius", "stroke", "textShadow", "textMotion", "underline"]);
const GEOMETRY_FIELDS = new Set(["transform", "crop"]);
type FieldGroup = "label" | "text" | "geometry" | "audio";
const IN_GROUP: Record<FieldGroup, (field: string) => boolean> = {
  label: (field) => field === "label",
  text: (field) => TEXT_STYLE_FIELDS.has(field),
  geometry: (field) => GEOMETRY_FIELDS.has(field),
  audio: (field) => field.startsWith("audio") || field === "volume" || field === "embeddedAudioMuted",
};

function changesOnlyIn(a: TimelineItem, b: TimelineItem, groups: readonly FieldGroup[]): boolean {
  if (a === b) return true;
  const ignored = (field: string) => groups.some((group) => IN_GROUP[group](field));
  for (const [field, value] of Object.entries(a)) if (value !== Reflect.get(b, field) && !ignored(field)) return false;
  for (const field of Object.keys(b)) if (!(field in a) && !ignored(field)) return false;
  return true;
}

function ignoring(...groups: FieldGroup[]) {
  return (a: { item: TimelineItem }, b: { item: TimelineItem }) => changesOnlyIn(a.item, b.item, groups);
}

const Transform = memo(TransformSection, ignoring("label", "text", "audio"));
const Keyframes = memo(KeyframesSection, ignoring("label", "text", "geometry", "audio"));
const Text = memo(TextSection, ignoring("label", "geometry", "audio"));
const Captions = memo(CaptionsSection, ignoring("label", "text", "geometry", "audio"));
const Waveform = memo(WaveformSection, ignoring("label", "text", "geometry", "audio"));
const Color = memo(ColorSection, ignoring("label", "text", "geometry", "audio"));
const Audio = memo(AudioSection, ignoring("label", "text", "geometry"));
const TemplateAnchor = memo(TemplateAnchorSection, ignoring("label", "text", "geometry", "audio"));
const Effects = memo(EffectsSection, ignoring("label", "text", "geometry", "audio"));
/** Clip types that carry sound (music, video, nested groups); other clips hide the audio section. */
const CLIPS_WITH_AUDIO: Record<string, true> = { audio: true, video: true, composition: true };
/** Clips that accept colour grading/LUTs: images and effect layers, not text clips (they have the Text section). */
const CLIPS_WITH_GRADING: Record<string, true> = { video: true, image: true, lottie: true, shape: true, adjustment: true, composition: true };

export function InspectorPanel() {
  const selectedId = useSelectionStore(s => s.selectedItemIds[0]);
  const transitionId = useSelectionStore(s => s.selectedTransitionId);
  const item = useItemsStore(s => selectedId ? s.itemById[selectedId] : undefined);
  const transition = useTransitionsStore(s => s.transitions.find(t => t.id === transitionId));
  const [requestedTab, setRequestedTab] = useState(transitionId ? "transition" : "clip");
  const [lastSelection, setLastSelection] = useState<string | null | undefined>(selectedId ?? transitionId);
  if (lastSelection !== (selectedId ?? transitionId)) { setLastSelection(selectedId ?? transitionId); setRequestedTab(current => transitionId ? "transition" : current === "effects" ? "effects" : "clip"); }
  return (
    <aside aria-label="Thuộc tính" data-shortcut-scope="inspector" className="flex h-full min-h-0 min-w-0 flex-col bg-card text-foreground">
      <div className="flex h-11 shrink-0 items-center gap-2 border-b border-border px-3"><SlidersHorizontal aria-hidden className="size-3.5 text-muted-foreground" /><h2 className="text-xs font-semibold">Thuộc tính</h2></div>
      <Tabs value={requestedTab} onValueChange={setRequestedTab} className="flex min-h-0 flex-1 flex-col gap-0">
        <TabsList variant="segmented" aria-label="Nhóm thuộc tính" className="mx-2 mt-2 flex w-auto shrink-0">
          <TabsTrigger variant="segmented" value="clip" className="min-w-0 flex-auto px-1.5">Clip</TabsTrigger>
          <TabsTrigger variant="segmented" value="effects" data-tour="editor-inspector-effects" className="min-w-0 flex-auto px-1.5">Hiệu ứng</TabsTrigger>
          <TabsTrigger variant="segmented" value="transition" className="min-w-0 flex-auto px-1.5">Chuyển tiếp</TabsTrigger>
          <TabsTrigger variant="segmented" value="canvas" className="min-w-0 flex-auto px-1.5">Khung hình</TabsTrigger>
        </TabsList>
        <TabsContent value="clip" className="relative min-h-0 flex-1 overflow-y-auto px-3 pb-4">
          {item ? <><p className="break-words py-3 text-xs text-muted-foreground">{item.type === "shape" && item.waveformVisualizer ? "Sóng giọng" : item.label}</p><Text item={item} /><Waveform item={item} />{item.type !== "audio" && <Transform item={item} />}<Captions item={item} /><Keyframes item={item} />{CLIPS_WITH_GRADING[item.type] && <Color item={item} />}{CLIPS_WITH_AUDIO[item.type] && <Audio item={item} />}<TemplateAnchor item={item} /></> : <p className="py-6 text-xs text-muted-foreground">Chọn một đoạn trên timeline để chỉnh sửa.</p>}
        </TabsContent>
        <TabsContent value="effects" className="relative min-h-0 flex-1 overflow-y-auto px-3 pb-4">{item ? <Effects item={item} /> : <p className="py-6 text-xs text-muted-foreground">Chọn một đoạn trên timeline để thêm hiệu ứng.</p>}</TabsContent>
        <TabsContent value="transition" className="relative min-h-0 flex-1 overflow-y-auto px-3 pb-4">{transition ? <TransitionSection transition={transition} /> : <p className="py-6 text-xs text-muted-foreground">Chọn một điểm chuyển cảnh trên timeline để chỉnh.</p>}</TabsContent>
        <TabsContent value="canvas" className="relative min-h-0 flex-1 overflow-y-auto px-3 pb-4"><CanvasSection /><MixerSection /></TabsContent>
      </Tabs>
    </aside>
  );
}
