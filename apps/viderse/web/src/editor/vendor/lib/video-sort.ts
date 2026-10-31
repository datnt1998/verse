// Shared by the UI and the `ytf` CLI so both rank videos the same way.
import { z } from "zod";

/** One coherent channel baseline; unknown or zero baselines do not invent a ratio. */
export function videoOutlierScore(viewCount: number | null, medianViews: number | null): number | null {
  return viewCount !== null && medianViews !== null && medianViews > 0 ? viewCount / medianViews : null;
}

export const VIDEO_SORTS = ["outlier", "views", "newest"] as const;
export type VideoSort = (typeof VIDEO_SORTS)[number];

/** Channel page defaults: outliers first, long-form only. */
export const DEFAULT_VIDEO_SORT: VideoSort = "outlier";

/** Videos per infinite-scroll page; divisible by every grid column count (1–4). */
export const VIDEO_PAGE_SIZE = 24;

/** Long-form threshold used by the "long videos only" filter (8 minutes). */
export const LONG_FORM_MIN_SEC = 8 * 60;

/** A video is an outlier when it has at least this many times the channel's median views. */
export const OUTLIER_MIN_SCORE = 3;

/* ------------------------------ list filters ------------------------------ */

/** Duration bands of the competitor list filter; "long" is the old `longOnly` toggle. */
export const VIDEO_DURATIONS = ["any", "long", "medium", "short"] as const;
export type VideoDurationFilter = (typeof VIDEO_DURATIONS)[number];

/** Minimum-view thresholds of the competitor list filter. */
export const VIDEO_MIN_VIEWS = [0, 10_000, 100_000, 1_000_000] as const;
export type VideoMinViewsFilter = (typeof VIDEO_MIN_VIEWS)[number];

/** Recency windows (days) of the competitor list filter; 0 = no window. */
export const VIDEO_PUBLISHED_WITHIN = [0, 7, 30, 90] as const;
export type VideoPublishedWithin = (typeof VIDEO_PUBLISHED_WITHIN)[number];

/** Every filter of the competitor video list; both `pageVideos` callers share this shape. */
export const videoListFiltersSchema = z.object({
  sort: z.enum(VIDEO_SORTS),
  duration: z.enum(VIDEO_DURATIONS),
  publishedWithinDays: z.union([z.literal(0), z.literal(7), z.literal(30), z.literal(90)]),
  minViews: z.union([z.literal(0), z.literal(10_000), z.literal(100_000), z.literal(1_000_000)]),
});
export type VideoListFilters = z.infer<typeof videoListFiltersSchema>;

export const DEFAULT_VIDEO_FILTERS: VideoListFilters = {
  sort: DEFAULT_VIDEO_SORT,
  duration: "long",
  publishedWithinDays: 0,
  minViews: 0,
};

type Sortable = {
  outlierScore: number | null;
  viewCount: number | null;
  position: number;
  id: string;
  durationSec: number | null;
};


export function rankVideos<T extends Sortable>(list: readonly T[], sort: VideoSort, minDurationSec = 0): T[] {
  const kept = minDurationSec > 0 ? list.filter((v) => (v.durationSec ?? 0) >= minDurationSec) : [...list];
  const key: Record<VideoSort, (v: T) => number> = {
    outlier: (v) => v.outlierScore ?? -1,
    views: (v) => v.viewCount ?? -1,
    newest: (v) => -v.position,
  };
  return kept.sort((a, b) => key[sort](b) - key[sort](a) || a.position - b.position || (a.id < b.id ? -1 : a.id > b.id ? 1 : 0));
}

/** The loaded videos a shift-click range covers: `anchorIndex`..`index` in display order. */
export function rangeIds<T extends { id: string }>(list: readonly T[], anchorIndex: number, index: number, disabled: ReadonlySet<string>): string[] {
  const [from, to] = anchorIndex <= index ? [anchorIndex, index] : [index, anchorIndex];
  const ids: string[] = [];
  for (let i = from; i <= to; i++) {
    const video = list[i];
    if (video && !disabled.has(video.id)) ids.push(video.id);
  }
  return ids;
}
