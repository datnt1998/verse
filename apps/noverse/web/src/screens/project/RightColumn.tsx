// Cột phải (Outline.dc.html › aside 320 “Đang áp dụng”): thẻ áp dụng + thẻ Tình trạng.
import { useEffect, useRef, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { color, kit, line } from '@noverse/ui/tokens.stylex'
import { canManageProject, canWriteStory, getConfigPresets, getSnapshot, getStyleSamples, revealFolder, setReview } from '../../api/client'
import { useSession } from '../../api/session'
import type { Config, ProjectDetail, StorySummary } from '../../api/types'
import { keys, useAction, useConnection, useResource } from '../../data'
import { Icon, Switch, toast, useEscapeLayer } from '@noverse/ui'
import { Spinner } from '@noverse/ui'
import { budgetPercent, usd } from '../more/mobile/storySetLogic'
import { appliedRows, saveQuickConfig, useEffectiveConfig, useRuleCounts } from './applied'
import type { Health } from './health'
import { settingsLink } from './links'
import { useCanReveal } from './useProject'
import { paths, useRoute } from '../../router'
import { MetricsCard } from '../settings/sections/rules/MetricsCard'
import { findPack, packLabel } from '../settings/lib/packs'
import { StyleQuickPicker, WriterQuickPicker } from '../settings/sections/quick-pickers'
import { saveStyle, styleDisplayName, writerConfig } from '../settings/sections/style'
import { effectiveFromChapter } from '../settings/lib/labels'
import { mergeConfig, modelOptions, type ModelOption } from '../settings/lib/config'

const quickStyles = stylex.create({
  row: { width: '100%', display: 'flex', alignItems: 'center', gap: 10, padding: '9px 6px', borderWidth: 0, borderRadius: 0, backgroundColor: 'transparent', color: color['--color-fg'], textAlign: 'left', cursor: 'pointer', ':hover': { backgroundColor: kit['--kit-acc-8'] } },
  pop: { position: 'absolute', right: 0, top: '100%', zIndex: 30, width: 300, maxWidth: 'calc(100vw - 32px)', borderRadius: 14, borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-4'], backgroundColor: color['--color-surface-4'], boxShadow: kit['--kit-shadow-pop'] },
  tag: { fontSize: 11, color: kit['--kit-info-fg'], backgroundColor: kit['--kit-acc-8'], borderRadius: 6, padding: '2px 6px', whiteSpace: 'nowrap' },
})


function AppliedCard({ project, story }: { project: string; story: StorySummary | null }) {
  const storyId = story?.story ?? null
  const scope = storyId ? 'story' : 'project'
  const { user } = useSession()
  const connection = useConnection()
  const canEdit = connection.writable && !!user && (storyId ? canWriteStory(user, `${project}/${storyId}`) : canManageProject(user, project))
  const config = useEffectiveConfig(project, storyId)
  const counts = useRuleCounts(project, storyId)
  const snapshot = useResource(keys.story(project, storyId ?? '', 'snapshot'), ({ signal }) => getSnapshot(project, storyId!, signal), { enabled: !!storyId })
  const presets = useResource(['config-presets'], ({ signal }) => getConfigPresets(signal), { staleTime: 60000 })
  const samples = useResource(keys.story(project, storyId ?? '', 'style-samples'), ({ signal }) => getStyleSamples(project, storyId!, signal), { enabled: !!storyId })
  const [open, setOpen] = useState<'style' | 'roles' | null>(null)
  const root = useRef<HTMLElement>(null)
  useEscapeLayer(!!open, () => setOpen(null))
  useEffect(() => {
    if (!open) return
    const outside = (e: PointerEvent) => { if (!root.current?.contains(e.target as Node)) setOpen(null) }
    document.addEventListener('pointerdown', outside)
    return () => document.removeEventListener('pointerdown', outside)
  }, [open])
  const chapter = effectiveFromChapter(snapshot.data)
  const pickStyle = useAction(async (id: string | null) => {
    if (!config) return
    await saveStyle(scope, project, storyId ?? undefined, id ?? undefined, config)
    setOpen(null)
    toast.ok(storyId ? `Đã đổi văn phong · áp dụng từ chương ${chapter}` : 'Đã đổi văn phong mặc định của dự án · truyện mới sẽ dùng')
  }, { invalidate: [['project', project, 'style-by']] })
  const pickWriter = useAction(async (model: ModelOption | null) => {
    if (!config || !storyId) return
    const own = writerConfig(config.target.config, model)
    await saveQuickConfig('story', project, storyId, { ...config, target: { ...config.target, config: own } }, {})
    setOpen(null)
    toast.ok(model ? `Người viết dùng ${model.model} từ chương ${chapter}` : `Người viết dùng theo dự án từ chương ${chapter}`)
  })
  const toggle = useAction(async (on: boolean) => {
    if (!config) return
    if (storyId) await setReview(project, storyId, on)
    else await saveQuickConfig('project', project, undefined, config, { advance_mode: on ? 'review' : 'auto' })
    toast.ok(storyId ? on ? 'Đã bật duyệt từng chương · dừng sau chương đang viết' : 'Đã tắt duyệt từng chương · trợ lý viết liền' : on ? 'Đã bật duyệt cho truyện mới' : 'Đã tắt duyệt cho truyện mới')
  }, { invalidate: [keys.config(scope, project, storyId ?? undefined), keys.story(project, storyId ?? '', 'snapshot')] })
  const rows = config ? appliedRows(config, counts, !!storyId, presets.data?.packs) : []
  const target = { project, story: storyId ?? undefined }
  const writingHref = storyId ? `${paths.story(project, storyId)}?tab=style` : `${paths.project(project)}?section=pstyle`
  const parent = config?.chain.filter((d) => d.scope !== scope).reduce<Config>((acc, d) => mergeConfig(acc, d.config), {})
  const projectStyle = parent?.style ?? 'default'
  const writer = config?.effective.roles?.writer ?? config?.effective.roles?.default
  const parentWriter = parent?.roles?.writer ?? parent?.roles?.default
  const review = (storyId ? snapshot.data?.advanceMode ?? config?.effective.advance_mode : config?.effective.advance_mode) === 'review'
  const busy = !canEdit || pickStyle.busy || pickWriter.busy || toggle.busy
  const spent = snapshot.data?.totalCostUSD ?? 0
  const limit = config?.effective.budget?.book_usd ?? snapshot.data?.budgetLimitUSD ?? 0
  return <section ref={root} className="card" style={{ borderRadius: 16, padding: '14px 16px', display: 'flex', flexDirection: 'column', gap: 4 }}>
    <div style={{ display: 'flex', alignItems: 'center', gap: 8, paddingBottom: 6 }}><span style={{ flex: 1, fontSize: 13.5, fontWeight: 600 }}>Đang áp dụng</span><a href={`#${settingsLink(undefined, target)}`} style={{ fontSize: 12.5, color: 'var(--color-fg-4)' }}>Tuỳ chỉnh ›</a></div>
    {rows.map((row) => {
      const quick = row.section === 'style' || storyId && row.section === 'roles'
      const sw = row.section === 'writing'
      const value = row.section === 'style' ? styleDisplayName(presets.data, config?.effective.style) : sw ? review ? storyId ? 'Bật · dừng sau mỗi chương' : 'Bật cho truyện mới' : storyId ? 'Tắt · viết liền' : 'Tắt cho truyện mới' : row.value
      const content = <><Icon name={row.section === 'style' ? 'brush-2' : row.icon} size={16} style={{ color: 'var(--color-fg-4)' }} /><span style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', lineHeight: 1.4 }}><span style={{ fontSize: 12.5, color: 'var(--color-fg-4)' }}>{!storyId && sw ? 'Duyệt cho truyện mới' : row.label}</span><span className="pj-kvv" title={value} style={{ fontSize: 13.5, color: 'var(--color-fg-2)' }}>{value}</span></span>{storyId && row.section === 'style' && !row.own && <span {...stylex.props(quickStyles.tag)}>theo dự án</span>}</>
      return <div key={row.section} style={{ position: 'relative' }}>
        {sw ? <div {...stylex.props(quickStyles.row)}>{content}<Switch checked={review} label={storyId ? 'Duyệt từng chương' : 'Duyệt cho truyện mới'} disabled={busy} onChange={() => void toggle.run(!review)} /></div>
          : quick ? <button type="button" {...stylex.props(quickStyles.row)} aria-haspopup="listbox" aria-expanded={open === row.section} disabled={busy} onClick={() => setOpen(open === row.section ? null : row.section as 'style' | 'roles')}>{content}<Icon name="arrow-down-2" size={13} /></button>
          : <a {...stylex.props(quickStyles.row)} href={`#${row.section === 'rules' ? writingHref : settingsLink(row.section, target)}`}>{content}<Icon name="arrow-right-2" size={13} /></a>}
        {open === row.section && <div {...stylex.props(quickStyles.pop)}>{open === 'style'
          ? <StyleQuickPicker value={row.own ? config?.effective.style ?? 'default' : null} projectStyle={projectStyle} inheritance={storyId ? 'project' : 'global'} presets={presets.data ?? null} samples={samples.data?.total} disabled={busy} onPick={(id) => void pickStyle.run(id)} footer={<a href={`#${writingHref}`} onClick={() => setOpen(null)}>{storyId ? 'Văn mẫu và chi tiết gói ở tab Cách viết ›' : 'Xem truyện nào chọn riêng ›'}</a>} />
          : <WriterQuickPicker value={config?.target.config.roles?.writer ? { provider: writer?.provider ?? '', model: writer?.model ?? '' } : null} projectModel={parentWriter?.model ?? parent?.model ?? ''} models={modelOptions(config?.effective ?? {})} presets={presets.data} disabled={busy} onPick={(model) => void pickWriter.run(model)} footer={<a href={`#${settingsLink('roles', target)}`}>Mức suy nghĩ, ngữ cảnh, model dự phòng ở Tuỳ chỉnh ›</a>} />}</div>}
      </div>
    })}
    {storyId && <div style={{ paddingTop: 8, display: 'flex', flexDirection: 'column', gap: 6 }}><span style={{ display: 'flex', justifyContent: 'space-between', fontSize: 12.5, color: 'var(--color-fg-4)' }}><span>Chi phí truyện</span><span>{usd(spent)} / {limit > 0 ? usd(limit) : '—'}</span></span><span style={{ height: 6, borderRadius: 999, background: 'var(--line-1)', overflow: 'hidden' }}><span style={{ display: 'block', height: '100%', width: budgetPercent(spent, limit), background: 'var(--color-accent)' }} /></span></div>}
  </section>
}

function HealthCard({ project, detail, story, health, rootDisplay }: {
  project: string; detail: ProjectDetail; story: StorySummary | null; health: Health; rootDisplay: string | null
}) {
  const { user } = useSession()
  const connection = useConnection()
  const canReveal = useCanReveal()
  const reveal = useAction(() => revealFolder(story ? { target: 'story', project, story: story.story } : { target: 'project', project }), { success: 'Đã mở thư mục' })
  const bad = health.state === 'issues'
  const failed = health.state === 'error'
  const accent = bad || failed ? '#F6CB8C' : '#A6EDC1'
  const title = failed ? 'Chưa kiểm tra được' : bad ? `${health.issues.length} việc cần sửa` : 'Mọi thứ đã sẵn sàng'
  const text = failed ? health.errorText
    : bad ? (health.canFix ? 'Dự án được tạo hoặc chép từ ngoài Noverse nên thiếu vài phần. Noverse sửa được ngay:' : '')
      : health.checked ? 'Vừa kiểm tra xong · không thiếu gì.' : 'Noverse kiểm tra tự động mỗi khi mở dự án.'
  const techPath = [rootDisplay, story ? story.folder : detail.project.path].filter(Boolean).join('/')
  const techCodes = health.raw.length > 0 ? [...new Set(health.raw.map((issue) => issue.code))].join(' · ') : 'validate: ok · 0 vấn đề'
  const busy = health.fixing || health.checking || !connection.writable
  return (
    <section id="health" className="card" style={{
      borderRadius: 16, padding: '14px 16px', display: 'flex', flexDirection: 'column', gap: 10,
      borderColor: bad || failed ? 'rgba(246,203,140,0.3)' : 'rgba(255,255,255,0.07)',
    }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
        <Icon name={bad || failed ? 'warning-2' : 'shield-tick'} size={18} style={{ color: accent }} />
        <span style={{ flex: '1 1 auto', fontSize: 13.5, fontWeight: 600 }}>{health.state === 'loading' ? 'Đang kiểm tra…' : title}</span>
      </div>
      {text && <span style={{ fontSize: 12.5, color: '#8E8E99', lineHeight: 1.5 }}>{text}</span>}
      {bad && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
          {health.issues.map((issue) => (
            <div key={issue.key} style={{ display: 'flex', gap: 8, fontSize: 13, color: '#E9D9BD', lineHeight: 1.5 }}>
              <span style={{ width: 6, height: 6, borderRadius: '50%', background: '#F6CB8C', marginTop: 7, flex: '0 0 auto' }} />
              {issue.text}
            </div>
          ))}
          {health.canFix && (
            <>
              <button className="primary hov-primary" type="button" onClick={() => void health.fix()} disabled={busy} style={{
                height: 34, borderRadius: 10, border: 'none', background: '#EDEDEF', color: '#0B0B0D', fontSize: 13, fontWeight: 600,
                display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 8,
              }}>
                {health.fixing && <Spinner size={13} color="#0B0B0D" style={{ borderColor: 'rgba(0,0,0,0.15)', borderTopColor: '#0B0B0D' }} />}
                {health.fixing ? 'Đang sửa…' : 'Sửa tự động'}
              </button>
              <span style={{ fontSize: 12, color: '#7E7E8A' }}>Chỉ thêm phần còn thiếu, không xoá hay sửa nội dung truyện.</span>
            </>
          )}
        </div>
      )}
      {(health.state === 'ok' || failed) && (
        <button className="hov-ghost" type="button" onClick={() => void health.recheck()} disabled={health.checking} style={{
          alignSelf: 'flex-start', height: 28, padding: '0 10px', borderRadius: 8, border: '1px solid rgba(255,255,255,0.1)',
          background: 'transparent', color: '#C9C9D1', fontSize: 12, display: 'flex', alignItems: 'center', gap: 6,
        }}>
          <Icon name="refresh-2" size={13} />Kiểm tra lại
        </button>
      )}
      {user?.role === 'admin' && (
        <details style={{ borderTop: '1px solid rgba(255,255,255,0.06)', paddingTop: 8 }}>
          <summary style={{ cursor: 'pointer', fontSize: 12, color: '#7E7E8A', display: 'flex', alignItems: 'center', gap: 6 }}>
            <Icon name="crown" size={12} style={{ color: '#B89A6E' }} />Chi tiết cho quản trị
          </summary>
          <div style={{ paddingTop: 8, display: 'flex', flexDirection: 'column', gap: 6, fontSize: 11.5, color: '#8E8E99' }}>
            <span style={{ overflowWrap: 'anywhere' }}>{techPath}</span>
            <span style={{ overflowWrap: 'anywhere' }}>{techCodes}</span>
            {canReveal && (
              <button type="button" onClick={() => void reveal.run()} disabled={reveal.busy} style={{
                alignSelf: 'flex-start', padding: 0, border: 'none', background: 'transparent', cursor: 'pointer',
                fontFamily: "'Be Vietnam Pro', sans-serif", fontSize: 12, color: '#C9A6F5',
              }}>Mở thư mục trên máy chủ ›</button>
            )}
          </div>
        </details>
      )}
    </section>
  )
}

export function RightColumn({ project, detail, story, health, rootDisplay, narrow }: {
  project: string
  detail: ProjectDetail
  story: StorySummary | null
  health: Health
  rootDisplay: string | null
  narrow: boolean
}) {
  const route = useRoute()
  const config = useEffectiveConfig(project, story?.story ?? null)
  const presets = useResource(['config-presets'], ({ signal }) => getConfigPresets(signal), { staleTime: 60000 })
  const pack = findPack(presets.data?.packs, config?.effective.style)
  return (
    <aside aria-label="Đang áp dụng" className="scroll" style={{
      flex: narrow ? '0 0 auto' : '0 0 320px', padding: '18px 16px 20px', borderLeft: narrow ? 'none' : '1px solid rgba(255,255,255,0.06)',
      display: 'flex', flexDirection: 'column', gap: 12,
    }}>
      <AppliedCard project={project} story={story} />
      {story && ['style', 'rules'].includes(route.search.get('tab') ?? '') && <MetricsCard project={project} story={story.story} packName={pack ? packLabel(pack) : ''} />}
      {health.supported && <HealthCard project={project} detail={detail} story={story} health={health} rootDisplay={rootDisplay} />}
    </aside>
  )
}
