// Thanh chương của Não truyện (22-brief-2510.md › D10 “Thanh chương (sửa 25/10)”).
// Chấm luôn tròn (vị trí tuyệt đối, tâm chấm nằm trên đường nối); ba trạng thái chấm;
// chương đang xem phóng 1,4 lần + vòng sáng; đường nối tô tới chương đang viết.
import type { Brain } from '../../api/types'
import { AgentOrb } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { t } from '../../i18n/vi'
import { chapterNote, railFill, railTicks } from './model'
import './brain.css'

export function ChapterRail({ brain, chapter, onChapter, compact = false }: {
  brain: Brain
  chapter: number
  onChapter: (chapter: number) => void
  compact?: boolean
}) {
  const ticks = railTicks(brain.chapters, chapter)
  const note = chapterNote(brain, chapter)
  const title = brain.chapters.find((item) => item.n === chapter)?.title ?? ''
  const first = brain.chapters[0]?.n ?? 1
  const last = brain.chapters[brain.chapters.length - 1]?.n ?? 1
  return (
    <div className={`brain-tl${compact ? ' brain-tl-compact' : ''}`}>
      <div className="brain-tlh">
        <button className="brain-tlb" type="button" onClick={() => onChapter(Math.max(first, chapter - 1))}
          disabled={chapter <= first} aria-label={t.brain.prevChapter}>
          <Icon name="arrow-left-2" size={13} />
        </button>
        <button className="brain-tlb" type="button" onClick={() => onChapter(Math.min(last, chapter + 1))}
          disabled={chapter >= last} aria-label={t.brain.nextChapter}>
          <Icon name="arrow-right-2" size={13} />
        </button>
        <span style={{ fontSize: 13, fontWeight: 600, marginLeft: 4 }}>{t.brain.chapterWord} <span className="num">{chapter}</span></span>
        <span title={title} style={{ fontSize: 12.5, color: 'var(--brain-muted)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{title}</span>
        <span style={{ flex: 1 }} />
        <span style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 12, color: note.live ? 'var(--brain-chg)' : 'var(--brain-muted)', whiteSpace: 'nowrap' }}>
          {note.live && <AgentOrb agent="writer" size={9} />}
          {note.text}
        </span>
      </div>
      <div className="brain-tlt" role="group" aria-label={t.brain.railLabel}>
        <span className="brain-tll" aria-hidden="true" />
        <span className="brain-tlf" aria-hidden="true" style={{ width: railFill(brain.chapters) }} />
        {ticks.map((tick) => (
          <button key={tick.n} className={`tk ${tick.cls}`} type="button" aria-pressed={tick.n === chapter}
            aria-label={tick.aria} title={tick.aria} onClick={() => onChapter(tick.n)} style={{ left: tick.x }}>
            <i />
            <span className="num" style={{ color: tick.fg }}>{tick.n}</span>
          </button>
        ))}
      </div>
    </div>
  )
}
