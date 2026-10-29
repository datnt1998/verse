// Tín hiệu "phiên hết hạn" bên trong app: mọi request 401 (kể cả probe của
// luồng sự kiện) đều đi qua đây. SessionProvider đăng ký listener xoá user —
// Root unmount shell → startDataEvents tháo: dừng poll/SSE và quét cache,
// không cần tải lại trang. /api/me 401 lúc boot không qua đường này (chỉ là
// "chưa có phiên"), nên không có vòng lặp: listener chỉ setUser(null), không
// gọi request mới.
const listeners = new Set<() => void>()

export function onSessionExpired(fn: () => void): () => void {
  listeners.add(fn)
  return () => listeners.delete(fn)
}

export function notifySessionExpired(): void {
  for (const fn of [...listeners]) fn()
}
