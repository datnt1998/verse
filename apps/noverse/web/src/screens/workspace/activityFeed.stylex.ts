// ActivityFeed (panel Nhật ký tiến độ trong Bàn viết) — StyleX.
import * as stylex from '@stylexjs/stylex'

export const s = stylex.create({
  // Gói O mục 2: mờ 16 px mép trên vùng cuộn — dòng đầu không nằm hẳn dưới tiêu đề.
  logFade: {
    WebkitMaskImage: 'linear-gradient(to bottom, transparent 0, black 16px)',
    maskImage: 'linear-gradient(to bottom, transparent 0, black 16px)',
  },
})
