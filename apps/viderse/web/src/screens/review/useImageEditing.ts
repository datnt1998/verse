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
  /** Bỏ một ảnh đang hiện: ảnh vừa thêm thì huỷ lần thêm, ảnh bản nguồn thì ghi đã bỏ. */
  toggleRef: (id: string) => void
  addRef: (ref: ImageRef) => void
  /** Số chỗ đã sửa (phần chữ + ảnh bỏ + ảnh thêm). */
  changedCount: number
  reset: () => void
}

/** Chưa đổi ảnh tham chiếu nào: dùng lại một đối tượng rỗng. */
const NO_REF_EDITS: { removed: string[]; added: ImageRef[] } = { removed: [], added: [] }

export function useImageEditing(imageId: string | undefined, base: ImageVersion | undefined): ImageEditing {
  const [mode, setModeState] = useState<'view' | 'raw' | 'edit'>('view')
  const [edits, setEdits] = useState<Partial<Record<PromptPartKind, string>>>({})
  const [refEdits, setRefEdits] = useState(NO_REF_EDITS)
  const [source, setSource] = useState<ImageVersion | undefined>(base)
  const [sourceImageId, setSourceImageId] = useState(imageId)
  const [sourceKey, setSourceKey] = useState(`${imageId ?? ''}:${base?.n ?? ''}`)
  const key = `${imageId ?? ''}:${base?.n ?? ''}`
  const dirty = Object.keys(edits).length > 0 || refEdits.removed.length > 0 || refEdits.added.length > 0
  // Đổi ảnh (kể cả đang sửa) hoặc đổi bản khi đang xem → bỏ mọi sửa chưa gửi.
  // Bản đang xem đổi do SSE trong lúc sửa/raw có nháp thì GIỮ nguyên bản nguồn.
  if (key !== sourceKey && (imageId !== sourceImageId || mode === 'view')) {
    setSourceKey(key)
    setSourceImageId(imageId)
    setSource(base)
    setEdits({})
    setRefEdits(NO_REF_EDITS)
    setModeState('view')
  }
  const rememberSource = useCallback(() => {
    setSource(base)
    setSourceKey(key)
    setSourceImageId(imageId)
  }, [base, imageId, key])
  const clear = useCallback(() => { setEdits({}); setRefEdits(NO_REF_EDITS) }, [])
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
    () => [...(active?.refs ?? []).filter((ref) => !refEdits.removed.includes(ref.id)), ...refEdits.added],
    [active, refEdits],
  )
  const toggleRef = useCallback((id: string) => {
    enterEdit()
    setRefEdits((current) => current.added.some((ref) => ref.id === id)
      // Ảnh vừa thêm: bỏ nó là huỷ lần thêm, không ghi vào danh sách ảnh bản nguồn đã bỏ.
      ? { ...current, added: current.added.filter((ref) => ref.id !== id) }
      : {
          ...current,
          removed: current.removed.includes(id) ? current.removed.filter((entry) => entry !== id) : [...current.removed, id],
        })
  }, [enterEdit])
  const addRef = useCallback((ref: ImageRef) => {
    enterEdit()
    // Cùng một ảnh (cùng `id` của ứng viên) chỉ có một mục: thêm lại thì thay, không nhân đôi.
    setRefEdits((current) => ({
      removed: current.removed.filter((id) => id !== ref.id),
      added: current.added.some((entry) => entry.id === ref.id)
        ? current.added.map((entry) => (entry.id === ref.id ? ref : entry))
        : [...current.added, ref],
    }))
  }, [enterEdit])
  const changed = useMemo(() => changedParts(active, edits), [active, edits])
  const reset = useCallback(() => { clear(); rememberSource(); setModeState('view') }, [clear, rememberSource])
  return {
    mode, setMode, edits, editText, resetPart, changed, base: active, refs, toggleRef, addRef,
    changedCount: Object.keys(changed).length + refEdits.removed.length + refEdits.added.length,
    reset,
  }
}
