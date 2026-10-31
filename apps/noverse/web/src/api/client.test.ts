import { afterEach, describe, expect, it, vi } from 'vitest'
import { getTtsEngines, getTtsVoices, login } from './client'

import { createAction, type ActionNotice } from '../data/action'
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
  it('dịch lỗi Trọng tài cho dòng lỗi và toast, giữ nguyên dữ liệu chẩn đoán', async () => {
    const raw = 'arbiter: arbiter_intervention: đầu ra vẫn lỗi sau 3 lần tự sửa: Không tìm thấy JSON object trong đầu ra'
    const payload = { error: raw }
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(Response.json(payload, { status: 500 })))
    const notices: ActionNotice[] = []
    const action = createAction(() => login('viet', 'sai'), {}, (notice) => notices.push(notice))
    await action.run()
    const error = action.get().error as Error & { payload: unknown }
    expect(error.message).toContain('Trọng tài')
    expect(error.message).not.toMatch(/\b[A-Za-z][A-Za-z0-9]*_[A-Za-z0-9_]+\b|\bArbiter\b|JSON object/i)
    expect(notices[0]).toMatchObject({ text: error.message, error: true })
    expect(error.payload).toEqual(payload)
    action.dispose()
  })
})
