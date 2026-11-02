import { color, line } from '@noverse/ui/tokens.stylex'
import { libraryVars } from './libraryTokens.stylex'
import { space } from '../../styles/viderse.stylex'
// Thư viện (artboard Library): cột kênh + 4 mục dùng chung, đầu trang một hành
// động chính theo tab, tab Template / Prompt ảnh / Video nền / Lớp phủ. Video
// nền và phần nền của template do mảng Video nền dựng (BackdropsTab.tsx,
// template/BackdropSection.tsx) — ở đây chỉ gắn khung.
import { useRef, useState } from 'react'
import { Banner, Button, Dialog, FieldLabel, Icon, Tabs, TextField } from '@noverse/ui'
import { toast } from '@noverse/ui'
import { invalidate } from '@noverse/ui/data'
import { t } from '../../i18n/library'
import { switcherCopy } from '../../i18n/switcher'
import { navigate, paths, type LibraryTab, type PromptView, type Route } from '../../router'
import { useResource } from '../../data/useResource'
import { useStatus } from '../../data/status'
import { keys } from '../../data/keys'
import {
  createTemplate, fieldErrors, importPrompt, listOverlays, listOwnerChannels, listPrompts, listTemplates,
} from '../../api/library'
import { getChannelSummaries } from '../../api/workbenchChannels'
import { getQueue } from '../../api/clone'
import { listBackdropChannels, listBackdropVideos } from '../../api/backdrop'
import { ActionButton, ChannelAvatar } from '../../components'
import { channelTint } from '../../lib/channelTint'
import { useMediaQuery } from '../../lib/useMediaQuery'
import { BackdropsTab } from './backdrops/BackdropsTab'
import { OverlaysTab } from './OverlaysTab'
import { PromptsTab } from './PromptsTab'
import { TemplateImportDialog } from './TemplateImportDialog'
import { TemplatesTab } from './TemplatesTab'
import { MLibraryScreen } from './mobile/MLibraryScreen'
import * as stylex from '@stylexjs/stylex'

const LIB_ICON: Record<LibraryTab, 'layer' | 'magicpen' | 'video-play' | 'magic'> = {
  templates: 'layer', prompts: 'magicpen', backdrops: 'video-play', overlays: 'magic',
}

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
  const status = useStatus()
  const queue = useResource(keys.queue(), ({ signal }) => getQueue(signal))
  const backdropChannels = useResource(keys.backdropChannels(), ({ signal }) => listBackdropChannels(signal))
  const backdropVideos = useResource(keys.backdropVideos(), ({ signal }) => listBackdropVideos(signal))
  // Nhịp "Hôm nay x/y · n chờ bạn" của cột kênh lấy từ /api/channels/summary —
  // cùng resource khung app đã đọc, nên không thêm yêu cầu mới khi đã có.
  const summaries = useResource(['channels', 'summary'], ({ signal }) => getChannelSummaries(signal))
  const summaryById = new Map((summaries.data ?? []).map((item) => [item.channelId, item]))
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
      navigate(paths.library('prompts', 'mine'))
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
      <aside className={`vlb-side ${stylex.props(sx.side).className}`} aria-label={copy.sideAria}>
        <div className={`vlb-side-group ${stylex.props(sx.sideGroup).className}`}>
          <span className={`vlb-side-label ${stylex.props(sx.sideLabel).className}`}>
            {copy.sideChannels}
            {ownerChannels.length > 0 && <span className="num">{ownerChannels.length}</span>}
          </span>
          <ul className={`vlb-side-list ${stylex.props(sx.sideList).className}`}>
            {ownerChannels.map((channel) => {
              const summary = summaryById.get(channel.id)
              const waiting = summary?.waitingYou ?? 0
              return (
                <li key={channel.id}>
                  <a className={`vlb-side-item vlb-side-item--channel ${stylex.props(sx.sideItemChannel).className}`} href={`#${paths.channel(channel.id)}`}>
                    <ChannelAvatar
                      name={channel.name}
                      src={channel.avatarUrl ?? undefined}
                      color={{ bg: channelTint(channel.id), ring: channelTint(channel.id) }}
                      size={30}
                    />
                    <span className={`vlb-side-text ${stylex.props(sx.sideText).className}`}>
                      <span className={`vlb-side-name ${stylex.props(sx.sideName).className}`}>{channel.name}</span>
                      {summary && (
                        <span className={`vlb-side-sub num ${stylex.props(sx.sideSub).className}`}>
                          {copy.channelToday(summary.today.done, summary.today.cap)}
                          {waiting > 0 && ` · ${switcherCopy.waiting(waiting)}`}
                        </span>
                      )}
                    </span>
                    {waiting > 0 && <span className={`vlb-side-waiting ${stylex.props(sx.waiting).className}`} role="img" aria-label={copy.waitingAria} />}
                  </a>
                </li>
              )
            })}
          </ul>
          <a className={`vlb-side-add ${stylex.props(sx.sideAdd).className}`} href={`#${paths.channelNew()}`}>
            <Icon name="add-square" size={16} />{copy.createChannel}
          </a>
        </div>
        <div className={`vlb-side-group ${stylex.props(sx.sideGroup).className}`}>
          <span className={`vlb-side-label ${stylex.props(sx.sideLabel).className}`}>{copy.sideLibrary}</span>
          <ul className={`vlb-side-list ${stylex.props(sx.sideList).className}`}>
            {(Object.keys(copy.tabs) as LibraryTab[]).map((id) => (
              <li key={id}>
                <a className={`vlb-side-item ${stylex.props(sx.sideItem, tab === id && sx.sideItemAriaCurrentPage).className}`} href={`#${paths.library(id)}`} aria-current={tab === id ? 'page' : undefined}>
                  <span className={`vlb-side-ic ${stylex.props(sx.sideIcon, tab === id && sx.sideIconCurrent).className}`}><Icon name={LIB_ICON[id]} size={16} /></span>
                  <span className={`vlb-side-name ${stylex.props(sx.sideName).className}`}>{copy.tabs[id]}</span>
                  {counts[id] !== undefined && <span className={`vlb-count ${stylex.props(sx.count).className}`}>{counts[id]}</span>}
                </a>
              </li>
            ))}
          </ul>
        </div>
        <div className={`vlb-machine ${stylex.props(sx.machine, (status.data?.running ?? 0) > 0 && sx.machineDataStateBusy).className}`} data-state={(status.data?.running ?? 0) === 0 ? 'ready' : 'busy'}>
          <Icon name={(status.data?.running ?? 0) === 0 ? 'tick-b' : 'activity'} size={15} />
          {(status.data?.running ?? 0) === 0
            ? queue.data
              ? copy.machineCapacity(Math.max(0, queue.data.workers.total - queue.data.workers.used), queue.data.workers.total)
              : copy.machineReady
            : copy.machineBusy(status.data!.running, status.data!.queued)}
        </div>
      </aside>

      <div className={`vlb-body vv-wide vv-wide--grid ${stylex.props(sx.body).className}`}>
        <header className={`vlb-head ${stylex.props(sx.head).className}`}>
          <div className={`vlb-head-main ${stylex.props(sx.headMain).className}`}>
            <h1 {...stylex.props(sx.vlbHeadMainH1)}>{copy.title}</h1>
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
              all={allPrompts}
              allLoading={prompts.initialLoading}
              onView={(view: PromptView | undefined) => navigate(paths.library('prompts', view))}
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
  side: { display: { default: "flex", "@media (max-width: 1099px)": "none" }, flexDirection: "column", flexGrow: 0, flexShrink: 0, flexBasis: libraryVars.sidebarWidth, maxWidth: libraryVars.sidebarWidth, rowGap: "var(--vv-s-16)", columnGap: "var(--vv-s-16)", paddingTop: "var(--vv-s-12)", paddingRight: "var(--vv-s-10)", paddingBottom: "var(--vv-s-12)", paddingLeft: "var(--vv-s-10)", boxShadow: libraryVars.libraryScreenSideBoxShadow, minHeight: 0, overflowY: "auto" },
  sideGroup: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-2)", columnGap: "var(--vv-s-2)" },
  sideLabel: { display: "flex", alignItems: "baseline", justifyContent: "space-between", fontSize: "var(--vv-s-11)", fontWeight: libraryVars.headingWeight, letterSpacing: libraryVars.libraryScreenSideLabelLetterSpacing, textTransform: "uppercase", color: color["--color-fg-4"], paddingTop: "var(--vv-s-12)", paddingRight: "var(--vv-s-10)", paddingBottom: "var(--vv-s-4)", paddingLeft: "var(--vv-s-10)" },
  sideList: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-1)", columnGap: "var(--vv-s-1)", marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, paddingTop: 0, paddingRight: 0, paddingBottom: 0, paddingLeft: 0, listStyle: "none" },
  sideItemChannel: { minHeight: { default: "var(--vv-s-46)", "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" }, display: "flex", alignItems: "center", rowGap: "var(--vv-s-10)", columnGap: "var(--vv-s-10)", paddingTop: "var(--vv-s-8)", paddingRight: "var(--vv-s-10)", paddingBottom: "var(--vv-s-8)", paddingLeft: "var(--vv-s-10)", borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: "transparent", borderRightWidth: "var(--vv-s-1)", borderRightStyle: "solid", borderRightColor: "transparent", borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: "transparent", borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "solid", borderLeftColor: "transparent", borderTopLeftRadius: "var(--vv-r-10)", borderTopRightRadius: "var(--vv-r-10)", borderBottomRightRadius: "var(--vv-r-10)", borderBottomLeftRadius: "var(--vv-r-10)", color: { default: color["--color-fg"], ":hover": color["--color-fg"] }, textDecoration: "none", backgroundColor: { default: null, ":hover": "var(--vv-hover-row)" }, backgroundImage: { default: null, ":hover": "none" } },
  sideText: { flexGrow: libraryVars.fillRatio, flexShrink: libraryVars.fillRatio, flexBasis: libraryVars.flexFillBasis, minWidth: 0, display: "flex", flexDirection: "column", rowGap: "var(--vv-s-1)", columnGap: "var(--vv-s-1)" },
  sideName: { minWidth: 0, fontSize: "var(--vv-s-13)", fontWeight: libraryVars.labelWeight, overflowX: "hidden", overflowY: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" },
  sideSub: { fontSize: "var(--vv-s-11)", color: color["--color-fg-4"], overflowX: "hidden", overflowY: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" },
  waiting: { width: "var(--vv-s-8)", height: "var(--vv-s-8)", borderTopLeftRadius: "50%", borderTopRightRadius: "50%", borderBottomRightRadius: "50%", borderBottomLeftRadius: "50%", backgroundColor: color["--color-warn"], backgroundImage: "none", flexShrink: 0 },
  sideAdd: { display: "flex", alignItems: "center", rowGap: "var(--vv-s-10)", columnGap: "var(--vv-s-10)", paddingTop: "var(--vv-s-8)", paddingRight: "var(--vv-s-10)", paddingBottom: "var(--vv-s-8)", paddingLeft: "var(--vv-s-10)", borderTopLeftRadius: "var(--vv-r-10)", borderTopRightRadius: "var(--vv-r-10)", borderBottomRightRadius: "var(--vv-r-10)", borderBottomLeftRadius: "var(--vv-r-10)", color: { default: color["--color-fg-3"], ":hover": color["--color-fg"] }, fontSize: "var(--vv-s-13)", textDecoration: "none", minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" }, backgroundColor: { default: null, ":hover": "var(--vv-hover-row)" }, backgroundImage: { default: null, ":hover": "none" } },
  sideItem: { display: "flex", alignItems: "center", rowGap: "var(--vv-s-10)", columnGap: "var(--vv-s-10)", minHeight: { default: "var(--vv-s-32)", "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" }, paddingTop: "var(--vv-s-7)", paddingRight: "var(--vv-s-10)", paddingBottom: "var(--vv-s-7)", paddingLeft: "var(--vv-s-10)", borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: "transparent", borderRightWidth: "var(--vv-s-1)", borderRightStyle: "solid", borderRightColor: "transparent", borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: "transparent", borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "solid", borderLeftColor: "transparent", borderTopLeftRadius: "var(--vv-r-8)", borderTopRightRadius: "var(--vv-r-8)", borderBottomRightRadius: "var(--vv-r-8)", borderBottomLeftRadius: "var(--vv-r-8)", color: { default: color["--color-fg-2"], ":hover": color["--color-fg"] }, textDecoration: "none", fontSize: "var(--vv-s-13)", backgroundColor: { default: null, ":hover": "var(--vv-hover-row)" }, backgroundImage: { default: null, ":hover": "none" } },
  sideIcon: { display: "inline-flex", alignItems: "center", justifyContent: "center", color: color["--color-fg-4"], flexShrink: 0 },
  sideIconCurrent: { color: color["--color-accent"] },
  sideItemAriaCurrentPage: { backgroundColor: color["--color-surface-4"], backgroundImage: "none", borderTopColor: line["--line-2"], borderRightColor: line["--line-2"], borderBottomColor: line["--line-2"], borderLeftColor: line["--line-2"], color: color["--color-fg"], fontWeight: libraryVars.labelWeight },
  count: { fontSize: "var(--vv-s-12)", color: color["--color-fg-3"], fontVariantNumeric: "tabular-nums", flexShrink: 0 },
  machine: { marginTop: "auto", display: "flex", alignItems: "center", rowGap: "var(--vv-s-8)", columnGap: "var(--vv-s-8)", height: "var(--vv-s-34)", paddingTop: 0, paddingRight: "var(--vv-s-10)", paddingBottom: 0, paddingLeft: "var(--vv-s-10)", borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: "var(--vv-ok-20)", borderRightWidth: "var(--vv-s-1)", borderRightStyle: "solid", borderRightColor: "var(--vv-ok-20)", borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: "var(--vv-ok-20)", borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "solid", borderLeftColor: "var(--vv-ok-20)", borderTopLeftRadius: "var(--vv-r-10)", borderTopRightRadius: "var(--vv-r-10)", borderBottomRightRadius: "var(--vv-r-10)", borderBottomLeftRadius: "var(--vv-r-10)", color: color["--color-ok"], fontSize: libraryVars.bodySmallFontSize, whiteSpace: "nowrap", overflowX: "hidden", overflowY: "hidden" },
  machineDataStateBusy: { color: color["--color-warn"], borderTopColor: "var(--vv-warn-bd)", borderRightColor: "var(--vv-warn-bd)", borderBottomColor: "var(--vv-warn-bd)", borderLeftColor: "var(--vv-warn-bd)" },
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
  channelAvatar: { width: "var(--vv-s-30)", height: "var(--vv-s-30)" },
})
