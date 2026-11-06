import { useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { color, line } from '@noverse/ui/tokens.stylex'
import { libraryVars } from './libraryTokens.stylex'
// “Đặt làm mặc định cho kênh…” — chọn kênh rồi mở tab Template của kênh; việc
// đặt mặc định thuộc Kênh (PATCH defaultTemplateId), không ghi từ Thư viện.
import { Button, Dialog, Sheet, PHONE_MAX_VIDERSE, phoneQuery } from '@noverse/ui'
import { t } from '../../i18n/library'
import { navigate, paths } from '../../router'
import { listOwnerChannels } from '../../api/library'
import { keys } from '../../data/keys'
import { useResource } from '../../data/useResource'
import { useMediaQuery } from '../../lib/useMediaQuery'
import { ChannelAvatar } from '../../components'
import { channelTint } from '../../lib/channelTint'

export function AttachChannelDialog({ onClose, inLibrary }: {
  inLibrary: boolean
  onClose: () => void
}) {
  const copy = t.library.templates
  const catalog = useResource(keys.ownerChannels(), ({ signal }) => listOwnerChannels(signal))
  const channels = catalog.data?.channels ?? []
  const mobile = useMediaQuery(phoneQuery(PHONE_MAX_VIDERSE))
  const [phone] = useState(mobile)
  const [open, setOpen] = useState(true)
  const [pickedChannel, setPickedChannel] = useState<string | null>(null)
  const close = () => { if (phone) setOpen(false); else onClose() }
  const choose = (id: string) => {
    if (phone) {
      setPickedChannel(id)
      setOpen(false)
      return
    }
    onClose()
    navigate(paths.channel(id, 'templates'))
  }
  const exited = () => {
    onClose()
    if (pickedChannel) navigate(paths.channel(pickedChannel, 'templates'))
  }
  const body = <>
      <p className={`vlb-dialog-text ${stylex.props(inLibrary && sx.dialogText).className}`} role={catalog.error ? 'alert' : catalog.initialLoading ? 'status' : undefined}>
        {catalog.initialLoading ? copy.attachLoading : catalog.error ? copy.attachFailed : channels.length ? copy.attachBody : copy.attachEmpty}
      </p>
      {catalog.error && <Button onClick={() => void catalog.refresh()}>{t.library.retry}</Button>}
      {channels.length > 0 && (
        <div className={`vlb-channel-list ${stylex.props(inLibrary && sx.channelList).className}`}>
          {channels.map((channel) => (
            <button
              key={channel.id}
              type="button"
              className={`vlb-channel-pick ${stylex.props(inLibrary && sx.channelPick).className}`}
              onClick={() => choose(channel.id)}
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
    </>
  if (phone) return <Sheet open={open} onClose={close} onExited={exited} title={copy.attachTitle} height="auto" size="md" homeIndicator bodyStyle={sx.sheetBody}>
    {body}
    <Button xstyle={sx.closeButton} onClick={close}>{t.library.close}</Button>
  </Sheet>
  return <Dialog title={copy.attachTitle} onClose={close} width={420}
    footer={<Button size="sm" onClick={close}>{t.library.close}</Button>}>{body}</Dialog>
}

const sx = stylex.create({
  dialogText: { marginTop: 0, marginRight: 0, marginBottom: "var(--vv-s-12)", marginLeft: 0, fontSize: libraryVars.bodySmallFontSize, color: color["--color-fg-3"], lineHeight: libraryVars.helpLineHeight },
  channelList: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-4)", columnGap: "var(--vv-s-4)" },
  sheetBody: { paddingInline: 'var(--vv-s-16)', paddingBottom: 'var(--vv-s-16)' },
  closeButton: { minHeight: 'var(--vv-s-44)', marginTop: 'var(--vv-s-12)' },
  channelPick: { display: "flex", alignItems: "center", rowGap: "var(--vv-s-10)", columnGap: "var(--vv-s-10)", width: libraryVars.fullExtent, paddingTop: "var(--vv-s-8)", paddingRight: "var(--vv-s-10)", paddingBottom: "var(--vv-s-8)", paddingLeft: "var(--vv-s-10)", borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: { default: line["--line-2"], ":hover": line["--line-4"] }, borderRightWidth: "var(--vv-s-1)", borderRightStyle: "solid", borderRightColor: { default: line["--line-2"], ":hover": line["--line-4"] }, borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: { default: line["--line-2"], ":hover": line["--line-4"] }, borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "solid", borderLeftColor: { default: line["--line-2"], ":hover": line["--line-4"] }, borderTopLeftRadius: "var(--vv-r-9)", borderTopRightRadius: "var(--vv-r-9)", borderBottomRightRadius: "var(--vv-r-9)", borderBottomLeftRadius: "var(--vv-r-9)", backgroundColor: { default: "transparent", ":hover": "var(--vv-hover-row)" }, backgroundImage: { default: "none", ":hover": "none" }, color: color["--color-fg"], fontSize: "var(--vv-s-13)", textAlign: "left", cursor: "pointer" },
  channelName: { flexGrow: libraryVars.fillRatio, flexShrink: libraryVars.fillRatio, flexBasis: "auto", minWidth: 0, overflowX: "hidden", overflowY: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" },
  channelArrow: { color: color["--color-fg-4"] },
})
