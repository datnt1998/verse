// Nhớ vị trí cuộn của từng trang (Gói Q › mục 7 “nhớ vị trí cuộn … khi quay lại”).
//
// Không sửa từng màn: nghe `scroll` ở pha capture và ghi `scrollTop` của từng vùng cuộn
// (`.scroll`) theo khoá route hiện tại + chỉ số vùng. Khi quay lại, khôi phục đúng chỉ số đó.
// Khoá do AppShell đặt (`setScrollRouteKey`) vì chỉ nó biết route đã đổi.
const positions = new Map<string, number[]>()
let currentKey = ''
let listening = false

/** Khoá route hiện tại — `AppShell` gọi mỗi khi route đổi. */
export function setScrollRouteKey(key: string) {
  currentKey = key
}

const scrollRegions = (): HTMLElement[] =>
  Array.from(document.querySelectorAll<HTMLElement>('.scroll'))

function remember(target: EventTarget | null) {
  if (!currentKey || !(target instanceof HTMLElement)) return
  const index = scrollRegions().indexOf(target)
  if (index < 0) return
  const list = positions.get(currentKey) ?? []
  list[index] = target.scrollTop
  positions.set(currentKey, list)
}

/** Gắn bộ nghe một lần (AppShell). Trả hàm gỡ. */
export function startScrollMemory() {
  if (listening) return () => undefined
  listening = true
  const onScroll = (event: Event) => remember(event.target)
  document.addEventListener('scroll', onScroll, true)
  return () => {
    document.removeEventListener('scroll', onScroll, true)
    listening = false
  }
}

/**
 * Khôi phục vị trí đã nhớ cho khoá hiện tại. Gọi vài lần sau khi route đổi: vùng cuộn của màn
 * mới có thể chỉ xuất hiện sau khi dữ liệu về (khung tải → nội dung).
 */
export function restoreScroll(key = currentKey) {
  const list = positions.get(key)
  if (!list?.length) return
  for (const [index, el] of scrollRegions().entries()) {
    const top = list[index]
    if (typeof top === 'number' && Math.abs(el.scrollTop - top) > 1) el.scrollTop = top
  }
}
