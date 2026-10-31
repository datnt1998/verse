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
import './library.css'

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
    <div className="vlb-shell">
      <aside className="vlb-side" aria-label={copy.sideAria}>
        <div className="vlb-side-group">
          <span className="vlb-side-label">
            {copy.sideChannels}
            {ownerChannels.length > 0 && <span className="num">{ownerChannels.length}</span>}
          </span>
          <ul className="vlb-side-list">
            {ownerChannels.map((channel) => (
              <li key={channel.id}>
                <a className="vlb-side-item vlb-side-item--channel" href={`#${paths.channel(channel.id)}`}>
                  <ChannelAvatar
                    name={channel.name}
                    src={channel.avatarUrl ?? undefined}
                    color={{ bg: channelTint(channel.id), ring: channelTint(channel.id) }}
                    size={30}
                  />
                  <span className="vlb-side-name">{channel.name}</span>
                </a>
              </li>
            ))}
          </ul>
          <a className="vlb-side-add" href={`#${paths.channelNew()}`}>
            <Icon name="add" size={14} />{copy.createChannel}
          </a>
        </div>
        <div className="vlb-side-group">
          <span className="vlb-side-label">{copy.sideLibrary}</span>
          <ul className="vlb-side-list">
            {(Object.keys(copy.tabs) as LibraryTab[]).map((id) => (
              <li key={id}>
                <a className="vlb-side-item" href={`#${paths.library(id)}`} aria-current={tab === id ? 'page' : undefined}>
                  <Icon name={LIB_ICON[id]} size={15} />
                  <span className="vlb-side-name">{copy.tabs[id]}</span>
                  {counts[id] !== undefined && <span className="vlb-count">{counts[id]}</span>}
                </a>
              </li>
            ))}
          </ul>
        </div>
        <div className="vlb-machine" data-state={(status.data?.running ?? 0) === 0 ? 'ready' : 'busy'}>
          <Icon name="tick-circle" size={14} />
          {(status.data?.running ?? 0) === 0
            ? copy.machineReady
            : copy.machineBusy(status.data!.running, status.data!.queued)}
        </div>
      </aside>

      <div className="vlb-body">
        <header className="vlb-head">
          <div className="vlb-head-main">
            <h1>{copy.title}</h1>
            <p className="vlb-hint">{ownerChannels.length ? copy.subtitle(ownerChannels.length) : copy.subtitleEmpty}</p>
          </div>
          <div className="vlb-head-actions">
            {tab === 'prompts' && (
              <>
                <input ref={importInput} type="file" accept=".json,application/json" hidden
                  onChange={(event) => void runImport(event.currentTarget.files?.[0])} />
                <ActionButton variant="outline" icon={<Icon name="download" size={16} />} pending={importing} onClick={pickImport}>
                  {copy.prompts.importPrompt}
                </ActionButton>
              </>
            )}
            {primary && <ActionButton variant="primary" icon={primary.icon} onClick={primary.onClick}>{primary.label}</ActionButton>}
          </div>
        </header>

        {importError && <Banner tone="error" title={importError} />}

        <div className="vlb-tabs" role="tablist" aria-label={copy.tabAria}>
          {(Object.keys(copy.tabs) as LibraryTab[]).map((id) => (
            <a
              key={id}
              id={`vlb-tabs-${id}`}
              role="tab"
              className="vlb-tab"
              href={`#${paths.library(id)}`}
              aria-selected={tab === id}
              aria-controls="vlb-panel"
            >
              {copy.tabs[id]}
              {counts[id] !== undefined && <span className="vlb-tab-count">{counts[id]}</span>}
            </a>
          ))}
        </div>

        <div id="vlb-panel" role="tabpanel" aria-labelledby={`vlb-tabs-${tab}`} className="vlb-panel">
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
          <p className="vlb-hint">{copy.templates.createBody}</p>
          <label className="vlb-field">
            <FieldLabel htmlFor="vlb-new-name">{copy.nameLabel}</FieldLabel>
            <TextField id="vlb-new-name" autoFocus value={newName} placeholder={copy.templates.createPlaceholder}
              aria-invalid={createFieldError ? true : undefined}
              onChange={setNewName}
              onKeyDown={(event) => { if (event.key === 'Enter') void create() }} />
            {createFieldError && <p className="vlb-fielderr" role="alert">{createFieldError}</p>}
          </label>
          {createError && <p className="vlb-fielderr" role="alert">{createError}</p>}
        </Dialog>
      )}
    </div>
  )
}
