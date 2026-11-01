// Chữ của màn Khám phá (chỉ chunk Khám phá nạp): chữ khung app + khối
// `screens.explore`. Nhãn của màn khác mà Khám phá có đọc (tên kênh trống, toast
// tạo job) nằm trong ./base nên không kéo cả chữ màn đó sang.
import { base } from './vi'
import { screenExplore } from './screens'

export const t = { ...base, screens: { ...base.screens, explore: screenExplore } } as const
