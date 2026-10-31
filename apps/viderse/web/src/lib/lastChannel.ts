// Kênh đang xem gần nhất — nav "Kênh" mở kênh đó (hoặc kênh đầu) thay vì luôn
// mở màn Tạo kênh; chỉ khi chưa có kênh nào mới trỏ '#/channels/new'.
import { paths } from '../router'

const KEY = 'viderse:last-channel'

export function rememberChannel(id: string): void {
  try { localStorage.setItem(KEY, id) } catch { /* phiên cấm storage: vẫn dùng kênh đầu */ }
}

export function lastViewedChannelId(): string | null {
  try { return localStorage.getItem(KEY) } catch { return null }
}

/** Đích của mục nav "Kênh" (và tab Kênh ở thanh dưới điện thoại). */
export function channelsHref(channels: readonly { id: string }[]): string {
  if (channels.length === 0) return paths.channelNew()
  const last = lastViewedChannelId()
  const id = last && channels.some((channel) => channel.id === last) ? last : channels[0].id
  return paths.channel(id)
}
