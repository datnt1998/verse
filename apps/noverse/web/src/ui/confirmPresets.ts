import { createElement } from 'react'
import { Icon } from '@noverse/ui'
import { confirm, type ConfirmOptions } from '@noverse/ui'

type PresetContext = { name?: string; displayName?: string; path?: string; user?: string; count?: number; phrase?: string; chapter?: number }
export type ConfirmSituation = 'pause' | 'overwrite' | 'device' | 'user' | 'story' | 'allDevices' | 'leave'
export const CONFIRM_SITUATIONS: { id: ConfirmSituation; label: string; location: string }[] = [
  { id: 'pause', label: 'Tạm dừng trợ lý', location: 'Nút Tạm dừng · Esc ở Bàn viết' },
  { id: 'overwrite', label: 'Ghi đè tệp xuất', location: '/export khi tệp đã có' },
  { id: 'device', label: 'Đăng xuất một thiết bị', location: 'Thiết bị đang đăng nhập' },
  { id: 'user', label: 'Xoá tài khoản', location: 'Quản lý tài khoản · chỉ Quản trị' },
  { id: 'story', label: 'Xoá truyện', location: 'Dự án › ⋯ › Xoá truyện' },
  { id: 'allDevices', label: 'Đăng xuất mọi thiết bị', location: 'Menu tài khoản › Thiết bị' },
  { id: 'leave', label: 'Rời khi chưa lưu', location: 'Bản đọc · sửa đoạn chưa lưu' },
]
function presetContent(situation: ConfirmSituation, context: PresetContext = {}): Omit<ConfirmOptions, 'onConfirm'> {
  const account = context.name ?? 'linh'
  const person = context.displayName ?? account.charAt(0).toLocaleUpperCase('vi') + account.slice(1)
  const path = context.path ?? 'exports/tra-xanh.epub'
  const device = context.name ?? 'iPhone của Linh'
  const count = context.count ?? 3
  switch (situation) {
    case 'pause': return { title: 'Tạm dừng Người viết?', body: 'Chương 25 đang ở bước Soát nhất quán. Bạn có thể chạy tiếp bất cứ lúc nào.', agent: true, confirmLabel: 'Tạm dừng', cancelLabel: 'Tiếp tục viết', pendingLabel: 'Đang tạm dừng…', choices: [{ value: 'after', label: 'Sau bước này', description: 'Khuyên dùng — giữ nguyên phần đã viết' }, { value: 'now', label: 'Ngay bây giờ', description: 'Bỏ phần đang viết dở của chương 25' }], remember: context.user ? { user: context.user, kind: 'pause-writer' } : undefined, successMessage: (choice) => choice === 'after' ? 'Đã tạm dừng Người viết sau bước này' : 'Đã tạm dừng Người viết' }
    case 'overwrite': return { title: 'Thay bản EPUB cũ?', body: `${path} đã có. Bản cũ sẽ được thay bằng bản mới.`, tone: 'warn', confirmLabel: 'Thay bản cũ', pendingLabel: 'Đang xuất…', compare: [{ label: 'Bản đang có', summary: '20 chương · 1,0 MB', meta: '2 ngày trước' }, { label: 'Bản mới', summary: '24 chương · ~1,2 MB', meta: 'vừa tạo' }], successMessage: `Đã thay ${path}` }
    case 'device': return { title: `Đăng xuất “${device}”?`, body: 'Thiết bị này phải quét mã QR hoặc nhập mật khẩu để vào lại. Truyện và dữ liệu không bị ảnh hưởng.', tone: 'dsoft', confirmLabel: 'Đăng xuất thiết bị', pendingLabel: 'Đang đăng xuất…', successMessage: `Đã đăng xuất ${device}` }
    case 'user': return { title: `Xoá tài khoản “${account}”?`, body: 'Việc này không hoàn tác được.', tone: 'danger', confirmLabel: 'Xoá tài khoản', consequences: [`${person} không đăng nhập được nữa, mọi thiết bị của ${person} bị đăng xuất`, `Truyện ${person} đã viết vẫn giữ nguyên trong thư viện`, 'Có thể tạo lại tài khoản cùng tên sau'], successMessage: `Đã xoá tài khoản ${account}` }
    case 'story': return { title: `Xoá truyện “${context.name ?? 'Trà Xanh Cướp Kịch Bản…'}”?`, body: 'Xoá 24 chương, đại cương, bản đọc và audio của truyện. Tệp trong thư mục exports vẫn giữ lại.', tone: 'danger', phrase: context.phrase ?? '01-tra-xanh', confirmLabel: 'Xoá vĩnh viễn', successMessage: 'Đã xoá truyện · bản xuất vẫn còn trong exports' }
    case 'allDevices': return { title: 'Đăng xuất mọi thiết bị khác?', body: `${count} thiết bị sẽ bị đăng xuất ngay. Thiết bị bạn đang dùng vẫn giữ phiên.`, tone: 'dsoft', hold: true, confirmLabel: 'Giữ để đăng xuất tất cả', pendingLabel: 'Đang đăng xuất…', successMessage: `Đã đăng xuất ${count} thiết bị khác` }
    case 'leave': return { title: 'Bỏ thay đổi chưa lưu?', body: `Bạn đã sửa ${count} đoạn trong bản đọc${context.chapter ? ` chương ${context.chapter}` : ''} nhưng chưa lưu.`, confirmLabel: 'Lưu rồi rời đi', cancelLabel: 'Ở lại', pendingLabel: 'Đang lưu…', successMessage: context.count === undefined ? 'Đã lưu thay đổi' : `Đã lưu ${context.count} đoạn` }
  }
}
export function confirmationPreset(situation: ConfirmSituation, context?: PresetContext) {
  const icons: Record<ConfirmSituation, string> = { pause: 'pause', overwrite: 'warning-2', device: 'devices', user: 'profile-delete', story: 'trash', allDevices: 'logout', leave: 'edit-2' }
  return { ...presetContent(situation, context), icon: createElement(Icon, { name: icons[situation], size: 20 }) }
}
export const confirmPause = (options: Pick<ConfirmOptions, 'onConfirm'> & Partial<ConfirmOptions>, context?: PresetContext) => confirm({ ...confirmationPreset('pause', context), ...options })
export const confirmOverwrite = (options: Pick<ConfirmOptions, 'onConfirm' | 'secondary'> & Partial<ConfirmOptions>, context?: PresetContext) => confirm({ ...confirmationPreset('overwrite', context), ...options })
export const confirmDeviceLogout = (options: Pick<ConfirmOptions, 'onConfirm'> & Partial<ConfirmOptions>, context?: PresetContext) => confirm({ ...confirmationPreset('device', context), ...options })
export const confirmDeleteAccount = (options: Pick<ConfirmOptions, 'onConfirm'> & Partial<ConfirmOptions>, context?: PresetContext) => confirm({ ...confirmationPreset('user', context), ...options })
export const confirmDeleteStory = (options: Pick<ConfirmOptions, 'onConfirm'> & Partial<ConfirmOptions>, context?: PresetContext) => confirm({ ...confirmationPreset('story', context), ...options })
export const confirmAllDevicesLogout = (options: Pick<ConfirmOptions, 'onConfirm'> & Partial<ConfirmOptions>, context?: PresetContext) => confirm({ ...confirmationPreset('allDevices', context), ...options })
export const confirmUnsavedLeave = (options: Pick<ConfirmOptions, 'onConfirm' | 'secondary'> & Partial<ConfirmOptions>, context?: PresetContext) => confirm({ ...confirmationPreset('leave', context), ...options })
