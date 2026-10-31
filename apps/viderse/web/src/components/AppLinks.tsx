import * as stylex from '@stylexjs/stylex'
import { BrandMark, Icon } from '@noverse/ui'
import { color, line, kit } from '@noverse/ui/tokens.stylex'
import type { AppLink } from '../api/types'
import { LensLogo } from './LensLogo'
import { t } from '../i18n/vi'
const sx = stylex.create({
  row: { minHeight: 52, padding: '6px 10px', display: 'flex', alignItems: 'center', gap: 12, borderRadius: 10, color: color['--color-fg'], textDecoration: 'none', backgroundColor: { default: 'transparent', ':hover': kit['--kit-wash-5'] } },
  current: { backgroundColor: line['--line-3'] },
  logo: { width: 32, height: 32, borderRadius: '50%', display: 'grid', placeItems: 'center', flexShrink: 0, backgroundColor: color['--color-surface-3'], border: '1px solid', borderColor: line['--line-2'] },
  text: { flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', lineHeight: 1.3 },
  name: { fontSize: 13.5, fontWeight: 500 },
  host: { fontFamily: 'var(--font-mono)', fontSize: 11.5, color: color['--color-fg-4'], overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' },
  tag: { padding: '2px 6px', borderRadius: 5, backgroundColor: line['--line-3'], fontSize: 11.5 },
  help: { fontSize: 12.5, lineHeight: 1.55, color: color['--color-fg-3'] },
  button: { display: 'inline-flex', alignItems: 'center', justifyContent: 'center', gap: 8, padding: '6px 12px', borderRadius: 8, border: '1px solid', borderColor: line['--line-3'], backgroundColor: { default: kit['--kit-wash-5'], ':hover': kit['--kit-wash-10'] }, color: color['--color-fg'], textDecoration: 'none', fontSize: 13, lineHeight: '18px', fontWeight: 500 },
  primary: { backgroundColor: { default: color['--color-fg'], ':hover': color['--color-fg-2'] }, borderColor: 'transparent', color: color['--color-bg'] },
})
export { sx as accountStyles }
export function AppLinks({ apps, menu = false }: { apps: readonly AppLink[]; menu?: boolean }) {
  return apps.map((app) => <a key={app.id} href={app.id === 'viderse' ? '#/account' : app.origin}
    target={app.id === 'viderse' ? undefined : '_blank'} rel={app.id === 'viderse' ? undefined : 'noopener noreferrer'}
    role={menu ? 'menuitem' : undefined} aria-current={app.id === 'viderse' ? 'true' : undefined}
    {...stylex.props(sx.row, app.id === 'viderse' && sx.current)}>
    <span {...stylex.props(sx.logo)}>{app.id === 'viderse' ? <LensLogo size={20} /> : <BrandMark size={22} />}</span>
    <span {...stylex.props(sx.text)}><span {...stylex.props(sx.name)}>{app.name}</span><span {...stylex.props(sx.host)} title={app.origin}>{new URL(app.origin).host}</span></span>
    {app.id === 'viderse' ? <span {...stylex.props(sx.tag)}>{t.account.currentApp}</span> : <ExternalIcon />}
  </a>)
}
export function ExternalIcon() {
  return <svg aria-hidden="true" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round"><path d="M14 4h6v6M20 4l-9 9M18 14v4a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h4" /></svg>
}
export function MembersLink({ href, settings = false }: { href: string; settings?: boolean }) {
  return <a href={href} target="_blank" rel="noopener noreferrer" {...(settings ? { className: 'vv-set__nav-link' } : stylex.props(sx.row))}>
    <Icon name="crown" size={17} />
    <span style={{ flex: 1 }}>{settings ? t.account.membersSettings : t.account.manageMembers}</span><ExternalIcon />
  </a>
}
