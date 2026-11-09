"use client";

import { useId, useRef, useState, type KeyboardEvent } from "react";
import { toast } from "@/editor/ui/toast";
import { ArrowDown, ArrowUp, Eye, EyeOff, Pipette, Plus, Sparkles, Trash2 } from "lucide-react";
import { cn } from "cn";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import { ColorField, EditorIconButton, EditorSelect, KeyframeToggle, PropertyRow } from "@/editor/ui/controls";
import { AnimatedNumber } from "@/editor/ui/inspector/fields";
import { useShortcutAction } from "@/editor/ui/shortcuts";
import { announce } from "@/editor/ui/shell/announcer";
import { executeCommands, getCapabilityManifest, getCommandExecutor } from "@editor/commands";
import { GPU_EFFECT_REGISTRY, getGpuEffect } from "@editor/infrastructure/gpu-effects/registry";
import type { GpuEffectDefinition } from "@editor/infrastructure/gpu-effects/types";
import { useTimelineStore } from "@editor/features/timeline/stores/timeline-store";
import { useItemsStore } from "@editor/features/timeline/stores/items-store";
import { useKeyframesStore } from "@editor/features/timeline/stores/keyframes-store";
import { usePlaybackStore } from "@editor/shared/state/playback";
import { getAnimatablePropertiesForItem } from "@editor/features/keyframes/utils/animatable-properties";
import { getPropertyKeyframes } from "@editor/features/keyframes/utils/interpolation";
import { colorStringToKeyframeValue, interpolateColorKeyframesToHex, normalizeHexColor } from "@editor/features/keyframes/utils/color-keyframes";
import { BLEND_MODE_GROUPS, type BlendMode } from "@editor/types/blend-modes";
import { buildEffectAnimatableProperty, parseEffectAnimatableProperty } from "@editor/types/keyframe";
import type { ItemEffect } from "@editor/types/effects";
import type { TimelineItem } from "@editor/types/timeline";
import { detectChromaKeyColorForItem } from "@editor/features/timeline/utils/chroma-key-detect";

type ParamValue = number | boolean | string;

/** Asks the library panel to switch to the given tab (e.g. "effects"). */
export const OPEN_LIBRARY_TAB_EVENT = "ytf-open-library-tab";

/** Effects that need their own panel (curves, wheels, HSL, mask) or have their own section (LUT), so they are not listed here. */
const BROWSE_EXCLUDED: Record<string, true> = { "gpu-curves": true, "gpu-color-wheels": true, "gpu-secondary-qualifier": true, "gpu-power-window": true, "gpu-lut": true, "gpu-levels": true, "gpu-gradient-map": true };

/** Extra English/technical keywords matched by the effects search alongside the Vietnamese name. */
const EFFECT_SEARCH_KEYWORDS: Record<string, string> = { "gpu-chroma-key": "chroma key xanh green screen phông xanh" };

export function effectSearchText(gpuEffectType: string): string {
  return `${effectName(gpuEffectType)} ${EFFECT_SEARCH_KEYWORDS[gpuEffectType] ?? ""}`;
}

const EFFECT_NAMES: Record<string, string> = {
  "gpu-brightness": "Độ sáng", "gpu-contrast": "Tương phản", "gpu-exposure": "Phơi sáng", "gpu-hue-shift": "Dịch sắc độ", "gpu-invert": "Đảo màu",
  "gpu-levels": "Mức đen/trắng (Levels)", "gpu-saturation": "Bão hoà màu", "gpu-temperature": "Nhiệt độ màu", "gpu-grayscale": "Trắng đen", "gpu-sepia": "Nâu cổ điển (Sepia)",
  "gpu-curves": "Đường cong màu", "gpu-color-wheels": "Bánh xe màu", "gpu-secondary-qualifier": "Chọn màu thứ cấp", "gpu-power-window": "Cửa sổ màu", "gpu-vibrance": "Sắc rực (Vibrance)",
  "gpu-gradient-map": "Bản đồ chuyển màu", "gpu-lut": "Bảng màu (.cube)", "gpu-gaussian-blur": "Làm mờ Gaussian", "gpu-box-blur": "Làm mờ hộp", "gpu-motion-blur": "Mờ chuyển động",
  "gpu-radial-blur": "Mờ xoay tròn", "gpu-zoom-blur": "Mờ phóng to", "gpu-pixelate": "Pixel hoá", "gpu-rgb-split": "Tách kênh RGB", "gpu-twirl": "Xoáy", "gpu-wave": "Sóng",
  "gpu-trigger-wave": "Sóng lan toả", "gpu-bulge": "Phình/Thắt", "gpu-kaleidoscope": "Kính vạn hoa", "gpu-mirror": "Đối xứng gương", "gpu-fluted-glass": "Kính gân",
  "gpu-ripple-glass": "Kính gợn sóng", "gpu-glass-mosaic": "Kính khảm", "gpu-blocks": "Khối vuông", "gpu-droste": "Hiệu ứng Droste", "gpu-vignette": "Tối viền",
  "gpu-grain": "Hạt phim", "gpu-sharpen": "Làm nét", "gpu-posterize": "Giảm mức màu", "gpu-glow": "Phát sáng", "gpu-edge-detect": "Dò cạnh", "gpu-scanlines": "Đường quét",
  "gpu-color-glitch": "Nhiễu màu", "gpu-block-glitch": "Nhiễu khối", "gpu-crt": "Màn hình CRT", "gpu-halftone": "Điểm bán sắc (Halftone)", "gpu-dither": "Dither",
  "gpu-ascii": "ASCII", "gpu-threshold": "Ngưỡng đen trắng", "gpu-vhs": "Băng VHS", "gpu-ink": "Nét mực", "gpu-pixel-sort": "Sắp xếp pixel (vệt)",
  "gpu-pixel-sort-hq": "Sắp xếp pixel", "gpu-chroma-key": "Tách phông màu",
};

export const PARAM_LABELS: Record<string, string> = {
  amount: "Mức", radius: "Bán kính", samples: "Độ mịn", angle: "Góc", shutterAngle: "Độ nhoè chuyển động", centerX: "Tâm X", centerY: "Tâm Y", size: "Kích thước",
  strength: "Cường độ", intensity: "Cường độ", shift: "Độ dịch", span: "Dải", flow: "Dòng chảy", inputBlack: "Đen đầu vào", inputWhite: "Trắng đầu vào",
  outputBlack: "Đen đầu ra", outputWhite: "Trắng đầu ra", gamma: "Độ sáng vùng giữa", temperature: "Nhiệt độ", tint: "Sắc xanh – hồng", exposure: "Độ phơi sáng", offset: "Độ lệch",
  preset: "Bảng màu", customStops: "Điểm màu tuỳ chỉnh", mix: "Độ pha trộn", rotation: "Xoay", amplitudeX: "Biên độ ngang", amplitudeY: "Biên độ dọc",
  frequencyX: "Tần số ngang", frequencyY: "Tần số dọc", frequency: "Tần số", decay: "Độ tắt dần", phase: "Vị trí sóng", speed: "Tốc độ", chroma: "Lệch màu",
  scanlineMix: "Đường quét", glowColor: "Màu phát sáng", segments: "Số mảnh", colorBack: "Màu nền", shadows: "Vùng tối", highlights: "Vùng sáng",
  shape: "Hình dạng", distortionShape: "Kiểu biến dạng", distortion: "Biến dạng", stretch: "Kéo giãn", blur: "Làm mờ", edges: "Cạnh", margin: "Lề",
  marginLeft: "Lề trái", marginRight: "Lề phải", marginTop: "Lề trên", marginBottom: "Lề dưới", grainMixer: "Trộn hạt", grainOverlay: "Phủ hạt",
  grainSize: "Cỡ hạt", colorShadow: "Màu bóng", rings: "Số vòng", originX: "Gốc X", originY: "Gốc Y", falloff: "Độ suy giảm", aberration: "Lệch màu viền",
  cells: "Số ô", depth: "Độ sâu", studSize: "Cỡ núm", gap: "Khoảng hở", scale: "Tỉ lệ", spin: "Vòng xoáy", softness: "Độ mềm", roundness: "Độ tròn",
  threshold: "Ngưỡng", samplesPerRing: "Mẫu mỗi vòng", density: "Mật độ", opacity: "Độ mờ đục", coverage: "Độ phủ", blockSize: "Cỡ khối",
  curvature: "Độ cong", scanlines: "Đường quét", vignette: "Tối viền", colorFront: "Màu trước", type: "Kiểu", contrast: "Tương phản", pattern: "Mẫu",
  mode: "Chế độ", style: "Kiểu", palette: "Bảng màu", cellSize: "Cỡ ô", offsetX: "Lệch X", offsetY: "Lệch Y", charSet: "Bộ ký tự",
  customChars: "Ký tự tuỳ chỉnh", font: "Phông chữ", fontSize: "Cỡ chữ", letterSpacing: "Giãn chữ", lineHeight: "Giãn dòng", matchSourceColor: "Giữ màu gốc",
  bgColor: "Màu nền", transparentBg: "Nền trong suốt", asciiOpacity: "Độ đậm ASCII", originalOpacity: "Độ đậm ảnh gốc", brightness: "Độ sáng",
  level: "Mức", bleed: "Lem màu", waviness: "Nhiễu dọc", noise: "Nhiễu băng", scanline: "Đường quét", spacing: "Khoảng cách nét", thickness: "Độ dày nét",
  edgeStrength: "Viền", tone: "Sắc độ", inkColor: "Màu mực", direction: "Hướng", order: "Thứ tự", low: "Ngưỡng thấp", high: "Ngưỡng cao", length: "Độ dài",
  orientation: "Phương", levels: "Số mức", horizontal: "Theo chiều ngang", invert: "Đảo ngược", tolerance: "Độ rộng màu tách", keyColor: "Màu tách", spillSuppression: "Giảm tràn màu",
  invertMask: "Đảo vùng chọn", showMask: "Hiện vùng chọn", pivot: "Điểm xoay", lift: "Vùng tối", gain: "Vùng sáng", hue: "Sắc độ", saturation: "Bão hoà",
};

const OPTION_LABELS: Record<string, string> = {
  hex: "Lục giác", square: "Vuông", classic: "Cổ điển", gooey: "Dẻo", holes: "Lỗ", soft: "Mềm", image: "Theo ảnh", linear: "Tuyến tính", radial: "Xuyên tâm",
  threshold: "Ngưỡng", scaled: "Theo tỉ lệ", bw: "Đen trắng", sepia: "Nâu cổ điển", dense: "Dày", binary: "Nhị phân", symbols: "Ký hiệu", custom: "Tuỳ chỉnh",
  standard: "Chuẩn", simple: "Đơn giản", blocks: "Khối", dots: "Chấm", minimal: "Tối giản", right: "Phải", left: "Trái", down: "Xuống", up: "Lên", bright: "Sáng",
  dark: "Tối", ascending: "Tăng dần", descending: "Giảm dần", horizontal: "Ngang", vertical: "Dọc", green: "Xanh lá", blue: "Xanh dương", ellipse: "Elip",
  rectangle: "Chữ nhật", lines: "Đường kẻ", linesIrregular: "Đường kẻ lệch", wave: "Sóng", zigzag: "Zích zắc", pattern: "Hoa văn", circle: "Tròn", diamond: "Thoi",
  prism: "Lăng kính", lens: "Thấu kính", contour: "Đường viền", cascade: "Thác", flat: "Phẳng", grayscale: "Thang xám", fire: "Lửa", ice: "Băng", sunset: "Hoàng hôn",
  crosses: "Chữ thập", grid: "Lưới", scales: "Vảy",
};


const BLEND_GROUP_LABELS: Record<string, string> = { Normal: "Thông thường", Darken: "Làm tối", Lighten: "Làm sáng", Contrast: "Tương phản", Inversion: "Đảo / trừ", Component: "Thành phần màu" };
const BLEND_LABELS: Record<BlendMode, string> = {
  normal: "Bình thường", dissolve: "Hoà tan (Dissolve)", darken: "Tối hơn (Darken)", multiply: "Nhân (Multiply)", "color-burn": "Nung màu (Color Burn)",
  "linear-burn": "Nung tuyến tính (Linear Burn)", lighten: "Sáng hơn (Lighten)", screen: "Màn hình (Screen)", "color-dodge": "Né màu (Color Dodge)",
  "linear-dodge": "Cộng (Linear Dodge)", overlay: "Phủ (Overlay)", "soft-light": "Ánh sáng dịu (Soft Light)", "hard-light": "Ánh sáng gắt (Hard Light)",
  "vivid-light": "Ánh sáng rực (Vivid Light)", "linear-light": "Ánh sáng tuyến tính (Linear Light)", "pin-light": "Ánh sáng điểm (Pin Light)", "hard-mix": "Trộn cứng (Hard Mix)",
  difference: "Hiệu (Difference)", exclusion: "Loại trừ (Exclusion)", subtract: "Trừ (Subtract)", divide: "Chia (Divide)", hue: "Sắc độ (Hue)",
  saturation: "Bão hoà (Saturation)", color: "Màu (Color)", luminosity: "Độ sáng (Luminosity)",
};

export function effectName(gpuEffectType: string): string {
  return EFFECT_NAMES[gpuEffectType] ?? getGpuEffect(gpuEffectType)?.name ?? gpuEffectType;
}

let supportedEffectIds: ReadonlySet<string> | null | undefined;
/** Effects the engine publishes through its capability manifest; null while the manifest sets no limit. */
function manifestEffectIds(): ReadonlySet<string> | null {
  if (supportedEffectIds === undefined) {
    const ids = getCapabilityManifest().effects;
    supportedEffectIds = ids.length ? new Set(ids) : null;
  }
  return supportedEffectIds;
}

export function browsableEffects(): GpuEffectDefinition[] {
  const supported = manifestEffectIds();
  return [...GPU_EFFECT_REGISTRY.values()].filter(effect => !BROWSE_EXCLUDED[effect.id] && (!supported || supported.has(effect.id)));
}

export function describeEffectError(error: unknown): string {
  return error instanceof Error && error.message ? error.message : "Vui lòng thử lại.";
}

/** Runs one engine action; on failure shows a Vietnamese toast and returns false. */
export function runEffectAction(failure: string, action: () => void): boolean {
  try { action(); return true; } catch (error) { console.error(failure, error); toast.error(failure); return false; }
}

export function currentEffects(itemId: string): ItemEffect[] { return useItemsStore.getState().itemById[itemId]?.effects ?? []; }

/** Changes one effect param; reads the latest state so a continuous slider drag never lags. */
export function setEffectParam(itemId: string, effectId: string, key: string, value: ParamValue): void {
  const entry = currentEffects(itemId).find(candidate => candidate.id === effectId);
  if (!entry) return;
  useTimelineStore.getState().updateEffect(itemId, effectId, { effect: { ...entry.effect, params: { ...entry.effect.params, [key]: value } } });
}

export function removeEffectWithKeyframes(itemId: string, entry: ItemEffect): void {
  const store = useTimelineStore.getState();
  const animation = useKeyframesStore.getState().keyframesByItemId[itemId];
  const owned = (animation?.properties ?? []).filter(group => parseEffectAnimatableProperty(group.property)?.effectId === entry.id).map(group => group.property);
  getCommandExecutor().executeAnonymous("effect.removeWithKeyframes", () => {
    store.removeEffect(itemId, entry.id);
    for (const property of owned) store.removeKeyframesForProperty(itemId, property);
  }, { itemId, effectId: entry.id });
}


function AnimatedColor({ item, entry, paramKey, label, value }: { item: TimelineItem; entry: ItemEffect; paramKey: string; label: string; value: string }) {
  const frame = usePlaybackStore(s => s.currentFrame);
  const animation = useKeyframesStore(s => s.keyframesByItemId[item.id]);
  const [draft, setDraft] = useState<string | null>(null);
  const focused = useRef(false);
  const property = buildEffectAnimatableProperty(entry.effect.gpuEffectType, entry.id, paramKey);
  const localFrame = Math.max(0, Math.min(item.durationInFrames - 1, frame - item.from));
  const keys = getPropertyKeyframes(animation, property);
  const key = keys.find(candidate => candidate.frame === localFrame);
  const supported = getAnimatablePropertiesForItem(item).includes(property);
  const current = keys.length ? interpolateColorKeyframesToHex(keys, localFrame, value) ?? value : value;
  const toggle = () => {
    const store = useTimelineStore.getState();
    const packed = colorStringToKeyframeValue(current);
    if (key) store.removeKeyframe(item.id, property, key.id);
    else if (packed !== null) store.addKeyframe(item.id, property, localFrame, packed);
    announce(`${key ? "Đã xoá" : "Đã thêm"} điểm mốc ${label} tại khung ${localFrame}`);
  };
  useShortcutAction({ id: `keyframe-${item.id}-${property}`, label: `Điểm mốc ${label}`, scope: "inspector", bindings: ["KeyK"], enabled: () => focused.current && supported, run: toggle });
  const change = (next: string) => {
    setDraft(next);
    if (!/^#([0-9a-f]{6}|[0-9a-f]{8})$/i.test(next)) return;
    setDraft(null);
    if (key) {
      const packed = colorStringToKeyframeValue(next);
      if (packed !== null) runEffectAction("Không thể đổi màu", () => useTimelineStore.getState().updateKeyframe(item.id, property, key.id, { value: packed }));
    } else runEffectAction("Không thể đổi màu", () => setEffectParam(item.id, entry.id, paramKey, normalizeHexColor(next) ?? next));
  };
  return <div onFocusCapture={() => { focused.current = true; }} onBlurCapture={event => { if (!event.currentTarget.contains(event.relatedTarget)) { focused.current = false; setDraft(null); } }}>
    <PropertyRow label={label} action={supported && <KeyframeToggle label={label} active={Boolean(key)} onToggle={toggle} />}><ColorField label={label} value={draft ?? current} onChange={change} /></PropertyRow>
  </div>;
}

function BooleanParam({ label, value, onChange }: { label: string; value: boolean; onChange: (next: boolean) => void }) {
  return <PropertyRow label={label}><Switch checked={value} onCheckedChange={onChange} aria-label={label} /></PropertyRow>;
}

function EffectParams({ item, entry, definition }: { item: TimelineItem; entry: ItemEffect; definition: GpuEffectDefinition }) {
  const params = entry.effect.params;
  const rows = Object.entries(definition.params).filter(([, param]) => param.type !== "json" && param.type !== "point" && param.visibleWhen?.(params) !== false);
  const apply = (key: string, value: ParamValue) => runEffectAction("Không thể đổi tham số hiệu ứng", () => setEffectParam(item.id, entry.id, key, value));
  const lutName = definition.id === "gpu-lut" && typeof params.lutName === "string" && params.lutName ? params.lutName : null;
  const [detecting, setDetecting] = useState(false);
  const autoDetect = () => {
    setDetecting(true);
    void detectChromaKeyColorForItem(item)
      .then((keyColor) => { if (keyColor) apply("keyColor", keyColor); else toast("Không tìm thấy nền xanh trong media này"); })
      .catch(() => toast("Không dò được nền media"))
      .finally(() => setDetecting(false));
  };
  return <div className="space-y-2 pt-2">
    {definition.id === "gpu-chroma-key" && item.mediaId && (
      <PropertyRow label="Màu nền">
        <EditorIconButton icon={Pipette} label="Tự dò màu nền" data-tooltip={detecting ? "Đang dò…" : "Dò màu nền từ video"} disabled={detecting} onClick={autoDetect} />
      </PropertyRow>
    )}
    {definition.id === "gpu-lut" && <PropertyRow label="Bảng màu"><span className="block truncate text-xs">{lutName ?? "Chưa nhập file .cube (mục Màu)"}</span></PropertyRow>}
    {rows.length === 0 && definition.id !== "gpu-lut" && <p className="text-xs text-muted-foreground">Hiệu ứng này không có gì để chỉnh.</p>}
    {rows.map(([key, param]) => {
      const label = PARAM_LABELS[key] ?? param.label;
      const value = params[key] ?? param.default;
      if (param.type === "number") {
        const min = param.min ?? -Infinity;
        const max = param.max ?? Infinity;
        const step = param.step ?? (Number.isFinite(max - min) && max - min <= 10 ? 0.01 : 1);
        return <AnimatedNumber key={key} slider={Number.isFinite(min) && Number.isFinite(max)} item={item} property={buildEffectAnimatableProperty(definition.id, entry.id, key)} label={label} value={Number(value)} onChange={next => apply(key, next)} min={min} max={max} step={step} />;
      }
      if (param.type === "boolean") return <BooleanParam key={key} label={label} value={value === true} onChange={next => apply(key, next)} />;
      if (param.type === "color") return <AnimatedColor key={key} item={item} entry={entry} paramKey={key} label={label} value={String(value)} />;
      if (param.type === "select") {
        return <PropertyRow key={key} label={label}><EditorSelect label={label} value={String(value)} onValueChange={next => apply(key, next)}
          options={(param.options ?? []).map(option => ({ value: option.value, label: OPTION_LABELS[option.value] ?? option.label }))}
        /></PropertyRow>;
      }
      return <PropertyRow key={key} label={label}><Input aria-label={label} className="h-7 min-w-0 border-input text-xs" value={String(value)} onChange={event => apply(key, event.target.value)} /></PropertyRow>;
    })}
  </div>;
}

function EffectCard({ item, entry, index, count, open, onToggleOpen, onRemoved }: { item: TimelineItem; entry: ItemEffect; index: number; count: number; open: boolean; onToggleOpen: () => void; onRemoved: () => void }) {
  const bodyId = useId();
  const definition = getGpuEffect(entry.effect.gpuEffectType);
  const name = effectName(entry.effect.gpuEffectType);
  const move = (delta: -1 | 1) => {
    const list = [...currentEffects(item.id)];
    const from = list.findIndex(candidate => candidate.id === entry.id);
    const to = from + delta;
    if (from < 0 || to < 0 || to >= list.length) return;
    [list[from], list[to]] = [list[to]!, list[from]!];
    if (runEffectAction("Không thể sắp xếp hiệu ứng", () => useTimelineStore.getState().setItemEffects([{ itemId: item.id, effects: list }]))) announce(`Đã chuyển ${name} sang vị trí ${to + 1} trên ${list.length}`);
  };
  const toggle = () => {
    if (runEffectAction("Không thể bật hoặc tắt hiệu ứng", () => useTimelineStore.getState().toggleEffect(item.id, entry.id))) announce(`${entry.enabled ? "Đã tắt" : "Đã bật"} hiệu ứng ${name}`);
  };
  const remove = () => {
    if (runEffectAction("Không thể xoá hiệu ứng", () => removeEffectWithKeyframes(item.id, entry))) { announce(`Đã xoá hiệu ứng ${name}`); onRemoved(); }
  };
  const onKeyDown = (event: KeyboardEvent<HTMLElement>) => {
    if (!event.altKey || (event.key !== "ArrowUp" && event.key !== "ArrowDown")) return;
    event.preventDefault();
    move(event.key === "ArrowUp" ? -1 : 1);
  };
  return <li className="rounded-md border border-border bg-card/40 px-1.5 py-1" aria-label={`Hiệu ứng ${index + 1}: ${name}`}>
    <div className="flex items-center gap-0.5" onKeyDown={onKeyDown}>
      <EditorIconButton icon={entry.enabled ? Eye : EyeOff} label={`Bật hiệu ứng ${name}`} aria-pressed={entry.enabled} onClick={toggle} />
      <button type="button" aria-expanded={open} aria-controls={bodyId} onClick={onToggleOpen} className={cn("min-h-7 min-w-0 flex-1 truncate rounded-md px-1 text-left text-xs font-medium focus-visible:ring-2 focus-visible:ring-ring", !entry.enabled && "text-muted-foreground line-through")}>{name}</button>
      <EditorIconButton icon={ArrowUp} label={`Chuyển ${name} lên trước`} shortcut="Alt+↑" aria-disabled={index === 0} onClick={() => move(-1)} />
      <EditorIconButton icon={ArrowDown} label={`Chuyển ${name} xuống sau`} shortcut="Alt+↓" aria-disabled={index === count - 1} onClick={() => move(1)} />
      <EditorIconButton icon={Trash2} label={`Xoá hiệu ứng ${name}`} onClick={remove} />
    </div>
    <div id={bodyId} hidden={!open}>{open && definition && <EffectParams item={item} entry={entry} definition={definition} />}
      {open && !definition && <p className="pt-2 text-xs text-muted-foreground">Hiệu ứng này không còn dùng được. Hãy gỡ nó khỏi đoạn.</p>}</div>
  </li>;
}

function BlendModeRow({ item }: { item: TimelineItem }) {
  const locked = item.type === "shape" && item.isMask === true;
  const value: BlendMode = locked ? "normal" : item.blendMode ?? "normal";
  const change = (blendMode: BlendMode) => {
    const result = executeCommands([{ type: "effect.setBlendMode", params: { itemId: item.id, blendMode } }]);
    if (!result.ok) { toast.error("Không thể đổi chế độ hoà trộn", { description: result.issues.map(issue => issue.message).join("; ") }); return; }
    announce(`Đã đổi chế độ hoà trộn sang ${BLEND_LABELS[blendMode]}`);
  };
  return <PropertyRow label="Hoà trộn"><EditorSelect label="Chế độ hoà trộn" disabled={locked} value={value} onValueChange={next => change(next as BlendMode)}
    groups={BLEND_MODE_GROUPS.map(group => ({ label: BLEND_GROUP_LABELS[group.label] ?? group.label, options: group.modes.map(mode => ({ value: mode, label: BLEND_LABELS[mode] })) }))}
  /></PropertyRow>;
}

export function EffectsSection({ item, onChooseEffect }: { item: TimelineItem; onChooseEffect?: () => void }) {
  const [collapsed, setCollapsed] = useState<ReadonlySet<string>>(new Set());
  const chainRef = useRef<HTMLDivElement>(null);
  if (item.type === "audio") return <p className="py-6 text-xs text-muted-foreground">Đoạn âm thanh không dùng hiệu ứng hình ảnh. Hãy chọn một đoạn video hoặc ảnh.</p>;
  const effects = item.effects ?? [];
  const toggleOpen = (id: string) => setCollapsed(prev => { const next = new Set(prev); if (!next.delete(id)) next.add(id); return next; });
  const chooseEffect = onChooseEffect ?? (() => { window.dispatchEvent(new CustomEvent(OPEN_LIBRARY_TAB_EVENT, { detail: "effects" })); });
  return <div className="space-y-3 pt-3" data-tour="editor-effects-panel">
    <div ref={chainRef} tabIndex={-1} className="space-y-2 outline-none">
      <h3 className="text-xs font-semibold">Hiệu ứng đang dùng ({effects.length})</h3>
      {effects.length === 0
        ? <div className="space-y-2 rounded-md border border-dashed border-border p-3 text-xs text-muted-foreground"><p>Đoạn này chưa có hiệu ứng nào.</p><Button type="button" variant="outline" size="sm" onClick={chooseEffect}><Sparkles aria-hidden />Chọn hiệu ứng</Button></div>
        : <ol className="space-y-1.5">{effects.map((entry, index) => <EffectCard key={entry.id} item={item} entry={entry} index={index} count={effects.length} open={!collapsed.has(entry.id)} onToggleOpen={() => toggleOpen(entry.id)} onRemoved={() => chainRef.current?.focus()} />)}</ol>}
    </div>
    {effects.length > 0 && <Button type="button" variant="outline" size="sm" className="w-full" onClick={chooseEffect}><Plus aria-hidden />Thêm hiệu ứng khác</Button>}
    <details className="group border-t border-border pt-2"><summary className="flex min-h-7 cursor-pointer list-none items-center gap-1 text-xs text-muted-foreground">Nâng cao: cách hoà trộn hình</summary><div className="pt-2"><BlendModeRow item={item} /></div></details>
  </div>;
}
