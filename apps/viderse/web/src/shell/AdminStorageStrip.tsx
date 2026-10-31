import * as stylex from '@stylexjs/stylex'
import { Button, Icon } from '@noverse/ui'
import type { Status } from '../api/status'
import type { User } from '../api/types'
import { serverStorageCopy as t } from '../i18n/storageServer'
import { fill } from '../screens/settings/fmt'
import { styles as s } from '../screens/settings/storage/serverStorage.styles'
import { navigate, paths } from '../router'

export function AdminStorageStrip({ user, status }: { user: User; status?: Status }) {
  const admin = user.role === 'admin' || user.grants?.viderse === 'admin'
  if (!admin || !status?.storageDown) return null
  return <div role="alert" {...stylex.props(s.strip)}><Icon name="danger" size={16} /><span {...stylex.props(s.grow)}><b>{fill(t.stripTitle, { label: status.storageDown.label })}</b> {status.running > 0 ? fill(t.held, { n: status.running }) : t.downBody}</span><Button variant="ghost" size="compact" onClick={() => navigate(paths.settings('storage'))}>{t.openStorage}</Button></div>
}
