import { Segmented as KitSegmented } from '@noverse/ui'
import type { ComponentProps } from 'react'

// Nhóm chọn của Viderse: 04-components › SegmentedTabs (đợt 34, #80) chốt mục
// CO THEO NHÃN, không bao giờ cắt “…” — nhóm hẹp hơn nội dung thì cuộn ngang.
// Kit dùng chung vẫn mặc định chia đều cho Noverse, nên mặc định của Viderse
// đặt ở đây; nhóm cần chia đều thật (nhãn ngắn ngang nhau, chiếm hết bề rộng)
// truyền `fit="fill"` — đúng `.vv-seg--fill` của kit.
export function Segmented(props: ComponentProps<typeof KitSegmented>) {
  return <KitSegmented fit="content" {...props} />
}
