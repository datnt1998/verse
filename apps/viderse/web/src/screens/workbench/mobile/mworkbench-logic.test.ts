import { describe, expect, it } from 'vitest'
import type { ChannelSummary } from '../../../api/workbenchChannels'
import type { JobImage } from '../../../api/workbench'
import { dayProgress, imageLabel, imagePrompt, imageUrl, imageVersion, scopeName, stageToneOf } from './mworkbench-logic'

const channel = (over: Partial<ChannelSummary> & Pick<ChannelSummary, 'channelId'>): ChannelSummary => ({
  name: over.channelId, initials: over.channelId.slice(0, 2), color: 'var(--color-fg)', today: { done: 0, cap: 0 },
  waitingYou: 0, ready: 0, postedThisWeek: 0, autoFetch: false, ...over,
})

describe('Bàn dựng điện thoại', () => {
  it('cộng tiến độ ngày của mọi kênh và của một kênh', () => {
    const channels = [
      channel({ channelId: 'a', today: { done: 3, cap: 5 }, autoFetch: true, waitingYou: 2 }),
      channel({ channelId: 'b', today: { done: 1, cap: 4 }, autoFetch: false }),
    ]
    expect(dayProgress(channels, 'all')).toMatchObject({ done: 4, cap: 9, autoOn: 1, autoTotal: 2, pct: 4 / 9 })
    expect(dayProgress(channels, 'a')).toMatchObject({ done: 3, cap: 5, label: '3/5 video hôm nay', autoLine: 'Tự lấy video bật' })
    expect(dayProgress(channels, 'a').autoLine).toContain('bật')
    expect(dayProgress(channels, 'b').autoLine).toContain('tắt')
    expect(dayProgress(channels, 'missing').label).toBe('0/0 video hôm nay')
  })

  it('giữ đúng tên phạm vi', () => {
    const channels = [channel({ channelId: 'a', name: 'Kênh A' })]
    expect(scopeName(channels, 'all')).toBe('Mọi kênh')
    expect(scopeName(channels, 'a')).toBe('Kênh A')
    expect(scopeName(channels, 'gone')).toBe('gone')
  })

  it('chọn phiên bản ảnh: ưu tiên bản đang chọn, rồi bản duyệt, rồi bản hiện tại', () => {
    const image: JobImage = {
      id: 'img', kind: 'scene', sceneId: '2', narration: '', status: 'done', currentVersion: 2, reviewVersion: 3,
      versions: [
        { version: 1, originalUrl: 'u1' },
        { version: 2, originalUrl: 'u2' },
        { version: 3, originalUrl: 'u3' },
      ],
    }
    expect(imageVersion(image)).toBe(3)
    expect(imageVersion(image, 1)).toBe(1)
    expect(imageVersion(image, 9)).toBe(3)
    expect(imageUrl(image)).toBe('u3')
    expect(imageUrl(image, 2)).toBe('u2')
    expect(imageLabel(image, 0)).toBe('Cảnh 2')
    expect(imageLabel({ ...image, kind: 'cover', sceneId: undefined }, 0)).toBe('Ảnh bìa')
    expect(imageLabel({ ...image, sceneId: 'sc-f90' }, 1)).toBe('Cảnh 2')
  })

  it('đọc đúng prompt đã lưu theo phiên bản, không đoán prompt của ảnh lịch sử', () => {
    const image: JobImage = {
      id: 'scene-1', kind: 'scene', status: 'ready', currentVersion: 3, reviewVersion: 2,
      prompt: 'Prompt hiện tại',
      versions: [{ version: 1 }, { version: 2, prompt: 'Prompt đã dùng cho bản 2' }, { version: 3 }],
    }
    expect(imagePrompt(image)).toBe('Prompt đã dùng cho bản 2')
    expect(imagePrompt(image, 3)).toBe('Prompt hiện tại')
    expect(imagePrompt(image, 1)).toBeUndefined()
    expect(imagePrompt({ ...image, prompt: undefined }, 3)).toBeUndefined()
  })

  it('nhãn trạng thái theo nhóm video', () => {
    expect(stageToneOf('attention')).toBe('review')
    expect(stageToneOf('working')).toBe('working')
    expect(stageToneOf('ready')).toBe('ready')
  })
})
