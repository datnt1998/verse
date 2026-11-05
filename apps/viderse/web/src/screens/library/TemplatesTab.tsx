import * as stylex from '@stylexjs/stylex'
import { color, line } from '@noverse/ui/tokens.stylex'
import { libraryVars } from './libraryTokens.stylex'
// Tab Mẫu video của Thư viện (Library.dc.html › tweak tabs=templates): lưới thẻ
// mẫu video gồm sức khoẻ, prompt đang chọn, nền/overlay đang dùng, kênh dùng làm
// mặc định, và menu Đổi tên / Đặt làm mặc định cho kênh… / Xoá (khoá kèm lý do
// nêu đúng kênh đang dùng).
import { useState } from 'react'
import { ActionMenu, Banner, Button, Dialog, Icon, TextField, confirm, toast } from '@noverse/ui'
import { invalidate } from '@noverse/ui/data'
import { t } from '../../i18n/library'
import { navigate, paths } from '../../router'
import { useResource } from '../../data/useResource'
import { keys } from '../../data/keys'
import {
  downloadTemplate, duplicateTemplate, listPrompts, listTemplates, patchTemplate, removeTemplate,
  type ImagePrompt, type OwnerChannelRef, type TemplateSummary,
} from '../../api/library'
import { AttachChannelDialog } from './AttachChannelDialog'
import { editedLabel, initials, num } from './fmt'
import { TemplateArtwork } from './TemplateArtwork'

export const styleOf = (template: TemplateSummary) => template.style === 'full'
  ? t.library.templates.style.full
  : template.framing === 'close' ? t.library.templates.style.close : t.library.templates.style.cutout

export const imagesText = (template: TemplateSummary) => template.images.mode === 'none'
  ? t.library.templates.imagesNone
  : template.images.mode === 'per-minutes'
    ? t.library.templates.imagesPerMinutes(num(template.images.minutes))
    : t.library.templates.imagesFixed(num(template.images.count))

function TemplateCard({ template, prompts, channels, onRename, onAttach, onDeleted }: {
  template: TemplateSummary
  prompts?: readonly ImagePrompt[]
  channels: OwnerChannelRef[]
  onRename: () => void
  onAttach: () => void
  onDeleted: (message?: string) => void
}) {
  const copy = t.library.templates
  const names = template.channels.map((channel) => channel.name)
  const inUse = names.length > 0
  const broken = template.backdrop.enabled && template.backdrop.channelCount === 0 && template.backdrop.videoCount === 0
  const backdropText = t.library.templates.backdrop(template.backdrop.channelCount, template.backdrop.videoCount)

  const remove = async () => {
    const accepted = await confirm({
      title: copy.deleteTitle(template.name),
      body: copy.deleteBody,
      tone: 'danger',
      confirmLabel: t.library.delete,
      cancelLabel: t.library.cancel,
      onConfirm: () => undefined,
    })
    if (!accepted) return
    try {
      await removeTemplate(template.id)
      await invalidate(keys.templates())
      onDeleted()
    } catch (err) {
      onDeleted(err instanceof Error ? err.message : String(err))
    }
  }

  const exportFile = async () => {
    try {
      await downloadTemplate(template.id)
      toast(copy.exported, 'ok')
    } catch (err) {
      onDeleted(err instanceof Error ? err.message : copy.exportFail)
    }
  }

  const duplicate = async () => {
    try {
      await duplicateTemplate(template.id)
      await invalidate(keys.templates())
      toast(copy.duplicated, 'ok')
    } catch (err) {
      onDeleted(err instanceof Error ? err.message : String(err))
    }
  }

  return (
    <article className={`vlb-card ${stylex.props(sx.card).className}`}>
      <div className={`vlb-strip ${stylex.props(sx.strip).className}`} data-style={template.framing === 'close' ? 'close' : template.style} aria-hidden="true">
        <TemplateArtwork template={template} prompts={prompts} xstyle={sx.stripArtwork} />
        <span className={`vlb-strip-chip vlb-strip-chip--health ${stylex.props(sx.stripChipStripChipHealth, ((broken ? 'warn' : 'ok') === "ok") && sx.stripChipStripChipHealthDataToneOk, ((broken ? 'warn' : 'ok') === "warn") && sx.stripChipStripChipHealthDataToneWarn).className}`} data-tone={broken ? 'warn' : 'ok'}>{broken ? copy.healthFix : copy.healthOk}</span>
        <span className={`vlb-strip-chip vlb-strip-chip--style ${stylex.props(sx.stripChipStripChipStyle).className}`}>{styleOf(template)}</span>
      </div>
      <div className={`vlb-card-body ${stylex.props(sx.cardBody).className}`}>
        <div className={`vlb-card-top ${stylex.props(sx.cardTop).className}`}>
          <h3 className={`vlb-card-name ${stylex.props(sx.cardName).className}`}>
            <a {...stylex.props(sx.vlbCardNameA)} href={`#${paths.template(template.id)}`}>{template.name}</a>
          </h3>
          <span className={`vlb-card-edited ${stylex.props(sx.cardEdited).className}`}>{editedLabel(template.updatedAt, { today: copy.editedToday, date: copy.editedDate })}</span>
        </div>
        <div className={`vlb-chips ${stylex.props(sx.chips).className}`}>
          {backdropText && <span className={`vlb-chip ${stylex.props(sx.chip).className}`}>{backdropText}</span>}
        </div>
        <p className={`vlb-card-sum ${stylex.props(sx.cardSum).className}`}>
          {template.prompts.summary}
          {' · '}
          {imagesText(template)}
          {template.overlayCount > 0 && ` · ${t.library.templates.overlays(template.overlayCount)}`}
        </p>
        <div className={`vlb-card-meta ${stylex.props(sx.cardMeta).className}`}>
          {inUse && (
            <span className={`vlb-avatars ${stylex.props(sx.avatars).className}`} aria-hidden="true">
              {template.channels.slice(0, 3).map((channel, index) => (
                <span key={channel.id} className={`vlb-avatar vlb-avatar--sm ${stylex.props(sx.avatarAvatarSm, index > 0 && sx.avatarOverlap).className}`} title={channel.name}>{initials(channel.name)}</span>
              ))}
              {template.channels.length > 3 && <span className={`vlb-avatar vlb-avatar--sm ${stylex.props(sx.avatarAvatarSm, sx.avatarOverlap).className}`}>+{template.channels.length - 3}</span>}
            </span>
          )}
          <span>{inUse ? copy.usedText(names) : copy.usedText([])}</span>
        </div>
        <div className={`vlb-card-actions ${stylex.props(sx.cardActions).className}`}>
          <Button xstyle={[sx.vlbCardActionsButton]} size="sm" variant="ghost" onClick={() => navigate(paths.template(template.id))}>{broken ? copy.openFix : copy.open}</Button>
          <Button xstyle={[sx.vlbCardActionsButton]} size="sm" variant="ghost" aria-label={copy.duplicateAria(template.name)}
            onClick={() => void duplicate()}><Icon name="copy" size={14} />{t.library.duplicate}</Button>
          <span className={`vlb-card-end ${stylex.props(sx.cardEnd).className}`}>
            <Button xstyle={[sx.vlbCardActionsButton]} size="sm" variant="icon" aria-label={copy.exportAria(template.name)} title={copy.exportAria(template.name)}
              onClick={() => void exportFile()}><Icon name="export" size={15} /></Button>
            <ActionMenu alwaysVisible triggerStyle={sx.touchTarget}
              label={copy.menuAria(template.name)}
              items={[
                { id: 'rename', label: copy.rename, icon: <Icon name="edit" size={16} />, onSelect: onRename },
                {
                  id: 'channel', label: copy.addToChannel, icon: <Icon name="channel" size={16} />,
                  ...(channels.length ? { onSelect: onAttach } : { lockedReason: copy.noChannelsReason }),
                },
                {
                  id: 'delete', label: copy.deleteItem, icon: <Icon name="trash" size={16} />, danger: true,
                  ...(inUse ? { lockedReason: copy.deleteReason(names) } : { onSelect: () => void remove() }),
                },
              ]}
            />
          </span>
        </div>
      </div>
    </article>
  )
}

/** Ô cuối lưới template (artboard Library): thả tệp .zip để mở hộp Nhập template. */
function ImportTile({ onOpen, onFiles }: { onOpen: () => void; onFiles: (files: File[]) => void }) {
  const copy = t.library.templates
  const [over, setOver] = useState(false)
  return (
    <button
      type="button"
      className={`vlb-droptile ${stylex.props(sx.droptile, over && sx.droptileOver).className}`}
      data-over={over || undefined}
      onClick={onOpen}
      onDragOver={(event) => {
        if (!Array.from(event.dataTransfer.types).includes('Files')) return
        event.preventDefault()
        setOver(true)
      }}
      onDragLeave={() => setOver(false)}
      onDrop={(event) => {
        event.preventDefault()
        setOver(false)
        const files = Array.from(event.dataTransfer.files)
        if (files.length > 0) onFiles(files)
      }}
    >
      <Icon name="download" size={22} />
      <span className={`vlb-droptile-title ${stylex.props(sx.droptileTitle).className}`}>{copy.importTileTitle}</span>
      <span className={`vlb-droptile-hint ${stylex.props(sx.droptileHint).className}`}>{copy.importTileHint}</span>
    </button>
  )
}

export function TemplatesTab({ channels, onImport }: {
  channels: OwnerChannelRef[]
  /** mở hộp Nhập template, kèm tệp vừa thả vào ô cuối lưới */
  onImport: (files: File[]) => void
}) {
  const copy = t.library.templates
  const templates = useResource(keys.templates(), ({ signal }) => listTemplates(signal))
  const prompts = useResource(keys.prompts('', ''), ({ signal }) => listPrompts({}, signal))
  const [renaming, setRenaming] = useState<TemplateSummary | null>(null)
  const [renameName, setRenameName] = useState('')
  const [renameError, setRenameError] = useState<string | null>(null)
  const [actionError, setActionError] = useState<string | null>(null)
  const [attach, setAttach] = useState(false)
  const items = templates.data?.items ?? []

  const openRename = (template: TemplateSummary) => {
    setRenaming(template)
    setRenameName(template.name)
    setRenameError(null)
  }

  const saveRename = async () => {
    if (!renaming) return
    try {
      await patchTemplate(renaming.id, { revision: renaming.revision, name: renameName })
      await invalidate(keys.templates())
      setRenaming(null)
    } catch (err) {
      setRenameError(err instanceof Error ? err.message : String(err))
    }
  }

  return (
    <div className={`vlb-scroll ${stylex.props(sx.scroll).className}`}>
      <p className={`vlb-intro ${stylex.props(sx.intro).className}`}>{copy.intro}</p>
      {actionError && <Banner tone="error" title={actionError} />}
      {Boolean(templates.error) && <Banner tone="error" title={t.library.loadFail}><Button size="sm" onClick={() => void templates.refresh()}>{t.library.retry}</Button></Banner>}
      {items.length === 0 && !templates.initialLoading && !templates.error && (
        <div className={`vlb-empty ${stylex.props(sx.empty).className}`}>
          <p className={`vlb-empty-title ${stylex.props(sx.emptyTitle).className}`}>{copy.emptyTitle}</p>
          <p className={`vlb-hint ${stylex.props(sx.hint).className}`}>{copy.emptyHint}</p>
        </div>
      )}
      {!templates.initialLoading && (!templates.error || items.length > 0) && (
        <div className={`vlb-grid ${stylex.props(sx.grid).className}`}>
          {items.map((template) => (
            <TemplateCard
              key={template.id}
              template={template}
              prompts={prompts.data?.items}
              channels={channels}
              onRename={() => openRename(template)}
              onAttach={() => setAttach(true)}
              onDeleted={(message) => { if (message) setActionError(message); else setActionError(null) }}
            />
          ))}
          <ImportTile onOpen={() => onImport([])} onFiles={onImport} />
        </div>
      )}

      {renaming && (
        <Dialog title={copy.renameTitle(renaming.name)} onClose={() => setRenaming(null)} width={420}
          footer={<>
            <Button size="sm" onClick={() => setRenaming(null)}>{t.library.cancel}</Button>
            <Button size="sm" variant="primary" onClick={() => void saveRename()}>{copy.renameAction}</Button>
          </>}>
          <label className={`vlb-field ${stylex.props(sx.field).className}`}>
            <span className="lbl">{t.library.nameLabel}</span>
            <TextField value={renameName} autoFocus aria-invalid={renameError ? true : undefined}
              onChange={setRenameName}
              onKeyDown={(event) => { if (event.key === 'Enter') void saveRename() }} />
          </label>
          {renameError && <p className={`vlb-fielderr ${stylex.props(sx.fielderr).className}`} role="alert">{renameError}</p>}
        </Dialog>
      )}
      {attach && <AttachChannelDialog inLibrary channels={channels} onClose={() => setAttach(false)} />}
    </div>
  )
}

const sx = stylex.create({
  card: { display: "flex", flexDirection: "column", minWidth: 0, backgroundColor: "transparent", backgroundImage: libraryVars.promptEditorCardCardFlatBackgroundImage, borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: { default: line["--line-2"], ":hover": line["--line-3"] }, borderRightWidth: "var(--vv-s-1)", borderRightStyle: "solid", borderRightColor: { default: line["--line-2"], ":hover": line["--line-3"] }, borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: { default: line["--line-2"], ":hover": line["--line-3"] }, borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "solid", borderLeftColor: { default: line["--line-2"], ":hover": line["--line-3"] }, borderTopLeftRadius: "var(--vv-r-14)", borderTopRightRadius: "var(--vv-r-14)", borderBottomRightRadius: "var(--vv-r-14)", borderBottomLeftRadius: "var(--vv-r-14)", overflowX: "hidden", overflowY: "hidden" },
  strip: { display: "flex", alignItems: "flex-end", justifyContent: "center", rowGap: "var(--vv-s-6)", columnGap: "var(--vv-s-6)", height: libraryVars.templatesTabStripHeight, paddingTop: "var(--vv-s-10)", paddingRight: "var(--vv-s-10)", paddingBottom: "var(--vv-s-10)", paddingLeft: "var(--vv-s-10)", backgroundColor: "transparent", backgroundImage: libraryVars.templatesTabStripBackgroundImage, position: "relative", overflowX: "hidden", overflowY: "hidden" },
  stripArtwork: { position: 'absolute', inset: 0 },
  stripChipStripChipHealth: { position: "absolute", zIndex: libraryVars.fillRatio, paddingTop: "var(--vv-s-2)", paddingRight: "var(--vv-s-7)", paddingBottom: "var(--vv-s-2)", paddingLeft: "var(--vv-s-7)", borderTopLeftRadius: "var(--vv-r-pill)", borderTopRightRadius: "var(--vv-r-pill)", borderBottomRightRadius: "var(--vv-r-pill)", borderBottomLeftRadius: "var(--vv-r-pill)", borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: line["--line-2"], borderRightWidth: "var(--vv-s-1)", borderRightStyle: "solid", borderRightColor: line["--line-2"], borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: line["--line-2"], borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "solid", borderLeftColor: line["--line-2"], backgroundColor: "var(--vv-glass)", backgroundImage: "none", backdropFilter: libraryVars.overlaysTabOartPlayBackdropFilter, fontSize: libraryVars.microFontSize, color: color["--color-fg-2"], whiteSpace: "nowrap", top: "var(--vv-s-8)", left: "var(--vv-s-8)" },
  stripChipStripChipHealthDataToneOk: { color: color["--color-ok"], borderTopColor: "var(--vv-ok-20)", borderRightColor: "var(--vv-ok-20)", borderBottomColor: "var(--vv-ok-20)", borderLeftColor: "var(--vv-ok-20)" },
  stripChipStripChipHealthDataToneWarn: { color: color["--color-warn"], borderTopColor: "var(--vv-warn-bd)", borderRightColor: "var(--vv-warn-bd)", borderBottomColor: "var(--vv-warn-bd)", borderLeftColor: "var(--vv-warn-bd)" },
  stripChipStripChipStyle: { position: "absolute", zIndex: libraryVars.fillRatio, paddingTop: "var(--vv-s-2)", paddingRight: "var(--vv-s-7)", paddingBottom: "var(--vv-s-2)", paddingLeft: "var(--vv-s-7)", borderTopLeftRadius: "var(--vv-r-pill)", borderTopRightRadius: "var(--vv-r-pill)", borderBottomRightRadius: "var(--vv-r-pill)", borderBottomLeftRadius: "var(--vv-r-pill)", borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: line["--line-2"], borderRightWidth: "var(--vv-s-1)", borderRightStyle: "solid", borderRightColor: line["--line-2"], borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: line["--line-2"], borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "solid", borderLeftColor: line["--line-2"], backgroundColor: "var(--vv-glass)", backgroundImage: "none", backdropFilter: libraryVars.overlaysTabOartPlayBackdropFilter, fontSize: libraryVars.microFontSize, color: color["--color-fg-2"], whiteSpace: "nowrap", right: "var(--vv-s-8)", bottom: "var(--vv-s-8)" },
  cardBody: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-8)", columnGap: "var(--vv-s-8)", paddingTop: "var(--vv-s-12)", paddingRight: "var(--vv-s-12)", paddingBottom: "var(--vv-s-12)", paddingLeft: "var(--vv-s-12)", minWidth: 0 },
  cardTop: { display: "flex", alignItems: "flex-start", rowGap: "var(--vv-s-6)", columnGap: "var(--vv-s-6)", minWidth: 0 },
  cardName: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, flexGrow: libraryVars.fillRatio, flexShrink: libraryVars.fillRatio, flexBasis: "auto", minWidth: 0, fontSize: "var(--vv-s-14)", fontWeight: libraryVars.labelWeight, overflowX: "hidden", overflowY: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" },
  vlbCardNameA: { color: color["--color-fg"], textDecoration: { default: "none", ":hover": "underline" }, display: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "inline-flex" }, alignItems: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "center" }, minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" } },
  cardEdited: { flexGrow: 0, flexShrink: 0, flexBasis: "auto", fontSize: "var(--vv-s-11)", color: color["--color-fg-4"], paddingTop: "var(--vv-s-3)" },
  chips: { display: "flex", flexWrap: "wrap", rowGap: "var(--vv-s-5)", columnGap: "var(--vv-s-5)" },
  chip: { display: "inline-flex", alignItems: "center", rowGap: "var(--vv-s-4)", columnGap: "var(--vv-s-4)", paddingTop: "var(--vv-s-2)", paddingRight: "var(--vv-s-7)", paddingBottom: "var(--vv-s-2)", paddingLeft: "var(--vv-s-7)", borderTopLeftRadius: "var(--vv-r-pill)", borderTopRightRadius: "var(--vv-r-pill)", borderBottomRightRadius: "var(--vv-r-pill)", borderBottomLeftRadius: "var(--vv-r-pill)", borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: line["--line-2"], borderRightWidth: "var(--vv-s-1)", borderRightStyle: "solid", borderRightColor: line["--line-2"], borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: line["--line-2"], borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "solid", borderLeftColor: line["--line-2"], color: color["--color-fg-3"], fontSize: "var(--vv-s-11)", whiteSpace: "nowrap" },
  cardSum: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: libraryVars.bodySmallFontSize, color: color["--color-fg-2"], lineHeight: libraryVars.bodyLineHeight, display: "-webkit-box", WebkitLineClamp: libraryVars.promptsTabPcardTextWebkitLineClamp, WebkitBoxOrient: "vertical", overflowX: "hidden", overflowY: "hidden" },
  cardMeta: { display: "flex", flexWrap: "wrap", rowGap: "var(--vv-s-8)", columnGap: "var(--vv-s-8)", fontSize: libraryVars.captionFontSize, color: color["--color-fg-4"] },
  avatars: { display: "inline-flex", alignItems: "center" },
  avatarAvatarSm: { display: "inline-flex", alignItems: "center", justifyContent: "center", flexGrow: 0, flexShrink: 0, flexBasis: "auto", width: "var(--vv-s-22)", height: "var(--vv-s-22)", borderTopLeftRadius: "50%", borderTopRightRadius: "50%", borderBottomRightRadius: "50%", borderBottomLeftRadius: "50%", backgroundColor: color["--color-surface-6"], backgroundImage: "none", color: color["--color-fg-2"], fontSize: libraryVars.badgeFontSize, fontWeight: libraryVars.headingWeight, letterSpacing: libraryVars.templatesTabAvatarAvatarSmLetterSpacing, borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: color["--color-bg"], borderRightWidth: "var(--vv-s-1)", borderRightStyle: "solid", borderRightColor: color["--color-bg"], borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: color["--color-bg"], borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "solid", borderLeftColor: color["--color-bg"] },
  cardActions: { display: "flex", alignItems: "center", rowGap: "var(--vv-s-6)", columnGap: "var(--vv-s-6)", paddingTop: "var(--vv-s-2)" },
  vlbCardActionsButton: { minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" } },
  cardEnd: { position: "relative", display: "inline-flex", alignItems: "center", rowGap: "var(--vv-s-2)", columnGap: "var(--vv-s-2)", marginLeft: "auto" },
  scroll: { flexGrow: libraryVars.fillRatio, flexShrink: libraryVars.fillRatio, flexBasis: "auto", minHeight: 0, minWidth: 0, overflowY: "auto", overscrollBehavior: "contain", paddingTop: { default: "var(--vv-s-4)", "@media (max-width: 767px)": "var(--vv-s-4)" }, paddingRight: { default: "var(--vv-s-28)", "@media (max-width: 767px)": "var(--vv-s-16)" }, paddingBottom: { default: "var(--vv-s-28)", "@media (max-width: 767px)": "var(--vv-s-24)" }, paddingLeft: { default: "var(--vv-s-28)", "@media (max-width: 767px)": "var(--vv-s-16)" }, display: "flex", flexDirection: "column", rowGap: "var(--vv-s-12)", columnGap: "var(--vv-s-12)" },
  intro: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: libraryVars.bodySmallFontSize, color: color["--color-fg-3"], lineHeight: libraryVars.helpLineHeight },
  empty: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-6)", columnGap: "var(--vv-s-6)", paddingTop: "var(--vv-s-28)", paddingRight: 0, paddingBottom: "var(--vv-s-28)", paddingLeft: 0 },
  emptyTitle: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: "var(--vv-s-14)", fontWeight: libraryVars.labelWeight, color: color["--color-fg-2"] },
  hint: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: "var(--vv-s-12)", color: color["--color-fg-4"], lineHeight: libraryVars.bodyLineHeight },
  grid: { display: "grid", rowGap: { default: "var(--vv-s-14)", "@media (max-width: 767px)": "var(--vv-s-12)" }, columnGap: { default: "var(--vv-s-14)", "@media (max-width: 767px)": "var(--vv-s-12)" }, gridTemplateColumns: { default: libraryVars.overlaysTabGridGridOverlaysGridTemplateColumns, "@media (max-width: 767px)": libraryVars.overlaysTabGridGridOverlaysGridTemplateColumns3 }, alignContent: "start" },
  field: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-6)", columnGap: "var(--vv-s-6)" },
  fielderr: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: "var(--vv-s-12)", color: color["--color-err"] },
  droptile: { minHeight: { default: "var(--vv-s-240)", "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" }, display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", rowGap: "var(--vv-s-8)", columnGap: "var(--vv-s-8)", paddingTop: "var(--vv-s-18)", paddingRight: "var(--vv-s-18)", paddingBottom: "var(--vv-s-18)", paddingLeft: "var(--vv-s-18)", borderTopWidth: "var(--vv-s-1)", borderTopStyle: "dashed", borderTopColor: { default: line["--line-3"], ":hover": color["--color-accent"] }, borderRightWidth: "var(--vv-s-1)", borderRightStyle: "dashed", borderRightColor: { default: line["--line-3"], ":hover": color["--color-accent"] }, borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "dashed", borderBottomColor: { default: line["--line-3"], ":hover": color["--color-accent"] }, borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "dashed", borderLeftColor: { default: line["--line-3"], ":hover": color["--color-accent"] }, borderTopLeftRadius: "var(--vv-r-14)", borderTopRightRadius: "var(--vv-r-14)", borderBottomRightRadius: "var(--vv-r-14)", borderBottomLeftRadius: "var(--vv-r-14)", backgroundColor: "transparent", backgroundImage: "none", color: color["--color-fg-4"], cursor: "pointer", textAlign: "center" },
  droptileOver: { borderTopColor: color["--color-accent"], borderRightColor: color["--color-accent"], borderBottomColor: color["--color-accent"], borderLeftColor: color["--color-accent"], backgroundColor: "var(--vv-accent-6)", backgroundImage: "none" },
  droptileTitle: { fontSize: "var(--vv-s-13)", fontWeight: libraryVars.labelWeight, color: color["--color-fg"] },
  droptileHint: { maxWidth: libraryVars.promptsTabIntroMaxWidth, fontSize: "var(--vv-s-12)", color: color["--color-fg-4"], lineHeight: libraryVars.helpLineHeight },
  avatarOverlap: { marginLeft: libraryVars.avatarOverlap },
  touchTarget: { minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" } },
})
