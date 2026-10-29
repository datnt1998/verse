// Ngăn kéo Não truyện (Gói U): dùng chung đồ thị, dòng thời gian và chi tiết với màn Não truyện.
import { useEffect, useId, useRef, useState, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import * as stylex from '@stylexjs/stylex'
import { t } from '../../i18n/vi'
import { paths } from '../../router'
import { useStoryName } from '../../shell/useNames'
import { Kbd } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { EmptyState, Skeleton } from '@noverse/ui'
import { useEscapeLayer } from '@noverse/ui'
import { DetailColumn } from '../brain/DetailColumn'
import { initialChapter, narratorOf } from '../brain/model'
import { ChapterRail } from '../brain/ChapterRail'
import { StoryGraph } from '../brain/StoryGraph'
import { useAddressTerms, useBrain } from '../project/cast/data'
import { useBrainDrawer } from './brainDrawer'
import { s } from './brainDrawer.stylex'

export function BrainDrawer({ project, story }: { project: string; story: string }) {
  const { isOpen, target, chapter, close } = useBrainDrawer()
  if (!isOpen) return null
  return createPortal(<DrawerBody project={project} story={story} target={target} chapter={chapter} onClose={close} />, document.body)
}

function DrawerBody({ project, story, target, chapter: wanted, onClose }: {
  project: string
  story: string
  target: string | null
  /** Chương đang viết của Bàn viết lúc mở; null thì Não truyện tự chọn. */
  chapter: number | null
  onClose: () => void
}) {
  const brain = useBrain(project, story)
  const address = useAddressTerms(project, story)
  const storyTitle = useStoryName(project, story)
  const data = brain.data ?? null
  const writing = address.data?.writing ?? null
  const [selected, setSelected] = useState<string | null>(null)
  const [chapter, setChapter] = useState<number | null>(null)
  const panelRef = useRef<HTMLDivElement>(null)
  const titleId = useId()

  useEscapeLayer(true, onClose)

  // Lớp phủ giữ bàn phím/chuột trong ngăn kéo: nền thành inert + aria-hidden, focus vào panel.
  // Đóng thì trả focus về đúng chỗ đã mở (hàng nhân vật / nút Đồ thị) — Tab tiếp nối từ đó.
  useEffect(() => {
    const root = document.getElementById('root')
    const panel = panelRef.current
    const trigger = document.activeElement instanceof HTMLElement ? document.activeElement : null
    root?.setAttribute('inert', '')
    root?.setAttribute('aria-hidden', 'true')
    panel?.focus()
    return () => {
      root?.removeAttribute('inert')
      root?.removeAttribute('aria-hidden')
      if (trigger?.isConnected) trigger.focus()
    }
  }, [])

  // Tab vòng trong ngăn kéo (dialog modal): ra khỏi panel thì quay về mục đầu.
  useEffect(() => {
    const panel = panelRef.current
    if (!panel) return
    const onKey = (event: KeyboardEvent) => {
      if (event.key !== 'Tab') return
      const items = Array.from(panel.querySelectorAll<HTMLElement>(
        'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])',
      )).filter((element) => !element.hasAttribute('disabled'))
      if (items.length === 0) return
      const first = items[0]
      const last = items[items.length - 1]
      const active = document.activeElement
      const outside = !panel.contains(active)
      if (event.shiftKey && (active === first || outside)) { event.preventDefault(); last.focus() }
      else if (!event.shiftKey && (active === last || outside)) { event.preventDefault(); first.focus() }
    }
    document.addEventListener('keydown', onKey, true)
    return () => document.removeEventListener('keydown', onKey, true)
  }, [])

  // Chọn sẵn người mở ngăn kéo (tên hoặc id); dữ liệu về sau thì chọn lại theo tên.
  const targetId = target && data
    ? data.entities.find((entity) => entity.id === target || entity.name === target)?.id ?? null
    : null
  useEffect(() => { if (targetId) setSelected(targetId) }, [targetId])
  useEffect(() => {
    if (!data) return
    setSelected((current) => (current && data.entities.some((entity) => entity.id === current))
      ? current
      : narratorOf(data.entities) ?? data.entities[0]?.id ?? null)
  }, [data])
  useEffect(() => {
    if (!data) return
    setChapter((current) => (current !== null && data.chapters.some((item) => item.n === current))
      ? current
      : (wanted && data.chapters.some((item) => item.n === wanted) ? wanted : initialChapter(data.chapters)))
  }, [data, wanted])

  const overlay = stylex.props(s.overlay)
  const scrim = stylex.props(s.scrim)
  const panel = stylex.props(s.panel)
  const header = stylex.props(s.header)
  const headCopy = stylex.props(s.headCopy)
  const title = stylex.props(s.title)
  const sub = stylex.props(s.sub)
  const lockChip = stylex.props(s.lockChip)
  const headActions = stylex.props(s.headActions)
  const openFull = stylex.props(s.openFull)
  const close = stylex.props(s.close)
  const body = stylex.props(s.body)
  const graphCol = stylex.props(s.graphCol)
  const detailCol = stylex.props(s.detailCol)
  const detailScroll = stylex.props(s.detailScroll)
  const lockNote = stylex.props(s.lockNote)

  const shell = (children: ReactNode) => (
    <div {...overlay}>
      <div aria-hidden="true" className={scrim.className} style={scrim.style} onClick={onClose} />
      <div ref={panelRef} tabIndex={-1} role="dialog" aria-modal="true" aria-labelledby={titleId}
        className={panel.className} style={panel.style}>
        <div {...header}>
          <Icon name="routing-2" size={18} style={{ color: 'var(--color-accent)', flexShrink: 0 }} />
          <div {...headCopy}>
            <h2 id={titleId} {...title}>{t.brain.title}</h2>
            <span {...sub} title={storyTitle}>
              {storyTitle}{chapter !== null ? ` · ${t.brain.drawerChapter(chapter)}` : ''}
            </span>
          </div>
          <span {...lockChip}>
            <Icon name="pause" size={13} />
            {writing ? t.brain.writingChip : t.brain.readOnlyChip}
          </span>
          <div {...headActions}>
            {/* base.css đặt `a { color: var(--color-fg) }` ngoài layer nên màu link để inline. */}
            <a className={openFull.className} style={openFull.style}
              href={`#${paths.brain(project, story, chapter)}`} onClick={onClose}>
              {t.brain.openFull}<Icon name="arrow-right-2" size={14} />
            </a>
            <button type="button" className={close.className} style={close.style} onClick={onClose}>
              {t.brain.close}<Kbd>Esc</Kbd>
            </button>
          </div>
        </div>
        {children}
      </div>
    </div>
  )

  if (brain.error && !data) {
    return shell(
      <EmptyState icon="warning-2" title={t.brain.loadError} hint={t.brain.loadHint}
        action={<button type="button" className="hov-ghost" onClick={() => void brain.refresh()}>{t.brain.retry}</button>} />,
    )
  }

  if (!data || chapter === null || selected === null) {
    return shell(
      <div {...body}>
        <div style={{ padding: 16, display: 'flex', flexDirection: 'column', gap: 10 }}>
          <Skeleton height={34} radius={10} />
          <Skeleton height={420} radius={16} />
        </div>
        <div style={{ padding: 16, display: 'flex', flexDirection: 'column', gap: 10 }}>
          {[0, 1, 2, 3, 4].map((key) => <Skeleton key={key} height={24} radius={6} />)}
        </div>
      </div>,
    )
  }

  if (data.entities.length === 0) {
    return shell(<EmptyState icon="routing-2" title={t.brain.emptyTitle} hint={t.brain.emptyHint} />)
  }

  return shell(
    <div {...body}>
      <div {...graphCol}>
        <StoryGraph brain={data} selected={selected} chapter={chapter} onSelect={setSelected} compact />
        <ChapterRail brain={data} chapter={chapter} onChapter={setChapter} compact />
      </div>
      <div {...detailCol}>
        <div {...detailScroll}>
          <DetailColumn brain={data} chapter={chapter} entityId={selected} address={address.data ?? null}
            writing={writing} project={project} story={story} variant="drawer"
            footer={<div role="status" {...lockNote}>{writing ? t.brain.lockNote : t.brain.lockNoteIdle}</div>} />
        </div>
      </div>
    </div>,
  )
}

