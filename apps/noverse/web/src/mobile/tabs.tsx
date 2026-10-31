import { useEffect, useRef, type CSSProperties, type ReactNode } from 'react'
import { navigateNow, paths, type Parsed } from '../router'
import { Icon } from '@noverse/ui'
import { KeepAlive } from '../shell/KeepAlive'
import type { TabMode } from './logic'
import { MobilePaneScope } from './runtime'

export type TabKey = 'work' | 'read' | 'audio' | 'more'
/** `disabled`: tab chưa có đích (chưa chọn truyện) — hiện mờ, không bấm được. */
export type TabItem = { key: TabKey; label: string; icon: string; href: string; dot?: boolean; disabled?: boolean }

/** Pane giữ-alive của một tab: route + truyện lúc pane được mở lần cuối. */
export type PaneSnapshot = {
  parsed: Parsed
  project: string | null
  story: string | null
  /** dấu nội dung — giữ nguyên để `memo` của PaneView không bị phá mỗi lần router parse lại */
  signature: string
}

/**
 * Tab pane của một route; `null` = màn riêng (Hoạt động, Não, Thông báo, ghi chú máy tính…)
 * phủ lên trên nhưng vẫn để các pane đã mở nằm nguyên bên dưới.
 *
 * Mọi route khác đều thuộc một pane — kể cả route chỉ có trên máy tính — vì thanh tab luôn hiện
 * dưới màn hình điện thoại: trả `null` cho chúng sẽ lộ màn “Trên máy tính” ngay dưới tab vừa bấm.
 */
export function paneKeyOf(parsed: Parsed): string | null {
  const { route, search } = parsed
  switch (route.name) {
    case 'workspace': return 'work'
    case 'story': return search.has('tab') ? null : 'work'
    case 'reader': return 'read'
    // Mọi chế độ nghe (review/make/listen) đều dựng bằng trình nghe điện thoại.
    case 'audio': return 'audio'
    case 'project': return 'more:projects'
    case 'help': return `more:help:${route.cmd ?? ''}`
    case 'activity':
    case 'brain':
    case 'projbrain':
    case 'notifications': return null
    case 'settings': {
      if (route.section === 'notify') return null
      const page = search.get('page') ?? ''
      return route.section === 'rules' ? `more:rules:${page}` : route.section === 'storage' ? 'more:storage' : `more:mobile:${page}`
    }
    // Route chỉ có ở bản máy tính (Chất lượng, Não dự án, Truyện mới, Não hệ thống, 404…):
    // vẫn thuộc tab Thêm; `phoneRedirect` thay URL ngay sau khung hình đầu.
    default: return 'more:mobile:'
  }
}

const PHONE_MORE = '/settings/mobile'

/**
 * Route chỉ tồn tại ở bản máy tính → đường dẫn thay thế trên điện thoại (`replace`, không thêm lịch sử).
 * Nhờ vậy không màn nào mở từ thanh tab rơi vào màn “Trên máy tính”.
 */
export function phoneRedirect(parsed: Parsed): string | null {
  const { route } = parsed
  switch (route.name) {
    case 'settings': {
      const section = route.section
      return section === 'mobile' || section === 'rules' || section === 'notify' || section === 'storage' ? null : PHONE_MORE
    }
    // Chất lượng truyện có nghĩa nhất khi quay về màn Viết của chính truyện đó.
    case 'quality': return paths.workspace(route.project, route.story)
    case 'newstory':
    case 'sysbrain':
    case 'missing': return PHONE_MORE
    default: return null
  }
}

/**
 * Dấu nhận dạng nội dung của một ảnh chụp pane. `Parsed` là object mới sau mỗi `hashchange`,
 * nên so sánh tham chiếu sẽ luôn “đổi” và kéo mọi pane render lại; so dấu này thì không.
 */
export function paneSignature(parsed: Parsed, project: string | null, story: string | null): string {
  return JSON.stringify([parsed.route, [...parsed.search], parsed.dialog, project, story])
}

/** Cập nhật ảnh chụp pane, giữ nguyên object cũ khi nội dung không đổi (ổn định `memo` của PaneView). */
export function keepPane(panes: Map<string, PaneSnapshot>, id: string, parsed: Parsed, project: string | null, story: string | null): PaneSnapshot {
  const signature = paneSignature(parsed, project, story)
  const previous = panes.get(id)
  if (previous && previous.signature === signature) return previous
  const next: PaneSnapshot = { parsed, project, story, signature }
  panes.set(id, next)
  return next
}

/** Trang đầu của pane “Thêm” theo route lúc mở (giữ nguyên hành vi cũ của MMore). */
export function moreStart(view: Parsed): { initialPage?: 'home' | 'projects' | 'model' | 'devices' | 'password' | 'guide' | 'storyset' | 'rules' | 'storage'; initialCommand?: string } {
  const { route, search } = view
  const page = search.get('page')
  const initialPage = route.name === 'help' ? 'guide' : route.name === 'settings' && route.section === 'storage' ? 'storage' : route.name === 'settings' && route.section === 'rules' ? 'rules' : page === 'projects' || page === 'devices' || page === 'model' || page === 'password' || page === 'guide' || page === 'storyset' || page === 'rules' ? page : route.name === 'project' ? 'projects' : undefined
  return { initialPage, initialCommand: route.name === 'help' ? route.cmd ?? undefined : search.get('cmd') ?? undefined }
}

/** Sau khi đổi tab, chỉnh vạch chỉ báo lạc quan trở lại tab đang thật sự hiển thị nếu điều hướng bị chặn. */
const RELEASE_MS = 800

/**
 * Thanh tab dưới: `button` + `navigate()`, không `<a href="#…">` — trên iOS link hash bắn
 * cú chạm/điều hướng mặc định trùng với router nên nội dung bị nháy như tải lại trang.
 * `touch-action: manipulation` (CSS) bỏ trễ chạm; `aria-current` giữ điều hướng bằng bàn phím.
 */
export function MobileTabBar({ tabs, index, mode, dot, onSelect = navigateNow }: {
  tabs: TabItem[]
  index: number
  mode: TabMode
  dot: boolean
  onSelect?: (href: string) => void
}) {
  const ref = useRef<HTMLElement>(null)
  const release = useRef<number | undefined>(undefined)
  const committed = useRef(index)
  committed.current = index
  const hidden = mode === 'hide'
  useEffect(() => () => window.clearTimeout(release.current), [])

  const select = (item: TabItem, position: number) => {
    if (hidden || item.disabled) return
    // Vạch chỉ báo lạc quan theo cú chạm ngay; React vẫn giữ tab thật cho tới khi route đổi.
    if (committed.current !== position) ref.current?.style.setProperty('--tab-index', String(position))
    onSelect(item.href)
    window.clearTimeout(release.current)
    release.current = window.setTimeout(() => {
      if (ref.current && committed.current !== position) ref.current.style.setProperty('--tab-index', String(committed.current))
    }, RELEASE_MS)
  }

  return <nav ref={ref} className={`m-tabs ${mode}`} aria-label="Điều hướng chính" aria-hidden={hidden} style={{ '--tab-index': index } as CSSProperties}>
    <span className="tind" aria-hidden="true" />
    {tabs.map((item, position) => <button
      key={item.key}
      type="button"
      className="m-tab"
      aria-current={index === position ? 'page' : undefined}
      disabled={item.disabled}
      tabIndex={hidden ? -1 : 0}
      onClick={() => select(item, position)}
    >
      <Icon className="icon" name={`${item.icon}${index === position ? '-bulk' : ''}`} size={24} />
      <span className="m-tab-label">{item.label}</span>
      {item.key === 'read' && dot && index !== position && <span className="m-dot" aria-label="Có việc cần bạn" />}
    </button>)}
  </nav>
}

/**
 * Các pane của tab: pane đang mở hiện, pane đã mở khác giữ trong `<Activity mode="hidden">`
 * — state, DOM và vị trí cuộn còn nguyên, effect (SSE, poll, timer) tự dừng khi ẩn.
 */
export function MobilePanes({ panes, active }: { panes: [string, ReactNode][]; active: string | null }) {
  return <>{panes.map(([id, node]) => <KeepAlive key={id} active={id === active}>
    <div data-pane={id} className={`m-pane${id === active ? ' on' : ''}`} aria-hidden={id !== active}>
      <MobilePaneScope active={id === active}>{node}</MobilePaneScope>
    </div>
  </KeepAlive>)}</>
}
