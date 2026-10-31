import type { GpuEffectCategory } from "@editor/infrastructure/gpu-effects/types";

export interface EffectPreview {
  /** CSS filter that approximates the effect on the sample picture; absent when only an overlay can hint at it. */
  filter?: string;
  /** Extra layer drawn over the sample picture to hint at effects a CSS filter cannot imitate. */
  overlay?: string;
}

/** Sample picture every effect tile is drawn on, so tiles are comparable at a glance. */
export const EFFECT_SAMPLE_BACKGROUND =
  "radial-gradient(circle at 28% 32%, #fde68a 0 11%, transparent 12%), linear-gradient(180deg, #60a5fa 0%, #f9a8d4 55%, #166534 56%, #14532d 100%)";

const STRIPES = "repeating-linear-gradient(0deg, rgb(0 0 0 / 0.35) 0 1px, transparent 1px 3px)";
const DOTS = "radial-gradient(rgb(0 0 0 / 0.45) 1px, transparent 1.5px) 0 0 / 4px 4px";
const EDGE_DARK = "radial-gradient(circle at center, transparent 40%, rgb(0 0 0 / 0.75) 100%)";
const GLOW = "radial-gradient(circle at 28% 32%, rgb(255 255 255 / 0.85), transparent 55%)";
const SPLIT = "linear-gradient(90deg, rgb(255 0 0 / 0.3) 0 33%, transparent 33% 66%, rgb(0 120 255 / 0.3) 66%)";
const BLOCKS = "conic-gradient(rgb(255 255 255 / 0.25) 25%, transparent 0 50%, rgb(255 255 255 / 0.25) 0 75%, transparent 0) 0 0 / 10px 10px";

const SPECIFIC: Record<string, EffectPreview> = {
  "gpu-brightness": { filter: "brightness(1.45)" },
  "gpu-exposure": { filter: "brightness(1.3) contrast(1.1)" },
  "gpu-contrast": { filter: "contrast(1.7)" },
  "gpu-saturation": { filter: "saturate(2.4)" },
  "gpu-vibrance": { filter: "saturate(1.7)" },
  "gpu-hue-shift": { filter: "hue-rotate(110deg)" },
  "gpu-temperature": { filter: "sepia(0.35) saturate(1.3)" },
  "gpu-grayscale": { filter: "grayscale(1)" },
  "gpu-sepia": { filter: "sepia(1)" },
  "gpu-invert": { filter: "invert(1)" },
  "gpu-threshold": { filter: "grayscale(1) contrast(9)" },
  "gpu-posterize": { filter: "contrast(1.6) saturate(1.8)", overlay: BLOCKS },
  "gpu-gaussian-blur": { filter: "blur(2px)" },
  "gpu-box-blur": { filter: "blur(1.6px)" },
  "gpu-motion-blur": { filter: "blur(1.2px)", overlay: "linear-gradient(90deg, rgb(255 255 255 / 0.25), transparent 60%)" },
  "gpu-radial-blur": { filter: "blur(1.4px)" },
  "gpu-zoom-blur": { filter: "blur(1.4px)", overlay: EDGE_DARK },
  "gpu-sharpen": { filter: "contrast(1.35) saturate(1.15)" },
  "gpu-vignette": { overlay: EDGE_DARK },
  "gpu-glow": { overlay: GLOW },
  "gpu-grain": { overlay: DOTS },
  "gpu-scanlines": { overlay: STRIPES },
  "gpu-crt": { overlay: `${STRIPES}, ${EDGE_DARK}` },
  "gpu-vhs": { filter: "saturate(1.4) contrast(1.1)", overlay: `${STRIPES}, ${SPLIT}` },
  "gpu-rgb-split": { overlay: SPLIT },
  "gpu-color-glitch": { overlay: SPLIT },
  "gpu-block-glitch": { overlay: BLOCKS },
  "gpu-pixelate": { overlay: BLOCKS },
  "gpu-blocks": { overlay: BLOCKS },
  "gpu-halftone": { overlay: DOTS },
  "gpu-dither": { filter: "grayscale(1) contrast(2)", overlay: DOTS },
  "gpu-ascii": { filter: "grayscale(1) contrast(2)", overlay: DOTS },
  "gpu-edge-detect": { filter: "grayscale(1) contrast(3) invert(1)" },
  "gpu-ink": { filter: "grayscale(1) contrast(2.2)" },
};

const BY_CATEGORY: Record<GpuEffectCategory, EffectPreview> = {
  color: { filter: "saturate(1.4) hue-rotate(20deg)" },
  blur: { filter: "blur(1.5px)" },
  distort: { overlay: "repeating-radial-gradient(circle at 50% 50%, rgb(255 255 255 / 0.25) 0 3px, transparent 3px 8px)" },
  stylize: { filter: "contrast(1.3) saturate(1.3)", overlay: DOTS },
  keying: { overlay: "linear-gradient(135deg, rgb(0 255 0 / 0.35), transparent 70%)" },
};

/** What an effect tile should look like: a recognisable approximation drawn with plain CSS, never a real render. */
export function effectPreview(effectId: string, category: GpuEffectCategory): EffectPreview {
  return SPECIFIC[effectId] ?? BY_CATEGORY[category];
}
