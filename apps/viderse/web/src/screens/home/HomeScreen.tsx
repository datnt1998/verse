// Bàn dựng — '#/' (ia-flows §5 S2). Chưa có dữ liệu → trạng thái trống của
// design (cụm 4 orb + lời mời + CTA Làm video); ?video=<id> mở hàng chi tiết —
// id lạ → trạng thái Không tìm thấy. Nội dung thật dựng ở gói màn hình sau.
import type { Route } from '../../router'
import { EmptyWorkbench, SystemNotFound } from '../../components'

export function HomeScreen({ route }: { route: Extract<Route, { name: 'workbench' }> }) {
  if (route.video) return <SystemNotFound />
  return <EmptyWorkbench />
}
