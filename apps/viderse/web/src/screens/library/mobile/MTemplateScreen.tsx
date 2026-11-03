// MTemplate — sửa template trên điện thoại (artboard MTemplate.dc.html, tweak
// screen=list|images|cover|backdrop|output, preview=fresh|stale|building,
// sheet=none|more). Dùng cùng useTemplateDraft / section của Template máy tính
// — mỗi mục một màn con, tự lưu và hiện thanh "Đã lưu · HH:MM".
import { useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import {
  ActionSheet, Button, ConfirmSheet, Dialog, Icon, MobilePage, MobilePageAction,
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
import { listBackdropChannels } from '../../../api/backdrop'
import { useTemplateDraft } from '../template/useTemplateDraft'
import { ImagesSection } from '../template/ImagesSection'
import { CoverSection } from '../template/CoverSection'
import { BackdropSection } from '../template/BackdropSection'
import { OutputSection } from '../template/OutputSection'
import { AttachChannelDialog } from '../AttachChannelDialog'
import { TemplatePreviewPlayer } from '../template/TemplatePreviewPlayer'
import { useTemplatePreview } from '../template/useTemplatePreview'
import { styleOf, imagesText } from '../TemplatesTab'
import { clock } from '../fmt'
import { healthWarn } from '../TemplateScreen'
import { mtpl } from './mtemplate.stylex'

type TemplateRoute = Extract<Route, { name: 'template' }>
type SectionId = TemplateSection

const SECTIONS = TEMPLATE_SECTIONS

export function MTemplateScreen({ route }: { route: TemplateRoute }) {
  const detail = useResource(keys.template(route.templateId), ({ signal }) => getTemplate(route.templateId, signal))
  if (detail.initialLoading) return <MobilePage title={t.library.title}><p {...stylex.props(mtpl.loading)}>{t.library.loading}</p></MobilePage>
  if (detail.error || !detail.data) return <MobilePage title={t.library.title} onBack={() => navigate(paths.library('templates'))}><p role="alert" {...stylex.props(mtpl.loading)}>{t.library.template.loadFail}</p></MobilePage>
  return <MTemplateEditor key={detail.data.id} initial={detail.data} section={route.section} />
}

function MTemplateEditor({ initial, section }: { initial: TemplateDetail; section?: SectionId }) {
  useTabMode('hide')
  const copy = t.library.mobile
  const draft = useTemplateDraft(initial)
  const detail = draft.detail
  const recipe = detail.recipe
  const editorLockedReason = draft.saving ? t.library.template.saving : draft.dirty ? draft.conflict ? t.library.template.conflict : t.library.template.saveFail : undefined
  const [more, setMore] = useState(false)
  const [renaming, setRenaming] = useState(false)
  const [renameName, setRenameName] = useState(detail.name)
  const [renameError, setRenameError] = useState<string | null>(null)
  const [deleting, setDeleting] = useState(false)
  const [attaching, setAttaching] = useState(false)
  const [previewSheet, setPreviewSheet] = useState(false)

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
  const savedTime = clock(detail.savedAt || detail.updatedAt)
  const subtitle = copy.templateSub(health, used, copy.savedAt(savedTime))

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

  const saveRename = async () => {
    const name = renameName.trim()
    if (!name) { setRenameError(t.library.templates.renameEmpty); return }
    draft.saveName(name)
    toast(t.library.templates.renamed, 'ok')
    setRenaming(false)
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
      ? { key: 'delete', icon: 'trash', label: copy.deleteTemplate, destructive: true, disabled: true, description: copy.deleteHintBusy(detail.channels.length), onSelect: () => undefined }
      : { key: 'delete', icon: 'trash', label: copy.deleteTemplate, destructive: true, description: copy.deleteHintFree, onSelect: () => setDeleting(true) },
  ]

  const sectionMeta = SECTIONS.map((id) => ({
    id,
    icon: t.library.template.sections[id].icon as IconName,
    label: t.library.template.sections[id].label,
    sub: sectionSub[id],
  }))

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
        footer={(
          <div className={stylex.props(mtpl.saveBar).className} role="status">
            <Icon name="tick-b" size={14} xstyle={mtpl.tick} />
            <span {...stylex.props(mtpl.saveText)}>{draft.saving ? copy.savingBar : copy.saveBar(clock(detail.savedAt ?? detail.updatedAt), stalePreview ? copy.previewStaleNote : null)}</span>
            <Button size="sm" onClick={() => navigate(paths.template(detail.id))}>{copy.previewAction}</Button>
          </div>
        )}
      >
        <div className={stylex.props(mtpl.section).className}>
          {section === 'images' && <ImagesSection recipe={recipe} price={detail.price} prompts={scenePrompts.data?.items ?? []} errors={draft.fieldErrors} onSave={draft.save} hideTitle />}
          {section === 'cover' && <CoverSection recipe={recipe} prompts={coverPrompts.data?.items ?? []} errors={draft.fieldErrors} onSave={draft.save} hideTitle />}
          {section === 'backdrop' && <BackdropSection template={detail} onSave={async (patch) => { draft.save(patch) }} saving={draft.saving} hideTitle />}
          {section === 'output' && <OutputSection detail={detail} onSave={draft.save} onDuplicate={() => void duplicate()} onDelete={remove} onAttach={() => setAttaching(true)} editorLockedReason={editorLockedReason} hideTitle />}
        </div>
        <ActionSheet open={more} onClose={() => setMore(false)} title={styleName} actions={moreItems} />
      <ActionSheet open={previewSheet} onClose={() => setPreviewSheet(false)}
        title={t.library.template.preview.pickSource}
        actions={preview.sources.filter((source) => !source.disabled).map((source) => ({
          key: source.jobId, icon: 'play' as IconName, label: source.title,
          description: t.library.template.preview.sourceOption(source.title, Math.max(1, Math.round(source.durationSec / 60))),
          onSelect: () => { setPreviewSheet(false); void preview.start(source.jobId, preview.minutes) },
        }))} />
        {renaming && (
          <Dialog onClose={() => setRenaming(false)} title={t.library.templates.renameTitle(detail.name)}
            footer={<>
              <Button variant="quiet" onClick={() => setRenaming(false)}>{t.library.cancel}</Button>
              <Button variant="primary" onClick={() => void saveRename()}>{t.library.templates.renameAction}</Button>
            </>}>
            <div className={stylex.props(mtpl.field).className}>
              <label htmlFor="mtpl-rename" {...stylex.props(mtpl.fieldLabel)}>{t.library.nameLabel}</label>
              <TextField id="mtpl-rename" value={renameName} maxLength={60} aria-invalid={Boolean(renameError) || undefined} onChange={setRenameName} />
              {renameError && <span role="alert" {...stylex.props(mtpl.fieldLabel)}>{renameError}</span>}
            </div>
          </Dialog>
        )}
        <ConfirmSheet open={deleting} onClose={() => setDeleting(false)} title={t.library.template.deleteTitle(detail.name)}
          description={t.library.template.deleteBody} tone="danger"
          action={{ label: t.library.template.deleteAction, destructive: true, onSelect: remove }} cancel={t.library.cancel} />
        {attaching && <AttachChannelDialog channels={ownerChannels.data?.channels ?? []} inLibrary onClose={() => setAttaching(false)} />}
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
    >
      <div className={stylex.props(mtpl.scroll).className}>
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
                aria-disabled={!preview.sources.some((source) => !source.disabled) || preview.busy || undefined}
                title={preview.sources.some((source) => !source.disabled) ? undefined : (preview.sources[0]?.reason ?? t.library.template.preview.buildLocked)}
                onClick={() => { if (preview.sources.some((source) => !source.disabled) && !preview.busy) setPreviewSheet(true) }}
              >
                <Icon name={preview.state === 'fresh' ? 'refresh' : 'play'} size={16} />
                {preview.state === 'fresh' ? t.library.template.preview.rebuild : t.library.template.preview.rebuildStale}
              </Button>
            )}
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

      <ActionSheet open={more} onClose={() => setMore(false)} title={styleName} actions={moreItems} />
      <ActionSheet open={previewSheet} onClose={() => setPreviewSheet(false)}
        title={t.library.template.preview.pickSource}
        actions={preview.sources.filter((source) => !source.disabled).map((source) => ({
          key: source.jobId, icon: 'play' as IconName, label: source.title,
          description: t.library.template.preview.sourceOption(source.title, Math.max(1, Math.round(source.durationSec / 60))),
          onSelect: () => { setPreviewSheet(false); void preview.start(source.jobId, preview.minutes) },
        }))} />
      {renaming && (
        <Dialog onClose={() => setRenaming(false)} title={t.library.templates.renameTitle(detail.name)}
          footer={<>
            <Button variant="quiet" onClick={() => setRenaming(false)}>{t.library.cancel}</Button>
            <Button variant="primary" onClick={() => void saveRename()}>{t.library.templates.renameAction}</Button>
          </>}>
          <div className={stylex.props(mtpl.field).className}>
            <label htmlFor="mtpl-rename" {...stylex.props(mtpl.fieldLabel)}>{t.library.nameLabel}</label>
            <TextField id="mtpl-rename" value={renameName} maxLength={60} aria-invalid={Boolean(renameError) || undefined} onChange={setRenameName} />
            {renameError && <span role="alert" {...stylex.props(mtpl.fieldLabel)}>{renameError}</span>}
          </div>
        </Dialog>
      )}
      <ConfirmSheet open={deleting} onClose={() => setDeleting(false)} title={t.library.template.deleteTitle(detail.name)}
        description={t.library.template.deleteBody} tone="danger"
        action={{ label: t.library.template.deleteAction, destructive: true, onSelect: remove }} cancel={t.library.cancel} />
      {attaching && <AttachChannelDialog channels={ownerChannels.data?.channels ?? []} inLibrary onClose={() => setAttaching(false)} />}
    </MobilePage>
  )
}
