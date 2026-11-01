// Dựng các dòng của khối "Đang áp dụng cho kênh": mỗi dòng là một giá
// trị thật của kênh, nhãn "riêng" chỉ khi khác mặc định của kênh mới, và một
// lối "Tuỳ chỉnh" mở đúng tab. Hàm thuần để test; TSX chỉ kết xuất.
import type { Channel } from '../../api/channels'
import { t } from '../../i18n/channels'
import { paths, type ChannelTab } from '../../router'

export type AppliedRow = {
  key: 'auto' | 'templates' | 'titles' | 'ai'
  label: string
  value: string
  /** khác mặc định của kênh mới → hiện nhãn "riêng" */
  own: boolean
  linkLabel: string
  href: string
}

/** Tên kênh mới coi Mẫu chuẩn là mặc định: đúng 1 template tên bắt đầu "Mẫu chuẩn". */
export const isDefaultTemplateSet = (channel: Channel): boolean =>
  channel.templates.length === 1 && channel.templates[0].name.startsWith('Mẫu chuẩn')

export function appliedRows(channel: Channel): AppliedRow[] {
  const tt = t.channels.applied
  const next = channel.templates.find((template) => template.position === channel.templateCursor)
    ?? channel.templates[0]
  const customize = (tab: ChannelTab) => ({ linkLabel: tt.customize, href: `#${paths.channel(channel.id, tab)}` })

  const titleParts = [
    channel.styleNote ? tt.partTone : '',
    channel.descriptionFooter ? tt.partFooter : '',
    channel.alwaysTags.length ? tt.partTags(channel.alwaysTags.length) : '',
  ].filter(Boolean)

  return [
    {
      key: 'auto',
      label: tt.autoTitle,
      value: tt.auto(channel.autoPick ? tt.on : tt.off, channel.pacePerDay),
      own: !channel.autoPick || channel.pacePerDay !== 5,
      ...customize('sources'),
    },
    {
      key: 'templates',
      label: tt.templatesTitle,
      value: tt.templates(channel.templates.length, next?.name),
      own: !isDefaultTemplateSet(channel),
      ...customize('templates'),
    },
    {
      key: 'titles',
      label: tt.titlesTitle,
      value: `${t.channels.page.language[channel.language]} · ${titleParts.join(', ') || tt.titlesDefault}`,
      own: titleParts.length > 0,
      ...customize('settings'),
    },
    {
      key: 'ai',
      label: tt.aiTitle,
      value: tt.aiShared,
      own: false,
      linkLabel: tt.settingsLink,
      href: `#${paths.settings('agents')}`,
    },
  ]
}
