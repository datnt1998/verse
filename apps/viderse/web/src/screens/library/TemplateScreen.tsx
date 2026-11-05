import * as stylex from '@stylexjs/stylex'
import { color, line } from '@noverse/ui/tokens.stylex'
import { libraryVars } from './libraryTokens.stylex'
// Cài đặt template (artboard Template): cột mục + khung xem thử/sức
// khoẻ, 4 mục. Mọi thay đổi tự lưu: ô chữ khi rời ô, điều khiển rời rạc khi
// đổi; một lần lưu đang chạy thì gộp lần sau (useTemplateDraft). Mục Nền & hiệu
// ứng do mảng Video nền dựng trong BackdropSection.tsx.
import { useEffect, useRef, useState } from 'react'
import { ActionMenu, Banner, EmptyState, Spinner, Icon, confirm, toast } from '@noverse/ui'
import { invalidate } from '@noverse/ui/data'
import { t } from '../../i18n/library'
import { navigate, paths, type Route, type TemplateSection } from '../../router'
import { useResource } from '../../data/useResource'
import { keys } from '../../data/keys'
import {
  downloadTemplate, duplicateTemplate, getTemplate, listOverlays, listOwnerChannels, listPrompts, removeTemplate,
  type Recipe, type TemplateDetail,
} from '../../api/library'
import { ApiError } from '../../api/request'
import { ActionButton } from '../../components'
import { BackdropSection } from './template/BackdropSection'
import { CoverSection } from './template/CoverSection'
import { ImagesSection } from './template/ImagesSection'
import { OutputSection } from './template/OutputSection'
import { PreviewPane } from './template/PreviewPane'
import { TemplateNav } from './template/TemplateNav'
import { useTemplateDraft } from './template/useTemplateDraft'
import { AttachChannelDialog } from './AttachChannelDialog'
import { clock, editedLabel, num } from './fmt'
import { MTemplateScreen } from './mobile/MTemplateScreen'
import { useMediaQuery } from '../../lib/useMediaQuery'


/** Cảnh báo sức khoẻ dùng chung cho chip trên đầu trang và thẻ ở cột trái. */
export function healthWarn(detail: TemplateDetail) {
  const backdrop = detail.recipe.backdrop
  return !!backdrop && backdrop.fixedVideoIds.length > 0 && backdrop.channels.length === 0
}

export function tplHealthChecks(detail: TemplateDetail) {
  const copy = t.library.template.health
  const backdrop = detail.recipe.backdrop
  const linked = backdrop
    ? t.library.templates.backdrop(backdrop.channels.length, backdrop.fixedVideoIds.length)
    : ''
  return [
    linked || copy.noBackdrop,
    t.library.templates.imagesFixed(num(detail.images.perVideo ?? detail.images.count)),
    copy.subtitles,
    copy.loudness,
  ]
}

export function TemplateScreen({ route }: { route: Extract<Route, { name: 'template' }> }) {
  const mobile = useMediaQuery('(max-width: 767px)')
  if (mobile) return <MTemplateScreen route={route} />
  return <DesktopTemplateScreen route={route} />
}

function DesktopTemplateScreen({ route }: { route: Extract<Route, { name: 'template' }> }) {
  const copy = t.library.template
  const detail = useResource(keys.template(route.templateId), ({ signal }) => getTemplate(route.templateId, signal))
  const section: TemplateSection = route.section ?? 'images'

  if (detail.initialLoading) {
    return (
      <div className={`vt-shell vt-shell--center ${stylex.props(sx.shellShellCenter).className}`} role="status" aria-label={copy.loadFail}>
        <Spinner />
      </div>
    )
  }
  if (!detail.data) {
    const notFound = detail.error instanceof ApiError && detail.error.status === 404
    return (
      <div className={`vt-shell vt-shell--center ${stylex.props(sx.shellShellCenter).className}`}>
        <EmptyState variant="frame" icon="warning-2" title={notFound ? copy.notFound : copy.loadFail}
          action={notFound
            ? <ActionButton xstyle={[sx.vtShellActionButton]} variant="outline" onClick={() => navigate(paths.library('templates'))}>{copy.backToLibrary}</ActionButton>
            : <ActionButton xstyle={[sx.vtShellActionButton]} variant="outline" onClick={() => void detail.refresh()}>{t.library.retry}</ActionButton>} />
      </div>
    )
  }
  return <TemplateEditor key={route.templateId} initial={detail.data} section={section} />
}

/** Trường đã có chỗ hiện lỗi riêng — còn lại gom vào băng lỗi đầu trang. */
const FIELD_SLOT: Record<string, true> = {
  name: true,
  'scenes.style': true,
  'scenes.framing': true,
  'scenes.scenePromptId': true,
  'scenes.images.mode': true,
  'scenes.images.count': true,
  'scenes.images.minutes': true,
  'scenes.motion': true,
  'scenes.sceneOpacity': true,
  'scenes.coverPromptId': true,
  'scenes.coverMediaId': true,
}

function TemplateEditor({ initial, section }: { initial: TemplateDetail; section: TemplateSection }) {
  const copy = t.library.template
  const draft = useTemplateDraft(initial)
  const detail = draft.detail
  const recipe = detail.recipe
  const nameError = draft.fieldErrors.name
  const editorLockedReason = draft.reloading ? 'Đang tải lại…' : draft.saving ? copy.saving : draft.dirty ? draft.conflict ? t.library.conflict : copy.saveFail : undefined
  const unmatched = Object.entries(draft.fieldErrors).filter(([field]) => !FIELD_SLOT[field])
  const saveState = draft.saving || draft.reloading ? 'saving' : draft.conflict ? 'conflict' : draft.dirty || draft.error ? 'error' : 'saved'
  const saveText = draft.reloading ? 'Đang tải lại…' : draft.saving ? copy.saving : draft.conflict ? t.library.conflict : draft.dirty || draft.error ? copy.saveFail : draft.justSaved ? copy.savedNow : copy.savedAt(clock(detail.savedAt))

  const scenePrompts = useResource(keys.prompts('scene', ''), ({ signal }) => listPrompts({ kind: 'scene' }, signal))
  const coverPrompts = useResource(keys.prompts('cover', ''), ({ signal }) => listPrompts({ kind: 'cover' }, signal))
  const channels = useResource(keys.ownerChannels(), ({ signal }) => listOwnerChannels(signal))
  const overlays = useResource(keys.overlays(), ({ signal }) => listOverlays(signal))
  const savedPromptIds = useRef([initial.recipe.scenes.scenePromptId, initial.recipe.scenes.coverPromptId])
  useEffect(() => {
    if (!draft.justSaved || draft.saving || draft.dirty || draft.error || draft.conflict) return
    const ids = [recipe.scenes.scenePromptId, recipe.scenes.coverPromptId]
    const lists = [scenePrompts.data?.items, coverPrompts.data?.items]
    for (let index = 0; index < ids.length; index++) {
      if (ids[index] === savedPromptIds.current[index]) continue
      const prompt = lists[index]?.find(item => item.id === ids[index])
      if (prompt) toast(`Template dùng prompt “${prompt.name}” · video mới vẽ theo prompt này`, 'ok')
    }
    savedPromptIds.current = ids
  }, [draft.justSaved, draft.saving, draft.dirty, draft.error, draft.conflict, recipe.scenes.scenePromptId, recipe.scenes.coverPromptId, scenePrompts.data, coverPrompts.data])

  const [name, setName] = useState(initial.name)
  const [nameFocused, setNameFocused] = useState(false)
  const cancelNameCommit = useRef(false)
  const [attach, setAttach] = useState(false)
  const [deleteError, setDeleteError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (!nameFocused && !draft.dirty && !draft.saving && !draft.reloading) setName(detail.name)
  }, [nameFocused, draft.dirty, draft.saving, draft.reloading, detail.name])

  const commitName = async () => {
    setNameFocused(false)
    if (cancelNameCommit.current) { cancelNameCommit.current = false; return }
    if (name.trim() === detail.name && !nameError) { setName(detail.name); return }
    await draft.saveName(name)
  }

  const save = (patch: Partial<Recipe>) => { draft.save(patch) }

  const duplicate = async () => {
    setBusy(true)
    try {
      const copyOf = await duplicateTemplate(detail.id)
      await invalidate(keys.templates())
      toast(t.library.templates.duplicated, 'ok')
      navigate(paths.template(copyOf.id, section))
    } catch (err) {
      toast(err instanceof Error ? err.message : String(err), 'err')
    } finally { setBusy(false) }
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
    setDeleteError(null)
    try {
      await removeTemplate(detail.id)
      toast(t.library.templates.deleted, 'ok')
      await invalidate(keys.templates())
      navigate(paths.library('templates'))
    } catch (err) {
      setDeleteError(err instanceof Error ? err.message : String(err))
      throw err
    }
  }

  return (
    <div className={`vt-shell ${stylex.props(sx.shell).className}`}>
      <TemplateNav detail={detail} active={section}
        overlayNames={(overlays.data?.items ?? []).filter((item) => recipe.overlays.includes(item.id)).map((item) => item.name)} />
      <main className={`vt-main vv-wide vv-wide--read ${stylex.props(sx.main).className}`}>
        <header className={`vt-head ${stylex.props(sx.head).className}`}>
          <div className={`vt-head-main ${stylex.props(sx.headMain).className}`}>
            <div className={`vt-title-row ${stylex.props(sx.titleRow).className}`}>
            <input
              className={`vt-name ${stylex.props(sx.name, (Boolean((nameError ? true : undefined))) && sx.nameAriaInvalid).className}`}
              value={name}
              aria-label={copy.nameLabel}
              aria-invalid={nameError ? true : undefined}
              disabled={draft.reloading}
              aria-describedby={nameError ? 'vt-name-error' : undefined}
              onChange={(event) => setName(event.currentTarget.value)}
              onFocus={() => setNameFocused(true)}
              onBlur={() => void commitName()}
              onKeyDown={(event) => {
                if (event.key === 'Enter') event.currentTarget.blur()
                if (event.key === 'Escape') { cancelNameCommit.current = true; setName(detail.name); event.currentTarget.blur() }
              }}
            />
            <div className={`vt-chips ${stylex.props(sx.chips).className}`}>
              <span className={`vt-chip ${stylex.props(sx.chip, ((healthWarn(detail) ? 'warn' : 'ok') === "ok") && sx.chipDataToneOk, ((healthWarn(detail) ? 'warn' : 'ok') === "warn") && sx.chipDataToneWarn).className}`} data-tone={healthWarn(detail) ? 'warn' : 'ok'}>
                <span className={`vt-dot ${stylex.props(sx.dot, ((healthWarn(detail) ? 'warn' : 'ok') === "warn") && sx.dotDataToneWarn).className}`} data-tone={healthWarn(detail) ? 'warn' : 'ok'} aria-hidden="true" />
                {healthWarn(detail) ? copy.healthWarnChip : copy.healthOkChip}
              </span>
              <span className={`vt-chip ${stylex.props(sx.chip).className}`}>{detail.channels.length ? copy.channelsChip(detail.channels.length) : copy.noChannelsChip}</span>
            </div>
            </div>
            {nameError && <p id="vt-name-error" className={`vt-fielderr ${stylex.props(sx.fielderr).className}`} role="alert">{nameError}</p>}
          </div>
          <div className={`vt-head-actions ${stylex.props(sx.headActions).className}`} aria-live="polite">
            <span className={`vt-saved ${stylex.props(sx.saved, saveState === 'saving' && sx.savedDataStateSaving, saveState === 'error' && sx.savedDataStateError, saveState === 'conflict' && sx.savedDataStateConflict).className}`} data-state={saveState} aria-live="polite">
              {saveText}
            </span>
            <a {...stylex.props(sx.editorLink, sx.vtShellActionButton)} href={editorLockedReason ? undefined : `#${paths.editor(detail.id)}`} role="link" aria-disabled={!!editorLockedReason} title={editorLockedReason} onClick={event => { if (editorLockedReason) { event.preventDefault(); toast(editorLockedReason, 'err') } }}>{copy.openEditor}</a>
            <ActionMenu triggerStyle={sx.touchTarget}
              label={copy.moreAria}
              items={[
                { id: 'export', label: t.library.template.output.exportFile, icon: <Icon name="export" size={16} />, onSelect: () => void exportFile() },
                { id: 'duplicate', label: t.library.duplicate, icon: <Icon name="copy" size={16} />, onSelect: () => void duplicate() },
                { id: 'library', label: copy.backToLibrary, href: `#${paths.library('templates')}` },
                { id: 'delete', label: t.library.delete, icon: <Icon name="trash" size={16} />, danger: true, lockedReason: detail.channels.length ? copy.output.deleteReason(detail.channels.map((channel) => channel.name)) : undefined, onSelect: () => void confirm({ title: copy.deleteTitle(detail.name), body: copy.deleteBody, tone: 'danger', confirmLabel: copy.deleteAction, cancelLabel: t.library.cancel, onConfirm: remove }) },
              ]}
              trigger={<Icon name="more" size={16} />}
            />
          </div>
        </header>

        {draft.conflict && (
          <Banner tone="warning" title={t.library.conflict}
            action={{ label: copy.conflictReload, onClick: () => { void draft.reload().then((fresh) => { if (fresh) setName(fresh.name) }) } }}>
            Bản đang sửa vẫn được giữ. Tải lại sẽ thay bằng bản mới trên máy chủ.
          </Banner>
        )}
        {draft.error && <Banner tone="error" title={draft.error}
          action={draft.conflict ? undefined : { label: t.library.retry, onClick: draft.retry }} />}
        {deleteError && <Banner tone="error" title={copy.deleteBlockedTitle}>{deleteError}</Banner>}
        {unmatched.length > 0 && (
          <Banner tone="error" title={copy.saveFail}>
            {unmatched.map(([field, message]) => (
              <span key={field} className={`vt-field-line ${stylex.props(sx.fieldLine).className}`}>
                {copy.fieldError(copy.fieldLabels[field] ?? field, message)}
              </span>
            ))}
          </Banner>
        )}

        <div className={`vt-row2 ${stylex.props(sx.row2).className}`}>
          <fieldset disabled={draft.reloading} className={`vt-settings ${stylex.props(sx.settings).className}`}>
            {section === 'images' && (
              <ImagesSection recipe={recipe} price={detail.price} prompts={scenePrompts.data?.items ?? []}
                errors={draft.fieldErrors} onSave={save} />
            )}
            {section === 'cover' && (
              <CoverSection recipe={recipe} prompts={coverPrompts.data?.items ?? []}
                errors={draft.fieldErrors} onSave={save} />
            )}
            {section === 'backdrop' && (
              <BackdropSection template={detail} saving={draft.saving} onSave={async (patch) => { save(patch) }} />
            )}
            {section === 'output' && (
              <OutputSection detail={detail} onSave={save} onDuplicate={duplicate} onDelete={remove}
                onAttach={() => setAttach(true)} editorLockedReason={editorLockedReason} />
            )}
          </fieldset>
          <aside className={`vt-aside ${stylex.props(sx.aside).className}`}>
            <PreviewPane detail={detail} dirty={draft.dirty} editLockedReason={editorLockedReason} />
          </aside>
        </div>
      </main>
      {attach && <AttachChannelDialog inLibrary={false} channels={channels.data?.channels ?? []} onClose={() => setAttach(false)} />}
    </div>
  )
}

const sx = stylex.create({
  shellShellCenter: { display: "flex", flexGrow: libraryVars.fillRatio, flexShrink: libraryVars.fillRatio, flexBasis: "auto", minHeight: 0, minWidth: 0, color: color["--color-fg"], alignItems: "center", justifyContent: "center", paddingTop: "var(--vv-s-24)", paddingRight: "var(--vv-s-24)", paddingBottom: "var(--vv-s-24)", paddingLeft: "var(--vv-s-24)" },
  vtShellActionButton: { minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" } },
  editorLink: {
    display: 'inline-flex', alignItems: 'center', minHeight: 'var(--vv-s-32)', paddingInline: 'var(--vv-s-12)', borderRadius: 'var(--vv-r-8)',
    borderWidth: 'var(--vv-s-1)', borderStyle: 'solid', borderColor: line['--line-2'], color: color['--color-fg-2'], fontSize: 'var(--vv-s-13)', textDecoration: 'none',
    ':hover': { backgroundColor: color['--color-surface-5'], color: color['--color-fg'] },
    ':focus-visible': { outlineWidth: 'var(--vv-s-2)', outlineStyle: 'solid', outlineColor: color['--color-fg'] },
  },
  shell: { display: "flex", flexGrow: libraryVars.fillRatio, flexShrink: libraryVars.fillRatio, flexBasis: "auto", minHeight: 0, minWidth: 0, color: color["--color-fg"] },
  main: { display: "flex", flexGrow: libraryVars.fillRatio, flexShrink: libraryVars.fillRatio, flexBasis: "auto", flexDirection: "column", minWidth: 0, minHeight: 0, overflowY: "auto" },
  head: { display: "flex", alignItems: { default: "center", "@media (max-width: 767px)": "stretch" }, justifyContent: "space-between", rowGap: { default: "var(--vv-s-8)", "@media (max-width: 767px)": "var(--vv-s-8)" }, columnGap: { default: "var(--vv-s-12)", "@media (max-width: 767px)": "var(--vv-s-8)" }, paddingTop: { default: "var(--vv-s-14)", "@media (max-width: 767px)": "var(--vv-s-16)" }, paddingRight: { default: "var(--vv-s-24)", "@media (max-width: 767px)": "var(--vv-s-16)" }, paddingBottom: { default: "var(--vv-s-12)", "@media (max-width: 767px)": "var(--vv-s-10)" }, paddingLeft: { default: "var(--vv-s-24)", "@media (max-width: 767px)": "var(--vv-s-16)" }, flexDirection: { default: null, "@media (max-width: 767px)": "column" }, borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: line["--line-2"] },
  headMain: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-3)", columnGap: "var(--vv-s-3)", minWidth: 0 },
  titleRow: { display: { default: "flex", "@media (max-width: 767px)": "flex" }, alignItems: { default: "center", "@media (max-width: 767px)": "center" }, rowGap: { default: "var(--vv-s-10)", "@media (max-width: 767px)": "var(--vv-s-10)" }, columnGap: { default: "var(--vv-s-10)", "@media (max-width: 767px)": "var(--vv-s-10)" }, flexWrap: { default: "wrap", "@media (max-width: 767px)": "wrap" } },
  name: { minWidth: 0, fieldSizing: "content", maxWidth: { default: libraryVars.templateScreenNameMaxWidth, "@media (max-width: 767px)": "none" }, paddingTop: "var(--vv-s-2)", paddingRight: "var(--vv-s-6)", paddingBottom: "var(--vv-s-2)", paddingLeft: "var(--vv-s-6)", marginLeft: libraryVars.templateScreenNameMarginLeft, borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: { default: "transparent", ":hover": line["--line-2"], ":focus": "var(--vv-accent-45)" }, borderRightWidth: "var(--vv-s-1)", borderRightStyle: "solid", borderRightColor: { default: "transparent", ":hover": line["--line-2"], ":focus": "var(--vv-accent-45)" }, borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: { default: "transparent", ":hover": line["--line-2"], ":focus": "var(--vv-accent-45)" }, borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "solid", borderLeftColor: { default: "transparent", ":hover": line["--line-2"], ":focus": "var(--vv-accent-45)" }, borderTopLeftRadius: "var(--vv-r-7)", borderTopRightRadius: "var(--vv-r-7)", borderBottomRightRadius: "var(--vv-r-7)", borderBottomLeftRadius: "var(--vv-r-7)", backgroundColor: { default: "transparent", ":focus": color["--color-surface-0"] }, backgroundImage: { default: "none", ":focus": "none" }, color: color["--color-fg"], fontFamily: "inherit", fontSize: { default: libraryVars.templateScreenNameFontSize, "@media (max-width: 767px)": "var(--vv-s-18)" }, fontWeight: libraryVars.headingWeight, letterSpacing: libraryVars.libraryScreenVlbHeadMainH1LetterSpacing, minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" }, outlineWidth: { default: null, ":focus": "medium" }, outlineStyle: { default: null, ":focus": "none" }, outlineColor: { default: null, ":focus": "currentColor" } },
  nameAriaInvalid: { borderTopColor: color["--color-err"], borderRightColor: color["--color-err"], borderBottomColor: color["--color-err"], borderLeftColor: color["--color-err"] },
  chips: { display: "flex", flexWrap: "wrap", rowGap: "var(--vv-s-5)", columnGap: "var(--vv-s-5)" },
  chip: { paddingTop: "var(--vv-s-2)", paddingRight: "var(--vv-s-7)", paddingBottom: "var(--vv-s-2)", paddingLeft: "var(--vv-s-7)", borderTopLeftRadius: "var(--vv-r-pill)", borderTopRightRadius: "var(--vv-r-pill)", borderBottomRightRadius: "var(--vv-r-pill)", borderBottomLeftRadius: "var(--vv-r-pill)", borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: line["--line-2"], borderRightWidth: "var(--vv-s-1)", borderRightStyle: "solid", borderRightColor: line["--line-2"], borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: line["--line-2"], borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "solid", borderLeftColor: line["--line-2"], fontSize: libraryVars.captionFontSize, color: color["--color-fg-3"], whiteSpace: "nowrap" },
  chipDataToneOk: { color: color["--color-ok"], borderTopColor: "var(--vv-ok-20)", borderRightColor: "var(--vv-ok-20)", borderBottomColor: "var(--vv-ok-20)", borderLeftColor: "var(--vv-ok-20)" },
  chipDataToneWarn: { color: color["--color-warn"], borderTopColor: "var(--vv-warn-bd)", borderRightColor: "var(--vv-warn-bd)", borderBottomColor: "var(--vv-warn-bd)", borderLeftColor: "var(--vv-warn-bd)" },
  dot: { width: "var(--vv-s-6)", height: "var(--vv-s-6)", borderTopLeftRadius: "50%", borderTopRightRadius: "50%", borderBottomRightRadius: "50%", borderBottomLeftRadius: "50%", backgroundColor: color["--color-ok"], backgroundImage: "none" },
  dotDataToneWarn: { backgroundColor: color["--color-warn"], backgroundImage: "none" },
  fielderr: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: "var(--vv-s-12)", color: color["--color-err"] },
  headActions: { display: "flex", alignItems: "center", rowGap: "var(--vv-s-10)", columnGap: "var(--vv-s-10)", flexGrow: 0, flexShrink: 0, flexBasis: "auto", flexWrap: "wrap", justifyContent: { default: "flex-end", "@media (max-width: 767px)": "space-between" } },
  saved: { fontSize: "var(--vv-s-12)", color: color["--color-fg-4"], fontVariantNumeric: "tabular-nums", whiteSpace: "nowrap", minWidth: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" } },
  savedDataStateSaving: { color: color["--color-fg-2"] },
  savedDataStateError: { color: color["--color-err"] },
  savedDataStateConflict: { color: color["--color-warn"] },
  fieldLine: { display: "block" },
  row2: { display: "grid", gridTemplateColumns: { default: libraryVars.templateScreenRow2GridTemplateColumns, "@media (max-width: 1099px)": libraryVars.overlaysTabGridGridOverlaysGridTemplateColumns3 }, gap: 0, padding: 0, alignItems: "start", flexGrow: 1 },
  settings: { display: "flex", flexDirection: "column", gap: "var(--vv-s-16)", minWidth: 0, width: "100%", maxWidth: libraryVars.templateSettingsMaxWidth, justifySelf: "center", margin: 0, padding: "var(--vv-s-20) var(--vv-s-28) var(--vv-s-32)", borderWidth: 0, boxSizing: "border-box" },
  aside: { display: "flex", flexDirection: "column", gap: "var(--vv-s-14)", minWidth: 0, padding: "var(--vv-s-18)", borderLeftWidth: { default: "var(--vv-s-1)", "@media (max-width: 1099px)": 0 }, borderLeftStyle: "solid", borderLeftColor: line["--line-2"], position: { default: "sticky", "@media (max-width: 1099px)": "static" }, top: 0, order: { default: null, "@media (max-width: 1099px)": libraryVars.templateScreenAsideOrder } },
  touchTarget: { minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" } },
})
