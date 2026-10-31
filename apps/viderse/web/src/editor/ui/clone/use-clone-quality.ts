"use client";

/**
 * Shared clone-quality state for the editor header: which job owns this
 * project, the latest score + gate decision, any live measurement request,
 * and actions to measure and to jump at a failing item's fix target.
 */
import { useCallback, useEffect, useRef, useState } from "react";
import { useParams, useRouter } from "../../bridge/navigation";

import type { CloneFixTarget, CloneScore } from "@/lib/clone-quality";
import { cloneEditorExportGate } from "./quality-gate";
import {
  acceptCloneQuality,
  fetchMeasurementRuntime,
  fetchMeasurementState,
  type CloneProjectInfo,
  type PublicMeasurementRequest,
} from "./measurement-runtime";
import { runAudioMeasurement, type MeasureProgress } from "./audio-measurement";

import { useSelectionStore } from "@editor/shared/state/selection";
import { usePlaybackStore } from "@editor/shared/state/playback";
import { useTimelineSettingsStore } from "@editor/features/timeline/stores/timeline-settings-store";
import { useStorageConflictStore } from "@editor/storage/conflict-store";
export interface CloneQualityState {
  /** Non-null when this project belongs to a clone job. */
  clone: CloneProjectInfo | null;
  score: CloneScore | null;
  /** Latest request (running/queued/measured…) with live progress. */
  request: PublicMeasurementRequest | null;
  measuring: boolean;
  measureError: string | null;
  progress: MeasureProgress | null;
  loading: boolean;
}

export interface CloneQualityActions {
  /** Save-audit-measure: POST create → browser mix → server ebur128 + score. */
  measure: () => Promise<void>;
  refresh: () => Promise<void>;
  /** Reveal a failing item in the editor (timeline item, frame, or route). */
  openFix: (target: CloneFixTarget) => void;
  /** The owner's call: export despite the checklist (`false` puts the checklist back in charge). */
  acceptQuality: (accept: boolean) => Promise<void>;
  canExport: boolean;
  gateReason: string | null;
  gateBlockers: { id: string; message: string }[];
  /** The owner chose "Vẫn xuất" for this video. */
  accepted: boolean;
  /** Saved, but the checklist alone keeps export closed — the owner may export anyway. */
  checklistBlocked: boolean;
}

export type CloneQuality = CloneQualityState & CloneQualityActions;

/** ms between gate polls while a measurement request is live. */
const POLL_MS = 2_000;

export function useCloneQuality(session: { save: () => Promise<boolean>; dirty: boolean; saving: boolean; conflict: unknown }): CloneQualityState & CloneQualityActions {
  const params = useParams<{ projectId: string }>();
  const projectId = params?.projectId ?? "";
  const dirty = useTimelineSettingsStore((s) => s.isDirty);
  const conflict = useStorageConflictStore((s) => s.conflicts[projectId]);
  const [state, setState] = useState<CloneQualityState>({
    clone: null,
    score: null,
    request: null,
    measuring: false,
    measureError: null,
    progress: null,
    loading: true,
  });
  const [gate, setGate] = useState<{ allowed: boolean; accepted: boolean; reason: string | null; blockers: { id: string; message: string }[] }>({
    allowed: false,
    accepted: false,
    reason: null,
    blockers: [],
  });
  const ownerRef = useRef(`editor-${crypto.randomUUID()}`);
  const router = useRouter();

  const refresh = useCallback(async () => {
    if (!projectId) return;
    try {
      const { clone } = await fetchMeasurementRuntime(projectId);
      const detail = clone ? await fetchMeasurementState(clone.jobId) : null;
      const next = { clone, score: detail ? (detail.score ?? clone?.score ?? null) : null, request: detail?.relevant ?? null };
      // Unchanged answers keep the same state object: a new one re-renders the whole editor.
      setState((s) => (!s.loading && sameJson(next, { clone: s.clone, score: s.score, request: s.request }) ? s : { ...s, ...next, loading: false }));
      if (detail) {
        const nextGate = { allowed: detail.gate.allowed, accepted: detail.gate.accepted, reason: detail.gate.reason, blockers: detail.gate.blockers };
        setGate((g) => (sameJson(g, nextGate) ? g : nextGate));
      }
    } catch {
      setState((s) => (s.loading ? { ...s, loading: false } : s));
    }
  }, [projectId]);

  // Fresh on open and after every save (the server's gate tracks the saved revision).
  useEffect(() => {
    if (!session.saving) void refresh();
  }, [refresh, session.saving]);

  // Poll only while a measurement request is live so the meter tracks progress.
  const live = state.measuring || state.request?.status === "queued" || state.request?.status === "running";
  useEffect(() => {
    if (!live) return;
    const interval = setInterval(() => { void refresh(); }, POLL_MS);
    return () => clearInterval(interval);
  }, [live, refresh]);

  const measure = useCallback(async () => {
    const clone = state.clone;
    if (!clone || state.measuring) return;
    setState((s) => ({ ...s, measuring: true, measureError: null, progress: { phase: "claim", bytesSent: 0, chunksSent: 0 } }));
    try {
      if (session.conflict || !(await session.save())) {
        throw new Error("Chưa lưu được project hiện tại — giải quyết xung đột rồi đo lại");
      }
      await runAudioMeasurement({
        projectId,
        jobId: clone.jobId,
        ownerId: ownerRef.current,
        onProgress: (progress) => setState((s) => ({ ...s, progress })),
      });
      await refresh();
    } catch (error) {
      setState((s) => ({
        ...s,
        measureError: error instanceof Error ? error.message : String(error),
      }));
    } finally {
      setState((s) => ({ ...s, measuring: false, progress: null }));
    }
  }, [projectId, state.clone, state.measuring, refresh, session]);

  const openFix = useCallback(
    (target: CloneFixTarget) => {
      if (target.itemId) {
        // Reveal the item on the main timeline via the shared selection store.
        useSelectionStore.getState().selectItems([target.itemId]);
      }
      if (target.frame !== undefined) {
        usePlaybackStore.getState().setCurrentFrame(target.frame);
      }
      if (target.href) router.push(target.href);
    },
    [router],
  );

  const acceptQuality = useCallback(
    async (accept: boolean) => {
      if (!state.clone) return;
      await acceptCloneQuality(state.clone.jobId, accept);
      await refresh();
    },
    [state.clone, refresh],
  );

  return {
    ...state,
    measure,
    refresh,
    openFix,
    acceptQuality,
    ...cloneEditorExportGate({ score: state.score, gate, dirty, conflict, saving: session.saving }),
    gateBlockers: gate.blockers,
    accepted: gate.accepted,
  };
}

function sameJson(a: unknown, b: unknown): boolean {
  return JSON.stringify(a) === JSON.stringify(b);
}
