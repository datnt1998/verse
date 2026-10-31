// @vitest-environment happy-dom
import { act, type Dispatch, type SetStateAction } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, expect, it, vi } from 'vitest'
import type { StorageView } from '../../../api/storage-settings'
import { cache } from '@noverse/ui/data'
import { StorageSection } from './index'
import { storageKey } from './resource'

const { retainedStates, api } = vi.hoisted(() => ({
  retainedStates: new Map<unknown, unknown>(),
  api: { getStorage: vi.fn(), testStorage: vi.fn(), putStorage: vi.fn(), controlStorageMigration: vi.fn() },
}))
// Observe current state values while using React's real hooks and setters unchanged.
// DOM-only assertions cannot detect a draft retained after its form is unmounted.
vi.mock('react', async (original) => {
  const react = await original<typeof import('react')>()
  function useState<S>(initial: S | (() => S)): [S, Dispatch<SetStateAction<S>>]
  function useState<S = undefined>(): [S | undefined, Dispatch<SetStateAction<S | undefined>>]
  function useState<S>(initial?: S | (() => S)) {
    const state = react.useState(initial)
    retainedStates.set(state[1], state[0])
    return state
  }
  return { ...react, useState }
})
vi.mock('../../../api/session', () => ({ useSession: () => ({ user: { role: 'admin' } }) }))
vi.mock('../../../api/client', async (original) => ({ ...await original<typeof import('../../../api/client')>(), ...api }))
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const key = 'GK-disposable-component-key'
const secret = 'disposable-component-secret-not-to-retain'
const initial: StorageView = {
  mode: 'postgres', version: 4, encryption: { available: true }, operations: [], migration: null,
  active: { driver: 'local', provider: '', endpointHost: '', accessKeyLast4: '', env: 'dev', source: {}, delivery: 'proxy', pathStyle: false, allowHTTP: false },
  total: { count: 2, bytes: 3000 },
}
const pending: StorageView = { ...initial, version: 5, pending: { provider: 'garage', endpointHost: 'garage:3900', bucket: 'novel-media' } }
const storesCredentials = () => [...retainedStates.values()].some((value) => value !== null && typeof value === 'object' && ('secretAccessKey' in value && value.secretAccessKey === secret || 'accessKeyId' in value && value.accessKeyId === key))

async function click(label: string, scope: ParentNode = document) {
  const button = [...scope.querySelectorAll<HTMLButtonElement>('button')].find((element) => element.textContent?.trim() === label)
  expect(button, `Missing button: ${label}`).toBeTruthy()
  await act(async () => button!.click())
}
async function fill(id: string, value: string) {
  const input = document.getElementById(id) as HTMLInputElement
  expect(input).toBeTruthy()
  await act(async () => {
    Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!.call(input, value)
    input.dispatchEvent(new Event('input', { bubbles: true }))
  })
}
afterEach(() => { cache.clear(); retainedStates.clear(); vi.clearAllMocks(); document.body.replaceChildren() })

it('discards saved credentials before start settles and retries a failed start using only the saved version', async () => {
  let rejectStart!: (error: Error) => void
  // ES2022 has no Promise.withResolvers; hold the real component's network boundary.
  const starting = new Promise<StorageView>((_, reject) => { rejectStart = reject })
  api.getStorage.mockResolvedValue(initial)
  api.testStorage.mockResolvedValue({ ok: true, steps: [] })
  api.putStorage.mockImplementation(async () => {
    api.getStorage.mockResolvedValue(pending)
    return { version: 5, needsMigration: true, count: 2, bytes: 3000, test: { ok: true, steps: [] } }
  })
  api.controlStorageMigration.mockReturnValueOnce(starting).mockResolvedValueOnce({ ...pending, migration: { id: 'move', state: 'running', copiedCount: 0, copiedBytes: 0, totalCount: 2, totalBytes: 3000, bytesPerSecond: 0, errorCount: 0, startedAt: new Date().toISOString(), errors: [] } })
  cache.setData(storageKey, initial)
  const container = document.createElement('div'); document.body.append(container)
  const root = createRoot(container)
  const assertErased = () => {
    expect(storesCredentials()).toBe(false)
    expect([...document.querySelectorAll<HTMLInputElement>('input')].some((input) => input.value === key || input.value === secret)).toBe(false)
    expect(document.body.textContent).not.toContain(secret)
    expect(document.body.textContent).not.toContain(key)
  }
  try {
    await act(async () => root.render(<StorageSection />))
    await click('Chọn kho S3')
    const garage = [...container.querySelectorAll<HTMLButtonElement>('button')].find((button) => button.querySelector('b')?.textContent === 'Garage')!
    await act(async () => garage.click())
    await fill('storage-bucket', 'novel-media'); await fill('storage-accessKeyId', key); await fill('storage-secretAccessKey', secret)
    expect(storesCredentials()).toBe(true)
    await click('Kiểm tra kết nối')
    await click('Lưu…')
    await click('Bắt đầu chuyển', document.querySelector('[role=alertdialog]')!)
    expect(api.putStorage).toHaveBeenCalledTimes(1)
    expect(api.controlStorageMigration).toHaveBeenCalledWith('start', 5)
    assertErased()
    await act(async () => { rejectStart(new Error('Không bắt đầu được chuyển tệp')); await starting.catch(() => undefined) })
    assertErased()
    await click('Bắt đầu chuyển')
    expect(api.controlStorageMigration.mock.calls).toEqual([['start', 5], ['start', 5]])
    expect(api.putStorage).toHaveBeenCalledTimes(1)
    assertErased()
    expect(document.querySelector('section[aria-label="Chuyển kho tệp"]')).not.toBeNull()
  } finally {
    // Settle the held boundary even when the fail-before assertion exits early.
    await act(async () => { rejectStart(new Error('Kết thúc kiểm tra')); await starting.catch(() => undefined) })
    await act(async () => root.unmount())
  }
})
