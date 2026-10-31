// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { privateEndpoint, probeBrowser, saveAndMigrate, startVisiblePolling, displaySteps } from './logic'

afterEach(() => { vi.useRealTimers(); vi.restoreAllMocks() })

describe('privateEndpoint mirrors Go net.ParseIP private/loopback/link-local semantics', () => {
  it.each([
    // Keep the canonical corpus identical to internal/bootstrap/blob_endpoint_test.go.
    ['', false], ['relative/path', false], ['ftp://garage', false], ['://bad', false],
    ['https://s3.amazonaws.com', false], ['https://acct.r2.cloudflarestorage.com', false], ['http://s3.amazonaws.com', true],
    ['https://garage', true], ['https://10.0.0.1', true], ['https://172.16.0.1', true],
    ['https://172.31.255.255', true], ['https://172.32.0.1', false], ['https://192.168.0.1', true],
    ['https://127.0.0.1', true], ['https://169.254.10.1', true], ['https://8.8.8.8', false],
    ['https://[fd00::1]', true], ['https://[fe80::1]', true], ['https://[::1]', true],
    ['https://[::ffff:192.168.1.1]', true], ['https://[::ffff:8.8.8.8]', false],
    ['https://[::ffff:808:808]', false], ['https://[2606:4700:4700::1111]', false],
  ])('%s → %s', (endpoint, expected) => expect(privateEndpoint(endpoint)).toBe(expected))
})

describe('browser probe classification', () => {
  it('reads range with CORS and accepts a successful response', async () => {
    const fetcher = vi.fn().mockResolvedValue(new Response('ab', { status: 206 }))
    expect(await probeBrowser('https://store.example/probe', undefined, fetcher)).toMatchObject({ ok: true })
    expect(fetcher).toHaveBeenCalledWith('https://store.example/probe', expect.objectContaining({ headers: { Range: 'bytes=0-1' }, mode: 'cors' }))
  })
  it('classifies opaque no-cors success as missing CORS', async () => {
    const fetcher = vi.fn().mockRejectedValueOnce(new TypeError('Failed to fetch')).mockResolvedValueOnce({ type: 'opaque', ok: false })
    expect(await probeBrowser('https://store.example/probe', undefined, fetcher)).toMatchObject({ ok: false, code: 'browser_cors' })
    expect(fetcher.mock.calls[1][1]).toMatchObject({ mode: 'no-cors' })
  })
  it('classifies two network failures as unreachable', async () => {
    const fetcher = vi.fn().mockRejectedValue(new TypeError('Failed to fetch'))
    expect(await probeBrowser('https://store.example/probe', undefined, fetcher)).toMatchObject({ ok: false, code: 'browser_unreachable' })
  })
  it('never retries an aborted probe', async () => {
    const abort = new AbortController(); abort.abort()
    const fetcher = vi.fn().mockRejectedValue(new DOMException('Aborted', 'AbortError'))
    await expect(probeBrowser('https://store.example/probe', abort.signal, fetcher)).rejects.toThrow('Aborted')
    expect(fetcher).toHaveBeenCalledTimes(1)
  })
})

describe('visibility-aware migration polling', () => {
  it('polls every five seconds, stops hidden, resumes visible and cleans up', async () => {
    vi.useFakeTimers()
    let hidden = false
    vi.spyOn(document, 'hidden', 'get').mockImplementation(() => hidden)
    const refresh = vi.fn().mockResolvedValue(undefined)
    const stop = startVisiblePolling(5000, refresh)
    await vi.advanceTimersByTimeAsync(5000); expect(refresh).toHaveBeenCalledTimes(1)
    hidden = true; document.dispatchEvent(new Event('visibilitychange'))
    await vi.advanceTimersByTimeAsync(15000); expect(refresh).toHaveBeenCalledTimes(1)
    hidden = false; document.dispatchEvent(new Event('visibilitychange'))
    await vi.advanceTimersByTimeAsync(5000); expect(refresh).toHaveBeenCalledTimes(2)
    stop(); await vi.advanceTimersByTimeAsync(15000); expect(refresh).toHaveBeenCalledTimes(2)
  })
  it('does not overlap a pending request and reports rejection', async () => {
    vi.useFakeTimers()
    // The consuming app targets ES2022, where Promise.withResolvers is unavailable.
    let reject!: (error: Error) => void
    const refresh = vi.fn(() => new Promise<void>((_, fail) => { reject = fail }))
    const failure = vi.fn()
    const stop = startVisiblePolling(5000, refresh, failure)
    await vi.advanceTimersByTimeAsync(15000); expect(refresh).toHaveBeenCalledTimes(1)
    reject(new Error('offline')); await vi.advanceTimersByTimeAsync(0)
    expect(failure).toHaveBeenCalledOnce(); stop()
  })
})

describe('save then migrate', () => {
  const candidate = { version: 2, driver: 's3' as const, provider: 'garage', delivery: 'proxy' as const, pathStyle: true, allowHTTP: true }
  it('starts only after successful PUT and uses the returned version', async () => {
    const calls: string[] = []
    const save = vi.fn(async () => { calls.push('put'); return { version: 3, needsMigration: true, count: 4, bytes: 20, test: { ok: true, steps: [] } } })
    const migrate = vi.fn(async (version: number) => { calls.push(`start:${version}`) })
    await saveAndMigrate(candidate, true, save, migrate)
    expect(calls).toEqual(['put', 'start:3'])
  })
  it('does not PUT or migrate without a successful browser-inclusive test', async () => {
    const save = vi.fn(); const migrate = vi.fn()
    await expect(saveAndMigrate(candidate, false, save, migrate)).rejects.toThrow()
    expect(save).not.toHaveBeenCalled(); expect(migrate).not.toHaveBeenCalled()
  })
  it('never starts migration after a conflict', async () => {
    const save = vi.fn().mockRejectedValue(new Error('409')); const migrate = vi.fn()
    await expect(saveAndMigrate(candidate, true, save, migrate)).rejects.toThrow('409')
    expect(migrate).not.toHaveBeenCalled()
  })
  it('never starts migration after server test failure or same-store update', async () => {
    const migrate = vi.fn()
    await expect(saveAndMigrate(candidate, true, vi.fn().mockResolvedValue({ version: 3, needsMigration: true, test: { ok: false } }), migrate)).rejects.toThrow()
    await saveAndMigrate(candidate, true, vi.fn().mockResolvedValue({ version: 3, needsMigration: false, test: { ok: true } }), migrate)
    expect(migrate).not.toHaveBeenCalled()
  })
})

describe('six design steps mapped by code', () => {
  it('folds clock skew into write, preserves delete, and fills skipped browser steps', () => {
    const result = displaySteps({ ok: false, steps: [
      { step: 'delete', ok: true, ms: 7, code: 'ok', reason: '' },
      { step: 'clock', ok: false, ms: 4, code: 'clock_skew', reason: 'Lệch giờ' },
      { step: 'write', ok: true, ms: 8, code: 'ok', reason: '' },
    ] }, 'redirect')
    expect(result.map((step) => step.step)).toEqual(['write', 'read', 'range', 'presign', 'cors_probe', 'delete'])
    expect(result[0]).toMatchObject({ ok: false, code: 'clock_skew', ms: 12 })
    expect(result[4].code).toBe('skipped')
    expect(result[5].ok).toBe(true)
  })
})
