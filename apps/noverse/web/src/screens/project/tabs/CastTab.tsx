// Tab Nhân vật — lưới 2 cột thẻ (avatar chữ cái · tên · vai có màu · mô tả · “Xưng hô N dòng · xuất hiện X/N chương”)
// (Outline.dc.html › tab cast, person=grid). Bấm thẻ mở trang nhân vật (person=detail).
import { useState } from 'react'
import { getCharacters } from '../../../api/client'
import { keys, useResource } from '../../../data'
import { translateHost } from '../../../i18n/hostStrings'
import { useRoute } from '../../../router'
import { EmptyState, Skeleton } from '@noverse/ui'
import { StaleBanner } from '@noverse/ui'
import { Chip } from '../bits'
import { roleLabel, roleText } from '../../../lib/cast'
import { CharAvatar } from '../../../ui/CharAvatar'
import { PersonPage } from '../cast/PersonPage'
import { useAddressTerms, useBrain, useCharacters } from '../cast/data'
import { castMeta, termsFor } from '../cast/logic'
import { PanelRefreshBar } from '../../../ui/PanelRefreshBar'

export type Tag = { label: string; background: string; color: string }

// Màu theo đúng bảng ROLE của artboard; vai ngoài bảng dùng màu “Phụ chức năng”.
const PURPLE = { background: 'rgba(201,166,245,0.12)', color: '#D9CCF7' }
const RED = { background: 'rgba(255,154,162,0.1)', color: '#FF9AA2' }
// Đồng phạm dùng token `--role-accomplice-*` (cùng màu với MOutline) — không viết cứng mã màu.
const RED_SOFT = { background: 'var(--role-accomplice-bg)', color: 'var(--role-accomplice-fg)' }
const AMBER = { background: 'rgba(246,203,140,0.1)', color: '#F6CB8C' }
const BLUE = { background: 'rgba(185,208,247,0.1)', color: '#B9D0F7' }
const NEUTRAL = { background: 'rgba(255,255,255,0.06)', color: '#A0A0AB' }

const ROLE_COLOR: Record<string, Omit<Tag, 'label'>> = {
  'nữ chính': PURPLE,
  'phản diện chính': RED,
  'đồng phạm': RED_SOFT,
  'đồng minh hài': AMBER,
  'nam chính phụ trợ': BLUE,
  'phụ chức năng': NEUTRAL,
  main: PURPLE,
  protagonist: PURPLE,
  antagonist: RED,
  villain: RED,
  supporting: BLUE,
  mentor: AMBER,
  minor: NEUTRAL,
}

export function roleTag(role: string): Tag {
  const normalized = roleText(role)
  const firstPart = normalized.split(' · ')[0] ?? ''
  // Khóa bảng ROLE theo ý đầu của vai; nhãn tiếng Việt ở lib/cast (một nguồn cho mọi màn).
  return {
    label: roleLabel(role) || translateHost(normalized) || '—',
    ...(ROLE_COLOR[firstPart.toLowerCase()] ?? NEUTRAL),
  }
}

export function CastTab({ project, story }: { project: string; story: string }) {
  const resource = useCharacters(project, story)
  const address = useAddressTerms(project, story)
  const brain = useBrain(project, story)
  const parsed = useRoute()
  // `?person=<tên>` mở thẳng trang nhân vật (Não truyện › “Mở trang nhân vật”).
  const [selected, setSelected] = useState<string | null>(parsed.search.get('person'))
  const data = resource.data
  const character = selected ? data?.characters.find((item) => item.name === selected) ?? null : null

  if (resource.error && !data) {
    return (
      <EmptyState icon="warning-2" title="Không tải được nhân vật"
        hint="Máy chủ chưa đọc được danh sách nhân vật của truyện."
        action={
          <button type="button" className="hov-ghost" onClick={() => void resource.refresh()}
            style={{ height: 32, padding: '0 12px', borderRadius: 8, border: '1px solid var(--line-4)', background: 'transparent', color: 'var(--color-fg)', fontSize: 13 }}>
            Thử lại
          </button>
        } />
    )
  }
  if (!data) {
    return (
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(2, minmax(0, 1fr))', gap: 10 }}>
        {[0, 1, 2, 3].map((key) => <Skeleton key={key} height={86} radius={16} />)}
      </div>
    )
  }
  if (data.characters.length === 0) {
    return <EmptyState icon="profile-2user" title="Chưa có nhân vật" hint="Nhân vật xuất hiện sau khi truyện có tiền đề." />
  }

  // Trang nhân vật cần bảng xưng hô; chưa tải xong thì giữ lưới với khung xương.
  if (character && !address.data) {
    return (
      <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
        <Skeleton height={30} width={160} radius={8} />
        <Skeleton height={110} radius={16} />
        <Skeleton height={220} radius={16} />
      </div>
    )
  }

  const names = data.characters.map((item) => item.name)

  if (character && address.data) {
    return (
      <PersonPage project={project} story={story} character={character} tag={roleTag(character.role)} names={names}
        address={address.data} brain={brain.data ?? null} onBack={() => setSelected(null)} />
    )
  }

  return (
    <>
      <PanelRefreshBar resourceKeys={[keys.story(project, story, 'characters'), keys.story(project, story, 'brain'), keys.story(project, story, 'address')]} updatedAt={resource.updatedAt} />
      <StaleBanner error={(data && resource.error) || address.error || brain.error} onRetry={() => { void resource.refresh(); void address.refresh(); void brain.refresh() }} />
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(2, minmax(0, 1fr))', gap: 10 }}>
        {data.characters.map((item) => {
          const tag = roleTag(item.role)
          const lines = address.data ? termsFor(address.data.terms, item.name).length : 0
          const appears = brain.data?.entities.find((entity) => entity.name === item.name)?.appears.length ?? 0
          const meta = brain.data
            ? castMeta(lines, appears, brain.data.chapters.length)
            : lines > 0 ? `Xưng hô ${lines} dòng` : ''
          return (
            <button key={item.name} type="button" className="card hov-ghost" aria-label={`Mở trang nhân vật ${item.name}`}
              onClick={() => setSelected(item.name)}
              style={{
                borderRadius: 16, padding: 14, display: 'flex', gap: 12, alignItems: 'flex-start',
                textAlign: 'left', color: 'var(--color-fg)', font: 'inherit', cursor: 'pointer',
              }}>
              <CharAvatar name={item.name} ini={item.ini} kind={item.kind} size={36} />
              <div style={{ flex: '1 1 auto', minWidth: 0, display: 'flex', flexDirection: 'column', gap: 4 }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  <span style={{ fontSize: 14.5, fontWeight: 600 }}>{item.name}</span>
                  <Chip tone={tag}>{tag.label}</Chip>
                </div>
                <span style={{ fontSize: 13, color: 'var(--color-fg-2)', lineHeight: 1.55 }}>{item.description || '—'}</span>
                {meta && <span style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>{meta}</span>}
              </div>
            </button>
          )
        })}
      </div>
    </>
  )
}
