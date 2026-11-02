// Lưu tự động của trang Cài đặt template (W12): ô chữ lưu khi rời ô, điều
// khiển rời rạc (radio, công tắc, stepper, select) lưu ngay khi đổi. Một lần
// lưu đang chạy thì lần sau chỉ giữ bản mới nhất (gộp) — mỗi PATCH luôn mang
// `revision` mới nhất nên hai người sửa thì đúng một người thắng.
//
// 409 conflict: giữ nguyên bản đang sửa trên màn, dừng lưu, đưa nút Tải lại —
// không tự ghi đè bản của người khác. 400 theo trường: giữ nguyên chữ đã gõ và
// hiện câu lỗi dưới đúng ô (không lấy lại giá trị từ máy chủ).
import { useCallback, useRef, useState } from 'react'
import { ApiError } from '../../../api/request'
import {
  getTemplate, libraryErrorInfo, patchTemplate,
  type Recipe, type TemplateDetail,
} from '../../../api/library'

export type SaveStatus = {
  /** bản đang hiện trên màn (gồm cả sửa chưa lưu) */
  detail: TemplateDetail
  saving: boolean
  /** có thay đổi chưa lên máy chủ */
  dirty: boolean
  /** lần lưu vừa xong trong phiên này — đầu trang nói "Đã lưu vừa xong" */
  justSaved: boolean
  /** máy chủ đã có bản khác — dừng lưu tới khi Tải lại */
  conflict: boolean
  /** lỗi theo trường API (`fields[].field` → câu) */
  fieldErrors: Record<string, string>
  /** câu lỗi chung khi lỗi không thuộc một trường */
  error: string | null
}

type Pending = { name?: string; recipe?: Recipe }

export function useTemplateDraft(initial: TemplateDetail) {
  const [state, setState] = useState<SaveStatus>({
    detail: initial, saving: false, dirty: false, justSaved: false, conflict: false, fieldErrors: {}, error: null,
  })
  // Bản mới nhất để đọc ngay trong callback (setState không đồng bộ).
  const draft = useRef<TemplateDetail>(initial)
  const revision = useRef(initial.revision)
  const savedAt = useRef(initial.savedAt)
  const pending = useRef<Pending | null>(null)
  const busy = useRef(false)
  const conflict = useRef(false)

  const publish = useCallback((patch: Partial<SaveStatus>) => setState((prev) => ({ ...prev, detail: draft.current, ...patch })), [])

  const flush = useCallback(async function run(): Promise<void> {
    if (busy.current) return
    const next = pending.current
    if (!next) {
      publish({ saving: false, dirty: false })
      return
    }
    pending.current = null
    busy.current = true
    publish({ saving: true, dirty: true, justSaved: false, error: null })
    try {
      const saved = await patchTemplate(draft.current.id, {
        revision: revision.current,
        ...(next.name !== undefined ? { name: next.name } : {}),
        ...(next.recipe !== undefined ? { recipe: next.recipe } : {}),
      })
      revision.current = saved.revision
      savedAt.current = saved.savedAt
      const queued = pending.current as Pending | null
      // Không có bản chờ mới → nhận luôn tên/recipe máy chủ (đã chuẩn hoá);
      // còn bản chờ thì giữ nguyên chữ đang hiện trên màn, chỉ cập nhật mốc.
      draft.current = {
        ...draft.current,
        revision: saved.revision,
        savedAt: saved.savedAt,
        updatedAt: saved.updatedAt,
        preview: saved.preview,
        price: saved.price,
        ...(queued === null
          ? { name: saved.name ?? draft.current.name, ...(saved.recipe ? { recipe: saved.recipe } : {}) }
          : queued.name !== undefined ? { name: queued.name } : {}),
      }
      conflict.current = false
      publish({ saving: false, dirty: queued !== null, justSaved: true, conflict: false, fieldErrors: {} })
    } catch (err) {
      const info = libraryErrorInfo(err)
      if (err instanceof ApiError && err.status === 409 && info.code === 'conflict') {
        conflict.current = true
        pending.current = null
        publish({ saving: false, dirty: true, justSaved: false, conflict: true, error: null })
      } else if (err instanceof ApiError && err.status === 400) {
        const fields: Record<string, string> = {}
        for (const field of info.fields) fields[field.field] = field.message
        publish({
          saving: false,
          dirty: true,
          justSaved: false,
          fieldErrors: fields,
          error: info.fields.length ? null : err.message,
        })
      } else {
        publish({ saving: false, dirty: true, justSaved: false, error: err instanceof Error ? err.message : String(err) })
      }
    } finally {
      busy.current = false
    }
    if (pending.current && !conflict.current) void run()
  }, [publish])

  const enqueue = useCallback((patch: Pending) => {
    pending.current = { ...(pending.current ?? {}), ...patch }
    void flush()
  }, [flush])

  /** Đổi recipe (một khối đầy đủ như `scenes`/`appearance`) — lưu ngay. */
  const save = useCallback((patch: Partial<Recipe>) => {
    const recipe = { ...draft.current.recipe, ...patch }
    draft.current = { ...draft.current, recipe }
    publish({ dirty: true, error: null })
    enqueue({ recipe })
  }, [enqueue, publish])

  /** Đổi tên: gửi đúng chữ đã gõ; chỉ nhận tên máy chủ trả khi không còn bản chờ. */
  const saveName = useCallback((name: string) => {
    enqueue({ name })
  }, [enqueue])

  /** Tải lại sau 409 (hoặc khi người dùng chủ động): bỏ bản đang sửa, lấy bản máy chủ. */
  const reload = useCallback(async () => {
    const fresh = await getTemplate(draft.current.id)
    draft.current = fresh
    revision.current = fresh.revision
    savedAt.current = fresh.savedAt
    pending.current = null
    conflict.current = false
    setState({ detail: fresh, saving: false, dirty: false, justSaved: false, conflict: false, fieldErrors: {}, error: null })
    return fresh
  }, [])

  return { ...state, save, saveName, reload }
}
