import { expect, it } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import type { HostEvent } from '../../api/types'
import { ActivityFeed } from './ActivityFeed'

it('hides internal judgments and historical judgment bubbles while preserving public answers and user quotes', () => {
  const event = (id: string, summary: string, kind = '', category = 'SYSTEM'): HostEvent => ({
    id, summary, kind, category, time: '2026-10-28T22:00:00Z', finishedAt: '', failed: false,
    agent: '', level: 'info', depth: 0, durationNs: 0, retryAt: '', sessionId: '', canRetryNow: false,
  })
  const html = renderToStaticMarkup(<ActivityFeed project="p" story="s" events={[
    event('internal', 'private reasoning', 'internal'),
    event('legacy', 'Phán định: Người dùng hỏi khả năng xử lý lỗi'),
    event('old', '裁定: private legacy reasoning'),
    event('answer', 'Em sẽ kiểm tra cảnh báo.'),
    event('user', 'Phán định: lời tôi trích', 'user', 'USER'),
  ]} />)
  expect(html).not.toContain('private reasoning')
  expect(html).not.toContain('Người dùng hỏi khả năng xử lý lỗi')
  expect(html).not.toContain('private legacy reasoning')
  expect(html).toContain('Em sẽ kiểm tra cảnh báo.')
  expect(html).toContain('Phán định: lời tôi trích')
})
