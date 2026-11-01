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
    template_changed: 'Template đã đổi số ảnh hoặc prompt ảnh. Đạo diễn cần chia cảnh lại.',
    failed: 'Có lỗi xảy ra. Thử lại.',
  }
} as const
