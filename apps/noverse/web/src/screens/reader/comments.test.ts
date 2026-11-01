import { afterEach, describe, expect, it, vi } from 'vitest'
import { toast } from '@noverse/ui'
import type { User } from '../../api/types'
import { canChangeComment, deleteCommentWithUndo } from './comments'

vi.mock('@noverse/ui', async (importOriginal) => ({ ...(await importOriginal<object>()),  toast: Object.assign(vi.fn(), { ok: vi.fn() }) }))
afterEach(() => { vi.useRealTimers(); vi.clearAllMocks() })

describe('E2ELIB-7 paragraph comment deletion', () => {
  it('offers Hoàn tác for 5 s and never sends DELETE when undone', async () => {
    vi.useFakeTimers()
    const commit = vi.fn(async () => ({ ok: true }))
    const result = deleteCommentWithUndo(commit)
    const [text, tone, action, options] = vi.mocked(toast).mock.calls[0]
    expect({ text, tone, label: action?.label, options }).toEqual({ text: 'Đã xoá góp ý', tone: 'ok', label: 'Hoàn tác', options: { undo: true, duration: 5000 } })
    await vi.advanceTimersByTimeAsync(4999)
    await action!.onClick()
    await vi.advanceTimersByTimeAsync(5000)
    await expect(result).resolves.toBeUndefined()
    expect(commit).not.toHaveBeenCalled()
  })

  it('commits after the undo window and reports server failures for rollback', async () => {
    vi.useFakeTimers()
    const commit = vi.fn(async () => ({ ok: true }))
    const result = deleteCommentWithUndo(commit)
    await vi.advanceTimersByTimeAsync(4999)
    expect(commit).not.toHaveBeenCalled()
    await vi.advanceTimersByTimeAsync(1)
    await expect(result).resolves.toEqual({ ok: true })
    expect(commit).toHaveBeenCalledOnce()

    const failed = deleteCommentWithUndo(async () => { throw new Error('Không có quyền') })
    const assertion = expect(failed).rejects.toThrow('Không có quyền')
    await vi.advanceTimersByTimeAsync(5000)
    await assertion
  })
})

describe('§AI · ai được sửa/xoá góp ý', () => {
  const collab = { username: 'collab', role: 'member', projects: { proj: 'collab' } } as unknown as User
  const owner = { username: 'owner', role: 'member', projects: { proj: 'owner' } } as unknown as User
  const admin = { username: 'admin', role: 'admin', projects: {} } as unknown as User

  it('người tạo sửa được góp ý của mình, góp ý người khác thì không', () => {
    expect(canChangeComment('collab', collab, false)).toBe(true)
    expect(canChangeComment('owner', collab, false)).toBe(false)
  })

  it('chủ dự án và quản trị viên sửa được mọi góp ý; chưa đăng nhập thì không', () => {
    expect(canChangeComment('collab', owner, true)).toBe(true)
    expect(canChangeComment('collab', admin, true)).toBe(true)
    expect(canChangeComment('collab', null, true)).toBe(false)
  })
})
