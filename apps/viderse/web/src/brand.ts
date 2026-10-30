// Brand Viderse theo design 02 §2: nền tối của kit, tím Noverse → đỏ Viderse,
// đỏ/hồng lỗi Noverse → cam Viderse. Nút chính là nút sáng; đỏ không làm nền nút.
import { applyBrand, defineBrand } from '@noverse/ui'

export const brand = defineBrand({
  name: 'viderse',
  primary: '#FF4D63', // đang chọn, focus, dấu tick, icon tab đang mở
  status: {
    success: '#A6EDC1', // Sẵn sàng, Xong, Đã kết nối
    warning: '#F6CB8C', // tới lượt bạn (Chờ duyệt, Chờ bạn)
    danger: '#FF9A5C', // lỗi và xoá (cam)
    info: '#B9D0F7', // banner thông tin, Đang làm
  },
  roles: { accentDeep: '#E6002E' }, // công tắc bật; nút + tròn ở thanh dưới
  // Thang nguy hiểm của kit: đỏ/hồng Noverse → cam Viderse (design 02 §2).
  // Nền mờ/border/chữ lấy đúng artboard; nút đặc cam sáng đi với mực tối.
  danger: {
    soft: '#FF9A5C', border: '#FF9A5C', ink: '#FFD2B5', solid: '#FF9A5C', solidInk: '#1A0E06',
    ghostInk: '#FFC09A', // chữ `.ghost.btn-warn` của artboard
  },
  // Rãnh công tắc của kit là literal #9A63E3 — ghim thẳng token để đúng #E6002E của design.
  overrides: { '--kit-switch-on': '#E6002E' },
})

export function applyViderseBrand() {
  applyBrand(brand)
}
