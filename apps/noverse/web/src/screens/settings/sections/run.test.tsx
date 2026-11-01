// @vitest-environment happy-dom
import { act, useState } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { Config } from '../../../api/types'
import { setPath } from '../lib/config'
import type { SettingsContext } from '../types'

vi.mock('@noverse/ui', async (importOriginal) => ({ ...(await importOriginal<object>()), Icon: () => null }))
import { RunCard } from './RunCard'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
const roots: Root[] = []
afterEach(async () => { for (const root of roots.splice(0)) await act(() => root.unmount()); document.body.replaceChildren() })

/** Cách chạy ở cấp dự án: hai lựa chọn độc lập, ghi bản nháp như index.tsx. */
function Harness() {
  const [draft, setDraft] = useState<Config>({})
  const ctx = {
    scope: 'project', admin: true, canEdit: true, draft, effective: {}, presets: null, snapshot: null,
    setField: (path: string, value: unknown) => setDraft((prev) => setPath(prev, path, value)),
    resetField: (path: string) => setDraft((prev) => setPath(prev, path, undefined)),
    ovr: () => ({ kind: 'none' }), connection: { writable: true },
  } as unknown as SettingsContext
  return <RunCard ctx={ctx} />
}

describe('Cách chạy ở cấp dự án', () => {
  it('giữ hai lựa chọn độc lập và bật tắt lại được', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)
    roots.push(root)
    await act(async () => { root.render(<Harness />) })
    const review = container.querySelector<HTMLButtonElement>('[role="switch"][aria-label="Duyệt từng chương (truyện mới)"]')!
    const ask = container.querySelector<HTMLButtonElement>('[role="switch"][aria-label="Trọng tài hỏi bạn khi phân vân"]')!
    expect(review.getAttribute('aria-checked')).toBe('false')
    await act(async () => review.click())
    expect(review.getAttribute('aria-checked')).toBe('true')
    expect(ask.getAttribute('aria-checked')).toBe('false')
    await act(async () => ask.click())
    expect(ask.getAttribute('aria-checked')).toBe('true')
    await act(async () => review.click())
    expect(review.getAttribute('aria-checked')).toBe('false')
    expect(ask.getAttribute('aria-checked')).toBe('true')
  })
})
