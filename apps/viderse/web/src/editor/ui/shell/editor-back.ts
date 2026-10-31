"use client";

/**
 * Where the editor's back control leads, decided without `router.back()` so direct links behave
 * the same as in-app ones: an explicit valid `?returnTo=` first, then the owning template page,
 * then the clone job that built the project, then the project list. Kept light (no editor stores)
 * because the blocked-screen fallbacks outside the lazy editor chunk use it too.
 */
import { useEffect } from "react";
import { useSearchParams } from "../../bridge/navigation";
import { create } from "zustand";
import { appRoutes } from "@/lib/app-routes";
import { cloneJobSummarySchema } from "@/lib/clone-view";
import { returnLabel, safeReturnTo } from "@/lib/navigation-return";
import { useTemplateStatus } from "@/editor/ui/template/template-status";

export interface EditorBack {
  href: string;
  /** Accessible name and tooltip of the back control. */
  label: string;
}

const useCloneOwnerStore = create<{ owners: Record<string, string | null> }>(() => ({ owners: {} }));
const inFlight: Record<string, Promise<void> | undefined> = {};

/** Look up the clone job that owns a project; callers racing on one project share one request. */
export function loadCloneOwner(projectId: string): Promise<void> {
  const running = inFlight[projectId];
  if (running) return running;
  const request = (async () => {
    let jobId: string | null = null;
    try {
      const response = await fetch(`/api/clone?projectId=${encodeURIComponent(projectId)}`);
      if (response.ok) {
        const parsed = cloneJobSummarySchema.safeParse(await response.json());
        if (parsed.success) jobId = parsed.data.id;
      }
    } catch {
      // Offline: no owning job is claimed.
    } finally {
      inFlight[projectId] = undefined;
    }
    useCloneOwnerStore.setState((state) => ({ owners: { ...state.owners, [projectId]: jobId } }));
  })();
  inFlight[projectId] = request;
  return request;
}

/** The clone job id behind a clone-built project, or null while unknown / when there is none. */
export function useCloneJobId(projectId: string): string | null {
  const jobId = useCloneOwnerStore((state) => state.owners[projectId] ?? null);
  useEffect(() => {
    void loadCloneOwner(projectId);
  }, [projectId]);
  return jobId;
}

export function useEditorBack(projectId: string): EditorBack {
  const explicit = safeReturnTo(useSearchParams().get("returnTo"));
  const { status } = useTemplateStatus(projectId);
  const jobId = useCloneJobId(projectId);
  if (explicit) return { href: explicit, label: `Quay lại ${returnLabel(explicit)}` };
  if (status?.template) return { href: appRoutes.template(projectId), label: "Về trang template" };
  if (jobId) return { href: appRoutes.cloneJob(jobId), label: "Về video clone" };
  return { href: "/editor", label: "Về danh sách video" };
}
