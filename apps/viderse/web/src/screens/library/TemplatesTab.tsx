// Tab Template của Thư viện (Library.dc.html › tweak tabs=templates): lưới thẻ
// template gồm sức khoẻ, prompt đang chọn, nền/overlay đang dùng, kênh dùng, và
// menu Đổi tên / Thêm vào kênh / Xoá (khoá kèm lý do khi kênh còn dùng).
import { useState } from 'react'
import { ActionMenu, Banner, Button, Dialog, Icon, TextField, confirm } from '@noverse/ui'
import { invalidate } from '@noverse/ui/data'
import { vi as t } from '../../i18n/vi'
import { navigate, paths } from '../../router'
import { useResource } from '../../data/useResource'
import { keys } from '../../data/keys'
import {
  duplicateTemplate, listTemplates, patchTemplate, removeTemplate,
  type OwnerChannelRef, type TemplateSummary,
} from '../../api/library'
import { AttachChannelDialog } from './AttachChannelDialog'
import { editedLabel, initials, num } from './fmt'

const styleOf = (template: TemplateSummary) => template.style === 'full'
  ? t.library.templates.style.full
  : template.framing === 'close' ? t.library.templates.style.close : t.library.templates.style.cutout

export const imagesText = (template: TemplateSummary) => template.images.mode === 'none'
  ? t.library.templates.imagesNone
  : template.images.mode === 'per-minutes'
    ? t.library.templates.imagesPerMinutes(num(template.images.minutes))
    : t.library.templates.imagesFixed(num(template.images.count))

function TemplateCard({ template, channels, onRename, onAttach, onDeleted }: {
  template: TemplateSummary
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

  const duplicate = async () => {
    try {
      await duplicateTemplate(template.id)
      await invalidate(keys.templates())
    } catch (err) {
      onDeleted(err instanceof Error ? err.message : String(err))
    }
  }

  return (
    <article className="vlb-card">
      <div className="vlb-strip" data-style={template.framing === 'close' ? 'close' : template.style} aria-hidden="true">
        <span className="vlb-strip-bar" /><span className="vlb-strip-bar" /><span className="vlb-strip-bar" />
        <span className="vlb-strip-chip vlb-strip-chip--health" data-tone={broken ? 'warn' : 'ok'}>{broken ? copy.healthFix : copy.healthOk}</span>
        <span className="vlb-strip-chip vlb-strip-chip--style">{styleOf(template)}</span>
      </div>
      <div className="vlb-card-body">
        <div className="vlb-card-top">
          <h3 className="vlb-card-name">
            <a href={`#${paths.template(template.id)}`}>{template.name}</a>
          </h3>
          <span className="vlb-card-edited">{editedLabel(template.updatedAt, { today: copy.editedToday, date: copy.editedDate })}</span>
        </div>
        <div className="vlb-chips">
          {backdropText && <span className="vlb-chip">{backdropText}</span>}
        </div>
        <p className="vlb-card-sum">
          {template.prompts.summary}
          {' · '}
          {imagesText(template)}
          {template.overlayCount > 0 && ` · ${t.library.templates.overlays(template.overlayCount)}`}
        </p>
        <div className="vlb-card-meta">
          {inUse && (
            <span className="vlb-avatars" aria-hidden="true">
              {template.channels.slice(0, 3).map((channel) => (
                <span key={channel.id} className="vlb-avatar vlb-avatar--sm" title={channel.name}>{initials(channel.name)}</span>
              ))}
              {template.channels.length > 3 && <span className="vlb-avatar vlb-avatar--sm">+{template.channels.length - 3}</span>}
            </span>
          )}
          <span>{inUse ? copy.usedText(names) : copy.usedText([])}</span>
        </div>
        <div className="vlb-card-actions">
          <Button size="sm" variant="ghost" onClick={() => navigate(paths.template(template.id))}>{broken ? copy.openFix : copy.open}</Button>
          <Button size="sm" variant="ghost" aria-label={copy.duplicateAria(template.name)}
            onClick={() => void duplicate()}><Icon name="copy" size={14} />{t.library.duplicate}</Button>
          <span className="vlb-card-end">
            <Button size="sm" variant="icon" aria-label={copy.exportAria(template.name)} title={copy.exportCardLocked}
              aria-disabled onClick={(event) => event.preventDefault()}><Icon name="export" size={15} /></Button>
            <ActionMenu
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

export function TemplatesTab({ channels }: { channels: OwnerChannelRef[] }) {
  const copy = t.library.templates
  const templates = useResource(keys.templates(), ({ signal }) => listTemplates(signal))
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
    <div className="vlb-scroll">
      <p className="vlb-intro">{copy.intro}</p>
      {actionError && <Banner tone="error" title={actionError} />}
      {items.length === 0 && !templates.initialLoading
        ? (
          <div className="vlb-empty">
            <p className="vlb-empty-title">{copy.emptyTitle}</p>
            <p className="vlb-hint">{copy.emptyHint}</p>
          </div>
        )
        : (
          <div className="vlb-grid">
            {items.map((template) => (
              <TemplateCard
                key={template.id}
                template={template}
                channels={channels}
                onRename={() => openRename(template)}
                onAttach={() => setAttach(true)}
                onDeleted={(message) => { if (message) setActionError(message); else setActionError(null) }}
              />
            ))}
          </div>
        )}

      {renaming && (
        <Dialog title={copy.renameTitle(renaming.name)} onClose={() => setRenaming(null)} width={420}
          footer={<>
            <Button size="sm" onClick={() => setRenaming(null)}>{t.library.cancel}</Button>
            <Button size="sm" variant="primary" onClick={() => void saveRename()}>{copy.renameAction}</Button>
          </>}>
          <label className="vlb-field">
            <span className="lbl">{t.library.nameLabel}</span>
            <TextField value={renameName} autoFocus aria-invalid={renameError ? true : undefined}
              onChange={setRenameName}
              onKeyDown={(event) => { if (event.key === 'Enter') void saveRename() }} />
          </label>
          {renameError && <p className="vlb-fielderr" role="alert">{renameError}</p>}
        </Dialog>
      )}
      {attach && <AttachChannelDialog channels={channels} onClose={() => setAttach(false)} />}
    </div>
  )
}
