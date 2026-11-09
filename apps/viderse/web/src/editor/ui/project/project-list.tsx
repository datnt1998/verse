"use client";

import Link, { useRouter } from "../../bridge/navigation";
import { useRef, useState, useTransition, type FormEvent } from "react";
import { Clapperboard, Copy, Pencil, Plus, Trash2 } from "lucide-react";
import { toast } from "@/editor/ui/toast";
import { IconButton } from "@/components/icon-tip";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle, AlertDialogTrigger } from "@/components/ui/alert-dialog";
import { createEditorProject, deleteEditorProject, duplicateEditorProject, renameEditorProject } from "@/app/editor/actions";
import type { ProjectCardData } from "./types";
import { JOB_STATUS_LABELS } from "@/components/clone/clone-entry";
import { formatDuration } from "@/lib/format";
import { EditorSelect } from "@/editor/ui/controls";
import { withReturnTo } from "@/lib/navigation-return";
import { LinkPending } from "@/components/link-pending";
import { VideoThumb } from "@/components/video-thumb";

const dateFormatter = new Intl.DateTimeFormat("vi-VN", { dateStyle: "short", timeStyle: "short" });

export function ProjectList({ projects }: { projects: ProjectCardData[] }) {
  const router = useRouter();
  const [createOpen, setCreateOpen] = useState(false);
  const [name, setName] = useState("Video mới");
  const [template, setTemplate] = useState("youtube-1080p30");
  const [pending, startTransition] = useTransition();
  const createRef = useRef<HTMLButtonElement>(null);
  const create = (event: FormEvent) => {
    event.preventDefault();
    startTransition(async () => {
      const result = await createEditorProject(name, template);
      if (!result.ok) { toast.error(result.error); return; }
      setCreateOpen(false);
      toast.success("Đã tạo video mới");
      router.push(withReturnTo(`/editor/${result.id}`, "/editor"));
    });
  };
  const createDialog = <Dialog open={createOpen} onOpenChange={setCreateOpen}>
    <DialogTrigger asChild><IconButton ref={createRef} icon={Plus} label="Tạo video mới" variant="default" className="text-background" /></DialogTrigger>
    <DialogContent showCloseButton={false}>
      <DialogHeader><DialogTitle>Tạo video mới</DialogTitle><DialogDescription>Đặt tên và chọn kiểu video để bắt đầu.</DialogDescription></DialogHeader>
      <form onSubmit={create} className="grid min-w-0 gap-4">
        <div className="grid gap-2"><label htmlFor="new-project-name" className="text-sm">Tên video</label><Input id="new-project-name" className="h-11 border-input" value={name} onChange={(event) => setName(event.target.value)} required maxLength={200} autoFocus /></div>
        <div className="grid gap-2"><label htmlFor="new-project-preset" className="text-sm">Kiểu video</label><EditorSelect id="new-project-preset" size="field" value={template} onValueChange={setTemplate} options={[{ value: "youtube-1080p30", label: "YouTube · 1920×1080 · 30 fps" }, { value: "shorts-1080x1920", label: "Shorts · 1080×1920 · 30 fps" }]} /></div>
        <DialogFooter><DialogClose asChild><Button type="button" variant="outline" className="h-11" disabled={pending}>Huỷ</Button></DialogClose><Button type="submit" className="h-11 text-background" disabled={pending || !name.trim()}>{pending ? "Đang tạo…" : "Tạo video"}</Button></DialogFooter>
      </form>
    </DialogContent>
  </Dialog>;
  return <main className="mx-auto max-w-7xl px-4 py-8 sm:px-6">
    <header className="mb-8 flex min-w-0 flex-wrap items-center justify-between gap-4"><div className="min-w-0"><h1 className="text-xl font-semibold">Editor</h1><p className="mt-1 text-sm tabular-nums text-muted-foreground">{projects.length} dự án · Chỉnh sửa video thủ công</p></div>{createDialog}</header>
    {projects.length === 0 ? <section className="flex min-h-80 flex-col items-center justify-center gap-4 rounded-xl border border-dashed bg-card/30 px-4 text-center"><Clapperboard className="size-10 text-muted-foreground" aria-hidden /><h2 className="font-semibold">Chưa có video nào</h2><p className="text-sm text-muted-foreground">Tạo video đầu tiên để bắt đầu chỉnh sửa.</p><Button className="h-11 text-background" onClick={() => setCreateOpen(true)}>Tạo video mới</Button></section> : <div className="grid min-w-0 gap-6 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">{projects.map((project) => <ProjectCard key={project.id} project={project} onDeleted={() => createRef.current?.focus()} />)}</div>}
  </main>;
}

function ProjectCard({ project, onDeleted }: { project: ProjectCardData; onDeleted: () => void }) {
  const [renameOpen, setRenameOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [name, setName] = useState(project.name);
  const [pending, startTransition] = useTransition();
  const duration = Math.max(0, Math.floor(project.duration));
  const time = formatDuration(duration) ?? "0:00";
  const modified = dateFormatter.format(project.updatedAt);
  const rename = (event: FormEvent) => {
    event.preventDefault();
    startTransition(async () => {
      const result = await renameEditorProject(project.id, name, project.revision);
      if (!result.ok) { toast.error(result.error); return; }
      setRenameOpen(false); toast.success("Đã đổi tên video");
    });
  };
  const cloneStatus = project.clone ? (JOB_STATUS_LABELS[project.clone.status] ?? "Đang cập nhật") : "";
  return <article className="flex h-full min-w-0 flex-col overflow-hidden rounded-xl border bg-card transition-colors duration-150 hover:border-ring/50">
    <Link href={withReturnTo(`/editor/${project.id}`, "/editor")} className="group relative flex min-w-0 flex-1 flex-col">
      <LinkPending variant="ring" />
      <div className="relative flex aspect-video items-center justify-center overflow-hidden border-b bg-background">
        {project.clone?.source?.thumbnailUrl ? (
          <VideoThumb videoId={null} src={project.clone.source.thumbnailUrl} score={null} sizes="(min-width: 1280px) 25vw, (min-width: 768px) 33vw, 100vw" className="absolute inset-0 size-full rounded-none" />
        ) : (
          <Clapperboard className="size-10 text-muted-foreground transition-transform duration-150 group-hover:scale-105" aria-hidden />
        )}
      </div>
      <div className="min-w-0 p-4"><h2 className="line-clamp-2 break-words font-medium">{project.name}</h2><p className="mt-2 text-xs tabular-nums text-muted-foreground">{project.width > project.height ? "Video ngang" : project.width < project.height ? "Video dọc" : "Video vuông"}</p><p className="mt-1 text-xs tabular-nums text-muted-foreground">Dài {time}</p><p className="mt-3 break-words text-xs tabular-nums text-muted-foreground">Sửa {modified}</p>
      </div>
    </Link>
    {project.clone ? (
      <Link
        href={withReturnTo(project.clone.wizardUrl, "/editor")}
        className="relative flex min-h-11 min-w-0 items-center gap-3 border-b px-4 py-2 transition-colors hover:bg-muted/40"
      >
        <LinkPending variant="ring" />
        <span className="min-w-0 flex-1">
          <span className="block truncate text-xs font-medium">{project.clone.source?.title ?? "Clone job"}</span>
          <span className="mt-0.5 block truncate text-xs text-muted-foreground">
            {project.clone.source?.channelTitle ?? ""}{project.clone.source?.channelTitle ? " · " : ""}
            {cloneStatus} · {project.clone.progress.done}/{project.clone.progress.total}
          </span>
        </span>
      </Link>
    ) : null}
    <div className="mt-auto flex items-center gap-2 border-t px-2 py-1">
      <Dialog open={renameOpen} onOpenChange={(open) => { setRenameOpen(open); if (open) setName(project.name); }}>
        <DialogTrigger asChild><IconButton icon={Pencil} label={`Đổi tên ${project.name}`} disabled={pending} /></DialogTrigger>
        <DialogContent showCloseButton={false}><DialogHeader><DialogTitle>Đổi tên video</DialogTitle><DialogDescription>Đặt tên dễ nhận biết cho video.</DialogDescription></DialogHeader><form onSubmit={rename} className="grid gap-4"><label htmlFor={`rename-${project.id}`} className="sr-only">Tên video</label><Input id={`rename-${project.id}`} className="h-11 border-input" value={name} onChange={(event) => setName(event.target.value)} required maxLength={200} autoFocus /><DialogFooter><DialogClose asChild><Button type="button" variant="outline" className="h-11" disabled={pending}>Huỷ</Button></DialogClose><Button className="h-11 text-background" disabled={pending || !name.trim()}>{pending ? "Đang lưu…" : "Lưu tên"}</Button></DialogFooter></form></DialogContent>
      </Dialog>
      <IconButton icon={Copy} label={`Nhân bản ${project.name}`} disabled={pending} onClick={() => startTransition(async () => { const result = await duplicateEditorProject(project.id, project.name); if (!result.ok) toast.error(result.error); else toast.success("Đã nhân bản video"); })} />
      <AlertDialog open={deleteOpen} onOpenChange={setDeleteOpen}><AlertDialogTrigger asChild><IconButton icon={Trash2} label={`Xoá ${project.name}`} disabled={pending} /></AlertDialogTrigger><AlertDialogContent onCloseAutoFocus={(event) => { if (pending) return; if (!document.querySelector(`a[href="${withReturnTo(`/editor/${project.id}`, "/editor")}"]`)) { event.preventDefault(); onDeleted(); } }}><AlertDialogHeader><AlertDialogTitle>Xoá “{project.name}”?</AlertDialogTitle><AlertDialogDescription>Video và các bản xuất của video sẽ bị xoá. Media dùng chung vẫn được giữ trên máy.</AlertDialogDescription></AlertDialogHeader><AlertDialogFooter><AlertDialogCancel className="h-11" disabled={pending}>Giữ lại</AlertDialogCancel><AlertDialogAction className="h-11" disabled={pending} onClick={(event) => { event.preventDefault(); startTransition(async () => { const result = await deleteEditorProject(project.id); if (!result.ok) toast.error(result.error); else { setDeleteOpen(false); toast.success("Đã xoá video"); requestAnimationFrame(onDeleted); } }); }}>{pending ? "Đang xoá…" : "Xoá video"}</AlertDialogAction></AlertDialogFooter></AlertDialogContent></AlertDialog>
    </div>
  </article>;
}
