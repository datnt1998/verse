// Lối vào đúng lúc của Chất lượng truyện (22-brief-2510.md › D13) ở tab Thẩm duyệt của Trình đọc:
// “Xem chất lượng cả truyện · N việc cần xem” (Reader.dc.html), N = Đang chặn chương + Cần bạn xem.
import { t } from '../../i18n/vi'
import { paths, useRoute } from '../../router'
import { Icon } from '@noverse/ui'
import { useQuality } from './data'
import { todoCount } from './model'

export function ReaderQualityLink() {
  const { route } = useRoute()
  const target = route.name === 'reader' ? route : null
  const quality = useQuality(target?.project ?? '', target?.story ?? '', target !== null)
  if (!target || !quality.data) return null
  return (
    <a href={`#${paths.quality(target.project, target.story)}`} style={{
      alignSelf: 'flex-start', display: 'flex', alignItems: 'center', gap: 6, fontSize: 12.5, color: 'var(--color-fg-3)',
    }}>
      <Icon name="category" size={13} />{t.quality.readerLink(todoCount(quality.data))}
    </a>
  )
}
