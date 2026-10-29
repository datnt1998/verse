// Cột phải (Outline.dc.html › aside 320 “Đang áp dụng”): thẻ áp dụng + thẻ Tình trạng.
import { getConfigPresets, getSnapshot, revealFolder } from '../../api/client'
import { useSession } from '../../api/session'
import type { ProjectDetail, StorySummary } from '../../api/types'
import { keys, useAction, useConnection, useResource } from '../../data'
import { Icon } from '@noverse/ui'
import { Spinner } from '@noverse/ui'
import { budgetPercent, usd } from '../more/mobile/storySetLogic'
import { appliedRows, useEffectiveConfig, useRuleCounts } from './applied'
import type { Health } from './health'
import { settingsLink } from './links'
import { useCanReveal } from './useProject'
import { paths, useRoute } from '../../router'
import { MetricsCard } from '../settings/sections/rules/MetricsCard'
import { findPack, packLabel } from '../settings/lib/packs'

const link = { fontSize: 12.5, color: '#C9A6F5', whiteSpace: 'nowrap' } as const

function AppliedCard({ project, story }: { project: string; story: StorySummary | null }) {
  const storyId = story?.story ?? null
  const config = useEffectiveConfig(project, storyId)
  const counts = useRuleCounts(project, storyId)
  const snapshot = useResource(keys.story(project, storyId ?? '', 'snapshot'), ({ signal }) => getSnapshot(project, storyId!, signal), { enabled: !!storyId })
  // D2 — gói thể loại: cùng khoá `config-presets` với Tuỳ chỉnh › Cách viết và Bắt đầu nhanh.
  const presets = useResource(['config-presets'], ({ signal }) => getConfigPresets(signal), { staleTime: 60000 })
  const rows = config ? appliedRows(config, counts, !!storyId, presets.data?.packs) : []
  const target = { project, story: storyId ?? undefined }
  const spent = snapshot.data?.totalCostUSD ?? 0
  const limit = config?.effective.budget?.book_usd ?? snapshot.data?.budgetLimitUSD ?? 0
  return (
    <section className="card" style={{ borderRadius: 16, padding: '14px 16px', display: 'flex', flexDirection: 'column', gap: 4 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8, paddingBottom: 6 }}>
        <span style={{ flex: '1 1 auto', fontSize: 13.5, fontWeight: 600 }}>{storyId ? 'Đang áp dụng cho truyện' : 'Mặc định của dự án'}</span>
        <a href={`#${settingsLink(undefined, target)}`} style={link}>Tuỳ chỉnh ›</a>
      </div>
      {rows.map((row) => {
        // Gói AL: Văn phong sửa ở tab Cách viết của truyện / mục Cách viết của dự án; Quy tắc tương tự.
        const href = row.section === 'rules'
          ? storyId ? `${paths.story(project, storyId)}?tab=rules` : `${paths.project(project)}?section=prules`
          : row.section === 'style'
            ? storyId ? `${paths.story(project, storyId)}?tab=style` : `${paths.project(project)}?section=pstyle`
            : settingsLink(row.section, target)
        return (
        <a key={row.label} className="pj-kv" href={`#${href}`}>
          <Icon name={row.icon} size={16} style={{ color: '#8E8E99' }} />
          <span style={{ flex: '1 1 auto', minWidth: 0, display: 'flex', flexDirection: 'column', lineHeight: 1.4 }}>
            <span style={{ fontSize: 12.5, color: '#8E8E99' }}>{row.label}</span>
            <span className="pj-kvv" style={{ fontSize: 13.5, color: '#D6D6DC' }}>{row.value}</span>
          </span>
          {row.own && <span className="pj-chip" style={{ background: 'rgba(201,166,245,0.1)', color: '#DCC8F8' }}>riêng</span>}
        </a>
        )
      })}
      {storyId && (
        <div style={{ paddingTop: 8, display: 'flex', flexDirection: 'column', gap: 6 }}>
          <span style={{ display: 'flex', justifyContent: 'space-between', fontSize: 12.5, color: '#8E8E99' }}>
            <span>Chi phí truyện</span>
            <span className="num" style={{ color: '#C9C9D1' }}>{usd(spent)} / {limit > 0 ? usd(limit) : '—'}</span>
          </span>
          <span style={{ height: 6, borderRadius: 999, background: 'rgba(255,255,255,0.07)', overflow: 'hidden' }}>
            <span style={{ display: 'block', height: '100%', width: budgetPercent(spent, limit), background: 'linear-gradient(90deg, #7C93F3, #9A63E3)' }} />
          </span>
        </div>
      )}
    </section>
  )
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
      {story && route.search.get('tab') === 'rules' && <MetricsCard project={project} story={story.story} packName={pack ? packLabel(pack) : ''} />}
      <HealthCard project={project} detail={detail} story={story} health={health} rootDisplay={rootDisplay} />
    </aside>
  )
}
