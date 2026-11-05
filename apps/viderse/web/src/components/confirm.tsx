// ConfirmDialog của Viderse — bọc confirm()/ConfirmDialog của kit theo 7 tình
// huống của Kit › Xác nhận (+ Tự chạy của Dialogs): tiêu đề là câu hỏi có tên
// đối tượng, thân nói hậu quả, nút ghi đúng động từ, Esc = Huỷ.
import type { ReactNode } from 'react'
import { confirm, type ConfirmOptions } from '@noverse/ui'
import { t } from '../i18n/vi'
import { ASSISTANT_ORB, assistantName, type Assistant } from './agents'

export { ConfirmDialog } from '@noverse/ui'

type Run = (choice: string) => void | Promise<void>

const open = (options: ConfirmOptions) => confirm(options)

/** pause — Tạm dừng trợ lý giữa chừng (lựa chọn radio + Không hỏi lại). */
export function confirmPause(args: {
  agent: Assistant
  video: string
  /** cảnh đang làm, vd “cảnh 5/6” */
  scene: string
  /** 'after' = sau cảnh này · 'now' = ngay bây giờ */
  onConfirm: (choice: 'after' | 'now') => void | Promise<void>
  /** bật “Không hỏi lại” (kit lưu theo user + kind) */
  remember?: { user: string; kind: string }
  undo?: () => void
}): Promise<boolean> {
  const c = t.confirm.pause
  return open({
    agent: ASSISTANT_ORB[args.agent],
    title: c.title(assistantName(args.agent)),
    body: c.body(args.video, args.scene),
    choices: [
      { value: 'after', label: c.choiceAfter, description: c.choiceAfterNote },
      { value: 'now', label: c.choiceNow, description: c.choiceNowNote },
    ],
    defaultChoice: 'after',
    cancelLabel: c.cancel,
    confirmLabel: c.ok,
    pendingLabel: c.busy,
    remember: args.remember,
    undo: args.undo,
    successMessage: c.done(assistantName(args.agent)),
    onConfirm: ((choice) => args.onConfirm(choice === 'now' ? 'now' : 'after')) as Run,
  })
}

/** overwrite — Ghi đè MP4 đã xuất (so hai bản + nút phụ Giữ cả hai). */
export function confirmOverwrite(args: {
  file: string
  current: { summary: ReactNode; meta?: ReactNode }
  next: { summary: ReactNode; meta?: ReactNode }
  onConfirm: () => void | Promise<void>
  onKeepBoth: () => void | Promise<void>
}): Promise<boolean> {
  const c = t.confirm.overwrite
  return open({
    tone: 'warn',
    title: c.title,
    body: c.body(args.file),
    compare: [
      { label: c.current, summary: args.current.summary, meta: args.current.meta },
      { label: c.next, summary: args.next.summary, meta: args.next.meta },
    ],
    cancelLabel: t.common.cancel,
    confirmLabel: c.ok,
    pendingLabel: c.busy,
    secondary: { label: c.alt, onConfirm: args.onKeepBoth },
    successMessage: c.done(args.file),
    onConfirm: args.onConfirm,
  })
}

/** logout — Đăng xuất nhà cung cấp AI (nguy hiểm làm lại được → cam nhạt). */
export function confirmProviderLogout(args: {
  provider: string
  fallback: string
  onConfirm: () => void | Promise<void>
}): Promise<boolean> {
  const c = t.confirm.logout
  return open({
    tone: 'dsoft',
    title: c.title(args.provider),
    body: c.body(args.fallback),
    cancelLabel: t.common.cancel,
    confirmLabel: c.ok(args.provider),
    pendingLabel: c.busy,
    successMessage: c.done(args.provider, args.fallback),
    onConfirm: args.onConfirm,
  })
}

/** video — Xoá video vĩnh viễn (danh sách hệ quả, nút cam đặc, focus vào Huỷ). */
export function confirmDeleteVideo(args: {
  name: string
  /** chi tiết video (vd “Midnight Tales · Sẵn sàng · MP4 38:12 · 172 MB”) */
  meta?: string
  /** mục sẽ mất (file MP4, ảnh đã duyệt, project…) */
  deleting: string[]
  /** mục giữ lại (video nguồn, clip nền, kênh…) */
  keeping: string[]
  running?: boolean
  onConfirm: () => void | Promise<void>
}): Promise<boolean> {
  const c = t.confirm.deleteVideo
  return open({
    tone: 'danger',
    title: c.title(args.name),
    body: (
      <>
        {args.meta && <div style={{ fontSize: 12, color: 'var(--color-fg-4)', marginBottom: 'var(--vv-s-8)' }}>{args.meta}</div>}
        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 'var(--vv-s-12)' }}>
          <span style={{ display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-7)' }}>
            <span className="lbl" style={{ color: 'var(--color-err)', letterSpacing: '.04em' }}>{c.willDelete}</span>
            {args.deleting.map((item) => <span key={item} style={{ fontSize: 12.5 }}>{item}</span>)}
          </span>
          <span style={{ display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-7)' }}>
            <span className="lbl" style={{ color: 'var(--color-ok)', letterSpacing: '.04em' }}>{c.willKeep}</span>
            {args.keeping.map((item) => <span key={item} style={{ fontSize: 12.5 }}>{item}</span>)}
          </span>
        </div>
        <p style={{ margin: 'var(--vv-s-8) 0 0', fontSize: 12.5, color: 'var(--color-fg-3)' }}>
          {c.body}{args.running ? ` ${c.runningHint}` : ''}
        </p>
      </>
    ),
    cancelLabel: t.common.cancel,
    confirmLabel: c.ok,
    pendingLabel: c.busy,
    successMessage: c.done,
    onConfirm: args.onConfirm,
  })
}

/** channel — Xoá kênh (gõ đúng tên mới mở khoá; có video chạy thì khoá + lý do). */
export function confirmDeleteChannel(args: {
  name: string
  /** số video đang làm của kênh — > 0 thì hiện lý do khoá */
  runningCount?: number
  consequences?: string[]
  onConfirm: () => void | Promise<void>
}): Promise<boolean> {
  const c = t.confirm.deleteChannel
  const running = (args.runningCount ?? 0) > 0
  return open({
    tone: 'danger',
    title: c.title(args.name),
    body: (
      <>
        <p style={{ margin: 0, fontSize: 13, color: 'var(--color-fg-3)' }}>{c.body}</p>
        {args.consequences && args.consequences.length > 0 && (
          <ul style={{ margin: 'var(--vv-s-10) 0 0', padding: 'var(--vv-s-10) var(--vv-s-12) var(--vv-s-10) var(--vv-s-26)', borderRadius: 'var(--radius-tile)', background: 'var(--color-surface-1)', border: '1px solid var(--line-1)', fontSize: 12.5, display: 'flex', flexDirection: 'column', gap: 'var(--vv-s-7)' }}>
            {args.consequences.map((item) => <li key={item}>{item}</li>)}
          </ul>
        )}
        <p style={{ margin: 'var(--vv-s-10) 0 0', fontSize: 12, color: 'var(--color-fg-4)' }}>
          {running ? c.runningHint(args.runningCount ?? 0) : c.idleHint}
        </p>
      </>
    ),
    phrase: args.name,
    // Kênh còn việc đang chạy → khoá hẳn nút (click/giữ/Enter), không chỉ cảnh báo.
    disabledReason: running ? c.runningHint(args.runningCount ?? 0) : undefined,
    cancelLabel: t.common.cancel,
    confirmLabel: c.ok,
    pendingLabel: c.busy,
    successMessage: c.done,
    onConfirm: args.onConfirm,
  })
}

/** stopAll — Dừng mọi việc đang chạy (giữ để xác nhận). */
export function confirmStopAll(args: {
  running: number
  onConfirm: () => void | Promise<void>
}): Promise<boolean> {
  const c = t.confirm.stopAll
  return open({
    tone: 'dsoft',
    hold: true,
    title: c.title,
    body: c.body(args.running),
    cancelLabel: t.common.cancel,
    confirmLabel: c.holdLabel,
    successMessage: c.done(args.running),
    onConfirm: args.onConfirm,
  })
}

/** leave — Rời khi chưa lưu (nút phụ Bỏ thay đổi cam nhạt). */
export function confirmLeaveUnsaved(args: {
  changed: number
  template: string
  onConfirm: () => void | Promise<void>
  onDiscard: () => void | Promise<void>
}): Promise<boolean> {
  const c = t.confirm.leave
  return open({
    title: c.title,
    body: c.body(args.changed, args.template),
    cancelLabel: c.cancel,
    confirmLabel: c.ok,
    pendingLabel: c.busy,
    secondary: { label: c.alt, danger: true, onConfirm: args.onDiscard },
    successMessage: c.done(args.changed),
    onConfirm: args.onConfirm,
  })
}

