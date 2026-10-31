import { afterEach, describe, expect, it, vi } from 'vitest'
import { createCache } from '@noverse/ui/data'
import { createAction, type ActionNotice } from './action'
import { keysForEvent, watchListOf } from './eventKeys'

const deferred = <T>() => {
  let resolve!: (value: T) => void
  let reject!: (reason: unknown) => void
  const promise = new Promise<T>((yes, no) => { resolve = yes; reject = no })
  return { promise, resolve, reject }
}
afterEach(() => { vi.useRealTimers() })

it('ánh xạ sự kiện sang prefix, kế thừa cấu hình và danh sách theo dõi', () => {
  expect(keysForEvent({ type: 'story.changed', data: { project: 'p', story: 's', parts: ['chapters', 'audio/scripts'] } })).toContainEqual(['story', 'p', 's', 'chapter'])
  expect(keysForEvent({ type: 'story.changed', data: { project: 'p', story: 's', parts: ['audio/scripts'] } })).toEqual([['story', 'p', 's', 'audio', 'script']])
  expect(keysForEvent({ type: 'config.changed', data: { scope: 'project', project: 'p' } })).toEqual([['config', 'project', 'p'], ['config', 'story', 'p']])
  expect(keysForEvent({ type: 'reset', data: {} })).toBe('*')
  expect(watchListOf([['story', 'p', 's', 'chapter', 1], ['story', 'p', 's', 'reviews'], ['project', 'p', 'meta']])).toEqual(['p', 'p/s'])
})

it('làm mới lịch sử khi thao tác live cập nhật hoặc lượt chạy kết thúc', () => {
  expect(keysForEvent({ type: 'live', data: { project: 'p', story: 's', kind: 'event', data: { category: 'TOOL' } } })).toEqual([['story', 'p', 's', 'live-history']])
  expect(keysForEvent({ type: 'live', data: { project: 'p', story: 's', kind: 'event', data: { category: 'TEXT' } } })).toEqual([])
  expect(keysForEvent({ type: 'live', data: { project: 'p', story: 's', kind: 'done' } })).toContainEqual(['story', 'p', 's', 'live-history'])
})

it('gộp prefix trùng do một story.changed có nhiều phần', async () => {
  const cache = createCache()
  const fetcher = vi.fn().mockResolvedValue(1)
  cache.retain(['story', 'p', 's', 'plan', 1], fetcher)
  await Promise.resolve(); await Promise.resolve()
  const prefixes = keysForEvent({ type: 'story.changed', data: { project: 'p', story: 's', parts: ['chapters', 'outline', 'chapters'] } })
  if (prefixes === '*') throw new Error('Không phải sự kiện reset')
  await Promise.all(prefixes.map((prefix) => cache.invalidate(prefix)))
  await Promise.resolve(); await Promise.resolve()
  expect(fetcher).toHaveBeenCalledTimes(2)
})

it('theo dõi scope của quy tắc ngay cả khi chỉ mở Tuỳ chỉnh', () => {
  expect(watchListOf([['rules', 'project', 'p'], ['rules', 'story', 'p', 's'], ['config', 'story', 'p', 's']])).toEqual(['p', 'p/s'])
})

it('sự kiện âm thanh cấp dự án làm mới mọi truyện trong dự án', () => {
  expect(keysForEvent({ type: 'story.changed', data: { project: 'p', parts: ['audio'] } })).toContainEqual(['story', 'p'])
})

it('action khoá ngay, spinner sau 150 ms và giữ 400 ms', async () => {
  vi.useFakeTimers()
  const work = deferred<string>()
  const fn = vi.fn(() => work.promise)
  const action = createAction(fn)
  const run = action.run()
  void action.run()
  expect(fn).toHaveBeenCalledTimes(1)
  expect(action.get().busy).toBe(true)
  expect(action.get().pending).toBe(false)
  await vi.advanceTimersByTimeAsync(149)
  expect(action.get().pending).toBe(false)
  await vi.advanceTimersByTimeAsync(1)
  expect(action.get().pending).toBe(true)
  work.resolve('ok'); await Promise.resolve()
  await vi.advanceTimersByTimeAsync(399)
  expect(action.get().pending).toBe(true)
  await vi.advanceTimersByTimeAsync(1)
  await run
  expect(action.get().pending).toBe(false)
  action.dispose()
})

it('lạc quan trả lại dữ liệu khi lỗi và đưa hành động thử lại', async () => {
  const cache = createCache()
  cache.setData(['a'], 'cũ')
  const work = deferred<void>()
  const notices: ActionNotice[] = []
  const action = createAction(() => work.promise, { optimistic: () => {
    const old = cache.setData(['a'], 'mới')
    return () => cache.restore(['a'], old)
  } }, (notice) => notices.push(notice))
  const run = action.run()
  expect(cache.get(['a']).data).toBe('mới')
  work.reject(new Error('Không lưu được'))
  await run
  expect(cache.get(['a']).data).toBe('cũ')
  expect(notices[0].text).toBe('Không lưu được')
  expect(notices[0].action?.label).toBe('Thử lại')
  action.dispose()
})

it('lỗi needsConfig không đưa "Thử lại" vô ích mà dẫn tới Tuỳ chỉnh (E2EM-12)', async () => {
  const notices: ActionNotice[] = []
  const action = createAction(() => Promise.reject(Object.assign(new Error('Chưa có cấu hình mô hình'), { payload: { needsConfig: true } })), {}, (notice) => notices.push(notice))
  await action.run()
  expect(notices[0].action?.label).toBe('Mở Tuỳ chỉnh')
  action.dispose()
})

it('lỗi theo trường (error: false) giữ lỗi trong state, không toast', async () => {
  const notices: ActionNotice[] = []
  const failure = new Error('Sai tên đăng nhập hoặc mật khẩu')
  const action = createAction(async () => { throw failure }, { error: false }, (notice) => notices.push(notice))
  await action.run()
  expect(action.get().error).toBe(failure)
  expect(notices).toEqual([])
  action.dispose()
})

it('hoàn tác 5 giây chỉ chạy một lần, lỗi có thể thử lại', async () => {
  const notices: ActionNotice[] = []
  const undo = vi.fn().mockRejectedValueOnce(new Error('Không hoàn tác được')).mockResolvedValue(undefined)
  const action = createAction(async () => 'id', { success: 'Đã xoá', undo }, (notice) => notices.push(notice))
  await action.run()
  expect(notices[0].undoMs).toBe(5000)
  await notices[0].action!.onClick()
  expect(notices[1].error).toBe(true)
  await notices[1].action!.onClick()
  await notices[0].action!.onClick()
  expect(undo).toHaveBeenCalledTimes(2)
  expect(notices.at(-1)?.text).toBe('Đã hoàn tác')
  action.dispose()
})

it('hoàn tác giữ đúng tài nguyên gốc sau khi đổi chương', async () => {
  const original = vi.fn().mockResolvedValue(undefined)
  const nextChapter = vi.fn().mockResolvedValue(undefined)
  const options = { success: 'Đã nhận xét', undo: original }
  const notices: ActionNotice[] = []
  const action = createAction(async () => 'comment-id', options, (notice) => notices.push(notice))
  await action.run()
  options.undo = nextChapter
  await notices[0].action!.onClick()
  expect(original).toHaveBeenCalledTimes(1)
  expect(nextChapter).not.toHaveBeenCalled()
  action.dispose()
})
