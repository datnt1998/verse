// MainNav — nhóm link segmented ở header: Trang chủ · Tài khoản.
// Bề rộng cố định theo nhãn (đậm 500 ở mọi trạng thái) nên mục đang chọn chỉ đổi
// màu, không đẩy các mục bên cạnh.
import type { CSSProperties } from 'react'
import { t } from '../i18n/vi'
import { paths, type Route } from '../router'
import { Icon } from '@noverse/ui'

export type NavKey = 'home' | 'account'

const ITEMS: { key: NavKey; route: Route['name']; label: string; icon: string; href: string }[] = [
  { key: 'home', route: 'home', label: t.nav.home, icon: 'category', href: `#${paths.home()}` },
  { key: 'account', route: 'account', label: t.nav.account, icon: 'user', href: `#${paths.account()}` },
]

const itemStyle = (on: boolean): CSSProperties => ({
  height: 30, padding: '0 12px 0 10px', display: 'flex', alignItems: 'center', gap: 8,
  borderRadius: 7, color: on ? 'var(--color-fg)' : 'var(--color-fg-3)',
  background: on ? 'var(--color-surface-2)' : 'transparent',
  fontSize: 13, fontWeight: 500, whiteSpace: 'nowrap', transition: 'color 160ms ease',
})

export function MainNav({ route }: { route: Route }) {
  return (
    <nav aria-label={t.nav.main} style={{ display: 'flex', alignItems: 'center', gap: 2, padding: 3, borderRadius: 9, background: 'var(--color-surface-1)', border: '1px solid var(--line-1)' }}>
      {ITEMS.map((item) => {
        const on = route.name === item.route
        return (
          <a key={item.key} href={item.href} aria-current={on ? 'page' : undefined} style={itemStyle(on)}>
            <Icon name={item.icon} size={15} style={on ? { color: 'var(--color-accent)' } : undefined} />
            {item.label}
          </a>
        )
      })}
    </nav>
  )
}
