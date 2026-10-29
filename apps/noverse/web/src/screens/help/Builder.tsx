// Ghép lệnh thử (Commands.dc.html cột phải): điều khiển theo loại tham số, dòng lệnh tô màu,
// trạng thái kiểm tra giống popover, Chép + “Điền vào ô chat Workspace”.
import type { CSSProperties } from 'react'
import type { CommandSpec, Snapshot } from '../../api/types'
import {
  checkLine, composeLine, composeTokens, initialValues, tokenize, valuesOf, writingChapterOf,
  type CommandValues,
} from '../../commands/model'
import type { CommandTarget } from '../../commands/target'
import { CMD_COLOR, STATUS_COLOR } from '../../commands/tokens'
import { CMD_UI, copyOf, paramCopy } from '../../i18n/commands.vi'
import { paths } from '../../router'
import { Icon } from '@noverse/ui'

const FIELD: CSSProperties = {
  height: 34, padding: '0 10px', borderRadius: 9, background: 'var(--color-surface-0)',
  border: '1px solid var(--line-4)', color: 'var(--color-fg)', fontSize: 13,
  width: '100%',
}

export function Builder({ spec, values, onChange, snapshot, target, flash, copiedId, onCopy, onRun }: {
  spec: CommandSpec
  values: CommandValues
  onChange: (values: CommandValues) => void
  snapshot: Snapshot | null
  target: CommandTarget | null
  flash: number
  copiedId: string | null
  onCopy: (text: string, id: string) => void
  onRun: (line: string) => void
}) {
  const copy = copyOf(spec.name)
  const line = composeLine(spec, values.values)
  const tokens = composeTokens(spec, values.values)
  // bỏ token đầu (/lệnh) — valuesOf chỉ nhận phần tham số
  const parsed = valuesOf(spec, tokenize(line).slice(1))
  const completed = snapshot?.completedCount ?? 0
  const busy = Boolean(snapshot?.isRunning)
  const check = checkLine(spec, parsed.values, {
    completedCount: completed,
    bookTitle: snapshot?.bookTitle ?? '',
    isRunning: busy,
    writingChapter: writingChapterOf(snapshot, completed),
    quoted: parsed.quoted,
    extra: parsed.extra,
    progressUnknown: !snapshot,
  })
  const blocked = Boolean(check.err) || (busy && spec.needsIdle) || !target
  const blockLabel = check.err ? CMD_UI.fixFirst : busy && spec.needsIdle ? CMD_UI.waitIdle : CMD_UI.needStory
  const lineCopied = copiedId === `${spec.name}:line`
  const runPath = target ? (spec.name === 'start' ? paths.newstory(target.project) : paths.workspace(target.project, target.story)) : ''
  const set = (key: string, value: string | boolean) =>
    onChange({ ...values, values: { ...values.values, [key]: value }, quoted: { ...values.quoted, [key]: false } })

  return (
    <section aria-labelledby="h-try" className={`card${flash % 2 ? ' cmd-flash' : ''}`} style={{
      borderRadius: 16, padding: 14, display: 'flex', flexDirection: 'column', gap: 12,
    }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 8 }}>
        <h3 id="h-try" style={{ margin: 0, fontSize: 14, fontWeight: 600, display: 'flex', alignItems: 'center', gap: 8 }}>
          <Icon name="command" size={16} style={{ color: 'var(--cmd-name)' }} />{CMD_UI.builderTitle}
        </h3>
        {spec.params.length > 0 && (
          <button type="button" onClick={() => onChange(initialValues(spec))} className="hov-ghost" style={{
            height: 26, padding: '0 8px', borderRadius: 7, border: 'none', background: 'transparent',
            color: 'var(--color-fg-4)', fontSize: 12, display: 'flex', alignItems: 'center',
            gap: 5, whiteSpace: 'nowrap',
          }}>
            <Icon name="refresh-2" size={13} />{CMD_UI.reset}
          </button>
        )}
      </div>

      {spec.params.length === 0 && (
        <span style={{ fontSize: 12.5, color: 'var(--color-fg-3)' }}>{CMD_UI.builderEmpty}</span>
      )}

      {spec.params.map((param) => {
        const text = paramCopy(spec, param.key)
        const raw = values.values[param.key]
        const short = param.kind === 'flag' ? param.flag ?? param.token : param.token.replace(/^\[|\]$/g, '')
        const shortColor = param.kind === 'flag' || param.prefix ? 'var(--cmd-flag)' : 'var(--cmd-value)'
        const id = `f-${spec.name}-${param.key}`
        return (
          <div key={param.key} style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
            <div style={{ display: 'flex', alignItems: 'baseline', justifyContent: 'space-between', gap: 8 }}>
              <label htmlFor={id} style={{ fontSize: 12.5, color: '#D6D6DC' }}>{text?.label ?? param.token}</label>
              <span className="mono" style={{ fontSize: 11, color: shortColor, whiteSpace: 'nowrap' }}>{short}</span>
            </div>
            {(param.kind === 'path' || param.kind === 'text') && (
              <input
                id={id}
                type="text"
                spellCheck={false}
                className={param.kind === 'path' ? 'mono' : undefined}
                value={String(raw ?? '')}
                placeholder={text?.ph ?? ''}
                onChange={(event) => set(param.key, event.target.value)}
                style={FIELD}
              />
            )}
            {param.kind === 'num' && (
              <input
                id={id}
                type="text"
                inputMode="numeric"
                className="mono num"
                value={String(raw ?? '')}
                placeholder={text?.ph ?? ''}
                onChange={(event) => set(param.key, event.target.value)}
                style={{ ...FIELD, width: 120 }}
              />
            )}
            {param.kind === 'enum' && (
              <div id={id} role="radiogroup" aria-label={text?.label ?? param.token} style={{
                display: 'flex', gap: 4, padding: 3, borderRadius: 9,
                background: 'var(--color-surface-0)', border: '1px solid var(--line-3)',
              }}>
                {(param.required ? [] : [{ value: '', label: text?.none ?? CMD_UI.noValue }])
                  .concat((param.options ?? []).map((option) => ({
                    value: option.value,
                    label: text?.optLabels?.[option.value] ?? option.value,
                  })))
                  .map((option) => {
                    const on = String(raw ?? '') === option.value
                    return (
                      <button
                        key={option.value || 'none'}
                        type="button"
                        role="radio"
                        aria-checked={on}
                        onClick={() => set(param.key, option.value)}
                        style={{
                          flex: '1 1 0', minWidth: 0, height: 28, padding: '1px 6px', border: 'none', borderRadius: 6,
                          background: on ? '#2A2A2E' : 'transparent',
                          color: on ? 'var(--color-fg)' : 'var(--color-fg-3)', fontSize: 12,
                          whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
                        }}
                      >{option.label}</button>
                    )
                  })}
              </div>
            )}
            {param.kind === 'flag' && (
              <button
                id={id}
                type="button"
                role="switch"
                aria-checked={Boolean(raw)}
                onClick={() => set(param.key, !raw)}
                style={{
                  display: 'flex', alignItems: 'center', gap: 10, padding: 0, border: 'none',
                  background: 'transparent', color: 'var(--color-fg-3)', fontSize: 12.5, textAlign: 'left',
                }}
              >
                <span style={{
                  position: 'relative', width: 34, height: 20, borderRadius: 999, flex: '0 0 auto',
                  background: raw ? '#9A63E3' : 'var(--color-surface-7)', transition: 'background .15s',
                }}>
                  <i style={{
                    position: 'absolute', top: 3, left: raw ? 17 : 3, width: 14, height: 14,
                    borderRadius: '50%', background: 'var(--color-fg)', transition: 'left .15s',
                  }} />
                </span>
                <span>{text?.hint ?? text?.desc ?? param.token}</span>
              </button>
            )}
          </div>
        )
      })}

      <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
        <span className="lbl" style={{ fontSize: 10.5 }}>{CMD_UI.yourLine}</span>
        <div className="mono" style={{
          minHeight: 44, padding: '11px 12px', borderRadius: 10, background: '#0A0A0B',
          border: `1px solid ${check.err ? 'rgba(255,159,168,0.35)' : 'var(--line-3)'}`,
          fontSize: 13, whiteSpace: 'pre-wrap', wordBreak: 'break-word',
        }}>
          {tokens.map((token, index) => (
            <span key={index} style={{ color: CMD_COLOR[token.tone] }}>{token.text}</span>
          ))}
        </div>
        <div role="status" style={{
          display: 'flex', gap: 8, alignItems: 'flex-start', fontSize: 12,
          color: check.err ? STATUS_COLOR.err : check.warn ? STATUS_COLOR.warn : STATUS_COLOR.ok,
        }}>
          <Icon name={check.err ? 'warning-2' : check.warn ? 'info-circle' : 'tick-circle'} size={14} style={{ marginTop: 2 }} />
          <span>{check.err || check.warn || CMD_UI.okLine}</span>
        </div>
        {check.infos.map((info, index) => (
          <div key={index} style={{ display: 'flex', gap: 8, alignItems: 'flex-start', fontSize: 12, color: 'var(--color-fg-3)' }}>
            <Icon name="info-circle" size={14} style={{ marginTop: 2, color: 'var(--color-fg-4)' }} />
            <span>{info}</span>
          </div>
        ))}
      </div>

      <div style={{ display: 'flex', gap: 8 }}>
        <button
          type="button"
          onClick={() => onCopy(line, `${spec.name}:line`)}
          className="hov-ghost"
          style={{
            flex: '0 0 auto', height: 36, padding: '0 12px', borderRadius: 9,
            border: '1px solid var(--line-4)', background: 'transparent',
            color: 'var(--color-fg)', fontSize: 12.5,
            display: 'flex', alignItems: 'center', gap: 6, whiteSpace: 'nowrap',
          }}
        >
          <Icon name={lineCopied ? 'tick-circle' : 'copy'} size={15} />
          {lineCopied ? CMD_UI.copied : CMD_UI.copy}
        </button>
        {blocked ? (
          <button type="button" disabled style={{
            flex: '1 1 auto', minWidth: 0, height: 36, padding: '0 12px', borderRadius: 9,
            border: 'none', background: 'var(--color-surface-6)', color: 'var(--color-fg-4)',
            fontSize: 12.5, fontWeight: 600, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
            opacity: 1, cursor: 'not-allowed',
          }}>{blockLabel}</button>
        ) : (
          <a href={`#${runPath}`} onClick={(event) => { event.preventDefault(); onRun(line) }} className="hov-primary" style={{
            flex: '1 1 auto', minWidth: 0, height: 36, padding: '0 12px', borderRadius: 9,
            background: 'var(--color-fg)', color: 'var(--color-bg)',
            fontSize: 12.5, fontWeight: 600, display: 'flex', alignItems: 'center',
            justifyContent: 'center', gap: 6, whiteSpace: 'nowrap', overflow: 'hidden',
          }}>
            {spec.name === 'start' ? CMD_UI.openNewStory : CMD_UI.fillWorkspace}
            <Icon name="arrow-right-2" size={15} />
          </a>
        )}
      </div>
      {!copy && <span style={{ fontSize: 11.5, color: 'var(--color-fg-4)' }}>{spec.usage}</span>}
    </section>
  )
}
