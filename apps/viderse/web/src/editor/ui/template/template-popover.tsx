"use client";

/**
 * Top-bar "Template của kênh" button: opens a small popover listing the owner
 * channels so the owner can mark this project as a channel's template (or
 * clear it) and assign template roles to tracks. Everything else about a
 * template (clips, overlays, image recipe, preview) lives on its template page,
 * linked from here.
 * Legacy clone projects get their track roles auto-assigned in one
 * undoable batch, and the project is saved before the mark so the channel
 * always points at what is on disk.
 */
import { useEffect, useState, type ReactNode } from "react";
import Link from "../../bridge/navigation";
import { Check, LoaderCircle, SlidersHorizontal, Tv } from "lucide-react";
import { toast } from "@/editor/ui/toast";
import { useTimelineSettingsStore } from "@editor/features/timeline/stores/timeline-settings-store";
import { z } from "zod";
import { executeCommands } from "@editor/commands";
import type { SerializedCommand } from "@editor/commands/types";
import { useTimelineStore } from "@editor/features/timeline/stores/timeline-store";
import { useSelectionStore } from "@editor/shared/state/selection";
import { appRoutes } from "@/lib/app-routes";
import { suggestTemplateRoleCommands, TEMPLATE_SLOT_LABELS } from "@editor/features/editing-templates/channel-template";
import { Popover, PopoverContent, PopoverHeader, PopoverTitle, PopoverTrigger } from "@/components/ui/popover";
import { EditorIconButton, EditorSelect } from "@/editor/ui/controls";
import { announce } from "@/editor/ui/shell/announcer";
import { refreshTemplateStatus, useTemplateStatus } from "./template-status";

const channelsResponse = z.object({
  channels: z.array(
    z.object({
      id: z.string().min(1),
      name: z.string().min(1),
      templateProjectIds: z.array(z.string()),
    }),
  ),
});

type OwnerChannelOption = z.infer<typeof channelsResponse>["channels"][number];

interface TemplatePopoverProps {
  projectId: string;
  save: () => Promise<boolean>;
  saving: boolean;
}

export function TemplatePopover({ projectId, save, saving }: TemplatePopoverProps) {
  const { status, ready } = useTemplateStatus(projectId);
  const current = status?.channel ?? null;
  const [open, setOpen] = useState(false);
  const [channels, setChannels] = useState<OwnerChannelOption[] | null>(null);
  const [busy, setBusy] = useState<string | null>(null);
  const [loadFailed, setLoadFailed] = useState(false);
  const tracks = useTimelineStore((state) => state.tracks);
  const activeTrackId = useSelectionStore((state) => state.activeTrackId);
  const roleTracks = tracks.filter((track) => !track.isGroup);
  const track = roleTracks.find((entry) => entry.id === activeTrackId) ?? roleTracks[0];

  const updateTracks = (commands: SerializedCommand[], done: string) => {
    const result = executeCommands(commands);
    if (!result.ok) {
      const message = result.issues.find((issue) => /[À-ỹ]/u.test(issue.message))?.message ?? "Không đổi được cài đặt track.";
      toast.error(message);
      announce(message, "assertive");
      return;
    }
    announce(done);
  };

  /** Assigning a slot another track already holds frees it first — one batch, one undo. */
  const setRole = (slot: string) => {
    if (!track) return;
    const value = slot === "none" ? null : slot;
    const commands: SerializedCommand[] = [];
    if (value) {
      const holder = tracks.find((entry) => entry.templateSlot === value && entry.id !== track.id);
      if (holder) commands.push({ type: "track.update", params: { id: holder.id, updates: { templateSlot: null } } });
    }
    commands.push({ type: "track.update", params: { id: track.id, updates: { templateSlot: value } } });
    updateTracks(commands, value ? `Đã gán vai trò ${TEMPLATE_SLOT_LABELS[value as keyof typeof TEMPLATE_SLOT_LABELS]} cho ${track.name}` : `Đã bỏ vai trò của ${track.name}`);
  };

  useEffect(() => {
    if (!open) return;
    const controller = new AbortController();
    const load = async () => {
      try {
        const response = await fetch("/api/my-channels", { signal: controller.signal });
        if (!response.ok) throw new Error(String(response.status));
        const parsed = channelsResponse.safeParse(await response.json());
        if (parsed.success) setChannels(parsed.data.channels);
        else setLoadFailed(true);
      } catch {
        if (!controller.signal.aborted) setLoadFailed(true);
      }
    };
    void load();
    return () => controller.abort();
  }, [open]);

  const choose = async (channel: OwnerChannelOption | null) => {
    if (busy) return;
    const adding = channel !== null && !channel.templateProjectIds.includes(projectId);
    setBusy(channel?.id ?? "clear");
    try {
      if (adding) {
        // One batch = one undo step; when the legacy track names are absent or
        // a role already exists the suggestion is empty and nothing changes.
        const roles = suggestTemplateRoleCommands(useTimelineStore.getState().tracks);
        if (roles.length) {
          const result = executeCommands(roles);
          if (!result.ok) {
            toast.error(`Chưa gán được vai trò template: ${result.issues.map((issue) => issue.message).join("; ")}`);
            return;
          }
          announce("Đã gán vai trò template cho các track");
        }
        // A save already in flight may predate the role batch: retry until a
        // save lands with nothing left dirty, so the channel points at what is on disk.
        let saved = false;
        for (let attempt = 0; attempt < 3 && !saved; attempt++) {
          saved = (await save()) && !useTimelineSettingsStore.getState().isDirty;
        }
        if (!saved) {
          toast.error("Chưa lưu được project nên chưa đặt template. Thử lại sau khi lưu xong.");
          return;
        }
      }
      const target = channel?.id ?? current?.id;
      if (!target) return;
      const response = await fetch(`/api/my-channels/${encodeURIComponent(target)}/template`, {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ op: adding ? "add" : "remove", projectId }),
      });
      if (!response.ok) {
        const parsed = z.object({ error: z.string().optional(), recovery: z.string().nullish() }).safeParse(await response.json().catch(() => null));
        const body = parsed.success ? parsed.data : null;
        toast.error(body?.error ?? "Chưa đổi được template của kênh", body?.recovery ? { description: body.recovery } : undefined);
        return;
      }
      await refreshTemplateStatus(projectId);
      announce(adding ? `Đã thêm project này vào template của kênh ${channel?.name}` : "Đã bỏ template này khỏi kênh");
      setOpen(false);
    } finally {
      setBusy(null);
    }
  };

  return (
    <Popover open={open} onOpenChange={next => {
      setOpen(next);
      if (next) setLoadFailed(false);
    }}>
      <PopoverTrigger asChild>
        <EditorIconButton
          icon={Tv}
          label="Template của kênh"
          aria-pressed={Boolean(current)}
          className={current ? "text-primary aria-pressed:text-primary" : undefined}
        />
      </PopoverTrigger>
      <PopoverContent align="end" className="w-72 p-3">
        <PopoverHeader className="pb-2">
          <PopoverTitle className="text-xs font-semibold">Template của kênh</PopoverTitle>
        </PopoverHeader>
        {current && (
          <p className="pb-2 text-xs text-muted-foreground">
            Project này đang là template của <span className="font-medium text-foreground">{current.name}</span>.
          </p>
        )}
        {status?.template && (
          <Link
            href={appRoutes.template(projectId)}
            data-hit-area
            className="mb-2 flex min-h-7 items-center gap-2 rounded-md px-2 py-1 text-xs hover:bg-secondary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring [@media(pointer:coarse)]:min-h-11"
          >
            <SlidersHorizontal aria-hidden className="size-3.5 shrink-0" /> Cài đặt template
          </Link>
        )}
        {loadFailed ? (
          <p className="text-xs text-muted-foreground">Chưa tải được danh sách kênh. Đóng rồi mở lại để thử tiếp.</p>
        ) : channels === null ? (
          <p className="flex items-center gap-2 text-xs text-muted-foreground" role="status">
            <LoaderCircle aria-hidden className="size-3.5 animate-spin" /> Đang tải kênh của tôi…
          </p>
        ) : channels.length === 0 ? (
          <p className="text-xs text-muted-foreground">
            Chưa có kênh của tôi nào. Tạo kênh ở trang <span className="font-medium text-foreground">Kênh của tôi</span> trước.
          </p>
        ) : (
          <ul className="flex flex-col gap-0.5" aria-label="Chọn kênh nhận template này">
            {channels.map((channel) => {
              const contains = channel.templateProjectIds.includes(projectId);
              return (
                <li key={channel.id}>
                  <button
                    type="button"
                    data-hit-area
                    disabled={busy !== null || saving || !ready}
                    className="flex min-h-7 w-full items-center gap-2 rounded-md px-2 py-1 text-left text-xs hover:bg-secondary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-60 [@media(pointer:coarse)]:min-h-11"
                    onClick={() => void choose(channel)}
                  >
                    <span className="flex-1 truncate">
                      {channel.name}
                      {contains && channel.id !== current?.id && <span className="block text-[10px] text-muted-foreground">Kênh này cũng đang dùng project này làm template</span>}
                    </span>
                    {busy === channel.id ? (
                      <LoaderCircle aria-hidden className="size-3.5 shrink-0 animate-spin text-muted-foreground" />
                    ) : contains ? (
                      <Check aria-hidden className="size-3.5 shrink-0 text-primary" />
                    ) : null}
                  </button>
                </li>
              );
            })}
            {current && (
              <li>
                <button
                  type="button"
                  data-hit-area
                    disabled={busy !== null || saving}
                  className="flex min-h-7 w-full items-center gap-2 rounded-md px-2 py-1 text-left text-xs text-destructive hover:bg-destructive/10 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:opacity-60 [@media(pointer:coarse)]:min-h-11"
                  onClick={() => void choose(null)}
                >
                  Bỏ template của kênh {current.name}
                </button>
              </li>
            )}
          </ul>
        )}
        {current && track && (
          <div className="mt-2 flex flex-col gap-1.5 border-t border-border pt-2">
            <FieldRow label="Track">
              <EditorSelect
                label="Track gán vai trò"
                value={track.id}
                onValueChange={(id) => useSelectionStore.getState().setActiveTrack(id)}
                options={roleTracks.map((entry) => ({ value: entry.id, label: entry.name }))}
              />
            </FieldRow>
            <FieldRow label="Vai trò">
              <EditorSelect
                label={`Vai trò template của ${track.name}`}
                value={track.templateSlot ?? "none"}
                onValueChange={setRole}
                options={[
                  ...(Object.entries(TEMPLATE_SLOT_LABELS) as [keyof typeof TEMPLATE_SLOT_LABELS, string][]).map(([slot, label]) => ({
                    value: slot,
                    label,
                    disabled: slot === "source-audio" ? track.kind !== "audio" : track.kind === "audio",
                  })),
                  { value: "none", label: "Không" },
                ]}
              />
            </FieldRow>
          </div>
        )}
        <p className="pt-2 text-[10px] leading-snug text-muted-foreground">
          Track đang chọn trên timeline được chọn sẵn ở đây; vai trò cũng gán được qua menu chuột phải trên tên track.
        </p>
      </PopoverContent>
    </Popover>
  );
}

/** Compact label + control line, matching the popover's 12px density. */
function FieldRow({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="grid grid-cols-[4.5rem_minmax(0,1fr)] items-center gap-2">
      <span className="text-xs text-muted-foreground">{label}</span>
      <div className="min-w-0">{children}</div>
    </div>
  );
}
