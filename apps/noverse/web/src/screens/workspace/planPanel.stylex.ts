// “Kế hoạch truyện” (21-dev-prompt.md › Gói P · P1 + Main.dc.html › phase=plan) — StyleX cho
// khung kế hoạch thay khung “Luồng viết trực tiếp” ở giai đoạn lập kế hoạch.
// Màu chủ đạo = Kiến trúc sư (--color-info · tint-info). Bài giải ở chân danh sách là
// <p> trong vùng cuộn (không viền trên), đúng như artboard.
import * as stylex from '@stylexjs/stylex'
import { brain, color, line } from '@noverse/ui/tokens.stylex'

export const s = stylex.create({
  // Hàng bước: icon · tên · tóm tắt · thời lượng · mũi tên (Main.dc.html: padding 9×12).
  step: {
    width: '100%',
    display: 'flex',
    alignItems: 'center',
    gap: 10,
    paddingBlock: 9,
    paddingInline: 12,
    borderWidth: 0,
    borderRadius: 12,
    backgroundColor: 'transparent',
    color: color['--color-fg'],
    textAlign: 'left',
    cursor: 'pointer',
  },
  stepName: {
    flexShrink: 0,
    fontSize: 13,
    fontWeight: 500,
    color: color['--color-fg'],
  },
  // Tóm tắt nằm ngay sau tên (trái), không đẩy sang phải; màu #8E8E99 như artboard.
  stepSummary: {
    flex: 1,
    minWidth: 0,
    fontSize: 12.5,
    color: brain['--brain-muted'],
    whiteSpace: 'nowrap',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    textAlign: 'left',
  },
  stepDuration: {
    flexShrink: 0,
    fontSize: 12,
    fontStyle: 'italic',
    color: color['--color-fg-4'],
  },
  // Thân mở rộng của một bước: thụt 37 px bằng icon + khoảng cách của hàng.
  body: {
    paddingTop: 0,
    paddingBottom: 12,
    paddingLeft: 37,
    paddingRight: 12,
    display: 'flex',
    flexDirection: 'column',
    fontSize: 13,
    color: color['--color-fg-2'],
  },
  // Thông tin truyện: dải chip khoá–giá trị (Main.dc.html › p.isKv).
  kvRow: {
    display: 'flex',
    flexWrap: 'wrap',
    gap: 6,
  },
  kvChip: {
    paddingBlock: 3,
    paddingInline: 8,
    borderRadius: 6,
    backgroundColor: color['--color-surface-0'],
    borderWidth: 1,
    borderStyle: 'solid',
    borderColor: line['--line-2'],
    fontSize: 12,
    color: color['--color-fg-4'],
    maxWidth: '100%',
    whiteSpace: 'nowrap',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
  },
  kvKey: {
    flexShrink: 0,
    fontSize: 12,
    color: color['--color-fg-4'],
  },
  kvValue: {
    fontWeight: 500,
    color: color['--color-fg'],
  },
  // Tiền đề: một đoạn văn.
  premise: {
    marginTop: 0,
    marginBottom: 0,
    fontSize: 13,
    lineHeight: 1.6,
    color: color['--color-fg-2'],
    whiteSpace: 'pre-wrap',
  },
  // Nhân vật: chip bấm mở Não truyện, chữ tắt trong vòng tròn.
  chipRow: {
    display: 'flex',
    flexWrap: 'wrap',
    gap: 6,
  },
  charChip: {
    display: 'inline-flex',
    alignItems: 'center',
    gap: 6,
    height: 28,
    paddingLeft: 4,
    paddingRight: 10,
    borderRadius: 999,
    borderWidth: 1,
    borderStyle: 'solid',
    borderColor: line['--line-3'],
    backgroundColor: color['--color-surface-0'],
    color: color['--color-fg'],
    fontSize: 12.5,
    cursor: 'pointer',
    maxWidth: '100%',
    whiteSpace: 'nowrap',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
  },
  charRole: {
    fontSize: 12,
    color: color['--color-fg-4'],
  },
  // Quy tắc thế giới: danh sách chấm đầu dòng (artboard dùng <ul> mặc định).
  ruleList: {
    display: 'flex',
    flexDirection: 'column',
    gap: 4,
    marginTop: 0,
    marginBottom: 0,
    paddingLeft: 16,
    fontSize: 13,
    color: color['--color-fg-2'],
    lineHeight: 1.55,
    listStyleType: 'disc',
  },
  // Dàn ý: 8 dòng, 2 cột dọc (artboard grid-auto-flow: column, 4 hàng).
  outlineGrid: {
    display: 'grid',
    gridTemplateColumns: 'repeat(2, minmax(0, 1fr))',
    gridTemplateRows: 'repeat(4, auto)',
    gridAutoFlow: 'column',
    columnGap: 18,
    rowGap: 0,
    marginTop: 0,
    marginBottom: 0,
    paddingLeft: 0,
    listStyleType: 'none',
  },
  outlineCell: {
    display: 'grid',
    gridTemplateColumns: '22px minmax(0, 1fr)',
    gap: 8,
    alignItems: 'baseline',
    paddingBlock: 4,
    minWidth: 0,
    fontSize: 13,
  },
  outlineNum: {
    fontSize: 12,
    color: color['--color-fg-4'],
  },
  outlineTitle: {
    minWidth: 0,
    fontSize: 13,
    color: color['--color-fg'],
    whiteSpace: 'nowrap',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
  },
  outlineHook: {
    color: brain['--brain-muted'],
  },
  /** Chân khung: lời nhắc trong vùng cuộn, không có đường kẻ trên. */
  footer: {
    marginTop: 8,
    marginBottom: 0,
    marginLeft: 2,
    marginRight: 2,
    fontSize: 12,
    color: color['--color-fg-4'],
  },
  // Badge trạng thái đang chạy của panel (màu Kiến trúc sư).
  liveBadge: {
    display: 'inline-flex',
    alignItems: 'center',
    gap: 6,
    fontSize: 12,
    color: color['--color-info'],
  },
})
