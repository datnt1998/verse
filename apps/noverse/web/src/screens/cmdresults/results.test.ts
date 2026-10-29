import { afterEach, describe, expect, it, vi } from 'vitest'
import type { JobEvent } from '../../api/types'
import { readCommandResults, writeCommandResults, type CmdItem } from './results'

afterEach(() => vi.unstubAllGlobals())

describe('command results reload', () => {
  it('restores export cards and their completed worker payload per user and story', () => {
    const data = new Map<string, string>()
    vi.stubGlobal('sessionStorage', { getItem: (key: string) => data.get(key) ?? null, setItem: (key: string, value: string) => data.set(key, value) })
    const item: CmdItem = { id: 12, command: '/export a.epub from=1 to=2', kind: 'job', jobKind: 'export', job: 'export/1', path: 'a.epub' }
    const job: JobEvent = { job: 'export/1', kind: 'export', stage: 'done', current: 2, total: 2, message: 'Đã xuất', level: 'success', retryAt: null, done: true, error: '', awaiting: '', export: { name: 'a.epub', path: 'a.epub', format: 'epub', chapters: 2, bytes: 1234, title: 'Truyện', skipped: [] } }
    writeCommandResults('["writer","p","s"]', [item], new Map([[job.job, job]]))
    expect(readCommandResults('["writer","p","s"]')).toEqual({ items: [item], jobs: [[job.job, job]] })
    expect(readCommandResults('["reader","p","s"]')).toEqual({ items: [], jobs: [] })
    expect(readCommandResults('["writer","p","other"]')).toEqual({ items: [], jobs: [] })
  })

  it('does not reopen panels or preserve an in-flight button after reload', () => {
    const data = new Map<string, string>()
    vi.stubGlobal('sessionStorage', { getItem: (key: string) => data.get(key) ?? null, setItem: (key: string, value: string) => data.set(key, value) })
    writeCommandResults('key', [{ id: 1, command: '/diag', kind: 'panel', panel: 'diag' }, { id: 2, command: '/sync --check', kind: 'syncCheck', changed: [], busy: true }], new Map())
    expect(readCommandResults('key').items).toEqual([{ id: 2, command: '/sync --check', kind: 'syncCheck', changed: [], busy: false }])
  })
})
