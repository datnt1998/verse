// Não hệ thống (22-brief-2510.md › D15, artboard SysBrain.dc.html, chỉ quản trị).
// Trái: bài học Chờ duyệt · Đã giữ · Đã bỏ (giữ / bỏ / sửa câu chữ / hoàn tác, chọn “Áp dụng cho”).
// Phải: mục luật hay vấp theo gói + lời nhắc Kiến trúc sư nhận khi lên dàn ý truyện cùng gói.
// Dữ liệu: GAP-46 `GET/POST /api/system/lessons`, `GET /api/system/violations?pack=`.
import { useEffect, useRef, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { actOnLesson, getLessons, getViolations } from '../../api/client'
import { useSession } from '../../api/session'
import type { Lesson, LessonAction, LessonScope, LessonState, LessonsResponse } from '../../api/types'
import { keys, useAction, useResource } from '../../data'
import { t } from '../../i18n/vi'
import { navigate, paths, useRoute } from '../../router'
import { Button } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { StaleBanner } from '@noverse/ui'
import { Segmented } from '@noverse/ui'
import { Skeleton } from '@noverse/ui'
import { alertTip, dayMonth, doneLine, pendingRows, showsScopeAll, violationBar } from './model'
import { btn2Bare, btn2Ghost, btn2Primary, packOption, segGroup, segOption, segThumb, styles as s } from './styles'
import { PanelRefreshBar } from '../../ui/PanelRefreshBar'

const copy = t.sysbrain
const TABS: LessonState[] = ['pending', 'kept', 'dropped']
const LESSONS_PREFIX = [...keys.system(), 'lessons'] as const

export function SysBrainScreen() {
  const { user, checked } = useSession()
  const admin = user?.role === 'admin'
  // Không phải quản trị: không có trang này — về Tuỳ chỉnh (API cũng trả 403).
  useEffect(() => { if (checked && user && !admin) navigate(paths.settings(), true) }, [checked, user, admin])
  if (!admin) return null
  return <SysBrain />
}

function SysBrain() {
  const parsed = useRoute()
  const { setPendingLessons } = useSession()
  const tabParam = parsed.search.get('tab')
  const tab: LessonState = TABS.includes(tabParam as LessonState) ? (tabParam as LessonState) : 'pending'
  const lessons = useResource([...keys.system(), 'lessons', tab], ({ signal }) => getLessons(tab, signal))
  const data = lessons.data
  const packs = data?.packs ?? []
  const packParam = parsed.search.get('pack')
  const pack = packs.find((entry) => entry.id === packParam) ?? packs[0] ?? null
  const violations = useResource([...keys.system(), 'violations', pack?.id ?? ''], ({ signal }) => getViolations(pack!.id, signal), { enabled: !!pack })

  useEffect(() => { if (data) setPendingLessons(data.counts.pending) }, [data, setPendingLessons])

  const setParam = (name: string, value: string) => {
    const search = new URLSearchParams(parsed.search)
    search.set(name, value)
    navigate(`${paths.sysbrain()}?${search}`, true)
  }
  const root = stylex.props(s.root)

  return (
    <main className={`scroll ${root.className ?? ''}`} style={root.style} aria-label={copy.title}>
      <div {...stylex.props(s.head)}>
        <div {...stylex.props(s.intro)}>
          <a href={`#${paths.settings()}`} {...stylex.props(s.back)} style={{ color: 'var(--kit-fg-icon)' }}>
            <Icon name="arrow-left-2" size={13} />{copy.back}
          </a>
          <h1 {...stylex.props(s.h1)}>
            {copy.title}
            <span {...stylex.props(s.chip, s.chipAdmin)}><Icon name="crown" size={12} style={{ marginRight: 4 }} />{copy.adminOnly}</span>
          </h1>
          <span {...stylex.props(s.sub)}>{copy.description}</span>
        </div>
        <div {...stylex.props(s.stats)}>
          {([['projects', data?.stats.projects], ['stories', data?.stats.stories], ['rounds', data?.stats.rounds]] as const).map(([key, value]) => (
            <div key={key} {...stylex.props(s.card, s.stat)}>
              {value === undefined ? <Skeleton width={28} height={22} /> : <span {...stylex.props(s.statValue)}>{value}</span>}
              <span {...stylex.props(s.statLabel)}>{copy.stats[key]}</span>
            </div>
          ))}
        </div>
      </div>
      <PanelRefreshBar resourceKeys={[[...keys.system(), 'lessons', tab], [...keys.system(), 'violations', pack?.id ?? '']]} updatedAt={lessons.updatedAt}
        style={{ minHeight: 2, height: 2, overflow: 'hidden', margin: '-8px 0 -6px' }} />
      {data && <StaleBanner error={lessons.error || violations.error} onRetry={() => { void lessons.refresh(); void violations.refresh() }} />}
      <div {...stylex.props(s.grid)}>
        <section aria-label={copy.lessonsLabel} {...stylex.props(s.column)}>
          <div {...stylex.props(s.toolbar)}>
            <Segmented label={copy.tabsLabel} value={tab} onChange={(next) => setParam('tab', next)}
              tabs={{ id: 'sysbrain-tab', panel: 'sysbrain-lessons' }}
              style={segGroup} thumbStyle={segThumb}
              options={TABS.map((id) => ({
                id, style: segOption(id === tab),
                label: <>{copy.tabs[id]}<span className="num" style={{ fontSize: 11.5, color: 'var(--color-fg-4)', marginLeft: 6 }}>{data ? data.counts[id] : ''}</span></>,
              }))} />
            <span {...stylex.props(s.grow)} />
            <span title={copy.appliesNext} {...stylex.props(s.note)}>{copy.appliesNext}</span>
          </div>
          <div role="tabpanel" id="sysbrain-lessons" aria-labelledby={`sysbrain-tab-${tab}`}
            {...stylex.props(s.column)}>
            {!data && lessons.error ? <LoadError onRetry={() => void lessons.refresh()} />
              : !data ? <Skeleton height={190} radius={16} />
                : <LessonList key={tab} tab={tab} data={data} />}
          </div>
        </section>
        <aside {...stylex.props(s.aside)}>
          <section aria-label={copy.violationsLabel} {...stylex.props(s.card, s.panel)}>
            <div {...stylex.props(s.panelHead)}>
              <span {...stylex.props(s.panelTitle)}>{copy.violations}</span>
              <span {...stylex.props(s.when)}>{copy.violationsHint}</span>
            </div>
            {packs.length > 0 && (
              <Segmented label={copy.packsLabel} value={pack?.id ?? ''} onChange={(next) => setParam('pack', next)}
                style={segGroup} thumbStyle={segThumb}
                options={packs.map((entry) => ({
                  id: entry.id, style: packOption(entry.id === pack?.id),
                  label: <span title={entry.name} style={{ display: 'flex', minWidth: 0, maxWidth: '100%' }}><span style={{ minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{entry.name}</span><span className="num" style={{ fontSize: 11.5, color: 'var(--color-fg-4)', marginLeft: 6, flex: 'none' }}>{entry.stories}</span></span>,
                }))} />
            )}
            {!violations.data && violations.error ? <LoadError onRetry={() => void violations.refresh()} />
              : !violations.data ? <Skeleton height={120} />
                : violations.data.rows.map((row) => {
                  const bar = violationBar(row, violations.data!.stories)
                  // Câu 2: chuông và nhãn “· mọi gói” chung một tooltip (artboard SysBrain › v.tip);
                  // nhãn chỉ đứng ở mục dùng chung ĐANG bật chuông (v.allLbl).
                  const tip = alertTip(row)
                  return (
                    <div key={row.ref} {...stylex.props(s.violation)}>
                      <span {...stylex.props(s.refCell)}><span title={row.ref} {...stylex.props(s.chip, s.chipRef, s.refChip)}>{row.ref}</span></span>
                      <span {...stylex.props(s.nameCell)}>
                        <span title={row.refName} {...stylex.props(s.name)}>{row.refName}</span>
                        <span {...stylex.props(s.track)}><span {...stylex.props(s.bar, row.level === 'error' ? s.barError : s.barWarning)} style={{ width: bar.width }} /></span>
                      </span>
                      <span {...stylex.props(s.countCell)}>
                        <span {...stylex.props(s.countLine)}>
                          <span {...stylex.props(s.count)}>{bar.label}</span>
                          {row.alert && <span title={tip} aria-label={tip} role="img" {...stylex.props(s.alert)}><Icon name="notification" size={14} /></span>}
                        </span>
                        {showsScopeAll(row) && <span title={tip} {...stylex.props(s.scopeAll)}>{copy.alertScopeAll}</span>}
                      </span>
                    </div>
                  )
                })}
            <div {...stylex.props(s.legend)}>
              <span {...stylex.props(s.legendItem)}><span {...stylex.props(s.swatch, s.barError)} />{copy.legend.error}</span>
              <span {...stylex.props(s.legendItem)}><span {...stylex.props(s.swatch, s.barWarning)} />{copy.legend.warning}</span>
              <span {...stylex.props(s.legendItem)}><Icon name="notification" size={12} style={{ color: 'var(--kit-err-soft)' }} />{copy.legend.alert}</span>
              {/* Artboard: câu chú thích trần cuối hàng legend; số gói lấy từ danh sách gói của dữ liệu. */}
              {packs.length > 0 && <span>{copy.legend.scopeAll(packs.length)}</span>}
            </div>
          </section>
          <section aria-label={copy.reminderLabel} {...stylex.props(s.card, s.reminder)}>
            <span {...stylex.props(s.reminderTitle)}><Icon name="notification" size={16} style={{ color: 'var(--kit-err-soft)' }} />{copy.reminder}</span>
            <span {...stylex.props(s.reminderLead)}>{copy.reminderLead(violations.data?.packName ?? pack?.name ?? '')}</span>
            {violations.data ? <p {...stylex.props(s.reminderText)}>{violations.data.preamble}</p> : <Skeleton height={64} radius={12} />}
          </section>
          <p {...stylex.props(s.footnote)}>{copy.footnote}</p>
        </aside>
      </div>
    </main>
  )
}

function LoadError({ onRetry }: { onRetry: () => void }) {
  return (
    <div role="alert" {...stylex.props(s.card, s.empty, s.error)} style={{ justifyContent: 'center' }}>
      {copy.error}<Button size="sm" onClick={onRetry}>{copy.retry}</Button>
    </div>
  )
}

/**
 * Danh sách thẻ của một tab. Tab Chờ duyệt giữ bài vừa giữ/bỏ tại chỗ (có Hoàn tác) như artboard;
 * đổi tab là dựng lại (key) nên các bài đó rời khỏi tab Chờ duyệt.
 */
export function LessonList({ tab, data }: { tab: LessonState; data: LessonsResponse }) {
  const order = useRef<string[]>([])
  const [local, setLocal] = useState<Record<string, Lesson>>({})
  const [scopes, setScopes] = useState<Record<string, LessonScope>>({})
  const [editing, setEditing] = useState<{ id: string; text: string } | null>(null)
  const [running, setRunning] = useState<string | null>(null)
  const act = useAction((id: string, input: LessonAction) => actOnLesson(id, input), { invalidate: [[...LESSONS_PREFIX]] })

  if (tab === 'pending') {
    for (const lesson of data.lessons) if (!order.current.includes(lesson.id)) order.current.push(lesson.id)
  }
  const rows = tab === 'pending' ? pendingRows(order.current, data.lessons, local) : data.lessons

  const run = async (lesson: Lesson, input: LessonAction) => {
    setRunning(lesson.id)
    const result = await act.run(lesson.id, input)
    setRunning(null)
    if (result) setLocal((value) => ({ ...value, [lesson.id]: result }))
    return result
  }
  const scopeOf = (lesson: Lesson) => scopes[lesson.id] ?? lesson.scope
  const editedText = (lesson: Lesson) => (editing?.id === lesson.id && editing.text.trim() && editing.text.trim() !== lesson.text ? editing.text.trim() : undefined)

  if (rows.length === 0) return <div {...stylex.props(s.card, s.empty)}>{copy.empty}</div>
  return (
    <>
      {rows.map((lesson) => {
        const pending = lesson.state === 'pending'
        const isEditing = editing?.id === lesson.id
        const busy = running === lesson.id
        // Bài vừa giữ/bỏ ở tab Chờ duyệt trồi lên (.rise) như artboard.
        const card = stylex.props(s.card, s.lesson)
        return (
          <article key={lesson.id} data-lesson={lesson.id} style={card.style}
            className={tab === 'pending' && !pending ? `${card.className ?? ''} rise` : card.className}>
            <div {...stylex.props(s.chipRow)}>
              <span {...stylex.props(s.chip, lesson.origin === 'user' ? s.chipUser : s.chipReview)}>{copy.origin[lesson.origin]}</span>
              {lesson.refs.map((ref) => <span key={ref} {...stylex.props(s.chip, s.chipRef)}>{ref}</span>)}
              <span {...stylex.props(s.grow)} />
              <span {...stylex.props(s.when)}>{pending ? copy.fresh(dayMonth(lesson.createdAt)) : ''}</span>
            </div>
            {isEditing
              ? <textarea aria-label={copy.editArea} value={editing.text} autoFocus
                  onChange={(event) => setEditing({ id: lesson.id, text: event.target.value })} {...stylex.props(s.textarea)} />
              : <p {...stylex.props(s.text)}>{lesson.text}</p>}
            <div {...stylex.props(s.evidence)}>
              <span {...stylex.props(s.evidenceSource)}>{copy.source(lesson)}</span>
              <div {...stylex.props(s.evidenceChips)}>
                {lesson.evidence.chips.map((chip, index) => <span key={`${chip}-${index}`} title={chip} {...stylex.props(s.chip, s.chipEvidence)}>{chip}</span>)}
              </div>
            </div>
            {pending ? (
              <div {...stylex.props(s.actions)}>
                <span {...stylex.props(s.appliesTo)}>{copy.appliesTo}</span>
                <Segmented label={copy.appliesTo} value={scopeOf(lesson)} onChange={(next) => setScopes((value) => ({ ...value, [lesson.id]: next as LessonScope }))}
                  style={segGroup} thumbStyle={segThumb}
                  options={([['pack', copy.scopePack(lesson.packName)], ['all', copy.scopeAll]] as const).map(([id, label]) => ({ id, label, style: segOption(scopeOf(lesson) === id) }))} />
                <span {...stylex.props(s.grow)} />
                <Button disabled={act.busy} style={btn2Bare} onClick={() => void run(lesson, { action: 'drop' })}>{copy.drop}</Button>
                <Button disabled={act.busy} style={btn2Ghost} onClick={async () => {
                  if (!isEditing) { setEditing({ id: lesson.id, text: lesson.text }); return }
                  const text = editedText(lesson)
                  if (text && !(await run(lesson, { action: 'edit', text }))) return
                  setEditing(null)
                }}>{isEditing ? copy.editDone : copy.edit}</Button>
                <Button variant="primary" disabled={act.busy && !busy} pending={busy && act.pending} style={btn2Primary}
                  icon={<Icon name="tick-circle" size={14} />} onClick={async () => {
                    // Máy chủ chỉ đổi câu chữ qua action `edit` (có kiểm tên riêng/đoạn truyện) → lưu câu chữ trước rồi mới giữ.
                    const text = editedText(lesson)
                    if (text && !(await run(lesson, { action: 'edit', text }))) return
                    if (await run(lesson, { action: 'keep', scope: scopeOf(lesson) })) setEditing(null)
                  }}>{copy.keep}</Button>
              </div>
            ) : (
              <div {...stylex.props(s.done, lesson.state === 'kept' ? s.doneKept : s.doneDropped)}>
                <Icon name={lesson.state === 'kept' ? 'tick-circle-bulk' : 'trash'} size={14} />
                <span {...stylex.props(s.doneText)}>{doneLine(lesson, tab === 'pending')}</span>
                <Button disabled={act.busy} style={btn2Bare} onClick={() => void run(lesson, { action: 'undo' })}>{copy.undo}</Button>
              </div>
            )}
          </article>
        )
      })}
    </>
  )
}
