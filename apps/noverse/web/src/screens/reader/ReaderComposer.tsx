// QuickActions + ô "Ý kiến sửa chương" của Trình đọc. QuickActions điền sẵn ô chat
// Workspace qua setComposerDraft() rồi mới điều hướng — không tự gửi (F0 contract).
// Mobile: ReaderScreen đặt khối này ngoài pane cuộn nên nó neo đáy, không phủ lên bài đọc.
import { useState, type FormEvent, type RefObject } from 'react'
import { sendInput } from '../../api/client'
import { t } from '../../i18n/vi'
import { paths, navigate } from '../../router'
import { setComposerDraft } from '../../shell'
import { Icon } from '@noverse/ui'
import { copy } from './copy'
import { keys, useAction, useConnection } from '../../data'
import { Button } from '@noverse/ui'

export function ReaderComposer({ project, story, chapter, writable, locked, value, onChange, inputRef, narrow }: {
  project: string
  story: string
  chapter: number
  writable: boolean
  locked: boolean
  value: string
  onChange: (value: string) => void
  inputRef: RefObject<HTMLInputElement | null>
  narrow: boolean
}) {
  const connection = useConnection()
  const action = useAction(async (text: string) => {
    const result = await sendInput(project, story, text)
    onChange('')
    return result
  }, { success: copy.sent(chapter), invalidate: [keys.story(project, story)] })
  const [focused, setFocused] = useState(false)

  const quick = [
    { icon: 'refresh-2', label: copy.quick.rewrite(chapter), text: `viết lại chương ${chapter}` },
    { icon: 'magicpen', label: copy.quick.polish(chapter), text: `gọt giũa chương ${chapter}: ` },
    { icon: 'shield-tick', label: copy.quick.review(), text: `thẩm duyệt lại chương ${chapter}` },
    { icon: 'code', label: copy.quick.sync, text: '/sync' },
  ]

  const useQuick = (text: string) => {
    setComposerDraft(project, story, text)
    navigate(paths.workspace(project, story))
  }

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    const text = value.trim()
    if (!text || action.busy || !writable || !connection.writable) return
    await action.run(text)
  }

  return (
    <div style={{
      flex: '0 0 auto', padding: narrow ? '8px 12px 12px' : '8px 20px 12px', borderTop: '1px solid var(--line-1)',
      display: 'flex', flexDirection: 'column', gap: 8,
      background: narrow ? 'var(--color-bg)' : undefined,
    }}>
      {writable && (
        <div className={narrow ? 'scroll-x' : undefined}
          style={{ display: 'flex', flexWrap: narrow ? 'nowrap' : 'wrap', gap: 6, overflowX: narrow ? 'auto' : undefined }}>
          {quick.map((action) => (
            <button key={action.text} type="button" className="hov-ghost" onClick={() => useQuick(action.text)}
              style={{
                flex: narrow ? '0 0 auto' : undefined,
                height: 30, padding: '0 10px 0 8px', display: 'flex', alignItems: 'center', gap: 6,
                borderRadius: 8, border: '1px solid var(--line-3)', background: 'var(--color-surface-1)',
                color: 'var(--color-fg-2)', fontSize: 12,
              }}>
              <Icon name={action.icon} size={14} />{action.label}
            </button>
          ))}
        </div>
      )}

      {writable ? (
        <form className="card kit-frame" aria-label={copy.composerLabel} onSubmit={submit}
          onFocus={() => setFocused(true)} onBlur={() => setFocused(false)}
          style={{
            borderRadius: 18, padding: '7px 7px 7px 14px', display: 'flex', alignItems: 'center', gap: 10,
            background: 'var(--color-surface-3)',
            borderColor: focused ? 'var(--line-focus)' : undefined,
            transition: 'border-color .15s var(--ease-out)',
          }}>
          <Icon name="messages-2" size={18} style={{ color: 'var(--color-fg-4)' }} />
          <label htmlFor="reader-fb" className="sr-only">{copy.composerLabel}</label>
          <input id="reader-fb" ref={inputRef} type="text" value={value}
            onChange={(event) => onChange(event.target.value)}
            placeholder={t.placeholder.reader(chapter)}
            style={{
              flex: 1, minWidth: 0, height: 38, background: 'transparent', border: 'none',
              outline: 'none', color: 'var(--color-fg)', fontSize: 14,
            }} />
          <Button type="submit" variant="icon" aria-label={copy.send} title={connection.writable ? copy.send : 'Mất kết nối · Không gửi được'} pending={action.pending} success={action.success} disabled={action.busy || !value.trim() || !connection.writable} style={{ width: 38, height: 38, borderRadius: '50%', flex: '0 0 auto', background: 'var(--color-fg)', color: 'var(--color-bg)' }}>
            <Icon name="arrow-up" size={20} />
          </Button>
        </form>
      ) : (
        <div className="card" style={{
          borderRadius: 14, padding: '10px 14px', display: 'flex', alignItems: 'center', gap: 10,
          color: 'var(--color-fg-4)', fontSize: 12,
        }}>
          <Icon name="shield-security" size={16} />
          <span>{locked ? copy.lockedNote : copy.readOnlyNote}</span>
        </div>
      )}
    </div>
  )
}
