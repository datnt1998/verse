// Gói AR · việc 6 — dán văn mẫu trước khi tạo truyện (M11 `sheet=samples` dùng chung với máy tính).
// Nghiệm thu: 2 đoạn → createStory một lần, 2 POST style-samples, rồi startSimulate;
// hỏng ở POST thứ hai rồi thử lại → không tạo truyện thứ hai.
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createStory, createStyleSample, startSimulate } from '../../api/client'
import type { StorySummary } from '../../api/types'
import { normalizeSamples, sampleWords, simulateStarter, splitSampleText } from './simulateStart'

vi.mock('../../api/client', () => ({ createStory: vi.fn(), createStyleSample: vi.fn(), startSimulate: vi.fn() }))
vi.mock('../../data', () => ({
  invalidate: vi.fn(),
  keys: { project: () => ['project'], library: () => ['library'], jobs: () => ['jobs'], story: () => ['story'] },
}))

const story = { story: '01-truyen-moi' } as StorySummary
const A = 'Mẹ tôi khâu áo bằng tay trái. Tôi chưa bao giờ hỏi vì sao.'
const B = 'Đêm bà mất, cuốn sổ ghi nợ vẫn nằm dưới gối.'

beforeEach(() => {
  vi.resetAllMocks()
  vi.mocked(createStory).mockResolvedValue(story)
  vi.mocked(createStyleSample).mockResolvedValue({ id: 'van-mau.txt', title: 'Văn mẫu', chars: 12, addedAt: '' })
  vi.mocked(startSimulate).mockResolvedValue({ job: 'simulate/1' })
})

describe('AR · mô phỏng văn phong khi tạo truyện', () => {
  it('creates one story, posts every sample, then starts the simulation', async () => {
    const phases: string[] = []
    const result = await simulateStarter().start('p', [A, B], (progress) => phases.push(progress.phase))

    expect(result).toEqual({ story: story.story, samples: 2 })
    expect(createStory).toHaveBeenCalledTimes(1)
    expect(createStory).toHaveBeenCalledWith('p', {})
    expect(createStyleSample).toHaveBeenCalledTimes(2)
    expect(vi.mocked(createStyleSample).mock.calls).toEqual([['p', story.story, { text: A }], ['p', story.story, { text: B }]])
    expect(startSimulate).toHaveBeenCalledWith('p', story.story)

    const order = (fn: { mock: { invocationCallOrder: number[] } }) => fn.mock.invocationCallOrder[0]
    expect(order(vi.mocked(createStory))).toBeLessThan(order(vi.mocked(createStyleSample)))
    expect(vi.mocked(createStyleSample).mock.invocationCallOrder[1]).toBeLessThan(order(vi.mocked(startSimulate)))
    expect(phases).toEqual(['creating', 'samples', 'samples', 'samples', 'simulating', 'done'])
  })

  it('refuses an empty list without touching the network', async () => {
    const starter = simulateStarter()
    await expect(starter.start('p', ['   ', '\n\n'])).rejects.toThrow()
    expect(createStory).not.toHaveBeenCalled()
    expect(createStyleSample).not.toHaveBeenCalled()
    expect(startSimulate).not.toHaveBeenCalled()
  })

  it('keeps the story after a failed sample and the retry posts only the missing one', async () => {
    const starter = simulateStarter()
    vi.mocked(createStyleSample).mockResolvedValueOnce({ id: 'a.txt', title: 'Văn mẫu 1', chars: 12, addedAt: '' })
      .mockRejectedValueOnce(new Error('Đã đủ 20 văn mẫu, xoá bớt trước khi thêm'))

    await expect(starter.start('p', [A, B])).rejects.toThrow('Đã đủ 20 văn mẫu')
    expect(starter.story('p')).toBe(story.story)
    expect(startSimulate).not.toHaveBeenCalled()

    vi.mocked(createStyleSample).mockResolvedValue({ id: 'b.txt', title: 'Văn mẫu 2', chars: 10, addedAt: '' })
    await expect(starter.retry('p')).resolves.toEqual({ story: story.story, samples: 2 })

    expect(createStory).toHaveBeenCalledTimes(1)
    expect(vi.mocked(createStyleSample).mock.calls).toEqual([['p', story.story, { text: A }], ['p', story.story, { text: B }], ['p', story.story, { text: B }]])
    expect(startSimulate).toHaveBeenCalledTimes(1)
    expect(startSimulate).toHaveBeenCalledWith('p', story.story)
  })

  it('does not post the same sample twice when the retry repeats a successful run', async () => {
    const starter = simulateStarter()
    vi.mocked(startSimulate).mockRejectedValueOnce(new Error('Chưa sẵn sàng'))
    await expect(starter.start('p', [A, B])).rejects.toThrow('Chưa sẵn sàng')
    await expect(starter.retry('p')).resolves.toEqual({ story: story.story, samples: 2 })
    expect(createStyleSample).toHaveBeenCalledTimes(2)
    expect(createStory).toHaveBeenCalledTimes(1)
    expect(startSimulate).toHaveBeenCalledTimes(2)
  })

  it('creates a new story only for a different project', async () => {
    const starter = simulateStarter()
    await starter.start('p', [A])
    expect(starter.story('khac')).toBeNull()
    await starter.start('khac', [A])
    expect(createStory).toHaveBeenCalledTimes(2)
  })

  it('starts a fresh story for the next simulate flow after a successful one', async () => {
    // Màn Truyện mới được Activity giữ lại: bộ chạy phải quên truyện cũ sau khi xong,
    // nếu không lần dán văn mẫu sau sẽ nhét văn mẫu vào truyện trước.
    const starter = simulateStarter()
    await starter.start('p', [A])
    expect(starter.story('p')).toBeNull()
    await starter.start('p', [A])
    expect(createStory).toHaveBeenCalledTimes(2)
    expect(vi.mocked(createStyleSample).mock.calls).toEqual([['p', story.story, { text: A }], ['p', story.story, { text: A }]])
  })
})

describe('AR · tách đoạn văn mẫu', () => {
  it('splits a pasted file on blank lines and drops empty paragraphs', () => {
    expect(splitSampleText(`${A}\n\n${B}\n\n   \n`)).toEqual([A, B])
    expect(splitSampleText('một dòng\nxuống dòng vẫn là một đoạn')).toEqual(['một dòng\nxuống dòng vẫn là một đoạn'])
  })

  it('keeps at most 20 non-empty paragraphs', () => {
    expect(normalizeSamples(['  ', A, '   ', B])).toEqual([A, B])
    expect(normalizeSamples(Array.from({ length: 25 }, (_, i) => `đoạn ${i}`))).toHaveLength(20)
  })

  it('counts words the way the sample row shows them', () => {
    expect(sampleWords(A)).toBe(14)
    expect(sampleWords('   ')).toBe(0)
  })
})
