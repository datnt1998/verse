// Trang nhân vật (Outline.dc.html › person=detail): đầu trang + hàng “Xuất hiện” mở Trình đọc,
// banner khoá khi trợ lý đang viết, bảng xưng hô và lịch sử quan hệ.
import { useState } from 'react'
import { abortStory } from '../../../api/client'
import type { AddressTerms, Brain, Character } from '../../../api/types'
import { invalidate, keys } from '../../../data'
import { paths } from '../../../router'
import { Icon } from '@noverse/ui'
import { toast } from '@noverse/ui'
import { Chip } from '../bits'
import { CharAvatar, KindChip } from '../../../ui/CharAvatar'
import { isKindHidden } from '../../../lib/charKind'
import { latestWritten } from '../../brain/model'
import { AddressTable } from './AddressTable'
import { KindEditor } from './KindEditor'
import { appearCells, lockMessage } from './logic'
import type { Tag } from '../tabs/CastTab'

const IBTN = {
  height: 28, padding: '0 10px', borderRadius: 8, border: '1px solid var(--line-4)', background: 'transparent',
  color: 'var(--color-fg-2)', fontSize: 12, display: 'inline-flex', alignItems: 'center', gap: 6,
  whiteSpace: 'nowrap' as const, textDecoration: 'none',
}

export function PersonPage({ project, story, character, tag, names, address, brain, onBack }: {
  project: string
  story: string
  character: Character
  tag: Tag
  names: string[]
  address: AddressTerms
  brain: Brain | null
  onBack: () => void
}) {
  const [pausing, setPausing] = useState(false)
  const entity = brain?.entities.find((item) => item.name === character.name) ?? null
  const appears = entity?.appears ?? []
  const cells = brain ? appearCells(brain, appears, address.writing?.chapter ?? null) : []

  const pause = async () => {
    if (pausing) return
    setPausing(true)
    try {
      await abortStory(project, story)
      void invalidate(keys.story(project, story, 'snapshot'))
      void invalidate(keys.story(project, story, 'address'))
      toast.ok('Đã tạm dừng Người viết · sửa xong bấm Tiếp tục ở Bàn viết')
    } catch (error) {
      toast.err(error instanceof Error ? error.message : 'Không tạm dừng được Người viết.')
    } finally {
      setPausing(false)
    }
  }

  return (
    <>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 12 }}>
        <button type="button" className="hov-ghost" style={IBTN} onClick={onBack}>
          <Icon name="arrow-left-2" size={13} />Nhân vật
        </button>
        <span style={{ fontSize: 12.5, color: 'var(--color-fg-4)' }}>/ {character.name}</span>
        <span style={{ flex: 1 }} />
        <a className="hov-ghost" style={IBTN} href={`#${paths.story(project, story)}/brain`}>
          <Icon name="routing-2" size={13} />Xem trong Não truyện
        </a>
      </div>

      <section className="card" style={{ borderRadius: 16, padding: 16, display: 'flex', gap: 14, alignItems: 'flex-start' }}>
        <CharAvatar name={character.name} ini={character.ini} kind={character.kind} size={52} />
        <div style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', gap: 6 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
            <span style={{ fontSize: 17, fontWeight: 600 }}>{character.name}</span>
            <Chip tone={tag}>{tag.label}</Chip>
            <KindChip kind={character.kind ?? 'support'} hidden={isKindHidden(character.revealAt, brain ? latestWritten(brain.chapters) : 0)} />
          </div>
          <span style={{ fontSize: 13.5, color: 'var(--color-fg-2)', lineHeight: 1.55 }}>{character.description || '—'}</span>
          {brain && (
            <div style={{ display: 'flex', alignItems: 'center', gap: 6, flexWrap: 'wrap', marginTop: 2 }}>
              <span style={{ fontSize: 12, color: 'var(--color-fg-4)', marginRight: 2 }}>Xuất hiện</span>
              {cells.map((cell) => (
                <a key={cell.n} href={`#${paths.reader(project, story, cell.n)}`} aria-label={`Chương ${cell.n}`} title={cell.title}
                  className="num" style={{
                    width: 26, height: 24, borderRadius: 6, display: 'flex', alignItems: 'center', justifyContent: 'center',
                    fontSize: 12, background: cell.bg, color: cell.fg, border: `1px solid ${cell.border}`, textDecoration: 'none',
                  }}>{cell.n}</a>
              ))}
              <span style={{ fontSize: 12, color: 'var(--color-fg-4)', marginLeft: 4 }}>{appears.length}/{brain.chapters.length} chương</span>
            </div>
          )}
        </div>
      </section>

      {/* Gói S · Dự án › Nhân vật: “Loại vai” + “Lộ vai ở chương”, cả hai lưu qua PUT /characters. */}
      <KindEditor project={project} story={story} character={character} />

      {address.writing && (
        <div role="status" style={{
          marginTop: 10, display: 'flex', alignItems: 'center', gap: 10, padding: '10px 14px', borderRadius: 12,
          background: 'var(--kit-acc-6)', border: '1px solid var(--kit-acc-25)', fontSize: 13,
        }}>
          <Icon name="lock" size={16} style={{ color: 'var(--color-accent)' }} />
          <span style={{ flex: 1, minWidth: 0, color: 'var(--kit-badge-run)' }}>{lockMessage(address.writing)}</span>
          <button type="button" className="hov-ghost" disabled={pausing} onClick={() => void pause()}
            style={{ ...IBTN, border: '1px solid var(--kit-acc-40)', color: 'var(--color-fg)', opacity: pausing ? 0.45 : 1 }}>
            <Icon name="pause" size={13} />Tạm dừng để sửa
          </button>
        </div>
      )}

      <AddressTable project={project} story={story} person={character.name} names={names} address={address} />
    </>
  )
}
