// Cột trái (Outline.dc.html › aside 264 · Gói AG): thẻ truyện + popover đổi truyện,
// Tổng quan dự án = menu dự án + danh sách truyện gọn; trong truyện = mục lục của đúng truyện đó.
// Đổi dự án chỉ ở thanh trên (ContextCrumb) — cột này không còn bộ chọn dự án.
import type { Outline, ProjectDetail, StorySummary } from '../../api/types'
import { Icon } from '@noverse/ui'
import { ProjectNav, StoryCard, StoryToc } from './nav'
import type { Health } from './health'

const HEALTH_TONE = {
  ok: { border: 'rgba(166,237,193,0.18)', background: 'transparent', color: '#A6EDC1', icon: 'shield-tick', text: 'Dự án đã sẵn sàng' },
  issues: { border: 'rgba(246,203,140,0.3)', background: 'rgba(246,203,140,0.06)', color: '#F6CB8C', icon: 'warning-2' },
} as const

export function ProjectTree({ project, detail, activeStory, outline, health, narrow }: {
  project: string
  detail: ProjectDetail
  /** truyện đang mở theo route (null ở màn tổng quan) */
  activeStory: StorySummary | null
  /** đại cương của truyện đang mở (màn truyện) */
  outline: Outline | null
  health: Health
  narrow: boolean
}) {
  const storyView = !!activeStory
  return (
    <aside aria-label="Truyện và mục lục" style={{
      // minWidth 0: nội dung nowrap (tên truyện dài) không được phình cột quá 264 của artboard — cắt bằng “…”.
      flex: narrow ? '0 0 auto' : '0 0 264px', minWidth: 0, minHeight: 0, padding: '10px 10px 12px',
      borderRight: narrow ? 'none' : '1px solid rgba(255,255,255,0.06)',
      display: 'flex', flexDirection: 'column', gap: 8,
    }}>
      <StoryCard project={project} detail={detail} active={activeStory} storyView={storyView} />
      {storyView && activeStory
        ? <StoryToc project={project} summary={activeStory} outline={outline} />
        : <ProjectNav project={project} detail={detail} />}
      {(health.state === 'ok' || health.state === 'issues') && (
        <div style={{ flex: '0 0 auto', padding: '4px 0 0' }}>
          {(() => {
            const tone = HEALTH_TONE[health.state]
            const text = health.state === 'ok' ? HEALTH_TONE.ok.text : `Dự án có ${health.issues.length} việc cần sửa`
            return (
              <button className="pj-sb" type="button"
                onClick={() => document.getElementById('health')?.scrollIntoView({ block: 'nearest', behavior: 'smooth' })}
                style={{ border: `1px solid ${tone.border}`, background: tone.background, color: tone.color, borderRadius: 11 }}>
                <Icon name={tone.icon} size={16} />
                <span style={{ flex: '1 1 auto' }}>{text}</span>
              </button>
            )
          })()}
        </div>
      )}
    </aside>
  )
}
