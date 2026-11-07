"use client";

import { useEffect, useMemo, useRef, useState } from 'react'
import { create } from 'zustand'
import { toast } from '@/editor/ui/toast'
import type { ExportSettings, CompositionInputProps } from '@editor/types/export'
import { useProjectStore } from '@editor/features/projects/stores/project-store'
import { useMediaLibraryStore } from '@editor/features/media-library/stores/media-library-store'
import { getActiveExportSequenceId, getExportableSequence, type ExportableSequence } from '@editor/features/export/deps/timeline-compositions'
import { convertTimelineToComposition } from '@editor/features/export/utils/timeline-to-composition'
import { assessExportPreflight, type ExportPreflightResult } from '@editor/features/export/utils/export-preflight'
import { releaseTemporaryExportOutput } from '@editor/features/export/utils/export-output-target'
import type { RunRenderOutcome } from '@editor/features/export/utils/render-pipeline'
import { runServerExport, saveServerExport } from './export-runner'
import type { RenderProgress } from '@editor/features/export/utils/client-renderer'
import { getWebCodecsExportWarnings } from '@editor/donor/openreel/export-limits/webcodecs-limits'
import { getEditorStorage } from '@editor/storage/storage-backend'
import { getProjectRevision } from '@editor/storage/project-persistence'
import { StorageClientError } from '@editor/storage/errors'
import type { ServerExportRecord } from '@editor/storage/types'
import { announce } from '@/editor/ui/shell/announcer'
import { exportSettings, type ExportPreset } from './export-settings'

export const useExportDialogState = create<{ open: boolean; sequence: ExportableSequence | null; show: () => void; close: () => void }>((set) => ({
  open: false, sequence: null,
  show: () => set({ open: true, sequence: structuredClone(getExportableSequence(getActiveExportSequenceId())) }),
  close: () => set({ open: false }),
}))

type ExportStage = 'settings' | 'rendering' | 'saving' | 'done' | 'error' | 'cancelled'
interface Scratch { outcome: RunRenderOutcome; projectId: string; fileName: string; projectRevision?: string }
/** Our own messages are Vietnamese; anything pure ASCII came from the browser or an engine and stays out of the main view. */
function isUserFacing(error: unknown): boolean { return error instanceof Error && /[^\x00-\x7F]/.test(error.message) }
function exportError(error: unknown): string {
  if (error instanceof StorageClientError && error.code === 'insufficient_storage') return 'Máy không đủ dung lượng để lưu video. Dọn dung lượng rồi bấm Lưu lại.'
  if (error instanceof TypeError) return 'Không kết nối được với server trên máy. Kiểm tra kết nối rồi thử lại.'
  return error instanceof Error && isUserFacing(error) ? error.message : 'Chưa xuất được video. Hãy bấm thử lại; nếu vẫn lỗi, mở Chi tiết để xem nguyên nhân.'
}
function exportErrorDetail(error: unknown): string { return error instanceof Error && !isUserFacing(error) ? error.message : '' }

export function useExportController(cloneQuality: { clone: { jobId: string } | null; canExport: boolean; gateReason: string | null } | null = null) {
  const { open, sequence } = useExportDialogState()
  const project = useProjectStore(state => state.currentProject)
  const brokenMediaIds = useMediaLibraryStore(state => state.brokenMediaIds)
  const [preset, setPreset] = useState<ExportPreset>('youtube')
  const [quality, setQuality] = useState<ExportSettings['quality']>('high')
  const [whole, setWhole] = useState(true)
  const [stage, setStage] = useState<ExportStage>('settings')
  const [preflight, setPreflight] = useState<ExportPreflightResult | null>(null)
  const [preflightError, setPreflightError] = useState('')
  const [progress, setProgress] = useState<RenderProgress | null>(null)
  const [saveProgress, setSaveProgress] = useState({ sent: 0, total: 0 })
  const [error, setError] = useState('')
  const [errorDetail, setErrorDetail] = useState('')
  const [elapsedSeconds, setElapsedSeconds] = useState(0)
  const [outcome, setOutcome] = useState<RunRenderOutcome | null>(null)
  const [saved, setSaved] = useState<ServerExportRecord | null>(null)
  const [exports, setExports] = useState<ServerExportRecord[]>([])
  const [listError, setListError] = useState('')
  const [listLoading, setListLoading] = useState(false)
  const [deleting, setDeleting] = useState<string | null>(null)
  const abort = useRef<AbortController | null>(null)
  const scratch = useRef<Scratch | null>(null)
  const session = useRef<{ projectId: string; exportId: string } | null>(null)
  const fail = (reason: unknown) => {
    const message = exportError(reason)
    setError(message); setErrorDetail(exportErrorDetail(reason)); setStage('error')
    announce(`Xuất lỗi: ${message}`, 'assertive')
  }
  const settings = useMemo(() => exportSettings(preset, quality, whole), [preset, quality, whole])
  const validRange = sequence?.inPoint != null && sequence.outPoint != null && sequence.outPoint > sequence.inPoint
  const durationFrames = sequence ? (whole ? sequence.durationFrames : validRange ? sequence.outPoint! - sequence.inPoint! : 0) : 0
  const composition = useMemo<CompositionInputProps | null>(() => sequence ? convertTimelineToComposition(sequence.tracks, sequence.items, sequence.transitions, sequence.fps, sequence.width, sequence.height, whole ? null : sequence.inPoint, whole ? null : sequence.outPoint, sequence.keyframes, sequence.backgroundColor, sequence.busAudioEq, sequence.masterBusDb) : null, [sequence, whole])
  const donorWarnings = useMemo(() => getWebCodecsExportWarnings({ codec: settings.codec, width: settings.resolution.width, height: settings.resolution.height, frameRate: settings.fps ?? sequence?.fps ?? 30 }, durationFrames / (sequence?.fps ?? 30)), [settings, durationFrames, sequence?.fps])

  useEffect(() => {
    if (!open || !composition || !sequence) return
    let active = true
    assessExportPreflight({ settings, fps: sequence.fps, composition, durationFrames, brokenMediaIds }).then(result => { if (active) { setPreflight(result); setPreflightError('') } }).catch(reason => { if (active) { setPreflight(null); setPreflightError(exportError(reason)) } })
    return () => { active = false }
  }, [open, composition, sequence, settings, durationFrames, brokenMediaIds])

  const refreshExports = async () => {
    if (!project) return
    setListLoading(true)
    try { setExports((await getEditorStorage().listExports(project.id)).filter(entry => entry.status === 'ready')); setListError('') }
    catch (reason) { setListError(exportError(reason)) }
    finally { setListLoading(false) }
  }
  useEffect(() => {
    if (!open || !project?.id) return
    let active = true
    getEditorStorage().listExports(project.id).then(records => { if (active) { setExports(records.filter(entry => entry.status === 'ready')); setListError('') } }).catch(reason => { if (active) setListError(exportError(reason)) })
    return () => { active = false }
  }, [open, project?.id])
  useEffect(() => () => {
    abort.current?.abort()
    const active = session.current
    if (active) void getEditorStorage().abortExport(active.projectId, active.exportId).catch(() => undefined)
    if (scratch.current) void releaseTemporaryExportOutput(scratch.current.outcome.result)
  }, [])

  const saveScratch = async (controller: AbortController) => {
    const retained = scratch.current
    if (!retained) return
    const { outcome: rendered, projectId, fileName, projectRevision } = retained
    const storage = getEditorStorage()
    setStage('saving'); setSaveProgress({ sent: 0, total: rendered.result.blob.size })
    try {
      const { record } = await saveServerExport({
        projectId,
        fileName,
        outcome: rendered,
        projectRevision,
        clone: cloneQuality?.clone ? { jobId: cloneQuality.clone.jobId } : undefined,
        exportId: session.current?.exportId,
        signal: controller.signal,
        onExportId: (exportId) => { session.current = { projectId, exportId } },
        onProgress: (p) => { if (p.kind === 'upload') setSaveProgress({ sent: p.sent, total: p.total }) },
      })
      session.current = null
      scratch.current = null
      setSaved(record); setStage('done'); setExports(previous => [record, ...previous.filter(entry => entry.id !== record.id)])
      announce(`Xuất xong: ${record.fileName}`)
      toast.success(`Xuất xong: ${record.fileName}`, { action: { label: 'Mở', onClick: () => useExportDialogState.getState().show() } })
    } catch (reason) {
      const active = session.current
      if (active) {
        try { await storage.abortExport(active.projectId, active.exportId); session.current = null }
        catch { setError('Chưa dọn được phiên lưu trên server. Thử Lưu lại khi kết nối trở lại.') }
      }
      if (controller.signal.aborted) {
        setStage('cancelled'); announce('Đã huỷ xuất')
      } else fail(reason)
    }
  }

  const start = async () => {
    if (abort.current) return
    if (!project || !composition || !sequence || durationFrames <= 0 || stage === 'rendering' || stage === 'saving') return
    const controller = new AbortController(); abort.current = controller
    const startedAt = Date.now()
    const renderRevision = getProjectRevision(project.id)
    setError(''); setErrorDetail(''); setSaved(null); setOutcome(null); setProgress(null); setStage('rendering'); setElapsedSeconds(0)
    try {
      const result = await runServerExport({
        project: { id: project.id, name: project.name },
        revision: renderRevision,
        composition,
        settings,
        fps: sequence.fps,
        durationFrames,
        clone: cloneQuality?.clone
          ? { jobId: cloneQuality.clone.jobId, canExport: cloneQuality.canExport, gateReason: cloneQuality.gateReason }
          : undefined,
        brokenMediaIds,
        signal: controller.signal,
        onExportId: (exportId) => { session.current = { projectId: project.id, exportId } },
        onPreflight: (checked) => setPreflight(checked),
        onRendered: (rendered) => {
          scratch.current = { outcome: rendered, projectId: project.id, fileName: `${project.name.replace(/[\\/:*?"<>|]/g, '-').trim() || 'video'}.mp4`, projectRevision: renderRevision }
          setOutcome(rendered)
        },
        onProgress: (p) => {
          if (p.kind === 'render') { setProgress(p.progress); setElapsedSeconds((Date.now() - startedAt) / 1000) }
          else {
            setStage('saving')
            if (p.kind === 'upload') setSaveProgress({ sent: p.sent, total: p.total })
          }
        },
      })
      // The runner already uploaded and committed; mirror saveScratch's done state.
      session.current = null
      scratch.current = null
      setSaved(result.record); setStage('done'); setExports(previous => [result.record, ...previous.filter(entry => entry.id !== result.record.id)])
      announce(`Xuất xong: ${result.record.fileName}`)
      toast.success(`Xuất xong: ${result.record.fileName}`, { action: { label: 'Mở', onClick: () => useExportDialogState.getState().show() } })
    } catch (reason) {
      const active = session.current
      if (active && !scratch.current) {
        await getEditorStorage().abortExport(active.projectId, active.exportId).catch(() => undefined)
        session.current = null
      }
      if (controller.signal.aborted) {
        await releaseTemporaryExportOutput(scratch.current?.outcome.result); scratch.current = null
        setOutcome(null)
        setStage('cancelled'); announce('Đã huỷ xuất')
      } else fail(reason)
    } finally { abort.current = null }
  }

  const retrySave = async () => {
    if (abort.current) return
    if (!scratch.current) return
    const controller = new AbortController(); abort.current = controller; setError('')
    try {
      if (session.current) { await getEditorStorage().abortExport(session.current.projectId, session.current.exportId); session.current = null }
      await saveScratch(controller)
    } catch (reason) { fail(reason) }
    finally { abort.current = null }
  }
  const reset = async () => {
    if (abort.current) return
    if (session.current) {
      try { await getEditorStorage().abortExport(session.current.projectId, session.current.exportId); session.current = null }
      catch (reason) { fail(reason); return }
    }
    if (scratch.current) await releaseTemporaryExportOutput(scratch.current.outcome.result)
    scratch.current = null; setStage('settings'); setError(''); setErrorDetail(''); setOutcome(null); setSaved(null)
    useExportDialogState.getState().show()
  }
  const deleteExport = async (record: ServerExportRecord) => {
    if (!project) return
    setDeleting(record.id)
    try {
      await getEditorStorage().deleteExport(project.id, record.id)
      setExports(previous => previous.filter(entry => entry.id !== record.id))
      if (saved?.id === record.id) { setSaved(null); setOutcome(null); setStage('settings') }
      announce(`Đã xoá bản xuất ${record.fileName}`)
    }
    catch (reason) { setListError(exportError(reason)) }
    finally { setDeleting(null) }
  }
  return { project, open, sequence, preset, setPreset, quality, setQuality, whole, setWhole, settings, stage, preflight, preflightError, donorWarnings, validRange, durationFrames, progress, saveProgress, elapsedSeconds, error, errorDetail, outcome, saved, exports, listError, listLoading, deleting, hasScratch: outcome !== null && stage !== 'done' && stage !== 'settings', start, retrySave, reset, refreshExports, deleteExport, cancel: () => abort.current?.abort(), busy: stage === 'rendering' || stage === 'saving' }
}
