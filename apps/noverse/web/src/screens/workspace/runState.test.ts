// Truyện đã hoàn thành trên host thật (dit-dang-audio/01-tra-xanh-cuop-kich-ban):
// engine ghi `phase=complete` vào tiến độ nhưng worker vừa khởi động lại nên
// `runtimeState=idle`, `statusLabel=COMPLETE`, `advanceMode=auto`.
// Các test dưới đây chốt: nhận đúng trạng thái hoàn thành, không in mã enum thô,
// và Composer dùng placeholder “Hoàn thành” chứ không phải “bị gián đoạn”.
import { describe, expect, it } from 'vitest'
import type { ChapterInfo, Outline, OutlineChapter, ParagraphComment, Snapshot, User } from '../../api/types'
import { t } from '../../i18n/vi'
import { composerPlaceholder } from './Composer'
import { isCompleted, isGate, latestGateChapter, pendingCommentCount } from './GateCard'
import { runChips, statusChip, statusText, statusTone } from './RunHeader'
import { chapterFooter } from './OutlineRail'
import { showReviewDot } from '../../mobile/logic'

const snapshot = (patch: Partial<Snapshot> = {}): Snapshot => ({
  provider: '', bookTitle: '', modelName: '', modelContextWindow: 0, thinkingLevel: '', style: '',
  runtimeState: 'idle', statusLabel: '', phase: '', flow: '',
  waitingFor: '',
  currentChapter: 0, totalChapters: 0, completedCount: 0, totalWordCount: 0,
  inProgressChapter: 0, pendingRewrites: null, rewriteReason: '', pendingSteer: '',
  advanceMode: 'auto', advancePermitChapter: 0, hasAdvanceHold: false, advanceHoldReason: '',
  recoveryLabel: '', isRunning: false, agents: null,
  totalInputTokens: 0, totalOutputTokens: 0, totalCacheReadTokens: 0, totalCacheWriteTokens: 0,
  totalCostUSD: 0, totalSavedUSD: 0, budgetLimitUSD: 0,
  overallCacheCapable: false, overallRecentCacheRead: 0, overallRecentInput: 0, overallRecentSamples: 0,
  totalCacheBreaks: 0, missingAssistantUsage: 0,
  cachePerAgent: null, cachePerModel: null,
  synopsis: '', premise: '', outline: null, characters: null,
  supportingCount: 0, recentSupporting: null, layered: false,
  currentVolumeArc: '', nextVolumeTitle: '', compassDirection: '', compassScale: '',
  lastCommitSummary: '', lastReviewSummary: '', lastCheckpointName: '', recentSummaries: null,
  ...patch,
})

const done = snapshot({
  runtimeState: 'idle', statusLabel: 'COMPLETE', phase: 'complete', flow: 'writing',
  currentChapter: 8, totalChapters: 8, completedCount: 8,
})

describe('truyện đã hoàn thành (phase=complete, runtimeState=idle)', () => {
  it('isCompleted nhận phase=complete làm dấu bền, không chỉ runtimeState', () => {
    expect(isCompleted(done)).toBe(true)
    expect(isCompleted(snapshot({ runtimeState: 'completed', phase: 'writing' }))).toBe(true)
    expect(isCompleted(snapshot({ runtimeState: 'idle', phase: 'writing', completedCount: 8 }))).toBe(false)
    expect(isCompleted(null)).toBe(false)
  })

  it('RunHeader dịch mã enum, không in COMPLETE/complete/auto thô', () => {
    expect(statusText(done.statusLabel, done.runtimeState)).toBe(t.status.completed)
    expect(statusText('', 'running')).toBe(t.status.running)
    expect(statusText('写第 6 章', 'running')).toBe('Viết chương 6')
    expect(runChips(done)).toEqual({ phase: t.phase.complete, phaseColor: '', flow: t.flow.writing, flowColor: 'var(--color-acc-soft)', mode: t.advanceMode.auto, modeColor: '' })
    expect(statusTone(done, false, false)).toBe('ok')
    expect(statusTone(done, true, false)).toBe('accent')
    expect(statusTone(snapshot({ runtimeState: 'paused' }), false, true)).toBe('warn')
  })

  it('Composer dùng placeholder “Hoàn thành” ở cả desktop lẫn mobile', () => {
    expect(composerPlaceholder(done, false, false)).toBe(t.placeholder.completed)
    expect(composerPlaceholder(done, false, true)).toBe(t.placeholder.completed)
  })

  it('lượt viết bị dừng giữa đường vẫn dùng placeholder “bị gián đoạn”', () => {
    const interrupted = snapshot({ phase: 'writing', flow: 'writing', completedCount: 8, currentChapter: 8 })
    expect(composerPlaceholder(interrupted, false, false)).toBe(t.placeholder.interrupted)
  })

  it('không hiện cổng nghiệm thu cho truyện đã hoàn thành', () => {
    expect(isGate(snapshot({ ...done, advanceMode: 'review', advancePermitChapter: 0 }))).toBe(false)
  })
  it('chấm Đọc chỉ hiện cổng duyệt thật có quyền; complete + input không phải cổng', () => {
    const writer: User = { username: 'dat', name: 'Đạt', role: 'member', projects: { 'dit-dang-audio': 'collab' }, disabled: false, createdAt: '', updatedAt: '', onboardedAt: null, mustChangePassword: false, lastSeenAt: null }
    const storyId = 'dit-dang-audio/01-tra-xanh-cuop-kich-ban'
    const completedInput = snapshot({ ...done, waitingFor: 'input', isRunning: false })
    expect(showReviewDot(completedInput, writer, storyId)).toBe(false)
    expect(showReviewDot({ ...completedInput, advanceMode: 'review', advancePermitChapter: 0 }, writer, storyId)).toBe(false)
    const gate = snapshot({ advanceMode: 'review', completedCount: 3, currentChapter: 4, advancePermitChapter: 3 })
    expect(showReviewDot(gate, writer, storyId)).toBe(true)
    expect(showReviewDot(gate, { ...writer, role: 'admin', projects: {} }, storyId)).toBe(true)
    expect(showReviewDot(gate, { ...writer, projects: { 'dit-dang-audio': 'view' } }, storyId)).toBe(false)
    expect(showReviewDot(gate, { ...writer, projects: {} }, storyId)).toBe(false)
    expect(showReviewDot(gate, null, storyId)).toBe(false)
    expect(showReviewDot(null, writer, storyId)).toBe(false)
    expect(showReviewDot({ ...gate, isRunning: true }, writer, storyId)).toBe(false)
    expect(showReviewDot({ ...gate, advanceMode: 'auto' }, writer, storyId)).toBe(false)
  })
})

// G5 (19-gap-closure.md): StatusChip đọc runtimeState + waitingFor.
describe('StatusChip theo waitingFor', () => {
  it('đang chạy → “Đang chạy”, kể cả khi statusLabel còn tiếng Trung', () => {
    expect(statusChip(snapshot({ isRunning: true, runtimeState: 'running', statusLabel: '写第 6 章' }))).toBe(t.status.running)
  })

  it('waitingFor phiên (cocreate/confirm) → “Đang chờ bạn” và chip màu cam', () => {
    for (const waitingFor of ['cocreate', 'confirm']) {
      const state = snapshot({ waitingFor, isRunning: false, runtimeState: 'idle', statusLabel: '就绪' })
      expect(statusChip(state)).toBe(t.workspace.waiting)
      expect(statusTone(state, false, false)).toBe('warn')
    }
  })

  it('waitingFor=input vẫn đọc nhãn host (“Sẵn sàng”, “Hoàn thành”, “Đang chờ bạn”)', () => {
    expect(statusChip(snapshot({ waitingFor: 'input', isRunning: false, runtimeState: 'idle', statusLabel: '就绪' }))).toBe('Sẵn sàng')
    expect(statusChip(snapshot({ waitingFor: 'input', isRunning: false, runtimeState: 'idle', statusLabel: 'WAITING' }))).toBe(t.workspace.waiting)
  })

  it('cổng duyệt giữ nhãn host “Thẩm duyệt” (Main.dc.html phase=gate), vẫn màu cam', () => {
    const gate = snapshot({ waitingFor: 'gate', isRunning: false, runtimeState: 'paused', statusLabel: '审阅' })
    expect(statusChip(gate)).toBe('Thẩm duyệt')
    expect(statusTone(gate, false, false)).toBe('warn')
    const gateRunning = snapshot({ waitingFor: 'gate', isRunning: true, runtimeState: 'running', statusLabel: '审阅' })
    expect(statusChip(gateRunning)).toBe('Thẩm duyệt')
  })

  it('rỗng → giữ nhãn cũ (dịch enum/Chinese, tạm dừng, hoàn thành)', () => {
    expect(statusChip(done)).toBe(t.status.completed)
    expect(statusChip(snapshot({ runtimeState: 'paused', statusLabel: '暂停' }))).toBe('Tạm dừng')
    expect(statusChip(null)).toBe('')
  })
})

// 22-brief-2510 › 27/10: nền chip trạng thái theo Kit — --tint-* 10%, viền cùng màu 30%.
describe('StatusChip theo tông artboard Main (27/10)', () => {
  it('rảnh / sẵn sàng → tím (accent), thẩm duyệt và chờ phiên → cam, lỗi → đỏ', () => {
    expect(statusTone(snapshot({ runtimeState: 'idle', statusLabel: 'READY' }), false, false)).toBe('accent')
    expect(statusTone(snapshot({ waitingFor: 'input', runtimeState: 'idle', statusLabel: '就绪' }), false, false)).toBe('accent')
    expect(statusTone(snapshot({ runtimeState: 'idle', statusLabel: 'REVIEW' }), false, false)).toBe('warn')
    expect(statusTone(snapshot({ runtimeState: 'idle', statusLabel: 'ERROR' }), false, false)).toBe('err')
    expect(statusTone(snapshot({ runtimeState: 'paused', statusLabel: 'PAUSED' }), false, false)).toBe('warn')
    expect(statusTone(snapshot({ runtimeState: 'idle', statusLabel: 'REWRITE' }), false, false)).toBe('neutral')
  })

  it('waitingFor khác phiên/cổng mà nhãn dịch ra “Đang chờ bạn” → cam', () => {
    const waiting = snapshot({ waitingFor: 'input', isRunning: false, runtimeState: 'idle', statusLabel: 'WAITING' })
    expect(statusChip(waiting)).toBe(t.workspace.waiting)
    expect(statusTone(waiting, false, false)).toBe('warn')
  })
})

describe('góp ý tại cổng duyệt', () => {
  it('chỉ đếm góp ý chưa giao cho Người viết', () => {
    const comment = (appliedAt?: string): ParagraphComment => ({
      id: 'c', paragraph: 0, quote: '', chips: [], text: 'Thêm thoại',
      author: 'linh', createdAt: '', appliedAt,
    })
    expect(pendingCommentCount([comment(), comment(''), comment('2026-10-25T10:00:00Z')])).toBe(2)
    expect(pendingCommentCount([])).toBe(0)
  })

  it('lấy số chương hoàn thành lớn nhất thay vì số lượng khi chương thưa', () => {
    const chapter = (number: number, completed: boolean): OutlineChapter => ({
      chapter: number, title: '', coreEvent: '', summary: '', words: 0, completed,
      status: completed ? 'done' : 'pending', hasPlan: false, hasReview: false,
    })
    const outline: Outline = {
      layered: false, path: '', nextVolumeTitle: '', compassDirection: '', compassScale: '',
      volumes: [{ index: 1, title: '', summary: '', arcs: [],
        chapters: [chapter(2, true), chapter(11, true), chapter(12, false)] }],
    }
    expect(latestGateChapter(snapshot({ completedCount: 2 }), outline)).toBe(11)
  })

  it('dùng snapshot trong lúc chưa tải xong đại cương', () => {
    expect(latestGateChapter(snapshot({ completedCount: 5 }), null)).toBe(5)
    expect(latestGateChapter(null, null)).toBe(0)
  })
})

describe('đại cương khi worker chưa khởi động được', () => {
  const chapters: ChapterInfo[] = [
    { chapter: 9, title: '', completed: true, words: 20000, hasDraft: false, volume: 1, arc: 1 },
    { chapter: 2, title: '', completed: true, words: 40000, hasDraft: false, volume: 1, arc: 1 },
    { chapter: 10, title: '', completed: false, words: 1693, hasDraft: true, volume: 1, arc: 1 },
  ]

  it('giữ tổng chữ và chọn bản nộp cao nhất từ danh sách chương', () => {
    expect(chapterFooter(null, chapters)).toEqual({ words: 61693, latest: 'Chương 9 · 20.000 chữ' })
  })

  it('ưu tiên snapshot khi đã có, phân biệt chưa tải và danh sách rỗng', () => {
    expect(chapterFooter(snapshot({ totalWordCount: 7, lastCommitSummary: 'Đã nộp chương 12' }), chapters))
      .toEqual({ words: 7, latest: 'Đã nộp chương 12' })
    expect(chapterFooter(null)).toEqual({ words: null, latest: '' })
    expect(chapterFooter(null, [])).toEqual({ words: 0, latest: '' })
  })
})
