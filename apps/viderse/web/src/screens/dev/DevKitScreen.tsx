// '#/dev/kit' — trang tra cứu kiểu thật (chỉ dev, 08-dev-prompt): dựng lại
// bảng Kit (máy tính) và MKit (điện thoại) của design bằng component thật
// của app + kit, không phải bản chụp. Chọn bảng/nhóm bằng query
// ?board=&section= để link đến đúng chỗ.
import { useMemo, useState, type ReactNode } from 'react'
import * as stylex from '@stylexjs/stylex'
import {
  ActionMenu, Banner, Card, Chip, Combobox, EmptyState, KitSelect,
  InfoPopover, NumberStepper, Segmented, Sheet, Switch, TextField, Tooltip, toast, Icon,
} from '@noverse/ui'
import { t } from '../../i18n/vi'
import { navigate, paths, type DevKitBoard, type Route } from '../../router'
import {
  ActionButton, ChannelAvatar, EmptyWorkbench, LensLogo, Orb, StatusLabel,
  SystemLoading, SystemNotFound,
  confirmAutonomy, confirmDeleteChannel, confirmDeleteVideo, confirmLeaveUnsaved,
  confirmOverwrite, confirmPause, confirmProviderLogout, confirmStopAll,
  type Assistant, ASSISTANTS, ASSISTANT_ORB as ASSISTANT_ORB_MAP, assistantName,
} from '../../components'
import { AppHeader } from '../../shell/AppHeader'
import { JobsPill } from '../../shell/JobsPill'
import { ChannelSwitcher } from '../../shell/ChannelSwitcher'
import { OfflineStrip } from '../../shell/OfflineStrip'
import { MobileTabBar, mobileShellStyles } from '../../shell/MobileShell'
import { useOnline } from '../../data/status'
import type { Status } from '../../api/status'
import { TokenSheet } from './TokenSheet'

const KIT_SECTIONS = ['foundation', 'confirm', 'menus', 'select', 'inputs', 'overlays', 'video', 'agent'] as const
const MKIT_SECTIONS = ['bars', 'controls', 'lists', 'sheets', 'agent'] as const
type KitSection = (typeof KIT_SECTIONS)[number]
type MKitSection = (typeof MKIT_SECTIONS)[number]

export function DevKitScreen({ route }: { route: Extract<Route, { name: 'devkit' }> }) {
  const board: DevKitBoard = route.board ?? 'kit'
  const section = route.section ?? (board === 'kit' ? 'foundation' : 'bars')
  const go = (next: { board?: DevKitBoard; section?: string }) =>
    navigate(paths.devkit(next.board ?? board, next.section ?? section), true)

  const boardItems = [
    { id: 'kit' as const, label: t.devkit.boards.kit },
    { id: 'mkit' as const, label: t.devkit.boards.mkit },
  ]
  const kitSections = KIT_SECTIONS.map((id) => ({ id, label: t.devkit.groups[id] }))
  const mkitSections = MKIT_SECTIONS.map((id) => ({ id, label: t.devkit.groups[id] }))

  return (
    <div className="vv-dev">
      <div className="vv-dev__head">
        <h1 className="vv-dev__title">{t.devkit.title}</h1>
        <Segmented label="Bảng" options={boardItems.map((b) => ({ id: b.id, label: b.label }))} value={board} onChange={(id) => go({ board: id as DevKitBoard, section: undefined })} />
        <Segmented
          label="Nhóm"
          options={(board === 'kit' ? kitSections : mkitSections).map((s) => ({ id: s.id, label: s.label }))}
          value={section}
          onChange={(id) => go({ section: id })}
        />
      </div>
      {board === 'kit'
        ? <KitBoard section={KIT_SECTIONS.includes(section as KitSection) ? (section as KitSection) : 'foundation'} />
        : <MKitBoard section={MKIT_SECTIONS.includes(section as MKitSection) ? (section as MKitSection) : 'bars'} />}
    </div>
  )
}

const Cell = ({ title, wide, children }: { title: string; wide?: boolean; children: ReactNode }) => (
  <div className={wide ? 'vv-dev__cell vv-dev__cell--wide' : 'vv-dev__cell'}>
    <span className="vv-dev__cell-head">{title}</span>
    {children}
  </div>
)
const Row = ({ children }: { children: ReactNode }) => <div className="vv-dev__row">{children}</div>

// Trạng thái đầu trang mẫu cho các khung demo.
const DEMO_STATUS: Status = {
  running: 2, queued: 5, paused: false, reviewCount: 3,
  attention: [{ kind: 'login_expired', text: 'Đăng nhập ChatGPT hết hạn', href: '#/settings?section=connect' }],
}
const DEMO_PAUSED: Status = { ...DEMO_STATUS, running: 0, paused: true }
const DEMO_CHANNELS = [
  { id: 'kenh-thuc-te', name: 'Kênh Thực Tế', initials: 'KT', color: { bg: 'rgba(230,0,46,.22)', ring: 'rgba(230,0,46,.6)' }, pace: 'Hôm nay 3/5', waiting: true },
  { id: 'doc-la', name: 'Độc Lạ Studio', initials: 'ĐL', color: { bg: 'rgba(127,163,255,.2)', ring: 'rgba(127,163,255,.55)' }, pace: 'Hôm nay 1/7' },
]

function KitBoard({ section }: { section: KitSection }) {
  switch (section) {
    case 'foundation': return <KitFoundation />
    case 'confirm': return <KitConfirm />
    case 'menus': return <KitMenus />
    case 'select': return <KitSelects />
    case 'inputs': return <KitInputs />
    case 'overlays': return <KitOverlays />
    case 'video': return <KitVideo />
    case 'agent': return <KitAgents />
  }
}

function MKitBoard({ section }: { section: MKitSection }) {
  return (
    <div className="vv-dev__mobile">
      {section === 'bars' && <MBars />}
      {section === 'controls' && <MControls />}
      {section === 'lists' && <MLists />}
      {section === 'sheets' && <MSheets />}
      {section === 'agent' && <MAgents />}
      <MobileTabBar route={{ name: 'workbench' }} reviewCount={DEMO_STATUS.reviewCount} fixed={false} />
    </div>
  )
}

// ─── Kit › Nền tảng ────────────────────────────────────────────────────────

function KitFoundation() {
  return (
    <>
      <TokenSheet />
      <div className="vv-dev__grid" style={{ marginTop: 'var(--vv-s-14)' }}>
      <Cell title="Logo ống kính">
        <Row>
          <LensLogo size={26} />
          <LensLogo size={26} spinning />
          <LensLogo size={64} />
          <LensLogo size={16} />
        </Row>
        <Row>
          <span style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>đứng yên · xoay khi có job · 64px · 16px</span>
        </Row>
      </Cell>
      <Cell title="Trợ lý — orb & màu chữ">
        <Row>
          {ASSISTANTS.map((a) => (
            <span key={a} style={{ display: 'inline-flex', alignItems: 'center', gap: 'var(--vv-s-6)', fontSize: 12.5 }}>
              <Orb of={a} size={18} />
              {assistantName(a)}
            </span>
          ))}
        </Row>
        <Row>
          {ASSISTANTS.map((a) => <Orb key={a} of={a} size={18} live />)}
          <Orb of="artist" size={18} dim />
        </Row>
      </Cell>
      <Cell title="Nhãn trạng thái">
        {(['queued', 'working', 'review', 'issue', 'paused', 'ready', 'posted'] as const).map((s) => (
          <Row key={s}><StatusLabel status={s} hint={s === 'working' ? 'Hoạ sĩ đang vẽ cảnh 5/6' : undefined} /></Row>
        ))}
      </Cell>
      <Cell title="Chip">
        <Row>
          <Chip label="Đang làm" />
          <Chip label="Hàng chờ" />
          <Chip label="Hôm nay" />
          <Chip label="Mẫu" />
        </Row>
      </Cell>
      <Cell title="Avatar kênh">
        <Row>
          {DEMO_CHANNELS.map((c) => <ChannelAvatar key={c.id} name={c.name} initials={c.initials} color={c.color} size={30} />)}
          <ChannelAvatar name="Đối Thủ" initials="ĐT" color={{ bg: 'rgba(93,221,176,.18)' }} size={30} />
          <ChannelAvatar name="Kênh mới" size={30} />
        </Row>
      </Cell>
      <Cell title="Icon (bộ kit — Iconsax Linear/Bold)">
        <Row>
          {(['workbench', 'flag', 'channel', 'radar', 'video-add', 'setting', 'gallery', 'magicpen', 'scan', 'send', 'trend', 'lamp'] as const).map((name) => (
            <Tooltip key={name} content={name}><Icon name={name} size={18} /></Tooltip>
          ))}
        </Row>
      </Cell>
      </div>
    </>
  )
}

// ─── Kit › Xác nhận (7 tình huống) ─────────────────────────────────────────

function KitConfirm() {
  const online = useOnline()
  const [log, setLog] = useState<string[]>([])
  const note = (line: string) => setLog((l) => [line, ...l].slice(0, 6))
  const run = (label: string) => () => note(`✓ ${label}`)

  const cases: { id: string; label: string; sub: string; open: () => void }[] = [
    {
      id: 'pause', label: 'Tạm dừng trợ lý', sub: 'radio + Không hỏi lại',
      open: () => void confirmPause({ agent: 'artist', video: 'Midnight Tales', scene: 'cảnh 5/6', remember: { user: 'demo', kind: 'pause' }, onConfirm: run('Tạm dừng') }),
    },
    {
      id: 'overwrite', label: 'Ghi đè MP4', sub: 'so 2 bản + Giữ cả hai',
      open: () => void confirmOverwrite({
        file: 'Midnight-Tales.mp4',
        current: { summary: '38:12 · 172 MB', meta: 'xuất hôm qua 21:40' },
        next: { summary: '38:12 · 176 MB', meta: 'bản sửa mới, chữa 2 cảnh' },
        onConfirm: run('Thay bản cũ'), onKeepBoth: run('Giữ cả hai'),
      }),
    },
    {
      id: 'logout', label: 'Đăng xuất nhà cung cấp', sub: 'cam nhạt, nút chủ động',
      open: () => void confirmProviderLogout({ provider: 'ChatGPT', fallback: 'Gemini', onConfirm: run('Đăng xuất ChatGPT') }),
    },
    {
      id: 'video', label: 'Xoá video', sub: 'hệ quả 2 cột + nút cam đặc',
      open: () => void confirmDeleteVideo({
        name: 'Midnight Tales', meta: 'Sẵn sàng · MP4 38:12 · 172 MB',
        deleting: ['File MP4 đã xuất', '16 ảnh cảnh + 3 ảnh bìa', 'Project kiểm định', 'Bản ghi video'],
        keeping: ['Video nguồn + phụ đề', 'Clip nền đã tải', 'Kênh và template'],
        onConfirm: run('Xoá video'),
      }),
    },
    {
      id: 'channel', label: 'Xoá kênh', sub: 'gõ đúng tên + lý do khoá',
      open: () => void confirmDeleteChannel({
        name: 'Độc Lạ Studio', runningCount: 2,
        consequences: ['Hàng chờ và lịch sử video của kênh', 'Liên kết 4 đối thủ', 'Template riêng của kênh'],
        onConfirm: run('Xoá kênh'),
      }),
    },
    {
      id: 'stopAll', label: 'Dừng mọi việc', sub: 'giữ để xác nhận',
      open: () => void confirmStopAll({ running: 4, onConfirm: run('Dừng mọi việc') }),
    },
    {
      id: 'leave', label: 'Bỏ thay đổi chưa lưu', sub: 'nút phụ Bỏ thay đổi',
      open: () => void confirmLeaveUnsaved({ changed: 3, template: 'Ánh sáng neon', onConfirm: run('Lưu rồi rời'), onDiscard: run('Bỏ thay đổi') }),
    },
    {
      id: 'autonomy', label: 'Chuyển Tự chạy', sub: 'Dialogs · autonomy',
      open: () => void confirmAutonomy({ channel: 'Kênh Thực Tế', onConfirm: run('Chuyển Tự chạy') }),
    },
  ]

  return (
    <div className="vv-dev__grid">
      {cases.map((c) => (
        <Cell key={c.id} title={c.label}>
          <span style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>{c.sub}</span>
          <Row>
            <ActionButton variant={c.id === 'video' || c.id === 'channel' ? 'danger' : 'primary'} disabled={!online} disabledReason="Mất kết nối" onClick={c.open}>
              {c.label}
            </ActionButton>
          </Row>
        </Cell>
      ))}
      <Cell title="Nhật ký demo" wide>
        {log.length === 0
          ? <span style={{ fontSize: 12.5, color: 'var(--color-fg-4)' }}>Xác nhận một hộp để thấy kết quả.</span>
          : log.map((l, i) => <span key={i} className="num" style={{ fontSize: 12.5 }}>{l}</span>)}
      </Cell>
    </div>
  )
}

// ─── Kit › Menu ────────────────────────────────────────────────────────────

function KitMenus() {
  const items = [
    { id: 'redo', label: 'Làm lại từ đầu', description: 'Xoá toàn bộ tiến độ', onSelect: () => toast.ok('Làm lại từ đầu') },
    { id: 'template', label: 'Đổi template', onSelect: () => toast.ok('Đổi template') },
    { id: 'pause', label: 'Tạm dừng', onSelect: () => toast.ok('Tạm dừng') },
    { id: 'delete', label: 'Xoá video', danger: true, onSelect: () => toast.ok('Xoá video') },
  ]
  return (
    <div className="vv-dev__grid">
      <Cell title="ActionMenu">
        <Row>
          <ActionMenu label="Thao tác" items={[...items]} trigger={<ActionButton variant="outline" icon={<Icon name="more" />}>Thao tác</ActionButton>} />
          <ActionMenu label="Icon chỉ" items={[...items]} trigger={<ActionButton variant="quiet" icon={<Icon name="more" />} aria-label="Thao tác" />} />
        </Row>
      </Cell>
      <Cell title="Bộ đổi kênh">
        <Row>
          <ChannelSwitcher channels={DEMO_CHANNELS} sub="Hôm nay 4/7" onPick={() => {}} />
          <ChannelSwitcher channels={DEMO_CHANNELS} current="kenh-thuc-te" onPick={() => {}} />
        </Row>
      </Cell>
      <Cell title="Tooltip & gợi ý">
        <Row>
          <Tooltip content="Máy đang chạy 2/4 luồng"><ActionButton variant="quiet">Rê chuột</ActionButton></Tooltip>
          <InfoPopover>Xoá mọi ảnh, cảnh và bản nháp của video này rồi cho chạy lại từ lấy nguồn.</InfoPopover>
        </Row>
      </Cell>
    </div>
  )
}

// ─── Kit › Select & Combobox ───────────────────────────────────────────────

function KitSelects() {
  const [sel, setSel] = useState('balanced')
  const [combo, setCombo] = useState('all')
  return (
    <div className="vv-dev__grid">
      <Cell title="KitSelect">
        <KitSelect
          label="Template ảnh cảnh"
          options={[
            { value: 'careful', label: 'Phim ảnh', description: 'Ánh sáng cinematic, hạt film' },
            { value: 'balanced', label: 'Hoạt hình mực', description: 'Nét mực + màu nước' },
            { value: 'auto', label: 'Ảnh thật', description: 'Bố cục tài liệu' },
          ]}
          value={sel}
          onChange={setSel}
        />
      </Cell>
      <Cell title="Combobox">
        <Combobox
          label="Kênh đối thủ"
          options={[
            { value: 'all', label: 'Mọi kênh' },
            { value: 'a', label: 'Mọt Phim', description: '@motphim' },
            { value: 'b', label: 'One Cine', description: '@onecine' },
          ]}
          value={combo}
          onChange={setCombo}
        />
      </Cell>
    </div>
  )
}

// ─── Kit › Ô nhập & nút ───────────────────────────────────────────────────

function KitInputs() {
  const [text, setText] = useState('')
  const [n, setN] = useState(3)
  const [on, setOn] = useState(true)
  const online = useOnline()
  return (
    <div className="vv-dev__grid">
      <Cell title="ActionButton (kit Button)">
        <Row>
          <ActionButton variant="primary">Làm video</ActionButton>
          <ActionButton variant="outline">Xem trước</ActionButton>
          <ActionButton variant="quiet">Bỏ qua</ActionButton>
          <ActionButton variant="danger-soft">Làm lại từ đầu</ActionButton>
          <ActionButton variant="danger">Xoá</ActionButton>
        </Row>
        <Row>
          <ActionButton variant="primary" pending pendingLabel="Đang thêm…">Thêm vào hàng chờ</ActionButton>
          <ActionButton variant="outline" success successLabel="Đã lưu">Lưu</ActionButton>
          <ActionButton variant="primary" disabled>Primary khoá</ActionButton>
          <ActionButton variant="outline" disabled={!online} disabledReason={!online ? 'Mất kết nối' : undefined}>Ghi (theo kết nối)</ActionButton>
        </Row>
      </Cell>
      <Cell title="Ô nhập">
        <TextField label="Dán link video" placeholder="https://youtube.com/watch?v=…" value={text} onChange={setText} />
        <TextField label="Có lỗi" value="abc" onChange={() => {}} error="Link không hợp lệ" />
      </Cell>
      <Cell title="Stepper & công tắc">
        <Row>
          <NumberStepper value={n} onChange={setN} min={1} max={50} />
          <Switch checked={on} onChange={setOn} label="Tự chạy" />
        </Row>
      </Cell>
      <Cell title="Banner">
        <Banner tone="warning" title="3 video cần xử lý" action={{ label: 'Mở Bàn dựng', onClick: () => {} }}>
          Kiểm định xin bạn sửa lại trước khi dựng tiếp.
        </Banner>
        <Banner tone="agent" agent={ASSISTANT_ORB_MAP.artist} title="Hoạ sĩ" action={{ label: 'Xem cảnh', onClick: () => {} }}>
          Đã vẽ xong 6 cảnh, bạn duyệt nhé.
        </Banner>
        <Banner tone="error" title="YouTube từ chối cookie" action={{ label: 'Đăng nhập lại', onClick: () => {} }} />
      </Cell>
      <Cell title="Toast">
        <Row>
          <ActionButton variant="outline" onClick={() => toast.ok('Đã thêm “Midnight Tales” vào hàng chờ')}>toast.ok</ActionButton>
          <ActionButton variant="outline" onClick={() => toast('3 video chờ duyệt', 'neutral', { label: 'Mở Duyệt', onClick: () => {} })}>toast + nút</ActionButton>
          <ActionButton variant="outline" onClick={() => toast.card({ title: '“Midnight Tales” cần bạn', text: 'Kiểm định xin sửa 2 cảnh trước khi dựng tiếp.', action: { label: 'Mở Bàn dựng', onClick: () => {} } })}>toast.card</ActionButton>
        </Row>
      </Cell>
    </div>
  )
}

// ─── Kit › Gợi ý & trạng thái ─────────────────────────────────────────────

function KitOverlays() {
  return (
    <div className="vv-dev__grid">
      <Cell title="Trống & 404">
        <EmptyState variant="frame" icon="flag" title="Đã duyệt hết" hint="Video mới cần duyệt sẽ hiện ở đây." />
        <EmptyState icon="search-normal-1" title="Không tìm thấy video này" hint="Video có thể đã bị xoá." />
      </Cell>
      <Cell title="Dải mất kết nối (demo độc lập)" wide>
        <DemoOfflineStrip />
      </Cell>
      <Cell title="Skeleton trạng thái tải" wide>
        <SystemLoading />
      </Cell>
      <Cell title="Trạng thái trống Bàn dựng" wide>
        <EmptyWorkbench />
      </Cell>
    </div>
  )
}

// Bản demo tĩnh của OfflineStrip (bản thật đọc trạng thái SSE).
function DemoOfflineStrip() {
  const [details, setDetails] = useState(false)
  return (
    <div style={{ border: '1px solid var(--line-1)', borderRadius: 'var(--radius-row)', overflow: 'hidden' }}>
      <div role="alert" className="vv-offline" style={{ borderBottom: 'none' }}>
        <span aria-hidden="true" className="vv-offline__dot" />
        <strong style={{ fontWeight: 600 }}>{t.offline.title}</strong>
        <span style={{ color: 'var(--color-fg-4)', fontSize: 12.5 }}>· {t.offline.retryingIn(4)}</span>
        <span style={{ flex: 1 }} />
        <button type="button" aria-expanded={details} onClick={() => setDetails((v) => !v)} className="vv-offline__btn">{t.common.details}</button>
        <button type="button" className="vv-offline__btn">{t.common.retryNow}</button>
        {details && <div className="vv-offline__more">{t.offline.body}</div>}
      </div>
    </div>
  )
}

// ─── Kit › Video & thẻ ─────────────────────────────────────────────────────

function KitVideo() {
  return (
    <div className="vv-dev__grid">
      <Cell title="JobsPill">
        <Row>
          <JobsPill status={DEMO_STATUS} online />
          <JobsPill status={DEMO_PAUSED} online />
          <JobsPill status={{ ...DEMO_STATUS, running: 0, queued: 0 }} online />
          <JobsPill status={DEMO_STATUS} online={false} />
          <JobsPill status={DEMO_STATUS} online unread />
        </Row>
      </Cell>
      <Cell title="Đầu trang ứng dụng" wide>
        <div style={{ border: '1px solid var(--line-1)', borderRadius: 'var(--radius-row)', overflow: 'hidden' }}>
          <AppHeader route={{ name: 'workbench' }} status={DEMO_STATUS} online channels={DEMO_CHANNELS} />
        </div>
      </Cell>
      <Cell title="Thẻ video (nháp nhanh)">
        <Card style={{ width: 220, padding: 0, overflow: 'hidden' }}>
          <div style={{ aspectRatio: '16/9', background: 'linear-gradient(140deg, rgba(230,0,46,.25), rgba(127,163,255,.18))' }} />
          <div style={{ padding: 'var(--vv-s-10)', display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-4)' }}>
            <span style={{ fontSize: 13, fontWeight: 600 }}>Midnight Tales</span>
            <StatusLabel status="working" hint="Hoạ sĩ đang vẽ cảnh 5/6" />
          </div>
        </Card>
      </Cell>
    </div>
  )
}

// ─── Kit › Trợ lý ─────────────────────────────────────────────────────────

function KitAgents() {
  return (
    <div className="vv-dev__grid">
      {ASSISTANTS.map((a) => (
        <Cell key={a} title={assistantName(a)}>
          <Row>
            <Orb of={a} size={44} standalone />
            <span style={{ fontSize: 12.5, color: 'var(--color-fg-3)' }}>{t.agents[a].role}</span>
          </Row>
          <Banner tone="agent" agent={ASSISTANT_ORB_MAP[a]} title={assistantName(a)} action={{ label: 'Xem việc', onClick: () => {} }}>
            {t.agents[a].role}
          </Banner>
        </Cell>
      ))}
    </div>
  )
}

// ─── MKit ──────────────────────────────────────────────────────────────────

function MHeader({ title }: { title: string }) {
  return (
    <header role="banner" className={`vv-mbar ${stylex.props(mobileShellStyles.bar).className}`}>
      <LensLogo size={24} spinning />
      <span style={{ fontWeight: 600, fontSize: 15 }}>{title}</span>
      <span style={{ flex: 1 }} />
      <JobsPill mobileBar status={DEMO_STATUS} online />
      <a className={`vv-mavatar ${stylex.props(mobileShellStyles.avatar).className}`} aria-label={t.shell.account}><Icon name="user" size={15} /></a>
    </header>
  )
}

function MBars() {
  return (
    <>
      <MHeader title={t.nav.workbench} />
      <div className="vv-dev__mobile-body" style={{ padding: 'var(--vv-s-14)', display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-14)' }}>
        <Cell title="MobileAppBar">
          <ChannelSwitcher channels={DEMO_CHANNELS} sub="Hôm nay 4/7" onPick={() => {}} />
        </Cell>
        <Cell title="JobsPill các trạng thái">
          <Row>
            <JobsPill status={DEMO_STATUS} online />
            <JobsPill status={DEMO_PAUSED} online />
            <JobsPill status={DEMO_STATUS} online={false} />
          </Row>
        </Cell>
        <Cell title="OfflineStrip">
          <DemoOfflineStrip />
        </Cell>
      </div>
    </>
  )
}

function MControls() {
  const [on, setOn] = useState(true)
  const [seg, setSeg] = useState('link')
  const [n, setN] = useState(5)
  return (
    <>
      <MHeader title={t.devkit.groups.controls} />
      <div className="vv-dev__mobile-body" style={{ padding: 'var(--vv-s-14)', display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-14)' }}>
        <Cell title="Nút">
          <ActionButton variant="primary" style={{ width: '100%' }}>Làm video</ActionButton>
          <Row>
            <ActionButton variant="outline">Xem trước</ActionButton>
            <ActionButton variant="quiet">Bỏ qua</ActionButton>
            <ActionButton variant="danger-soft">Làm lại từ đầu</ActionButton>
          </Row>
        </Cell>
        <Cell title="Công tắc & phân đoạn">
          <Switch checked={on} onChange={setOn} label="Tự chạy" />
          <Segmented label="Nguồn" options={[{ id: 'link', label: 'Dán link' }, { id: 'multi', label: 'Nhiều video' }]} value={seg} onChange={setSeg} />
        </Cell>
        <Cell title="Bước số">
          <NumberStepper value={n} onChange={setN} min={1} max={50} />
        </Cell>
      </div>
    </>
  )
}

function MLists() {
  return (
    <>
      <MHeader title={t.devkit.groups.lists} />
      <div className="vv-dev__mobile-body" style={{ padding: 'var(--vv-s-14)', display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-8)' }}>
        {(['working', 'review', 'queued', 'ready'] as const).map((s, i) => (
          <div key={s} style={{ display: 'flex', gap: 'var(--vv-s-10)', padding: 'var(--vv-s-12)', borderRadius: 'var(--radius-tile)', border: '1px solid var(--line-1)', background: 'var(--color-surface-1)' }}>
            <div style={{ width: 96, borderRadius: 'var(--radius-control)', aspectRatio: '16/9', background: 'linear-gradient(140deg, rgba(230,0,46,.22), rgba(127,163,255,.16))', flex: '0 0 auto' }} />
            <span style={{ display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-4)', minWidth: 0 }}>
              <span style={{ fontSize: 13, fontWeight: 600 }}>Video mẫu {i + 1}</span>
              <StatusLabel status={s} hint={s === 'working' ? 'Hoạ sĩ đang vẽ cảnh 3/6' : undefined} />
            </span>
          </div>
        ))}
      </div>
    </>
  )
}

function MSheets() {
  const [open, setOpen] = useState(false)
  return (
    <>
      <MHeader title={t.devkit.groups.sheets} />
      <div className="vv-dev__mobile-body" style={{ padding: 'var(--vv-s-14)' }}>
        <ActionButton variant="outline" onClick={() => setOpen(true)}>Mở sheet</ActionButton>
        <Sheet open={open} onClose={() => setOpen(false)} title="Thao tác video">
          <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-10)' }}>
            <ActionButton variant="primary">Duyệt video</ActionButton>
            <ActionButton variant="outline">Đổi template</ActionButton>
            <ActionButton variant="danger-soft">Làm lại từ đầu</ActionButton>
          </div>
        </Sheet>
      </div>
    </>
  )
}

function MAgents() {
  return (
    <>
      <MHeader title={t.devkit.groups.agent} />
      <div className="vv-dev__mobile-body" style={{ padding: 'var(--vv-s-14)', display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-10)' }}>
        {ASSISTANTS.map((a) => (
          <Banner key={a} tone="agent" agent={ASSISTANT_ORB_MAP[a]} title={assistantName(a)} action={{ label: 'Xem', onClick: () => {} }}>
            {t.agents[a].role}
          </Banner>
        ))}
      </div>
    </>
  )
}
