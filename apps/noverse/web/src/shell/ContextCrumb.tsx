// Gói AG — thanh trên của màn Dự án (Outline.dc.html › crumbs): “Tên dự án ▾ / Tên truyện”.
// Đổi dự án CHỈ ở đây; bấm tên truyện mở cùng popover đổi truyện với thẻ truyện cột trái
// (F2: “Đổi truyện là một cú bấm” — không về Dự án rồi chọn lại).
// Variant 'link' (Main/NewStory): link về Dự án, không chevron, không popover.
import { useEffect, useRef, useState, type CSSProperties } from 'react'
import { useLibrary } from '../api/library'
import type { StorySummary } from '../api/types'
import { paths } from '../router'
import { t } from '../i18n/vi'
import { Icon } from '@noverse/ui'
import { projectLabel, storyLabel } from './labels'
import { ProjectPopover, StoryPopover } from '../screens/project/nav'

const crumbBase: CSSProperties = {
  display: 'flex', alignItems: 'center',
  borderRadius: 8, border: '1px solid var(--line-3)', background: 'var(--color-surface-1)',
  fontSize: 13, fontWeight: 500, whiteSpace: 'nowrap', color: 'var(--color-fg)', cursor: 'pointer',
}

export function ContextCrumb({ project, story, volume, variant = 'link', trailing }: {
  project: string | null
  /** truyện đang chọn (từ route); StorySummary khi đã tải */
  story: StorySummary | null
  /** nhãn tập hiển thị, vd "Tập 1" — chỉ render ở variant 'link' */
  volume?: string | null
  /** 'switcher': nút mở popover đổi dự án + đổi truyện (màn Dự án) · 'link': link tới Dự án */
  variant?: 'link' | 'switcher'
  /** phần sau dấu / khi chưa chọn truyện, vd "Truyện mới" */
  trailing?: string | null
}) {
  const { library, projectDetail, ensureProject } = useLibrary()
  const [open, setOpen] = useState<'' | 'project' | 'story'>('')
  const projectWrap = useRef<HTMLDivElement>(null)
  const storyWrap = useRef<HTMLDivElement>(null)
  const switcher = variant === 'switcher'

  useEffect(() => {
    if (project) ensureProject(project)
  }, [project, ensureProject])
  // Đóng popover khi bấm ra ngoài cả hai trigger (popover tự xử lý outside của nó).
  useEffect(() => {
    if (!switcher || !open) return
    const onDoc = (event: MouseEvent) => {
      const node = event.target as Node
      if (open === 'project' && projectWrap.current?.contains(node)) setOpen('')
      if (open === 'story' && storyWrap.current?.contains(node)) setOpen('')
    }
    document.addEventListener('mousedown', onDoc)
    return () => document.removeEventListener('mousedown', onDoc)
  }, [switcher, open])

  const projectName = projectLabel(library?.projects.find((p) => p.id === project), project)
  const title = storyLabel(story)
  const detail = project ? projectDetail(project) : null
  const projectFace = (
    <>
      {/* Outline.dc.html: icon thư mục của crumb switcher = accent 15px; variant link giữ fg-3. */}
      <Icon name="folder-open" size={15} style={{ color: switcher ? 'var(--color-accent)' : 'var(--color-fg-3)' }} />
      {/* Gói Q: đường dẫn bên trái tự cắt “…” — không bao giờ đẩy thanh điều hướng ở giữa. */}
      <span style={{ minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{projectName}</span>
    </>
  )
  const separator = <span style={{ color: 'var(--color-fg-5)' }} aria-hidden="true">/</span>

  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: switcher ? 6 : 10, minWidth: 0, maxWidth: '100%', overflow: 'visible' }}>
      {switcher ? (
        <div ref={projectWrap} style={{ position: 'relative' }}>
          <button type="button" className="hov-ghost pj-crumb" title={t.nav.changeProject}
            aria-expanded={open === 'project'} aria-haspopup="dialog" onClick={() => setOpen(open === 'project' ? '' : 'project')}
            onKeyDown={(event) => { if ((event.key === 'ArrowDown' || event.key === 'ArrowUp') && open !== 'project') { event.preventDefault(); setOpen('project') } }}
            style={{ ...crumbBase, height: 30, padding: '0 8px', gap: 6, borderColor: open === 'project' ? 'var(--kit-wash-18)' : undefined, background: open === 'project' ? 'var(--color-surface-4)' : undefined }}>
            {projectFace}
            <Icon name="arrow-down-2" size={13} style={{ color: 'var(--color-fg-4)' }} />
          </button>
          {open === 'project' && <ProjectPopover current={project} close={() => setOpen('')} />}
        </div>
      ) : (
        <a className="hov-ghost" href={project ? `#${paths.project(project)}` : '#'} title={t.nav.changeProject}
          style={{ ...crumbBase, height: 30, padding: '0 8px', gap: 6 }}>
          {projectFace}
        </a>
      )}
      {story ? (
        <>
          {separator}
          {switcher ? (
            <div ref={storyWrap} style={{ position: 'relative' }}>
              <button type="button" className="hov-ghost" title={t.nav.changeStory} aria-haspopup="dialog"
                aria-expanded={open === 'story'} onClick={() => setOpen(open === 'story' ? '' : 'story')}
                onKeyDown={(event) => { if ((event.key === 'ArrowDown' || event.key === 'ArrowUp') && open !== 'story') { event.preventDefault(); setOpen('story') } }}
                style={{
                  minWidth: 0, height: 30, border: 'none', background: 'transparent', padding: '0 4px', borderRadius: 7,
                  fontSize: 13, color: 'var(--color-fg-2)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
                  display: 'inline-flex', alignItems: 'center', gap: 5, cursor: 'pointer',
                }}>
                {title}
              </button>
              {open === 'story' && <StoryPopover project={project ?? story.project} detail={detail} active={story} storyView close={() => setOpen('')} />}
            </div>
          ) : (
            <>
              <a href={`#${paths.story(story.project, story.story)}`}
                style={{ minWidth: 0, fontSize: 13, color: 'var(--color-fg-2)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                {title}
              </a>
              {separator}
              <span style={{ fontSize: 13, color: 'var(--color-fg-3)', whiteSpace: 'nowrap' }}>{volume ?? 'Tập 1'}</span>
            </>
          )}
        </>
      ) : trailing ? (
        <>
          {separator}
          <span style={{ fontSize: 13, color: 'var(--color-fg-3)', whiteSpace: 'nowrap' }}>{trailing}</span>
        </>
      ) : null}
    </div>
  )
}
