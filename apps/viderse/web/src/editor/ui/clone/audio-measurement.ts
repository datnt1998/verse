"use client";

/**
 * Driver for one clone loudness measurement: claims the server request,
 * builds the export composition from the *saved* project JSON (never the
 * user's unsaved edits), streams the exact engine mix PCM to the server,
 * and completes the request. Used by both the minimal /measure page and the
 * in-editor "Đo độ hoàn thiện" action.
 */
import { getEditorStorage } from "@editor/storage/storage-backend";
import { serverMediaToMetadata } from "@editor/storage/media-adapter";
import { convertTimelineToComposition } from "@editor/features/export/utils/timeline-to-composition";
import { resolveMediaUrls } from "@editor/features/timeline/deps/media-library-resolver";
import type { CompositionInputProps } from "@editor/types/export";
import type { Project } from "@editor/types/project";
import type { SubComposition } from "@editor/features/timeline/stores/compositions-store";
import type {
  AudioMeasurementWorkerRequest,
  AudioMeasurementWorkerResponse,
} from "@editor/features/export/workers/audio-measurement-worker.types";
import {
  claimMeasurementApi,
  completeMeasurementApi,
  createMeasurementApi,
  failMeasurementApi,
  uploadMeasurementChunk,
  type MeasurementClaimCredential,
} from "./measurement-runtime";

export interface MeasureProgress {
  phase: "claim" | "decode" | "upload" | "finish";
  /** PCM bytes sent so far. */
  bytesSent: number;
  chunksSent: number;
}

export interface MeasureOutcome {
  requestId: string;
  status: "measured";
  lufs: number | null;
  samples: number;
  durationSec: number;
  /** Server-computed score when the binding stayed fresh. */
  score: unknown;
}

const CHUNK_BYTES = 4 * 1024 ** 2;

type CompositionArgs = Parameters<typeof convertTimelineToComposition>;
type TimelineJson = {
  tracks?: CompositionArgs[0];
  items?: CompositionArgs[1];
  transitions?: CompositionArgs[2];
  keyframes?: CompositionArgs[8];
  busAudioEq?: CompositionArgs[10];
  masterBusDb?: number;
  compositions?: SubComposition[];
};

/** Composition from the saved project JSON — mirrors the export path exactly. */
export function compositionFromProject(project: Project): {
  composition: CompositionInputProps;
  compositions: SubComposition[];
} {
  const timeline = (project.timeline ?? {}) as TimelineJson;
  const composition = convertTimelineToComposition(
    timeline.tracks ?? [],
    timeline.items ?? [],
    timeline.transitions ?? [],
    project.metadata.fps,
    project.metadata.width,
    project.metadata.height,
    null,
    null,
    timeline.keyframes ?? [],
    project.metadata.backgroundColor,
    timeline.busAudioEq,
    timeline.masterBusDb ?? 0,
  );
  return { composition, compositions: timeline.compositions ?? [] };
}

/**
 * Run one measurement end-to-end: create → claim → engine mix → stream PCM →
 * complete. The measured target is always the saved project revision the
 * server bound into the request — unsaved edits are never measured.
 */
export async function runAudioMeasurement(options: {
  projectId: string;
  jobId: string;
  /** Claim this launcher-issued request; the editor action creates its own. */
  request?: MeasurementClaimCredential;
  /** Stable identity for this runtime (tab id / headless session). */
  ownerId: string;
  signal?: AbortSignal;
  onProgress?: (progress: MeasureProgress) => void;
}): Promise<MeasureOutcome> {
  const { projectId, jobId, ownerId, signal } = options;
  const progress = (p: MeasureProgress) => options.onProgress?.(p);
  const storage = getEditorStorage();

  progress({ phase: "claim", bytesSent: 0, chunksSent: 0 });
  const { data: project, revision } = await storage.loadProject(projectId);

  let issued = options.request;
  if (!issued) {
    const created = await createMeasurementApi(jobId);
    issued = { requestId: created.request.id, nonce: created.request.nonce };
  }
  const { nonce } = issued;
  const claimed = await claimMeasurementApi(jobId, {
    requestId: issued.requestId,
    nonce,
    ownerId,
  });
  const request = claimed.request;
  const fence = request.fence!;
  if (request.binding.projectRevision !== revision) {
    await failMeasurementApi(jobId, request.id, fence, {
      code: "stale",
      message: "Project revision khác bản đã bind — tạo yêu cầu mới",
    }).catch(() => undefined);
    throw new Error("Project đã đổi sau khi tạo yêu cầu đo — tạo yêu cầu mới");
  }

  try {
    const { composition, compositions } = compositionFromProject(project);
    progress({ phase: "decode", bytesSent: 0, chunksSent: 0 });
    composition.tracks = await resolveMediaUrls(composition.tracks, { signal });
    const mediaRecords = (await storage.listMedia(projectId)).map(serverMediaToMetadata);

    const worker = new Worker(
      new URL("../../engine/features/export/workers/audio-measurement.worker.ts", import.meta.url),
      { type: "module" },
    );
    const onAbort = () => worker.postMessage({ type: "cancel", requestId: "measure" } satisfies AudioMeasurementWorkerRequest);
    try {
      return await new Promise<MeasureOutcome>((resolve, reject) => {
        let uploadSeq = 0;
        let bytesSent = 0;
        let chunksSent = 0;
        let done: Extract<AudioMeasurementWorkerResponse, { type: "done" }> | null = null;
        let sawError: Error | null = null;
        let pending = Promise.resolve();

        const finish = async () => {
          progress({ phase: "finish", bytesSent, chunksSent });
          await pending;
          if (sawError) throw sawError;
          if (!done) throw new Error("Worker kết thúc mà không gửi dữ liệu đo");
          const completed = await completeMeasurementApi(jobId, { requestId: request.id, fence });
          return {
            requestId: request.id,
            status: "measured" as const,
            lufs: completed.request.result?.lufs ?? null,
            samples: completed.request.result?.samples ?? done.samples,
            durationSec: completed.request.result?.durationSec ?? done.durationSec,
            score: completed.score,
          };
        };

        worker.onmessage = (event: MessageEvent<AudioMeasurementWorkerResponse>) => {
          const message = event.data;
          if (message.type === "pcm") {
            const body = new Uint8Array(message.bytes);
            const upload = pending.then(async () => {
              for (let offset = 0; offset < body.byteLength; offset += CHUNK_BYTES) {
                const slice = body.subarray(offset, Math.min(offset + CHUNK_BYTES, body.byteLength));
                if (signal?.aborted) throw new DOMException("Đã huỷ đo", "AbortError");
                await uploadMeasurementChunk(jobId, {
                  requestId: request.id,
                  nonce,
                  fence,
                  seq: uploadSeq++,
                  sampleRate: message.sampleRate,
                  channels: message.channels,
                  bytes: slice,
                });
                bytesSent += slice.byteLength;
                chunksSent += 1;
                progress({ phase: "upload", bytesSent, chunksSent });
              }
              worker.postMessage({ type: "ack", requestId: "measure", seq: message.seq } satisfies AudioMeasurementWorkerRequest);
            });
            pending = upload.catch((error: unknown) => {
              sawError = error instanceof Error ? error : new Error(String(error));
              reject(sawError);
            });
            return;
          }
          if (message.type === "done") {
            done = message;
            void finish().then(resolve, reject);
            return;
          }
          if (message.type === "cancelled") {
            reject(new DOMException("Đã huỷ đo", "AbortError"));
            return;
          }
          if (message.type === "error") {
            reject(new Error(message.error));
          }
        };
        worker.onerror = (event) => reject(new Error(event.message || "Worker đo lỗi"));

        if (signal?.aborted) {
          reject(new DOMException("Đã huỷ đo", "AbortError"));
          return;
        }
        signal?.addEventListener("abort", onAbort, { once: true });

        const start: AudioMeasurementWorkerRequest = {
          type: "start",
          requestId: "measure",
          composition,
          media: mediaRecords,
          compositions,
        };
        worker.postMessage(start);
      });
    } finally {
      signal?.removeEventListener("abort", onAbort);
      worker.terminate();
    }
  } catch (error) {
    await failMeasurementApi(jobId, request.id, fence, {
      code: "internal_error",
      message: error instanceof Error ? error.message : String(error),
    }).catch(() => undefined);
    throw error;
  }
}
