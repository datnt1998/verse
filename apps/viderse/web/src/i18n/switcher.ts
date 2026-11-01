// Chữ của bộ đổi kênh — tách khỏi i18n/vi.ts để thanh trên (entry) không kéo
// cả từ điển. Chỉ đúng những chuỗi ChannelSwitcher dùng.
export const switcherCopy = {
  title: 'Đổi kênh',
  allChannels: 'Mọi kênh',
  createChannel: 'Tạo kênh',
  viewingAllChannels: 'đang xem Mọi kênh',
  selectingChannel: (name: string) => `đang chọn ${name}`,
  waiting: (n: number) => `${n} chờ bạn`,
  waitingChannels: (n: number) => `${n} kênh chờ bạn`,
  waitingOne: 'có video chờ bạn',
} as const
