// ScopeBar — thanh “Áp dụng cho” + danh sách chọn dự án/truyện + dải “Đang xem…” (Settings.dc.html showScope,
// 21-dev-prompt.md › Gói R). Ô “Truyện”/“Dự án” gồm hai nút: phần chữ chọn cấp, ▾ mở danh sách 360 px.
import { useEffect, useRef, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import type { ConfigScope, ProjectSummary, StorySummary } from '../../api/types'
import { Icon } from '@noverse/ui'
import { useEscapeLayer } from '@noverse/ui'
import { ScopeList, UnsavedBox } from './scope/lists'
import {
  clipLabel, storyBoxLabel, targetKey, viewingElsewhereText, type OpenTarget, type ScopeTarget,
} from './scope/model'
import { sx } from './scope/scope.stylex'

/** Câu giải thích kế thừa dưới thanh (Settings.dc.html `scopeNote`). */
export const scopeNote = (scope: ConfigScope, projectName: string): string =>
  scope === 'story'
    ? `Chỉ truyện này. Những gì không chỉnh ở đây lấy theo dự án “${projectName}”.`
    : scope === 'project'
      ? `Mọi truyện trong dự án “${projectName}”, trừ chỗ một truyện chỉnh riêng.`
      : 'Mặc định cho mọi dự án và truyện. Dự án hoặc truyện chỉnh riêng sẽ được ưu tiên.'

type ListKind = 'project' | 'story' | 'global'

export type ScopeBarProps = {
  target: ScopeTarget
  projectName: string
  storyTitle: string
  storyCount: number
  open: OpenTarget
  projects: readonly ProjectSummary[]
  stories: readonly StorySummary[]
  storiesOf: (project: string) => readonly StorySummary[]
  /** số thay đổi chưa lưu của màn đang mở; > 0 thì đổi chỗ xem phải hỏi trước */
  dirtyCount: number
  /** đổi ngữ cảnh đang xem (không đổi truyện đang mở ở Bàn viết) */
  onTarget: (target: ScopeTarget) => void
  /** trả `true` khi đã lưu xong; lỗi/xung đột thì giữ hộp cam lại */
  onSave: () => Promise<unknown> | unknown
  onDiscard: () => void
  /** tải trước danh sách truyện của dự án sắp mở */
  ensureProject?: (project: string) => void
}

export function ScopeBar(props: ScopeBarProps) {
  const { target, projectName, storyTitle, storyCount, open, projects, stories, dirtyCount } = props
  const [list, setList] = useState<ListKind | null>(null)
  const [pending, setPending] = useState<ScopeTarget | null>(null)
  const root = useRef<HTMLDivElement>(null)

  const close = () => { setList(null); setPending(null) }
  useEscapeLayer(list !== null, close)
  useEffect(() => {
    if (!list) return
    const outside = (event: PointerEvent) => { if (!root.current?.contains(event.target as Node)) close() }
    document.addEventListener('pointerdown', outside)
    return () => document.removeEventListener('pointerdown', outside)
  }, [list])

  /** Đổi chỗ xem: còn thay đổi chưa lưu thì hỏi trong danh sách trước (R3). */
  const pick = (next: ScopeTarget) => {
    if (targetKey(next) === targetKey(target)) { close(); return }
    if (dirtyCount > 0) {
      setPending(next)
      setList(next.scope === 'global' ? 'global' : next.scope)
      if (next.scope === 'project') props.ensureProject?.(next.project)
      return
    }
    props.onTarget(next)
    close()
  }
  const openList = (kind: 'project' | 'story') => {
    setPending(null)
    setList(kind)
    if (kind === 'project') projects.forEach((item) => props.ensureProject?.(item.id))
  }
  /**
   * R1 — phần chữ của ô chọn cấp như cũ; bấm lại đúng ô đang chọn thì mở danh sách.
   * Người dùng bấm cả ô (không chỉ nút ▾) để đổi dự án/truyện; trước đây cú bấm đó rơi vào
   * khoảng trống, nên trên dữ liệu thật danh sách không mở.
   */
  const choose = (kind: 'project' | 'story') => {
    if (target.scope !== kind || !(kind === 'story' ? target.story : target.project)) {
      pick({ ...target, scope: kind })
      return
    }
    if (list === kind) close()
    else openList(kind)
  }
  const resolveSave = async () => {
    if (await props.onSave() !== true) return
    if (pending) props.onTarget(pending)
    close()
  }
  const resolveDiscard = () => { props.onDiscard(); if (pending) props.onTarget(pending); close() }
  const stayPending = () => setPending(null)

  const levelIcon = (on: boolean) => stylex.props(sx.levelIcon, on && sx.levelIconOn)
  const levelSub = (text: string) => <span {...stylex.props(sx.levelSub)} title={text}>{clipLabel(text, 30)}</span>
  const elsewhere = viewingElsewhereText(target, { projectName, storyTitle, open })

  return (
    <div ref={root}>
      <div role="group" aria-label="Áp dụng cho" {...stylex.props(sx.bar)}>
        <span {...stylex.props(sx.lead)}>Áp dụng cho</span>

        <div {...stylex.props(sx.box, target.scope === 'story' && sx.boxOn)}>
          <button type="button" aria-pressed={target.scope === 'story'} disabled={!target.story}
            onClick={() => choose('story')} {...stylex.props(sx.level, !target.story && sx.levelDisabled)}>
            <Icon name="book-1" size={18} {...levelIcon(target.scope === 'story')} />
            <span {...stylex.props(sx.levelText)}>
              <span {...stylex.props(sx.levelLabel)}>{storyBoxLabel(target, open)}</span>
              {levelSub(storyTitle || '—')}
            </span>
          </button>
          <button type="button" aria-haspopup="listbox" aria-expanded={list === 'story'} aria-label="Chọn truyện khác trong dự án"
            disabled={!target.project} onClick={() => (list === 'story' ? close() : openList('story'))}
            {...stylex.props(sx.caret, !target.project && sx.levelDisabled)}>
            <Icon name="arrow-down-2" size={14} />
          </button>
          {list === 'story' && (
            <ScopeList kind="story" target={target} open={open} projects={projects} stories={stories} storiesOf={props.storiesOf}
              projectName={projectName} storyTitle={storyTitle} pending={pending} dirtyCount={dirtyCount}
              onPick={pick} onSave={resolveSave} onDiscard={resolveDiscard} onStay={stayPending} />
          )}
        </div>

        <div {...stylex.props(sx.box, target.scope === 'project' && sx.boxOn)}>
          <button type="button" aria-pressed={target.scope === 'project'} disabled={!target.project}
            onClick={() => choose('project')} {...stylex.props(sx.level, !target.project && sx.levelDisabled)}>
            <Icon name="folder-open" size={18} {...levelIcon(target.scope === 'project')} />
            <span {...stylex.props(sx.levelText)}>
              <span {...stylex.props(sx.levelLabel)}>Dự án</span>
              {levelSub(`${projectName} · ${storyCount} truyện`)}
            </span>
          </button>
          <button type="button" aria-haspopup="listbox" aria-expanded={list === 'project'} aria-label="Chọn dự án khác"
            onClick={() => (list === 'project' ? close() : openList('project'))} {...stylex.props(sx.caret)}>
            <Icon name="arrow-down-2" size={14} />
          </button>
          {list === 'project' && (
            <ScopeList kind="project" target={target} open={open} projects={projects} stories={stories} storiesOf={props.storiesOf}
              projectName={projectName} storyTitle={storyTitle} pending={pending} dirtyCount={dirtyCount}
              onPick={pick} onSave={resolveSave} onDiscard={resolveDiscard} onStay={stayPending} />
          )}
        </div>

        {/* “Mọi truyện” không có ▾ (R1). */}
        <div {...stylex.props(sx.box, target.scope === 'global' && sx.boxOn)}>
          <button type="button" aria-pressed={target.scope === 'global'}
            onClick={() => pick({ ...target, scope: 'global' })}
            {...stylex.props(sx.level)}>
            <Icon name="global" size={18} {...levelIcon(target.scope === 'global')} />
            <span {...stylex.props(sx.levelText)}>
              <span {...stylex.props(sx.levelLabel)}>Mọi truyện</span>
              <span {...stylex.props(sx.levelSub)}>Toàn bộ thư viện</span>
            </span>
          </button>
          {list === 'global' && pending && (
            <div {...stylex.props(sx.pop)} role="dialog" aria-label="Chuyển sang “Mọi truyện”">
              <UnsavedBox count={dirtyCount}
                object={pending.scope === 'global' ? '“Mọi truyện”' : pending.scope === 'project' ? `dự án “${projectName}”` : `truyện “${storyTitle}”`}
                onSave={resolveSave} onDiscard={resolveDiscard} onStay={() => { stayPending(); close() }} />
            </div>
          )}
        </div>
      </div>

      {elsewhere && (
        <div role="status" {...stylex.props(sx.strip)}>
          <Icon name="eye" size={15} style={{ color: 'var(--kit-info-fg)', flex: '0 0 auto' }} />
          <span {...stylex.props(sx.stripText)}>{elsewhere}</span>
          <button type="button" {...stylex.props(sx.stripGo)}
            onClick={() => open && pick({ scope: 'story', project: open.project, story: open.story })}>Quay về truyện đang mở</button>
        </div>
      )}
      <p {...stylex.props(sx.note)}>{scopeNote(target.scope, projectName)}</p>
    </div>
  )
}
