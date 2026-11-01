// MLibrary — Thư viện dùng chung trên điện thoại (artboard MLibrary.dc.html,
// tweak tab=templates|prompts|backdrops|overlays, sheet=none|actions|new).
// Dùng đúng dữ liệu/API của Thư viện máy tính (listTemplates, listPrompts,
// listOverlays, listBackdrop*), không thêm endpoint nào.
import { useRef, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import {
  ActionSheet, Button, ConfirmSheet, Dialog, Icon, InitialsAvatar, MobilePage,
  MobilePageAction, Segmented, Sheet, TextField, toast, useTabMode,
  type MobileSheetAction,
} from '@noverse/ui'
import { invalidate } from '@noverse/ui/data'
import { t } from '../../../i18n/library'
import { navigate, paths, type LibraryTab, type Route } from '../../../router'
import { useResource } from '../../../data/useResource'
import { keys } from '../../../data/keys'
import {
  addBackdropChannel, addBackdropVideo, listBackdropChannels, listBackdropVideos, parseSource,
  type BackdropChannel, type BackdropVideo,
} from '../../../api/backdrop'
import {
  addOverlay, createPrompt, createTemplate, duplicateTemplate, fieldErrors, listOverlays, listOwnerChannels,
  listPrompts, listTemplates, patchTemplate, removeTemplate,
  type ImagePrompt, type Overlay, type PromptKind, type TemplateSummary,
} from '../../../api/library'
import { channelsHref } from '../../../lib/lastChannel'
import { AttachChannelDialog } from '../AttachChannelDialog'
import { TemplateImportDialog } from '../TemplateImportDialog'
import { PromptEditor } from '../PromptEditor'
import { styleOf, imagesText } from '../TemplatesTab'
import { editedLabel, initials } from '../fmt'
import { kindName, platformName, segmentLabel } from '../backdrops/fmt'
import { mlib } from './mlibrary.stylex'

type LibraryRoute = Extract<Route, { name: 'library' }>

type PromptFilter = 'all' | PromptKind

/** Mọi khoá prompt (lưới + hộp chọn ảnh cảnh/ảnh bìa của Sửa template). */
const refreshPrompts = () => Promise.all([
  invalidate(keys.prompts('', '')), invalidate(keys.prompts('scene', '')), invalidate(keys.prompts('cover', '')),
])

const TABS: readonly LibraryTab[] = ['templates', 'prompts', 'backdrops', 'overlays']

export function MLibraryScreen({ route }: { route: LibraryRoute }) {
  useTabMode('hide')
  const copy = t.library.mobile
  const tab: LibraryTab = route.tab && TABS.includes(route.tab) ? route.tab : 'templates'
  const [promptFilter, setPromptFilter] = useState<PromptFilter>('all')
  const [sheet, setSheet] = useState<'new' | null>(null)
  const [active, setActive] = useState<TemplateSummary | null>(null)
  /** Hộp đặt tên: `template` null = tạo template mới, có = đổi tên. */
  const [naming, setNaming] = useState<{ template: TemplateSummary | null } | null>(null)
  const [nameDraft, setNameDraft] = useState('')
  const [nameError, setNameError] = useState<string | null>(null)
  const [attaching, setAttaching] = useState(false)
  const [deleting, setDeleting] = useState<TemplateSummary | null>(null)
  const [editingPrompt, setEditingPrompt] = useState<ImagePrompt | null | undefined>(undefined)
  const [addingBackdrop, setAddingBackdrop] = useState(false)
  const [backdropInput, setBackdropInput] = useState('')
  const [backdropError, setBackdropError] = useState<string | null>(null)
  const [addingBackdropBusy, setAddingBackdropBusy] = useState(false)
  const [importing, setImporting] = useState(false)
  const overlayInput = useRef<HTMLInputElement>(null)

  const templates = useResource(keys.templates(), ({ signal }) => listTemplates(signal))
  const prompts = useResource(keys.prompts('', ''), ({ signal }) => listPrompts({}, signal))
  const overlays = useResource(keys.overlays(), ({ signal }) => listOverlays(signal))
  const channels = useResource(keys.ownerChannels(), ({ signal }) => listOwnerChannels(signal))
  const backdropChannels = useResource(keys.backdropChannels(), ({ signal }) => listBackdropChannels(signal))
  const backdropVideos = useResource(keys.backdropVideos(), ({ signal }) => listBackdropVideos(signal))

  const templateItems = templates.data?.items ?? []
  const promptItems = prompts.data?.items ?? []
  const overlayItems = overlays.data?.items ?? []
  const ownerChannels = channels.data?.channels ?? []
  const sourceChannels = backdropChannels.data ?? []
  const sourceVideos = backdropVideos.data ?? []
  const promptCounts = {
    all: promptItems.length,
    scene: promptItems.filter((prompt) => prompt.kind === 'scene').length,
    cover: promptItems.filter((prompt) => prompt.kind === 'cover').length,
  }
  const visiblePrompts = promptFilter === 'all' ? promptItems : promptItems.filter((prompt) => prompt.kind === promptFilter)
  const templateCount = templateItems.length

  const openName = (template: TemplateSummary | null) => {
    setNaming({ template })
    setNameDraft(template?.name ?? '')
    setNameError(null)
  }

  const saveName = async () => {
    if (!naming) return
    const name = nameDraft.trim()
    const target = naming.template
    if (target && !name) {
      setNameError(t.library.templates.renameEmpty)
      return
    }
    try {
      if (target) {
        await patchTemplate(target.id, { revision: target.revision, name })
        await Promise.all([invalidate(keys.templates()), invalidate(keys.template(target.id))])
        toast.ok(t.library.templates.renamed)
        setNaming(null)
      } else {
        const created = await createTemplate({ name })
        await invalidate(keys.templates())
        setNaming(null)
        navigate(paths.template(created.id))
      }
    } catch (err) {
      setNameError(fieldErrors(err).name ?? (err instanceof Error ? err.message : String(err)))
    }
  }

  const duplicatePrompt = async (prompt: ImagePrompt) => {
    try {
      const copyOf = await createPrompt({ name: t.library.prompts.copyName(prompt.name), kind: prompt.kind, prompt: prompt.prompt, fromId: prompt.id })
      await refreshPrompts()
      setEditingPrompt(copyOf)
    } catch (err) {
      toast.err(err instanceof Error ? err.message : String(err))
    }
  }

  const duplicate = async (template: TemplateSummary) => {
    try {
      const created = await duplicateTemplate(template.id)
      await invalidate(keys.templates())
      toast(t.library.templates.duplicated, 'ok')
      navigate(paths.template(created.id))
    } catch (err) {
      toast(err instanceof Error ? err.message : String(err), 'err')
    }
  }

  const remove = async (template: TemplateSummary) => {
    try {
      await removeTemplate(template.id)
      await invalidate(keys.templates())
      toast(t.library.templates.deleted, 'ok')
    } catch (err) {
      toast(err instanceof Error ? err.message : String(err), 'err')
    }
  }

  const addBackdrop = async () => {
    const input = backdropInput.trim()
    if (!input) return
    setAddingBackdropBusy(true)
    setBackdropError(null)
    try {
      const parsed = await parseSource(input)
      if (parsed.kind === 'channel') await addBackdropChannel(input)
      else if (parsed.kind === 'video') await addBackdropVideo(parsed.url)
      else {
        setBackdropError(parsed.kind === 'unsupported' ? t.backdrop.tab.unsupported(parsed.label) : t.backdrop.tab.invalidLink)
        return
      }
      await Promise.all([invalidate(keys.backdropChannels()), invalidate(keys.backdropVideos())])
      setAddingBackdrop(false)
      setBackdropInput('')
      toast(t.backdrop.tab.hintChannelOk, 'ok')
    } catch (err) {
      setBackdropError(err instanceof Error ? err.message : String(err))
    } finally {
      setAddingBackdropBusy(false)
    }
  }

  const uploadOverlay = async (file: File) => {
    try {
      await addOverlay(file)
      await invalidate(keys.overlays())
      toast(t.library.overlays.added, 'ok')
    } catch (err) {
      toast(err instanceof Error ? err.message : String(err), 'err')
    }
  }

  const activeChannels = active ? active.channels.map((channel) => channel.name) : []
  const actionItems: MobileSheetAction[] = active ? [
    {
      key: 'edit', icon: 'edit', label: copy.editTemplate,
      onSelect: () => navigate(paths.template(active.id)),
    },
    {
      key: 'rename', icon: 'edit', label: t.library.templates.rename,
      onSelect: () => openName(active),
    },
    {
      key: 'duplicate', icon: 'copy', label: t.library.duplicate,
      onSelect: () => duplicate(active),
    },
    ownerChannels.length
      ? {
          key: 'attach', icon: 'channel', label: t.library.templates.addToChannel,
          description: activeChannels.length ? copy.attachHint(activeChannels) : undefined,
          onSelect: () => setAttaching(true),
        }
      : {
          key: 'attach', icon: 'channel', label: t.library.templates.addToChannel,
          disabled: true, description: t.library.templates.attachEmpty, onSelect: () => undefined,
        },
    {
      key: 'export', icon: 'export', label: copy.exportFile,
      disabled: true, description: t.library.templates.exportCardLocked,
      onSelect: () => undefined,
    },
    {
      key: 'delete', icon: 'trash', label: copy.deleteTemplate,
      destructive: true,
      ...(activeChannels.length
        ? { disabled: true, description: copy.deleteHintBusy(activeChannels.length), onSelect: () => undefined }
        : { description: copy.deleteHintFree, onSelect: () => setDeleting(active) }),
    },
  ] : []

  const newItems: MobileSheetAction[] = [
    { key: 'template', icon: 'layer', label: t.library.templates.newTemplate, description: copy.newTemplateHint, onSelect: () => openName(null) },
    { key: 'import', icon: 'document-download', label: t.library.templates.importTemplate, description: copy.importTemplateHint, onSelect: () => setImporting(true) },
    { key: 'prompt', icon: 'magicpen', label: copy.newPrompt, onSelect: () => setEditingPrompt(null) },
    { key: 'backdrop', icon: 'video-play', label: copy.newBackdrop, onSelect: () => setAddingBackdrop(true) },
  ]

  const subtitle = ownerChannels.length ? copy.subtitle(ownerChannels.length) : copy.subtitleEmpty

  return (
    <MobilePage
      title={t.library.title}
      subtitle={subtitle}
      onBack={() => navigate(channelsHref(ownerChannels))}
      actions={<MobilePageAction icon="add" label={copy.addAria} onClick={() => setSheet('new')} />}
      banner={(
        <div className={stylex.props(mlib.tabsWrap).className}>
          <Segmented
            size="xl"
            fit="fill"
            label={t.library.tabAria}
            value={tab}
            onChange={(next) => navigate(paths.library(TABS.find((id) => id === next)))}
            options={[
              { id: 'templates', label: <span {...stylex.props(mlib.tabLabel)}>{t.library.tabs.templates}<span {...stylex.props(mlib.tabCount)}>{templateCount}</span></span> },
              { id: 'prompts', label: <span {...stylex.props(mlib.tabLabel)}>{t.library.tabs.prompts}<span {...stylex.props(mlib.tabCount)}>{promptItems.length}</span></span> },
              { id: 'backdrops', label: <span {...stylex.props(mlib.tabLabel)}>{t.library.tabs.backdrops}<span {...stylex.props(mlib.tabCount)}>{sourceChannels.length + sourceVideos.length}</span></span> },
              { id: 'overlays', label: <span {...stylex.props(mlib.tabLabel)}>{t.library.tabs.overlays}<span {...stylex.props(mlib.tabCount)}>{overlayItems.length}</span></span> },
            ]}
          />
        </div>
      )}
    >
      <div className={stylex.props(mlib.scroll).className}>
        {tab === 'templates' && (
          templates.initialLoading ? <p {...stylex.props(mlib.loading)}>{t.library.loading}</p>
            : templates.error ? <p role="alert" {...stylex.props(mlib.loading)}>{t.library.loadFail}</p>
              : templateCount === 0 ? (
                <div className={stylex.props(mlib.empty).className}>
                  <span className={stylex.props(mlib.emptyIcon).className} aria-hidden="true"><Icon name="layer" size={22} /></span>
                  <h2 {...stylex.props(mlib.emptyTitle)}>{t.library.templates.emptyTitle}</h2>
                  <p {...stylex.props(mlib.emptyText)}>{copy.templateEmptyText}</p>
                  <div className={stylex.props(mlib.emptyActions).className}>
                    <Button size="lg" variant="primary" onClick={() => openName(null)}>{t.library.templates.newTemplate}</Button>
                    <Button size="lg" variant="ghost" onClick={() => setImporting(true)}>{t.library.templates.importTemplate}</Button>
                  </div>
                </div>
              ) : (
                <>
                  <p {...stylex.props(mlib.help)}>{copy.templatesHelp}</p>
                  <div className={stylex.props(mlib.list).className}>
                    {templateItems.map((template) => (
                      <TemplateCard
                        key={template.id}
                        template={template}
                        onOpen={() => navigate(paths.template(template.id))}
                        onMenu={() => setActive(template)}
                      />
                    ))}
                  </div>
                </>
              )
        )}

        {tab === 'prompts' && (
          <>
            <div className={stylex.props(mlib.tabsWrap).className}>
              <Segmented
                label={t.library.prompts.filterAria}
                value={promptFilter}
                onChange={(next) => setPromptFilter(next === 'scene' || next === 'cover' ? next : 'all')}
                options={[
                  { id: 'all', label: `${t.library.prompts.filterAll} ${promptCounts.all}` },
                  { id: 'scene', label: `${t.library.prompts.filterScene} ${promptCounts.scene}` },
                  { id: 'cover', label: `${t.library.prompts.filterCover} ${promptCounts.cover}` },
                ]}
              />
            </div>
            {prompts.initialLoading ? <p {...stylex.props(mlib.loading)}>{t.library.loading}</p> : (
              <div className={stylex.props(mlib.grid2).className}>
                {visiblePrompts.map((prompt) => (
                  <button key={prompt.id} type="button" className={stylex.props(mlib.prompt).className}
                    onClick={() => setEditingPrompt(prompt)}>
                    <span {...stylex.props(mlib.promptArt, prompt.kind === 'cover' && mlib.promptArtCover)} aria-hidden="true">
                      {prompt.builtIn && <span {...stylex.props(mlib.promptBuilt)}>{copy.promptBuilt}</span>}
                    </span>
                    <span {...stylex.props(mlib.promptName)}>{prompt.name}</span>
                    <span {...stylex.props(mlib.promptSub)}>{copy.promptTag(t.library.prompts.kindName[prompt.kind], prompt.usedBy.length ? copy.usedAt(prompt.usedBy.map((ref) => ref.name)) : copy.unused)}</span>
                  </button>
                ))}
                <button type="button" className={stylex.props(mlib.promptNew).className} onClick={() => setEditingPrompt(null)}>
                  <Icon name="add" size={20} />{copy.newPrompt}
                </button>
              </div>
            )}
          </>
        )}

        {tab === 'backdrops' && (
          <>
            <p {...stylex.props(mlib.help)}>{copy.backdropsHelp}</p>
            {backdropChannels.initialLoading || backdropVideos.initialLoading ? <p {...stylex.props(mlib.loading)}>{t.library.loading}</p> : (
              <div className={stylex.props(mlib.list).className}>
                {sourceChannels.map((channel) => (
                  <SourceChannelRow key={channel.id} channel={channel} />
                ))}
                {sourceVideos.map((video) => (
                  <SourceVideoRow key={video.id} video={video} />
                ))}
                {sourceChannels.length + sourceVideos.length === 0 && (
                  <p {...stylex.props(mlib.help)}>{t.backdrop.tab.emptyTitle}</p>
                )}
                <div className={stylex.props(mlib.addRow).className}>
                  <Button size="lg" variant="ghost" icon={<Icon name="add" size={16} />} onClick={() => setAddingBackdrop(true)}>{copy.addBackdrop}</Button>
                </div>
              </div>
            )}
          </>
        )}

        {tab === 'overlays' && (
          <>
            <p {...stylex.props(mlib.help)}>{copy.overlaysHelp}</p>
            {overlays.initialLoading ? <p {...stylex.props(mlib.loading)}>{t.library.loading}</p> : (
              <>
                <div className={stylex.props(mlib.grid3).className}>
                  {overlayItems.map((overlay) => <OverlayTile key={overlay.id} overlay={overlay} />)}
                </div>
                <div className={stylex.props(mlib.addRow).className}>
                  <input ref={overlayInput} type="file" accept="video/mp4,video/webm" hidden
                    onChange={(event) => {
                      const file = event.currentTarget.files?.[0]
                      event.currentTarget.value = ''
                      if (file) void uploadOverlay(file)
                    }} />
                  <Button size="lg" variant="ghost" icon={<Icon name="add" size={16} />} onClick={() => overlayInput.current?.click()}>{copy.addOverlay}</Button>
                </div>
              </>
            )}
          </>
        )}
      </div>

      <ActionSheet
        open={active !== null}
        onClose={() => setActive(null)}
        title={active ? active.name : ''}
        actions={actionItems}
      />
      <ActionSheet
        open={sheet === 'new'}
        onClose={() => setSheet(null)}
        title={copy.addSheetTitle}
        actions={newItems}
      />

      {naming && (
        <Dialog onClose={() => setNaming(null)}
          title={naming.template ? t.library.templates.renameTitle(naming.template.name) : t.library.templates.createTitle}
          footer={<>
            <Button variant="quiet" onClick={() => setNaming(null)}>{t.library.cancel}</Button>
            <Button variant="primary" onClick={() => void saveName()}>{naming.template ? t.library.templates.renameAction : t.library.templates.createAction}</Button>
          </>}>
          <div className={stylex.props(mlib.field).className}>
            {!naming.template && <p {...stylex.props(mlib.fieldLabel)}>{t.library.templates.createBody}</p>}
            <TextField label={t.library.nameLabel} value={nameDraft} maxLength={60}
              placeholder={naming.template ? undefined : t.library.templates.createPlaceholder}
              error={nameError ?? undefined} onChange={setNameDraft} />
          </div>
        </Dialog>
      )}

      {attaching && (
        <AttachChannelDialog
          channels={ownerChannels}
          inLibrary
          onClose={() => setAttaching(false)}
        />
      )}

      <ConfirmSheet
        open={deleting !== null}
        onClose={() => setDeleting(null)}
        title={deleting ? t.library.template.deleteTitle(deleting.name) : ''}
        description={t.library.template.deleteBody}
        tone="danger"
        action={{ label: t.library.template.deleteAction, destructive: true, onSelect: async () => { if (deleting) await remove(deleting) } }}
        cancel={t.library.cancel}
      />

      {importing && (
        <TemplateImportDialog onClose={() => { setImporting(false); void invalidate(keys.templates()) }} />
      )}

      <Sheet open={editingPrompt !== undefined} onClose={() => setEditingPrompt(undefined)} size="md" height="92%">
        {editingPrompt !== undefined && (
          <PromptEditor
            key={editingPrompt?.id ?? 'new'}
            prompt={editingPrompt}
            onClose={() => setEditingPrompt(undefined)}
            onSaved={(saved) => { setEditingPrompt(saved); void refreshPrompts() }}
            onDeleted={() => { setEditingPrompt(undefined); void refreshPrompts() }}
            onDuplicate={editingPrompt ? () => void duplicatePrompt(editingPrompt) : undefined}
          />
        )}
      </Sheet>

      <Sheet open={addingBackdrop} onClose={() => setAddingBackdrop(false)} size="md" height="auto"
        title={t.backdrop.tab.linkLabel} subtitle={t.backdrop.tab.hint}>
        <div className={stylex.props(mlib.field).className}>
          <TextField value={backdropInput} placeholder={t.backdrop.tab.linkPlaceholder} error={backdropError ?? undefined} onChange={setBackdropInput} />
          <Button size="lg" variant="primary" pending={addingBackdropBusy} disabled={!backdropInput.trim()} onClick={() => void addBackdrop()}>{t.backdrop.tab.add}</Button>
        </div>
      </Sheet>
    </MobilePage>
  )
}

function TemplateCard({ template, onOpen, onMenu }: {
  template: TemplateSummary
  onOpen: () => void
  onMenu: () => void
}) {
  const copy = t.library.templates
  const mobile = t.library.mobile
  const broken = template.backdrop.enabled && template.backdrop.channelCount === 0 && template.backdrop.videoCount === 0
  const names = template.channels.map((channel) => channel.name)
  return (
    <article className={stylex.props(mlib.card).className}>
      <button type="button" className={stylex.props(mlib.thumb).className} onClick={onOpen} aria-label={mobile.openTemplateAria(template.name)}>
        <span {...stylex.props(mlib.thumbBars)} aria-hidden="true">
          <span {...stylex.props(mlib.thumbBar)} /><span {...stylex.props(mlib.thumbBar)} /><span {...stylex.props(mlib.thumbBar)} />
        </span>
        <span {...stylex.props(mlib.thumbTag)}>{styleOf(template)}</span>
      </button>
      <div className={stylex.props(mlib.body).className}>
        <div className={stylex.props(mlib.head).className}>
          <a href={`#${paths.template(template.id)}`} className={stylex.props(mlib.name).className}>{template.name}</a>
          <Button variant="icon" size="lg" aria-label={mobile.templateMenuAria(template.name)} onClick={onMenu} icon={<Icon name="more" size={20} />} xstyle={mlib.more} />
        </div>
        <span {...stylex.props(mlib.health)}>
          <span {...stylex.props(mlib.dot, broken && mlib.dotWarn)} aria-hidden="true" />
          {broken ? copy.healthFix : copy.healthOk}
        </span>
        <p {...stylex.props(mlib.recipe)}>
          {template.prompts.summary}{' · '}{imagesText(template)}{template.overlayCount > 0 && ` · ${copy.overlays(template.overlayCount)}`}
        </p>
        <div className={stylex.props(mlib.meta).className}>
          {names.length > 0 && (
            <span className={stylex.props(mlib.avatars).className} aria-hidden="true">
              {template.channels.slice(0, 3).map((channel) => (
                <InitialsAvatar key={channel.id} name={channel.name} initials={initials(channel.name)} size={20} xstyle={mlib.avatar} />
              ))}
            </span>
          )}
          <span>{names.length ? copy.usedText(names) : copy.usedText([])}</span>
          <span>{editedLabel(template.updatedAt, { today: copy.editedToday, date: copy.editedDate })}</span>
        </div>
      </div>
    </article>
  )
}

function SourceChannelRow({ channel }: { channel: BackdropChannel }) {
  const counts = Object.entries(channel.counts).filter(([, count]) => (count ?? 0) > 0)
  const kinds = counts.length ? counts.map(([kind, count]) => `${kindName(kind)} ${count}`).join(', ') : t.backdrop.tab.channelReading
  const used = channel.usedBy.map((ref) => ref.name)
  return (
    <div className={stylex.props(mlib.source).className}>
      {channel.avatarUrl
        ? <img src={channel.avatarUrl} alt="" className={stylex.props(mlib.sourceAvatar).className} width={40} height={40} />
        : <InitialsAvatar name={channel.title ?? channel.handle} size={40} xstyle={mlib.sourceAvatar} />}
      <span className={stylex.props(mlib.sourceMain).className}>
        <span {...stylex.props(mlib.sourceName)}>{channel.title ?? channel.handle}</span>
        <span {...stylex.props(mlib.sourceSub)}>{t.library.mobile.channelKind(platformName(channel.platform), kinds)}</span>
        {used.length > 0 && <span {...stylex.props(mlib.sourceUse)}>{t.library.mobile.usedAt(used)}</span>}
      </span>
      <span {...stylex.props(mlib.sourceChevron)} aria-hidden="true"><Icon name="arrow-r" size={16} /></span>
    </div>
  )
}

function SourceVideoRow({ video }: { video: BackdropVideo }) {
  const used = video.usedBy.map((ref) => ref.name)
  return (
    <div className={stylex.props(mlib.source).className}>
      <InitialsAvatar name={video.title} size={40} xstyle={mlib.sourceAvatar} />
      <span className={stylex.props(mlib.sourceMain).className}>
        <span {...stylex.props(mlib.sourceName)}>{video.title}</span>
        <span {...stylex.props(mlib.sourceSub)}>{t.library.mobile.videoKind(platformName(video.platform), segmentLabel(video.segment.fromSec, video.segment.toSec))}</span>
        {used.length > 0 && <span {...stylex.props(mlib.sourceUse)}>{t.library.mobile.usedAt(used)}</span>}
      </span>
      <span {...stylex.props(mlib.sourceChevron)} aria-hidden="true"><Icon name="arrow-r" size={16} /></span>
    </div>
  )
}

function OverlayTile({ overlay }: { overlay: Overlay }) {
  return (
    <div className={stylex.props(mlib.overlay).className}>
      <span className={stylex.props(mlib.overlayArt).className}>
        <video className={stylex.props(mlib.overlayVideo).className} src={overlay.url} muted playsInline loop preload="metadata" aria-label={overlay.name} />
      </span>
      <span {...stylex.props(mlib.overlayName)}>{overlay.name}</span>
      <span {...stylex.props(mlib.overlaySub)}>{overlay.usedBy.length ? t.library.mobile.usedAt(overlay.usedBy.map((ref) => ref.name)) : t.library.mobile.unused}</span>
    </div>
  )
}
