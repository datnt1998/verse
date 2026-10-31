import { afterEach, expect, it, vi } from 'vitest'
import { configFromStorage, connectionPassed, getServerStorage, testServerStorage, type ConnectionResult } from './storageServer'

const successful: ConnectionResult = { testId: 'test1', steps: ['write', 'read', 'range', 'presign', 'cors_probe', 'delete'].map(id => ({ id: id as ConnectionResult['steps'][number]['id'], status: 'ok' })) }
afterEach(() => vi.unstubAllGlobals())
it.each(['rejected', 'short', 'full-response'])('finalizes failed browser probes honestly: %s', async mode => {
  const calls: { url: string; body?: BodyInit | null }[] = []
  vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input); calls.push({ url, body: init?.body })
    if (url === '/api/storage/server/test') return Response.json({ ...successful, probeUrl: 'https://store.example/probe' })
    if (url === 'https://store.example/probe') {
      if (mode === 'rejected') throw new TypeError('Failed to fetch')
      return new Response(new Uint8Array(mode === 'short' ? 8 : 1024), { status: mode === 'full-response' ? 200 : 206 })
    }
    return Response.json({ ...successful, error: { code: 'cors_blocked', step: 'cors_probe' } })
  }))
  const result = await testServerStorage({ provider: 'garage', bucket: 'media', delivery: 'redirect' })
  expect(calls.find(call => call.url.endsWith('/test/test1/probe'))?.body).toBe('{"ok":false,"reason":"network"}')
  expect(connectionPassed(result)).toBe(false)
})
it('refuses a six-row response missing a mandatory transport step', () => {
  const invalid = { ...successful, steps: successful.steps.map(step => step.id === 'write' ? { ...step, id: 'app_range' as const } : step) }
  expect(connectionPassed(invalid)).toBe(false)
})
it.each([{ role: 'owner', usage: [] }, { role: 'member', usage: null }, { role: 'admin', usage: [] }])('rejects malformed server boundaries instead of selecting local controls: %j', async body => {
  vi.stubGlobal('fetch', vi.fn(async () => Response.json(body)))
  await expect(getServerStorage()).rejects.toThrow()
})
it('retains the Cloudflare account when editing saved R2 configuration', () => {
  const config = configFromStorage({ kind: 's3', provider: 'r2', endpointHost: 'account123.r2.cloudflarestorage.com', bucket: 'media', delivery: 'redirect', sources: {}, encryptionKey: { ok: true, path: '/data/key' }, online: true, checkedAt: '2026-10-31T00:00:00Z' })
  expect(config.accountId).toBe('account123')
})
