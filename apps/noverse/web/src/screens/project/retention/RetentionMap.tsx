// Bản đồ giữ chân (Outline.dc.html › tab outline · omap=map, D5): dải vị trí 8 đoạn chương,
// vùng cảnh lời hứa 35–60% và vùng cú lật 45–60%, bốn dòng kiểm §2/§3/§7/§8, rồi bảng mẫu §21.
// Mọi con số lấy từ GET …/retention (máy chủ tính); màn này không tự suy ra phép kiểm nào.
import type { CSSProperties } from 'react'
import { Icon } from '@noverse/ui'
import type { Retention, RetentionChapter } from '../../../api/types'
import { Chip } from '../bits'
import { HOOK_LABEL, HOOK_NONE, HOOK_TONE, PAYOFF_LABEL } from './logic'

const MROW: CSSProperties = {
  display: 'grid',
  gridTemplateColumns: '52px minmax(0,1.15fr) minmax(0,1fr) minmax(0,1.1fr) minmax(0,1fr) 30px',
  gap: 12, alignItems: 'start', padding: '10px 14px', fontSize: 12.5, lineHeight: 1.5, color: 'var(--color-fg-2)',
}

/** Dải vị trí + chú giải + bốn dòng kiểm (card “Vị trí trong truyện”). */
function PositionStrip({ retention }: { retention: Retention }) {
  const chapters = retention.chapters
  const columns = `repeat(${Math.max(chapters.length, 1)}, minmax(0, 1fr))`
  const marks = [
    retention.promiseAt === null ? null : { at: retention.promiseAt, name: 'Lời hứa', color: 'var(--color-accent)' },
    retention.twistAt === null ? null : { at: retention.twistAt, name: 'Cú lật', color: 'var(--color-warn)' },
  ].filter((mark): mark is { at: number; name: string; color: string } => mark !== null)
  return (
    <section className="card" aria-label="Vị trí trong truyện" style={{ borderRadius: 16, padding: '14px 16px 12px', marginBottom: 10 }}>
      <div style={{ position: 'relative', height: 62 }}>
        <div aria-hidden="true" style={{
          position: 'absolute', left: '35%', width: '25%', top: 26, height: 20, borderRadius: 5,
          background: 'var(--kit-acc-7)', border: '1px dashed var(--kit-acc-40)',
        }} />
        <div aria-hidden="true" style={{
          position: 'absolute', left: '45%', width: '15%', top: 29, height: 14, borderRadius: 4,
          background: 'var(--tint-warn)', border: '1px dashed var(--frame-warn-50)',
        }} />
        <div style={{ position: 'absolute', left: 0, right: 0, top: 33, display: 'grid', gridTemplateColumns: columns, gap: 3 }}>
          {chapters.map((chapter) => (
            <span key={chapter.n} style={{
              height: 6, borderRadius: 3,
              background: chapter.status === 'written' ? 'var(--color-ok)' : chapter.status === 'writing' ? 'var(--color-accent)' : 'var(--kit-wash-12)',
            }} />
          ))}
        </div>
        {marks.map((mark) => (
          <span key={mark.name} style={{
            position: 'absolute', left: `${mark.at * 100}%`, top: 0, transform: 'translateX(-50%)',
            display: 'flex', flexDirection: 'column', alignItems: 'center',
          }}>
            <span style={{
              height: 20, padding: '0 7px', display: 'flex', alignItems: 'center', borderRadius: 6,
              background: mark.color, color: 'var(--color-bg)', fontSize: 11, fontWeight: 600, whiteSpace: 'nowrap',
            }}>{mark.name} · {Math.round(mark.at * 100)}%</span>
            <span aria-hidden="true" style={{ width: 2, height: 19, background: mark.color }} />
          </span>
        ))}
        <div style={{
          position: 'absolute', left: 0, right: 0, top: 48, display: 'grid', gridTemplateColumns: columns,
          gap: 3, fontSize: 11, color: 'var(--kit-fg-arrow)', textAlign: 'center',
        }}>
          {chapters.map((chapter) => <span key={chapter.n} className="num">{chapter.n}</span>)}
        </div>
      </div>
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: '6px 16px', marginTop: 8, fontSize: 12, color: 'var(--kit-fg-icon)' }}>
        <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
          <span aria-hidden="true" style={{ width: 14, height: 8, borderRadius: 2, border: '1px dashed var(--frame-acc-60)' }} />
          Vùng cảnh lời hứa 35–60% · Master §3
        </span>
        <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
          <span aria-hidden="true" style={{ width: 14, height: 8, borderRadius: 2, border: '1px dashed var(--frame-warn-70)' }} />
          Vùng cú lật giữa truyện 45–60% · Master §8
        </span>
      </div>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 6, marginTop: 12 }}>
        {retention.checks.map((check) => (
          <div key={check.ref} style={{ display: 'flex', alignItems: 'flex-start', gap: 8, fontSize: 12.5, lineHeight: 1.5 }}>
            <Icon name={check.ok ? 'tick-circle' : 'warning-2'} size={15}
              style={{ marginTop: 2, color: check.ok ? 'var(--color-ok)' : 'var(--color-warn)', flex: 'none' }} />
            <span style={{ flex: 1, minWidth: 0, color: 'var(--color-fg-2)' }}>{check.text}</span>
            <Chip tone={{ background: 'var(--kit-wash-5)', color: 'var(--color-fg-3)' }}>{check.ref}</Chip>
          </div>
        ))}
      </div>
    </section>
  )
}

/** Một dòng bảng §21: câu hỏi đang treo · lời hứa · trả thưởng · móc cuối chương. */
function MapRow({ chapter, last, first, flagged, onEditRow }: {
  chapter: RetentionChapter
  last: boolean
  first: boolean
  flagged: boolean
  onEditRow?: (chapter: number) => void
}) {
  const live = chapter.status === 'writing'
  const questions = chapter.questions.length > 0 ? chapter.questions : ['—']
  const tag = chapter.payoff?.tag ?? ''
  return (
    <div style={{
      ...MROW,
      background: live ? 'var(--frame-acc-4)' : 'transparent',
      borderTop: first ? 'none' : '1px solid var(--kit-wash-5)',
    }}>
      <span className="num" style={{ fontSize: 13, fontWeight: 500, color: live ? 'var(--kit-badge-run)' : 'var(--color-fg-3)' }}>{chapter.n}</span>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 4, minWidth: 0 }}>
        {questions.map((question, index) => (
          <span key={index} style={{ display: 'flex', gap: 6 }}>
            <span aria-hidden="true" style={{ color: 'var(--color-info)' }}>?</span>
            <span>{question}</span>
          </span>
        ))}
        {!last && chapter.questions.length < 2 && (
          <span style={{ fontSize: 11.5, color: 'var(--kit-warn-soft)' }}>Ít câu hỏi treo · nên 2–4</span>
        )}
      </div>
      <span>{chapter.promise || '—'}</span>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 4, minWidth: 0 }}>
        {tag !== '' && (
          <span className="pj-chip" style={{
            alignSelf: 'flex-start',
            background: tag === 'midpoint' ? 'var(--frame-warn-14)' : 'var(--kit-acc-12)',
            color: tag === 'midpoint' ? 'var(--kit-warn-soft)' : 'var(--kit-badge-run)',
          }}>{PAYOFF_LABEL[tag]}</span>
        )}
        <span>{chapter.payoff?.text || '—'}</span>
      </div>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 4, minWidth: 0 }}>
        <span className="pj-chip" style={{
          alignSelf: 'flex-start',
          ...(chapter.hook ? HOOK_TONE[chapter.hook.type] : HOOK_NONE),
          border: `1px solid ${flagged ? 'var(--frame-warn-55)' : 'transparent'}`,
        }}>{chapter.hook ? HOOK_LABEL[chapter.hook.type] : '—'}</span>
        {chapter.hook && chapter.hook.waiting !== '' && <span style={{ color: 'var(--kit-fg-icon)' }}>{chapter.hook.waiting}</span>}
      </div>
      {onEditRow
        ? (
          <button type="button" className="hov-ghost" aria-label={`Sửa dòng chương ${chapter.n}`} onClick={() => onEditRow(chapter.n)}
            style={{
              width: 28, height: 28, borderRadius: 7, border: 'none', background: 'transparent',
              color: 'var(--color-fg-4)', display: 'flex', alignItems: 'center', justifyContent: 'center',
            }}>
            <Icon name="edit-2" size={14} />
          </button>
        )
        : <span />}
    </div>
  )
}

export function RetentionMap({ retention, label, onEditRow }: {
  retention: Retention
  /** “Bản đồ giữ chân Tập 1” — tên tập khi truyện chỉ có một tập. */
  label: string
  /** Nút sửa mỗi dòng điền sẵn ô chat (chỉ người có quyền viết). */
  onEditRow?: (chapter: number) => void
}) {
  const flagged = new Set(retention.checks.find((check) => check.ref === 'Master §7' && !check.ok)?.chapters ?? [])
  return (
    <>
      <PositionStrip retention={retention} />
      <section className="card" aria-label={label} style={{ borderRadius: 16, overflow: 'hidden' }}>
        <div style={{ ...MROW, fontSize: 11.5, color: 'var(--color-fg-4)', paddingTop: 9, paddingBottom: 9, borderBottom: '1px solid var(--line-1)' }}>
          <span>Chương</span><span>Câu hỏi đang treo</span><span>Lời hứa · đếm ngược</span>
          <span>Trả thưởng · cú lật</span><span>Móc cuối chương</span><span />
        </div>
        {retention.chapters.map((chapter, index) => (
          <MapRow key={chapter.n} chapter={chapter} first={index === 0} last={index === retention.chapters.length - 1}
            flagged={flagged.has(chapter.n)} onEditRow={onEditRow} />
        ))}
        <div style={{ padding: '10px 14px', borderTop: '1px solid var(--line-1)', fontSize: 12, color: 'var(--color-fg-4)' }}>
          Kiến trúc sư lập bản đồ khi lên dàn ý và cập nhật sau mỗi chương. Sửa một dòng, hoặc nhắn ở ô chat bên dưới.
        </div>
      </section>
    </>
  )
}
