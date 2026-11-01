import { Fragment, useEffect, useState, type CSSProperties, type ReactNode } from 'react'
import * as stylex from '@stylexjs/stylex'
import { createUser, deleteUser, getAdminAudit, grantAppAccess, listUsers, mySessions, revokeAppAccess, revokeOtherSessions, revokeSession, revokeUserSessions, updateUser, userSessions } from '../../api/client'
import { useLibrary } from '../../api/library'
import { useSession } from '../../api/session'
import type { AdminAuditItem, AppId, ProjectPermissions, Role, User } from '../../api/types'
import { invalidate, keys, useConnection, useResource } from '../../data'
import { membersCopy as c } from '../../i18n/vi'
import { navigate, paths, useRoute } from '../../router'
import { Dialog } from '@noverse/ui'
import { BrandMark, Icon } from '@noverse/ui'
import { Checkbox, KitSelect } from '@noverse/ui'
import { StaleBanner } from '@noverse/ui'
import { toast } from '@noverse/ui'
import { PairingDialog } from '../account/PairingDialog'
import { relativeTime } from '../auth/parts'
import { SettingsNav } from './nav'
import { sx } from './members.styles'
import { useUnsavedGuard } from '../../ui/useUnsavedGuard'
import { ssoCopy } from '../../i18n/sso'

const roleKeys = ['none', 'view', 'collab', 'owner'] as const
const roleDots = { none: 'var(--ring-paused)', view: 'var(--color-fg-3)', collab: 'var(--kit-info-fg)', owner: 'var(--color-accent)' }
/** Hai app của kho tài khoản chung — thứ tự hiển thị, không phải trọng số. */
const appIds = ['noverse', 'viderse'] as const
const appIcons: Record<AppId, string> = { noverse: 'book-1', viderse: 'video-play' }
type Permission = typeof roleKeys[number]
type Draft = { role: Role; projects: ProjectPermissions }
type Secret = { username: string; name: string; password: string; reset?: boolean; role?: Role; projects?: ProjectPermissions }
const date = (value?: string | null) => value ? new Date(value).toLocaleDateString('vi-VN') : '—'
export const suggestUsername = (name: string) => name.normalize('NFD').replace(/\p{M}/gu, '').replace(/[đĐ]/g, 'd').toLowerCase().replace(/[^a-z0-9]+/g, '.').replace(/^\.+|\.+$/g, '').slice(0, 32)
export function temporaryPassword() {
  const bytes = crypto.getRandomValues(new Uint8Array(12))
  return Array.from(bytes, (value) => 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnpqrstuvwxyz23456789'[value % 54]).join('')
}
function Button({ children, onClick, disabled, primary, red, big, tiny, label, style }: { children: ReactNode; onClick?: () => void; disabled?: boolean; primary?: boolean; red?: boolean; big?: boolean; tiny?: boolean; label?: string; style?: CSSProperties }) {
  return <button type="button" {...stylex.props(sx.button, primary && sx.primary, red && sx.red, big && sx.btn36, tiny && sx.btn28)} style={style} disabled={disabled} onClick={onClick} aria-label={label}>{children}</button>
}
function SystemRoles({ role, disabled, compact, onChange }: { role: Role; disabled?: boolean; compact?: boolean; onChange: (role: Role) => void }) {
  return <div role="radiogroup" aria-label={c.system} {...stylex.props(sx.roleGrid)} style={compact ? { gap: 8 } : undefined}>
    {(['member', 'admin'] as const).map((value) => <button type="button" key={value} role="radio" aria-checked={role === value} disabled={disabled} onClick={() => onChange(value)} {...stylex.props(sx.radio, role === value && sx.radioOn)} style={compact ? { padding: '10px 12px' } : undefined}>
      <span {...stylex.props(sx.radioDot, role === value && sx.radioDotOn)} aria-hidden="true">{role === value && <i {...stylex.props(sx.radioInner)} />}</span>
      <span {...stylex.props(sx.col)} style={{ gap: compact ? 0 : 2, lineHeight: compact ? 1.4 : 1.45 }}>
        <span {...stylex.props(sx.row)} style={{ gap: 6, fontSize: compact ? 13.5 : 14, fontWeight: 500 }}>{!compact && <Icon name={value === 'admin' ? 'crown' : 'profile-2user'} size={14} />}{c[value]}</span>
        <span {...stylex.props(sx.small)} style={compact ? { fontSize: 12 } : undefined}>{compact ? c.compactHints[value] : value === 'admin' ? c.adminHint : c.memberHint}</span>
      </span>
    </button>)}
  </div>
}
function Section({ title, hint, action, children, first }: { title: string; hint?: string; action?: ReactNode; children: ReactNode; first?: boolean }) {
  return <section {...stylex.props(sx.section)} style={first ? { marginTop: 14 } : undefined}><div {...stylex.props(sx.sectionHead)}><span>{title}</span>{hint && <span {...stylex.props(sx.small)} style={{ fontWeight: 400 }}>{hint}</span>}<span style={{ flex: 1 }} />{action}</div>{children}</section>
}
export function MembersSettings() {
  const { user } = useSession()
  const accounts = useResource(keys.users(), listUsers, { enabled: user?.role === 'admin' })
  if (user?.role !== 'admin') return null
  return <div style={{ position: 'relative', display: 'flex', flex: 1, minWidth: 0, minHeight: 0, width: '100%', lineHeight: 1.5 }}><SettingsNav current="members" scope="global" admin membersCount={accounts.data?.users.length} dots={{}} onPick={(id) => navigate(id === 'info' ? `${paths.settings(id)}?scope=project` : paths.settings(id))} /><MembersScreen users={accounts.data?.users} error={accounts.error} refresh={accounts.refresh} /></div>
}
export function MembersScreen({ users, error: loadError, refresh }: { users?: User[]; error?: unknown; refresh: () => unknown }) {
  const { user: me, apps = [] } = useSession()
  const library = useLibrary()
  const connection = useConnection()
  const requestedUser = useRoute().search.get('user') ?? ''
  useEffect(() => { setSelected(requestedUser) }, [requestedUser])
  const projects = library.library?.projects ?? []
  const [view, setView] = useState<'members' | 'log'>('members')
  const [selected, setSelected] = useState(requestedUser)
  const [drafts, setDrafts] = useState<Record<string, Draft>>({})
  const [query, setQuery] = useState('')
  const [filter, setFilter] = useState(0)
  const [matrix, setMatrix] = useState(false)
  const [dialog, setDialog] = useState<'add' | 'reset' | 'lock' | 'remove' | null>(null)
  const [secret, setSecret] = useState<Secret | null>(null)
  const [pairFor, setPairFor] = useState<string | null>(null)
  const [reason, setReason] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [logType, setLogType] = useState('')
  const [actor, setActor] = useState('')
  const [older, setOlder] = useState<AdminAuditItem[]>([])
  const [next, setNext] = useState('')
  // grants=null (backend file độc lập) → không có khái niệm cấp quyền liên app →
  // ẩn toàn bộ điều khiển. grants là object (kể cả {}) = kho chung Postgres.
  const [grantBusy, setGrantBusy] = useState('')
  const [grantErrors, setGrantErrors] = useState<Record<string, string>>({})
  const [grantChange, setGrantChange] = useState<{ username: string; kind: 'changed' | 'viderseOff' | 'none' } | null>(null)
  const all = users ?? []
  const displayed = all.map((entry) => drafts[entry.username] ? { ...entry, ...drafts[entry.username] } : entry)
  const sso = all.length > 0 && all.every((entry) => entry.grants != null)
  const current = all.find((entry) => entry.username === selected) ?? (selected ? undefined : all.find((entry) => entry.role !== 'admin') ?? all[0])
  const selectedName = current?.username ?? ''
  const draft = current ? drafts[current.username] ?? { role: current.role, projects: current.projects } : null
  const devices = useResource([...keys.sessions(), selectedName], () => selectedName === me?.username ? mySessions() : userSessions(selectedName), { enabled: !!current && me?.role === 'admin' })
  const audit = useResource(['admin-audit', logType, actor], () => getAdminAudit({ type: logType, actor, limit: 50 }), { enabled: view === 'log' && me?.role === 'admin' })
  const auditCounts = useResource(['admin-audit', 'counts'], () => getAdminAudit({ limit: 200 }), { enabled: view === 'log' && me?.role === 'admin' })
  useEffect(() => { setOlder([]); setNext(audit.data?.next ?? '') }, [audit.data, logType, actor])
  const admins = displayed.filter((entry) => entry.role === 'admin' && !entry.disabled).length
  const changed = all.flatMap((entry) => {
    const value = drafts[entry.username]
    if (!value) return []
    const projectIds = new Set([...Object.keys(value.projects), ...Object.keys(entry.projects)])
    const count = Number(value.role !== entry.role) + [...projectIds].filter((id) => value.projects[id] !== entry.projects[id]).length
    return count ? [{ user: entry, draft: value, count }] : []
  })
  const changes = changed.reduce((sum, entry) => sum + entry.count, 0)
  useUnsavedGuard(changes > 0, undefined, { save: async () => await save() === true })
  if (me?.role !== 'admin') return null
  const lastAdmin = current?.role === 'admin' && !current.disabled && all.filter((entry) => entry.role === 'admin' && !entry.disabled).length <= 1
  const self = current?.username === me.username
  const disabled = busy || !connection.writable
  const noDanger = self || lastAdmin
  const sessionList = devices.data?.sessions ?? []
  const summary = (entry: Pick<User, 'role' | 'projects'>) => {
    if (entry.role === 'admin') return c.allProjects
    const assigned = Object.entries(entry.projects)
    if (!assigned.length) return c.noProjects
    const [id, role] = assigned[0]
    return assigned.every(([, value]) => value === role) ? `${c.roles[role][0]} ở ${assigned.length === 1 ? `“${projects.find((p) => p.id === id)?.name ?? c.projects}”` : `${assigned.length} dự án`}` : assigned.map(([p, r]) => `${c.roles[r][0]} ở ${projects.find((x) => x.id === p)?.name ?? c.projects}`).join(' · ')
  }
  const status = (entry: User) => entry.disabled ? c.locked : entry.mustChangePassword ? c.pending : entry.username === me.username || entry.lastSeenAt && Date.now() - new Date(entry.lastSeenAt).getTime() < 600000 ? c.active : c.normal
  const matches = [(entry: User) => !!entry, (entry: User) => entry.role === 'admin', (entry: User) => entry.role !== 'admin' && !Object.keys(entry.projects).length, (entry: User) => entry.disabled]
  const rows = displayed.filter(matches[filter]).filter((entry) => `${entry.name} ${entry.username}`.toLocaleLowerCase('vi').includes(query.trim().toLocaleLowerCase('vi')))
  function patch(value: Partial<Draft>) { if (current && draft) setDrafts({ ...drafts, [current.username]: { ...draft, ...value } }) }
  function permission(id: string, role: Permission) {
    if (!draft) return
    const permissions = { ...draft.projects }
    if (role === 'none') delete permissions[id]; else permissions[id] = role
    patch({ projects: permissions })
  }
  async function run(action: () => Promise<unknown>, message?: string) {
    if (disabled) return
    setBusy(true); setError('')
    try { await action(); await Promise.all([invalidate(keys.users()), invalidate(keys.sessions()), invalidate(keys.library()), invalidate(['admin-audit'])]); library.reload(); if (message) toast(message); return true }
    catch (err) { setError(err instanceof Error ? err.message : c.error); return false }
    finally { setBusy(false) }
  }
  // Quyền vào app lưu ngay từng request (không qua savebar): một request lỗi
  // hiện ngay cạnh checkbox đó, không bao giờ đếm là đã lưu thành công.
  async function saveGrant(target: string, app: AppId, on: boolean) {
    const key = `${target}:${app}`
    if (disabled || grantBusy) return
    setGrantBusy(key); setGrantErrors((previous) => { const next = { ...previous }; delete next[key]; return next })
    try {
      const updated = await (on ? grantAppAccess(target, app) : revokeAppAccess(target, app))
      const noApps = !updated.grants?.noverse && !updated.grants?.viderse
      setGrantChange({ username: target, kind: noApps ? 'none' : app === 'viderse' && !on ? 'viderseOff' : 'changed' })
      await invalidate(keys.users()); void refresh()
    } catch (err) {
      setGrantErrors((previous) => ({ ...previous, [key]: err instanceof Error ? err.message : c.error }))
    } finally { setGrantBusy('') }
  }
  async function loadMore() {
    if (disabled || !next) return
    setBusy(true); setError('')
    try {
      const page = await getAdminAudit({ type: logType, actor, before: next, limit: 50 })
      setOlder((previous) => [...previous, ...page.items]); setNext(page.next)
    } catch (err) { setError(err instanceof Error ? err.message : c.error) }
    finally { setBusy(false) }
  }
  async function save() {
    // Promote first; demote the acting administrator last so every request remains authorized.
    const priority = (item: typeof changed[number]) => item.draft.role === 'admin' ? 0 : item.user.username === me?.username ? 2 : 1
    return await run(async () => {
      for (const item of [...changed].sort((a, b) => priority(a) - priority(b))) {
        await updateUser(item.user.username, item.draft, item.user.version)
        setDrafts((previous) => { const next = { ...previous }; delete next[item.user.username]; return next })
      }
    }, c.saved)
  }
  const close = () => { setDialog(null); setSecret(null); setReason(''); setError('') }
  const open = (kind: typeof dialog) => { setReason(''); setError(''); setDialog(kind) }
  async function confirm() {
    if (!current) return
    const ok = await run(async () => {
      if (dialog === 'reset') {
        const password = temporaryPassword()
        await updateUser(current.username, { password }, current.version)
        setSecret({ username: current.username, name: current.name, password, reset: true })
      } else if (dialog === 'remove') { await deleteUser(current.username, reason.trim() || undefined); setSelected('') }
      else await updateUser(current.username, { disabled: !current.disabled, reason: reason.trim() || undefined }, current.version)
    })
    if (ok) { setDialog(null); setReason('') }
  }
  const copy = async (text: string, message = c.copied) => { try { await navigator.clipboard.writeText(text); toast(message) } catch { setError(c.error) } }
  const confirmAction = dialog === 'reset' ? c.reset : dialog === 'remove' ? c.removeAccount : current?.disabled ? c.unlock : c.lockAccount
  const confirmLines = !current ? [] : dialog === 'reset' ? c.resetLines(current.name, sso) : dialog === 'remove' ? c.removeLines(current.name, current.username) : current.disabled ? c.unlockLines(current.name) : c.lockLines(current.name, sso)
  const entries = [...(audit.data?.items ?? []), ...older]
  const days = [...new Set(entries.map((entry) => date(entry.at)))]
  return <main {...stylex.props(sx.root)}>
    <div {...stylex.props(sx.head)}><div {...stylex.props(sx.grow)}><h1 {...stylex.props(sx.title)}>{c.title}</h1><p {...stylex.props(sx.subtitle)}>{c.subtitle(all.length, admins)}</p></div><div role="group" aria-label="Xem" {...stylex.props(sx.seg)}>{(['members', 'log'] as const).map((value) => <button key={value} type="button" aria-pressed={view === value} {...stylex.props(sx.segButton, view === value && sx.segOn)} onClick={() => setView(value)}><Icon name={value === 'members' ? 'profile-2user' : 'activity'} size={15} />{value === 'members' ? c.title : c.log}</button>)}</div><Button primary big style={{ padding: '0 14px 0 12px' }} disabled={disabled} onClick={() => open('add')}><Icon name="user-add" size={16} />{c.add}</Button></div>
    <StaleBanner error={loadError || audit.error} onRetry={() => { void refresh(); void audit.refresh() }} />
    {error && !dialog && !secret && <p role="alert" {...stylex.props(sx.err)}>{error}</p>}
    {view === 'members' ? <div {...stylex.props(sx.body)}>
      <section aria-label={c.list} {...stylex.props(sx.card, sx.list)}><div {...stylex.props(sx.listHead)}><label {...stylex.props(sx.search)}><Icon name="search-normal-1" size={15} /><input type="search" value={query} onChange={(e) => setQuery(e.target.value)} placeholder={c.search} aria-label={c.search} {...stylex.props(sx.searchInput)} /></label><div role="group" aria-label="Lọc" {...stylex.props(sx.filters)}>{c.filters.map((label, index) => <button key={label} type="button" aria-pressed={filter === index} onClick={() => setFilter(index)} {...stylex.props(sx.chipButton, filter === index && sx.chipOn)}>{label}<span className="num" style={{ color: 'var(--color-fg-4)' }}>{displayed.filter(matches[index]).length}</span></button>)}</div></div><div className={`scroll ${stylex.props(sx.listScroll).className}`}>{!users ? <p>{c.loading}</p> : rows.map((entry) => <button key={entry.username} type="button" aria-current={current?.username === entry.username} onClick={() => setSelected(entry.username)} {...stylex.props(sx.userRow, current?.username === entry.username && sx.userSelected, entry.disabled && sx.lockedRow)}><span {...stylex.props(sx.avatar)}>{entry.name[0]?.toLocaleUpperCase('vi')}</span><span {...stylex.props(sx.grow, sx.col)} style={{ lineHeight: 1.35 }}><span {...stylex.props(sx.row)} style={{ gap: 6 }}><span {...stylex.props(sx.userName)} title={entry.name}>{entry.name}</span>{entry.username === me.username && <span {...stylex.props(sx.small)}>{c.me}</span>}{entry.role === 'admin' && <span {...stylex.props(sx.chip, sx.adminChip)}><Icon name="crown" size={11} />{c.admin}</span>}</span><span {...stylex.props(sx.small, sx.ellipsis, !Object.keys(entry.projects).length && entry.role !== 'admin' && sx.warn)} title={summary(entry)}>{summary(entry)}</span></span><span {...stylex.props(sx.col)} style={{ alignItems: 'flex-end', gap: 3 }}><span {...stylex.props(sx.status, entry.disabled && sx.err, status(entry) === c.active && sx.active)}><i {...stylex.props(sx.dot)} />{status(entry)}</span>{entry.mustChangePassword && <span {...stylex.props(sx.chip, sx.tempChip)}>{c.tempPassword}</span>}</span></button>)}{users && !rows.length && <p {...stylex.props(sx.small)} style={{ padding: '28px 16px', textAlign: 'center' }}>{c.noMatch(query)}</p>}</div></section>
      {current && draft && <section aria-label={c.detail} className={`scroll ${stylex.props(sx.card, sx.detail).className}`}><div {...stylex.props(sx.row)} style={{ gap: 14 }}><span {...stylex.props(sx.avatar, sx.avatarBig)}>{current.name[0]?.toLocaleUpperCase('vi')}</span><div {...stylex.props(sx.grow, sx.col)} style={{ lineHeight: 1.35 }}><span {...stylex.props(sx.row)} style={{ gap: 8, flexWrap: 'wrap' }}><strong style={{ fontSize: 18, fontWeight: 600 }}>{current.name}</strong>{self && <span {...stylex.props(sx.small)}>{c.me}</span>}<span {...stylex.props(sx.chip, current.disabled ? sx.err : status(current) === c.active ? sx.active : sx.muted)}>{status(current)}</span></span><span {...stylex.props(sx.small)} style={{ fontSize: 13 }}>@{current.username} · {c.createdAt(date(current.createdAt))} · {current.lastSeenAt ? c.seenAt(relativeTime(current.lastSeenAt)) : c.noSeen}</span></div><Button disabled={current.disabled || disabled} onClick={() => setPairFor(current.username)}><Icon name="mobile" size={15} />{c.phoneFor(current.name)}</Button></div>
      {current.disabled && <div role="status" {...stylex.props(sx.danger, sx.lockBanner)} style={{ marginTop: 14 }}><Icon name="lock" size={17} /><div {...stylex.props(sx.grow, sx.col)} style={{ lineHeight: 1.45 }}><span style={{ fontSize: 13.5, fontWeight: 500 }}>{c.locked} {date(current.disabledAt)}{current.disabledBy ? ` bởi ${all.find((u) => u.username === current.disabledBy)?.name ?? current.disabledBy}` : ''}</span><span {...stylex.props(sx.small, sx.lockNote)}>{current.disabledReason && `“${current.disabledReason}”. `}{c.lockedHint}</span></div><Button disabled={disabled} onClick={() => open('lock')}>{c.unlock}</Button></div>}
      <Section title={c.system} hint={c.systemHint} first><SystemRoles role={draft.role} disabled={lastAdmin || current.disabled || disabled} onChange={(role) => patch({ role })} />{draft.role !== current.role && <span {...stylex.props(sx.small, sx.warn)}>{draft.role === 'admin' ? c.promotion(current.name) : c.demotion(current.name)}</span>}{lastAdmin && <span {...stylex.props(sx.small)}>{c.lastAdmin}</span>}</Section>
      <Section title={c.projects} hint={c.projectsHint} action={<Button tiny onClick={() => setMatrix(!matrix)}><Icon name="message-question" size={14} />{matrix ? c.hideMatrix : c.showMatrix}</Button>}>
      {matrix && <div {...stylex.props(sx.inset)} style={{ padding: '4px 6px' }}><div {...stylex.props(sx.matrix)}>{[c.matrixHead, c.roles.view[0], c.roles.collab[0], c.roles.owner[0]].map((label) => <span key={label} {...stylex.props(sx.small)} style={{ padding: '7px 8px', fontSize: 11.5 }}>{label}</span>)}{c.matrix.map((label, index) => <Fragment key={label}><span {...stylex.props(sx.matrixCell)}>{label}</span>{[index === 0, index < 5, true].map((yes, column) => <span key={column} {...stylex.props(sx.matrixCell)} style={{ textAlign: 'center', color: yes ? 'var(--color-ok)' : 'var(--ring-paused)' }}><Icon name={yes ? 'tick-circle' : 'minus'} size={14} /></span>)}</Fragment>)}</div><p {...stylex.props(sx.small)} style={{ padding: '0 8px' }}>{c.matrixNote}</p></div>}
      <div {...stylex.props(sx.inset)}>{projects.map((project) => { const role = draft.role === 'admin' ? 'owner' : draft.projects[project.id] ?? 'none'; return <div key={project.id} {...stylex.props(sx.project)}><Icon name="folder-open" size={17} /><span {...stylex.props(sx.grow, sx.col)} style={{ lineHeight: 1.4 }}><span {...stylex.props(sx.userName)} title={project.name}>{project.name}</span><span {...stylex.props(sx.small)}>{project.storyCount ? c.stories(project.storyCount) : c.noStories}{draft.role === 'admin' ? ` · ${c.inherited}` : ''}</span></span>{draft.role !== 'admin' && draft.projects[project.id] !== current.projects[project.id] && <span {...stylex.props(sx.chip)}>{c.changed}</span>}<KitSelect variant="project-role" aria-label={c.projectRole(project.name)} value={role} disabled={draft.role === 'admin' || current.disabled || disabled} onChange={(value) => permission(project.id, value as Permission)} icon={<i {...stylex.props(sx.dot)} style={{ width: 7, height: 7, background: roleDots[role] }} />} options={roleKeys.map((value) => ({ value, label: c.roles[value][0], description: c.roles[value][1], icon: <i {...stylex.props(sx.dot)} style={{ width: 7, height: 7, background: roleDots[value] }} /> }))} style={{ minWidth: 136, width: 136 }} /></div> })}</div><span {...stylex.props(sx.small)}>{draft.role === 'admin' ? c.adminNote : c.projectNote(current.name)}</span></Section>
      {sso && <Section title={c.apps} hint={c.appsHint}>
        <div {...stylex.props(sx.appGrid)}>
          {appIds.map((app) => {
            const inherited = current.role === 'admin'
            const on = inherited || current.grants?.[app] != null
            const key = `${current.username}:${app}`
            const origin = apps.find((entry) => entry.id === app)?.origin
            return <div key={app} {...stylex.props(sx.col)} style={{ gap: 6 }}>
              <Checkbox checked={on} disabled={inherited || current.disabled || disabled || grantBusy === key}
                aria-label={c.appAccess(c.appNames[app], current.name)} onChange={(next) => void saveGrant(current.username, app, next)}
                style={{ gap: 12, margin: 0, padding: '12px 14px', borderRadius: 12, border: '1px solid var(--line-3)', background: 'var(--color-surface-1)', width: '100%', minWidth: 0, overflow: 'hidden', fontSize: 14 }}
                label={<><span {...stylex.props(sx.appLogo)} aria-hidden="true">{app === 'noverse' ? <BrandMark size={20} /> : <img src="/icons/viderse-mark.svg" width="20" height="20" alt="" />}</span><span {...stylex.props(sx.grow, sx.col)} style={{ lineHeight: 1.4 }}><span {...stylex.props(sx.userName)}>{c.appNames[app]}</span>{origin && <span {...stylex.props(sx.appHost)} title={origin}>{new URL(origin).host}</span>}</span></>} />
              {grantErrors[key] && <span role="alert" {...stylex.props(sx.small, sx.err)}>{grantErrors[key]}</span>}
            </div>
          })}
        </div>
        <span {...stylex.props(sx.small)}>{current.role === 'admin' ? `${c.appInherited}.` : c.appsNote}</span>
        {current.role !== 'admin' && grantChange?.username === current.username && <div role="status" {...stylex.props(sx.appNotice)}>
          {(grantChange.kind === 'none'
            ? [c.appNoneHint, ssoCopy.affected(current.name)]
            : [ssoCopy.affected(current.name), ...(grantChange.kind === 'viderseOff' ? [ssoCopy.remaining(current.name, 'Noverse', 'Viderse')] : [])]
          ).map((text) => <span key={text} {...stylex.props(sx.appNoticeLine)}><Icon name="info-circle" size={14} style={{ flexShrink: 0, marginTop: 3 }} /><span>{text}</span></span>)}
        </div>}
      </Section>}
      <Section title={c.devices} hint={`· ${sessionList.length}`} action={sessionList.some((s) => !s.current) && <Button style={{ height: 28 }} disabled={disabled} onClick={() => { void run(() => self ? revokeOtherSessions() : revokeUserSessions(current.username), self ? c.outOthers : c.outAll) }}>{self ? c.outOthers : c.outAll}</Button>}>{sso && <span {...stylex.props(sx.small)}>{ssoCopy.devices} {ssoCopy.revoke}</span>}{!!devices.error && <StaleBanner error={devices.error} onRetry={() => void devices.refresh()} />}{sessionList.length ? <div {...stylex.props(sx.inset)} style={{ padding: '0 14px' }}>{sessionList.map((session) => <div key={session.id} {...stylex.props(sx.project)} style={{ padding: '10px 0' }}><Icon name={/phone|pad|điện thoại/i.test(session.device) ? 'mobile' : 'monitor'} size={18} /><span {...stylex.props(sx.grow, sx.col)} style={{ lineHeight: 1.4 }}><span {...stylex.props(sx.row)} style={{ fontSize: 13.5, fontWeight: 500 }}>{session.device}{session.browser && ` · ${session.browser}`}{session.current && <span {...stylex.props(sx.chip, sx.active)}>{c.thisDevice}</span>}</span><span {...stylex.props(sx.small)}>{c.seenAt(date(session.lastActiveAt))} · {c.remaining(Math.max(0, Math.ceil((new Date(session.expiresAt).getTime() - Date.now()) / 86400000)))}</span></span>{!session.current && <Button style={{ height: 28 }} disabled={disabled} onClick={() => { void run(() => revokeSession(session.id), c.out) }}>{c.out}</Button>}</div>)}</div> : <span {...stylex.props(sx.small)} style={{ fontSize: 13 }}>{current.disabled ? c.lockedDevices : current.mustChangePassword ? c.tempDevices : c.noDevices}</span>}</Section>
      <Section title={c.password}><div {...stylex.props(sx.row)}><Icon name="key" size={17} /><span {...stylex.props(sx.grow, sx.col)} style={{ lineHeight: 1.45 }}><span style={{ fontSize: 13.5 }}>{current.mustChangePassword ? c.usingTemp : c.changedAt(date(current.passwordChangedAt ?? current.updatedAt))}</span><span {...stylex.props(sx.small)}>{self ? c.passwordOwn : c.passwordNote(current.name)}</span></span><Button disabled={self || disabled} onClick={() => open('reset')}>{c.reset}</Button></div></Section>
      <Section title={c.danger}><div {...stylex.props(sx.danger)}><span {...stylex.props(sx.grow, sx.col)} style={{ lineHeight: 1.45 }}><span style={{ fontSize: 13.5, fontWeight: 500 }}>{current.disabled ? c.unlockAccount : c.lockAccount}</span><span {...stylex.props(sx.small)}>{current.disabled ? c.unlockHint : c.lockHint}</span></span><Button disabled={noDanger || disabled} onClick={() => open('lock')}>{current.disabled ? c.unlock : c.lock}</Button></div><div {...stylex.props(sx.danger)}><span {...stylex.props(sx.grow, sx.col)} style={{ lineHeight: 1.45 }}><span style={{ fontSize: 13.5, fontWeight: 500 }}>{c.removeAccount}</span><span {...stylex.props(sx.small)}>{c.deleteHint(current.name)}</span></span><Button red disabled={noDanger || disabled} onClick={() => open('remove')}><Icon name="trash" size={14} />{c.remove}</Button></div>{noDanger && <span {...stylex.props(sx.small)}>{self ? c.selfDanger : c.lastDanger}</span>}</Section>
      </section>}
    </div> : <div {...stylex.props(sx.col)} style={{ flex: 1, minHeight: 0, gap: 12, paddingBottom: 20 }}><div {...stylex.props(sx.row)}><div {...stylex.props(sx.filters)}>{['', 'acct', 'perm', 'sec'].map((type, index) => <button key={type} type="button" aria-pressed={logType === type} onClick={() => setLogType(type)} {...stylex.props(sx.chipButton, logType === type && sx.chipOn)}>{c.logTypes[index]}<span className={`num ${stylex.props(sx.muted).className}`}>{auditCounts.data?.items.filter((entry) => (!type || entry.type === type) && (!actor || (actor === 'system' ? entry.actor === null : entry.actor?.username === actor))).length ?? 0}{auditCounts.data?.next ? '+' : ''}</span></button>)}</div><span style={{ flex: 1 }} /><div role="group" aria-label="Người" {...stylex.props(sx.filters)}>{[{ value: '', label: c.everyone }, ...all.map((u) => ({ value: u.username, label: u.name })), { value: 'system', label: c.systemActor }].map((person) => <button key={person.value} type="button" aria-pressed={actor === person.value} onClick={() => setActor(person.value)} {...stylex.props(sx.chipButton, actor === person.value && sx.chipOn)}>{person.label}</button>)}</div></div><section className={`scroll ${stylex.props(sx.card, sx.logCard).className}`}>{days.map((day) => <div key={day}><div {...stylex.props(sx.small)} style={{ padding: '14px 2px 6px', fontWeight: 600 }}>{day}</div><div {...stylex.props(sx.inset)}>{entries.filter((entry) => date(entry.at) === day).map((entry) => <div key={entry.id} {...stylex.props(sx.logRow)}><span className={`num ${stylex.props(sx.small).className}`} style={{ paddingTop: 5 }}>{new Date(entry.at).toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' })}</span><span {...stylex.props(sx.avatar, sx.avatarSmall)}>{entry.actor ? entry.actor.name[0] : <Icon name="shield-security" size={13} />}</span><span {...stylex.props(sx.col)} style={{ gap: 5 }}><span style={{ fontSize: 13.5 }}>{entry.actor?.name ?? c.systemActor} {entry.action} {all.find((u) => u.username === entry.target)?.name ?? entry.target}{entry.project ? ` · ${projects.find((p) => p.id === entry.project)?.name ?? c.projects}` : ''}</span>{entry.from !== undefined && <span {...stylex.props(sx.row)} style={{ gap: 6 }}><span {...stylex.props(sx.chip, sx.muted)} style={{ textDecoration: 'line-through' }}>{c.roles[entry.from as Permission]?.[0] ?? entry.from}</span><Icon name="arrow-right-2" size={12} /><span {...stylex.props(sx.chip)}>{c.roles[entry.to as Permission]?.[0] ?? entry.to}</span></span>}{entry.reason && <span {...stylex.props(sx.small)}>“{entry.reason}”</span>}</span></div>)}</div></div>)}{!entries.length && <p {...stylex.props(sx.small)} style={{ padding: '40px 0', textAlign: 'center' }}>{c.noLog}</p>}{next && <Button disabled={disabled} onClick={() => { void loadMore() }}>{c.more}</Button>}<p {...stylex.props(sx.small)} style={{ margin: '16px 2px 0', fontSize: 12 }}>{c.logNote}</p></section></div>}
    {changes > 0 && <div role="region" aria-label="Thay đổi chưa lưu" {...stylex.props(sx.savebar)}><Icon name="info-circle" size={17} /><span>{c.saveText(changes, changed.map((item) => item.user.name).join(', '))}</span><Button style={{ height: 34 }} onClick={() => setDrafts({})} disabled={disabled}>{c.cancel}</Button><Button primary style={{ height: 34 }} onClick={() => { void save() }} disabled={disabled}>{c.save}</Button></div>}
    {dialog === 'add' && <AddMember users={all} sso={sso} disabled={disabled} onClose={close} onCreate={async (input) => {
      const ok = await run(async () => {
        const result = await createUser(input.username, input.name, input.role, input.projects, input.password)
        // Máy chủ mặc định cấp Noverse cho thành viên trên kho chung; lựa chọn của
        // admin là nguồn đúng nên chỉnh từng app khác lựa chọn ngay sau khi tạo.
        // Hiện mật khẩu tạm trước: nếu chỉnh grant lỗi, tài khoản vẫn dùng được.
        setSelected(result.username)
        setSecret({ username: result.username, name: result.name, password: input.password, role: input.role, projects: input.projects })
        if (sso && input.role === 'member') {
          for (const app of appIds) {
            const have = result.grants?.[app] != null
            if (input.apps[app] === have) continue
            if (input.apps[app]) await grantAppAccess(input.username, app)
            else await revokeAppAccess(input.username, app)
          }
        }
      })
      if (ok) setDialog(null); return ok === true
    }} error={error} copy={copy} />}
    {dialog && dialog !== 'add' && current && <Dialog padBody={false} label={c.confirmTitle(confirmAction, current.name)} width={460} alertDialog onClose={close} style={{ borderRadius: 20, background: 'var(--members-dialog-bg)', lineHeight: 1.5 }}><div style={{ padding: '20px 22px 4px', display: 'flex', gap: 14 }}><span {...stylex.props(sx.confirmBadge, dialog === 'reset' && sx.resetBadge)} aria-hidden="true"><Icon name={dialog === 'reset' ? 'key' : dialog === 'remove' ? 'trash' : current.disabled ? 'unlock' : 'lock'} size={19} /></span><div {...stylex.props(sx.col)} style={{ gap: 6 }}><h2 style={{ margin: 0, fontSize: 16, fontWeight: 600 }}>{c.confirmTitle(confirmAction, current.name)}</h2>{confirmLines.map((text) => <span key={text} style={{ display: 'flex', gap: 8, fontSize: 13, color: 'var(--color-fg-2)' }}><i aria-hidden="true" style={{ width: 5, height: 5, marginTop: 8, borderRadius: '50%', background: 'var(--ring-paused)', flexShrink: 0 }} />{text}</span>)}</div></div>{dialog !== 'reset' && !current.disabled || dialog === 'remove' ? <label {...stylex.props(sx.field)} style={{ margin: '12px 22px 0 76px' }}><span {...stylex.props(sx.label)}>{c.reason}</span><input {...stylex.props(sx.input)} style={{ height: 36, fontSize: 13.5 }} value={reason} onChange={(e) => setReason(e.target.value)} placeholder={dialog === 'remove' ? c.reasonRemove : c.reasonLock} /></label> : null}{error && <p role="alert" {...stylex.props(sx.err)} style={{ padding: '0 22px' }}>{error}</p>}<div {...stylex.props(sx.foot)} style={{ borderTop: 'none', padding: '18px 22px' }}>{dialog === 'remove' && !current.disabled && <Button big onClick={() => setDialog('lock')}>{c.lockInstead}</Button>}<span style={{ flex: 1 }} /><Button big disabled={disabled} onClick={close}>{c.cancel}</Button><Button big red={dialog !== 'reset' && !current.disabled} primary={dialog === 'reset' || current.disabled} disabled={disabled} onClick={() => { void confirm() }}>{dialog === 'reset' ? c.reset : confirmAction}</Button></div></Dialog>}
    {secret && <Dialog padBody={false} label={secret.reset ? c.resetDone(secret.name) : c.created(secret.name)} width={500} onClose={close} style={{ borderRadius: 20, background: 'var(--members-dialog-bg)', lineHeight: 1.5 }}><div {...stylex.props(sx.col)} style={{ padding: '24px 22px 6px', alignItems: 'center', gap: 10, textAlign: 'center' }}><span {...stylex.props(sx.successBadge)} aria-hidden="true"><Icon name="tick-circle" size={24} /></span><h2 style={{ margin: 0, fontSize: 17, fontWeight: 600 }}>{secret.reset ? c.resetDone(secret.name) : c.created(secret.name)}</h2><span {...stylex.props(sx.small)} style={{ fontSize: 13, maxWidth: 400, color: 'var(--color-fg-3)' }}>{secret.reset ? c.resetSub(secret.name) : c.createdSub(secret.name, summary({ role: secret.role ?? 'member', projects: secret.projects ?? {} }), secret.role !== 'admin' && Object.keys(secret.projects ?? {}).length > 0 && Object.keys(secret.projects ?? {}).length < projects.length)}</span></div><div {...stylex.props(sx.inset)} style={{ margin: '14px 22px 0' }}>{[[c.username, secret.username], [c.tempPassword, secret.password], [c.openNoverse, location.origin]].map(([label, value]) => <div key={label} {...stylex.props(sx.credential)}><span {...stylex.props(sx.small)}>{label}</span><span className={`num ${stylex.props(sx.ellipsis).className}`} title={value} style={{ fontSize: 14 }}>{value}</span>{label === c.tempPassword && <Button style={{ height: 28, borderWidth: 0 }} onClick={() => { void copy(secret.password) }}><Icon name="copy" size={14} />{c.copyShort}</Button>}</div>)}</div><div {...stylex.props(sx.secretNote)}><Icon name="warning-2" size={15} />{c.once}</div>{error && <p role="alert" {...stylex.props(sx.err)}>{error}</p>}<div {...stylex.props(sx.row)} style={{ padding: '16px 22px 18px', gap: 8 }}><Button big onClick={() => { void copy(`${c.username}: ${secret.username}\n${c.tempPassword}: ${secret.password}\n${c.openNoverse}: ${location.origin}`, c.messageCopied) }}><Icon name="messages-2" size={15} />{c.copyMessage(secret.name)}</Button><Button big onClick={() => { const target = secret.username; close(); setPairFor(target) }}><Icon name="mobile" size={15} />{c.phone}</Button><span style={{ flex: 1 }} /><Button primary big onClick={close}>{c.done}</Button></div></Dialog>}
    {pairFor && <PairingDialog forUser={pairFor} onClose={() => setPairFor(null)} />}
  </main>
}

type NewMember = { username: string; name: string; role: Role; projects: ProjectPermissions; password: string; apps: Record<AppId, boolean> }
function AddMember({ users, sso, disabled, onClose, onCreate, error, copy }: { users: User[]; sso: boolean; disabled: boolean; onClose: () => void; onCreate: (input: NewMember) => Promise<boolean>; error: string; copy: (text: string) => Promise<void> }) {
  const { library } = useLibrary()
  const projects = library?.projects ?? []
  const [name, setName] = useState('')
  const [username, setUsername] = useState('')
  const [touched, setTouched] = useState(false)
  const [checkedUsername, setCheckedUsername] = useState('')
  const [role, setRole] = useState<Role>('member')
  const [permissions, setPermissions] = useState<ProjectPermissions>({})
  // Thành viên mới mặc định được vào Noverse; Viderse chỉ khi quản trị chọn.
  const [apps, setApps] = useState<Record<AppId, boolean>>({ noverse: true, viderse: false })
  const [password, setPassword] = useState(temporaryPassword)
  useEffect(() => { const timer = setTimeout(() => setCheckedUsername(username), 250); return () => clearTimeout(timer) }, [username])
  const duplicate = users.some((entry) => entry.username === checkedUsername)
  const invalid = !!username && !/^[a-z0-9._-]{2,32}$/.test(username)
  const taken = users.some((entry) => entry.username === username)
  let suggest = `${username}.2`
  for (let n = 2; users.some((entry) => entry.username === suggest); n++) suggest = `${username}.${n + 1}`
  const off = disabled || !name.trim() || !username || invalid || taken
  const short = name.trim() || c.member.toLocaleLowerCase('vi')
  return <Dialog padBody={false} label={c.add} width={560} onClose={onClose} style={{ borderRadius: 20, background: 'var(--members-dialog-bg)', lineHeight: 1.5 }}>
    <header style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '18px 20px 14px', borderBottom: '1px solid var(--line-2)', flexShrink: 0 }}>
      <Icon name="user-add" size={19} />
      <h2 style={{ margin: 0, fontSize: 16, fontWeight: 600, flex: 1 }}>{c.add}</h2>
      <Button style={{ width: 32, padding: 0, borderWidth: 0 }} label={c.close} onClick={onClose}><Icon name="close-circle-bulk" size={18} /></Button>
    </header>
    <div {...stylex.props(sx.form)}>
      <div {...stylex.props(sx.roleGrid)} style={{ gap: 12 }}>
        <label {...stylex.props(sx.field)}>
          <span {...stylex.props(sx.label)}>{c.name}</span>
          <input {...stylex.props(sx.input)} value={name} onChange={(e) => { setName(e.target.value); if (!touched) setUsername(suggestUsername(e.target.value)) }} placeholder={c.nameExample} />
        </label>
        <label {...stylex.props(sx.field)}>
          <span {...stylex.props(sx.label)}>{c.username}</span>
          <input {...stylex.props(sx.input)} value={username} onChange={(e) => { setUsername(e.target.value.toLowerCase()); setTouched(true) }} placeholder="ngoc" aria-invalid={duplicate || invalid} aria-describedby="member-username-hint" />
          <span id="member-username-hint" {...stylex.props(sx.small, (duplicate || invalid) && sx.err)} style={{ fontSize: 12 }}>{duplicate ? c.duplicate(username, suggest) : invalid ? c.usernameBad : c.usernameHint}</span>
        </label>
      </div>
      <div {...stylex.props(sx.field)} style={{ gap: 8 }}>
        <span {...stylex.props(sx.label)}>{c.system}</span>
        <SystemRoles compact role={role} disabled={disabled} onChange={setRole} />
      </div>
      {sso && (
        <div {...stylex.props(sx.field)} style={{ gap: 8 }}>
          <span {...stylex.props(sx.label)}>{c.apps}</span>
          {role === 'member' ? (
            <>
              <div {...stylex.props(sx.inset)}>
                {appIds.map((app) => (
                  <div key={app} {...stylex.props(sx.project)}>
                    <Icon name={appIcons[app]} size={16} />
                    <span {...stylex.props(sx.grow, sx.col)} style={{ lineHeight: 1.35 }}>
                      <span style={{ fontSize: 13.5, fontWeight: 500 }}>{c.appNames[app]}</span>
                    </span>
                    <Checkbox checked={apps[app]} disabled={disabled} aria-label={c.appAccess(c.appNames[app], short)} onChange={(next) => setApps((old) => ({ ...old, [app]: next }))} />
                  </div>
                ))}
              </div>
              {!apps.noverse && !apps.viderse && <span {...stylex.props(sx.small, sx.warn)} style={{ fontSize: 12 }}>{c.appNoneHint}</span>}
            </>
          ) : (
            <span {...stylex.props(sx.small)} style={{ fontSize: 12 }}>{c.appInherited}</span>
          )}
        </div>
      )}
      {role === 'member' && (
        <div {...stylex.props(sx.field)} style={{ gap: 8 }}>
          <span {...stylex.props(sx.label)}>{c.allowedProjects}</span>
          <div {...stylex.props(sx.inset)}>
            {projects.map((project) => (
              <div key={project.id} {...stylex.props(sx.project)}>
                <Icon name="folder-open" size={16} />
                <span {...stylex.props(sx.grow, sx.col)} style={{ lineHeight: 1.35 }}>
                  <span style={{ fontSize: 13.5, fontWeight: 500 }} title={project.name}>{project.name}</span>
                  <span {...stylex.props(sx.small)} style={{ fontSize: 12 }}>{project.storyCount ? c.stories(project.storyCount) : c.noStories}</span>
                </span>
                <div role="radiogroup" aria-label={c.projectRole(project.name)} {...stylex.props(sx.seg)} style={{ padding: 2 }}>
                  {roleKeys.map((value) => (
                    <button key={value} type="button" role="radio" aria-checked={(permissions[project.id] ?? 'none') === value} onClick={() => { const next = { ...permissions }; if (value === 'none') delete next[project.id]; else next[project.id] = value; setPermissions(next) }} {...stylex.props(sx.segButton, (permissions[project.id] ?? 'none') === value && sx.segOn)} style={{ height: 26, padding: '0 9px', fontSize: 12 }}>{c.roles[value][0]}</button>
                  ))}
                </div>
              </div>
            ))}
          </div>
          <span {...stylex.props(sx.small, !Object.keys(permissions).length && sx.warn)} style={{ fontSize: 12 }}>{Object.keys(permissions).length ? c.later : c.unassigned(short)}</span>
        </div>
      )}
      <label {...stylex.props(sx.field)}>
        <span {...stylex.props(sx.label)}>{c.tempPassword}</span>
        <span {...stylex.props(sx.row)} style={{ gap: 8 }}>
          <input {...stylex.props(sx.input)} className={`num ${stylex.props(sx.input).className}`} value={password} readOnly aria-label={c.tempPassword} />
          <Button big onClick={() => { void copy(password) }}><Icon name="copy" size={15} />{c.copy}</Button>
          <Button big label={c.regenerate} onClick={() => setPassword(temporaryPassword())}><Icon name="refresh-2" size={15} /></Button>
        </span>
        <span {...stylex.props(sx.small)} style={{ fontSize: 12 }}>{c.tempHint(short)}</span>
      </label>
      {error && <span role="alert" {...stylex.props(sx.err)}>{error}</span>}
    </div>
    <div {...stylex.props(sx.foot)}>
      <span {...stylex.props(sx.small, sx.grow)}>{taken || invalid ? c.fixUsername : off ? c.createHint : `${name} · ${role === 'admin' ? c.admin : `${Object.keys(permissions).length} dự án`}`}</span>
      <Button big disabled={disabled} onClick={onClose}>{c.cancel}</Button>
      <Button primary big disabled={off} onClick={() => { void onCreate({ username, name: name.trim(), role, projects: permissions, password, apps }) }}>{c.create}</Button>
    </div>
  </Dialog>
}
