// “Ghép & xuất Tập N” — AudioExport.dc.html (state confirm). Gói AE: hộp đóng ngay,
// ghép chạy nền trong dock việc (job.* SSE); công tắc Tự tải nhớ theo thiết bị.
import type { ReactNode } from 'react'
import * as stylex from '@stylexjs/stylex'
import type { AudioCast, AudioChapter, AudioVoice } from '../../api/types'
import { ApiError, createAudioExport } from '../../api/client'
import { keys } from '../../data'
import { invalidate } from '../../data/cache'
import { navigate, paths } from '../../router'
import { Dialog } from '@noverse/ui'
import { Button } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { toast } from '@noverse/ui'
import { allDone, formatBytes, formatDuration, formatClock, exportDiskNeed } from './logic'
import { MiniSwitch } from './ui'
import { Segmented } from '@noverse/ui'
import { useExportForm } from './useExportForm'
import { exportStyles as sx } from './export.stylex'
import { exportStartError, registerExportWatch, useAutoDownload } from '../../shell/jobs/exportFlow'
import type { PackVoice } from './packVoice'
import { errText } from './useAudio'
import { t, audioExportCopy as c } from '../../i18n/vi'
import { CharAvatar } from '../../ui/CharAvatar'

const MINI_RING = ['rgba(185,208,247,.7)', 'rgba(246,203,140,.7)', 'rgba(166,237,193,.7)', 'rgba(201,201,209,.6)', 'rgba(246,203,140,.5)', 'rgba(255,255,255,.25)']
const MINI_MAX = 5

const ONES = ['không', 'một', 'hai', 'ba', 'bốn', 'năm', 'sáu', 'bảy', 'tám', 'chín']
/** Số chương đọc thành chữ (1–99), dùng cho ví dụ “Chương ba. …”. */
function spokenNumber(n: number): string {
  if (n < 10) return ONES[n]
  if (n < 20) return n === 10 ? 'mười' : `mười ${n === 15 ? 'lăm' : ONES[n - 10]}`
  if (n > 99) return String(n)
  const unit = n % 10
  const tail = unit === 0 ? '' : unit === 1 ? ' mốt' : unit === 5 ? ' lăm' : ` ${ONES[unit]}`
  return `${ONES[Math.floor(n / 10)]} mươi${tail}`
}

export function ConfirmExportDialog({ project, story, volume, chapters, cast, voices, pack, onClose, initialChapters }: {
  project: string; story: string; volume: number
  chapters: AudioChapter[]; cast: AudioCast | null; voices: AudioVoice[]
  /** D11 — gói một giọng kể: thẻ Giọng đọc ghi “Một giọng kể · theo gói”. */
  pack: PackVoice | null
  onClose: () => void
  initialChapters?: number[]
}) {
  const done = allDone(chapters)
  const shared = useExportForm(project, story, true, { whenReady: !done, split: initialChapters !== undefined, chapters: initialChapters ?? [] })
  const { form, update, visible } = shared
  const [autoDl, setAutoDl] = useAutoDownload()
  const selected = form.chapters ?? []
  const ready = chapters.filter((item) => item.audio && item.audio.segmentsDone >= item.audio.segments)
  const chosen = form.split ? ready.filter((item) => selected.includes(item.chapter)) : chapters
  const duration = chosen.reduce((sum, item) => sum + (item.durationSec ?? item.audio?.durationSec ?? 0), 0)
    + (!form.split && form.gap ? form.gapSec : 0) * Math.max(0, chosen.length - 1)
  const single = pack !== null || cast?.mode === 'single'
  const voiceCount = single ? 1 : new Set(cast?.speakers.map((speaker) => speaker.voiceId).filter(Boolean)).size
  const completed = chapters.filter((chapter) => chapter.audio && chapter.audio.segmentsDone >= chapter.audio.segments).length
  const sample = chapters[2] ?? chapters[chapters.length - 1]
  const scopeLabel = form.split
    ? (selected.length === 1 ? `chương ${selected[0]}` : `${selected.length} chương`)
    : `Tập ${volume}`
  /** §AE — hộp đóng ngay; lệnh ghép chạy nền, tiến độ/kết quả qua dock việc + toast. */
  const start = () => {
    registerExportWatch(project, story, volume, {
      what: scopeLabel,
      verb: form.split ? 'xuất' : 'ghép',
      auto: autoDl,
      all: form.split === true && (form.pack ?? 'files') === 'files',
    })
    onClose()
    void (async () => {
      try {
        await shared.save()
        await createAudioExport(project, story, volume, shared.options)
        void invalidate(keys.story(project, story, 'audio'))
      } catch (cause) {
        const notice = exportStartError(cause instanceof ApiError ? cause : { message: errText(cause) })
        toast.err(notice?.text ?? errText(cause))
      }
    })()
  }
  const pending = form.whenReady && !done
  const shown = cast?.speakers.slice(0, MINI_MAX) ?? []
  const more = (cast?.speakers.length ?? 0) > MINI_MAX
  const singleVoice = voices.find((voice) => voice.id === cast?.single.voiceId)

  return (
    <Dialog title={form.split ? c.title(volume) : t.audio.confirm.ghepXuatTap(volume)} icon={<Icon name="sound" size={20} style={{ color: 'var(--color-accent)' }} />} width={820} padX={20}
      onClose={onClose} style={{ position: 'absolute', top: 130, maxHeight: 'calc(100dvh - 146px)' }}
      footer={<>
        <span style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', gap: 6, alignSelf: 'center' }}>
          <span className="num" style={{ fontSize: 12.5, color: 'var(--color-fg-3)' }}>
            {form.split ? selected.length ? c.footer(selected.length, Math.floor(duration / 60), Math.round(duration / 60 * ({ mp3: 1.18, m4b: .95, wav: 10.1 })[form.format]), form.subtitles) : c.selectMinimum : t.audio.confirm.chayNenKhoang(1)}</span>
          <label style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 12.5, color: 'var(--color-fg-2)', cursor: 'pointer', width: 'max-content' }}>
            <MiniSwitch on={autoDl} onChange={setAutoDl} label={c.autoDownload} />{c.autoDownload}
          </label>
        </span>
        <Button variant="ghost" onClick={onClose}>{t.dialogs.cancel}</Button>
        <Button variant="primary" icon={<Icon name="export-curve" size={14} />} onClick={start}
          disabled={shared.loading || !!shared.error || (form.split && selected.length === 0)}>
          {form.split ? c.export(selected) : pending ? t.audio.confirm.xuatKhiXong : t.audio.confirm.ghepXuatTap(volume)}
        </Button>
      </>}>
      <div className="scroll" style={{ minHeight: 0, padding: '18px 20px', display: 'flex', flexDirection: 'column', gap: 18 }}>
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, minmax(0, 1fr))', gap: 10 }}>
          <Cell label="Giọng đọc đã tạo"
            value={<span style={{ display: 'flex', alignItems: 'center', gap: 6 }}>{done && <Icon name="tick-circle-bulk" size={18} style={{ color: 'var(--color-ok)' }} />}{completed}/{chapters.length} chương</span>}
            hint={`${chapters.reduce((sum, item) => sum + item.segments, 0)} đoạn · đã nghe & duyệt · không tạo lại`} />
          <Cell label={form.split ? c.selectedDuration : 'Độ dài cả tập'} value={<span className="num">{formatDuration(duration)}</span>} hint={form.split ? c.noMusic : 'kể cả nhạc mở/kết và khoảng lặng'} />
          <div className="card" style={{ borderRadius: 14, padding: '12px 14px', display: 'flex', flexDirection: 'column', gap: 6 }}>
            <span style={{ fontSize: 12, color: 'var(--color-fg-3)' }}>Giọng đọc</span>
            {pack
              ? <span style={{ fontSize: 18, fontWeight: 600, display: 'flex', alignItems: 'center', gap: 8 }}>{t.audio.pack.motGiongKe}
                <span style={{
                  height: 20, padding: '0 7px', display: 'flex', alignItems: 'center', borderRadius: 6,
                  background: 'var(--tint-ok)', color: 'var(--color-ok)', fontSize: 11.5, fontWeight: 500,
                }}>{t.audio.pack.badge}</span>
              </span>
              : <span style={{ fontSize: 18, fontWeight: 600 }}>{voiceCount} giọng</span>}
            {single
              ? <span style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 12.5, color: 'var(--color-fg-2)' }}>
                <Icon name="microphone-2" size={14} style={{ color: 'var(--color-fg-3)' }} />
                {pack ? t.audio.pack.tomTat(pack.ten, pack.gender)
                  : `${singleVoice?.name ?? 'Giọng đọc chung'}${cast?.single.dialogueShift ? ' · lời thoại đổi giọng nhẹ' : ''}`}
              </span>
              : <div style={{ display: 'flex' }}>
                {shown.map((speaker, index) => (
                  <CharAvatar key={speaker.id} name={speaker.name} ini={speaker.ini} kind={speaker.kind} size={28}
                    style={{ marginLeft: index ? -3 : 0, boxShadow: '0 0 0 2px var(--color-surface-2)' }} />
                ))}
                {more && <span title="Nhân vật phụ" style={{
                  width: 28, height: 28, flex: '0 0 auto', marginLeft: -3, borderRadius: '50%', background: 'var(--color-surface-6)',
                  boxShadow: `0 0 0 2px var(--color-surface-2), 0 0 0 3.5px ${MINI_RING[5]}`,
                  display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: 9.5, fontWeight: 600,
                }}>…</span>}
              </div>}
            <button type="button" style={{ textAlign: 'left', padding: 0, fontSize: 12, color: 'var(--color-fg-3)' }}
              onClick={() => {
                onClose()
                navigate(pack ? paths.settings('writing') : paths.audio(project, story, volume, 'review'))
              }}>
              {pack ? t.audio.pack.goiChonSan : single ? 'Đổi giọng' : 'Sửa phân vai'}
            </button>
          </div>
        </div>
        <div style={{ display: 'flex', gap: 10, padding: '10px 12px', borderRadius: 12, background: 'var(--color-surface-0)', border: '1px solid var(--line-1)', fontSize: 12.5, color: 'var(--color-fg-3)' }}>
          <Icon name="info-circle" size={16} style={{ marginTop: 1 }} />
          <span>Bước này <span style={{ color: 'var(--color-fg)' }}>chỉ ghép</span> audio đã có của {chapters.length} chương thành file để đăng — không đọc lại gì. Đoạn nào muốn đổi giọng hãy sửa ở trình nghe trước khi ghép.</span>
        </div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
          <span className="lbl">{c.scope}</span>
          <div {...stylex.props(sx.scopes)} role="radiogroup" aria-label={c.scope}>
            {[false, true].map((split) => <button key={String(split)} {...stylex.props(sx.scope, !!form.split === split && sx.selected)} type="button" role="radio" aria-checked={!!form.split === split} onClick={() => update('split', split)}>
              <input type="radio" checked={!!form.split === split} readOnly tabIndex={-1} aria-hidden="true" />
              <span>{split ? c.chapters : c.whole}<small style={{ display: 'block', color: 'var(--color-fg-4)', fontSize: 12 }}>{split ? c.splitHint : c.wholeHint}</small></span>
            </button>)}
          </div>
          {form.split && <div className="card" style={{ borderRadius: 14, padding: '12px 14px', display: 'flex', flexDirection: 'column', gap: 10 }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}><span style={{ flex: 1, fontSize: 13 }}>{c.select(selected.length, chapters.length)}</span><Button size="sm" onClick={() => update('chapters', selected.length === ready.length ? [] : ready.map((item) => item.chapter))}>{selected.length === ready.length ? c.clear : c.all(ready.length)}</Button></div>
            <div {...stylex.props(sx.grid)} role="group" aria-label={c.chapters}>{chapters.map((item) => {
              const on = selected.includes(item.chapter), enabled = ready.includes(item)
              return <button key={item.chapter} type="button" role="checkbox" aria-checked={on} disabled={!enabled} title={item.title} {...stylex.props(sx.chapter, on && sx.selected, !enabled && sx.disabled)} onClick={() => update('chapters', on ? selected.filter((n) => n !== item.chapter) : [...selected, item.chapter].sort((a, b) => a - b))}><input type="checkbox" checked={on} readOnly tabIndex={-1} aria-hidden="true" /><span>{item.chapter}</span><small style={{ marginLeft: 'auto', color: 'var(--color-fg-4)' }}>{formatClock(item.durationSec ?? 0)}</small></button>
            })}</div><span style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>{c.selectHint}</span>
          </div>}
        </div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
          <span className="lbl">Thành phẩm</span>
          <div className="card" style={{ borderRadius: 14, padding: '4px 14px' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '10px 0' }}>
              <Icon name="audio-square" size={18} style={{ color: 'var(--color-fg-3)' }} />
              <span style={{ flex: 1, display: 'flex', flexDirection: 'column' }}>
                <span style={{ fontSize: 13.5 }}>{c.format}</span>
                <span style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>
                  {c.formatNotes[form.format]}
                </span>
              </span>
              <Segmented label={t.audio.confirm.dinhDang} size="sm" radius={6} value={form.format}
                onChange={(format) => update('format', format as typeof form.format)}
                options={(['mp3', 'm4b', 'wav'] as const).map((format) => ({
                  id: format, label: format.toUpperCase(), style: { padding: '0 12px', flex: '0 1 auto' },
                }))}
                style={{ gap: 2, borderRadius: 9 }} />
            </div>
            {visible.intro && <Row icon="music" label={c.intro} hint={c.introHint} on={form.intro} onChange={(value) => update('intro', value)} />}
            <Row icon="voice-square" label="Đọc tên chương ở đầu mỗi chương"
              hint={sample ? `“Chương ${spokenNumber(sample.chapter)}. ${sample.title}.”` : 'Đọc số và tên chương trước nội dung'}
              on={form.chapterTitles} onChange={(value) => update('chapterTitles', value)} />
            <Row icon="volume-high" label="Cân bằng âm lượng để đăng mạng" hint="Các giọng to nhỏ đều nhau, hợp YouTube và podcast" on={form.loudness} onChange={(value) => update('loudness', value)} />
            {visible.gap && <Row icon="timer-1" label={c.gap} hint={c.gapHint} on={form.gap} onChange={(value) => update('gap', value)} />}
            {visible.chapterMarks && <Row icon="tag" label={c.marks} hint={c.marksHint} on={form.chapterMarks} onChange={(value) => update('chapterMarks', value)} />}
            <Row icon="document-text" label={c.srt} hint={c.srtHint} on={form.subtitles} onChange={(value) => update('subtitles', value)} />
            {visible.pack && <div style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '10px 0', borderTop: '1px solid var(--line-1)' }}><Icon name="folder-open" size={18} /><span style={{ flex: 1, fontSize: 13.5 }}>{c.receive}<small style={{ display: 'block', fontSize: 12, color: 'var(--color-fg-4)' }}>{c.splitNote}</small></span><Segmented label={c.receive} value={form.pack ?? 'files'} onChange={(pack) => update('pack', pack as 'files' | 'zip')} size="sm" options={[{ id: 'files', label: c.separate }, { id: 'zip', label: c.zip }]} /></div>}
          </div>
        </div>
        <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap' }}>
          <div style={{ flex: '1 1 300px', display: 'flex', alignItems: 'center', gap: 10, padding: '10px 12px', borderRadius: 12, background: 'var(--color-surface-0)', border: '1px solid var(--line-1)', fontSize: 12.5, color: 'var(--color-fg-3)' }}>
            <Icon name="folder-open" size={16} /><span>{t.audio.confirm.luuVao} <span style={{ color: 'var(--color-fg)' }}>{t.audio.confirm.banXuatTap(volume)}</span></span>
          </div>
          <div style={{ flex: '1 1 220px', display: 'flex', alignItems: 'center', gap: 10, padding: '10px 12px', borderRadius: 12, background: 'var(--color-surface-0)', border: '1px solid var(--line-1)', fontSize: 12.5, color: 'var(--color-fg-3)' }}>
            <Icon name="box" size={16} /><span className="num">{t.audio.confirm.canTrong(formatBytes(exportDiskNeed(duration, form.format)))}</span>
          </div>
        </div>
        <div style={{ display: 'flex', gap: 10, padding: '10px 12px', borderRadius: 12, border: '1px solid var(--line-2)', fontSize: 12.5, color: 'var(--color-fg-3)' }}>
          <Icon name="cpu" size={16} style={{ marginTop: 1 }} /><span>Đọc ngay trên máy của bạn, không gửi chữ đi đâu. Máy sẽ chạy nặng trong lúc tạo — tạm dừng được bất cứ lúc nào, chương đã xong được giữ lại.</span>
        </div>
        {shared.error != null && <span role="alert">{errText(shared.error)}</span>}
      </div>
    </Dialog>
  )
}

function Cell({ label, value, hint }: { label: string; value: ReactNode; hint: string }) {
  return <div className="card" style={{ borderRadius: 14, padding: '12px 14px', display: 'flex', flexDirection: 'column', gap: 2 }}>
    <span style={{ fontSize: 12, color: 'var(--color-fg-3)' }}>{label}</span>
    <span style={{ fontSize: 18, fontWeight: 600 }}>{value}</span>
    <span style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>{hint}</span>
  </div>
}

function Row({ icon, label, hint, on, onChange }: {
  icon: string; label: string; hint: string; on: boolean; onChange: (next: boolean) => void
}) {
  return <div style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '10px 0', borderTop: '1px solid var(--line-1)' }}>
    <Icon name={icon} size={18} style={{ color: 'var(--color-fg-3)' }} />
    <span style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column' }}>
      <span style={{ fontSize: 13.5 }}>{label}</span>
      <span style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>{hint}</span>
    </span>
    <MiniSwitch on={on} onChange={onChange} label={label} width={36} height={20} />
  </div>
}
