// Điểm Kiểm định và nhịp lấy video; không còn cấu hình Mức tự chủ.
import { base } from './vi'

export const autonomyCopy = {
  pacing: {
    paused: 'Tạm dừng · cần ít nhất một đối thủ liên kết',
    note: 'Nhịp này áp cho video tự lấy và video thêm hàng loạt. Video bạn chọn từng cái luôn được làm trước, không tính vào nhịp.',
    unit: 'video mỗi ngày',
    today: (done: number, cap: number) => `Hôm nay ${done}/${cap}`,
    lastSeen: (time: string) => `lần xem gần nhất ${time}`,
    seeNow: 'Xem ngay',
    seeNowDone: 'Đã yêu cầu trinh sát xem lại',
    seeNowError: 'Chưa yêu cầu được. Thử lại.',
  },
  /** Review › điểm ảnh (artboard Review score=true). */
  score: {
    panelTitle: 'Điểm của Kiểm định',
    instructionIssue: 'Chỉ dẫn chưa được áp dụng',
    lead: (scored: number) => `Kiểm định chấm ${scored} ảnh:`,
    passed: (n: number, min: number, all: boolean) => `${all ? 'cả ' : ''}${n} ảnh từ ${min} điểm`,
    below: (label: string, value: number) => `${label} được ${value}`,
    lookFirst: ' — xem trước khi duyệt.',
    pending: (n: number) => `Còn ${n} ảnh đang chấm.`,
    askTail: (channel: string) => `${channel} chưa bật Tự duyệt ảnh nên vẫn hỏi bạn.`,
    scoreTail: (channel: string, min: number) => `${channel} duyệt theo điểm ≥ ${min} nên cả video chờ bạn.`,
    jump: (label: string) => `Xem ${label}`,
    badge: (value: number) => `Điểm Kiểm định ${value}`,
  },
}

/** Chữ khung app + điểm Kiểm định và nhịp kênh. */
export const t = { ...base, autonomy: autonomyCopy } as const
