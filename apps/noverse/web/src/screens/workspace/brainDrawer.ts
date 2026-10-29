// Ngăn kéo Não truyện trên Bàn viết (Gói P3 · 21-dev-prompt.md:637-653) — store nhỏ như shell/stores.ts.
// Ai cũng mở được mà không cần luồn prop: tab Truyện (hàng nhân vật, nút Đồ thị), panel “Kế hoạch truyện”
// (chip người), phím G. WorkspaceScreen mount <BrainDrawer> một lần và đọc store này.
import { useEffect, useState } from 'react'

type DrawerState = {
  /** Đang mở hay không. */
  isOpen: boolean
  /** Tên hoặc id thực thể chọn sẵn; null = người kể chuyện / mục đầu. */
  target: string | null
  /** Chương đang viết của Bàn viết (`snapshot.inProgressChapter || currentChapter`); null = Não truyện tự chọn. */
  chapter: number | null
}

let state: DrawerState = { isOpen: false, target: null, chapter: null }
const subs = new Set<() => void>()
const emit = () => subs.forEach((fn) => fn())

/** Mở ngăn kéo, chọn sẵn một nhân vật (tên hoặc id Não truyện) và chương đang viết. */
export function openBrainDrawer(target?: string | null, chapter?: number | null) {
  state = { isOpen: true, target: target?.trim() || null, chapter: chapter && chapter > 0 ? chapter : null }
  emit()
}

export function closeBrainDrawer() {
  if (!state.isOpen) return
  state = { isOpen: false, target: null, chapter: null }
  emit()
}

export function useBrainDrawer(): DrawerState & { open: (target?: string | null) => void; close: () => void } {
  const [current, setCurrent] = useState(state)
  useEffect(() => {
    const update = () => setCurrent(state)
    subs.add(update)
    update()
    return () => { subs.delete(update) }
  }, [])
  return { ...current, open: openBrainDrawer, close: closeBrainDrawer }
}
