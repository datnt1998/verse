/**
 * The app's route shape in one place: every channel-centred link is built here
 * so a rename touches one module. Pure — safe for the client bundle.
 */
import type { CloneStageGroup } from "./clone-view";

export type CloneListRouteQuery = { stage?: CloneStageGroup | "active" | "all"; q?: string };

/** Shareable list filters; owner is spelled `channel` on pages and `owner` on API routes. */
export function cloneListQuery(ownerChannelId?: string, filters: CloneListRouteQuery = {}): string {
  const params = new URLSearchParams();
  if (ownerChannelId !== undefined) params.set("channel", ownerChannelId);
  if (filters.stage && filters.stage !== "all") params.set("stage", filters.stage);
  if (filters.q?.trim()) params.set("q", filters.q.trim());
  return params.size > 0 ? `?${params.toString().replace(/\+/g, "%20")}` : "";
}

/** Tabs of a channel page, in display order. */
export const CHANNEL_TABS = ["videos", "sources", "templates", "design", "settings"] as const;
export type ChannelTab = (typeof CHANNEL_TABS)[number];

/** URL `?tab=` value → tab; the default tab and any unknown value are "videos". */
export function parseChannelTab(raw: string | undefined): ChannelTab {
  return (CHANNEL_TABS as readonly string[]).includes(raw ?? "") ? (raw as ChannelTab) : "videos";
}

export const appRoutes = {
  overview: "/",
  channels: "/",
  channel: (id: string, tab?: ChannelTab) =>
    tab === undefined || tab === "videos" ? `/channels/${encodeURIComponent(id)}` : `/channels/${encodeURIComponent(id)}?tab=${tab === 'design' ? 'templates' : tab}`,
  /** `undefined` = every channel; "none" = legacy jobs with no owner channel. */
  review: (ownerChannelId?: string, filters: CloneListRouteQuery = {}) => `/review${cloneListQuery(ownerChannelId, filters)}`,
  board: (ownerChannelId?: string, filters: CloneListRouteQuery = {}) => `/${cloneListQuery(ownerChannelId, filters)}`,
  cloneJob: (jobId: string) => `/?video=${encodeURIComponent(jobId)}`,
  queue: "/queue",
  competitors: "/explore",
  competitor: (id: string) => `/explore?rival=${encodeURIComponent(id)}`,
  video: (videoId: string) => `/explore?video=${encodeURIComponent(videoId)}`,
  editor: (projectId: string) => `/editor/${encodeURIComponent(projectId)}`,
  templates: "/library?tab=templates",
  template: (projectId: string) => `/templates/${encodeURIComponent(projectId)}`,
  /** The Kênh nền library; `id` is a channel id such as `twitch:shroud`. */
  backdropChannels: "/library?tab=backdrops",
  backdropChannel: (id: string) => `/library?tab=backdrops&kenh=${encodeURIComponent(id)}`,
} as const;
