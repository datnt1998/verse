// M11 (18-mobile.md › M11) — luật bố cục thuần của màn Truyện mới trên điện thoại.
// Tách khỏi JSX để kiểm được: bàn phím mở thì ẩn gì, thẻ “Bản nháp yêu cầu” đang ở trạng thái nào.
// Chuỗi lấy từ `i18n/vi.ts` (07-copy.md › Gói AR) — hàm ở đây chỉ chọn nhãn, không tự viết câu.

export type QuickChrome = {
  /** Tiêu đề “Bạn muốn viết câu chuyện nào?” — ẩn khi đang gõ. */
  heading: boolean
  /** Bốn orb + câu dẫn — chỉ khi chưa so với truyện trước và chưa gõ. */
  orbs: boolean
  subtitle: boolean
  /** Thử một ý tưởng · Điều sẽ diễn ra · Cách khác để bắt đầu — ẩn khi gõ hoặc đã so. */
  extras: boolean
  /** `bottom` của nút chính: nằm ngay trên bàn phím khi bàn phím mở. */
  barOffset: number
}

/**
 * Artboard MNewStory (tweak `state=typing`): khi bàn phím mở, màn chỉ còn ô yêu cầu + khối so với
 * truyện trước + nút dính đáy; tiêu đề, câu dẫn và các khối phụ ẩn đi để ô chữ không bị đẩy khỏi màn.
 */
export function quickChrome({ keyboard, check }: { keyboard: number; check: boolean }): QuickChrome {
  // Bàn phím mở = `visualViewport` co lại (mobile/runtime.tsx đo và đặt `--kb`).
  const typing = keyboard > 0
  return {
    heading: !typing,
    orbs: !typing && !check,
    subtitle: !typing && !check,
    extras: !typing && !check,
    barOffset: typing ? Math.max(0, Math.round(keyboard)) : 0,
  }
}

/** Trạng thái của thẻ “Bản nháp yêu cầu” (M11 › Đồng sáng tác). */
export type DraftStage = 'none' | 'clarifying' | 'updating' | 'ready' | 'blocked'

export const DRAFT_LABEL: Record<DraftStage, string> = {
  none: 'Chưa có',
  clarifying: 'Đang làm rõ',
  updating: 'Đang cập nhật',
  ready: 'Sẵn sàng viết',
  blocked: 'Còn 1 mục Bắt sửa',
}

/**
 * `blocked` thắng `ready`: bản nháp đủ ý nhưng còn mục Bắt sửa khi so với truyện trước thì
 * thẻ đáy phải nói đúng lý do khoá, không được mời bấm Bắt đầu.
 * `updating` (đang chờ AI trả lời) cũng thắng `ready` cũ của lượt trước.
 */
export function draftStage({ draft, ready, busy, blocked }: { draft: string; ready: boolean; busy: boolean; blocked: boolean }): DraftStage {
  if (!draft.trim()) return 'none'
  if (busy) return 'updating'
  if (blocked) return 'blocked'
  return ready ? 'ready' : 'clarifying'
}

export type DraftTone = 'neutral' | 'ok' | 'err'

export function draftCard(stage: DraftStage, draft: string): { label: string; tone: DraftTone; line: string } {
  return {
    label: DRAFT_LABEL[stage],
    tone: stage === 'blocked' ? 'err' : stage === 'ready' ? 'ok' : 'neutral',
    // Dòng đầu của bản nháp, gộp khoảng trắng — thẻ đáy chỉ có một dòng.
    line: stage === 'none' ? '' : draft.split(/\r?\n/).map((row) => row.trim()).filter(Boolean)[0] ?? '',
  }
}

/** Nút nhỏ “Bắt đầu” ở thẻ đáy: chỉ mời bấm khi bản nháp sẵn sàng và không bị khoá. */
export function draftStartable(stage: DraftStage, writable: boolean): boolean {
  return writable && stage === 'ready'
}
