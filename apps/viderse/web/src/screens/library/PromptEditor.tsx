import * as stylex from '@stylexjs/stylex'
import { color, line } from '@noverse/ui/tokens.stylex'
import { libraryVars } from './libraryTokens.stylex'
// Khung sửa prompt ảnh (Library.dc.html › view=editor): tên 1–60, loại
// ảnh cảnh/ảnh bìa, bố cục bìa, nội dung ≤ 2.000 ký tự, luật áp mọi prompt,
// template đang dùng. Prompt có sẵn chỉ nhân bản được; prompt đang được dùng
// thì không xoá và không đổi loại (máy chủ trả 409 in_use kèm tên template).
import { useState } from 'react'
import { Banner, Button, Dialog, FieldError, FieldLabel, RadioCard, TextArea, TextField, Icon } from '@noverse/ui'
import { Segmented } from '../../components/Segmented'
import { ActionButton } from '../../components'
import { invalidate } from '@noverse/ui/data'
import { t } from '../../i18n/library'
import {
  createPrompt, fieldErrors, libraryErrorInfo, patchPrompt, removePrompt,
  type ImagePrompt, type PromptKind, type ThumbnailStyle,
} from '../../api/library'
import { keys } from '../../data/keys'
import { clock } from './fmt'
import { LearnedRulesList } from '../../components/learn/LearnedRulesList'

const COMPOSITIONS: ThumbnailStyle[] = ['clean', 'hook', 'circle-arrow', 'hook-arrow', 'split', 'closeup']

export function PromptEditor({ prompt, onClose, onSaved, onDeleted, onDuplicate }: {
  /** null = prompt mới */
  prompt: ImagePrompt | null
  onClose: () => void
  onSaved: (saved: ImagePrompt) => void
  onDeleted: () => void
  /** nhân bản prompt đang mở thành một prompt của bạn (prompt mới không có) */
  onDuplicate?: () => void
}) {
  const copy = t.library.prompts.editor
  const builtin = prompt?.builtIn ?? false
  const [name, setName] = useState(prompt?.name ?? '')
  const [kind, setKind] = useState<PromptKind>(prompt?.kind ?? 'scene')
  const [composition, setComposition] = useState<ThumbnailStyle>(prompt?.composition ?? 'clean')
  const [text, setText] = useState(prompt?.prompt ?? '')
  const [saving, setSaving] = useState(false)
  const [confirming, setConfirming] = useState(false)
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [error, setError] = useState<string | null>(null)
  const [savedAt, setSavedAt] = useState<string | null>(prompt?.updatedAt ?? null)
  const dirty = prompt === null
    || name !== prompt.name
    || kind !== prompt.kind
    || (kind === 'cover' && composition !== (prompt.composition ?? 'clean'))
    || text !== prompt.prompt
  const kindLocked = builtin || (prompt?.usedBy.length ?? 0) > 0

  const save = async (): Promise<void> => {
    setError(null)
    setErrors({})
    setSaving(true)
    try {
      let saved: ImagePrompt
      if (builtin && prompt) {
        saved = await createPrompt({
          ...blankDraft(t.library.prompts.copyName(prompt.name), kind, composition, text),
          fromId: prompt.id,
        })
        setName(saved.name)
      } else if (prompt === null) {
        saved = await createPrompt(blankDraft(name, kind, composition, text))
      } else {
        saved = await patchPrompt(prompt.id, blankDraft(name, kind, composition, text))
      }
      setSavedAt(saved.updatedAt)
      await Promise.all([invalidate(keys.prompts('', '')), invalidate(keys.prompts('scene', '')), invalidate(keys.prompts('cover', ''))])
      onSaved(saved)
    } catch (err) {
      const info = libraryErrorInfo(err)
      const fields = fieldErrors(err)
      if (info.code === 'in_use') {
        const names = prompt?.usedBy.map((ref) => ref.name) ?? []
        setErrors({ kind: names.length ? copy.inUseKind(names) : err instanceof Error ? err.message : String(err) })
      } else if (Object.keys(fields).length) {
        setErrors(fields)
      } else {
        setError(info.code === 'builtin' ? copy.builtReadonly : err instanceof Error ? err.message : String(err))
      }
    } finally {
      setSaving(false)
    }
  }

  const remove = async () => {
    if (!prompt) return
    setConfirming(false)
    setError(null)
    try {
      await removePrompt(prompt.id)
      await Promise.all([invalidate(keys.prompts('', '')), invalidate(keys.prompts('scene', '')), invalidate(keys.prompts('cover', ''))])
      onDeleted()
    } catch (err) {
      const info = libraryErrorInfo(err)
      const names = prompt.usedBy.map((ref) => ref.name)
      setError(info.code === 'in_use' || info.code === 'builtin'
        ? `${err instanceof Error ? err.message : String(err)}${names.length ? ` · ${copy.deleteLocked(names)}` : ''}`
        : err instanceof Error ? err.message : String(err))
    }
  }

  const heading = prompt === null ? copy.newTitle : prompt.name
  const badge = prompt === null
    ? copy.badgeNew
    : prompt.builtIn ? copy.badgeBuilt(kindName(kind)) : copy.badgeMine(kindName(kind))
  const status = dirty
    ? (prompt === null ? copy.statusNew : copy.statusDirty)
    : savedAt ? copy.statusSaved(clock(savedAt)) : copy.statusNew
  const primaryLabel = builtin ? copy.duplicateToEdit : prompt === null ? copy.saveNew : copy.save

  return (
    <div className={`vlb-editor ${stylex.props(sx.editor).className}`}>
      <div className={`vlb-scroll ${stylex.props(sx.scroll).className}`}>
        <button type="button" className={`vlb-back ${stylex.props(sx.back).className}`} onClick={onClose} aria-label={copy.backAria}>
          <span aria-hidden="true">←</span>{copy.back}
        </button>
        <header className={`vlb-ed-head ${stylex.props(sx.edHead).className}`}>
          <h2 {...stylex.props(sx.vlbEdHeadH2)}>{heading}</h2>
          <span className={`vlb-chip ${stylex.props(sx.chip).className}`}>{badge}</span>
        </header>
        {builtin && <Banner tone="info" title={copy.builtNote} />}
        {error && <Banner tone="error" title={error} />}

        <div className={`vlb-ed-grid ${stylex.props(sx.edGrid).className}`}>
          <div className={`vlb-ed-main ${stylex.props(sx.edMain).className}`}>
            <label className={`vlb-field ${stylex.props(sx.field).className}`}>
              <FieldLabel htmlFor="vlb-prompt-name">{copy.nameLabel}</FieldLabel>
              <TextField id="vlb-prompt-name" value={name} maxLength={60} readOnly={builtin}
                aria-invalid={errors.name ? true : undefined}
                onChange={setName} />
              {errors.name && <FieldError>{errors.name}</FieldError>}
            </label>

            <div className={`vlb-field ${stylex.props(sx.field).className}`}>
              <FieldLabel>{copy.kindLabel}</FieldLabel>
              <Segmented buttonStyle={sx.touchTarget}
                label={copy.kindLabel}
                value={kind}
               
                onChange={(next) => { if (!kindLocked) setKind(next as PromptKind) }}
                options={[
                  { id: 'scene', label: copy.kindScene, disabled: kindLocked },
                  { id: 'cover', label: copy.kindCover, disabled: kindLocked },
                ]}
              />
              <p className={`vlb-hint ${stylex.props(sx.hint).className}`}>{kindLocked ? copy.kindLockedHelp : kind === 'scene' ? copy.kindSceneHelp : copy.kindCoverHelp}</p>
              {errors.kind && <FieldError>{errors.kind}</FieldError>}
            </div>

            {kind === 'cover' && (
              <div className={`vlb-field ${stylex.props(sx.field).className}`}>
                <FieldLabel>{copy.compositionTitle}</FieldLabel>
                <div role="radiogroup" aria-label={copy.compositionTitle} className={`vlb-comps ${stylex.props(sx.comps).className}`}>
                  {COMPOSITIONS.map((id) => (
                    <RadioCard xstyle={[sx.vlbCompsRadioCard]} key={id} checked={composition === id} disabled={builtin}
                      onChange={() => setComposition(id)}
                      label={<span className={`vlb-comp ${stylex.props(sx.comp).className}`} data-comp={id}><span className={`vlb-comp-art ${stylex.props(sx.compArt, ((id) === "hook") && sx.compArtDataCompHook, ((id) === "hook-arrow") && sx.compArtDataCompHookArrow, ((id) === "circle-arrow") && sx.compArtDataCompCircleArrow, ((id) === "split") && sx.compArtDataCompSplit, ((id) === "closeup") && sx.compArtDataCompCloseup).className}`} aria-hidden="true" /><span>{t.library.prompts.composition[id]}</span></span>} />
                  ))}
                </div>
                <p className={`vlb-hint ${stylex.props(sx.hint).className}`}>{copy.compositionHelp}</p>
                {errors.composition && <FieldError>{errors.composition}</FieldError>}
              </div>
            )}

            <div className={`vlb-field ${stylex.props(sx.field).className}`}>
              <TextArea
                label={copy.promptLabel}
                value={text}
                maxLength={2000}
                readOnly={builtin}
                placeholder={kind === 'scene' ? copy.promptPlaceholder : copy.coverModeHelp}
                onChange={(event) => setText(event.currentTarget.value)}
                error={errors.prompt}
                onSubmitValue={() => void save()}
                style={{ minHeight: 180 }}
              />
              <p className={`vlb-hint ${stylex.props(sx.hint).className}`}>{kind === 'scene' ? copy.sceneModeHelp : copy.coverModeHelp}</p>
            </div>

            <div className={`vlb-field ${stylex.props(sx.field).className}`}>
              <FieldLabel>{copy.rulesTitle}</FieldLabel>
              <ul className={`vlb-rules ${stylex.props(sx.rules).className}`}>
                {(kind === 'scene' ? copy.rulesScene : copy.rulesCover).map((rule) => <li key={rule}>{rule}</li>)}
              </ul>
            </div>
            {prompt && <LearnedRulesList scope={{ imagePromptId: prompt.id }} title={t.learn.notes.title} help={t.learn.notes.help} empty={t.learn.notes.empty} errorText={t.learn.notes.error} />}
          </div>

          <aside className={`vlb-ed-aside ${stylex.props(sx.edAside).className}`}>
            <section className={`vlb-card vlb-card--flat ${stylex.props(sx.cardCardFlat).className}`}>
              <div className={`vlb-card-top ${stylex.props(sx.cardTop).className}`}>
                <FieldLabel>{copy.samplesTitle}</FieldLabel>
                <span className={`vlb-hint ${stylex.props(sx.hint).className}`}>{builtin ? copy.sampleNoteBuilt : copy.sampleNoteNone}</span>
              </div>
              <div className={`vlb-samples ${stylex.props(sx.samples).className}`} aria-label={copy.samplesTitle}>
                <span className={`vlb-sample ${stylex.props(sx.sample, ((kind) === "cover") && sx.sampleDataKindCover).className}`} data-kind={kind} aria-hidden="true" />
              </div>
              <p className={`vlb-hint ${stylex.props(sx.hint).className}`}>{copy.noSamples}</p>
              <p className={`vlb-hint ${stylex.props(sx.hint).className}`}>{copy.noSamplesHint}</p>
              <div className={`vlb-row ${stylex.props(sx.row).className}`}>
                <ActionButton xstyle={[sx.vlbCompsRadioCard]} variant="outline" icon={<Icon name="magicpen" size={16} />} lockedReason={copy.tryLocked}>{copy.tryAction}</ActionButton>
                <p className={`vlb-hint ${stylex.props(sx.hint).className}`}>{copy.tryNote}</p>
              </div>
            </section>

            <section className={`vlb-card vlb-card--flat ${stylex.props(sx.cardCardFlat).className}`}>
              <FieldLabel>{copy.usedTitle}</FieldLabel>
              {(prompt?.usedBy.length ?? 0) > 0
                ? (
                  <>
                    <ul className={`vlb-usedby ${stylex.props(sx.usedby).className}`}>
                      {prompt!.usedBy.map((ref) => <li key={ref.id}>{ref.name}</li>)}
                    </ul>
                    <p className={`vlb-hint ${stylex.props(sx.hint).className}`}>{builtin ? copy.usedNoteBuilt : copy.usedNoteInUse(prompt!.usedBy.length)}</p>
                  </>
                )
                : <p className={`vlb-hint ${stylex.props(sx.hint).className}`}>{builtin ? copy.usedNoteBuilt : copy.usedNoteEmpty}</p>}
            </section>
          </aside>
        </div>
      </div>

      <footer className={`vlb-ed-foot ${stylex.props(sx.edFoot).className}`}>
        <span className={`vlb-status ${stylex.props(sx.status).className}`} aria-live="polite">{status}</span>
        <span className={`vlb-ed-actions ${stylex.props(sx.edActions).className}`}>
          {prompt !== null && !builtin && (
            <ActionButton xstyle={[sx.vlbCompsRadioCard]} variant="danger-soft" icon={<Icon name="trash" size={16} />} onClick={() => setConfirming(true)}
              lockedReason={(prompt.usedBy.length ?? 0) > 0 ? copy.deleteDisabled : undefined}>
              {copy.deleteAction}
            </ActionButton>
          )}
          {prompt !== null && onDuplicate && (
            <Button xstyle={[sx.vlbCompsRadioCard]} size="sm" variant="ghost" onClick={onDuplicate}>
              <Icon name="copy" size={14} />{t.library.prompts.duplicateAction}
            </Button>
          )}
          <Button xstyle={[sx.vlbCompsRadioCard]} size="sm" onClick={onClose}>{t.library.cancel}</Button>
          <ActionButton xstyle={[sx.vlbCompsRadioCard]} variant="primary" pending={saving} onClick={() => void save()}>{primaryLabel}</ActionButton>
        </span>
      </footer>
      {confirming && prompt && (
        <Dialog title={copy.deleteTitle(prompt.name)} onClose={() => setConfirming(false)} width={420}
          footer={<>
            <Button size="sm" onClick={() => setConfirming(false)}>{t.library.cancel}</Button>
            <ActionButton variant="danger" onClick={() => void remove()}>{t.library.delete}</ActionButton>
          </>}>
          <p className={`vlb-dialog-text ${stylex.props(sx.dialogText).className}`}>{copy.deleteBody}</p>
        </Dialog>
      )}
    </div>
  )
}

function kindName(kind: PromptKind): string {
  return kind === 'scene' ? t.library.prompts.kindScene : t.library.prompts.kindCover
}

function blankDraft(name: string, kind: PromptKind, composition: ThumbnailStyle, prompt: string) {
  return { name, kind, prompt, ...(kind === 'cover' ? { composition } : {}) }
}

const sx = stylex.create({
  editor: { display: "flex", flexGrow: libraryVars.fillRatio, flexShrink: libraryVars.fillRatio, flexBasis: "auto", flexDirection: "column", minHeight: 0, minWidth: 0 },
  scroll: { flexGrow: libraryVars.fillRatio, flexShrink: libraryVars.fillRatio, flexBasis: "auto", minHeight: 0, minWidth: 0, overflowY: "auto", overscrollBehavior: "contain", paddingTop: { default: "var(--vv-s-4)", "@media (max-width: 767px)": "var(--vv-s-4)" }, paddingRight: { default: "var(--vv-s-28)", "@media (max-width: 767px)": "var(--vv-s-16)" }, paddingBottom: { default: "var(--vv-s-28)", "@media (max-width: 767px)": "var(--vv-s-24)" }, paddingLeft: { default: "var(--vv-s-28)", "@media (max-width: 767px)": "var(--vv-s-16)" }, display: "flex", flexDirection: "column", rowGap: "var(--vv-s-16)", columnGap: "var(--vv-s-16)" },
  back: { display: "inline-flex", alignItems: "center", rowGap: "var(--vv-s-6)", columnGap: "var(--vv-s-6)", alignSelf: "flex-start", paddingTop: "var(--vv-s-4)", paddingRight: "var(--vv-s-8)", paddingBottom: "var(--vv-s-4)", paddingLeft: "var(--vv-s-8)", borderTopWidth: 0, borderTopStyle: "none", borderTopColor: "currentColor", borderRightWidth: 0, borderRightStyle: "none", borderRightColor: "currentColor", borderBottomWidth: 0, borderBottomStyle: "none", borderBottomColor: "currentColor", borderLeftWidth: 0, borderLeftStyle: "none", borderLeftColor: "currentColor", borderTopLeftRadius: "var(--vv-r-7)", borderTopRightRadius: "var(--vv-r-7)", borderBottomRightRadius: "var(--vv-r-7)", borderBottomLeftRadius: "var(--vv-r-7)", backgroundColor: { default: "transparent", ":hover": "var(--vv-hover)" }, backgroundImage: { default: "none", ":hover": "none" }, color: { default: color["--color-fg-3"], ":hover": color["--color-fg"] }, fontSize: libraryVars.bodySmallFontSize, cursor: "pointer", minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" } },
  edHead: { display: "flex", alignItems: "center", rowGap: "var(--vv-s-10)", columnGap: "var(--vv-s-10)", flexWrap: "wrap" },
  vlbEdHeadH2: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: libraryVars.promptEditorVlbEdHeadH2FontSize, fontWeight: libraryVars.headingWeight },
  chip: { display: "inline-flex", alignItems: "center", rowGap: "var(--vv-s-4)", columnGap: "var(--vv-s-4)", paddingTop: "var(--vv-s-2)", paddingRight: "var(--vv-s-7)", paddingBottom: "var(--vv-s-2)", paddingLeft: "var(--vv-s-7)", borderTopLeftRadius: "var(--vv-r-pill)", borderTopRightRadius: "var(--vv-r-pill)", borderBottomRightRadius: "var(--vv-r-pill)", borderBottomLeftRadius: "var(--vv-r-pill)", borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: line["--line-2"], borderRightWidth: "var(--vv-s-1)", borderRightStyle: "solid", borderRightColor: line["--line-2"], borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: line["--line-2"], borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "solid", borderLeftColor: line["--line-2"], color: color["--color-fg-3"], fontSize: "var(--vv-s-11)", whiteSpace: "nowrap" },
  edGrid: { display: "grid", gridTemplateColumns: { default: libraryVars.promptEditorEdGridGridTemplateColumns, "@media (max-width: 767px)": libraryVars.overlaysTabGridGridOverlaysGridTemplateColumns3 }, rowGap: "var(--vv-s-20)", columnGap: "var(--vv-s-20)", alignItems: "start" },
  edMain: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-16)", columnGap: "var(--vv-s-16)", minWidth: 0 },
  field: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-6)", columnGap: "var(--vv-s-6)" },
  touchTarget: { minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" } },
  hint: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: "var(--vv-s-12)", color: color["--color-fg-4"], lineHeight: libraryVars.bodyLineHeight },
  comps: { display: "grid", gridTemplateColumns: libraryVars.promptEditorCompsGridTemplateColumns, rowGap: "var(--vv-s-8)", columnGap: "var(--vv-s-8)" },
  vlbCompsRadioCard: { minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" } },
  comp: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-5)", columnGap: "var(--vv-s-5)", fontSize: "var(--vv-s-12)" },
  compArt: { display: "block", height: "var(--vv-s-34)", borderTopLeftRadius: "var(--vv-r-5)", borderTopRightRadius: "var(--vv-r-5)", borderBottomRightRadius: "var(--vv-r-5)", borderBottomLeftRadius: "var(--vv-r-5)", backgroundColor: "var(--vv-tile-idle)", backgroundImage: "none" },
  compArtDataCompHook: { backgroundColor: "transparent", backgroundImage: libraryVars.promptEditorCompArtDataCompHookBackgroundImage },
  compArtDataCompHookArrow: { backgroundColor: "transparent", backgroundImage: libraryVars.promptEditorCompArtDataCompHookBackgroundImage },
  compArtDataCompCircleArrow: { backgroundColor: "transparent", backgroundImage: libraryVars.promptEditorCompArtDataCompCircleArrowBackgroundImage },
  compArtDataCompSplit: { backgroundColor: "transparent", backgroundImage: libraryVars.promptEditorCompArtDataCompCircleArrowBackgroundImage },
  compArtDataCompCloseup: { backgroundColor: "transparent", backgroundImage: libraryVars.promptEditorCompArtDataCompCircleArrowBackgroundImage },
  rules: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, paddingLeft: "var(--vv-s-18)", display: "flex", flexDirection: "column", rowGap: "var(--vv-s-4)", columnGap: "var(--vv-s-4)", fontSize: "var(--vv-s-12)", color: color["--color-fg-3"] },
  edAside: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-12)", columnGap: "var(--vv-s-12)", minWidth: 0 },
  cardCardFlat: { display: "flex", flexDirection: "column", minWidth: 0, backgroundColor: color["--color-surface-1"], backgroundImage: "none", borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: { default: line["--line-2"], ":hover": line["--line-3"] }, borderRightWidth: "var(--vv-s-1)", borderRightStyle: "solid", borderRightColor: { default: line["--line-2"], ":hover": line["--line-3"] }, borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: { default: line["--line-2"], ":hover": line["--line-3"] }, borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "solid", borderLeftColor: { default: line["--line-2"], ":hover": line["--line-3"] }, borderTopLeftRadius: "var(--vv-r-10)", borderTopRightRadius: "var(--vv-r-10)", borderBottomRightRadius: "var(--vv-r-10)", borderBottomLeftRadius: "var(--vv-r-10)", overflowX: "hidden", overflowY: "hidden", paddingTop: "var(--vv-s-12)", paddingRight: "var(--vv-s-12)", paddingBottom: "var(--vv-s-12)", paddingLeft: "var(--vv-s-12)", rowGap: "var(--vv-s-10)", columnGap: "var(--vv-s-10)" },
  cardTop: { display: "flex", alignItems: "flex-start", rowGap: "var(--vv-s-6)", columnGap: "var(--vv-s-6)", minWidth: 0 },
  samples: { display: "grid", gridTemplateColumns: libraryVars.promptEditorSamplesGridTemplateColumns, rowGap: "var(--vv-s-6)", columnGap: "var(--vv-s-6)" },
  sample: { display: "block", aspectRatio: libraryVars.videoAspectRatio, borderTopLeftRadius: "var(--vv-r-7)", borderTopRightRadius: "var(--vv-r-7)", borderBottomRightRadius: "var(--vv-r-7)", borderBottomLeftRadius: "var(--vv-r-7)", backgroundColor: "var(--vv-tile-idle)", backgroundImage: "none" },
  sampleDataKindCover: { backgroundColor: "transparent", backgroundImage: libraryVars.promptEditorSampleDataKindCoverBackgroundImage },
  row: { display: "flex", alignItems: "center", rowGap: "var(--vv-s-10)", columnGap: "var(--vv-s-10)", flexWrap: "wrap" },
  usedby: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, paddingLeft: "var(--vv-s-18)", fontSize: "var(--vv-s-12)", color: color["--color-fg-2"], display: "flex", flexDirection: "column", rowGap: "var(--vv-s-4)", columnGap: "var(--vv-s-4)" },
  edFoot: { position: "sticky", bottom: 0, display: "flex", alignItems: "center", justifyContent: "space-between", rowGap: "var(--vv-s-12)", columnGap: "var(--vv-s-12)", paddingTop: { default: "var(--vv-s-10)", "@media (max-width: 767px)": "var(--vv-s-10)" }, paddingRight: { default: "var(--vv-s-28)", "@media (max-width: 767px)": "var(--vv-s-16)" }, paddingBottom: { default: "var(--vv-s-10)", "@media (max-width: 767px)": "var(--vv-s-10)" }, paddingLeft: { default: "var(--vv-s-28)", "@media (max-width: 767px)": "var(--vv-s-16)" }, borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: line["--line-2"], backgroundColor: color["--color-bg"], backgroundImage: "none", flexWrap: "wrap" },
  status: { fontSize: "var(--vv-s-12)", color: color["--color-fg-4"], fontVariantNumeric: "tabular-nums" },
  edActions: { display: "flex", alignItems: "center", rowGap: "var(--vv-s-8)", columnGap: "var(--vv-s-8)" },
  dialogText: { marginTop: 0, marginRight: 0, marginBottom: "var(--vv-s-12)", marginLeft: 0, fontSize: libraryVars.bodySmallFontSize, color: color["--color-fg-3"], lineHeight: libraryVars.helpLineHeight },
})
