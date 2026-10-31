// Cách viết của truyện: văn phong, văn mẫu và quy tắc trong cùng một tab.
import { useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { color, kit, line } from '@noverse/ui/tokens.stylex'
import { canWriteStory, getConfig, getConfigPresets, getSnapshot, getStyleSamples } from '../../../api/client'
import { useSession } from '../../../api/session'
import type { StorySummary } from '../../../api/types'
import { keys, useAction, useResource } from '../../../data'
import { effectiveFromChapter } from '../../settings/lib/labels'
import { findPack, packBadge, packDesc, packInfoMeta } from '../../settings/lib/packs'
import { BUILT_IN, PackSummaryCard, saveStyle, styleDisplayName, StylePicker } from '../../settings/sections/style'
import { RulesPanel } from '../../settings/sections/rules'
import { amWritingCopy } from '../../../i18n/vi'
import { paths } from '../../../router'
import { Icon, Skeleton, toast } from '@noverse/ui'
import { StyleSamples } from './StyleSamples'
import '../../settings/style.css'

export const writingStyles = stylex.create({
  root: { maxWidth: 860, display: 'flex', flexDirection: 'column', gap: 14, lineHeight: 1.5 },
  intro: { margin: 0, fontSize: 13.5, color: color['--color-fg-3'], lineHeight: 1.55 },
  heading: { margin: '4px 0 0', display: 'flex', alignItems: 'center', gap: 10, fontSize: 15, fontWeight: 600, lineHeight: 1.5 },
  card: { borderRadius: 16, overflow: 'hidden' },
  top: { display: 'flex', alignItems: 'center', gap: 12, padding: '14px 16px' },
  copy: { flex: '1 1 auto', minWidth: 0, display: 'flex', flexDirection: 'column', gap: 3, lineHeight: 1.4 },
  name: { display: 'flex', alignItems: 'center', flexWrap: 'wrap', gap: 8, fontSize: 15, fontWeight: 600 },
  desc: { fontSize: 12.5, color: kit['--kit-fg-meta'], lineHeight: 1.5 },
  badge: { padding: '2px 7px', borderRadius: 6, fontSize: 11.5, fontWeight: 400, color: color['--color-ok'], backgroundColor: kit['--kit-ok-8'] },
  inherited: { padding: '2px 7px', borderRadius: 6, fontSize: 11.5, fontWeight: 400, color: kit['--kit-info-fg'], backgroundColor: kit['--kit-acc-8'] },
  button: { height: 32, padding: '0 12px', borderRadius: 9, borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-4'], backgroundColor: 'transparent', color: color['--color-fg'], fontSize: 12.5, display: 'flex', alignItems: 'center', gap: 6, whiteSpace: 'nowrap', cursor: 'pointer' },
  grid: { padding: '0 16px 16px' },
  inheritRow: { width: '100%', display: 'flex', alignItems: 'center', gap: 12, textAlign: 'left', marginBottom: 12 },
  rules: { marginTop: 8 },
})

export function StyleTab({ project, summary }: { project: string; summary: StorySummary }) {
  const story = summary.story
  const { user } = useSession()
  const canEdit = !!user && canWriteStory(user, `${project}/${story}`)
  const [open, setOpen] = useState(false)
  const config = useResource(keys.config('story', project, story), ({ signal }) => getConfig('story', project, story, signal))
  const presets = useResource(['config-presets'], ({ signal }) => getConfigPresets(signal), { staleTime: 60000 })
  const snapshot = useResource(keys.story(project, story, 'snapshot'), ({ signal }) => getSnapshot(project, story, signal))
  const samples = useResource(keys.story(project, story, 'style-samples'), ({ signal }) => getStyleSamples(project, story, signal))
  const own = config.data?.sources?.style === 'story'
  const effectiveStyle = config.data?.effective.style ?? 'default'
  const projectStyle = config.data?.chain.find((doc) => doc.scope === 'project')?.config.style ?? config.data?.chain.find((doc) => doc.scope === 'global')?.config.style ?? 'default'
  const pick = useAction(async (id: string | undefined) => {
    if (!config.data) return
    const next = await saveStyle('story', project, story, id, config.data)
    toast.ok(`Đã đổi văn phong · áp dụng từ chương ${effectiveFromChapter(snapshot.data)}`)
    return next
  }, { invalidate: [['project', project, 'style-by']] })
  if (config.initialLoading) return <Skeleton height={300} radius={16} />
  if (!config.data) return <section className="card st-sec"><span>Không tải được cách viết của truyện.</span><button type="button" {...stylex.props(writingStyles.button)} onClick={() => void config.refresh()}>Thử lại</button></section>
  const pack = findPack(presets.data?.packs, effectiveStyle)
  const desc = pack ? [packDesc(pack), packInfoMeta(pack)].filter(Boolean).join(' · ') : effectiveStyle === 'giong-tre' && samples.data ? `Học từ ${samples.data.total.count} văn mẫu của truyện này` : BUILT_IN.find((s) => s.id === effectiveStyle)?.desc
  return <div {...stylex.props(writingStyles.root)}>
    <p {...stylex.props(writingStyles.intro)}>{amWritingCopy.intro}</p>
    <h2 {...stylex.props(writingStyles.heading)}><Icon name="brush-2" size={17} />Văn phong</h2>
    <section className={`card ${stylex.props(writingStyles.card).className}`} aria-label="Văn phong">
      <div {...stylex.props(writingStyles.top)}><span {...stylex.props(writingStyles.copy)}>
        <span {...stylex.props(writingStyles.name)}>{styleDisplayName(presets.data, effectiveStyle)}{pack && packBadge(pack).ok && <span {...stylex.props(writingStyles.badge)}><Icon name="shield-tick" size={12} />Có chấm</span>}{!own && <span {...stylex.props(writingStyles.inherited)}>theo dự án</span>}</span>
        {desc && <span {...stylex.props(writingStyles.desc)}>{desc}</span>}
      </span><button type="button" {...stylex.props(writingStyles.button)} aria-expanded={open} onClick={() => setOpen(!open)}>{open ? 'Xong' : 'Đổi'}<Icon name="arrow-down-2" size={13} /></button></div>
      {open && <div {...stylex.props(writingStyles.grid)}>
        <button type="button" role="radio" aria-checked={!own} className={`st-style${!own ? ' on' : ''} ${stylex.props(writingStyles.inheritRow).className}`} disabled={!canEdit || pick.busy} onClick={() => void pick.run(undefined)}><Icon name="folder-open" size={17} /><span {...stylex.props(writingStyles.copy)}><b>Theo dự án · {styleDisplayName(presets.data, projectStyle)}</b><span {...stylex.props(writingStyles.desc)}>Dùng văn phong mặc định của dự án, đổi theo khi dự án đổi</span></span></button>
        <StylePicker value={own ? effectiveStyle : ''} presets={presets.data ?? null} custom="story" story samples={samples.data?.total ?? null} disabled={!canEdit || pick.busy} onPick={(id) => void pick.run(id)} />
      </div>}
      <StyleSamples project={project} story={story} canEdit={canEdit} />
    </section>
    {open && pack && <PackSummaryCard pack={pack} qualityHref={`#${paths.quality(project, story)}`} />}
    <div {...stylex.props(writingStyles.rules)}><RulesPanel scope="story" project={project} story={story} /></div>
  </div>
}
