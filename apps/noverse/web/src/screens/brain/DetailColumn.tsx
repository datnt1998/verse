// Cột phải Não truyện (22-brief-2510.md › D10 “Cột phải: chi tiết thực thể đang chọn”).
// Đầu mục · quan hệ tại chương N · xưng hô tại chương N (có chiều, dòng cam khi đã đổi) ·
// diễn biến (mốc sau N mờ) · hàng Xuất hiện · banner khoá D8 · “Mở trang nhân vật”.
// Variant `drawer` = cùng bảng chi tiết ở cột 320 px, chỉ xem, không có thao tác sửa.
import type { CSSProperties, ReactNode } from 'react'
import type { AddressTerms, Brain } from '../../api/types'
import { navigate, paths } from '../../router'
import { Icon } from '@noverse/ui'
import { t } from '../../i18n/vi'
import { entityView, addressRowsAt, appearRow, lockSentence, relsAt, timelineAt } from './model'
import './brain.css'
import { CharAvatar, KindChip } from '../../ui/CharAvatar'

const ROW: CSSProperties = {
  display: 'flex', alignItems: 'flex-start', gap: 10, padding: '8px 0',
  borderTop: '1px solid var(--kit-wash-5)', fontSize: 13,
}

const EDIT: CSSProperties = {
  width: 26, height: 26, borderRadius: 7, border: 'none', background: 'transparent',
  color: 'var(--color-fg-4)', display: 'flex', alignItems: 'center', justifyContent: 'center',
}

export function DetailColumn({ brain, chapter, entityId, address, writing, project, story, variant = 'full', footer }: {
  brain: Brain
  chapter: number
  entityId: string
  address: AddressTerms | null
  writing: AddressTerms['writing']
  project: string
  story: string
  /** Cột 320 px của ngăn kéo; cùng dữ kiện, 5 diễn biến, avatar 40 px và không sửa. */
  variant?: 'full' | 'drawer'
  footer?: ReactNode
}) {
  const entity = brain.entities.find((item) => item.id === entityId) ?? null
  if (!entity) {
    return (
      <aside aria-label={t.brain.detailLabel} className="scroll"
        style={{ borderLeft: '1px solid var(--line-1)', padding: 16, display: 'flex', flexDirection: 'column', gap: 12 }}>
        <span style={{ fontSize: 12.5, color: 'var(--color-fg-4)' }}>{t.brain.pickEntity}</span>
      </aside>
    )
  }
  const narrator = brain.entities.find((item) => item.type === 'char')?.id ?? null
  const view = entityView(entity, chapter, narrator)
  const rels = relsAt(brain, entity.id, chapter)
  const terms = address ? addressRowsAt(address, entity, chapter) : []
  const timeline = timelineAt(entity, brain, address ?? { terms: [] }, chapter)
  const appears = appearRow(brain.chapters, entity.appears, chapter)
  const locked = !!writing
  const lockOpacity = locked ? 0.4 : 1
  const notYet = entity.fromChapter > chapter
  const personLink = `#${paths.story(project, story)}?tab=cast&person=${encodeURIComponent(entity.name)}`
  const openPerson = () => { if (!locked && view.isChar) navigate(`${paths.story(project, story)}?tab=cast&person=${encodeURIComponent(entity.name)}`, true) }

  return (
    <aside aria-label="Chi tiết" className="scroll"
      style={{ flex: variant === 'drawer' ? 1 : undefined, minWidth: 0, minHeight: 0, overflowX: 'hidden', overflowWrap: 'anywhere',
        borderLeft: variant === 'drawer' ? undefined : '1px solid var(--line-1)', padding: 16,
        display: 'flex', flexDirection: 'column', gap: 12 }}>
      <div style={{ display: 'flex', gap: 12, alignItems: 'flex-start' }}>
        {view.isChar
          ? <CharAvatar name={entity.name} ini={entity.ini} kind={view.kind} hidden={view.hidden} size={variant === 'drawer' ? 40 : 44} />
          : <span aria-hidden="true" style={{
            width: variant === 'drawer' ? 40 : 44, height: variant === 'drawer' ? 40 : 44, borderRadius: 11, background: 'var(--brain-node-bg-other)',
            border: `1.5px solid color-mix(in srgb, ${view.ring} 53%, transparent)`, color: view.ring,
            display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: 14, fontWeight: 600, flex: 'none',
          }}><Icon name={view.type.icon} size={18} /></span>}
        <div style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', gap: 4 }}>
          <span style={{ fontSize: 16, fontWeight: 600, lineHeight: 1.3 }}>{entity.name}</span>
          <span style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
            {view.isChar && <KindChip kind={view.kind} hidden={view.hidden} />}
            {(variant !== 'drawer' || !view.isChar) && <span style={{
              height: 20, padding: '0 7px', display: 'flex', alignItems: 'center', borderRadius: 6,
              background: 'var(--kit-wash-5)', color: view.type.color, fontSize: 11.5,
            }}>{view.type.label}</span>}
            <span style={{
              height: 20, padding: '0 7px', display: 'flex', alignItems: 'center', borderRadius: 6,
              background: variant === 'drawer' ? 'transparent' : 'var(--kit-wash-5)', border: variant === 'drawer' ? '1px solid var(--line-3)' : undefined, color: 'var(--color-fg-3)', fontSize: 11.5,
            }}>{t.brain.since(entity.fromChapter)}</span>
          </span>
        </div>
      </div>

      <span style={{ fontSize: 13, color: 'var(--color-fg-2)', lineHeight: 1.55 }}>{entity.desc}</span>

      {notYet && (
        <div style={{
          padding: '10px 12px', borderRadius: 12, background: 'var(--brain-dash-bg)',
          border: '1px dashed var(--kit-line-12)', fontSize: 12.5, color: 'var(--color-fg-3)',
        }}>{t.brain.notYet(chapter, entity.fromChapter)}</div>
      )}

      <section className="card" style={{ borderRadius: 14, padding: '10px 12px' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, paddingBottom: 4 }}>
          <span style={{ fontSize: 13, fontWeight: 600 }}>{t.brain.relsTitle(chapter)}</span>
          <span style={{ flex: 1 }} />
          {variant !== 'drawer' && <button className="hov-ghost" type="button" disabled={locked || !view.isChar} aria-disabled={locked || !view.isChar}
            onClick={openPerson} style={{
              height: 26, padding: '0 8px', borderRadius: 7, border: '1px solid var(--line-4)', background: 'transparent',
              color: 'var(--color-fg-2)', fontSize: 12, display: 'flex', alignItems: 'center', gap: 5,
              opacity: locked || !view.isChar ? lockOpacity : 1,
            }}>
            <Icon name="add" size={12} />{t.brain.add}
          </button>}
        </div>
        {rels.length === 0 && <span style={{ fontSize: 12.5, color: 'var(--color-fg-4)' }}>{t.brain.noRels}</span>}
        <div>
          {rels.map((row, index) => (
            <div key={row.id} style={index === 0 ? { ...ROW, borderTop: 'none' } : ROW}>
              <span aria-hidden="true" style={{
                width: 14, height: 0, marginTop: 10, flex: 'none',
                borderTop: `2px ${row.dashed ? 'dashed' : 'solid'} ${row.color}`,
              }} />
              <span style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', gap: 1 }}>
                <span><span style={{ color: row.color }}>{row.label}</span> · {row.otherName}</span>
                <span style={{ fontSize: 11.5, color: 'var(--color-fg-4)' }}>{variant === 'drawer' && row.fresh ? t.brain.newAtChapter(chapter) : row.since}</span>
              </span>
              {variant !== 'drawer' && <button className="hov-ghost" type="button" disabled={locked || !view.isChar} aria-label={t.brain.editRel(row.otherName)}
                onClick={openPerson} style={{ ...EDIT, opacity: locked || !view.isChar ? lockOpacity : 1 }}>
                <Icon name="edit-2" size={13} />
              </button>}
            </div>
          ))}
        </div>
      </section>

      {view.isChar && (
        <section className="card" style={{ borderRadius: 14, padding: '10px 12px' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, paddingBottom: 4 }}>
            <span style={{ fontSize: 13, fontWeight: 600 }}>{t.brain.xhTitle(chapter)}</span>
            {variant !== 'drawer' && <span style={{ fontSize: 11.5, color: 'var(--color-fg-4)' }}>{t.brain.directed}</span>}
          </div>
          {terms.length === 0 && <span style={{ fontSize: 12.5, color: 'var(--color-fg-4)' }}>{t.brain.noXh}</span>}
          <div>
            {terms.map((row, index) => (
              <div key={row.id} style={{
                ...ROW, flexDirection: 'column', gap: 2, ...(index === 0 ? { borderTop: 'none' } : {}),
              }}>
                <span style={{
                  display: 'flex', alignItems: 'center', gap: 6, fontSize: 12.5, color: 'var(--color-fg-3)',
                  minWidth: 0, width: '100%',
                }}>
                  <span style={{
                    whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis', maxWidth: 150, flex: 'none',
                  }}>{row.from}</span>
                  <Icon name="arrow-right-2" size={11} />
                  <span style={{ whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis', minWidth: 0 }}>{row.to}</span>
                </span>
                <span>{t.brain.callsVerb} <b style={{ fontWeight: 600 }}>“{row.call}”</b> · {t.brain.selfVerb} <b style={{ fontWeight: 600 }}>“{row.self}”</b></span>
                {row.was && <span style={{ fontSize: 11.5, color: 'var(--kit-warn-text)' }}>{row.was}</span>}
              </div>
            ))}
          </div>
        </section>
      )}

      <section className="card" style={{ borderRadius: 14, padding: '10px 12px' }}>
        <span style={{ fontSize: 13, fontWeight: 600 }}>{t.brain.timeline}</span>
        <ol style={{ listStyle: 'none', margin: '6px 0 0', padding: 0, display: 'flex', flexDirection: 'column', gap: 6 }}>
          {(variant === 'drawer' ? timeline.slice(0, 5) : timeline).map((item) => (
            <li key={`${item.chapter}-${item.text}`} style={{
              display: 'grid', gridTemplateColumns: `${variant === 'drawer' ? 44 : 54}px minmax(0, 1fr)`, gap: 8, fontSize: 12.5,
              opacity: item.future ? (variant === 'drawer' ? 0.42 : 0.5) : 1,
            }}>
              <span className="num" style={{ color: 'var(--brain-muted)' }}>{t.brain.timelineCh(item.chapter)}</span>
              <span style={{ color: 'var(--color-fg-2)' }}>
                {item.text}
                {item.future && <span style={{ color: 'var(--color-fg-4)' }}>{t.brain.timelineFuture(chapter)}</span>}
              </span>
            </li>
          ))}
        </ol>
      </section>

      {variant !== 'drawer' && <div style={{ display: 'flex', alignItems: 'center', gap: 6, flexWrap: 'wrap' }}>
        <span style={{ fontSize: 12, color: 'var(--color-fg-4)', marginRight: 2 }}>{t.brain.appears}</span>
        {appears.map((cell) => (
          <span key={cell.n} title={cell.title} className="num" style={{
            width: 24, height: 22, borderRadius: 6, display: 'flex', alignItems: 'center', justifyContent: 'center',
            fontSize: 11.5, background: cell.bg, color: cell.fg, border: `1px solid ${cell.border}`,
          }}>{cell.n}</span>
        ))}
      </div>}

      {variant !== 'drawer' && writing && (
        <div role="status" style={{
          display: 'flex', gap: 8, padding: '10px 12px', borderRadius: 12, background: 'var(--kit-acc-6)',
          border: '1px solid var(--kit-acc-22)', fontSize: 12.5, color: 'var(--brain-chg)', lineHeight: 1.5,
        }}>
          <Icon name="lock" size={14} style={{ marginTop: 2 }} />
          <span>
            {lockSentence(writing)}
            {' '}
            <a href={`#${paths.workspace(project, story)}`} style={{
              color: 'var(--color-fg)', textDecoration: 'underline', textUnderlineOffset: 3,
            }}>{t.brain.pauseAt}</a>{t.brain.pauseTail}
          </span>
        </div>
      )}

      {variant !== 'drawer' && view.isChar && (
        <a className="hov-ghost" href={personLink} style={{
          height: 34, borderRadius: 9, border: '1px solid var(--line-4)', display: 'flex', alignItems: 'center',
          justifyContent: 'center', gap: 8, fontSize: 13, color: 'var(--color-fg)',
        }}>
          <Icon name="user" size={15} />{t.brain.openPerson}
        </a>
      )}
      {footer}
    </aside>
  )
}
