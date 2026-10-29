// CocreatePanel — panel đồng sáng tác trực tiếp trong Workspace khi engine bật
// (`GET /cocreate` active, GAP-6): hội thoại · gợi ý · bản nháp · bắt đầu/huỷ.
import type { CoCreateState } from '../../api/types'
import { t } from '../../i18n/vi'
import { AgentOrb } from '@noverse/ui'
import { Button } from '@noverse/ui'
import { Badge } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { Scroll } from '@noverse/ui'
import { Shimmer } from '@noverse/ui'

export function CocreatePanel({ state, liveText, busy, pending = false, success = false, onSend, onCommit, onCancel }: {
  state: CoCreateState
  /** delta `cocreate` đang stream (nối vào câu trả lời cuối) */
  liveText: string
  busy: boolean
  pending?: boolean
  success?: boolean
  onSend: (text: string) => void
  onCommit: () => void
  onCancel: () => void
}) {
  if (!state.active) return null
  const messages = state.messages.map((message, index) => {
    const last = index === state.messages.length - 1
    return {
      ...message,
      content: last && message.role === 'assistant' && liveText ? `${message.content}${liveText}` : message.content,
    }
  })
  return (
    <section className="card" aria-label={t.workspace.cocreate} style={{
      flex: '0 0 auto', borderRadius: 14, padding: 12, display: 'flex', flexDirection: 'column', gap: 10,
    }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
        <AgentOrb agent="architect" size={18} live={busy} />
        <span style={{ flex: 1, fontSize: 13, fontWeight: 500 }}>{t.workspace.cocreate}</span>
        <Badge tone={state.ready ? 'ok' : 'neutral'}>{state.ready ? t.workspace.cocreateReady : t.workspace.cocreateDrafting}</Badge>
      </div>
      {messages.length > 0 && (
        <Scroll style={{ maxHeight: 168, display: 'flex', flexDirection: 'column', gap: 8 }}>
          {messages.map((message, index) => (
            <div key={index} style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
              <span style={{ fontSize: 11, fontWeight: 600, color: message.role === 'user' ? 'var(--color-user)' : 'var(--color-accent)' }}>
                {message.role === 'user' ? t.workspace.cocreateYou : t.workspace.cocreateAi}
              </span>
              <p style={{ margin: 0, fontSize: 13, color: 'var(--color-fg-2)', whiteSpace: 'pre-wrap' }}>
                {message.content}
                {busy && index === messages.length - 1 && message.role === 'assistant' && <Shimmer>▍</Shimmer>}
              </p>
            </div>
          ))}
        </Scroll>
      )}
      {state.suggestions.length > 0 && (
        <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
          {state.suggestions.map((suggestion, index) => (
            <button key={suggestion} type="button" className="hov-ghost" disabled={busy}
              onClick={() => onSend(suggestion)} style={{
                height: 28, padding: '0 10px', borderRadius: 999, border: '1px solid var(--line-4)',
                background: 'transparent', color: 'var(--color-fg-2)', fontSize: 12,
              }}>
              {['❶', '❷', '❸'][index] ?? '•'} {suggestion}
            </button>
          ))}
        </div>
      )}
      {state.draft !== '' && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
          <span style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>{t.workspace.cocreateDraft}</span>
          <p style={{
            margin: 0, padding: '8px 10px', borderRadius: 8, background: 'var(--color-surface-0)',
            border: '1px solid var(--line-2)', fontSize: 13, color: 'var(--color-fg-2)', whiteSpace: 'pre-wrap',
          }}>{state.draft}</p>
        </div>
      )}
      <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 8 }}>
        <Button icon={<Icon name="close-circle" size={15} />} onClick={onCancel} disabled={busy} pending={pending} success={success}>{t.workspace.cocreateCancel}</Button>
        <Button variant="primary" icon={<Icon name="magic-star" size={15} />} onClick={onCommit} disabled={busy || !state.ready} pending={pending} success={success}>
          {t.workspace.cocreateStart}
        </Button>
      </div>
    </section>
  )
}
