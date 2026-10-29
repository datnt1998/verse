// A3 · Onboard.dc.html: layout and copy; all reads/actions remain in OnboardScreen.
import { useEffect, useState, type CSSProperties, type ReactNode } from 'react'
import { t } from '../../i18n/vi'
import { AgentOrb, type AgentKind } from '@noverse/ui'
import { Avatar, Button } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { Checkbox, RadioCard, Switch } from '@noverse/ui'
import { Segmented } from '@noverse/ui'
import { EmptyState, Skeleton, Spinner } from '@noverse/ui'
import { Combobox, KitSelect } from '@noverse/ui'
import { TextArea } from '@noverse/ui'
import { AuthInput, OrbHero, StrengthMeter, fmtDate } from '../auth/parts'
import { AGENT_PALETTE } from '../../ui/agentPalette'
import { PASTE_CODE_PLACEHOLDER } from '../../lib/authCode'
import { formatContextK } from '../../lib/modelContext'
import { EFFORTS, FORMAT_LABEL, ONBOARD_FORMATS, PASTE_CODE_HINT, PROVIDER_INTRO, PROVIDER_PRESENTATION, deviceCodeHint, effortLabel, formatDate, oauthChoices, oauthDescription, passwordStepCopy, type OnboardModel, type StepKey } from './model'
import { projectLabel, storyLabel } from '../../shell/labels'
import { MobileStep } from './MobileStep'

const cardStyle: CSSProperties = { borderRadius: 16, padding: '16px 18px', display: 'flex', flexDirection: 'column', gap: 12 }
const fieldStyle: CSSProperties = { display: 'flex', flexDirection: 'column', gap: 6 }
const smallButton: CSSProperties = { height: 32, padding: '0 12px', borderRadius: 8, fontSize: 12.5 }
const selectedBorder = 'rgba(201,166,245,.5)'
const selectedBackground = 'rgba(201,166,245,.06)'
const agents: { key: AgentKind; label: string; desc: string }[] = [
  { key: 'architect', label: AGENT_PALETTE.architect.name, desc: 'Lên dàn ý' },
  { key: 'writer', label: AGENT_PALETTE.writer.name, desc: 'Viết chương' },
  { key: 'editor', label: AGENT_PALETTE.editor.name, desc: 'Thẩm duyệt' },
  { key: 'arbiter', label: AGENT_PALETTE.arbiter.name, desc: 'Phân xử can thiệp' },
]
/** Onboard.dc.html › agents: mô tả ở thẻ bước Hồ sơ (khác lời mô tả vai trò ở bước Model). */
const PROFILE_AGENTS: { key: AgentKind; label: string; desc: string }[] = [
  { key: 'architect', label: AGENT_PALETTE.architect.name, desc: 'Lên dàn ý, nhân vật, chi tiết cài cắm' },
  { key: 'writer', label: AGENT_PALETTE.writer.name, desc: 'Viết từng chương theo kế hoạch' },
  { key: 'editor', label: AGENT_PALETTE.editor.name, desc: 'Thẩm duyệt, yêu cầu viết lại' },
  { key: 'arbiter', label: AGENT_PALETTE.arbiter.name, desc: 'Phân xử khi bạn can thiệp' },
]

function Card({ children, style }: { children: ReactNode; style?: CSSProperties }) {
  return <section className="card" style={{ ...cardStyle, ...style }}>{children}</section>
}
function Heading({ title, children }: { title: string; children: ReactNode }) {
  return <div><h1 style={{ margin: 0, fontSize: 22, fontWeight: 600, letterSpacing: '-.015em' }}>{title}</h1><p style={{ margin: '6px 0 0', fontSize: 14, color: 'var(--color-fg-3)', maxWidth: 560 }}>{children}</p></div>
}
function Note({ children, warn = false }: { children: ReactNode; warn?: boolean }) {
  return <div style={{ display: 'flex', gap: 10, padding: '10px 12px', borderRadius: 12, background: warn ? 'rgba(246,203,140,.06)' : '#0F0F11', border: `1px solid ${warn ? 'rgba(246,203,140,.18)' : 'var(--line-1)'}`, fontSize: 12, color: 'var(--color-fg-2)' }}><Icon name={warn ? 'warning-2' : 'info-circle'} size={15} style={{ color: warn ? 'var(--color-warn)' : 'var(--color-fg-3)', marginTop: 1 }} /><span>{children}</span></div>
}
function SwitchRow({ checked, onChange, title, description, icon }: { checked: boolean; onChange: (checked: boolean) => void; title: string; description: string; icon?: string }) {
  return <div style={{ width: '100%', display: 'flex', alignItems: 'center', gap: 12, padding: '10px 12px', borderRadius: 10, color: 'var(--color-fg)' }}>
    {icon && <Icon name={icon} style={{ color: 'var(--color-fg-3)' }} />}
    <span style={{ flex: 1, display: 'flex', flexDirection: 'column' }}><span style={{ fontSize: 13.5, fontWeight: 500 }}>{title}</span><span style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>{description}</span></span>
    <Switch checked={checked} onChange={onChange} label={title} size="sm" tone="mono" />
  </div>
}

export function StepBody({ model }: { model: OnboardModel }) {
  switch (model.stepKey) {
    case 'profile': return <ProfileStep model={model} />
    case 'password': return <PasswordStep model={model} />
    case 'provider': return <ProviderStep model={model} />
    case 'model': return <ModelStep model={model} />
    case 'library': return <LibraryStep model={model} />
    case 'project': return <ProjectStep model={model} />
    case 'mobile': return <MobileStep model={model} />
    default: return <DoneStep model={model} />
  }
}

function ProfileStep({ model }: { model: OnboardModel }) {
  const { user } = model
  return <>
    <div style={{ display: 'flex', alignItems: 'center', gap: 18 }}><OrbHero /><div><h1 style={{ margin: 0, fontSize: 26, fontWeight: 600, letterSpacing: '-.015em' }}>{model.isAdmin ? `Chào mừng đến Noverse, ${model.name.trim() || user.name || user.username}` : `Chào ${user.name || user.username}`}</h1><p style={{ margin: '6px 0 0', fontSize: 14, color: 'var(--color-fg-3)' }}>{model.isAdmin ? 'Vài bước để bốn trợ lý AI sẵn sàng viết truyện. Mọi thứ đổi lại được trong Tuỳ chỉnh.' : Object.values(user.projects).some((role) => role === 'owner' || role === 'collab') ? 'Bạn là Người viết: viết và can thiệp các truyện được giao, tạo truyện mới.' : 'Tài khoản của bạn đã sẵn sàng — chỉ còn đặt mật khẩu riêng và kết nối điện thoại.'}</p></div></div>
    <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4,minmax(0,1fr))', gap: 8 }}>{PROFILE_AGENTS.map((agent) => <div key={agent.key} className="card" style={{ borderRadius: 14, padding: 12, display: 'flex', flexDirection: 'column', gap: 8 }}><AgentOrb agent={agent.key} size={22} /><span style={{ fontSize: 13, fontWeight: 600 }}>{agent.label}</span><span style={{ fontSize: 12, color: '#A0A0AB' }}>{agent.desc}</span></div>)}</div>
    <Card style={{ padding: 18, gap: 14 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}><Icon name="user" style={{ color: 'var(--color-fg-3)' }} /><span style={{ flex: 1, fontSize: 14, fontWeight: 600 }}>Hồ sơ của bạn</span><span style={{ height: 22, padding: '0 8px', display: 'inline-flex', alignItems: 'center', gap: 5, borderRadius: 6, background: model.isAdmin ? 'rgba(201,166,245,.12)' : 'rgba(185,208,247,.10)', color: model.isAdmin ? 'var(--color-accent)' : 'var(--color-info)', fontSize: 11.5 }}><Icon name={model.isAdmin ? 'crown' : 'edit-2'} size={12} />{t.role[user.role]}</span></div>
      <div style={{ display: 'flex', gap: 16, alignItems: 'flex-end', flexWrap: 'wrap' }}><Avatar name={model.name || user.username} size={56} /><div style={{ ...fieldStyle, flex: '1 1 220px' }}><label htmlFor="ob-name" className="lbl">Tên hiển thị</label><AuthInput id="ob-name" value={model.name} onChange={model.setName} placeholder="Tên hiện trên ảnh đại diện" /></div><div style={{ ...fieldStyle, flex: '0 1 180px' }}><span className="lbl">Tên đăng nhập</span><span className="mono" style={{ height: 38, display: 'flex', alignItems: 'center', padding: '0 12px', borderRadius: 10, background: '#131315', border: '1px dashed var(--line-3)', color: 'var(--color-fg-3)', fontSize: 13 }}>@{user.username}</span></div></div>
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px 18px', fontSize: 12.5, color: 'var(--color-fg-3)' }}><span style={{ display: 'flex', alignItems: 'center', gap: 6 }}><Icon name="shield-security" size={14} />Phiên trên máy này đến <span className="num" style={{ color: 'var(--color-fg-2)' }}>{model.sessionExpiresAt ? fmtDate(new Date(model.sessionExpiresAt)) : '—'}</span></span><span style={{ display: 'flex', alignItems: 'center', gap: 6 }}><Icon name="monitor" size={14} />{model.isAdmin ? 'Vừa tạo ở bước Lần chạy đầu' : user.createdAt ? `Tài khoản được tạo ngày ${formatDate(new Date(user.createdAt))}` : 'Do quản trị viên tạo'}</span></div>
    </Card>
    {!model.isAdmin && <Card style={{ gap: 10 }}><div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline' }}><span style={{ fontSize: 14, fontWeight: 600 }}>Truyện bạn được giao</span><span style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>do quản trị viên cấp</span></div>{model.assignedLoading ? <Skeleton height={64} /> : model.assigned.length ? model.assigned.map(({ project, story }) => <div key={story.id} style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '10px 12px', borderRadius: 12, background: '#0F0F11', border: '1px solid var(--line-1)' }}><Icon name="book-1" style={{ color: 'var(--color-fg-3)' }} /><span style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column' }}><span style={{ fontSize: 13.5 }}>{storyLabel(story)}</span><span style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>Dự án {projectLabel(project, project.id)}</span></span><span style={{ fontSize: 12, color: 'var(--color-ok)' }}>{!Object.values(user.projects).some((role) => role === 'owner' || role === 'collab') ? 'Chỉ đọc' : 'Viết & can thiệp'}</span></div>) : <EmptyState icon="book-1" title="Chưa có truyện nào được giao" hint="Quản trị viên giao truyện trong Tài khoản › Quản lý tài khoản." />}<span style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>{!Object.values(user.projects).some((role) => role === 'owner' || role === 'collab') ? 'Người đọc được đọc các truyện được giao; cài đặt nhà cung cấp AI do quản trị viên quản lý.' : 'Người viết được viết các truyện được giao và tạo truyện mới; cài đặt nhà cung cấp AI do quản trị viên quản lý.'}</span></Card>}
  </>
}

function PasswordStep({ model }: { model: OnboardModel }) {
  const mismatch = Boolean(model.pwRepeat && model.pwNext !== model.pwRepeat)
  const copy = passwordStepCopy(model.mustChangePassword)
  return <><Heading title={copy.title}>{copy.lead}</Heading><Card style={{ padding: 18, gap: 14, maxWidth: 460 }}><div style={fieldStyle}><label htmlFor="ob-pw-cur" className="lbl">{copy.currentLabel}</label><AuthInput id="ob-pw-cur" type="password" autoComplete="current-password" value={model.pwCurrent} onChange={model.setPwCurrent} invalid={Boolean(model.pwCurrentError)} aria-describedby={model.pwCurrentError ? 'ob-pw-cur-err' : undefined} />{model.pwCurrentError && <span id="ob-pw-cur-err" role="alert" style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 12, color: 'var(--color-err)' }}><Icon name="warning-2" size={14} />{model.pwCurrentError}</span>}</div><div style={fieldStyle}><label htmlFor="ob-pw-new" className="lbl">Mật khẩu mới</label><AuthInput id="ob-pw-new" type="password" autoComplete="new-password" placeholder="Ít nhất 8 ký tự" value={model.pwNext} onChange={model.setPwNext} /><StrengthMeter value={model.pwNext} /></div><div style={fieldStyle}><label htmlFor="ob-pw-re" className="lbl">Nhập lại</label><AuthInput id="ob-pw-re" type="password" autoComplete="new-password" value={model.pwRepeat} onChange={model.setPwRepeat} invalid={mismatch} />{mismatch && <span role="alert" style={{ fontSize: 12, color: 'var(--color-err)' }}>Hai lần nhập chưa khớp</span>}</div></Card></>
}

function ProviderStep({ model }: { model: OnboardModel }) {
  const [now, setNow] = useState(Date.now())
  const oauth = model.oauth
  useEffect(() => {
    if (oauth.phase !== 'browser' && oauth.phase !== 'device') return
    const timer = setInterval(() => setNow(Date.now()), 1000)
    return () => clearInterval(timer)
  }, [oauth.phase])
  if (!model.config && !model.presets.length) return <Skeleton height={280} radius={16} />
  const selected = model.preset
  const presentation = selected ? PROVIDER_PRESENTATION[selected.id] : null
  const oauthAccount = oauth.phase === 'done' ? oauth.account : model.accountInfo
  const waitSeconds = oauth.phase === 'browser' || oauth.phase === 'device' ? Math.max(0, Math.ceil((new Date(oauth.expiresAt).getTime() - now) / 1000)) : 0
  const loopback = /^(localhost|.+\.localhost|127\.0\.0\.1|\[::1\])$/.test(location.hostname)
  return <>
    <Heading title="Nhà cung cấp AI">Bốn trợ lý cần một nhà cung cấp AI để viết. {model.configuredProvider ? 'Noverse đã tìm thấy thiết lập bạn làm lúc cài đặt — kiểm tra lại hoặc đổi tại đây.' : PROVIDER_INTRO}</Heading>
    <Card style={{ borderColor: model.testState === 'ok' ? 'rgba(166,237,193,.3)' : undefined }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
        <span aria-hidden="true" style={{ width: 40, height: 40, borderRadius: 12, background: '#1E1E21', display: 'flex', alignItems: 'center', justifyContent: 'center', color: 'var(--color-fg-2)' }}><Icon name="cloud-connection" size={21} /></span>
        <div style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column' }}>
          <span style={{ fontSize: 14.5, fontWeight: 600 }}>{model.providerLabel}</span>
          <span style={{ fontSize: 12.5, color: 'var(--color-fg-4)' }}>
            {selected?.oauth
              ? model.accountInfo?.connected || oauth.phase === 'done'
                ? `Đã kết nối · ${oauthAccount?.email || oauthAccount?.label || ''}${oauthAccount?.plan ? ` · gói ${oauthAccount.plan}` : ''}`
                : 'Chưa kết nối'
              : presentation?.hint || model.providerDetail || 'Chưa có nhà cung cấp'}
          </span>
        </div>
        <span style={{ fontSize: 11.5, color: 'var(--color-fg-4)' }}>{model.changing ? 'chưa lưu' : 'thiết lập lúc cài đặt'}</span>
      </div>
      <div style={{ display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap' }}>
      {model.testState === 'run' ? <span role="status" style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 13, color: 'var(--color-fg-3)' }}><Spinner />Đang gọi thử model…</span> : model.testState === 'ok' ? <span role="status" style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 13, color: 'var(--color-ok)' }}><Icon name="tick-circle-bulk" size={16} />Kết nối được · <span className="num">{model.testLatency} ms</span> · {model.testModel}</span> : <Button disabled={model.busy || !model.providerId} onClick={() => { void model.runTest() }} style={{ ...smallButton, fontSize: 13 }}><Icon name="activity" size={15} />Kiểm tra kết nối</Button>}
      <span style={{ flex: 1 }} /><Button variant="ghost" aria-expanded={model.changing} onClick={() => model.setChanging(!model.changing)} style={{ ...smallButton, border: 'none', color: 'var(--color-fg-2)', fontSize: 13 }}><Icon name="arrow-swap-horizontal" size={15} />{model.changing ? 'Giữ cấu hình cũ' : 'Đổi nhà cung cấp'}</Button></div>{model.testState === 'fail' && <span role="alert" style={{ fontSize: 13, color: 'var(--color-err)' }}>{model.testMessage}</span>}</Card>
    {model.changing && <>
      <div role="radiogroup" aria-label="Nhà cung cấp" style={{ display: 'grid', gridTemplateColumns: 'repeat(3,minmax(0,1fr))', gap: 8 }}>{model.presets.map((preset) => { const copy = PROVIDER_PRESENTATION[preset.id]; const picked = model.providerId === preset.id; return <RadioCard key={preset.id} checked={picked} mark="none" onChange={() => model.setProviderId(preset.id)} style={{ flexDirection: 'column', gap: 2, padding: '10px 12px', borderRadius: 12, border: `1px solid ${picked ? selectedBorder : 'var(--line-3)'}`, background: picked ? selectedBackground : 'var(--color-surface-1)', minWidth: 0 }}><span style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 13, fontWeight: 500 }}>{copy?.label ?? preset.label}{preset.oauth && <Icon name="login" size={13} style={{ color: 'var(--color-accent)' }} />}</span><span style={{ fontSize: 11.5, color: 'var(--color-fg-4)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis', maxWidth: '100%' }}>{copy?.hint ?? (preset.oauth ? 'Gói thuê bao · đăng nhập OAuth' : preset.keyRequired ? 'API Key' : 'API Key · không bắt buộc')}</span></RadioCard> })}</div>
      {selected && <Card>{selected.oauth ? <>
        <span style={{ fontSize: 14, fontWeight: 600 }}>Đăng nhập {presentation?.label ?? selected.label}</span><span style={{ fontSize: 13, color: 'var(--color-fg-3)' }}>{oauthDescription(selected)}</span>
        {oauth.phase === 'idle' && <div style={{ display: 'grid', gridTemplateColumns: `repeat(${oauthChoices(selected).length},minmax(0,1fr))`, gap: 8 }}>{oauthChoices(selected).map((choice) => <Button key={choice.title} disabled={model.oauthBusy || !choice.device && !loopback} pending={model.oauthPending} onClick={() => model.startOauth(choice.device)} style={{ height: 'auto', minHeight: 78, padding: 12, borderRadius: 12, background: '#0F0F11', textAlign: 'left', whiteSpace: 'normal' }}><span style={{ display: 'flex', flexDirection: 'column', alignItems: 'stretch', gap: 4, width: '100%' }}><span style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 13, fontWeight: 500 }}><Icon name={choice.icon} size={16} />{choice.title}</span><span style={{ fontSize: 12, fontWeight: 400, color: 'var(--color-fg-4)' }}>{choice.desc}</span></span></Button>)}</div>}
        {oauth.phase === 'browser' && <span role="status" style={{ display: 'flex', alignItems: 'center', gap: 10, fontSize: 13, color: 'var(--color-fg-2)' }}><Spinner />Đang chờ đăng nhập ở tab mới · hết hạn sau <span className="mono num">{Math.floor(waitSeconds / 60)}:{String(waitSeconds % 60).padStart(2, '0')}</span></span>}
        {oauth.phase === 'device' && <><div style={{ display: 'flex', alignItems: 'center', gap: 14, flexWrap: 'wrap', padding: '12px 14px', borderRadius: 12, background: '#0F0F11', border: '1px solid var(--line-1)' }}><span className="mono" style={{ fontSize: 24, fontWeight: 500, letterSpacing: '.12em' }}>{oauth.userCode}</span><Button variant="icon-only" tooltip="Sao chép mã" onClick={() => { void navigator.clipboard?.writeText(oauth.userCode) }} style={{ width: 30, height: 30 }}><Icon name="copy" size={15} /></Button><span style={{ flex: 1, minWidth: 200, fontSize: 12.5, color: 'var(--color-fg-3)' }}>Mở trang <a className="mono" href={oauth.verifyURL || oauth.url} target="_blank" rel="noreferrer" style={{ color: 'var(--color-fg)' }}>{oauth.verifyURL || oauth.url}</a> {deviceCodeHint(selected.id)}</span></div><span role="status" style={{ display: 'flex', alignItems: 'center', gap: 10, fontSize: 13, color: 'var(--color-fg-3)' }}><Spinner />Đang chờ xác nhận…</span></>}
        {oauth.phase === 'paste' && <><div style={{ display: 'flex', alignItems: 'center', gap: 14, flexWrap: 'wrap', padding: '12px 14px', borderRadius: 12, background: '#0F0F11', border: '1px solid var(--line-1)' }}><AuthInput id="ob-oauth-code" mono aria-label={PASTE_CODE_PLACEHOLDER} aria-invalid={!!model.oauthCodeError} autoComplete="off" spellCheck={false} placeholder={PASTE_CODE_PLACEHOLDER} value={model.oauthCode} onChange={model.setOauthCode} onKeyDown={(event) => { if (event.key === 'Enter' && model.oauthCode.trim()) { event.preventDefault(); model.submitOauthCode() } }} style={{ flex: '1 1 220px', height: 34, fontSize: 12.5 }} /><Button variant="primary" pending={model.oauthCodePending} disabled={!model.oauthCode.trim() || model.busy} onClick={model.submitOauthCode} style={smallButton}>Xác nhận</Button><Button variant="icon-only" tooltip="Sao chép liên kết" onClick={() => { void navigator.clipboard?.writeText(oauth.url) }} style={{ width: 30, height: 30 }}><Icon name="copy" size={15} /></Button><span style={{ flex: '1 1 100%', fontSize: 12.5, color: 'var(--color-fg-3)' }}>Mở trang <a className="mono" href={oauth.url} target="_blank" rel="noreferrer" style={{ color: 'var(--color-fg)' }}>{oauth.url.replace(/^https?:\/\//, '').split('?')[0]}</a> {PASTE_CODE_HINT}</span></div>{model.oauthCodeError && <span role="alert" style={{ fontSize: 12.5, color: 'var(--color-err)' }}>{model.oauthCodeError}</span>}<span role="status" style={{ display: 'flex', alignItems: 'center', gap: 10, fontSize: 13, color: 'var(--color-fg-3)' }}><Spinner />Đang chờ xác nhận…</span></>}
        {(oauth.phase === 'browser' || oauth.phase === 'device' || oauth.phase === 'paste') && <Button size="sm" pending={model.oauthMutationPending} disabled={model.busy} onClick={model.cancelOauth} style={{ alignSelf: 'flex-start' }}>Huỷ</Button>}
        {oauth.phase === 'done' && <div role="status" style={{ display: 'flex', flexDirection: 'column', borderRadius: 14, border: '1px solid rgba(166,237,193,.3)', background: 'rgba(166,237,193,.04)', overflow: 'hidden' }}><div style={{ display: 'flex', alignItems: 'center', gap: 12, padding: 14 }}><span style={{ width: 38, height: 38, borderRadius: '50%', background: 'rgba(166,237,193,.12)', boxShadow: '0 0 0 1px rgba(166,237,193,.3),0 0 24px -6px rgba(166,237,193,.45)', display: 'flex', alignItems: 'center', justifyContent: 'center', color: 'var(--color-ok)' }}><Icon name="tick-circle-bulk" size={22} /></span><span style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column' }}><span style={{ fontSize: 14, fontWeight: 600 }}>Đã đăng nhập {presentation?.label ?? selected.label}</span><span style={{ fontSize: 12.5, color: 'var(--color-fg-3)' }}>Qua {oauth.via === 'device' ? 'mã thiết bị' : oauth.via === 'paste' ? 'mã đã dán' : 'trình duyệt'} · vừa xong</span></span>{oauth.account.plan && <span style={{ padding: '2px 8px', borderRadius: 6, background: 'rgba(201,166,245,.12)', color: 'var(--color-accent)', fontSize: 11.5 }}>{oauth.account.plan}</span>}</div><div style={{ padding: '0 14px 6px' }}>{[{ icon: 'user', label: 'Tài khoản', value: oauth.account.email || oauth.account.label }, { icon: 'refresh-2', label: 'Giữ kết nối', value: 'Tự động gia hạn — không phải đăng nhập lại' }, { icon: 'cpu', label: 'Model dùng được', value: oauth.models.map((entry) => entry.name).join(', ') || 'Chọn ở bước sau' }].map((row) => <div key={row.label} style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '9px 0', borderTop: '1px solid var(--line-1)', fontSize: 12.5 }}><Icon name={row.icon} size={16} style={{ color: 'var(--color-fg-4)' }} /><span style={{ flex: '0 0 120px', color: 'var(--color-fg-3)' }}>{row.label}</span><span style={{ flex: 1, overflowWrap: 'anywhere' }}>{row.value}</span></div>)}</div><div style={{ display: 'flex', gap: 8, padding: '10px 14px', borderTop: '1px solid var(--line-1)', background: 'rgba(0,0,0,.15)' }}><span style={{ flex: 1, display: 'flex', alignItems: 'center', gap: 6, fontSize: 12, color: 'var(--color-fg-4)' }}><Icon name="setting-2" size={14} />Ngắt kết nối bất cứ lúc nào trong Tuỳ chỉnh › Nhà cung cấp AI</span><Button size="sm" pending={model.oauthMutationPending} disabled={model.busy} onClick={model.disconnectProvider}><Icon name="logout" size={14} />Dùng tài khoản khác</Button></div></div>}
        {oauth.phase === 'failed' && <div role="alert" style={{ display: 'flex', flexDirection: 'column', gap: 10, padding: 14, borderRadius: 14, border: '1px solid rgba(255,154,162,.3)', background: 'rgba(255,154,162,.04)' }}><span style={{ display: 'flex', alignItems: 'center', gap: 10, fontSize: 14, fontWeight: 600 }}><Icon name="close-circle-bulk" style={{ color: 'var(--color-err)' }} />Đăng nhập thất bại</span><span style={{ fontSize: 13, color: 'var(--color-fg-2)' }}>{oauth.message}</span><div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>{selected.deviceOnly ? <Button variant="primary" pending={model.oauthPending} disabled={model.oauthBusy} onClick={() => model.startOauth(true)} style={smallButton}><Icon name="refresh-2" size={14} />Thử lại</Button> : <><Button variant="primary" pending={model.oauthPending} disabled={!loopback || model.oauthBusy} onClick={() => model.startOauth(false)} style={smallButton}><Icon name="refresh-2" size={14} />Thử lại</Button><Button pending={model.oauthPending} disabled={model.oauthBusy} onClick={() => model.startOauth(true)} style={smallButton}><Icon name="keyboard" size={14} />Dùng mã thiết bị</Button></>}</div></div>}
      </> : <><span style={{ fontSize: 14, fontWeight: 600 }}>{presentation?.label ?? selected.label}</span>{selected.id === 'custom' && <div style={{ display: 'grid', gridTemplateColumns: 'repeat(2,minmax(0,1fr))', gap: 10 }}><div style={fieldStyle}><label htmlFor="pv-name" className="lbl">Tên provider</label><AuthInput id="pv-name" mono placeholder="my-proxy" value={model.customName} onChange={model.setCustomName} /></div><div style={fieldStyle}><span className="lbl">Giao thức API</span><Segmented label="Giao thức API" value={model.customType} onChange={model.setCustomType}
        options={['openai', 'anthropic', 'gemini'].map((type) => ({ id: type, label: type === 'openai' ? 'OpenAI' : type === 'anthropic' ? 'Anthropic' : 'Gemini' }))}
        thumbStyle={{ background: 'var(--color-surface-6)' }} style={{ gap: 2 }} /></div></div>}<div style={fieldStyle}><label htmlFor="ob-key" className="lbl">API Key{!selected.keyRequired && ' · không bắt buộc'}</label><AuthInput id="ob-key" mono type="password" autoComplete="off" value={model.apiKey} onChange={model.setApiKey} placeholder={selected.keyRequired ? 'sk-…' : 'Để trống nếu không cần API Key'} /></div><div style={fieldStyle}><label htmlFor="ob-base" className="lbl">Base URL</label><AuthInput id="ob-base" mono value={model.baseURL} onChange={model.setBaseURL} placeholder="Để trống dùng địa chỉ mặc định" /></div></>}</Card>}
    </>}
  </>
}

function ModelStep({ model }: { model: OnboardModel }) {
  const options = model.models.map((entry) => ({ value: entry.name, label: entry.displayName || entry.name, description: entry.contextWindow ? `ngữ cảnh ${formatContextK(entry.contextWindow)}` : undefined }))
  return <>
    <Heading title="Model cho các trợ lý">Một model chính dùng chung là đủ để bắt đầu. Đổi riêng từng trợ lý khi cần cân bằng chất lượng và chi phí — đổi lại bất cứ lúc nào trong Tuỳ chỉnh.</Heading>
    <Card>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline' }}><span className="lbl">Model chính</span><span style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>từ {model.preset?.oauth ? 'tài khoản ChatGPT' : model.providerLabel}</span></div>
      {options.length > 6 ? <Combobox aria-label="Model chính" value={model.model} onChange={model.setModel} options={options} /> : options.length ? (
        <div role="radiogroup" aria-label="Model chính" style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
          {model.models.map((entry) => {
            const picked = model.model === entry.name
            return <RadioCard key={entry.name} checked={picked} dotSize={16} onChange={() => model.setModel(entry.name)}
              style={{ alignItems: 'center', gap: 12, padding: '10px 12px', borderRadius: 12, border: `1px solid ${picked ? selectedBorder : 'var(--line-3)'}`, background: picked ? selectedBackground : 'var(--color-surface-0)' }}>
              <span className="mono" style={{ flex: 1, fontSize: 13 }}>{entry.name}</span>
              <span className="num" style={{ fontSize: 12, color: 'var(--color-fg-3)' }}>{entry.contextWindow ? `ngữ cảnh ${formatContextK(entry.contextWindow)}` : ''}</span>
            </RadioCard>
          })}
        </div>
      ) : <AuthInput mono aria-label="Model chính" value={model.model} onChange={model.setModel} placeholder="Tên model của nhà cung cấp" />}
      <div style={fieldStyle}>
        <span className="lbl">Mức suy luận</span>
        <Segmented label="Mức suy luận" value={model.effort} onChange={model.setEffort}
          options={EFFORTS.map((entry) => ({ id: entry.key, label: entry.label, style: { fontSize: 12 } }))}
          thumbStyle={{ background: 'var(--color-surface-6)' }} style={{ gap: 2 }} />
      </div>
    </Card>
    <Card style={{ padding: '6px 6px 8px' }}>
      <SwitchRow checked={model.shared} onChange={model.setShared} title="Dùng model chính cho mọi trợ lý" description={`Tắt để chọn riêng cho ${agents.slice(0, 3).map((agent) => agent.label).join(', ')}`} />
      <div style={{ display: 'flex', flexDirection: 'column', gap: 2, padding: '0 6px' }}>{agents.map((agent) => {
    const role = agent.key === 'arbiter' ? null : agent.key
    const editable = !model.shared && role
    return <div key={agent.key} style={{ display: 'flex', alignItems: 'center', flexWrap: 'wrap', gap: 12, padding: '8px 6px', borderTop: '1px solid rgba(255,255,255,.05)', opacity: model.shared && role ? .6 : 1 }}>
      <AgentOrb agent={agent.key} size={22} />
      <span style={{ flex: '0 0 110px', display: 'flex', flexDirection: 'column' }}><span style={{ fontSize: 13, fontWeight: 500 }}>{agent.label}</span><span style={{ fontSize: 11, color: 'var(--color-fg-4)' }}>{role ? agent.desc : 'Luôn dùng model chính'}</span></span>
      {editable ? <>
        <div style={{ flex: '1 1 150px', minWidth: 150 }}><Combobox compact aria-label={`Model ${agent.label}`} value={model.roleModels[editable].model || model.model} onChange={(value) => model.setRoleModel(editable, { model: value })} options={options} /></div>
        <div style={{ flex: '0 0 110px' }}><KitSelect compact aria-label={`Mức suy luận ${agent.label}`} value={model.roleModels[editable].effort} onChange={(value) => model.setRoleModel(editable, { effort: value })} options={EFFORTS.map((entry) => ({ value: entry.key, label: entry.label }))} /></div>
      </> : role ? <>
        <span className="mono" style={{ flex: '1 1 150px', minWidth: 150, height: 32, display: 'flex', alignItems: 'center', padding: '0 10px', borderRadius: 8, background: '#0F0F11', border: '1px solid var(--line-3)', fontSize: 12, color: 'var(--color-fg-4)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{model.model || '—'}</span>
        <span style={{ flex: '0 0 110px', height: 32, display: 'flex', alignItems: 'center', padding: '0 10px', borderRadius: 8, background: '#0F0F11', border: '1px solid var(--line-3)', fontSize: 12, color: 'var(--color-fg-4)' }}>{effortLabel(model.effort)}</span>
      </> : <>
        <span className="mono" style={{ flex: '1 1 150px', minWidth: 150, height: 32, display: 'flex', alignItems: 'center', padding: '0 10px', borderRadius: 8, background: '#0F0F11', border: '1px solid var(--line-3)', fontSize: 12, color: 'var(--color-fg-4)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{model.model || '—'}</span>
        <span style={{ flex: '0 0 110px', height: 32, display: 'flex', alignItems: 'center', padding: '0 10px', borderRadius: 8, background: '#0F0F11', border: '1px solid var(--line-3)', fontSize: 12, color: 'var(--color-fg-4)' }}>—</span>
      </>}
    </div>
  })}</div></Card></>
}

/** Bốn mục mỗi dự án tự có — Onboard.dc.html dirChips. Chỉ tên người dùng thấy, không có tên thư mục. */
const PROJECT_PARTS = [
  { icon: 'lamp-on', label: 'Ý tưởng', note: 'Ghi nhanh ý tưởng truyện', color: '#F6CB8C' },
  { icon: 'box', label: 'Tư liệu', note: 'Bìa, nhạc nền, giọng mẫu', color: '#9DB0F7' },
  { icon: 'book-1', label: 'Truyện', note: 'Bản thảo do trợ lý viết', color: '#C9A6F5' },
  { icon: 'export-square', label: 'Bản xuất', note: 'Văn bản, ebook, bản đọc TTS, sách nói', color: '#A6EDC1' },
]

function LibraryStep({ model }: { model: OnboardModel }) {
  const choice = model.libChoice
  const { picker, status } = choice
  return <>
    <Heading title="Lưu truyện ở đâu?">Chọn một thư mục trên máy đang chạy Noverse. Mọi dự án, truyện và bản xuất sẽ nằm gọn trong đó.</Heading>
    <div role="radiogroup" aria-label="Thư mục lưu truyện" style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
      {choice.loading && !choice.options.length && <Skeleton height={68} />}
      {choice.options.map((option) => {
        const on = option.key === choice.sel
        const used = option.tag === 'Đang dùng'
        return <RadioCard key={option.key} className="opt" checked={on} dotSize={20} onChange={() => choice.select(option.key)}
          style={{ alignItems: 'center', gap: 14, padding: '14px 16px', borderRadius: 14, border: `1px solid ${on ? 'rgba(201,166,245,0.5)' : 'rgba(255,255,255,0.08)'}`, background: on ? 'rgba(201,166,245,0.06)' : '#141416', color: '#EDEDEF', textAlign: 'left' }}>
          <span style={{ width: 38, height: 38, borderRadius: 11, background: on ? 'rgba(201,166,245,0.14)' : 'rgba(255,255,255,0.05)', display: 'flex', alignItems: 'center', justifyContent: 'center', flex: '0 0 auto' }}>
            <Icon name={option.icon} size={19} style={{ color: on ? '#C9A6F5' : '#A0A0AB' }} />
          </span>
          <span style={{ flex: '1 1 auto', minWidth: 0, display: 'flex', flexDirection: 'column', gap: 2, lineHeight: 1.4 }}>
            <span style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <span style={{ fontSize: 14.5, fontWeight: 500 }}>{option.title}</span>
              {(on || option.key !== 'custom') && <span style={{ height: 20, padding: '0 7px', borderRadius: 6, background: used ? 'rgba(166,237,193,0.12)' : 'rgba(201,166,245,0.12)', color: used ? '#A6EDC1' : '#DCC8F8', fontSize: 11, display: 'flex', alignItems: 'center' }}>{option.tag}</span>}
            </span>
            <span style={{ fontSize: 12.5, color: '#8E8E99', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{option.where}</span>
          </span>
          <span style={{ fontSize: 12.5, color: option.noteFg, whiteSpace: 'nowrap', flex: '0 0 auto' }}>{option.note}</span>
        </RadioCard>
      })}
      <button className="ghost hov-ghost" type="button" onClick={() => void picker.pick()} disabled={picker.picking}
        style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '12px 16px', borderRadius: 14, border: '1px dashed rgba(255,255,255,0.14)', background: 'transparent', color: '#C9C9D1', fontSize: 13.5, textAlign: 'left' }}>
        {picker.picking ? <Spinner size={14} /> : <Icon name="folder-open" size={18} style={{ color: '#A0A0AB' }} />}
        <span style={{ flex: '1 1 auto' }}>{picker.picking ? 'Đang mở cửa sổ chọn thư mục trên máy…' : 'Chọn thư mục khác…'}</span>
        <span style={{ fontSize: 12, color: '#7E7E8A' }}>{picker.picking ? 'chọn xong quay lại đây' : 'iCloud, Google Drive, ổ ngoài…'}</span>
      </button>
      {picker.error && <span role="alert" style={{ fontSize: 12.5, color: '#FF9AA2', display: 'flex', alignItems: 'center', gap: 6 }}><Icon name="warning-2" size={14} />{picker.error}</span>}
    </div>
    {status && <div role="status" style={{ display: 'flex', gap: 10, alignItems: 'flex-start', padding: '12px 14px', borderRadius: 12, background: status.bg, border: `1px solid ${status.bd}`, fontSize: 13, color: status.fg, lineHeight: 1.5 }}>
      <Icon name={status.icon} size={16} style={{ marginTop: 2 }} /><span>{status.text}</span>
    </div>}
    {picker.dialog}
    <Card style={{ padding: 16 }}>
      <span className="lbl">Mỗi dự án tự có sẵn</span>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4,minmax(0,1fr))', gap: 8 }}>
        {PROJECT_PARTS.map((part) => <div key={part.label} style={{ padding: 12, borderRadius: 12, background: '#0F0F11', border: '1px solid rgba(255,255,255,0.06)', display: 'flex', flexDirection: 'column', gap: 6 }}>
          <Icon name={part.icon} size={18} style={{ color: part.color }} />
          <span style={{ fontSize: 13.5, fontWeight: 500 }}>{part.label}</span>
          <span style={{ fontSize: 12, color: '#8E8E99', lineHeight: 1.45 }}>{part.note}</span>
        </div>)}
      </div>
      <span style={{ fontSize: 12, color: '#7E7E8A' }}>Noverse tự sắp xếp tệp, bạn không cần tạo hay đặt tên thư mục.</span>
    </Card>
    <Card style={{ padding: '6px 6px 12px', gap: 0 }}>
      <SwitchRow checked={model.autoExport} onChange={model.setAutoExport} icon="export-square"
        title="Tự xuất bản khi xong một tập"
        description="Tự lưu bản đọc được vào mục Bản xuất của mỗi truyện, chia theo tập" />
      <div style={{ display: 'flex', gap: 6, padding: '0 12px 0 42px', opacity: model.autoExport ? 1 : 0.4 }}>
        {ONBOARD_FORMATS.map((format) => {
          const on = model.formats.includes(format)
          return <Checkbox key={format} checked={on} variant="chip" onChange={() => model.toggleFormat(format)}
            label={FORMAT_LABEL[format]}
            style={{ height: 28, padding: '0 10px', borderRadius: 999, border: `1px solid ${on ? 'rgba(255,255,255,0.3)' : 'rgba(255,255,255,0.1)'}`, background: on ? '#222225' : 'transparent', color: on ? '#EDEDEF' : '#A0A0AB', fontSize: 12, gap: 6 }} />
        })}
      </div>
    </Card>
  </>
}

function ProjectStep({ model }: { model: OnboardModel }) {
  const project = model.project
  if (project) {
    return <><Heading title="Dự án đầu tiên">Dự án là một kênh / series, gom nhiều truyện dùng chung văn phong và cấu hình.</Heading>
      <Card style={{ flexDirection: 'row', alignItems: 'center', gap: 12, borderColor: 'rgba(166,237,193,.3)' }}>
        <Icon name="folder-open" style={{ color: 'var(--color-accent)' }} />
        <span style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', gap: 2 }}>
          <span style={{ fontSize: 14, fontWeight: 500 }}>{project.name || project.id}</span>
          <span style={{ fontSize: 12.5, color: 'var(--color-fg-3)' }}>{project.storyCount} truyện</span>
        </span>
        <span role="status" style={{ fontSize: 12, color: 'var(--color-ok)', display: 'flex', alignItems: 'center', gap: 6 }}><Icon name="tick-circle-bulk" size={14} />Đã có trong thư viện</span>
      </Card>
      <Button variant="ghost" size="sm" onClick={() => model.setAddingProject(true)} style={{ alignSelf: 'flex-start' }}><Icon name="add-square" size={15} />Tạo thêm dự án</Button>
    </>
  }
  return <><ProjectForm model={model} />{model.addingProject && <Button variant="ghost" size="sm" onClick={() => model.setAddingProject(false)} style={{ alignSelf: 'flex-start' }}>Huỷ — giữ dự án đã có</Button>}</>
}

function ProjectForm({ model }: { model: OnboardModel }) {
  const noCandidates = !model.candidates.length
  return <><Heading title="Dự án đầu tiên">Dự án là một kênh / series, gom nhiều truyện dùng chung văn phong và cấu hình.</Heading><div role="radiogroup" aria-label="Cách tạo dự án" style={{ display: 'grid', gridTemplateColumns: 'repeat(2,minmax(0,1fr))', gap: 10 }}>{[{ key: 'adopt' as const, icon: 'folder-open', title: 'Dùng thư mục có sẵn', desc: model.storyCount > 0 ? `Đã có ${model.storyCount} truyện trong thư viện · Noverse gom thành một dự án` : 'Chưa có truyện nào trong thư viện' }, { key: 'new' as const, icon: 'add-square', title: 'Tạo dự án mới', desc: 'Bắt đầu trống, có sẵn Ý tưởng · Tư liệu · Truyện · Bản xuất' }].map((choice) => <RadioCard key={choice.key} checked={model.projMode === choice.key} mark="none" disabled={choice.key === 'adopt' && noCandidates} onChange={() => model.setProjMode(choice.key)} style={{ gap: 12, padding: 14, borderRadius: 14, border: `1px solid ${model.projMode === choice.key ? selectedBorder : 'var(--line-3)'}`, background: model.projMode === choice.key ? selectedBackground : 'var(--color-surface-1)', opacity: choice.key === 'adopt' && noCandidates ? .5 : 1 }}><Icon name={choice.icon} style={{ color: model.projMode === choice.key ? 'var(--color-accent)' : 'var(--color-fg-3)', marginTop: 1 }} /><span style={{ display: 'flex', flexDirection: 'column', gap: 2 }}><span style={{ fontSize: 14, fontWeight: 500 }}>{choice.title}</span><span style={{ fontSize: 12.5, color: 'var(--color-fg-3)' }}>{choice.desc}</span></span></RadioCard>)}</div><Card>{model.projMode === 'adopt' && model.candidates.length > 1 && <KitSelect label="Thư mục có sẵn" value={model.candidate} onChange={model.setCandidate} options={model.candidates.map((candidate) => ({ value: candidate, label: candidate }))} />}<div style={{ display: 'grid', gridTemplateColumns: 'repeat(2,minmax(0,1fr))', gap: 12 }}><div style={fieldStyle}><label htmlFor="ob-proj-name" className="lbl">Tên hiển thị</label><AuthInput id="ob-proj-name" value={model.projName} onChange={model.setProjName} /></div><div style={fieldStyle}><label htmlFor="ob-proj-slug" className="lbl">Thư mục</label><AuthInput id="ob-proj-slug" mono readOnly value={model.projMode === 'adopt' ? model.candidate : model.projSlug} invalid={Boolean(model.slugError)} style={{ color: 'var(--color-fg-3)' }} /><span style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 11.5, color: model.slugError ? 'var(--color-err)' : 'var(--color-ok)' }}><Icon name={model.slugError ? 'warning-2' : 'tick-circle-bulk'} size={13} />{model.slugError || (model.projMode === 'adopt' ? 'Giữ nguyên tên thư mục hiện có' : 'Hợp lệ · tạo tự động từ tên hiển thị')}</span></div></div><div style={fieldStyle}><label htmlFor="ob-proj-desc" className="lbl">Mô tả kênh · tùy chọn</label><TextArea id="ob-proj-desc" rows={2} value={model.projDesc} onChange={(event) => model.setProjDesc(event.target.value)} placeholder="Kênh / series này nói về gì, giọng đọc, đối tượng…" style={{ height: 'auto', padding: '9px 12px', resize: 'none' }} /></div>{model.projMode === 'adopt' && <div style={{ ...fieldStyle, padding: '10px 12px', borderRadius: 12, background: '#0F0F11', border: '1px solid var(--line-1)', fontSize: 12.5 }}><span style={{ color: 'var(--color-fg-2)' }}>Trong thư mục đã có:</span><span style={{ display: 'flex', alignItems: 'center', gap: 8, color: 'var(--color-fg-3)' }}><Icon name="book-1" size={14} />{model.storyCount} truyện đã có — giữ nguyên</span><span style={{ display: 'flex', alignItems: 'center', gap: 8, color: 'var(--color-fg-3)' }}><Icon name="note-text" size={14} />Quy tắc văn phong của kênh — giữ nguyên</span></div>}</Card></>
}

function copyText(text: string) {
  void navigator.clipboard?.writeText(text).catch(() => undefined)
}

/** Mã khôi phục hiện đúng một lần: bản chép/tải về là cách duy nhất để giữ lại. */
function RecoveryCard({ recovery }: { recovery: OnboardModel['recovery'] }) {
  const [copied, setCopied] = useState(false)
  useEffect(() => {
    if (!copied) return
    const timer = window.setTimeout(() => setCopied(false), 2000)
    return () => window.clearTimeout(timer)
  }, [copied])
  if (recovery.status === 'exists' || recovery.status === 'idle') return null
  const download = () => {
    const url = URL.createObjectURL(new Blob([`Mã khôi phục của quản trị ${t.brand.name}\n${recovery.code}\n`], { type: 'text/plain;charset=utf-8' }))
    const link = document.createElement('a')
    link.href = url
    link.download = t.adv.recovery.fileName
    link.click()
    URL.revokeObjectURL(url)
  }
  const ready = recovery.status === 'ready'
  const action: CSSProperties = { height: 42, padding: '0 14px', borderRadius: 10, border: '1px solid rgba(255,255,255,0.12)', color: '#EDEDEF', fontSize: 13 }
  return (
    <section className="card" style={{ borderRadius: 16, padding: 16, display: 'flex', flexDirection: 'column', gap: 10, borderColor: 'rgba(246,203,140,0.28)' }}>
      <span style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 14, fontWeight: 600 }}><Icon name="key" size={17} style={{ color: '#F6CB8C' }} />Mã khôi phục của quản trị</span>
      <span style={{ fontSize: 13, color: '#A0A0AB', lineHeight: 1.5 }}>Dùng khi bạn quên mật khẩu mà không có quản trị viên khác. Mã chỉ hiện một lần — hãy cất ở nơi an toàn.</span>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
        {recovery.status === 'error'
          ? <span role="alert" style={{ flex: '1 1 auto', fontSize: 13, color: 'var(--color-err)' }}>{recovery.error}</span>
          : <span aria-live="polite" style={{ flex: '1 1 auto', minWidth: 0, height: 42, padding: '0 14px', borderRadius: 10, background: '#0F0F11', border: '1px dashed rgba(246,203,140,0.35)', display: 'flex', alignItems: 'center', fontSize: 17, fontWeight: 600, letterSpacing: '.12em', color: '#FBE3BD' }}>{ready ? recovery.code : <Spinner size={16} color="#F6CB8C" />}</span>}
        <Button variant="ghost" disabled={!ready} onClick={() => { copyText(recovery.code); setCopied(true) }} style={action}><span style={{ display: 'flex', alignItems: 'center', gap: 6 }}><Icon name={copied ? 'tick-circle' : 'copy'} size={15} />{copied ? 'Đã chép' : 'Chép'}</span></Button>
        <Button variant="ghost" disabled={!ready} onClick={download} style={action}><span style={{ display: 'flex', alignItems: 'center', gap: 6 }}><Icon name="document-download" size={15} />Tải tệp</span></Button>
      </div>
    </section>
  )
}

function DoneStep({ model }: { model: OnboardModel }) {
  const { user } = model
  const first = model.assigned[0]
  const rows: { key: StepKey; icon: string; label: string; value: string; mono?: boolean }[] = model.isAdmin
    ? [
      { key: 'profile', icon: 'user', label: 'Tài khoản', value: `${model.name} · @${user.username} · ${t.role[user.role]}` },
      { key: 'provider', icon: 'cloud-connection', label: 'Nhà cung cấp', value: model.providerLabel },
      // QĐ 6: model có tên tiếng Việt/unicode → không dùng font mono ở dòng tóm tắt.
      { key: 'model', icon: 'cpu', label: 'Model', value: `${model.model || '—'} · suy luận ${effortLabel(model.effort).toLowerCase()}` },
      { key: 'library', icon: 'folder-open', label: 'Nơi lưu truyện', value: model.libChoice.display || '—' },
      { key: 'project', icon: 'book-1', label: 'Dự án', value: model.project ? model.project.name || model.project.id : 'Chưa tạo' },
    ]
    : [
      { key: 'profile', icon: 'user', label: 'Tài khoản', value: `${model.name} · @${user.username} · ${t.role[user.role]}` },
      { key: 'password', icon: 'password-check', label: 'Mật khẩu', value: model.pwChanged || !user.mustChangePassword ? 'Đã đặt mật khẩu riêng' : 'Vẫn dùng mật khẩu tạm' },
      { key: 'profile', icon: 'book-1', label: 'Truyện được giao', value: first ? storyLabel(first.story) : 'Chưa có truyện nào được giao' },
    ]
  const tips = [
    { icon: 'messages-2', title: 'Can thiệp bất cứ lúc nào', desc: 'Gõ vào ô chat khi trợ lý đang viết — Trọng tài phân xử.' },
    { icon: 'command', title: 'Bảng lệnh ⌘K', desc: 'Gõ / để duyệt chương, xuất bản, viết lại…' },
    { icon: 'mobile', title: 'Theo dõi trên điện thoại', desc: 'Ảnh đại diện → Đăng nhập điện thoại bằng QR.' },
  ]
  return (
    <>
      <div style={{ display: 'flex', alignItems: 'center', gap: 18 }}>
        <OrbHero variant="done" />
        <div>
          <h1 style={{ margin: 0, fontSize: 26, fontWeight: 600, letterSpacing: '-.015em' }}>Mọi thứ đã sẵn sàng</h1>
          <p style={{ margin: '6px 0 0', fontSize: 14, color: '#A0A0AB', maxWidth: 560 }}>{model.isAdmin ? 'Thư viện, model và dự án đã sẵn sàng. Bắt đầu bằng một câu ý tưởng — Kiến trúc sư sẽ lo phần còn lại.' : 'Mở truyện được giao để xem Người viết làm việc, hoặc tạo truyện mới.'}</p>
        </div>
      </div>
      {model.isAdmin && <RecoveryCard recovery={model.recovery} />}
      <Card style={{ padding: '6px 16px', gap: 0 }}>
        {rows.map((row, index) => (
          <div key={row.label} style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '10px 0', borderTop: index ? '1px solid rgba(255,255,255,0.06)' : 'none' }}>
            <Icon name={row.icon} style={{ color: '#A6EDC1' }} />
            <span style={{ flex: '0 0 160px', fontSize: 13, color: '#A0A0AB' }}>{row.label}</span>
            <span className={row.mono ? 'mono' : undefined} style={{ flex: 1, minWidth: 0, fontSize: 13, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{row.value}</span>
            <Button variant="ghost" size="sm" onClick={() => model.go(model.steps.indexOf(row.key))} style={{ height: 26, padding: '0 8px', borderRadius: 7, border: 'none', color: '#7E7E8A', fontSize: 12 }}>Sửa</Button>
          </div>
        ))}
      </Card>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3,minmax(0,1fr))', gap: 8 }}>
        {tips.map((tip) => (
          <div key={tip.title} className="card" style={{ borderRadius: 14, padding: 12, display: 'flex', flexDirection: 'column', gap: 6 }}>
            <Icon name={tip.icon} size={17} style={{ color: '#C9A6F5' }} />
            <span style={{ fontSize: 13, fontWeight: 500 }}>{tip.title}</span>
            <span style={{ fontSize: 12, color: '#A0A0AB' }}>{tip.desc}</span>
          </div>
        ))}
      </div>
    </>
  )
}
