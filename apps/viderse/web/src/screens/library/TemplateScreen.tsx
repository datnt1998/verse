// Cài đặt template (artboard Template): cột mục + khung xem thử/sức
// khoẻ, 4 mục. Mọi thay đổi tự lưu: ô chữ khi rời ô, điều khiển rời rạc khi
// đổi; một lần lưu đang chạy thì gộp lần sau (useTemplateDraft). Mục Nền & hiệu
// ứng do mảng Video nền dựng trong BackdropSection.tsx.
import { useState } from 'react'
import { ActionMenu, Banner, EmptyState, Spinner, Icon } from '@noverse/ui'
import { invalidate } from '@noverse/ui/data'
import { vi as t } from '../../i18n/vi'
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
import './template.css'

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
  const copy = t.library.template
  const detail = useResource(keys.template(route.templateId), ({ signal }) => getTemplate(route.templateId, signal))
  const section: TemplateSection = route.section ?? 'images'

  if (detail.initialLoading) {
    return (
      <div className="vt-shell vt-shell--center" role="status" aria-label={copy.loadFail}>
        <Spinner />
      </div>
    )
  }
  if (!detail.data) {
    return (
      <div className="vt-shell vt-shell--center">
        <EmptyState variant="frame" icon="warning-2" title={copy.notFound}
          hint={detail.error ? String(detail.error) : undefined}
          action={<ActionButton variant="outline" onClick={() => navigate(paths.library('templates'))}>{copy.backToLibrary}</ActionButton>} />
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
    <div className="vt-shell">
      <TemplateNav detail={detail} active={section}
        overlayNames={(overlays.data?.items ?? []).filter((item) => recipe.overlays.includes(item.id)).map((item) => item.name)} />
      <main className="vt-main">
        <header className="vt-head">
          <div className="vt-head-main">
            <div className="vt-title-row">
            <input
              className="vt-name"
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
            <div className="vt-chips">
              <span className="vt-chip" data-tone={healthWarn(detail) ? 'warn' : 'ok'}>
                <span className="vt-dot" data-tone={healthWarn(detail) ? 'warn' : 'ok'} aria-hidden="true" />
                {healthWarn(detail) ? copy.healthWarnChip : copy.healthOkChip}
              </span>
              <span className="vt-chip">{detail.channels.length ? copy.channelsChip(detail.channels.length) : copy.noChannelsChip}</span>
            </div>
            </div>
            {nameError && <p id="vt-name-error" className="vt-fielderr" role="alert">{nameError}</p>}
          </div>
          <div className="vt-head-actions" aria-live="polite">
            <span className="vt-saved" data-state={draft.saving ? 'saving' : draft.conflict ? 'conflict' : 'saved'} aria-live="polite">
              {draft.saving ? copy.saving : t.library.saved(clock(detail.savedAt))}
            </span>
            <ActionButton variant="outline" lockedReason={copy.openEditorLocked}>{copy.openEditor}</ActionButton>
            <ActionMenu
              label={copy.moreAria}
              items={[
                { id: 'duplicate', label: t.library.duplicate, icon: <Icon name="copy" size={16} />, onSelect: () => void duplicate() },
              ]}
              trigger={<button type="button" className="vt-iconbtn" aria-label={copy.moreAria} aria-haspopup="menu"><Icon name="more" size={16} /></button>}
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
              <span key={field} className="vt-field-line">
                {copy.fieldError(copy.fieldLabels[field] ?? field, message)}
              </span>
            ))}
          </Banner>
        )}

        <div className="vt-row2">
          <div className="vt-settings">
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
          <aside className="vt-aside">
            <PreviewPane detail={detail} dirty={draft.dirty} />
          </aside>
        </div>
      </main>
      {attach && <AttachChannelDialog channels={channels.data?.channels ?? []} onClose={() => setAttach(false)} />}
    </div>
  )
}
