// MobileShell — khung điện thoại: header 1 dòng (brand) · nội dung cuộn · tab đáy.
// App chọn khung này khi matchMedia('(max-width: 759px)') — cùng ranh giới Noverse.
import type { ReactNode } from 'react'
import { t } from '../i18n/vi'
import { navigate, paths, type Route } from '../router'
import { BrandLockup, Icon } from '@noverse/ui'

// Tab đáy: Trang chủ · Tài khoản (cùng mục với MainNav desktop).
const MOBILE_TABS = [
  { key: 'home', route: 'home', label: t.nav.home, icon: 'category', iconOn: 'category-bulk' },
  { key: 'account', route: 'account', label: t.nav.account, icon: 'user', iconOn: 'profile-2user' },
] as const

const hrefOf = (key: (typeof MOBILE_TABS)[number]['key']) => `#${key === 'account' ? paths.account() : paths.home()}`

export function MobileTabBar({ route }: { route: Route }) {
  return (
    <nav aria-label={t.nav.bottom} style={{
      flex: '0 0 auto', display: 'grid', gridTemplateColumns: `repeat(${MOBILE_TABS.length}, minmax(0, 1fr))`,
      padding: '4px 6px calc(22px + env(safe-area-inset-bottom, 0px))',
      borderTop: '1px solid var(--line-1)', background: 'var(--color-bg)',
    }}>
      {MOBILE_TABS.map((tab) => {
        const on = route.name === tab.route
        return (
          <button key={tab.key} type="button" aria-current={on ? 'page' : undefined}
            onClick={() => navigate(hrefOf(tab.key).slice(1))}
            style={{
              display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 3,
              padding: '8px 0 2px', border: 0, background: 'none',
              color: on ? 'var(--color-fg)' : 'var(--color-fg-4)',
              fontSize: 11, fontWeight: on ? 600 : 400, touchAction: 'manipulation',
            }}>
            <Icon name={on ? tab.iconOn : tab.icon} size={21} />
            {tab.label}
          </button>
        )
      })}
    </nav>
  )
}

export function MobileShell({ children, tabs, style }: {
  children: ReactNode
  /** route để tô mục tab đang ở */
  tabs?: Route
  style?: React.CSSProperties
}) {
  return (
    <div style={{
      height: '100dvh', minHeight: '100dvh', display: 'flex', flexDirection: 'column',
      background: 'var(--color-bg)', color: 'var(--color-fg)', overflow: 'hidden', ...style,
    }}>
      <header style={{
        flex: '0 0 auto', display: 'flex', alignItems: 'center', gap: 10,
        height: 52, padding: '0 16px', borderBottom: '1px solid var(--line-1)',
      }}>
        <BrandLockup size={22} />
      </header>
      <main className="scroll" style={{ flex: 1, minHeight: 0, padding: '12px 14px', display: 'flex', flexDirection: 'column', gap: 12 }}>
        {children}
      </main>
      {tabs && <MobileTabBar route={tabs} />}
    </div>
  )
}
