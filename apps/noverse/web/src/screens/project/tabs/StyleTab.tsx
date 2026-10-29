// Gói AL — tab “Cách viết” của truyện (Outline.dc.html › tab=style · wstyle: pack · builtin · inherit · samples).
// Văn phong + văn mẫu là của truyện: chọn một ô là lưu ngay `config.style` ở cấp truyện (không đổi
// API), “Dùng theo dự án” xoá ghi đè. “Văn phong của bạn” chỉ chọn được khi truyện đã có văn mẫu.
import { canWriteStory, getConfig, getConfigPresets, getProject, getSnapshot, getStyleSamples } from '../../../api/client'
import { useSession } from '../../../api/session'
import type { StorySummary } from '../../../api/types'
import { keys, useAction, useResource } from '../../../data'
import { effectiveFromChapter } from '../../settings/lib/labels'
import { findPack } from '../../settings/lib/packs'
import { PackSummaryCard, saveStyle, styleDisplayName, StylePicker } from '../../settings/sections/style'
import { paths } from '../../../router'
import { Icon, Skeleton, toast } from '@noverse/ui'
import { StyleSamples } from './StyleSamples'
import '../../settings/style.css'

export function StyleTab({ project, summary }: { project: string; summary: StorySummary }) {
  const story = summary.story
  const { user } = useSession()
  const canEdit = !!user && canWriteStory(user, `${project}/${story}`)
  const config = useResource(keys.config('story', project, story), ({ signal }) => getConfig('story', project, story, signal))
  const presets = useResource(['config-presets'], ({ signal }) => getConfigPresets(signal), { staleTime: 60000 })
  const detail = useResource(keys.project(project), () => getProject(project), { staleTime: 60000 })
  const snapshot = useResource(keys.story(project, story, 'snapshot'), ({ signal }) => getSnapshot(project, story, signal))
  const samples = useResource(keys.story(project, story, 'style-samples'), ({ signal }) => getStyleSamples(project, story, signal))

  const own = config.data?.sources?.style === 'story'
  const effectiveStyle = config.data?.effective.style ?? 'default'
  // Văn phong của dự án = tệp cấp dự án (chưa ghi thì lấy “Mọi truyện”).
  const projectStyle = config.data
    ? config.data.chain.find((doc) => doc.scope === 'project')?.config.style ?? config.data.chain.find((doc) => doc.scope === 'global')?.config.style ?? 'default'
    : 'default'

  const pick = useAction(async (id: string) => {
    if (!config.data) return
    const next = await saveStyle('story', project, story, id, config.data)
    toast.ok(`Đã đổi văn phong của truyện · áp dụng từ chương ${effectiveFromChapter(snapshot.data)}`)
    return next
  }, { invalidate: [['project', project, 'style-by']] })

  const useProject = useAction(async () => {
    if (!config.data) return
    const next = await saveStyle('story', project, story, undefined, config.data)
    toast.ok(`Truyện dùng lại văn phong của dự án · từ chương ${effectiveFromChapter(snapshot.data)}`)
    return next
  }, { invalidate: [['project', project, 'style-by']] })

  if (config.initialLoading) return <Skeleton height={300} radius={16} />
  if (config.error || !config.data) {
    return (
      <section className="card st-sec" aria-label="Cách viết">
        <span style={{ fontSize: 13, color: 'var(--color-err)' }}>Không tải được cách viết của truyện.</span>
        <button type="button" className="hov-ghost" onClick={() => void config.refresh()}
          style={{ alignSelf: 'flex-start', height: 30, padding: '0 12px', borderRadius: 8, border: '1px solid var(--line-4)', background: 'transparent', color: 'var(--color-fg)', fontSize: 12.5 }}>
          Thử lại
        </button>
      </section>
    )
  }

  const pack = findPack(presets.data?.packs, effectiveStyle)
  const projectName = detail.data?.project.name ?? project
  // Icon màu của thanh kế thừa theo artboard: riêng → book tím, kế thừa → folder xanh.
  const inhIconColor = own ? 'var(--color-accent)' : 'var(--kit-info-fg)'

  return (
    <div style={{ maxWidth: 860, display: 'flex', flexDirection: 'column', gap: 14 }}>
      <p style={{ margin: 0, fontSize: 13.5, color: 'var(--color-fg-3)', lineHeight: 1.55 }}>
        Văn phong của truyện này. Đổi ở đây áp dụng từ chương tiếp theo; chương đã viết giữ nguyên.
      </p>
      <div className={`st-inh${own ? ' own' : ''}`}>
        <Icon name={own ? 'book-1' : 'folder-open'} size={18} style={{ color: inhIconColor, flex: '0 0 auto' }} />
        <span style={{ flex: '1 1 auto', minWidth: 0, display: 'flex', flexDirection: 'column', lineHeight: 1.4 }}>
          <b style={{ fontSize: 14, fontWeight: 600 }}>{styleDisplayName(presets.data, effectiveStyle)}</b>
          <span style={{ fontSize: 12.5, color: 'var(--kit-fg-icon)' }}>
            {own
              ? `Riêng truyện này · dự án đang dùng “${styleDisplayName(presets.data, projectStyle)}”`
              : `Theo dự án “${projectName}” · chọn một kiểu bên dưới để dùng riêng`}
          </span>
        </span>
        {own && canEdit && (
          <button type="button" className="hov-ghost" disabled={useProject.busy} onClick={() => void useProject.run()}
            style={{
              height: 30, padding: '0 12px', borderRadius: 8, border: '1px solid var(--kit-line-12)',
              background: 'transparent', color: 'var(--color-fg)', fontSize: 12.5, whiteSpace: 'nowrap', flex: '0 0 auto',
            }}>
            Dùng theo dự án
          </button>
        )}
      </div>
      <section className="card" style={{ borderRadius: 16, padding: '16px 18px' }}>
        <StylePicker value={effectiveStyle} presets={presets.data ?? null}
          custom="story" story
          samples={samples.data?.total ?? null}
          disabled={!canEdit || pick.busy || useProject.busy}
          onPick={(id) => { if (!pick.busy) void pick.run(id) }} />
      </section>
      {pack && <PackSummaryCard pack={pack} qualityHref={`#${paths.quality(project, story)}`} />}
      <StyleSamples project={project} story={story} canEdit={canEdit} />
    </div>
  )
}
