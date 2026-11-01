import * as stylex from '@stylexjs/stylex'
import { color, line } from '@noverse/ui/tokens.stylex'
import { libraryVars } from './libraryTokens.stylex'
// Cài đặt template (artboard Template): cột mục + khung xem thử/sức
// khoẻ, 4 mục. Mọi thay đổi tự lưu: ô chữ khi rời ô, điều khiển rời rạc khi
// đổi; một lần lưu đang chạy thì gộp lần sau (useTemplateDraft). Mục Nền & hiệu
// ứng do mảng Video nền dựng trong BackdropSection.tsx.
import { useState } from 'react'
import { ActionMenu, Banner, EmptyState, Spinner, Icon } from '@noverse/ui'
import { invalidate } from '@noverse/ui/data'
import { t } from '../../i18n/library'
import { navigate, paths, type Route, type TemplateSection } from '../../router'
import { useResource } from '../../data/useResource'
import { keys } from '../../data/keys'
import {
  duplicateTemplate, getTemplate, listOverlays, listOwnerChannels, listPrompts, removeTemplate,
  type Recipe, type TemplateDetail,
} from '../../api/library'
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
    return (
      <div className={`vt-shell vt-shell--center ${stylex.props(sx.shellShellCenter).className}`}>
        <EmptyState variant="frame" icon="warning-2" title={copy.notFound}
          hint={detail.error ? String(detail.error) : undefined}
          action={<ActionButton xstyle={[sx.vtShellActionButton]} variant="outline" onClick={() => navigate(paths.library('templates'))}>{copy.backToLibrary}</ActionButton>} />
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
  const unmatched = Object.entries(draft.fieldErrors).filter(([field]) => !FIELD_SLOT[field])

  const scenePrompts = useResource(keys.prompts('scene', ''), ({ signal }) => listPrompts({ kind: 'scene' }, signal))
  const coverPrompts = useResource(keys.prompts('cover', ''), ({ signal }) => listPrompts({ kind: 'cover' }, signal))
  const channels = useResource(keys.ownerChannels(), ({ signal }) => listOwnerChannels(signal))
  const overlays = useResource(keys.overlays(), ({ signal }) => listOverlays(signal))

  const [name, setName] = useState(initial.name)
  const [nameFocused, setNameFocused] = useState(false)
  const [attach, setAttach] = useState(false)
  const [deleteError, setDeleteError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  const commitName = () => {
    setNameFocused(false)
    if (name.trim() === detail.name) { setName(detail.name); return }
    draft.saveName(name)
  }

  const save = (patch: Partial<Recipe>) => { draft.save(patch) }

  const duplicate = async () => {
    setBusy(true)
    try {
      const copyOf = await duplicateTemplate(detail.id)
      await invalidate(keys.templates())
      navigate(paths.template(copyOf.id, section))
    } finally { setBusy(false) }
  }

  const remove = async () => {
    setDeleteError(null)
    try {
      await removeTemplate(detail.id)
      await invalidate(keys.templates())
      navigate(paths.library('templates'))
    } catch (err) {
      setDeleteError(err instanceof Error ? err.message : String(err))
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
              aria-describedby={nameError ? 'vt-name-error' : undefined}
              onChange={(event) => setName(event.currentTarget.value)}
              onFocus={() => setNameFocused(true)}
              onBlur={commitName}
              onKeyDown={(event) => {
                if (event.key === 'Enter') event.currentTarget.blur()
                if (event.key === 'Escape') { setName(detail.name); event.currentTarget.blur() }
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
            <span className={`vt-saved ${stylex.props(sx.saved, ((draft.saving ? 'saving' : draft.conflict ? 'conflict' : 'saved') === "saving") && sx.savedDataStateSaving, ((draft.saving ? 'saving' : draft.conflict ? 'conflict' : 'saved') === "conflict") && sx.savedDataStateConflict).className}`} data-state={draft.saving ? 'saving' : draft.conflict ? 'conflict' : 'saved'} aria-live="polite">
              {draft.saving ? copy.saving : t.library.saved(clock(detail.savedAt))}
            </span>
            <ActionButton xstyle={[sx.vtShellActionButton]} variant="outline" lockedReason={copy.openEditorLocked}>{copy.openEditor}</ActionButton>
            <ActionMenu triggerStyle={sx.touchTarget}
              label={copy.moreAria}
              items={[
                { id: 'duplicate', label: t.library.duplicate, icon: <Icon name="copy" size={16} />, onSelect: () => void duplicate() },
              ]}
              trigger={<Icon name="more" size={16} />}
            />
          </div>
        </header>

        {draft.conflict && (
          <Banner tone="warning" title={copy.conflict}
            action={{ label: copy.conflictReload, onClick: () => { void draft.reload().then((fresh) => setName(fresh.name)) } }} />
        )}
        {draft.error && <Banner tone="error" title={draft.error} />}
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
          <div className={`vt-settings ${stylex.props(sx.settings).className}`}>
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
                onAttach={() => setAttach(true)} />
            )}
          </div>
          <aside className={`vt-aside ${stylex.props(sx.aside).className}`}>
            <PreviewPane detail={detail} dirty={draft.dirty} />
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
  shell: { display: "flex", flexGrow: libraryVars.fillRatio, flexShrink: libraryVars.fillRatio, flexBasis: "auto", minHeight: 0, minWidth: 0, color: color["--color-fg"] },
  main: { display: "flex", flexGrow: libraryVars.fillRatio, flexShrink: libraryVars.fillRatio, flexBasis: "auto", flexDirection: "column", minWidth: 0, minHeight: 0, overflowY: "auto" },
  head: { display: "flex", alignItems: { default: "flex-start", "@media (max-width: 767px)": "stretch" }, justifyContent: "space-between", rowGap: { default: "var(--vv-s-16)", "@media (max-width: 767px)": "var(--vv-s-8)" }, columnGap: { default: "var(--vv-s-16)", "@media (max-width: 767px)": "var(--vv-s-8)" }, paddingTop: { default: "var(--vv-s-22)", "@media (max-width: 767px)": "var(--vv-s-16)" }, paddingRight: { default: "var(--vv-s-28)", "@media (max-width: 767px)": "var(--vv-s-16)" }, paddingBottom: { default: "var(--vv-s-12)", "@media (max-width: 767px)": "var(--vv-s-10)" }, paddingLeft: { default: "var(--vv-s-28)", "@media (max-width: 767px)": "var(--vv-s-16)" }, flexDirection: { default: null, "@media (max-width: 767px)": "column" } },
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
  savedDataStateConflict: { color: color["--color-warn"] },
  fieldLine: { display: "block" },
  row2: { display: "grid", gridTemplateColumns: { default: libraryVars.templateScreenRow2GridTemplateColumns, "@media (max-width: 1099px)": libraryVars.overlaysTabGridGridOverlaysGridTemplateColumns3 }, rowGap: { default: "var(--vv-s-24)", "@media (max-width: 767px)": "var(--vv-s-14)" }, columnGap: { default: "var(--vv-s-24)", "@media (max-width: 767px)": "var(--vv-s-14)" }, paddingTop: { default: 0, "@media (max-width: 767px)": 0 }, paddingRight: { default: "var(--vv-s-28)", "@media (max-width: 767px)": "var(--vv-s-16)" }, paddingBottom: { default: "var(--vv-s-40)", "@media (max-width: 767px)": "var(--vv-s-32)" }, paddingLeft: { default: "var(--vv-s-28)", "@media (max-width: 767px)": "var(--vv-s-16)" }, alignItems: "start" },
  settings: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-16)", columnGap: "var(--vv-s-16)", minWidth: 0 },
  aside: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-14)", columnGap: "var(--vv-s-14)", minWidth: 0, position: { default: "sticky", "@media (max-width: 1099px)": "static" }, top: "var(--vv-s-12)", order: { default: null, "@media (max-width: 1099px)": libraryVars.templateScreenAsideOrder } },
  touchTarget: { minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" } },
})
