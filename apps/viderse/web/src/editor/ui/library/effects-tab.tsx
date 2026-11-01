"use client";

import type { DragEvent } from "react";
import { Check, Sparkles, Trash2 } from "lucide-react";
import { toast } from "@/editor/ui/toast";
import { cn } from "cn";
import { EditorIconButton } from "@/editor/ui/controls";
import { EFFECT_SAMPLE_BACKGROUND, effectPreview } from "@/editor/ui/inspector/effect-preview";
import { browsableEffects, effectName, effectSearchText, removeEffectWithKeyframes, runEffectAction } from "@/editor/ui/inspector/effects";
import { announce } from "@/editor/ui/shell/announcer";
import { applyLibraryDrop, LIBRARY_DRAG_MIME } from "./library-actions";
import { useItemsStore } from "@editor/features/timeline/stores/items-store";
import { useSelectionStore } from "@editor/shared/state/selection";
import type { GpuEffectCategory, GpuEffectDefinition } from "@editor/infrastructure/gpu-effects/types";

export const EFFECT_CATEGORY_LABELS: Record<GpuEffectCategory, string> = { color: "Màu sắc", blur: "Làm mờ", distort: "Biến dạng", stylize: "Phong cách", keying: "Tách phông" };
const CATEGORY_ORDER: GpuEffectCategory[] = ["color", "blur", "distort", "stylize", "keying"];
export const EFFECT_NEEDS_CLIP = "Chọn một đoạn video hoặc ảnh trên timeline để thêm hiệu ứng.";

function EffectSwatch({ effect }: { effect: GpuEffectDefinition }) {
  const preview = effectPreview(effect.id, effect.category);
  return (
    <span aria-hidden className="relative block aspect-[16/9] w-full overflow-hidden rounded-t-md" style={{ background: EFFECT_SAMPLE_BACKGROUND }}>
      <span className="absolute inset-0" style={{ background: EFFECT_SAMPLE_BACKGROUND, filter: preview.filter }} />
      {preview.overlay && <span className="absolute inset-0" style={{ background: preview.overlay }} />}
    </span>
  );
}

function startDrag(event: DragEvent<HTMLButtonElement>, effectId: string) {
  event.dataTransfer.effectAllowed = "copy";
  event.dataTransfer.setData(LIBRARY_DRAG_MIME, JSON.stringify({ type: "effect", effectId }));
}

/** Effect gallery: click to apply to the selected clip, or drag a tile onto any clip on the timeline. */
export function EffectsTab({ query }: { query: string }) {
  const selectedId = useSelectionStore((state) => state.selectedItemIds[0]);
  const selected = useItemsStore((state) => (selectedId ? state.itemById[selectedId] : undefined));
  const target = selected && selected.type !== "audio" ? selected : undefined;
  const applied = target?.effects ?? [];
  const appliedTypes = new Set(applied.map((entry) => entry.effect.gpuEffectType));
  const needle = query.trim().toLocaleLowerCase("vi");
  const matches = browsableEffects().filter((effect) => effectSearchText(effect.id).toLocaleLowerCase("vi").includes(needle));

  const apply = (effect: GpuEffectDefinition) => {
    if (!target) { toast(EFFECT_NEEDS_CLIP); return; }
    runEffectAction("Chưa thêm được hiệu ứng. Hãy thử lại.", () => applyLibraryDrop({ type: "effect", effectId: effect.id }, target.id));
  };

  return (
    <div className="space-y-3" data-tour="editor-effects-gallery">
      <p role="status" className="rounded-md border border-border bg-background/60 p-2 text-[11px] leading-relaxed text-muted-foreground">
        {target ? <>Bấm vào một hiệu ứng để thêm cho <b className="font-medium text-foreground">{target.label}</b>, hoặc kéo nó thả lên đoạn trên timeline.</> : <>{EFFECT_NEEDS_CLIP} Bạn cũng có thể kéo hiệu ứng thả thẳng lên một đoạn.</>}
      </p>
      {target && applied.length > 0 && (
        <section aria-label="Hiệu ứng đang dùng" className="space-y-1">
          <h3 className="text-[11px] font-semibold">Đang dùng cho đoạn này</h3>
          <ul className="space-y-1">
            {applied.map((entry) => {
              const name = effectName(entry.effect.gpuEffectType);
              return (
                <li key={entry.id} className="flex items-center gap-1 rounded-md border border-border bg-card/40 pl-2">
                  <span className="min-w-0 flex-1 truncate text-xs">{name}</span>
                  <EditorIconButton icon={Trash2} label={`Gỡ hiệu ứng ${name}`} onClick={() => { if (runEffectAction("Chưa gỡ được hiệu ứng. Hãy thử lại.", () => removeEffectWithKeyframes(target.id, entry))) announce(`Đã gỡ hiệu ứng ${name}`); }} />
                </li>
              );
            })}
          </ul>
        </section>
      )}
      {CATEGORY_ORDER.map((category) => {
        const inCategory = matches.filter((effect) => effect.category === category);
        if (!inCategory.length) return null;
        return (
          <section key={category} aria-label={EFFECT_CATEGORY_LABELS[category]} className="space-y-1.5">
            <h3 className="text-[11px] font-semibold text-muted-foreground">{EFFECT_CATEGORY_LABELS[category]}</h3>
            <div className="grid grid-cols-2 gap-2">
              {inCategory.map((effect) => {
                const name = effectName(effect.id);
                const used = appliedTypes.has(effect.id);
                return (
                  <button
                    key={effect.id}
                    type="button"
                    draggable
                    aria-disabled={!target}
                    aria-label={target ? `Thêm hiệu ứng ${name}${used ? " (đang dùng)" : ""}` : `${name}. ${EFFECT_NEEDS_CLIP}`}
                    onDragStart={(event) => startDrag(event, effect.id)}
                    onClick={() => apply(effect)}
                    className={cn("group min-w-0 overflow-hidden rounded-md border bg-background text-left transition-colors focus-visible:ring-2 focus-visible:ring-ring", used ? "border-primary" : "border-border hover:border-ring", !target && "opacity-70")}
                  >
                    <EffectSwatch effect={effect} />
                    <span className="flex min-h-9 items-center gap-1 px-1.5 py-1 text-[11px]">
                      <span className="min-w-0 flex-1 break-words">{name}</span>
                      {used && <Check aria-hidden className="size-3.5 shrink-0 text-primary" />}
                    </span>
                  </button>
                );
              })}
            </div>
          </section>
        );
      })}
      {matches.length === 0 && (
        <div className="flex flex-col items-center gap-2 py-6 text-center text-muted-foreground">
          <Sparkles aria-hidden className="size-6" />
          <p>Không có hiệu ứng nào khớp với “{query.trim()}”.</p>
        </div>
      )}
    </div>
  );
}
