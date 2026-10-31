// Tuỳ chỉnh (#/settings/:mục?scope=&project=&story=) — Settings.dc.html: menu trái + thanh “Áp dụng cho” +
// nội dung theo mục + thanh nổi đáy. Cấp truyện/dự án/mọi truyện theo 11-api-contract.md §6.
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  ApiError, ConflictError, canCreateStory, canManageProject, canWriteStory, getAudioSettings, getConfig, getConfigPresets, getSnapshot, getSystem,
  putAudioSettings, putConfig, putLibrary,
} from '../../api/client'
import { useLibrary } from '../../api/library'
import { useSession } from '../../api/session'
import { getLastStory } from '../../api/storage'
import type { AudioSettings, Config, ConfigPresets, ConfigResponse, ConfigScope, LibraryConfig, Snapshot, StorySummary } from '../../api/types'
import { invalidate, keys, setData, useAction, useConnection, useResource } from '../../data'
import { navigate, paths } from '../../router'
import { projectLabel, storyLabel } from '../../shell/labels'
import { helpTarget } from '../../commands/target'
import { WhatsNewDialog } from '../../shell/WhatsNew'
import { hasNewVersion, versionText } from '../../shell/version'
import { Icon } from '@noverse/ui'
import { ConflictBar } from '@noverse/ui'
import { StaleBanner } from '@noverse/ui'
import { toast } from '@noverse/ui'
import { useSaveShortcut } from '@noverse/ui'
import { ErrorCard, LoadingRows, ReadOnlyNotice, type FieldState } from './components'
import { configForCompare, configForSave, configsEqual, getPath, rebaseConfig, setPath, sourceOf, stableStringify } from './lib/config'
import { countChanges, hasOwn, inheritedFrom, ovrOf } from './lib/labels'
import { isSystemSection, parseSettingsTarget, resolveSection, type PageId, type SectionId } from './lib/target'
import { SettingsNav } from './nav'
import { ScopeBar } from './scopebar'
import { showsScope, staleScopeFallback, type OpenTarget, type ScopeTarget } from './scope/model'
import { SaveBar, dirtyText } from './savebar'
import { AdvancedSection } from './advanced'
import { AppearanceSection } from './sections/appearance'
import { BudgetSection } from './sections/budget'
import { ExportSection } from './sections/library'
import { InfoSection } from './sections/info'
import { NotifySection } from './sections/notify'
import { ProvidersSection } from './sections/providers'
import { RolesSection } from './sections/roles'
import { VoiceSection } from './sections/voice'
import type { PendingExtra, SettingsContext } from './types'
import { MembersSettings } from './members'
import { StorageSection } from './storage'
import './settings.css'
import './style.css'
import { PanelRefreshBar } from '../../ui/PanelRefreshBar'
import { useUnsavedGuard } from '../../ui/useUnsavedGuard'

const EMPTY_STORIES: StorySummary[] = []

/** Mục cần `GET /api/config` (các mục còn lại tự lấy dữ liệu hoặc lưu riêng). */
const CONFIG_SECTIONS: readonly PageId[] = ['models', 'budget', 'connect', 'notify', 'voice']

/** Khoá cấu hình thuộc từng mục — chấm tím ở menu khi cấp đang chọn có ghi đè. */
const DOT_KEYS: Partial<Record<SectionId, string[]>> = {
  models: ['provider', 'model', 'reasoning_effort', 'roles', 'advance_mode', 'ask_user'],
  budget: ['budget'],
}

function errText(error: unknown): string {
  return error instanceof ApiError || error instanceof Error ? error.message : 'Không tải được tuỳ chỉnh.'
}

/** Số thứ tự trong `folder` ("kich_ban/01-tra-xanh") → 1 khi không đọc được. */
function folderIndex(folder: string): number {
  const match = /(?:^|\/)(\d+)-/.exec(folder)
  return match ? Number(match[1]) : 1
}

/** Đường dẫn Tuỳ chỉnh giữ dự án/truyện đang xem (Gói R): `#/settings/<mục>?project=&story=`. */
function settingsHref(section: PageId | null, scope: ConfigScope, project: string, story: string): string {
  const query = new URLSearchParams()
  if (project) query.set('project', project)
  if (story) query.set('story', story)
  if (scope !== 'story') query.set('scope', scope)
  const qs = query.toString()
  return `${paths.settings(section ?? undefined)}${qs ? `?${qs}` : ''}`
}

export function SettingsScreen(props: { section: string | null; search?: URLSearchParams }) {
  const { user } = useSession()
  const target = parseSettingsTarget(props.section, props.search ?? new URLSearchParams())
  return resolveSection(target.section, user?.role === 'admin') === 'members' ? <MembersSettings /> : <GeneralSettingsScreen {...props} />
}

function GeneralSettingsScreen({ section, search }: { section: string | null; search?: URLSearchParams }) {
  const { user } = useSession()
  const admin = user?.role === 'admin'
  const library = useLibrary()
  const connection = useConnection()
  const target = useMemo(() => parseSettingsTarget(section, search ?? new URLSearchParams()), [section, search])
  const current = resolveSection(target.section, admin)
  const system = current !== 'voice' && isSystemSection(current)

  // ─── Ngữ cảnh: cấp + dự án + truyện (từ đường dẫn, rồi từ truyện đang mở ở Bàn viết) ─────────────
  // Gói R: “chỗ đang xem” tách khỏi truyện đang mở — thanh đường dẫn và Bàn viết không đổi khi xem nơi khác.
  const openRaw = useMemo(() => helpTarget() ?? (user ? getLastStory(user.username) : null), [user])
  const openSummary = openRaw ? library.storyOf(openRaw.project, openRaw.story) : null
  const open: OpenTarget = useMemo(
    () => (openRaw ? { ...openRaw, title: storyLabel(openSummary ?? { story: openRaw.story }) } : null),
    [openRaw, openSummary],
  )
  const [touched, setTouched] = useState(false)
  const [project, setProject] = useState(target.project || openRaw?.project || '')
  const [story, setStory] = useState(target.story || openRaw?.story || '')
  const [scope, setScope] = useState<ConfigScope>(current === 'info' ? 'project' : target.scope ?? (target.story || openRaw?.story ? 'story' : target.project || openRaw?.project ? 'project' : 'global'))
  const appliedTarget = useRef(`${target.scope ?? ''}/${target.project}/${target.story}`)
  useEffect(() => {
    // Liên kết mới trong lúc đã mở Tuỳ chỉnh (vd nút “Sửa” ở màn Dự án) đổi cấp + ngữ cảnh theo đường dẫn.
    const key = `${target.scope ?? ''}/${target.project}/${target.story}`
    if (key === appliedTarget.current) return
    appliedTarget.current = key
    if (target.scope) setScope(target.scope)
    if (target.project) setProject(target.project)
    if (target.story) setStory(target.story)
  }, [target])

  const projects = library.library?.projects ?? []
  const detail = project ? library.projectDetail(project) : null
  const stories = detail?.stories ?? EMPTY_STORIES
  const projectName = project ? projectLabel(projects.find((item) => item.id === project), project) : ''
  // Tên truyện hiển thị qua `storyLabel` (tên sách máy chủ trả), không dùng tiêu đề thô của story.json.
  const storySummary = stories.find((item) => item.story === story) ?? null
  const storyTitle = storySummary ? storyLabel(storySummary) : ''
  const sample = stories[0]
  const sampleStoryTitle = sample ? storyLabel(sample) : 'Truyện mới'
  const sampleStoryIndex = sample ? folderIndex(sample.folder) : 1

  useEffect(() => {
    if (project || projects.length === 0) return
    setProject((projects.find((item) => !item.legacy) ?? projects[0]).id)
  }, [project, projects])
  useEffect(() => {
    if (project) library.ensureProject(project)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [project])
  useEffect(() => {
    if (openRaw) library.ensureProject(openRaw.project)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [openRaw?.project])
  // Chưa có tham số trên đường dẫn và người dùng chưa chọn gì → mở đúng truyện đang ở Bàn viết.
  useEffect(() => {
    if (touched || target.project || target.story || !openRaw) return
    setProject(openRaw.project)
    setStory(openRaw.story)
    setScope('story')
  }, [touched, target, openRaw])
  // Gói R — ngữ cảnh trên đường dẫn (hoặc nhớ từ máy khác) có thể trỏ vào dự án/truyện
  // không có trên máy chủ này: về truyện đang mở, không thì dự án đầu; `touched` để đường
  // dẫn được ghi lại và giá trị hỏng bị bỏ.
  useEffect(() => {
    if (!library.library || !project) return
    const fixed = staleScopeFallback({ project, story }, {
      projects, open,
      storiesOf: (id) => library.projectDetail(id)?.stories ?? null,
    })
    if (!fixed) return
    setTouched(true)
    if (fixed.project !== project) setProject(fixed.project)
    if (fixed.story !== story) setStory(fixed.story)
  }, [library, project, story, projects, open])
  useEffect(() => {
    if (!project || !detail || (story && stories.some((item) => item.story === story))) return
    setStory(stories[0]?.story ?? '')
  }, [project, detail, story, stories])

  // Cấp bắt buộc có ngữ cảnh: không có truyện thì rơi về dự án, không có dự án thì rơi về mọi truyện.
  const effectiveScope: ConfigScope = current === 'info' && project ? 'project' : scope === 'story' && !story ? (project ? 'project' : 'global') : scope === 'project' && !project ? 'global' : scope
  // Gói R — mở lại trang thì giữ lựa chọn: ghi chỗ đang xem vào đường dẫn (thay, không thêm lịch sử).
  // Đường dẫn đã có tham số là nguồn sự thật: chỉ ghi đè khi người dùng vừa đổi chỗ xem, tránh vòng
  // route → state → route khi máy chủ thư viện còn đang tải.
  useEffect(() => {
    if (!user) return
    if (!touched && (target.project || target.story || target.scope)) return
    const next = `#${settingsHref(current, effectiveScope, project, story)}`
    if (location.hash === next) return
    navigate(next.slice(1), true)
  }, [user, current, effectiveScope, project, story, touched, target])
  const cfgScope: ConfigScope = system ? 'global' : effectiveScope
  const needsStory = cfgScope === 'story'
  const cfgProject = cfgScope === 'global' ? undefined : project
  const cfgStory = needsStory ? story : undefined
  const ready = !!user && (cfgScope === 'global' || (project !== '' && (!needsStory || story !== '')))

  // ─── Cấu hình ────────────────────────────────────────────────────────────────────────────
  const [response, setResponse] = useState<ConfigResponse | null>(null)
  const [draft, setDraft] = useState<Config>({})
  const [baseline, setBaseline] = useState<Config>({})
  const resourceKey = keys.config(cfgScope, cfgProject, cfgStory)
  const configResource = useResource(resourceKey, ({ signal }) => getConfig(cfgScope, cfgProject, cfgStory, signal), { enabled: ready })
  const configLoading = configResource.initialLoading
  const configError = configResource.error ? errText(configResource.error) : ''
  const [conflict, setConflict] = useState<ConflictError<ConfigResponse> | null>(null)
  const identity = `${cfgScope}/${project}/${story}`
  const lastIdentity = useRef(identity)
  useEffect(() => {
    const changed = lastIdentity.current !== identity
    if (changed) {
      lastIdentity.current = identity
      setResponse(null); setConflict(null)
    }
    const data = configResource.data
    if (!data) return
    // Cập nhật từ xa không bao giờ ghi đè biểu mẫu đang sửa dở.
    if (!changed && response && !configsEqual(draft, baseline)) return
    setResponse(data)
    const doc = configForCompare(data.target.config)
    setDraft(doc); setBaseline(doc)
  }, [configResource.data, identity])
  const refreshConfig = configResource.refresh
  const sources = configResource.data?.sources ?? response?.sources

  // autoOnLock thuộc truyện (audio/settings), không thuộc cấu hình TTS kế thừa.
  const audioKey = keys.story(project, story, 'audio', 'settings')
  const audioResource = useResource(audioKey, ({ signal }) => getAudioSettings(project, story, signal), { enabled: ready && needsStory && current === 'voice' })
  const [autoOnLockDraft, setAutoOnLockDraft] = useState<boolean | null>(null)
  const [audioConflict, setAudioConflict] = useState<ConflictError<AudioSettings> | null>(null)
  useEffect(() => { setAutoOnLockDraft(null); setAudioConflict(null) }, [identity])
  const audioDirty = needsStory && autoOnLockDraft !== null && autoOnLockDraft !== audioResource.data?.autoOnLock

  const presetsResource = useResource(['config-presets'], ({ signal }) => getConfigPresets(signal), { enabled: !!user, staleTime: 60000 })
  const presets: ConfigPresets | null = presetsResource.data ?? null

  // Ảnh chụp truyện: chi phí đã dùng (Ngân sách) · chương đang viết (hiệu lực từ chương X) · chế độ duyệt (Cách viết).
  const snapshotKey = keys.story(project, story, 'snapshot')
  const snapshotResource = useResource(snapshotKey, ({ signal }) => getSnapshot(project, story, signal), { enabled: !!user && !!project && !!story && effectiveScope === 'story' })
  const snapshot: Snapshot | null = effectiveScope === 'story' ? snapshotResource.data ?? null : null

  // ─── Thư viện (Xuất tự động · Nâng cao) ───────────────────────────────────────────────────
  const [libDraft, setLibDraft] = useState<LibraryConfig | null>(null)
  const [libBaseline, setLibBaseline] = useState<LibraryConfig | null>(null)
  useEffect(() => {
    if (!library.library) return
    setLibDraft(library.library.config)
    setLibBaseline(library.library.config)
  }, [library.library])

  const systemResource = useResource(keys.system(), ({ signal }) => getSystem(signal), { enabled: !!user, staleTime: 60000 })
  const canReveal = !!systemResource.data?.canReveal
  // D1 — banner “Có bản Noverse mới” (artboard `update=available`) ẩn được trong phiên; hộp “Có gì mới” mở từ
  // banner, chân thanh bên và nhóm “Phiên bản Noverse” ở Nâng cao.
  const version = systemResource.data ?? null
  const [updateHidden, setUpdateHidden] = useState(false)
  const [whatsNew, setWhatsNew] = useState(false)

  // ─── Quyền theo cấp ───────────────────────────────────────────────────────────────────────
  const canEditScope = useCallback((on: ConfigScope): boolean => {
    if (!user) return false
    if (on === 'global') return user.role === 'admin'
    if (!project) return false
    if (on === 'project') return canManageProject(user, project)
    return !!story && canWriteStory(user, `${project}/${story}`)
  }, [user, project, story])
  const canEdit = canEditScope(cfgScope)

  // ─── Phần việc của từng mục (Thông tin, Quy tắc…) ────────────────────────────────────────
  const extrasRef = useRef<Record<string, PendingExtra>>({})
  const [extraMeta, setExtraMeta] = useState<Record<string, { count: number; hint?: string }>>({})
  const registerExtra = useCallback((key: string, extra: PendingExtra | null) => {
    if (!extra || extra.count <= 0) {
      delete extrasRef.current[key]
      setExtraMeta((prev) => { if (!(key in prev)) return prev; const next = { ...prev }; delete next[key]; return next })
      return
    }
    extrasRef.current[key] = extra
    setExtraMeta((prev) => (prev[key]?.count === extra.count && prev[key]?.hint === extra.hint ? prev : { ...prev, [key]: { count: extra.count, hint: extra.hint } }))
  }, [])
  const extraCount = Object.values(extraMeta).reduce((sum, item) => sum + item.count, 0)

  // ─── Dirty + lưu ──────────────────────────────────────────────────────────────────────────
  const configDirty = !!response && !configsEqual(draft, baseline)
  const libDirty = !!libDraft && !!libBaseline && stableStringify(libDraft) !== stableStringify(libBaseline)
  const dirtyCount = (configDirty ? Math.max(1, countChanges(baseline, draft)) : 0) + (audioDirty ? 1 : 0) + (libDirty ? 1 : 0) + extraCount
  const dirty = dirtyCount > 0

  const setField = (path: string, value: unknown) => setDraft((prev) => setPath(prev, path, value))

  const fieldState = (path: string): FieldState | undefined => {
    if (cfgScope === 'global' || !response) return undefined
    const reported = sourceOf(path, response.sources, draft, cfgScope)
    const source: ConfigScope = reported === 'project' || reported === 'story' ? reported : 'global'
    // Cấp dự án/truyện từ chối providers.*.api_key và notify (11-api-contract.md §6)
    const restricted = path === 'notify' || path.startsWith('notify.') || (path.startsWith('providers.') && path.endsWith('.api_key'))
    return {
      path, scope: cfgScope, source, overridden: getPath(draft, path) !== undefined, canEdit: canEdit && !restricted,
      onOverride: () => setField(path, getPath(response.effective, path)),
      onReset: () => setField(path, undefined),
    }
  }
  const ovr = (keysOfRow: string[]) => ovrOf(cfgScope, hasOwn(draft, keysOfRow), inheritedFrom(sources, keysOfRow, cfgScope, response?.chain))

  const saveAction = useAction(async (overwrite = false) => {
    if (!connection.writable) return
    let message = ''
    try {
      if (configDirty && response) {
        const next = await putConfig(
          cfgScope, overwrite && conflict ? rebaseConfig(baseline, draft, conflict.current.target.config) : configForSave(draft),
          cfgProject, cfgStory, overwrite && conflict ? conflict.version ?? conflict.current.version : response.version,
        )
        const doc = configForCompare(next.target.config)
        setResponse(next); setDraft(doc); setBaseline(doc); setData(resourceKey, next)
        void invalidate(['config'])
        setConflict(null)
      }
      if (audioDirty && audioResource.data) {
        const base = overwrite && audioConflict ? audioConflict.current : audioResource.data
        const next = await putAudioSettings(project, story, { ...base, autoOnLock: autoOnLockDraft! }, base.version)
        setData(audioKey, next); setAutoOnLockDraft(null); setAudioConflict(null)
      }
      if (libDraft && libDirty) {
        const next = await putLibrary(libDraft)
        setLibDraft(next.config); setLibBaseline(next.config)
        library.reload()
      }
      for (const [key, extra] of Object.entries(extrasRef.current)) {
        const result = await extra.apply()
        if (typeof result === 'string' && (key === current || !message)) message = result
      }
    } catch (error) {
      // 409 phiên bản → thanh xung đột (xem / ghi đè), bản nháp giữ nguyên; lỗi khác để useAction hiện thông báo.
      if (error instanceof ConflictError) {
        if (error.current && typeof error.current === 'object' && 'target' in error.current) setConflict(error as ConflictError<ConfigResponse>)
        else setAudioConflict(error as ConflictError<AudioSettings>)
        return false
      }
      throw error
    }
    toast.ok(message || 'Đã lưu')
    return true
  })
  const saving = saveAction.pending
  const save = () => saveAction.run(false)
  useUnsavedGuard(dirty, undefined, { save: async () => await save() === true, allow: (url) => url.origin === location.origin && url.hash.startsWith('#/settings') })
  useSaveShortcut(save, dirty && !saveAction.busy && connection.writable)

  const discard = () => {
    setDraft(configForCompare(baseline))
    setLibDraft(libBaseline)
    setConflict(null)
    setAutoOnLockDraft(null)
    setAudioConflict(null)
    for (const extra of Object.values(extrasRef.current)) extra.cancel()
  }

  const guardDirty = (): boolean => {
    if (!dirty) return false
    toast.warn('Hãy lưu hoặc hủy thay đổi trước khi đổi phạm vi.')
    return true
  }

  // ─── Chuyển mục / cấp / chỗ đang xem ─────────────────────────────────────────────────────
  const goSection = (id: SectionId | 'voice') => {
    const nextScope = id === 'info' ? 'project' : effectiveScope
    const nextCfgScope: ConfigScope = id !== 'voice' && isSystemSection(id) ? 'global' : nextScope
    if (nextCfgScope !== cfgScope && guardDirty()) return
    navigate(settingsHref(id, nextScope, project, story))
  }
  /** Gói R — đổi chỗ xem (dự án/truyện): chỉ đổi ngữ cảnh Tuỳ chỉnh, không mở truyện khác ở Bàn viết. */
  const handleTarget = (next: ScopeTarget) => {
    setTouched(true)
    setScope(next.scope)
    setProject(next.project)
    setStory(next.story)
    if (next.project) library.ensureProject(next.project)
    if (current === 'info' && next.scope !== 'project') navigate(settingsHref('models', next.scope, next.project, next.story))
  }
  const [sectionHint, setSectionHint] = useState<{ id: string; open: string } | null>(null)
  const goAdvanced = (open?: string) => { goSection('adv'); setSectionHint(open ? { id: 'adv', open } : null) }
  const goConnect = (provider?: string) => { goSection('connect'); setSectionHint(provider ? { id: 'connect', open: provider } : null) }

  const dots: Partial<Record<SectionId, boolean>> = {}
  if (effectiveScope !== 'global') for (const [id, list] of Object.entries(DOT_KEYS)) dots[id as SectionId] = hasOwn(draft, list)

  const ctx: SettingsContext = {
    scope: cfgScope, viewScope: effectiveScope, project, story, projectName, storyTitle, admin, canEdit, canEditScope,
    response, draft, effective: response?.effective ?? {}, presets, snapshot,
    setField, resetField: (path) => setField(path, undefined), fieldState, ovr, refreshConfig,
    audioSettings: audioResource.data ? { ...audioResource.data, autoOnLock: autoOnLockDraft ?? audioResource.data.autoOnLock } : null,
    audioSettingsError: audioResource.error ? errText(audioResource.error) : '',
    setAutoOnLock: setAutoOnLockDraft, refreshAudioSettings: audioResource.refresh,
    library: library.library, libDraft, setLibDraft, reloadLibrary: library.reload, canReveal,
    sampleStoryTitle, sampleStoryIndex, registerExtra,
    goSection: (id, open) => (id === 'adv' ? goAdvanced(open) : id === 'connect' ? goConnect(open) : goSection(id)),
  }

  // Mục đã mở một lần thì giữ nguyên (ẩn) để bản nháp của nó không mất khi sang mục khác — như artboard (dirty giữ qua mục).
  const [visited, setVisited] = useState<ReadonlySet<PageId>>(() => new Set([current]))
  useEffect(() => { setVisited((prev) => (prev.has(current) ? prev : new Set(prev).add(current))) }, [current])
  const keepInfo = visited.has('info') && (current === 'info' || !!extraMeta.info)

  const needsConfig = CONFIG_SECTIONS.includes(current)
  // Chỉ mục có 3 cấp mới hiện thanh “Áp dụng cho” (Gói R, R3).
  const showScope = showsScope(current)
  const readOnlyNotice = !canEdit
    ? cfgScope === 'global' ? 'Chỉ quản trị viên sửa thiết lập chung — bạn đang xem bản chỉ đọc.'
      : cfgScope === 'project' ? 'Bạn chỉ có quyền đọc tuỳ chỉnh của dự án này.' : 'Bạn chỉ có quyền đọc tuỳ chỉnh của truyện này.'
    : ''

  const body = () => {
    switch (current) {
      case 'models': return <RolesSection ctx={ctx} />
      case 'budget': return <BudgetSection ctx={ctx} />
      case 'connect': return <ProvidersSection ctx={ctx} open={sectionHint?.id === 'connect' ? sectionHint.open : undefined} />
      case 'storage': return <StorageSection />
      case 'notify': return <NotifySection ctx={ctx} />
      case 'voice': return <VoiceSection ctx={ctx} />
      case 'export':
        return libDraft ? <ExportSection ctx={ctx} /> : library.error
          ? <ErrorCard message="Không đọc được cấu hình thư viện." onRetry={library.reload} /> : <LoadingRows rows={3} />
      case 'app': return <AppearanceSection />
      case 'adv': return <AdvancedSection ctx={ctx} open={sectionHint?.id === 'adv' ? sectionHint.open : undefined} onWhatsNew={() => setWhatsNew(true)} />
      default: return null
    }
  }


  return (
    // line-height 1.5 như khung gốc của Settings.dc.html (nền ứng dụng dùng 1.55 → mỗi dòng lệch ~0,7px, cộng dồn theo trang).
    <div style={{ position: 'relative', display: 'flex', flex: 1, minWidth: 0, minHeight: 0, width: '100%', lineHeight: 1.5 }}>
      <SettingsNav current={current} scope={effectiveScope} admin={admin} dots={dots}
        version={version} onWhatsNew={() => setWhatsNew(true)} onPick={goSection} />
      <main className="scroll" style={{ flex: '1 1 auto', minWidth: 0, padding: '18px 28px 120px' }}>
        {hasNewVersion(version) && !updateHidden && (
          <div role="status" style={{
            maxWidth: 820, marginBottom: 16, display: 'flex', alignItems: 'center', gap: 12, padding: '12px 14px',
            borderRadius: 14, background: 'var(--kit-up-bg)', border: '1px solid var(--kit-up-bd)',
          }}>
            <Icon name="refresh-2" size={18} style={{ color: 'var(--kit-info-fg)' }} />
            <span style={{ flex: '1 1 auto', display: 'flex', flexDirection: 'column', lineHeight: 1.45, minWidth: 0 }}>
              <span style={{ fontSize: 13.5, fontWeight: 500 }}>Có bản Noverse mới · {versionText(version?.latestVersion)}</span>
              <span style={{ fontSize: 12.5, color: 'var(--color-fg-3)' }}>Bạn đang dùng {versionText(version?.version)}.</span>
            </span>
            <button type="button" className="hov-ghost" onClick={() => setWhatsNew(true)} style={{
              height: 32, padding: '0 12px', borderRadius: 9, border: '1px solid var(--line-4)',
              background: 'transparent', color: 'var(--color-fg)', fontSize: 12.5, whiteSpace: 'nowrap',
            }}>Có gì mới</button>
            <button type="button" className="hov-ghost" onClick={() => setUpdateHidden(true)} aria-label="Ẩn thông báo" style={{
              width: 32, height: 32, border: 'none', borderRadius: 9, background: 'transparent', color: 'var(--color-fg-4)',
              display: 'flex', alignItems: 'center', justifyContent: 'center',
            }}>
              <Icon name="close-circle-bulk" size={16} />
            </button>
          </div>
        )}
        {showScope && (
          <ScopeBar
            target={{ scope: effectiveScope, project, story }}
            projectName={projectName} storyTitle={storyTitle} storyCount={stories.length}
            open={open} projects={projects} stories={stories}
            storiesOf={(id) => library.projectDetail(id)?.stories ?? EMPTY_STORIES}
            dirtyCount={dirtyCount}
            onTarget={handleTarget}
            onSave={() => save()}
            onDiscard={discard}
            ensureProject={library.ensureProject} />
        )}
        {needsConfig && <PanelRefreshBar resourceKeys={[resourceKey]} updatedAt={configResource.updatedAt} />}
        {needsConfig && !!configResource.data && <StaleBanner error={configResource.error} onRetry={refreshConfig} />}
        {needsConfig && conflict && <ConflictBar pending={saveAction.busy} onView={() => {
          const next = conflict.current
          setResponse(next); setData(resourceKey, next)
          const doc = configForCompare(next.target.config)
          setDraft(doc); setBaseline(doc); setConflict(null)
        }} onOverwrite={() => saveAction.run(true)} />}
        {audioConflict && <ConflictBar pending={saveAction.busy} onView={() => {
          setData(audioKey, audioConflict.current); setAutoOnLockDraft(null); setAudioConflict(null)
        }} onOverwrite={() => saveAction.run(true)} />}
        {!canEdit && needsConfig && <div style={{ marginTop: 14 }}><ReadOnlyNotice>{readOnlyNotice}</ReadOnlyNotice></div>}

        {keepInfo && (
          <div hidden={current !== 'info'}>
            <InfoSection key={project} ctx={ctx} />
          </div>
        )}
        {current !== 'info' && (
          needsConfig
            ? configError && !response ? <ErrorCard message={configError} onRetry={refreshConfig} />
              : !response || configLoading ? <LoadingRows rows={4} /> : body()
            : body()
        )}
      </main>
      {whatsNew && <WhatsNewDialog onClose={() => setWhatsNew(false)} />}
      {dirty && (
        <SaveBar text={current === 'connect' ? `${dirtyCount} thay đổi ở Kết nối AI · dùng cho cả máy chủ` : dirtyText(dirtyCount, cfgScope)}
          applyLabel="Lưu" saving={saving} disabled={saveAction.busy || !connection.writable} onApply={() => void save()} onCancel={discard} />
      )}
    </div>
  )
}
