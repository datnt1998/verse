// Dòng phụ template lấy từ TemplateSummary thật: câu tóm tắt recipe do backend
// soạn ("Prompt … · Tách nền · bìa …") + số ảnh; không dùng tên mục tab.
import { describe, expect, it } from 'vitest'
import type { TemplateSummary } from '../../api/library'
import { templateLine } from './templateLine'

const summary = (patch: Partial<TemplateSummary> = {}): TemplateSummary => (({
  id: 't1', name: 'Mẫu chuẩn', revision: 1, style: 'cinematic',
  images: { mode: 'auto', count: 6, minutes: 42 },
  prompts: {
    sceneId: 's1', sceneName: 'Chữ hook', coverId: 'c1', coverName: 'Hook đỏ',
    summary: 'Prompt Chữ hook · Tách nền · bìa Hook đỏ',
  },
  backdrop: { channelCount: 1, videoCount: 30, clipCount: 30, clipSec: 8, enabled: true },
  overlayCount: 0, channels: [], preview: { state: 'ready', builtAt: null },
  createdAt: '2026-10-20T00:00:00Z', updatedAt: '2026-10-20T00:00:00Z',
  ...patch,
})) as TemplateSummary

describe('dòng phụ template', () => {
  it('dùng câu tóm tắt recipe của backend rồi nối số ảnh', () => {
    expect(templateLine(summary())).toBe('Prompt Chữ hook · Tách nền · bìa Hook đỏ · 6 ảnh')
  })

  it('chưa biết số ảnh (count 0) thì bỏ phần cuối, không bịa "0 ảnh"', () => {
    expect(templateLine(summary({
      images: { mode: 'auto', count: 0, minutes: 0 } as unknown as TemplateSummary['images'],
    }))).toBe('Prompt Chữ hook · Tách nền · bìa Hook đỏ')
  })
})
