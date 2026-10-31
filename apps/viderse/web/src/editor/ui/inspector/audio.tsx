"use client";

import { useState, type ReactNode } from "react";
import { ArrowDown, ArrowUp, Headphones, Plus, RotateCcw, Trash2, Volume2, VolumeX } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Switch } from "@/components/ui/switch";
import { EditorIconButton, EditorSelect, NumberField, PropertyRow, SectionHeader, SliderField, type NumberFieldProps } from "@/editor/ui/controls";
import { announce } from "@/editor/ui/shell/announcer";
import { AnimatedNumber } from "./fields";
import { executeCommands, mintId, type SerializedCommand } from "@editor/commands";
import type { AudioEqSettings, AudioFxEffect, ResolvedAudioEqSettings } from "@editor/types/audio";
import { useItemsStore } from "@editor/features/timeline/stores/items-store";
import type { TimelineItem, VideoItem } from "@editor/types/timeline";
import { useTimelineStore } from "@editor/features/timeline/stores/timeline-store";
import { usePlaybackStore } from "@editor/shared/state/playback";
import { useSelectionStore } from "@editor/shared/state/selection";
import {
  AUDIO_EQ_GAIN_DB_MAX,
  AUDIO_EQ_GAIN_DB_MIN,
  AUDIO_EQ_PRESETS,
  AUDIO_EQ_Q_MAX,
  AUDIO_EQ_Q_MIN,
  findAudioEqPresetId,
  getAudioEqSettings,
  resolveAudioEqSettings,
  sampleAudioEqResponseCurve,
  type AudioEqPresetId,
} from "@editor/shared/utils/audio-eq";
import { AUDIO_FADE_CURVE_X_DEFAULT, AUDIO_FADE_CURVE_X_MAX, AUDIO_FADE_CURVE_X_MIN, getAudioFadeMultiplier } from "@editor/shared/utils/audio-fade-curve";
import { AUDIO_PITCH_CENTS_MAX, AUDIO_PITCH_CENTS_MIN, AUDIO_PITCH_SEMITONES_MAX, AUDIO_PITCH_SEMITONES_MIN } from "@editor/shared/utils/audio-pitch";
import { createAudioFx } from "./create-audio-fx";
import { learnNoiseProfile } from "@editor/features/audio-fx/noise-profile";
import { getOrDecodeAudioSliceForPlayback } from "@editor/features/timeline/deps/composition-runtime";
import { resolveMediaUrl } from "@editor/features/timeline/deps/media-library-resolver";
import { getItemSourceSpanSeconds } from "@editor/features/timeline/utils/media-item-frames";

type Patch = Record<string, number | boolean | string>;
type FxTarget = { itemId: string } | { trackId: string };
type FxType = AudioFxEffect["type"];

const MIN_DB = -60;
const MAX_DB = 12;
const DUCK_DEFAULTS = { duckOthersDb: -12, attackSec: 0.08, releaseSec: 0.25 };

function hasAudio(item: TimelineItem): boolean {
  return item.type === "audio" || item.type === "video" || item.type === "composition";
}

function run(commands: SerializedCommand[]): boolean {
  const result = executeCommands(commands);
  if (result.ok) return true;
  announce(`Không áp dụng được thay đổi âm thanh: ${result.issues.map(issue => issue.message).join("; ")}`, "assertive");
  return false;
}

function setClip(itemId: string, updates: Record<string, unknown>): boolean {
  return run([{ type: "audio.setClip", params: { itemId, updates } }]);
}

function compact(settings: AudioEqSettings): Record<string, number | boolean | string> {
  const out: Record<string, number | boolean | string> = {};
  for (const [key, value] of Object.entries(settings)) if (value !== undefined) out[key] = value;
  return out;
}

function say(label: string, value: number, unit?: string) {
  announce(`${label}: ${Number(value.toFixed(3))}${unit ? ` ${unit}` : ""}`);
}

function NumRow({ label, onChange, unit, ...props }: NumberFieldProps) {
  return <PropertyRow label={label}><NumberField {...props} label={label} unit={unit} onChange={onChange} onCommit={value => say(label, value, unit)} /></PropertyRow>;
}

function ParamRow({ label, onChange, unit, ...props }: NumberFieldProps) {
  return <PropertyRow label={label}><SliderField {...props} label={label} unit={unit} onChange={onChange} onCommit={value => say(label, value, unit)} /></PropertyRow>;
}

function ToggleRow({ label, checked, onChange, disabled }: { label: string; checked: boolean; onChange: (checked: boolean) => void; disabled?: boolean }) {
  return <PropertyRow label={label}><Switch aria-label={label} checked={checked} disabled={disabled} onCheckedChange={next => { onChange(next); announce(`${label}: ${next ? "bật" : "tắt"}`); }} /></PropertyRow>;
}

/**
 * "Lặp lại" — picture-only loop of the source window. Requires the clip to be
 * muted (no embedded audio playing) and to have no linked audio; turning the
 * loop off trims an over-long clip back to its source in the same command.
 */
function LoopRow({ item }: { item: VideoItem }) {
  const hasLinkedAudio = useItemsStore(state => !!item.linkedGroupId && state.items.some(other => other.type === "audio" && other.linkedGroupId === item.linkedGroupId));
  const needsMute = item.embeddedAudioMuted !== true || hasLinkedAudio;
  const disabled = needsMute;
  return <PropertyRow label="Lặp lại">
    <Switch
      aria-label="Lặp lại"
      data-tooltip={needsMute ? "Chỉ lặp được clip đã tắt tiếng và không có âm thanh liên kết" : undefined}
      checked={item.loop === true}
      disabled={disabled}
      onCheckedChange={loop => {
        if (run([{ type: "item.update", params: { id: item.id, updates: { loop } } }])) announce(`Lặp lại: ${loop ? "bật" : "tắt"}`);
      }}
    />
  </PropertyRow>;
}

function SelectRow({ label, value, options, onChange }: { label: string; value: string; options: ReadonlyArray<{ value: string; label: string }>; onChange: (value: string) => void }) {
  return <PropertyRow label={label}><EditorSelect label={label} value={value} onValueChange={next => { onChange(next); announce(`${label}: ${options.find(option => option.value === next)?.label ?? next}`); }} options={options} /></PropertyRow>;
}

function Card({ children }: { children: ReactNode }) {
  return <div className="space-y-2 rounded-md border border-border bg-card/40 p-2">{children}</div>;
}

// ---------------------------------------------------------------------------
// 6-band EQ
// ---------------------------------------------------------------------------

const INNER_TYPES = [
  { value: "low-shelf", label: "Kệ trầm" },
  { value: "peaking", label: "Đỉnh (peaking)" },
  { value: "high-shelf", label: "Kệ cao" },
  { value: "notch", label: "Khuyết (notch)" },
] as const;

type BandSpec = { prefix: "band1" | "low" | "lowMid" | "highMid" | "high" | "band6"; label: string; types: ReadonlyArray<{ value: string; label: string }>; min: number; max: number; slope: boolean; pass?: string };

const EQ_BANDS: readonly BandSpec[] = [
  { prefix: "band1", label: "Dải 1 · cắt/kệ thấp", min: 20, max: 399, slope: true, pass: "high-pass", types: [{ value: "high-pass", label: "Lọc cắt thấp" }, { value: "low-shelf", label: "Kệ trầm" }, { value: "peaking", label: "Đỉnh (peaking)" }, { value: "high-shelf", label: "Kệ cao" }] },
  { prefix: "low", label: "Dải 2 · trầm", min: 20, max: 22000, slope: false, types: INNER_TYPES },
  { prefix: "lowMid", label: "Dải 3 · trung trầm", min: 20, max: 22000, slope: false, types: INNER_TYPES },
  { prefix: "highMid", label: "Dải 4 · trung cao", min: 20, max: 22000, slope: false, types: INNER_TYPES },
  { prefix: "high", label: "Dải 5 · cao", min: 20, max: 22000, slope: false, types: INNER_TYPES },
  { prefix: "band6", label: "Dải 6 · cắt/kệ cao", min: 1400, max: 22000, slope: true, pass: "low-pass", types: [{ value: "low-pass", label: "Lọc cắt cao" }, { value: "low-shelf", label: "Kệ trầm" }, { value: "peaking", label: "Đỉnh (peaking)" }, { value: "high-shelf", label: "Kệ cao" }] },
];

const SLOPES = [6, 12, 18, 24].map(slope => ({ value: String(slope), label: `${slope} dB/quãng tám` }));

const PRESET_LABELS: Record<AudioEqPresetId, string> = {
  flat: "Phẳng",
  "voice-clarity": "Giọng nói rõ",
  podcast: "Podcast",
  warmth: "Ấm áp",
  "bass-boost": "Tăng trầm",
  "de-mud": "Giảm đục",
  smile: "Chữ V (smile)",
  sparkle: "Lấp lánh",
  air: "Thoáng",
  soften: "Làm mềm",
  radio: "Radio",
  telephone: "Điện thoại",
  "dialog-lift": "Nâng thoại",
  "rumble-cut": "Cắt ù trầm",
  brighten: "Sáng hơn",
};

function record(settings: ResolvedAudioEqSettings): Record<string, unknown> {
  return settings as unknown as Record<string, unknown>;
}
const numberAt = (settings: ResolvedAudioEqSettings, key: string): number => { const value = record(settings)[key]; return typeof value === "number" ? value : 0; };
const boolAt = (settings: ResolvedAudioEqSettings, key: string): boolean => record(settings)[key] === true;
const stringAt = (settings: ResolvedAudioEqSettings, key: string): string => { const value = record(settings)[key]; return typeof value === "string" ? value : ""; };

function EqResponse({ settings, label }: { settings: AudioEqSettings; label: string }) {
  const points = sampleAudioEqResponseCurve(settings, { sampleCount: 72, minFrequencyHz: 20, maxFrequencyHz: 20000 });
  const range = 24;
  const min = Math.log(20);
  const span = Math.log(20000) - min;
  const path = points.map((point, index) => `${index ? "L" : "M"}${((Math.log(point.frequencyHz) - min) / span * 240).toFixed(1)},${(32 - Math.max(-range, Math.min(range, point.gainDb)) / range * 30).toFixed(1)}`).join(" ");
  return <svg role="img" aria-label={`${label}: đường đáp ứng tần số, 20 Hz đến 20 kHz, ±24 dB`} viewBox="0 0 240 64" className="h-16 w-full rounded-md border border-border bg-background" preserveAspectRatio="none">
    <title>{label}</title>
    <line x1="0" x2="240" y1="32" y2="32" stroke="currentColor" strokeOpacity="0.3" className="text-muted-foreground" strokeDasharray="3 3" vectorEffect="non-scaling-stroke" />
    <path d={path} fill="none" strokeWidth="1.5" style={{ stroke: "var(--ed-track-audio)" }} vectorEffect="non-scaling-stroke" />
  </svg>;
}

function EqEditor({ title, settings, onPatch, onPreset }: { title: string; settings: AudioEqSettings; onPatch: (patch: Patch) => void; onPreset: (settings: AudioEqSettings | null) => void }) {
  const resolved = resolveAudioEqSettings(settings);
  const presetId = findAudioEqPresetId(settings);
  return <div className="space-y-2">
    <EqResponse settings={settings} label={title} />
    <ToggleRow label={`${title}: bật`} checked={settings.enabled !== false} onChange={enabled => onPatch({ enabled })} />
    <PropertyRow label="Mẫu sẵn" action={<EditorIconButton icon={RotateCcw} label={`Đặt lại ${title}`} onClick={() => { onPreset(null); announce(`Đã đặt lại ${title}`); }} />}>
      <EditorSelect label={`${title}: mẫu sẵn`} value={presetId ?? "custom"} onValueChange={id => {
        const preset = AUDIO_EQ_PRESETS.find(candidate => candidate.id === id);
        if (!preset) return;
        onPreset(preset.settings);
        announce(`${title}: mẫu ${PRESET_LABELS[preset.id]}`);
      }} options={[...(presetId === null ? [{ value: "custom", label: "Tuỳ chỉnh" }] : []), ...AUDIO_EQ_PRESETS.map(preset => ({ value: preset.id, label: PRESET_LABELS[preset.id] }))]} />
    </PropertyRow>
    <ParamRow label={`${title}: độ lợi đầu ra`} value={resolved.outputGainDb} min={AUDIO_EQ_GAIN_DB_MIN} max={AUDIO_EQ_GAIN_DB_MAX} step={0.5} unit="dB" onChange={outputGainDb => onPatch({ outputGainDb })} />
    {EQ_BANDS.map(band => {
      const type = stringAt(resolved, `${band.prefix}Type`);
      const isPass = type === band.pass;
      const enabled = boolAt(resolved, `${band.prefix}Enabled`);
      const name = `${title} · ${band.label}`;
      return <Card key={band.prefix}>
        <ToggleRow label={`${name}: bật`} checked={enabled} onChange={next => onPatch({ [`${band.prefix}Enabled`]: next })} />
        {enabled && <>
          <SelectRow label={`${name}: loại`} value={type} options={band.types} onChange={value => onPatch({ [`${band.prefix}Type`]: value })} />
          <NumRow label={`${name}: tần số`} value={Math.round(numberAt(resolved, `${band.prefix}FrequencyHz`))} min={band.min} max={band.max} step={10} unit="Hz" onChange={value => onPatch({ [`${band.prefix}FrequencyHz`]: value })} />
          {isPass && band.slope && <SelectRow label={`${name}: độ dốc`} value={String(numberAt(resolved, `${band.prefix}SlopeDbPerOct`))} options={SLOPES} onChange={value => onPatch({ [`${band.prefix}SlopeDbPerOct`]: Number(value) })} />}
          {!isPass && <>
            <ParamRow label={`${name}: độ lợi`} value={numberAt(resolved, `${band.prefix}GainDb`)} min={AUDIO_EQ_GAIN_DB_MIN} max={AUDIO_EQ_GAIN_DB_MAX} step={0.5} unit="dB" onChange={value => onPatch({ [`${band.prefix}GainDb`]: value })} />
            <NumRow label={`${name}: Q`} value={Number(numberAt(resolved, `${band.prefix}Q`).toFixed(2))} min={AUDIO_EQ_Q_MIN} max={AUDIO_EQ_Q_MAX} step={0.1} onChange={value => onPatch({ [`${band.prefix}Q`]: value })} />
          </>}
        </>}
      </Card>;
    })}
  </div>;
}

function ClipEq({ item }: { item: TimelineItem }) {
  const settings = getAudioEqSettings(item);
  return <EqEditor title="Âm sắc clip" settings={settings}
    onPatch={patch => setClip(item.id, Object.fromEntries(Object.entries(patch).map(([key, value]) => [`audioEq${key.charAt(0).toUpperCase()}${key.slice(1)}`, value])))}
    onPreset={eq => { run([{ type: "audio.setClipEq", params: { itemId: item.id, eq: eq ? compact(eq) : null } }]); }} />;
}

// ---------------------------------------------------------------------------
// Fade, ducking
// ---------------------------------------------------------------------------

function FadePreview({ item, fps }: { item: TimelineItem; fps: number }) {
  const samples = 64;
  const points = Array.from({ length: samples }, (_, index) => {
    const ratio = index / (samples - 1);
    const gain = getAudioFadeMultiplier({
      frame: ratio * item.durationInFrames,
      durationInFrames: item.durationInFrames,
      fadeInFrames: Math.round((item.audioFadeIn ?? 0) * fps),
      fadeOutFrames: Math.round((item.audioFadeOut ?? 0) * fps),
      fadeInCurve: item.audioFadeInCurve,
      fadeOutCurve: item.audioFadeOutCurve,
      fadeInCurveX: item.audioFadeInCurveX,
      fadeOutCurveX: item.audioFadeOutCurveX,
    });
    return `${(ratio * 100).toFixed(2)},${(35 - Math.max(0, Math.min(1, gain)) * 34).toFixed(2)}`;
  }).join(" ");
  return <svg role="img" aria-label="Xem trước đường cong fade in và fade out theo độ dài clip" viewBox="0 0 100 36" preserveAspectRatio="none" className="h-10 w-full rounded-md border border-border bg-background">
    <title>Đường cong fade</title>
    <polyline points={points} fill="none" strokeWidth="1.5" style={{ stroke: "var(--ed-track-audio)" }} vectorEffect="non-scaling-stroke" />
  </svg>;
}

function DuckingFields({ item }: { item: TimelineItem }) {
  const tracks = useTimelineStore(state => state.tracks);
  const ducking = item.audioDucking;
  const others = tracks.filter(track => !track.isGroup && track.id !== item.trackId);
  const write = (next: { duckOthersDb: number; attackSec?: number; releaseSec?: number; targetTrackIds?: string[] } | null) => setClip(item.id, { audioDucking: next });
  const build = (patch: Partial<NonNullable<typeof ducking>>) => {
    const merged = { ...DUCK_DEFAULTS, ...ducking, ...patch };
    return { duckOthersDb: merged.duckOthersDb, attackSec: merged.attackSec, releaseSec: merged.releaseSec, ...(merged.targetTrackIds?.length ? { targetTrackIds: merged.targetTrackIds } : {}) };
  };
  return <div className="space-y-2">
    <ToggleRow label="Tự hạ âm nền: bật" checked={Boolean(ducking)} onChange={enabled => { write(enabled ? build({}) : null); }} />
    {ducking && <>
      <ParamRow label="Hạ các rãnh khác" value={ducking.duckOthersDb} min={MIN_DB} max={0} step={0.5} unit="dB" onChange={duckOthersDb => write(build({ duckOthersDb }))} />
      <NumRow label="Tự hạ âm nền: thời gian hạ" value={ducking.attackSec ?? DUCK_DEFAULTS.attackSec} min={0.001} max={2} step={0.01} unit="giây" onChange={attackSec => write(build({ attackSec }))} />
      <NumRow label="Tự hạ âm nền: thời gian nhả" value={ducking.releaseSec ?? DUCK_DEFAULTS.releaseSec} min={0.001} max={5} step={0.01} unit="giây" onChange={releaseSec => write(build({ releaseSec }))} />
      <fieldset className="space-y-1">
        <legend className="text-xs text-muted-foreground">Rãnh bị hạ âm (không chọn = mọi rãnh khác)</legend>
        {others.length === 0 && <p className="text-xs text-muted-foreground">Chưa có rãnh nào khác.</p>}
        {others.map(track => {
          const checked = ducking.targetTrackIds?.includes(track.id) ?? false;
          return <label key={track.id} className="flex min-h-7 items-center gap-2 text-xs">
            <Checkbox aria-label={`Hạ âm rãnh ${track.name}`} checked={checked} onCheckedChange={next => {
              const current = ducking.targetTrackIds ?? [];
              const targetTrackIds = next === true ? [...current, track.id] : current.filter(id => id !== track.id);
              write(build({ targetTrackIds }));
              announce(`Tự hạ âm nền ${next === true ? "áp dụng cho" : "bỏ"} rãnh ${track.name}`);
            }} />
            <span className="min-w-0 truncate">{track.name}</span>
          </label>;
        })}
      </fieldset>
    </>}
  </div>;
}

// ---------------------------------------------------------------------------
// Audio effect chain (compressor, noise reduction, reverb, delay)
// ---------------------------------------------------------------------------

type ParamSpec = { key: string; label: string; min: number; max: number; step: number; unit?: string };
const FX_SPECS: Record<FxType, { label: string; params: ParamSpec[] }> = {
  compressor: { label: "Nén âm", params: [
    { key: "threshold", label: "Ngưỡng", min: -60, max: 0, step: 1, unit: "dB" },
    { key: "ratio", label: "Tỷ lệ nén", min: 1, max: 20, step: 0.5 },
    { key: "attack", label: "Thời gian vào", min: 0.001, max: 1, step: 0.001, unit: "giây" },
    { key: "release", label: "Thời gian ra", min: 0.01, max: 3, step: 0.01, unit: "giây" },
    { key: "knee", label: "Độ mềm ngưỡng", min: 0, max: 40, step: 1, unit: "dB" },
    { key: "makeupGain", label: "Bù âm lượng", min: 0, max: 24, step: 0.5, unit: "dB" },
  ] },
  noiseReduction: { label: "Khử nhiễu", params: [
    { key: "threshold", label: "Ngưỡng", min: -60, max: 0, step: 1, unit: "dB" },
    { key: "reduction", label: "Mức giảm", min: 0, max: 1, step: 0.01 },
    { key: "smoothing", label: "Làm mượt", min: 0, max: 1, step: 0.01 },
  ] },
  reverb: { label: "Vang", params: [
    { key: "roomSize", label: "Cỡ phòng", min: 0, max: 1, step: 0.01 },
    { key: "damping", label: "Hấp thụ", min: 0, max: 1, step: 0.01 },
    { key: "wetLevel", label: "Mức hiệu ứng", min: 0, max: 1, step: 0.01 },
    { key: "dryLevel", label: "Mức gốc", min: 0, max: 1, step: 0.01 },
    { key: "preDelay", label: "Trễ trước", min: 0, max: 100, step: 1, unit: "ms" },
  ] },
  delay: { label: "Tiếng vọng", params: [
    { key: "time", label: "Thời gian", min: 0, max: 2, step: 0.01, unit: "giây" },
    { key: "feedback", label: "Phản hồi", min: 0, max: 0.95, step: 0.01 },
    { key: "wetLevel", label: "Mức hiệu ứng", min: 0, max: 1, step: 0.01 },
  ] },
};
const FX_ORDER: readonly FxType[] = ["compressor", "noiseReduction", "reverb", "delay"];


function paramAt(effect: AudioFxEffect, key: string): number {
  const value = (effect.params as unknown as Record<string, unknown>)[key];
  return typeof value === "number" ? value : 0;
}

function NoiseLearn({ item, effect }: { item: TimelineItem; effect: Extract<AudioFxEffect, { type: "noiseReduction" }> }) {
  const fps = useTimelineStore(state => state.fps);
  const inPoint = useTimelineStore(state => state.inPoint);
  const outPoint = useTimelineStore(state => state.outPoint);
  const clipSeconds = item.durationInFrames / fps;
  const [range, setRange] = useState({ start: 0, end: Math.min(1, clipSeconds) });
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const markRange = (() => {
    if (inPoint === null || outPoint === null) return null;
    const start = Math.max(inPoint, item.from);
    const end = Math.min(outPoint, item.from + item.durationInFrames);
    return end > start ? { start: (start - item.from) / fps, end: (end - item.from) / fps } : null;
  })();
  const learn = async () => {
    setBusy(true);
    setError(null);
    try {
      if (!item.mediaId) throw new Error("Clip không có media nguồn để lấy mẫu nhiễu");
      const url = await resolveMediaUrl(item.mediaId);
      if (!url) throw new Error("Không tải được media nguồn");
      const span = getItemSourceSpanSeconds(item, fps);
      if (!span) throw new Error("Clip không có âm thanh nguồn");
      const speed = item.speed ?? 1;
      const toSource = (seconds: number) => item.isReversed ? span.end - seconds * speed : span.start + seconds * speed;
      const a = toSource(range.start);
      const b = toSource(range.end);
      const start = Math.min(a, b);
      const end = Math.max(a, b);
      const slice = await getOrDecodeAudioSliceForPlayback(item.mediaId, url, { targetTimeSeconds: start, minReadySeconds: Math.max(1, end - start), preRollSeconds: 0 });
      const buffer = slice.buffer;
      if (start < slice.startTime - 0.001 || end > slice.startTime + buffer.duration + 0.001) throw new Error("Chưa giải mã đủ vùng đã chọn, hãy chọn vùng ngắn hơn");
      const channels = Array.from({ length: buffer.numberOfChannels }, (_, channel) => buffer.getChannelData(channel));
      const profile = learnNoiseProfile(channels, buffer.sampleRate, start - slice.startTime, end - slice.startTime);
      if (run([{ type: "audioFx.learnNoiseProfile", params: { target: { itemId: item.id }, effectId: effect.id, profile, range: { startSeconds: start, endSeconds: end } } }])) announce("Đã học mẫu nhiễu từ vùng đã chọn");
    } catch (reason) {
      const message = reason instanceof Error ? reason.message : String(reason);
      setError(message);
      announce(`Không học được mẫu nhiễu: ${message}`, "assertive");
    } finally {
      setBusy(false);
    }
  };
  return <div className="space-y-2">
    <p className="text-xs text-muted-foreground" role="status">{effect.params.profile ? `Đã học mẫu nhiễu (${effect.params.profile.frequencyBins.length} dải tần).` : "Chưa có mẫu nhiễu. Chọn vùng chỉ có tiếng nhiễu rồi bấm học."}</p>
    <NumRow label="Vùng nhiễu: từ" value={Number(range.start.toFixed(3))} min={0} max={Math.max(0, clipSeconds)} step={0.1} unit="giây" onChange={start => setRange(current => ({ start, end: Math.max(current.end, start) }))} />
    <NumRow label="Vùng nhiễu: đến" value={Number(range.end.toFixed(3))} min={0} max={Math.max(0, clipSeconds)} step={0.1} unit="giây" onChange={end => setRange(current => ({ start: Math.min(current.start, end), end }))} />
    <div className="flex flex-wrap gap-2">
      <Button type="button" variant="outline" size="sm" disabled={!markRange} onClick={() => { if (markRange) { setRange(markRange); announce("Đã lấy vùng nhiễu từ điểm vào/ra"); } }}>Lấy từ điểm vào/ra</Button>
      <Button type="button" size="sm" disabled={busy || range.end <= range.start} onClick={() => { void learn(); }}>{busy ? "Đang học…" : "Học mẫu nhiễu"}</Button>
    </div>
    {error && <p role="alert" className="text-xs text-muted-foreground">{error}</p>}
  </div>;
}

function FxCard({ target, effect, index, count, ids, clip }: { target: FxTarget; effect: AudioFxEffect; index: number; count: number; ids: string[]; clip?: TimelineItem }) {
  const spec = FX_SPECS[effect.type];
  const move = (delta: number) => {
    const next = [...ids];
    const [moved] = next.splice(index, 1);
    if (moved === undefined) return;
    next.splice(index + delta, 0, moved);
    if (run([{ type: "audioFx.reorder", params: { target, effectIds: next } }])) announce(`${spec.label}: vị trí ${index + delta + 1} trên ${count}`);
  };
  return <Card>
    <div className="flex items-center gap-1">
      <span className="min-w-0 flex-1 truncate text-xs font-medium">{spec.label}</span>
      <Switch aria-label={`${spec.label}: bật`} checked={effect.enabled} onCheckedChange={enabled => { run([{ type: "audioFx.update", params: { target, effectId: effect.id, enabled } }]); announce(`${spec.label}: ${enabled ? "bật" : "tắt"}`); }} />
      <EditorIconButton icon={ArrowUp} label={`Chuyển ${spec.label} lên`} disabled={index === 0} onClick={() => move(-1)} />
      <EditorIconButton icon={ArrowDown} label={`Chuyển ${spec.label} xuống`} disabled={index === count - 1} onClick={() => move(1)} />
      <EditorIconButton icon={Trash2} label={`Xoá ${spec.label}`} onClick={() => { if (run([{ type: "audioFx.remove", params: { target, effectId: effect.id } }])) announce(`Đã xoá ${spec.label}`); }} />
    </div>
    {spec.params.map(param => <ParamRow key={param.key} label={`${spec.label}: ${param.label}`} value={paramAt(effect, param.key)} min={param.min} max={param.max} step={param.step} unit={param.unit}
      onChange={value => { run([{ type: "audioFx.update", params: { target, effectId: effect.id, params: { [param.key]: value } } }]); }} />)}
    {effect.type === "noiseReduction" && (clip ? <NoiseLearn item={clip} effect={effect} /> : <p className="text-xs text-muted-foreground">Mẫu nhiễu chỉ học được trên từng clip.</p>)}
  </Card>;
}

function FxChain({ target, effects, clip }: { target: FxTarget; effects: readonly AudioFxEffect[] | undefined; clip?: TimelineItem }) {
  const list = effects ?? [];
  const ids = list.map(effect => effect.id);
  return <div className="space-y-2">
    {list.length === 0 && <p className="text-xs text-muted-foreground">Chưa có hiệu ứng âm thanh.</p>}
    {list.map((effect, index) => <FxCard key={effect.id} target={target} effect={effect} index={index} count={list.length} ids={ids} clip={clip} />)}
    <div className="grid grid-cols-2 gap-2">
      {FX_ORDER.filter(type => type !== "noiseReduction" || clip).map(type => <Button key={type} type="button" variant="outline" size="sm" className="justify-start" onClick={() => { if (run([{ type: "audioFx.add", params: { target, effect: createAudioFx(type, mintId()) } }])) announce(`Đã thêm ${FX_SPECS[type].label}`); }}><Plus aria-hidden className="size-3.5" />{FX_SPECS[type].label}</Button>)}
    </div>
  </div>;
}

// ---------------------------------------------------------------------------
// Audio clip
// ---------------------------------------------------------------------------

function AudioBody({ item }: { item: TimelineItem }) {
  const fps = useTimelineStore(state => state.fps);
  const durationSeconds = item.durationInFrames / fps;
  const speed = item.speed ?? 1;
  const mediaClip = item.type === "audio" || item.type === "video";
  const changeSpeed = (next: number) => {
    const clamped = Math.max(0.1, Math.min(10, next));
    if (Math.abs(clamped - speed) < 1e-6) return;
    useTimelineStore.getState().rateStretchItem(item.id, item.from, Math.max(1, Math.round(item.durationInFrames * speed / clamped)), clamped);
  };
  return <>
    <SectionHeader title="Âm lượng">
      <AnimatedNumber item={item} property="volume" label="Âm lượng" value={item.volume ?? 0} min={MIN_DB} max={MAX_DB} step={0.5} unit="dB" onChange={volume => { setClip(item.id, { volume }); }} />
      {item.type === "video" && <ToggleRow label="Tắt tiếng âm thanh gốc" checked={item.embeddedAudioMuted === true} onChange={embeddedAudioMuted => setClip(item.id, { embeddedAudioMuted })} />}
    </SectionHeader>
    <SectionHeader title="Âm vào ra dần">
      <FadePreview item={item} fps={fps} />
      <NumRow label="Vào dần" value={item.audioFadeIn ?? 0} min={0} max={durationSeconds} step={0.1} unit="giây" onChange={audioFadeIn => setClip(item.id, { audioFadeIn })} />
      <ParamRow label="Vào dần: độ cong" value={item.audioFadeInCurve ?? 0} min={-1} max={1} step={0.01} onChange={audioFadeInCurve => setClip(item.id, { audioFadeInCurve })} />
      <ParamRow label="Vào dần: điểm uốn" value={item.audioFadeInCurveX ?? AUDIO_FADE_CURVE_X_DEFAULT} min={AUDIO_FADE_CURVE_X_MIN} max={AUDIO_FADE_CURVE_X_MAX} step={0.01} onChange={audioFadeInCurveX => setClip(item.id, { audioFadeInCurveX })} />
      <NumRow label="Ra dần" value={item.audioFadeOut ?? 0} min={0} max={durationSeconds} step={0.1} unit="giây" onChange={audioFadeOut => setClip(item.id, { audioFadeOut })} />
      <ParamRow label="Ra dần: độ cong" value={item.audioFadeOutCurve ?? 0} min={-1} max={1} step={0.01} onChange={audioFadeOutCurve => setClip(item.id, { audioFadeOutCurve })} />
      <ParamRow label="Ra dần: điểm uốn" value={item.audioFadeOutCurveX ?? AUDIO_FADE_CURVE_X_DEFAULT} min={AUDIO_FADE_CURVE_X_MIN} max={AUDIO_FADE_CURVE_X_MAX} step={0.01} onChange={audioFadeOutCurveX => setClip(item.id, { audioFadeOutCurveX })} />
    </SectionHeader>
    <SectionHeader title="Âm sắc (EQ)" defaultOpen={false}><ClipEq item={item} /></SectionHeader>
    <SectionHeader title="Cao độ" defaultOpen={false}>
      <ParamRow label="Cao độ: nửa cung" value={item.audioPitchSemitones ?? 0} min={AUDIO_PITCH_SEMITONES_MIN} max={AUDIO_PITCH_SEMITONES_MAX} step={1} onChange={audioPitchSemitones => setClip(item.id, { audioPitchSemitones })} />
      <ParamRow label="Cao độ: cent" value={item.audioPitchCents ?? 0} min={AUDIO_PITCH_CENTS_MIN} max={AUDIO_PITCH_CENTS_MAX} step={1} onChange={audioPitchCents => setClip(item.id, { audioPitchCents })} />
    </SectionHeader>
    {mediaClip && <SectionHeader title="Tốc độ và đảo ngược" defaultOpen={false}>
      <NumRow label="Tốc độ" value={Number(speed.toFixed(3))} min={0.1} max={10} step={0.05} unit="×" onChange={changeSpeed} />
      <ToggleRow label="Phát ngược" checked={item.isReversed === true} onChange={() => useTimelineStore.getState().reverseItems([item.id])} />
      {item.type === "video" && <LoopRow item={item} />}
    </SectionHeader>}
    {mediaClip && <SectionHeader title="Hiệu ứng âm thanh" defaultOpen={false}><FxChain target={{ itemId: item.id }} effects={item.audioFx} clip={item} /></SectionHeader>}
    <SectionHeader title="Tự hạ âm nền" defaultOpen={false}><DuckingFields item={item} /></SectionHeader>
  </>;
}

export function AudioSection({ item }: { item: TimelineItem }) {
  if (!hasAudio(item)) return null;
  return <AudioBody item={item} />;
}

// ---------------------------------------------------------------------------
// Mixer
// ---------------------------------------------------------------------------

function TrackStrip({ trackId }: { trackId: string }) {
  const track = useTimelineStore(state => state.tracks.find(candidate => candidate.id === trackId));
  if (!track) return null;
  const eqSettings: AudioEqSettings = track.audioEq ?? {};
  return <Card>
    <div className="flex items-center gap-1">
      <span className="min-w-0 flex-1 truncate text-xs font-medium">{track.name}</span>
      <EditorIconButton icon={track.muted ? VolumeX : Volume2} label={`${track.muted ? "Bật tiếng" : "Tắt tiếng"} rãnh ${track.name}`} aria-pressed={track.muted} onClick={() => { if (run([{ type: "audio.setTrackMute", params: { trackId, muted: !track.muted } }])) announce(`Rãnh ${track.name}: ${track.muted ? "bật tiếng" : "tắt tiếng"}`); }} />
      <EditorIconButton icon={Headphones} label={`${track.solo ? "Bỏ solo" : "Solo"} rãnh ${track.name}`} aria-pressed={track.solo} onClick={() => { if (run([{ type: "audio.setTrackSolo", params: { trackId, solo: !track.solo } }])) announce(`Rãnh ${track.name}: ${track.solo ? "bỏ solo" : "solo"}`); }} />
    </div>
    <ParamRow label={`Âm lượng rãnh ${track.name}`} value={track.volume ?? 0} min={MIN_DB} max={MAX_DB} step={0.5} unit="dB" onChange={volume => { run([{ type: "audio.setTrackVolume", params: { trackId, volume } }]); }} />
    <SectionHeader title={`Âm sắc rãnh ${track.name}`} defaultOpen={false}>
      <EqEditor title={`Âm sắc rãnh ${track.name}`} settings={eqSettings}
        onPatch={patch => { run([{ type: "audio.setTrackEq", params: { trackId, eq: { ...compact(eqSettings), ...patch } } }]); }}
        onPreset={eq => { run([{ type: "audio.setTrackEq", params: { trackId, eq: eq ? compact(eq) : null } }]); }} />
    </SectionHeader>
    <SectionHeader title={`Hiệu ứng rãnh ${track.name}`} defaultOpen={false}><FxChain target={{ trackId }} effects={track.audioFx} /></SectionHeader>
  </Card>;
}

function MasterStrip() {
  const masterDb = usePlaybackStore(state => state.masterBusDb);
  const busEq = usePlaybackStore(state => state.busAudioEq);
  const eqSettings: AudioEqSettings = busEq ?? {};
  return <div className="space-y-2">
    <PropertyRow label="Âm lượng tổng" action={<EditorIconButton icon={RotateCcw} label="Đặt âm lượng tổng về 0 dB" disabled={masterDb === 0} onClick={() => { if (run([{ type: "audio.setMasterGain", params: { db: 0 } }])) announce("Âm lượng tổng: 0 dB"); }} />}>
      <SliderField label="Âm lượng tổng" value={masterDb} min={MIN_DB} max={MAX_DB} step={0.5} unit="dB" onChange={db => { run([{ type: "audio.setMasterGain", params: { db } }]); }} onCommit={db => say("Âm lượng tổng", db, "dB")} />
    </PropertyRow>
    <SectionHeader title="Âm sắc tổng" defaultOpen={false}>
      <EqEditor title="Âm sắc tổng" settings={eqSettings}
        onPatch={patch => { run([{ type: "audio.setBusEq", params: { eq: { ...compact(eqSettings), ...patch } } }]); }}
        onPreset={eq => { run([{ type: "audio.setBusEq", params: { eq: eq ? compact(eq) : null } }]); }} />
    </SectionHeader>
  </div>;
}

export function MixerSection() {
  const tracks = useTimelineStore(state => state.tracks);
  const items = useTimelineStore(state => state.items);
  const selectedIds = useSelectionStore(state => state.selectedItemIds);
  const selected = items.find(item => selectedIds.includes(item.id) && hasAudio(item));
  const ordered = tracks.filter(track => !track.isGroup).sort((a, b) => a.order - b.order);
  return <>
    <SectionHeader title="Âm lượng tổng"><MasterStrip /></SectionHeader>
    <SectionHeader title="Âm lượng từng rãnh">
      {ordered.length === 0 && <p className="text-xs text-muted-foreground">Chưa có rãnh nào.</p>}
      {ordered.map(track => <TrackStrip key={track.id} trackId={track.id} />)}
    </SectionHeader>
    <SectionHeader title="Tự hạ âm nền của clip đang chọn" defaultOpen={false}>
      {selected ? <><p className="text-xs text-muted-foreground">Clip: {selected.label}</p><DuckingFields item={selected} /></> : <p className="text-xs text-muted-foreground">Chọn một clip có âm thanh để tự hạ âm nền.</p>}
    </SectionHeader>
  </>;
}
