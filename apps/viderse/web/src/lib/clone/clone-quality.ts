/**
 * Shared contract for the clone "Độ hoàn thiện" checklist: item ids, weights,
 * tolerances, threshold, score JSON schema and the freshness/gate math used by
 * the server scorer, the export gate, the editor meter and the CLI.
 *
 * Scores are integers summed from item weights; a `fail`/`unknown` item earns
 * zero — never a partial credit — and nothing rounds 89.99 up to 90.
 */
import { z } from "zod";

export const CLONE_QUALITY_ITEMS = [
  "duration",
  "sceneCount",
  "cutRhythm",
  "images",
  "captions",
  "loudness",
  "resolution1080p",
  "thumbnail",
  "captionAlignment",
  "captionKaraoke",
  "background",
] as const;
export type CloneQualityItemId = (typeof CLONE_QUALITY_ITEMS)[number];

/** Points per checklist item; a pass earns the full weight, anything else 0. */
export const CLONE_QUALITY_WEIGHTS: Record<CloneQualityItemId, number> = {
  duration: 15,
  sceneCount: 10,
  cutRhythm: 15,
  images: 20,
  captions: 20,
  loudness: 10,
  resolution1080p: 5,
  thumbnail: 5,
  captionAlignment: 0,
  captionKaraoke: 0,
  background: 0,
};

export const CLONE_QUALITY_MAX_TOTAL = 100;
/** Fresh total needed to unlock export (hard prerequisites must also pass). */
export const CLONE_EXPORT_MIN_SCORE = 90;
/** Output duration may differ from the source by at most 10%. */
export const CLONE_DURATION_TOLERANCE_RATIO = 0.1;
/** The editor mix must stay within ±1 LUFS of the source audio's own loudness. */
export const CLONE_LOUDNESS_TOLERANCE_LUFS = 1;
/** Target mode is independent of source-relative loudness. */
export const CLONE_LOUDNESS_TARGET_TOLERANCE_LUFS = 1;
/** Cut tolerance: max(2 frames, 100 ms) — expressed in ms of the timeline fps. */
export const CLONE_CUT_TOLERANCE_MAX_MS = 100;
export const CLONE_CUT_TOLERANCE_MAX_FRAMES = 2;
export const CLONE_REQUIRED_WIDTH = 1920;
export const CLONE_REQUIRED_HEIGHT = 1080;
export const CLONE_REQUIRED_FPS = 30;
export const CLONE_THUMBNAIL_WIDTH = 1280;
export const CLONE_THUMBNAIL_HEIGHT = 720;

/** Vietnamese labels + short recovery hints for the meter and CLI. */
export const CLONE_QUALITY_ITEM_LABELS: Record<CloneQualityItemId, { label: string; fix: string }> = {
  duration: { label: "Thời lượng", fix: "Kéo/cắt clip để thời lượng trong ±10% video gốc" },
  sceneCount: { label: "Số cảnh", fix: "Dựng lại project hoặc sửa số ảnh cảnh cho khớp nguồn" },
  cutRhythm: { label: "Nhịp cắt", fix: "Đặt mép ảnh cảnh đúng ranh giới đã duyệt" },
  images: { label: "Ảnh tất cả cảnh", fix: "Duyệt đủ ảnh và giữ clip ảnh hiển thị đủ cảnh" },
  captions: { label: "Phụ đề phủ lời nói", fix: "Hiện rãnh phụ đề và phủ mọi đoạn lời nói" },
  loudness: { label: "Âm lượng", fix: "Đo lại hoặc đưa âm lượng về bằng video gốc" },
  resolution1080p: { label: "1080p", fix: "Dùng cài đặt 1920×1080 · 30 fps, xuất cả project" },
  thumbnail: { label: "Ảnh bìa", fix: "Duyệt ảnh bìa WebP 1280×720 hiện hành" },
  captionAlignment: { label: "Phụ đề khớp tiếng", fix: "Chỉnh mốc từng từ trong Biên tập theo các đoạn lệch" },
  captionKaraoke: { label: "Phụ đề karaoke", fix: "Đo mốc từng từ để giảm các trang ước lượng" },
  background: { label: "Nền video", fix: "Dựng lại project — các clip nền phải phủ kín toàn bộ video" },
};

export const CLONE_QUALITY_ITEM_STATUSES = ["pass", "fail", "unknown"] as const;
export type CloneQualityItemStatus = (typeof CLONE_QUALITY_ITEM_STATUSES)[number];

/** Deep link target the editor header can act on for a failing item. */
export const cloneFixTargetSchema = z.object({
  /** Timeline item to reveal/select, when the fix is a clip edit. */
  itemId: z.string().optional(),
  /** Timeline frame to seek to. */
  frame: z.number().int().nonnegative().optional(),
  /** App route to open (wizard step, settings, …). */
  href: z.string().optional(),
  /** Media library entry relevant to the fix. */
  mediaId: z.string().optional(),
});
export type CloneFixTarget = z.infer<typeof cloneFixTargetSchema>;

export const cloneQualityItemSchema = z.object({
  id: z.enum(CLONE_QUALITY_ITEMS),
  status: z.enum(CLONE_QUALITY_ITEM_STATUSES),
  /** Weight earned right now: the item weight on pass, always 0 otherwise. */
  points: z.number(),
  /** Full weight of the item. */
  weight: z.number(),
  measured: z.unknown().optional(),
  expected: z.string().optional(),
  evidence: z.string().optional(),
  fix: cloneFixTargetSchema.optional(),
});
export type CloneQualityItem = z.infer<typeof cloneQualityItemSchema>;

/** Hard prerequisites that no total can out-weigh. `id` is stable for tests. */
export const cloneHardBlockerSchema = z.object({
  id: z.string(),
  message: z.string(),
  fix: cloneFixTargetSchema.optional(),
});
export type CloneHardBlocker = z.infer<typeof cloneHardBlockerSchema>;

/** The project/artifact binding a score was computed against. */
export const cloneScoreBindingSchema = z.object({
  projectRevision: z.string(),
  settingsHash: z.string(),
  /** sha256-prefixed fingerprint of every upstream artifact hash consulted. */
  artifactHash: z.string(),
});
export type CloneScoreBinding = z.infer<typeof cloneScoreBindingSchema>;

/** Timing evidence is measured on renderer-resolved pages of the scored revision. */
export const captionAlignmentSchema = z.object({
  matchedRatio: z.number().min(0).max(1),
  medianMs: z.number().nonnegative().nullable(),
  p95Ms: z.number().nonnegative().nullable(),
  karaokeRatio: z.number().min(0).max(1),
  worst: z.array(z.object({ itemId: z.string(), frame: z.number().int().nonnegative(), deltaMs: z.number(), text: z.string() })),
});
export type CaptionAlignment = z.infer<typeof captionAlignmentSchema>;
export const CLONE_SCORE_VERSION = 1;
export const cloneScoreSchema = z.object({
  version: z.literal(CLONE_SCORE_VERSION),
  total: z.number(),
  passed: z.boolean(),
  items: z.array(cloneQualityItemSchema),
  /** Hard prerequisites that failed — empty means none. */
  blockers: z.array(cloneHardBlockerSchema),
  binding: cloneScoreBindingSchema,
  /** The measurement request that produced the loudness evidence, when any. */
  measurementId: z.string().nullable(),
  measuredAt: z.string(),
  generation: z.number().int(),
});
export type CloneScore = z.infer<typeof cloneScoreSchema>;

/** Sum the earned points; a pass earns its weight, fail/unknown earn 0. */
export function cloneQualityTotal(items: readonly { weight: number; status: CloneQualityItemStatus }[]): number {
  return items.reduce((sum, item) => sum + (item.status === "pass" ? item.weight : 0), 0);
}

/**
 * Whether a score currently unlocks export: total ≥ 90 with no failed hard
 * prerequisites and every item measured (any `unknown` keeps the gate closed).
 * Note `passed` in the stored score is recomputed here, never trusted blindly.
 */
export function cloneScoreAllowsExport(score: CloneScore): boolean {
  return score.total >= CLONE_EXPORT_MIN_SCORE && score.blockers.length === 0 &&
    score.items.every((item) => item.status !== "unknown");
}

/**
 * Plain reasons a score keeps export closed, hard prerequisites first; empty
 * when it unlocks export. Items already reported as a blocker are not repeated.
 */
export function cloneScoreGateReasons(score: CloneScore): string[] {
  if (cloneScoreAllowsExport(score)) return [];
  const blocked: Partial<Record<string, true>> = {};
  for (const blocker of score.blockers) blocked[blocker.id] = true;
  return [
    ...score.blockers.map((blocker) => blocker.message),
    ...(score.total < CLONE_EXPORT_MIN_SCORE ? [`Điểm ${score.total}/100 chưa đạt ${CLONE_EXPORT_MIN_SCORE}`] : []),
    ...score.items.flatMap((item) => {
      if (blocked[item.id] || item.status === "pass") return [];
      const label = CLONE_QUALITY_ITEM_LABELS[item.id].label;
      return [item.status === "unknown" ? `${label}: chưa đo được` : `${label}: ${item.evidence ?? "không đạt"}`];
    }),
  ];
}

/** ms tolerance for scene-boundary matching at the project fps: max(2 frames, 100 ms). */
export function cloneCutToleranceMs(fps: number): number {
  return Math.max(CLONE_CUT_TOLERANCE_MAX_MS, (CLONE_CUT_TOLERANCE_MAX_FRAMES / fps) * 1000);
}
