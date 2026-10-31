// M4 · Thêm — các trang độc lập trên điện thoại (18-mobile.md / MMore.dc.html).
import { useEffect, useState } from 'react'
import { canReadProject, canReadStory, canWriteStory, getConfig, getConfigPresets, getLibrary, getModels, getProject, mySessions, revokeSession } from '../../../api/client'
import { useSession } from '../../../api/session'
import type { ModelRoleKey, ProjectSummary, Session, StorySummary } from '../../../api/types'
import { cache, invalidate } from '@noverse/ui/data'
import { keys } from '../../../data/keys'
import { useAction } from '../../../data/useAction'
import { useResource } from '../../../data/useResource'
import { ConfirmSheet, MobilePage, useMobileToast } from '../../../mobile'
import { navigate, paths } from '../../../router'
import { AgentOrb } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { relativeTime } from '../../auth/parts'
import { sessionIcon } from '../../account/shared'
import { ROLE_META } from '../../cmdresults/logic'
import { Models, modelsKey } from './Models'
import { ResourceNotice } from './parts'
import { MobileStorySet } from './StorySet'
import { rulesSub } from './rulesSummary'
import { MobileRules } from './Rules'
import { useRulesCount } from '../../settings/sections/rules'
import { useNotifyPrefs } from '../../notify/useNotify'
import { useCommands } from '../../../commands/store'
import { AccountsSheet, AccountAvatar, RoleBadge } from './Accounts'
import { MobilePassword } from './Password'
import { MobileGuide, desktopCommand } from './Guide'
import { useTabMode } from '../../../mobile/runtime'
import { styleDisplayName } from '../../settings/sections/style'
import { usd } from './storySetLogic'
export { MobileFirstPassword } from './Password'
import './more.css'
import { projectLabel, storyLabel } from '../../../shell/labels'
import { storageCopy } from '../../../i18n/vi'
import { StorageSection } from '../../settings/storage'
import { StorageNavSummary } from '../../settings/storage/indicators'
import { AppSwitcher } from '../../../shell/AppSwitcher'
import { ssoCopy } from '../../../i18n/sso'

type Page = 'home' | 'projects' | 'model' | 'pick' | 'devices' | 'password' | 'guide' | 'guideCmd' | 'storyset' | 'rules' | 'storage'
const STATUS: Record<StorySummary['status'], string> = { idea: 'Nháp', writing: 'Đang viết', paused: 'Tạm dừng', done: 'Đã xong', published: 'Đã xuất bản' }

function remaining(expiresAt: string | undefined) {
  if (!expiresAt) return 'Đang tải thời hạn phiên'
  const time = new Date(expiresAt).getTime() - Date.now()
  if (!Number.isFinite(time)) return 'Chưa rõ thời hạn phiên'
  if (time <= 0) return 'Phiên đã hết hạn'
  if (time < 86_400_000) return `còn ${Math.max(1, Math.ceil(time / 3_600_000))} giờ`
  return `còn ${Math.ceil(time / 86_400_000)} ngày`
}

function ProjectStories({ project, currentProject, currentStory, onChoose, offline }: {
  project: ProjectSummary; currentProject: string | null; currentStory: string | null
  onChoose: (story: StorySummary) => void; offline: boolean
}) {
  const { user } = useSession()
  const resource = useResource(keys.project(project.id, 'meta'), () => getProject(project.id))
  const stories = resource.data?.stories.filter((item) => user && canReadStory(user, item.id)) ?? []
  return <section>
    <div className="mm-heading"><span>{projectLabel(project, project.id)}</span><span>{resource.data ? stories.length : project.storyCount} truyện</span></div>
    <ResourceNotice resource={resource} label={`truyện trong ${projectLabel(project, project.id)}`} />
    {resource.data && <div className="mm-group">{stories.map((item, index) => {
      const selected = currentProject === item.project && currentStory === item.story
      return <button key={item.id} className="mm-row" type="button" aria-current={selected ? 'true' : undefined} disabled={offline} onClick={() => onChoose(item)}>
        <span className="mm-story-number mm-mono">{String(index + 1).padStart(2, '0')}</span>
        <span className="mm-copy"><span>{storyLabel(item)}</span><span className={`mm-sub${selected ? ' mm-selected' : ''}`}>{STATUS[item.status]} · {item.completedChapters}{item.totalChapters > 0 ? `/${item.totalChapters}` : ''} chương{item.locked ? ' · Đang khoá' : ''}</span></span>
        {selected && <Icon name="tick-circle" size={20} className="mm-check" />}
      </button>
    })}{!stories.length && <p className="mm-empty">Chưa có truyện bạn có thể mở.</p>}</div>}
  </section>
}

export function MobileMore({ project, story, initialPage, initialCommand, onStoryChange }: { project: string | null; story: string | null; initialPage?: 'home' | 'projects' | 'model' | 'devices' | 'password' | 'guide' | 'storyset' | 'rules' | 'storage'; initialCommand?: string; onStoryChange?: (target: { project: string; story: string }) => void }) {
  const { user, session, signOut } = useSession()
  const toast = useMobileToast()
  const firstPage = initialPage ?? (story ? 'home' : 'projects')
  const [stack, setStack] = useState<Page[]>(initialCommand ? ['home', 'guide', 'guideCmd'] : firstPage === 'home' ? ['home'] : ['home', firstPage])
  const [transition, setTransition] = useState({ number: 0, back: false })
  const [role, setRole] = useState<ModelRoleKey>('writer')
  const [confirm, setConfirm] = useState<Session | 'all' | null>(null)
  const [online, setOnline] = useState(navigator.onLine)
  const [accountsOpen, setAccountsOpen] = useState(false)
  const [command, setCommand] = useState(initialCommand ?? 'export')
  const [commandHistory, setCommandHistory] = useState<string[]>(initialCommand ? [initialCommand] : [])
  const [clock, setClock] = useState(Date.now())
  const page = stack[stack.length - 1]
  useTabMode(page === 'password' || page === 'guideCmd' ? 'hide' : undefined)
  const hasStory = !!project && !!story
  const canWrite = !!user && hasStory && canWriteStory(user, `${project}/${story}`)
  const library = useResource(keys.library(), getLibrary)
  const selected = useResource(keys.project(project ?? '', 'meta'), () => getProject(project!), { enabled: !!project })
  const config = useResource(keys.config('story', project ?? '', story ?? ''), () => getConfig('story', project!, story!), { enabled: canWrite })
  const models = useResource(modelsKey(project ?? '', story ?? ''), () => getModels(project!, story!), { enabled: canWrite })
  const devices = useResource(keys.sessions(), mySessions)
  const currentStory = selected.data?.stories.find((item) => item.story === story)
  const sessions = devices.data?.sessions ?? []
  const others = sessions.filter((item) => !item.current && item.id !== session?.id)
  const review = config.data?.effective.advance_mode === 'review'
  const ruleCount = useRulesCount('story', project ?? undefined, story ?? undefined)
  const presets = useResource(['config-presets'], ({ signal }) => getConfigPresets(signal), { staleTime: 60000 })
  const notifyPrefs = useNotifyPrefs()
  const notifyCount = notifyPrefs.data ? (notifyPrefs.data.enabled ? Object.values(notifyPrefs.data.types).filter(Boolean).length : 0) : undefined
  const commandCount = useCommands().specs.length
  const writerModel = models.data?.roles.find((item) => item.role === 'writer')?.model
  const showModel = !hasStory || canWrite || !!writerModel
  const modelLabel = writerModel || (!hasStory ? 'Chưa chọn truyện' : models.error ? 'Không tải được model' : models.initialLoading ? 'Đang tải…' : 'Chưa chọn model')

  useEffect(() => {
    const update = () => { setOnline(navigator.onLine) }
    addEventListener('online', update); addEventListener('offline', update); addEventListener('focus', update)
    const timer = setInterval(() => setClock(Date.now()), 60_000)
    return () => { removeEventListener('online', update); removeEventListener('offline', update); removeEventListener('focus', update); clearInterval(timer) }
  }, [])

  const push = (next: Page) => { setStack((old) => [...old, next]); setTransition((old) => ({ number: old.number + 1, back: false })) }
  const back = () => {
    if (page === 'guideCmd') { const next = commandHistory.slice(0, -1); setCommandHistory(next); setCommand(next[next.length - 1] ?? 'export') }
    setStack((old) => old.length > 1 ? old.slice(0, -1) : old)
    setTransition((old) => ({ number: old.number + 1, back: true }))
  }
  const logoutAction = useAction(async () => {
    await signOut()
    cache.clear()
  })
  const revokeAction = useAction(async () => {
    if (!confirm || !online) throw new Error('Không có kết nối. Vui lòng thử lại khi có mạng.')
    const targets = confirm === 'all' ? others : [confirm]
    let removed = 0
    try {
      for (const target of targets) {
        await revokeSession(target.id)
        removed++
        cache.setData<{ sessions: Session[] }>(keys.sessions(), (previous) => ({ sessions: (previous?.sessions ?? sessions).filter((item) => item.id !== target.id) }))
      }
      setConfirm(null)
      toast(confirm === 'all' ? `Đã đăng xuất ${removed} thiết bị khác` : `Đã đăng xuất ${confirm.device}`)
      return true
    } catch (error) {
      if (removed > 0) toast(`Đã đăng xuất ${removed} thiết bị; một số thiết bị chưa đăng xuất được`)
      throw error
    } finally { void invalidate(keys.sessions()) }
  })
  const title = page === 'pick' ? ROLE_META[role].label : { home: 'Thêm', projects: 'Dự án & truyện', model: 'Model trợ lý', devices: 'Thiết bị', password: 'Đổi mật khẩu', guide: 'Hướng dẫn lệnh', guideCmd: 'Lệnh', storyset: 'Tuỳ chỉnh truyện', rules: 'Cách viết', storage: storageCopy.title }[page]
  const choose = (item: StorySummary) => {
    setStack(['home'])
    toast(`Đã mở “${storyLabel(item)}”`)
    if (onStoryChange) onStoryChange({ project: item.project, story: item.story })
    else navigate(`/p/${encodeURIComponent(item.project)}/s/${encodeURIComponent(item.story)}/work`)
  }
  const useCommand = (line: string) => {
    if (!project || !story) { toast('Chọn một truyện trước khi dùng lệnh'); return }
    navigate(`${paths.workspace(project, story)}?command=${encodeURIComponent(line)}`)
  }
  if (page === 'password') return <div key={transition.number} className={transition.number === 0 ? undefined : 'm-push'} style={{ display: 'flex', flex: 1, minHeight: 0 }}><MobilePassword onDone={back} /></div>
  const footer = page === 'guideCmd' ? <div className="mm-cta"><button className="mm-primary" type="button" disabled={desktopCommand(command) || !hasStory} onClick={() => useCommand(`/${command}`)}>{desktopCommand(command) ? 'Lệnh này chỉ dùng trên máy tính' : `Thử /${command} ở màn Viết`}</button>{!hasStory && <p>Chọn một truyện trước để dùng lệnh.</p>}</div> : undefined

  return <MobilePage title={title} backTitle={page === 'pick' ? 'Model' : page === 'guideCmd' ? 'Lệnh' : 'Thêm'} onBack={page === 'home' ? undefined : back} footer={footer}>
    <div className="mm-root">
      {!online && <p className="mm-error" role="status">Mất kết nối. Các thao tác thay đổi tạm thời bị khoá.</p>}
      <div key={transition.number} className={transition.number === 0 ? undefined : transition.back ? 'mm-pop' : 'mm-push'}>
        {page === 'home' && <>
          <button className="mm-account" type="button" aria-label="Đổi tài khoản" onClick={() => setAccountsOpen(true)}>
            <AccountAvatar name={user?.name || user?.username || '?'} role={user?.role ?? 'member'} />
            <span className="mm-copy"><span className="mm-account-name">{user?.name || user?.username}{user && <RoleBadge role={user.role} />}</span>
              <span className="mm-sub" key={clock}>Phiên trên {session?.device || 'thiết bị này'} · {remaining(session?.expiresAt)}</span></span><Icon name="arrow-swap-horizontal" size={20} className="mm-muted" />
          </button>
          <div className="mm-heading">Đang mở</div>
          <div className="mm-group"><button className="mm-row" type="button" onClick={() => push('projects')}>
            <Icon name="folder-open" size={22} className="mm-muted" /><span className="mm-copy"><span>{story ? (currentStory ? storyLabel(currentStory) : '…') : 'Chọn dự án & truyện'}</span><span className="mm-sub">{project ? `${projectLabel(library.data?.projects.find((item) => item.id === project), project)}${currentStory ? ` · ${currentStory.volumes} tập` : ''}` : 'Mở một truyện để theo dõi và điều khiển'}</span></span><span className="mm-muted">Đổi</span><Icon name="arrow-right-2" size={16} className="mm-chevron" />
          </button>
          <button className="mm-row" type="button" disabled={!hasStory} onClick={() => push('rules')}>
            <Icon name="brush-2" size={22} className="mm-muted" /><span className="mm-copy"><span>Cách viết</span><span className="mm-sub">{hasStory ? `${styleDisplayName(presets.data, config.data?.effective.style)} · ${rulesSub(ruleCount)}` : 'Chọn một truyện trước'}</span></span><Icon name="arrow-right-2" size={16} className="mm-chevron" />
          </button>
          <button className="mm-row" type="button" disabled={!canWrite} onClick={() => push('storyset')}>
            <Icon name="note-text" size={22} className="mm-muted" /><span className="mm-copy"><span>Tuỳ chỉnh truyện</span><span className="mm-sub">{!hasStory ? 'Chọn một truyện trước' : !canWrite ? 'Chỉ người có quyền viết mới đổi được' : `${review ? 'Duyệt từng chương' : 'Viết liền'}${config.data?.effective.budget?.book_usd ? ` · ${usd(config.data.effective.budget.book_usd)}/truyện` : ''}`}</span></span>{canWrite && <Icon name="arrow-right-2" size={16} className="mm-chevron" />}
          </button>
          {/* M8 · Não truyện — ngay sau “Tuỳ chỉnh truyện” (MBrain.dc.html) */}
          <button className="mm-row" type="button" disabled={!hasStory} onClick={() => navigate(paths.brain(project!, story!))}>
            <Icon name="routing-2" size={22} className="mm-muted" /><span className="mm-copy"><span>Não truyện</span><span className="mm-sub">{hasStory ? 'Nhân vật, quan hệ, xưng hô theo chương' : 'Chọn một truyện trước'}</span></span>{hasStory && <Icon name="arrow-right-2" size={16} className="mm-chevron" />}
          </button></div>
          {!!project && <ResourceNotice resource={selected} label="truyện đang mở" />}
          <div className="mm-heading">Trợ lý</div>
          <div className="mm-group">
            {showModel && <button className="mm-row" type="button" disabled={!canWrite} onClick={() => push('model')}>
              <span className="mm-orbs"><AgentOrb agent="architect" size={14} /><AgentOrb agent="writer" size={14} style={{ marginLeft: -5 }} /></span>
              <span className="mm-copy"><span>Model cho từng trợ lý</span><span className="mm-sub">Người viết · <span className="mm-mono">{modelLabel}</span></span>{hasStory && !canWrite && <span className="mm-sub">Chỉ người có quyền viết mới đổi được</span>}</span>{canWrite && <Icon name="arrow-right-2" size={16} className="mm-chevron" />}
            </button>}
          </div>
          {canWrite && <ResourceNotice resource={models} label="model trợ lý" />}
          {logoutAction.error != null && <p className="mm-error" role="alert">{logoutAction.error instanceof Error ? logoutAction.error.message : 'Không thực hiện được. Vui lòng thử lại.'}</p>}
          <AppSwitcher mobile />
          <div className="mm-heading">Tài khoản</div>
          <div className="mm-group">
            <button className="mm-row" type="button" onClick={() => push('devices')}><Icon name="devices" size={22} className="mm-muted" /><span className="mm-fill">Thiết bị đang đăng nhập</span><span className="mm-muted">{devices.data ? sessions.length : devices.error ? 'Không tải được' : '…'}</span><Icon name="arrow-right-2" size={16} className="mm-chevron" /></button>
            <button className="mm-row" type="button" onClick={() => push('password')}><Icon name="key" size={22} className="mm-muted" /><span className="mm-fill">Đổi mật khẩu</span><Icon name="arrow-right-2" size={16} className="mm-chevron" /></button>
            <button className="mm-row" type="button" onClick={() => navigate('/notifications?view=settings')}><Icon name="notification" size={22} className="mm-muted" /><span className="mm-fill">Thông báo</span>{notifyCount !== undefined && <span className="mm-muted">{notifyCount} loại</span>}<Icon name="arrow-right-2" size={16} className="mm-chevron" /></button>
          </div>
          {devices.error != null && <ResourceNotice resource={devices} label="thiết bị" />}
          {user?.role === 'admin' && <><div className="mm-heading">{storageCopy.admin}</div><div className="mm-group"><button className="mm-row" type="button" onClick={() => push('storage')}><Icon name="box" size={22} className="mm-muted" /><span className="mm-copy"><span>{storageCopy.title}</span><StorageNavSummary /></span><Icon name="arrow-right-2" size={16} className="mm-chevron" /></button></div></>}
          <div className="mm-heading">Trợ giúp</div><div className="mm-group"><button className="mm-row" type="button" onClick={() => push('guide')}><Icon name="message-question" size={22} className="mm-muted" /><span className="mm-fill">Hướng dẫn lệnh</span>{commandCount > 0 && <span className="mm-muted">{commandCount} lệnh</span>}<Icon name="arrow-right-2" size={16} className="mm-chevron" /></button></div>
          <div className="mm-group mm-spaced"><button className={`mm-row mm-danger${user?.grants != null ? '' : ' mm-center'}`} type="button" disabled={!online || logoutAction.busy} aria-busy={logoutAction.pending} onClick={() => { void logoutAction.run() }}>{user?.grants != null ? <span className="mm-copy"><span>{logoutAction.pending ? 'Đang đăng xuất…' : 'Đăng xuất'}</span><span className="mm-sub">{ssoCopy.logout}</span></span> : logoutAction.pending ? 'Đang đăng xuất…' : 'Đăng xuất'}</button></div>
          <p className="mm-note mm-center">Kết nối AI, xuất tự động và quản lý tài khoản nằm trong Tuỳ chỉnh trên máy tính.</p>
        </>}
        {page === 'rules' && <MobileRules project={project} story={story} online={online} toast={toast} />}
        {page === 'storage' && user?.role === 'admin' && <StorageSection mobile />}
        {page === 'storyset' && <MobileStorySet project={project} story={story} nextChapter={(currentStory?.completedChapters ?? 0) + 1} online={online} toast={toast} onModel={() => push('model')} writerModel={modelLabel} />}
        {(page === 'guide' || page === 'guideCmd') && <MobileGuide command={page === 'guideCmd' ? command : null} onPick={(name) => { setCommand(name); setCommandHistory((old) => [...old, name]); push('guideCmd') }} onUse={useCommand} />}
        {page === 'projects' && <>
          <ResourceNotice resource={library} label="dự án" />
          {library.data?.projects.filter((item) => user && canReadProject(user, item.id)).map((item) => <ProjectStories key={item.id} project={item} currentProject={project} currentStory={story} onChoose={choose} offline={!online} />)}
          {library.data?.projects.length === 0 && <p className="mm-note">Chưa có dự án nào trong thư viện.</p>}
          <p className="mm-note mm-center">Tạo truyện mới hoặc nhập truyện có sẵn trên máy tính.</p>
        </>}
        {(page === 'model' || page === 'pick') && project && story && (canWrite ? <Models project={project} story={story} active={page === 'pick' ? role : null} onPick={(next) => { setRole(next); push('pick') }} disabled={!online} toast={toast} /> : <p className="mm-note">Chỉ người có quyền viết mới đổi được model.</p>)}
        {page === 'devices' && <>
          <ResourceNotice resource={devices} label="thiết bị" />
          {user?.grants != null && <p className="mm-note">{ssoCopy.devices}</p>}
          {devices.data && <div className="mm-group mm-device-group">{sessions.map((item) => {
            const current = item.current || item.id === session?.id
            return <div className="mm-row" key={item.id}>
              <span className="mm-device-icon"><Icon name={sessionIcon(item.device)} size={20} /></span>
              <span className="mm-copy"><span>{item.device}{current ? ' này' : ''}</span><span className={`mm-sub${current ? ' mm-current' : ''}`}>{current ? 'Đang dùng' : `Hoạt động ${relativeTime(item.lastSeen)}`}{item.ip ? ` · ${item.ip}` : ''}</span></span>
              {!current && <button className="mm-revoke" type="button" disabled={!online || revokeAction.busy || !!devices.error} onClick={() => setConfirm(item)} aria-label={`Đăng xuất ${item.device}`}>Đăng xuất</button>}
            </div>
          })}{sessions.length === 0 && <p className="mm-empty">Không có thiết bị đang đăng nhập.</p>}</div>}
          {others.length > 0 && <div className="mm-group mm-spaced"><button className="mm-row mm-danger mm-center" type="button" disabled={!online || revokeAction.busy || !!devices.error} onClick={() => setConfirm('all')}>Đăng xuất mọi thiết bị khác</button></div>}
          {revokeAction.error != null && <p className="mm-error" role="alert">{revokeAction.error instanceof Error ? revokeAction.error.message : 'Không đăng xuất được thiết bị. Vui lòng thử lại.'}</p>}
          {revokeAction.pending && <p className="mm-note" role="status">Đang đăng xuất thiết bị…</p>}
          <p className="mm-note mm-center">Phiên tự hết hạn sau 30 ngày không dùng.</p>
          {user?.grants != null && <p className="mm-note">{ssoCopy.revoke}</p>}
        </>}
      </div>
    </div>
    <AccountsSheet open={accountsOpen} onClose={() => setAccountsOpen(false)} />
    <ConfirmSheet open={confirm !== null} onClose={() => { if (!revokeAction.busy) setConfirm(null) }}
      title={confirm === 'all' ? `Đăng xuất ${others.length} thiết bị khác?` : `Đăng xuất “${confirm?.device ?? ''}”?`}
      description={user?.grants != null ? ssoCopy.revoke : confirm === 'all' ? 'Điện thoại này vẫn giữ phiên. Các thiết bị kia cần quét mã QR hoặc nhập mật khẩu để vào lại.' : 'Thiết bị này cần quét mã QR hoặc nhập mật khẩu để vào lại. Truyện không bị ảnh hưởng.'}
      action={{ label: revokeAction.pending ? 'Đang đăng xuất…' : confirm === 'all' ? 'Đăng xuất tất cả' : 'Đăng xuất thiết bị', destructive: true, onSelect: async () => {
        const succeeded = await revokeAction.run()
        if (!succeeded) throw new Error('Chưa đăng xuất được thiết bị. Vui lòng thử lại.')
      } }} />
  </MobilePage>
}
