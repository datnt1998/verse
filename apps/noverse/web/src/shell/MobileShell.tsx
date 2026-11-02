// MobileShell — khung điện thoại (Phone.dc.html, 09-auth-onboard.md §4 › MobileShell):
// header 1 dòng (do màn truyền vào) · nội dung cuộn · footer nút 50px dính đáy · tab đáy 4 mục.
// AppShell dùng MobileTabBar cho Workspace mobile; màn đăng nhập/QR/onboard dùng MobileShell.
import type { CSSProperties, ReactNode } from 'react'
import { t } from '../i18n/vi'
import { navigate, paths, type Route } from '../router'
import { Icon } from '@noverse/ui'
import { navKeyOf } from './MainNav'

// Tab đáy mobile: Workspace · Nhật ký · Đọc · Dự án (Phone.dc.html › home)
const MOBILE_TABS = [
  { key: 'workspace', label: 'Workspace', icon: 'category' },
  { key: 'activity', label: t.nav.activity, icon: 'activity' },
  { key: 'reader', label: 'Đọc', icon: 'book-1' },
  { key: 'project', label: t.nav.project, icon: 'folder-open' },
] as const

function mobileTabHref(key: string, route: Route): string {
  const p = 'project' in route ? route.project ?? '_' : '_'
  const s = 'story' in route ? route.story : null
  switch (key) {
    case 'workspace': return s ? paths.workspace(p, s) : paths.project(p)
    case 'activity': return s ? paths.activity(p, s) : paths.project(p)
    case 'reader': return s ? paths.reader(p, s) : paths.project(p)
    default: return paths.project(p)
  }
}

export function MobileTabBar({ route }: { route: Route }) {
  const current = navKeyOf(route)
  const activeKey = route.name === 'activity' ? 'activity' : current ?? 'project'
  return (
    <nav aria-label="Điều hướng" style={{
      flex: '0 0 auto', display: 'grid', gridTemplateColumns: `repeat(${MOBILE_TABS.length}, minmax(0, 1fr))`,
      padding: '4px 6px calc(22px + var(--m-safe-bottom))',
      borderTop: '1px solid var(--line-1)', background: 'var(--color-bg)',
    }}>
      {MOBILE_TABS.map((tab) => {
        const on = tab.key === activeKey
        const icon = on && tab.key === 'workspace' ? 'category-bulk' : tab.icon
        return (
          <button key={tab.key} type="button" aria-current={on ? 'page' : undefined}
            onClick={() => navigate(mobileTabHref(tab.key, route))}
            style={{
              display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 3,
              padding: '8px 0 2px', border: 0, background: 'none',
              color: on ? 'var(--color-fg)' : 'var(--color-fg-4)',
              fontSize: 11, fontWeight: on ? 600 : 400, textDecoration: 'none', touchAction: 'manipulation',
            }}>
            <Icon name={icon} size={21} />
            {tab.label}
          </button>
        )
      })}
    </nav>
  )
}

export function MobileShell({ header, children, footer, tabs, style }: {
  /** header 1 dòng của màn (Phone.dc.html) */
  header?: ReactNode
  children: ReactNode
  /** vùng nút 50px dính đáy */
  footer?: ReactNode
  /** tab đáy 4 mục — route để tô mục hiện tại */
  tabs?: Route
  style?: CSSProperties
}) {
  return (
    <div style={{
      height: 'max(100dvh, var(--m-viewport-bottom, 0px))', minHeight: 'max(100dvh, var(--m-viewport-bottom, 0px))', display: 'flex', flexDirection: 'column',
      background: 'var(--color-bg)', color: 'var(--color-fg)', overflow: 'hidden', ...style,
    }}>
      {header}
      <main className="scroll" style={{ flex: 1, minHeight: 0, padding: '12px 14px', display: 'flex', flexDirection: 'column', gap: 12 }}>
        {children}
      </main>
      {footer && (
        <div style={{ flex: '0 0 auto', padding: '8px 14px 10px', borderTop: '1px solid var(--line-1)', background: 'var(--color-bg)' }}>
          {footer}
        </div>
      )}
      {tabs && <MobileTabBar route={tabs} />}
    </div>
  )
}
