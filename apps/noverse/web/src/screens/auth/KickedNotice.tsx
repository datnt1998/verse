import * as stylex from '@stylexjs/stylex'
import { Icon } from '@noverse/ui'
import { useSession } from '../../api/session'
import { ssoCopy } from '../../i18n/sso'
import { surface } from '@noverse/ui/tokens.stylex'
const sx = stylex.create({
  notice: { borderWidth: 1, borderStyle: 'solid', borderColor: 'color-mix(in srgb, var(--kit-info-fg) 30%, transparent)', borderRadius: { default: 14, '@media (max-width: 767px)': 16 }, padding: '12px 14px', display: 'flex', alignItems: 'flex-start', gap: 12, backgroundImage: surface['--card-bg'], boxShadow: surface['--card-shadow'] },
  icon: { color: 'var(--kit-info-fg)', flexShrink: 0, marginTop: 2 },
  copy: { display: 'flex', flexDirection: 'column', gap: 4, lineHeight: 1.5, fontSize: { default: 13, '@media (max-width: 767px)': 13.5 } },
  title: { fontWeight: 500 },
  body: { color: 'var(--color-fg-3)' },
})
export function KickedNotice() {
  const { kicked, sharedAccounts } = useSession()
  if (!sharedAccounts || !kicked || kicked === 'expired') return null
  const message = ssoCopy.kicked[kicked]
  return <div role="status" {...stylex.props(sx.notice)}><Icon name={message.icon} size={18} {...stylex.props(sx.icon)} /><div {...stylex.props(sx.copy)}><span {...stylex.props(sx.title)}>{message.title}</span><span {...stylex.props(sx.body)}>{message.body}</span></div></div>
}
