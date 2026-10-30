// useAsyncAction phải trả về nhàn rỗi sau mỗi lần chạy — trước đây `busy`
// kẹt true mãi trong finally nên mọi lần bấm sau đều bị nuốt (gồm cả retry).
import { describe, expect, it, vi } from 'vitest'
import { createAsyncAction } from './useAsyncAction'

describe('createAsyncAction', () => {
  it('chạy xong thành công thì run được tiếp lần nữa', async () => {
    const fn = vi.fn(async () => 'ok')
    const act = createAsyncAction(fn)
    await act.run()
    expect(act.get().busy).toBe(false)
    expect(act.get().pending).toBe(false)
    await act.run()
    expect(fn).toHaveBeenCalledTimes(2)
    expect(act.get().busy).toBe(false)
  })

  it('lỗi rồi vẫn retry được', async () => {
    const fn = vi.fn(async () => { throw new Error('mạng chập chờn') })
    const act = createAsyncAction(fn)
    await act.run()
    expect(act.get().error).toBeTruthy()
    expect(act.get().busy).toBe(false)
    await act.run()
    expect(fn).toHaveBeenCalledTimes(2)
  })

  it('chặn chạy chồng khi đang bận nhưng mở lại sau khi xong', async () => {
    vi.useFakeTimers()
    try {
      let release: (() => void) | null = null
      const fn = vi.fn(() => new Promise<string>((resolve) => { release = () => resolve('x') }))
      const act = createAsyncAction(fn)
      const first = act.run()
      expect(act.get().busy).toBe(true)
      expect(fn).toHaveBeenCalledTimes(1)
      await act.run() // bị chặn — vẫn là lần chạy đầu
      expect(fn).toHaveBeenCalledTimes(1)
      release!()
      await vi.runAllTimersAsync()
      await first
      const second = act.run()
      expect(fn).toHaveBeenCalledTimes(2)
      release!()
      await vi.runAllTimersAsync()
      await second
      expect(act.get().busy).toBe(false)
    } finally {
      vi.useRealTimers()
    }
  })

  it('hàm kéo dài >150ms bật spinner rồi tắt sạch', async () => {
    vi.useFakeTimers()
    try {
      const act = createAsyncAction(async () => {
        await new Promise<void>((resolve) => setTimeout(resolve, 300))
      })
      const p = act.run()
      await vi.advanceTimersByTimeAsync(200)
      expect(act.get().pending).toBe(true)
      await vi.advanceTimersByTimeAsync(400)
      await p
      await vi.runAllTimersAsync()
      expect(act.get().pending).toBe(false)
      expect(act.get().busy).toBe(false)
    } finally {
      vi.useRealTimers()
    }
  })
})
