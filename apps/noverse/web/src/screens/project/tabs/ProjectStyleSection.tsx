// Gói AL — “Cách viết mặc định của dự án” ở trang tổng quan dự án (Outline.dc.html › pstyle,
// wstyle=proj): văn phong đang dùng + truyện nào chọn riêng; Đổi mở cùng lưới chọn như tab truyện.
// Chọn một ô là lưu ngay `config.style` ở cấp dự án (PUT /api/config?scope=project — không đổi API).
import { useState } from 'react'
import { canManageProject, getConfig, getConfigPresets } from '../../../api/client'
import { useSession } from '../../../api/session'
import type { ConfigResponse, ProjectDetail } from '../../../api/types'
import { cache, keys, useAction, useResource } from '../../../data'
import { projectLabel, storyLabel } from '../../../shell/labels'
import { paths } from '../../../router'
import { Icon, toast } from '@noverse/ui'
import { saveStyle, styleDisplayName, StylePicker } from '../../settings/sections/style'
import '../../settings/style.css'

const KEY = (project: string) => ['project', project, 'style-by'] as const

/**
 * Truyện nào chọn văn phong riêng: `GET /api/config?scope=story` của từng truyện và nhìn
 * `sources.style === 'story'` (06-data-contract §AL — không có API gộp, gọi từng truyện).
 */
export async function fetchStyleBy(project: string, stories: { story: string }[], signal?: AbortSignal): Promise<{ story: string; own: boolean; name: string | null }[]> {
  const rows = await Promise.all(stories.map(async (item) => {
    try {
      const config = await getConfig('story', project, item.story, signal)
      const own = config.sources?.style === 'story'
      return { story: item.story, own, name: own ? (config.target.config.style ?? null) : null }
    } catch {
      return { story: item.story, own: false, name: null }
    }
  }))
  return rows
}

export function useStoryStyles(project: string, stories: { story: string }[]) {
  const resource = useResource(KEY(project), ({ signal }) => fetchStyleBy(project, stories, signal), {
    enabled: stories.length > 0,
    staleTime: 30000,
  })
  return resource.data ?? null
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
  const pick = useAction(async (id: string) => {
    if (!config.data) return
    const next = await saveStyle('project', project, undefined, id, config.data)
    toast.ok('Đã đổi văn phong mặc định của dự án · truyện mới sẽ dùng')
    return next
  }, { invalidate: [KEY(project) as unknown as readonly (string | number | boolean | null)[]] })

  const projectName = projectLabel(detail.project, project)
  const rows = detail.stories.map((story) => {
    const found = storyStyles?.find((row) => row.story === story.story)
    return { story, own: !!found?.own, name: found?.name ?? null }
  })

  return (
    <section id="pstyle" style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
        <h2 style={{ margin: 0, fontSize: 15, fontWeight: 600, display: 'flex', alignItems: 'center', gap: 8, flex: '1 1 auto' }}>
          <Icon name="brush-2" size={17} style={{ color: 'var(--color-accent)' }} />Cách viết mặc định của dự án
        </h2>
        {canEdit && (
          <button type="button" className="hov-ghost" aria-expanded={open} onClick={() => setOpen((value) => !value)}
            style={{
              height: 30, padding: '0 12px', borderRadius: 8, border: '1px solid var(--kit-line-12)',
              background: 'transparent', color: 'var(--color-fg)', fontSize: 12.5,
            }}>
            {open ? 'Xong' : 'Đổi'}
          </button>
        )}
      </div>
      <p style={{ margin: '-4px 0 0', fontSize: 13, color: 'var(--kit-fg-icon)', lineHeight: 1.5 }}>
        Truyện mới trong “{projectName}” dùng văn phong này. Truyện nào cần khác thì chọn ở tab Cách viết của truyện đó.
      </p>
      <div className="card" style={{ borderRadius: 14, overflow: 'hidden' }}>
        <div className="st-smp">
          <Icon name="brush-2" size={16} style={{ color: 'var(--color-accent)', flex: '0 0 auto' }} />
          <span style={{ flex: '1 1 auto', minWidth: 0, display: 'flex', flexDirection: 'column', lineHeight: 1.4 }}>
            <b style={{ fontSize: 14, fontWeight: 600 }}>{styleDisplayName(presets.data, effectiveStyle)}</b>
            <span style={{ fontSize: 12.5, color: 'var(--kit-fg-icon)' }}>
              {ownProject ? 'Riêng dự án này' : 'Theo “Mọi truyện” · đổi ở Tuỳ chỉnh › Cách viết chung'}
            </span>
          </span>
        </div>
        {rows.map((row) => (
          <a key={row.story.story} className="st-smp" href={`#${paths.story(project, row.story.story)}?tab=style`} style={{ textDecoration: 'none', color: 'inherit' }}>
            <Icon name="book-1" size={16} style={{ color: 'var(--kit-fg-icon)', flex: '0 0 auto' }} />
            <span title={storyLabel(row.story)} style={{ flex: '1 1 auto', minWidth: 0, fontSize: 13.5, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
              {storyLabel(row.story)}
            </span>
            <span className="st-uchip" style={{
              background: row.own ? 'var(--kit-acc-12)' : 'var(--toc-hov)',
              color: row.own ? 'var(--color-accent-soft)' : 'var(--kit-fg-icon)',
            }}>{row.own ? (row.name ? styleDisplayName(presets.data, row.name) : 'Riêng truyện') : 'Theo dự án'}</span>
          </a>
        ))}
      </div>
      {open && (
        <section className="card st-drawer" style={{ borderRadius: 16, padding: '16px 18px' }}>
          <StylePicker value={effectiveStyle} presets={presets.data ?? null}
            custom="blocked" disabled={!canEdit || pick.busy}
            onPick={(id) => { if (!pick.busy) void pick.run(id) }} />
        </section>
      )}
    </section>
  )
}
