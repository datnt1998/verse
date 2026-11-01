// M11 · Truyện mới trên điện thoại (18-mobile.md › M11, artboard MNewStory.dc.html).
// Cùng luồng với màn máy tính: Bắt đầu nhanh dùng quickStoryStarter + useStoryCheck; Đồng sáng tác
// dùng phiên nháp cấp dự án (cocreateDraft); ba lối khác dùng đúng luồng “bắt đầu từ tệp”, “nhập bản
// thảo” và “mô phỏng văn phong” của máy tính. Khác biệt chỉ ở bố cục: không thanh tab, nút dính đáy
// nhảy lên khi bàn phím mở, và mọi lựa chọn mở ngăn dưới thay vì popover.
import { useEffect, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Icon, Segmented } from '@noverse/ui'
import { createStory, getConfig, getIdea, startFromFile } from '../../../api/client'
import { useLibrary } from '../../../api/library'
import { keys, useAction, useConnection, useResource } from '../../../data'
import { t } from '../../../i18n/vi'
import { navigate, paths, useRoute } from '../../../router'
import { projectLabel } from '../../../shell/labels'
import { useMobileRuntime } from '../../../mobile/runtime'
import { useProjectDetail } from '../../project/useProject'
import { quickStoryConfig, quickStoryStarter, storyNameFromFile, type QuickStoryChoices } from '../creation'
import { readProjectDraft } from '../cocreateDraft'
import { useSimulateStart } from '../simulateStart'
import { useStoryCheck } from '../useStoryCheck'
import { CoPane } from './CoPane'
import { QuickPane, type QuickAlternative } from './QuickPane'
import { SamplesSheet, StartFileSheet, StyleSheet, WriterSheet } from './sheets'
import { styles } from './styles'

type SheetKind = 'style' | 'writer' | 'startFile' | 'samples' | null

export function MobileNewStory({ project }: { project: string }) {
  const parsed = useRoute()
  const runtime = useMobileRuntime()
  const connection = useConnection()
  const { library } = useLibrary()
  const { detail } = useProjectDetail(project)

  const [mode, setMode] = useState<'quick' | 'cocreate'>(() => (
    parsed.search.get('mode') === 'cocreate' || (!parsed.search.has('brief') && readProjectDraft(project)) ? 'cocreate' : 'quick'
  ))
  const [prompt, setPrompt] = useState(() => parsed.search.get('brief') ?? '')
  const [choices, setChoices] = useState<QuickStoryChoices>({ style: null, writer: null })
  const [idea, setIdea] = useState<{ id: string; title: string } | null>(null)
  const [sheet, setSheet] = useState<SheetKind>(null)
  const [alternative, setAlternative] = useState<QuickAlternative>(null)

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
    setLoadedIdea(full.id)
    setIdea({ id: full.id, title: full.title || full.excerpt })
    setPrompt(full.text.slice(0, 5000))
  }, [ideaResource.data, loadedIdea])

  const [starter] = useState(quickStoryStarter)
  const gate = useStoryCheck(project, prompt, setPrompt)
  const simulate = useSimulateStart(project)

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
  const error = failure ? { message: t.placeholder.initFailedQuick, detail: failure instanceof Error ? failure.message : '' } : null
  const writable = connection.writable
  const projectName = projectLabel(detail?.project ?? library?.projects.find((item) => item.id === project), project)

  const back = () => { if (history.length > 1) history.back(); else navigate(paths.project(project), true) }

  const startFile = (path: string) => {
    setSheet(null)
    setAlternative('startFile')
    void alternativeAction.run(async (story) => {
      await startFromFile(project, story, path)
      navigate(paths.workspace(project, story), true)
    }, storyNameFromFile(path)).finally(() => setAlternative(null))
  }

  const importStory = () => {
    setAlternative('import')
    void alternativeAction.run(async (story) => {
      // M5 · Nhập bản thảo: mở đúng màn kết quả lệnh /import của truyện vừa tạo.
      const query = new URLSearchParams({ project, story, command: '/import' })
      navigate(`/cmd/new?${query.toString()}`, true)
    }).finally(() => setAlternative(null))
  }

  const simulateSamples = (samples: string[]) => {
    setAlternative('simulate')
    void simulate.start(samples).then((story) => {
      setAlternative(null)
      if (!story) return
      setSheet(null)
      navigate(paths.workspace(project, story), true)
    })
  }

  return <section {...stylex.props(styles.page)} aria-label={t.nav.newStory}>
    <header {...stylex.props(styles.topbar)}>
      <button type="button" onClick={back} aria-label="Quay lại" {...stylex.props(styles.back)}>
        <Icon name="arrow-left-2" size={22} />
      </button>
      <div {...stylex.props(styles.headCopy)}>
        <span {...stylex.props(styles.headTitle)}>{t.nav.newStory}</span>
        <span {...stylex.props(styles.headSub)}>{t.newStory.saveInto(projectName)}</span>
      </div>
    </header>

    {runtime.offline && <div role="status" {...stylex.props(styles.offline)}>
      <Icon name="wifi" size={15} />
      <span {...stylex.props(styles.offlineText)}>{t.newStory.offlineNote}</span>
    </div>}

    <Segmented label={t.nav.newStory} size={38} radius={9} xstyle={styles.mode}
      buttonStyle={styles.modeButton} value={mode}
      onChange={(id) => { if (!startAction.busy && !simulate.busy) setMode(id as 'quick' | 'cocreate') }}
      options={[
        { id: 'quick', label: t.newStory.modeQuick, icon: <Icon name="flash" size={16} />, disabled: startAction.busy || simulate.busy },
        { id: 'cocreate', label: t.newStory.modeCo, icon: <Icon name="messages-2" size={16} />, disabled: startAction.busy || simulate.busy },
      ]} />

    {mode === 'quick'
      ? <QuickPane project={project} gate={gate} prompt={prompt} onPrompt={setPrompt} choices={choices} config={configResource.data ?? null}
        idea={idea} onClearIdea={() => setIdea(null)} offline={!writable} error={error} busy={busy || !writable}
        starting={startAction.busy} alternativePending={alternativeAction.pending || simulate.busy} alternative={alternative}
        keyboard={runtime.keyboard} onOpenStyle={() => setSheet('style')} onOpenWriter={() => setSheet('writer')}
        onStart={() => void startAction.run()} onStartFile={() => setSheet('startFile')} onImport={importStory}
        onSimulate={() => setSheet('samples')} />
      : <CoPane key={project} project={project} writable={writable} keyboard={runtime.keyboard}
        onStarted={(story) => navigate(paths.workspace(project, story), true)} onCancel={() => setMode('quick')} />}

    <StyleSheet open={sheet === 'style'} onClose={() => setSheet(null)} project={project}
      config={configResource.data ?? null} choices={choices} onChoices={setChoices} />
    <WriterSheet open={sheet === 'writer'} onClose={() => setSheet(null)}
      config={configResource.data ?? null} choices={choices} onChoices={setChoices} />
    <StartFileSheet open={sheet === 'startFile'} onClose={() => setSheet(null)} busy={alternativeAction.pending}
      error={alternative === 'startFile' && alternativeAction.error instanceof Error ? alternativeAction.error.message : null}
      onSubmit={startFile} />
    <SamplesSheet open={sheet === 'samples'} onClose={() => setSheet(null)} busy={simulate.busy} error={simulate.error}
      onSubmit={simulateSamples} />
  </section>
}
