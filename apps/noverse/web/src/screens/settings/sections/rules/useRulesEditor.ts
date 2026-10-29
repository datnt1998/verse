// Bộ điều khiển Quy tắc viết dùng chung cho RulesEditor và MobileRules.
// tải quy tắc hiệu lực, giữ bản chỉnh sửa chưa Áp dụng, thêm quy tắc qua /api/rules/understand, Áp dụng.
import { useCallback, useEffect, useMemo, useRef, useState, useSyncExternalStore } from 'react'
import {
  ApiError, applyRules, canManageProject, canWriteStory, dropRuntimeRule, getRulesEffective,
  keepRuntimeRule, putRuleFile, understandRule,
} from '../../../../api/client'
import { useSession } from '../../../../api/session'
import type { ConfigScope, RulesEffective, RulesTiming } from '../../../../api/types'
import { invalidate, keys, useResource, type Resource } from '../../../../data'
import {
  appliedToast, changeCount, degradedChips, emptyEdits, entryFromItem, entryFromRuntime, listOf, moveTarget, nextKey,
  oneLine, RAW_SAVED_CHIP, RULES_FILE, RULES_RETRY_ATTEMPTS, RULES_RETRY_INTERVAL_MS, serializeRuleLines, summarize,
  visibleRuntime,
  type Baseline, type EditState, type RuleEntry, type RuleRetry, type SummaryRow,
} from './logic'

const ackKey = 'ainovel.rules.ack'
const readAcks = (): string[] => { try { return JSON.parse(localStorage.getItem(ackKey) ?? '[]') as string[] } catch { return [] } }
const writeAck = (text: string) => { try { localStorage.setItem(ackKey, JSON.stringify([...new Set([...readAcks(), text])].slice(-200))) } catch { /* bộ nhớ trình duyệt bị chặn: khung cam hiện lại */ } }

/** Khoá tài nguyên của bản “hiệu lực”: nằm dưới `keys.rules(...)` nên sự kiện `story.changed {rules}` làm mới nó. */
export const effectiveKey = (scope: ConfigScope, project: string, story: string) =>
  [...keys.rules(scope, scope === 'global' ? undefined : project || undefined, scope === 'story' ? story || undefined : undefined), 'effective'] as const

const ready = (scope: ConfigScope, project: string, story: string) => scope === 'global' || (!!project && (scope === 'project' || !!story))

// ─── Số quy tắc “sống” cho menu trái (đổi ngay khi thêm/xoá, trước khi Áp dụng) ───

const live = new Map<string, number>()
const liveListeners = new Set<() => void>()
const setLive = (key: string, value: number | undefined) => {
  if (value === undefined) live.delete(key); else live.set(key, value)
  liveListeners.forEach((fn) => fn())
}
const subscribeLive = (fn: () => void) => { liveListeners.add(fn); return () => { liveListeners.delete(fn) } }
const liveId = (scope: ConfigScope, project: string, story: string) => effectiveKey(scope, project, story).join('/')

/** Số quy tắc đang hiệu lực ở cấp đó (kể cả luật học từ góp ý) — chữ số cạnh “Quy tắc viết” ở menu trái. */
export function useRulesCount(scope: ConfigScope, project?: string, story?: string): number | undefined {
  const p = project ?? '', s = story ?? ''
  const resource = useResource<RulesEffective>(effectiveKey(scope, p, s), ({ signal }) => getRulesEffective(scope, p || undefined, s || undefined, signal), { enabled: ready(scope, p, s) })
  const id = liveId(scope, p, s)
  const liveCount = useSyncExternalStore(subscribeLive, () => live.get(id), () => undefined)
  if (liveCount !== undefined) return liveCount
  return resource.data ? resource.data.items.filter((item) => item.scope === scope).length + (scope === 'story' ? resource.data.runtime?.length ?? 0 : 0) : undefined
}

/** Kết quả chuẩn hoá một quy tắc (hoặc `busy` khi máy chủ lỗi 5xx → chờ thử lại). */
type Understood = Pick<RuleEntry, 'chips' | 'uncertain' | 'note' | 'options' | 'status'> & { error?: string }

/** Giao diện của bộ điều khiển — màn hình máy tính và điện thoại cùng dùng. */
export interface RulesEditor {
  scope: ConfigScope
  canEdit: boolean
  enabled: boolean
  resource: Resource<RulesEffective>
  /** các danh sách theo cấp đang hiển thị (đã gồm chỉnh sửa chưa áp dụng) */
  lists: Record<ConfigScope, RuleEntry[]>
  runtime: RuleEntry[]
  base: Baseline
  edits: EditState
  /** số thay đổi chưa áp dụng (0 = sạch) */
  count: number
  summary: SummaryRow[]
  timing: RulesTiming | undefined
  /** nội dung đang được đọc (thẻ “Noverse đang đọc quy tắc…”), rỗng khi không có */
  thinking: string
  /** Máy chủ 500 khi đọc quy tắc: đang thử lại (Settings.dc.html › rules=retry) */
  retry: RuleRetry | null
  /** “Lưu nguyên văn ngay” — lưu nguyên văn, đánh dấu sẽ thử hiểu lại khi máy chủ ổn */
  saveRaw: () => void
  /** “Huỷ” thẻ chờ thử lại */
  cancelRetry: () => void
  /** key của thẻ đang chuẩn hoá lại sau khi chọn một cách hiểu */
  resolving: string
  recheck: boolean
  setRecheck: (on: boolean) => void
  /** “Sửa”: chữ cần đưa lên ô thêm (n đổi mỗi lần để effect nhận ra) */
  draftSeed: { text: string; n: number } | null
  /** Thêm: trả lời cảnh báo (nếu chuẩn hoá lỗi, thẻ vẫn được lưu như Lời dặn) */
  add: (raw: string) => Promise<string | undefined>
  remove: (level: ConfigScope, key: string) => RuleEntry | undefined
  edit: (level: ConfigScope, key: string) => void
  move: (level: ConfigScope, key: string) => ConfigScope | undefined
  keepLearned: (entry: RuleEntry) => void
  dropLearned: (entry: RuleEntry) => void
  resolve: (level: ConfigScope, key: string, option: string | null) => Promise<string | undefined>
  cancel: () => void
  apply: (object: string) => Promise<string>
  refresh: () => void
}

export function useRulesEditor({ scope, project, story, recheckDefault = true }: { scope: ConfigScope; project: string; story: string; recheckDefault?: boolean }): RulesEditor {
  const { user } = useSession()
  const enabled = ready(scope, project, story)
  const canEdit = !!user && (scope === 'global' ? user.role === 'admin' : scope === 'project' ? canManageProject(user, project) : canWriteStory(user, `${project}/${story}`))
  const resource = useResource<RulesEffective>(effectiveKey(scope, project, story), ({ signal }) => getRulesEffective(scope, project || undefined, story || undefined, signal), { enabled })

  const base = useMemo<Baseline>(() => {
    const items = resource.data?.items ?? []
    const acks = readAcks()
    const of = (level: ConfigScope) => items.filter((item) => item.scope === level).map((item, index) => ({ ...entryFromItem(item, `${level}:${index}`), acked: acks.includes(item.text) }))
    return { story: of('story'), project: of('project'), global: of('global'), runtime: (resource.data?.runtime ?? []).map(entryFromRuntime) }
  }, [resource.data])

  const [edits, setEdits] = useState<EditState>(emptyEdits)
  const [draftSeed, setDraftSeed] = useState<{ text: string; n: number } | null>(null)
  const [thinking, setThinking] = useState('')
  const [resolving, setResolving] = useState('')
  const [retry, setRetry] = useState<RuleRetry | null>(null)
  const [recheck, setRecheck] = useState(recheckDefault)
  const abort = useRef<AbortController | null>(null)
  const retryTimer = useRef<number | undefined>(undefined)
  const identity = `${scope}/${project}/${story}`
  const latestEdits = useRef(edits)
  latestEdits.current = edits

  const stopRetryTimer = () => {
    if (retryTimer.current !== undefined) { window.clearTimeout(retryTimer.current); retryTimer.current = undefined }
  }
  useEffect(() => { setEdits(emptyEdits()); setThinking(''); setResolving(''); setRetry(null); abort.current?.abort(); stopRetryTimer() }, [identity])
  useEffect(() => () => { abort.current?.abort(); stopRetryTimer() }, [])

  const count = changeCount(base, edits)
  const lists = useMemo(() => ({ story: listOf(base, edits, 'story'), project: listOf(base, edits, 'project'), global: listOf(base, edits, 'global') }), [base, edits])
  const runtime = useMemo(() => visibleRuntime(base, edits), [base, edits])

  // Số hiệu lực ở menu trái.
  const effectiveEntries = useMemo(() => {
    const own = scope === 'story' ? [...lists.story, ...runtime, ...lists.project, ...lists.global] : scope === 'project' ? [...lists.project, ...lists.global] : lists.global
    return own
  }, [scope, lists, runtime])
  useEffect(() => {
    if (!resource.data) return
    const id = liveId(scope, project, story)
    setLive(id, lists[scope].length + (scope === 'story' ? runtime.length : 0))
    return () => setLive(id, undefined)
  }, [scope, project, story, lists[scope].length, runtime.length, !!resource.data])

  const ownLevel = scope
  const update = (fn: (edits: EditState) => EditState) => setEdits((old) => fn(old))
  const setOwn = (level: ConfigScope, next: RuleEntry[], opsDelta = 1) => update((old) => ({ ...old, lists: { ...old.lists, [level]: next }, ops: old.ops + opsDelta }))
  const current = (level: ConfigScope) => listOf(base, latestEdits.current, level)

  /** Lỗi máy chủ (5xx) khi đọc quy tắc: thử lại (Settings.dc.html › rules=retry), không hạ cấp ngay. */
  const understand = async (text: string, signal: AbortSignal): Promise<Understood | { busy: true; message: string }> => {
    try {
      const out = await understandRule(text, scope, project || undefined, story || undefined, signal)
      const reasons = out.uncertain ?? []
      return { chips: out.chips ?? [], uncertain: reasons.length > 0, note: out.uncertainNote || reasons[0] || '', options: out.options ?? [], status: out.status }
    } catch (error) {
      if (signal.aborted) throw error
      const message = error instanceof ApiError ? error.message : 'Chưa đọc được quy tắc này'
      // Máy chủ đang bận: chờ thử lại 3 lần trong ~20 giây rồi lưu nguyên văn.
      if (error instanceof ApiError && error.status >= 500) return { busy: true, message }
      return { chips: degradedChips(text), uncertain: false, note: '', options: [], status: 'degraded', error: message }
    }
  }

  /** Thêm thẻ quy tắc đã chuẩn hoá vào đầu danh sách cấp đang sửa. */
  const addEntry = (text: string, result: Understood) => {
    const entry: RuleEntry = { key: nextKey('new'), text, chips: result.chips, uncertain: result.uncertain, note: result.note, options: result.options, status: result.status, fresh: true }
    setOwn(ownLevel, [entry, ...current(ownLevel)])
  }

  /** “Lưu nguyên văn ngay” / hết 3 lần thử: lưu nguyên văn, đánh dấu sẽ thử hiểu lại khi máy chủ ổn. */
  const saveRaw = (text?: string) => {
    const target = text ?? retry?.text
    stopRetryTimer()
    setRetry(null)
    if (!target) return
    addEntry(target, { chips: [RAW_SAVED_CHIP], uncertain: false, note: '', options: [], status: 'ready' })
  }

  const cancelRetry = () => { stopRetryTimer(); setRetry(null) }

  /** Thử lại lần `attempt` (2-based): thành công → thẻ thường; hết lượt → lưu nguyên văn. */
  const runRetry = async (text: string, attempt: number) => {
    const controller = new AbortController()
    abort.current = controller
    let result: Understood | { busy: true; message: string }
    try { result = await understand(text, controller.signal) } catch { return }
    if (!('busy' in result)) { stopRetryTimer(); setRetry(null); addEntry(text, result); return }
    if (attempt >= RULES_RETRY_ATTEMPTS) { saveRaw(text); return }
    setRetry({ text, attempt: attempt + 1, total: RULES_RETRY_ATTEMPTS })
    stopRetryTimer()
    retryTimer.current = window.setTimeout(() => { void runRetry(text, attempt + 1) }, RULES_RETRY_INTERVAL_MS)
  }

  const add = async (raw: string): Promise<string | undefined> => {
    const flat = oneLine(raw)
    if (!flat || thinking || retry || !canEdit) return undefined
    const text = /[.!?]$/.test(flat) ? flat : `${flat}.`
    const controller = new AbortController()
    abort.current = controller
    setThinking(text)
    try {
      const result = await understand(text, controller.signal)
      if ('busy' in result) {
        setRetry({ text, attempt: 2, total: RULES_RETRY_ATTEMPTS })
        retryTimer.current = window.setTimeout(() => { void runRetry(text, 2) }, RULES_RETRY_INTERVAL_MS)
        return result.message
      }
      addEntry(text, result)
      return result.error
    } catch { return undefined } finally { if (abort.current === controller) setThinking('') }
  }

  const remove = (level: ConfigScope, key: string) => {
    if (level !== scope || !canEdit) return undefined
    const entry = current(level).find((item) => item.key === key)
    update((old) => {
      const kept = entry?.fromRuntime && old.kept.includes(entry.fromRuntime)
      return {
        ...old, lists: { ...old.lists, [level]: listOf(base, old, level).filter((item) => item.key !== key) }, ops: old.ops + 1,
        // luật Trọng tài đã chuyển nhưng bị xoá/sửa: bản gốc cũng bỏ, đừng ghi lại dòng cũ
        kept: kept ? old.kept.filter((id) => id !== entry!.fromRuntime) : old.kept, dropped: kept ? [...old.dropped, entry!.fromRuntime!] : old.dropped,
      }
    })
    return entry
  }

  /** Sửa: lấy thẻ ra khỏi danh sách, đưa chữ lên ô thêm (như artboard). */
  const edit = (level: ConfigScope, key: string) => {
    const entry = remove(level, key)
    if (entry) setDraftSeed({ text: entry.text, n: Date.now() })
  }

  const move = (level: ConfigScope, key: string) => {
    if (level !== scope || !canEdit) return undefined
    const target = moveTarget(level)
    const entry = current(level).find((item) => item.key === key)
    if (!target || !entry) return undefined
    update((old) => ({
      ...old, ops: old.ops + 1,
      lists: { ...old.lists, [level]: listOf(base, old, level).filter((item) => item.key !== key), [target]: [...listOf(base, old, target), { ...entry, fresh: true }] },
    }))
    return target
  }

  const keepLearned = (entry: RuleEntry) => {
    if (!entry.fromRuntime) return
    update((old) => ({
      ...old, ops: old.ops + 1, kept: [...old.kept, entry.fromRuntime!],
      lists: { ...old.lists, story: [...listOf(base, old, 'story'), { ...entry, fresh: true }] },
    }))
  }
  const dropLearned = (entry: RuleEntry) => { if (entry.fromRuntime) update((old) => ({ ...old, ops: old.ops + 1, dropped: [...old.dropped, entry.fromRuntime!] })) }

  /** Khung cam: chọn một cách hiểu → viết lại quy tắc và chuẩn hoá lại; `null` = “Giữ như lời dặn”. */
  const resolve = async (level: ConfigScope, key: string, option: string | null): Promise<string | undefined> => {
    if (level !== scope || !canEdit) return undefined
    const entry = current(level).find((item) => item.key === key)
    if (!entry || resolving) return undefined
    if (option === null) {
      writeAck(entry.text)
      setOwn(level, current(level).map((item) => item.key === key ? { ...item, acked: true } : item), 0)
      return undefined
    }
    const controller = new AbortController()
    abort.current = controller
    setResolving(key)
    try {
      const result = await understand(option, controller.signal)
      if ('busy' in result) return result.message
      setOwn(level, current(level).map((item) => item.key === key ? { ...item, text: option, chips: result.chips, uncertain: result.uncertain, note: result.note, options: result.options, status: result.status, acked: false } : item))
      return result.error
    } catch { return undefined } finally { if (abort.current === controller) setResolving('') }
  }

  const cancel = () => { abort.current?.abort(); stopRetryTimer(); setRetry(null); setEdits(emptyEdits()); setThinking(''); setResolving('') }

  /** Ghi các tệp đã sửa → Giữ/Bỏ luật Trọng tài → dựng lại bản chốt của truyện. Trả câu toast; lỗi thì ném để nơi gọi báo. */
  const apply = async (object: string): Promise<string> => {
    const state = latestEdits.current
    for (const level of Object.keys(state.lists) as ConfigScope[]) {
      const entries = state.lists[level] ?? []
      const same = entries.length === base[level].length && entries.every((item, i) => item.text === base[level][i].text)
      if (same) continue
      await putRuleFile(level, RULES_FILE, serializeRuleLines(entries.map((item) => item.text)), level === 'global' ? undefined : project, level === 'story' ? story : undefined)
    }
    // Giữ: dòng đã nằm trong tệp truyện (ở trên) nên máy chủ chỉ gỡ khỏi danh sách runtime. Bỏ: 404 = đã mất rồi.
    for (const id of state.kept) await keepRuntimeRule(project, story, id)
    for (const id of state.dropped) { try { await dropRuntimeRule(project, story, id) } catch (error) { if (!(error instanceof ApiError && error.status === 404)) throw error } }
    let effectiveFrom: number | undefined
    if (scope === 'story') effectiveFrom = (await applyRules(project, story, recheck)).effectiveFrom
    await invalidate(['rules'])
    setEdits(emptyEdits())
    return appliedToast(scope, object, effectiveFrom, recheck, resource.data?.timing?.completed ?? 0)
  }

  const summary = useMemo(() => summarize(effectiveEntries), [effectiveEntries])
  const refresh = useCallback(() => { void resource.refresh() }, [resource.refresh])

  return {
    scope, canEdit, enabled, resource, base, lists, runtime, edits, count, summary,
    timing: resource.data?.timing, thinking, resolving, recheck, setRecheck, draftSeed,
    retry, saveRaw: () => saveRaw(), cancelRetry,
    add, remove, edit, move, keepLearned, dropLearned, resolve, cancel, apply, refresh,
  }
}
