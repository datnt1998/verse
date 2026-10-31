import { color, line } from '@noverse/ui/tokens.stylex'
import { libraryVars } from './libraryTokens.stylex'
// Thư viện (artboard Library): cột kênh + 4 mục dùng chung, đầu trang một hành
// động chính theo tab, tab Template / Prompt ảnh / Video nền / Lớp phủ. Video
// nền và phần nền của template do mảng Video nền dựng (BackdropsTab.tsx,
// template/BackdropSection.tsx) — ở đây chỉ gắn khung.
import { useRef, useState } from 'react'
import { Banner, Button, Dialog, FieldLabel, Icon, TextField } from '@noverse/ui'
import { toast } from '@noverse/ui'
import { invalidate } from '@noverse/ui/data'
import { vi as t } from '../../i18n/vi'
import { navigate, paths, type LibraryTab, type PromptView, type Route } from '../../router'
import { useResource } from '../../data/useResource'
import { useStatus } from '../../data/status'
import { keys } from '../../data/keys'
import {
  createTemplate, fieldErrors, importPrompt, listOverlays, listOwnerChannels, listPrompts, listTemplates,
} from '../../api/library'
import { ActionButton, ChannelAvatar } from '../../components'
import { channelTint } from '../../lib/channelTint'
import { BackdropsTab } from './backdrops/BackdropsTab'
import { OverlaysTab } from './OverlaysTab'
import { PromptsTab } from './PromptsTab'
import { TemplatesTab } from './TemplatesTab'
import * as stylex from '@stylexjs/stylex'

const LIB_ICON: Record<LibraryTab, 'box' | 'magicpen' | 'video-play' | 'layer'> = {
  templates: 'box', prompts: 'magicpen', backdrops: 'video-play', overlays: 'layer',
}

export function LibraryScreen({ route }: { route: Extract<Route, { name: 'library' }> }) {
  const copy = t.library
  const tab: LibraryTab = route.tab ?? 'templates'
  const templates = useResource(keys.templates(), ({ signal }) => listTemplates(signal))
  const prompts = useResource(keys.prompts('', ''), ({ signal }) => listPrompts({}, signal))
  const overlays = useResource(keys.overlays(), ({ signal }) => listOverlays(signal))
  const channels = useResource(keys.ownerChannels(), ({ signal }) => listOwnerChannels(signal))
  const status = useStatus()
  const ownerChannels = channels.data?.channels ?? []
  const allPrompts = prompts.data?.items ?? []

  const [createOpen, setCreateOpen] = useState(false)
  const [newName, setNewName] = useState('')
  const [createError, setCreateError] = useState<string | null>(null)
  const [createFieldError, setCreateFieldError] = useState<string | null>(null)
  const [importing, setImporting] = useState(false)
  const [importError, setImportError] = useState<string | null>(null)
  const importInput = useRef<HTMLInputElement>(null)

  const counts: Record<LibraryTab, number | undefined> = {
    templates: templates.data?.items.length,
    prompts: prompts.data?.items.length,
    overlays: overlays.data?.items.length,
    backdrops: undefined,
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
            {ownerChannels.map((channel) => (
              <li key={channel.id}>
                <a className={`vlb-side-item vlb-side-item--channel ${stylex.props(sx.sideItemSideItemChannel).className}`} href={`#${paths.channel(channel.id)}`}>
                  <ChannelAvatar
                    name={channel.name}
                    src={channel.avatarUrl ?? undefined}
                    color={{ bg: channelTint(channel.id), ring: channelTint(channel.id) }}
                    size={30}
                  />
                  <span className={`vlb-side-name ${stylex.props(sx.sideName).className}`}>{channel.name}</span>
                </a>
              </li>
            ))}
          </ul>
          <a className={`vlb-side-add ${stylex.props(sx.sideAdd).className}`} href={`#${paths.channelNew()}`}>
            <Icon name="add" size={14} />{copy.createChannel}
          </a>
        </div>
        <div className={`vlb-side-group ${stylex.props(sx.sideGroup).className}`}>
          <span className={`vlb-side-label ${stylex.props(sx.sideLabel).className}`}>{copy.sideLibrary}</span>
          <ul className={`vlb-side-list ${stylex.props(sx.sideList).className}`}>
            {(Object.keys(copy.tabs) as LibraryTab[]).map((id) => (
              <li key={id}>
                <a className={`vlb-side-item ${stylex.props(sx.sideItem, ((tab === id ? 'page' : undefined) === "page") && sx.sideItemAriaCurrentPage).className}`} href={`#${paths.library(id)}`} aria-current={tab === id ? 'page' : undefined}>
                  <Icon name={LIB_ICON[id]} size={15} />
                  <span className={`vlb-side-name ${stylex.props(sx.sideName).className}`}>{copy.tabs[id]}</span>
                  {counts[id] !== undefined && <span className={`vlb-count ${stylex.props(sx.count).className}`}>{counts[id]}</span>}
                </a>
              </li>
            ))}
          </ul>
        </div>
        <div className={`vlb-machine ${stylex.props(sx.machine, (((status.data?.running ?? 0) === 0 ? 'ready' : 'busy') === "busy") && sx.machineDataStateBusy).className}`} data-state={(status.data?.running ?? 0) === 0 ? 'ready' : 'busy'}>
          <Icon name="tick-circle" size={14} />
          {(status.data?.running ?? 0) === 0
            ? copy.machineReady
            : copy.machineBusy(status.data!.running, status.data!.queued)}
        </div>
      </aside>

      <div className={`vlb-body ${stylex.props(sx.body).className}`}>
        <header className={`vlb-head ${stylex.props(sx.head).className}`}>
          <div className={`vlb-head-main ${stylex.props(sx.headMain).className}`}>
            <h1 {...stylex.props(sx.vlbHeadMainH1)}>{copy.title}</h1>
            <p className={`vlb-hint ${stylex.props(sx.hint).className}`}>{ownerChannels.length ? copy.subtitle(ownerChannels.length) : copy.subtitleEmpty}</p>
          </div>
          <div className={`vlb-head-actions ${stylex.props(sx.headActions).className}`}>
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

        <div className={`vlb-tabs ${stylex.props(sx.tabs).className}`} role="tablist" aria-label={copy.tabAria}>
          {(Object.keys(copy.tabs) as LibraryTab[]).map((id) => (
            <a
              key={id}
              id={`vlb-tabs-${id}`}
              role="tab"
              className={`vlb-tab ${stylex.props(sx.tab, (id !== 'templates') && sx.tabSpaced, ((tab === id) === true) && sx.tabAriaSelectedTrue).className}`}
              href={`#${paths.library(id)}`}
              aria-selected={tab === id}
              aria-controls="vlb-panel"
            >
              {copy.tabs[id]}
              {counts[id] !== undefined && <span className={`vlb-tab-count ${stylex.props(sx.tabCount).className}`}>{counts[id]}</span>}
            </a>
          ))}
        </div>

        <div id="vlb-panel" role="tabpanel" aria-labelledby={`vlb-tabs-${tab}`} className={`vlb-panel ${stylex.props(sx.panel).className}`}>
          {tab === 'templates' && <TemplatesTab channels={ownerChannels} />}
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
  side: { display: { default: "flex", "@media (max-width: 1099px)": "none" }, flexDirection: "column", flexGrow: 0, flexShrink: 0, flexBasis: libraryVars.sidebarWidth, maxWidth: libraryVars.sidebarWidth, rowGap: "var(--vv-s-18)", columnGap: "var(--vv-s-18)", paddingTop: "var(--vv-s-14)", paddingRight: "var(--vv-s-12)", paddingBottom: "var(--vv-s-14)", paddingLeft: "var(--vv-s-14)", boxShadow: libraryVars.libraryScreenSideBoxShadow, minHeight: 0, overflowY: "auto" },
  sideGroup: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-4)", columnGap: "var(--vv-s-4)" },
  sideLabel: { display: "flex", alignItems: "baseline", justifyContent: "space-between", fontSize: libraryVars.captionFontSize, letterSpacing: libraryVars.libraryScreenSideLabelLetterSpacing, textTransform: "uppercase", color: color["--color-fg-4"], paddingTop: 0, paddingRight: "var(--vv-s-8)", paddingBottom: 0, paddingLeft: "var(--vv-s-8)" },
  sideList: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-1)", columnGap: "var(--vv-s-1)", marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, paddingTop: 0, paddingRight: 0, paddingBottom: 0, paddingLeft: 0, listStyle: "none" },
  sideItemSideItemChannel: { minHeight: { default: "var(--vv-s-32)", "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" }, paddingTop: "var(--vv-s-6)", paddingBottom: "var(--vv-s-6)", display: "flex", alignItems: "center", rowGap: "var(--vv-s-9)", columnGap: "var(--vv-s-9)", paddingRight: "var(--vv-s-8)", paddingLeft: "var(--vv-s-8)", borderTopLeftRadius: "var(--vv-r-8)", borderTopRightRadius: "var(--vv-r-8)", borderBottomRightRadius: "var(--vv-r-8)", borderBottomLeftRadius: "var(--vv-r-8)", color: { default: color["--color-fg-2"], ":hover": color["--color-fg"] }, textDecoration: "none", fontSize: "var(--vv-s-13)", backgroundColor: { default: null, ":hover": "var(--vv-hover-row)" }, backgroundImage: { default: null, ":hover": "none" } },
  sideName: { flexGrow: libraryVars.fillRatio, flexShrink: libraryVars.fillRatio, flexBasis: "auto", minWidth: 0, overflowX: "hidden", overflowY: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" },
  sideAdd: { display: "flex", alignItems: "center", rowGap: "var(--vv-s-6)", columnGap: "var(--vv-s-6)", paddingTop: "var(--vv-s-6)", paddingRight: "var(--vv-s-8)", paddingBottom: "var(--vv-s-6)", paddingLeft: "var(--vv-s-8)", borderTopLeftRadius: "var(--vv-r-8)", borderTopRightRadius: "var(--vv-r-8)", borderBottomRightRadius: "var(--vv-r-8)", borderBottomLeftRadius: "var(--vv-r-8)", color: { default: color["--color-fg-4"], ":hover": color["--color-fg"] }, fontSize: libraryVars.bodySmallFontSize, textDecoration: "none", minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" }, backgroundColor: { default: null, ":hover": "var(--vv-hover-row)" }, backgroundImage: { default: null, ":hover": "none" } },
  sideItem: { display: "flex", alignItems: "center", rowGap: "var(--vv-s-9)", columnGap: "var(--vv-s-9)", minHeight: { default: "var(--vv-s-32)", "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" }, paddingTop: "var(--vv-s-6)", paddingRight: "var(--vv-s-8)", paddingBottom: "var(--vv-s-6)", paddingLeft: "var(--vv-s-8)", borderTopLeftRadius: "var(--vv-r-8)", borderTopRightRadius: "var(--vv-r-8)", borderBottomRightRadius: "var(--vv-r-8)", borderBottomLeftRadius: "var(--vv-r-8)", color: { default: color["--color-fg-2"], ":hover": color["--color-fg"] }, textDecoration: "none", fontSize: "var(--vv-s-13)", backgroundColor: { default: null, ":hover": "var(--vv-hover-row)" }, backgroundImage: { default: null, ":hover": "none" } },
  sideItemAriaCurrentPage: { backgroundColor: color["--color-surface-5"], backgroundImage: "none", color: color["--color-fg"], fontWeight: libraryVars.labelWeight },
  count: { minWidth: "var(--vv-s-20)", paddingTop: 0, paddingRight: "var(--vv-s-5)", paddingBottom: 0, paddingLeft: "var(--vv-s-5)", borderTopLeftRadius: "var(--vv-r-pill)", borderTopRightRadius: "var(--vv-r-pill)", borderBottomRightRadius: "var(--vv-r-pill)", borderBottomLeftRadius: "var(--vv-r-pill)", backgroundColor: color["--color-surface-6"], backgroundImage: "none", color: color["--color-fg-3"], fontSize: "var(--vv-s-11)", fontVariantNumeric: "tabular-nums", textAlign: "center", lineHeight: "var(--vv-s-18)" },
  machine: { marginTop: "auto", display: "flex", alignItems: "center", rowGap: "var(--vv-s-8)", columnGap: "var(--vv-s-8)", height: "var(--vv-s-34)", paddingTop: 0, paddingRight: "var(--vv-s-10)", paddingBottom: 0, paddingLeft: "var(--vv-s-10)", borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: "var(--vv-ok-20)", borderRightWidth: "var(--vv-s-1)", borderRightStyle: "solid", borderRightColor: "var(--vv-ok-20)", borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: "var(--vv-ok-20)", borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "solid", borderLeftColor: "var(--vv-ok-20)", borderTopLeftRadius: "var(--vv-r-10)", borderTopRightRadius: "var(--vv-r-10)", borderBottomRightRadius: "var(--vv-r-10)", borderBottomLeftRadius: "var(--vv-r-10)", color: color["--color-ok"], fontSize: libraryVars.bodySmallFontSize, whiteSpace: "nowrap", overflowX: "hidden", overflowY: "hidden" },
  machineDataStateBusy: { color: color["--color-warn"], borderTopColor: "var(--vv-warn-bd)", borderRightColor: "var(--vv-warn-bd)", borderBottomColor: "var(--vv-warn-bd)", borderLeftColor: "var(--vv-warn-bd)" },
  body: { display: "flex", flexGrow: libraryVars.fillRatio, flexShrink: libraryVars.fillRatio, flexBasis: "auto", flexDirection: "column", minWidth: 0, minHeight: 0 },
  head: { display: "flex", alignItems: { default: "flex-start", "@media (max-width: 767px)": "stretch" }, justifyContent: "space-between", rowGap: { default: "var(--vv-s-16)", "@media (max-width: 767px)": "var(--vv-s-10)" }, columnGap: { default: "var(--vv-s-16)", "@media (max-width: 767px)": "var(--vv-s-10)" }, paddingTop: { default: "var(--vv-s-22)", "@media (max-width: 767px)": "var(--vv-s-16)" }, paddingRight: { default: "var(--vv-s-28)", "@media (max-width: 767px)": "var(--vv-s-16)" }, paddingBottom: { default: "var(--vv-s-12)", "@media (max-width: 767px)": "var(--vv-s-10)" }, paddingLeft: { default: "var(--vv-s-28)", "@media (max-width: 767px)": "var(--vv-s-16)" }, flexDirection: { default: null, "@media (max-width: 767px)": "column" } },
  headMain: { minWidth: 0 },
  vlbHeadMainH1: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: { default: "var(--vv-s-22)", "@media (max-width: 767px)": "var(--vv-s-18)" }, fontWeight: libraryVars.headingWeight, letterSpacing: libraryVars.libraryScreenVlbHeadMainH1LetterSpacing },
  hint: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: libraryVars.controlFontSize, color: color["--color-fg-3"], lineHeight: libraryVars.bodyLineHeight },
  headActions: { display: "flex", alignItems: "center", rowGap: "var(--vv-s-8)", columnGap: "var(--vv-s-8)", flexGrow: 0, flexShrink: 0, flexBasis: "auto", justifyContent: { default: null, "@media (max-width: 767px)": "flex-start" }, flexWrap: { default: null, "@media (max-width: 767px)": "wrap" } },
  vlbHeadActionsActionButton: { minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" } },
  tabs: { display: "flex", flexGrow: 0, flexShrink: 0, flexBasis: "auto", overflowX: { default: "auto", "@media (max-width: 767px)": "auto" }, scrollbarWidth: "none", rowGap: { default: "var(--vv-s-4)", "@media (max-width: 767px)": 0 }, columnGap: { default: "var(--vv-s-4)", "@media (max-width: 767px)": 0 }, paddingTop: { default: 0, "@media (max-width: 767px)": 0 }, paddingRight: { default: "var(--vv-s-28)", "@media (max-width: 767px)": "var(--vv-s-16)" }, paddingBottom: { default: 0, "@media (max-width: 767px)": "var(--vv-s-8)" }, paddingLeft: { default: "var(--vv-s-28)", "@media (max-width: 767px)": "var(--vv-s-16)" }, borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: line["--line-2"] },
  tab: { display: "inline-flex", alignItems: "center", rowGap: "var(--vv-s-6)", columnGap: "var(--vv-s-6)", paddingTop: { default: "var(--vv-s-7)", "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-11)" }, paddingRight: "var(--vv-s-2)", paddingBottom: { default: "var(--vv-s-9)", "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-11)" }, paddingLeft: "var(--vv-s-2)", minHeight: { default: "var(--vv-s-38)", "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" }, fontSize: { default: libraryVars.controlFontSize, "@media (max-width: 767px)": libraryVars.bodySmallFontSize }, borderBottomWidth: "var(--vv-s-2)", borderBottomStyle: "solid", borderBottomColor: "transparent", color: { default: color["--color-fg-3"], ":hover": color["--color-fg"] }, textDecoration: "none", whiteSpace: "nowrap" },
  tabSpaced: { marginLeft: { default: "var(--vv-s-18)", "@media (max-width: 767px)": "var(--vv-s-12)" } },
  tabAriaSelectedTrue: { color: color["--color-fg"], borderBottomColor: color["--color-accent"], fontWeight: libraryVars.labelWeight },
  tabCount: { color: color["--color-fg-4"], fontSize: { default: "var(--vv-s-12)", "@media (max-width: 767px)": libraryVars.captionFontSize } },
  panel: { display: "flex", flexGrow: libraryVars.fillRatio, flexShrink: libraryVars.fillRatio, flexBasis: "auto", minHeight: 0, minWidth: 0 },
  hint2: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: "var(--vv-s-12)", color: color["--color-fg-4"], lineHeight: libraryVars.bodyLineHeight },
  field: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-6)", columnGap: "var(--vv-s-6)" },
  fielderr: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: "var(--vv-s-12)", color: color["--color-err"] },
  channelAvatar: { width: "var(--vv-s-30)", height: "var(--vv-s-30)" },
})
