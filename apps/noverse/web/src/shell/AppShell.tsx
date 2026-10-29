// AppShell — khung app: height 100vh, overflow hidden, header + body + mobile tab bar.
// <1100px: height auto, body xếp chồng (xem 03-tokens.md breakpoint).
// <760px: header thu gọn + tab đáy 4 mục (Phone.dc.html › home).
import { useEffect, useRef, useState, type ReactNode } from 'react'
import { navigate, paths, setDialog, useRoute, type Route } from '../router'
import { rememberHelpTarget } from '../commands/target'
import { AppHeader } from './AppHeader'
import { LockedDialog } from './ConnectionBanner'
import { MobileTabBar } from './MobileShell'
import { setLocked, useConnection, usePanels } from './stores'
import { JobsDock } from './jobs/JobsDock'
import { useDocumentTitle } from './useNames'
import { endNav, useNavPhase } from './navTransition'
import { restoreScroll, setScrollRouteKey, startScrollMemory } from './scrollMemory'
import { RouteStage } from './RouteStage'


/** Khoá nhớ vị trí cuộn: tên route + phần định danh nội dung (dự án/truyện/chương/tập). */
function routeKeyOf(route: Route): string {
  const base = `${route.name}:${'project' in route && route.project ? route.project : ''}:${'story' in route ? route.story : ''}`
  switch (route.name) {
    case 'reader': return `${base}:${route.chapter ?? ''}`
    case 'audio': return `${base}:${route.volume ?? ''}:${route.mode}`
    case 'settings': return `${base}:${route.section ?? ''}`
    case 'help': return `${base}:${route.cmd ?? ''}`
    default: return base
  }
}

export function useMediaQuery(query: string): boolean {
  const [match, setMatch] = useState(() => typeof matchMedia !== 'undefined' && matchMedia(query).matches)
  useEffect(() => {
    const mql = matchMedia(query)
    const update = () => setMatch(mql.matches)
    update()
    mql.addEventListener('change', update)
    return () => mql.removeEventListener('change', update)
  }, [query])
  return match
}

/** Phím [ ] ẩn/hiện panel — chỉ trong Workspace; screens gọi usePanels().toggle */
function usePanelKeys(active: boolean) {
  const panels = usePanels()
  useEffect(() => {
    if (!active) return
    const onKey = (event: KeyboardEvent) => {
      const target = event.target as HTMLElement | null
      if (target && ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName)) return
      if (event.key === '[') panels.toggle('left')
      if (event.key === ']') panels.toggle('right')
    }
    addEventListener('keydown', onKey)
    return () => removeEventListener('keydown', onKey)
  }, [active, panels])
}

export function AppShell({ children }: { children: ReactNode }) {
  const { route, dialog } = useRoute()
  const mobile = useMediaQuery('(max-width: 759px)')
  const narrow = useMediaQuery('(max-width: 1099px)')
  const conn = useConnection()
  const phase = useNavPhase()
  const locked = conn.locked || dialog === 'locked'
  usePanelKeys(route.name === 'workspace')
  useDocumentTitle(route)

  // Khoá nhớ vị trí cuộn: route + chương/tập/tiết diện để mỗi trang (và mỗi chương) có chỗ riêng.
  const scrollKey = routeKeyOf(route)
  // Chapter/volume changes update the existing screen; each main tab has a stable Activity.
  const stageKey = `${route.name}:${'project' in route ? route.project ?? '' : ''}:${'story' in route ? route.story : ''}`

  // Chỉ báo trượt/vạch tiến độ kết thúc khi trang mới đã vẽ nội dung (hoặc hết 6 s ở store).
  useEffect(() => {
    if (!phase.pending) return
    let raf = 0
    let frames = 0
    const tick = () => {
      frames++
      const main = document.querySelector('main')
      const text = main?.textContent?.trim() ?? ''
      if ((location.hash !== phase.from && text.length >= 40) || frames > 400) { endNav(); return }
      raf = requestAnimationFrame(tick)
    }
    raf = requestAnimationFrame(tick)
    return () => cancelAnimationFrame(raf)
  }, [phase.pending, phase.from, scrollKey])

  // Vị trí cuộn: gắn bộ nghe một lần; mỗi lần route đổi thì nhớ khoá mới rồi khôi phục dần
  // (vùng cuộn của màn mới có thể chỉ có sau khi dữ liệu về).
  useEffect(() => startScrollMemory(), [])
  useEffect(() => {
    setScrollRouteKey(scrollKey)
    const frames = [requestAnimationFrame(() => restoreScroll(scrollKey))]
    const later = setTimeout(() => restoreScroll(scrollKey), 260)
    return () => { frames.forEach(cancelAnimationFrame); clearTimeout(later) }
  }, [scrollKey])

  // ⌘K / Ctrl+K: bảng lệnh toàn cục (02-ia-flows F8); Esc do Dialog đóng
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
        event.preventDefault()
        setDialog('palette')
      }
    }
    addEventListener('keydown', onKey)
    return () => removeEventListener('keydown', onKey)
  }, [])

  // Nhớ truyện đang mở để trang Hướng dẫn lệnh quay lại đúng chỗ (13-commands.md §2).
  useEffect(() => { rememberHelpTarget() }, [route])

  // ? — mở Hướng dẫn lệnh từ mọi màn hình, trừ khi đang gõ trong ô nhập.
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key !== '?' || event.metaKey || event.ctrlKey || event.altKey) return
      const target = event.target as HTMLElement | null
      if (target && (target.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName))) return
      if (route.name === 'help') return
      event.preventDefault()
      rememberHelpTarget()
      navigate(paths.help())
    }
    addEventListener('keydown', onKey)
    return () => removeEventListener('keydown', onKey)
  }, [route.name])

  return (
    <div style={{
      height: narrow ? 'auto' : '100vh', minHeight: '100vh',
      overflow: narrow ? 'visible' : 'hidden',
      display: 'flex', flexDirection: 'column',
      background: 'var(--color-bg)', color: 'var(--color-fg)',
    }}>
      <div style={{ position: 'relative', flex: '0 0 auto' }}>
        <AppHeader route={route} mobile={mobile} />
        {/* Gói Q: vạch tiến độ 2 px chỉ hiện khi trang mới quá 300 ms chưa có nội dung. */}
        {phase.slow && !mobile && <div className="navprog" aria-hidden="true"><i /></div>}
      </div>
      <RouteStage routeKey={stageKey}>{children}</RouteStage>
      {mobile && <MobileTabBar route={route} />}
      {!mobile && <JobsDock />}
      {locked && (
        <LockedDialog
          onReadOnly={() => { setLocked(false); if (dialog === 'locked') setDialog(null) }}
          onRetry={() => { setLocked(false); if (dialog === 'locked') setDialog(null); conn.retry() }}
        />
      )}
    </div>
  )
}
