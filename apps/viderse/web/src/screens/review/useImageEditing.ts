// Trạng thái sửa prompt của đúng một ảnh: chữ từng phần, ảnh tham chiếu bỏ/thêm,
// và phần đã đổi so với bản đang dùng — đúng thứ gửi lên POST …/redraw.
import { useCallback, useMemo, useState } from 'react'
import type { ImageRef, ImageVersion, PromptPartKind } from '../../api/images'
import { changedParts } from './imageView'

export interface ImageEditing {
  mode: 'view' | 'raw' | 'edit'
  setMode: (mode: 'view' | 'raw' | 'edit') => void
  edits: Partial<Record<PromptPartKind, string>>
  editText: (kind: PromptPartKind, text: string) => void
  resetPart: (kind: PromptPartKind) => void
  /** Phần đã khác bản đang dùng. */
  changed: Partial<Record<PromptPartKind, string>>
  /** Ảnh tham chiếu sẽ gửi: bản đang dùng trừ ảnh đã bỏ, cộng ảnh vừa thêm. */
  refs: ImageRef[]
  toggleRef: (id: string) => void
  addRef: (ref: ImageRef) => void
  /** Số chỗ đã sửa (phần chữ + ảnh bỏ + ảnh thêm). */
  changedCount: number
  reset: () => void
}

export function useImageEditing(imageId: string | undefined, base: ImageVersion | undefined): ImageEditing {
  const [mode, setModeState] = useState<'view' | 'raw' | 'edit'>('view')
  const [edits, setEdits] = useState<Partial<Record<PromptPartKind, string>>>({})
  const [removed, setRemoved] = useState<string[]>([])
  const [added, setAdded] = useState<ImageRef[]>([])
  const [baseKey, setBaseKey] = useState(`${imageId ?? ''}:${base?.n ?? ''}`)
  // Đổi ảnh hoặc đổi bản đang dùng → bỏ mọi sửa chưa gửi.
  const key = `${imageId ?? ''}:${base?.n ?? ''}`
  if (key !== baseKey) {
    setBaseKey(key)
    setEdits({})
    setRemoved([])
    setAdded([])
    setModeState('view')
  }
  const setMode = useCallback((next: 'view' | 'raw' | 'edit') => {
    setModeState(next)
    if (next === 'view') { setEdits({}); setRemoved([]); setAdded([]) }
  }, [])
  const editText = useCallback((kind: PromptPartKind, text: string) => {
    setEdits((old) => ({ ...old, [kind]: text }))
  }, [])
  const resetPart = useCallback((kind: PromptPartKind) => {
    setEdits((old) => {
      const next = { ...old }
      delete next[kind]
      return next
    })
  }, [])
  const refs = useMemo(
    () => [...(base?.refs ?? []).filter((ref) => !removed.includes(ref.id)), ...added],
    [base, removed, added],
  )
  const toggleRef = useCallback((id: string) => {
    setRemoved((old) => (old.includes(id) ? old.filter((entry) => entry !== id) : [...old, id]))
  }, [])
  const addRef = useCallback((ref: ImageRef) => { setAdded((old) => [...old, ref]) }, [])
  const changed = useMemo(() => changedParts(base, edits), [base, edits])
  const reset = useCallback(() => { setEdits({}); setRemoved([]); setAdded([]); setModeState('view') }, [])
  return {
    mode, setMode, edits, editText, resetPart, changed, refs, toggleRef, addRef,
    changedCount: Object.keys(changed).length + removed.length + added.length,
    reset,
  }
}
