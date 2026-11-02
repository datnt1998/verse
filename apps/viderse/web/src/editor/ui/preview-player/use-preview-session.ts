"use client";

import { useCallback, useEffect, useState } from "react";
import { useProjectStore } from "@editor/features/projects/stores/project-store";
import { useMediaLibraryStore } from "@editor/features/media-library/stores/media-library-store";
import { loadTimeline } from "@editor/features/timeline/stores/timeline-persistence";
import { usePlaybackStore } from "@editor/shared/state/playback";
import { StorageClientError } from "@editor/storage/errors";
import { createLogger } from "@editor/shared/logging/logger";

const logger = createLogger("PreviewPlayerSession");

export type PreviewSessionState = "loading" | "ready" | "not-found" | "error";

/**
 * Opens a project into the editor's stores for viewing only: the same load path as the editor session
 * (project, timeline, media library) with no autosave, no command history use and no save. The
 * timeline load is read-only, so migrations/normalization are applied in memory but never written
 * back — opening a preview must not bump the project revision. Changing `projectId` stops playback,
 * rewinds and loads the new project; unmounting pauses.
 */
export function usePreviewSession(projectId: string): { state: PreviewSessionState; retry: () => void } {
  const [attempt, setAttempt] = useState(0);
  // The result is keyed by what it loaded, so a new project or retry reads as loading until it settles.
  const key = `${projectId}:${attempt}`;
  const [settled, setSettled] = useState<{ key: string; state: Exclude<PreviewSessionState, "loading"> } | null>(null);

  useEffect(() => {
    let cancelled = false;
    const setState = (state: Exclude<PreviewSessionState, "loading">) => { if (!cancelled) setSettled({ key, state }); };
    const playback = usePlaybackStore.getState();
    playback.pause();
    playback.setCurrentFrame(0);
    const open = async () => {
      try {
        const project = await useProjectStore.getState().loadProject(projectId);
        if (!project) { setState("not-found"); return; }
        if (cancelled) return;
        await loadTimeline(projectId, { useRetainedProject: true, readOnly: true });
        if (cancelled) return;
        useMediaLibraryStore.getState().setCurrentProject(projectId);
        await useMediaLibraryStore.getState().loadMediaItems();
        setState("ready");
      } catch (error) {
        logger.error("Không mở được bản xem thử", error);
        setState(error instanceof StorageClientError && error.status === 404 ? "not-found" : "error");
      }
    };
    void open();
    return () => {
      cancelled = true;
      usePlaybackStore.getState().pause();
    };
  }, [projectId, key]);

  const retry = useCallback(() => setAttempt((value) => value + 1), []);
  return { state: settled?.key === key ? settled.state : "loading", retry };
}
