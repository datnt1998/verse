// Nhãn hiển thị của khung app: tiêu đề tab trình duyệt theo màn đang mở.
// Người dùng chỉ thấy tên màn và tên đã đặt (kênh, template); mã kênh/template
// và tham số query không bao giờ vào tiêu đề.
import { t } from '../i18n/vi'
import type { Route } from '../router'

/** Tên đang có trong bộ nhớ đệm cho màn đang mở; `null` = chưa tải xong. */
export type ScreenNames = {
  /** tên kênh của route `channel` (theo `route.channelId`) */
  channel?: string | null
  /** tên template của route `template` (theo `route.templateId`) */
  template?: string | null
  /** tên project của route `editor` (Editor tự đặt tiêu đề, cùng hàm này) */
  project?: string | null
}

/** "Kênh · Viderse" — mọi tiêu đề đều kết bằng tên thương hiệu. */
const title = (...parts: (string | null | undefined)[]) =>
  [...parts.map((part) => part?.trim()).filter((part): part is string => !!part), t.brand.name].join(' · ')

/**
 * Tiêu đề tab trình duyệt của một route. Màn chưa có tên riêng dùng nhãn màn
 * ("Kênh", "Template"); thiếu dữ liệu thì dừng ở nhãn màn, không lấy mã thay tên.
 */
export function documentTitle(route: Route, names: ScreenNames = {}): string {
  switch (route.name) {
    case 'workbench':
      return title(t.screens.workbench.title)
    case 'review':
      return title(t.screens.review.title)
    case 'queue':
      return title(t.screens.queue.title)
    case 'channel':
      return title(names.channel ? `${t.channels.page.title} ${names.channel}` : t.channels.page.title)
    case 'channel-new':
      return title(t.channels.create.title)
    case 'explore':
      return title(t.screens.explore.title)
    case 'new':
      return title(t.screens.new.title)
    case 'library':
      return title(t.library.titleShort)
    case 'template':
      return title(names.template ? `${t.library.tabs.templates} ${names.template}` : t.library.tabs.templates)
    case 'editor':
      return title(names.project, t.screens.workbench.title)
    case 'settings':
      return title(t.screens.settings.title)
    case 'setup':
      return title(t.prep.titleShort)
    case 'account':
      return title(t.account.title)
    default:
      return `${t.brand.name} · ${t.brand.tagline}`
  }
}
