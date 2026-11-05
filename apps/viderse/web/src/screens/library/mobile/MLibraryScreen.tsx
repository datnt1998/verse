// MLibrary — Thư viện dùng chung trên điện thoại (artboard MLibrary.dc.html,
// tweak tab=templates|prompts|backdrops|overlays, sheet=none|actions|new).
// Dùng đúng dữ liệu/API của Thư viện máy tính (listTemplates, listPrompts,
// listOverlays, listBackdrop*), không thêm endpoint nào.
import { useEffect, useRef, useState } from 'react'
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
import { channelTint } from '../../../lib/channelTint'
import {
  addBackdropChannel, addBackdropVideo, backdropVideoFileHref, listBackdropChannels, listBackdropVideos, parseSource,
  refreshBackdropChannel, removeBackdropChannel, removeBackdropVideo,
  type BackdropChannel, type BackdropVideo,
} from '../../../api/backdrop'
import {
  addOverlay, createPrompt, createTemplate, downloadTemplate, duplicateTemplate, fieldErrors, listOverlays, listOwnerChannels,
  listPrompts, listTemplates, patchTemplate, removeOverlay, removeTemplate,
  type ImagePrompt, type Overlay, type PromptKind, type TemplateSummary,
} from '../../../api/library'
import { channelsHref } from '../../../lib/lastChannel'
import { AttachChannelDialog } from '../AttachChannelDialog'
import { TemplateImportDialog } from '../TemplateImportDialog'
import { PromptEditor } from '../PromptEditor'
import { TemplateArtwork } from '../TemplateArtwork'
import { styleOf, imagesText } from '../TemplatesTab'
import { editedLabel, initials } from '../fmt'
import { kindName, platformName, segmentLabel } from '../backdrops/fmt'
import { mlib } from './mlibrary.stylex'

type LibraryRoute = Extract<Route, { name: 'library' }>

type PromptFilter = 'all' | PromptKind

type LibraryAsset =
  | { kind: 'channel'; value: BackdropChannel }
  | { kind: 'video'; value: BackdropVideo }
  | { kind: 'overlay'; value: Overlay }

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
  // Ảnh mẫu hỏng/404 theo prompt (lưu URL) → quay về ô trung tính, không bịa ảnh khác.
  const [brokenSamples, setBrokenSamples] = useState<Record<string, string>>({})
  const [addingBackdrop, setAddingBackdrop] = useState(false)
  const [backdropInput, setBackdropInput] = useState('')
  const [backdropError, setBackdropError] = useState<string | null>(null)
  const [addingBackdropBusy, setAddingBackdropBusy] = useState(false)
  const [importing, setImporting] = useState(false)
  const overlayInput = useRef<HTMLInputElement>(null)
  const [selectedAsset, setSelectedAsset] = useState<{ kind: LibraryAsset['kind']; id: string } | null>(null)
  const [promptRouteError, setPromptRouteError] = useState<string | null>(null)

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
  const visiblePrompts = promptFilter === 'all' ? promptItems : promptItems.filter((prompt) => prompt.kind === promptFilter)
  const templateCount = templateItems.length
  const selectedChannel = selectedAsset?.kind === 'channel' ? sourceChannels.find((item) => item.id === selectedAsset.id) : undefined
  const selectedVideo = selectedAsset?.kind === 'video' ? sourceVideos.find((item) => item.id === selectedAsset.id) : undefined
  const selectedOverlay = selectedAsset?.kind === 'overlay' ? overlayItems.find((item) => item.id === selectedAsset.id) : undefined
  const asset: LibraryAsset | null = selectedChannel ? { kind: 'channel', value: selectedChannel }
    : selectedVideo ? { kind: 'video', value: selectedVideo }
      : selectedOverlay ? { kind: 'overlay', value: selectedOverlay } : null

  useEffect(() => { setSelectedAsset(null) }, [tab])

  useEffect(() => {
    setPromptRouteError(null)
    setEditingPrompt(tab === 'prompts' && route.prompt === 'new' ? null : undefined)
  }, [tab, route.prompt, route.promptId])

  useEffect(() => {
    if (tab !== 'prompts' || route.prompt === 'new' || !route.promptId) return
    setPromptRouteError(null)
    if (!prompts.data || prompts.error) {
      setEditingPrompt(undefined)
      return
    }
    const requested = prompts.data.items.find((prompt) => prompt.id === route.promptId)
    setEditingPrompt(requested)
    if (!requested) setPromptRouteError('Không tìm thấy chỉ dẫn ảnh này trong thư viện. Có thể chỉ dẫn đã bị xoá.')
  }, [tab, route.prompt, route.promptId, prompts.data, prompts.error])

  const closePrompt = () => {
    setEditingPrompt(undefined)
    navigate(paths.library('prompts'))
  }

  const openPrompt = (prompt: ImagePrompt | null) => {
    setSheet(null)
    navigate(paths.library('prompts', prompt ? (prompt.builtIn ? 'builtin' : 'mine') : 'new', prompt ? { promptId: prompt.id } : undefined))
  }

  const openName = (template: TemplateSummary | null) => {
    setActive(null)
    setSheet(null)
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
      openPrompt(copyOf)
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

  const exportTemplate = async (template: TemplateSummary) => {
    try {
      await downloadTemplate(template.id)
      toast(t.library.templates.exported, 'ok')
    } catch (err) {
      toast(err instanceof Error ? err.message : t.library.templates.exportFail, 'err')
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
          description: activeChannels.length ? copy.attachHint(activeChannels) : copy.attachUnused,
          onSelect: () => { setActive(null); setAttaching(true) },
        }
      : {
          key: 'attach', icon: 'channel', label: t.library.templates.addToChannel,
          disabled: true, description: t.library.templates.attachEmpty, onSelect: () => undefined,
        },
    {
      key: 'export', icon: 'export', label: copy.exportFile,
      onSelect: () => void exportTemplate(active),
    },
    {
      key: 'delete', icon: 'trash', label: copy.deleteTemplate,
      destructive: true,
      ...(activeChannels.length
        ? { disabled: true, description: copy.deleteHintBusy(activeChannels), onSelect: () => undefined }
        : { description: copy.deleteHintFree, onSelect: () => { setActive(null); setDeleting(active) } }),
    },
  ] : []

  const newItems: MobileSheetAction[] = [
    { key: 'template', icon: 'layer', label: t.library.templates.newTemplate, description: copy.newTemplateHint, onSelect: () => openName(null) },
    { key: 'import', icon: 'document-download', label: t.library.templates.importTemplate, description: copy.importTemplateHint, onSelect: () => { setSheet(null); setImporting(true) } },
    { key: 'prompt', icon: 'magicpen', label: copy.newPrompt, onSelect: () => openPrompt(null) },
    { key: 'backdrop', icon: 'video-play', label: copy.newBackdrop, onSelect: () => { setSheet(null); setAddingBackdrop(true) } },
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
            fit="equal"
            columns={4}
            buttonStyle={mlib.tabButton}
            label={t.library.tabAria}
            value={tab}
            onChange={(next) => navigate(paths.library(TABS.find((id) => id === next)))}
            options={[
              { id: 'templates', label: t.library.tabs.templates },
              { id: 'prompts', label: t.library.tabs.prompts },
              { id: 'backdrops', label: t.library.tabs.backdrops },
              { id: 'overlays', label: t.library.tabs.overlays },
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
                  <span className={stylex.props(mlib.emptyIcon).className} aria-hidden="true"><Icon name="layer" size={19} /></span>
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
                        prompts={prompts.data?.items}
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
            <div className={stylex.props(mlib.filterWrap).className}>
              <Segmented
                label={t.library.prompts.filterAria}
                value={promptFilter}
                size={26}
                radius={6}
                fit="content"
                xstyle={mlib.promptFilter}
                buttonStyle={mlib.promptFilterButton}
                onChange={(next) => setPromptFilter(next === 'scene' || next === 'cover' ? next : 'all')}
                options={[
                  { id: 'all', label: t.library.prompts.filterAll },
                  { id: 'scene', label: t.library.prompts.filterScene },
                  { id: 'cover', label: t.library.prompts.filterCover },
                ]}
              />
            </div>
            {promptRouteError && <p role="alert" {...stylex.props(mlib.detailError)}>{promptRouteError}</p>}
            {prompts.initialLoading ? <p {...stylex.props(mlib.loading)}>{t.library.loading}</p>
              : prompts.error ? <p role="alert" {...stylex.props(mlib.loading)}>{t.library.loadFail}</p> : (
              <div className={stylex.props(mlib.grid2).className}>
                {visiblePrompts.map((prompt) => (
                  <button key={prompt.id} type="button" className={stylex.props(mlib.prompt).className}
                    onClick={() => openPrompt(prompt)}>
                    <span {...stylex.props(mlib.promptArt, prompt.kind === 'cover' && mlib.promptArtCover)} aria-hidden="true">
                      {prompt.sampleUrls?.[0] && brokenSamples[prompt.id] !== prompt.sampleUrls[0] && (
                        <img {...stylex.props(mlib.promptImg)} src={prompt.sampleUrls?.[0]} alt="" loading="lazy"
                          onError={() => setBrokenSamples((prev) => ({ ...prev, [prompt.id]: prompt.sampleUrls?.[0] ?? '' }))} />
                      )}
                      {prompt.builtIn && <span {...stylex.props(mlib.promptBuilt)}>{copy.promptBuilt}</span>}
                    </span>
                    <span {...stylex.props(mlib.promptName)}>{prompt.name}</span>
                    <span {...stylex.props(mlib.promptSub)}>{copy.promptTag(t.library.prompts.kindName[prompt.kind], prompt.usedBy.length ? copy.usedAt(prompt.usedBy.map((ref) => ref.name)) : copy.unused)}</span>
                  </button>
                ))}
                <button type="button" className={stylex.props(mlib.promptNew).className} onClick={() => openPrompt(null)}>
                  <Icon name="add" size={20} />{copy.newPrompt}
                </button>
              </div>
            )}
          </>
        )}

        {tab === 'backdrops' && (
          <>
            <p {...stylex.props(mlib.help, mlib.helpTight)}>{copy.backdropsHelp}</p>
            {backdropChannels.initialLoading || backdropVideos.initialLoading ? <p {...stylex.props(mlib.loading)}>{t.library.loading}</p>
              : backdropChannels.error || backdropVideos.error ? <p role="alert" {...stylex.props(mlib.loading)}>{t.library.loadFail}</p> : (
              <div className={stylex.props(mlib.sourceList).className}>
                {sourceChannels.map((channel) => (
                  <SourceChannelRow key={channel.id} channel={channel} onOpen={() => setSelectedAsset({ kind: 'channel', id: channel.id })} />
                ))}
                {sourceVideos.map((video) => (
                  <SourceVideoRow key={video.id} video={video} onOpen={() => setSelectedAsset({ kind: 'video', id: video.id })} />
                ))}
                {sourceChannels.length + sourceVideos.length === 0 && (
                  <p {...stylex.props(mlib.help)}>{t.backdrop.tab.emptyTitle}</p>
                )}
                <div className={stylex.props(mlib.addRow).className}>
                  <Button block size="lg" variant="ghost" icon={<Icon name="add" size={16} />} onClick={() => setAddingBackdrop(true)}>{copy.addBackdrop}</Button>
                </div>
              </div>
            )}
          </>
        )}

        {tab === 'overlays' && (
          <>
            <p {...stylex.props(mlib.help)}>{copy.overlaysHelp}</p>
            {overlays.initialLoading ? <p {...stylex.props(mlib.loading)}>{t.library.loading}</p>
              : overlays.error ? <p role="alert" {...stylex.props(mlib.loading)}>{t.library.loadFail}</p> : (
              <>
                <div className={stylex.props(mlib.grid3).className}>
                  {overlayItems.map((overlay) => <OverlayTile key={overlay.id} overlay={overlay} onOpen={() => setSelectedAsset({ kind: 'overlay', id: overlay.id })} />)}
                </div>
                <div className={stylex.props(mlib.addRow).className}>
                  <input ref={overlayInput} type="file" accept="video/mp4,video/webm" hidden
                    onChange={(event) => {
                      const file = event.currentTarget.files?.[0]
                      event.currentTarget.value = ''
                      if (file) void uploadOverlay(file)
                    }} />
                  <Button block size="lg" variant="ghost" icon={<Icon name="add" size={16} />} onClick={() => overlayInput.current?.click()}>{copy.addOverlay}</Button>
                </div>
              </>
            )}
          </>
        )}
      </div>

      {active && <ActionSheet
        open
        onClose={() => setActive(null)}
        title={active.name}
        actions={actionItems}
      />}
      {sheet === 'new' && <ActionSheet
        open
        onClose={() => setSheet(null)}
        title={copy.addSheetTitle}
        actions={newItems}
      />}

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

      <Sheet open={editingPrompt !== undefined} onClose={closePrompt} size="md" height="92%" label={t.library.tabs.prompts}>
        {editingPrompt !== undefined && (
          <PromptEditor
            key={editingPrompt?.id ?? 'new'}
            prompt={editingPrompt}
            onClose={closePrompt}
            onSaved={(saved) => { setEditingPrompt(saved); void refreshPrompts().then(() => openPrompt(saved)) }}
            onDeleted={() => { closePrompt(); void refreshPrompts() }}
            onDuplicate={editingPrompt ? () => void duplicatePrompt(editingPrompt) : undefined}
          />
        )}
      </Sheet>

      {asset && <AssetDetail
        key={`${asset.kind}:${asset.value.id}`}
        asset={asset}
        templates={templateItems}
        templatesLoading={templates.initialLoading}
        templatesError={!!templates.error}
        retryTemplates={() => void templates.refresh()}
        onClose={() => setSelectedAsset(null)}
      />}

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

function TemplateCard({ template, prompts, onOpen, onMenu }: {
  template: TemplateSummary
  prompts?: readonly ImagePrompt[]
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
        <TemplateArtwork template={template} prompts={prompts} xstyle={mlib.promptImg} />
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
              {template.channels.slice(0, 3).map((channel, index) => (
                <InitialsAvatar key={channel.id} name={channel.name} initials={initials(channel.name)} size={20} bg={channelTint(channel.id)} xstyle={[mlib.avatar, index > 0 && mlib.avatarOverlap]} />
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

function SourceChannelRow({ channel, onOpen }: { channel: BackdropChannel; onOpen: () => void }) {
  const counts = Object.entries(channel.counts).filter(([, count]) => (count ?? 0) > 0)
  const kinds = counts.length ? counts.map(([kind, count]) => `${kindName(kind)} ${count}`).join(', ') : t.backdrop.tab.channelReading
  const used = channel.usedBy.map((ref) => ref.name)
  return (
    <button type="button" className={stylex.props(mlib.source).className} onClick={onOpen}>
      {channel.avatarUrl
        ? <img src={channel.avatarUrl} alt="" className={stylex.props(mlib.sourceAvatar).className} width={40} height={40} />
        : <InitialsAvatar name={channel.title ?? channel.handle} size={40} bg={channelTint(channel.id)} xstyle={mlib.sourceAvatar} />}
      <span className={stylex.props(mlib.sourceMain).className}>
        <span {...stylex.props(mlib.sourceName)}>{channel.title ?? channel.handle}</span>
        <span {...stylex.props(mlib.sourceSub)}>{t.library.mobile.channelKind(platformName(channel.platform), kinds)}</span>
        {used.length > 0 && <span {...stylex.props(mlib.sourceUse)}>{t.library.mobile.usedAt(used)}</span>}
      </span>
      <span {...stylex.props(mlib.sourceChevron)} aria-hidden="true"><Icon name="arrow-r" size={16} /></span>
    </button>
  )
}

function SourceVideoRow({ video, onOpen }: { video: BackdropVideo; onOpen: () => void }) {
  const used = video.usedBy.map((ref) => ref.name)
  return (
    <button type="button" className={stylex.props(mlib.source).className} onClick={onOpen}>
      <InitialsAvatar name={video.title} size={40} xstyle={mlib.sourceAvatar} />
      <span className={stylex.props(mlib.sourceMain).className}>
        <span {...stylex.props(mlib.sourceName)}>{video.title}</span>
        <span {...stylex.props(mlib.sourceSub)}>{t.library.mobile.videoKind(platformName(video.platform), segmentLabel(video.segment.fromSec, video.segment.toSec))}</span>
        {used.length > 0 && <span {...stylex.props(mlib.sourceUse)}>{t.library.mobile.usedAt(used)}</span>}
      </span>
      <span {...stylex.props(mlib.sourceChevron)} aria-hidden="true"><Icon name="arrow-r" size={16} /></span>
    </button>
  )
}

function OverlayTile({ overlay, onOpen }: { overlay: Overlay; onOpen: () => void }) {
  return (
    <button type="button" className={stylex.props(mlib.overlay).className} onClick={onOpen}>
      <span className={stylex.props(mlib.overlayArt).className}>
        {overlay.mime.startsWith('image/')
          ? <img className={stylex.props(mlib.overlayVideo).className} src={overlay.url} alt="" loading="lazy" />
          : <video className={stylex.props(mlib.overlayVideo).className} src={overlay.url} muted playsInline preload="metadata" tabIndex={-1} aria-hidden="true" />}
        <span {...stylex.props(mlib.overlayPlay)} aria-hidden="true"><Icon name="play" size={20} /></span>
      </span>
      <span {...stylex.props(mlib.overlayName)}>{overlay.name}</span>
      <span {...stylex.props(mlib.overlaySub)}>{overlay.usedBy.length ? t.library.mobile.usedAt(overlay.usedBy.map((ref) => ref.name)) : t.library.mobile.unused}</span>
    </button>
  )
}

/** Chi tiết dùng cùng nguồn dữ liệu với hàng đã mở; không giữ bản usage cũ. */
function AssetDetail({ asset, templates, templatesLoading, templatesError, retryTemplates, onClose }: {
  asset: LibraryAsset
  templates: TemplateSummary[]
  templatesLoading: boolean
  templatesError: boolean
  retryTemplates: () => void
  onClose: () => void
}) {
  const [confirming, setConfirming] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [mediaError, setMediaError] = useState(false)
  const channel = asset.kind === 'channel' ? asset.value : null
  const video = asset.kind === 'video' ? asset.value : null
  const overlay = asset.kind === 'overlay' ? asset.value : null
  const name = channel ? channel.title ?? channel.handle : video?.title ?? overlay?.name ?? ''
  const usages = asset.kind === 'overlay'
    ? asset.value.usedBy
    : asset.value.usedBy.map((ref) => ({ id: ref.templateId, name: ref.name }))
  const usedNames = usages.map((ref) => ref.name)
  const deleteLocked = overlay ? overlay.builtIn !== '' : usedNames.length > 0
  const deleteHint = overlay?.builtIn
    ? 'Hiệu ứng có sẵn không xoá được.'
    : usedNames.length ? `Đang dùng ở ${usedNames.join(', ')} · bỏ khỏi mẫu video trước`
      : 'Không mẫu video nào dùng'
  const removeLabel = channel ? t.backdrop.tab.channel.removeAction : video ? t.backdrop.tab.videos.removeAction : t.library.delete

  const refresh = async () => {
    setBusy(true)
    setError(null)
    try {
      if (channel) await refreshBackdropChannel(channel.id)
      else if (video?.state === 'failed') await addBackdropVideo(video.url, video.segment)
      await invalidate(['backdrop'])
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setBusy(false)
    }
  }

  const remove = async () => {
    // Usage được đọc lại trong lúc xác nhận; máy chủ còn chặn xung đột đồng thời.
    if (deleteLocked) throw new Error(deleteHint)
    if (channel) await removeBackdropChannel(channel.id)
    else if (video) await removeBackdropVideo(video.id)
    else if (overlay) await removeOverlay(overlay.id)
    await invalidate(overlay ? keys.overlays() : ['backdrop'])
    onClose()
  }

  const openTemplate = (id: string) => {
    onClose()
    navigate(paths.template(id, 'backdrop'))
  }

  if (confirming) return (
    <ConfirmSheet
      open
      onClose={() => setConfirming(false)}
      title={channel ? t.backdrop.tab.channel.removeTitle(name) : video ? t.backdrop.tab.videos.removeTitle(name) : t.library.overlays.removeConfirmTitle(name)}
      description={overlay ? t.library.overlays.removeConfirmBody : 'Nguồn sẽ bị bỏ khỏi thư viện. Có thể thêm lại bằng liên kết nguồn.'}
      tone="danger"
      action={{ label: removeLabel, destructive: true, onSelect: remove }}
      cancel={t.library.cancel}
    />
  )

  return (
    <Sheet open onClose={onClose} title={name} size="md" height="auto"
      subtitle={overlay ? t.library.overlays.background[overlay.background] : platformName((channel ?? video)!.platform)}>
      <div {...stylex.props(mlib.detail)}>
        {overlay && (overlay.mime.startsWith('image/')
          ? <img src={overlay.url} alt={overlay.name} {...stylex.props(mlib.detailMedia)} onError={() => setMediaError(true)} />
          : <video src={overlay.url} controls playsInline loop preload="metadata" tabIndex={0}
              aria-label={t.library.overlays.play} {...stylex.props(mlib.detailMedia)} onError={() => setMediaError(true)} />)}
        {video?.state === 'ready' && <video src={backdropVideoFileHref(video.id)} controls playsInline preload="metadata" tabIndex={0}
          aria-label={video.title} {...stylex.props(mlib.detailMedia)} onError={() => setMediaError(true)} />}
        {mediaError && <p role="alert" {...stylex.props(mlib.detailError)}>Không phát được tệp này. Kiểm tra kết nối rồi mở lại.</p>}
        {channel && <>
          <p {...stylex.props(mlib.detailText)}>{Object.entries(channel.counts).filter(([, count]) => (count ?? 0) > 0)
            .map(([kind, count]) => `${kindName(kind)} ${count}`).join(' · ') || (channel.listedAt ? 'Chưa có video dùng được.' : t.backdrop.tab.channel.reading)}</p>
          {channel.listError && <p role="alert" {...stylex.props(mlib.detailError)}>{channel.listError.message}</p>}
          {channel.pausedUntil && <p {...stylex.props(mlib.detailText)}>
            {t.backdrop.tab.channel.readNextAt(new Date(channel.pausedUntil).toLocaleString('vi-VN'))}
          </p>}
          <Button size="lg" variant="ghost" pending={busy} icon={<Icon name="refresh" size={16} />}
            onClick={() => void refresh()}>{t.backdrop.tab.channel.refresh}</Button>
        </>}
        {video && <>
          <p {...stylex.props(mlib.detailText)}>{segmentLabel(video.segment.fromSec, video.segment.toSec)}</p>
          {video.state === 'downloading' && <p role="status" {...stylex.props(mlib.detailText)}>
            {t.backdrop.tab.videos.downloading(String(Math.round((video.progress ?? 0) * 100)))}
          </p>}
          {video.state === 'failed' && <>
            <p role="alert" {...stylex.props(mlib.detailError)}>{video.errorDetail?.message ?? t.backdrop.tab.videos.failedTitle}</p>
            <Button size="lg" variant="ghost" pending={busy} onClick={() => void refresh()}>{t.backdrop.tab.videos.retry}</Button>
          </>}
        </>}
        {(channel || video) && <a href={(channel ?? video)!.url} target="_blank" rel="noopener noreferrer" {...stylex.props(mlib.detailLink)}>
          Mở nguồn trên {platformName((channel ?? video)!.platform)} <Icon name="arrow-r" size={16} />
        </a>}
        <p {...stylex.props(mlib.detailText)}>{usedNames.length ? t.library.mobile.usedAt(usedNames) : t.library.mobile.unused}</p>
        {usages.map((ref) => <Button key={ref.id} size="lg" variant="ghost" icon={<Icon name="layer" size={16} />}
          onClick={() => openTemplate(ref.id)}>{ref.name} · Nền &amp; hiệu ứng</Button>)}
        {overlay && <>
          <p {...stylex.props(mlib.detailText)}>{usages.length ? 'Mở mẫu video để chỉnh ở Nền & hiệu ứng.' : 'Bật cho một mẫu video · chọn ở Nền & hiệu ứng.'}</p>
          {!usages.length && (templatesLoading ? <p role="status" {...stylex.props(mlib.detailText)}>{t.library.loading}</p>
            : templatesError ? <>
              <p role="alert" {...stylex.props(mlib.detailError)}>{t.library.loadFail}</p>
              <Button size="lg" variant="ghost" onClick={retryTemplates}>{t.library.retry}</Button>
            </> : templates.length ? templates.map((template) => <Button key={template.id} size="lg" variant="ghost"
              icon={<Icon name="layer" size={16} />} onClick={() => openTemplate(template.id)}>{template.name}</Button>)
              : <>
                <p {...stylex.props(mlib.detailText)}>{t.library.templates.emptyTitle}</p>
                <Button size="lg" variant="ghost" onClick={() => { onClose(); navigate(paths.library('templates')) }}>{t.library.tabs.templates}</Button>
              </>)}
        </>}
        {error && <p role="alert" {...stylex.props(mlib.detailError)}>{error}</p>}
        {!overlay || !overlay.builtIn ? <>
          <Button size="lg" variant="danger" disabled={busy || deleteLocked} onClick={() => setConfirming(true)}
            icon={<Icon name="trash" size={16} />}>{removeLabel}</Button>
          {!overlay && <p {...stylex.props(mlib.detailText)}>{deleteHint}</p>}
        </> : <p {...stylex.props(mlib.detailText)}>{deleteHint}</p>}
        <Button size="lg" variant="quiet" onClick={onClose}>{t.library.close}</Button>
      </div>
    </Sheet>
  )
}
