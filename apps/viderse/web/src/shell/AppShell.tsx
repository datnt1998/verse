// AppShell — khung desktop: header (brand · nav · menu tài khoản) + nội dung cuộn.
// <760px App chuyển sang MobileShell (cùng ranh giới của Noverse, 03-tokens.md breakpoint).
import { useEffect, useState, type ReactNode } from 'react'
import { useRoute } from '../router'
import { AppHeader } from './AppHeader'

export function useMediaQuery(query: string): boolean {
  const [match, setMatch] = useState(() => typeof matchMedia !== 'undefined' && matchMedia(query).matches)
  useEffect(() => {
    const list = matchMedia(query)
    const change = () => setMatch(list.matches)
    change()
    list.addEventListener('change', change)
    return () => list.removeEventListener('change', change)
  }, [query])
  return match
}

export function AppShell({ children }: { children: ReactNode }) {
  const { route } = useRoute()
  return (
    <div style={{
      height: '100vh', display: 'flex', flexDirection: 'column', overflow: 'hidden',
      background: 'var(--color-bg)', color: 'var(--color-fg)',
    }}>
      <AppHeader route={route} />
      <main className="scroll" style={{ flex: 1, minHeight: 0 }}>
        {children}
      </main>
    </div>
  )
}
