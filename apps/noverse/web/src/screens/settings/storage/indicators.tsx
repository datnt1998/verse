import * as stylex from '@stylexjs/stylex'
import { Icon, ProgressRing } from '@noverse/ui'
import { useSession } from '../../../api/session'
import { storageCopy as c } from '../../../i18n/vi'
import { paths } from '../../../router'
import { migrating, num, percent } from './logic'
import { storageSummary } from './overview'
import { useStorage } from './resource'
import { sx } from './styles'

export function StorageNavSummary() {
  const { data } = useStorage()
  return data ? <span {...stylex.props(sx.navSub, data.health?.error ? sx.errorText : migrating(data.migration) ? sx.infoText : undefined)}>{storageSummary(data)}</span> : null
}
export function StoragePill({ mobile = false }: { mobile?: boolean }) {
  const { user } = useSession()
  const { data } = useStorage(false, true, !mobile)
  if (user?.role !== 'admin' || !data) return null
  const down = !!data.health?.error
  const migration = data.migration
  if (!down && !migrating(migration)) return null
  const error = !!migration && (migration.errorCount > 0 || migration.state === 'error')
  const paused = migration?.state === 'paused'
  const pct = percent(migration?.copiedCount ?? 0, migration?.totalCount ?? 0)
  const label = down ? c.downPill : error ? `${mobile ? '' : 'Chuyển kho: '}${num(migration?.errorCount ?? 0)} tệp lỗi` : `${paused ? mobile ? 'Tạm dừng' : 'Chuyển kho tạm dừng' : mobile ? 'Chuyển kho' : 'Chuyển kho tệp'} · ${pct}%`
  return <a href={`#${paths.settings('storage')}`} title={label} {...stylex.props(sx.pill, down ? sx.error : error ? sx.warning : paused ? sx.pausedChip : undefined)}>{down ? <Icon name="warning-2" size={13} /> : <ProgressRing percent={pct} size={14} />}{label}</a>
}
export function StorageBanner({ mobile = false }: { mobile?: boolean }) {
  const { user } = useSession()
  const { data } = useStorage()
  if (user?.role !== 'admin' || !data?.health?.error) return null
  return <a href={`#${paths.settings('storage')}`} role="alert" {...stylex.props(sx.banner)}><Icon name="warning-2" size={15} /><span>{mobile ? c.downMobile : c.downLine}</span>{!mobile && <span>{c.view}</span>}</a>
}
