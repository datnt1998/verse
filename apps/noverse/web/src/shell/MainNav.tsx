// MainNav — 5 link segmented ở header (Nav.dc.html, Main.dc.html):
// Bàn viết · Dự án · Trình đọc · Audio · Truyện mới.
//
// Gói Q:
// - Mọi mục **đậm 500**; mục đang chọn chỉ đổi màu chữ (`fg`) và icon (`accent`) nên bề rộng
//   từng mục không đổi → các mục bên cạnh không xê dịch.
// - Một `span` nền duy nhất trượt bằng `transform` + `width` (280 ms). Đo bằng `offsetLeft` /
//   `offsetWidth`; đo lại khi đổi cỡ cửa sổ, khi font tải xong và khi nhãn đổi.
// - Chỉ báo trượt **ngay khi bấm** (`beginNav`), không chờ dữ liệu trang mới.
// - Rê chuột / focus → nạp trước dữ liệu màn đích (`prefetchScreen`).
// - Vòng focus chỉ dành cho bàn phím: chế độ nhập được ghi vào `data-input` để CSS bỏ vòng
//   khi vừa bấm chuột (ảnh chụp 27/10 cho thấy Chromium vẫn vẽ `:focus-visible` sau cú bấm).
import { useCallback, useEffect, useLayoutEffect, useRef, useState, type CSSProperties } from 'react'
import { prefetchScreen } from '../data/prefetch'
import { t } from '../i18n/vi'
import { paths, type Route } from '../router'
import { hasIcon, Icon } from '@noverse/ui'
import { beginNav, useNavPhase } from './navTransition'

export type NavKey = 'workspace' | 'project' | 'reader' | 'audio' | 'newstory'

export function navKeyOf(route: Route): NavKey | null {
  switch (route.name) {
    case 'workspace': case 'activity': return 'workspace'
    case 'project': case 'story': case 'projbrain': case 'quality': return 'project'
    case 'reader': return 'reader'
    case 'audio': return 'audio'
    case 'newstory': return 'newstory'
    default: return null
  }
}

/** Bề rộng cố định theo nhãn: đậm 500 ở mọi trạng thái nên không giãn khi chọn. */
const itemStyle = (on: boolean): CSSProperties => ({
  position: 'relative', zIndex: 1,
  height: 30, padding: '0 12px 0 10px', display: 'flex', alignItems: 'center', gap: 8,
  borderRadius: 7, background: 'transparent',
  color: on ? 'var(--color-fg)' : 'var(--color-fg-3)', fontSize: 13, fontWeight: 500,
  whiteSpace: 'nowrap', transition: 'color 160ms ease',
})

/** `:focus` không vẽ vòng; chỉ `:focus-visible` khi vừa dùng bàn phím (xem `.mainnav` trong base.css). */
const linkProps = { className: 'mainnav-item' } as const

type Item = { key: NavKey; label: string; icon: string; href: string | null; needsStory: boolean }

export function MainNav({ route, project, story }: { route: Route; project: string | null; story: string | null }) {
  const current = navKeyOf(route)
  const phase = useNavPhase()
  // Chỉ báo bám mục vừa bấm cho tới khi trang mới vẽ xong nội dung.
  const target = phase.pending ?? current
  const hasStory = Boolean(project && story)
  const items: Item[] = [
    { key: 'workspace', label: t.nav.workspace, icon: 'category', href: hasStory ? paths.workspace(project!, story!) : null, needsStory: true },
    // No project yet (e.g. Cài đặt before any story was opened): `#/` opens the first project (screens/project).
    { key: 'project', label: t.nav.project, icon: 'folder-open', href: project ? paths.project(project) : '/', needsStory: false },
    { key: 'reader', label: t.nav.reader, icon: 'book-1', href: hasStory ? paths.reader(project!, story!) : null, needsStory: true },
    { key: 'audio', label: t.nav.audio, icon: 'headphone', href: hasStory ? paths.audio(project!, story!) : null, needsStory: true },
    { key: 'newstory', label: t.nav.newStory, icon: 'add-square', href: project ? paths.newstory(project) : null, needsStory: false },
  ]

  const nav = useRef<HTMLElement | null>(null)
  const refs = useRef(new Map<NavKey, HTMLElement>())
  // Ý định rê chuột: chỉ nạp trước khi con trỏ đã đậu ~80 ms, để cú bấm ngang qua không bắn
  // cả loạt request rồi bỏ đi (nhất là Audio/Trình đọc).
  const hover = useRef<NodeJS.Timeout | undefined>(undefined)
  useEffect(() => () => clearTimeout(hover.current), [])
  const [box, setBox] = useState<{ x: number; w: number } | null>(null)

  /** Đo mục đang chọn: `offsetLeft` của mục = khoảng từ mép padding của `nav` (đúng gốc `left:0`). */
  const measure = useCallback(() => {
    const el = target ? refs.current.get(target) : null
    if (!el) { setBox(null); return }
    const next = { x: el.offsetLeft, w: el.offsetWidth }
    setBox((prev) => (prev && prev.x === next.x && prev.w === next.w ? prev : next))
  }, [target])

  useLayoutEffect(() => { measure() }, [measure, items.map((item) => item.label).join('|')])

  // Đo lại khi cỡ cửa sổ đổi, khi font tải xong (bề rộng nhãn đổi) và khi thanh đổi bề rộng.
  useEffect(() => {
    const onResize = () => measure()
    addEventListener('resize', onResize)
    const ro = typeof ResizeObserver !== 'undefined' ? new ResizeObserver(onResize) : null
    if (ro && nav.current) ro.observe(nav.current)
    void document.fonts?.ready.then(onResize).catch(() => undefined)
    return () => {
      removeEventListener('resize', onResize)
      ro?.disconnect()
    }
  }, [measure])

  // Chế độ nhập: bấm chuột → không vòng focus; Tab/Enter → có vòng focus.
  useEffect(() => {
    const el = nav.current
    if (!el) return
    const pointer = () => el.setAttribute('data-input', 'pointer')
    const keyboard = () => el.setAttribute('data-input', 'keyboard')
    el.addEventListener('pointerdown', pointer, true)
    addEventListener('keydown', keyboard, true)
    return () => {
      el.removeEventListener('pointerdown', pointer, true)
      removeEventListener('keydown', keyboard, true)
    }
  }, [])

  /** Bấm mục đã ở đúng trang (hoặc mở tab mới bằng ⌘/Ctrl) → không chạy chỉ báo/tiến độ. */
  const onNavClick = (event: React.MouseEvent, key: NavKey, href: string) => {
    clearTimeout(hover.current)
    if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey || event.button !== 0) return
    const target = `#${href}`
    if (location.hash === target || location.hash.startsWith(`${target}?`)) return
    beginNav(key)
  }

  return (
    <nav ref={nav} aria-label="Điều hướng chính" className="mainnav" style={{
      position: 'relative', display: 'flex', gap: 2, padding: 3, borderRadius: 10,
      background: 'var(--color-surface-1)', border: '1px solid var(--line-1)',
    }}>
      {box && target && (
        <span aria-hidden="true" className="mainnav-ind" style={{
          position: 'absolute', zIndex: 0, top: 3, left: 0, height: 30, borderRadius: 7,
          background: 'var(--color-surface-6)',
          width: box.w, transform: `translateX(${box.x}px)`,
        }} />
      )}
      {items.map((item) => {
        const on = item.key === target
        const icon = on && hasIcon(`${item.icon}-bulk`) ? `${item.icon}-bulk` : item.icon
        if (!item.href) {
          return (
            <span key={item.key} aria-disabled="true" title={t.nav.needStory} ref={(el) => {
              if (el) refs.current.set(item.key, el); else refs.current.delete(item.key)
            }} style={{ ...itemStyle(false), color: 'var(--color-fg-5)', cursor: 'default' }}>
              <Icon name={item.icon} size={16} />{item.label}
            </span>
          )
        }
        return (
          <a key={item.key} href={`#${item.href}`} {...linkProps} aria-current={on ? 'page' : undefined}
            ref={(el) => {
              if (el) refs.current.set(item.key, el); else refs.current.delete(item.key)
            }}
            onPointerEnter={() => {
              clearTimeout(hover.current)
              hover.current = setTimeout(() => prefetchScreen(item.key, project, story), 80)
            }}
            onPointerLeave={() => clearTimeout(hover.current)}
            onFocus={() => prefetchScreen(item.key, project, story)}
            onClick={(event) => onNavClick(event, item.key, item.href!)}
            style={itemStyle(on)}>
            <Icon name={icon} size={16} style={{ color: on ? 'var(--color-accent)' : undefined }} />
            {item.label}
          </a>
        )
      })}
    </nav>
  )
}
