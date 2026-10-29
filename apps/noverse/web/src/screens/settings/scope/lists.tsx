// Danh sách chọn dự án/truyện của Gói R — dùng chung cho popover máy tính và bottom sheet điện thoại.
// Bàn phím như `Select` của Kit: ↑↓/Home/End di chuyển, Enter chọn (Esc do lớp của nơi mở giữ).
import { useEffect, useId, useMemo, useRef, useState, type KeyboardEvent } from 'react'
import * as stylex from '@stylexjs/stylex'
import type { ProjectSummary, StorySummary } from '../../../api/types'
import { Icon } from '@noverse/ui'
import { nextActive, type Option } from '@noverse/ui'
import { projectLabel, storyLabel } from '../../../shell/labels'
import { useOwnRuleCount } from './counts'
import {
  clipLabel, pendingObject, projectRuleSub, storyForProject, storyRuleSub, unsavedText, type OpenTarget, type ScopeTarget,
} from './model'
import { sx } from './scope.stylex'

export type ScopeListKind = 'project' | 'story'

type Row = { key: string; target: ScopeTarget; disabled?: boolean }

export type ScopeListProps = {
  kind: ScopeListKind
  target: ScopeTarget
  open: OpenTarget
  projects: readonly ProjectSummary[]
  /** truyện của dự án đang xem (dự án của `target`) */
  stories: readonly StorySummary[]
  /** truyện của một dự án khác, nếu đã tải — dùng khi đổi dự án */
  storiesOf: (project: string) => readonly StorySummary[]
  projectName: string
  storyTitle: string
  /** mục tiêu chờ xác nhận khi còn thay đổi chưa lưu (hộp cam) */
  pending: ScopeTarget | null
  dirtyCount: number
  variant?: 'pop' | 'sheet'
  onPick: (target: ScopeTarget) => void
  onSave: () => void
  onDiscard: () => void
  onStay: () => void
}

/** Hộp cam “chưa lưu” (R3): Lưu rồi chuyển · Bỏ thay đổi · Ở lại. */
export function UnsavedBox({ count, object, onSave, onDiscard, onStay }: {
  count: number; object: string; onSave: () => void; onDiscard: () => void; onStay: () => void
}) {
  return (
    <div role="alert" {...stylex.props(sx.unsaved)}>
      <span {...stylex.props(sx.unsavedText)}>{unsavedText(count, object)}</span>
      <div {...stylex.props(sx.unsavedActions)}>
        <button type="button" data-scope-action="save" {...stylex.props(sx.action, sx.actionPrimary)} onClick={onSave}>Lưu rồi chuyển</button>
        <button type="button" data-scope-action="discard" {...stylex.props(sx.action)} onClick={onDiscard}>Bỏ thay đổi</button>
        <button type="button" data-scope-action="stay" {...stylex.props(sx.action)} onClick={onStay}>Ở lại</button>
      </div>
    </div>
  )
}

/** Dòng “M quy tắc riêng” của một truyện — tự tải, chỉ khi danh sách đang mở. */
function RuleCount({ project, story }: { project: string; story: string }) {
  const own = useOwnRuleCount('story', project, story)
  return <span {...stylex.props(sx.rowSub)}>{storyRuleSub(own)}</span>
}

export function ScopeList(props: ScopeListProps) {
  const { kind, target, open, projects, stories, projectName, storyTitle, pending, dirtyCount, variant = 'pop' } = props
  const id = useId()
  const [query, setQuery] = useState('')
  const [active, setActive] = useState(0)
  const search = useRef<HTMLInputElement>(null)
  const list = useRef<HTMLDivElement>(null)

  const needle = query.trim().toLocaleLowerCase('vi')
  const rows = useMemo<Row[]>(() => {
    if (kind === 'project') {
      return projects
        .filter((item) => !needle || projectLabel(item, item.id).toLocaleLowerCase('vi').includes(needle))
        .map((item) => ({
          key: `p:${item.id}`,
          target: { scope: 'project', project: item.id, story: storyForProject(item.id, props.storiesOf(item.id), open) },
        }))
    }
    return stories
      .filter((item) => !needle || storyLabel(item).toLocaleLowerCase('vi').includes(needle))
      .map((item) => ({ key: `s:${item.story}`, target: { scope: 'story', project: target.project, story: item.story } }))
  }, [kind, projects, stories, needle, open, props.storiesOf, target.project])

  // Mở danh sách: đặt mục sáng ở dòng đang xem (dự án/truyện), như Select của Kit.
  useEffect(() => {
    const index = rows.findIndex((row) => row.target.project === target.project && (kind === 'project' || row.target.story === target.story))
    setActive(index >= 0 ? index : 0)
    if (variant !== 'pop') return
    if (kind === 'project' || stories.length > 5) requestAnimationFrame(() => search.current?.focus())
    else requestAnimationFrame(() => list.current?.focus())
    // Chỉ khi mở/đổi danh sách, không mỗi lần gõ tìm.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [kind, target.project, target.story, variant])
  useEffect(() => {
    if (active < 0) return
    document.getElementById(`${id}-opt-${active}`)?.scrollIntoView({ block: 'nearest' })
  }, [active, id])

  // Mục kế tiếp dùng đúng luật của Kit Select: bỏ qua mục khoá, Home/End nhảy đầu/cuối.
  const options = useMemo<Option[]>(() => rows.map((row) => ({ value: row.key, label: row.key, disabled: row.disabled })), [rows])
  const keyboard = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.nativeEvent.isComposing) return
    if (['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) {
      event.preventDefault()
      setActive((current) => nextActive(options, current, event.key))
    } else if (event.key === 'Enter') {
      if (event.target instanceof HTMLButtonElement && event.target.dataset.scopeAction) return
      event.preventDefault()
      const row = rows[active]
      if (row && !row.disabled) props.onPick(row.target)
    }
  }

  const pendingTarget = pending
  const pendingName = pendingTarget
    ? pendingObject(
      pendingTarget,
      pendingTarget.scope === 'project'
        ? projectLabel(projects.find((item) => item.id === pendingTarget.project), pendingTarget.project)
        : pendingTarget.project === target.project ? projectName : pendingTarget.project,
      pendingTarget.scope === 'story'
        ? (pendingTarget.project === target.project ? storyTitle : storyLabel(stories.find((item) => item.story === pendingTarget.story)))
        : '',
    )
    : ''

  const title = kind === 'project' ? 'Chọn dự án' : projectName ? `Truyện trong “${projectName}”` : 'Truyện'
  const total = kind === 'project' ? `${projects.length} dự án` : `${stories.length} truyện`
  // “Quay về truyện đang mở” chỉ hiện khi đang xem chỗ khác truyện đang mở (R2).
  const back = !!open && (target.project !== open.project || target.story !== open.story)

  return (
    <div role={variant === 'pop' ? 'dialog' : 'group'} aria-label={title} {...stylex.props(variant === 'pop' ? sx.pop : sx.sheetSection)} onKeyDown={keyboard}>
      <div {...stylex.props(sx.head, variant === 'sheet' && sx.sheetHead)}>
        <span {...stylex.props(sx.headTitle, variant === 'sheet' && sx.sheetHeadTitle)} title={title}>{title}</span>
        <span {...stylex.props(sx.headCount)}>{total}</span>
      </div>
      {pendingTarget && (
        <UnsavedBox count={dirtyCount} object={pendingName} onSave={props.onSave} onDiscard={props.onDiscard} onStay={props.onStay} />
      )}
      {(kind === 'project' || stories.length > 5) && (
        variant === 'sheet' ? <label {...stylex.props(sx.sheetSearch)}>
          <Icon name="search-normal-1" size={15} {...stylex.props(sx.rowIcon)} />
          <input ref={search} {...stylex.props(sx.sheetSearchInput)} type="search" value={query} aria-label={kind === 'project' ? 'Tìm dự án' : 'Tìm truyện'}
            placeholder={kind === 'project' ? 'Tìm dự án…' : 'Tìm truyện…'} onChange={(event) => { setQuery(event.target.value); setActive(0) }} />
        </label> : <input ref={search} {...stylex.props(sx.search)} type="search" value={query} aria-label={kind === 'project' ? 'Tìm dự án' : 'Tìm truyện'}
          placeholder={kind === 'project' ? 'Tìm dự án…' : 'Tìm truyện…'} onChange={(event) => { setQuery(event.target.value); setActive(0) }} />
      )}
      <div id={`${id}-list`} ref={list} role="listbox" tabIndex={-1} aria-label={title} aria-activedescendant={rows[active] ? `${id}-opt-${active}` : undefined} {...stylex.props(sx.list, variant === 'sheet' && sx.sheetList)}>
        {rows.length === 0 && (
          <p {...stylex.props(sx.empty)}>{kind === 'story' ? (stories.length === 0 ? 'Dự án này chưa có truyện.' : 'Không tìm thấy truyện.') : 'Không tìm thấy dự án.'}</p>
        )}
        {rows.map((row, index) => {
          const project = kind === 'project' ? projects.find((item) => item.id === row.target.project) : undefined
          const story = kind === 'story' ? stories.find((item) => item.story === row.target.story) : undefined
          const selected = kind === 'project' ? row.target.project === target.project && target.scope !== 'global' : row.target.story === target.story && target.scope === 'story'
          const isOpenStory = kind === 'story' && !!open && open.project === row.target.project && open.story === row.target.story
          return (
            <button key={row.key} id={`${id}-opt-${index}`} type="button" role="option" aria-selected={selected} data-hi={index === active}
              {...stylex.props(sx.row, variant === 'sheet' && sx.sheetRow, index === active && sx.rowHi, selected && sx.rowOn)}
              onMouseEnter={() => setActive(index)} onMouseDown={(event) => event.preventDefault()} onClick={() => props.onPick(row.target)}>
              <Icon name={kind === 'project' ? 'folder-open' : 'book-1'} size={variant === 'sheet' ? 16 : 17} {...stylex.props(sx.rowIcon)} />
              <span {...stylex.props(sx.rowText, variant === 'sheet' && sx.sheetRowText)}>
                <span {...stylex.props(sx.rowName, variant === 'sheet' && sx.sheetRowName)} title={kind === 'project' ? projectLabel(project, row.target.project) : storyLabel(story)}>
                  {kind === 'project' ? projectLabel(project, row.target.project) : clipLabel(storyLabel(story), 34)}
                </span>
                {kind === 'project'
                  ? <ProjectSub project={project} />
                  : story && <RuleCount project={row.target.project} story={row.target.story} />}
              </span>
              {variant === 'sheet' ? <span {...stylex.props(sx.sheetMarks)}>
                {((kind === 'project' && open?.project === row.target.project) || isOpenStory) && <span {...stylex.props(sx.chip)}>Đang mở</span>}
                {selected && <Icon name="tick-circle" size={15} style={{ color: 'var(--color-accent)' }} />}
              </span> : <>
                {kind === 'project' && open?.project === row.target.project && <span {...stylex.props(sx.chip)}>Đang mở</span>}
                {kind === 'story' && isOpenStory && <span {...stylex.props(sx.chip)}>Đang mở</span>}
                {selected && <Icon name="tick-circle" size={16} style={{ color: 'var(--color-accent)' }} />}
              </>}
            </button>
          )
        })}
      </div>
      {back && open && (
        <>
          <div {...stylex.props(sx.sep)} />
          <button type="button" {...stylex.props(sx.footer)} onClick={() => props.onPick({ scope: 'story', project: open.project, story: open.story })}>
            <Icon name="arrow-left-2" size={15} style={{ color: 'var(--color-fg-3)' }} />
            <span {...stylex.props(sx.footerText)}>Quay về truyện đang mở · {clipLabel(open.title, 28)}</span>
          </button>
        </>
      )}
    </div>
  )
}

/** Dòng phụ của dự án: “N truyện · M quy tắc riêng” (đếm luật riêng của dự án). */
function ProjectSub({ project }: { project: ProjectSummary | undefined }) {
  const own = useOwnRuleCount('project', project?.id ?? '', '')
  return <span {...stylex.props(sx.rowSub)}>{project ? projectRuleSub(project.storyCount, own) : ''}</span>
}
