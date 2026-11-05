import * as stylex from '@stylexjs/stylex'
import { color, line } from '@noverse/ui/tokens.stylex'
import { libraryVars } from './libraryTokens.stylex'
// “Đặt làm mặc định cho kênh…” — chọn kênh rồi mở tab Template của kênh; việc
// đặt mặc định thuộc Kênh (PATCH defaultTemplateId), không ghi từ Thư viện.
import { Button, Dialog } from '@noverse/ui'
import { t } from '../../i18n/library'
import { navigate, paths } from '../../router'
import type { OwnerChannelRef } from '../../api/library'
import { ChannelAvatar } from '../../components'
import { channelTint } from '../../lib/channelTint'

export function AttachChannelDialog({ channels, onClose, inLibrary }: {
  channels: OwnerChannelRef[]
  inLibrary: boolean
  onClose: () => void
}) {
  const copy = t.library.templates
  return (
    <Dialog title={copy.attachTitle} onClose={onClose} width={420}
      footer={<Button size="sm" onClick={onClose}>{t.library.close}</Button>}>
      <p className={`vlb-dialog-text ${stylex.props(inLibrary && sx.dialogText).className}`}>{channels.length ? copy.attachBody : copy.attachEmpty}</p>
      {channels.length > 0 && (
        <div className={`vlb-channel-list ${stylex.props(inLibrary && sx.channelList).className}`}>
          {channels.map((channel) => (
            <button
              key={channel.id}
              type="button"
              className={`vlb-channel-pick ${stylex.props(inLibrary && sx.channelPick).className}`}
              onClick={() => { onClose(); navigate(paths.channel(channel.id, 'templates')) }}
            >
              <ChannelAvatar
                name={channel.name}
                src={channel.avatarUrl ?? undefined}
                color={{ bg: channelTint(channel.id), ring: channelTint(channel.id) }}
              />
              <span className={`vlb-channel-name ${stylex.props(inLibrary && sx.channelName).className}`}>{copy.attachGo(channel.name)}</span>
              <span className={`vlb-channel-arrow ${stylex.props(inLibrary && sx.channelArrow).className}`} aria-hidden="true">›</span>
            </button>
          ))}
        </div>
      )}
    </Dialog>
  )
}

const sx = stylex.create({
  dialogText: { marginTop: 0, marginRight: 0, marginBottom: "var(--vv-s-12)", marginLeft: 0, fontSize: libraryVars.bodySmallFontSize, color: color["--color-fg-3"], lineHeight: libraryVars.helpLineHeight },
  channelList: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-4)", columnGap: "var(--vv-s-4)" },
  channelPick: { display: "flex", alignItems: "center", rowGap: "var(--vv-s-10)", columnGap: "var(--vv-s-10)", width: libraryVars.fullExtent, paddingTop: "var(--vv-s-8)", paddingRight: "var(--vv-s-10)", paddingBottom: "var(--vv-s-8)", paddingLeft: "var(--vv-s-10)", borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: { default: line["--line-2"], ":hover": line["--line-4"] }, borderRightWidth: "var(--vv-s-1)", borderRightStyle: "solid", borderRightColor: { default: line["--line-2"], ":hover": line["--line-4"] }, borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: { default: line["--line-2"], ":hover": line["--line-4"] }, borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "solid", borderLeftColor: { default: line["--line-2"], ":hover": line["--line-4"] }, borderTopLeftRadius: "var(--vv-r-9)", borderTopRightRadius: "var(--vv-r-9)", borderBottomRightRadius: "var(--vv-r-9)", borderBottomLeftRadius: "var(--vv-r-9)", backgroundColor: { default: "transparent", ":hover": "var(--vv-hover-row)" }, backgroundImage: { default: "none", ":hover": "none" }, color: color["--color-fg"], fontSize: "var(--vv-s-13)", textAlign: "left", cursor: "pointer" },
  channelName: { flexGrow: libraryVars.fillRatio, flexShrink: libraryVars.fillRatio, flexBasis: "auto", minWidth: 0, overflowX: "hidden", overflowY: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" },
  channelArrow: { color: color["--color-fg-4"] },
})
