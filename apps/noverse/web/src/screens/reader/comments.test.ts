import { afterEach, describe, expect, it, vi } from 'vitest'
import { toast } from '@noverse/ui'
import { deleteCommentWithUndo } from './comments'

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
