// D13 · Chất lượng truyện (22-brief-2510.md › D13, artboard Quality.dc.html, GAP-42).
// Lưới `minmax(0,1fr) 380px`: cột giữa = tiêu đề + 3 nhóm nút (Cách xem · Mức · Phạm vi) + 5 ô số,
// rồi “Việc cần làm” hoặc “Ma trận”; panel phải = Tóm tắt / vùng đang chọn + “Mục hay vi phạm nhất”.
// Cách xem và bộ lọc nằm trong query (`view`, `level`, `scope`) để mở lại đúng trạng thái.
import { useEffect, useMemo, useState } from 'react'
import { addParagraphComment, deleteParagraphComment, keepReviewItem } from '../../api/client'
import type { QualityIssue } from '../../api/types'
import { invalidate, keys, useAction } from '../../data'
import { t } from '../../i18n/vi'
import { navigate, paths, useRoute } from '../../router'
import { EmptyState, Skeleton } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { toast } from '@noverse/ui'
import { useStoryAccess } from '../reader/useReaderData'
import { LEVELS, QualityControls, SCOPES, VIEWS } from './Controls'
import { qualityKey, useQuality } from './data'
import { Detail } from './Detail'
import { Matrix } from './Matrix'
import {
  filterIssues, initialPick, kpis, ruleLayers, summaryParts, todoGroups,
  type QualityLevelFilter, type QualityPick, type QualityScope, type QualityView,
} from './model'
import { Todo } from './Todo'
import './quality.css'
import { PanelRefreshBar } from '../../ui/PanelRefreshBar'
import { reviewFixPrefill } from '../../ui/ReviewIssue'

const pickOne = <T extends string>(value: string | null, options: readonly T[], fallback: T): T =>
  options.includes(value as T) ? (value as T) : fallback

/** Ý kiến “Nhờ sửa” gửi Người viết: dùng đúng câu điền sẵn của nút “Yêu cầu sửa” (ReviewIssue). */
const fixRequest = (issue: QualityIssue) => (issue.fix !== '' ? reviewFixPrefill(issue) : `${t.review.fixPrefill(issue.ref)}${issue.text}`)

export function QualityScreen({ project, story }: { project: string; story: string }) {
  const parsed = useRoute()
  const quality = useQuality(project, story)
  const access = useStoryAccess(project, story)
  const data = quality.data
  const view = pickOne(parsed.search.get('view'), VIEWS, 'todo')
  const level = pickOne(parsed.search.get('level'), LEVELS, 'all')
  const scope = pickOne(parsed.search.get('scope'), SCOPES, 'all')
  const [pick, setPick] = useState<QualityPick | null>(null)
  // Mục vừa “Nhờ sửa” / “Giữ nguyên” trong phiên: rời danh sách ngay như artboard (`done`).
  const [done, setDone] = useState<ReadonlySet<string>>(new Set())
  const [savingId, setSavingId] = useState('')

  useEffect(() => {
    if (data && pick === null) setPick(initialPick(data))
  }, [data, pick])

  const setQuery = (patch: Partial<Record<'view' | 'level' | 'scope', string>>) => {
    const next = new URLSearchParams(parsed.search)
    for (const [key, value] of Object.entries(patch)) next.set(key, value)
    navigate(`${paths.quality(project, story)}?${next.toString()}`, true)
  }

  const issues = useMemo(() => (data ? filterIssues(data.issues, level, scope) : []), [data, level, scope])
  const groups = useMemo(() => (data ? todoGroups(data, level, done) : { block: [], look: [], auto: [] }), [data, level, done])

  const markDone = (id: string, on: boolean) => setDone((current) => {
    const next = new Set(current)
    if (on) next.add(id)
    else next.delete(id)
    return next
  })

  const commentsKey = keys.story(project, story, 'comments')
  const comment = useAction(
    (issue: QualityIssue, text: string) => addParagraphComment(project, story, issue.ch, { paragraph: null, quote: issue.quote, chips: [], text }),
    {
      success: t.quality.fixSaved, invalidate: [commentsKey],
      undo: async (saved, [issue]) => {
        await deleteParagraphComment(project, story, issue.ch, saved.id)
        markDone(issue.id, false)
      },
    },
  )

  const askFix = async (issue: QualityIssue) => {
    setSavingId(issue.id)
    markDone(issue.id, true)
    const saved = await comment.run(issue, fixRequest(issue))
    if (!saved) markDone(issue.id, false)
    setSavingId('')
  }
  // GAP-47 — cùng luồng giữ nguyên với Trình đọc: lưu lý do vào quyết định thẩm duyệt của mục.
  const keep = async (issue: QualityIssue, reason: string) => {
    setSavingId(issue.id)
    try {
      await keepReviewItem(project, story, issue.id, reason)
      markDone(issue.id, true)
      toast(t.review.keepSaved)
      invalidate(qualityKey(project, story))
      return true
    } catch (error) {
      toast.err(error instanceof Error ? error.message : t.review.keepFailed)
      return false
    } finally {
      setSavingId('')
    }
  }

  if (quality.error && !data) {
    return (
      <div style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center', padding: 24 }}>
        <EmptyState icon="warning-2" title={t.quality.loadError}
          action={
            <button type="button" className="hov-ghost" onClick={() => void quality.refresh()}
              style={{ height: 32, padding: '0 12px', borderRadius: 8, border: '1px solid var(--line-4)', background: 'transparent', color: 'var(--color-fg)', fontSize: 13 }}>
              {t.quality.retry}
            </button>
          } />
      </div>
    )
  }

  if (!data || pick === null) {
    return (
      <div style={{ flex: 1, minWidth: 0, minHeight: 0, display: 'grid', gridTemplateColumns: 'minmax(0, 1fr) 380px' }}>
        <div style={{ padding: '18px 24px', display: 'flex', flexDirection: 'column', gap: 14 }}>
          <Skeleton width={320} height={56} radius={10} />
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(5, minmax(0, 1fr))', gap: 10 }}>
            {[0, 1, 2, 3, 4].map((key) => <Skeleton key={key} height={78} radius={14} />)}
          </div>
          <Skeleton height={360} radius={16} />
        </div>
        <div style={{ borderLeft: '1px solid var(--line-1)', padding: '18px 16px', display: 'flex', flexDirection: 'column', gap: 10 }}>
          {[0, 1, 2].map((key) => <Skeleton key={key} height={96} radius={14} />)}
        </div>
      </div>
    )
  }

  if (data.issues.length === 0 && data.chapters.length === 0) {
    return (
      <div style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center', padding: 24 }}>
        <EmptyState icon="chart-2" title={t.quality.empty} hint={t.quality.emptyHint} />
      </div>
    )
  }

  const k = kpis(data)
  const layers = ruleLayers(data).join(' + ')
  const kpiCards = [
    { label: t.quality.kpi.errors, value: String(k.all.e), sub: k.all.e ? t.quality.kpi.errorsSub(k.errorChapters) : t.quality.kpi.noErrors, color: k.all.e ? 'var(--kit-err-soft)' : 'var(--color-ok)' },
    { label: t.quality.kpi.warnings, value: String(k.all.w), sub: t.quality.kpi.warningsSub, color: 'var(--kit-warn-soft)' },
    { label: t.quality.kpi.fixed, value: String(k.all.f), sub: t.quality.kpi.fixedSub, color: 'var(--color-ok)' },
    { label: t.quality.kpi.kept, value: String(k.all.k), sub: t.quality.kpi.keptSub, color: 'var(--color-fg-2)' },
    { label: t.quality.kpi.passed, value: `${k.passed}/${k.total}`, sub: k.passed ? t.quality.kpi.passedSub(k.passedRange) : '', color: 'var(--color-fg)' },
  ]
  const todoN = groups.block.length + groups.look.length

  return (
    <div style={{ flex: 1, minWidth: 0, minHeight: 0, display: 'grid', gridTemplateColumns: 'minmax(0, 1fr) 380px', lineHeight: 1.5 }}>
      <main style={{ minWidth: 0, minHeight: 0, display: 'flex', flexDirection: 'column' }}>
        <PanelRefreshBar resourceKeys={[['story', project, story, 'reviews', 'quality']]}  />
        <div style={{ flex: '0 0 auto', padding: '18px 24px 12px', display: 'flex', flexDirection: 'column', gap: 14 }}>
          <div style={{ display: 'flex', alignItems: 'flex-end', gap: 16 }}>
            <div style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', gap: 4 }}>
              <a href={`#${paths.story(project, story)}`} style={{
                display: 'flex', alignItems: 'center', gap: 6, fontSize: 12.5, color: 'var(--kit-fg-icon)', minWidth: 0,
              }}>
                <Icon name="arrow-left-2" size={13} />
                <span title={access.storyTitle} style={{ whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{access.storyTitle}</span>
              </a>
              <h1 style={{ margin: 0, fontSize: 22, fontWeight: 600, letterSpacing: '-0.01em' }}>{t.quality.screenTitle}</h1>
              {data.pack && <span style={{ fontSize: 13, color: 'var(--color-fg-3)' }}>{t.quality.subtitle(data.pack.ten, layers)}</span>}
            </div>
            <QualityControls view={view} level={level} scope={scope} todo={todoN}
              onView={(next) => setQuery({ view: next })}
              onLevel={(next) => setQuery({ level: next })}
              onScope={(next) => setQuery({ scope: next })} />
          </div>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(5, minmax(0, 1fr))', gap: 10 }}>
            {kpiCards.map((card) => (
              <div key={card.label} className="card" style={{ borderRadius: 14, padding: '12px 14px', display: 'flex', flexDirection: 'column', gap: 2, minWidth: 0 }}>
                <span style={{ fontSize: 12, color: 'var(--color-fg-3)' }}>{card.label}</span>
                <span className="num" style={{ fontSize: 22, fontWeight: 600, color: card.color }}>{card.value}</span>
                <span title={card.sub} style={{
                  fontSize: 12, color: 'var(--color-fg-4)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
                }}>{card.sub}</span>
              </div>
            ))}
          </div>
        </div>
        <div role="tabpanel" id="quality-view-panel" aria-labelledby={`quality-view-${view}`}
          style={{ flex: '1 1 auto', minHeight: 0, display: 'flex', flexDirection: 'column' }}>
          {view === 'matrix' ? (
            <Matrix data={data} issues={issues} scope={scope} pick={pick} onPick={setPick} />
          ) : (
            <Todo groups={groups} project={project} story={story} writable={access.writable} savingId={savingId}
              onAskFix={(issue) => void askFix(issue)} onKeep={keep} />
          )}
        </div>
      </main>
      <Detail data={data} view={view} issues={issues} pick={pick} summary={summaryParts(data, groups)}
        project={project} story={story} onMatrix={() => setQuery({ view: 'matrix' })} />
    </div>
  )
}
