// Chuỗi Mức tự chủ + điểm ảnh (artboard Channel › tab=sources, CreateChannel,
// Review › score, Dialogs › autonomy). Chữ lấy nguyên văn từ artboard; trạng
// thái đang tải/lỗi/mất kết nối không có trên board nên theo mẫu V12 (11-hoi-dap).
import { base } from './vi'

export const autonomyCopy = {
  title: 'Mức tự chủ',
  custom: 'Tuỳ chỉnh',
  sub: 'Máy tự quyết tới đâu ở kênh này. Việc máy tự làm luôn được kể lại trong Bản tin, có Hoàn tác khi được.',
  // Hai nút chữ riêng trong chip flex (gap 6) như artboard, nên không có dấu cách cuối.
  headChip: 'Tự chủ ·',
  presets: { careful: 'Cẩn thận', balanced: 'Cân bằng', auto: 'Tự chạy' },
  rows: {
    pick: {
      label: 'Lấy video mới của đối thủ',
      hint: 'Trinh sát xem đối thủ mỗi giờ',
      options: { suggest: 'Chỉ đề xuất video ≥ ×3', score2: 'Video ≥ ×2', all: 'Mọi video mới' },
    },
    approve: {
      label: 'Duyệt ảnh',
      hint: 'Duyệt theo điểm vẫn duyệt cả video: một ảnh dưới ngưỡng thì hỏi bạn',
      options: { ask: 'Luôn hỏi', all: 'Tự duyệt hết' },
      score: (min: number) => `Duyệt theo điểm ≥ ${min}`,
    },
    onError: {
      label: 'Khi gặp lỗi',
      hint: 'Chỉ lỗi máy tự sửa được: tải lại nguồn, vẽ lại ảnh lỗi, dựng lại',
      options: { stop: 'Dừng và báo', retry_once: 'Tự thử sửa 1 lần' },
    },
    belowQuality: {
      label: 'Video chưa đạt 90 điểm',
      hint: 'Khi Kiểm định chấm cả video dưới 90 sau khi dựng',
      options: { ask: 'Hỏi', fix_then_ask: 'Tự sửa rồi hỏi' },
    },
  },
  threshold: 'Ngưỡng điểm ảnh',
  thresholdRange: '70–95 · bước 5',
  thresholdDown: 'Giảm 5',
  thresholdUp: 'Tăng 5',
  guard: 'Không bao giờ tự làm: đăng YouTube, xoá, đổi mẫu video hay chỉ dẫn, bỏ liên kết đối thủ, tiêu ảnh trả phí quá giới hạn ngày.',
  /** Dòng phụ ở Tạo kênh theo mức đang chọn (artboard CreateChannel › AUTS). */
  createSub: {
    careful: 'Chỉ đề xuất video ≥ ×3 · duyệt ảnh luôn hỏi',
    balanced: 'Lấy mọi video mới · duyệt ảnh luôn hỏi',
    auto: 'Lấy mọi video mới · tự duyệt khi mọi ảnh ≥ 85',
  },
  pacing: {
    title: 'Nhịp và ưu tiên',
    hint: 'Trinh sát xem đối thủ mỗi giờ. Lấy video nào do Mức tự chủ bên dưới quyết định.',
    paused: 'Tạm dừng · cần ít nhất một đối thủ liên kết',
    note: 'Nhịp này áp cho video tự lấy và video thêm hàng loạt. Video bạn chọn từng cái luôn được làm trước, không tính vào nhịp.',
    unit: 'video mỗi ngày',
    today: (done: number, cap: number) => `Hôm nay ${done}/${cap}`,
    lastSeen: (time: string) => `lần xem gần nhất ${time}`,
    seeNow: 'Xem ngay',
    seeNowDone: 'Đã yêu cầu trinh sát xem lại',
    seeNowError: 'Chưa yêu cầu được. Thử lại.',
    priority: 'Ưu tiên kênh',
    steady: 'Đều đặn',
    views: 'Lượt xem',
    steadyHint: 'Giữ nhịp, lấy video theo thứ tự mới đăng.',
    viewsHint: 'Video điểm nổi trội cao chạy trước; mẫu video có lượt xem 7 ngày tốt hơn chạy nhiều hơn (tối đa gấp đôi).',
  },
  /** Trạng thái hệ thống V12 (board Channel không vẽ). */
  loading: 'Đang tải Mức tự chủ…',
  loadError: 'Chưa đọc được Mức tự chủ của kênh này.',
  saveError: 'Chưa lưu được Mức tự chủ. Thử lại.',
  offline: 'Mất kết nối · chưa đổi được Mức tự chủ',
  saving: 'Đang lưu…',
  /** Review › điểm ảnh (artboard Review score=true). */
  score: {
    panelTitle: 'Điểm của Kiểm định',
    why: 'Vì sao?',
    lead: (scored: number) => `Kiểm định chấm ${scored} ảnh:`,
    passed: (n: number, min: number, all: boolean) => `${all ? 'cả ' : ''}${n} ảnh từ ${min} điểm`,
    below: (label: string, value: number) => `${label} được ${value}`,
    lookFirst: ' — xem trước khi duyệt.',
    pending: (n: number) => `Còn ${n} ảnh đang chấm.`,
    askTail: (channel: string, preset: string) => `${channel} đang ở mức ${preset} nên vẫn hỏi bạn.`,
    scoreTail: (channel: string, min: number) => `${channel} duyệt theo điểm ≥ ${min} nên cả video chờ bạn.`,
    jump: (label: string) => `Xem ${label}`,
    badge: (value: number) => `Điểm Kiểm định ${value}`,
  },
}

/** `t` của thẻ Mức tự chủ: chữ khung app + Mức tự chủ. */
export const t = { ...base, autonomy: autonomyCopy } as const
