// G8 — nhớ id phiên nháp đồng sáng tác cấp dự án trong sessionStorage để tải lại trang vẫn tiếp tục.
// Nháp nằm ở <dự án>/.ainovel/drafts/<id>/ và chỉ mất khi commit/cancel (hoặc quá 7 ngày).

const PREFIX = 'ainovel.cocreate.'

export const projectDraftKey = (project: string) => `${PREFIX}${project}`

type DraftStorage = Pick<Storage, 'getItem' | 'setItem' | 'removeItem'>

const fallback = (): DraftStorage => ({
  getItem: () => null,
  setItem: () => undefined,
  removeItem: () => undefined,
})

const storage = (): DraftStorage => {
  try {
    return sessionStorage
  } catch {
    // Chế độ riêng tư chặn sessionStorage — coi như không có phiên nháp.
    return fallback()
  }
}

export function readProjectDraft(project: string, store: DraftStorage = storage()): string | null {
  try {
    const value = store.getItem(projectDraftKey(project))
    return value && value.trim() ? value : null
  } catch {
    return null
  }
}

export function rememberProjectDraft(project: string, draftId: string, store: DraftStorage = storage()): void {
  try {
    store.setItem(projectDraftKey(project), draftId)
  } catch {
    // Không nhớ được thì vẫn dùng được trong phiên hiện tại.
  }
}

export function forgetProjectDraft(project: string, store: DraftStorage = storage()): void {
  try {
    store.removeItem(projectDraftKey(project))
  } catch {
    // bỏ qua
  }
}
