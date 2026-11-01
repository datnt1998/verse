// Định dạng số hiển thị dùng chung cho shell và editor (thông báo hoạt động,
// hộp xuất video). Tách khỏi editor để chunk đầu không kéo mã editor.

export function formatBytes(bytes: number): string {
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toLocaleString('vi-VN', { maximumFractionDigits: 1 })} KB`
  if (bytes < 1024 ** 3) return `${(bytes / 1024 ** 2).toLocaleString('vi-VN', { maximumFractionDigits: 1 })} MB`
  return `${(bytes / 1024 ** 3).toLocaleString('vi-VN', { maximumFractionDigits: 2 })} GB`
}

export function formatDuration(seconds: number): string {
  const rounded = Math.max(0, Math.ceil(seconds))
  return `${Math.floor(rounded / 60).toString().padStart(2, '0')}:${(rounded % 60).toString().padStart(2, '0')}`
}
