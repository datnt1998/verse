import * as stylex from '@stylexjs/stylex'
import { color, line } from '@noverse/ui/tokens.stylex'
import { libraryVars } from '../libraryTokens.stylex'
// Mục Xuất & kênh (Template.dc.html › lib=output): nén sau xuất, kênh dùng mẫu
// video này làm mặc định (Đổi ở Kênh mở tab Template của kênh — không gỡ từ
// đây), bố cục (mở trong editor), xuất tệp .ytf-template.zip và xoá mẫu video —
// xoá bị khoá kèm lý do nêu đúng kênh đang dùng (409 của máy chủ).
import { confirm, RadioCard, Switch, Icon, toast } from '@noverse/ui'
import { useState } from 'react'
import { ActionButton } from '../../../components'
import { t } from '../../../i18n/library'
import { initials } from '../fmt'
import { downloadTemplate, type Recipe, type TemplateDetail } from '../../../api/library'
import { navigate, paths } from '../../../router'

export function OutputSection({ detail, onSave, onDuplicate, onDelete, onAttach, hideTitle, editorLockedReason }: {
  detail: TemplateDetail
  onSave: (patch: Partial<Recipe>) => void
  onDuplicate: () => void
  onDelete: () => Promise<void>
  onAttach: () => void
  /** Ẩn h2 khi lồng trong màn con có tiêu đề riêng (MTemplate). */
  hideTitle?: boolean
  editorLockedReason?: string
}) {
  const copy = t.library.template.output
  const channels = detail.channels
  const inUse = channels.length > 0
  const compressed = detail.recipe.output.compressExport
  const [exporting, setExporting] = useState(false)
  const exportFile = async () => {
    if (exporting) return
    setExporting(true)
    try { await downloadTemplate(detail.id); toast(t.library.templates.exported, 'ok') }
    catch (error) { toast(error instanceof Error ? error.message : t.library.templates.exportFail, 'err') }
    finally { setExporting(false) }
  }

  const remove = async () => {
    await confirm({
      title: t.library.template.deleteTitle(detail.name),
      body: t.library.template.deleteBody,
      tone: 'danger',
      confirmLabel: t.library.template.deleteAction,
      cancelLabel: t.library.cancel,
      onConfirm: onDelete,
    })
  }

  return (
    <section className={`vt-sec ${stylex.props(sx.sec, hideTitle && sx.mobileSection).className}`} aria-label={hideTitle ? copy.title : undefined} aria-labelledby={hideTitle ? undefined : 'vt-output-title'}>
      <div className={`vt-sec-head ${stylex.props(sx.secHead, hideTitle && sx.mobileIntroHidden).className}`}>
        {!hideTitle && <h2 {...stylex.props(sx.vtSecHeadH2)} id="vt-output-title">{copy.title}</h2>}
        <p className={`vt-help ${stylex.props(sx.help).className}`}>{copy.help}</p>
      </div>

      <div className={`vt-card ${stylex.props(sx.card).className}`}>
        <div className={`vt-switch-row ${stylex.props(sx.switchRow).className}`}>
          <span className={`vt-label ${stylex.props(sx.label).className}`}>{copy.compressTitle}</span>
          <Switch xstyle={[sx.vtSwitchRowSwitch]} label={copy.compressTitle} checked={compressed}
            onChange={(on) => onSave({ output: { compressExport: on } })} />
        </div>
        <div role="radiogroup" aria-label={copy.compressTitle} className={`vt-export ${stylex.props(sx.export).className}`}>
          <RadioCard xstyle={[sx.vtSwitchRowSwitch, sx.exportChoice]} dotStyle={sx.exportDot} checked={compressed} onChange={() => onSave({ output: { compressExport: true } })}
            label={<span className={`vt-export-card ${stylex.props(sx.exportCard).className}`}>
              <span className={`vt-export-name ${stylex.props(sx.exportName).className}`}>{copy.compressed.title}</span>
              <span className={`vt-export-size num ${stylex.props(sx.exportSizeNum).className}`}>{copy.compressed.size}</span>
              <span className={`vt-hint ${stylex.props(sx.hint).className}`}>{copy.compressed.note}</span>
            </span>} />
          <RadioCard xstyle={[sx.vtSwitchRowSwitch, sx.exportChoice]} dotStyle={sx.exportDot} checked={!compressed} onChange={() => onSave({ output: { compressExport: false } })}
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
          <ActionButton xstyle={[sx.vtSwitchRowSwitch, sx.attachButton]} variant="quiet" icon={<Icon name="add" size={16} />} onClick={onAttach}>{hideTitle ? copy.usersAddMobile : copy.usersAdd}</ActionButton>
        </div>
        {inUse ? (
          <ul className={`vt-users ${stylex.props(sx.users).className}`}>
            {channels.map((channel) => (
              <li key={channel.id} className={`vt-user ${stylex.props(sx.user, hideTitle && sx.mobileUser).className}`}>
                <span className={`vt-avatar vt-avatar--sm ${stylex.props(sx.avatarAvatarSm).className}`} aria-hidden="true">{initials(channel.name)}</span>
                <span className={`vt-user-main ${stylex.props(sx.userMain).className}`}>
                  <span className={`vt-user-name ${stylex.props(sx.userName).className}`}>{channel.name}</span>
                  <span className={`vt-hint ${stylex.props(sx.hint2).className}`}>{copy.usersDefaultRow}</span>
                </span>
                <ActionButton xstyle={[sx.vtUserActionButton]} variant="quiet" onClick={() => navigate(paths.channel(channel.id, 'templates'))}>
                  {copy.usersGo}
                </ActionButton>
              </li>
            ))}
          </ul>
        ) : <p className={`vt-hint ${stylex.props(sx.hint).className}`}>{copy.usersEmpty}</p>}
        <p className={`vt-hint ${stylex.props(sx.hint).className}`}>{copy.usersNote}</p>
      </div>

      <div className={`vt-card ${stylex.props(sx.card).className}`}>
        <div {...stylex.props(sx.cardHead)}>
          <span className={`vt-label ${stylex.props(sx.label).className}`}>{copy.layoutTitle}</span>
          {!hideTitle && <a {...stylex.props(sx.exportLink)} href={editorLockedReason ? undefined : `#${paths.editor(detail.id)}`} role="link" aria-disabled={!!editorLockedReason} title={editorLockedReason} onClick={event => { if (editorLockedReason) { event.preventDefault(); toast(editorLockedReason, 'err') } }}>{copy.layoutEdit}</a>}
        </div>
        <div className={`vt-chips ${stylex.props(sx.chips).className}`}>
          {copy.layoutChips.map((chip) => <span key={chip} className={`vt-chip ${stylex.props(sx.chip).className}`}>{chip}</span>)}
        </div>
        <p className={`vt-hint ${stylex.props(sx.hint).className}`}>{hideTitle ? copy.layoutHelpMobile : copy.layoutHelp}</p>
      </div>

      <div className={`vt-card ${stylex.props(sx.card).className}`}>
        <span className={`vt-label ${stylex.props(sx.label).className}`}>{copy.fileTitle}</span>
        <div className={`vt-row ${stylex.props(sx.row).className}`}>
          <ActionButton xstyle={[sx.exportLink, sx.fileAction]} pending={exporting} disabled={exporting} onClick={() => void exportFile()} icon={<Icon name="export" size={16} />}>{copy.exportFile}</ActionButton>
          <ActionButton xstyle={[sx.vtSwitchRowSwitch, sx.fileAction]} variant="outline" icon={<Icon name="copy" size={16} />} onClick={onDuplicate}>{t.library.duplicate}</ActionButton>
        </div>
        <p className={`vt-hint ${stylex.props(sx.hint).className}`}>{copy.exportHelp}</p>
      </div>

      <div className={`vt-card vt-danger ${stylex.props(sx.cardDanger).className}`}>
        <span className={`vt-label ${stylex.props(sx.label).className}`}>{copy.deleteTitle}</span>
        <div className={`vt-switch-row ${stylex.props(sx.switchRow).className}`}>
          <p className={`vt-hint ${stylex.props(sx.hint).className}`}>{inUse ? copy.deleteReason(channels.map((channel) => channel.name)) : copy.deleteFree}</p>
          <ActionButton xstyle={[sx.vtSwitchRowSwitch]}
            variant="danger-soft"
            icon={<Icon name="trash" size={16} />}
            onClick={inUse ? undefined : () => void remove()}
            lockedReason={inUse ? copy.deleteReason(channels.map((channel) => channel.name)) : undefined}
          >{t.library.delete}</ActionButton>
        </div>
      </div>
    </section>
  )
}

const sx = stylex.create({
  mobileSection: { rowGap: { default: null, '@media (max-width: 767px)': 'var(--vv-s-18)' } },
  mobileIntroHidden: { display: { default: null, '@media (max-width: 767px)': 'none' } },
  exportDot: { display: 'none' },
  exportChoice: {
    minWidth: 0,
    padding: 'var(--vv-s-12) var(--vv-s-14)',
  },
  mobileUser: {
    paddingTop: { default: null, '@media (max-width: 767px)': 'var(--vv-s-8)' },
    paddingRight: { default: null, '@media (max-width: 767px)': 0 },
    paddingBottom: { default: null, '@media (max-width: 767px)': 'var(--vv-s-8)' },
    paddingLeft: { default: null, '@media (max-width: 767px)': 0 },
    borderTopWidth: { default: null, '@media (max-width: 767px)': 0 },
    borderRightWidth: { default: null, '@media (max-width: 767px)': 0 },
    borderLeftWidth: { default: null, '@media (max-width: 767px)': 0 },
    borderTopLeftRadius: { default: null, '@media (max-width: 767px)': 0 },
    borderTopRightRadius: { default: null, '@media (max-width: 767px)': 0 },
    borderBottomLeftRadius: { default: null, '@media (max-width: 767px)': 0 },
    borderBottomRightRadius: { default: null, '@media (max-width: 767px)': 0 },
  },
  sec: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-24)", minWidth: 0 },
  secHead: { display: "flex", flexDirection: 'column', alignItems: 'flex-start', rowGap: "var(--vv-s-4)", paddingBottom: 'var(--vv-s-6)' },
  vtSecHeadH2: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: "var(--vv-s-18)", fontWeight: libraryVars.headingWeight },
  help: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: libraryVars.bodySmallFontSize, color: color["--color-fg-3"], lineHeight: libraryVars.helpLineHeight, maxWidth: libraryVars.templateBackdropSectionHelpMaxWidth },
  card: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-10)", minWidth: 0 },
  switchRow: { display: "flex", alignItems: "center", justifyContent: "space-between", rowGap: "var(--vv-s-12)", columnGap: "var(--vv-s-12)", flexWrap: "wrap" },
  label: {
    fontSize: { default: 'var(--vv-s-14)', '@media (max-width: 767px)': 'var(--vv-s-12)' },
    fontWeight: { default: libraryVars.labelWeight, '@media (max-width: 767px)': 600 },
    color: { default: color['--color-fg'], '@media (max-width: 767px)': 'var(--kit-fg-sub)' },
    letterSpacing: { default: null, '@media (max-width: 767px)': '.06em' },
    textTransform: { default: null, '@media (max-width: 767px)': 'uppercase' },
  },
  vtSwitchRowSwitch: { minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" } },
  exportLink: { display: 'inline-flex', alignItems: 'center', justifyContent: 'center', rowGap: 'var(--vv-s-6)', columnGap: 'var(--vv-s-6)', paddingTop: 'var(--vv-s-8)', paddingRight: 'var(--vv-s-12)', paddingBottom: 'var(--vv-s-8)', paddingLeft: 'var(--vv-s-12)', borderTopWidth: 'var(--vv-s-1)', borderRightWidth: 'var(--vv-s-1)', borderBottomWidth: 'var(--vv-s-1)', borderLeftWidth: 'var(--vv-s-1)', borderTopStyle: 'solid', borderRightStyle: 'solid', borderBottomStyle: 'solid', borderLeftStyle: 'solid', borderTopColor: line['--line-2'], borderRightColor: line['--line-2'], borderBottomColor: line['--line-2'], borderLeftColor: line['--line-2'], borderTopLeftRadius: 'var(--vv-r-8)', borderTopRightRadius: 'var(--vv-r-8)', borderBottomRightRadius: 'var(--vv-r-8)', borderBottomLeftRadius: 'var(--vv-r-8)', color: color['--color-fg'], backgroundColor: { default: 'transparent', ':hover': color['--color-surface-6'] }, textDecorationLine: 'none', fontSize: libraryVars.bodySmallFontSize, minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" } },
  export: { display: "grid", gridTemplateColumns: { default: libraryVars.templateCoverSectionSourcesGridTemplateColumns, '@media (max-width: 767px)': 'repeat(2, minmax(0, 1fr))' }, rowGap: "var(--vv-s-8)", columnGap: "var(--vv-s-8)" },
  exportCard: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-2)", columnGap: "var(--vv-s-2)" },
  exportName: { fontSize: "var(--vv-s-13)", fontWeight: libraryVars.labelWeight, color: color["--color-fg"] },
  exportSizeNum: { fontSize: { default: "var(--vv-s-20)", '@media (max-width: 767px)': 'var(--vv-s-18)' }, fontWeight: libraryVars.headingWeight, color: color["--color-fg"] },
  hint: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: libraryVars.captionFontSize, color: color["--color-fg-4"], lineHeight: libraryVars.bodyLineHeight },
  cardHead: { display: "flex", alignItems: "center", justifyContent: "space-between", gap: "var(--vv-s-6)", minWidth: 0 },
  attachButton: { flexShrink: 0, fontSize: 'var(--vv-s-12)', paddingInline: 'var(--vv-s-8)' },
  fileAction: { flexGrow: { default: 0, '@media (max-width: 767px)': 1 }, flexBasis: { default: 'auto', '@media (max-width: 767px)': 0 }, minHeight: { default: null, '@media (max-width: 767px)': 'var(--vv-s-44)' } },
  users: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, paddingTop: 0, paddingRight: 0, paddingBottom: 0, paddingLeft: 0, listStyle: "none", display: "flex", flexDirection: "column", rowGap: "var(--vv-s-4)", columnGap: "var(--vv-s-4)" },
  user: { display: "flex", alignItems: "center", rowGap: "var(--vv-s-10)", columnGap: "var(--vv-s-10)", paddingTop: "var(--vv-s-5)", paddingRight: "var(--vv-s-8)", paddingBottom: "var(--vv-s-5)", paddingLeft: "var(--vv-s-8)", borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: line["--line-2"], borderRightWidth: "var(--vv-s-1)", borderRightStyle: "solid", borderRightColor: line["--line-2"], borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: line["--line-2"], borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "solid", borderLeftColor: line["--line-2"], borderTopLeftRadius: "var(--vv-r-7)", borderTopRightRadius: "var(--vv-r-7)", borderBottomRightRadius: "var(--vv-r-7)", borderBottomLeftRadius: "var(--vv-r-7)", fontSize: libraryVars.bodySmallFontSize },
  avatarAvatarSm: { display: "inline-flex", alignItems: "center", justifyContent: "center", flexShrink: 0, width: { default: 'var(--vv-s-34)', '@media (max-width: 767px)': 'var(--vv-s-40)' }, height: { default: 'var(--vv-s-34)', '@media (max-width: 767px)': 'var(--vv-s-40)' }, borderRadius: "50%", backgroundColor: color["--color-surface-6"], color: color["--color-fg-2"], fontSize: "var(--vv-s-12)", fontWeight: libraryVars.headingWeight },
  userMain: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-1)", columnGap: "var(--vv-s-1)", minWidth: 0, flexGrow: libraryVars.fillRatio, flexShrink: libraryVars.fillRatio, flexBasis: "auto" },
  userName: { color: color["--color-fg"] },
  hint2: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: libraryVars.captionFontSize, color: color["--color-fg-4"], lineHeight: libraryVars.bodyLineHeight },
  vtUserActionButton: { minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" } },
  chips: { display: "flex", flexWrap: "wrap", rowGap: "var(--vv-s-5)", columnGap: "var(--vv-s-5)" },
  chip: { paddingTop: "var(--vv-s-2)", paddingRight: "var(--vv-s-7)", paddingBottom: "var(--vv-s-2)", paddingLeft: "var(--vv-s-7)", borderTopLeftRadius: "var(--vv-r-pill)", borderTopRightRadius: "var(--vv-r-pill)", borderBottomRightRadius: "var(--vv-r-pill)", borderBottomLeftRadius: "var(--vv-r-pill)", borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: line["--line-2"], borderRightWidth: "var(--vv-s-1)", borderRightStyle: "solid", borderRightColor: line["--line-2"], borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: line["--line-2"], borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "solid", borderLeftColor: line["--line-2"], fontSize: libraryVars.captionFontSize, color: color["--color-fg-3"], whiteSpace: "nowrap" },
  row: { display: "flex", alignItems: "center", rowGap: "var(--vv-s-10)", columnGap: "var(--vv-s-10)", flexWrap: "wrap" },
  cardDanger: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-10)", columnGap: "var(--vv-s-10)", paddingTop: "var(--vv-s-14)", paddingRight: "var(--vv-s-14)", paddingBottom: "var(--vv-s-14)", paddingLeft: "var(--vv-s-14)", borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: "var(--vv-err-18)", borderRightWidth: "var(--vv-s-1)", borderRightStyle: "solid", borderRightColor: "var(--vv-err-18)", borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: "var(--vv-err-18)", borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "solid", borderLeftColor: "var(--vv-err-18)", borderTopLeftRadius: "var(--vv-r-12)", borderTopRightRadius: "var(--vv-r-12)", borderBottomRightRadius: "var(--vv-r-12)", borderBottomLeftRadius: "var(--vv-r-12)", backgroundColor: color["--color-card"], backgroundImage: "none", minWidth: 0 },
})
