// Chữ dùng ở nhiều màn: khung app đọc tiêu đề màn (tab, tiêu đề trang, crumb) và
// vài nhãn cảnh báo, nên không thể nằm riêng trong chunk của màn nào. Một nguồn
// duy nhất: ./base giữ đúng khoá khung app cần, module của màn giữ bản đầy đủ —
// cả hai cùng trỏ về đây, không chép chữ.
export const shared = {
  workbenchTitle: 'Bàn dựng',
  reviewTitle: 'Duyệt',
  queueTitle: 'Hàng chờ',
  exploreTitle: 'Khám phá',
  newTitle: 'Video mới',
  editorTitle: 'Editor',
  settingsTitle: 'Cài đặt',
  channelTitle: 'Kênh',
  channelNoName: 'Kênh chưa đặt tên',
  channelCreateTitle: 'Tạo kênh',
  libraryTitle: 'Thư viện',
  templatesTab: 'Template',
  setupTitle: 'Chuẩn bị máy',
  setupSkippedWarn: 'Máy này chưa cài đủ công cụ cho 4 trợ lý.',
  setupSkippedAction: 'Cài tiếp',
  allRivals: 'Tất cả đối thủ',
  blockedNoTemplate: 'Kênh chưa có template dùng được — sửa ở tab Template của kênh.',
  exploreHasSubs: 'Có phụ đề do kênh đăng',
  weekdays: ['T2', 'T3', 'T4', 'T5', 'T6', 'T7', 'CN'] as const,
  newToasts: {
    made: 'Đã tạo job — theo dõi ở Hàng chờ.',
    openQueue: 'Mở Hàng chờ',
    failed: 'Chưa tạo được job.',
  },
} as const
