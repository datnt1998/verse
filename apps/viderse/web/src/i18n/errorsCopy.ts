// Câu cho lỗi cấu hình/thiết bị dùng ở màn Cài đặt (không thuộc khung app).
export const errorsCopy = {
  errors: {
    login_required: 'Cần đăng nhập ChatGPT để Hoạ sĩ vẽ tiếp.',
    cookies_failed: 'YouTube yêu cầu đăng nhập để tải video này.',
    unsupported_platform: (platform: string) => `Chưa hỗ trợ ${platform}. App lấy nguồn nền từ Twitch và Bilibili.`,
    unavailable: 'Video nguồn không còn xem được (đã ẩn hoặc xoá).',
    no_subtitles: 'Video không có phụ đề. Trinh sát sẽ tự chép lời từ âm thanh.',
    quality_gate: (score: number) => `Chưa đạt độ hoàn thiện ${score}/100`,
    auth_failed: (provider: string) => `${provider} từ chối khoá API này. Kiểm tra lại khoá.`,
    template_changed: 'Mẫu video đã đổi số ảnh hoặc chỉ dẫn ảnh. Đạo diễn cần chia cảnh lại.',
    failed: 'Có lỗi xảy ra. Thử lại.',
  }
} as const

export const imageFailureCopy = {
  quota: 'Đã chạm hạn mức vẽ ảnh. Chờ hạn mức rồi vẽ lại.',
  filter: 'Bộ lọc của nhà cung cấp chặn nội dung này. Sửa chỉ dẫn rồi vẽ lại.',
  network: 'Mất kết nối tới nhà cung cấp. Kiểm tra mạng rồi vẽ lại.',
  provider: 'Nhà cung cấp chưa vẽ được ảnh này. Thử lại.',
} as const

export const backdropFailureCopy: Readonly<Record<string, string>> = {
  failed: 'Chưa tải được nền. Thử lại.',
  rate_limited: 'Nền tảng đang tạm chặn yêu cầu. Chờ rồi thử lại.',
  blocked: 'Nguồn chặn tải nền. Thử lại sau.',
  not_found: 'Không tìm thấy video nền. Chọn video khác.',
  transient: 'Mất kết nối khi tải nền. Thử lại.',
}

const trialFailures: Readonly<Record<string, string>> = {
  quota: imageFailureCopy.quota,
  filter: imageFailureCopy.filter,
  refused: imageFailureCopy.filter,
  network: imageFailureCopy.network,
  provider: imageFailureCopy.provider,
  timeout: 'Lần vẽ thử quá thời gian chờ. Thử lại.',
  interrupted: 'Lần vẽ thử bị gián đoạn. Thử lại để vẽ một ảnh mới.',
  login_required: 'Cần đăng nhập lại nguồn vẽ ảnh trong Kết nối AI.',
  auth_failed: 'Khoá kết nối không còn dùng được. Sửa khoá trong Kết nối AI.',
  tool_missing: 'Công cụ vẽ ảnh chưa sẵn sàng. Kiểm tra trong Cài đặt.',
  busy: 'Nguồn vẽ ảnh đang bận. Chờ rồi thử lại.',
}

export const trialFailureText = (code?: string): string =>
  (code && trialFailures[code]) || 'Chưa vẽ được ảnh thử. Thử lại.'
