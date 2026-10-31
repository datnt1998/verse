import * as stylex from '@stylexjs/stylex'
import { BrandMark, Icon } from '@noverse/ui'
import { color, line, kit } from '@noverse/ui/tokens.stylex'
import type { AppLink } from '../api/types'
import { LensLogo } from './LensLogo'
import { t } from '../i18n/vi'
import { useMediaQuery } from '../lib/useMediaQuery'
const sx = stylex.create({
  row: { minHeight: 52, padding: '6px 10px', display: 'flex', alignItems: 'center', gap: 12, borderRadius: 10, color: color['--color-fg'], textDecoration: 'none', backgroundColor: { default: 'transparent', ':hover': kit['--kit-wash-5'] } },
  mobileRow: { height: 56, borderRadius: 0, padding: '6px 14px', gap: 12, borderBottomWidth: 1, borderBottomStyle: 'solid', borderBottomColor: 'var(--vv-hairline)', ':last-child': { borderBottomWidth: 0 } },
  mobileLogo: { width: 34, height: 34, borderRadius: '50%' },
  mobileName: { fontSize: 15, fontWeight: 400 },
  mobileHost: { fontSize: 12 },
  current: { backgroundColor: line['--line-3'] },
  logo: { width: 32, height: 32, borderRadius: '50%', display: 'grid', placeItems: 'center', flexShrink: 0, backgroundColor: color['--color-surface-3'], borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-2'] },
  text: { flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', lineHeight: 1.3 },
  name: { fontSize: 13.5, fontWeight: 500 },
  host: { fontFamily: 'var(--font-mono)', fontSize: 11.5, color: 'var(--vv-fg-note)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' },
  tag: { display: 'inline-flex', alignItems: 'center', height: 22, paddingInline: 8, gap: 6, borderRadius: 6, borderWidth: 0, backgroundColor: kit['--kit-acc-12'], color: 'var(--vv-tabbar-on)', fontSize: 11.5 },
  neutralTag: { backgroundColor: kit['--kit-wash-6'], color: color['--color-fg-3'] },
  help: { fontSize: 12.5, lineHeight: 1.55, color: color['--color-fg-3'] },
  fieldHelp: { fontSize: 11.5, lineHeight: 1.5, color: 'var(--vv-fg-note)' },
  button: { display: 'inline-flex', alignItems: 'center', justifyContent: 'center', height: 36, gap: 7, padding: '0 14px', borderRadius: 10, borderWidth: 1, borderStyle: 'solid', borderColor: kit['--kit-wash-12'], backgroundColor: { default: 'transparent', ':hover': kit['--kit-wash-5'] }, color: color['--color-fg'], textDecoration: 'none', fontSize: 13, lineHeight: 1.55, fontWeight: 500 },
  primary: { backgroundColor: { default: color['--color-fg'], ':hover': color['--color-fg-2'] }, borderColor: 'transparent', color: color['--color-bg'] },
})
export { sx as accountStyles }
export function AppLinks({ apps, menu = false }: { apps: readonly AppLink[]; menu?: boolean }) {
  const mobile = useMediaQuery('(max-width: 767px)') && !menu
  return apps.map((app) => <a key={app.id} href={app.id === 'viderse' ? '#/account' : app.origin}
    target={app.id === 'viderse' ? undefined : '_blank'} rel={app.id === 'viderse' ? undefined : 'noopener noreferrer'}
    role={menu ? 'menuitem' : undefined} aria-current={app.id === 'viderse' ? 'true' : undefined}
    {...stylex.props(sx.row, app.id === 'viderse' && !mobile && sx.current, mobile && sx.mobileRow)}>
    <span {...stylex.props(sx.logo, mobile && sx.mobileLogo)}>{app.id === 'viderse' ? <LensLogo size={20} /> : <BrandMark size={22} style={{ width: 22 }} />}</span>
    <span {...stylex.props(sx.text)}><span {...stylex.props(sx.name, mobile && sx.mobileName)}>{app.name}</span><span {...stylex.props(sx.host, mobile && sx.mobileHost)} title={app.origin}>{new URL(app.origin).host}</span></span>
    {app.id === 'viderse' ? <span {...stylex.props(sx.tag)}>{t.account.currentApp}</span> : <ExternalIcon />}
  </a>)
}
export function ExternalIcon() {
  const mobile = useMediaQuery('(max-width: 767px)')
  return <svg aria-hidden="true" width={mobile ? 16 : 14} height={mobile ? 16 : 14} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" style={{ flexShrink: 0, color: 'var(--kit-fg-sub)' }}><path d="M14 4h6v6M20 4l-9 9M18 14v4a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h4" /></svg>
}
export function MembersLink({ href, settings = false }: { href: string; settings?: boolean }) {
  return <a href={href} target="_blank" rel="noopener noreferrer" {...(settings ? { className: 'vv-set__nav-link' } : stylex.props(sx.row))}>
    <Icon name="crown" size={17} />
    <span style={{ flex: 1 }}>{settings ? t.account.membersSettings : t.account.manageMembers}</span><ExternalIcon />
  </a>
}
