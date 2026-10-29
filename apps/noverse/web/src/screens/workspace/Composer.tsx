// Composer — ô chat neo đáy (Main.dc.html): icon · textarea tự cao ≤6 dòng ·
// đếm n/5000 · nút gửi tròn. Khi agent chạy: RunRing (viền conic + quầng blur).
// Gõ `/` → SlashPopover (Slash.dc.html, G11) ngay trên ô chat; ghost text phần còn lại
// của tên lệnh, viền đỏ + chip “Chưa chạy được” khi cú pháp sai (13-commands §1.2–1.3).
// Placeholder theo trạng thái snapshot (07-copy.md). ShortcutHints nằm dưới ô chat.
import { useEffect, useRef, useState, type CSSProperties, type KeyboardEvent } from 'react'
import type { Snapshot } from '../../api/types'
import { t } from '../../i18n/vi'
import { Icon } from '@noverse/ui'
import { Button, Kbd } from '@noverse/ui'
import { TextArea } from '@noverse/ui'
import { handleSlashKey, rememberSlashLine, useSlashHints } from '../../commands/store'
import { SlashPopover, SlashSentToast } from '../../commands/SlashPopover'
import { formatTokens, formatUSD } from '../../lib/format'
import { percentOf } from './format'
import { isCompleted, isGate } from './GateCard'

export function composerPlaceholder(snapshot: Snapshot | null, cocreateActive: boolean, mobile: boolean): string {
  if (cocreateActive) return t.placeholder.cocreate
  if (!snapshot) return t.placeholder.running
  // Gói P · P4: Trọng tài đang chờ trả lời — mời trả lời, kèm ví dụ từ phương án đầu.
  const ask = snapshot.pendingAsk
  if (ask) {
    const example = (ask.options ?? [])[0]?.label?.trim()
    return example ? `${t.idle.replyPlaceholder.replace('…', '')}${example}` : t.idle.replyPlaceholder
  }
  if (isCompleted(snapshot)) return t.placeholder.completed
  if (isGate(snapshot)) return t.placeholder.gate
  // Gói P · P1: lập kế hoạch — ô nhập mời góp ý kế hoạch; đang tạm dừng thì mời tiếp tục.
  if (snapshot.plan) return snapshot.runtimeState === 'paused' ? t.plan.composerPaused : t.plan.composerHint
  if (snapshot.isRunning) return mobile ? t.placeholder.composerMobile : t.placeholder.running
  if (snapshot.runtimeState === 'pausing') return t.placeholder.pausing
  if (snapshot.runtimeState === 'paused') return t.placeholder.paused
  if (snapshot.completedCount > 0) return t.placeholder.interrupted
  return t.placeholder.running
}

function RunRing() {
  // `from var(--run-angle)` PHẢI nằm trên chính hai lớp này: nếu đặt gradient đó trong token
  // `defineVars` (phát vào :root) thì var() được thay một lần tại :root và viền đứng hình —
  // animation `ai-run` vẫn “chạy” nhưng không đổi màu góc. Xem styles/motion.test.ts.
  const ring: CSSProperties = {
    position: 'absolute', pointerEvents: 'none',
    background: 'conic-gradient(from var(--run-angle), var(--ring-arc))',
    animation: 'ai-run 4.5s linear infinite',
  }
  return (
    <>
      <div aria-hidden="true" style={{ ...ring, inset: -2, borderRadius: 20, filter: 'blur(16px)', opacity: 0.45 }} />
      <div aria-hidden="true" style={{
        ...ring, inset: -1, borderRadius: 19, padding: 1.5,
        WebkitMask: 'linear-gradient(#000 0 0) content-box, linear-gradient(#000 0 0)',
        WebkitMaskComposite: 'xor', maskComposite: 'exclude',
      }} />
    </>
  )
}

export function Composer({ value, onChange, onSubmit, onPause, running, writable, canWrite, mobile, placeholder, snapshot = null, busy = false, pending = false, success = false }: {
  value: string
  onChange: (value: string) => void
  /** Enter/gửi: `line` là dòng popover đã ghép (nếu có), ngược lại là text ô chat */
  onSubmit: (line?: string) => void
  onPause: () => void
  running: boolean
  /** gửi được ngay không (quyền viết ∧ kết nối) — chỉ dùng cho ô chat/nút Gửi */
  writable: boolean
  /** chỉ quyền viết truyện này (không tính kết nối) — ẩn lệnh cần quyền viết; mặc định = `writable` */
  canWrite?: boolean
  mobile: boolean
  placeholder: string
  /** dữ liệu cho popover `/` (tiến độ, tên truyện, đang chạy) */
  snapshot?: Snapshot | null
  busy?: boolean
  pending?: boolean
  success?: boolean
}) {
  const ref = useRef<HTMLTextAreaElement>(null)
  // Slash.dc.html: viên “Đã chạy /lệnh” 2,6 giây sau khi gửi dòng `/…`.
  const [sentLine, setSentLine] = useState<string | null>(null)
  const sentTimer = useRef<number | undefined>(undefined)
  useEffect(() => () => window.clearTimeout(sentTimer.current), [])
  const slash = value.startsWith('/')
  // Ghost text + lỗi cú pháp do registry lệnh quyết định (13-commands §1.2–1.3).
  // Lệnh cần quyền viết (vd `/reopen`) ẩn theo QUYỀN, không theo kết nối: người viết đang mất
  // mạng vẫn thấy lệnh và nhận lỗi kết nối bình thường, không bị báo “chỉ có quyền đọc”.
  const writableByRole = canWrite ?? writable
  const { ghost, hasError } = useSlashHints(slash ? value : '', snapshot, writableByRole)
  const mono: CSSProperties = slash ? { fontFamily: 'var(--font-mono)', letterSpacing: 0 } : {}
  const textStyle: CSSProperties = {
    fontSize: mobile ? 16 : 14, lineHeight: 1.5, ...mono,
  }

  useEffect(() => {
    const node = ref.current
    if (!node) return
    node.style.height = 'auto'
    node.style.height = `${Math.min(node.scrollHeight, 6 * 21)}px`
  }, [value])

  const send = (line?: string) => {
    const sent = (line ?? value).trim()
    if (sent.startsWith('/') && writable && !busy && !hasError) {
      rememberSlashLine(sent)
      setSentLine(sent)
      window.clearTimeout(sentTimer.current)
      sentTimer.current = window.setTimeout(() => setSentLine(null), 2600)
    }
    onSubmit(line)
  }

  const onKeyDown = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    if (event.nativeEvent.isComposing) return
    // ↑ ↓ Tab Enter Esc của popover (13-commands §1.2) — xử lý trước mọi phím khác.
    const handled = handleSlashKey(event, value, snapshot, undefined, writableByRole)
    if (handled.handled) {
      event.preventDefault()
      if (handled.next !== undefined) onChange(handled.next)
      if (handled.submit !== undefined) send(handled.submit)
      return
    }
    if (event.key === 'j' && event.ctrlKey) {
      event.preventDefault()
      const node = event.currentTarget
      const start = node.selectionStart ?? value.length
      const end = node.selectionEnd ?? start
      onChange(`${value.slice(0, start)}\n${value.slice(end)}`)
      requestAnimationFrame(() => { node.selectionStart = start + 1; node.selectionEnd = start + 1 })
      return
    }
    if (event.key === 'Enter' && (event.ctrlKey || event.altKey)) return
    if (event.key === 'Enter' && !event.shiftKey) {
      event.preventDefault()
      send()
    }
  }

  const canSend = writable && !busy && value.trim() !== '' && value.length <= 5000
  const showPause = mobile && running && !canSend
  return (
    <form aria-label={t.placeholder.composerSr} onSubmit={(event) => { event.preventDefault(); send() }}
      style={{ position: 'relative', flex: '0 0 auto' }}>
      <SlashPopover value={value} onChange={onChange} onSubmit={(line) => send(line)} snapshot={snapshot} writable={writableByRole} />
      {sentLine && <SlashSentToast line={sentLine} />}
      {running && <RunRing />}
      <div className="card kit-frame" style={{
        position: 'relative', borderRadius: 18, padding: '7px 7px 7px 14px', display: 'flex',
        alignItems: 'flex-end', gap: 10, background: 'var(--color-surface-3)',
        // Slash.dc.html: cmpBd — tím khi gõ lệnh, đỏ nhạt khi cú pháp sai
        ...(hasError ? { borderColor: 'rgba(255,159,168,0.35)' } : slash ? { borderColor: 'rgba(201,166,245,0.35)' } : {}),
      }}>
        <Icon name={slash ? 'command' : 'messages-2'} size={18} style={{ color: slash ? '#C9A6F5' : 'var(--color-fg-4)', marginBottom: mobile ? 11 : 9 }} />
        <label htmlFor="workspace-composer" className="sr-only">{t.placeholder.composerSr}</label>
        <div style={{ position: 'relative', flex: 1, minWidth: 0 }}>
          {ghost !== '' && (
            <div aria-hidden="true" style={{
              position: 'absolute', inset: 0, padding: mobile ? '9px 0' : '8.5px 0', whiteSpace: 'pre', overflow: 'hidden',
              pointerEvents: 'none', color: 'transparent', ...textStyle,
            }}>
              <span style={{ visibility: 'hidden' }}>{value}</span>
              {/* 13-commands §3: ghost text #5E5E68 (tokens.css chưa có biến riêng) */}
              <span style={{ color: '#5E5E68' }}>{ghost}</span>
            </div>
          )}
          <TextArea id="workspace-composer" ref={ref} rows={1} value={value} maxLength={5000} showCount={false} bare
            onChange={(event) => onChange(event.target.value)} onKeyDown={onKeyDown}
            placeholder={placeholder} disabled={!writable} invalid={hasError}
            style={{
              display: 'block', width: '100%', minWidth: 0, minHeight: mobile ? 40 : 38, maxHeight: 6 * 21, resize: 'none',
              background: 'transparent', border: 'none', outline: 'none', color: 'var(--color-fg)',
              padding: mobile ? '9px 0' : '8.5px 0', ...textStyle,
            }} />
        </div>
        {hasError && (
          <span style={{
            height: 24, padding: '0 8px', borderRadius: 7, background: 'var(--tint-err)', color: '#FFC9CE',
            fontSize: 11.5, display: 'flex', alignItems: 'center', gap: 5, whiteSpace: 'nowrap', flex: '0 0 auto',
            marginBottom: mobile ? 8 : 6,
          }}>
            <Icon name="warning-2" size={13} />
            {t.workspace.notRunnable}
          </span>
        )}
        {!slash && <span className="num mono" style={{ fontSize: 11, color: 'var(--color-fg-4)', marginBottom: mobile ? 12 : 10 }}>{value.length}/5000</span>}
        {showPause ? (
          <Button type="button" onClick={onPause} aria-label={t.workspace.pause} tooltip={`${t.workspace.pause} · Esc`} disabled={busy}
            style={{
              width: 40, height: 40, borderRadius: 12, border: 'none', background: 'var(--color-fg)',
              color: 'var(--color-bg)', display: 'flex', alignItems: 'center', justifyContent: 'center', flex: '0 0 auto', padding: 0,
            }} icon={<Icon name="pause" size={18} />} />
        ) : (
          <Button type="submit" aria-label={t.workspace.send} tooltip={`${t.workspace.send} · Enter / ⌘S`}
            disabled={!canSend || hasError} pending={pending} success={success} style={{
              width: mobile ? 40 : 38, height: mobile ? 40 : 38, borderRadius: '50%', border: 'none', padding: 0,
              background: hasError ? '#3A3A40' : 'var(--color-fg)', color: 'var(--color-bg)', display: 'flex',
              alignItems: 'center', justifyContent: 'center', flex: '0 0 auto',
            }} icon={<Icon name="arrow-up" size={20} />} />
        )}
      </div>
    </form>
  )
}

/**
 * Tách "<phím> <nhãn>" → ['<phím>', '<nhãn>']: phím là phần ngắn nhất đứng trước một từ bắt đầu bằng
 * chữ thường, nên phím có dấu cách vẫn nguyên một ô (Main.dc.html: `<span class="kbd">[ ]</span>bảng bên`).
 */
export const splitHint = (text: string): [string, string] => {
  const match = /^(.+?)\s+(\p{Ll}.*)$/u.exec(text)
  return match ? [match[1], match[2]] : [text, '']
}

export function ShortcutHints({ snapshot, running }: { snapshot: Snapshot | null; running: boolean }) {
  const hints = [t.shortcuts.command, t.shortcuts.send, t.shortcuts.newline, t.shortcuts.pause, t.shortcuts.panels]
  const cachePercent = percentOf(
    snapshot?.overallRecentSamples ? snapshot.overallRecentCacheRead : snapshot?.totalCacheReadTokens,
    snapshot?.overallRecentSamples ? snapshot.overallRecentInput : snapshot?.totalInputTokens,
  )
  return (
    <div style={{ display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: '4px 14px', padding: '0 6px', fontSize: 12, color: 'var(--color-fg-4)' }}>
      <Icon name="command" size={14} />
      {hints.map((hint) => {
        const [key, label] = splitHint(hint)
        return (
          <span key={hint} style={{ display: 'flex', gap: 6, alignItems: 'center' }}>
            <Kbd>{key}</Kbd>{label}
          </span>
        )
      })}
      <span style={{ flex: 1 }} />
      <span className="num" title="Token gửi đi · nhận về · tỉ lệ dùng lại bộ nhớ đệm · chi phí" style={{ display: 'flex', alignItems: 'center', gap: 6, whiteSpace: 'nowrap' }}>
        <Icon name="cloud-connection" size={14} style={{ color: running ? 'var(--color-accent)' : 'var(--color-ok)' }} />
        {formatTokens(snapshot?.totalInputTokens, 'short')} vào · {formatTokens(snapshot?.totalOutputTokens, 'short')} ra · <span style={{ color: 'var(--color-ok)' }}>đệm {cachePercent}%</span> · {formatUSD(snapshot?.totalCostUSD)}
      </span>
    </div>
  )
}
