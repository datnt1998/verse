"use client";

/**
 * "Is this project some owner channel's template?" — one fetch shared by the
 * top-bar popover, the banner, the track role menu and the inspector anchor
 * section. A null answer means template mode stays off.
 */
import { useEffect } from "react";
import { z } from "zod";
import { create } from "zustand";

export interface TemplateChannel {
  id: string;
  name: string;
}

export interface TemplateStatus {
  /** The project this status belongs to; mismatches render as "not loaded". */
  projectId: string;
  channel: TemplateChannel | null;
  /** The project is a registered template: its settings page exists. */
  template: boolean;
}

const templateStatusResponse = z.object({
  projectId: z.string().min(1),
  channel: z.object({ id: z.string().min(1), name: z.string().min(1) }).nullable(),
  template: z.boolean(),
});

export const useTemplateStatusStore = create<{ status: TemplateStatus | null }>(() => ({ status: null }));

const inFlight: Record<string, Promise<void> | undefined> = {};

/** Fetch (or re-fetch) the status; callers racing on the same project share one request. */
export function refreshTemplateStatus(projectId: string): Promise<void> {
  const running = inFlight[projectId];
  if (running) return running;
  const request = (async () => {
    try {
      const response = await fetch(`/api/my-channels/template-of/${encodeURIComponent(projectId)}`);
      if (!response.ok) return;
      const parsed = templateStatusResponse.safeParse(await response.json());
      if (!parsed.success) return;
      useTemplateStatusStore.setState({ status: parsed.data });
    } catch {
      // Offline or aborted: template mode stays on the last known state.
    } finally {
      inFlight[projectId] = undefined;
    }
  })();
  inFlight[projectId] = request;
  return request;
}

/** Subscribe to the current project's template status, loading it once per project. */
export function useTemplateStatus(projectId: string): { status: TemplateStatus | null; ready: boolean } {
  const status = useTemplateStatusStore((state) => (state.status?.projectId === projectId ? state.status : null));
  const ready = status !== null;
  useEffect(() => {
    if (projectId && useTemplateStatusStore.getState().status?.projectId !== projectId) void refreshTemplateStatus(projectId);
  }, [projectId]);
  return { status, ready };
}
