// Nội dung chi tiết một lệnh (Commands.dc.html cột giữa, trạng thái detail).
import type { CommandSpec, Snapshot } from '../../api/types'
import {
  commandBadges, composeTokens, emptyValues, usageTokens, writingChapterOf,
  type CommandValues,
} from '../../commands/model'
import { BADGE_TONE, CMD_COLOR } from '../../commands/tokens'
import { CMD_UI, copyOf, groupLabel, type CmdExample } from '../../i18n/commands.vi'
import { AgentOrb } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { ErrorMessage } from './parts'

const BADGE_ICON: Record<string, string> = {
  swap: 'arrow-swap-horizontal', flash: 'flash', edit: 'edit-2',
  timer: 'timer-1', info: 'info-circle', tick: 'tick-circle',
}

const LOG_TIMES = ['21:14', '21:15', '21:16', '21:17']

const exampleValues = (spec: CommandSpec, example: CmdExample): CommandValues['values'] => {
  const values = emptyValues(spec).values
  for (const [key, value] of Object.entries(example.vals)) values[key] = value
  return values
}

export function CommandDetail({ spec, snapshot, copiedId, onCopy, onTry, onPick, needsWrite }: {
  spec: CommandSpec
  snapshot: Snapshot | null
  copiedId: string | null
  onCopy: (text: string, id: string) => void
  onTry: (values: CommandValues['values']) => void
  onPick: (name: string) => void
  /** `/reopen` khi tài khoản không có quyền viết truyện đang mở (13-commands.md §/reopen). */
  needsWrite?: boolean
}) {
  const copy = copyOf(spec.name)
  const busy = Boolean(snapshot?.isRunning)
  const chapter = writingChapterOf(snapshot, snapshot?.completedCount ?? 0)
  const badges = commandBadges(spec, busy, true)
  if (needsWrite) badges.push({ icon: 'edit-2', label: CMD_UI.needsWrite, tone: 'warn' })

  return (
    <article className="rise" aria-labelledby="cmd-h" style={{
      maxWidth: 820, padding: '26px 36px 56px', display: 'flex', flexDirection: 'column', gap: 20,
    }}>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
        <span className="lbl">{groupLabel(spec.group)}</span>
        <div style={{ display: 'flex', alignItems: 'center', gap: 12, flexWrap: 'wrap' }}>
          <h2 id="cmd-h" className="mono" style={{
            margin: 0, fontSize: 28, fontWeight: 500, letterSpacing: '-0.02em', color: 'var(--cmd-name)',
          }}>/{spec.name}</h2>
          <span style={{ fontSize: 20, fontWeight: 600, letterSpacing: '-0.01em' }}>
            {copy?.title ?? spec.usage}
          </span>
        </div>
        <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
          {badges.map((badge) => {
            const tone = BADGE_TONE[badge.tone]
            return (
              <span key={badge.label} style={{
                height: 24, padding: '0 9px', borderRadius: 7, background: tone.bg,
                border: `1px solid ${tone.bd}`, color: tone.fg, fontSize: 12,
                display: 'flex', alignItems: 'center', gap: 6, whiteSpace: 'nowrap',
              }}>
                <Icon name={BADGE_ICON[badge.icon] ?? 'info-circle'} size={14} />
                {badge.label}
              </span>
            )
          })}
        </div>
        {copy?.lead && (
          <p style={{ margin: '2px 0 0', fontSize: 14.5, color: '#B4B4BE', maxWidth: 680 }}>{copy.lead}</p>
        )}
      </div>

      {busy && (
        <div role="status" style={{
          display: 'flex', gap: 12, alignItems: 'flex-start', padding: '12px 14px', borderRadius: 12,
          background: 'rgba(246,203,140,0.07)', border: '1px solid rgba(246,203,140,0.25)',
        }}>
          <AgentOrb agent="writer" size={22} style={{ marginTop: 1 }} />
          <span style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
            <span style={{ fontSize: 13.5, fontWeight: 500, color: 'var(--color-warn)' }}>{CMD_UI.busyTitle(chapter)}</span>
            <span style={{ fontSize: 12.5, color: 'var(--color-fg-2)' }}>
              {spec.needsIdle ? CMD_UI.busyIdleText(chapter) : CMD_UI.busyUsableText}
            </span>
          </span>
        </div>
      )}

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(2, minmax(0, 1fr))', gap: 12 }}>
        <section className="card" style={{ borderRadius: 14, padding: '14px 16px', display: 'flex', flexDirection: 'column', gap: 8 }}>
          <h3 style={{ margin: 0, fontSize: 13, fontWeight: 600, display: 'flex', alignItems: 'center', gap: 8 }}>
            <Icon name="tick-circle" size={16} style={{ color: 'var(--color-ok)' }} />{CMD_UI.whenTitle}
          </h3>
          <ul style={{ margin: 0, padding: '0 0 0 18px', display: 'flex', flexDirection: 'column', gap: 4, fontSize: 13, color: 'var(--color-fg-2)' }}>
            {(copy?.when ?? []).map((line) => <li key={line}>{line}</li>)}
          </ul>
        </section>
        <section className="card" style={{ borderRadius: 14, padding: '14px 16px', display: 'flex', flexDirection: 'column', gap: 8 }}>
          <h3 style={{ margin: 0, fontSize: 13, fontWeight: 600, display: 'flex', alignItems: 'center', gap: 8 }}>
            <Icon name="info-circle" size={16} style={{ color: 'var(--color-warn)' }} />{CMD_UI.avoidTitle}
          </h3>
          <ul style={{ margin: 0, padding: '0 0 0 18px', display: 'flex', flexDirection: 'column', gap: 4, fontSize: 13, color: 'var(--color-fg-2)' }}>
            {(copy?.avoid ?? []).map((line) => <li key={line}>{line}</li>)}
          </ul>
        </section>
      </div>

      <section aria-labelledby="h-syn" style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
        <h3 id="h-syn" style={{ margin: 0, fontSize: 15, fontWeight: 600 }}>{CMD_UI.syntaxHeading}</h3>
        <div className="card" style={{ borderRadius: 14, overflow: 'hidden' }}>
          <div className="mono" style={{
            padding: '14px 16px', fontSize: 14.5, background: 'var(--color-surface-0)',
            borderBottom: '1px solid var(--line-1)', whiteSpace: 'pre-wrap', wordBreak: 'break-word',
          }}>
            {usageTokens(spec).map((token, index) => (
              <span key={index} style={{ color: CMD_COLOR[token.tone] }}>{index ? ' ' : ''}{token.text}</span>
            ))}
          </div>
          {spec.params.length === 0 && (
            <div style={{ padding: '14px 16px', fontSize: 13, color: 'var(--color-fg-3)' }}>{CMD_UI.noParams}</div>
          )}
          {spec.params.map((param, index) => {
            const text = copy?.params[param.key]
            const color = param.kind === 'flag' || param.prefix ? 'var(--cmd-flag)' : 'var(--cmd-value)'
            return (
              <div key={param.key} style={{
                display: 'grid', gridTemplateColumns: '170px minmax(0, 1fr)', gap: 16,
                padding: '12px 16px', borderTop: '1px solid var(--line-1)',
              }}>
                <div style={{ display: 'flex', flexDirection: 'column', gap: 6, alignItems: 'flex-start' }}>
                  <span className="mono" style={{ fontSize: 13, color, whiteSpace: 'nowrap' }}>{param.token}</span>
                  <span style={{
                    height: 20, padding: '0 7px', borderRadius: 6, fontSize: 11, display: 'flex',
                    alignItems: 'center', whiteSpace: 'nowrap',
                    background: param.required ? 'rgba(166,237,193,0.12)' : 'rgba(255,255,255,0.05)',
                    color: param.required ? 'var(--cmd-value)' : 'var(--color-fg-3)',
                  }}>{param.required ? CMD_UI.required : CMD_UI.optional}</span>
                </div>
                <div style={{ display: 'flex', flexDirection: 'column', gap: 6, minWidth: 0 }}>
                  <span style={{ fontSize: 13, color: '#D6D6DC' }}>{text?.desc ?? param.token}</span>
                  {(param.options ?? []).length > 0 && (
                    <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
                      {(param.options ?? []).map((option) => (
                        <div key={option.value} style={{ display: 'flex', gap: 10, fontSize: 12.5 }}>
                          <span className="mono" style={{ flex: '0 0 86px', color: 'var(--cmd-value)' }}>{option.value}</span>
                          <span style={{ color: 'var(--color-fg-3)' }}>{option.note}</span>
                        </div>
                      ))}
                    </div>
                  )}
                  {text?.defText && <span style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>{text.defText}</span>}
                </div>
              </div>
            )
          })}
        </div>
      </section>

      {(copy?.ex ?? []).length > 0 && (
        <section aria-labelledby="h-ex" style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
          <h3 id="h-ex" style={{ margin: 0, fontSize: 15, fontWeight: 600 }}>{CMD_UI.examplesTitle}</h3>
          {(copy?.ex ?? []).map((example, index) => {
            const values = exampleValues(spec, example)
            const tokens = composeTokens(spec, values, example.slash)
            const line = tokens.map((token) => token.text).join('')
            const id = `${spec.name}:ex${index}`
            const copied = copiedId === id
            return (
              <div key={index} className="card" style={{
                borderRadius: 14, padding: '12px 12px 12px 16px', display: 'flex',
                flexDirection: 'column', gap: 8,
              }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                  <div className="mono" style={{
                    flex: '1 1 auto', minWidth: 0, fontSize: 13.5, whiteSpace: 'pre-wrap', wordBreak: 'break-word',
                  }}>
                    {tokens.map((token, i) => (
                      <span key={i} style={{ color: CMD_COLOR[token.tone] }}>{token.text}</span>
                    ))}
                  </div>
                  <button
                    type="button"
                    onClick={() => onCopy(line, id)}
                    aria-label={CMD_UI.copyAria}
                    className="hov-ghost"
                    style={{
                      height: 30, padding: '0 10px', borderRadius: 8, border: '1px solid var(--line-3)',
                      background: 'transparent', color: copied ? 'var(--color-ok)' : 'var(--color-fg-2)',
                      fontSize: 12, display: 'flex', alignItems: 'center', gap: 6,
                      whiteSpace: 'nowrap', flex: '0 0 auto',
                    }}
                  >
                    <Icon name={copied ? 'tick-circle' : 'copy'} size={14} />
                    {copied ? CMD_UI.copied : CMD_UI.copy}
                  </button>
                  {spec.params.length > 0 && (
                    <button type="button" onClick={() => onTry(values)} className="hov-ghost" style={{
                      height: 30, padding: '0 10px', borderRadius: 8, border: '1px solid var(--line-3)',
                      background: 'transparent', color: 'var(--color-fg)', fontSize: 12,
                      display: 'flex', alignItems: 'center', gap: 6, whiteSpace: 'nowrap', flex: '0 0 auto',
                    }}>
                      {CMD_UI.tryIt}<Icon name="arrow-right-2" size={14} style={{ color: 'var(--cmd-name)' }} />
                    </button>
                  )}
                </div>
                <span style={{ fontSize: 12.5, color: 'var(--color-fg-3)' }}>{example.text}</span>
              </div>
            )
          })}
        </section>
      )}

      {(copy?.after ?? []).length > 0 && (
        <section aria-labelledby="h-after" style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
          <h3 id="h-after" style={{ margin: 0, fontSize: 15, fontWeight: 600 }}>{CMD_UI.afterTitle}</h3>
          <div className="card" style={{ borderRadius: 14, padding: '14px 16px', display: 'flex', flexDirection: 'column', gap: 12 }}>
            <ol style={{ margin: 0, padding: 0, listStyle: 'none', display: 'flex', flexDirection: 'column', gap: 8 }}>
              {(copy?.after ?? []).map((line, index) => (
                <li key={line} style={{ display: 'flex', gap: 10, alignItems: 'flex-start', fontSize: 13, color: 'var(--color-fg-2)' }}>
                  <span className="num" style={{
                    width: 20, height: 20, borderRadius: '50%', background: 'var(--color-surface-6)',
                    display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: 11,
                    color: 'var(--color-fg-3)', flex: '0 0 auto', marginTop: 1,
                  }}>{index + 1}</span>
                  <span>{line}</span>
                </li>
              ))}
            </ol>
            {(copy?.log ?? []).length > 0 && (
              <div style={{
                borderRadius: 10, background: 'var(--color-surface-0)', border: '1px solid rgba(255,255,255,0.05)',
                padding: '8px 10px', display: 'flex', flexDirection: 'column', gap: 4,
              }}>
                <span className="lbl" style={{ fontSize: 10.5, padding: '2px 0 4px' }}>{CMD_UI.logTitle}</span>
                {(copy?.log ?? []).map((entry, index) => (
                  <div key={index} style={{ display: 'flex', gap: 10, alignItems: 'baseline', fontSize: 12.5 }}>
                    <span className="mono num" style={{ fontSize: 11, color: '#5E5E68', flex: '0 0 auto' }}>
                      {LOG_TIMES[index] ?? LOG_TIMES[LOG_TIMES.length - 1]}
                    </span>
                    <span className="mono" style={{
                      fontSize: 10.5, padding: '0 5px', borderRadius: 4,
                      background: 'rgba(166,237,193,0.1)', color: 'var(--cmd-value)', flex: '0 0 auto',
                    }}>{entry.cat}</span>
                    <span style={{ color: 'var(--color-fg-2)' }}>{entry.text}</span>
                  </div>
                ))}
              </div>
            )}
          </div>
        </section>
      )}

      {(copy?.errs ?? []).length > 0 && (
        <section aria-labelledby="h-err" style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
          <h3 id="h-err" style={{ margin: 0, fontSize: 15, fontWeight: 600 }}>{CMD_UI.errsTitle}</h3>
          <div className="card" style={{ borderRadius: 14, padding: '4px 16px' }}>
            {(copy?.errs ?? []).map((entry, index) => (
              <div key={index} style={{
                display: 'grid', gridTemplateColumns: 'minmax(0, 1.15fr) minmax(0, 1fr)', gap: 16,
                padding: '11px 0', borderTop: index ? '1px solid var(--line-1)' : undefined,
              }}>
                <span style={{
                  fontSize: 13, color: 'var(--cmd-error-fg)', display: 'flex', gap: 8, alignItems: 'flex-start',
                }}>
                  <Icon name="warning-2" size={14} style={{ marginTop: 2, color: 'var(--cmd-error-icon)' }} />
                  <span style={{ minWidth: 0, wordBreak: 'break-word' }}><ErrorMessage text={entry.msg} /></span>
                </span>
                <span style={{ fontSize: 12.5, color: 'var(--color-fg-2)' }}>{entry.fix}</span>
              </div>
            ))}
          </div>
        </section>
      )}

      <div style={{ display: 'grid', gridTemplateColumns: 'minmax(0, 1.3fr) minmax(0, 1fr)', gap: 12 }}>
        <div style={{
          borderRadius: 14, padding: '14px 16px', background: 'rgba(201,166,245,0.06)',
          border: '1px solid rgba(201,166,245,0.18)', display: 'flex', gap: 12, alignItems: 'flex-start',
        }}>
          <Icon name="lamp-on" size={18} style={{ color: 'var(--cmd-name)', marginTop: 1 }} />
          <span style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
            <span style={{ fontSize: 13, fontWeight: 600 }}>{CMD_UI.tipTitle}</span>
            <span style={{ fontSize: 12.5, color: 'var(--color-fg-2)' }}>{copy?.tip}</span>
          </span>
        </div>
        <div className="card" style={{ borderRadius: 14, padding: '14px 16px', display: 'flex', flexDirection: 'column', gap: 8 }}>
          <span style={{ fontSize: 13, fontWeight: 600 }}>{CMD_UI.relTitle}</span>
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
            {(copy?.rel ?? []).map((name) => (
              <button key={name} type="button" onClick={() => onPick(name)} className="help-chip" style={{
                height: 28, padding: '0 10px', borderRadius: 8, border: '1px solid var(--line-4)',
                background: 'var(--color-surface-0)', color: 'var(--color-fg-2)', fontSize: 12,
                display: 'flex', alignItems: 'center', gap: 6, whiteSpace: 'nowrap',
              }}>
                <span className="mono" style={{ color: 'var(--cmd-name)' }}>/{name}</span>
                {copyOf(name)?.title}
              </button>
            ))}
          </div>
        </div>
      </div>
    </article>
  )
}
