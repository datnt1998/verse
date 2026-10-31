// WorkspaceScreen (#/p/:p/s/:s/work) — 1:1 Main.dc.html (phase rewrite · gate · done).
// 3 cột desktop: Đại cương 236 · cột giữa (RunHeader · Nhật ký 4 · Luồng viết 6 · Composer)
// · Inspector 340. Mobile <760: một pane + thanh tab đáy riêng + Composer sticky.
// Hành động: input/steer/continue · abort · resume · next · review · reopen · export.
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  ApiError, abortStory, canWriteStory, confirmImport, downloadExport, getCoCreate, getOutline, getReviews,
  approveWithComments, nextChapter, paragraphComments, resumeStory, sendCoCreate, sendInput, setReview,
  cancelCoCreate, commitCoCreate, getChapters, listStories,
} from '../../api/client'
import type { HostEvent } from '../../api/types'
import { useSession } from '../../api/session'
import { useStoryEvents } from '../../api/useStoryEvents'
import { useActivityHistory } from '../../api/useActivityHistory'
import { takeComposerDraft } from '../../shell'
import { storyLabel } from '../../shell/labels'
import { setConnection, setLocked, usePanels } from '../../shell/stores'
import { useMediaQuery } from '../../shell'
import { t } from '../../i18n/vi'
import { translateHost } from '../../i18n/hostStrings'
import { navigate, paths, setDialog } from '../../router'
import { Icon } from '@noverse/ui'
import { toast } from '@noverse/ui'
import { cache, invalidate, keys, setData, useAction, useConnection, useResource } from '../../data'
import { Banner, useSaveShortcut } from '@noverse/ui'
import { confirmPause } from '../../ui/confirmPresets'
import { PanelRefreshBar } from '../../ui/PanelRefreshBar'
import { useUnsavedGuard } from '../../ui/useUnsavedGuard'
import { ActivityFeed } from './ActivityFeed'
import { CocreatePanel } from './CocreatePanel'
import { Composer, ShortcutHints, composerPlaceholder } from './Composer'
import { commandEvent, mergeCommandLog } from './events'
import { GateCard, isCompleted, isGate, latestGateChapter, pendingCommentCount } from './GateCard'
import { Inspector, type InspectorTab } from './Inspector'
import { BrainDrawer } from './BrainDrawerPanel'
import { openBrainDrawer } from './brainDrawer'
import { JobProgress } from './JobProgress'
import { PlanPanel } from './PlanPanel'
import { CommandResults, useCommandResults } from '../cmdresults'
import { LiveStream } from './LiveStream'
import { idleModel, type IdleActions } from './IdleCard'
import { OutlineRail } from './OutlineRail'
import { RunHeader } from './RunHeader'
import { helpPath, slashRoute } from './slash'
import { useShellChrome } from '../../shell'
import { WorkspaceFeedback } from './WorkspaceFeedback'
import { CandidateProjectNotice, useCandidateProject } from '../project/CandidateProject'

const NOOP = () => {}

type MobileTab = 'activity' | 'stream' | 'agent' | 'cost' | 'story'

const MOBILE_TABS: { id: MobileTab; label: string; icon: string }[] = [
  { id: 'activity', label: t.workspace.mobileTabs.activity, icon: 'activity' },
  { id: 'stream', label: t.workspace.mobileTabs.stream, icon: 'magic-star' },
  { id: 'agent', label: t.workspace.mobileTabs.agent, icon: 'magicpen' },
  { id: 'cost', label: t.workspace.mobileTabs.cost, icon: 'coin-1' },
  { id: 'story', label: t.workspace.mobileTabs.story, icon: 'book-1' },
]

function MobilePaneTabs({ value, onChange }: { value: MobileTab; onChange: (tab: MobileTab) => void }) {
  return (
    <div role="tablist" aria-label={t.nav.detailPanel} style={{
      display: 'grid', gridTemplateColumns: `repeat(${MOBILE_TABS.length}, minmax(0, 1fr))`,
      gap: 2, padding: 3, borderRadius: 10, background: 'var(--color-surface-1)', border: '1px solid var(--line-1)',
    }}>
      {MOBILE_TABS.map((tab) => {
        const on = tab.id === value
        return (
          <button key={tab.id} type="button" role="tab" aria-selected={on} onClick={() => onChange(tab.id)}
            style={{
              minWidth: 0, minHeight: 44, padding: '4px 0', border: 'none', borderRadius: 7,
              background: on ? 'var(--color-surface-6)' : 'transparent',
              color: on ? 'var(--color-fg)' : 'var(--color-fg-3)',
              display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', gap: 3,
              fontSize: 10.5, fontWeight: on ? 600 : 400,
            }}>
            <Icon name={tab.icon} size={16} />
            <span style={{ whiteSpace: 'nowrap' }}>{tab.label}</span>
          </button>
        )
      })}
    </div>
  )
}

export function WorkspaceScreen({ project, story }: { project: string; story: string }) {
  const { user } = useSession()
  const { snapshot, events, streamText, cocreateText, jobs, connected, blocked, reconnectAt, retryNow,
    loading, refreshing, updatedAt, error, refresh } =
    useStoryEvents(project, story, NOOP)
  const history = useActivityHistory(project, story, { limit: 40 }, events)
  const panels = usePanels()
  const mobile = useMediaQuery('(max-width: 759px)')
  const shellChrome = useShellChrome()
  const [text, setText] = useState('')
  const [tab, setTab] = useState<InspectorTab>('agent')
  const [mobileTab, setMobileTab] = useState<MobileTab>('activity')
  const connection = useConnection()
  const canEdit = !!user && canWriteStory(user, `${project}/${story}`)
  const storyList = useResource(keys.project(project, 'stories'), () => listStories(project))
  const candidate = useCandidateProject(project, storyList.error)
  const storyTitle = storyLabel(storyList.data?.stories.find((item) => item.story === story) ?? { story })
  const chapters = useResource(keys.story(project, story, 'chapters'), ({ signal }) => getChapters(project, story, signal))
  const outlineResource = useResource(keys.story(project, story, 'outline'), ({ signal }) => getOutline(project, story, signal))
  const outline = outlineResource.data ?? null
  const cocreateKey = [...keys.story(project, story, 'snapshot'), 'cocreate']
  const cocreateResource = useResource(cocreateKey, ({ signal }) => getCoCreate(project, story, signal), { enabled: canEdit })
  const cocreate = cocreateResource.data ?? null
  // G13: lỗi/kết quả lệnh server trả về mà SSE không phát thành sự kiện riêng.
  const [commandLog, setCommandLog] = useState<HostEvent[]>([])
  const commandSeq = useRef(0)
  const pushCommandResult = useCallback((outcome: { message?: string; error?: string }) => {
    const event = commandEvent(++commandSeq.current, outcome)
    if (event) setCommandLog((previous) => [...previous, event].slice(-20))
  }, [])
  const feed = useMemo(() => mergeCommandLog([...history.events].reverse(), commandLog), [history.events, commandLog])
  const commandResults = useCommandResults(project, story)

  const writable = canEdit && connection.writable && connected
  const running = !!snapshot?.isRunning
  const paused = snapshot?.runtimeState === 'paused'
  // Gói P · P1: khung “Kế hoạch truyện” thay luồng viết; nhật ký 3 / kế hoạch 7.
  const planning = !!snapshot?.plan
  const completed = isCompleted(snapshot)
  const gate = isGate(snapshot)
  const gateChapter = latestGateChapter(snapshot, outline)
  const comments = useResource(keys.story(project, story, 'comments', gateChapter),
    ({ signal }) => paragraphComments(project, story, gateChapter, signal), { enabled: gate && gateChapter > 0 })
  // D4: thẩm duyệt mới nhất của chương đang chờ — dòng phụ khung “Thẩm duyệt chương N”
  // và điều kiện mở cửa duyệt (còn mục Bắt sửa thì đóng).
  const gateReviewResource = useResource([...keys.story(project, story, 'reviews'), 'gate', gateChapter],
    ({ signal }) => getReviews(project, story, gateChapter, signal).then((result) => result.chapter ?? null),
    { enabled: gate && gateChapter > 0 })
  const gateReview = gateReviewResource.data ?? null
  const pendingComments = pendingCommentCount(comments.data?.comments ?? [])
  useUnsavedGuard(text.trim() !== '')

  // Composer prefill một lần (Trình đọc điền sẵn lệnh — không tự gửi)
  useEffect(() => {
    const draft = takeComposerDraft(project, story)
    if (draft) setText(draft)
  }, [project, story])

  useEffect(() => {
    setConnection(error || connected ? null : reconnectAt, retryNow)
    return () => setConnection(null)
  }, [connected, reconnectAt, retryNow, error])

  useEffect(() => {
    setLocked(blocked === 'locked')
    return () => setLocked(false)
  }, [blocked])

  // Luồng worker làm mới cache; không có tải lại định kỳ cục bộ.
  const streamingCocreate = cocreateText.length > 0
  useEffect(() => {
    if (streamingCocreate) void invalidate(cocreateKey)
  }, [streamingCocreate, project, story])

  // Tab mặc định: Hoàn thành → Truyện (artboard), còn lại → Agent
  useEffect(() => { setTab(completed ? 'story' : 'agent') }, [completed, project, story])

  const miscAction = useAction(async (action: () => Promise<unknown>) => { await action(); return true },
    { success: 'Đã thực hiện', invalidate: [keys.story(project, story)] })
  const act = miscAction.run
  const pauseAction = useAction((afterStep: boolean) => abortStory(project, story, afterStep), {
    success: 'Đã yêu cầu tạm dừng Người viết',
    undo: async () => { await resumeStory(project, story) },
    invalidate: [keys.story(project, story, 'snapshot')],
  })
  const resumeAction = useAction(() => resumeStory(project, story), {
    success: 'Đã tiếp tục Người viết', invalidate: [keys.story(project, story, 'snapshot')],
  })
  const approveAction = useAction(() => gateChapter > 0
    ? approveWithComments(project, story, gateChapter) : nextChapter(project, story), {
    success: 'Đã duyệt chương', invalidate: [keys.story(project, story)],
  })
  const rejectAction = useAction(async () => {
    const feedback = text.trim()
    await sendInput(project, story, feedback)
    setText((current) => current.trim() === feedback ? '' : current)
    return true
  }, { success: 'Đã gửi ý kiến chỉnh sửa', invalidate: [keys.story(project, story)] })
  const pause = useCallback(() => {
    void confirmPause({
      body: 'Người viết sẽ lưu kết quả và tạm dừng theo lựa chọn của bạn. Bạn có thể chạy tiếp bất cứ lúc nào.',
      choices: [
        { value: 'after', label: 'Tạm dừng sau bước này', description: 'Khuyên dùng — giữ nguyên phần đã viết' },
        { value: 'now', label: 'Tạm dừng ngay', description: 'Dừng phần đang thực hiện' },
      ],
      defaultChoice: 'after', successMessage: undefined,
      onConfirm: async (choice) => {
        const result = await pauseAction.run(choice !== 'now')
        if (!result) throw new Error('Không tạm dừng được Người viết. Vui lòng thử lại.')
      },
    }, { user: user?.username })
  }, [pauseAction.run, user?.username])
  const resume = () => { void resumeAction.run() }
  const next = () => { void approveAction.run() }
  const reject = () => {
    if (!text.trim()) { document.getElementById('workspace-composer')?.focus(); return }
    void rejectAction.run()
  }
  const reviewAction = useAction((on: boolean) => setReview(project, story, on), {
    success: 'Đã đổi chế độ nghiệm thu', invalidate: [keys.story(project, story, 'snapshot')],
    optimistic: (on) => {
      const key = keys.story(project, story, 'snapshot')
      const previous = cache.setData(key, snapshot ? { ...snapshot, advanceMode: on ? 'review' : 'auto' } : snapshot)
      return () => cache.restore(key, previous)
    },
  })
  const toggleReview = () => { if (!reviewAction.busy) void reviewAction.run(snapshot?.advanceMode !== 'review') }

  // Esc tạm dừng (07-copy) · `/` mở bảng lệnh · G mở ngăn kéo Não truyện (Gói P3) — bỏ qua khi đang gõ.
  useEffect(() => {
    const anyDialog = () => !!document.querySelector('[role="dialog"],[role="alertdialog"]')
    const onKey = (event: KeyboardEvent) => {
      const target = event.target as HTMLElement | null
      const typing = !!target && (['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName) || target.isContentEditable)
      if (event.key === 'Escape' && running && writable && !anyDialog()) {
        pause()
        return
      }
      if (event.key === '/' && canEdit && !typing && !anyDialog()) {
        event.preventDefault()
        setDialog('palette')
        return
      }
      if ((event.key === 'g' || event.key === 'G') && !typing && !event.metaKey && !event.ctrlKey && !event.altKey && !anyDialog()) {
        event.preventDefault()
        openBrainDrawer(undefined, snapshot ? (snapshot.inProgressChapter || snapshot.currentChapter) : null)
      }
    }
    addEventListener('keydown', onKey)
    return () => removeEventListener('keydown', onKey)
  }, [running, writable, pause, canEdit, snapshot])

  const sendCoCreateText = async (value: string) => {
    const reply = await sendCoCreate(project, story, value)
    setData(cocreateKey, cocreate
      ? { ...cocreate, active: true, ready: reply.ready, draft: reply.draft, suggestions: reply.suggestions }
      : { active: true, messages: [], draft: reply.draft, ready: reply.ready, suggestions: reply.suggestions })
  }

  /** Giữ bản nháp nếu gửi lỗi hoặc người dùng đã tiếp tục gõ trong lúc chờ. */
  const submitAction = useAction(async (raw: string) => {
    const value = raw.trim()
    if (cocreate?.active) {
      await sendCoCreateText(value)
    } else if (value.startsWith('/')) {
      const route = slashRoute(value)
      if (route.kind === 'help') { setText(''); navigate(helpPath()); return true }
      if (route.kind === 'dialog') { setText(''); setDialog(route.dialog); return true }
      if (route.kind === 'cocreate') {
        await sendInput(project, story, value)
      } else {
        try {
          const result = await commandResults.run(value)
          pushCommandResult({ message: result.message })
        } catch (error) {
          if (error instanceof ApiError) pushCommandResult({ error: translateHost(error.message) })
          throw error
        }
      }
    } else {
      await sendInput(project, story, value)
    }
    setText((current) => current.trim() === value ? '' : current)
    return true
  }, { success: 'Đã gửi', invalidate: [keys.story(project, story)] })
  const submit = (value: string) => {
    if (writable && value.trim() && value.length <= 5000) void submitAction.run(value)
  }
  useSaveShortcut(() => submit(text), writable && !!text.trim() && !submitAction.busy)
  const busy = miscAction.busy || pauseAction.busy || resumeAction.busy || approveAction.busy || rejectAction.busy || submitAction.busy || reviewAction.busy

  const onExport = useCallback(() => void act(async () => {
    const file = await downloadExport(project, story, 'txt')
    toast.ok(`${t.workspace.jobLabel.export}: ${file.file}`)
  }), [act, project, story])
  const onReopen = useCallback(() => setDialog('reopen'), [])
  const onConfirmJob = useCallback(() => void act(() => confirmImport(project, story, true)), [act, project, story])

  // Gói P · P4 — nút của thẻ trạng thái rảnh. Không thêm hành động mới: dùng
  // đúng next/duyệt chương, ô nhập (sendInput), Trình đọc, Audio, Bản xuất.
  // “Xem kế hoạch” mở màn Truyện › Dàn ý (?tab=outline là tab mặc định, không có route riêng).
  // Câu hỏi đang chờ (nếu có) — dùng cho dòng nhật ký cuối tô cam.
  const idleQuestion = snapshot?.pendingAsk?.question ?? (snapshot?.hasAdvanceHold ? snapshot.advanceHoldReason : '')
  const idle = !!snapshot && !!idleModel(snapshot)
  const idleActions: IdleActions = {
    onStart: next,
    onPlan: () => navigate(`${paths.story(project, story)}?tab=outline`),
    onBrain: () => openBrainDrawer(undefined, snapshot ? (snapshot.inProgressChapter || snapshot.currentChapter) : null),
    onRead: (chapter) => navigate(paths.reader(project, story, chapter)),
    onAudio: () => navigate(paths.audio(project, story)),
    onExport,
    onReply: (value) => submit(value),
  }

  const header = (
    <RunHeader snapshot={snapshot} story={storyTitle} running={running} paused={paused} writable={writable}
      onPause={pause} onResume={resume} onExport={onExport} onReopen={onReopen} onToggleReview={toggleReview}
      pending={pauseAction.pending || resumeAction.pending} success={pauseAction.success || resumeAction.success} busy={busy} />
  )
  const openGateReader = () => navigate(paths.reader(project, story, gateChapter || undefined))
  const gateCard = canEdit && gate && snapshot ? <GateCard snapshot={snapshot} onNext={next} onReject={reject}
    pendingComments={pendingComments}
    commentsLoading={outlineResource.initialLoading || (gateChapter > 0 && comments.initialLoading) || gateReviewResource.initialLoading}
    review={gateReview} error={approveAction.error}
    pending={approveAction.pending} success={approveAction.success} rejecting={rejectAction.pending} disabled={!writable || busy} /> : null
  const jobsCard = <JobProgress jobs={jobs} trackedJobs={commandResults.trackedJobs} onConfirm={onConfirmJob}
    canConfirm={canEdit} busy={miscAction.busy || !writable} pending={miscAction.pending} success={miscAction.success} />
  const cocreateCard = canEdit && cocreate?.active
    ? (
      <CocreatePanel state={cocreate} liveText={cocreateText} busy={busy || !writable}
        pending={miscAction.pending || submitAction.pending} success={miscAction.success || submitAction.success}
        onSend={(value) => { void submitAction.run(value) }}
        onCommit={() => void act(() => commitCoCreate(project, story, cocreate.draft || undefined))}
        onCancel={() => void act(() => cancelCoCreate(project, story))} />
    )
    : null
  const composer = canEdit ? (
    <Composer value={text} onChange={setText} onSubmit={(line) => void submit(line ?? text)} onPause={pause}
      running={running} writable={writable} canWrite={canEdit} mobile={mobile} snapshot={snapshot}
      busy={busy} pending={submitAction.pending} success={submitAction.success}
      placeholder={composerPlaceholder(snapshot, !!cocreate?.active, mobile)} />
  ) : <Banner tone="info" title={t.locked.readonly}
    action={{ label: 'Mở Trình đọc', onClick: () => navigate(paths.reader(project, story)) }}>
    Bạn chỉ có quyền đọc truyện này — không gửi yêu cầu sửa được.
  </Banner>
  const feedback = <>
    <PanelRefreshBar resourceKeys={[keys.story(project, story), keys.project(project, 'stories')]} updatedAt={history.updatedAt ?? updatedAt} style={{ minHeight: 2, height: 2, overflow: 'hidden' }} />
    <WorkspaceFeedback admin={user?.role === 'admin'}
      error={history.error || error || outlineResource.error || chapters.error || (candidate ? null : storyList.error) || cocreateResource.error || comments.error || (!connection.writable || !connected ? new Error('Không kết nối được máy chủ. Vui lòng thử lại.') : null)}
      onRetry={() => { retryNow(); connection.retry(); void refresh(); void history.refresh(); void outlineResource.refresh(); void chapters.refresh(); if (!candidate) void storyList.refresh(); void cocreateResource.refresh(); void comments.refresh() }} />
    <CandidateProjectNotice project={project} error={storyList.error} />
  </>

  if (blocked === 'forbidden') {
    return (
      <main style={{ flex: 1, minWidth: 0, display: 'flex', alignItems: 'center', justifyContent: 'center', padding: 24 }}>
        <div className="card" style={{ borderRadius: 14, padding: 20, display: 'flex', alignItems: 'center', gap: 10, color: 'var(--color-fg-3)' }}>
          <Icon name="lock" size={18} />
          {t.workspace.loadError}
        </div>
      </main>
    )
  }

  if (mobile) {
    const pane = (() => {
      if (mobileTab === 'activity') {
        return <ActivityFeed events={feed} project={project} story={story} loading={loading || history.initialLoading} idle={idle} question={idleQuestion} mobile total={snapshot?.totalChapters}
          results={<CommandResults ctl={commandResults} jobs={jobs} project={project} story={story} />} />
      }
      if (mobileTab === 'stream') {
        return planning
          ? <PlanPanel snapshot={snapshot} loading={loading} mobile />
          : <LiveStream project={project} story={story} streamText={streamText} isRunning={running} snapshot={snapshot} mobile idleActions={idleActions}
            gate={gate} review={gateReview} reviewLoading={gateReviewResource.initialLoading}
            gateChapter={gateChapter} onOpenReader={openGateReader} />
      }
      return (
        <Inspector snapshot={snapshot} project={project} story={story}
          tab={mobileTab === 'agent' || mobileTab === 'cost' || mobileTab === 'story' ? mobileTab : 'agent'}
          onTab={(nextTab) => setMobileTab(nextTab)} running={running} paused={paused} loading={loading} mobile />
      )
    })()
    const chrome = shellChrome.header + shellChrome.tabbar
    return (
      <main style={{
        flex: '1 1 auto', minWidth: 0, minHeight: 0, overflow: 'hidden',
        height: chrome > 0 ? `calc(100dvh - ${chrome}px)` : undefined,
        display: 'flex', flexDirection: 'column', padding: '10px 12px 0',
      }}>
        <div className="scroll" style={{ flex: 1, minHeight: 0, display: 'flex', flexDirection: 'column', gap: 10, paddingBottom: 8 }}>
          {header}
          {feedback}
          {gateCard}
          {jobsCard}
          {cocreateCard}
          {mobileTab === 'activity' || mobileTab === 'stream'
            ? <div style={{ flex: '1 1 0', minHeight: 0, display: 'flex', flexDirection: 'column' }}>{pane}</div>
            : pane}
        </div>
        <div style={{
          flex: '0 0 auto', paddingTop: 6, paddingBottom: 6,
          display: 'flex', flexDirection: 'column', gap: 6, background: 'var(--color-bg)',
        }}>
          <MobilePaneTabs value={mobileTab} onChange={setMobileTab} />
          {composer}
        </div>
      </main>
    )
  }

  return (
    <>
      {panels.left && (
        <OutlineRail snapshot={snapshot} outline={outline} chapters={chapters.data?.chapters} project={project} story={story}
          loading={outlineResource.initialLoading} error={!!outlineResource.error} onRetry={() => void outlineResource.refresh()} />
      )}
      <main style={{
        flex: '1 1 520px', minWidth: 0, minHeight: 0, overflow: 'hidden',
        padding: '12px 18px 10px', display: 'flex', flexDirection: 'column', gap: 10,
      }}>
        {header}
        {feedback}
        {gateCard}
        {jobsCard}
        {cocreateCard}
        <div style={{ flex: 1, minHeight: 0, display: 'flex', flexDirection: 'column', gap: 10 }}>
          <ActivityFeed events={feed} project={project} story={story} loading={loading || history.initialLoading} planning={planning} idle={idle} question={idleQuestion} total={snapshot?.totalChapters}
            results={<CommandResults ctl={commandResults} jobs={jobs} project={project} story={story} />} />
          {planning
            ? <PlanPanel snapshot={snapshot} loading={loading} />
            : <LiveStream project={project} story={story} streamText={streamText} isRunning={running} snapshot={snapshot} idleActions={idleActions}
              gate={gate} review={gateReview} reviewLoading={gateReviewResource.initialLoading}
              gateChapter={gateChapter} onOpenReader={openGateReader} />}
        </div>
        <div style={{ flex: '0 0 auto', display: 'flex', flexDirection: 'column', gap: 6 }}>
          {composer}
          {canEdit && <ShortcutHints snapshot={snapshot} running={running} />}
        </div>
      </main>
      {panels.right && (
        <Inspector snapshot={snapshot} project={project} story={story} tab={tab} onTab={setTab}
          running={running} paused={paused} loading={loading} />
      )}
      <BrainDrawer project={project} story={story} />
    </>
  )
}
