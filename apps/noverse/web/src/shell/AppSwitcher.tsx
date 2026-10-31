import * as stylex from '@stylexjs/stylex'
import { BrandMark } from '@noverse/ui'
import { useSession } from '../api/session'
import { ssoCopy as c } from '../i18n/sso'

const sx = stylex.create({
  group: { display: 'flex', flexDirection: 'column' },
  mobileSection: { margin: '18px 16px 0' },
  mobileLogo: { width: 36, height: 36 },
  mobileName: { fontSize: 15, fontWeight: 400 },
  mobileHost: { fontSize: 12 },
  mobile: { borderWidth: 1, borderStyle: 'solid', borderColor: 'var(--line-1)', borderRadius: 16, backgroundColor: 'var(--color-card)', overflow: 'hidden' },
  heading: { padding: '6px 12px 4px', fontSize: 11, fontWeight: 600, letterSpacing: '.08em', textTransform: 'uppercase', color: 'var(--color-fg-4)' },
  mobileHeading: { padding: '0 14px 8px', fontSize: 12.5, textTransform: 'none', letterSpacing: '.02em', fontWeight: 400 },
  divider: { height: 1, margin: 6, backgroundColor: 'var(--line-2)' },
  row: { minHeight: 48, padding: '8px 12px', display: 'flex', alignItems: 'center', gap: 12, borderRadius: 10, textDecoration: 'none', color: 'var(--color-fg)', ':hover': { backgroundColor: 'var(--line-2)' } },
  mobileRow: { minHeight: 54, padding: '8px 14px', borderRadius: 0, borderTopWidth: { default: 1, ':first-child': 0 }, borderTopStyle: 'solid', borderTopColor: 'var(--line-1)' },
  selectedRow: { backgroundColor: 'var(--line-3)' },
  logo: { width: 30, height: 30, borderRadius: '50%', flexShrink: 0, display: 'flex', alignItems: 'center', justifyContent: 'center', backgroundColor: 'var(--color-surface-3)', borderWidth: 1, borderStyle: 'solid', borderColor: 'var(--line-3)' },
  copy: { flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', lineHeight: 1.35 },
  name: { fontSize: 13, fontWeight: 500 },
  host: { fontSize: 11, color: 'var(--color-fg-4)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' },
  current: { height: 20, padding: '0 7px', borderRadius: 6, fontSize: 11, display: 'inline-flex', alignItems: 'center', color: 'var(--color-fg-2)', backgroundColor: 'var(--line-3)', whiteSpace: 'nowrap' },
  mobileCurrent: { height: 22, padding: '0 8px', fontSize: 12 },
  external: { color: 'var(--color-fg-4)', flexShrink: 0 },
})
export function ExternalAppIcon() {
  return <svg {...stylex.props(sx.external)} width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"><path d="M14 4h6v6M20 4l-9 9M18 14v4a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h4" /></svg>
}
export function AppSwitcher({ mobile = false }: { mobile?: boolean }) {
  const { user, apps = [] } = useSession()
  const granted = user?.grants != null && (user.role === 'admin' || user.grants.viderse != null)
  if (!granted || !apps.some((app) => app.id === 'viderse')) return null
  const ordered = [...apps].sort((a, b) => Number(b.id === 'noverse') - Number(a.id === 'noverse'))
  return <section aria-label={c.apps} {...stylex.props(mobile && sx.mobileSection)}>
    {!mobile && <div {...stylex.props(sx.divider)} role="separator" />}
    <div {...stylex.props(sx.heading, mobile && sx.mobileHeading)}>{c.apps}</div>
    <div {...stylex.props(sx.group, mobile && sx.mobile)}>{ordered.map((app) => {
      const current = app.id === 'noverse'
      return <a key={app.id} href={app.origin} target={current ? undefined : '_blank'} rel={current ? undefined : 'noopener noreferrer'} role={mobile ? undefined : 'menuitem'} aria-current={current ? 'true' : undefined} title={current ? undefined : 'Mở Viderse ở thẻ mới'} onClick={current ? (event) => event.preventDefault() : undefined} {...stylex.props(sx.row, mobile && sx.mobileRow, current && sx.selectedRow)}>
        <span {...stylex.props(sx.logo, mobile && sx.mobileLogo)} aria-hidden="true">{current ? <BrandMark size={18} /> : <img src="/icons/viderse-mark.svg" alt="" width="20" height="20" />}</span>
        <span {...stylex.props(sx.copy)}><span {...stylex.props(sx.name, mobile && sx.mobileName)}>{app.name}</span><span {...stylex.props(sx.host, mobile && sx.mobileHost)} style={{ fontFamily: 'var(--font-mono)' }} title={app.origin}>{new URL(app.origin).host}</span></span>
        {current ? <span {...stylex.props(sx.current, mobile && sx.mobileCurrent)}>{c.current}</span> : <ExternalAppIcon />}
      </a>
    })}</div>
  </section>
}
