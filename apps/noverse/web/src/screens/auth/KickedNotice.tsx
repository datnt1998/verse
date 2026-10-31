import * as stylex from '@stylexjs/stylex'
import { Icon } from '@noverse/ui'
import { useSession } from '../../api/session'
import { ssoCopy } from '../../i18n/sso'
const sx = stylex.create({
  notice: { borderWidth: 1, borderStyle: 'solid', borderColor: 'var(--line-3)', borderRadius: 12, padding: '12px 14px', display: 'flex', alignItems: 'flex-start', gap: 10, backgroundColor: 'var(--color-surface-1)' },
  icon: { color: 'var(--color-info)', flexShrink: 0, marginTop: 2 },
  success: { color: 'var(--color-ok)' },
  copy: { display: 'flex', flexDirection: 'column', gap: 3, lineHeight: 1.5 },
  title: { fontSize: 13, fontWeight: 600 },
  body: { fontSize: 12.5, color: 'var(--color-fg-3)' },
})
export function KickedNotice() {
  const { kicked } = useSession()
  if (!kicked) return null
  const message = ssoCopy.kicked[kicked]
  return <div role="status" {...stylex.props(sx.notice)}><Icon name={message.icon} size={18} {...stylex.props(sx.icon, kicked === 'self' && sx.success)} /><div {...stylex.props(sx.copy)}><span {...stylex.props(sx.title)}>{message.title}</span><span {...stylex.props(sx.body)}>{message.body}</span></div></div>
}
