// Cách viết của dự án: cấu hình và danh sách truyện chọn văn phong khác.
import { useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { canManageProject, getConfig, getConfigPresets } from '../../../api/client'
import { useSession } from '../../../api/session'
import type { ProjectDetail } from '../../../api/types'
import { keys, useAction, useResource } from '../../../data'
import { storyLabel } from '../../../shell/labels'
import { paths } from '../../../router'
import { Icon, toast } from '@noverse/ui'
import { amWritingCopy } from '../../../i18n/vi'
import { BUILT_IN, saveStyle, styleDisplayName, StylePicker } from '../../settings/sections/style'
import { findPack, packBadge, packDesc, packInfoMeta } from '../../settings/lib/packs'
import { writingStyles as sx } from './StyleTab'
import '../../settings/style.css'

const KEY = (project: string) => ['project', project, 'style-by'] as const
export async function fetchStyleBy(project: string, stories: { story: string }[], signal?: AbortSignal) {
  return Promise.all(stories.map(async (item) => {
    const config = await getConfig('story', project, item.story, signal)
    const own = config.sources?.style === 'story'
    return { story: item.story, own, name: own ? config.target.config.style ?? null : null }
  }))
}
export function useStoryStyles(project: string, stories: { story: string }[]) {
  return useResource(KEY(project), ({ signal }) => fetchStyleBy(project, stories, signal), { enabled: stories.length > 0, staleTime: 30000 })
}
export function ProjectStyleSection({ project, detail }: { project: string; detail: ProjectDetail }) {
  const { user } = useSession()
  const canEdit = !!user && canManageProject(user, project)
  const [open, setOpen] = useState(false)
  const config = useResource(keys.config('project', project), ({ signal }) => getConfig('project', project, undefined, signal))
  const presets = useResource(['config-presets'], ({ signal }) => getConfigPresets(signal), { staleTime: 60000 })
  const storyStyles = useStoryStyles(project, detail.stories)
  const effectiveStyle = config.data?.effective.style ?? 'default'
  const ownProject = config.data?.sources?.style === 'project'
  const pack = findPack(presets.data?.packs, effectiveStyle)
  const desc = pack ? [packDesc(pack), packInfoMeta(pack)].filter(Boolean).join(' · ') : BUILT_IN.find((s) => s.id === effectiveStyle)?.desc
  const pick = useAction(async (id: string) => {
    if (!config.data) return
    const next = await saveStyle('project', project, undefined, id, config.data)
    toast.ok('Đã đổi văn phong mặc định của dự án · truyện mới sẽ dùng')
    return next
  }, { invalidate: [KEY(project)] })
  const rows = (storyStyles.data ?? []).filter((row) => row.own && row.name !== effectiveStyle)
  return <section {...stylex.props(sx.root)}>
    <h2 {...stylex.props(sx.heading)}><Icon name="brush-2" size={17} />{amWritingCopy.projectTitle}</h2>
    <p {...stylex.props(sx.desc)}>{amWritingCopy.projectIntro}</p>
    <h3 {...stylex.props(sx.heading)}>Văn phong</h3>
    <div className={`card ${stylex.props(sx.card).className}`}>
      <div {...stylex.props(sx.top)}><span {...stylex.props(sx.copy)}>
        <span {...stylex.props(sx.name)}>{styleDisplayName(presets.data, effectiveStyle)}{pack && packBadge(pack).ok && <span {...stylex.props(sx.badge)}><Icon name="shield-tick" size={12} />Có chấm</span>}{!ownProject && <span {...stylex.props(sx.inherited)}>chung mọi truyện</span>}</span>
        {desc && <span {...stylex.props(sx.desc)}>{desc}</span>}
      </span><button type="button" {...stylex.props(sx.button)} aria-expanded={open} disabled={!config.data} onClick={() => setOpen(!open)}>{open ? 'Xong' : 'Đổi'}</button></div>
      <div style={{ borderTop: '1px solid var(--line-1)', padding: '10px 16px 12px' }}>
        {storyStyles.error ? <span role="alert">Không tải được văn phong các truyện. <button type="button" onClick={() => void storyStyles.refresh()}>Thử lại</button></span> : !storyStyles.data && detail.stories.length ? <span {...stylex.props(sx.desc)}>Đang tải văn phong các truyện…</span> : <>
          <span {...stylex.props(sx.desc)}>{rows.length ? 'Truyện chọn văn phong khác:' : 'Mọi truyện trong dự án đang dùng văn phong này'}</span>
          {rows.map((row) => {
            const story = detail.stories.find((s) => s.story === row.story)
            if (!story) return null
            return <a key={row.story} className="st-smp" href={`#${paths.story(project, row.story)}?tab=style`}><Icon name="book-1" size={15} /><span title={storyLabel(story)} style={{ flex: 1, minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{storyLabel(story)}</span><span className="st-uchip">{styleDisplayName(presets.data, row.name ?? undefined)}</span></a>
          })}
        </>}
      </div>
    </div>
    {!!config.error && <span role="alert">Không tải được cách viết của dự án. <button type="button" onClick={() => void config.refresh()}>Thử lại</button></span>}
    {open && <section className="card st-drawer" style={{ borderRadius: 16, padding: '16px 18px' }}><StylePicker value={effectiveStyle} presets={presets.data ?? null} custom="blocked" disabled={!canEdit || pick.busy} onPick={(id) => void pick.run(id)} /></section>}
  </section>
}
