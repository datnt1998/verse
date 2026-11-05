import { createElement } from 'react'
import type { ConfirmDialogProps } from './ConfirmDialog'
import { toast } from './Toast'

let confirmLayout: 'dialog' | 'responsive' = 'dialog'
/** Configure once before mounting the app; other applications keep the centered dialog. */
export const setConfirmLayout = (layout: 'dialog' | 'responsive') => { confirmLayout = layout }
export { confirmLayout }

export const confirmationRememberKey = (user: string, kind: string) => `ainovel.confirm.${encodeURIComponent(user)}.${encodeURIComponent(kind)}`
export function isConfirmationRemembered(user: string, kind: string) {
  try { return localStorage.getItem(confirmationRememberKey(user, kind)) === '1' } catch { return false }
}

export type ConfirmOptions = Omit<ConfirmDialogProps, 'onClose'>
let confirmationGeneration = 0
const activeConfirmations = new Set<() => void>()
/** A session boundary discards UI, never retries or pretends to cancel a submitted mutation. */
export function dismissConfirmations() {
  confirmationGeneration++
  for (const dismiss of [...activeConfirmations]) dismiss()
}

export function confirm(options: ConfirmOptions): Promise<boolean> {
  const generation = confirmationGeneration
  if (options.remember && isConfirmationRemembered(options.remember.user, options.remember.kind)) return Promise.resolve().then(async () => {
    if (generation !== confirmationGeneration) return false
    await options.onConfirm(options.defaultChoice ?? options.choices?.[0]?.value ?? '')
    return generation === confirmationGeneration
  })
  return Promise.all([import('react-dom/client'), import('./ConfirmDialog')]).catch((error: unknown) => {
    // Chỉ nhánh tải module hỏng (deploy mới / ngoại tuyến sau khi mở tab). Thao tác không chạy;
    // báo bằng toast có việc để làm thay vì nuốt lỗi. Đổi phiên thì không dựng UI của phiên cũ.
    console.error('[confirm] Không tải được hộp thoại xác nhận', error)
    if (generation === confirmationGeneration) toast('Không mở được hộp thoại xác nhận. Kiểm tra kết nối rồi tải lại trang để thử lại.', 'err', { label: 'Tải lại', onClick: () => location.reload() })
    return null
  }).then((loaded) => {
    if (!loaded) return false
    const [{ createRoot }, { ConfirmDialog }] = loaded
    if (generation !== confirmationGeneration) return false
    return new Promise<boolean>((resolve) => {
      const container = document.createElement('div'); document.body.append(container)
      const root = createRoot(container)
      let finished = false
      const finish = (accepted = false) => {
        if (finished) return
        finished = true
        activeConfirmations.delete(dismiss)
        queueMicrotask(() => { root.unmount(); container.remove(); resolve(accepted) })
      }
      const dismiss = () => finish(false)
      activeConfirmations.add(dismiss)
      root.render(createElement(ConfirmDialog, { ...options, onClose: finish }))
    })
  })
}
