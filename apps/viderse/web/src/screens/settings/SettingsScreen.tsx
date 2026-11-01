import * as stylex from '@stylexjs/stylex'
import { styles as settingsStyles, setNavLinkMarker, setHelpCardMarker, msetRowMarker, msetQueueMarker, isWarnMarker } from './settings.stylex'
// Khung Cài đặt — desktop: cột mục trái + nội dung giữa + thẻ “Khi nào…” phải
// (artboard Settings); mobile: lớp phủ toàn màn theo MSettings — thẻ “Máy sẵn
// sàng”, danh sách mục kèm giá trị, thẻ Hàng chờ, chân phiên bản; màn con không
// có thanh tab dưới. Chấm trên tên mục khi /api/status báo attention trỏ tới
// #/settings?section=<mục>.
import { Suspense, lazy, useMemo } from 'react'
import { Icon, Spinner } from '@noverse/ui'
import { useMediaQuery } from '../../lib/useMediaQuery'
import { semverOf } from '../../lib/version'
import { strongestTone, type AttentionTone } from '../../lib/attentionTone'
import { navigate, paths, routePath, type SettingsSection } from '../../router'
import { vi as t } from '../../i18n/vi'
import { useStatus } from '../../data/status'
import { useResource } from '../../data/useResource'
import { keys } from '../../data/keys'
import { getStorage } from '../../api/storage'
import { getServerStorage } from '../../api/storageServer'
import { serverStorageCopy as serverCopy } from '../../i18n/storageServer'
import { getErrors } from '../../api/errors'
import { useSession } from '../../api/session'
import { AdminStorageStrip } from '../../shell/AdminStorageStrip'
import { getTools } from '../../api/tools'
import { getVersion } from '../../api/version'
import { getAgents, listProviders } from '../../api/ai'
import { listPlatforms } from '../../api/platforms'
import { logsExportUrl } from '../../api/logs'
import { fill, fmtBytes } from './fmt'
import { membersUrl } from '../../api/sso'
import { MembersLink } from '../../components/AppLinks'
import { AdminReadOnly } from '../../components/AdminReadOnly'
import { box } from '../../styles/shared.stylex'

const ToolsSection = lazy(() => import('./tools/ToolsSection'))
const StorageSection = lazy(() => import('./storage/StorageSection'))
const LogsSection = lazy(() => import('./logs/LogsSection'))
const ConnectSection = lazy(() => import('./connect/ConnectSection'))
const AgentsSection = lazy(() => import('./agents/AgentsSection'))
const AccountsSection = lazy(() => import('./accounts/AccountsSection'))
const NotificationSettings = lazy(() => import('./NotificationSettings'))

type NavItem = {
  id: SettingsSection
  icon: string
  label: string
  /** Số nhỏ bên phải tên mục — chỉ hiện khi có nghĩa (artboard Settings). */
  count?: string
}

/** Tông chấm của từng mục — bảng kind → tông ở lib/attentionTone (hỏi đáp #30). */
const DOT_STYLE: Record<AttentionTone, stylex.StyleXStyles> = {
  err: settingsStyles.dotErr,
  warn: settingsStyles.dotWarn,
  info: settingsStyles.dotInfo,
}

/** Mục nào đang có chấm attention — lấy từ href #/settings?section= của /api/status.
 *  Mỗi mục nhận tông của việc khẩn nhất đang trỏ vào nó. */
function useSectionDots(): Map<string, AttentionTone> {
  const status = useStatus()
  return useMemo(() => {
    const kindsBySection = new Map<string, string[]>()
    for (const a of status.data?.attention ?? []) {
      const match = a.href.match(/section=(\w+)/)
      if (!match) continue
      kindsBySection.set(match[1], [...(kindsBySection.get(match[1]) ?? []), a.kind])
    }
    const dots = new Map<string, AttentionTone>()
    for (const [id, kinds] of kindsBySection) dots.set(id, strongestTone(kinds)!)
    return dots
  }, [status])
}

const SECTIONS: Record<SettingsSection, React.ComponentType> = {
  tools: ToolsSection,
  storage: StorageSection,
  logs: LogsSection,
  connect: ConnectSection,
  agents: AgentsSection,
  accounts: AccountsSection,
  notifications: NotificationSettings,
}

/** Mục cấu hình toàn máy: thành viên chỉ xem (máy chủ trả 403 cho thao tác ghi). */
const ADMIN_WRITE_SECTIONS: Partial<Record<SettingsSection, true>> = { tools: true, connect: true, agents: true, accounts: true }

function SectionBody({ section }: { section: SettingsSection }) {
  const Body = SECTIONS[section]
  return ADMIN_WRITE_SECTIONS[section] ? <AdminReadOnly><Body /></AdminReadOnly> : <Body />
}

/** Số liệu khung cần cho cột mục + danh sách mobile (dung lượng, lỗi, công cụ, bản). */
export type FrameFacts = {
  toolsCount: string
  storageCount: string
  logsCount: string
  installing: number
  failed: number
  updates: number
  version: string
  used: number
  /** Số nhà cung cấp AI đã kết nối (undefined = chưa có dữ liệu). */
  providers?: number
  /** Nhãn nhà cung cấp đang vẽ ảnh (undefined = chưa có dữ liệu). */
  artist?: string
  /** Số nền tảng app hỗ trợ (undefined = chưa có dữ liệu). */
  platforms?: number
  server?: boolean
  storageMobile?: string
}

/** Số liệu khung cần cho cột mục + danh sách mobile (dung lượng, lỗi, công cụ, bản). */
function useFrameFacts(): FrameFacts {
  const tools = useResource(keys.tools(), ({ signal }) => getTools(signal))
  const server = useResource(['storage', 'server'], ({ signal }) => getServerStorage(signal))
  const storage = useResource(keys.storage(), ({ signal }) => getStorage(signal), { enabled: server.data === null })
  const errors = useResource(keys.errors(), ({ signal }) => getErrors(signal))
  const version = useResource(keys.appVersion(), ({ signal }) => getVersion(signal))
  // Cùng khoá cache với mục Kết nối AI / Trợ lý AI / Tài khoản nền tảng nên mở
  // mục đó không tải lại.
  const providers = useResource(keys.aiProviders(), ({ signal }) => listProviders(signal))
  const agents = useResource(keys.aiAgents(), ({ signal }) => getAgents(signal))
  const platforms = useResource(keys.platforms(), ({ signal }) => listPlatforms(signal))
  const rows = tools.data?.tools ?? []
  const installing = rows.filter((x) => x.state === 'installing').length
  const failed = rows.filter((x) => x.state === 'failed').length
  const updates = rows.filter((x) => x.state === 'update').length
  const done = rows.filter((x) => x.state === 'ready' || x.state === 'update').length
  const serverData = server.data
  const used = serverData ? serverData.usage.reduce((sum, group) => sum + (group.bytes ?? 0), 0) : storage.data ? storage.data.projects.usedBytes + storage.data.stayBytes : 0
  const providerList = providers.data
  const drawRef = agents.data?.artist.draw
  const artist = drawRef?.provider
    ? (providerList?.find((x) => x.id === drawRef.provider)?.label ?? drawRef.provider)
    : providerList && drawRef ? '' : undefined
  let storageMobile = serverData ? fmtBytes(used) : undefined
  if (serverData?.role === 'admin' && serverData.storage) {
    const saved = serverData.storage
    const providerName = saved.kind === 's3' ? serverCopy.providers[saved.provider ?? 'other'].name : serverCopy.diskShort
    const migration = serverData.migration
    if (!saved.online) storageMobile = fill(serverCopy.mobileDown, { label: providerName })
    else if (migration && !['done', 'cancelled'].includes(migration.state)) {
      const percent = migration.bytes.total > 0 ? Math.round(migration.bytes.done / migration.bytes.total * 100) : 0
      storageMobile = `${serverCopy.mobileMoving} · ${percent}%`
    } else storageMobile = `${providerName} · ${fmtBytes(used)}`
  }
  return {
    server: !!serverData,
    providers: providerList?.length,
    artist,
    platforms: platforms.data?.length,
    toolsCount: installing > 0 ? fill(t.settings.counts.toolsInstalling, { done, total: rows.length }) : '',
    storageCount: serverData || storage.data ? fmtBytes(used) : '',
    storageMobile,
    logsCount: errors.data ? String(errors.data.count7d) : '',
    installing, failed, updates,
    // Chân cột chỉ hiện semver (“Viderse 1.4.0” — artboard). Chuỗi build đầy đủ
    // (ngày + commit + `+dirty`) thuộc màn nhật ký, không phải chrome cài đặt.
    version: semverOf(version.data?.version ?? ''),
    used,
  }
}

function NavList({ section, dots, facts, onPick }: {
  section: SettingsSection | null
  dots: Map<string, AttentionTone>
  facts: FrameFacts
  onPick: (id: SettingsSection) => void
}) {
  const { user, apps } = useSession()
  const memberHref = membersUrl(apps)
  const machine: NavItem[] = [
    { id: 'tools', icon: 'cpu', label: t.settings.nav.tools, count: facts.toolsCount },
    { id: 'connect', icon: 'key', label: t.settings.nav.connect },
    { id: 'agents', icon: 'magic', label: t.settings.nav.agents },
    { id: 'accounts', icon: 'global', label: t.settings.nav.accounts },
    { id: 'notifications', icon: 'notification', label: 'Thông báo' },
  ]
  const data: NavItem[] = [
    { id: 'storage', icon: 'folder', label: t.settings.nav.storage, count: facts.storageCount },
    { id: 'logs', icon: 'flag', label: t.settings.nav.logs, count: facts.logsCount },
  ]
  const render = (items: NavItem[]) =>
    items.map((n) => (
      <button
        key={n.id}
        type="button"
        className={`vv-set__nav-it ${stylex.props(settingsStyles.setNavIt).className}`}
        aria-current={n.id === section ? 'true' : undefined}
        onClick={() => onPick(n.id)}
      >
        <Icon name={n.icon} size={17} className="vv-ic" xstyle={[settingsStyles.ic]} />
        <span className={`vv-set__nav-label ${stylex.props(settingsStyles.setNavLabel).className}`}>{n.label}</span>
        {dots.has(n.id) ? <span className={`vv-dot ${stylex.props(settingsStyles.dot, DOT_STYLE[dots.get(n.id)!]).className}`} aria-label={t.shell.settingsAttention(1)} /> : null}
        {/* Artboard luôn giữ span số ở cuối (kể cả rỗng) nên chấm không bị đẩy sát mép. */}
        <span className={`vv-set__nav-num ${stylex.props(settingsStyles.setNavNum).className}`}>{n.count ?? ''}</span>
      </button>
    ))
  return (
    <>
      <div className={`vv-set__grp ${stylex.props(settingsStyles.setGrp).className}`}>{facts.server ? serverCopy.server : t.settings.nav.machine}</div>
      {render(machine)}
      <div className={`vv-set__grp ${stylex.props(settingsStyles.setGrp).className}`}>{t.settings.nav.data}</div>
      {render(data)}
      <div className={`vv-set__grp ${stylex.props(settingsStyles.setGrp).className}`}>{t.settings.nav.elsewhere}</div>
      <a className={`vv-set__nav-link ${stylex.props(setNavLinkMarker, settingsStyles.setNavLink).className}`} href={`#${paths.queue()}`}>
        <Icon name="routing" size={17} className="vv-ic" xstyle={[settingsStyles.ic]} />
        <span className={`vv-set__nav-label ${stylex.props(settingsStyles.setNavLabel).className}`}>{t.settings.nav.queue}</span>
        <Icon name="arrow-r" size={13} className="vv-ic" xstyle={[settingsStyles.ic]} />
      </a>
      {user?.role === 'admin' && memberHref && <MembersLink href={memberHref} settings />}
    </>
  )
}

export function SettingsScreen({ route }: { route: { name: 'settings'; section?: SettingsSection } }) {
  return <SettingsFrame section={route.section} />
}

export default SettingsScreen

function SettingsFrame({ section }: { section?: SettingsSection }) {
  const mobile = useMediaQuery('(max-width: 767px)')
  const dots = useSectionDots()
  const facts = useFrameFacts()
  const active: SettingsSection = section ?? 'tools'
  const meta = active === 'notifications' ? { title: 'Thông báo', lede: 'Việc nền & thông báo' } : t.settings.meta[active] ?? t.settings.meta.tools
  const help = facts.server && active === 'storage' ? serverCopy.help : active === 'notifications' ? t.settings.help.tools : t.settings.help[active] ?? t.settings.help.tools
  const go = (id: SettingsSection) => navigate(routePath({ name: 'settings', section: id }))

  if (mobile) return <MobileSettings section={section} dots={dots} facts={facts} go={go} />

  return (
    <div className={`vv-set ${stylex.props(settingsStyles.set).className}`}>
      <aside className={`vv-set__side ${stylex.props(settingsStyles.setSide).className}`} aria-label={t.shell.settings}>
        <NavList section={active} dots={dots} facts={facts} onPick={go} />
        <div className={`vv-set__foot ${stylex.props(settingsStyles.setFoot).className}`}>
          <span className={`vv-set__foot-note ${stylex.props(settingsStyles.setFootNote).className}`}>{t.settings.nav.footerNote}</span>
          <span className={`vv-set__foot-ver num ${stylex.props(settingsStyles.setFootVer).className}`}>{fill(t.settings.nav.version, { version: facts.version })}</span>
        </div>
      </aside>
      <main className={`vv-set__center vv-wide vv-wide--read ${stylex.props(settingsStyles.setCenter).className}`}>
        <div className={`vv-set__body ${stylex.props(settingsStyles.setBody).className}`}>
          {!(facts.server && active === 'storage') ? <div>
            <h1 className={`vv-set__title ${stylex.props(settingsStyles.setTitle).className}`}>{meta.title}</h1>
            <p className={`vv-set__lede ${stylex.props(settingsStyles.setLede).className}`}>{meta.lede}</p>
          </div> : null}
          <Suspense fallback={<Spinner size={16} />}>
            <SectionBody section={active} />
          </Suspense>
        </div>
      </main>
      <aside className={`vv-set__help ${stylex.props(settingsStyles.setHelp).className}`} aria-label={t.shell.settings}>
        <section className={`vv-set__help-card ${stylex.props(settingsStyles.card, setHelpCardMarker, settingsStyles.setHelpCard).className}`}>
          <h2 {...stylex.props(settingsStyles.VvSetHelpCardH2)}>{help.title}</h2>
          {help.lines.map((line) => <p key={line} {...stylex.props(settingsStyles.VvSetHelpCardP)}>{line}</p>)}
        </section>
        <div className={`vv-set__tips ${stylex.props(settingsStyles.setTips).className}`}>
          <span className={`vv-set__tips-title ${stylex.props(settingsStyles.setTipsTitle).className}`}>{help.tips.title}</span>
          {help.tips.items.map((tip) => (
            <span key={tip} className={`vv-set__tip ${stylex.props(settingsStyles.setTip).className}`}><span aria-hidden="true">•</span><span>{tip}</span></span>
          ))}
        </div>
      </aside>
    </div>
  )
}


/** MSettings — thẻ tổng, danh sách mục kèm giá trị, thẻ Hàng chờ, chân bản. */
function MobileSettings({ section, dots, facts, go }: { section?: SettingsSection; dots: Map<string, AttentionTone>; facts: FrameFacts; go: (id: SettingsSection) => void }) {
  const { user, apps } = useSession()
  const memberHref = membersUrl(apps)
  const status = useStatus()
  const strip = user ? <AdminStorageStrip user={user} status={status.data} /> : null
  // Câu phụ của thẻ “Máy sẵn sàng” — theo việc nặng nhất đang có.
  const summarySub = facts.failed > 0
    ? fill(t.settings.m.summary.failedSub, { n: facts.failed })
    : facts.installing > 0
      ? fill(t.settings.m.summary.installingSub, { n: facts.installing })
      : facts.updates > 0
        ? fill(t.settings.m.summary.updateSub, { n: facts.updates })
        : t.settings.m.summary.okSub
  const toolsValue = facts.installing > 0
    ? fill(t.settings.m.values.toolsInstalling, { n: facts.installing })
    : facts.updates > 0
      ? fill(t.settings.m.values.updates, { n: facts.updates })
      : t.settings.m.values.toolsOk
  const rows: { id: SettingsSection | 'export'; icon: string; title: string; value: string; warm?: boolean }[] = [
    { id: 'tools', icon: 'cpu', title: t.settings.m.rows.tools.title, value: toolsValue, warm: facts.updates > 0 },
    { id: 'connect', icon: 'key', title: t.settings.m.rows.connect.title, value: facts.providers == null ? '' : fill(t.settings.m.values.providers, { n: facts.providers }) },
    { id: 'agents', icon: 'magic', title: t.settings.m.rows.agents.title, value: facts.artist == null ? '' : facts.artist ? fill(t.settings.m.values.artist, { name: facts.artist }) : t.settings.m.values.artistNone },
    { id: 'accounts', icon: 'global', title: t.settings.m.rows.accounts.title, value: facts.platforms == null ? '' : fill(t.settings.m.values.platforms, { n: facts.platforms }) },
    { id: 'storage', icon: 'folder', title: t.settings.m.rows.storage.title, value: facts.storageMobile ?? (facts.storageCount || t.settings.counts.storageFallback) },
    { id: 'export', icon: 'flag', title: t.settings.m.rows.logs.title, value: t.settings.m.values.logs },
  ]

  if (section) {
    const meta = section === 'notifications' ? { title: 'Thông báo', lede: 'Việc nền & thông báo' } : t.settings.meta[section] ?? t.settings.meta.tools
    return (
      <div className={`vv-mset ${stylex.props(settingsStyles.mset).className}`}>
        {strip}
        <header className={`vv-mset__head ${stylex.props(settingsStyles.msetHead).className}`}>
          <button type="button" className={`vv-mset__back ${stylex.props(settingsStyles.msetBack).className}`} aria-label={t.settings.backMobile} onClick={() => navigate(paths.settings())}>
            <Icon name="arrow-r" size={16} style={{ transform: 'rotate(180deg)' }} />
          </button>
          <span className={`vv-mset__title ${stylex.props(settingsStyles.msetTitle).className}`}>{meta.title}</span>
        </header>
        <div className={`vv-mset__body ${stylex.props(settingsStyles.msetBody).className}`}>
          <Suspense fallback={<Spinner size={16} />}>
            <SectionBody section={section} />
          </Suspense>
        </div>
      </div>
    )
  }

  return (
    <div className={`vv-mset ${stylex.props(settingsStyles.mset).className}`}>
      {strip}
      <header className={`vv-mset__head ${stylex.props(settingsStyles.msetHead).className}`}>
        <button type="button" className={`vv-mset__back ${stylex.props(settingsStyles.msetBack).className}`} aria-label={t.settings.backMobile} onClick={() => navigate(paths.workbench())}>
          <Icon name="arrow-r" size={16} style={{ transform: 'rotate(180deg)' }} />
        </button>
        <span className={`vv-mset__title ${stylex.props(settingsStyles.msetTitle).className}`}>{t.shell.settings}</span>
      </header>
      <div className={`vv-mset__body ${stylex.props(settingsStyles.msetBody).className}`}>
        <div className={`vv-mset__sum ${stylex.props(settingsStyles.card, settingsStyles.msetSum).className}`}>
          <span className={`vv-mset__sum-ic ${stylex.props((facts.failed > 0) && isWarnMarker, settingsStyles.msetSumIc, (facts.failed > 0) && settingsStyles.msetSumIcIsWarn).className}`} aria-hidden="true">
            <Icon name={facts.failed > 0 ? 'danger' : 'tick-b'} size={20} />
          </span>
          <span className={`vv-mset__sum-main ${stylex.props(settingsStyles.msetSumMain).className}`}>
            <span className={`vv-mset__sum-title ${stylex.props(settingsStyles.msetSumTitle).className}`}>
              {facts.failed > 0 ? fill(t.settings.m.summary.failedTitle, { n: facts.failed }) : t.settings.m.summary.ok}
            </span>
            <span className={`vv-mset__sum-sub ${stylex.props(settingsStyles.msetSumSub).className}`}>{summarySub}</span>
          </span>
        </div>
        <nav className={`vv-mset__list ${stylex.props(settingsStyles.msetList, box.card).className}`} aria-label={t.shell.settings}>
          {rows.map((r) => (
            <button
              key={r.id}
              type="button"
              className={`vv-mset__row ${stylex.props(msetRowMarker, settingsStyles.msetRow).className}`}
              onClick={() => { if (r.id === 'export') window.location.assign(logsExportUrl); else go(r.id as SettingsSection) }}
            >
              <Icon name={r.icon} size={19} className="vv-ic" xstyle={[settingsStyles.ic]} />
              <span className={`vv-mset__row-label ${stylex.props(settingsStyles.msetRowLabel).className}`}>{r.title}</span>
              <span className={`vv-mset__row-value-wrap ${stylex.props(settingsStyles.msetRowValueWrap).className}`}>
                {dots.has(r.id) ? <span className={`vv-dot ${stylex.props(settingsStyles.dot, DOT_STYLE[dots.get(r.id)!]).className}`} aria-label={t.shell.settingsAttention(1)} /> : null}
                <span className={`vv-mset__row-value ${stylex.props(settingsStyles.msetRowValue, (r.warm) && settingsStyles.msetRowValueIsWarm).className}`}>{r.value}</span>
              </span>
              <Icon name="arrow-r" size={15} className="vv-ic vv-mset__row-caret" xstyle={[settingsStyles.ic, settingsStyles.msetRowCaret]} />
            </button>
          ))}
        </nav>
        <a className={`vv-mset__queue ${stylex.props(settingsStyles.card, msetQueueMarker, settingsStyles.msetQueue).className}`} href={`#${paths.queue()}`}>
          <Icon name="routing" size={19} className="vv-ic" xstyle={[settingsStyles.ic]} />
          <span className={`vv-mset__queue-main ${stylex.props(settingsStyles.msetQueueMain).className}`}>
            <span className={`vv-mset__queue-title ${stylex.props(settingsStyles.msetQueueTitle).className}`}>{t.settings.m.rows.queue.title}</span>
            <span className={`vv-mset__queue-hint ${stylex.props(settingsStyles.msetQueueHint).className}`}>{t.settings.m.rows.queue.hint}</span>
          </span>
          <Icon name="arrow-r" size={15} className="vv-ic" xstyle={[settingsStyles.ic]} />
        </a>
        {user?.role === 'admin' && memberHref && <MembersLink href={memberHref} settings />}
        <span className={`vv-mset__foot ${stylex.props(settingsStyles.msetFoot).className}`}>{fill(t.settings.m.foot, { version: facts.version })}</span>
      </div>
    </div>
  )
}
