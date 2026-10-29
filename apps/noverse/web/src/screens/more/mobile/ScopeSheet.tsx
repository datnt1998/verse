// Gói R › điện thoại: dòng phạm vi + bottom sheet chọn dự án/truyện (MMore). Nội dung danh sách dùng
// chung `ScopeList` với popover máy tính, nên chip “Đang mở”, ✓, tìm và hộp cam “chưa lưu” giống hệt.
import { useEffect, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import type { ConfigScope, User } from '../../../api/types'
import { canCreateStory, canReadProject, canWriteStory } from '../../../api/client'
import { useLibrary } from '../../../api/library'
import { useSession } from '../../../api/session'
import { BottomSheet } from '../../../mobile/primitives'
import { Icon } from '@noverse/ui'
import { projectLabel, storyLabel } from '../../../shell/labels'
import { ScopeList, UnsavedBox } from '../../settings/scope/lists'
import { storyBoxLabel, type OpenTarget, type ScopeTarget } from '../../settings/scope/model'
import { sx } from '../../settings/scope/scope.stylex'

/** Quyền sửa đúng chỗ đang xem (không phải truyện đang mở). */
export const canEditTarget = (user: User | null, target: ScopeTarget): boolean =>
  !!user && (target.scope === 'global'
    ? user.role === 'admin'
    : target.scope === 'project'
      ? !!target.project && canCreateStory(user, target.project)
      : !!target.project && !!target.story && canWriteStory(user, `${target.project}/${target.story}`))

/** Dòng phạm vi của MMore — chạm để mở bottom sheet (thay thanh 3 nút cũ). */
export function MobileScopeRow({ target, open, projectName, storyTitle, onOpen }: {
  target: ScopeTarget; open: OpenTarget; projectName: string; storyTitle: string; onOpen: () => void
}) {
  const label = target.scope === 'global' ? 'Mọi truyện' : target.scope === 'project' ? 'Dự án' : storyBoxLabel(target, open)
  const sub = target.scope === 'global' ? 'Toàn bộ thư viện'
    : target.scope === 'project' ? projectName || projectLabel(undefined, target.project)
      : storyTitle || '—'
  return (
    <button className="mm-row" type="button" aria-haspopup="dialog" onClick={onOpen}
      style={{ margin: '4px 16px 0', width: 'auto', borderRadius: 13, background: 'var(--color-card)', border: '1px solid var(--line-1)' }}>
      <Icon name={target.scope === 'story' ? 'book-1' : target.scope === 'project' ? 'folder-open' : 'global'} size={20} style={{ color: 'var(--color-fg-3)' }} />
      <span className="mm-copy"><span>Áp dụng cho · {label}</span><span className="mm-sub">{sub}</span></span>
      <Icon name="arrow-down-2" size={16} className="mm-chevron" />
    </button>
  )
}

/** Bottom sheet “Áp dụng cho”: danh sách dự án, truyện của dự án đang xem, và “Mọi truyện”. */
export function MobileScopeSheet({ open, onClose, target, openTarget, dirtyCount, onTarget, onSave, onDiscard }: {
  open: boolean; onClose: () => void
  target: ScopeTarget; openTarget: OpenTarget; dirtyCount: number
  onTarget: (target: ScopeTarget) => void
  onSave: () => Promise<unknown> | unknown
  onDiscard: () => void
}) {
  const library = useLibrary()
  const { user } = useSession()
  const [pending, setPending] = useState<ScopeTarget | null>(null)
  const projects = (library.library?.projects ?? []).filter((item) => !!user && canReadProject(user, item.id))
  const stories = target.project ? library.projectDetail(target.project)?.stories ?? [] : []
  const projectName = projectLabel(projects.find((item) => item.id === target.project), target.project)
  const storyTitle = storyLabel(stories.find((item) => item.story === target.story))

  useEffect(() => {
    if (!open) { setPending(null); return }
    projects.forEach((item) => library.ensureProject(item.id))
    // Chỉ tải trước khi mở sheet, không tải lại mỗi lần danh sách đổi.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open])

  const close = () => { setPending(null); onClose() }
  const pick = (next: ScopeTarget) => {
    if (next.scope === target.scope && next.project === target.project && next.story === target.story) { close(); return }
    if (dirtyCount > 0) { setPending(next); return }
    onTarget(next)
    close()
  }
  const resolveSave = async () => { if (await onSave() !== true) return; if (pending) onTarget(pending); close() }
  const resolveDiscard = () => { onDiscard(); if (pending) onTarget(pending); close() }

  return (
    <BottomSheet open={open} onClose={close} title="Áp dụng cho" height="auto" style={{ maxHeight: 'min(86%, calc(100dvh - var(--m-top) - 12px))' }}>
      <div {...stylex.props(sx.sheetContent)}>
        {pending?.scope === 'global' && (
          <UnsavedBox count={dirtyCount} object="“Mọi truyện”" onSave={resolveSave} onDiscard={resolveDiscard} onStay={() => setPending(null)} />
        )}
        <ScopeList kind="project" variant="sheet" target={target} open={openTarget} projects={projects} stories={stories}
          storiesOf={(id) => library.projectDetail(id)?.stories ?? []}
          projectName={projectName} storyTitle={storyTitle}
          pending={pending?.scope === 'project' ? pending : null} dirtyCount={dirtyCount}
          onPick={pick} onSave={resolveSave} onDiscard={resolveDiscard} onStay={() => setPending(null)} />
        <ScopeList kind="story" variant="sheet" target={target} open={openTarget} projects={projects} stories={stories}
          storiesOf={(id) => library.projectDetail(id)?.stories ?? []}
          projectName={projectName} storyTitle={storyTitle}
          pending={pending?.scope === 'story' ? pending : null} dirtyCount={dirtyCount}
          onPick={pick} onSave={resolveSave} onDiscard={resolveDiscard} onStay={() => setPending(null)} />
        <button {...stylex.props(sx.row, sx.sheetRow, target.scope === 'global' && sx.rowOn)} type="button" aria-pressed={target.scope === 'global'}
          onClick={() => pick({ ...target, scope: 'global' })}>
          <Icon name="global" size={16} {...stylex.props(sx.rowIcon)} />
          <span {...stylex.props(sx.rowText, sx.sheetRowText)}><span {...stylex.props(sx.rowName, sx.sheetRowName)}>Mọi truyện</span><span {...stylex.props(sx.rowSub)}>Toàn bộ thư viện</span></span>
          {target.scope === 'global' && <Icon name="tick-circle" size={15} style={{ color: 'var(--color-accent)' }} />}
        </button>
      </div>
    </BottomSheet>
  )
}

export type { ConfigScope }
