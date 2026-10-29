// AppHeader — thanh trên cùng desktop: BrandLockup · MainNav (trái) · AccountMenu (phải).
import { BrandLockup } from '@noverse/ui'
import type { Route } from '../router'
import { AccountMenu } from './AccountMenu'
import { MainNav } from './MainNav'

export function AppHeader({ route }: { route: Route }) {
  return (
    <header style={{
      flex: '0 0 auto', display: 'flex', alignItems: 'center', gap: 16,
      height: 52, padding: '0 16px',
      borderBottom: '1px solid var(--line-1)', background: 'var(--color-bg)',
    }}>
      <a href="#/" aria-label="Vidverse" style={{ display: 'flex', alignItems: 'center', color: 'var(--color-fg)' }}>
        <BrandLockup size={22} studio />
      </a>
      <div style={{ flex: 1, display: 'flex', justifyContent: 'center' }}>
        <MainNav route={route} />
      </div>
      <AccountMenu />
    </header>
  )
}
