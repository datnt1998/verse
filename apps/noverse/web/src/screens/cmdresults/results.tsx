// Bộ điều khiển kết quả lệnh trong Workspace (14-jobs-dock §2): gửi POST …/command, đọc
// `result` (G13 + GAP-27/29) để mở bảng /model, /diag hoặc thêm thẻ kết quả vào nhật ký;
// thẻ việc nền (export/sync/simulate) đổi Run → Done theo sự kiện SSE `job` của truyện.
import { useCallback, useEffect, useMemo, useRef, useSyncExternalStore } from 'react'
import { ApiError, postCommand, runSync } from '../../api/client'
import type { CommandOutcome, ExportConflict, JobEvent, SyncChanged } from '../../api/types'
import { useSession } from '../../api/session'
import { useStoryTitle } from '../reader/useReaderData'
import { setDialog } from '../../router'
import { toast } from '@noverse/ui'
import {
  ExportDoneCard, ExportExistsCard, ExportRunCard, JobFailedCard, SimulateDoneCard, SimulateRunCard,
  SyncCheckCard, SyncDoneCard, SyncRunCard,
} from './cards'
import './cmdresults.css'
import { exportPathArg, withExportFlag } from './logic'
import { CmdChip, errorText } from './parts'

type TrackedKind = 'export' | 'sync' | 'simulate'
const TRACKED: readonly string[] = ['export', 'sync', 'simulate']

export type CmdItem = { id: number; command: string } & (
  | { kind: 'job'; jobKind: TrackedKind; job: string; path: string }
  | { kind: 'exportExists'; conflict: ExportConflict; busy: 'replace' | 'keep' | null }
  | { kind: 'syncCheck'; changed: SyncChanged[]; busy: boolean }
  | { kind: 'panel'; panel: 'model' | 'diag' }
)

let seq = 0

// Giữ thẻ khi chuyển Workspace ↔ Nhật ký; SSE vẫn là nguồn tiến trình/kết quả.
const commandItems = new Map<string, CmdItem[]>()
const commandJobSnapshots = new Map<string, Map<string, JobEvent>>()
const itemListeners = new Set<() => void>()
const EMPTY_ITEMS: CmdItem[] = []

// Result history is scoped to the current tab, user and story.

/** Reload keeps the same cards; all progress still comes from worker job events. */
export function readCommandResults(key: string): { items: CmdItem[]; jobs: [string, JobEvent][] } {
  try {
    const stored = JSON.parse(sessionStorage.getItem(`ainovel.commandResults.${key}`) ?? 'null')
    if (!stored || !Array.isArray(stored.items) || !Array.isArray(stored.jobs)) return { items: [], jobs: [] }
    return {
      items: stored.items.filter((item: CmdItem) => item && Number.isSafeInteger(item.id) && typeof item.command === 'string' && ['job', 'exportExists', 'syncCheck'].includes(item.kind))
        .map((item: CmdItem) => item.kind === 'exportExists' ? { ...item, busy: null } : item.kind === 'syncCheck' ? { ...item, busy: false } : item),
      jobs: stored.jobs.filter((entry: [string, JobEvent]) => Array.isArray(entry) && typeof entry[0] === 'string' && entry[1] && typeof entry[1] === 'object'),
    }
  } catch { return { items: [], jobs: [] } }
}

export function writeCommandResults(key: string, items: CmdItem[], jobs: Map<string, JobEvent>) {
  try {
    sessionStorage.setItem(`ainovel.commandResults.${key}`, JSON.stringify({
      items: items.filter((item) => item.kind !== 'panel'),
      jobs: [...jobs].filter(([id]) => items.some((item) => item.kind === 'job' && item.job === id)),
    }))
  } catch { /* Storage unavailable: keep the in-memory cards. */ }
}

function itemsForKey(key: string) {
  if (!commandItems.has(key)) {
    const stored = readCommandResults(key)
    stored.items.forEach((item) => { seq = Math.max(seq, item.id) })
    commandItems.set(key, stored.items)
    commandJobSnapshots.set(key, new Map(stored.jobs))
  }
  return commandItems.get(key) ?? EMPTY_ITEMS
}
const subscribeItems = (listener: () => void) => {
  itemListeners.add(listener)
  return () => { itemListeners.delete(listener) }
}

/** Thẻ (nếu có) cho một kết quả lệnh; null = lệnh không có thẻ riêng (vd /import do JobProgress lo). */
export function itemFor(command: string, outcome: CommandOutcome | undefined): CmdItem | null {
  if (!outcome) return null
  const id = ++seq
  switch (outcome.kind) {
    case 'job':
      return TRACKED.includes(outcome.jobKind)
        ? { id, command, kind: 'job', jobKind: outcome.jobKind as TrackedKind, job: outcome.job, path: outcome.path || exportPathArg(command) }
        : null
    case 'exportExists':
      return { id, command, kind: 'exportExists', conflict: outcome.exists, busy: null }
    case 'syncCheck':
      return { id, command, kind: 'syncCheck', changed: outcome.changed, busy: false }
    case 'panel':
      return { id, command, kind: 'panel', panel: outcome.panel }
    default:
      return null
  }
}

/** Ánh xạ kết quả `panel` sang ?dlg= (bảng sống ở AppDialogs). */
export function panelDialog(outcome: CommandOutcome | undefined): string | null {
  if (outcome?.kind !== 'panel') return null
  if (outcome.panel === 'diag') return 'diag'
  return outcome.role ? `model:${outcome.role}` : 'model'
}

/** 409 `/export` mang `result: {kind:'exportExists'}` — chưa ghi gì, hiện thẻ so sánh. */
function conflictOutcome(reason: unknown): CommandOutcome | undefined {
  if (!(reason instanceof ApiError) || reason.status !== 409) return undefined
  const result = (reason.payload as { result?: CommandOutcome } | undefined)?.result
  return result?.kind === 'exportExists' ? result : undefined
}

export type CommandResultsCtl = {
  items: CmdItem[]
  /** id việc nền đã có thẻ riêng — JobProgress bỏ qua để không hiện hai lần */
  trackedJobs: ReadonlySet<string>
  /** gửi một dòng lệnh; trả `message` để Workspace ghi vào nhật ký. Lỗi thật vẫn ném ApiError. */
  run: (line: string) => Promise<{ message?: string }>
  resolveExists: (id: number, mode: 'replace' | 'keep') => Promise<void>
  dismiss: (id: number) => void
  acceptSync: (id: number) => Promise<void>
  retry: (id: number) => Promise<void>
}

export function useCommandResults(project: string, story: string, options?: {
  onPanel?: (panel: 'model' | 'diag', role?: string) => void
}): CommandResultsCtl {
  const { user } = useSession()
  const key = JSON.stringify([user?.username, project, story])
  const items = useSyncExternalStore(subscribeItems, () => itemsForKey(key))
  const setItems = useCallback((update: (previous: CmdItem[]) => CmdItem[]) => {
    commandItems.set(key, update(itemsForKey(key)))
    writeCommandResults(key, commandItems.get(key) ?? EMPTY_ITEMS, commandJobSnapshots.get(key) ?? new Map())
    itemListeners.forEach((listener) => listener())
  }, [key])
  const itemsRef = useRef(items)
  itemsRef.current = items

  const patch = useCallback((id: number, next: (item: CmdItem) => CmdItem | null) => {
    setItems((previous) => previous.flatMap((item) => {
      if (item.id !== id) return [item]
      const replaced = next(item)
      return replaced ? [replaced] : []
    }))
  }, [setItems])

  const run = useCallback(async (line: string) => {
    let outcome: CommandOutcome | undefined
    let message: string | undefined
    try {
      const response = await postCommand(project, story, line)
      outcome = response.result
      message = response.message
    } catch (reason) {
      outcome = conflictOutcome(reason)
      if (!outcome) throw reason
    }
    const dialog = panelDialog(outcome)
    if (outcome?.kind === 'panel' && options?.onPanel) options.onPanel(outcome.panel, outcome.role)
    else if (dialog) setDialog(dialog)
    const item = itemFor(line, outcome)
    if (item) setItems((previous) => [...previous, item])
    return { message }
  }, [project, story, setItems, options?.onPanel])

  const resolveExists = useCallback(async (id: number, mode: 'replace' | 'keep') => {
    const item = itemsRef.current.find((entry) => entry.id === id)
    if (!item || item.kind !== 'exportExists' || item.busy) return
    patch(id, (entry) => (entry.kind === 'exportExists' ? { ...entry, busy: mode } : entry))
    const line = withExportFlag(item.command, mode)
    try {
      const response = await postCommand(project, story, line)
      patch(id, () => itemFor(line, response.result))
    } catch (reason) {
      const again = conflictOutcome(reason)
      if (again) { patch(id, () => itemFor(line, again)); return }
      patch(id, (entry) => (entry.kind === 'exportExists' ? { ...entry, busy: null } : entry))
      toast.err(errorText(reason, 'Không xuất được'))
    }
  }, [patch, project, story])

  const acceptSync = useCallback(async (id: number) => {
    const item = itemsRef.current.find((entry) => entry.id === id)
    if (!item || item.kind !== 'syncCheck' || item.busy) return
    patch(id, (entry) => (entry.kind === 'syncCheck' ? { ...entry, busy: true } : entry))
    try {
      const { job } = await runSync(project, story)
      patch(id, (entry) => ({ id: entry.id, command: '/sync', kind: 'job', jobKind: 'sync', job, path: '' }))
    } catch (reason) {
      patch(id, (entry) => (entry.kind === 'syncCheck' ? { ...entry, busy: false } : entry))
      toast.err(errorText(reason, 'Không tiếp nhận được'))
    }
  }, [patch, project, story])

  const dismiss = useCallback((id: number) => patch(id, () => null), [patch])

  const retry = useCallback(async (id: number) => {
    const item = itemsRef.current.find((entry) => entry.id === id)
    if (!item) return
    try {
      const response = await postCommand(project, story, item.command)
      patch(id, () => itemFor(item.command, response.result))
    } catch (reason) {
      const conflict = conflictOutcome(reason)
      if (conflict) patch(id, () => itemFor(item.command, conflict))
      else toast.err(errorText(reason, 'Không chạy lại được lệnh'))
    }
  }, [patch, project, story])

  const trackedJobs = useMemo(
    () => new Set(items.flatMap((item) => (item.kind === 'job' ? [item.job] : []))),
    [items],
  )

  return { items, trackedJobs, run, resolveExists, dismiss, acceptSync, retry }
}

function JobItem({ item, job, project, story, bookTitle, onRetry, onRun }: {
  item: Extract<CmdItem, { kind: 'job' }>; job: JobEvent | undefined; project: string; story: string; bookTitle: string
  onRetry: () => void; onRun: (line: string) => void
}) {
  if (job?.done && job.error) return <JobFailedCard kind={item.jobKind} error={job.error} onRetry={onRetry} />
  const done = !!job?.done
  switch (item.jobKind) {
    case 'export':
      return done && job?.export
        ? <ExportDoneCard result={job.export} command={item.command} onRun={onRun} project={project} story={story} bookTitle={bookTitle} />
        : <ExportRunCard job={job} path={job?.export?.path ?? item.path} bookTitle={bookTitle} />
    case 'sync':
      return done && job?.sync
        ? <SyncDoneCard result={job.sync} project={project} story={story} />
        : <SyncRunCard job={job} />
    case 'simulate':
      return done && job?.simulate
        ? <SimulateDoneCard result={job.simulate} project={project} story={story} />
        : <SimulateRunCard job={job} />
  }
}

/** Danh sách thẻ kết quả; mỗi thẻ có chip lệnh vừa gửi phía trên (CmdResults.dc.html). */
export function CommandResults({ ctl, jobs, project, story }: {
  ctl: CommandResultsCtl; jobs: Map<string, JobEvent>; project: string; story: string
}) {
  const { user } = useSession()
  const bookTitle = useStoryTitle(project, story)
  const key = JSON.stringify([user?.username, project, story])
  const remembered = commandJobSnapshots.get(key)
  useEffect(() => {
    const next = new Map(commandJobSnapshots.get(key))
    jobs.forEach((job, id) => next.set(id, job))
    commandJobSnapshots.set(key, next)
    writeCommandResults(key, ctl.items, next)
  }, [key, jobs, ctl.items])
  if (ctl.items.length === 0) return null
  return (
    <div aria-live="polite" style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
      {ctl.items.map((item) => (
        <div key={item.id} style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
          <CmdChip command={item.command} />
          {item.kind === 'job' && (
            <JobItem item={item} job={jobs.get(item.job) ?? remembered?.get(item.job)} project={project} story={story} bookTitle={bookTitle}
              onRetry={() => void ctl.retry(item.id)} onRun={(line) => void ctl.run(line)} />
          )}
          {item.kind === 'exportExists' && (
            <ExportExistsCard conflict={item.conflict} bookTitle={bookTitle} busy={item.busy}
              onReplace={() => ctl.resolveExists(item.id, 'replace')}
              onKeep={() => ctl.resolveExists(item.id, 'keep')}
              onCancel={() => ctl.dismiss(item.id)} />
          )}
          {item.kind === 'syncCheck' && (
            <SyncCheckCard changed={item.changed} project={project} story={story} busy={item.busy}
              onAccept={() => void ctl.acceptSync(item.id)} />
          )}
        </div>
      ))}
    </div>
  )
}
