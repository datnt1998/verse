// Chuỗi riêng của Trình đọc (Reader.dc.html · 05-screens.md · 07-copy.md).
// Nằm trong thư mục màn vì i18n/vi.ts chưa có namespace `reader`; khi foundation mở
// namespace, chuyển các hằng dưới sang `t.reader` mà không đổi component.

export const copy = {
  chapters: 'Chương',
  words: (formatted: string) => `${formatted} chữ`,
  chapterMeta: (n: number, words: string) => `Chương ${n} · ${words}`,
  volume: (n: number, title: string) => (title.startsWith('Tập') ? title : `Tập ${n} · ${title}`),
  arc: (n: number, title: string) => (title.startsWith('Hồi') ? title : `Hồi ${n} · ${title}`),
  final: 'Bản nộp',
  draft: 'Bản nháp',
  sourceLabel: 'Nguồn văn bản',
  diff: 'Thay đổi gần nhất',
  diffUnavailable: 'Chưa ghi lại thay đổi nào cho chương này',
  sizeDown: 'Giảm cỡ chữ',
  sizeUp: 'Tăng cỡ chữ',
  emptyChapter: 'Chương chưa có nội dung',
  emptyChapterHint: 'Agent chưa nộp bản cho chương này.',
  emptyDraft: 'Chưa có bản nháp cho chương này',
  emptyDraftHint: 'Chương này chưa được viết lại hoặc đang chờ agent.',
  viewFinal: 'Xem bản nộp',
  loadError: 'Không tải được chương',
  noChapters: 'Chưa có chương nào',
  noChaptersHint: 'Chương sẽ xuất hiện khi agent bắt đầu viết.',
  noPermission: 'Bạn không có quyền đọc truyện này',
  chapterMissing: (n: number) => `Chương ${n} không tồn tại`,
  goLatest: 'Về chương mới nhất',
  retry: 'Thử lại',
  quick: {
    rewrite: (n: number) => `Viết lại chương ${n}`,
    polish: (n: number) => `Gọt giũa chương ${n}: `,
    review: () => 'Nhờ Biên tập viên thẩm duyệt',
    sync: 'Nhận bản sửa tay',
  },
  composerLabel: 'Ý kiến sửa chương',
  send: 'Gửi',
  sent: (n: number) => `Đã gửi yêu cầu sửa cho chương ${n}`,
  sendFailed: 'Không gửi được yêu cầu sửa',
  readOnlyNote: 'Bạn chỉ có quyền đọc truyện này — không gửi yêu cầu sửa được.',
  lockedNote: 'Truyện đang được mở ở nơi khác — chỉ đọc.',
  reviewTab: 'Thẩm duyệt',
  exportTab: 'Xuất truyện',
  noIssues: 'Không có vấn đề',
  contract: 'Hợp đồng',
  criteria: 'Điểm theo tiêu chí',
  dimension: {
    consistency: 'Nhất quán', character: 'Nhân vật', pacing: 'Nhịp truyện', continuity: 'Liền mạch',
    foreshadow: 'Phục bút', hook: 'Móc', aesthetic: 'Thẩm mỹ',
  } as Record<string, string>,
  noReview: 'Chưa có thẩm duyệt',
  noReviewHint: 'Chương này chưa được Biên tập viên thẩm duyệt.',
  plan: 'Kế hoạch chương',
  planEmpty: 'Chương chưa có kế hoạch',
  facts: 'Dữ kiện chương',
  factsEmpty: 'Chưa có dữ kiện cho chương này',
  revisions: 'Thay đổi gần nhất',
  revisionsEmpty: 'Chưa ghi lại thay đổi nào',
  foreshadow: 'Phục bút liên quan',
  foreshadowEmpty: 'Không có phục bút liên quan',
  fields: {
    goal: 'Mục tiêu', conflict: 'Xung đột', hook: 'Móc', emotionArc: 'Cung cảm xúc', notes: 'Ghi chú',
    openingState: 'Trạng thái mở chương',
    summary: 'Tóm tắt', keyEvents: 'Sự kiện chính', characters: 'Nhân vật', timeline: 'Dòng thời gian',
    requiredBeats: 'Nhịp bắt buộc', forbiddenMoves: 'Điều cấm', continuity: 'Kiểm tra liền mạch',
    evaluation: 'Trọng tâm đánh giá', payoff: 'Điểm trả', hookGoal: 'Mục tiêu móc', emotionTarget: 'Đích cảm xúc',
  },
  plantedAt: (n: number) => `Ch.${n}`,
  dueBy: (n: number) => `hạn Ch.${n}`,
  foreshadowStatus: { planted: 'đã gieo', advanced: 'đã đẩy', resolved: 'đã trả', abandoned: 'đã bỏ' } as Record<string, string>,
  export: {
    title: 'Xuất truyện',
    fileName: 'Tên tệp · lưu trong mục Bản xuất',
    format: 'Định dạng · theo đuôi tệp',
    formats: { txt: 'Văn bản .txt', epub: 'Ebook .epub', narration: 'Bản đọc TTS' } as Record<string, string>,
    formatNote: {
      txt: 'Văn bản thường, giữ tiêu đề chương.',
      epub: 'Ebook có mục lục, đọc trên điện thoại.',
      narration: 'Dành cho giọng đọc AI: không tiêu đề chương, không in nghiêng hay in đậm, các chương cách nhau hai dòng trống.',
    } as Record<string, string>,
    readNumbers: 'Đọc số thành chữ',
    preview: 'Xem trước',
    previewNumbers: 'Ngày mười tám, chị ta chuyển cho tôi hai nghìn tệ, kèm một dòng: đừng làm to chuyện.',
    previewPlain: 'Ngày 18, chị ta chuyển cho tôi 2.000 tệ, kèm một dòng: đừng làm to chuyện.',
    from: 'Từ chương',
    to: 'Đến chương',
    overwrite: 'Ghi đè tệp đã có',
    exportRange: (a: number, b: number) => `Xuất chương ${a}–${b}`,
    exporting: 'Đang xuất…',
    note: 'Chương chưa hoàn thành sẽ được bỏ qua. Có thể xuất cả khi trợ lý đang viết.',
    latest: 'Lần xuất gần nhất',
    download: 'Tải về máy',
    downloadLatest: 'Tải xuống',
    downloadFile: 'Tải tệp',
    downloadStarted: (name: string) => `Đã tải ${name}`,
    downloadFailed: 'Không tải được tệp',
    exportFailed: 'Không xuất được truyện',
    exported: (chapters: number, name: string) => `Đã xuất ${chapters} chương · ${name}`,
    skipped: (n: number) => `bỏ qua ${n} chương chưa hoàn thành`,
    noExports: 'Chưa có bản xuất',
    noExportsHint: 'Xuất chương để tạo tệp trong mục Bản xuất.',
    matrix: 'Theo tập',
    readOnly: 'Chỉ đọc — không xuất được vào thư viện',
    invalidRange: 'Khoảng chương không hợp lệ',
    chaptersCount: (n: number) => `${n} chương`,
    missing: '—',
    copyFailed: 'Trình duyệt chặn sao chép — hãy chọn đường dẫn bằng tay.',
  },
} as const

const numberFormat = new Intl.NumberFormat('vi-VN')
export const formatNumber = (value: number) => numberFormat.format(value)

export function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B'
  if (bytes < 1024) return `${bytes} B`
  const kb = bytes / 1024
  if (kb < 1024) return `${kb.toFixed(1)} KB`
  return `${(kb / 1024).toFixed(1)} MB`
}

const pad = (value: number) => String(value).padStart(2, '0')
export function formatClock(iso: string | null): string {
  if (!iso) return '—'
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return '—'
  return `${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}

export function formatDateTime(iso: string | null): string {
  if (!iso) return '—'
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return '—'
  return `${pad(date.getDate())}/${pad(date.getMonth() + 1)} ${formatClock(iso)}`
}
