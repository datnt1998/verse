import { useEffect, useState } from 'react'
import type { CommandSpec, Snapshot } from '../../../api/types'
import { useCommands, loadCommands } from '../../../commands/store'
import { checkLine, composeTokens, emptyValues, groupsOf, matchesQuery, parseLine, pushRecentCommand, slashContext, valuesOf } from '../../../commands/model'
import { CMD_COLOR } from '../../../commands/tokens'
import { CMD_COPY, numSuggestions, paramCopy } from '../../../i18n/commands.vi'
import { BottomSheet } from '../../../mobile/primitives'
import { mobileCommands } from '../../../mobile/logic'
import { navigate, paths } from '../../../router'
import { Icon } from '@noverse/ui'
import { COMMAND_SHORT, mobileInitialValues } from './logic'

export function CommandSheet({ open, onClose, initialLine = '', story, snapshot, writable, readOnly = false, onRun }: {
  open: boolean; onClose: () => void; initialLine?: string; story: string; snapshot: Snapshot | null; writable: boolean; readOnly?: boolean; onRun: (line: string) => Promise<void>
}) {
  const { specs, state, error, refreshing } = useCommands()
  const available = mobileCommands(specs).filter((spec) => !readOnly || spec.name === 'export')
  const [selected, setSelected] = useState<CommandSpec | null>(null)
  const [query, setQuery] = useState('')
  const [values, setValues] = useState<Record<string, string | boolean>>({})
  const [busy, setBusy] = useState(false)
  const [failure, setFailure] = useState('')
  useEffect(() => {
    if (!open) return
    setFailure('')
    const name = initialLine.trim().split(/\s+/)[0].replace(/^\//, '')
    const spec = available.find((item) => item.name === name || item.aliases.includes(name))
    setSelected(spec ?? null)
    setQuery(spec ? '' : name)
    if (spec) {
      const parsed = parseLine(initialLine, available)
      setValues(parsed.mode === 'args' ? valuesOf(spec, parsed.tokens).values : mobileInitialValues(spec, story, snapshot?.completedCount ?? 0).values)
    } else setValues({})
  }, [open, initialLine, specs, readOnly])
  const select = (spec: CommandSpec) => { setSelected(spec); setValues(mobileInitialValues(spec, story, snapshot?.completedCount ?? 0).values); setFailure('') }
  const ctx = slashContext(snapshot, available)
  const check = selected ? checkLine(selected, values, ctx) : null
  const tokens = selected ? composeTokens(selected, values) : []
  const blocked = !writable || busy || !!check?.err || (!!selected?.needsIdle && !!snapshot?.isRunning)
  const update = (key: string, value: string | boolean) => { setValues((before) => ({ ...before, [key]: value })); setFailure('') }
  const copy = selected ? CMD_COPY[selected.name] : undefined
  const groups = groupsOf(available.filter((spec) => matchesQuery(spec, query)))
  return <BottomSheet open={open} onClose={() => { if (!busy) onClose() }} title={selected ? undefined : 'Lệnh'} height={650}>
    {refreshing && <div className="mw-refresh" role="status" aria-label="Đang cập nhật lệnh" />}
    {!selected ? <>
      <div className="mw-command-search"><button type="button" className="mw-recent-link" onClick={() => navigate(`${paths.settings('mobile')}?page=guide`)}>Hướng dẫn</button><input className="m-field" type="search" aria-label="Tìm lệnh" placeholder="Tìm lệnh hoặc việc muốn làm" value={query} onChange={(event) => setQuery(event.target.value.replace(/^\//, ''))} /></div>
      {state === 'loading' && <p className="m-muted mw-pad" role="status">Đang tải lệnh…</p>}
      {state === 'error' && <div className="m-error" role="alert">{error}<button className="m-chip" onClick={() => void loadCommands()}>Thử lại</button></div>}
      {groups.map((group) => <section key={group.key}><div className="m-section-label">{group.label}</div><div className="m-group">{group.specs.map((spec) => <button className="m-row" key={spec.name} onClick={() => select(spec)}><span className="mono mw-command-name">/{spec.name}</span><span className="m-row-copy"><strong className="mw-command-short">{COMMAND_SHORT[spec.name] ?? CMD_COPY[spec.name]?.title ?? spec.name}</strong>{spec.needsIdle && <small style={{ color: snapshot?.isRunning ? '#F6CB8C' : '#8E8E99' }}>Cần rảnh{snapshot?.isRunning ? ' · đang viết' : ''}</small>}</span><Icon name="arrow-right-2" size={15} /></button>)}</div></section>)}
      {state === 'ready' && !groups.length && <p className="m-muted mw-pad">Không tìm thấy lệnh. Thử “xuất”, “duyệt” hoặc tên lệnh không dấu.</p>}
    </> : <div className="mw-command-args m-push">
      <div className="mw-command-heading"><button className="m-icon-button" aria-label="Quay lại danh sách lệnh" disabled={busy} onClick={() => { setSelected(null); setFailure('') }}><Icon name="arrow-left-2" size={20} /></button><span><strong className="mono">/{selected.name}</strong><small className="m-muted">{COMMAND_SHORT[selected.name] ?? copy?.title}</small></span></div>
      <div className="mw-command-fields">
        <p className="m-muted">{copy?.lead}</p>
        {!!copy?.ex.length && <section className="mw-examples"><span className="mw-label">Ví dụ · chạm để dùng</span>{copy.ex.slice(0, 2).filter((example) => !example.slash).map((example, index) => <button className="mw-example" key={index} disabled={busy} onClick={() => { setValues({ ...emptyValues(selected).values, ...example.vals }); setFailure('') }}><span className="mono">{composeTokens(selected, { ...emptyValues(selected).values, ...example.vals }).map((token, i) => <span key={i} style={{ color: CMD_COLOR[token.tone] }}>{token.text}</span>)}</span><small className="m-muted">{example.text}</small></button>)}</section>}
        {selected.params.map((param) => {
          const paramText = paramCopy(selected, param.key)
          const value = values[param.key]
          const options = param.kind === 'enum' ? param.options ?? [] : param.kind === 'num' ? numSuggestions(param.key, ctx.completedCount) : []
          return <div className="mw-param" key={param.key}><label className="m-command-label" htmlFor={`mobile-param-${param.key}`}>{paramText?.label ?? param.token}<span className="mono m-muted">{param.token}</span></label>
            {param.kind === 'flag' ? <button id={`mobile-param-${param.key}`} type="button" className={`m-chip${value ? ' on' : ''}`} aria-pressed={!!value} disabled={busy} onClick={() => update(param.key, !value)}>{paramText?.hint ?? param.flag ?? param.token}</button> : <>
              {options.length > 0 && <div className="mw-param-chips">{!param.required && <button className={`m-chip${!value ? ' on' : ''}`} disabled={busy} onClick={() => update(param.key, '')}>{paramText?.none ?? 'Không ghi'}</button>}{options.map((option) => <button className={`m-chip${value === option.value ? ' on' : ''}`} aria-pressed={value === option.value} key={option.value} disabled={busy} onClick={() => update(param.key, option.value)}>{paramText?.optLabels?.[option.value] ?? option.value}</button>)}</div>}
              {param.kind !== 'enum' && <input id={`mobile-param-${param.key}`} className="m-field" inputMode={param.kind === 'num' ? 'numeric' : 'text'} value={typeof value === 'string' ? value : ''} disabled={busy} placeholder={paramText?.ph ?? param.token} onChange={(event) => update(param.key, event.target.value)} />}
            </>}<small className="m-muted">{paramText?.desc ?? (param.required ? 'Bắt buộc' : 'Tuỳ chọn')}</small></div>
        })}
        {selected.params.length === 0 && <p className="mw-example m-muted">Lệnh này không cần thêm gì — bấm Chạy.</p>}
        {copy?.tip && <div className="mw-tip"><Icon name="lamp-on" size={17} /><span>{copy.tip}</span></div>}
      </div>
      <div className="mw-command-submit"><div className="m-command-preview">{tokens.map((token, index) => <span key={index} style={{ color: CMD_COLOR[token.tone] }}>{token.text}</span>)}</div><div role="status" style={{ color: check?.err ? '#FF8E97' : '#A6EDC1', fontSize: 13 }}>{check?.err || check?.warn || check?.infos[0] || 'Đúng cú pháp — sẵn sàng chạy.'}</div>{failure && <div className="m-error" role="alert">{failure}</div>}<button className="m-button" disabled={blocked} aria-busy={busy} onClick={async () => { setBusy(true); setFailure(''); try { await onRun(tokens.map((token) => token.text).join('')); pushRecentCommand(selected.name); onClose() } catch (reason) { setFailure(reason instanceof Error ? reason.message : 'Không chạy được lệnh. Vui lòng thử lại.') } finally { setBusy(false) } }}>{busy ? 'Đang gửi…' : `Chạy /${selected.name}`}</button></div>
    </div>}
  </BottomSheet>
}
