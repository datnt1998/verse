// Hook tiêu đề tab: đọc tên kênh/template từ bộ nhớ đệm dùng chung (cùng khoá
// với Bộ đổi kênh và trang template) nên không thêm request. Chưa có tên thì
// tiêu đề dừng ở nhãn màn — không lấy mã kênh/template thay tên.
import { useEffect } from 'react'
import { listChannels } from '../api/channels'
import { getTemplate } from '../api/library'
import { keys } from '../data/keys'
import { useResource } from '../data/useResource'
import type { Route } from '../router'
import { documentTitle } from './labels'

/** Tên kênh của route `channel`; null khi chưa tải xong hoặc kênh chưa đặt tên. */
export function useChannelName(id: string | null): string | null {
  const channels = useResource(
    keys.channels(),
    ({ signal }) => listChannels(signal).catch(() => ({ channels: [] })),
    { enabled: id !== null },
  )
  const name = (id ? channels.data?.channels.find((channel) => channel.id === id)?.name : '')?.trim()
  return name ? name : null
}

/** Tên template của route `template`; null khi chưa tải xong. */
export function useTemplateName(id: string | null): string | null {
  const template = useResource(
    keys.template(id ?? ''),
    ({ signal }) => getTemplate(id ?? '', signal),
    { enabled: id !== null },
  )
  const name = template.data?.name.trim()
  return name ? name : null
}

/** Đặt tiêu đề tab theo màn đang mở — gọi một lần ở khung app (cả desktop lẫn điện thoại). */
export function useDocumentTitle(route: Route): void {
  const channel = useChannelName(route.name === 'channel' ? route.channelId : null)
  const template = useTemplateName(route.name === 'template' ? route.templateId : null)
  const title = documentTitle(route, { channel, template })
  useEffect(() => { document.title = title }, [title])
}
