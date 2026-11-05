// MTemplate — sửa template trên điện thoại (artboard MTemplate.dc.html, tweak
// screen=list|images|cover|backdrop|output, preview=fresh|stale|building,
// sheet=none|more). Dùng cùng useTemplateDraft / section của Template máy tính
// — mỗi mục một màn con, tự lưu và hiện thanh "Đã lưu · HH:MM".
import { useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import {
  ActionSheet, Banner, Button, ConfirmSheet, Dialog, Icon, MobilePage, MobilePageAction,
  TextField, toast, useTabMode, type IconName, type MobileSheetAction,
} from '@noverse/ui'
import { invalidate } from '@noverse/ui/data'
import { t } from '../../../i18n/library'
import { navigate, paths, TEMPLATE_SECTIONS, type Route, type TemplateSection } from '../../../router'
import { useResource } from '../../../data/useResource'
import { keys } from '../../../data/keys'
import {
  downloadTemplate, duplicateTemplate, getTemplate, listOverlays, listOwnerChannels, listPrompts, removeTemplate,
  type TemplateDetail,
} from '../../../api/library'
import { ApiError } from '../../../api/request'
import { listBackdropChannels } from '../../../api/backdrop'
import { ResponsiveSelect } from '../../../components/responsive-overlays'
import { useTemplateDraft } from '../template/useTemplateDraft'
import { ImagesSection } from '../template/ImagesSection'
import { CoverSection } from '../template/CoverSection'
import { BackdropSection } from '../template/BackdropSection'
import { OutputSection } from '../template/OutputSection'
import { AttachChannelDialog } from '../AttachChannelDialog'
import { TemplatePreviewPlayer } from '../template/TemplatePreviewPlayer'
import { useTemplatePreview } from '../template/useTemplatePreview'
import { styleOf, imagesText } from '../TemplatesTab'
import { clock, duration } from '../fmt'
import { healthWarn } from '../TemplateScreen'
import { mtpl } from './mtemplate.stylex'

type TemplateRoute = Extract<Route, { name: 'template' }>
type SectionId = TemplateSection

const SECTIONS = TEMPLATE_SECTIONS

export function MTemplateScreen({ route }: { route: TemplateRoute }) {
  const detail = useResource(keys.template(route.templateId), ({ signal }) => getTemplate(route.templateId, signal))
  if (detail.initialLoading) return <MobilePage title={t.library.title}><p {...stylex.props(mtpl.loading)}>{t.library.loading}</p></MobilePage>
  const notFound = detail.error instanceof ApiError && detail.error.status === 404
  if (!detail.data) return <MobilePage title={t.library.title} onBack={() => navigate(paths.library('templates'))}>
    <div {...stylex.props(mtpl.loading)}>
      <p role="alert">{notFound ? t.library.template.notFound : t.library.template.loadFail}</p>
      {notFound
        ? <Button xstyle={sx.touch} onClick={() => navigate(paths.library('templates'))}>{t.library.template.backToLibrary}</Button>
        : <Button xstyle={sx.touch} onClick={() => void detail.refresh()}>{t.library.retry}</Button>}
    </div>
  </MobilePage>
  return <MTemplateEditor key={detail.data.id} initial={detail.data} section={route.section} />
}

function MTemplateEditor({ initial, section }: { initial: TemplateDetail; section?: SectionId }) {
  useTabMode('hide')
  const copy = t.library.mobile
  const draft = useTemplateDraft(initial)
  const detail = draft.detail
  const recipe = detail.recipe
  const editorLockedReason = draft.reloading ? 'Đang tải lại…' : draft.saving ? t.library.template.saving : draft.dirty ? draft.conflict ? t.library.conflict : t.library.template.saveFail : undefined
  const [more, setMore] = useState(false)
  const [renaming, setRenaming] = useState(false)
  const [renameName, setRenameName] = useState(detail.name)
  const [renameError, setRenameError] = useState<string | null>(null)
  const [renameSaving, setRenameSaving] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const [attaching, setAttaching] = useState(false)
  const [pickedJob, setPickedJob] = useState<string | undefined>(undefined)

  const scenePrompts = useResource(keys.prompts('scene', ''), ({ signal }) => listPrompts({ kind: 'scene' }, signal))
  const coverPrompts = useResource(keys.prompts('cover', ''), ({ signal }) => listPrompts({ kind: 'cover' }, signal))
  const ownerChannels = useResource(keys.ownerChannels(), ({ signal }) => listOwnerChannels(signal))
  const overlays = useResource(keys.overlays(), ({ signal }) => listOverlays(signal))
  const backdropChannels = useResource(keys.backdropChannels(), ({ signal }) => listBackdropChannels(signal))

  const styleName = `${detail.name} · ${styleOf(detail)}`
  const bd = recipe.backdrop
  const warn = healthWarn(detail)
  const health = warn ? t.library.template.healthWarnChip : t.library.template.healthOkChip
  const used = detail.channels.length ? copy.usedChannels(detail.channels.length) : copy.noChannels
  const saveText = draft.reloading ? 'Đang tải lại…' : draft.saving ? copy.savingBar : draft.conflict ? t.library.conflict : draft.dirty || draft.error ? t.library.template.saveFail : copy.saveBar(clock(detail.savedAt || detail.updatedAt), null)
  const subtitle = copy.templateSub(health, used, saveText)
  const backdropSources = bd ? t.library.templates.backdrop(bd.channels.length, bd.fixedVideoIds.length) : ''
  const overlayNames = recipe.overlays
    .map((id) => overlays.data?.items.find((overlay) => overlay.id === id)?.name)
    .filter((name): name is string => Boolean(name))
  const linkedIds = new Set((bd?.channels ?? []).map((channel) => channel.channelId))
  let poolVideos = 0
  for (const channel of backdropChannels.data ?? []) {
    if (!linkedIds.has(channel.id)) continue
    for (const count of Object.values(channel.counts)) poolVideos += count ?? 0
  }
  poolVideos += bd?.fixedVideoIds.length ?? 0

  const preview = useTemplatePreview(detail.id, detail, draft.dirty)
  const previewKey = preview.state
  const stalePreview = previewKey === 'stale'
  const previewStateText = copy.previewState[previewKey]
  const previewProgress = preview.task
    ? t.library.template.preview.progress(preview.task.phase || t.library.template.preview.badge.building, preview.task.done, preview.task.total)
    : t.library.template.preview.badge.building
  const knownJob = preview.sources.some((source) => source.jobId === preview.jobId) ? preview.jobId : undefined
  const sourceJob = pickedJob ?? knownJob ?? preview.sources.find((source) => !source.disabled)?.jobId
  const selectedSource = preview.sources.find((source) => source.jobId === sourceJob)
  const canBuild = !!selectedSource && !selectedSource.disabled && !preview.busy && !draft.saving && !draft.reloading && !draft.dirty
  const buildLockedReason = draft.reloading ? 'Đang tải lại…' : draft.saving ? t.library.template.saving : draft.dirty ? draft.conflict ? t.library.conflict : t.library.template.saveFail : selectedSource?.reason ?? t.library.template.preview.buildLocked
  const renameFieldError = renameError ?? draft.fieldErrors.name

  const saveRename = async () => {
    const name = renameName.trim()
    if (!name) { setRenameError(t.library.templates.renameEmpty); return }
    setRenameError(null)
    setRenameSaving(true)
    const saved = await draft.saveName(name)
    setRenameSaving(false)
    if (saved) {
      toast(t.library.templates.renamed, 'ok')
      setRenaming(false)
    }
  }

  const duplicate = async () => {
    try {
      const created = await duplicateTemplate(detail.id)
      await invalidate(keys.templates())
      toast(t.library.templates.duplicated, 'ok')
      navigate(paths.template(created.id))
    } catch (err) {
      toast(err instanceof Error ? err.message : String(err), 'err')
    }
  }

  const exportFile = async () => {
    try {
      await downloadTemplate(detail.id)
      toast(t.library.templates.exported, 'ok')
    } catch (err) {
      toast(err instanceof Error ? err.message : t.library.templates.exportFail, 'err')
    }
  }

  const remove = async () => {
    await removeTemplate(detail.id)
    await Promise.all([invalidate(keys.templates()), invalidate(keys.template(detail.id))])
    toast(t.library.templates.deleted, 'ok')
    navigate(paths.library('templates'))
  }

  const compress = recipe.output.compressExport ? t.library.template.output.compressed.title : t.library.template.output.original.title
  const sectionSub: Record<SectionId, string> = {
    images: copy.sectionSub.images(styleOf(detail), detail.prompts.sceneName, imagesText(detail)),
    cover: detail.prompts.coverName,
    backdrop: copy.sectionSub.backdrop(backdropSources || t.library.template.health.noBackdrop, overlayNames.join(', ')),
    output: detail.channels.length ? copy.sectionSub.output(compress, detail.channels.length) : copy.sectionSub.outputNoChannels(compress),
  }

  const moreItems: MobileSheetAction[] = [
    { key: 'rename', icon: 'edit', label: t.library.templates.rename, onSelect: () => { setRenameName(detail.name); setRenameError(null); setRenaming(true) } },
    { key: 'duplicate', icon: 'copy', label: t.library.duplicate, onSelect: () => duplicate() },
    { key: 'export', icon: 'export', label: copy.exportFile, onSelect: () => void exportFile() },
    detail.channels.length
      ? { key: 'delete', icon: 'trash', label: copy.deleteTemplate, destructive: true, disabled: true, description: copy.deleteHintBusy(detail.channels.map((channel) => channel.name)), onSelect: () => undefined }
      : { key: 'delete', icon: 'trash', label: copy.deleteTemplate, destructive: true, description: copy.deleteHintFree, onSelect: () => setDeleting(true) },
  ]

  const sectionMeta = SECTIONS.map((id) => ({
    id,
    icon: t.library.template.sections[id].icon as IconName,
    label: t.library.template.sections[id].label,
    sub: sectionSub[id],
  }))

  const reload = () => {
    void draft.reload().then((fresh) => { if (fresh) { setRenameName(fresh.name); setRenameError(null) } })
  }
  const saveFeedback = <>
    {draft.conflict && <Banner tone="warning" title={t.library.conflict}
      action={{ label: t.library.template.conflictReload, onClick: reload }}>
      Bản đang sửa vẫn được giữ. Tải lại sẽ thay bằng bản mới trên máy chủ.
    </Banner>}
    {draft.error && <Banner tone="error" title={draft.error}
      action={draft.conflict ? undefined : { label: t.library.retry, onClick: draft.retry }} />}
    {Object.keys(draft.fieldErrors).length > 0 && <Banner tone="error" title={t.library.template.saveFail}>
      {Object.entries(draft.fieldErrors).map(([field, message]) => <p key={field} {...stylex.props(mtpl.previewError)}>
        {t.library.template.fieldError(t.library.template.fieldLabels[field] ?? (field === 'name' ? t.library.nameLabel : t.library.template.sectionGroup), message)}
      </p>)}
    </Banner>}
  </>
  const saveFooter = <div {...stylex.props(mtpl.saveBar)} role="status" aria-live="polite">
    <Icon name={draft.saving || draft.reloading || draft.dirty || draft.error || draft.conflict ? 'info' : 'tick-b'} size={14}
      xstyle={draft.saving || draft.reloading || draft.dirty || draft.error || draft.conflict ? mtpl.readyNeutral : mtpl.tick} />
    <span {...stylex.props(mtpl.saveText)}>{draft.dirty || draft.saving || draft.reloading || draft.error || draft.conflict ? saveText : copy.saveBar(clock(detail.savedAt ?? detail.updatedAt), stalePreview ? copy.previewStaleNote : null)}</span>
    {section && <Button size="sm" xstyle={sx.touch} onClick={() => navigate(paths.template(detail.id))}>{copy.previewAction}</Button>}
  </div>
  const overlaysAndSheets = <>
    <ActionSheet open={more} onClose={() => setMore(false)} title={styleName} actions={moreItems} />
    {renaming && <Dialog onClose={() => { if (!renameSaving && !draft.reloading) setRenaming(false) }} title={t.library.templates.renameTitle(detail.name)}
      footer={<>
        <Button variant="quiet" xstyle={sx.touch} disabled={renameSaving || draft.reloading} onClick={() => setRenaming(false)}>{t.library.cancel}</Button>
        <Button variant="primary" xstyle={sx.touch} disabled={renameSaving || draft.reloading || draft.conflict} onClick={() => void saveRename()}>{renameSaving ? t.library.saving : t.library.templates.renameAction}</Button>
      </>}>
      <div {...stylex.props(mtpl.field)}>
        <label htmlFor="mtpl-rename" {...stylex.props(mtpl.fieldLabel)}>{t.library.nameLabel}</label>
        <TextField id="mtpl-rename" value={renameName} maxLength={60} disabled={renameSaving || draft.reloading}
          aria-invalid={Boolean(renameFieldError) || undefined} aria-describedby={renameFieldError ? 'mtpl-rename-error' : undefined} onChange={(value) => { setRenameName(value); setRenameError(null) }} />
        {renameFieldError && <span id="mtpl-rename-error" role="alert" {...stylex.props(mtpl.previewError)}>{renameFieldError}</span>}
        {draft.error && <p role="alert" {...stylex.props(mtpl.previewError)}>{draft.error}</p>}
        {draft.conflict && <Banner tone="warning" title={t.library.conflict}
          action={{ label: t.library.template.conflictReload, onClick: reload }}>
          Bản đang sửa vẫn được giữ. Tải lại sẽ thay bằng bản mới trên máy chủ.
        </Banner>}
      </div>
    </Dialog>}
    <ConfirmSheet open={deleting} onClose={() => setDeleting(false)} title={t.library.template.deleteTitle(detail.name)}
      description={t.library.template.deleteBody} tone="danger"
      action={{ label: t.library.template.deleteAction, destructive: true, onSelect: remove }} cancel={t.library.cancel} />
    {attaching && <AttachChannelDialog channels={ownerChannels.data?.channels ?? []} inLibrary onClose={() => setAttaching(false)} />}
  </>

  // ── Màn con một mục ──────────────────────────────────────────────────────
  if (section && SECTIONS.includes(section)) {
    const label = t.library.template.sections[section].label
    return (
      <MobilePage
        title={<span {...stylex.props(mtpl.headerTitle)} title={label}>{label}</span>}
        subtitle={<span title={styleName}>{styleName}</span>}
        onBack={() => navigate(paths.template(detail.id))}
        actions={<MobilePageAction icon="more" label={copy.moreSheetAria} onClick={() => setMore(true)} />}
        footStyle={mtpl.footer}
        footer={saveFooter}
      >
        <div className={stylex.props(mtpl.section).className}>
          {saveFeedback}
          <fieldset disabled={draft.reloading} {...stylex.props(sx.editingFields)}>
          {section === 'images' && <ImagesSection recipe={recipe} price={detail.price} prompts={scenePrompts.data?.items ?? []} errors={draft.fieldErrors} onSave={draft.save} hideTitle />}
          {section === 'cover' && <CoverSection recipe={recipe} prompts={coverPrompts.data?.items ?? []} errors={draft.fieldErrors} onSave={draft.save} hideTitle />}
          {section === 'backdrop' && <BackdropSection template={detail} onSave={async (patch) => { draft.save(patch) }} saving={draft.saving} hideTitle />}
          {section === 'output' && <OutputSection detail={detail} onSave={draft.save} onDuplicate={() => void duplicate()} onDelete={remove} onAttach={() => setAttaching(true)} editorLockedReason={editorLockedReason} hideTitle />}
          </fieldset>
        </div>
        {overlaysAndSheets}
      </MobilePage>
    )
  }

  // ── Danh sách ────────────────────────────────────────────────────────────
  return (
    <MobilePage
      title={<span {...stylex.props(mtpl.headerTitle)} title={styleName}>{styleName}</span>}
      subtitle={<span title={subtitle}>{subtitle}</span>}
      onBack={() => navigate(paths.library('templates'))}
      actions={<MobilePageAction icon="more" label={copy.moreSheetAria} onClick={() => setMore(true)} />}
      footStyle={mtpl.footer}
      footer={saveFooter}
    >
      <div className={stylex.props(mtpl.scroll).className}>
        {saveFeedback}
        <div className={stylex.props(mtpl.card).className}>
          {preview.projectId ? (
            <TemplatePreviewPlayer projectId={preview.projectId} />
          ) : (
            <div className={stylex.props(mtpl.frame).className}>
              <span {...stylex.props(mtpl.frameArt)} aria-hidden="true" />
            </div>
          )}
          <div className={stylex.props(mtpl.body).className}>
            <div {...stylex.props(mtpl.stateRow)}>
              <span {...stylex.props(mtpl.previewState, stalePreview && mtpl.previewWarn)}>
                <span {...stylex.props(mtpl.dot, stalePreview && mtpl.dotWarn)} aria-hidden="true" />
                {previewStateText}
              </span>
              <span {...stylex.props(mtpl.builtAt)}>
                {preview.builtAt ? t.library.template.preview.builtAt(clock(preview.builtAt)) : t.library.template.preview.never}
              </span>
            </div>
            {preview.sources.length > 0 ? <ResponsiveSelect
              id="mtpl-preview-source"
              label={t.library.template.preview.videoTitle}
              aria-label={t.library.template.preview.pickSource}
              value={sourceJob ?? ''}
              disabled={preview.state === 'building' || preview.busy}
              inlineDescription
              onChange={(value) => setPickedJob(value || undefined)}
              options={preview.sources.map((source) => ({
                value: source.jobId,
                label: source.title,
                description: [source.channelName, duration(source.durationSec), source.disabled ? source.reason ?? t.library.template.preview.buildLocked : null].filter(Boolean).join(' · '),
                disabled: source.disabled,
              }))}
            /> : <p {...stylex.props(mtpl.help)}>{t.library.template.preview.noVideo}</p>}
            {preview.state === 'building' ? (
              <>
                <p role="status" {...stylex.props(mtpl.previewProgress)}>{previewProgress}</p>
                <Button variant="ghost" xstyle={mtpl.rebuild} disabled={preview.busy} onClick={() => void preview.cancel()}>
                  <Icon name="close" size={16} />
                  {t.library.template.preview.cancel}
                </Button>
              </>
            ) : (
              <Button
                variant="ghost"
                xstyle={mtpl.rebuild}
                disabled={!canBuild}
                title={canBuild ? undefined : buildLockedReason}
                onClick={() => { if (canBuild && selectedSource) void preview.start(selectedSource.jobId, preview.minutes) }}
              >
                <Icon name={preview.state === 'fresh' ? 'refresh' : 'play'} size={16} />
                {preview.state === 'fresh' ? t.library.template.preview.rebuild : t.library.template.preview.rebuildStale}
              </Button>
            )}
            {!canBuild && preview.state !== 'building' && <p {...stylex.props(mtpl.help)}>{buildLockedReason}</p>}
            {preview.error && <p role="alert" {...stylex.props(mtpl.previewError)}>{preview.error}</p>}
            <p {...stylex.props(mtpl.help)}>{t.library.template.preview.effect}</p>
          </div>
        </div>

        <section {...stylex.props(mtpl.settings)} aria-label={copy.settingsTitle}>
          <h2 {...stylex.props(mtpl.sectionTitle)}>{copy.settingsTitle}</h2>
          <div {...stylex.props(mtpl.navRows)}>
            {sectionMeta.map((item) => (
              <button key={item.id} type="button" {...stylex.props(mtpl.navRow)}
                onClick={() => navigate(paths.template(detail.id, item.id))}>
                <span {...stylex.props(mtpl.navIcon)} aria-hidden="true"><Icon name={item.icon} size={18} /></span>
                <span {...stylex.props(mtpl.navText)}>
                  <span {...stylex.props(mtpl.navLabel)} title={item.label}>{item.label}</span>
                  <span {...stylex.props(mtpl.navHint)} title={item.sub}>{item.sub}</span>
                </span>
                <Icon name="arrow-r" size={16} xstyle={mtpl.navChevron} />
              </button>
            ))}
          </div>
        </section>

        <div className={stylex.props(mtpl.ready).className}>
          <span {...stylex.props(mtpl.readyTitle)}>{copy.readiness.title}</span>
          <span {...stylex.props(mtpl.readyItem)}>
            <Icon name={backdropSources ? 'tick-b' : 'info'} size={15} xstyle={backdropSources ? mtpl.tick : mtpl.readyNeutral} />
            {backdropSources ? `${backdropSources} · ${t.library.template.health.pool(copy.readiness.poolLinked(linkedIds.size, bd?.fixedVideoIds.length ?? 0), String(poolVideos)).split('· ').pop()}` : t.library.template.health.noBackdrop}
          </span>
          <span {...stylex.props(mtpl.readyItem)}><Icon name="tick-b" size={15} xstyle={mtpl.tick} />{t.library.template.health.subtitles}</span>
          <span {...stylex.props(mtpl.readyItem)}><Icon name="tick-b" size={15} xstyle={mtpl.tick} />{t.library.template.health.loudness}</span>
        </div>

        <p {...stylex.props(mtpl.help)}>{t.library.template.preview.whenBody}</p>
      </div>

      {overlaysAndSheets}
    </MobilePage>
  )
}

const sx = stylex.create({
  touch: { minHeight: 'var(--vv-s-44)' },
  editingFields: { margin: 0, padding: 0, borderWidth: 0, minWidth: 0 },
})
