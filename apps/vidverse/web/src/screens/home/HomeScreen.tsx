// Trang chủ — trạng thái trống thật: Vidverse mới cài chưa có nội dung nào.
import { t } from '../../i18n/vi'
import { EmptyState } from '@noverse/ui'

export function HomeScreen() {
  return (
    <div style={{ padding: '24px 20px', display: 'flex', flexDirection: 'column', gap: 20 }}>
      <h1 className="sr-only">{t.home.title}</h1>
      <EmptyState
        variant="frame"
        icon="camera-bulk"
        title={t.home.emptyTitle}
        hint={t.home.emptyHint}
        style={{ minHeight: '50vh' }}
      />
    </div>
  )
}
