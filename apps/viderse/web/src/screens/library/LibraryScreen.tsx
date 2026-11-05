import { color, line } from '@noverse/ui/tokens.stylex'
import { libraryVars } from './libraryTokens.stylex'
import { space } from '../../styles/viderse.stylex'
// Thư viện (artboard Library): 4 mục dùng chung toàn bề ngang, đầu trang một
// hành động chính theo tab, tab Template / Prompt ảnh / Video nền / Lớp phủ.
// Không còn cột kênh bên trái — lên thanh trên từ đợt 42. Video nền và phần nền
// của template do mảng Video nền dựng (BackdropsTab.tsx,
// template/BackdropSection.tsx) — ở đây chỉ gắn khung.
import { useRef, useState } from 'react'
import { Banner, Button, Dialog, FieldLabel, Icon, Tabs, TextField } from '@noverse/ui'
import { toast } from '@noverse/ui'
import { invalidate } from '@noverse/ui/data'
import { t } from '../../i18n/library'
import { navigate, paths, type LibraryTab, type PromptView, type Route } from '../../router'
import { useResource } from '../../data/useResource'
import { keys } from '../../data/keys'
import {
  createTemplate, fieldErrors, importPrompt, listOverlays, listOwnerChannels, listPrompts, listTemplates,
} from '../../api/library'
import { listBackdropChannels, listBackdropVideos } from '../../api/backdrop'
import { ActionButton } from '../../components'
import { useMediaQuery } from '../../lib/useMediaQuery'
import { BackdropsTab } from './backdrops/BackdropsTab'
import { OverlaysTab } from './OverlaysTab'
import { PromptsTab } from './PromptsTab'
import { TemplateImportDialog } from './TemplateImportDialog'
import { TemplatesTab } from './TemplatesTab'
import { MLibraryScreen } from './mobile/MLibraryScreen'
import * as stylex from '@stylexjs/stylex'

export function LibraryScreen({ route }: { route: Extract<Route, { name: 'library' }> }) {
  const mobile = useMediaQuery('(max-width: 767px)')
  if (mobile) return <MLibraryScreen route={route} />
  return <DesktopLibraryScreen route={route} />
}

function DesktopLibraryScreen({ route }: { route: Extract<Route, { name: 'library' }> }) {
  const copy = t.library
  const tab: LibraryTab = route.tab ?? 'templates'
  const templates = useResource(keys.templates(), ({ signal }) => listTemplates(signal))
  const prompts = useResource(keys.prompts('', ''), ({ signal }) => listPrompts({}, signal))
  const overlays = useResource(keys.overlays(), ({ signal }) => listOverlays(signal))
  const channels = useResource(keys.ownerChannels(), ({ signal }) => listOwnerChannels(signal))
  const backdropChannels = useResource(keys.backdropChannels(), ({ signal }) => listBackdropChannels(signal))
  const backdropVideos = useResource(keys.backdropVideos(), ({ signal }) => listBackdropVideos(signal))
  const ownerChannels = channels.data?.channels ?? []
  const allPrompts = prompts.data?.items ?? []

  const [createOpen, setCreateOpen] = useState(false)
  const [newName, setNewName] = useState('')
  const [createError, setCreateError] = useState<string | null>(null)
  const [createFieldError, setCreateFieldError] = useState<string | null>(null)
  const [importing, setImporting] = useState(false)
  const [importError, setImportError] = useState<string | null>(null)
  const [templateImportOpen, setTemplateImportOpen] = useState(false)
  const [importFiles, setImportFiles] = useState<File[]>([])
  const importInput = useRef<HTMLInputElement>(null)

  const counts: Record<LibraryTab, number | undefined> = {
    templates: templates.data?.items.length,
    prompts: prompts.data?.items.length,
    overlays: overlays.data?.items.length,
    backdrops: backdropChannels.data && backdropVideos.data
      ? backdropChannels.data.length + backdropVideos.data.length
      : undefined,
  }

  const openCreate = () => { setCreateOpen(true); setNewName(''); setCreateError(null); setCreateFieldError(null) }

  const create = async () => {
    setCreateError(null)
    setCreateFieldError(null)
    try {
      const created = await createTemplate({ name: newName })
      await invalidate(keys.templates())
      setCreateOpen(false)
      navigate(paths.template(created.id, 'images'))
    } catch (err) {
      const fields = fieldErrors(err)
      if (fields.name) setCreateFieldError(fields.name)
      else setCreateError(err instanceof Error ? err.message : String(err))
    }
  }

  const pickImport = () => { setImportError(null); importInput.current?.click() }

  const runImport = async (file: File | undefined) => {
    if (!file) return
    setImporting(true)
    setImportError(null)
    try {
      const imported = await importPrompt(file)
      await invalidate(keys.prompts('', ''))
      toast.ok(copy.prompts.imported(imported.name))
      navigate(paths.library('prompts', 'mine', { promptId: imported.id }))
    } catch (err) {
      setImportError(err instanceof Error ? err.message : String(err))
    } finally {
      setImporting(false)
      if (importInput.current) importInput.current.value = ''
    }
  }

  const primary = tab === 'templates'
    ? { label: copy.templates.newTemplate, icon: <Icon name="add" size={16} />, onClick: openCreate }
    : tab === 'prompts'
      ? { label: copy.prompts.newPrompt, icon: <Icon name="add" size={16} />, onClick: () => navigate(paths.library('prompts', 'new')) }
      : null

  return (
    <div className={`vlb-shell ${stylex.props(sx.shell).className}`}>
      <div className={`vlb-body vv-wide vv-wide--grid ${stylex.props(sx.body).className}`}>
        <header className={`vlb-head ${stylex.props(sx.head).className}`}>
          <div className={`vlb-head-main ${stylex.props(sx.headMain).className}`}>
            <h1 {...stylex.props(sx.vlbHeadMainH1)}>{copy.heading}</h1>
            <p className={`vlb-hint ${stylex.props(sx.hint).className}`}>{ownerChannels.length ? copy.subtitle(ownerChannels.length) : copy.subtitleEmpty}</p>
          </div>
          <div className={`vlb-head-actions ${stylex.props(sx.headActions).className}`}>
            {tab === 'templates' && (
              <ActionButton xstyle={[sx.vlbHeadActionsActionButton]} variant="outline" icon={<Icon name="download" size={16} />} onClick={() => { setImportFiles([]); setTemplateImportOpen(true) }}>
                {copy.templates.importTemplate}
              </ActionButton>
            )}
            {tab === 'prompts' && (
              <>
                <input ref={importInput} type="file" accept=".json,application/json" hidden
                  onChange={(event) => void runImport(event.currentTarget.files?.[0])} />
                <ActionButton xstyle={[sx.vlbHeadActionsActionButton]} variant="outline" icon={<Icon name="download" size={16} />} pending={importing} onClick={pickImport}>
                  {copy.prompts.importPrompt}
                </ActionButton>
              </>
            )}
            {primary && <ActionButton xstyle={[sx.vlbHeadActionsActionButton]} variant="primary" icon={primary.icon} onClick={primary.onClick}>{primary.label}</ActionButton>}
          </div>
        </header>

        {importError && <Banner tone="error" title={importError} />}

        <div {...stylex.props(sx.tabsFrame)}>
          <Tabs
            variant="underline"
            label={copy.tabAria}
            value={tab}
            onChange={(id) => navigate(paths.library(id as LibraryTab))}
            tabs={{ id: 'vlb-tabs', panel: 'vlb-panel' }}
            items={(Object.keys(copy.tabs) as LibraryTab[]).map((id) => ({
              id,
              label: copy.tabs[id],
              dot: counts[id] !== undefined ? <span {...stylex.props(sx.tabCount)}>{counts[id]}</span> : undefined,
            }))}
          />
        </div>

        <div id="vlb-panel" role="tabpanel" aria-labelledby={`vlb-tabs-${tab}`} className={`vlb-panel ${stylex.props(sx.panel).className}`}>
          {tab === 'templates' && <TemplatesTab channels={ownerChannels} onImport={(files) => { setImportFiles(files); setTemplateImportOpen(true) }} />}
          {tab === 'prompts' && (
            <PromptsTab
              view={route.prompt}
              promptId={route.promptId}
              all={allPrompts}
              allLoading={prompts.initialLoading}
              allError={prompts.error}
              onView={(view: PromptView | undefined, promptId?: string) => navigate(paths.library('prompts', view, { promptId }))}
            />
          )}
          {tab === 'backdrops' && <BackdropsTab />}
          {tab === 'overlays' && <OverlaysTab />}
        </div>
      </div>

      {templateImportOpen && <TemplateImportDialog initialFiles={importFiles} onClose={() => { setTemplateImportOpen(false); setImportFiles([]) }} />}

      {createOpen && (
        <Dialog title={copy.templates.createTitle} onClose={() => setCreateOpen(false)} width={440}
          footer={<>
            <Button size="sm" onClick={() => setCreateOpen(false)}>{copy.cancel}</Button>
            <Button size="sm" variant="primary" onClick={() => void create()}>{copy.templates.createAction}</Button>
          </>}>
          <p className={`vlb-hint ${stylex.props(sx.hint2).className}`}>{copy.templates.createBody}</p>
          <label className={`vlb-field ${stylex.props(sx.field).className}`}>
            <FieldLabel htmlFor="vlb-new-name">{copy.nameLabel}</FieldLabel>
            <TextField id="vlb-new-name" autoFocus value={newName} placeholder={copy.templates.createPlaceholder}
              aria-invalid={createFieldError ? true : undefined}
              onChange={setNewName}
              onKeyDown={(event) => { if (event.key === 'Enter') void create() }} />
            {createFieldError && <p className={`vlb-fielderr ${stylex.props(sx.fielderr).className}`} role="alert">{createFieldError}</p>}
          </label>
          {createError && <p className={`vlb-fielderr ${stylex.props(sx.fielderr).className}`} role="alert">{createError}</p>}
        </Dialog>
      )}
    </div>
  )
}

const sx = stylex.create({
  shell: { display: "flex", flexGrow: libraryVars.fillRatio, flexShrink: libraryVars.fillRatio, flexBasis: "auto", minHeight: 0, minWidth: 0, color: color["--color-fg"] },
  body: { display: "flex", flexGrow: libraryVars.fillRatio, flexShrink: libraryVars.fillRatio, flexBasis: "auto", flexDirection: "column", minWidth: 0, minHeight: 0 },
  head: { display: "flex", alignItems: { default: "flex-start", "@media (max-width: 767px)": "stretch" }, justifyContent: "space-between", rowGap: { default: "var(--vv-s-16)", "@media (max-width: 767px)": "var(--vv-s-10)" }, columnGap: { default: "var(--vv-s-16)", "@media (max-width: 767px)": "var(--vv-s-10)" }, paddingTop: { default: "var(--vv-s-22)", "@media (max-width: 767px)": "var(--vv-s-16)" }, paddingRight: { default: "var(--vv-s-28)", "@media (max-width: 767px)": "var(--vv-s-16)" }, paddingBottom: { default: "var(--vv-s-12)", "@media (max-width: 767px)": "var(--vv-s-10)" }, paddingLeft: { default: "var(--vv-s-28)", "@media (max-width: 767px)": "var(--vv-s-16)" }, flexDirection: { default: null, "@media (max-width: 767px)": "column" } },
  headMain: { minWidth: 0 },
  vlbHeadMainH1: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: { default: "var(--vv-s-22)", "@media (max-width: 767px)": "var(--vv-s-18)" }, fontWeight: libraryVars.headingWeight, letterSpacing: libraryVars.libraryScreenVlbHeadMainH1LetterSpacing },
  hint: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: libraryVars.controlFontSize, color: color["--color-fg-3"], lineHeight: libraryVars.bodyLineHeight },
  headActions: { display: "flex", alignItems: "center", rowGap: "var(--vv-s-8)", columnGap: "var(--vv-s-8)", flexGrow: 0, flexShrink: 0, flexBasis: "auto", justifyContent: { default: null, "@media (max-width: 767px)": "flex-start" }, flexWrap: { default: null, "@media (max-width: 767px)": "wrap" } },
  vlbHeadActionsActionButton: { paddingInline: space['--vv-s-12'], borderColor: line['--line-4'], minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" } },
  tabsFrame: { paddingInline: "var(--vv-s-28)" },
  tabCount: { color: color["--color-fg-4"], fontSize: { default: "var(--vv-s-12)", "@media (max-width: 767px)": libraryVars.captionFontSize } },
  panel: { display: "flex", flexGrow: libraryVars.fillRatio, flexShrink: libraryVars.fillRatio, flexBasis: "auto", minHeight: 0, minWidth: 0 },
  hint2: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: "var(--vv-s-12)", color: color["--color-fg-4"], lineHeight: libraryVars.bodyLineHeight },
  field: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-6)", columnGap: "var(--vv-s-6)" },
  fielderr: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: "var(--vv-s-12)", color: color["--color-err"] },
})
