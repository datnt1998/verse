import { afterEach, describe, expect, it, vi } from 'vitest'
import { getTtsEngines, getTtsVoices, login } from './client'

afterEach(() => vi.unstubAllGlobals())

describe('gateway list response envelopes', () => {
  it('unwraps the voices object returned by the Go gateway', async () => {
    const voices = [{ id: 'builtin', name: 'Giọng mẫu', gender: '', source: 'builtin', builtIn: true }]
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(Response.json({ voices })))
    expect(await getTtsVoices()).toEqual(voices)
  })

  it('unwraps the engines object returned by the Go gateway', async () => {
    const engines = [{ id: 'vieneu', name: 'VieNeu', description: '', installed: false }]
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(Response.json({ engines, active: 'vieneu' })))
    expect(await getTtsEngines()).toEqual(engines)
  })
})

describe('server error copy', () => {
  it('capitalises the first letter of the server message (09 §5)', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(Response.json({ error: 'sai tên đăng nhập hoặc mật khẩu' }, { status: 401 })))
    await expect(login('viet', 'sai')).rejects.toMatchObject({ status: 401, message: 'Sai tên đăng nhập hoặc mật khẩu' })
  })
})
