// Mục Xuất & kênh (Template.dc.html › lib=output): nén sau xuất, kênh đang
// dùng template, bố cục (chỉ chỉnh trong editor), xuất tệp (khoá) và xoá
// template — xoá bị khoá kèm lý do khi kênh còn dùng (409 của máy chủ).
import { confirm, RadioCard, Switch, Icon, toast } from '@noverse/ui'
import { useState } from 'react'
import { invalidate } from '@noverse/ui/data'
import { keys } from '../../../data/keys'
import { removeTemplateFromChannel } from '../../../api/library'
import { ActionButton } from '../../../components'
import { vi as t } from '../../../i18n/vi'
import { initials, num } from '../fmt'
import type { Recipe, TemplateDetail } from '../../../api/library'

export function OutputSection({ detail, onSave, onDuplicate, onDelete, onAttach }: {
  detail: TemplateDetail
  onSave: (patch: Partial<Recipe>) => void
  onDuplicate: () => void
  onDelete: () => Promise<void>
  onAttach: () => void
}) {
  const copy = t.library.template.output
  const channels = detail.channels
  const inUse = channels.length > 0
  const compressed = detail.recipe.output.compressExport
  const [removing, setRemoving] = useState<string | null>(null)

  const detach = async (channelId: string) => {
    setRemoving(channelId)
    try {
      await removeTemplateFromChannel(channelId, detail.id)
      await Promise.all([invalidate(keys.template(detail.id)), invalidate(keys.templates())])
      toast(copy.usersRemove, 'ok')
    } catch {
      toast(copy.usersRemoveFail, 'err')
    } finally { setRemoving(null) }
  }

  const remove = async () => {
    const accepted = await confirm({
      title: t.library.template.deleteTitle(detail.name),
      body: t.library.template.deleteBody,
      tone: 'danger',
      confirmLabel: t.library.template.deleteAction,
      cancelLabel: t.library.cancel,
      onConfirm: () => undefined,
    })
    if (accepted) await onDelete()
  }

  return (
    <section className="vt-sec" aria-labelledby="vt-output-title">
      <div className="vt-sec-head">
        <h2 id="vt-output-title">{copy.title}</h2>
        <p className="vt-help">{copy.help}</p>
      </div>

      <div className="vt-card">
        <div className="vt-switch-row">
          <span className="vt-label">{copy.compressTitle}</span>
          <Switch label={copy.compressTitle} checked={compressed}
            onChange={(on) => onSave({ output: { compressExport: on } })} />
        </div>
        <div role="radiogroup" aria-label={copy.compressTitle} className="vt-export">
          <RadioCard checked={compressed} onChange={() => onSave({ output: { compressExport: true } })}
            label={<span className="vt-export-card">
              <span className="vt-export-name">{copy.compressed.title}</span>
              <span className="vt-export-size num">{copy.compressed.size}</span>
              <span className="vt-hint">{copy.compressed.note}</span>
            </span>} />
          <RadioCard checked={!compressed} onChange={() => onSave({ output: { compressExport: false } })}
            label={<span className="vt-export-card">
              <span className="vt-export-name">{copy.original.title}</span>
              <span className="vt-export-size num">{copy.original.size}</span>
              <span className="vt-hint">{copy.original.note}</span>
            </span>} />
        </div>
      </div>

      <div className="vt-card">
        <div className="vt-card-head">
          <span className="vt-label">{copy.usersTitle}</span>
          <ActionButton variant="quiet" icon={<Icon name="add" size={16} />} onClick={onAttach}>{copy.usersAdd}</ActionButton>
        </div>
        {inUse ? (
          <ul className="vt-users">
            {channels.map((channel) => (
              <li key={channel.id} className="vt-user">
                <span className="vt-avatar vt-avatar--sm" aria-hidden="true">{initials(channel.name)}</span>
                <span className="vt-user-main">
                  <span className="vt-user-name">{channel.name}</span>
                  {channel.position !== undefined && (
                    <span className="vt-hint">{copy.usersPosition(num(channel.position + 1))}</span>
                  )}
                </span>
                <ActionButton variant="quiet" pending={removing === channel.id} onClick={() => void detach(channel.id)}>
                  {copy.usersRemove}
                </ActionButton>
              </li>
            ))}
          </ul>
        ) : <p className="vt-hint">{copy.usersEmpty}</p>}
        <p className="vt-hint">{copy.usersNote}</p>
      </div>

      <div className="vt-card">
        <span className="vt-label">{copy.layoutTitle}</span>
        <div className="vt-chips">
          {copy.layoutChips.map((chip) => <span key={chip} className="vt-chip">{chip}</span>)}
        </div>
        <p className="vt-hint">{copy.layoutHelp}</p>
        <ActionButton variant="outline" lockedReason={copy.layoutEditLocked}>{copy.layoutEdit}</ActionButton>
      </div>

      <div className="vt-card">
        <span className="vt-label">{copy.fileTitle}</span>
        <div className="vt-row">
          <ActionButton variant="outline" icon={<Icon name="export" size={16} />} lockedReason={copy.exportLocked}>{copy.exportFile}</ActionButton>
          <ActionButton variant="outline" icon={<Icon name="copy" size={16} />} onClick={onDuplicate}>{t.library.duplicate}</ActionButton>
        </div>
      </div>

      <div className="vt-card vt-danger">
        <span className="vt-label">{copy.deleteTitle}</span>
        <div className="vt-switch-row">
          <p className="vt-hint">{inUse ? copy.deleteReason(channels.map((channel) => channel.name)) : copy.deleteFree}</p>
          <ActionButton
            variant="danger-soft"
            icon={<Icon name="trash" size={16} />}
            onClick={inUse ? undefined : () => void remove()}
            lockedReason={inUse ? copy.deleteLocked : undefined}
          >{t.library.delete}</ActionButton>
        </div>
      </div>
    </section>
  )
}
