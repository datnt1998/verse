"use client";

/**
 * Same-origin client for the clone measurement endpoints and the runtime
 * readiness check. Kept free of engine imports so unit tests run under
 * plain node:test.
 */
import { z } from "zod";
import { cloneScoreSchema, type CloneScore } from "@/lib/clone-quality";
import { cloneMeasurementEvidenceSchema } from "@/lib/clone-view";

export interface MeasurementBinding {
  projectRevision: string;
  settingsHash: string;
  artifactHash: string;
}

/** Credential issued by the launcher; nonce authorizes the fenced claim. */
export interface MeasurementClaimCredential {
  requestId: string;
  nonce: string;
}

export interface PublicMeasurementRequest {
  version: number;
  id: string;
  status: "queued" | "running" | "measured" | "failed" | "cancelled" | "stale";
  createdAt: string;
  binding: MeasurementBinding;
  ownerId: string | null;
  leaseExpiresAt: string | null;
  fence: string | null;
  chunksReceived: number;
  bytesReceived: number;
  sampleRate: number | null;
  channels: number | null;
  result: { lufs: number; samples: number; durationSec: number; measuredAt: string } | null;
  error: { code: string; message: string } | null;
  finishedAt: string | null;
}

export interface CloneRuntimeInfo {
  workspaceId: string;
  ready: boolean;
  capabilities: { audioDecode: boolean; gpuRequired: boolean };
}

export interface CloneProjectInfo {
  jobId: string;
  generation: number;
  score: CloneScore | null;
  gate: { allowed: boolean; blockers: { id: string; message: string }[] };
  latestRequest: { id: string; status: PublicMeasurementRequest["status"] } | null;
}

export const publicMeasurementRequestSchema = cloneMeasurementEvidenceSchema.unwrap().extend({
  version: z.number(),
  createdAt: z.string(),
  ownerId: z.string().nullable(),
  leaseExpiresAt: z.string().nullable(),
  fence: z.string().nullable(),
  chunksReceived: z.number(),
  bytesReceived: z.number(),
  sampleRate: z.number().nullable(),
  channels: z.number().nullable(),
  result: z.object({ lufs: z.number(), samples: z.number(), durationSec: z.number(), measuredAt: z.string() }).nullable(),
  error: z.object({ code: z.string(), message: z.string() }).nullable(),
  finishedAt: z.string().nullable(),
}).passthrough();
const issuedRequestResponseSchema = z.object({ request: publicMeasurementRequestSchema.extend({ nonce: z.string() }) });
const requestResponseSchema = z.object({ request: publicMeasurementRequestSchema });
const blockersSchema = z.array(z.object({ id: z.string(), message: z.string() }));
const runtimeResponseSchema = z.object({
  runtime: z.object({ workspaceId: z.string(), ready: z.boolean(), capabilities: z.object({ audioDecode: z.boolean(), gpuRequired: z.boolean() }) }),
  clone: z.object({
    jobId: z.string(), generation: z.number(), score: cloneScoreSchema.nullable(),
    gate: z.object({ allowed: z.boolean(), blockers: blockersSchema }),
    latestRequest: z.object({ id: z.string(), status: publicMeasurementRequestSchema.shape.status }).nullable(),
  }).nullable(),
});
const stateResponseSchema = z.object({
  requests: z.array(publicMeasurementRequestSchema),
  relevant: publicMeasurementRequestSchema.nullable(),
  score: cloneScoreSchema.nullable(),
  gate: z.object({ allowed: z.boolean(), accepted: z.boolean(), fresh: z.boolean(), reason: z.string().nullable(), blockers: blockersSchema }),
});
const completedResponseSchema = requestResponseSchema.extend({ score: cloneScoreSchema.nullable() });
const chunkResponseSchema = z.object({ chunksReceived: z.number(), bytesReceived: z.number() });
const failureResponseSchema = z.object({ error: z.unknown().optional() });

const base = "/api/clone";

export class MeasurementApiError extends Error {
  readonly status: number;
  readonly body: unknown;
  constructor(message: string, status: number, body: unknown) {
    super(message);
    this.name = "MeasurementApiError";
    this.status = status;
    this.body = body;
  }
}

export interface MeasurementFailure {
  /** Stable machine code persisted with the request for support tooling. */
  code: string;
  /** User-facing Vietnamese text; never the decoder's English message. */
  message: string;
}

/** Minimal Vietnamese-diacritic test: technical/browser errors are ASCII. */
const VIETNAMESE_TEXT = /[ăâđêôơưĂÂĐÊÔƠƯàáảãạằắẳẵặầấẩẫậèéẻẽẹềếểễệìíỉĩịòóỏõọồốổỗộờớởỡợùúủũụừứửữựỳýỷỹỵ]/;
const DECODE_FAILURE = /unable to decode|decode audio|no audio track|audio decode|mediabunny|audio-decode-fallback/i;
const SOURCE_FAILURE = /failed to fetch audio|fetch failed|networkerror|load failed/i;

function codedFailure(error: unknown): MeasurementFailure | null {
  if (!(error instanceof Error) || !("code" in error)) return null;
  const code: unknown = error.code;
  if (code !== "audio_decode_failed" && code !== "audio_source_unavailable") return null;
  return { code, message: error.message };
}

function failureCode(error: unknown): string | undefined {
  if (!error || typeof error !== "object" || !("code" in error)) return undefined;
  const code: unknown = error.code;
  return typeof code === "string" && code.length > 0 && code.length <= 64 ? code : undefined;
}

/**
 * Convert any measurement/engine error to the typed, Vietnamese failure that
 * the request journal and the editor meter show. The original error's message
 * stays available to callers as `cause` for logs; it is never rendered.
 */
export function measurementFailure(error: unknown): MeasurementFailure {
  if (error instanceof DOMException && error.name === "AbortError") {
    return { code: "cancelled", message: "Đã huỷ đo." };
  }
  const coded = codedFailure(error);
  if (coded) return coded;
  const raw = error instanceof Error ? error.message : String(error);
  if (VIETNAMESE_TEXT.test(raw)) return { code: failureCode(error) ?? "internal_error", message: raw };
  if (DECODE_FAILURE.test(raw)) {
    return {
      code: "audio_decode_failed",
      message: "Không giải mã được âm thanh của tệp nguồn. Kiểm tra tệp âm thanh rồi đo lại.",
    };
  }
  if (SOURCE_FAILURE.test(raw)) {
    return {
      code: "audio_source_unavailable",
      message: "Không tải được tệp nguồn để đo. Kiểm tra kết nối rồi thử lại.",
    };
  }
  return {
    code: failureCode(error) ?? "internal_error",
    message: "Đo độ hoàn thiện thất bại. Thử lại; nếu vẫn lỗi, kiểm tra tệp nguồn.",
  };
}

/** Thrown to callers so UI code can display a safe message and log `cause`. */
export class MeasurementFailureError extends Error {
  readonly code: string;
  constructor(failure: MeasurementFailure, options?: { cause?: unknown }) {
    super(failure.message, options);
    this.name = "MeasurementFailureError";
    this.code = failure.code;
  }
}

async function api<T>(input: string, schema: z.ZodType<T>, init?: RequestInit): Promise<T> {
  const response = await fetch(input, { cache: "no-store", ...init });
  const body: unknown = await response.json().catch(() => ({}));
  if (!response.ok) {
    const failure = failureResponseSchema.safeParse(body);
    const message = failure.success && typeof failure.data.error === "string" ? failure.data.error : `Lỗi đo (${response.status})`;
    throw new MeasurementApiError(message, response.status, body);
  }
  return schema.parse(body);
}

const json = (data: unknown): RequestInit => ({
  method: "POST",
  headers: { "content-type": "application/json" },
  body: JSON.stringify(data),
});

export async function fetchMeasurementRuntime(projectId?: string): Promise<{
  runtime: CloneRuntimeInfo;
  clone: CloneProjectInfo | null;
}> {
  return api(`${base}/measurement-runtime${projectId ? `?projectId=${encodeURIComponent(projectId)}` : ""}`, runtimeResponseSchema);
}

export async function fetchMeasurementState(jobId: string): Promise<{
  requests: PublicMeasurementRequest[];
  relevant: PublicMeasurementRequest | null;
  score: CloneScore | null;
  gate: { allowed: boolean; accepted: boolean; fresh: boolean; reason: string | null; blockers: { id: string; message: string }[] };
}> {
  return api(`${base}/${encodeURIComponent(jobId)}/measurements`, stateResponseSchema);
}

/**
 * The owner's call on a failing checklist: "Vẫn xuất" — accept the gate and
 * queue the clone export. Same action as the build screen's Vẫn xuất button
 * (POST /api/clone/:id/export {accept:true}); the pipeline then renders.
 */
export async function acceptCloneExport(jobId: string): Promise<void> {
  await api(`${base}/${encodeURIComponent(jobId)}/export`, z.object({}).passthrough(), json({ accept: true }));
}

/** Put the checklist back in charge after "Vẫn xuất" (POST /api/clone/:id/quality). */
export async function setCloneQualityAcceptance(jobId: string, accept: boolean): Promise<void> {
  await api(`${base}/${encodeURIComponent(jobId)}/quality`, z.object({}).passthrough(), json({ accept }));
}

export async function createMeasurementApi(jobId: string): Promise<{
  request: PublicMeasurementRequest & { nonce: string };
}> {
  return api(`${base}/${encodeURIComponent(jobId)}/measurements`, issuedRequestResponseSchema, json({ action: "create" }));
}

export async function claimMeasurementApi(
  jobId: string,
  options: { requestId: string; nonce: string; ownerId: string },
): Promise<{ request: PublicMeasurementRequest & { nonce: string } }> {
  return api(`${base}/${encodeURIComponent(jobId)}/measurements`, issuedRequestResponseSchema, json({ action: "claim", ...options }));
}

export async function completeMeasurementApi(
  jobId: string,
  options: { requestId: string; fence: string },
): Promise<{ request: PublicMeasurementRequest; score: CloneScore | null }> {
  return api(`${base}/${encodeURIComponent(jobId)}/measurements`, completedResponseSchema, json({ action: "complete", ...options }));
}

export async function failMeasurementApi(
  jobId: string,
  requestId: string,
  fence: string,
  error: { code: string; message: string },
): Promise<{ request: PublicMeasurementRequest }> {
  return api(`${base}/${encodeURIComponent(jobId)}/measurements`, requestResponseSchema, json({ action: "fail", requestId, fence, error }));
}

export async function cancelMeasurementApi(
  jobId: string,
  requestId: string,
  nonce: string,
): Promise<{ request: PublicMeasurementRequest }> {
  return api(`${base}/${encodeURIComponent(jobId)}/measurements`, requestResponseSchema, json({ action: "cancel", requestId, nonce }));
}

export async function uploadMeasurementChunk(
  jobId: string,
  options: {
    requestId: string;
    nonce: string;
    fence: string;
    seq: number;
    sampleRate: number;
    channels: number;
    bytes: Uint8Array;
  },
): Promise<{ chunksReceived: number; bytesReceived: number }> {
  const query = new URLSearchParams({
    requestId: options.requestId,
    nonce: options.nonce,
    fence: options.fence,
    seq: String(options.seq),
    sampleRate: String(options.sampleRate),
    channels: String(options.channels),
  });
  const response = await fetch(`${base}/${encodeURIComponent(jobId)}/measurements?${query}`, {
    method: "PUT",
    headers: { "content-type": "application/octet-stream" },
    body: new Uint8Array(options.bytes),
  });
  const body: unknown = await response.json().catch(() => ({}));
  if (!response.ok) {
    const failure = failureResponseSchema.safeParse(body);
    throw new MeasurementApiError(failure.success && typeof failure.data.error === "string" ? failure.data.error : `Tải chunk lỗi (${response.status})`, response.status, body);
  }
  return chunkResponseSchema.parse(body);
}

