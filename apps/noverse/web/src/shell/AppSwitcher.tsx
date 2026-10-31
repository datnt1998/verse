import * as stylex from '@stylexjs/stylex'
import { BrandMark } from '@noverse/ui'
import { useSession } from '../api/session'
import { ssoCopy as c } from '../i18n/sso'

const sx = stylex.create({
  group: { display: 'flex', flexDirection: 'column' },
  mobileSection: { margin: '18px 16px 0' },
  mobileLogo: { width: 34, height: 34 },
  mobileName: { fontSize: 15, fontWeight: 400 },
  mobileHost: { fontSize: 12 },
  mobile: { borderWidth: 1, borderStyle: 'solid', borderColor: 'var(--line-2)', borderRadius: 16, backgroundColor: 'var(--color-surface-1)', overflow: 'hidden' },
  heading: { padding: '6px 10px 4px', fontSize: 11.5, fontWeight: 600, letterSpacing: '.06em', textTransform: 'uppercase', color: 'var(--color-fg-4)' },
  mobileHeading: { padding: '0 4px 8px', fontSize: 12.5, textTransform: 'none', letterSpacing: 'normal', fontWeight: 400 },
  divider: { height: 1, margin: 6, backgroundColor: 'var(--line-2)' },
  row: { minHeight: 52, padding: '6px 10px', display: 'flex', alignItems: 'center', gap: 12, borderRadius: 10, textDecoration: 'none', color: 'var(--color-fg)', ':hover': { backgroundColor: 'var(--color-surface-2)' } },
  mobileRow: { minHeight: 56, padding: '6px 14px', borderRadius: 0, borderBottomWidth: 1, borderBottomStyle: 'solid', borderBottomColor: 'var(--line-2)', ':last-child': { borderBottomWidth: 0 } },
  logo: { width: 32, height: 32, borderRadius: '50%', flexShrink: 0, display: 'flex', alignItems: 'center', justifyContent: 'center', backgroundColor: 'var(--color-surface-2)', borderWidth: 1, borderStyle: 'solid', borderColor: 'var(--line-3)' },
  copy: { flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', lineHeight: 1.3 },
  name: { fontSize: 13.5, fontWeight: 500 },
  host: { fontSize: 11.5, color: 'var(--color-fg-4)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' },
  current: { padding: '2px 7px', borderRadius: 6, fontSize: 11, color: 'var(--color-accent)', backgroundColor: 'var(--tint-accent)', whiteSpace: 'nowrap' },
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
      return <a key={app.id} href={app.origin} target={current ? undefined : '_blank'} rel={current ? undefined : 'noopener noreferrer'} role={mobile ? undefined : 'menuitem'} aria-current={current ? 'true' : undefined} onClick={current ? (event) => event.preventDefault() : undefined} {...stylex.props(sx.row, mobile && sx.mobileRow)}>
        <span {...stylex.props(sx.logo, mobile && sx.mobileLogo)} aria-hidden="true">{current ? <BrandMark size={18} /> : <img src="/icons/viderse-mark.svg" alt="" width="20" height="20" />}</span>
        <span {...stylex.props(sx.copy)}><span {...stylex.props(sx.name, mobile && sx.mobileName)}>{app.name}</span><span {...stylex.props(sx.host, mobile && sx.mobileHost)} style={{ fontFamily: 'var(--font-mono)' }} title={app.origin}>{new URL(app.origin).host}</span></span>
        {current ? <span {...stylex.props(sx.current)}>{c.current}</span> : <ExternalAppIcon />}
      </a>
    })}</div>
  </section>
}
