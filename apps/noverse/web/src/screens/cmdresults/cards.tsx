// Thẻ kết quả trong nhật ký sau khi gửi lệnh (CmdResults.dc.html: exportRun · exportDone ·
// exportExists · syncCheck · syncDone · simulateRun · simulateDone). Mỗi thẻ chỉ hiển thị dữ
// liệu thật từ API/SSE; hành động (tải, mở thư mục, tiếp nhận…) gọi qua props hoặc client.
import { useState } from 'react'
import {
  ApiError, downloadExportBook, downloadSimulationProfile, exportDownloadLink, getSyncDiff, revealFolder, sendInput,
} from '../../api/client'
import type {
  ExportConflict, ExportJobResult, JobEvent, SimulateJobResult, SyncChanged, SyncDiff, SyncResult,
} from '../../api/types'
import { friendlyExportName, exportFormatOf, inferExportVolume } from '../../lib/exportName'
import { translateHost } from '../../i18n/hostStrings'
import { navigate, paths } from '../../router'
import { dispatchDock } from '../../shell/jobs/store'
import { Icon } from '@noverse/ui'
import { ConfirmDialog } from '@noverse/ui'
import { useAction, useResource } from '../../data'
import { toast } from '@noverse/ui'
import {
  exportGrid, formatAgo, formatChapterList, formatChapterRanges, formatEditedAt, formatInt, formatSize,
  formatWordDelta, paragraphDiff, signatureQuote, simulationTraits, syncSummary, withExportFormat,
} from './logic'
import { CardHead, DrawTick, MiniLabel, OutlineBtn, PrimaryBtn, ResultCard, errorText } from './parts'
import { copyText } from '../../lib/clipboard'
import { PanelRefreshBar } from '../../ui/PanelRefreshBar'

type StoryRef = { project: string; story: string }

/** D3 — tên hiển thị thống nhất của ba định dạng xuất. */
const FORMAT_WORD: Record<string, string> = { txt: 'văn bản', epub: 'ebook', narration: 'bản đọc TTS' }
const FORMAT_LABEL: Record<string, string> = { txt: 'Văn bản', epub: 'Ebook', narration: 'Bản đọc TTS' }
const FORMAT_OPTION: Record<string, string> = { txt: 'Văn bản .txt', epub: 'Ebook .epub', narration: 'Bản đọc TTS' }
/** Nhãn mono trên bìa tệp — bản đọc TTS vẫn là tệp .txt nhưng mang nhãn “TTS” (câu 9). */
const FORMAT_BADGE: Record<string, string> = { txt: 'TXT', epub: 'EPUB', narration: 'TTS' }
/** Tiêu đề thẻ xuất xong — bản đọc TTS nói rõ định dạng (câu 9). */
const doneTitle = (format: string) => (format === 'narration' ? 'Đã xuất bản đọc TTS' : 'Đã xuất xong')

// ─── /export ───────────────────────────────────────────────────────────────

export function ExportRunCard({ job, path, bookTitle }: { job: JobEvent | undefined; path: string; bookTitle: string }) {
  const format = exportFormatOf(path)
  const total = job?.total ?? 0
  const current = job?.current ?? 0
  const packing = !!job && total > 0 && current >= total
  const { cells, percent } = exportGrid(current, total)
  const message = job?.message ? translateHost(job.message) : 'Đang chuẩn bị'
  const word = FORMAT_WORD[format] ?? 'ebook'
  const status = total > 0
    ? `Chương ${formatInt(current)}/${formatInt(total)} · ${packing ? `đang đóng gói ${word}` : message.charAt(0).toLowerCase() + message.slice(1)}`
    : message
  return (
    <ResultCard role="status" label={`Đang xuất ${word}`} style={{ borderColor: 'rgba(166,237,193,0.18)' }}>
      <CardHead tint="rgba(166,237,193,0.1)" title={`Đang xuất ${word}`} sub={bookTitle ? `Bản xuất › ${bookTitle}` : undefined}
        aside={<span className="mono num" style={{ fontSize: 20, fontWeight: 500 }}>{percent}%</span>}>
        <Icon name="export-square" size={20} style={{ color: '#A6EDC1' }} />
      </CardHead>
      <div style={{ display: 'grid', gridTemplateColumns: `repeat(${cells.length}, minmax(0, 1fr))`, gap: 4 }}>
        {cells.map((cell, i) => (
          <span key={i} className={`cr-cell${cell.state !== 'todo' ? ' cr-on' : ''}`} title={cell.label} style={{
            height: 18, borderRadius: 4,
            background: cell.state === 'done' ? '#A6EDC1' : cell.state === 'now' ? 'rgba(166,237,193,0.55)' : 'rgba(255,255,255,0.06)',
          }} />
        ))}
      </div>
      <div style={{ display: 'flex', justifyContent: 'space-between', gap: 12, fontSize: 12, color: '#8E8E99' }}>
        <span className="num" style={{ minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{status}</span>
        <button type="button" onClick={() => dispatchDock({ type: 'open' })} style={{
          border: 'none', background: 'transparent', padding: 0, color: '#A0A0AB', fontSize: 12, whiteSpace: 'nowrap',
          textDecoration: 'underline', textDecorationColor: 'rgba(255,255,255,0.2)', textUnderlineOffset: 3,
        }}>Có thể chuyển màn — việc này hiện ở góc dưới phải</button>
      </div>
    </ResultCard>
  )
}

/** “Tên truyện · Tập N.epub” cho tệp xuất — không bao giờ là đường dẫn hay tên thư mục. */
const exportDisplayName = (bookTitle: string, path: string, format: string) =>
  friendlyExportName(bookTitle, inferExportVolume(path), exportFormatOf(path) || format)

export function ExportDoneCard({ result, command, onRun, project, story, bookTitle }: StoryRef & {
  result: ExportJobResult; command: string; onRun: (line: string) => void; bookTitle: string
}) {
  const [copied, setCopied] = useState(false)
  const skipped = result.skipped ?? []
  const format = result.format
  const name = exportDisplayName(bookTitle || result.title, result.path, format)

  const copyAction = useAction(async () => {
    await copyText(exportDownloadLink(project, story, result.path))
    setCopied(true)
    setTimeout(() => setCopied(false), 1400)
  }, { error: 'Không chép được liên kết tải' })
  const folderAction = useAction(async () => {
    try {
      await revealFolder({ target: 'exports', project, story })
      toast.ok('Đã mở thư mục Bản xuất')
    } catch (reason) {
      if (reason instanceof ApiError && reason.status === 403) {
        toast.err('Chỉ mở được thư mục Bản xuất trên máy chủ. Hãy dùng Tải xuống.')
      } else throw reason
    }
  }, { error: 'Không mở được thư mục' })
  const downloadAction = useAction(() => downloadExportBook(project, story, result.path, name), { success: 'Đã tải tệp xuất', error: 'Không tải được tệp' })
  const otherFormats = (['txt', 'epub', 'narration'] as const).filter((entry) => entry !== format)

  return (
    <ResultCard role="status" label={doneTitle(format)} style={{
      gap: 14, borderColor: 'rgba(166,237,193,0.25)',
      background: 'radial-gradient(120% 120% at 0% 0%, rgba(166,237,193,0.07), transparent 50%), linear-gradient(180deg,#171719 0%,#131315 100%)',
    }}>
      <div style={{ display: 'flex', gap: 14, alignItems: 'center' }}>
        <span style={{
          position: 'relative', width: 48, height: 60, borderRadius: 8, background: 'linear-gradient(160deg, #24342C, #16201B)',
          border: '1px solid rgba(166,237,193,0.3)', display: 'flex', alignItems: 'flex-end', justifyContent: 'center', paddingBottom: 7, flex: '0 0 auto',
        }}>
          <span className="mono" style={{ fontSize: 10, fontWeight: 600, color: '#A6EDC1', letterSpacing: '.06em' }}>{FORMAT_BADGE[format] ?? 'TXT'}</span>
          <span style={{
            position: 'absolute', top: -6, right: -6, width: 20, height: 20, borderRadius: '50%', background: '#A6EDC1',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
          }}><DrawTick size={12} color="#0B0B0D" stroke={3} /></span>
        </span>
        <div style={{ flex: '1 1 auto', minWidth: 0, display: 'flex', flexDirection: 'column', gap: 2 }}>
          <span style={{ fontSize: 15, fontWeight: 600 }}>{doneTitle(format)}</span>
          <span style={{ fontSize: 13, color: '#D6D6DC', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{name}</span>
          <span className="num" style={{ fontSize: 12, color: '#8E8E99', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
            {FORMAT_LABEL[format] ?? 'Ebook'} · {formatInt(result.chapters)} chương · {formatSize(result.bytes)}
          </span>
        </div>
      </div>
      {skipped.length > 0 && (
        <div style={{ display: 'flex', gap: 8, alignItems: 'flex-start', padding: '8px 10px', borderRadius: 10, background: 'rgba(255,255,255,0.03)', fontSize: 12.5, color: '#A0A0AB' }}>
          <Icon name="info-circle" size={14} style={{ marginTop: 3 }} />
          <span>Bỏ qua {skipped.length} chương chưa hoàn thành: {formatChapterRanges(skipped)}. Xuất lại khi viết xong để có bản đầy đủ.</span>
        </div>
      )}
      <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap', fontSize: 12.5, color: 'var(--kit-fg-icon)' }}>
        <span>Xuất thêm định dạng:</span>
        {otherFormats.map((entry) => (
          <button key={entry} type="button" className="tbtn" onClick={() => onRun(withExportFormat(command, entry))} style={{
            height: 28, padding: '0 10px', borderRadius: 8, border: '1px solid var(--line-4)',
            background: 'transparent', color: 'var(--color-fg-2)', fontSize: 12, whiteSpace: 'nowrap',
            display: 'flex', alignItems: 'center', gap: 6,
          }}>
            {FORMAT_OPTION[entry] ?? entry}
            {entry === 'narration' && <span style={{ fontSize: 11, color: 'var(--color-fg-4)' }}>cho giọng đọc AI</span>}
          </button>
        ))}
      </div>
      <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
        <PrimaryBtn icon="document-download" onClick={() => void downloadAction.run()} disabled={downloadAction.busy} pending={downloadAction.pending}>Tải xuống</PrimaryBtn>
        <OutlineBtn icon="folder-open" onClick={() => void folderAction.run()} disabled={folderAction.busy} pending={folderAction.pending}>Mở thư mục</OutlineBtn>
        <OutlineBtn icon={copied ? 'tick-circle' : 'copy'} onClick={() => void copyAction.run()} disabled={copyAction.busy} pending={copyAction.pending} style={{ color: copied ? '#A6EDC1' : '#EDEDEF' }}>
          {copied ? 'Đã chép' : 'Chép liên kết tải'}
        </OutlineBtn>
        <span style={{ flex: 1 }} />
        <button type="button" onClick={() => navigate(paths.help('export'))} style={{
          height: 34, padding: '0 4px', border: 'none', background: 'transparent', display: 'flex', alignItems: 'center', gap: 6,
          color: '#A0A0AB', fontSize: 12.5, whiteSpace: 'nowrap',
        }}>Xuất với tuỳ chọn khác<Icon name="arrow-right-2" size={14} /></button>
      </div>
    </ResultCard>
  )
}

export function ExportExistsCard({ conflict, bookTitle, busy, onReplace, onKeep, onCancel }: {
  conflict: ExportConflict
  bookTitle: string
  busy: 'replace' | 'keep' | null
  onReplace: () => void; onKeep: () => void; onCancel: () => void
}) {
  const { existing, incoming } = conflict
  const name = exportDisplayName(bookTitle, conflict.path, conflict.format)
  const [confirmReplace, setConfirmReplace] = useState(false)
  const existingLine = [
    existing.chapters !== null ? `${formatInt(existing.chapters)} chương` : null,
    formatSize(existing.bytes),
    existing.modifiedAt ? formatAgo(existing.modifiedAt) : null,
  ].filter(Boolean).join(' · ')
  const incomingLine = [
    `${formatInt(incoming.chapters)} chương`,
    incoming.bytesEstimate !== null ? `khoảng ${formatSize(incoming.bytesEstimate)}` : 'chưa rõ dung lượng',
  ].join(' · ')
  return (
    <>
    {confirmReplace && <ConfirmDialog tone="warn" title="Thay bản xuất cũ?"
      body="Bản mới sẽ ghi đè tệp đang có. Chọn Giữ cả hai để giữ lại bản cũ."
      confirmLabel="Thay bản cũ" pendingLabel="Đang xuất…"
      compare={[
        { label: 'Bản đang có', summary: name, meta: existingLine },
        { label: 'Bản mới', summary: name, meta: incomingLine },
      ]}
      secondary={{ label: 'Giữ cả hai', onConfirm: onKeep }}
      onConfirm={onReplace} onClose={() => setConfirmReplace(false)} />}
    <ResultCard role="alert" style={{ borderColor: 'rgba(246,203,140,0.28)' }}>
      <CardHead alignTop tint="rgba(246,203,140,0.1)" title="Đã có tệp trùng tên — chưa xuất gì" sub="Noverse dừng lại để không ghi đè mất bản cũ.">
        <Icon name="warning-2" size={20} style={{ color: '#F6CB8C' }} />
      </CardHead>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(2, minmax(0, 1fr))', gap: 8 }}>
        {[
          { label: 'Bản đang có', line: existingLine, border: 'rgba(255,255,255,0.06)', color: undefined },
          { label: 'Bản mới', line: incomingLine, border: 'rgba(166,237,193,0.18)', color: '#A6EDC1' },
        ].map((box) => (
          <div key={box.label} style={{
            padding: '10px 12px', borderRadius: 10, background: '#0F0F11', border: `1px solid ${box.border}`,
            display: 'flex', flexDirection: 'column', gap: 2, minWidth: 0,
          }}>
            <MiniLabel color={box.color}>{box.label}</MiniLabel>
            <span style={{ fontSize: 12.5, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{FORMAT_LABEL[conflict.format] ?? 'Ebook'} · {name}</span>
            <span className="num" style={{ fontSize: 12, color: '#8E8E99' }}>{box.line}</span>
          </div>
        ))}
      </div>
      <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
        <PrimaryBtn icon={busy === 'replace' ? undefined : 'refresh-2'} onClick={() => setConfirmReplace(true)} disabled={busy !== null}>
          {busy === 'replace' && <span className="cr-spinner" style={{ width: 13, height: 13, borderTopColor: '#0B0B0D' }} />}Thay bản cũ
        </PrimaryBtn>
        <OutlineBtn icon={busy === 'keep' ? undefined : 'copy'} onClick={onKeep} disabled={busy !== null}>
          {busy === 'keep' && <span className="cr-spinner" style={{ width: 13, height: 13 }} />}Giữ cả hai (thêm giờ vào tên)
        </OutlineBtn>
        <OutlineBtn bare onClick={onCancel} disabled={busy !== null}>Huỷ</OutlineBtn>
      </div>
      <span style={{ fontSize: 11.5, color: '#5E5E68' }}>
        Tương đương: thêm <span className="mono">--overwrite</span> để thay, <span className="mono">--keep</span> để giữ cả hai
      </span>
    </ResultCard>
    </>
  )
}

// ─── /sync ────────────────────────────────────────────────────────────────

function DiffView({ diff }: { diff: SyncDiff }) {
  const lines = paragraphDiff(diff.before, diff.after)
  return (
    <div className="scroll" style={{ maxHeight: 260, overflowY: 'auto', padding: '8px 12px 12px 70px', display: 'flex', flexDirection: 'column', gap: 4 }}>
      {lines.length === 0 && <span style={{ fontSize: 12.5, color: '#7E7E8A' }}>Không có khác biệt về nội dung.</span>}
      {lines.map((line, i) => (
        <span key={i} style={{
          fontSize: 12.5, lineHeight: 1.55, padding: '3px 8px', borderRadius: 6,
          color: line.kind === 'same' ? '#7E7E8A' : line.kind === 'add' ? '#CFF5DD' : '#FFD3D7',
          background: line.kind === 'add' ? 'rgba(166,237,193,0.08)' : line.kind === 'del' ? 'rgba(255,159,168,0.08)' : 'transparent',
          textDecoration: line.kind === 'del' ? 'line-through' : undefined, textDecorationColor: 'rgba(255,159,168,0.45)',
          fontStyle: line.kind === 'same' && line.text.startsWith('…') ? 'italic' : undefined,
        }}>{line.kind === 'add' ? '+ ' : line.kind === 'del' ? '− ' : ''}{line.text}</span>
      ))}
    </div>
  )
}

function SyncDiffPanel({ project, story, chapter }: StoryRef & { chapter: number }) {
  const resource = useResource(['story', project, story, 'sync-diff', chapter], ({ signal }) => getSyncDiff(project, story, chapter, signal))
  return (
    <div>
      <PanelRefreshBar resourceKeys={[resource.key]} />
      {!!resource.error && <div role="alert" style={{ padding: '6px 12px 12px 70px', fontSize: 12.5, color: '#FF9FA8' }}>
        {errorText(resource.error, 'Không tải được khác biệt')} · <button onClick={() => void resource.refresh()}>Thử lại</button>
      </div>}
      {resource.data ? <DiffView diff={resource.data} /> : !resource.error && (
        <div role="status" style={{ padding: '6px 12px 12px 70px', display: 'flex', gap: 8, alignItems: 'center', fontSize: 12.5, color: '#8E8E99' }}>
          <span className="cr-spinner" />Đang so sánh…
        </div>
      )}
    </div>
  )
}

export function SyncCheckCard({ changed, project, story, busy, onAccept }: StoryRef & {
  changed: SyncChanged[]; busy: boolean; onAccept: () => void
}) {
  const [openDiff, setOpenDiff] = useState<number | null>(null)

  if (changed.length === 0) {
    return (
      <ResultCard role="status" style={{ borderColor: 'rgba(124,147,243,0.25)' }}>
        <CardHead tint="rgba(124,147,243,0.12)" title="Không có chương nào bị sửa bên ngoài Noverse" sub="Mọi chương khớp với bản Noverse đã lưu — không cần tiếp nhận.">
          <Icon name="refresh-2" size={20} style={{ color: '#9DB0F7' }} />
        </CardHead>
      </ResultCard>
    )
  }
  return (
    <ResultCard style={{ borderColor: 'rgba(124,147,243,0.25)' }}>
      <CardHead tint="rgba(124,147,243,0.12)" title={`${changed.length} chương đã được sửa bên ngoài Noverse`} sub="Chưa tiếp nhận gì. Kiểm tra lại rồi bấm Tiếp nhận.">
        <Icon name="refresh-2" size={20} style={{ color: '#9DB0F7' }} />
      </CardHead>
      <div style={{ display: 'flex', flexDirection: 'column', borderRadius: 12, border: '1px solid rgba(255,255,255,0.06)', overflow: 'hidden' }}>
        {changed.map((row, i) => {
          const isOpen = openDiff === row.chapter
          return (
            <div key={row.chapter} style={{ borderTop: i ? '1px solid rgba(255,255,255,0.06)' : 'none' }}>
              <div className="cr-crow" style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '10px 12px' }}>
                <span className="num" style={{ fontSize: 12, color: '#7E7E8A', minWidth: 46, flex: '0 0 auto', whiteSpace: 'nowrap' }}>Chương {row.chapter}</span>
                <span style={{ flex: '1 1 auto', minWidth: 0, fontSize: 13, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{row.title || `Chương ${row.chapter}`}</span>
                <span className="mono num" style={{ fontSize: 12, color: '#A6EDC1' }}>{formatWordDelta(row.added, '+')}</span>
                <span className="mono num" style={{ fontSize: 12, color: '#FF9FA8' }}>{formatWordDelta(row.removed, '-')}</span>
                <span className="num" style={{ fontSize: 12, color: '#7E7E8A', width: 100, textAlign: 'right', whiteSpace: 'nowrap' }}>{formatEditedAt(row.modifiedAt)}</span>
                <button className="cr-tbtn" type="button" aria-expanded={isOpen} onClick={() => setOpenDiff(isOpen ? null : row.chapter)} style={{
                  height: 26, padding: '0 8px', borderRadius: 7, border: '1px solid rgba(255,255,255,0.08)',
                  background: isOpen ? 'rgba(255,255,255,0.06)' : 'transparent', color: '#C9C9D1', fontSize: 11.5, whiteSpace: 'nowrap',
                }}>{isOpen ? 'Ẩn khác biệt' : 'Xem khác biệt'}</button>
              </div>
              {isOpen && <SyncDiffPanel project={project} story={story} chapter={row.chapter} />}
            </div>
          )
        })}
      </div>
      <div style={{ display: 'flex', gap: 8, alignItems: 'center', flexWrap: 'wrap' }}>
        <PrimaryBtn icon={busy ? undefined : 'tick-circle'} onClick={onAccept} disabled={busy}>
          {busy && <span className="cr-spinner" style={{ width: 13, height: 13, borderTopColor: '#0B0B0D' }} />}
          {changed.length === 1 ? 'Tiếp nhận chương này' : `Tiếp nhận cả ${changed.length} chương`}
        </PrimaryBtn>
        <span style={{ fontSize: 12, color: '#7E7E8A' }}>Noverse sẽ đọc lại, cập nhật tóm tắt và báo nếu chỗ sửa mâu thuẫn với chương sau.</span>
      </div>
    </ResultCard>
  )
}

/** Việc nền đang chạy không có % (tiếp nhận chương) — vạch chạy + dòng trạng thái của job. */
export function SyncRunCard({ job }: { job: JobEvent | undefined }) {
  return (
    <ResultCard role="status" label="Đang tiếp nhận chương sửa tay" style={{ borderColor: 'rgba(124,147,243,0.25)' }}>
      <CardHead tint="rgba(124,147,243,0.12)" title="Đang tiếp nhận chương sửa tay" sub={job?.message ? translateHost(job.message) : 'Đang đọc lại các chương đã sửa…'}>
        <span className="cr-spinner" style={{ borderTopColor: '#9DB0F7' }} />
      </CardHead>
      <div className="cr-bar cr-indet"><i style={{ background: 'linear-gradient(90deg, #9DB0F7, #7C93F3)' }} /></div>
    </ResultCard>
  )
}

export function SyncDoneCard({ result, project, story }: StoryRef & { result: SyncResult }) {
  const [asked, setAsked] = useState<Set<string>>(() => new Set())
  const { issues } = syncSummary(result.analyses)
  const byChapter = new Map(result.analyses.map((a) => [a.chapter, a]))
  const chapters = [...new Set([...result.applied, ...result.analyses.map((a) => a.chapter)])].sort((a, b) => a - b)

  const fixAction = useAction(async (key: string, chapter: number, text: string) => {
    await sendInput(project, story, `Sửa chương ${chapter} cho khớp với chỗ vừa sửa tay: ${text}`)
    setAsked((previous) => new Set(previous).add(key))
    toast.ok(`Đã nhờ sửa chương ${chapter}`)
  }, { error: 'Không gửi được yêu cầu sửa', invalidate: [['story', project, story, 'snapshot']] })

  if (chapters.length === 0) {
    return (
      <ResultCard role="status">
        <CardHead tint="rgba(166,237,193,0.1)" title="Không có chương nào cần tiếp nhận" sub="Các chương đều khớp với bản Noverse đã lưu.">
          <DrawTick size={20} color="#A6EDC1" stroke={2.4} />
        </CardHead>
      </ResultCard>
    )
  }
  return (
    <ResultCard role="status">
      <CardHead tint="rgba(166,237,193,0.1)" title={`Đã tiếp nhận chương ${formatChapterList(chapters)}`}
        sub={issues > 0 ? `Tóm tắt và sự thật cốt truyện đã cập nhật · ${issues} chỗ cần bạn xem` : 'Tóm tắt và sự thật cốt truyện đã cập nhật'}>
        <DrawTick size={20} color="#A6EDC1" stroke={2.4} />
      </CardHead>
      {chapters.map((chapter) => {
        const analysis = byChapter.get(chapter)
        const downstream = analysis?.downstreamIssues ?? []
        const chips = [
          analysis?.storyChanged
            ? { icon: 'flag', label: 'Sự thật cốt truyện đã cập nhật', bg: 'rgba(201,166,245,0.1)', fg: '#DCC8F8' }
            : { icon: 'tick-circle', label: 'Chỉ đổi lời văn', bg: 'rgba(255,255,255,0.05)', fg: '#C9C9D1' },
          ...(downstream.length > 0 ? [{ icon: 'warning-2', label: `${downstream.length} mâu thuẫn phía sau`, bg: 'rgba(246,203,140,0.1)', fg: '#F6CB8C' }] : []),
        ]
        return (
          <div key={chapter} style={{
            display: 'flex', flexDirection: 'column', gap: 8, padding: '10px 12px', borderRadius: 12, background: '#0F0F11',
            border: `1px solid ${downstream.length > 0 ? 'rgba(246,203,140,0.22)' : 'rgba(255,255,255,0.06)'}`,
          }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap' }}>
              <span className="num" style={{ fontSize: 12, color: '#7E7E8A', whiteSpace: 'nowrap' }}>Chương {chapter}</span>
              <span style={{ fontSize: 13, flex: '1 1 auto', minWidth: 0 }}>{analysis?.changeSummary || analysis?.title || `Chương ${chapter}`}</span>
              {chips.map((chip) => (
                <span key={chip.label} style={{
                  height: 22, padding: '0 8px', borderRadius: 6, background: chip.bg, color: chip.fg, fontSize: 11.5,
                  display: 'flex', alignItems: 'center', gap: 5, whiteSpace: 'nowrap',
                }}><Icon name={chip.icon} size={12} />{chip.label}</span>
              ))}
            </div>
            {downstream.map((issue, k) => {
              const key = `${chapter}:${k}`
              return (
                <div key={key} style={{
                  display: 'flex', gap: 10, alignItems: 'center', padding: '8px 10px', borderRadius: 9,
                  background: 'rgba(246,203,140,0.06)', border: '1px solid rgba(246,203,140,0.2)', flexWrap: 'wrap',
                }}>
                  <span style={{ flex: '1 1 260px', fontSize: 12.5, color: '#E9D9BD' }}>{issue.text}</span>
                  {issue.chapter !== null && (
                    <>
                      <OutlineBtn icon="book-1" iconSize={14} height={28} onClick={() => navigate(paths.reader(project, story, issue.chapter ?? undefined))}
                        style={{ padding: '0 10px', borderRadius: 8, fontSize: 12 }}>Mở chương {issue.chapter}</OutlineBtn>
                      <OutlineBtn icon={asked.has(key) ? 'tick-circle' : 'magic-star'} iconSize={14} iconColor={asked.has(key) ? '#A6EDC1' : '#C9A6F5'} height={28}
                        disabled={asked.has(key) || fixAction.busy} onClick={() => void fixAction.run(key, issue.chapter!, issue.text)}
                        style={{ padding: '0 10px', borderRadius: 8, fontSize: 12 }}>
                        {asked.has(key) ? 'Đã nhờ sửa' : `Nhờ sửa chương ${issue.chapter}`}
                      </OutlineBtn>
                    </>
                  )}
                </div>
              )
            })}
          </div>
        )
      })}
    </ResultCard>
  )
}

// ─── /simulate ────────────────────────────────────────────────────────────

const SIM_STAGE_INDEX: Record<string, number> = { scan: 0, analyze: 1, merge: 2, done: 3 }

export function SimulateRunCard({ job }: { job: JobEvent | undefined }) {
  const info = job?.simulate
  const step = SIM_STAGE_INDEX[job?.stage ?? 'scan'] ?? 0
  const steps = [
    info && info.sources > 0 ? `Đọc ${formatInt(info.sources)} văn mẫu bạn đã thêm` : 'Đọc mọi văn mẫu bạn đã thêm',
    'Phân tích nhịp câu, xưng hô và từ vựng',
    'Tạo hồ sơ văn phong',
  ]
  return (
    <ResultCard role="status" label="Đang học văn phong" style={{ borderColor: 'rgba(246,203,140,0.2)' }}>
      <CardHead tint="rgba(246,203,140,0.1)" title="Đang học văn phong"
        sub={info && info.sources > 0 ? `${formatInt(info.sources)} văn mẫu · ${formatInt(info.words)} chữ` : 'Đang tìm văn mẫu…'}>
        <Icon name="magic-star" size={20} style={{ color: '#F6CB8C' }} />
      </CardHead>
      <div className="cr-bar cr-indet"><i style={{ background: 'linear-gradient(90deg, #F6CB8C, #EC9548)' }} /></div>
      <ol style={{ margin: 0, padding: 0, listStyle: 'none', display: 'flex', flexDirection: 'column', gap: 8 }}>
        {steps.map((text, i) => (
          <li key={i} style={{ display: 'flex', gap: 10, alignItems: 'center', fontSize: 13, color: i <= step ? '#EDEDEF' : '#7E7E8A' }}>
            {i < step && <Icon name="tick-circle" size={15} style={{ color: '#A6EDC1' }} />}
            {i === step && <span className="cr-spinner" style={{ margin: 1 }} />}
            {i > step && <span aria-hidden="true" style={{ width: 15, height: 15, borderRadius: '50%', border: '1.5px dashed rgba(255,255,255,0.2)', flex: '0 0 auto' }} />}
            <span>{text}</span>
          </li>
        ))}
      </ol>
    </ResultCard>
  )
}

export function SimulateDoneCard({ result, project, story }: StoryRef & { result: SimulateJobResult }) {
  const traits = simulationTraits(result.synthesis)
  const quote = signatureQuote(result.synthesis)
  const saveAction = useAction(() => downloadSimulationProfile(project, story), { success: 'Đã tải hồ sơ văn phong', error: 'Không tải được hồ sơ' })
  return (
    <ResultCard role="status" label="Hồ sơ văn phong đã cập nhật" style={{
      background: 'radial-gradient(120% 120% at 0% 0%, rgba(246,203,140,0.06), transparent 50%), linear-gradient(180deg,#171719 0%,#131315 100%)',
    }}>
      <CardHead tint="rgba(246,203,140,0.1)" title="Hồ sơ văn phong đã cập nhật"
        sub={`Học từ ${formatInt(result.sources)} văn mẫu · các chương viết sau sẽ theo giọng này`}
        aside={(
          <span style={{
            height: 24, padding: '0 9px', borderRadius: 7, background: 'rgba(166,237,193,0.1)', color: '#A6EDC1', fontSize: 12,
            display: 'flex', alignItems: 'center', gap: 5, whiteSpace: 'nowrap',
          }}><Icon name="tick-circle" size={13} />Đang dùng</span>
        )}>
        <Icon name="magic-star-bulk" size={20} style={{ color: '#F6CB8C' }} />
      </CardHead>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, minmax(0, 1fr))', gap: 8 }}>
        {traits.map((trait) => (
          <div key={trait.key} style={{
            padding: '10px 12px', borderRadius: 10, background: '#0F0F11', border: '1px solid rgba(255,255,255,0.05)',
            display: 'flex', flexDirection: 'column', gap: 2, minWidth: 0,
          }}>
            <MiniLabel>{trait.key}</MiniLabel>
            <span style={{ fontSize: 13 }}>{trait.value}</span>
          </div>
        ))}
      </div>
      {quote && (
        <div style={{
          padding: '10px 14px', borderLeft: '2px solid #F6CB8C', background: 'rgba(246,203,140,0.04)', borderRadius: '0 10px 10px 0',
          fontSize: 13.5, color: '#D6D6DC', fontStyle: 'italic',
        }}>“{quote}” — câu tiêu biểu trong văn mẫu</div>
      )}
      <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
        <OutlineBtn icon="document-download" height={32} onClick={() => void saveAction.run()} disabled={saveAction.busy} pending={saveAction.pending}>Tải hồ sơ văn phong</OutlineBtn>
        <OutlineBtn icon="setting-2" height={32} onClick={() => navigate(paths.settings('writing'))}>Quản lý hồ sơ văn phong</OutlineBtn>
      </div>
    </ResultCard>
  )
}

// ─── việc lỗi ─────────────────────────────────────────────────────────────

const FAIL_TITLE: Record<string, string> = {
  export: 'Xuất không thành công', sync: 'Tiếp nhận không thành công', simulate: 'Học văn phong không thành công',
}

export function JobFailedCard({ kind, error, onRetry }: { kind: string; error: string; onRetry: () => void }) {
  return (
    <ResultCard role="alert" style={{ borderColor: 'rgba(255,159,168,0.28)' }}>
      <CardHead alignTop tint="rgba(255,159,168,0.1)" title={FAIL_TITLE[kind] ?? 'Lệnh không thành công'} sub={translateHost(error)}>
        <Icon name="warning-2" size={20} style={{ color: '#FF9FA8' }} />
      </CardHead>
      {kind === 'simulate' && (
        <span style={{ fontSize: 12.5, color: 'var(--color-fg-2)' }}>
          Thêm vài đoạn văn bạn thích vào Văn mẫu (Tuỳ chỉnh › Cách viết › Văn phong của bạn) rồi chạy lại.
        </span>
      )}
      <div style={{ display: 'flex', gap: 8 }}>
        <OutlineBtn icon="refresh-2" onClick={onRetry}>Thử lại</OutlineBtn>
      </div>
    </ResultCard>
  )
}
