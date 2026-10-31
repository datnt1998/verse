// Khung sửa prompt ảnh (Library.dc.html › view=editor): tên 1–60, loại
// ảnh cảnh/ảnh bìa, bố cục bìa, nội dung ≤ 2.000 ký tự, luật áp mọi prompt,
// template đang dùng. Prompt có sẵn chỉ nhân bản được; prompt đang được dùng
// thì không xoá và không đổi loại (máy chủ trả 409 in_use kèm tên template).
import { useState } from 'react'
import { Banner, Button, Dialog, FieldError, FieldLabel, RadioCard, Segmented, TextArea, TextField, Icon } from '@noverse/ui'
import { ActionButton } from '../../components'
import { invalidate } from '@noverse/ui/data'
import { vi as t } from '../../i18n/vi'
import {
  createPrompt, fieldErrors, libraryErrorInfo, patchPrompt, removePrompt,
  type ImagePrompt, type PromptKind, type ThumbnailStyle,
} from '../../api/library'
import { keys } from '../../data/keys'
import { clock } from './fmt'

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
    <div className="vlb-editor">
      <div className="vlb-scroll">
        <button type="button" className="vlb-back" onClick={onClose} aria-label={copy.backAria}>
          <span aria-hidden="true">←</span>{copy.back}
        </button>
        <header className="vlb-ed-head">
          <h2>{heading}</h2>
          <span className="vlb-chip">{badge}</span>
        </header>
        {builtin && <Banner tone="info" title={copy.builtNote} />}
        {error && <Banner tone="error" title={error} />}

        <div className="vlb-ed-grid">
          <div className="vlb-ed-main">
            <label className="vlb-field">
              <FieldLabel htmlFor="vlb-prompt-name">{copy.nameLabel}</FieldLabel>
              <TextField id="vlb-prompt-name" value={name} maxLength={60} readOnly={builtin}
                aria-invalid={errors.name ? true : undefined}
                onChange={setName} />
              {errors.name && <FieldError>{errors.name}</FieldError>}
            </label>

            <div className="vlb-field">
              <FieldLabel>{copy.kindLabel}</FieldLabel>
              <Segmented
                label={copy.kindLabel}
                value={kind}
                fit="content"
                onChange={(next) => { if (!kindLocked) setKind(next as PromptKind) }}
                options={[
                  { id: 'scene', label: copy.kindScene, disabled: kindLocked },
                  { id: 'cover', label: copy.kindCover, disabled: kindLocked },
                ]}
              />
              <p className="vlb-hint">{kindLocked ? copy.kindLockedHelp : kind === 'scene' ? copy.kindSceneHelp : copy.kindCoverHelp}</p>
              {errors.kind && <FieldError>{errors.kind}</FieldError>}
            </div>

            {kind === 'cover' && (
              <div className="vlb-field">
                <FieldLabel>{copy.compositionTitle}</FieldLabel>
                <div role="radiogroup" aria-label={copy.compositionTitle} className="vlb-comps">
                  {COMPOSITIONS.map((id) => (
                    <RadioCard key={id} checked={composition === id} disabled={builtin}
                      onChange={() => setComposition(id)}
                      label={<span className="vlb-comp" data-comp={id}><span className="vlb-comp-art" aria-hidden="true" /><span>{t.library.prompts.composition[id]}</span></span>} />
                  ))}
                </div>
                <p className="vlb-hint">{copy.compositionHelp}</p>
                {errors.composition && <FieldError>{errors.composition}</FieldError>}
              </div>
            )}

            <div className="vlb-field">
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
              <p className="vlb-hint">{kind === 'scene' ? copy.sceneModeHelp : copy.coverModeHelp}</p>
            </div>

            <div className="vlb-field">
              <FieldLabel>{copy.rulesTitle}</FieldLabel>
              <ul className="vlb-rules">
                {(kind === 'scene' ? copy.rulesScene : copy.rulesCover).map((rule) => <li key={rule}>{rule}</li>)}
              </ul>
            </div>
          </div>

          <aside className="vlb-ed-aside">
            <section className="vlb-card vlb-card--flat">
              <div className="vlb-card-top">
                <FieldLabel>{copy.samplesTitle}</FieldLabel>
                <span className="vlb-hint">{builtin ? copy.sampleNoteBuilt : copy.sampleNoteNone}</span>
              </div>
              <div className="vlb-samples" aria-label={copy.samplesTitle}>
                <span className="vlb-sample" data-kind={kind} aria-hidden="true" />
              </div>
              <p className="vlb-hint">{copy.noSamples}</p>
              <p className="vlb-hint">{copy.noSamplesHint}</p>
              <div className="vlb-row">
                <ActionButton variant="outline" icon={<Icon name="magicpen" size={16} />} lockedReason={copy.tryLocked}>{copy.tryAction}</ActionButton>
                <p className="vlb-hint">{copy.tryNote}</p>
              </div>
            </section>

            <section className="vlb-card vlb-card--flat">
              <FieldLabel>{copy.usedTitle}</FieldLabel>
              {(prompt?.usedBy.length ?? 0) > 0
                ? (
                  <>
                    <ul className="vlb-usedby">
                      {prompt!.usedBy.map((ref) => <li key={ref.id}>{ref.name}</li>)}
                    </ul>
                    <p className="vlb-hint">{builtin ? copy.usedNoteBuilt : copy.usedNoteInUse(prompt!.usedBy.length)}</p>
                  </>
                )
                : <p className="vlb-hint">{builtin ? copy.usedNoteBuilt : copy.usedNoteEmpty}</p>}
            </section>
          </aside>
        </div>
      </div>

      <footer className="vlb-ed-foot">
        <span className="vlb-status" aria-live="polite">{status}</span>
        <span className="vlb-ed-actions">
          {prompt !== null && !builtin && (
            <ActionButton variant="danger-soft" icon={<Icon name="trash" size={16} />} onClick={() => setConfirming(true)}
              lockedReason={(prompt.usedBy.length ?? 0) > 0 ? copy.deleteDisabled : undefined}>
              {copy.deleteAction}
            </ActionButton>
          )}
          {prompt !== null && onDuplicate && (
            <Button size="sm" variant="ghost" onClick={onDuplicate}>
              <Icon name="copy" size={14} />{t.library.prompts.duplicateAction}
            </Button>
          )}
          <Button size="sm" onClick={onClose}>{t.library.cancel}</Button>
          <ActionButton variant="primary" pending={saving} onClick={() => void save()}>{primaryLabel}</ActionButton>
        </span>
      </footer>
      {confirming && prompt && (
        <Dialog title={copy.deleteTitle(prompt.name)} onClose={() => setConfirming(false)} width={420}
          footer={<>
            <Button size="sm" onClick={() => setConfirming(false)}>{t.library.cancel}</Button>
            <ActionButton variant="danger" onClick={() => void remove()}>{t.library.delete}</ActionButton>
          </>}>
          <p className="vlb-dialog-text">{copy.deleteBody}</p>
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
