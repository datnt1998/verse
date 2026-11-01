/**
 * Clone presentation contract: the wire schemas the clone screens validate after every
 * `GET /api/clone/…` response, plus the pure helpers that turn a snapshot into plain state,
 * progress, failures, media links and image slots. Pure/type-only — safe for the client bundle.
 */
import { z } from "zod";

import { AWAITING_REVIEW_CODE, type CloneStatusSnapshot, type CloneStepName } from "./clone-contract";
import { autoRetryAt } from "./clone-retry";
import { CLONE_STEPS, stageOf } from "./clone-steps";
import { captionAlignmentSchema, cloneScoreAllowsExport, cloneScoreSchema } from "./clone-quality";

/** Unknown totals stay indeterminate; never invent completion from elapsed time. */
export function measuredProgress(progress: { unitsDone: number; unitsTotal: number | null; phase?: string } | null): number | null {
  if (!progress || progress.unitsTotal === null || !Number.isFinite(progress.unitsTotal) || progress.unitsTotal <= 0 || !Number.isFinite(progress.unitsDone)) return null;
  return Math.min(100, Math.max(0, Math.floor(progress.unitsDone / progress.unitsTotal * 100)));
}

/** Out-of-order polling responses may not overwrite a newer job snapshot. */
export function isNewerSnapshot(current: CloneStatusSnapshot | null, incoming: CloneStatusSnapshot): boolean {
  return !current || (incoming.job.id === current.job.id && incoming.version >= current.version);
}

/* ------------------------------ wire schemas ------------------------------ */

const isoString = z.string().min(1);

const ownerChannelWireSchema = z.object({ id: z.string(), name: z.string(), avatarUrl: z.string().nullable() }).nullable();

export const cloneSourceWireSchema = z.object({
  id: z.string(),
  url: z.string(),
  title: z.string(),
  channelId: z.string().nullable(),
  channelTitle: z.string().nullable(),
  thumbnailUrl: z.string().nullable(),
  durationSec: z.number().nullable(),
  scannedAt: isoString,
});
export type CloneSourceWire = z.infer<typeof cloneSourceWireSchema>;

export const cloneStepErrorWireSchema = z.object({
  code: z.string(),
  message: z.string(),
  recovery: z.string().nullable(),
  retryAt: z.string().nullable(),
  /** "provider" only for a real provider reset timestamp; "worker"/absent = own backoff. */
  retryAtSource: z.enum(["provider", "worker"]).optional(),
});

export const cloneStepWireSchema = z.object({
  step: z.string(),
  status: z.string(),
  attempt: z.number().int(),
  progress: z
    .object({ unitsDone: z.number(), unitsTotal: z.number().nullable(), phase: z.string() })
    .nullable(),
  error: cloneStepErrorWireSchema.nullable(),
  retryAt: z.string().nullable(),
  startedAt: z.string().nullable(),
  finishedAt: z.string().nullable(),
});
export type CloneStepWire = z.infer<typeof cloneStepWireSchema>;

export const cloneArtifactWireSchema = z.object({
  path: z.string(),
  sha256: z.string(),
  bytes: z.number().int().nonnegative(),
  step: z.string().optional(),
});

const cloneScoreWireSchema = z
  .object({
    version: z.literal(1),
    total: z.number(),
    passed: z.boolean(),
    items: z.array(
      z.object({
        id: z.string(),
        status: z.enum(["pass", "fail", "unknown"]),
        points: z.number(),
        weight: z.number(),
        measured: z.unknown().optional(),
        expected: z.string().optional(),
        evidence: z.string().optional(),
        fix: z
          .object({
            itemId: z.string().optional(),
            frame: z.number().int().nonnegative().optional(),
            href: z.string().optional(),
            mediaId: z.string().optional(),
          })
          .optional(),
      }),
    ),
    blockers: z.array(
      z.object({
        id: z.string(),
        message: z.string(),
        fix: z
          .object({
            itemId: z.string().optional(),
            frame: z.number().int().nonnegative().optional(),
            href: z.string().optional(),
            mediaId: z.string().optional(),
          })
          .optional(),
      }),
    ),
    binding: z.object({ projectRevision: z.string(), settingsHash: z.string(), artifactHash: z.string() }),
    measurementId: z.string().nullable(),
    measuredAt: z.string(),
    generation: z.number().int(),
  })
  .nullable();

export const cloneGateWireSchema = z.object({
  fresh: z.boolean(),
  allowed: z.boolean(),
  /** The owner chose "Vẫn xuất": the checklist no longer gates export. */
  accepted: z.boolean(),
  reason: z.string().nullable(),
  blockers: z.array(z.object({ id: z.string(), message: z.string() })),
  /** Plain reasons the checklist is not met. */
  reasons: z.array(z.string()),
});
export type CloneGateWire = z.infer<typeof cloneGateWireSchema>;

export const cloneStatusWireSchema = z.object({
  version: z.number().int(),
  job: z.object({
    id: z.string(),
    sourceKey: z.string(),
    status: z.string(),
    activeStep: z.string().nullable(),
    generation: z.number().int(),
    projectId: z.string().nullable(),
    cancelRequestedAt: z.string().nullable(),
    createdAt: isoString,
    /** Per-video "không tạo ảnh": the pipeline draws no scene images for this video. */
    noImages: z.boolean(),
    /** The owner marked the video as uploaded to YouTube ("Đã đăng"). */
    postedAt: isoString.nullable(),
    /** The owner deleted the exported MP4 after posting; `outputs.mp4` is null from then on. */
    exportRemovedAt: isoString.nullable(),
    updatedAt: isoString,
  }),
  source: cloneSourceWireSchema,
  sourceChannelAvatarUrl: z.string().nullable(),
  sourceChannelTracked: z.boolean(),
  steps: z.array(cloneStepWireSchema),
  ownerChannel: ownerChannelWireSchema,
  artifacts: z.array(cloneArtifactWireSchema),
  retryAt: z.string().optional(),
  /** Published outputs: the exported MP4, the thumbnail and suggested copy. */
  outputs: z.object({
    mp4: z.object({ exportId: z.string(), fileName: z.string(), url: z.string() }).nullable(),
    thumbnail: z.object({ url: z.string() }).nullable(),
    metadata: z
      .object({ title: z.string(), description: z.string(), tags: z.array(z.string()) })
      .nullable(),
  }),
  score: cloneScoreWireSchema,
});
export type CloneStatusWire = z.infer<typeof cloneStatusWireSchema>;

/* -------------------------------- evidence -------------------------------- */

export const cloneScenesEvidenceSchema = z.object({
  revision: z.number().int(),
  generation: z.number().int(),
  approved: z.boolean(),
  detection: z.object({ threshold: z.number(), minSceneSec: z.number() }),
  sceneCount: z.number().int(),
  sourceDurationSec: z.number().nullable(),
  scenes: z.array(
    z.object({
      sceneId: z.string(),
      startSec: z.number(),
      endSec: z.number(),
      text: z.string(),
      keyframeUrl: z.string().nullable(),
      keyframeTimeSec: z.number().nullable(),
      blackFrame: z.boolean(),
      keyframeStale: z.boolean(),
    }),
  ),
});
export type CloneScenesEvidence = z.infer<typeof cloneScenesEvidenceSchema>;

export const cloneImageItemEvidenceSchema = z.object({
  id: z.string(),
  kind: z.enum(["scene", "thumbnail"]),
  status: z.enum(["pending", "running", "review", "approved", "failed", "skipped"]),
  /** Why a skipped scene has no image: the image tool refused it twice, or the owner went on without it. */
  skipReason: z.enum(["refused", "owner"]).optional(),
  prompt: z.string(),
  promptVersion: z.number().int(),
  currentVersion: z.number().int().nullable(),
  reviewVersion: z.number().int().nullable(),
  queuePosition: z.number().int().nullable(),
  referenceUrl: z.string().nullable(),
  url: z.string().nullable(),
  /** Streamable transparent-subject PNG of the displayed version — set only for `cutout` recipe versions. */
  cutoutUrl: z.string().nullable().optional(),
  versions: z.array(
    z.object({
      version: z.number().int(),
      /** Streamable artifact URL — null only for legacy unpublishable versions. */
      url: z.string().nullable(),
      sha256: z.string(),
      width: z.number().int(),
      height: z.number().int(),
      bytes: z.number().int(),
      createdAt: z.string(),
      promptVersion: z.number().int(),
      /** Where the picture came from: drawn by Codex, uploaded by the owner, or the template's cover. */
      source: z.enum(["codex", "upload", "template"]).optional(),
    }),
  ),
});
export type CloneImageItemEvidence = z.infer<typeof cloneImageItemEvidenceSchema>;

export const cloneImagesEvidenceSchema = z.object({
  revision: z.number().int(),
  sceneGeneration: z.number().int(),
  sceneRevision: z.number().int(),
  approved: z.boolean(),
  waiting: z.object({ code: z.string(), retryAt: z.string().nullable() }).nullable(),
  /** `none` when the template draws no images; the cover is then the only item. */
  imageMode: z.literal("none").nullable().optional(),
  items: z.array(cloneImageItemEvidenceSchema),
});
export type CloneImagesEvidence = z.infer<typeof cloneImagesEvidenceSchema>;

export const cloneCaptionsEvidenceSchema = z.object({
  revision: z.number().int(),
  cueCount: z.number().int(),
  language: z.string().nullable(),
  coverage: z.object({ fps: z.number(), speechFrames: z.number(), coveredFrames: z.number(), ratio: z.number() }),
  unresolvedGaps: z.array(z.object({ startMs: z.number(), endMs: z.number() })),
  needsReview: z.array(z.object({ cueId: z.string(), reason: z.string() })),
  vttUrl: z.string().nullable(),
  srtUrl: z.string().nullable(),
});
export type CloneCaptionsEvidence = z.infer<typeof cloneCaptionsEvidenceSchema>;

export const cloneCueSchema = z.object({
  id: z.string(),
  startMs: z.number(),
  endMs: z.number().nullable(),
  text: z.string(),
  source: z.enum(["captions", "asr", "edited"]),
  estimatedWords: z.boolean(),
});
export type CloneCue = z.infer<typeof cloneCueSchema>;

export const cloneBuildEvidenceSchema = z.object({
  projectId: z.string(),
  committedRevision: z.string(),
  generation: z.number().int(),
  itemIds: z.object({
    scenes: z.array(z.string()),
    captions: z.string().nullable(),
    waveform: z.string().nullable(),
    subscribe: z.array(z.string()),
  }),
  media: z.object({
    audioMediaId: z.string().nullable(),
    thumbnailMediaId: z.string().nullable(),
    sceneCount: z.number().int().nullable(),
  }),
});
export type CloneBuildEvidence = z.infer<typeof cloneBuildEvidenceSchema>;

/** One background clip of the job: where it comes from (channel, video, window) and how the fetch stands. */
export const cloneBackdropItemEvidenceSchema = z.object({
  id: z.string(),
  channelId: z.string(),
  /** `null` when the channel left the library after the clip was picked. */
  platform: z.enum(["twitch", "bilibili"]).nullable(),
  channelHandle: z.string().nullable(),
  channelTitle: z.string().nullable(),
  videoId: z.string(),
  kind: z.enum(["clip", "highlight", "vod", "video"]),
  url: z.string(),
  title: z.string().nullable(),
  /** The catalogue's thumbnail of the source video (https), for the tile image. */
  thumbnailUrl: z.string().nullable(),
  /** Whole length of the source video when known, so a range picker can bound itself. */
  videoDurationSec: z.number().nullable(),
  startSec: z.number().int(),
  endSec: z.number().int(),
  /** `random` items are re-picked by "Lấy lại tất cả"; `owner` items were chosen by hand. */
  source: z.enum(["random", "owner"]),
  status: z.enum(["pending", "ready", "failed"]),
  mediaId: z.string().nullable(),
  error: z.object({ code: z.string(), message: z.string() }).nullable(),
});
export type CloneBackdropItemEvidence = z.infer<typeof cloneBackdropItemEvidenceSchema>;

export const cloneBackdropEvidenceSchema = z.object({
  /** Content hash of the manifest; send it back as `revision` so a stale tab cannot overwrite a newer edit. */
  revision: z.string(),
  wantedCount: z.number().int(),
  readyCount: z.number().int(),
  /** `template-clips`: no channel clip could be fetched, so the build uses the template's own clips. */
  fallback: z.enum(["none", "template-clips"]),
  warnings: z.array(z.string()),
  /** The template links channels, so a random re-roll / random add has a source to draw from. */
  randomPicks: z.boolean(),
  /** The backdrop or build step is running: edits are refused until it finishes. */
  busy: z.boolean(),
  /**
   * How well the clips cover the video. `fill`: the job picks clips until their seconds cover the video;
   * `count`: the template fixed the number of clips. `videoSec` is null while the video's length is unknown;
   * `fixedSec` is the template's own background clips, `channelSec` the ready channel clips.
   */
  coverage: z.object({
    mode: z.enum(["fill", "count"]),
    videoSec: z.number().nullable(),
    fixedSec: z.number(),
    channelSec: z.number(),
  }),
  items: z.array(cloneBackdropItemEvidenceSchema),
});
export type CloneBackdropEvidence = z.infer<typeof cloneBackdropEvidenceSchema>;

export const cloneExportEvidenceSchema = z.object({
  id: z.string(),
  fileName: z.string(),
  mime: z.string(),
  size: z.number(),
  status: z.enum(["staged", "ready"]),
  createdAt: z.number(),
  committedAt: z.number().nullable(),
  downloadUrl: z.string(),
  /** Real hash of the committed bytes — null only when the file is gone. */
  sha256: z.string().nullable(),
  gateDiagnostics: z.record(z.string(), z.unknown()).nullable(),
});
export type CloneExportEvidence = z.infer<typeof cloneExportEvidenceSchema>;

export const cloneMeasurementEvidenceSchema = z
  .object({
    id: z.string(),
    status: z.enum(["queued", "running", "measured", "failed", "cancelled", "stale"]),
    binding: z.object({ projectRevision: z.string(), settingsHash: z.string(), artifactHash: z.string() }),
    result: z
      .object({ lufs: z.number(), samples: z.number(), durationSec: z.number() })
      .nullable(),
    error: z.object({ code: z.string(), message: z.string() }).nullable().optional(),
    createdAt: z.string().optional(),
  })
  .nullable();
export type CloneMeasurementEvidence = z.infer<typeof cloneMeasurementEvidenceSchema>;

export const cloneToolsEvidenceSchema = z.object({
  preflights: z.record(
    z.enum(["download", "media", "audio", "images", "measure"]),
    z.object({ ok: z.boolean(), code: z.string().nullable() }),
  ),
  recovery: z.array(
    z.object({
      kind: z.enum(["install", "repair", "login", "pick_detected", "external_setup", "recheck"]),
      tool: z.string(),
      label: z.string(),
      detail: z.string().optional(),
    }),
  ),
  /** Free-space headroom for app-managed installs; null fields mean unknown, never guessed. */
  disk: z
    .object({
      ok: z.boolean(),
      freeBytes: z.number().nullable(),
      requiredBytes: z.number().nullable(),
      recovery: z.string().nullable(),
    })
    .nullable(),
});
export type CloneToolsEvidence = z.infer<typeof cloneToolsEvidenceSchema>;

export const cloneEvidenceSchema = z.object({
  lastLog: z.object({ step: z.string(), line: z.string() }).nullable(),
  scenes: cloneScenesEvidenceSchema.nullable(),
  images: cloneImagesEvidenceSchema.nullable(),
  captions: cloneCaptionsEvidenceSchema.nullable(),
  build: cloneBuildEvidenceSchema.nullable(),
  /** The job's channel background clips; null when neither its template nor an edit uses channels. */
  backdrop: cloneBackdropEvidenceSchema.nullable().optional(),
  /** Full cue payloads arrive only for `?section=captions` — absent on normal polls. */
  captionCues: z.array(cloneCueSchema).optional(),
  captionAlignment: captionAlignmentSchema.nullable().optional(),
  exports: z.array(cloneExportEvidenceSchema),
  measurement: cloneMeasurementEvidenceSchema,
  tools: cloneToolsEvidenceSchema.nullable(),
});
export type CloneJobEvidence = z.infer<typeof cloneEvidenceSchema>;

export const cloneJobResponseSchema = cloneStatusWireSchema.extend({
  wizardUrl: z.string(),
  projectUrl: z.string().nullable(),
  gate: cloneGateWireSchema,
  evidence: cloneEvidenceSchema,
});
export type CloneJobResponse = z.infer<typeof cloneJobResponseSchema>;

export const CLONE_JOB_STATES = ["preparing", "queued", "running", "waiting", "failed", "cancelled", "legacy", "review", "approved"] as const;

export const cloneJobSummarySchema = z.object({
  id: z.string(),
  sourceKey: z.string(),
  status: z.string(),
  activeStep: z.string().nullable(),
  generation: z.number().int(),
  projectId: z.string().nullable(),
  cancelRequestedAt: z.string().nullable(),
  createdAt: isoString,
  updatedAt: isoString,
  sourceTitle: z.string().nullable(),
  source: z.object({
    title: z.string().nullable(),
    thumbnailUrl: z.string().nullable(),
    channelTitle: z.string().nullable(),
  }),
  ownerChannel: ownerChannelWireSchema,
  /** Succeeded step count / total steps — a real fraction, never elapsed-time guesswork. */
  progress: z.object({ done: z.number().int().nonnegative(), total: z.number().int().nonnegative() }),
  projectUrl: z.string().nullable(),
  wizardUrl: z.string(),
  /** Plain state, computed server-side from the step rows (same as `cloneJobState`). */
  state: z.enum(CLONE_JOB_STATES),
});
export type CloneJobSummary = z.infer<typeof cloneJobSummarySchema>;

export const cloneJobListSchema = z.object({ jobs: z.array(cloneJobSummarySchema) });

/** The counters the nav badge, Overview totals and channel table share; `queued` counts the persistent clone queue. */
export const cloneBucketCountsSchema = z.object({
  review: z.number().int().nonnegative(),
  active: z.number().int().nonnegative(),
  attention: z.number().int().nonnegative(),
  doneToday: z.number().int().nonnegative(),
  queued: z.number().int().nonnegative(),
});
export type CloneBucketCounts = z.infer<typeof cloneBucketCountsSchema>;

export const cloneCountsSchema = z.object({
  totals: cloneBucketCountsSchema,
  /** One entry per owner channel that has countable jobs; "none" holds legacy jobs. */
  byOwner: z.record(z.string(), cloneBucketCountsSchema),
});
export type CloneCounts = z.infer<typeof cloneCountsSchema>;
/* ------------------------------ view helpers ------------------------------ */

/** URL that streams a published job artifact through the contained route. */
export function cloneArtifactUrl(jobId: string, artifactPath: string): string {
  return `/api/clone/${encodeURIComponent(jobId)}/artifacts/${encodeURIComponent(artifactPath)}`;
}

/** Tool id → the Settings card anchor that owns its recovery. */
export const TOOL_SETTINGS_ANCHOR: Record<string, string> = {
  "yt-dlp": "ytdlp-title",
  ffmpeg: "ffmpeg-title",
  uv: "pyenv-title",
  transcription: "pyenv-title",
  codex: "codex-title",
  chrome: "chrome-title",
  cookies: "cookies-title",
};

/** Settings URL that preserves the clone return destination and focuses the tool card. */
export function settingsRecoveryUrl(tool: string, returnTo: string): string {
  const anchor = TOOL_SETTINGS_ANCHOR[tool] ?? TOOL_SETTINGS_ANCHOR.cookies;
  return `/settings?returnTo=${encodeURIComponent(returnTo)}#${anchor}`;
}

/** Remaining-time label's smallest honest sample: below this much elapsed time, say nothing. */
const ETA_MIN_ELAPSED_MS = 5_000;

/**
 * Estimated seconds left for a step: elapsed × (1 − p) / p. Honest only once at
 * least 5% of the measured units are done; null means "đang ước tính" —
 * unmeasured progress, no start time, or a near-complete step all return null.
 */
export function cloneEta(
  step: { progress: { unitsDone: number; unitsTotal: number | null } | null; startedAt: string | null },
  nowMs: number,
): number | null {
  const percent = measuredProgress(step.progress);
  if (percent === null || !step.startedAt || percent < 5 || percent >= 100) return null;
  const fraction = percent / 100;
  const elapsed = nowMs - Date.parse(step.startedAt);
  if (!Number.isFinite(elapsed) || elapsed <= 0) return null;
  return Math.max(1, Math.round((elapsed * (1 - fraction)) / fraction / 1000));
}

const RUNNING_POLL_MS = 1_000;
const PARKED_POLL_MS = 3_000;
const PARKED_IDLE_POLLS = 10;
const PARKED_IDLE_POLL_MS = 10_000;
const LONG_IDLE_POLLS = 60;
const LONG_IDLE_POLL_MS = 5_000;

/**
 * Poll cadence for one job: running work updates every second, parked work slowly, a snapshot that
 * stopped changing backs off, finished jobs and hidden pages never poll.
 */
export function clonePollIntervalMs(
  snapshot: Pick<CloneStatusWire, "job" | "steps"> | null,
  options: { visible?: boolean; unchangedPolls?: number } = {},
): number | null {
  if (options.visible === false) return null;
  if (!snapshot) return RUNNING_POLL_MS;
  const { status } = snapshot.job;
  if (status === "succeeded" || status === "failed" || status === "cancelled") return null;
  const running = status === "running" || snapshot.steps.some((step) => step.status === "running");
  const unchanged = options.unchangedPolls ?? 0;
  if (running) return unchanged >= LONG_IDLE_POLLS ? LONG_IDLE_POLL_MS : RUNNING_POLL_MS;
  return unchanged >= PARKED_IDLE_POLLS ? PARKED_IDLE_POLL_MS : PARKED_POLL_MS;
}

/** Everything a card renders from a snapshot; equal signatures mean "nothing changed". */
export function cloneSnapshotSignature(snapshot: Pick<CloneStatusWire, "version" | "job" | "steps">): string {
  const steps = snapshot.steps.map((s) => `${s.step}:${s.status}:${s.attempt}:${s.progress?.unitsDone ?? ""}`);
  return `${snapshot.version}|${snapshot.job.status}|${snapshot.job.cancelRequestedAt ?? ""}|${steps.join(",")}`;
}

/** True while any clone job still has live work — drives notification polling. */
export function cloneJobsActive(jobs: readonly CloneJobSummary[]): boolean {
  // A job parked for the owner's review has no worker; nothing changes until the owner acts.
  return jobs.some((job) => job.state !== "review" && (job.status === "running" || job.status === "pending" || job.status === "waiting"));
}

/* -------------------------------- job view -------------------------------- */

/**
 * Plain wording per step. `weight` is the presentation share of a typical run, used only to place
 * a job on one overall percentage.
 */
const CHAIN_WORDING = {
  download: { weight: 10, running: "Đang tải video", done: "Đã tải video", waiting: "Chờ tải video", failed: "Chưa tải được video gốc." },
  backdrop: { weight: 5, running: "Đang lấy nền video", done: "Đã lấy nền video", waiting: "Chờ lấy nền video", failed: "Chưa lấy được nền video." },
  transcript: { weight: 20, running: "Đang làm phụ đề", done: "Đã làm xong phụ đề", waiting: "Chờ làm phụ đề", failed: "Chưa làm xong phụ đề." },
  metadata: { weight: 5, running: "Đang soạn tiêu đề", done: "Đã soạn xong tiêu đề", waiting: "Chờ soạn tiêu đề", failed: "Chưa soạn được tiêu đề." },
  scenes: { weight: 10, running: "Đang chia cảnh", done: "Đã chia cảnh", waiting: "Chờ chia cảnh", failed: "Chưa chia được cảnh." },
  images: { weight: 30, running: "Đang vẽ ảnh", done: "Đã vẽ xong ảnh", waiting: "Chờ vẽ ảnh", failed: "Chưa vẽ xong ảnh." },
  build: { weight: 10, running: "Đang dựng video", done: "Đã dựng xong video", waiting: "Chờ duyệt ảnh", failed: "Chưa dựng được video." },
  score: { weight: 5, running: "Đang chấm điểm video", done: "Đã chấm xong điểm", waiting: "Chờ chấm điểm", failed: "Chưa chấm được điểm." },
  export: { weight: 5, running: "Đang xuất video", done: "Đã xuất video", waiting: "Chờ xuất video", failed: "Chưa xuất được video." },
} as const satisfies Record<CloneStepName, { weight: number; running: string; done: string; waiting: string; failed: string }>;

/** The unattended chain in run order with its plain wording. */
export const CLONE_CHAIN = CLONE_STEPS.map((step) => ({ step, ...CHAIN_WORDING[step] }));

type JobViewInput = { job: { status: string; postedAt?: string | null }; steps: readonly CloneStepWire[] };

export type CloneJobState = (typeof CLONE_JOB_STATES)[number];

function chainRows(steps: readonly CloneStepWire[]) {
  return CLONE_CHAIN.map((entry) => ({ entry, row: steps.find((step) => step.step === entry.step) ?? null }));
}

/** One plain state per job, from the snapshot alone. */
export function cloneJobState(snapshot: JobViewInput): CloneJobState {
  const rows = chainRows(snapshot.steps);
  if (rows.some(({ row }) => row === null)) return "legacy";
  // The owner-approval gate: build parked on awaiting_review means every image
  // is drawn and the owner has not approved them yet. An image-free job parks
  // the images step the same way when no cover is available — that is a review
  // ask too (upload the thumbnail).
  if (snapshot.steps.some((step) => (step.step === "build" || step.step === "images") && step.status === "waiting" && step.error?.code === AWAITING_REVIEW_CODE)) {
    return "review";
  }
  // The video is finished once every chain step succeeded — later work (opening the editor) never un-finishes it.
  if (rows.every(({ row }) => row?.status === "succeeded")) return "approved";
  switch (snapshot.job.status) {
    case "failed":
      return "failed";
    case "cancelled":
      return "cancelled";
    case "waiting":
      return "waiting";
  }
  // Waiting for a free `clone-job` worker slot — the worker flags the next
  // runnable step `queued` while it holds a lease but not yet a slot.
  if ((snapshot.job.status === "running" || snapshot.job.status === "pending") && rows.some(({ row }) => row?.status !== "running" && row?.progress?.phase === "queued")) return "queued";
  // A step waiting out a short backoff while its worker stays alive.
  if (rows.some(({ row }) => row?.status === "waiting")) return "waiting";
  const running = rows.find(({ row }) => row?.status === "running")?.row;
  if (running) return running.progress?.phase === "queued" ? "queued" : "running";
  return rows.some(({ row }) => row?.status === "succeeded") ? "running" : "preparing";
}

/** Work is still in flight or parked for an automatic retry. */
export function cloneStateActive(state: CloneJobState): boolean {
  return state === "preparing" || state === "queued" || state === "running" || state === "waiting";
}

/** The counter a job lands in; null when it belongs in none (superseded/idle legacy, old finishes). */
export const CLONE_COUNT_BUCKETS = ["review", "active", "attention", "doneToday"] as const;
export type CloneCountBucket = (typeof CLONE_COUNT_BUCKETS)[number];

/**
 * The one place counts and the board agree on a job's bucket. "Cần xử lý"
 * mirrors `cloneStageGroup`: failed, cancelled, and parked jobs (job status
 * `waiting` — a worker only parks a job after a step recorded an error or an
 * exhausted backoff, e.g. the export gate after a below-threshold score).
 */
export function cloneCountBucket(
  state: CloneJobState,
  /** The wire snapshot fields the buckets need; `updatedAt`/`score` tolerate absence. */
  snapshot: JobViewInput & { job: { status: string; updatedAt?: string | null }; score?: unknown },
  now: Date = new Date(),
): CloneCountBucket | null {
  switch (state) {
    case "review":
      return "review";
    case "failed":
    case "cancelled":
      return "attention";
    case "legacy":
      return null;
    case "approved": {
      // The chain's real finish: the latest step `finishedAt`, else the job's last write.
      const stepMs = snapshot.steps
        .map((step) => (step.finishedAt ? Date.parse(step.finishedAt) : Number.NaN))
        .filter(Number.isFinite);
      const finished = stepMs.length > 0 ? Math.max(...stepMs) : (snapshot.job.updatedAt ? Date.parse(snapshot.job.updatedAt) : Number.NaN);
      if (!Number.isFinite(finished)) return null;
      const midnight = new Date(now);
      midnight.setHours(0, 0, 0, 0);
      return finished >= midnight.getTime() ? "doneToday" : null;
    }
    case "waiting":
      return snapshot.job.status === "waiting" ? "attention" : "active";
    case "preparing":
    case "queued":
    case "running":
      // A parked score that cannot unlock export counts even when the row still reads as work.
      if (snapshot.job.status === "waiting") return "attention";
      if (snapshot.job.status !== "running") {
        const score = cloneScoreSchema.safeParse(snapshot.score);
        if (score.success && !cloneScoreAllowsExport(score.data)) return "attention";
      }
      return "active";
    default:
      state satisfies never;
      return null;
  }
}

/* ------------------------------ board groups ------------------------------ */

export const CLONE_STAGE_GROUPS = ["preparing", "images", "review", "rendering", "done", "posted", "attention", "legacy"] as const;
export type CloneStageGroup = (typeof CLONE_STAGE_GROUPS)[number];

/**
 * The board column a job belongs to. Active jobs group by the step they are on;
 * parked, finished and broken jobs group by how they stopped. "Cần xử lý"
 * covers every state the owner must act on: failed, cancelled, parked with an
 * error (incl. a score below the export threshold). A finished video the
 * owner marked as uploaded moves from "Xong" to "Đã đăng".
 */
export function cloneStageGroup(snapshot: JobViewInput): CloneStageGroup {
  const state = cloneJobState(snapshot);
  switch (state) {
    case "legacy":
      return "legacy";
    case "review":
      return "review";
    case "approved":
      return snapshot.job.postedAt ? "posted" : "done";
    case "failed":
    case "cancelled":
      return "attention";
  }
  // A parked job (worker exited on an error or an exhausted backoff) is never
  // an in-flight stage, even when the last step is still an early one.
  if (snapshot.job.status === "waiting") return "attention";
  const rows = chainRows(snapshot.steps);
  const current = rows.find(({ row }) => row?.status === "running")
    ?? rows.find(({ row }) => row !== null && row.status !== "succeeded");
  return current ? stageOf(current.entry.step) : "preparing";
}

export type CloneStepStatusLine = {
  /** Plain label of the step being worked on: "Đang vẽ ảnh (2/6)", "Đang chờ lượt". */
  label: string;
  /** 0–100 from the step's own units; null while unmeasured. */
  percent: number | null;
  /** Seconds left for this step; null means "đang ước tính" (or hidden when `showEta` is false). */
  etaSec: number | null;
  /** Only a running step has an honest ETA; queued, parked and finished jobs show none. */
  showEta: boolean;
  /** The export step is the one running — the owner must not edit the project meanwhile. */
  exporting: boolean;
};

/** The step a board card reports: label, step percent, ETA and whether the export is live. */
export function cloneStepStatus(snapshot: JobViewInput, nowMs: number): CloneStepStatusLine | null {
  const state = cloneJobState(snapshot);
  if (state !== "running" && state !== "queued" && state !== "preparing") return null;
  const progress = cloneJobProgress(snapshot, nowMs);
  const running = chainRows(snapshot.steps).find(({ row }) => row?.status === "running");
  const row = running?.row ?? null;
  const live = state !== "queued" && row !== null;
  return {
    label: progress.phase,
    percent: live ? measuredProgress(row.progress) : null,
    etaSec: live ? cloneEta(row, nowMs) : null,
    showEta: live,
    exporting: live && running?.entry.step === "export",
  };
}

export type CloneJobProgress = {
  /** 0–100 across the whole chain; null while nothing is known yet. */
  percent: number | null;
  /** Plain phase: "Đang vẽ ảnh (2/6)", "Đang chờ lượt"… */
  phase: string;
  /** Whole seconds left for the current phase, only when the progress units support an honest guess. */
  remainingSec: number | null;
};

/** Unit counts that read as "x/y" for the owner; steps measured in seconds or frames stay a bar only. */
const COUNT_PHASES: Record<string, true> = { generate: true };

/** Sub-phase of the export step, so the owner sees why the bar is where it is (and when a render starts over). */
const PHASE_DETAIL: Record<string, string> = {
  rendering: "dựng hình",
  uploading: "lưu file",
  compressing: "nén cho nhẹ",
  "rendering-again": "dựng lại từ đầu",
  "uploading-again": "lưu lại file",
};

/** " (2/6)" for counted phases, " · nén cho nhẹ" for export sub-phases, "" otherwise. */
function phaseSuffix(progress: { phase: string; unitsDone: number; unitsTotal: number | null } | null): string {
  if (!progress) return "";
  if (COUNT_PHASES[progress.phase] && progress.unitsTotal !== null) return ` (${Math.min(progress.unitsDone, progress.unitsTotal)}/${progress.unitsTotal})`;
  const detail = PHASE_DETAIL[progress.phase];
  return detail ? ` · ${detail}` : "";
}

/** One bar across the chain, with a plain phase and (when measurable) the time left. */
export function cloneJobProgress(snapshot: JobViewInput, nowMs: number): CloneJobProgress {
  const state = cloneJobState(snapshot);
  if (state === "preparing") return { percent: null, phase: "Đang chuẩn bị…", remainingSec: null };
  if (state === "review" || state === "approved") return { percent: 100, phase: "Đã xong", remainingSec: null };
  const rows = chainRows(snapshot.steps);
  const active = rows.find(({ row }) => row?.status === "running") ?? rows.find(({ row }) => row !== null && row.status !== "succeeded") ?? null;
  let percent = 0;
  for (const { entry, row } of rows) {
    if (row?.status === "succeeded") percent += entry.weight;
    else if (entry === active?.entry) percent += entry.weight * (measuredProgress(row?.progress ?? null) ?? 0) / 100;
  }
  const rounded = Math.min(99, Math.floor(percent));
  const row = active?.row ?? null;
  if (state === "queued") return { percent: rounded, phase: "Đang chờ lượt", remainingSec: null };
  if (state === "waiting") return { percent: rounded, phase: "Đang chờ để thử lại", remainingSec: null };
  if (!active) return { percent: rounded, phase: "Đang chuẩn bị…", remainingSec: null };
  return { percent: rounded, phase: `${active.entry.running}${phaseSuffix(row?.progress ?? null)}`, remainingSec: remainingSeconds(row, nowMs) };
}

function remainingSeconds(row: CloneStatusWire["steps"][number] | null, nowMs: number): number | null {
  if (!row?.startedAt) return null;
  // The chain bar is stricter than the card ETA: under 10% or 5 s in, say nothing.
  const percent = measuredProgress(row.progress);
  if (percent === null || percent < 10 || nowMs - Date.parse(row.startedAt) < ETA_MIN_ELAPSED_MS) return null;
  return cloneEta(row, nowMs);
}

/** "khoảng 40 giây" / "khoảng 3 phút" for a remaining-time label. */
export function formatRemaining(seconds: number): string {
  if (seconds < 60) return `khoảng ${Math.max(5, Math.round(seconds / 5) * 5)} giây`;
  return `khoảng ${Math.round(seconds / 60)} phút`;
}

/* ----------------------------- progress panel ----------------------------- */

export type CloneTerminalGlyph = "running" | "done" | "failed" | "waiting";

export type CloneTerminalLine = {
  step: CloneStepName;
  glyph: CloneTerminalGlyph;
  /** Plain sentence: "Đang vẽ ảnh (2/6)", "Đã tải video"… */
  text: string;
  /** Whole seconds this step has taken (so far, while it runs); null when it has not started or the clock is unknown. */
  elapsedSec: number | null;
  /** Text bar for the running step, only when its progress is measurable. */
  bar: { cells: string; percent: number; remainingSec: number | null } | null;
};

export type CloneTerminal = {
  /** Whole seconds since the job was created (until its last step finished); null when the clock is unknown. */
  elapsedSec: number | null;
  lines: CloneTerminalLine[];
  /** Latest plain-language events, oldest first. */
  events: string[];
};

const BAR_CELLS = 12;
const EVENT_LIMIT = 4;

/** `████████░░░░` for a 0–100 percentage. */
export function textBar(percent: number): string {
  const filled = Math.round(Math.min(100, Math.max(0, percent)) / 100 * BAR_CELLS);
  return "█".repeat(filled) + "░".repeat(BAR_CELLS - filled);
}

/** "12 giây", "2 phút 05 giây", "1 giờ 03 phút". */
export function formatElapsed(seconds: number): string {
  const whole = Math.max(0, Math.floor(seconds));
  if (whole < 60) return `${whole} giây`;
  if (whole < 3600) return `${Math.floor(whole / 60)} phút ${String(whole % 60).padStart(2, "0")} giây`;
  return `${Math.floor(whole / 3600)} giờ ${String(Math.floor((whole % 3600) / 60)).padStart(2, "0")} phút`;
}

function secondsBetween(fromIso: string | null, toMs: number | null): number | null {
  if (!fromIso || toMs === null) return null;
  const from = Date.parse(fromIso);
  return Number.isFinite(from) ? Math.max(0, Math.round((toMs - from) / 1000)) : null;
}

/**
 * One line per chain step with a glyph, plain text, its own elapsed time and (while it runs) a text
 * bar, plus the latest events. `nowMs` is the client time of the last read; null when none happened.
 */
export function cloneTerminal(
  snapshot: JobViewInput & { job: { createdAt: string } },
  images: readonly CloneImageItemEvidence[] | null | undefined,
  nowMs: number | null,
): CloneTerminal {
  const state = cloneJobState(snapshot);
  const lines = chainRows(snapshot.steps).map(({ entry, row }): CloneTerminalLine => {
    const status = row?.status;
    const queued = status === "running" && row?.progress?.phase === "queued";
    const finishedMs = row?.finishedAt ? Date.parse(row.finishedAt) : null;
    const glyph: CloneTerminalGlyph = status === "succeeded" ? "done" : status === "failed" ? "failed" : status === "running" && !queued ? "running" : "waiting";
    const percent = glyph === "running" ? measuredProgress(row?.progress ?? null) : null;
    const text =
      glyph === "done" ? entry.done : glyph === "failed" ? entry.failed : glyph === "running" ? `${entry.running}${phaseSuffix(row?.progress ?? null)}` : queued ? "Đang chờ lượt" : entry.waiting;
    return {
      step: entry.step,
      glyph,
      text,
      elapsedSec: glyph === "waiting" ? null : secondsBetween(row?.startedAt ?? null, glyph === "running" ? nowMs : Number.isFinite(finishedMs) ? finishedMs : null),
      bar: percent === null ? null : { cells: textBar(percent), percent, remainingSec: remainingSeconds(row, nowMs ?? 0) },
    };
  });
  const finishes = snapshot.steps.flatMap((step) => (step.finishedAt ? [Date.parse(step.finishedAt)] : [])).filter(Number.isFinite);
  const endMs = state === "review" || state === "approved" || state === "failed" || state === "cancelled" ? (finishes.length ? Math.max(...finishes) : null) : nowMs;
  const slots = cloneImageSlots(images);
  const events = slots
    .flatMap((slot) => (slot.status === "ready" ? [`Đã vẽ xong ${slot.label}`] : slot.status === "drawing" ? [`Đang vẽ ${slot.label}`] : slot.status === "queued" ? [`Chờ vẽ lại ${slot.label}`] : slot.status === "failed" ? [`Chưa vẽ được ${slot.label}`] : slot.status === "skipped" ? [`Bỏ qua ${slot.label}`] : []))
    .slice(-EVENT_LIMIT);
  return { elapsedSec: secondsBetween(snapshot.job.createdAt, endMs), lines, events };
}

const STEP_TOOL: Record<string, string> = {
  download: "yt-dlp",
  backdrop: "yt-dlp",
  transcript: "transcription",
  metadata: "codex",
  scenes: "ffmpeg",
  images: "codex",
  build: "ffmpeg",
  export: "chrome",
};

export type CloneFailure = {
  /** One plain sentence for the owner. */
  sentence: string;
  /** Settings card that can fix the cause, when the cause is a missing tool or login. */
  settingsTool: string | null;
  /** Engineering text, shown only behind "Chi tiết". */
  detail: string;
  /** ISO time the app retries by itself — only while it really will (the job is still running). */
  retryAt: string | null;
  /**
   * Set when the quality checklist stopped export: the reasons to show, and the
   * owner may export anyway. Empty when the checklist could not be measured.
   */
  quality: { reasons: string[] } | null;
};

/**
 * The chain step that stopped the job, described in plain words; null when none reported a problem.
 * `now` only decides whether a waiting step's backoff is short enough for the app to ride out.
 */
export function cloneFailure(snapshot: Pick<CloneStatusWire, "job" | "steps"> & { gate?: Pick<CloneGateWire, "reasons" | "accepted"> }, now = Date.now()): CloneFailure | null {
  const found = chainRows(snapshot.steps).find(({ row }) => row?.error && row.error.code !== AWAITING_REVIEW_CODE && (row.status === "failed" || row.status === "waiting"));
  const row = found?.row;
  const error = row?.error;
  if (!found || !row || !error) return null;
  const base = found.entry.failed;
  const needsLogin = row.step === "download" && /cookie|login/i.test(`${error.message} ${error.recovery ?? ""}`);
  // A parked job has no worker left: only a running job waits out its backoff by itself.
  const autoRetry = row.status === "waiting" && snapshot.job.status === "running" && autoRetryAt(row, now) !== null ? error.retryAt : null;
  let sentence: string;
  switch (error.code) {
    case "source_unavailable":
      sentence = "Video gốc không còn tải được. Có thể đã bị gỡ hoặc đang phát trực tiếp.";
      break;
    case "tool_missing":
      sentence = `${base} Máy chưa sẵn sàng cho việc này — mở Cài đặt để hoàn tất.`;
      break;
    case "quota":
      sentence = row.step === "images" ? "Đã hết lượt vẽ ảnh. Bấm Thử lại khi có lượt mới." : autoRetry ? "Gặp sự cố tạm thời. App sẽ tự thử lại." : "Gặp sự cố tạm thời. Bấm Thử lại.";
      break;
    case "interrupted":
      sentence = autoRetry ? "Quá trình bị dừng giữa chừng. App sẽ tự làm tiếp." : "Quá trình bị dừng giữa chừng. Bấm Thử lại để làm tiếp.";
      break;
    case "quality_gate":
      sentence = "Video chưa đạt Độ hoàn thiện nên app chưa xuất. Bạn sửa các mục bên dưới, hoặc bấm Vẫn xuất để xuất như hiện tại.";
      break;
    default:
      sentence = needsLogin ? `${base} Cần đăng nhập YouTube trên máy — mở Cài đặt để chọn.` : `${base} Bấm Thử lại; nếu vẫn lỗi, mở Chi tiết.`;
  }
  return {
    sentence,
    settingsTool: needsLogin ? "cookies" : error.code === "tool_missing" ? (STEP_TOOL[row.step] ?? null) : null,
    detail: [`${row.step} · ${error.code} · lần ${row.attempt}`, error.message, error.recovery].filter(Boolean).join("\n"),
    retryAt: autoRetry,
    // Once the owner chose Vẫn xuất the stop is settled; the job is resuming, so offer no second choice.
    quality: error.code === "quality_gate" && !snapshot.gate?.accepted ? { reasons: snapshot.gate?.reasons ?? [] } : null,
  };
}

const LEGACY_RENDER_VIDEO_PATH = "render/video.mp4";

/**
 * Streamable MP4 of a legacy job made before the approval-gate chain — the
 * only video link a legacy card may offer; kept until legacy jobs age out.
 */
export function cloneLegacyVideo(jobId: string, artifacts: readonly { path: string }[]): { videoUrl: string } | null {
  const published = new Set(artifacts.map((artifact) => artifact.path));
  return published.has(LEGACY_RENDER_VIDEO_PATH)
    ? { videoUrl: cloneArtifactUrl(jobId, LEGACY_RENDER_VIDEO_PATH) }
    : null;
}

/** File name for a download: the source title made safe for every file system. */
export function cloneDownloadName(title: string | null, extension: "mp4" | "jpg"): string {
  const safe = (title ?? "").replace(/[\\/:*?"<>|\u0000-\u001f]+/g, " ").replace(/\s+/g, " ").trim().slice(0, 80).trim();
  return `${safe || "video-clone"}.${extension}`;
}

/**
 * `queued`: a redraw was requested and the old image still shows until the new
 * one is drawn; `waiting`: never drawn yet; `skipped`: the scene shows the
 * image of the scene before it.
 */
export type CloneImageSlotStatus = "ready" | "drawing" | "queued" | "waiting" | "failed" | "skipped";
export type CloneImageSlot = {
  id: string;
  /** "Cảnh 2", "Ảnh bìa". */
  label: string;
  kind: "scene" | "thumbnail";
  status: CloneImageSlotStatus;
  /** Set on a skipped slot: the image tool refused it twice, or the owner chose to go on without it. */
  skipReason: "refused" | "owner" | null;
  url: string | null;
  /** Transparent-subject variant URL when the recipe cut the subject out. */
  cutoutUrl: string | null;
};

/** A slot whose image is being (re)drawn now or queued to be: no new redraw, no approval. */
export function cloneSlotBusy(slot: CloneImageSlot): boolean {
  return slot.status === "drawing" || slot.status === "queued";
}

/** Scenes in order, then the cover — one slot per image the video uses. */
export function cloneImageSlots(items: readonly CloneImageItemEvidence[] | null | undefined): CloneImageSlot[] {
  const list = items ?? [];
  const scenes = list.filter((item) => item.kind === "scene");
  const cover = list.filter((item) => item.kind === "thumbnail");
  return [...scenes, ...cover].map((item) => ({
    id: item.id,
    label: item.kind === "thumbnail" ? "Ảnh bìa" : `Cảnh ${scenes.indexOf(item) + 1}`,
    kind: item.kind,
    status: item.status === "running" ? "drawing" : item.status === "failed" ? "failed" : item.status === "skipped" ? "skipped" : item.status === "pending" ? (item.url ? "queued" : "waiting") : item.url ? "ready" : "waiting",
    skipReason: item.status === "skipped" ? (item.skipReason ?? "owner") : null,
    url: item.url,
    cutoutUrl: item.cutoutUrl ?? null,
  }));
}
