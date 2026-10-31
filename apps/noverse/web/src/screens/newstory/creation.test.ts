import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createStory, putConfig, sendInput } from '../../api/client'
import { quickStoryConfig, quickStoryStarter } from './creation'
import type { ConfigResponse, StorySummary } from '../../api/types'

vi.mock('../../api/client', () => ({ createStory: vi.fn(), putConfig: vi.fn(), sendInput: vi.fn() }))
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
  it('saves story overrides after creation and waits for them before starting', async () => {
    const starter = quickStoryStarter()
    // ES2022 project target does not provide Promise.withResolvers.
    let finishSave!: (value: ConfigResponse) => void
    const saving = new Promise<ConfigResponse>(resolve => { finishSave = resolve })
    vi.mocked(putConfig).mockReturnValue(saving)
    const overrides = { style: 'suspense', roles: { writer: { provider: 'openai', model: 'gpt-6-luna' } } }
    const starting = starter.start('p', 'Một truyện mới', undefined, overrides)
    await vi.waitFor(() => expect(putConfig).toHaveBeenCalledWith('story', overrides, 'p', story.story))
    expect(vi.mocked(createStory).mock.invocationCallOrder[0]).toBeLessThan(vi.mocked(putConfig).mock.invocationCallOrder[0])
    expect(sendInput).not.toHaveBeenCalled()
    finishSave({} as ConfigResponse)
    await starting
    expect(sendInput).toHaveBeenCalledWith('p', story.story, 'Một truyện mới')
  })
  it('does not write overrides when both choices follow the project', async () => {
    const project = { style: 'romance', provider: 'openai', model: 'gpt-6.1-sol' }
    const config = quickStoryConfig({ style: null, writer: null }, project)
    expect(config).toEqual({})
    await quickStoryStarter().start('p', 'Một truyện mới', undefined, config)
    expect(putConfig).not.toHaveBeenCalled()
    expect(sendInput).toHaveBeenCalledOnce()
  })
  it('writes only choices that differ from the project, including the provider identity', () => {
    const project = { style: 'romance', roles: { default: { provider: 'openai', model: 'gpt-6.1-sol' } } }
    expect(quickStoryConfig({ style: 'romance', writer: { provider: 'openai', model: 'gpt-6.1-sol' } }, project)).toEqual({})
    expect(quickStoryConfig({ style: 'suspense', writer: null }, project)).toEqual({ style: 'suspense' })
    expect(quickStoryConfig({ style: null, writer: { provider: 'another', model: 'gpt-6.1-sol' } }, project))
      .toEqual({ roles: { writer: { provider: 'another', model: 'gpt-6.1-sol' } } })
  })
  it('uses the project writer role before its default model when deciding inheritance', () => {
    const project = { roles: {
      default: { provider: 'openai', model: 'gpt-6.1-sol' },
      writer: { provider: 'anthropic', model: 'claude-sonnet-4' },
    } }
    expect(quickStoryConfig({ style: null, writer: project.roles.writer }, project)).toEqual({})
    expect(quickStoryConfig({ style: null, writer: project.roles.default }, project)).toEqual({ roles: { writer: project.roles.default } })
  })
  it('does not start after a config failure and retries saving on the same story', async () => {
    const starter = quickStoryStarter()
    const overrides = { style: 'suspense' }
    vi.mocked(putConfig).mockRejectedValueOnce(new Error('Mất kết nối')).mockResolvedValue({} as ConfigResponse)
    await expect(starter.start('p', 'Một truyện mới', undefined, overrides)).rejects.toThrow('Chưa lưu được')
    expect(sendInput).not.toHaveBeenCalled()
    await starter.start('p', 'Một truyện mới', undefined, overrides)
    expect(createStory).toHaveBeenCalledOnce()
    expect(putConfig).toHaveBeenCalledTimes(2)
    expect(sendInput).toHaveBeenCalledOnce()
  })
  it('clears previously saved overrides when a failed start is retried following the project', async () => {
    const starter = quickStoryStarter()
    vi.mocked(sendInput).mockRejectedValueOnce(new Error('Chưa sẵn sàng'))
    await expect(starter.start('p', 'Một truyện mới', undefined, { style: 'suspense' })).rejects.toThrow()
    await starter.start('p', 'Một truyện mới', undefined, {})
    expect(putConfig).toHaveBeenLastCalledWith('story', {}, 'p', story.story)
    expect(createStory).toHaveBeenCalledOnce()
  })
  it('resets an uncertain failed config write before retrying with project defaults', async () => {
    const starter = quickStoryStarter()
    vi.mocked(putConfig).mockRejectedValueOnce(new Error('Mất phản hồi')).mockResolvedValue({} as ConfigResponse)
    await expect(starter.start('p', 'Một truyện mới', undefined, { style: 'suspense' })).rejects.toThrow()
    await starter.start('p', 'Một truyện mới', undefined, {})
    expect(putConfig).toHaveBeenLastCalledWith('story', {}, 'p', story.story)
    expect(sendInput).toHaveBeenCalledOnce()
  })
})
