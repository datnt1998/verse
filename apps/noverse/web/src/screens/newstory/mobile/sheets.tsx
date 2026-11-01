// M11 — các ngăn dưới (MNewStory.dc.html: `sheet=style|writer|startFile|samples`).
// Bốn ngăn dùng đúng dữ liệu của bản máy tính: StyleQuickPicker/WriterQuickPicker, luồng
// “bắt đầu từ tệp” và luồng văn mẫu của simulateStart.ts (chung với hộp thoại máy tính).
import { useRef, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Icon, Sheet, Spinner } from '@noverse/ui'
import { getConfigPresets } from '../../../api/client'
import type { ConfigResponse } from '../../../api/types'
import { useResource } from '../../../data'
import { amWritingCopy, t } from '../../../i18n/vi'
import { paths } from '../../../router'
import { modelOptions } from '../../settings/lib/config'
import { StyleQuickPicker, WriterQuickPicker } from '../../settings/sections/quick-pickers'
import type { QuickStoryChoices } from '../creation'
import { SAMPLE_LIMIT, normalizeSamples, sampleWords, splitSampleText } from '../simulateStart'
import { styles } from './styles'

const EMPTY_SAMPLE = ['']

function SheetHead({ title, onClose }: { title: string; onClose: () => void }) {
  return <div {...stylex.props(styles.sheetHead)}>
    <span {...stylex.props(styles.sheetTitle)}>{title}</span>
    <button type="button" onClick={onClose} {...stylex.props(styles.sheetClose)}>{t.newStory.sheetClose}</button>
  </div>
}

/** `sheet=style` — chọn văn phong cho truyện này. */
export function StyleSheet({ open, onClose, project, config, choices, onChoices }: {
  open: boolean
  onClose: () => void
  project: string
  config: ConfigResponse | null
  choices: QuickStoryChoices
  onChoices: (choices: QuickStoryChoices) => void
}) {
  const presets = useResource(['config-presets'], ({ signal }) => getConfigPresets(signal), { staleTime: 60000 })
  const projectStyle = config?.effective?.style || 'default'
  return <Sheet open={open} onClose={onClose} label={t.newStory.sheetStyle}>
    <div {...stylex.props(styles.sheetBody)}>
      <SheetHead title={t.newStory.sheetStyle} onClose={onClose} />
      <StyleQuickPicker variant="newstory" value={choices.style} projectStyle={projectStyle} presets={presets.data ?? null}
        onPick={(value) => { onChoices({ ...choices, style: value === projectStyle ? null : value }); onClose() }}
        footer={<a href={`#${paths.project(project)}?section=pstyle`} {...stylex.props(styles.note)}>{amWritingCopy.projectFooter}</a>} />
    </div>
  </Sheet>
}

/** `sheet=writer` — chọn model viết cho truyện này (thêm model thì phải lên máy tính). */
export function WriterSheet({ open, onClose, config, choices, onChoices }: {
  open: boolean
  onClose: () => void
  config: ConfigResponse | null
  choices: QuickStoryChoices
  onChoices: (choices: QuickStoryChoices) => void
}) {
  const presets = useResource(['config-presets'], ({ signal }) => getConfigPresets(signal), { staleTime: 60000 })
  const effective = config?.effective
  const projectModel = effective?.roles?.writer?.model || effective?.roles?.default?.model || effective?.model || '—'
  return <Sheet open={open} onClose={onClose} label={t.newStory.sheetWriter}>
    <div {...stylex.props(styles.sheetBody)}>
      <SheetHead title={t.newStory.sheetWriter} onClose={onClose} />
      <WriterQuickPicker variant="newstory" value={choices.writer} projectModel={projectModel}
        models={modelOptions(effective ?? {})} presets={presets.data ?? null}
        onPick={(value) => {
          const inherited = value?.model === projectModel && value?.provider === (effective?.roles?.writer?.provider || effective?.roles?.default?.provider || effective?.provider)
          onChoices({ ...choices, writer: inherited ? null : value })
          onClose()
        }} />
      <p {...stylex.props(styles.note)}>{t.newStory.writerNote}</p>
    </div>
  </Sheet>
}

/** `sheet=startFile` — bắt đầu từ thiết lập/đại cương có sẵn; submit là submit của StartFileDialog. */
export function StartFileSheet({ open, onClose, busy, error, onSubmit }: {
  open: boolean
  onClose: () => void
  busy: boolean
  error: string | null
  onSubmit: (path: string) => void
}) {
  const [path, setPath] = useState('')
  const value = path.trim()
  const submit = () => { if (value && !busy) onSubmit(value) }
  return <Sheet open={open} onClose={onClose} label={t.newStory.startFileTitle}>
    <form {...stylex.props(styles.sheetBody)} onSubmit={(event) => { event.preventDefault(); submit() }}>
      <SheetHead title={t.newStory.startFileTitle} onClose={onClose} />
      <p {...stylex.props(styles.intro)}>{t.newStory.startFileIntro}</p>
      {error && <span role="alert" {...stylex.props(styles.errorDetail)}>{error}</span>}
      <label htmlFor="m11-start-file" {...stylex.props(styles.fieldLabel)}>{t.newStory.fileName}</label>
      <input id="m11-start-file" type="text" value={path} autoCapitalize="off" autoCorrect="off" spellCheck={false}
        onChange={(event) => setPath(event.target.value)} {...stylex.props(styles.field)} />
      <p {...stylex.props(styles.note)}>{t.newStory.startFileNote}</p>
      <button type="submit" disabled={!value || busy} aria-disabled={!value || busy}
        {...stylex.props(styles.button, value && !busy ? styles.primary : styles.off)}>
        {busy ? <Spinner size={15} /> : null}
        {t.newStory.startFileSubmit}
      </button>
    </form>
  </Sheet>
}

/** `sheet=samples` — dán văn mẫu trước, rồi tạo truyện và học văn phong (việc 6, dùng chung luồng). */
export function SamplesSheet({ open, onClose, busy, error, onSubmit }: {
  open: boolean
  onClose: () => void
  busy: boolean
  error: string | null
  onSubmit: (samples: string[]) => void
}) {
  const [rows, setRows] = useState<string[]>(EMPTY_SAMPLE)
  const file = useRef<HTMLInputElement>(null)
  const samples = normalizeSamples(rows)
  const full = rows.length >= SAMPLE_LIMIT
  const submit = () => { if (samples.length && !busy) onSubmit(samples) }

  const edit = (index: number, value: string) => setRows((current) => current.map((row, i) => i === index ? value : row))
  const remove = (index: number) => setRows((current) => {
    const next = current.filter((_, i) => i !== index)
    return next.length ? next : EMPTY_SAMPLE
  })
  const add = () => setRows((current) => current.length >= SAMPLE_LIMIT ? current : [...current, ''])
  const pick = async (picked: File | undefined) => {
    if (!picked) return
    const parts = splitSampleText(await picked.text())
    setRows((current) => {
      const kept = current.filter((row) => row.trim())
      const next = [...kept, ...parts.slice(0, Math.max(0, SAMPLE_LIMIT - kept.length))]
      return next.length ? next : EMPTY_SAMPLE
    })
    if (file.current) file.current.value = ''
  }

  return <Sheet open={open} onClose={onClose} label={t.newStory.simulateTitle}>
    <div {...stylex.props(styles.sheetBody)}>
      <SheetHead title={t.newStory.simulateTitle} onClose={onClose} />
      <p {...stylex.props(styles.intro)}>{t.newStory.simulateIntro}</p>
      {error && <span role="alert" {...stylex.props(styles.errorDetail)}>{error}</span>}
      {rows.map((row, index) => {
        const label = t.newStory.simulateSample(index + 1)
        const words = row.trim() ? ` · ${t.newStory.simulateWords(sampleWords(row))}` : ''
        return <div key={index} {...stylex.props(styles.sample)}>
          <span {...stylex.props(styles.sampleHead)}>
            <span {...stylex.props(styles.sampleLabel)}>{label}{words}</span>
            <button type="button" onClick={() => remove(index)} disabled={busy} aria-label={t.newStory.simulateDelete(`${label}${words}`)}
              {...stylex.props(styles.sampleDelete)}>
              <Icon name="trash" size={15} />
            </button>
          </span>
          <textarea rows={4} value={row} maxLength={20000} placeholder={t.newStory.simulatePlaceholder}
            onChange={(event) => edit(index, event.target.value)} {...stylex.props(styles.sampleText)} />
        </div>
      })}
      <div {...stylex.props(styles.sampleRow)}>
        <button type="button" onClick={add} disabled={busy || full} {...stylex.props(styles.chipSmall)}>
          <Icon name="add" size={14} />{t.newStory.simulateAdd}
        </button>
        <button type="button" onClick={() => file.current?.click()} disabled={busy || full} {...stylex.props(styles.chipSmall)}>
          <Icon name="document-text" size={14} />{t.newStory.simulatePickFile}
        </button>
        <input ref={file} type="file" accept=".txt,.md,text/plain,text/markdown" hidden
          onChange={(event) => void pick(event.target.files?.[0])} />
        <span {...stylex.props(styles.note)}>{t.newStory.simulateCount(rows.length)}</span>
      </div>
      <p {...stylex.props(styles.note)}>{t.newStory.simulateNote}</p>
      <button type="button" onClick={submit} disabled={busy || samples.length === 0} aria-disabled={busy || samples.length === 0}
        {...stylex.props(styles.button, samples.length > 0 && !busy ? styles.primary : styles.off)}>
        {busy ? <Spinner size={15} /> : null}
        {error ? t.newStory.simulateRetry : t.newStory.simulateSubmit}
      </button>
    </div>
  </Sheet>
}
