import { useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Banner, Button, ProgressBar, useAsyncAction } from '@noverse/ui'
import { invalidate } from '@noverse/ui/data'
import { controlStorageMigration, type Migration } from '../../../api/storageServer'
import { serverStorageCopy as t } from '../../../i18n/storageServer'
import { keys } from '../../../data/keys'
import { fill, fmtBytes, fmtEta, fmtTime } from '../fmt'
import { styles as s } from './serverStorage.styles'

export function MigrationCard({ migration: m, mobile }: { migration: Migration; mobile: boolean }) {
  const [cancel, setCancel] = useState(false)
  const action = useAsyncAction(async (kind: 'pause' | 'resume' | 'retry' | 'cancel') => {
    await controlStorageMigration(m.taskId, kind)
    setCancel(false)
    invalidate(keys.storage())
    invalidate(keys.activity())
  })
  const failed = m.state === 'error'
  const elapsed = Date.now() - Date.parse(m.startedAt)
  const percent = m.files.total > 0 ? Math.min(100, Math.round(m.files.done / m.files.total * 100)) : 0
  if (mobile) return <section {...stylex.props(s.card, s.mobileCard)} aria-label={fill(t.migrationTitle, { provider: m.to.label })}>
    <h2 {...stylex.props(s.heading)}>{fill(failed ? t.migrationFailed : t.migrationTitle, { provider: m.to.label })}</h2>
    <span {...stylex.props(s.hint)}>{m.files.done} / {fill(t.files, { n: m.files.total })} · {fmtBytes(m.bytes.done)} / {fmtBytes(m.bytes.total)}{elapsed >= 10_000 && m.rateBps > 0 ? ` · ${fill(t.eta, { time: fmtEta(Math.max(0, m.bytes.total - m.bytes.done) / m.rateBps) })}` : ''}</span>
    <ProgressBar value={m.bytes.total > 0 ? Math.min(100, m.bytes.done / m.bytes.total * 100) : 0} height={6} tone={failed ? 'err' : 'info'} />
    <span {...stylex.props(s.hint)}>{fill(t.migrationMobile, { from: m.from.label })}</span>
  </section>
  return <section {...stylex.props(s.card)} aria-label={fill(t.migrationTitle, { provider: m.to.label })}>
    <div {...stylex.props(s.row)}><h2 {...stylex.props(s.heading, s.grow)}>{fill(failed ? t.migrationFailed : t.migrationTitle, { provider: m.to.label })}</h2><span {...stylex.props(s.hint, failed && s.error)}>{failed ? fill(t.errorFiles, { n: m.errors.length }) : m.state === 'paused' ? t.paused : t.running}</span></div>
    <span {...stylex.props(s.hint)}>{fill(t.migrationNote, { time: fmtTime(m.startedAt), from: m.from.label })}</span>
    <div {...stylex.props(s.grid)}>{([{ label: t.fileCount, value: m.files }, { label: t.byteCount, value: m.bytes }] as const).map(({ label, value }, index) => <div key={label} role="group" aria-label={label} {...stylex.props(s.column)}><span {...stylex.props(s.row)}><span {...stylex.props(s.grow, s.hint)}>{label}</span><span {...stylex.props(s.numeric)}>{index === 0 ? `${value.done.toLocaleString('vi-VN')} / ${value.total.toLocaleString('vi-VN')}` : `${fmtBytes(value.done)} / ${fmtBytes(value.total)}`}</span></span><ProgressBar value={value.total > 0 ? Math.min(100, value.done / value.total * 100) : 0} tone={failed ? 'err' : 'info'} height={6} /></div>)}</div>
    <div role="list" aria-label={t.progress}>{m.groups.map(group => <div key={group.group} role="listitem" {...stylex.props(s.kv)}><span {...stylex.props(s.grow)}>{t.groups[group.group]}</span><span {...stylex.props(s.numeric)}>{group.files.done} / {group.files.total}</span><span {...stylex.props(s.numeric)}>{fmtBytes(group.bytes.done)} / {fmtBytes(group.bytes.total)}</span></div>)}</div>
    <div {...stylex.props(s.row, s.hint)}><span {...stylex.props(s.grow)}>{m.rateBps > 0 ? fill(t.rate, { rate: fmtBytes(m.rateBps) }) : `${percent}%`}{elapsed >= 10_000 && m.rateBps > 0 ? ` · ${fill(t.eta, { time: fmtEta(Math.max(0, m.bytes.total - m.bytes.done) / m.rateBps) })}` : ''}</span><span>{t.copiedCount} <b>{m.files.done}</b></span><span>{t.checkedCount} <b>{m.checked}</b></span><span>{t.errorCount} <b>{m.errors.length}</b></span></div>
    {failed ? <><span {...stylex.props(s.hint)}>{fill(t.errorNote, { from: m.from.label })}</span><div {...stylex.props(s.overflow)}><table {...stylex.props(s.table, s.left)}><thead><tr>{[t.group, t.video, t.reason].map(text => <th key={text} {...stylex.props(s.cell)}>{text}</th>)}</tr></thead><tbody>{m.errors.map((error, index) => <tr key={index}><td {...stylex.props(s.cell)}>{t.groups[error.group]}</td><td {...stylex.props(s.cell)}>{error.title}</td><td {...stylex.props(s.cell, s.error)}>{t.migrationErrors[error.code]}</td></tr>)}</tbody></table></div></> : null}
    {cancel ? <div role="alertdialog" aria-label={t.cancelMove}><Banner tone="warning" title={t.cancelQuestion}><span>{fill(t.cancelBody, { from: m.from.label, files: m.files.done, to: m.to.label })}</span><div {...stylex.props(s.row)}><Button variant="quiet" size="compact" onClick={() => setCancel(false)}>{t.keepMoving}</Button><Button variant="danger" size="compact" pending={action.pending} onClick={() => void action.run('cancel')}>{t.cancelMove}</Button></div></Banner></div> : <div {...stylex.props(s.row)}>{failed ? <Button variant="primary" size="compact" pending={action.pending} onClick={() => void action.run('retry')}>{fill(t.retryFiles, { n: m.errors.length })}</Button> : <Button variant="ghost" size="compact" pending={action.pending} onClick={() => void action.run(m.state === 'paused' ? 'resume' : 'pause')}>{m.state === 'paused' ? t.resume : t.pause}</Button>}<Button variant="quiet" size="compact" disabled={action.pending} onClick={() => setCancel(true)}>{t.cancelMove}</Button></div>}
  </section>
}
