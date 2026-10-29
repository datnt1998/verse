// ExportPanel — xuất truyện: định dạng, khoảng chương, ghi đè, POST vào thư mục xuất bản,
// tải qua client, danh sách/matrix các lần xuất. Chỉ đọc → chỉ tải, không ghi.
import { useEffect, useState } from 'react'
import {
  ApiError, createExport, downloadExport, downloadExportBook,
} from '../../api/client'
import type { ChapterInfo, ExportFile, ExportFormat, ExportsResponse } from '../../api/types'
import { Button } from '@noverse/ui'
import { Card } from '@noverse/ui'
import { Checkbox } from '@noverse/ui'
import { Field, FieldLabel } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { toast } from '@noverse/ui'
import { CompactSegmented } from './CompactSegmented'
import { copy, formatBytes, formatClock } from './copy'
import { exportChapterCount, latestExport, normalizeRange } from './logic'
import { exportExt, exportFormatOf, exportLocationLabel, friendlyExportName, inferExportVolume } from '../../lib/exportName'
import { keys, useAction, useConnection } from '../../data'
import { StaleBanner } from '@noverse/ui'

export function ExportPanel({ project, story, storyTitle, chapters, data, loading, error, onReload, writable }: {
  project: string
  story: string
  chapters: ChapterInfo[]
  data: ExportsResponse | null
  loading: boolean
  error: ApiError | null
  onReload: () => void
  writable: boolean
  storyTitle: string
}) {
  const [format, setFormat] = useState<ExportFormat>('txt')
  const [from, setFrom] = useState('1')
  const [to, setTo] = useState('')
  const [rangeTouched, setRangeTouched] = useState(false)
  /** “Ghi đè tệp đã có” → onExists replace (bật) / fail (tắt). */
  const [overwrite, setOverwrite] = useState(false)
  /** `doc-so` — chỉ có nghĩa với bản đọc TTS; artboard bật sẵn. */
  const [readNumbers, setReadNumbers] = useState(true)
  const connection = useConnection()

  const total = chapters.length
  useEffect(() => {
    setFrom('1')
    setTo('')
    setRangeTouched(false)
  }, [project, story])
  useEffect(() => {
    if (!rangeTouched && total > 0) setTo(String(total))
  }, [total, rangeTouched])

  const fromNumber = Number.parseInt(from, 10)
  const toNumber = Number.parseInt(to, 10)
  const range = normalizeRange(fromNumber, toNumber, total)
  const showRangeError = from !== '' && to !== '' && range === null

  const previewVolume = chapters.find((item) => item.chapter === (range?.from ?? 1))?.volume ?? data?.matrix[0]?.volume ?? 1
  const fileName = friendlyExportName(storyTitle, previewVolume, format)
  /** Tên thân thiện của một tệp đã xuất — số tập lấy từ danh sách, không có thì suy từ đường dẫn. */
  const nameOf = (file: ExportFile) => friendlyExportName(storyTitle, file.volume ?? inferExportVolume(file.path), exportFormatOf(file.path))

  const exportAction = useAction(async () => {
    if (!range || !connection.writable) return
    const result = await createExport(project, story, {
      format, from: range.from, to: range.to,
      onExists: overwrite ? 'replace' : 'fail',
      ...(format === 'narration' ? { readNumbers } : {}),
    })
    const name = friendlyExportName(storyTitle, inferExportVolume(result.path) ?? previewVolume, exportFormatOf(result.path) || format)
    const skipped = result.skipped.length > 0 ? ` · ${copy.export.skipped(result.skipped.length)}` : ''
    toast(`${copy.export.exported(result.chapters, name)}${skipped}`, 'ok', { label: copy.export.downloadLatest, onClick: () => { void downloadExportBook(project, story, result.path, name).catch((error: unknown) => toast.err(error instanceof Error ? error.message : copy.export.downloadFailed)) } })
    onReload()
    return result
  }, { invalidate: [keys.story(project, story, 'exports')] })
  const pending = exportAction.pending
  const runExport = () => exportAction.run()

  const downloadAction = useAction(async (file: ExportFile | null) => {
    if (!file && !range) return
    const result = file ? await downloadExportBook(project, story, file.path, nameOf(file)) : await downloadExport(project, story, format, { from: range!.from, to: range!.to }, format === 'narration' ? readNumbers : undefined)
    toast.ok(copy.export.downloadStarted(result.file))
    return result
  })
  const latestAction = useAction(async (file: ExportFile) => {
    const result = await downloadExportBook(project, story, file.path, nameOf(file))
    toast.ok(copy.export.downloadStarted(result.file))
    return result
  })
  const downloading = downloadAction.busy || latestAction.busy
  const downloadBook = () => downloadAction.run(null)
  const downloadFile = (file: ExportFile) => downloadAction.run(file)

  const latest = data ? latestExport(data.files) : null
  const latestLabel = latest ? exportLocationLabel(nameOf(latest), latest.volume ?? inferExportVolume(latest.path)) : ''
  const latestChapters = latest && data ? exportChapterCount(latest, data.matrix) : null

  return (
    <>
      <Card padding={14} radius={14} style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
        <h3 className="h3" style={{ margin: 0, display: 'flex', alignItems: 'center', gap: 8, fontSize: 13, fontWeight: 500, color: 'var(--color-fg-3)' }}>
          <Icon name="export-curve" size={15} />
          {copy.export.title}
        </h3>

        <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
          <FieldLabel htmlFor="reader-xpath">{copy.export.fileName}</FieldLabel>
          <Field id="reader-xpath" readOnly value={fileName} style={{ fontSize: 12, height: 34, borderRadius: 8 }} />
        </div>

        <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
          <span className="lbl">{copy.export.format}</span>
          <CompactSegmented label={copy.export.format} columns={3} value={format}
            height={30} activeWeight={400} surface="var(--color-surface-0)" border="var(--line-3)"
            onChange={(id) => setFormat(id === 'epub' ? 'epub' : id === 'narration' ? 'narration' : 'txt')}
            options={[
              { id: 'txt', label: copy.export.formats.txt },
              { id: 'epub', label: copy.export.formats.epub },
              { id: 'narration', label: copy.export.formats.narration },
            ]} />
          <span style={{ fontSize: 12, color: 'var(--kit-fg-icon)', lineHeight: 1.5 }}>{copy.export.formatNote[format]}</span>
          {format === 'narration' && (
            <div style={{
              display: 'flex', flexDirection: 'column', gap: 8, padding: '10px 12px', borderRadius: 10,
              background: 'var(--color-surface-0)', border: '1px solid var(--line-1)',
            }}>
              <Checkbox tone="mono" checked={readNumbers} onChange={setReadNumbers} label={copy.export.readNumbers} style={{ minHeight: 30, padding: 0 }} />
              <span className="lbl">{copy.export.preview}</span>
              <p style={{
                margin: 0, fontFamily: "'Literata', Georgia, serif", fontSize: 13, lineHeight: 1.65,
                color: 'var(--color-fg-2)',
              }}>{readNumbers ? copy.export.previewNumbers : copy.export.previewPlain}</p>
            </div>
          )}
        </div>

        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(2, minmax(0, 1fr))', gap: 8 }}>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
            <FieldLabel htmlFor="reader-xfrom">{copy.export.from}</FieldLabel>
            <Field id="reader-xfrom" type="number" inputMode="numeric" min={1} max={total || undefined}
              value={from} style={{ height: 34, borderRadius: 8, fontVariantNumeric: 'tabular-nums' }} onChange={(event) => { setRangeTouched(true); setFrom(event.target.value) }} />
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
            <FieldLabel htmlFor="reader-xto">{copy.export.to}</FieldLabel>
            <Field id="reader-xto" type="number" inputMode="numeric" min={1} max={total || undefined}
              value={to} style={{ height: 34, borderRadius: 8, fontVariantNumeric: 'tabular-nums' }} onChange={(event) => { setRangeTouched(true); setTo(event.target.value) }} />
          </div>
        </div>

        {showRangeError && <span role="alert" style={{ fontSize: 12, color: 'var(--color-err)' }}>{copy.export.invalidRange}</span>}

        {writable ? (
          <Checkbox tone="mono" checked={overwrite} onChange={setOverwrite} label={copy.export.overwrite} style={{ minHeight: 32, padding: 0, color: 'var(--color-fg-2)' }} />
        ) : (
          <span style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 12, color: 'var(--color-fg-4)' }}>
            <Icon name="shield-security" size={14} />{copy.export.readOnly}
          </span>
        )}

        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
          {writable && (
            <Button type="button" variant="primary" onClick={() => { void runExport() }} pending={pending} success={exportAction.success} disabled={!range || exportAction.busy || !connection.writable} title={connection.writable ? 'Xuất truyện' : 'Mất kết nối · Không xuất được'} style={{ height: 38, borderRadius: 10, fontSize: 13, fontWeight: 600 }}>
              <Icon name="export-curve" size={16} />
              {pending ? copy.export.exporting : range ? copy.export.exportRange(range.from, range.to) : copy.export.title}
            </Button>
          )}
          <Button size="sm" icon={<Icon name="export-square" size={15} />} pending={downloadAction.pending} success={downloadAction.success} disabled={!range || downloading}
            onClick={() => void downloadBook()}>
            {copy.export.download}
          </Button>
          <span style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>{copy.export.note}</span>
        </div>
      </Card>

      {loading && !data && <Card padding="12px 14px" radius={14} aria-busy="true"><span style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>{copy.export.missing}</span></Card>}
      {!!data && <StaleBanner error={error} onRetry={onReload} />}

      {!loading && error && !data && (
        <Card padding="12px 14px" radius={14} style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
          <span style={{ fontSize: 12, color: 'var(--color-err)' }}>{error.message}</span>
          <Button size="sm" onClick={onReload}>{copy.retry}</Button>
        </Card>
      )}

      {data && (
        latest ? (
          <Card padding="12px 14px" radius={14} style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
            <span className="lbl">{copy.export.latest} · {formatClock(latest.updatedAt)}</span>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 13 }}>
              <Icon name="tick-circle-bulk" size={16} style={{ color: 'var(--color-ok)' }} />
              {latestChapters !== null ? `${copy.export.chaptersCount(latestChapters)} / ` : ''}{formatBytes(latest.size)}
            </div>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <code className="mono" style={{
                flex: 1, minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
                fontSize: 11, color: 'var(--color-fg-3)',
              }}>{latestLabel}</code>
              <button type="button" className="hov-ghost" aria-label={copy.export.downloadLatest} title={copy.export.downloadLatest}
                onClick={() => void latestAction.run(latest)} disabled={downloading}
                style={{
                  width: 30, height: 30, borderRadius: 7, border: '1px solid var(--line-4)', background: 'transparent',
                  color: 'var(--color-fg-2)', display: 'flex', alignItems: 'center', justifyContent: 'center',
                }}>
                <Icon name="folder-open" size={15} />
              </button>
            </div>
          </Card>
        ) : (
          <Card padding="12px 14px" radius={14}>
            <span style={{ fontSize: 13, color: 'var(--color-fg-3)' }}>{copy.export.noExports}</span>
            <p style={{ margin: '4px 0 0', fontSize: 12, color: 'var(--color-fg-4)' }}>{copy.export.noExportsHint}</p>
          </Card>
        )
      )}

      {data && data.matrix.length > 0 && (
        <Card padding="12px 14px" radius={14} style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
          <span className="lbl">{copy.export.matrix}</span>
          {data.matrix.map((row) => (
            <div key={row.volume} style={{
              display: 'flex', alignItems: 'center', gap: 8, paddingTop: 6,
              borderTop: '1px solid var(--line-1)',
            }}>
              <div style={{ flex: 1, minWidth: 0 }}>
                <div style={{ fontSize: 12, color: 'var(--color-fg-2)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{row.title}</div>
                <div className="num" style={{ fontSize: 11, color: 'var(--color-fg-4)' }}>
                  Ch.{row.from}–{row.to}{row.complete ? '' : ' · thiếu'}
                </div>
              </div>
              {(['txt', 'epub', 'narration'] as const).map((key) => {
                const file = key === 'narration' ? row.narration ?? null : row[key]
                if (!file) return <span key={key} style={{ fontSize: 11, color: 'var(--color-fg-4)' }}>—</span>
                return (
                  <button key={key} type="button" className="hov-ghost" aria-label={`${copy.export.downloadFile} ${nameOf(file)}`}
                    title={nameOf(file)}
                    onClick={() => void downloadFile(file)} disabled={downloading}
                    style={{
                      height: 30, padding: '0 8px', borderRadius: 7, border: '1px solid var(--line-4)',
                      background: 'transparent', color: 'var(--color-fg-2)', fontSize: 11,
                      display: 'flex', alignItems: 'center', gap: 5, whiteSpace: 'nowrap', flex: '0 0 auto',
                    }}>
                    <Icon name="export-square" size={13} />
                    {copy.export.formats[key]}
                  </button>
                )
              })}
            </div>
          ))}
        </Card>
      )}
    </>
  )
}
