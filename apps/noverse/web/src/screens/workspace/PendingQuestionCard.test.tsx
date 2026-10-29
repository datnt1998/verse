// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, expect, it, vi } from 'vitest'
import type { Snapshot } from '../../api/types'
import { PendingQuestionCard } from './IdleCard'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
let root: Root | null = null

afterEach(async () => {
  await act(async () => { root?.unmount() })
  root = null
  document.body.replaceChildren()
})

it.each([false, true])('question choices send their full reply through the existing input path (mobile=%s)', async (mobile) => {
  const onReply = vi.fn()
  const host = document.createElement('div')
  document.body.append(host)
  root = createRoot(host)
  await act(async () => {
    root?.render(<PendingQuestionCard mobile={mobile} snapshot={{ completedCount: 2 } as Snapshot}
      model={{ phase: 'wait', question: '**Cần chọn hướng đi** trước khi gọi `save_arc_summary`.', chapter: 2,
        options: [{ label: 'Tóm tắt', reply: 'Tóm tắt cung hiện tại theo nhiệm vụ đang làm', isDefault: true },
          { label: 'Kiểm tra lại', reply: 'Đọc lại chương 2 trước khi tóm tắt', isDefault: false }] }}
      onReply={onReply} onRead={vi.fn()} />)
  })
  const choice = [...host.querySelectorAll('button')].find((button) => button.textContent === 'Tóm tắt')!
  await act(async () => { choice.click() })
  expect(onReply).toHaveBeenCalledWith('Tóm tắt cung hiện tại theo nhiệm vụ đang làm')
  expect(host.querySelector('strong')?.textContent).toBe('Cần chọn hướng đi')
  expect(host.querySelector('code')?.textContent).toBe('Tóm tắt cung')
  expect(host.textContent).not.toContain('save_arc_summary')
})
