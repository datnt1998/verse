"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { getCommandExecutor } from "@editor/commands";
import { useProjectStore } from "@editor/features/projects/stores/project-store";
import { useMediaLibraryStore } from "@editor/features/media-library/stores/media-library-store";
import { loadTimeline, saveTimeline } from "@editor/features/timeline/stores/timeline-persistence";
import { useTimelineSettingsStore } from "@editor/features/timeline/stores/timeline-settings-store";
import { getStorageConflict, useStorageConflictStore } from "@editor/storage/conflict-store";
import { StorageClientError, StorageConflictError } from "@editor/storage/errors";
import { createLogger } from "@editor/shared/logging/logger";
import { announce } from "./announcer";

const logger = createLogger("EditorSession");

export function useProjectSession(projectId: string, autosaveEnabled = true) {
  const [opening, setOpening] = useState<"loading" | "ready" | "not-found" | "error">("loading");
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState(false);
  const [reload, setReload] = useState(0);
  const dirty = useTimelineSettingsStore((state) => state.isDirty);
  const conflict = useStorageConflictStore((state) => state.conflicts[projectId]);
  const generation = useRef(0);
  const mounted = useRef(false);
  const inFlight = useRef<Promise<boolean> | null>(null);
  const opened = useRef(false);
  const save = useCallback((): Promise<boolean> => {
    if (inFlight.current) return inFlight.current;
    if (!opened.current || getStorageConflict(projectId)) return Promise.resolve(false);
    const startedGeneration = generation.current;
    setSaving(true); setSaveError(false);
    const operation = (async () => {
      try {
        await saveTimeline(projectId);
        // A successful write only covers the snapshot captured when it started.
        const complete = generation.current === startedGeneration;
        if (!complete) useTimelineSettingsStore.getState().markDirty();
        if (mounted.current && complete) announce("Đã lưu");
        return complete;
      } catch (error) {
        if (mounted.current) {
          setSaveError(!(error instanceof StorageConflictError));
          announce(error instanceof StorageConflictError ? "Có phiên bản mới hơn từ nơi khác" : "Lưu thất bại, thử lại", "assertive");
        }
        return false;
      } finally {
        inFlight.current = null;
        if (mounted.current) setSaving(false);
      }
    })();
    inFlight.current = operation;
    return operation;
  }, [projectId]);
  useEffect(() => {
    let cancelled = false;
    mounted.current = true;
    opened.current = false;
    const open = async () => {
      try {
        // Complete any previous snapshot write before discarding its revision.
        if (inFlight.current) await inFlight.current;
        const project = await useProjectStore.getState().loadProject(projectId);
        if (!project) { if (!cancelled) setOpening("not-found"); return; }
        if (cancelled) return;
        await loadTimeline(projectId, { useRetainedProject: true });
        if (cancelled) return;
        useMediaLibraryStore.getState().setCurrentProject(projectId);
        await useMediaLibraryStore.getState().loadMediaItems();
        if (cancelled) return;
        getCommandExecutor().clearHistory();
        opened.current = true; setOpening("ready"); setSaveError(false);
      } catch (error) {
        logger.error("Không mở được project", error);
        if (!cancelled) setOpening(error instanceof StorageClientError && error.status === 404 ? "not-found" : "error");
      }
    };
    void open();
    return () => { cancelled = true; mounted.current = false; opened.current = false; };
  }, [projectId, reload]);
  useEffect(() => getCommandExecutor().on("commit", () => { generation.current++; }), []);
  useEffect(() => {
    if (opening !== "ready" || !autosaveEnabled) return;
    const interval = window.setInterval(() => {
      if (useTimelineSettingsStore.getState().isDirty && !getStorageConflict(projectId) && !saveError) void save();
    }, 3000);
    return () => window.clearInterval(interval);
  }, [opening, projectId, save, saveError, autosaveEnabled]);
  const reloadProject = () => { setOpening("loading"); setReload((value) => value + 1); };
  return { opening, saving, saveError, dirty, conflict, save, reloadProject };
}
