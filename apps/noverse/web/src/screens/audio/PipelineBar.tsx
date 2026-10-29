// Thanh quy trình Audio dùng chung 06/07 — Transcript.dc.html (mode review: chip audio + chất lượng + nút
// “Nghe & xuất Tập N”) và AudioExport.dc.html (mode make/listen: bước có dấu tick + dòng tóm tắt bên phải).
import { useEffect, useRef, useState } from 'react'
import { paths, type AudioMode } from '../../router'
import type { AudioChapter, AudioQueueJob } from '../../api/types'
import { Icon } from '@noverse/ui'
import { allDone, allLocked, activeJob, displayPercent, lockedCount, overallPercent } from './logic'
import { t } from '../../i18n/vi'
import { ProgressRing } from '@noverse/ui'

const QUALITY_LABEL: Record<'fast' | 'standard' | 'high', string> = {
  fast: t.audio.pipeline.ngheThuNhanh,
  standard: t.audio.pipeline.chuan,
  high: t.audio.pipeline.caoNhat,
}

type Step = { key: AudioMode; n: number; label: string; meta: string; on: boolean; done: boolean; href: string | null }

export function PipelineBar({ project, story, volume, mode, chapters, jobs, quality, onQuality, audioRunning, topMeta, exported }: {
  project: string; story: string; volume: number; mode: AudioMode
  chapters: AudioChapter[]; jobs: AudioQueueJob[]
  quality: 'fast' | 'standard' | 'high'
  onQuality: (next: 'fast' | 'standard' | 'high') => void
  /** đang có job render chạy (kể cả khi mở màn khác) */
  audioRunning?: boolean
  /** dòng tóm tắt bên phải ở phòng thu / nghe & xuất */
  topMeta?: string
  /** màn “Đã xuất”: cả ba bước đã xong */
  exported?: boolean
}) {
  const [openQuality, setOpenQuality] = useState(false)
  const qualityRef = useRef<HTMLDivElement>(null)
  useEffect(() => {
    if (!openQuality) return
    const close = (event: MouseEvent) => {
      if (!qualityRef.current?.contains(event.target as Node)) setOpenQuality(false)
    }
    document.addEventListener('mousedown', close)
    return () => document.removeEventListener('mousedown', close)
  }, [openQuality])

  const job = activeJob(jobs)
  const done = allDone(chapters)
  const canListen = allLocked(chapters)
  const total = chapters.length
  const locked = lockedCount(chapters)
  const percent = job ? displayPercent(job.percent) : overallPercent(chapters)
  const running = Boolean(job && job.state === 'rendering') || Boolean(audioRunning)
  const paused = job?.state === 'paused'
  const failed = job?.state === 'error'
  const rendered = chapters.filter((chapter) => chapter.audio && chapter.audio.segmentsDone >= chapter.audio.segments).length
  const previewed = chapters.filter((chapter) => chapter.status !== 'locked').reduce((sum, chapter) => sum + (chapter.audio?.segmentsDone ?? 0), 0)

  const href = (key: AudioMode) => `#${paths.audio(project, story, volume, key)}`
  const stepIndex = mode === 'review' ? 0 : mode === 'make' ? 1 : 2
  const steps: Step[] = mode === 'review'
    ? [
        { key: 'review', n: 1, label: t.audio.pipeline.duyetVaChot, meta: `${locked}/${total} chương`, on: true, done: false, href: href('review') },
        { key: 'make', n: 2, label: t.audio.pipeline.taoGiong, meta: done ? `${total}/${total} xong` : 'chạy nền', on: false, done: false, href: href('make') },
        { key: 'listen', n: 3, label: t.audio.pipeline.ngheVaXuat, meta: '', on: false, done: false, href: href('listen') },
      ]
    : [
        { key: 'review', n: 1, label: t.audio.pipeline.duyetVaChot, meta: `${stepIndex === 1 ? locked : total}/${total} chương`, on: stepIndex === 0, done: stepIndex > 0 || !!exported, href: href('review') },
        { key: 'make', n: 2, label: t.audio.pipeline.taoGiong, meta: stepIndex === 1 ? `${done ? total : rendered}/${total} chương` : '', on: stepIndex === 1, done: stepIndex > 1 || !!exported, href: href('make') },
        { key: 'listen', n: 3, label: t.audio.pipeline.ngheVaXuat, meta: exported ? t.audio.pipeline.daXuat : '', on: stepIndex === 2, done: !!exported, href: canListen ? href('listen') : null },
      ]

  const audioText = done
    ? `Audio ${total}/${total} chương`
    : failed ? t.audio.pipeline.coLoiKhiTao
      : paused ? t.audio.pipeline.tamDung(percent)
        : running ? `Đang tạo giọng chương ${job?.chapter ?? ''} · ${percent}%`.replace(/ chương  ·/, ' ·')
          : `Audio ${locked}/${total} chương · ${previewed > 0 ? `đã nghe thử ${previewed} đoạn` : 'chờ chốt'}`
  const ringColor = done ? 'var(--color-ok)' : failed ? 'var(--cmd-error-icon)' : paused ? 'var(--color-warn)' : 'var(--color-accent)'
  const ringPercent = done ? 100 : running || paused ? percent : total ? (locked / total) * 100 : 0

  return (
    <div style={{
      flex: '0 0 auto', display: 'flex', alignItems: 'center', gap: 14, flexWrap: 'wrap',
      padding: '10px 16px', borderBottom: '1px solid rgba(255,255,255,0.06)',
    }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
        <Icon name="headphone-bulk" size={20} style={{ color: '#C9A6F5' }} />
        <h1 style={{ margin: 0, fontSize: 16, fontWeight: 600 }}>{t.audio.pipeline.audioTap(volume)}</h1>
      </div>
      <ol aria-label={t.audio.pipeline.quyTrinhAudio} style={{
        listStyle: 'none', margin: 0, padding: 3, display: 'flex', gap: 2, borderRadius: 10,
        background: '#131315', border: '1px solid rgba(255,255,255,0.06)',
      }}>
        {steps.map((step) => {
          const circleBg = step.done ? 'rgba(166,237,193,0.15)' : step.on ? '#C9A6F5' : '#2A2A2E'
          const circleFg = step.done ? '#A6EDC1' : step.on ? '#0B0B0D' : '#A0A0AB'
          return (
            <li key={step.key}>
              <a href={step.href ?? undefined} aria-current={step.on ? 'step' : undefined} aria-disabled={step.href ? undefined : true}
                className={step.on ? undefined : 'hov-ghost'}
                style={{
                  height: 30, padding: '0 12px 0 8px', display: 'flex', alignItems: 'center', gap: 8, borderRadius: 7,
                  background: step.on ? '#222225' : 'transparent',
                  color: mode === 'review' ? (step.on ? '#EDEDEF' : '#A0A0AB') : (step.on || step.done ? '#EDEDEF' : '#A0A0AB'),
                  fontSize: 13, fontWeight: step.on ? 600 : 400,
                  pointerEvents: step.href ? undefined : 'none',
                }}>
                <span className={mode === 'review' ? 'num' : undefined} style={{
                  width: 20, height: 20, borderRadius: '50%', display: 'flex', alignItems: 'center', justifyContent: 'center',
                  fontSize: 11, fontWeight: 600, background: circleBg, color: circleFg,
                }}>
                  {step.done ? <Icon name="tick-circle-bulk" size={14} /> : step.n}
                </span>
                {step.label}
                <span style={{ fontSize: 12, color: '#7E7E8A', fontWeight: 400 }}>{step.meta}</span>
              </a>
            </li>
          )
        })}
      </ol>
      <span style={{ flex: 1 }} />

      {mode === 'review' ? (
        <>
          <a href={href('make')} title={t.audio.pipeline.xemTienDoTaoGiong}
            style={{
              height: 34, padding: '0 12px 0 8px', borderRadius: 999, background: '#131315',
              border: `1px solid ${running ? 'rgba(201,166,245,0.35)' : 'rgba(255,255,255,0.08)'}`,
              display: 'flex', alignItems: 'center', gap: 8, fontSize: 12.5, color: '#C9C9D1',
            }}>
            <ProgressRing size={22} stroke={2.5} percent={ringPercent} color={ringColor} track="var(--color-surface-7)">
              {running && (
                <span className="audio-eq" style={{
                  position: 'absolute', inset: 0, display: 'flex', alignItems: 'center', justifyContent: 'center',
                  gap: 1.5, color: '#C9A6F5', transform: 'scale(.7)',
                }}><span /><span /><span /></span>
              )}
            </ProgressRing>
            <span className="num" style={{ whiteSpace: 'nowrap' }}>{audioText}</span>
          </a>
          <div ref={qualityRef} style={{ position: 'relative' }}>
            <button type="button" onClick={() => setOpenQuality((open) => !open)} aria-haspopup="menu" aria-expanded={openQuality}
              title={t.audio.pipeline.apDungChoMoiDoan}
              style={{
                height: 34, padding: '0 10px', borderRadius: 9, border: '1px solid rgba(255,255,255,0.08)', background: 'transparent',
                display: 'flex', alignItems: 'center', gap: 6, fontSize: 12.5, color: '#A0A0AB', whiteSpace: 'nowrap',
              }}>
              <Icon name="speedometer" size={14} />
              Chất lượng: <span style={{ color: '#EDEDEF' }}>{QUALITY_LABEL[quality]}</span>
              <Icon name="arrow-down-2" size={13} />
            </button>
            {openQuality && (
              <div role="menu" style={{
                position: 'absolute', top: 'calc(100% + 6px)', right: 0, zIndex: 30, width: 244,
                background: 'var(--color-surface-3)', border: '1px solid var(--line-3)', borderRadius: 12,
                boxShadow: 'var(--dialog-shadow, 0 18px 44px rgba(0,0,0,.5))', padding: 5,
              }}>
                {(['fast', 'standard', 'high'] as const).map((id) => (
                  <button key={id} role="menuitemradio" aria-checked={quality === id} type="button"
                    onClick={() => { setOpenQuality(false); if (id !== quality) onQuality(id) }}
                    style={{
                      width: '100%', textAlign: 'left', padding: '8px 9px', borderRadius: 8,
                      display: 'flex', flexDirection: 'column', gap: 2,
                      background: quality === id ? 'var(--color-surface-6)' : 'transparent',
                    }}>
                    <span style={{ fontSize: 12.5, color: 'var(--color-fg)' }}>{QUALITY_LABEL[id]}</span>
                    <span style={{ fontSize: 11.5, color: 'var(--color-fg-4)' }}>
                      {id === 'fast' ? t.audio.pipeline.kiemTraTruocGiong : id === 'standard' ? t.audio.pipeline.duTotDeDang : t.audio.pipeline.muotNhatChamHon}
                    </span>
                  </button>
                ))}
                <div style={{ padding: '6px 9px 4px', fontSize: 11, color: 'var(--color-fg-4)', borderTop: '1px solid var(--line-1)', marginTop: 4 }}>
                  {t.audio.pipeline.doiChatLuongSe}</div>
              </div>
            )}
          </div>
          <a href={canListen ? href('listen') : undefined} aria-disabled={!canListen} title={canListen ? undefined : t.audio.pipeline.chotDu8Chuong}
            className={canListen ? 'hov-primary' : undefined}
            style={{
              height: 34, padding: '0 14px 0 12px', borderRadius: 9, background: canListen ? '#EDEDEF' : '#2A2A2E',
              color: canListen ? '#0B0B0D' : '#7E7E8A', fontSize: 13, fontWeight: 600, display: 'flex', alignItems: 'center', gap: 8,
              whiteSpace: 'nowrap', pointerEvents: canListen ? 'auto' : 'none',
            }}>
            <Icon name="headphone" size={16} />{t.audio.pipeline.ngheXuatTap(volume)}
          </a>
        </>
      ) : topMeta ? (
        <span className="num" style={{ fontSize: 12.5, color: '#A0A0AB' }}>{topMeta}</span>
      ) : null}
      {job && job.state === 'rendering' && (
        <span className="sr-only" aria-live="polite">
          {t.audio.pipeline.dangTaoGiongChuong(job.chapter, percent, job.etaSec)}</span>
      )}
    </div>
  )
}
