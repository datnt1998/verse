// Mảnh dùng chung của trang Hướng dẫn lệnh: ô tìm, phím tắt, dòng cuối, lỗi tải.
import type { CSSProperties, ReactNode } from 'react'
import { CMD_UI } from '../../i18n/commands.vi'
import { Kbd } from '@noverse/ui'
import { Icon } from '@noverse/ui'

export function KeysPanel() {
  return (
    <section className="card" aria-labelledby="h-keys" style={{
      borderRadius: 16, padding: 14, display: 'flex', flexDirection: 'column', gap: 10,
    }}>
      <h3 id="h-keys" style={{ margin: 0, fontSize: 14, fontWeight: 600, display: 'flex', alignItems: 'center', gap: 8 }}>
        <Icon name="keyboard" size={16} style={{ color: 'var(--color-fg-3)' }} />{CMD_UI.keysTitle}
      </h3>
      {CMD_UI.keys.map((section) => (
        <div key={section.label} style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
          <span className="lbl" style={{ fontSize: 10.5, paddingTop: 2 }}>{section.label}</span>
          {section.rows.map((row) => (
            <div key={row.what} style={{
              display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 10,
              minHeight: 26, fontSize: 12.5, color: 'var(--color-fg-2)',
            }}>
              <span style={{ minWidth: 0 }}>{row.what}</span>
              <span style={{ display: 'flex', gap: 4, flex: '0 0 auto' }}>
                {row.keys.map((key) => <Kbd key={key}>{key}</Kbd>)}
              </span>
            </div>
          ))}
        </div>
      ))}
    </section>
  )
}

export function FooterNote() {
  return (
    <div style={{ display: 'flex', gap: 10, alignItems: 'flex-start', padding: '0 4px', fontSize: 12, color: 'var(--color-fg-4)' }}>
      <Icon name="message-question" size={15} style={{ marginTop: 1 }} />
      <span>
        {CMD_UI.footerNoteBefore}
        <span className="mono" style={{ color: 'var(--cmd-name)' }}>{CMD_UI.footerNoteSlash}</span>
        {CMD_UI.footerNoteAfter}
        <Kbd>{CMD_UI.footerNoteKey}</Kbd>
        {CMD_UI.footerNoteTail}
      </span>
    </div>
  )
}

/** Lỗi tải registry: nói thật, có nút thử lại. */
export function HelpError({ message, onRetry }: { message: string; onRetry: () => void }) {
  return (
    <div role="status" style={{
      margin: 24, padding: 16, borderRadius: 12, border: '1px solid rgba(255,159,168,0.35)',
      background: 'var(--tint-err)', color: 'var(--cmd-error-fg)', display: 'flex',
      flexDirection: 'column', gap: 10, maxWidth: 520,
    }}>
      <span style={{ display: 'flex', gap: 8, alignItems: 'flex-start' }}>
        <Icon name="warning-2" size={16} style={{ marginTop: 2 }} />
        <span>{message}</span>
      </span>
      <button type="button" onClick={onRetry} className="hov-ghost" style={{
        alignSelf: 'flex-start', height: 30, padding: '0 12px', borderRadius: 8,
        border: '1px solid var(--line-4)', background: 'transparent', color: 'var(--color-fg)',
        fontSize: 12.5, display: 'flex', alignItems: 'center', gap: 6,
      }}>
        <Icon name="refresh-2" size={14} />{CMD_UI.retry}
      </button>
    </div>
  )
}

export function SectionCard({ children, style }: { children: ReactNode; style?: CSSProperties }) {
  return (
    <section className="card" style={{ borderRadius: 14, padding: '14px 16px', ...style }}>{children}</section>
  )
}

/**
 * Chữ Việt trong lỗi phải là font thường; chỉ tên tham số / đuôi tệp / đường dẫn ví dụ mới in mono
 * (`--overwrite`, `from=30`, `".pdf"`, `exports/tra-xanh.epub`). D3 · rà font mono 25/10.
 */
const SYNTAX_TOKEN = /--[a-z-]+|\b[a-z]+=[^\s,)]+|"[^"]*"|[\w./-]*\.(?:txt|epub|pdf)\b/gi

export function ErrorMessage({ text }: { text: string }) {
  const parts: ReactNode[] = []
  let last = 0
  for (const match of text.matchAll(SYNTAX_TOKEN)) {
    const start = match.index ?? 0
    if (start > last) parts.push(text.slice(last, start))
    parts.push(<span key={start} className="mono">{match[0]}</span>)
    last = start + match[0].length
  }
  if (last < text.length) parts.push(text.slice(last))
  return <>{parts}</>
}
