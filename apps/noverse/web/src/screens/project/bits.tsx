// Mảnh UI dùng chung trong màn Dự án (Outline.dc.html): chip, ô nhập neo đáy, menu chuột phải của truyện.
import { useState, type CSSProperties, type FormEvent, type ReactNode, type RefObject } from 'react'
import { canManageProject, canWriteStory, deleteStory, revealFolder } from '../../api/client'
import type { StorySummary } from '../../api/types'
import { useSession } from '../../api/session'
import { invalidate, keys, useAction, useConnection } from '../../data'
import { navigate, paths } from '../../router'
import { storyLabel } from '../../shell/labels'
import { confirmDeleteStory } from '../../ui/confirmPresets'
import { Icon } from '@noverse/ui'
import { ActionMenu, type ActionMenuItem, type MenuPoint } from '@noverse/ui'
import { settingsLink } from './links'
import { useCanReveal } from './useProject'

export const CHIP_PURPLE = { background: 'rgba(201,166,245,0.12)', color: '#DCC8F8' } as const
export const CHIP_NEUTRAL = { background: 'rgba(255,255,255,0.05)', color: '#A0A0AB' } as const

/** `.chip` của artboard (22px, radius 6, 11.5px). `title` cho chip có nhãn dài bị cắt. */
export function Chip({ tone, style, title, children }: { tone: { background: string; color: string }; style?: CSSProperties; title?: string; children: ReactNode }) {
  return <span className="pj-chip" style={{ ...tone, ...style }} title={title}>{children}</span>
}

/** Ô nhập neo đáy cột giữa (card bo 14): “Ghi nhanh một ý tưởng…” và “Dặn hướng cốt truyện…”. */
export function Dock({ maxWidth, icon, iconColor, label, placeholder, sendLabel, value, onChange, onSubmit, buttonStyle, extra, inputRef }: {
  maxWidth: number
  icon: string
  iconColor: string
  label: string
  placeholder: string
  sendLabel: string
  value: string
  onChange: (value: string) => void
  onSubmit: () => void
  buttonStyle?: CSSProperties
  extra?: ReactNode
  inputRef?: RefObject<HTMLInputElement | null>
}) {
  const submit = (event: FormEvent) => { event.preventDefault(); onSubmit() }
  return (
    <div style={{
      position: 'absolute', left: 0, right: 0, bottom: 0, padding: '14px 28px 20px',
      background: 'linear-gradient(180deg, rgba(11,11,13,0), #0B0B0D 40%)', pointerEvents: 'none',
    }}>
      <form className="card kit-frame" onSubmit={submit} style={{
        pointerEvents: 'auto', maxWidth, margin: '0 auto', display: 'flex', alignItems: 'center', gap: 10,
        padding: '6px 6px 6px 14px', borderRadius: 14,
      }}>
        <Icon name={icon} size={17} style={{ color: iconColor }} />
        <input ref={inputRef} className="pj-bare" type="text" value={value} onChange={(event) => onChange(event.target.value)}
          placeholder={placeholder} aria-label={label} />
        {extra}
        <button className="hov-primary" type="submit" aria-label={sendLabel} style={{
          width: 36, height: 36, borderRadius: 10, border: 'none', background: '#EDEDEF', color: '#0B0B0D',
          display: 'flex', alignItems: 'center', justifyContent: 'center', flex: '0 0 auto', ...buttonStyle,
        }}>
          <Icon name="arrow-up" size={17} />
        </button>
      </form>
    </div>
  )
}

/**
 * Menu chuột phải của một truyện (mở · xuất bản · đổi tên · xoá). Artboard không vẽ nút “…”, nên menu chỉ
 * mở bằng chuột phải và không chiếm chỗ trong bố cục (`display: contents`).
 */
export function StoryContextMenu({ project, story, children }: { project: string; story: StorySummary; children: ReactNode }) {
  const connection = useConnection()
  const canReveal = useCanReveal()
  const { user } = useSession()
  const [point, setPoint] = useState<MenuPoint | null>(null)
  const reveal = useAction(() => revealFolder({ target: 'story', project, story: story.story }), { success: 'Đã mở thư mục truyện' })
  const blocked = story.running || story.locked
  const deleteReason = !user || !canManageProject(user, project) ? 'Chỉ Chủ dự án được xoá truyện' : story.running ? 'Truyện đang chạy — dừng trước khi xoá' : story.locked ? 'Truyện đang bị khoá' : !connection.writable ? 'Đang mất kết nối' : undefined
  const items: ActionMenuItem[] = [
    { id: 'open', label: 'Mở truyện', icon: <Icon name="book-1" size={16} />, onSelect: () => navigate(paths.story(project, story.story)) },
    ...(canReveal ? [{ id: 'reveal', label: 'Mở thư mục', icon: <Icon name="folder-open" size={16} />, disabled: reveal.busy, onSelect: () => { void reveal.run() } }] : []),
    { id: 'export', label: 'Bản xuất', icon: <Icon name="export-square" size={16} />, onSelect: () => navigate(`${paths.story(project, story.story)}?tab=export`) },
    { id: 'rename', label: 'Đổi tên', icon: <Icon name="edit-2" size={16} />, disabled: blocked || !user || !canWriteStory(user, `${project}/${story.story}`), onSelect: () => navigate(settingsLink('info', { project, story: story.story })) },
    {
      id: 'delete', label: 'Xoá truyện', icon: <Icon name="trash" size={16} />, danger: true,
      disabled: !!deleteReason, tooltip: deleteReason, description: deleteReason,
      onSelect: () => {
        void confirmDeleteStory({
          body: `Xoá vĩnh viễn ${story.completedChapters} chương, đại cương, bản đọc, audio và mọi thứ của truyện này. Việc này không hoàn tác được.`,
          successMessage: 'Đã xoá truyện',
          onConfirm: async () => {
            await deleteStory(project, story.story, story.story)
            await Promise.all([invalidate(keys.project(project)), invalidate(keys.library()), invalidate(keys.story(project, story.story))])
            const current = location.hash.split('?')[0]
            const deleted = `#${paths.story(project, story.story)}`
            if (current === deleted || current.startsWith(`${deleted}/`)) navigate(paths.project(project), true)
          },
        }, { name: storyLabel(story), phrase: story.story })
      },
    },
  ]
  return (
    <div style={{ display: 'contents' }} onContextMenu={(event) => { event.preventDefault(); setPoint({ x: event.clientX, y: event.clientY }) }}>
      {children}
      <span hidden><ActionMenu items={items} point={point} onClose={() => setPoint(null)} /></span>
    </div>
  )
}
