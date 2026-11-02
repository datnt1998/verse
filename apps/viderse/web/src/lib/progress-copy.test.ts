import { describe, expect, it } from 'vitest'
import { progressState, progressText, type ProgressStep } from './progress-copy'

const step = (over: Partial<ProgressStep> = {}): ProgressStep => ({
  step: over.step ?? 'transcript',
  status: over.status ?? 'running',
  progress: over.progress ?? { unitsDone: 0, unitsTotal: null, phase: 'vad' },
  error: over.error ?? null,
})

describe('Nhật ký tiến độ — câu tiếng Việt', () => {
  it('dịch mã pha cũ của máy chủ, không lộ mã ra câu', () => {
    const text = progressText(step())
    expect(text).toContain('Đang')
    expect(text).not.toContain('vad')
    expect(progressText(step({ progress: { unitsDone: 0, unitsTotal: null, phase: 'rendering-again' } }))).not.toContain('rendering')
  })

  it('cùng một mã pha mang nghĩa theo bước', () => {
    const download = progressText(step({ step: 'download', progress: { unitsDone: 1, unitsTotal: 2, phase: 'publish' } }))
    const transcript = progressText(step({ step: 'transcript', progress: { unitsDone: 1, unitsTotal: 2, phase: 'publish' } }))
    expect(download).not.toBe(transcript)
    expect(download).toContain('nguồn')
    expect(transcript).toContain('lời kể')
  })

  it('mã lạ không hiện trần: dùng nhãn bước đang chạy', () => {
    const text = progressText(step({ progress: { unitsDone: 0, unitsTotal: null, phase: 'brand_new_token' } }))
    expect(text).toContain('Đang')
    expect(text).not.toContain('brand_new_token')
    expect(text).toContain('lời kể')
    expect(progressText(step({ step: 'mystery', progress: { unitsDone: 0, unitsTotal: null, phase: 'whatever' } }))).not.toContain('whatever')
  })

  it('pha đã viết tiếng Việt nhưng không có trong bảng mã vẫn rơi về nhãn bước', () => {
    const text = progressText(step({ progress: { unitsDone: 0, unitsTotal: null, phase: 'Đang nghe lời kể' } }))
    expect(text).toBe('Đang lấy lời kể')
    const accented = progressText(step({ progress: { unitsDone: 0, unitsTotal: null, phase: 'Café rendering déjà vu' } }))
    expect(accented).not.toContain('Café')
    expect(accented).toContain('Đang')
  })

  it('không hiện error.message thô kể cả khi máy chủ cũ để lại tiếng Anh', () => {
    const message = 'Yt-dlp exited with code 1'
    const failed = step({ status: 'failed', error: { code: 'source_unavailable', message } })
    const text = progressText(failed)
    expect(text).not.toContain(message)
  })

  it('lỗi có mã đã biết thì câu Việt theo mã, không hiện mã', () => {
    const text = progressText(step({ status: 'failed', error: { code: 'tool_missing', message: 'missing ffmpeg' } }))
    expect(text).not.toContain('tool_missing')
    expect(text).not.toContain('missing ffmpeg')
  })

  it('lỗi mã lạ cũng chỉ ra câu lỗi chung tiếng Việt', () => {
    const text = progressText(step({ status: 'failed', error: { code: 'not_a_real_code', message: 'boom' } }))
    expect(text).not.toContain('not_a_real_code')
    expect(text).not.toContain('boom')
  })

  it('bước chưa chạy hoặc đã xong đọc bằng nhãn bước', () => {
    expect(progressText(step({ status: 'pending', progress: { unitsDone: 0, unitsTotal: null, phase: '' } }))).toContain('lời kể')
    expect(progressText(step({ status: 'succeeded', progress: { unitsDone: 1, unitsTotal: 1, phase: 'asr' } }))).toContain('lời kể')
  })
})

describe('Nhật ký tiến độ — nhãn trạng thái', () => {
  it('mọi trạng thái bước thật đều có nhãn Việt, khác mã', () => {
    for (const status of ['pending', 'queued', 'running', 'waiting', 'succeeded', 'done', 'failed', 'cancelled', 'skipped', 'stale']) {
      const label = progressState(status)
      expect(label).not.toBe(status)
    }
  })

  it('trạng thái lạ trả rỗng thay vì mã tiếng Anh', () => {
    expect(progressState('unknown_state')).toBe('')
    expect(progressState('')).toBe('')
  })
})
