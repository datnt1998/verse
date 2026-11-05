// @vitest-environment happy-dom
import { act, type ReactNode } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, expect, it, vi } from 'vitest'
import type { AudioChapter } from '../../../api/types'
import type { GenRow } from '../genWork'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
vi.mock('../../../mobile/primitives', () => ({
  BottomSheet: ({ open, children, label }: { open: boolean; children: ReactNode; label?: string }) =>
    open ? <section role="dialog" aria-label={label}>{children}</section> : null,
  ConfirmSheet: ({ open, title, description, action, onClose }: {
    open: boolean; title: string; description: string; onClose: () => void
    action: { label: string; onSelect: () => Promise<void> | void }
  }) => open ? <section role="alertdialog" aria-label={title}><p>{description}</p>
    <button type="button" onClick={() => void action.onSelect()}>{action.label}</button>
    <button type="button" onClick={onClose}>Huỷ</button>
  </section> : null,
}))
vi.mock('@noverse/ui', () => ({
  Icon: ({ name }: { name: string }) => <i data-icon={name} />,
  Spinner: () => <i data-spinner="" />,
}))
vi.mock('../useAudio', () => ({ errText: (cause: unknown) => cause instanceof Error ? cause.message : String(cause) }))
import { ChapterActions } from './ChapterActions'
import { chapterActions } from './chapter-actions'

const chapter = (patch: Partial<AudioChapter> = {}): AudioChapter => ({
  chapter: 1, title: 'Bảy mươi ba giây', status: 'locked', issues: 0, durationSec: 960, lockedAt: null,
  segments: 46, speakers: 5, changedSegments: 3,
  audio: { segments: 46, segmentsDone: 46, durationSec: 960, rendering: false, percent: 100 },
  chapterChanged: true,
  ...patch,
})

const row: GenRow = { chapter: chapter(), job: null, state: 'stale', position: 0 }
const items = chapterActions(row, { ready: true, playing: false, writable: true, quality: 'standard' })

let root: Root
let container: HTMLDivElement
afterEach(async () => { await act(() => root?.unmount()); container?.remove() })

async function mount(onAction = vi.fn(async () => {}), onClose = vi.fn()) {
  container = document.createElement('div'); document.body.append(container); root = createRoot(container)
  await act(() => root.render(<ChapterActions open title="Chương 1 · Bảy mươi ba giây" meta="16:00 · Chuẩn · Phân vai · 5 giọng" items={items} onClose={onClose} onAction={onAction} />))
  return { view: container, onAction, onClose }
}

it('mở menu chỉ bày thao tác, chưa gọi máy chủ lần nào', async () => {
  const { view, onAction } = await mount()
  expect(view.querySelector('[data-action="update"]')).not.toBeNull()
  expect(view.querySelector('[data-action="redo"]')).not.toBeNull()
  expect(onAction).not.toHaveBeenCalled()
})


it('Tạo lại hỏi lại trước: chưa gọi máy chủ cho tới khi xác nhận', async () => {
  const { view, onAction } = await mount()
  const button = view.querySelector<HTMLButtonElement>('[data-action="redo"]')!
  await act(async () => button.click())
  expect(onAction).not.toHaveBeenCalled()
  expect(view.querySelector('[role="alertdialog"]')).not.toBeNull()
  const confirm = view.querySelector<HTMLButtonElement>('[role="alertdialog"] button')!
  await act(async () => confirm.click())
  expect(onAction).toHaveBeenCalledWith('redo')
})

it('huỷ xác nhận không chạy thao tác và vẫn giữ menu chương', async () => {
  const { view, onAction } = await mount()
  await act(async () => view.querySelector<HTMLButtonElement>('[data-action="redo"]')!.click())
  await act(async () => view.querySelectorAll<HTMLButtonElement>('[role="alertdialog"] button')[1].click())
  expect(view.querySelector('[role="alertdialog"]')).toBeNull()
  expect(view.querySelector('[role="dialog"]')).not.toBeNull()
  expect(onAction).not.toHaveBeenCalled()
})

it('đang chờ máy chủ thì khoá thao tác lặp và đóng menu', async () => {
  let finish!: () => void
  const onAction = vi.fn(() => new Promise<void>((resolve) => { finish = resolve }))
  const { view, onClose } = await mount(onAction)
  const update = view.querySelector<HTMLButtonElement>('[data-action="update"]')!
  await act(async () => update.click())
  expect([...view.querySelectorAll<HTMLButtonElement>('[role="menuitem"],.ma-chm-close')].every((button) => button.disabled)).toBe(true)
  await act(async () => { update.click(); view.querySelector<HTMLButtonElement>('.ma-chm-close')!.click() })
  expect(onAction).toHaveBeenCalledTimes(1)
  expect(onClose).not.toHaveBeenCalled()
  await act(async () => finish())
  expect(update.disabled).toBe(false)
})

it('máy chủ từ chối thì lỗi hiện trong tấm, tấm vẫn mở để thử lại', async () => {
  const onAction = vi.fn(async () => { throw new Error('Chương 1 chưa chốt bản đọc.') })
  const { view, onClose } = await mount(onAction)
  const button = view.querySelector<HTMLButtonElement>('[data-action="update"]')!
  await act(async () => button.click())
  expect(view.querySelector('[role="alert"]')?.textContent).toContain('chưa chốt bản đọc')
  expect(view.querySelector('[role="dialog"]')).not.toBeNull()
  expect(onClose).not.toHaveBeenCalled()
})
