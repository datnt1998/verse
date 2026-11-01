import * as stylex from '@stylexjs/stylex'
import { Button, Icon } from '@noverse/ui'
import { color } from '@noverse/ui/tokens.stylex'
import type { Status } from '../api/status'
import type { User } from '../api/types'
import { t } from '../i18n/vi'
import { fill } from '../screens/settings/fmt'
import { navigate, paths } from '../router'

// Dải này nằm trong khung app nên giữ style riêng, không kéo cả stylesheet của
// màn Lưu trữ vào entry.
const s = stylex.create({
  strip: { flexShrink: 0, display: 'flex', alignItems: 'center', gap: 10, padding: '8px 16px', backgroundColor: 'var(--vv-err-11)', borderBottomWidth: 1, borderBottomStyle: 'solid', borderBottomColor: 'var(--vv-err-28)', fontSize: 13, color: color['--color-err'] },
  grow: { flex: 1, minWidth: 0 },
})

export function AdminStorageStrip({ user, status }: { user: User; status?: Status }) {
  const admin = user.role === 'admin' || user.grants?.viderse === 'admin'
  if (!admin || !status?.storageDown) return null
  const c = t.shell.storageDown
  return <div role="alert" {...stylex.props(s.strip)}><Icon name="danger" size={16} /><span {...stylex.props(s.grow)}><b>{fill(c.title, { label: status.storageDown.label })}</b> {status.running > 0 ? fill(c.held, { n: status.running }) : c.body}</span><Button variant="ghost" size="compact" onClick={() => navigate(paths.settings('storage'))}>{c.open}</Button></div>
}
