// ReaderToolbar — tiêu đề chương, nguồn văn bản (Bản nộp/Bản nháp), diff, cỡ chữ 14–24.
import type { RefObject } from 'react'
import { Icon } from '@noverse/ui'
import { copy, formatNumber } from './copy'
import { CompactSegmented } from './CompactSegmented'

export function ReaderToolbar({ chapter, title, words, source, onSource, diff, onDiff, canDiff, size, onSize, headingRef }: {
  chapter: number
  title: string
  words: number
  source: 'committed' | 'draft'
  onSource: (source: 'committed' | 'draft') => void
  diff: boolean
  onDiff: () => void
  canDiff: boolean
  size: number
  onSize: (delta: number) => void
  headingRef?: RefObject<HTMLHeadingElement | null>
}) {
  return (
    <div style={{
      flex: '0 0 auto', display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: '10px 14px',
      padding: '12px 20px', borderBottom: '1px solid var(--line-1)',
    }}>
      <div style={{ flex: '1 1 300px', minWidth: 0, display: 'flex', flexDirection: 'column' }}>
        <span className="num" style={{ fontSize: 12, color: 'var(--color-fg-3)' }}>
          {copy.chapterMeta(chapter, words > 0 ? copy.words(formatNumber(words)) : '—')}
        </span>
        <h1 ref={headingRef} tabIndex={-1} style={{
          margin: 0, fontSize: 19, fontWeight: 600, letterSpacing: '-0.015em',
          whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
        }}>{title}</h1>
      </div>

      <CompactSegmented
        label={copy.sourceLabel}
        value={source}
        onChange={(id) => onSource(id === 'draft' ? 'draft' : 'committed')}
        options={[{ id: 'committed', label: copy.final }, { id: 'draft', label: copy.draft }]}
      />

      <button type="button" className="hov-ghost" onClick={onDiff} aria-pressed={canDiff && diff} disabled={!canDiff}
        title={canDiff ? undefined : copy.diffUnavailable}
        style={{
          height: 34, padding: '0 10px', display: 'flex', alignItems: 'center', gap: 6,
          borderRadius: 8, border: `1px solid ${diff && canDiff ? 'var(--line-5)' : 'var(--line-3)'}`,
          background: diff && canDiff ? 'var(--color-surface-5)' : 'transparent',
          color: 'var(--color-fg-2)', fontSize: 12, opacity: canDiff ? 1 : 0.55,
        }}>
        <Icon name="code" size={15} />{copy.diff}
      </button>

      <div style={{
        display: 'flex', alignItems: 'center', gap: 2, padding: 3, borderRadius: 9,
        background: 'var(--color-surface-1)', border: '1px solid var(--line-1)',
      }}>
        <button type="button" className="hov-ghost" onClick={() => onSize(-1)} aria-label={copy.sizeDown} disabled={size <= 14}
          style={{
            width: 28, height: 28, borderRadius: 6, border: 'none', background: 'transparent',
            color: 'var(--color-fg)', display: 'flex', alignItems: 'center', justifyContent: 'center',
            opacity: size <= 14 ? 0.4 : 1,
          }}>
          <Icon name="minus" size={14} />
        </button>
        <span className="num" style={{ width: 36, textAlign: 'center', fontSize: 12, color: 'var(--color-fg-3)' }}>{size}px</span>
        <button type="button" className="hov-ghost" onClick={() => onSize(1)} aria-label={copy.sizeUp} disabled={size >= 24}
          style={{
            width: 28, height: 28, borderRadius: 6, border: 'none', background: 'transparent',
            color: 'var(--color-fg)', display: 'flex', alignItems: 'center', justifyContent: 'center',
            opacity: size >= 24 ? 0.4 : 1,
          }}>
          <Icon name="add" size={14} />
        </button>
      </div>
    </div>
  )
}
