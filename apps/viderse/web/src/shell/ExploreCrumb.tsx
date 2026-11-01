// Breadcrumb vùng trái thanh đầu (04-components › AppHeader): "Khám phá /
// <đối thủ>". Đọc cùng khoá cache `rivals` mà màn Khám phá dùng nên không thêm
// lượt gọi mạng; chưa theo dõi đối thủ nào thì chỉ hiện tên màn như artboard
// (phần sau dấu / chỉ có khi đã có dữ liệu).
import * as stylex from '@stylexjs/stylex'
import { vi as t } from '../i18n/vi'
import { useResource } from '../data/useResource'
import { keys } from '../data/keys'
import { listRivals } from '../api/rivals'
import { shellChrome } from '../shell/shell.stylex'

const sx = stylex.create({
  row: { display: 'flex', alignItems: 'center', gap: 'var(--vv-s-6)', minWidth: 0 },
  sep: { color: 'var(--color-fg-5)' },
  tail: { color: 'var(--color-fg-2)', overflow: 'hidden', textOverflow: 'ellipsis' },
})

export function ExploreCrumb({ scope }: {
  /** ?rival= của route — handle (@…) hoặc id */
  scope: string | undefined
}) {
  const tt = t.screens.explore
  const list = useResource(keys.rivals(), ({ signal }) => listRivals(signal), { keepPrevious: true })
  const rivals = list.data?.rivals ?? []
  const hit = scope ? rivals.find((r) => r.handle === scope || r.id === scope) : undefined
  const tail = rivals.length === 0 ? undefined : (hit?.title ?? tt.allRivals)
  return (
    <span {...stylex.props(shellChrome.crumb, sx.row)}>
      <span>{tt.title}</span>
      {tail && (
        <>
          <span aria-hidden="true" {...stylex.props(sx.sep)}>/</span>
          <span {...stylex.props(sx.tail)}>{tail}</span>
        </>
      )}
    </span>
  )
}
