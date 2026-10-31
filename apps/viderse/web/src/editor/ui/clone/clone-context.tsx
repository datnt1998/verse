"use client";

/**
 * Clone awareness inside the editor header: for clone-linked projects it
 * shows the video state, a link back to the clone page and the
 * source-vs-project comparison.
 */
import { useEffect, useRef, useState, useSyncExternalStore } from "react";
import Link from "../../bridge/navigation";
import { Clapperboard, MonitorPlay } from "lucide-react";
import { EditorIconButton } from "@/editor/ui/controls";
import { cloneArtifactUrl, cloneJobResponseSchema, type CloneJobResponse } from "@/lib/clone-view";
import { appRoutes } from "@/lib/app-routes";
import { withReturnTo } from "@/lib/navigation-return";
import { useCloneJobId } from "@/editor/ui/shell/editor-back";
import { SourceComparison } from "@/components/clone/source-comparison";
import { usePlaybackStore } from "@editor/shared/state/playback";
import { useTimelineStore } from "@editor/features/timeline/stores/timeline-store-facade";
import { useSelectionStore } from "@editor/shared/state/selection";
import { useEditorStore } from "@editor/shared/state/editor";

const JOB_STATUS: Record<string, string> = {
  pending: "đang chờ", running: "đang chạy", waiting: "cần xử lý",
  succeeded: "đã xong", failed: "có lỗi", cancelled: "đã huỷ",
};

/** Clone video banner for clone-built projects only. */
export function CloneEditorSlot({ projectId }: { projectId: string }) {
  const jobId = useCloneJobId(projectId);
  const [job, setJob] = useState<CloneJobResponse | null>(null);

  useEffect(() => {
    if (!jobId) return;
    const controller = new AbortController();
    const load = async () => {
      try {
        const detail = await fetch(`/api/clone/${encodeURIComponent(jobId)}`, { signal: controller.signal });
        if (!detail.ok) return;
        const body = cloneJobResponseSchema.safeParse(await detail.json());
        if (body.success) setJob(body.data);
      } catch {
        // Abort/offline: slot stays hidden rather than claiming stale state.
      }
    };
    void load();
    return () => controller.abort();
  }, [jobId]);

  if (!jobId || !job) return null;
  return (
    <>
      {/* One quiet link, not a coloured chip: the status is information, the only action is opening the clone page. */}
      <Link
        href={withReturnTo(appRoutes.cloneJob(job.job.id), appRoutes.editor(projectId))}
        data-tooltip={`Clone ${JOB_STATUS[job.job.status] ?? job.job.status} · mở trang clone`}
        aria-label={`Xem video clone (${JOB_STATUS[job.job.status] ?? job.job.status})`}
        className="hit-area inline-flex h-7 items-center gap-1.5 rounded-md px-2 text-xs text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        <Clapperboard aria-hidden className="size-3.5" />
        Video clone
      </Link>
      <CloneSourceCompare job={job} />
      <CloneFixBridge projectId={projectId} jobId={job.job.id} />
    </>
  );
}

/**
 * Source-vs-project comparison: the muted source player mirrors the editor
 * playhead through the shared playback store — seek/play/pause stay linked
 * and only the project preview is ever audible.
 */
function CloneSourceCompare({ job }: { job: CloneJobResponse }) {
  const [open, setOpen] = useState(false);
  const source = job.artifacts.find((artifact) => artifact.path.startsWith("download/") && /\.(mp4|webm|mkv|mov)$/.test(artifact.path));
  const fps = useTimelineStore((state) => state.fps);
  const currentFrame = usePlaybackStore((state) => state.currentFrame);
  const isPlaying = usePlaybackStore((state) => state.isPlaying);
  if (!source) return null;
  const rate = Number.isFinite(fps) && fps > 0 ? fps : 30;
  return (
    <span className="relative">
      <EditorIconButton icon={MonitorPlay} label="So sánh với video gốc" aria-expanded={open} onClick={() => setOpen((v) => !v)} />
      {open ? (
        <div className="absolute right-0 top-full z-50 mt-1 w-[min(26rem,calc(100vw-2rem))] rounded-md border border-border bg-popover p-3 shadow-lg" role="dialog" aria-label="So sánh video gốc">
          <SourceComparison
            sourceUrl={cloneArtifactUrl(job.job.id, source.path)}
            time={currentFrame / rate}
            playing={isPlaying}
            label="Video gốc · đồng bộ theo playhead · tiếng tắt"
            onSeek={(seconds) => usePlaybackStore.getState().setCurrentFrame(Math.max(0, Math.round(seconds * rate)))}
            onPlayingChange={(next) => { if (next) usePlaybackStore.getState().play(); else usePlaybackStore.getState().pause(); }}
          />
          <p className="mt-2 text-xs text-muted-foreground">Kéo playhead trong editor hoặc tua video gốc — hai bên đi cùng nhau.</p>
        </div>
      ) : null}
    </span>
  );
}

/** The deep link is read once per page load; it never changes without navigation. */
const subscribeNever = () => () => {};

/** Consume the meter's deep link once the saved timeline objects are loaded. */
function CloneFixBridge({ projectId, jobId }: { projectId: string; jobId: string }) {
  const items = useTimelineStore((state) => state.items);
  const search = useSyncExternalStore(subscribeNever, () => window.location.search, () => "");
  const handled = useRef(false);
  const params = new URLSearchParams(search);
  const itemId = params.get("cloneItem");
  const mediaId = params.get("cloneMedia");
  const frameText = params.get("cloneFrame");
  const item = items.find((entry) => entry.id === itemId || ("mediaId" in entry && entry.mediaId === mediaId));
  // Loaded timeline without the linked object: the fix target was deleted or rebuilt away.
  const missing = (itemId || mediaId) && items.length > 0 && !item ? (itemId ?? mediaId) : null;
  useEffect(() => {
    if (handled.current || ((itemId || mediaId) && !item)) return;
    if (item) {
      const selection = useSelectionStore.getState();
      selection.setActiveTrack(item.trackId);
      selection.selectItems([item.id]);
      const panels = useEditorStore.getState();
      panels.setRightSidebarOpen(true);
      panels.setActivePanel("properties");
    }
    const frame = frameText !== null && /^\d+$/.test(frameText) ? Number(frameText) : null;
    if (frame !== null && Number.isSafeInteger(frame)) usePlaybackStore.getState().setCurrentFrame(frame);
    handled.current = true;
  }, [projectId, item, itemId, mediaId, frameText]);
  return missing ? <p role="alert" className="text-xs text-amber-300">Không còn đối tượng {missing}. <Link className="underline" href={`/clone/${encodeURIComponent(jobId)}?group=build`}>Mở lại job clone để khôi phục bản dựng.</Link></p> : null;
}
