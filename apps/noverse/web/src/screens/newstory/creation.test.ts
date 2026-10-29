import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createStory, sendInput } from '../../api/client'
import { quickStoryStarter } from './creation'
import type { StorySummary } from '../../api/types'

vi.mock('../../api/client', () => ({ createStory: vi.fn(), sendInput: vi.fn() }))
const story = { story: '01-truyen-moi' } as StorySummary
beforeEach(() => { vi.resetAllMocks(); vi.mocked(createStory).mockResolvedValue(story); vi.mocked(sendInput).mockResolvedValue({ ok: true, action: 'start' }) })

describe('E2ELIB-2 quick start', () => {
  it('reuses the created folder when input fails and the same request is retried', async () => {
    const starter = quickStoryStarter()
    vi.mocked(sendInput).mockRejectedValueOnce(new Error('Nhà cung cấp chưa sẵn sàng'))
    await expect(starter.start('p', 'Viết truyện đô thị trinh thám')).rejects.toThrow('Nhà cung cấp')
    expect(starter.reusable('p', ' Viết truyện đô thị trinh thám ')).toBe(story.story)
    await expect(starter.start('p', 'Viết truyện đô thị trinh thám')).resolves.toBe(story)
    expect(createStory).toHaveBeenCalledTimes(1)
    expect(sendInput).toHaveBeenCalledTimes(2)
    expect(starter.reusable('p', 'Một yêu cầu khác')).toBeNull()
    await starter.start('p', 'Một yêu cầu khác')
    expect(createStory).toHaveBeenCalledTimes(2)
  })
  it('never sends the prompt as the story name — the prompt only goes to /input', async () => {
    const starter = quickStoryStarter()
    const prompt = 'Rules kịch bản: “Khi sếp học Vibecode” — viết kịch bản audio dài 8 chương'
    await starter.start('p', prompt)
    expect(createStory).toHaveBeenCalledWith('p', { idea: undefined })
    expect(createStory).not.toHaveBeenCalledWith('p', expect.objectContaining({ name: expect.anything() }))
    expect(sendInput).toHaveBeenCalledWith('p', story.story, prompt)
  })
})
