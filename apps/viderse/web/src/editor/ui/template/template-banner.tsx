"use client";

/**
 * Banner under the editor header while this project is a registered template:
 * names the channel using it (if any), runs the shared template validator
 * live, and links back to the template page, whose preview rebuilds from these
 * edits. Every issue that names a track or item is a button — clicking selects
 * the offender on the timeline so the owner lands where the fix goes.
 */
import { useMemo } from "react";
import Link from "../../bridge/navigation";
import { CircleAlert, CircleCheck, Info, LayoutTemplate, MonitorPlay, Tv } from "lucide-react";
import { validateChannelTemplate, type TemplateIssue } from "@editor/features/editing-templates/channel-template";
import { useTimelineStore } from "@editor/features/timeline/stores/timeline-store";
import { useItemsStore } from "@editor/features/timeline/stores/items-store";
import { useMediaLibraryStore } from "@editor/features/media-library/stores/media-library-store";
import { useProjectStore } from "@editor/features/projects/stores/project-store";
import { useSelectionStore } from "@editor/shared/state/selection";
import { announce } from "@/editor/ui/shell/announcer";
import { useTemplateStatus } from "./template-status";
import { appRoutes } from "@/lib/app-routes";

function selectIssue(issue: TemplateIssue) {
  const selection = useSelectionStore.getState();
  if (issue.itemId) {
    const item = useTimelineStore.getState().items.find((entry) => entry.id === issue.itemId);
    if (item) {
      selection.setActiveTrack(item.trackId);
      selection.selectItems([item.id]);
      announce(`Đã chọn clip ${item.label}`);
      return;
    }
  }
  if (issue.trackId) {
    selection.clearItemSelection();
    selection.setActiveTrack(issue.trackId);
    const track = useTimelineStore.getState().tracks.find((entry) => entry.id === issue.trackId);
    announce(track ? `Đã chọn track ${track.name}` : "Đã chọn track");
  }
}

export function TemplateBanner({ projectId }: { projectId: string }) {
  const { status, ready } = useTemplateStatus(projectId);
  const metadata = useProjectStore((state) => state.currentProject?.metadata);
  const tracks = useTimelineStore((state) => state.tracks);
  const items = useItemsStore((state) => state.items);
  const mediaItems = useMediaLibraryStore((state) => state.mediaItems);
  const mediaLoading = useMediaLibraryStore((state) => state.isLoading);
  const issues = useMemo<TemplateIssue[]>(() => {
    // An unloaded library would flag every clip as "media missing".
    if (!status?.template || !metadata || mediaLoading) return [];
    const found = validateChannelTemplate(
      { metadata: { width: metadata.width, height: metadata.height, fps: metadata.fps }, tracks, items },
      new Set(mediaItems.map((media) => media.id)),
    );
    // Several clips on a style slot: only the earliest one is the prototype.
    for (const [slot, type] of [["scenes", "image"], ["captions", "subtitle"]] as const) {
      const track = tracks.find((entry) => entry.templateSlot === slot);
      if (track && items.filter((item) => item.trackId === track.id && item.type === type).length > 1) {
        found.push({ severity: "info", message: "Chỉ dùng clip đầu làm mẫu", trackId: track.id });
      }
    }
    return found;
  }, [status, metadata, tracks, items, mediaItems, mediaLoading]);

  if (!ready || !status?.template) return null;
  const channel = status.channel;
  const title = channel ? `Template của kênh ${channel.name}` : "Đang sửa template";
  const errors = issues.filter((issue) => issue.severity === "error");
  return (
    <section aria-label={title} className="shrink-0 border-b border-border bg-card px-3 py-1.5">
      <div className="flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1">
        <p className="flex min-w-0 items-center gap-1.5 text-xs font-medium">
          {channel ? <Tv aria-hidden className="size-3.5 shrink-0 text-primary" /> : <LayoutTemplate aria-hidden className="size-3.5 shrink-0 text-primary" />}
          <span className="truncate">{title}</span>
        </p>
        {issues.length === 0 ? (
          <p className="flex items-center gap-1 text-[11px] text-muted-foreground">
            <CircleCheck aria-hidden className="size-3" /> Hợp lệ — sẵn sàng cho job clone
          </p>
        ) : (
          <ul className="flex min-w-0 flex-wrap items-center gap-x-3 gap-y-0.5" aria-label="Vấn đề của template">
            {issues.map((issue, index) => {
              const Icon = issue.severity === "error" ? CircleAlert : Info;
              const clickable = Boolean(issue.trackId || issue.itemId);
              const content = (
                <>
                  <Icon aria-hidden className="size-3 shrink-0" />
                  <span className={clickable ? "underline decoration-dotted underline-offset-2" : undefined}>{issue.message}</span>
                </>
              );
              return (
                <li key={index} className={`flex min-w-0 items-center gap-1 text-[11px] ${issue.severity === "error" ? "text-destructive" : "text-muted-foreground"}`}>
                  {clickable ? (
                    <button
                      type="button"
                      className="flex min-w-0 items-center gap-1 rounded focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                      onClick={() => selectIssue(issue)}
                    >
                      {content}
                    </button>
                  ) : (
                    <span className="flex min-w-0 items-center gap-1">{content}</span>
                  )}
                </li>
              );
            })}
          </ul>
        )}
        {errors.length > 0 && (
          <span className="text-[10px] text-muted-foreground">Clone sẽ không chạy được cho tới khi sửa hết lỗi.</span>
        )}
        <Link
          href={appRoutes.template(projectId)}
          data-hit-area
          data-tooltip="Mọi chỉnh sửa ở đây tự lưu; bản xem thử cập nhật khi bạn quay lại trang template"
          className="ml-auto flex min-h-7 items-center gap-1 rounded px-1 text-[11px] font-medium text-primary hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring [@media(pointer:coarse)]:min-h-11"
        >
          <MonitorPlay aria-hidden className="size-3" /> Xem thử template
        </Link>
      </div>
    </section>
  );
}
