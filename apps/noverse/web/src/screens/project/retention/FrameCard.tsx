// Thẻ “Khung truyện” (Outline.dc.html › tab outline, D5): người nghe Master §1.2, điểm khác §17,
// hạt giống phần hai §19. Sửa tại chỗ, lưu qua PUT …/frame; chỉ người có quyền viết mới thấy nút Sửa.
import { useState, type CSSProperties } from 'react'
import { canWriteStory, getConfigPresets, getFrame, putFrame } from '../../../api/client'
import { useSession } from '../../../api/session'
import type { Frame, FrameInput } from '../../../api/types'
import { keys, setData, useResource } from '../../../data'
import { storyFrameCopy } from '../../../i18n/vi'
import { Icon } from '@noverse/ui'
import { Skeleton } from '@noverse/ui'
import { toast } from '@noverse/ui'
import { findPack, packLabel } from '../../settings/lib/packs'
import { useEffectiveConfig } from '../applied'
import { Chip } from '../bits'
import { FRAME_ROWS, audienceLabels, audienceWants, frameSourceChip, storyDifference, type FrameRowKey } from './logic'

/** `.ibtn` của artboard — nút nhỏ trong thẻ. */
const IBTN: CSSProperties = {
  height: 28, padding: '0 10px', borderRadius: 8, border: '1px solid var(--line-4)',
  background: 'transparent', color: 'var(--color-fg-2)', fontSize: 12,
  display: 'inline-flex', alignItems: 'center', gap: 6, whiteSpace: 'nowrap',
}
/** `.fin` — ô chữ một dòng của dòng đang sửa. */
const FIN: CSSProperties = {
  height: 30, padding: '0 9px', borderRadius: 8, border: '1px solid var(--kit-acc-45)',
  background: 'var(--color-surface-0)', color: 'var(--color-fg)', font: 'inherit', fontSize: 13,
  width: '100%', minWidth: 0,
}
const SAVE: CSSProperties = {
  ...IBTN, background: 'var(--color-fg)', color: 'var(--color-bg)', border: '1px solid var(--color-fg)', fontWeight: 600,
}

export function FrameCard({ project, story }: { project: string; story: string }) {
  const key = keys.story(project, story, 'frame')
  const resource = useResource(key, ({ signal }) => getFrame(project, story, signal))
  const { user } = useSession()
  const config = useEffectiveConfig(project, story)
  const presets = useResource(['config-presets'], ({ signal }) => getConfigPresets(signal), { staleTime: 60000 })
  const canWrite = !!user && canWriteStory(user, `${project}/${story}`)
  const [editing, setEditing] = useState<FrameRowKey | null>(null)
  const [draft, setDraft] = useState('')
  const [audience, setAudience] = useState<number[]>([])
  const [saving, setSaving] = useState(false)

  const frame = resource.data
  if (!frame) return <Skeleton height={140} radius={16} />

  const pack = findPack(presets.data?.packs, config?.effective.style)
  const chip = frameSourceChip(frame.source, pack ? packLabel(pack) : null)
  const labels = audienceLabels(frame)
  const wants = audienceWants(frame)
  const draftWants = audienceWants({ audience, audiences: frame.audiences ?? [] })

  const start = (row: FrameRowKey) => {
    setEditing(row)
    setDraft(row === 'difference' ? storyDifference(frame.difference) : row === 'sequelSeed' ? frame.sequelSeed : '')
    setAudience(frame.audience ?? [])
  }

  /** Chip nhóm là bật/tắt (Câu 18: `audience` là mảng chỉ số nhóm). */
  const toggleAudience = (n: number) => {
    setAudience((selected) => selected.includes(n)
      ? selected.filter((value) => value !== n)
      : [...selected, n])
  }

  const save = async () => {
    if (!editing || saving) return
    const input: FrameInput = editing === 'audience' ? { audience } : editing === 'difference' ? { difference: draft } : { sequelSeed: draft }
    setSaving(true)
    try {
      const next = await putFrame(project, story, input)
      setData(key, next)
      const label = FRAME_ROWS.find((row) => row.key === editing)?.label ?? ''
      toast.ok(`Đã lưu ${label.toLocaleLowerCase('vi')} · Kiến trúc sư dùng từ chương sau`)
      setEditing(null)
    } catch (error) {
      toast.err(error instanceof Error ? error.message : 'Không lưu được khung truyện.')
    } finally {
      setSaving(false)
    }
  }

  return (
    <section className="card" aria-labelledby="fr-t" style={{ borderRadius: 16, padding: '12px 16px 6px', marginBottom: 12 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 10, paddingBottom: 4 }}>
        <span id="fr-t" style={{ fontSize: 14, fontWeight: 600 }}>Khung truyện</span>
        <span title={chip} style={{ minWidth: 0, display: 'inline-flex' }}>
          <Chip tone={{ background: 'var(--tint-ok)', color: 'var(--color-ok)' }}
            style={{ display: 'inline-block', maxWidth: 360, overflow: 'hidden', textOverflow: 'ellipsis' }}>{chip}</Chip>
        </span>
        <span style={{ flex: 1 }} />
        <span style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>Kiến trúc sư dựa vào đây khi lập dàn ý</span>
      </div>

      {FRAME_ROWS.map((row, index) => {
        const editingThis = editing === row.key
        const text = row.key === 'difference' ? storyDifference(frame.difference) : row.key === 'sequelSeed' ? frame.sequelSeed : ''
        const value = row.key === 'audience' ? labels : text
        const sub = row.key === 'audience' && wants !== '' ? `Muốn ${wants}` : ''
        return (
          <div key={row.key} style={{
            display: 'grid', gridTemplateColumns: '150px minmax(0,1fr) auto', gap: 12, alignItems: 'start',
            padding: '11px 0', borderTop: index === 0 ? 'none' : '1px solid var(--line-1)',
          }}>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 3 }}>
              <span style={{ fontSize: 13, fontWeight: 500 }}>{row.label}</span>
              <span style={{ fontSize: 11.5, color: 'var(--color-fg-4)' }}>{row.ref}</span>
            </div>
            <div style={{ minWidth: 0, display: 'flex', flexDirection: 'column', gap: 6 }}>
              {!editingThis && (
                <>
                  <span style={{ fontSize: 13.5, color: value ? 'var(--color-fg)' : 'var(--color-fg-4)', lineHeight: 1.5 }}>{value || (row.key === 'difference' ? storyFrameCopy.noPrevious : '—')}</span>
                  {sub !== '' && <span style={{ fontSize: 12.5, color: 'var(--kit-fg-icon)' }}>{sub}</span>}
                </>
              )}
              {editingThis && row.key === 'audience' && (
                <>
                  <div role="group" aria-label="Nhóm người nghe" style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
                    {(frame.audiences ?? []).map((group, index) => {
                      const on = audience.includes(index)
                      return (
                        <button key={group.n} type="button" aria-pressed={on} onClick={() => toggleAudience(index)}
                          style={{
                            ...IBTN,
                            borderColor: on ? 'var(--kit-acc-55)' : 'var(--line-4)',
                            background: on ? 'var(--tint-accent)' : 'transparent',
                            color: on ? 'var(--color-fg)' : 'var(--color-fg-3)',
                          }}>{group.label}</button>
                      )
                    })}
                  </div>
                  <span style={{ fontSize: 12, color: 'var(--kit-fg-icon)' }}>
                    {draftWants !== ''
                      && `Họ muốn ${draftWants}. Đổi nhóm sẽ đổi nhịp, mức mỏ hỗn và kiểu kết.`}
                  </span>
                </>
              )}
              {editingThis && row.key !== 'audience' && (
                <input type="text" value={draft} aria-label={row.label} style={FIN} autoFocus
                  onChange={(event) => setDraft(event.target.value)} />
              )}
            </div>
            {!editingThis && canWrite && (row.key !== 'audience' || (frame.audiences ?? []).length > 0) && (
              <button type="button" style={IBTN} onClick={() => start(row.key)} aria-label={row.key === 'audience' && labels === '' ? storyFrameCopy.chooseAudience : `Sửa ${row.label}`}>
                <Icon name="edit-2" size={13} />{row.key === 'audience' && labels === '' ? storyFrameCopy.choose : 'Sửa'}
              </button>
            )}
            {editingThis && (
              <button type="button" style={SAVE} onClick={() => void save()} disabled={saving}>{saving ? 'Đang lưu…' : 'Xong'}</button>
            )}
          </div>
        )
      })}
    </section>
  )
}
