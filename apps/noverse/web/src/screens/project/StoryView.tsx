// StoryView — nền móng, nội dung và cách viết của truyện; ô Dặn hướng neo đáy.
import { Activity, useRef, useState } from 'react'
import { ApiError, getSnapshot, sendInput } from '../../api/client'
import type { Outline, StorySummary } from '../../api/types'
import { keys, useAction, useConnection, useResource } from '../../data'
import { t } from '../../i18n/vi'
import { paths, setDialog } from '../../router'
import { setLocked } from '../../shell/stores'
import { AgentOrb } from '@noverse/ui'
import { EmptyState, Skeleton } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { StaleBanner } from '@noverse/ui'
import { toast } from '@noverse/ui'
import { CHIP_NEUTRAL, CHIP_PURPLE, Chip, Dock } from './bits'
import { STORY_STATUS_LABEL, isRewriteFlow, storyTitle } from './format'
import { CastTab } from './tabs/CastTab'
import { useCharacters } from './cast/data'
import { ExportTab } from './tabs/ExportTab'
import { ForeshadowTab, useForeshadow } from './tabs/ForeshadowTab'
import { OutlineTab } from './tabs/OutlineTab'
import { ScrollTopButton } from '@noverse/ui'
import { styleCopy } from '../../i18n/vi'
import { StyleTab } from './tabs/StyleTab'
import { PanelRefreshBar } from '../../ui/PanelRefreshBar'
import { InfoTab } from './tabs/InfoTab'
import { PremiseTab, WorldTab } from './tabs/FoundationTabs'
import { useFoundation } from './tabs/useFoundation'

export type TabId = 'info' | 'premise' | 'cast' | 'world' | 'outline' | 'fs' | 'style' | 'export'

const TABS: { id: TabId; label: string }[] = [
  { id: 'info', label: 'Thông tin' },
  { id: 'premise', label: 'Tiền đề' },
  { id: 'cast', label: 'Nhân vật' },
  { id: 'world', label: 'Thế giới' },
  { id: 'outline', label: 'Dàn ý' },
  { id: 'fs', label: 'Cài cắm' },
  { id: 'style', label: styleCopy.tab },
  { id: 'export', label: 'Bản xuất' },
]

export const STORY_TABS = TABS.map((tab) => tab.id)
export function resolveStoryTab(raw: string | null): TabId {
  if (raw === 'rules') return 'style'
  return STORY_TABS.includes(raw as TabId) ? raw as TabId : 'outline'
}

function StoryTabs({ tab, onTab, counts }: { tab: TabId; onTab: (id: TabId) => void; counts: Partial<Record<TabId, number>> }) {
  return (
    <div role="tablist" aria-label="Truyện" style={{ display: 'flex', gap: 2, paddingBottom: 12, overflowX: 'auto', flexShrink: 0, borderBottom: '1px solid rgba(255,255,255,0.06)' }}>
      {TABS.map((item) => {
        const on = item.id === tab
        const count = counts[item.id]
        return (
          <button key={item.id} className="pj-tab" type="button" role="tab" aria-selected={on} aria-controls={`story-panel-${item.id}`}
            onClick={() => onTab(item.id)}
            style={{ background: on ? '#222225' : 'transparent', color: on ? '#EDEDEF' : '#A0A0AB', fontWeight: on ? 500 : 400 }}>
            {item.label}
            {count !== undefined && count > 0 && <span className="num" style={{ fontSize: 11.5, color: '#7E7E8A' }}>{count}</span>}
          </button>
        )
      })}
    </div>
  )
}

export function StoryView({ project, summary, narrow, tab, onTab, outline, outlineLoading, outlineError, onRetryOutline }: {
  project: string
  summary: StorySummary
  narrow: boolean
  tab: TabId
  onTab: (id: TabId) => void
  outline: Outline | null
  outlineLoading: boolean
  outlineError: boolean
  onRetryOutline: () => void
}) {
  const connection = useConnection()
  const [draft, setDraft] = useState('')
  const dockRef = useRef<HTMLInputElement>(null)
  const scrollRef = useRef<HTMLDivElement>(null)
  const headingRef = useRef<HTMLHeadingElement>(null)
  /** D5 — “Sửa” một dòng bản đồ giữ chân: điền sẵn ô chat rồi để người dùng bấm gửi (02-ia-flows). */
  const prefillSteer = (text: string) => {
    setDraft(text)
    requestAnimationFrame(() => {
      dockRef.current?.focus()
      dockRef.current?.setSelectionRange(text.length, text.length)
    })
  }
  const snapshot = useResource(keys.story(project, summary.story, 'snapshot'), ({ signal }) => getSnapshot(project, summary.story, signal))
  const characters = useCharacters(project, summary.story)
  const foreshadow = useForeshadow(project, summary.story)
  const foundation = useFoundation(project, summary.story)
  const steerAction = useAction(async (text: string) => {
    try {
      const result = await sendInput(project, summary.story, text)
      toast.ok(result.action === 'steer' ? 'Đã gửi can thiệp' : 'Đã gửi yêu cầu sáng tác')
      return result
    } catch (error) {
      if (error instanceof ApiError && error.status === 409) setLocked(true)
      throw error
    }
  }, { invalidate: [keys.story(project, summary.story), keys.project(project)] })
  const send = () => {
    const text = draft.trim()
    if (!text || steerAction.busy || !connection.writable) return
    void steerAction.run(text).then((result) => { if (result) setDraft('') })
  }

  const chapters = outline?.volumes.flatMap((volume) => volume.chapters) ?? []
  const rewriting = isRewriteFlow(chapters)
  const current = snapshot.data?.inProgressChapter ?? 0
  const writing = summary.status === 'writing' || summary.running
  const statusText = summary.running
    ? `${rewriting ? 'Đang viết lại' : 'Đang viết'}${current > 0 ? ` · chương ${current}` : ''}`
    : STORY_STATUS_LABEL[summary.status]

  const body = (panel: TabId) => {
    if (panel === 'info') return <InfoTab key={summary.story} project={project} summary={summary} snapshot={snapshot.data} outline={outline} foundation={foundation}
      castCount={characters.data?.characters.length ?? 0} mainCount={characters.data?.characters.filter((character) => character.kind === 'lead').length ?? 0}
      onTab={onTab} onSteer={prefillSteer} />
    if (panel === 'premise') return <PremiseTab foundation={foundation} onSteer={prefillSteer} />
    if (panel === 'world') return <WorldTab foundation={foundation} project={project} completed={summary.completedChapters} onSteer={prefillSteer} />
    if (panel === 'outline') {
      if (outlineLoading && !outline) return <Skeleton height={260} radius={16} />
      if (outlineError) {
        return (
          <EmptyState icon="warning-2" title="Không tải được dàn ý" hint="Máy chủ chưa đọc được dàn ý của truyện."
            action={
              <button type="button" className="hov-ghost" onClick={onRetryOutline}
                style={{ height: 32, padding: '0 12px', borderRadius: 8, border: '1px solid rgba(255,255,255,0.1)', background: 'transparent', color: '#EDEDEF', fontSize: 13 }}>
                Thử lại
              </button>
            } />
        )
      }
      return outline
        ? <OutlineTab key={summary.story} project={project} story={summary.story} bookTitle={storyTitle(summary)} outline={outline} onSteer={prefillSteer} />
        : <Skeleton height={260} radius={16} />
    }
    if (panel === 'cast') return <CastTab key={summary.story} project={project} story={summary.story} />
    if (panel === 'fs') return <ForeshadowTab key={summary.story} project={project} story={summary.story} />
    if (panel === 'style') return <StyleTab key={summary.story} project={project} summary={summary} />
    return <ExportTab key={summary.story} project={project} story={summary.story} bookTitle={storyTitle(summary)} />
  }

  return (
    <main style={{ flex: '1 1 auto', minWidth: 0, minHeight: 0, display: 'flex', flexDirection: 'column', position: 'relative' }}>
      <PanelRefreshBar resourceKeys={[keys.story(project, summary.story, 'snapshot')]} updatedAt={snapshot.updatedAt} />
      <StaleBanner error={snapshot.error} onRetry={snapshot.refresh} />
      <div style={{ flex: '0 0 auto', padding: narrow ? '16px 14px 0' : '20px 28px 0', display: 'flex', flexDirection: 'column', gap: 14 }}>
        <div style={{ display: 'flex', alignItems: 'flex-start', gap: 16 }}>
          <div style={{ flex: '1 1 auto', minWidth: 0, display: 'flex', flexDirection: 'column', gap: 6 }}>
            <span style={{ display: 'flex', gap: 8 }}>
              <Chip tone={writing ? CHIP_PURPLE : CHIP_NEUTRAL}>
                {writing && <AgentOrb agent="writer" size={8} live={summary.running} dim={!summary.running} />}
                {statusText}
              </Chip>
              {summary.tags.map((tag) => <Chip key={tag} tone={CHIP_NEUTRAL}>{tag}</Chip>)}
            </span>
            <h1 ref={headingRef} tabIndex={-1} style={{ margin: 0, fontSize: 22, fontWeight: 600, letterSpacing: '-0.01em', lineHeight: 1.35 }}>{storyTitle(summary)}</h1>
          </div>
          <span style={{ display: 'flex', gap: 8, flex: '0 0 auto' }}>
            <a className="hov-ghost" href={`#${paths.quality(project, summary.story)}`} style={{
              height: 36, padding: '0 14px 0 10px', borderRadius: 10, border: '1px solid var(--line-4)',
              display: 'flex', alignItems: 'center', gap: 8, fontSize: 13, whiteSpace: 'nowrap', flex: '0 0 auto', color: 'var(--color-fg)',
            }}>
              <Icon name="chart-2" size={16} />{t.quality.title}
              {!!summary.openErrors && <span className="num" style={{ fontSize: 11.5, color: 'var(--kit-err-soft)' }}>{summary.openErrors}</span>}
            </a>
            <a className="hov-ghost" href={`#${paths.brain(project, summary.story)}`} title={t.brain.entryHint} style={{
              height: 36, padding: '0 14px 0 10px', borderRadius: 10, border: '1px solid var(--line-4)',
              display: 'flex', alignItems: 'center', gap: 8, fontSize: 13, whiteSpace: 'nowrap', flex: '0 0 auto', color: 'var(--color-fg)',
            }}>
              <Icon name="routing-2" size={16} />{t.brain.title}
            </a>
            <a className="hov-ghost" href={`#${paths.workspace(project, summary.story)}`} style={{
              height: 36, padding: '0 14px 0 10px', borderRadius: 10, border: '1px solid rgba(255,255,255,0.1)',
              display: 'flex', alignItems: 'center', gap: 8, fontSize: 13, whiteSpace: 'nowrap', flex: '0 0 auto', color: '#EDEDEF',
            }}>
              <Icon name="category" size={16} />Mở Workspace
            </a>
          </span>
        </div>
        <StoryTabs tab={tab} onTab={onTab} counts={{ cast: characters.data?.characters.length, world: foundation.world.data?.length, fs: foreshadow.data?.entries.length }} />
      </div>

      <div ref={scrollRef} id={`story-panel-${tab}`} role="tabpanel" aria-label={TABS.find((item) => item.id === tab)?.label}
        className="scroll" style={{ flex: '1 1 auto', minHeight: 0, padding: narrow ? '16px 14px 110px' : '16px 28px 110px' }}>
        {TABS.map((item) => <Activity key={`${summary.story}-${item.id}`} mode={tab === item.id ? 'visible' : 'hidden'}>{body(item.id)}</Activity>)}
      </div>
      <ScrollTopButton target={scrollRef} focusTarget={headingRef} label={t.scrollTop.page} size="md" style={{ right: 48, bottom: 92 }} />

      <Dock inputId="steer" maxWidth={760} icon="magic-star" iconColor="#C9A6F5" label="Dặn hướng cốt truyện"
        placeholder="Dặn hướng cốt truyện, ví dụ: đẩy tuyến tình cảm lên chương 4" sendLabel="Gửi"
        value={draft} onChange={setDraft} onSubmit={send} inputRef={dockRef}
        extra={
          <button className="hov-ghost" type="button" onClick={() => setDialog('cocreate')} style={{
            height: 34, padding: '0 12px', borderRadius: 9, border: '1px solid rgba(255,255,255,0.1)', background: 'transparent',
            color: '#EDEDEF', fontSize: 12.5, display: 'flex', alignItems: 'center', whiteSpace: 'nowrap',
          }}>Bàn kỹ với Kiến trúc sư</button>
        } />
    </main>
  )
}
