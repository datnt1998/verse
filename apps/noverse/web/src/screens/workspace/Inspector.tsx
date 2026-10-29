// Inspector — panel phải Workspace: tab Agent · Chi phí · Truyện (Main.dc.html).
// Dữ liệu thuần Snapshot (06-data-contract.md): agents, cachePerAgent/Model, totals,
// pendingRewrites, synopsis/premise/characters/recentSummaries.
import * as stylex from '@stylexjs/stylex'
import type { AgentSnapshot, AgentCacheStat, Snapshot } from '../../api/types'
import { t } from '../../i18n/vi'
import { translateHost } from '../../i18n/hostStrings'
import { enumLabel } from '../../i18n/enums'
import { agentLabel, agentProgressLine, humanizeToolText } from '../../lib/labels'
import { formatTokens, formatUSD } from '../../lib/format'
import { AgentOrb } from '@noverse/ui'
import type { AgentKind } from '@noverse/ui'
import { agentKindOf } from '../../ui/agentPalette'
import { Badge, CardTitle } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { Tabs } from '@noverse/ui'
import { EmptyState, ProgressBar, Skeleton } from '@noverse/ui'
import { StoryTab } from './StoryTab'
import { s } from './inspector.stylex'
import { cacheTone, contextTone, percentOf } from './format'

export type InspectorTab = 'agent' | 'cost' | 'story'


const AGENT_ROLES: AgentKind[] = ['architect', 'writer', 'editor', 'arbiter']
const activeState = (state: string) => state === 'running' || state === 'working'
const stateLabel = (state: string, paused: boolean): string => {
  if (paused && (state === 'running' || state === 'working')) return t.workspace.agentState.paused
  return enumLabel('agentState', state)
}

const statOf = (snapshot: Snapshot | null, name: string): AgentCacheStat | undefined =>
  snapshot?.cachePerAgent?.find((stat) => stat.role.toLowerCase() === name.toLowerCase())

function AgentCard({ agent, role, snapshot, running, paused }: {
  agent?: AgentSnapshot
  role: AgentKind
  snapshot: Snapshot
  running: boolean
  paused: boolean
}) {
  const stat = statOf(snapshot, agent?.name ?? role)
  const state = agent?.state.toLowerCase() ?? ''
  const active = activeState(state)
  const failed = state === 'failed'
  const idle = !agent || state === 'idle' || state === 'standby'
  const percent = Math.round(agent?.context.percent ?? 0)
  const scope = agent ? (t.ctxScope[agent.context.scope] ?? agent.context.scope) : ''
  const strategy = agent ? t.ctxStrategy[agent.context.strategy] : ''
  const kind = role
  // Main.dc.html › tab Trợ lý + 22-brief 27/10: thẻ trơn — không nền loang, không quầng;
  // dấu hiệu đang chạy chỉ là badge “Đang chạy” tím + orb “thở” (orb giữ bóng shade chuẩn).
  const live = active && running && !paused
  const dimmed = active && paused
  const name = agentLabel(role)
  const meta = stat ? `${formatTokens(stat.input + stat.output, 'long')} · ${formatUSD(stat.cost)}` : '—'
  // Gói P · P1: tóm tắt dạng điều phối “agent → tool[biến thể]” nói đúng việc đang làm
  // (vd “đang lưu dàn ý”); tóm tắt tự do giữ nhãn chung của agentProgressLine.
  const progress = agent ? (/→/.test(agent.summary)
    ? humanizeToolText(agent.summary)
    : agentProgressLine(agent.name, agent.tool ?? '', snapshot.inProgressChapter)) : ''
  const stateText = agent ? stateLabel(state, paused) : t.workspace.agentMissingData
  const card = stylex.props(s.agentCard(idle))
  const head = stylex.props(s.agentHead)
  const nameStyle = stylex.props(s.agentName)
  const metaStyle = stylex.props(s.agentMeta)
  const badgeStyle = stylex.props(s.agentBadge)
  const badgeText = stylex.props(s.agentBadgeText)
  const ctxRow = stylex.props(s.ctxRow)
  const ctxLabel = stylex.props(s.ctxLabel)
  const ctxTrack = stylex.props(s.ctxTrack)
  const ctxScope = stylex.props(s.ctxScope)
  const ctxStrategy = stylex.props(s.ctxStrategy)
  const ctxPct = stylex.props(s.ctxPct(contextTone(percent)))
  return (
    <div className={['card', card.className].filter(Boolean).join(' ')} style={card.style}>
      <div className={head.className} style={head.style}>
        <span style={{ flex: '0 0 auto', display: 'flex', ...(dimmed ? { filter: 'saturate(.25)', opacity: 0.75 } : {}) }}>
          <AgentOrb agent={kind} size={26} dim={idle} live={live} />
        </span>
        <div style={{ flex: 1, minWidth: 0 }}>
          <div {...nameStyle} title={name}>{name}</div>
          <div {...metaStyle} className={['num', metaStyle.className].filter(Boolean).join(' ')} title={meta}>{meta}</div>
        </div>
        <Badge tone={failed ? 'err' : dimmed ? 'warn' : active ? 'accent' : 'neutral'} title={stateText}
          fg={failed || dimmed || active ? undefined : 'var(--color-fg-4)'}
          style={badgeStyle.style}>
          <span {...badgeText}>{stateText}</span>
        </Badge>
      </div>
      {active && (
        <>
          <code title={progress} style={{
            fontFamily: 'inherit', fontSize: 12, color: 'var(--color-fg-2)', padding: '6px 8px', borderRadius: 7,
            background: 'var(--color-surface-0)', border: '1px solid var(--line-2)',
            whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis', display: 'block',
          }}>
            {progress}
          </code>
          <div {...ctxRow}>
            <span {...ctxLabel}>{t.workspace.context}</span>
            <div {...ctxTrack}><ProgressBar value={percent} height={3} tone="ok" style={{ background: 'var(--color-surface-5)' }} /></div>
            <span className={['num', 'mono', ctxPct.className].filter(Boolean).join(' ')} style={ctxPct.style}>{percent}%</span>
            <span {...ctxScope} title={`· ${scope}`}>· {scope}</span>
          </div>
          {/* 04-components › AgentCard: chỉ hiện `context.strategy` (bỏ dòng lặp lại summary của dòng tiến trình). */}
          {strategy && <div {...ctxStrategy} title={strategy}>{strategy}</div>}
        </>
      )}
    </div>
  )
}

function RewriteQueueCard({ snapshot, paused }: { snapshot: Snapshot; paused: boolean }) {
  const queue = snapshot.pendingRewrites ?? []
  if (queue.length === 0 && !snapshot.pendingSteer) return null
  const current = paused
    ? t.workspace.rewritePaused
    : snapshot.pendingSteer
      ? t.workspace.steerPending
      : t.workspace.rewritePending
  const reason = translateHost(snapshot.rewriteReason)
  return (
    <div className="card" style={{ borderRadius: 14, padding: 12, display: 'flex', flexDirection: 'column', gap: 10 }}>
      <CardTitle icon={<Icon name="edit-2" size={15} />}>{t.workspace.rewriteQueue}</CardTitle>
      {queue.length > 0 && (
        <div style={{ display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: 6 }}>
          <span style={{ fontSize: 12, color: 'var(--color-fg-4)', marginRight: 2 }}>{t.workspace.queue}</span>
          {queue.map((chapter) => {
            const active = chapter === snapshot.inProgressChapter
            return (
              <span key={chapter} className="num mono" style={{
                minWidth: 26, height: 24, padding: '0 6px', display: 'flex', alignItems: 'center',
                justifyContent: 'center', borderRadius: 6, fontSize: 12,
                background: active ? 'rgba(201,166,245,0.14)' : 'transparent',
                color: active ? 'var(--color-accent-soft)' : 'var(--color-fg-2)',
                border: `1px solid ${active ? 'rgba(201,166,245,0.4)' : 'var(--line-4)'}`,
              }}>{chapter}</span>
            )
          })}
        </div>
      )}
      <div style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>
        {t.workspace.reason} · <span {...stylex.props(s.rewriteValue)} title={reason}>{reason || '—'}</span>
      </div>
      <div style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>
        {t.workspace.current} · <span {...stylex.props(s.rewriteCurrent)} title={current}>{current}</span>
      </div>
    </div>
  )
}

function CostTab({ snapshot }: { snapshot: Snapshot }) {
  const roles = snapshot.cachePerAgent ?? []
  const models = snapshot.cachePerModel ?? []
  const totalCost = snapshot.totalCostUSD ?? 0
  const splitLabel = roles.length > 0
    ? roles.map((role) => `${agentLabel(role.role)} ${formatUSD(role.cost)}`).join(', ')
    : ''
  const costHead = stylex.props(s.costHead)
  const savedBadge = stylex.props(s.savedBadge)
  const row = stylex.props(s.row)
  const rowName = stylex.props(s.rowName)
  const rowMeta = stylex.props(s.rowMeta)
  const rowCost = stylex.props(s.rowCost)
  const cacheRow = stylex.props(s.cacheRow)
  const cacheHead = stylex.props(s.cacheHead)
  const cacheNames = stylex.props(s.cacheNames)
  const cacheDetail = stylex.props(s.cacheDetail)
  return (
    <>
      <div className="card" style={{ borderRadius: 14, padding: 14, display: 'flex', flexDirection: 'column', gap: 12 }}>
        <div {...costHead}>
          <span className="num" style={{ fontSize: 28, fontWeight: 600, letterSpacing: '-0.03em', lineHeight: 1 }}>{formatUSD(totalCost)}</span>
          <span className={['num', savedBadge.className].filter(Boolean).join(' ')} style={savedBadge.style}
            title={t.workspace.saved(formatUSD(snapshot.totalSavedUSD))}>
            {t.workspace.saved(formatUSD(snapshot.totalSavedUSD))}
          </span>
        </div>
        {roles.length > 0 && (
          <div role="img" aria-label={splitLabel} style={{ display: 'flex', gap: 2, height: 6, borderRadius: 3, overflow: 'hidden' }}>
            {roles.map((role) => (
              <div key={role.role} style={{ flex: Math.max(role.cost, 0.0001), background: `var(--orb-${agentKindOf(role.role)})` }} />
            ))}
          </div>
        )}
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(2, minmax(0, 1fr))', gap: 8 }}>
          <div>
            <div style={{ display: 'flex', alignItems: 'center', gap: 4, fontSize: 12, color: 'var(--color-fg-4)' }}>
              <Icon name="arrow-up" size={13} />{t.workspace.input}
            </div>
            <div className="num" style={{ fontSize: 14, fontWeight: 500 }}>{formatTokens(snapshot.totalInputTokens, 'long')}</div>
          </div>
          <div>
            <div style={{ display: 'flex', alignItems: 'center', gap: 4, fontSize: 12, color: 'var(--color-fg-4)' }}>
              <Icon name="arrow-down" size={13} />{t.workspace.output}
            </div>
            <div className="num" style={{ fontSize: 14, fontWeight: 500 }}>{formatTokens(snapshot.totalOutputTokens, 'long')}</div>
          </div>
        </div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 2, paddingTop: 8, borderTop: '1px solid var(--line-1)' }}>
          {roles.map((role) => (
            <div key={`role-${role.role}`} {...row}>
              <AgentOrb agent={agentKindOf(role.role)} size={14} />
              <span {...rowName} title={agentLabel(role.role)}>{agentLabel(role.role)}</span>
              <span className="num" style={rowMeta.style}>{formatTokens(role.input + role.output)}</span>
              <span className="num" style={rowCost.style}>{formatUSD(role.cost)}</span>
            </div>
          ))}
          {models.map((model) => (
            <div key={`model-${model.model}`} {...row}>
              <Icon name="cpu" size={14} style={{ color: 'var(--color-fg-4)' }} />
              <span className={['mono', rowName.className].filter(Boolean).join(' ')} style={rowName.style} title={model.model}>{model.model}</span>
              <span className="num" style={rowMeta.style}>{formatTokens(model.input + model.output)}</span>
              <span className="num" style={rowCost.style}>{formatUSD(model.cost)}</span>
            </div>
          ))}
        </div>
        {roles.length === 0 && models.length === 0 && <span style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>—</span>}
      </div>

      <div className="card" style={{ borderRadius: 14, padding: 14, display: 'flex', flexDirection: 'column', gap: 10 }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline' }}>
          <CardTitle icon={<Icon name="data" size={15} />}>{t.workspace.cache}</CardTitle>
          <span className="num" style={{ fontSize: 18, fontWeight: 600, color: 'var(--color-ok)' }}>
            {percentOf(
              snapshot.overallRecentSamples > 0 ? snapshot.overallRecentCacheRead : snapshot.totalCacheReadTokens,
              snapshot.overallRecentSamples > 0 ? snapshot.overallRecentInput : snapshot.totalInputTokens,
            )}%
          </span>
        </div>
        {roles.map((role) => {
          const percent = percentOf(role.cacheRead, role.input)
          return (
            <div key={`cache-${role.role}`} {...cacheRow}>
              <div {...cacheHead}>
                <span {...cacheNames}>
                  {agentLabel(role.role)} <span className="num" style={{ color: cacheTone(percent) }}>{percent}%</span>
                </span>
                <span className={['num', cacheDetail.className].filter(Boolean).join(' ')} style={cacheDetail.style}
                  title={`${formatTokens(role.cacheRead, 'long')} / ${formatTokens(role.input, 'long')}`}>
                  {formatTokens(role.cacheRead)} / {formatTokens(role.input)}
                </span>
              </div>
              <ProgressBar value={percent} height={3} tone={percent >= 50 ? 'ok' : percent >= 20 ? 'warn' : 'err'} />
            </div>
          )
        })}
        <div style={{
          display: 'grid', gridTemplateColumns: 'repeat(2, minmax(0, 1fr))', gap: '8px 12px',
          paddingTop: 8, borderTop: '1px solid var(--line-1)', fontSize: 12,
        }}>
          {[
            { key: 'recent', label: t.workspace.recent(snapshot.overallRecentSamples), value: `${percentOf(snapshot.overallRecentCacheRead, snapshot.overallRecentInput)}%`, color: 'var(--color-ok)' },
            { key: 'saved', label: t.workspace.savedLabel, value: formatUSD(snapshot.totalSavedUSD), color: 'var(--color-ok)' },
            { key: 'read', label: t.workspace.cacheRead, value: formatTokens(snapshot.totalCacheReadTokens, 'long'), color: 'var(--color-fg)' },
            { key: 'write', label: t.workspace.cacheWrite, value: snapshot.totalCacheWriteTokens > 0 ? formatTokens(snapshot.totalCacheWriteTokens, 'long') : t.workspace.cacheAuto, color: 'var(--color-fg-4)' },
            { key: 'breaks', label: t.workspace.cacheBreaks, value: t.workspace.times(snapshot.totalCacheBreaks), color: 'var(--color-warn)' },
          ].map((stat) => (
            <div key={stat.key}>
              <div style={{ color: 'var(--color-fg-4)' }}>{stat.label}</div>
              <div className="num" style={{ fontSize: 13, color: stat.color }}>{stat.value}</div>
            </div>
          ))}
        </div>
      </div>
    </>
  )
}

export function Inspector({ snapshot, project, story, tab, onTab, running, paused, mobile = false, loading = !snapshot }: {
  snapshot: Snapshot | null
  project: string
  story: string
  tab: InspectorTab
  onTab: (tab: InspectorTab) => void
  running: boolean
  paused: boolean
  /** mobile: chỉ render nội dung tab (thanh tab do màn dựng) */
  mobile?: boolean
  loading?: boolean
}) {
  const content = (() => {
    if (!snapshot && !loading) return <EmptyState icon="cpu" title={t.workspace.loadError} />
    if (!snapshot) return <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
      {[0, 1, 2].map((row) => <Skeleton key={row} height={132} radius={14} />)}
    </div>
    if (tab === 'agent') {
      // Host chỉ gửi các agent từng được quan sát. Không suy diễn “Chờ lệnh” hoặc
      // số liệu bằng 0 cho vai chưa có snapshot; vẫn giữ đủ bốn thẻ của artboard.
      const agents = new Map<AgentKind, AgentSnapshot>()
      for (const agent of snapshot.agents ?? []) {
        if (!/^(architect(?:_long|_short)?|writer|editor|arbiter)$/i.test(agent.name)) continue
        const role = agentKindOf(agent.name)
        const previous = agents.get(role)
        if (!previous || (activeState(agent.state.toLowerCase()) && !activeState(previous.state.toLowerCase()))
          || (activeState(agent.state.toLowerCase()) === activeState(previous.state.toLowerCase()) && agent.updatedAt > previous.updatedAt)) agents.set(role, agent)
      }
      const roles = [...AGENT_ROLES].sort((a, b) => Number(activeState(agents.get(b)?.state.toLowerCase() ?? '')) - Number(activeState(agents.get(a)?.state.toLowerCase() ?? '')))
      return (
        <>
          {roles.map((role) => (
            <AgentCard key={role} role={role} agent={agents.get(role)} snapshot={snapshot} running={running} paused={paused} />
          ))}
          <RewriteQueueCard snapshot={snapshot} paused={paused} />
        </>
      )
    }
    if (tab === 'cost') return <CostTab snapshot={snapshot} />
    return <StoryTab snapshot={snapshot} project={project} story={story} mobile={mobile} />
  })()

  const aside = stylex.props(s.aside(mobile))
  const scroll = stylex.props(s.scroll(mobile))
  return (
    <aside aria-label={t.nav.detailPanel} {...aside}>
      {!mobile && (
        <Tabs label={t.nav.detailPanel} value={tab} onChange={(id) => onTab(id as InspectorTab)} style={{ margin: '12px 12px 0' }}
          items={[
            { id: 'agent', label: t.workspace.tabs.agent, icon: <Icon name="magicpen" size={15} />, dot: running && tab !== 'agent' ? <AgentOrb agent="writer" size={7} live /> : undefined },
            { id: 'cost', label: t.workspace.tabs.cost, icon: <Icon name="coin-1" size={15} /> },
            { id: 'story', label: t.workspace.tabs.story, icon: <Icon name="book-1" size={15} /> },
          ]} />
      )}
      <div id={mobile ? undefined : 'workspace-inspector-scroll'}
        className={[mobile ? undefined : 'scroll', scroll.className].filter(Boolean).join(' ')}
        style={scroll.style}>
        {content}
      </div>
    </aside>
  )
}
