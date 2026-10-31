"use client";

import { useRef, useState } from 'react'
import { AlertCircle, Check, ChevronRight, Clapperboard, Download, FileVideo, Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from '@/components/ui/alert-dialog'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { EditorIconButton, EditorSelect } from '@/editor/ui/controls'
import { getEditorStorage } from '@editor/storage/storage-backend'
import type { ServerExportRecord } from '@editor/storage/types'
import { EXPORT_PRESETS, EXPORT_QUALITY_OPTIONS, exportTechnicalSummary, formatBytes, formatDuration, preflightMessage, type ExportPreset } from './export-settings'
import { useExportController, useExportDialogState } from './use-export-controller'
import type { CloneQuality } from '@/editor/ui/clone/use-clone-quality'

const actionClass = 'min-h-9 disabled:bg-muted disabled:text-muted-foreground disabled:opacity-100 [@media(pointer:coarse)]:min-h-11'
const summaryClass = 'flex min-h-7 cursor-pointer list-none items-center gap-1.5 [&::-webkit-details-marker]:hidden [@media(pointer:coarse)]:min-h-11'
export function ExportButton({ quality }: { quality?: CloneQuality }) {
  const show = useExportDialogState(state => state.show)
  const blocked = quality?.clone != null && !quality.canExport
  return <Tooltip><TooltipTrigger asChild><EditorIconButton id="editor-export-trigger" icon={Clapperboard} label={blocked ? "Xuất video (chưa đạt Độ hoàn thiện)" : "Xuất video"} shortcut="⌘E" onClick={show} /></TooltipTrigger><TooltipContent>{blocked ? "Xuất video — chưa đạt Độ hoàn thiện, xem chi tiết" : "Xuất video"}</TooltipContent></Tooltip>
}

function PhaseProgress({ label, value, detail }: { label: string; value: number; detail: string }) {
  return <div className="space-y-3"><div className="flex items-center justify-between gap-4"><span className="text-sm">{label}</span><span className="text-xl font-semibold tabular-nums">{Math.round(value)}%</span></div><progress className="h-2 w-full overflow-hidden rounded-full accent-primary [&::-webkit-progress-bar]:bg-muted [&::-webkit-progress-value]:bg-primary" value={value} max={100} aria-label={label} /><p className="font-mono text-xs tabular-nums text-muted-foreground">{detail}</p></div>
}

export function ExportDialog({ visible = true, quality }: { visible?: boolean; quality?: CloneQuality } = {}) {
  const cloneLocked = quality?.clone != null
  const cloneBlocked = cloneLocked && !quality!.canExport
  const flow = useExportController(quality ?? null)
  const firstControl = useRef<HTMLButtonElement>(null)
  const [deleteTarget, setDeleteTarget] = useState<ServerExportRecord | null>(null)
  const [accepting, setAccepting] = useState(false)
  const [acceptError, setAcceptError] = useState<string | null>(null)
  const exportAnyway = () => {
    if (!quality || accepting) return
    setAccepting(true)
    setAcceptError(null)
    quality.acceptQuality(true)
      .catch((error: unknown) => setAcceptError(error instanceof Error ? error.message : String(error)))
      .finally(() => setAccepting(false))
  }
  const checklistBlocked = cloneBlocked && quality!.checklistBlocked
  const finishedVideo = flow.saved && flow.project ? getEditorStorage().exportUrl(flow.project.id, flow.saved.id) : null
  const close = () => useExportDialogState.getState().close()
  const title = flow.stage === 'rendering' ? 'Đang xuất video' : flow.stage === 'saving' ? 'Đang lưu vào máy' : flow.stage === 'done' ? 'Xuất xong' : flow.stage === 'error' ? flow.hasScratch ? 'Chưa lưu được video' : 'Chưa dựng được video' : flow.stage === 'cancelled' ? 'Đã huỷ xuất' : 'Xuất video'
  const description = flow.stage === 'rendering' ? 'Đang ghép hình và tiếng thành một video. Bạn có thể huỷ bất cứ lúc nào.' : flow.stage === 'saving' ? 'Video đã dựng xong. Giữ tab này mở đến khi lưu xong.' : flow.stage === 'done' ? 'Video đã được lưu an toàn trên máy của bạn.' : flow.hasScratch ? 'Video đã dựng xong. Giữ tab này mở để lưu lại mà không cần dựng lại.' : flow.stage === 'cancelled' ? 'Đã dừng và dọn file tạm. Dự án của bạn không thay đổi.' : 'Tạo file video để đăng lên YouTube. Video được lưu trên máy của bạn.'
  const renderPercent = flow.progress?.progress ?? 0
  const elapsed = flow.elapsedSeconds
  const eta = renderPercent > 0 ? elapsed * (100 - renderPercent) / renderPercent : null
  const canExport = !cloneBlocked && flow.preflight?.canExport === true && flow.preflight.resolvedSettings?.resolution.width === flow.settings.resolution.width && flow.preflight.resolvedSettings.resolution.height === flow.settings.resolution.height && flow.preflight.resolvedSettings.fps === flow.settings.fps && flow.preflight.resolvedSettings.quality === flow.quality && flow.preflight.estimatedDurationSeconds === flow.durationFrames / (flow.sequence?.fps ?? 30) && !flow.preflightError

  return <>
    <Dialog open={flow.open && visible} onOpenChange={open => { if (!open) close() }}>
      <DialogContent showCloseButton={false} className="max-h-[calc(100dvh-3rem)] overflow-y-auto rounded-xl border-ring/60 bg-card sm:max-w-[560px]" onOpenAutoFocus={event => { if (flow.stage === 'settings') { event.preventDefault(); firstControl.current?.focus() } }} onCloseAutoFocus={event => { event.preventDefault(); document.getElementById('editor-export-trigger')?.focus() }}>
        <DialogHeader><DialogTitle className="text-xl">{title}</DialogTitle><DialogDescription>{description}</DialogDescription></DialogHeader>
        {flow.stage === 'settings' && <>
          {cloneBlocked && <div role="alert" className="mb-3 flex flex-col gap-2 rounded-lg border border-rose-500/40 bg-rose-500/10 p-3 text-xs text-rose-700 dark:text-rose-300"><p>Chưa đạt Độ hoàn thiện để xuất{quality?.score ? ` (${quality.score.total}/100)` : ''}{quality?.gateReason ? ` — ${quality.gateReason}` : ''}. Mở đồng hồ Độ hoàn thiện trên header để xem mục cần sửa{checklistBlocked ? ', hoặc vẫn xuất như hiện tại' : ''}.</p>{checklistBlocked && <Button type="button" variant="outline" className={`${actionClass} w-fit text-foreground`} aria-busy={accepting || undefined} onClick={exportAnyway}><Download aria-hidden />{accepting ? 'Đang lưu…' : 'Vẫn xuất như hiện tại'}</Button>}{acceptError && <p>{acceptError}</p>}</div>}
          {cloneLocked && <p className="mb-3 text-xs text-muted-foreground">Video clone: giữ chất lượng Full HD và xuất toàn bộ video.</p>}
          <div className="grid grid-cols-2 gap-4">
            <div className="grid gap-2 text-xs"><label htmlFor="export-preset">Kiểu video</label><EditorSelect ref={firstControl} id="export-preset" size="field" value={flow.preset} disabled={cloneLocked} onValueChange={value => flow.setPreset(value as ExportPreset)} options={EXPORT_PRESETS.map(preset => ({ value: preset.id, label: preset.label }))} /></div>
            <div className="grid gap-2 text-xs"><label htmlFor="export-range">Phần cần xuất</label><EditorSelect id="export-range" size="field" value={flow.whole ? 'whole' : 'range'} disabled={cloneLocked} onValueChange={value => flow.setWhole(value === 'whole')} options={[{ value: 'whole', label: 'Cả video' }, { value: 'range', label: `Đoạn đã đánh dấu${flow.validRange ? '' : ' · Chưa đánh dấu'}`, disabled: !flow.validRange }]} /></div>
            <div className="grid gap-2 text-xs"><label htmlFor="export-quality">Chất lượng</label><EditorSelect id="export-quality" size="field" value={flow.quality} onValueChange={value => flow.setQuality(value as typeof flow.quality)} options={EXPORT_QUALITY_OPTIONS} /></div>
          </div>
          <section className="space-y-2 rounded-lg border bg-background p-3 text-xs" aria-label="Kiểm tra trước khi xuất">
            <p className="font-medium tabular-nums">Video dài {formatDuration(flow.durationFrames / (flow.sequence?.fps ?? 30))}</p>
            <p className="text-muted-foreground">{flow.preflight?.estimatedFileSizeBytes ? `Khoảng ${formatBytes(flow.preflight.estimatedFileSizeBytes)} · ` : ''}{canExport ? 'Sẵn sàng xuất' : flow.preflightError || (flow.preflight ? 'Cần kiểm tra trước khi xuất' : 'Đang kiểm tra khả năng xuất…')}</p>
            {flow.preflight?.checks.filter(check => check.severity !== 'ok').map(check => <p key={check.id} className="flex items-start gap-2"><AlertCircle aria-hidden className="mt-0.5 size-3.5 shrink-0 text-muted-foreground" />{preflightMessage(check)}</p>)}
            {flow.donorWarnings.map(warning => <p key={warning.code} className="flex items-start gap-2"><AlertCircle aria-hidden className="mt-0.5 size-3.5 shrink-0 text-muted-foreground" />{warning.message}</p>)}
            <details className="group"><summary className={`${summaryClass} text-muted-foreground`}><ChevronRight aria-hidden className="size-3.5 transition-transform duration-150 group-open:rotate-90" />Chi tiết</summary><ul className="space-y-1 pt-2"><li>{exportTechnicalSummary(flow.settings)}</li>{flow.preflight?.checks.map(check => <li key={check.id}>{preflightMessage(check)}</li>)}</ul></details>
          </section>
        </>}
        {flow.stage === 'rendering' && <PhaseProgress label="Bước 1: Đang dựng video" value={renderPercent} detail={eta !== null ? `Còn khoảng ${formatDuration(eta)}` : 'Đang chuẩn bị…'} />}
        {flow.stage === 'saving' && <PhaseProgress label="Bước 2: Đang lưu video vào máy" value={flow.saveProgress.total ? flow.saveProgress.sent / flow.saveProgress.total * 100 : 0} detail={`${formatBytes(flow.saveProgress.sent)} / ${formatBytes(flow.saveProgress.total)}${flow.saveProgress.sent === flow.saveProgress.total ? ' · Đang kiểm tra lần cuối' : ''}`} />}
        {flow.busy && <div className="rounded-lg border bg-background p-3 text-xs"><p className={flow.stage === 'rendering' ? 'font-medium' : 'text-muted-foreground'}>1. Dựng video{flow.stage === 'saving' ? ' · Xong' : ''}</p><p className={flow.stage === 'saving' ? 'font-medium' : 'text-muted-foreground'}>2. Lưu vào máy{flow.stage === 'rendering' ? ' · Chưa bắt đầu' : ''}</p></div>}
        {flow.stage === 'error' && <div role="alert" className="rounded-lg border border-ring/60 bg-background p-4 text-sm"><AlertCircle aria-hidden className="mb-2 size-5 text-muted-foreground" /><p>{flow.error}</p>{flow.hasScratch && <p className="mt-2 text-xs text-muted-foreground">Video đã dựng xong và vẫn còn đó. Bấm Lưu lại để lưu tiếp, không cần dựng lại.</p>}{flow.errorDetail && <details className="group mt-2 text-xs"><summary className={`${summaryClass} text-muted-foreground`}><ChevronRight aria-hidden className="size-3.5 transition-transform duration-150 group-open:rotate-90" />Chi tiết</summary><p className="break-words pt-2 font-mono text-muted-foreground">{flow.errorDetail}</p></details>}</div>}
        {flow.stage === 'cancelled' && <p className="rounded-lg border bg-background p-3 text-sm">{flow.hasScratch ? 'Đã dừng lúc đang lưu. Video đã dựng xong vẫn còn trong tab này; bấm Lưu lại hoặc bỏ nó đi.' : 'Chưa có file nào được tạo. Dự án của bạn không thay đổi.'}</p>}
        {flow.stage === 'done' && flow.saved && <div className="space-y-3"><div className="rounded-lg border bg-background p-3"><p className="flex items-center gap-2 text-sm font-medium"><Check aria-hidden className="size-4" />{flow.saved.fileName}</p><p className="mt-1 font-mono text-xs tabular-nums text-muted-foreground">{formatBytes(flow.saved.size)} · {flow.outcome ? formatDuration(flow.outcome.result.duration) : ''}</p></div>{finishedVideo && <video controls preload="metadata" src={finishedVideo} aria-label={`Phát thử ${flow.saved.fileName}`} className="max-h-64 w-full rounded-lg bg-background" />}</div>}
        {flow.outcome && <details className="group text-xs"><summary className={`${summaryClass} text-muted-foreground`}><ChevronRight aria-hidden className="size-3.5 transition-transform duration-150 group-open:rotate-90" />Chi tiết</summary><p className="pt-2">{flow.outcome.renderPath === 'worker' ? 'Video được dựng nền nên editor vẫn dùng mượt.' : 'Video được dựng theo cách chậm hơn nên editor có thể đứng hình khi xuất.'}</p>{flow.outcome.fallbackReason && <p className="mt-1 break-all font-mono text-muted-foreground">Lý do: {flow.outcome.fallbackReason}</p>}</details>}
        {!flow.busy && <details className="group border-t pt-3"><summary className={`${summaryClass} text-sm`}><ChevronRight aria-hidden className="size-3.5 transition-transform duration-150 group-open:rotate-90" />Bản xuất đã lưu <span className="text-muted-foreground tabular-nums">({flow.exports.length})</span></summary><div className="space-y-2 pt-3">
          {flow.listError && <div role="alert" className="text-xs"><p>{flow.listError}</p><Button variant="outline" size="sm" className={actionClass} onClick={() => void flow.refreshExports()} disabled={flow.listLoading}>Thử lại</Button></div>}
          {!flow.exports.length && !flow.listError && <p className="flex items-center gap-2 py-2 text-xs text-muted-foreground"><Clapperboard aria-hidden className="size-4" />Chưa có bản xuất đã lưu.</p>}
          {flow.exports.map(record => <div key={record.id} className="flex items-center gap-2 rounded-lg border bg-background p-2"><FileVideo aria-hidden className="size-4 shrink-0 text-muted-foreground" /><div className="min-w-0 flex-1"><p className="break-words text-xs font-medium">{record.fileName}</p><p className="text-xs tabular-nums text-muted-foreground">{formatBytes(record.size)} · {new Date(record.committedAt ?? record.createdAt).toLocaleString('vi-VN')}</p></div><Tooltip><TooltipTrigger asChild><a className="flex size-7 shrink-0 items-center justify-center rounded-md hover:bg-secondary [@media(pointer:coarse)]:size-11" data-hit-area aria-label={`Tải ${record.fileName}`} href={getEditorStorage().exportUrl(flow.project!.id, record.id)} download={record.fileName}><Download aria-hidden className="size-3.5" /></a></TooltipTrigger><TooltipContent>Tải {record.fileName}</TooltipContent></Tooltip><EditorIconButton icon={Trash2} label={`Xoá ${record.fileName}`} onClick={() => setDeleteTarget(record)} disabled={flow.deleting !== null} /></div>)}
        </div></details>}
        <DialogFooter>
          <Button variant="outline" className={actionClass} onClick={close}>{flow.busy ? 'Thu gọn' : 'Đóng'}</Button>
          {flow.busy ? <Button variant="outline" className={actionClass} onClick={flow.cancel}>{flow.stage === 'saving' ? 'Huỷ lưu' : 'Huỷ xuất'}</Button> : flow.stage === 'settings' ? <Button className={`${actionClass} bg-primary`} disabled={!canExport} onClick={() => void flow.start()}>Xuất video</Button> : flow.hasScratch ? <><Button variant="outline" className={actionClass} onClick={() => void flow.reset()}>Bỏ bản dựng</Button><Button className={`${actionClass} bg-primary`} onClick={() => void flow.retrySave()}>Lưu lại</Button></> : <><Button variant="outline" className={actionClass} onClick={() => void flow.reset()}>Xuất lại</Button>{finishedVideo && flow.saved && <Button asChild className={`${actionClass} bg-primary`}><a href={finishedVideo} download={flow.saved.fileName}><Download aria-hidden />Tải về</a></Button>}</>}
        </DialogFooter>
      </DialogContent>
    </Dialog>
    <AlertDialog open={deleteTarget !== null && visible} onOpenChange={open => { if (!open) setDeleteTarget(null) }}><AlertDialogContent><AlertDialogHeader><AlertDialogTitle>Xoá bản xuất?</AlertDialogTitle><AlertDialogDescription>Xoá “{deleteTarget?.fileName}” khỏi máy. Dự án và các file gốc không thay đổi.</AlertDialogDescription></AlertDialogHeader><AlertDialogFooter><AlertDialogCancel className={actionClass}>Huỷ</AlertDialogCancel><AlertDialogAction className={`${actionClass} bg-primary`} onClick={() => { if (deleteTarget) void flow.deleteExport(deleteTarget); setDeleteTarget(null) }}>Xoá bản xuất</AlertDialogAction></AlertDialogFooter></AlertDialogContent></AlertDialog>
  </>
}
