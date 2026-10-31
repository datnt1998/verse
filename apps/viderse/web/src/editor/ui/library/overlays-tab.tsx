"use client";

import { useEffect, useRef, useState } from "react";
import { Layers, Link2, Plus, Trash2, Upload } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { EditorIconButton } from "@/editor/ui/controls";
import { announce } from "@/editor/ui/shell/announcer";
import { useMediaLibraryStore } from "@editor/features/media-library/stores/media-library-store";
import { mediaLibraryService } from "@editor/features/media-library/services/media-library-service";
import { serverMediaToMetadata } from "@editor/storage/media-adapter";
import { getEditorStorage } from "@editor/storage/storage-backend";
import { useProjectStore } from "@editor/features/projects/stores/project-store";
import { associateMediaWithProject } from "@editor/infrastructure/storage/workspace-fs/project-media";
import type { MediaMetadata } from "@editor/types/storage";
import { addOverlay } from "./library-actions";
import { LinkImportForm } from "./link-import-form";
import { MediaThumbnail } from "./library-panel";

const showError = (error: unknown) => toast.error(error instanceof Error ? error.message : "Không thực hiện được. Thử lại.");

/**
 * "Lớp phủ" — the shared overlay library: effects uploaded once (here, from a
 * link, or on a template page) and reused by every project and template.
 * Adding one links the stored file into this project; nothing is copied.
 */
export function OverlaysTab({ query }: { query: string }) {
  const picker = useRef<HTMLInputElement>(null);
  const [busy, setBusy] = useState<string | null>(null);
  const [linkOpen, setLinkOpen] = useState(false);
  const [library, setLibrary] = useState<MediaMetadata[] | null>(null);
  const [version, setVersion] = useState(0);
  // Link imports tag their media as overlays inside the project library; refresh when it changes.
  const overlayCount = useMediaLibraryStore((state) => state.mediaItems.filter((media) => media.overlay === true).length);
  useEffect(() => {
    let live = true;
    getEditorStorage()
      .listOverlayLibrary()
      .then((records) => { if (live) setLibrary(records.map(serverMediaToMetadata)); })
      .catch(showError);
    return () => { live = false; };
  }, [version, overlayCount]);
  const needle = query.trim().toLocaleLowerCase("vi");
  const overlays = (library ?? []).filter((media) => media.fileName.toLocaleLowerCase("vi").includes(needle));

  const importOverlays = async (files: File[]) => {
    for (const file of files) {
      setBusy(file.name);
      try {
        const imported = await mediaLibraryService.importFile(file, { projectId: useProjectStore.getState().currentProject?.id });
        const { isDuplicate, hasUnsupportedCodec, ...media } = imported;
        if (!isDuplicate) useMediaLibraryStore.getState().prependMediaItem(media);
        if (hasUnsupportedCodec) toast.warning('Media có codec trình duyệt không đọc được. Hãy thay file.');
        // Detect the background and tag the media as a reusable overlay layer.
        const record = await getEditorStorage().detectMediaBackground(media.id, { overlay: true });
        const meta = serverMediaToMetadata(record);
        useMediaLibraryStore.setState((state) => ({
          mediaItems: state.mediaItems.map((item) => (item.id === meta.id ? { ...item, ...meta } : item)),
        }));
        const bg = meta.background?.kind;
        announce(`Đã lưu lớp phủ ${media.fileName} vào kho${bg === "green" ? " (nền xanh)" : bg === "black" ? " (nền đen)" : " (không có nền)"}`);
        setVersion((current) => current + 1);
      } catch (error) {
        showError(error);
      } finally {
        setBusy(null);
      }
    }
  };

  /** The project's own copy of the effect (same bytes), linking the library file first when it has none. */
  const projectCopy = async (media: MediaMetadata): Promise<MediaMetadata> => {
    const own = useMediaLibraryStore.getState().mediaItems.find((item) => item.id === media.id || (media.contentHash && item.contentHash === media.contentHash));
    if (own) return own;
    const projectId = useProjectStore.getState().currentProject?.id;
    if (!projectId) throw new Error("Chưa mở project.");
    await associateMediaWithProject(projectId, media.id);
    useMediaLibraryStore.getState().prependMediaItem(media);
    return media;
  };
  const add = (media: MediaMetadata) => void projectCopy(media).then(addOverlay).catch(showError);
  const remove = (media: MediaMetadata) =>
    void getEditorStorage()
      .removeFromOverlayLibrary(media.id)
      .then((records) => {
        setLibrary(records.map(serverMediaToMetadata));
        announce(`Đã bỏ ${media.fileName} khỏi kho lớp phủ`);
      })
      .catch(showError);

  return (
    <div className="space-y-3">
      <input ref={picker} type="file" accept="video/*,image/*" multiple className="hidden" aria-label="Chọn lớp phủ" onChange={(event) => { void importOverlays(Array.from(event.target.files ?? [])); event.target.value = ""; }} />
      <p className="text-[10px] leading-relaxed text-muted-foreground">
        Kho lớp phủ dùng chung cho mọi video và template: tải một lần, dùng lại ở đâu cũng được. Nền đen hoặc nền xanh được tự dò và gỡ khi thêm lên timeline.
      </p>
      <Button variant="outline" size="sm" className="w-full" disabled={busy !== null} onClick={() => picker.current?.click()}>
        <Upload aria-hidden />{busy ? `Đang nhập ${busy}` : "Tải lớp phủ"}
      </Button>
      <button type="button" className="flex w-full items-center justify-center gap-1 hit-area min-h-7 rounded-md border border-ring/50 bg-card px-2 text-xs" aria-pressed={linkOpen} onClick={() => setLinkOpen((open) => !open)}>
        <Link2 aria-hidden className="size-3.5" />Lấy từ link
      </button>
      {linkOpen && <LinkImportForm overlay />}
      {overlays.length ? (
        <div className="grid grid-cols-2 gap-2">
          {overlays.map((media) => (
            <div key={media.id} className="min-w-0 overflow-hidden rounded-md border border-border bg-background">
              <MediaThumbnail media={media} />
              <div className="flex items-center gap-1 border-t border-border/60 px-1 py-0.5">
                <span className="min-w-0 flex-1 truncate px-1 text-[10px]" title={media.fileName}>{media.fileName}</span>
                <EditorIconButton icon={Plus} label={`Thêm ${media.fileName} vào timeline tại playhead`} onClick={() => add(media)} />
                <EditorIconButton icon={Trash2} label={`Bỏ ${media.fileName} khỏi kho lớp phủ`} onClick={() => remove(media)} />
              </div>
            </div>
          ))}
        </div>
      ) : (
        <div className="flex flex-col items-center gap-2 py-8 text-center text-muted-foreground">
          <Layers aria-hidden className="size-7" />
          <p>{library === null ? "Đang tải kho lớp phủ…" : library.length ? "Không có kết quả" : "Kho lớp phủ còn trống"}</p>
        </div>
      )}
    </div>
  );
}
