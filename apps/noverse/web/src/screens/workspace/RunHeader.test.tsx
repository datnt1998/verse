// 27/10 — người dùng báo: sáng tác dừng vì lỗi thì không có nút “Tiếp tục” nào trên
// Workspace, chỉ còn lời nhắc gõ lệnh. Nút phải hiện cho mọi trạng thái dừng còn điểm
// khôi phục (host phát `recoveryLabel`), không chỉ `runtimeState=paused`.
import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it } from 'vitest'
import type { Snapshot } from '../../api/types'
import { canResume, RunHeader } from './RunHeader'

const snapshot = (patch: Partial<Snapshot> = {}): Snapshot => ({
  provider: '', bookTitle: 'Trà Xanh Cướp Kịch Bản', modelName: '', modelContextWindow: 0,
  thinkingLevel: '', style: '', runtimeState: 'idle', statusLabel: '', phase: 'writing', flow: '',
  waitingFor: '', currentChapter: 5, totalChapters: 8, completedCount: 4, totalWordCount: 0,
  inProgressChapter: 0, pendingRewrites: null, rewriteReason: '', pendingSteer: '',
  advanceMode: 'auto', advancePermitChapter: 0, hasAdvanceHold: false, advanceHoldReason: '',
  recoveryLabel: '', isRunning: false, agents: null,
  totalInputTokens: 0, totalOutputTokens: 0, totalCacheReadTokens: 0, totalCacheWriteTokens: 0,
  totalCostUSD: 0, totalSavedUSD: 0, budgetLimitUSD: 0,
  overallCacheCapable: false, overallRecentCacheRead: 0, overallRecentInput: 0, overallRecentSamples: 0,
  totalCacheBreaks: 0, missingAssistantUsage: 0, cachePerAgent: null, cachePerModel: null,
  synopsis: '', premise: '', outline: null, characters: null,
  supportingCount: 0, recentSupporting: null, layered: false,
  currentVolumeArc: '', nextVolumeTitle: '', compassDirection: '', compassScale: '',
  lastCommitSummary: '', lastReviewSummary: '', lastCheckpointName: '', recentSummaries: null,
  ...patch,
})

const header = (state: Snapshot, paused = state.runtimeState === 'paused') => renderToStaticMarkup(
  <RunHeader snapshot={state} story="tra-xanh" running={state.isRunning} paused={paused} writable
    onPause={() => {}} onResume={() => {}} onExport={() => {}} onReopen={() => {}} onToggleReview={() => {}} />,
)

const RESUME = '>Tiếp tục<'

describe('nút Tiếp tục cho trạng thái dừng (27/10)', () => {
  it('lỗi/error có điểm khôi phục → có nút Tiếp tục', () => {
    const stopped = snapshot({ runtimeState: 'error', statusLabel: 'error', recoveryLabel: 'Khôi phục: chương 5 đang viết' })
    expect(canResume(stopped, false, false)).toBe(true)
    expect(header(stopped, false)).toContain(RESUME)
  })

  it('host hạ về idle sau lỗi (phase=writing + recoveryLabel) → vẫn có nút Tiếp tục', () => {
    const stopped = snapshot({ runtimeState: 'idle', statusLabel: 'READY', recoveryLabel: 'Khôi phục: chương 5 đang viết' })
    expect(header(stopped, false)).toContain(RESUME)
  })

  it('tạm dừng sau lỗi → vẫn có nút Tiếp tục', () => {
    const stopped = snapshot({ runtimeState: 'paused', statusLabel: '暂停', recoveryLabel: 'Khôi phục: chương 5 đang viết' })
    expect(header(stopped, true)).toContain(RESUME)
  })

  it('đang chạy, đã xong, cổng duyệt hoặc không có điểm khôi phục → không mượn nút này', () => {
    expect(canResume(snapshot({ isRunning: true, runtimeState: 'running', recoveryLabel: 'Khôi phục: chương 5 đang viết' }), true, false)).toBe(false)
    expect(canResume(snapshot({ runtimeState: 'completed', phase: 'complete', recoveryLabel: '' }), false, false)).toBe(false)
    expect(canResume(snapshot({ advanceMode: 'review', advancePermitChapter: 3, completedCount: 4, recoveryLabel: 'Khôi phục: chương 5 đang viết' }), false, false)).toBe(false)
    expect(canResume(snapshot({ recoveryLabel: '' }), false, false)).toBe(false)
    expect(header(snapshot({ recoveryLabel: '' }), false)).not.toContain(RESUME)
  })
})
