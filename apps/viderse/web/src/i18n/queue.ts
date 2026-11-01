// Chữ của màn Hàng chờ + sheet Việc nền của điện thoại (chỉ chunk Hàng chờ nạp).
import { base } from './vi'
import { screenQueue } from './screens'

export const t = { ...base, screens: { ...base.screens, queue: screenQueue } } as const
