"use client";

import { useId, useRef, useState, type FormEvent } from "react";
import { toast } from "@/editor/ui/toast";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { announce } from "@/editor/ui/shell/announcer";
import { useMediaLibraryStore } from "@editor/features/media-library/stores/media-library-store";
import { useProjectStore } from "@editor/features/projects/stores/project-store";
import { serverMediaToMetadata } from "@editor/storage/media-adapter";
import { getEditorStorage } from "@editor/storage/storage-backend";

const showError = (error: unknown) => toast.error(error instanceof Error ? error.message : "Không thực hiện được. Thử lại.");

/**
 * Tải link chạy như một job phía server nên form chỉ chờ trạng thái: đóng form
 * giữa chừng không dừng việc tải, xong thì thư viện tự nạp lại và chọn media
 * mới — quay lại editor vẫn thấy clip đã tải.
 */
async function importLinkMedia(options: { projectId: string; url: string; from?: string; to?: string; overlay?: boolean; signal?: AbortSignal }) {
  const { media, reused } = await getEditorStorage().importMediaFromLink(options);
  const meta = serverMediaToMetadata(media);
  // Reload like an upload finishing: the new record lands in the project list,
  // then becomes the selected card.
  await useMediaLibraryStore.getState().loadMediaItems();
  useMediaLibraryStore.getState().selectMedia([meta.id]);
  announce(reused ? `${meta.fileName} đã có trong thư viện` : `Đã nhập ${meta.fileName}`);
  return meta;
}

/**
 * "Lấy từ link": fetch a Twitch or Bilibili video (optionally a Từ/Đến cut)
 * straight into the project media. With `overlay` the media is saved as an
 * overlay layer like an overlay uploaded from disk. The download runs as a
 * server job — the form stays busy and Huỷ cancels the job.
 */
export function LinkImportForm({ overlay = false, onImported }: { overlay?: boolean; onImported?: (mediaId: string) => void }) {
  const id = useId();
  const [url, setUrl] = useState("");
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");
  const [busy, setBusy] = useState(false);
  const controller = useRef<AbortController | null>(null);

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    const projectId = useProjectStore.getState().currentProject?.id;
    if (!projectId) {
      showError(new Error("Chưa mở project nào"));
      return;
    }
    const abort = new AbortController();
    controller.current = abort;
    setBusy(true);
    try {
      const meta = await importLinkMedia({
        projectId,
        url: url.trim(),
        from: from.trim() || undefined,
        to: to.trim() || undefined,
        overlay,
        signal: abort.signal,
      });
      setUrl("");
      setFrom("");
      setTo("");
      onImported?.(meta.id);
    } catch (error) {
      if (abort.signal.aborted) announce("Đã huỷ lấy video");
      else showError(error);
    } finally {
      controller.current = null;
      setBusy(false);
    }
  };

  return (
    <form aria-busy={busy} onSubmit={(event) => void submit(event)} className="space-y-2 rounded-md border border-ring/50 bg-card p-2">
      <label htmlFor={`${id}-url`} className="block text-[10px] text-muted-foreground">Link video Twitch hoặc Bilibili</label>
      <Input id={`${id}-url`} required value={url} onChange={(event) => setUrl(event.target.value)} placeholder="https://www.twitch.tv/videos/…" className="h-8 border-ring/60 text-xs" />
      <div className="grid grid-cols-2 gap-1">
        <label className="space-y-1 text-[10px] text-muted-foreground">Từ
          <Input value={from} onChange={(event) => setFrom(event.target.value)} placeholder="1:05" className="h-7 border-ring/60 font-mono text-xs tabular-nums" />
        </label>
        <label className="space-y-1 text-[10px] text-muted-foreground">Đến
          <Input value={to} onChange={(event) => setTo(event.target.value)} placeholder="1:35" className="h-7 border-ring/60 font-mono text-xs tabular-nums" />
        </label>
      </div>
      <p className="text-[10px] leading-relaxed text-muted-foreground">Video dài cần đoạn Từ/Đến, tối đa 10 phút.</p>
      <div className="flex items-center gap-1">
        <Button type="submit" size="sm" className="min-w-0 flex-1 text-background" disabled={busy}>{busy ? "Đang lấy…" : "Lấy"}</Button>
        {busy && <Button type="button" variant="outline" size="sm" onClick={() => controller.current?.abort()}>Huỷ</Button>}
      </div>
      {busy && <p role="status" className="text-[10px] text-muted-foreground">Đang tải video từ link — có thể mất một lúc.</p>}
    </form>
  );
}
