// @vitest-environment happy-dom
// §H câu 2 — bản gọn Đồng sáng tác dùng câu ngắn của máy chủ; chỉ khi `short` rỗng mới cắt “…” + `title`.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it } from 'vitest'
import type { LedgerCheckResult } from '../../api/types'
import { StoryCheckBlock } from './StoryCheck'
import type { StoryCheck } from './useStoryCheck'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
let root: Root | null = null
let container: HTMLElement | null = null

const RESULT: LedgerCheckResult = {
  axes: [{ n: 1, name: 'Bộ quy tắc và biến thể' }],
  values: ['A trọng sinh'],
  cells: [],
  checks: [{
    level: 'error', rule: 'combination', axes: [1],
    text: 'Giống “Mẹ Kế Cướp Tiệm May…” ở 4 trục chính: A trọng sinh, Thợ may, Mẹ kế, Sổ sách viết tay. Người nghe sẽ thấy như nghe lại truyện cũ. Đổi ít nhất 1 trong 4 trục.',
    short: 'Giống “Mẹ Kế Cướp Tiệm May…” ở 4 trục chính (bộ, nghề, phản diện, vũ khí).',
    fix: 'Đổi ít nhất 1 trong 4 trục',
  }],
}

const gate = (result: LedgerCheckResult): StoryCheck => ({
  state: { brief: 'brief', phase: 'result', result },
  blocked: true, checking: false, suggesting: false,
  check: async () => false, acceptConflict: () => false, suggest: async () => {}, undo: () => {},
})

async function mount(result: LedgerCheckResult) {
  container = document.createElement('div')
  document.body.append(container)
  root = createRoot(container)
  await act(async () => { root!.render(<StoryCheckBlock project="kenh" gate={gate(result)} compact />) })
  return container
}
afterEach(async () => {
  if (root) await act(() => root!.unmount())
  container?.remove()
  root = null; container = null
})

describe('Kiểm trùng — bản gọn', () => {
  it('uses the server `short` sentence as-is', async () => {
    const el = await mount(RESULT)
    const span = [...el.querySelectorAll('span')].find(node => node.textContent === RESULT.checks[0].short)!
    expect(span).toBeTruthy()
    expect(span.getAttribute('title')).toBeNull()
    expect(el.textContent).not.toContain('Đổi ít nhất 1 trong 4 trục')
  })

  it('falls back to the full text with an ellipsis and a title when `short` is empty', async () => {
    const el = await mount({ ...RESULT, checks: [{ ...RESULT.checks[0], short: '' }] })
    const truncated = [...el.querySelectorAll('span')].find(node => node.getAttribute('title') === RESULT.checks[0].text)!
    expect(truncated).toBeTruthy()
    expect(truncated.textContent).toBe(RESULT.checks[0].text)
  })
})
