// screens/newstory — F3 (NewStory.dc.html): Bắt đầu nhanh · Đồng sáng tác.
// Quick: POST /stories {name?, idea?} → POST /input (start) → Workspace.
// CoCreate: POST /projects/:p/cocreate/message (nháp cấp dự án) … → /cocreate/commit → Workspace.
import { useEffect, useState } from 'react'
import {
  createStory, getConfig, getIdea, startFromFile,
} from '../../api/client'
import { useLibrary } from '../../api/library'
import { t } from '../../i18n/vi'
import { navigate, paths, useRoute } from '../../router'
import { useMediaQuery } from '../../shell/AppShell'
import { keys, useAction, useConnection, useResource } from '../../data'
import { useProjectDetail } from '../project/useProject'
import { projectLabel } from '../../shell/labels'
import { StartFileDialog } from './AltDialogs'
import { SimulateDialog } from './SimulateDialog'
import { useSimulateStart } from './simulateStart'
import { CoCreatePanel } from './CoCreate'
import { readProjectDraft } from './cocreateDraft'
import { ModeSwitch, type StoryMode } from './ModeSwitch'
import { QuickMode, QuickModeHeading, ShortcutHints } from './QuickMode'
import { StaleBanner } from '@noverse/ui'
import { useSaveShortcut } from '@noverse/ui'
import { quickStoryConfig, quickStoryStarter, type QuickStoryChoices } from './creation'
import { useStoryCheck } from './useStoryCheck'
import { PanelRefreshBar } from '../../ui/PanelRefreshBar'
import { useUnsavedGuard } from '../../ui/useUnsavedGuard'

export function NewStoryScreen({ project }: { project: string }) {
  const connection = useConnection()
  const { library } = useLibrary()
  const { detail } = useProjectDetail(project)
  const parsed = useRoute()
  const narrow = useMediaQuery('(max-width: 1099px)')

  const [mode, setMode] = useState<StoryMode>(() => (
    parsed.search.get('mode') === 'cocreate' || (!parsed.search.has('brief') && readProjectDraft(project)) ? 'cocreate' : 'quick'
  ))
  const [prompt, setPrompt] = useState(() => parsed.search.get('brief') ?? '')
  const [choices, setChoices] = useState<QuickStoryChoices>({ style: null, writer: null })
  const [idea, setIdea] = useState<{ id: string; title: string } | null>(null)
  const [activeAlternative, setActiveAlternative] = useState<'import' | 'simulate' | null>(null)
  const [startFileOpen, setStartFileOpen] = useState(false)
  // Gói AR · việc 6: Mô phỏng văn phong dán văn mẫu trước, rồi mới tạo truyện (dùng chung với M11).
  const [simulateOpen, setSimulateOpen] = useState(false)
  const simulate = useSimulateStart(project)

  const ideaParam = parsed.search.get('idea')
  const briefParam = parsed.search.get('brief')
  const [loadedBrief, setLoadedBrief] = useState(briefParam)
  useEffect(() => {
    if (briefParam === loadedBrief) return
    setLoadedBrief(briefParam)
    if (briefParam !== null) { setPrompt(briefParam); setMode('quick') }
  }, [briefParam, loadedBrief])

  const configResource = useResource(keys.config('project', project), ({ signal }) => getConfig('project', project, undefined, signal))
  const ideaResource = useResource(keys.project(project, 'ideas', ideaParam), () => getIdea(project, ideaParam!), { enabled: !!ideaParam })
  const [loadedIdea, setLoadedIdea] = useState<string | null>(null)
  useEffect(() => {
    const full = ideaResource.data
    if (!full || loadedIdea === full.id) return
    setLoadedIdea(full.id); setIdea({ id: full.id, title: full.title || full.excerpt }); setPrompt(full.text.slice(0, 5000))
  }, [ideaResource.data, loadedIdea])

  // Tab đổi chế độ (07-copy.md › phím tắt) — bỏ qua khi đang gõ hoặc đang ở control
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key !== 'Tab' || event.shiftKey || event.metaKey || event.ctrlKey || event.altKey) return
      const target = event.target as HTMLElement | null
      if (target && (target.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT', 'BUTTON', 'A'].includes(target.tagName))) return
      event.preventDefault()
      setMode((prev) => (prev === 'quick' ? 'cocreate' : 'quick'))
    }
    addEventListener('keydown', onKey)
    return () => removeEventListener('keydown', onKey)
  }, [])

  const [starter] = useState(quickStoryStarter)
  const gate = useStoryCheck(project, prompt, setPrompt)
  const startAction = useAction(async () => {
    if (!await gate.check()) return
    try {
      const story = await starter.start(project, prompt, idea?.id, quickStoryConfig(choices, configResource.data?.effective))
      setPrompt('')
      navigate(paths.workspace(project, story.story), true)
      return story
    } catch (error) {
      if (!gate.acceptConflict(error)) throw error
    }
  }, { invalidate: [keys.project(project), keys.library(), keys.jobs()] })
  const alternativeAction = useAction(async (action: (story: string) => Promise<void>, name?: string) => {
    const story = await createStory(project, name ? { name } : {})
    await action(story.story)
    return story
  }, { success: 'Đã tạo truyện', invalidate: [keys.project(project), keys.library(), keys.jobs()] })
  const busy = startAction.busy || alternativeAction.busy || simulate.busy || gate.checking || gate.suggesting
  const failure = startAction.error || alternativeAction.error
  const error = failure ? { message: t.placeholder.initFailed, detail: failure instanceof Error ? failure.message : '' } : null
  const start = () => { if (!busy && !gate.blocked && prompt.trim() && connection.writable) void startAction.run() }
  useUnsavedGuard(!!prompt.trim() && !busy)
  useSaveShortcut(start, !!prompt.trim() && !busy)
  const withNewStory = (action: (story: string) => Promise<void>, name?: string) => alternativeAction.run(action, name)

  const submitStartFile = (path: string) => {
    const base = path.split('/').pop()?.replace(/\.[^.]+$/, '') ?? ''
    setStartFileOpen(false)
    void withNewStory(async (story) => {
      await startFromFile(project, story, path)
      navigate(paths.workspace(project, story), true)
    }, base)
  }

  const projectName = projectLabel(detail?.project ?? library?.projects.find((item) => item.id === project), project)
  const committed = (storyId?: string) => {
    if (storyId) navigate(paths.workspace(project, storyId), true)
  }

  return (
    <main style={{
      flex: 1, minHeight: narrow ? 'auto' : 0, width: '100%', minWidth: 0, display: 'flex', flexDirection: 'column',
      alignItems: 'center', padding: '0 20px',
    }}>
      <PanelRefreshBar resourceKeys={[keys.config('project', project), keys.project(project, 'ideas', ideaParam)]} updatedAt={configResource.updatedAt} />
      <StaleBanner error={configResource.error || ideaResource.error || (connection.state === 'retrying' ? 'Mất luồng cập nhật' : null)} onRetry={() => { connection.retry(); void configResource.refresh(); void ideaResource.refresh() }} />
      <div className={mode === 'quick' && !narrow ? 'scroll' : undefined} style={{
        flex: narrow ? '1 1 auto' : 1, minHeight: narrow ? undefined : 0, width: '100%',
        display: 'flex', flexDirection: 'column', alignItems: 'center',
      }}>
        <div style={{
          flex: mode === 'cocreate' ? (narrow ? '1 1 auto' : 1) : undefined,
          minHeight: mode === 'cocreate' && !narrow ? 0 : undefined,
          width: '100%', maxWidth: mode === 'quick' ? 760 : 1160,
          margin: mode === 'quick' ? 'auto 0' : undefined,
          padding: mode === 'quick' ? '32px 0 24px' : '16px 0 12px',
          display: 'flex', flexDirection: 'column',
          alignItems: mode === 'quick' ? 'center' : undefined,
          gap: mode === 'quick' ? 22 : 12,
        }}>
          {mode === 'quick' && <QuickModeHeading checked={!!gate.state} />}
          <div style={{
            flex: '0 0 auto', display: 'flex', flexWrap: 'wrap', alignItems: 'center',
            alignSelf: mode === 'quick' ? undefined : 'stretch', gap: '10px 16px',
          }}>
            <ModeSwitch mode={mode} onChange={setMode} compact={mode === 'cocreate'} />
            {mode === 'cocreate' && <span style={{ fontSize: 13, color: 'var(--color-fg-3)' }}>{t.newStory.modeCoHint}</span>}
          </div>
      {mode === 'quick' ? (
        <QuickMode project={project} gate={gate} prompt={prompt} onPrompt={setPrompt} onStart={() => void start()}
          choices={choices} onChoices={setChoices}
          busy={busy || !connection.writable} pending={startAction.pending} alternativePending={alternativeAction.pending} activeAlternative={activeAlternative} success={startAction.success && !gate.blocked} error={error} projectName={projectName} offline={!connection.writable}
          config={configResource.data ?? null} idea={idea} onClearIdea={() => setIdea(null)}
          onStartFile={() => setStartFileOpen(true)}
          onImport={() => { setActiveAlternative('import'); void withNewStory(async (story) => {
            navigate(paths.workspace(project, story), true, 'import')
          }) }}
          onSimulate={() => setSimulateOpen(true)} />
      ) : (
        <>
          {error && (
            <div role="alert" style={{
              display: 'flex', alignItems: 'flex-start', gap: 8, padding: '10px 14px', borderRadius: 12,
              background: 'var(--tint-err)', border: '1px solid var(--color-err)', color: 'var(--color-err)', fontSize: 13,
            }}>
              <span style={{ flex: 1 }}>
                <b style={{ fontWeight: 500 }}>{error.message}</b>
                {error.detail && <span style={{ display: 'block', color: 'var(--color-fg-3)', fontSize: 12 }}>{error.detail}</span>}
              </span>
            </div>
          )}
          <CoCreatePanel project={project} narrow={narrow}
            onCommitted={committed}
            onCancel={() => { /* phiên nháp đã bị xoá; ở lại màn Truyện mới */ }} />
        </>
      )}
        </div>
      </div>

      <ShortcutHints mode={mode} />

      {startFileOpen && (
        <StartFileDialog onClose={() => setStartFileOpen(false)} onSubmit={submitStartFile} busy={busy || !connection.writable} pending={alternativeAction.pending} />
      )}

      {simulateOpen && (
        <SimulateDialog onClose={() => { setSimulateOpen(false); simulate.clearError() }} busy={simulate.busy || !connection.writable} error={simulate.error}
          onSubmit={(samples) => { void simulate.start(samples).then((story) => {
            if (!story) return
            setSimulateOpen(false)
            navigate(paths.workspace(project, story), true)
          }) }} />
      )}
    </main>
  )
}
