// ActivityFeed — panel “Nhật ký tiến độ” trong Workspace (Main.dc.html).
// Sự kiện upsert theo id ở hook SSE; ở đây chỉ cắt N dòng gần nhất và bám đáy.
import type { ReactNode } from 'react'
import * as stylex from '@stylexjs/stylex'
import { t } from '../../i18n/vi'
import { paths } from '../../router'
import type { HostEvent } from '../../api/types'
import { Icon } from '@noverse/ui'
import { Scroll } from '@noverse/ui'
import { EmptyState, Skeleton } from '@noverse/ui'
import { useNow } from './hooks'
import { ActivityRow } from './ActivityRow'
import { s } from './activityFeed.stylex'
import { activityEmpty } from './events'
import { deduplicateUserEvents, isInternalEvent } from '../../lib/activity'

export function ActivityFeed({ events, project, story, loading = false, mobile = false, planning = false, idle = false, question = '', results, total }: {
  events: HostEvent[]
  project: string
  story: string
  loading?: boolean
  mobile?: boolean
  /** Số chương của truyện — chặn số chương ngoài phạm vi trong tóm tắt công cụ. */
  total?: number | null
  /** Gói P · P1: giai đoạn lập kế hoạch — nhật ký flex 3, khung kế hoạch flex 7. */
  planning?: boolean
  /** Gói P · P4: khung luồng đang hiện thẻ trạng thái rảnh — chia 5/5. */
  idle?: boolean
  /** Gói P · P4: câu hỏi Trọng tài đang chờ — dòng nhật ký cuối tô cam. */
  question?: string
  results?: ReactNode
}) {
  const now = useNow(1000)
  const rows = deduplicateUserEvents(events.filter((event) => !isInternalEvent(event))).slice(-40)
  const empty = activityEmpty(events.length, rows.length)
  return (
    <section className="card" aria-label={t.workspace.activity} style={{
      flex: mobile ? '1 1 0' : idle ? '5 1 0' : planning ? '3 1 0' : '4 1 0', minHeight: 0, display: 'flex', flexDirection: 'column',
      borderRadius: 14, padding: '8px 6px 4px',
    }}>
      <div style={{ flex: '0 0 auto', display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '0 10px 6px' }}>
        <h2 style={{ margin: 0, display: 'flex', alignItems: 'center', gap: 8, fontSize: 13, fontWeight: 500, color: 'var(--color-fg-3)' }}>
          <Icon name="activity" size={16} />
          {t.workspace.activity}
        </h2>
        <a href={`#${paths.activity(project, story)}`} style={{ display: 'flex', alignItems: 'center', gap: 4, fontSize: 12, color: 'var(--color-fg-3)' }}>
          {t.workspace.activityAll}
          <Icon name="arrow-right-2" size={13} />
        </a>
      </div>
      {/* Gói O mục 2: mờ 16 px mép trên vùng cuộn — dòng đầu không nằm hẳn dưới tiêu đề. */}
      <Scroll stickToBottom className={stylex.props(s.logFade).className}>
        <ol style={{ listStyle: 'none', margin: 0, padding: 0, display: 'flex', flexDirection: 'column' }}>
          {loading && rows.length === 0
            ? [0, 1, 2, 3].map((row) => (
              <li key={row} style={{ padding: '5px 10px' }}><Skeleton height={14} width={row % 2 ? '62%' : '82%'} /></li>
            ))
            : rows.map((event, index) => (
              <ActivityRow key={event.id || `row-${index}`} event={event} now={now} compact total={total}
                highlight={index === rows.length - 1 && question.length > 0 && event.summary.includes(question)} />
            ))}
        </ol>
        {results && <div style={{ padding: '10px 10px 14px' }}>{results}</div>}
        {!loading && empty === 'noActivity' && (
          <EmptyState icon="activity" title={t.empty.noActivity} hint={t.empty.noActivityHint} />
        )}
        {!loading && empty === 'filtered' && <EmptyState icon="activity" title={t.empty.events} />}
      </Scroll>
    </section>
  )
}
