// “Loại vai” + “Lộ vai ở chương” của trang nhân vật (Gói S · 06-data-contract.md › *Bổ sung §S*,
// 04-components.md › *Avatar nhân vật*). Tách khỏi PersonPage để test riêng phần lưu.
import { useState } from 'react'
import { putCharacters } from '../../../api/client'
import type { Character, CharacterKind } from '../../../api/types'
import { invalidate, keys } from '../../../data'
import { KIND_META, KIND_ORDER } from '../../../lib/charKind'
import { Icon } from '@noverse/ui'
import { KitSelect } from '@noverse/ui'
import { toast } from '@noverse/ui'

/**
 * Đổi một trong hai ô thì nút Lưu bật lên; lưu gửi đúng hai trường, `revealAt` rỗng gửi `null`
 * (không còn bí mật). Ô chương chỉ nhận số nguyên ≥ 1; máy chủ vẫn kiểm lại và trả 400.
 */
export function KindEditor({ project, story, character }: { project: string; story: string; character: Character }) {
  const savedKind = character.kind ?? 'support'
  const savedReveal = character.revealAt ?? null
  const [kind, setKind] = useState<CharacterKind>(savedKind)
  const [reveal, setReveal] = useState(savedReveal === null ? '' : String(savedReveal))
  const [saving, setSaving] = useState(false)
  const chapter = reveal.trim() === '' ? null : Number(reveal)
  const invalid = chapter !== null && (!Number.isInteger(chapter) || chapter < 1)
  const dirty = kind !== savedKind || chapter !== savedReveal
  const save = async () => {
    if (!dirty || invalid || saving) return
    setSaving(true)
    try {
      await putCharacters(project, story, [{ name: character.name, kind, revealAt: chapter }])
      // characters.json đổi → danh sách Nhân vật, Não truyện (avatar) và snapshot phải đọc lại.
      void invalidate(keys.story(project, story, 'characters'))
      void invalidate(keys.story(project, story, 'brain'))
      void invalidate(keys.story(project, story, 'snapshot'))
      toast.ok(`Đã lưu loại vai cho ${character.name}`)
    } catch (error) {
      toast.err(error instanceof Error ? error.message : 'Không lưu được nhân vật.')
    } finally {
      setSaving(false)
    }
  }
  const options = KIND_ORDER.map((key) => ({ value: key, label: KIND_META[key].label }))
  return (
    <div className="card" style={{
      marginTop: 10, borderRadius: 16, padding: 14, display: 'flex', gap: 14, alignItems: 'flex-end', flexWrap: 'wrap',
    }}>
      <label style={FIELD}>
        <span style={FIELD_LABEL}>Loại vai</span>
        <KitSelect label="Loại vai" options={options} value={kind} onChange={(value) => setKind(value as CharacterKind)}
          compact style={{ width: 150 }} />
      </label>
      <label style={FIELD}>
        <span style={FIELD_LABEL}>Lộ vai ở chương</span>
        <input className="num" type="number" min={1} step={1} inputMode="numeric" value={reveal}
          placeholder="Không có" aria-label="Lộ vai ở chương" aria-invalid={invalid || undefined}
          onChange={(event) => setReveal(event.target.value)}
          style={{
            width: 128, height: 32, padding: '0 10px', borderRadius: 8, fontSize: 13,
            border: `1px solid ${invalid ? 'var(--frame-danger-45)' : 'var(--line-4)'}`,
            background: 'transparent', color: 'var(--color-fg)',
          }} />
      </label>
      <span style={{ fontSize: 12, color: 'var(--color-fg-4)', flex: 1, minWidth: 180 }}>
        {invalid
          ? 'Số chương phải từ 1 trở lên. Để trống nếu không giấu thân phận.'
          : 'Để trống nếu nhân vật không có bí mật thân phận; có số thì avatar viền nét đứt tới chương đó.'}
      </span>
      <button type="button" className="hov-ghost" disabled={!dirty || invalid || saving} onClick={() => void save()}
        style={{ ...IBTN, border: '1px solid var(--line-4)', opacity: !dirty || invalid || saving ? 0.45 : 1 }}>
        <Icon name="tick-circle" size={13} />{saving ? 'Đang lưu…' : 'Lưu'}
      </button>
    </div>
  )
}

const IBTN = {
  height: 28, padding: '0 10px', borderRadius: 8, border: '1px solid var(--line-4)', background: 'transparent',
  color: 'var(--color-fg-2)', fontSize: 12, display: 'inline-flex', alignItems: 'center', gap: 6,
  whiteSpace: 'nowrap' as const, textDecoration: 'none',
}
const FIELD = { display: 'flex', flexDirection: 'column' as const, gap: 5 }
const FIELD_LABEL = { fontSize: 12, color: 'var(--color-fg-3)' }
