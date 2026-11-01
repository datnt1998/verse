import * as stylex from '@stylexjs/stylex'
import { color, line } from '@noverse/ui/tokens.stylex'
import { libraryVars } from '../libraryTokens.stylex'
// Mục Xuất & kênh (Template.dc.html › lib=output): nén sau xuất, kênh đang
// dùng template, bố cục (chỉ chỉnh trong editor), xuất tệp (khoá) và xoá
// template — xoá bị khoá kèm lý do khi kênh còn dùng (409 của máy chủ).
import { confirm, RadioCard, Switch, Icon, toast } from '@noverse/ui'
import { useState } from 'react'
import { invalidate } from '@noverse/ui/data'
import { keys } from '../../../data/keys'
import { removeTemplateFromChannel } from '../../../api/library'
import { ActionButton } from '../../../components'
import { t } from '../../../i18n/library'
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
    <section className={`vt-sec ${stylex.props(sx.sec).className}`} aria-labelledby="vt-output-title">
      <div className={`vt-sec-head ${stylex.props(sx.secHead).className}`}>
        <h2 {...stylex.props(sx.vtSecHeadH2)} id="vt-output-title">{copy.title}</h2>
        <p className={`vt-help ${stylex.props(sx.help).className}`}>{copy.help}</p>
      </div>

      <div className={`vt-card ${stylex.props(sx.card).className}`}>
        <div className={`vt-switch-row ${stylex.props(sx.switchRow).className}`}>
          <span className={`vt-label ${stylex.props(sx.label).className}`}>{copy.compressTitle}</span>
          <Switch xstyle={[sx.vtSwitchRowSwitch]} label={copy.compressTitle} checked={compressed}
            onChange={(on) => onSave({ output: { compressExport: on } })} />
        </div>
        <div role="radiogroup" aria-label={copy.compressTitle} className={`vt-export ${stylex.props(sx.export).className}`}>
          <RadioCard xstyle={[sx.vtSwitchRowSwitch]} checked={compressed} onChange={() => onSave({ output: { compressExport: true } })}
            label={<span className={`vt-export-card ${stylex.props(sx.exportCard).className}`}>
              <span className={`vt-export-name ${stylex.props(sx.exportName).className}`}>{copy.compressed.title}</span>
              <span className={`vt-export-size num ${stylex.props(sx.exportSizeNum).className}`}>{copy.compressed.size}</span>
              <span className={`vt-hint ${stylex.props(sx.hint).className}`}>{copy.compressed.note}</span>
            </span>} />
          <RadioCard xstyle={[sx.vtSwitchRowSwitch]} checked={!compressed} onChange={() => onSave({ output: { compressExport: false } })}
            label={<span className={`vt-export-card ${stylex.props(sx.exportCard).className}`}>
              <span className={`vt-export-name ${stylex.props(sx.exportName).className}`}>{copy.original.title}</span>
              <span className={`vt-export-size num ${stylex.props(sx.exportSizeNum).className}`}>{copy.original.size}</span>
              <span className={`vt-hint ${stylex.props(sx.hint).className}`}>{copy.original.note}</span>
            </span>} />
        </div>
      </div>

      <div className={`vt-card ${stylex.props(sx.card).className}`}>
        <div className={`vt-card-head ${stylex.props(sx.cardHead).className}`}>
          <span className={`vt-label ${stylex.props(sx.label).className}`}>{copy.usersTitle}</span>
          <ActionButton xstyle={[sx.vtSwitchRowSwitch]} variant="quiet" icon={<Icon name="add" size={16} />} onClick={onAttach}>{copy.usersAdd}</ActionButton>
        </div>
        {inUse ? (
          <ul className={`vt-users ${stylex.props(sx.users).className}`}>
            {channels.map((channel) => (
              <li key={channel.id} className={`vt-user ${stylex.props(sx.user).className}`}>
                <span className={`vt-avatar vt-avatar--sm ${stylex.props(sx.avatarAvatarSm).className}`} aria-hidden="true">{initials(channel.name)}</span>
                <span className={`vt-user-main ${stylex.props(sx.userMain).className}`}>
                  <span className={`vt-user-name ${stylex.props(sx.userName).className}`}>{channel.name}</span>
                  {channel.position !== undefined && (
                    <span className={`vt-hint ${stylex.props(sx.hint2).className}`}>{copy.usersPosition(num(channel.position + 1))}</span>
                  )}
                </span>
                <ActionButton xstyle={[sx.vtUserActionButton]} variant="quiet" pending={removing === channel.id} onClick={() => void detach(channel.id)}>
                  {copy.usersRemove}
                </ActionButton>
              </li>
            ))}
          </ul>
        ) : <p className={`vt-hint ${stylex.props(sx.hint).className}`}>{copy.usersEmpty}</p>}
        <p className={`vt-hint ${stylex.props(sx.hint).className}`}>{copy.usersNote}</p>
      </div>

      <div className={`vt-card ${stylex.props(sx.card).className}`}>
        <span className={`vt-label ${stylex.props(sx.label).className}`}>{copy.layoutTitle}</span>
        <div className={`vt-chips ${stylex.props(sx.chips).className}`}>
          {copy.layoutChips.map((chip) => <span key={chip} className={`vt-chip ${stylex.props(sx.chip).className}`}>{chip}</span>)}
        </div>
        <p className={`vt-hint ${stylex.props(sx.hint).className}`}>{copy.layoutHelp}</p>
        <ActionButton xstyle={[sx.vtSwitchRowSwitch]} variant="outline" lockedReason={copy.layoutEditLocked}>{copy.layoutEdit}</ActionButton>
      </div>

      <div className={`vt-card ${stylex.props(sx.card).className}`}>
        <span className={`vt-label ${stylex.props(sx.label).className}`}>{copy.fileTitle}</span>
        <div className={`vt-row ${stylex.props(sx.row).className}`}>
          <ActionButton xstyle={[sx.vtSwitchRowSwitch]} variant="outline" icon={<Icon name="export" size={16} />} lockedReason={copy.exportLocked}>{copy.exportFile}</ActionButton>
          <ActionButton xstyle={[sx.vtSwitchRowSwitch]} variant="outline" icon={<Icon name="copy" size={16} />} onClick={onDuplicate}>{t.library.duplicate}</ActionButton>
        </div>
      </div>

      <div className={`vt-card vt-danger ${stylex.props(sx.cardDanger).className}`}>
        <span className={`vt-label ${stylex.props(sx.label).className}`}>{copy.deleteTitle}</span>
        <div className={`vt-switch-row ${stylex.props(sx.switchRow).className}`}>
          <p className={`vt-hint ${stylex.props(sx.hint).className}`}>{inUse ? copy.deleteReason(channels.map((channel) => channel.name)) : copy.deleteFree}</p>
          <ActionButton xstyle={[sx.vtSwitchRowSwitch]}
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

const sx = stylex.create({
  sec: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-12)", columnGap: "var(--vv-s-12)", minWidth: 0 },
  secHead: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-4)", columnGap: "var(--vv-s-4)" },
  vtSecHeadH2: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: "var(--vv-s-16)", fontWeight: libraryVars.headingWeight },
  help: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: libraryVars.bodySmallFontSize, color: color["--color-fg-3"], lineHeight: libraryVars.helpLineHeight, maxWidth: libraryVars.templateBackdropSectionHelpMaxWidth },
  card: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-10)", columnGap: "var(--vv-s-10)", paddingTop: "var(--vv-s-14)", paddingRight: "var(--vv-s-14)", paddingBottom: "var(--vv-s-14)", paddingLeft: "var(--vv-s-14)", borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: line["--line-2"], borderRightWidth: "var(--vv-s-1)", borderRightStyle: "solid", borderRightColor: line["--line-2"], borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: line["--line-2"], borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "solid", borderLeftColor: line["--line-2"], borderTopLeftRadius: "var(--vv-r-12)", borderTopRightRadius: "var(--vv-r-12)", borderBottomRightRadius: "var(--vv-r-12)", borderBottomLeftRadius: "var(--vv-r-12)", backgroundColor: color["--color-card"], backgroundImage: "none", minWidth: 0 },
  switchRow: { display: "flex", alignItems: "center", justifyContent: "space-between", rowGap: "var(--vv-s-12)", columnGap: "var(--vv-s-12)", flexWrap: "wrap" },
  label: { fontSize: "var(--vv-s-14)", fontWeight: libraryVars.labelWeight, color: color["--color-fg"] },
  vtSwitchRowSwitch: { minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" } },
  export: { display: "grid", gridTemplateColumns: libraryVars.templateCoverSectionSourcesGridTemplateColumns, rowGap: "var(--vv-s-8)", columnGap: "var(--vv-s-8)" },
  exportCard: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-2)", columnGap: "var(--vv-s-2)" },
  exportName: { fontSize: "var(--vv-s-13)", fontWeight: libraryVars.labelWeight, color: color["--color-fg"] },
  exportSizeNum: { fontSize: "var(--vv-s-20)", fontWeight: libraryVars.headingWeight, color: color["--color-fg"] },
  hint: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: libraryVars.captionFontSize, color: color["--color-fg-4"], lineHeight: libraryVars.bodyLineHeight },
  cardHead: { display: "flex", alignItems: "baseline", justifyContent: "space-between", rowGap: "var(--vv-s-10)", columnGap: "var(--vv-s-10)", flexWrap: "wrap" },
  users: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, paddingTop: 0, paddingRight: 0, paddingBottom: 0, paddingLeft: 0, listStyle: "none", display: "flex", flexDirection: "column", rowGap: "var(--vv-s-4)", columnGap: "var(--vv-s-4)" },
  user: { display: "flex", alignItems: "center", rowGap: "var(--vv-s-10)", columnGap: "var(--vv-s-10)", paddingTop: "var(--vv-s-5)", paddingRight: "var(--vv-s-8)", paddingBottom: "var(--vv-s-5)", paddingLeft: "var(--vv-s-8)", borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: line["--line-2"], borderRightWidth: "var(--vv-s-1)", borderRightStyle: "solid", borderRightColor: line["--line-2"], borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: line["--line-2"], borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "solid", borderLeftColor: line["--line-2"], borderTopLeftRadius: "var(--vv-r-7)", borderTopRightRadius: "var(--vv-r-7)", borderBottomRightRadius: "var(--vv-r-7)", borderBottomLeftRadius: "var(--vv-r-7)", fontSize: libraryVars.bodySmallFontSize },
  avatarAvatarSm: { display: "inline-flex", alignItems: "center", justifyContent: "center", flexGrow: 0, flexShrink: 0, flexBasis: "auto", width: "var(--vv-s-22)", height: "var(--vv-s-22)", borderTopLeftRadius: "50%", borderTopRightRadius: "50%", borderBottomRightRadius: "50%", borderBottomLeftRadius: "50%", backgroundColor: color["--color-surface-6"], backgroundImage: "none", color: color["--color-fg-2"], fontSize: "var(--vv-s-9)", fontWeight: libraryVars.headingWeight },
  userMain: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-1)", columnGap: "var(--vv-s-1)", minWidth: 0, flexGrow: libraryVars.fillRatio, flexShrink: libraryVars.fillRatio, flexBasis: "auto" },
  userName: { color: color["--color-fg"] },
  hint2: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: libraryVars.captionFontSize, color: color["--color-fg-4"], lineHeight: libraryVars.bodyLineHeight },
  vtUserActionButton: { minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" } },
  chips: { display: "flex", flexWrap: "wrap", rowGap: "var(--vv-s-5)", columnGap: "var(--vv-s-5)" },
  chip: { paddingTop: "var(--vv-s-2)", paddingRight: "var(--vv-s-7)", paddingBottom: "var(--vv-s-2)", paddingLeft: "var(--vv-s-7)", borderTopLeftRadius: "var(--vv-r-pill)", borderTopRightRadius: "var(--vv-r-pill)", borderBottomRightRadius: "var(--vv-r-pill)", borderBottomLeftRadius: "var(--vv-r-pill)", borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: line["--line-2"], borderRightWidth: "var(--vv-s-1)", borderRightStyle: "solid", borderRightColor: line["--line-2"], borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: line["--line-2"], borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "solid", borderLeftColor: line["--line-2"], fontSize: libraryVars.captionFontSize, color: color["--color-fg-3"], whiteSpace: "nowrap" },
  row: { display: "flex", alignItems: "center", rowGap: "var(--vv-s-10)", columnGap: "var(--vv-s-10)", flexWrap: "wrap" },
  cardDanger: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-10)", columnGap: "var(--vv-s-10)", paddingTop: "var(--vv-s-14)", paddingRight: "var(--vv-s-14)", paddingBottom: "var(--vv-s-14)", paddingLeft: "var(--vv-s-14)", borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: "var(--vv-err-18)", borderRightWidth: "var(--vv-s-1)", borderRightStyle: "solid", borderRightColor: "var(--vv-err-18)", borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: "var(--vv-err-18)", borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "solid", borderLeftColor: "var(--vv-err-18)", borderTopLeftRadius: "var(--vv-r-12)", borderTopRightRadius: "var(--vv-r-12)", borderBottomRightRadius: "var(--vv-r-12)", borderBottomLeftRadius: "var(--vv-r-12)", backgroundColor: color["--color-card"], backgroundImage: "none", minWidth: 0 },
})
