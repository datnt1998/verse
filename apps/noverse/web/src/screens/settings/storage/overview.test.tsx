// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { expect, it, vi } from 'vitest'
import type { StorageView } from '../../../api/storage-settings'
import { storageCopy as c } from '../../../i18n/vi'
import { StorageOverview } from './overview'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
it('renders actual presign_get metrics, including errors and timings, alongside open/get metrics', async () => {
  // MetricsStore records these operation/driver/outcome labels, not the UI label presign.
  const operations = [
    { op: 'presign_get', driver: 's3', outcome: 'ok', count: 3, bytes: 0, durationSeconds: .3, maxDurationSeconds: .2 },
    { op: 'presign_get', driver: 's3', outcome: 'error', count: 2, bytes: 0, durationSeconds: .15, maxDurationSeconds: .25 },
    { op: 'open', driver: 's3', outcome: 'ok', count: 2, bytes: 2000, durationSeconds: .09, maxDurationSeconds: .06 },
    { op: 'get', driver: 's3', outcome: 'ok', count: 1, bytes: 1000, durationSeconds: .06, maxDurationSeconds: .06 },
    { op: 'put', driver: 's3', outcome: 'ok', count: 99, bytes: 99000, durationSeconds: 9, maxDurationSeconds: .9 },
  ]
  const view: StorageView = {
    mode: 'postgres', encryption: { available: true }, migration: null, operations,
    active: { driver: 's3', provider: 'garage', endpointHost: 'media.example', endpointScheme: 'https', bucket: 'novel-media', region: 'garage', accessKeyLast4: '1234', env: 'dev', source: {}, delivery: 'redirect', pathStyle: true, allowHTTP: false },
  }
  const host = document.createElement('div'); const root = createRoot(host)
  const metricValues = (label: string) => {
    const section = host.querySelector(`section[aria-label="${c.operations}"]`)!
    const row = [...section.querySelectorAll('div')].find((element) => element.children.length === 6 && element.firstElementChild?.textContent === label)
    expect(row).toBeTruthy()
    return [...row!.children].slice(1).map((cell) => cell.textContent)
  }
  try {
    await act(async () => root.render(<StorageOverview view={view} onRefresh={vi.fn().mockResolvedValue(undefined)} />))
    expect(metricValues(c.opNames.presign)).toEqual(['5', '2', '0 KB', '90 ms', '250 ms'])
    expect(metricValues(c.opNames.get)).toEqual(['3', '0', '3 KB', '50 ms', '60 ms'])
    expect(metricValues(c.opNames.put)).toEqual(['99', '0', '99 KB', '91 ms', '900 ms'])
  } finally { await act(async () => root.unmount()) }
})
