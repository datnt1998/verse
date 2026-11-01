// Copy của mục Cài đặt › Công cụ trên máy — Settings.dc.html (tools=ok|update|
// failed|installing) và MSettings.dc.html (screen=tools). Một hàng = một việc
// app cần máy làm; câu phụ có phiên bản do server ghép, ở đây chỉ có câu mô tả
// gốc (dùng khi lượt cài đang chạy) + nhãn trạng thái/nút.
export const tools = {
  lead: 'Viderse tự tải, cài và cập nhật các công cụ này. Mọi việc xử lý video đều chạy trên máy của bạn.',
  recheck: 'Kiểm tra lại',
  jobNames: {
    'yt-dlp': 'Tải video và phụ đề',
    ffmpeg: 'Xử lý hình và tiếng',
    transcription: 'Tách tiếng và tạo phụ đề',
    chrome: 'Dựng và kiểm tra video',
    codex: 'Công cụ vẽ ảnh',
    'codex-login': 'Đăng nhập ChatGPT',
  } as Record<string, string>,
  /** Câu mô tả gốc — server ghép thêm “· bản X” khi công cụ đã cài. */
  descs: {
    'yt-dlp': 'Lấy video, thông tin và lời thoại từ YouTube',
    ffmpeg: 'Cắt cảnh, xử lý âm thanh và dựng video',
    transcription: 'Chép lời thoại thành chữ ngay trên máy',
    chrome: 'Dùng Chrome chạy ngầm để xuất video và đo âm thanh',
    codex: 'Vẽ ảnh cho từng cảnh · chạy tách riêng, không đụng dữ liệu khác',
    'codex-login': 'Ảnh được vẽ bằng tài khoản ChatGPT của bạn, nên bước này cần bạn tự đăng nhập.',
  } as Record<string, string>,
  /** Câu mô tả bản điện thoại (MSettings › screen=tools): ngắn hơn bản desktop,
   *  không kèm phiên bản — phiên bản nằm trong thẻ “Chi tiết”. */
  descsMobile: {
    'yt-dlp': 'YouTube hay đổi cách tải, nên cập nhật để tải không lỗi.',
    ffmpeg: 'Cắt cảnh, xử lý âm thanh và dựng video',
    transcription: 'Chép lời thoại thành chữ ngay trên máy',
    chrome: 'Chrome chạy ngầm để xuất video và đo âm thanh',
    codex: 'Vẽ ảnh cho từng cảnh, chạy tách riêng',
    'codex-login': 'Đã đăng nhập · quản lý ở Kết nối AI',
  } as Record<string, string>,
  sizeProvided: 'Có sẵn',
  pills: {
    ready: 'Đã cài',
    update: 'Có bản mới',
    missing: 'Chưa cài',
    failed: 'Chưa cài được',
    installing: 'Đang cài',
    needsLogin: 'Cần bạn',
    queued: 'Chờ',
    loggedIn: 'Đã đăng nhập',
  },
  actions: {
    install: 'Cài',
    update: 'Cập nhật',
    retry: 'Tải tiếp',
    login: 'Đăng nhập',
    cancel: 'Huỷ',
  },
  /** Thẻ tổng trên cùng (artboard: tHead/tSub theo tweak tools). */
  summary: {
    ok: 'Máy sẵn sàng',
    okSub: 'Mọi công cụ đã cài và chạy được · kiểm tra lúc {time}',
    update: 'Máy sẵn sàng · có {n} bản cập nhật',
    updateSub: 'Kiểm tra lúc {time}. Nên cập nhật nếu tải video bị lỗi.',
    installing: 'Đang cài {i}/6 · còn khoảng {eta}',
    installingNoEta: 'Đang cài {i}/6',
    installingSub: 'Cứ dùng app bình thường. Bước nào cần công cụ đang cài sẽ tự chờ.',
    failed: '{n} công cụ cần sửa',
    failedSub: 'Video đang làm vẫn giữ nguyên; video mới chờ ở bước {step}.',
    missing: '{n} công cụ chưa cài',
    missingSub: 'Cài để mọi việc chạy được trên máy này.',
  },
  /** Bước dựng video mà mỗi công cụ chặn khi lỗi (artboard: “bước Phụ đề”). */
  blockedStep: {
    'yt-dlp': 'Tải',
    ffmpeg: 'Dựng',
    transcription: 'Phụ đề',
    chrome: 'Xuất',
    codex: 'Vẽ ảnh',
  } as Record<string, string>,
  /** Nối sau câu lỗi của server trên hàng lỗi (artboard toolsBad.hint). */
  failConsequence: 'Video mới sẽ dừng ở bước {step} tới khi sửa xong.',
  advanced: {
    title: 'Nâng cao · thư mục dữ liệu và phiên bản',
    mobileTitle: 'Chi tiết · phiên bản và thư mục',
    dataDir: 'Thư mục dữ liệu',
    none: '—',
    /** Nhãn kỹ thuật của vùng Nâng cao (được phép lộ tên binary ở đây). */
    techNames: {
      'yt-dlp': 'yt-dlp',
      ffmpeg: 'ffmpeg · ffprobe',
      transcription: 'faster-whisper',
      chrome: 'Chrome',
      codex: 'Codex',
      'codex-login': 'ChatGPT',
    } as Record<string, string>,
  },
  checkedAt: 'Kiểm tra lúc {time}',
} as const
