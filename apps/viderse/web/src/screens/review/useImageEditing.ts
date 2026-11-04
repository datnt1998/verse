// Trạng thái sửa prompt của đúng một ảnh: chữ từng phần, ảnh tham chiếu bỏ/thêm,
// và phần đã đổi so với bản nguồn — đúng thứ gửi lên POST …/redraw.
import { useCallback, useMemo, useState } from 'react'
import type { ImageRef, ImageVersion, PromptPartKind } from '../../api/images'
import { changedParts } from './imageView'

export interface ImageEditing {
  mode: 'view' | 'raw' | 'edit'
  setMode: (mode: 'view' | 'raw' | 'edit') => void
  edits: Partial<Record<PromptPartKind, string>>
  editText: (kind: PromptPartKind, text: string) => void
  resetPart: (kind: PromptPartKind) => void
  /** Phần đã khác bản nguồn của bản nháp. */
  changed: Partial<Record<PromptPartKind, string>>
  /**
   * Bản nguồn của bản nháp, chốt lúc bắt đầu sửa. SSE đổi bản đang xem trong
   * lúc sửa không được đổi nguồn này; người dùng đổi ảnh/bản thì mới đổi.
   */
  base: ImageVersion | undefined
  /** Ảnh tham chiếu sẽ gửi: bản nguồn trừ ảnh đã bỏ, cộng ảnh vừa thêm. */
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
  const [source, setSource] = useState<ImageVersion | undefined>(base)
  const [sourceImageId, setSourceImageId] = useState(imageId)
  const [sourceKey, setSourceKey] = useState(`${imageId ?? ''}:${base?.n ?? ''}`)
  const key = `${imageId ?? ''}:${base?.n ?? ''}`
  const dirty = Object.keys(edits).length > 0 || removed.length > 0 || added.length > 0
  // Đổi ảnh (kể cả đang sửa) hoặc đổi bản khi đang xem → bỏ mọi sửa chưa gửi.
  // Bản đang xem đổi do SSE trong lúc sửa/raw có nháp thì GIỮ nguyên bản nguồn.
  if (key !== sourceKey && (imageId !== sourceImageId || mode === 'view')) {
    setSourceKey(key)
    setSourceImageId(imageId)
    setSource(base)
    setEdits({})
    setRemoved([])
    setAdded([])
    setModeState('view')
  }
  const rememberSource = useCallback(() => {
    setSource(base)
    setSourceKey(key)
    setSourceImageId(imageId)
  }, [base, imageId, key])
  const clear = useCallback(() => { setEdits({}); setRemoved([]); setAdded([]) }, [])
  const setMode = useCallback((next: 'view' | 'raw' | 'edit') => {
    if (next === 'view') { clear(); rememberSource(); setModeState('view'); return }
    // Bắt đầu sửa từ trạng thái chưa có nháp: chốt đúng bản đang xem lúc này.
    if (next === 'edit' && mode !== 'edit' && !dirty) rememberSource()
    setModeState(next)
  }, [clear, dirty, mode, rememberSource])
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
  const enterEdit = useCallback(() => {
    if (!dirty) rememberSource()
    setModeState('edit')
  }, [dirty, rememberSource])
  // Raw chỉ đọc không phải bản nguồn; raw còn nháp (đang sửa dở) thì vẫn giữ bản nguồn.
  const active = mode === 'edit' || (mode === 'raw' && dirty) ? source : base
  const refs = useMemo(
    () => [...(active?.refs ?? []).filter((ref) => !removed.includes(ref.id)), ...added],
    [active, removed, added],
  )
  const toggleRef = useCallback((id: string) => {
    enterEdit()
    setRemoved((old) => (old.includes(id) ? old.filter((entry) => entry !== id) : [...old, id]))
  }, [enterEdit])
  const addRef = useCallback((ref: ImageRef) => { enterEdit(); setAdded((old) => [...old, ref]) }, [enterEdit])
  const changed = useMemo(() => changedParts(active, edits), [active, edits])
  const reset = useCallback(() => { clear(); rememberSource(); setModeState('view') }, [clear, rememberSource])
  return {
    mode, setMode, edits, editText, resetPart, changed, base: active, refs, toggleRef, addRef,
    changedCount: Object.keys(changed).length + removed.length + added.length,
    reset,
  }
}
