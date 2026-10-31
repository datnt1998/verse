// Tab Dàn ý — nút chuyển “Danh sách chương | Bản đồ giữ chân”, thẻ theo tập +
// hàng chương mở rộng (Outline.dc.html › view story › tab outline).
// Kế hoạch/hợp đồng lấy từ GET /chapters/:n/plan khi mở hàng; bản đồ giữ chân từ GET /retention (không bịa dữ liệu).
import { useEffect, useState, type CSSProperties } from 'react'
import { canWriteStory, getChapterPlan, getRetention } from '../../../api/client'
import { useSession } from '../../../api/session'
import type { ChapterPlan, Outline } from '../../../api/types'
import { keys, useResource } from '../../../data'
import { t } from '../../../i18n/vi'
import { navigate, paths, setDialog, useRoute } from '../../../router'
import { EmptyState, Skeleton, Spinner } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { chapterStatusLabel, formatWords, isRewriteFlow, volumeLabel } from '../format'
import { Chip } from '../bits'
import { RetentionMap } from '../retention/RetentionMap'
import { mapSummary } from '../retention/logic'

const STATUS_COLOR = { done: '#A6EDC1', live: '#D9CCF7', idle: '#7E7E8A' } as const

/** `.seg` của artboard — hai nút aria-pressed, mục đang chọn nền surface-6. */
const SEG: CSSProperties = {
  display: 'inline-flex', padding: 3, borderRadius: 10, background: 'var(--color-card)', border: '1px solid var(--line-2)',
}
const SEG_BTN: CSSProperties = { height: 28, padding: '0 12px', border: 'none', borderRadius: 7, background: 'transparent', fontSize: 12.5, whiteSpace: 'nowrap' }
const SEG_ON: CSSProperties = { background: 'var(--color-surface-6)', color: 'var(--color-fg)', fontWeight: 500 }
const SEG_OFF: CSSProperties = { color: 'var(--color-fg-3)', fontWeight: 400 }

const ghostLink = {
  height: 30, padding: '0 12px', borderRadius: 8, fontSize: 12.5, display: 'flex', alignItems: 'center', gap: 6,
} as const

/** “Mục tiêu · …” — các dòng có nội dung của kế hoạch chương. */
function planLines(plan: ChapterPlan | null | undefined): string[] {
  if (!plan) return []
  return [
    plan.goal && `Mục tiêu · ${plan.goal}`,
    plan.conflict && `Xung đột · ${plan.conflict}`,
    plan.hook && `Móc · ${plan.hook}`,
    plan.emotion_arc && `Cung cảm xúc · ${plan.emotion_arc}`,
  ].filter((line): line is string => !!line)
}

/** “Nhịp bắt buộc · …” — hợp đồng chương (`contract`). */
function contractLines(plan: ChapterPlan | null | undefined): string[] {
  const contract = plan?.contract
  if (!contract) return []
  return [
    ...(contract.required_beats ?? []).map((beat) => `Nhịp bắt buộc · ${beat}`),
    ...(contract.forbidden_moves ?? []).map((move) => `Điều cần tránh · ${move}`),
    ...(contract.payoff_points ?? []).map((point) => `Điểm thu hoạch · ${point}`),
  ]
}

function Field({ label, lines, placeholder }: { label: string; lines: string[]; placeholder?: string }) {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
      <span className="lbl" style={{ color: '#7E7E8A' }}>{label}</span>
      {lines.length > 0
        ? lines.map((line) => <span key={line} style={{ fontSize: 13.5, color: '#D6D6DC', lineHeight: 1.55 }}>{line}</span>)
        : <span className="pj-ph" style={{ fontSize: 13.5, lineHeight: 1.55 }}>{placeholder ?? '—'}</span>}
    </div>
  )
}

export function OutlineTab({ project, story, bookTitle, outline, onSteer }: {
  project: string
  story: string
  /** tên sách — để nhận ra tập có tiêu đề trùng tên truyện và hiện “Tập N” */
  bookTitle: string
  outline: Outline
  /** Điền sẵn ô chat khi bấm “Sửa” một dòng bản đồ giữ chân (02-ia-flows: người dùng vẫn bấm gửi). */
  onSteer?: (text: string) => void
}) {
  const chapters = outline.volumes.flatMap((volume) => volume.chapters)
  const rewriteFlow = isRewriteFlow(chapters)
  const live = chapters.find((chapter) => chapter.status === 'writing' || chapter.status === 'rewriting')
  const [openChapter, setOpenChapter] = useState<number | null>(live?.chapter ?? null)
  const [omap, setOmap] = useState<'list' | 'map'>('list')
  const planResource = useResource(keys.story(project, story, 'plan', openChapter), ({ signal }) => getChapterPlan(project, story, openChapter!, signal), { enabled: openChapter !== null })
  const retention = useResource(keys.story(project, story, 'retention'), ({ signal }) => getRetention(project, story, signal), { enabled: omap === 'map' })
  const { user } = useSession()
  const canEditRow = !!user && canWriteStory(user, `${project}/${story}`) && !!onSteer
  const route = useRoute()
  const toggle = (chapter: number) => setOpenChapter((current) => current === chapter ? null : chapter)
  // Mục lục cột trái bấm chương → ?ch=N: mở đúng chương + cuộn tới (Gói AG).
  useEffect(() => {
    const ch = Number(route.search.get('ch') ?? '') || 0
    if (!ch) return
    setOpenChapter((current) => current === ch ? current : ch)
    requestAnimationFrame(() => document.getElementById(`och-${ch}`)?.scrollIntoView({ block: 'center', behavior: 'smooth' }))
  }, [route.search])

  const lastVolume = outline.volumes[outline.volumes.length - 1]
  const empty = (
    <EmptyState icon="note-text" title="Chưa có dàn ý"
      hint="Dàn ý hiện ra sau khi truyện bắt đầu được viết."
      action={
        <button type="button" className="hov-ghost" onClick={() => navigate(paths.workspace(project, story))}
          style={{ height: 32, padding: '0 12px', borderRadius: 8, border: '1px solid rgba(255,255,255,0.1)', background: 'transparent', color: '#EDEDEF', fontSize: 13 }}>
          Mở Workspace
        </button>
      } />
  )

  return (
    <>
      {chapters.length === 0 || outline.volumes.length === 0
        ? empty
        : (
          <>
            <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 10 }}>
              <div role="group" aria-label="Cách xem dàn ý" style={SEG}>
                <button type="button" aria-pressed={omap === 'list'} onClick={() => setOmap('list')}
                  style={{ ...SEG_BTN, ...(omap === 'list' ? SEG_ON : SEG_OFF) }}>Danh sách chương</button>
                <button type="button" aria-pressed={omap === 'map'} onClick={() => setOmap('map')}
                  style={{ ...SEG_BTN, ...(omap === 'map' ? SEG_ON : SEG_OFF) }}>Bản đồ giữ chân</button>
              </div>
              <span style={{ flex: 1 }} />
              {omap === 'map' && retention.data && !retention.data.empty && (
                <span style={{ fontSize: 12, color: 'var(--kit-fg-icon)' }}>{mapSummary(retention.data)}</span>
              )}
            </div>

            {omap === 'map' && (
              retention.data
                ? retention.data.empty
                  ? (
                    // Câu 19: dàn ý cũ chưa có trường giữ chân — không chạy phép kiểm, mời Kiến trúc sư lập.
                    <EmptyState icon="note-text" title={t.retention.empty}
                      action={canEditRow
                        ? (
                          <button type="button" className="hov-ghost" onClick={() => onSteer?.(t.retention.prefill)}
                            style={{ height: 32, padding: '0 12px', borderRadius: 8, border: '1px solid var(--line-4)', background: 'transparent', color: 'var(--color-fg)', fontSize: 13 }}>
                            {t.retention.request}
                          </button>
                        )
                        : undefined} />
                  )
                  : (
                    <RetentionMap retention={retention.data}
                      label={outline.volumes.length === 1 ? `Bản đồ giữ chân ${volumeLabel(outline.volumes[0], bookTitle)}` : 'Bản đồ giữ chân'}
                      onEditRow={canEditRow ? (chapter) => onSteer?.(`Sửa dòng chương ${chapter}: `) : undefined} />
                  )
                : retention.error
                  ? (
                    <EmptyState icon="warning-2" title="Không tải được bản đồ giữ chân"
                      hint="Máy chủ chưa đọc được đại cương của truyện."
                      action={
                        <button type="button" className="hov-ghost" onClick={() => void retention.refresh()}
                          style={{ height: 32, padding: '0 12px', borderRadius: 8, border: '1px solid var(--line-4)', background: 'transparent', color: 'var(--color-fg)', fontSize: 13 }}>
                          Thử lại
                        </button>
                      } />
                  )
                  : <Skeleton height={260} radius={16} />
            )}

            {omap === 'list' && outline.volumes.map((volume) => (
        <section key={volume.index} className="card" style={{ borderRadius: 16, overflow: 'hidden' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '14px 16px', borderBottom: '1px solid rgba(255,255,255,0.06)' }}>
            <span style={{ fontSize: 15, fontWeight: 600 }}>{volumeLabel(volume, bookTitle)}</span>
            <span className="num" style={{ fontSize: 12.5, color: '#8E8E99' }}>
              {volume.chapters.length} chương · {outline.layered ? 'dàn ý nhiều tầng' : 'dàn ý một tầng'}
            </span>
            <span style={{ flex: '1 1 auto' }} />
            {rewriteFlow && (
              <Chip tone={{ background: 'rgba(201,166,245,0.1)', color: '#DCC8F8' }}>
                Viết lại {volume.chapters.filter((chapter) => chapter.status === 'done').length} / {volume.chapters.length}
              </Chip>
            )}
          </div>

          {volume.chapters.map((chapter, index) => {
            const open = openChapter === chapter.chapter
            const writing = chapter.status === 'writing' || chapter.status === 'rewriting'
            const plan = open ? planResource.data : null
            const loading = open && planResource.initialLoading
            const statusColor = chapter.status === 'done' ? STATUS_COLOR.done : writing ? STATUS_COLOR.live : STATUS_COLOR.idle
            const planText = planLines(plan)
            return (
              <div key={chapter.chapter} id={`och-${chapter.chapter}`} style={{ borderTop: index ? '1px solid rgba(255,255,255,0.05)' : 'none', background: open ? 'rgba(255,255,255,0.02)' : 'transparent' }}>
                <button className="pj-crow hov-row" type="button" onClick={() => toggle(chapter.chapter)} aria-expanded={open}>
                  <span className="num" style={{ width: 22, color: '#7E7E8A' }}>{chapter.chapter}</span>
                  <span style={{ flex: '1 1 auto', minWidth: 0, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis', fontWeight: writing ? 600 : 400 }}>{chapter.title || '—'}</span>
                  <span className="num" style={{ fontSize: 12, color: '#7E7E8A', width: 70, textAlign: 'right' }}>{chapter.words > 0 ? `${formatWords(chapter.words)} chữ` : ''}</span>
                  <span style={{ fontSize: 12.5, color: statusColor, width: 104, textAlign: 'right', whiteSpace: 'nowrap' }}>{chapterStatusLabel(chapter.status, rewriteFlow)}</span>
                  <Icon name="arrow-down-2" size={14} style={{ color: '#7E7E8A', transform: `rotate(${open ? 180 : 0}deg)`, transition: 'transform .2s' }} />
                </button>
                {open && (
                  <div className="pj-cbody">
                    <Field label="Tóm tắt" lines={chapter.summary ? [chapter.summary] : []} placeholder="—" />
                    <Field label="Sự kiện chính" lines={chapter.coreEvent ? [chapter.coreEvent] : []} placeholder="—" />
                    {loading
                      ? <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}><span className="lbl" style={{ color: '#7E7E8A' }}>Mục tiêu chương</span><Spinner size={13} /></div>
                      : <Field label="Mục tiêu chương" lines={planText} />}
                    <Field label="Bắt buộc có" lines={loading ? [] : contractLines(plan)} />
                    <div style={{ gridColumn: '1 / -1', display: 'flex', gap: 8 }}>
                      <a className="hov-ghost" href={`#${paths.reader(project, story, chapter.chapter)}`}
                        style={{ ...ghostLink, border: '1px solid rgba(255,255,255,0.1)', color: '#EDEDEF' }}>
                        <Icon name="book-1" size={14} />Đọc chương
                      </a>
                      <a className="hov-ghost" href={`#${paths.reader(project, story, chapter.chapter)}`}
                        style={{ ...ghostLink, border: 'none', background: 'transparent', color: '#A0A0AB' }}>
                        <Icon name="messages-2" size={14} />Góp ý cho chương này
                      </a>
                    </div>
                  </div>
                )}
              </div>
            )
          })}
        </section>
            ))}

            {lastVolume && (
              <section className="card" style={{ marginTop: 10, borderRadius: 16, padding: '14px 16px', display: 'flex', alignItems: 'center', gap: 14 }}>
                <Icon name="unlock" size={18} style={{ color: '#7E7E8A' }} />
                <div style={{ flex: '1 1 auto' }}>
                  <div style={{ fontSize: 14, fontWeight: 500 }}>Tập {lastVolume.index + 1}</div>
                  <div style={{ fontSize: 12.5, color: '#8E8E99' }}>
                    Khi truyện hoàn thành, bấm “Viết tiếp tập mới” và ghi hướng bạn muốn. Truyện dài tự chia Tập → Hồi → Chương.
                  </div>
                </div>
                <button className="hov-ghost" type="button" onClick={() => setDialog('reopen')}
                  style={{ height: 32, padding: '0 12px', borderRadius: 9, border: '1px solid rgba(255,255,255,0.1)', background: 'transparent', color: '#EDEDEF', fontSize: 12.5, whiteSpace: 'nowrap' }}>
                  Viết tiếp tập mới
                </button>
              </section>
            )}
          </>
        )}
    </>
  )
}
