// “Thêm vào kênh…” — chọn kênh rồi mở tab Template của kênh (việc gắn template
// thuộc danh sách luân phiên của kênh, không sửa từ Thư viện).
import { Button, Dialog } from '@noverse/ui'
import { vi as t } from '../../i18n/vi'
import { navigate, paths } from '../../router'
import type { OwnerChannelRef } from '../../api/library'
import { ChannelAvatar } from '../../components'
import { channelTint } from '../../lib/channelTint'

export function AttachChannelDialog({ channels, onClose }: {
  channels: OwnerChannelRef[]
  onClose: () => void
}) {
  const copy = t.library.templates
  return (
    <Dialog title={copy.attachTitle} onClose={onClose} width={420}
      footer={<Button size="sm" onClick={onClose}>{t.library.close}</Button>}>
      <p className="vlb-dialog-text">{channels.length ? copy.attachBody : copy.attachEmpty}</p>
      {channels.length > 0 && (
        <div className="vlb-channel-list">
          {channels.map((channel) => (
            <button
              key={channel.id}
              type="button"
              className="vlb-channel-pick"
              onClick={() => { onClose(); navigate(paths.channel(channel.id, 'templates')) }}
            >
              <ChannelAvatar
                name={channel.name}
                src={channel.avatarUrl ?? undefined}
                color={{ bg: channelTint(channel.id), ring: channelTint(channel.id) }}
              />
              <span className="vlb-channel-name">{copy.attachGo(channel.name)}</span>
              <span className="vlb-channel-arrow" aria-hidden="true">›</span>
            </button>
          ))}
        </div>
      )}
    </Dialog>
  )
}
