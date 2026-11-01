// Chuỗi Hiệu quả sau đăng (artboard Channel › tab=videos filter=posted|all,
// Settings › Kết nối AI). Chữ lấy nguyên văn từ artboard; hộp dán link YouTube,
// hàng khoá YouTube Data API và các trạng thái chờ/lỗi/mất kết nối không có trên
// board nên theo mẫu V12 (11-hoi-dap #68–#72) và ghi vào hỏi-đáp #74.
const compact = new Intl.NumberFormat('vi-VN', { maximumFractionDigits: 1, minimumFractionDigits: 1 })
const whole = new Intl.NumberFormat('vi-VN')

/** 8100 → “8,1 N”, 1_200_000 → “1,2 Tr”; dưới 1000 giữ nguyên số. */
export function viewsShort(views: number): string {
  if (views >= 1_000_000) return `${compact.format(views / 1_000_000)} Tr`
  if (views >= 1_000) return `${compact.format(views / 1_000)} N`
  return whole.format(views)
}

/** 1.4 → “×1,4” */
export function ratioText(ratio: number): string {
  return `×${compact.format(ratio)}`
}

export const performanceCopy = {
  insights: {
    title: 'Trợ lý nhận ra',
    prelim: (counted: number) => `Sơ bộ · ${counted} video có số liệu 7 ngày`,
    help: 'So theo lượt xem 7 ngày · không tự áp dụng',
    // Cột nhãn của thanh rộng 150px: nhãn dài (“Không phải câu hỏi”) bỏ chữ “video” để vừa, như artboard.
    bar: (label: string, n: number) => (label.length > 14 ? `${label} · ${n}` : `${label} · ${n} video`),
    apply: { add_rule: 'Thêm vào quy tắc viết', 'template.rotation': 'Cho chạy nhiều hơn' } as const,
    applying: 'Đang áp dụng…',
    applied: 'Đã áp dụng',
    appliedRule: 'Đã thêm vào quy tắc viết — xem ở Hồ sơ kênh › Khi viết tiêu đề, mô tả, tag',
    appliedRotation: 'Đã cho template này chạy nhiều hơn',
    gone: 'Nhận xét này đã thay đổi theo số liệu mới — mở lại để xem.',
    needMore: (n: number) => `Cần thêm ${n} video có số liệu 7 ngày để Trợ lý so sánh`,
    needMoreSub: 'Trợ lý so theo template, kiểu tiêu đề, độ dài và giờ đăng khi kênh có từ 6 video đã đăng có số 7 ngày.',
    nothing: 'Chưa có khác biệt đủ rõ giữa các nhóm video — Trợ lý sẽ nói khi thấy.',
    loading: 'Đang đọc số liệu sau đăng…',
    loadFail: 'Không đọc được số liệu sau đăng.',
    retry: 'Thử lại',
    offline: 'Mất kết nối · số liệu cập nhật lại khi có kết nối',
    teaser: (n: number, counted: number) => `Trợ lý nhận ra ${n} điều từ ${counted} video có số 7 ngày`,
    teaserOpen: 'Xem ở Đã đăng ›',
  },
  posted: {
    detail: (views: string, window: '24 giờ' | '7 ngày' | '28 ngày', vs?: string) =>
      `${views} / ${window}${vs ? ` · ${vs} so với trung vị` : ''}`,
  },
  mark: {
    title: 'Đánh dấu đã đăng?',
    body: 'Video chuyển sang Đã đăng. Dán link YouTube nếu có để Trinh sát đọc lượt xem sau 24 giờ, 7 ngày, 28 ngày; bỏ trống thì app tự khớp theo link kênh.',
    label: 'Link YouTube (không bắt buộc)',
    placeholder: 'https://www.youtube.com/watch?v=…',
    ok: 'Đánh dấu đã đăng',
    cancel: 'Huỷ',
    pending: 'Đang lưu…',
    done: 'Đã đánh dấu đã đăng',
    badLink: 'Link này không phải video YouTube — dán link dạng youtube.com/watch?v=… hoặc youtu.be/…',
    failed: 'Không đánh dấu được — thử lại.',
    offline: 'Mất kết nối máy chủ — chưa đánh dấu được.',
  },
  dataKey: {
    name: 'YouTube Data API',
    /* Nhóm ngoài danh sách nhà cung cấp AI (r38): khoá này không phải AI. */
    groupTitle: 'Số liệu YouTube',
    groupSub: 'Không phải AI · Trinh sát dùng để đọc số liệu video đã đăng',
    kind: 'Khoá API',
    pill: 'Không bắt buộc',
    sub: {
      empty: 'Chưa có khoá · số liệu sau đăng đọc từ trang công khai',
      ok: (hint: string) => `Đã lưu ${hint} · Trinh sát đọc lượt xem, thích, bình luận qua API`,
      loading: 'Đang tải…',
      fail: 'Không đọc được trạng thái khoá',
      offline: 'Mất kết nối máy chủ — mọi nút của hàng này tắt cho tới khi kết nối lại',
    },
    add: 'Thêm khoá',
    change: 'Đổi khoá',
    remove: 'Gỡ khoá',
    /* failed: chưa đọc được trạng thái khoá — đọc lại chứ không mở ô dán. */
    retry: 'Thử lại',
    keyLabel: 'Khoá API',
    /* Dòng gợi ý dưới ô khi ở chế độ sửa (artboard Settings/MSettings ytkey=edit). */
    hint: 'Tạo khoá trong Google Cloud Console rồi bật YouTube Data API v3 cho dự án đó. Không có khoá, Trinh sát vẫn đọc số liệu từ trang công khai của video.',
    save: 'Lưu khoá',
    cancel: 'Huỷ',
    saved: 'Đã lưu khoá YouTube Data API',
    removed: 'Đã gỡ khoá YouTube Data API',
    rejected: 'Google không nhận khoá này — kiểm tra lại khoá và bật YouTube Data API v3.',
    avatar: 'YT',
  },
} as const
