// @vitest-environment happy-dom
import { act, useState } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { ApiError, checkProjectLedger, suggestProjectLedger } from '../../api/client'
import type { LedgerCheckResult } from '../../api/types'
import type * as Client from '../../api/client'
import { useStoryCheck, type StoryCheck } from './useStoryCheck'

vi.mock('../../api/client', async (original) => ({ ...await original<typeof Client>(), checkProjectLedger: vi.fn(), suggestProjectLedger: vi.fn() }))
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
let root: Root
let gate: StoryCheck
let edit: (brief: string) => void
let brief: string
const result = (level: 'error' | 'warning' | 'ok'): LedgerCheckResult => ({ axes: [{ n: 1, name: 'Bộ & biến thể' }], values: ['A'], cells: [], checks: [{ level, rule: 'combo', text: 'So với truyện trước', short: 'So với truyện trước.', fix: null, axes: [1] }] })
function Probe() {
  const [value, setValue] = useState('Yêu cầu cũ')
  brief = value
  edit = setValue
  gate = useStoryCheck('p', value, setValue)
  return null
}
async function mount() {
  root = createRoot(document.createElement('div'))
  await act(async () => { root.render(<Probe />) })
}
beforeEach(() => { vi.clearAllMocks(); vi.mocked(checkProjectLedger).mockResolvedValue(result('error')) })
afterEach(async () => { if (root) await act(() => root.unmount()) })

it('blocks Bắt sửa but permits warnings without a second confirmation', async () => {
  await mount()
  let allowed = true
  await act(async () => { allowed = await gate.check() })
  expect(allowed).toBe(false)
  expect(gate.blocked).toBe(true)
  await act(() => edit('Yêu cầu khác'))
  vi.mocked(checkProjectLedger).mockResolvedValue(result('warning'))
  await act(async () => { allowed = await gate.check() })
  expect(allowed).toBe(true)
  expect(gate.blocked).toBe(false)
})

it('rewrites the actual brief, rechecks it and restores the old request on undo', async () => {
  await mount()
  await act(async () => { await gate.check() })
  vi.mocked(suggestProjectLedger).mockResolvedValue({ values: ['B'], changed: [1], brief: 'Yêu cầu mới' })
  vi.mocked(checkProjectLedger).mockResolvedValue({ ...result('warning'), values: ['B'] })
  await act(async () => { await gate.suggest() })
  expect(brief).toBe('Yêu cầu mới')
  expect(checkProjectLedger).toHaveBeenLastCalledWith('p', 'Yêu cầu mới')
  expect(gate.state?.changed).toEqual([1])
  expect(gate.state?.previous?.result.values).toEqual(['A'])
  await act(() => gate.undo())
  expect(brief).toBe('Yêu cầu cũ')
  expect(gate.blocked).toBe(true)
})

it('does not apply a stale check after the request is edited', async () => {
  let resolve!: (value: LedgerCheckResult) => void
  vi.mocked(checkProjectLedger).mockImplementation(() => new Promise((done) => { resolve = done }))
  await mount()
  let pending!: Promise<boolean>
  await act(() => { pending = gate.check() })
  expect(gate.checking).toBe(true)
  await act(() => edit('Yêu cầu được sửa trong lúc so'))
  expect(gate.state).toBeNull()
  let allowed = true
  await act(async () => { resolve(result('ok')); allowed = await pending })
  expect(allowed).toBe(false)
  expect(gate.state).toBeNull()
})

it('renders structured start conflicts rather than treating them as generic failures', async () => {
  await mount()
  await act(async () => { await gate.check() })
  const checks = [{ ...result('error').checks[0], text: 'Có truyện mới vừa đăng' }]
  let handled = false
  await act(() => { handled = gate.acceptConflict(new ApiError(409, 'Cần đổi trục', undefined, { checks })) })
  expect(handled).toBe(true)
  expect(gate.state?.result?.checks).toEqual(checks)
  expect(gate.state?.result?.values).toEqual(['A'])
  expect(gate.acceptConflict(new ApiError(409, 'Tên truyện đã có'))).toBe(false)
})
