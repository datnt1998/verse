// OutlineRail — cột Đại cương (Main.dc.html): tiến độ, danh sách chương (tick ok ·
// orb writer live · refresh chờ viết lại), số chữ + bản nộp mới nhất.
// Nguồn: GET /outline (trạng thái từng chương) hợp nhất snapshot (hàng đợi, đang chạy).
// Gói P · P1: giai đoạn lập kế hoạch — “Đang lập dàn ý · n/N”, chương đã lưu hiện tên,
// chương chưa lưu là thanh chờ; “Mới nhất” ghi “Chưa có chương”.
import type { ChapterInfo, Outline, PlanStepDetail, Snapshot } from '../../api/types'
import { t } from '../../i18n/vi'
import { translateHost } from '../../i18n/hostStrings'
import { paths } from '../../router'
import { AgentOrb } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { EmptyState, ProgressBar, Skeleton, Spinner } from '@noverse/ui'
import { formatNumber } from '../../lib/format'

type RailItem = { chapter: number; title: string; hint: string; done: boolean; live: boolean; queued: boolean; waiting?: boolean; askWaiting?: boolean }

/** Chi tiết bước “Dàn ý” của khung kế hoạch (lập kế hoạch mới có). */
export function planOutlineOf(snapshot: Snapshot | null | undefined): Extract<PlanStepDetail, { kind: 'outline' }> | null {
  const detail = snapshot?.plan?.steps.find((step) => step.key === 'outline')?.detail
  return detail?.kind === 'outline' ? detail : null
}

export function outlineItems(snapshot: Snapshot | null, outline: Outline | null): RailItem[] {
  // Gói P · P1: lập kế hoạch — lưới chương lấy từ bước “Dàn ý” của plan.
  const planOutline = snapshot?.plan ? planOutlineOf(snapshot) : null
  if (snapshot?.plan && planOutline !== null) {
    const saved = new Map(planOutline.chapters.map((c) => [c.chapter, c]))
    const items: RailItem[] = []
    for (let n = 1; n <= Math.max(planOutline.total, planOutline.saved); n++) {
      const chapter = saved.get(n)
      items.push({
        chapter: n,
        title: chapter?.title ?? '',
        hint: chapter?.hook ?? '',
        done: false,
        live: false,
        queued: false,
        waiting: !chapter,
      })
    }
    return items
  }

  const inProgress = snapshot?.inProgressChapter ?? 0
  const running = !!snapshot?.isRunning
  const queue = snapshot?.pendingRewrites ?? []
  // Gói P · P4: chương Trọng tài đang chờ người dùng trả lời (hoặc chương của lần
  // tạm dừng theo chương) — tô cam nhạt trong Đại cương.
  const askChapter = snapshot?.pendingAsk?.chapter || snapshot?.advanceHoldChapter || 0
  if (outline) {
    return outline.volumes.flatMap((volume) => volume.chapters).map((chapter) => ({
      chapter: chapter.chapter,
      title: chapter.title,
      hint: chapter.coreEvent || chapter.summary,
      done: chapter.completed || chapter.status === 'done',
      live: running && chapter.chapter === inProgress,
      queued: chapter.status === 'rewrite' || queue.includes(chapter.chapter),
      askWaiting: chapter.chapter === askChapter,
    }))
  }
  const completed = snapshot?.completedCount ?? 0
  return (snapshot?.outline ?? []).map((item) => ({
    chapter: item.chapter,
    title: item.title,
    hint: item.coreEvent,
    done: item.chapter <= completed,
    live: running && item.chapter === inProgress,
    queued: queue.includes(item.chapter),
    askWaiting: item.chapter === askChapter,
  }))
}

export function chapterFooter(snapshot: Snapshot | null, chapters?: readonly ChapterInfo[]) {
  if (snapshot) return { words: snapshot.totalWordCount, latest: translateHost(snapshot.lastCommitSummary.trim()) }
  if (!chapters) return { words: null, latest: '' }
  let words = 0
  let latest: ChapterInfo | undefined
  for (const chapter of chapters) {
    words += chapter.words
    if (chapter.completed && (!latest || chapter.chapter > latest.chapter)) latest = chapter
  }
  return { words, latest: latest ? `Chương ${latest.chapter} · ${formatNumber(latest.words)} chữ` : '' }
}

const LIVE_LABEL: Record<string, string> = {
  writing: t.status.chapterWriting,
  rewriting: t.status.chapterRewriting,
  polishing: t.workspace.polishing,
}

export function OutlineRail({ snapshot, outline, chapters, project, story, loading, error, onRetry }: {
  snapshot: Snapshot | null
  outline: Outline | null
  chapters?: readonly ChapterInfo[]
  project: string
  story: string
  loading: boolean
  error: boolean
  onRetry: () => void
}) {
  const items = outlineItems(snapshot, outline)
  const planning = !!snapshot?.plan
  const planOutline = planning ? planOutlineOf(snapshot) : null
  const done = items.length > 0 ? items.filter((item) => item.done).length : snapshot?.completedCount ?? 0
  const total = items.length || snapshot?.totalChapters || 0
  const percent = total > 0 ? (done / total) * 100 : 0
  const liveLabel = LIVE_LABEL[snapshot?.flow ?? 'writing'] ?? t.status.chapterWriting
  const footer = chapterFooter(snapshot, chapters)
  // “Mới nhất” ở giai đoạn lập kế hoạch ghi “Chưa có chương”.
  const latest = planning ? t.plan.noChapter : footer.latest
  return (
    <aside className="scroll" aria-label={t.nav.outline} style={{
      flex: '0 0 236px', maxWidth: 236, minHeight: 0, padding: '14px 10px',
      display: 'flex', flexDirection: 'column', gap: 12, borderRight: '1px solid var(--line-1)',
    }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', padding: '0 8px' }}>
        <h2 style={{ margin: 0, display: 'flex', alignItems: 'center', gap: 8, fontSize: 13, fontWeight: 500, color: 'var(--color-fg-3)' }}>
          <Icon name="note-text" size={16} />
          {t.nav.outline}
        </h2>
        <span className="num" style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>{done} / {total || '—'}</span>
      </div>
      <div style={{ padding: '0 8px', display: 'flex', flexDirection: 'column', gap: 6 }}>
        {/* Main.dc.html: giai đoạn lập kế hoạch vẫn có thanh tiến độ; dòng “Đang lập dàn ý · n/N” ở dưới. */}
        <ProgressBar value={percent} />
        {planning && (
          <div role="status" style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 12, color: 'var(--color-info)' }}>
            {snapshot?.isRunning && <Spinner size={11} color="var(--color-info)" />}
            {planOutline ? t.plan.railDrafting(planOutline.saved, planOutline.total) : '—'}
          </div>
        )}
      </div>

      {loading && items.length === 0 && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8, padding: '0 8px' }}>
          {[0, 1, 2, 3, 4, 5].map((row) => <Skeleton key={row} height={18} width={`${88 - row * 7}%`} />)}
        </div>
      )}
      {error && items.length === 0 && (
        <div style={{ padding: '4px 8px', display: 'flex', flexDirection: 'column', gap: 8 }}>
          <span style={{ fontSize: 12.5, color: 'var(--color-err)' }}>{t.workspace.outlineError}</span>
          <button type="button" className="hov-ghost" onClick={onRetry} style={{
            alignSelf: 'flex-start', height: 28, padding: '0 10px', borderRadius: 7,
            border: '1px solid var(--line-4)', background: 'transparent', color: 'var(--color-fg-2)', fontSize: 12,
          }}>{t.workspace.retry}</button>
        </div>
      )}
      {!loading && !error && items.length === 0 && !planning && (
        <EmptyState icon="note-text" title={t.workspace.outlineEmpty} style={{ padding: '4px 8px' }} />
      )}

      <ol style={{ listStyle: 'none', margin: 0, padding: 0, display: 'flex', flexDirection: 'column', gap: 1 }}>
        {items.map((item) => (
          <li key={item.chapter}>
            <a className="hov-row" href={`#${paths.reader(project, story, item.chapter)}`}
              title={item.hint || item.title || (item.waiting ? t.plan.chapterWaiting(item.chapter) : '')} style={{
              display: 'flex', alignItems: 'center', gap: 10, padding: '7px 8px', borderRadius: 8,
              color: 'var(--color-fg-2)',
              background: item.askWaiting ? 'var(--kit-warn-7)' : item.live ? 'rgba(201,166,245,0.08)' : 'transparent',
            }}>
              <span className="num" style={{ flex: '0 0 16px', fontSize: 12, color: 'var(--color-fg-4)' }}>{item.chapter}</span>
              {item.waiting ? (
                <Skeleton height={11} width="68%" style={{ flex: 1 }} />
              ) : (
                <span style={{
                  flex: 1, minWidth: 0, fontSize: 13, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
                  fontWeight: item.live || item.askWaiting ? 500 : 400,
                  // Main.dc.html › phase=plan: chương đã lưu của dàn ý dùng chữ mờ hơn.
                  color: item.askWaiting ? 'var(--color-fg-2)' : planning || (item.queued && !item.done) ? 'var(--color-fg-3)' : 'var(--color-fg-2)',
                }}>{item.title}</span>
              )}
              {item.done && <Icon name="tick-circle-bulk" size={15} label={t.status.chapterDone} style={{ color: 'var(--color-ok)' }} />}
              {item.live && <AgentOrb agent="writer" size={12} live label={liveLabel} />}
              {!item.live && item.queued && <Icon name="refresh-2" size={15} label={t.status.chapterQueued} style={{ color: 'var(--color-fg-4)' }} />}
            </a>
          </li>
        ))}
      </ol>

      <div style={{
        marginTop: 'auto', padding: '10px 8px 0', borderTop: '1px solid var(--line-1)',
        display: 'flex', flexDirection: 'column', gap: 6, fontSize: 12, color: 'var(--color-fg-4)',
      }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', gap: 10 }}>
          <span>{t.workspace.words}</span>
          <span className="num" style={{ color: 'var(--color-fg-2)' }}>{footer.words !== null && footer.words > 0 ? formatNumber(footer.words) : '—'}</span>
        </div>
        <div style={{ display: 'flex', justifyContent: 'space-between', gap: 10 }}>
          <span style={{ flex: '0 0 auto' }}>{t.workspace.latestShort}</span>
          <span className="num" style={{
            color: 'var(--color-fg-2)', minWidth: 0, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
          }} title={latest}>{latest || '—'}</span>
        </div>
      </div>
    </aside>
  )
}
