// Tab Bản xuất — bảng Văn bản · Ebook · Sách nói + câu xuất tự động + Xuất ngay (Outline.dc.html › tab export).
import { createExport, downloadExportFile, getExports } from '../../../api/client'
import type { ExportFile, ExportFormat } from '../../../api/types'
import { keys, useAction, useResource } from '../../../data'
import { paths } from '../../../router'
import { EmptyState, Skeleton } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { StaleBanner } from '@noverse/ui'
import { toast } from '@noverse/ui'
import { autoExportSentence, formatBytes, volumeLabel } from '../format'
import { settingsLink } from '../links'
import { PanelRefreshBar } from '../../../ui/PanelRefreshBar'

const COLUMNS = '1.3fr 1fr 1fr 1fr 1fr'

function FileCell({ file, label, project, story }: { file: ExportFile | null; label: string; project: string; story: string }) {
  const download = useAction(async () => {
    if (file) await downloadExportFile(project, story, file.path)
  }, { success: 'Đã tải tệp xuất bản' })
  if (!file) return <span style={{ color: '#8E8E99' }}>Chưa xuất</span>
  return (
    <span style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
      <Icon name="tick-circle" size={15} style={{ color: '#A6EDC1' }} />
      <span className="num">{formatBytes(file.size)}</span>
      <button className="hov-ghost" type="button" aria-label={label} onClick={() => void download.run()} disabled={download.busy} style={{
        width: 28, height: 28, borderRadius: 7, border: 'none', background: 'transparent', color: '#C9C9D1',
        display: 'flex', alignItems: 'center', justifyContent: 'center',
      }}>
        <Icon name="document-download" size={15} />
      </button>
    </span>
  )
}

export function ExportTab({ project, story, bookTitle }: {
  project: string
  story: string
  /** tên sách — nhãn tập dùng “Tập N” khi tiêu đề tập trùng tên truyện */
  bookTitle: string
}) {
  const resource = useResource(keys.story(project, story, 'exports'), ({ signal }) => getExports(project, story, signal))
  const data = resource.data
  const exportAction = useAction(async (format: ExportFormat, volume: number, name: string) => {
    toast.ok(`Đang xuất ${name} — theo dõi ở góc dưới phải`)
    await createExport(project, story, { format, volume, ...(format === 'narration' ? { readNumbers: true } : {}) })
  }, { invalidate: [keys.story(project, story, 'exports')] })
  if (resource.error && !data) {
    return (
      <EmptyState icon="warning-2" title="Không tải được bản xuất" hint="Máy chủ chưa đọc được các bản xuất của truyện."
        action={
          <button type="button" className="hov-ghost" onClick={() => void resource.refresh()}
            style={{ height: 32, padding: '0 12px', borderRadius: 8, border: '1px solid rgba(255,255,255,0.1)', background: 'transparent', color: '#EDEDEF', fontSize: 13 }}>
            Thử lại
          </button>
        } />
    )
  }
  if (!data) return <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}><Skeleton height={120} radius={16} /><Skeleton height={56} radius={16} /></div>

  const row = data.matrix[0] ?? null
  const volume = row?.volume ?? 1
  const volumeName = volumeLabel({ index: volume, title: row?.title }, bookTitle)
  const format = data.autoExport.formats[0] ?? 'txt'
  return (
    <>
      <PanelRefreshBar resourceKeys={[keys.story(project, story, 'exports')]} updatedAt={resource.updatedAt} />
      <StaleBanner error={data && resource.error} onRetry={resource.refresh} />
      <section className="card" style={{ borderRadius: 16, overflow: 'hidden' }}>
        <div style={{ display: 'grid', gridTemplateColumns: COLUMNS, padding: '10px 16px', fontSize: 12, color: '#8E8E99', borderBottom: '1px solid rgba(255,255,255,0.06)' }}>
          <span>Phần</span><span>Văn bản</span><span>Ebook</span><span>Bản đọc TTS</span><span>Sách nói</span>
        </div>
        {data.matrix.length === 0 && (
          <div style={{ padding: '14px 16px', fontSize: 13.5, color: '#8E8E99' }}>Chưa có tập nào đủ chương để xuất.</div>
        )}
        {data.matrix.map((matrixRow, index) => {
          const name = volumeLabel({ index: matrixRow.volume, title: matrixRow.title }, bookTitle)
          return (
            <div key={matrixRow.volume} style={{
              display: 'grid', gridTemplateColumns: COLUMNS, alignItems: 'center', padding: '14px 16px', fontSize: 13.5,
              borderTop: index ? '1px solid rgba(255,255,255,0.05)' : 'none',
            }}>
              <span style={{ fontWeight: 500 }}>{name} · chương {matrixRow.from}–{matrixRow.to}</span>
              <FileCell file={matrixRow.txt} label={`Tải văn bản ${name}`} project={project} story={story} />
              <FileCell file={matrixRow.epub} label={`Tải ebook ${name}`} project={project} story={story} />
              {matrixRow.narration
                ? <FileCell file={matrixRow.narration} label={`Tải bản đọc TTS ${name}`} project={project} story={story} />
                : <span style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
                  <span style={{ color: 'var(--kit-fg-icon)' }}>Chưa xuất</span>
                  <span style={{ fontSize: 11.5, color: 'var(--kit-fg-faint)' }}>cho giọng đọc AI</span>
                </span>}
              <span>
                {matrixRow.audio.length > 0
                  ? <a href={`#${paths.audio(project, story, matrixRow.volume)}`} style={{ color: '#C9A6F5', fontSize: 13 }}>{matrixRow.audio.length} tệp ›</a>
                  : <a href={`#${paths.audio(project, story, matrixRow.volume, 'make')}`} style={{ color: '#C9A6F5', fontSize: 13 }}>Làm sách nói ›</a>}
              </span>
            </div>
          )
        })}
      </section>
      <section className="card" style={{ marginTop: 10, borderRadius: 16, padding: '14px 16px', display: 'flex', alignItems: 'center', gap: 12 }}>
        <Icon name="refresh-2" size={17} style={{ color: '#A6EDC1' }} />
        <span style={{ flex: '1 1 auto', fontSize: 13.5 }}>{autoExportSentence(data.autoExport)}</span>
        <a href={`#${settingsLink('library', { project })}`} style={{ fontSize: 12.5, color: '#C9A6F5', whiteSpace: 'nowrap' }}>Đổi trong Tuỳ chỉnh</a>
      </section>
      <div style={{ marginTop: 14, display: 'flex', gap: 8 }}>
        <button className="primary hov-primary" type="button" onClick={() => void exportAction.run(format, volume, volumeName)} disabled={exportAction.busy || !row} style={{
          height: 36, padding: '0 14px', borderRadius: 10, border: 'none', background: '#EDEDEF', color: '#0B0B0D', fontSize: 13, fontWeight: 600,
          display: 'flex', alignItems: 'center', gap: 8,
        }}>
          <Icon name="export-square" size={16} />Xuất {volumeName} ngay
        </button>
        <a className="hov-ghost" href={`#${paths.reader(project, story)}`} style={{
          height: 36, padding: '0 14px', borderRadius: 10, border: '1px solid rgba(255,255,255,0.1)', fontSize: 13,
          display: 'flex', alignItems: 'center', color: '#EDEDEF',
        }}>Chọn chương, định dạng…</a>
      </div>
    </>
  )
}
