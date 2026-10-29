import * as stylex from '@stylexjs/stylex'
import { brain, color, line, kit, surface, font, frame, projbrain } from '@noverse/ui/tokens.stylex'

export const styles = stylex.create({
  root: { flex: 1, minWidth: 0, minHeight: 0, padding: '18px 24px 24px', display: 'flex', flexDirection: 'column', gap: 14, overflowY: 'auto', lineHeight: 1.5 },
  panel: { display: 'flex', flexDirection: 'column', gap: 14, minWidth: 0, flexShrink: 0 },
  heading: { display: 'flex', alignItems: 'flex-end', gap: 16 },
  intro: { flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', gap: 4 },
  /** Màu đặt inline ở <a>: quy tắc `a { color }` của base.css thắng lớp StyleX (như SysBrain). */
  back: { display: 'flex', alignItems: 'center', gap: 6, fontSize: 12.5, color: kit['--kit-fg-icon'] },
  title: { margin: 0, fontSize: 22, fontWeight: 600, letterSpacing: '-.01em' },
  sub: { fontSize: 13, color: color['--color-fg-3'] },
  card: { backgroundImage: surface['--card-bg'], borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-2'], boxShadow: surface['--card-shadow'], borderRadius: 16, flexShrink: 0, overflow: 'hidden' },
  padded: { padding: '14px 16px', display: 'flex', flexDirection: 'column', gap: 10 },
  blocked: { borderColor: kit['--kit-errf-30'] },
  passed: { borderColor: projbrain['--projbrain-ok-25'] },
  /** Thẻ phát hiện ở cột phải Vũ trụ chung (artboard viền vàng 28%). */
  recapCard: { borderColor: kit['--kit-warn-28'] },
  /** Hai thẻ cột phải Vũ trụ chung: padding 14/16, cách 8 (artboard), khác thẻ kiểm trùng (10). */
  universeCard: { padding: '14px 16px', display: 'flex', flexDirection: 'column', gap: 8 },
  recapTitle: { fontSize: 13.5 },
  row: { display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap' },
  grow: { flex: 1, minWidth: 0 },
  strong: { fontSize: 14, fontWeight: 600 },
  note: { fontSize: 12, color: kit['--kit-fg-icon'] },
  chip: { height: 20, padding: '0 7px', borderRadius: 6, display: 'inline-flex', alignItems: 'center', fontSize: 11.5, fontWeight: 500, whiteSpace: 'nowrap', flexShrink: 0, maxWidth: '100%', overflow: 'hidden', textOverflow: 'ellipsis' },
  draftChip: { backgroundColor: kit['--kit-acc-12'], color: kit['--kit-badge-run'] },
  errorChip: { backgroundColor: projbrain['--projbrain-err-14'], color: kit['--kit-err-soft'] },
  warningChip: { backgroundColor: frame['--frame-warn-14'], color: kit['--kit-warn-soft'] },
  okChip: { backgroundColor: kit['--kit-ok-12'], color: color['--color-ok'] },
  check: { display: 'flex', alignItems: 'flex-start', gap: 10, fontSize: 13, lineHeight: 1.5, color: kit['--kit-fg-note'] },
  /** Câu 3 — gợi ý sửa căn phải: chữ co giãn, ô gợi ý `flex: none` (artboard ProjBrain 12 px, màu `--color-fg-3`). */
  checkFix: { flexShrink: 0, fontSize: 12, color: color['--color-fg-3'] },
  actions: { display: 'flex', gap: 8, marginTop: 2 },
  button: { height: 32, padding: '0 12px', borderRadius: 9, borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-4'], backgroundColor: 'transparent', color: color['--color-fg'], fontSize: 12.5, display: 'inline-flex', alignItems: 'center', gap: 6 },
  table: { overflowX: 'auto' },
  ledgerRow: { display: 'grid', gridTemplateColumns: 'minmax(0,1.7fr) repeat(10,minmax(0,1fr)) 64px', gap: 6, alignItems: 'center', padding: '8px 14px', borderTopWidth: 1, borderTopStyle: 'solid', borderTopColor: kit['--kit-wash-4'], minWidth: 1060 },
  /** §H câu 5 — mọi dòng Ngày 7 đều null: bỏ hẳn cột cuối, bảng hẹp lại 70 px. */
  ledgerRowNoDay7: { gridTemplateColumns: 'minmax(0,1.7fr) repeat(10,minmax(0,1fr))', minWidth: 990 },
  tableHead: { fontSize: 11.5, color: kit['--kit-fg-icon'], paddingTop: 10, paddingBottom: 9, borderTopWidth: 0, borderBottomWidth: 1, borderBottomStyle: 'solid', borderBottomColor: line['--line-2'], alignItems: 'end' },
  axis: { display: 'flex', flexDirection: 'column', gap: 1, minWidth: 0 },
  axisNumber: { color: kit['--kit-fg-arrow'], fontSize: 10.5 },
  /** Ô tên truyện: tiêu đề (2 dòng) + dòng phụ, cách 2 px như artboard. */
  storyCell: { display: 'flex', flexDirection: 'column', gap: 2, minWidth: 0 },
  storyTitle: { fontSize: 13, fontWeight: 500, color: color['--color-fg'], lineHeight: 1.35, display: '-webkit-box', WebkitLineClamp: 2, WebkitBoxOrient: 'vertical', overflow: 'hidden' },
  /** Artboard: hàng truyện mới in đậm hơn hàng cũ (600 so với 500). */
  storyTitleDraft: { fontWeight: 600 },
  /** Dòng phụ của truyện mới: “Truyện mới · brief” (11,5 px, tím nhạt). */
  briefMeta: { fontSize: 11.5, color: kit['--kit-badge-run'] },
  /** Dòng phụ của truyện đã có: ngày thêm (11,5 px, `--color-fg-4`). */
  rowMeta: { fontSize: 11.5, color: color['--color-fg-4'] },
  /**
   * Ô trục của sổ cái: khung đệm/bo/nền đặt ở ngoài, phần chữ có `cellText` bên
   * trong. Clamp không được nằm trên khung có đệm dọc — Chromium vẫn vẽ dòng thứ 3
   * vào đệm dưới, lộ ra một mẩu chữ dưới dòng “…” (ảnh lỗi 26/10).
   */
  cell: { minWidth: 0, padding: '4px 7px', borderRadius: 7, overflow: 'hidden', fontSize: 12, lineHeight: 1.35, color: color['--color-fg-3'] },
  /** Hai dòng đầu, dòng sau cắt “…” — khung không đệm nên dòng thừa bị cắt sạch. */
  cellText: { display: '-webkit-box', WebkitLineClamp: 2, WebkitBoxOrient: 'vertical', overflow: 'hidden', overflowWrap: 'anywhere' },
  candidateRow: { backgroundColor: frame['--frame-acc-4'] },
  candidateCell: { color: color['--color-fg'], backgroundColor: kit['--kit-acc-6'], boxShadow: `inset 0 0 0 1px ${kit['--kit-acc-25']}` },
  duplicate: { color: kit['--kit-err-soft'], backgroundColor: projbrain['--projbrain-err-8'], boxShadow: `inset 0 0 0 1px ${projbrain['--projbrain-err-60']}` },
  overused: { color: kit['--kit-warn-soft'], backgroundColor: projbrain['--projbrain-warn-7'], boxShadow: `inset 0 0 0 1px ${projbrain['--projbrain-warn-50']}` },
  number: { textAlign: 'right', fontSize: 12.5, color: color['--color-fg-3'], fontVariantNumeric: 'tabular-nums' },
  legend: { display: 'flex', flexWrap: 'wrap', gap: '6px 18px', fontSize: 12, color: kit['--kit-fg-icon'] },
  legendItem: { display: 'flex', alignItems: 'center', gap: 6 },
  universe: { display: 'grid', gridTemplateColumns: 'minmax(0,1fr) 340px', gap: 14, alignItems: 'start' },
  universeRow: { display: 'grid', gridTemplateColumns: 'minmax(0,1.3fr) minmax(0,1.6fr) minmax(0,1.5fr) 92px', gap: 12, alignItems: 'center', padding: '10px 14px', borderTopWidth: 1, borderTopStyle: 'solid', borderTopColor: kit['--kit-wash-5'] },
  /** Cột “Kiểm” căn phải ở cả hàng tiêu đề lẫn hàng dữ liệu (artboard). */
  checkCol: { textAlign: 'right' },
  /** Vai trong truyện gần nhất: 12,5 px, `--color-fg-3`, dòng 1,45 (artboard). */
  role: { fontSize: 12.5, color: color['--color-fg-3'], lineHeight: 1.45 },
  /** Hai phép kiểm của thẻ tóm tắt: 12,5 px, cách 6, thụt 4 so với thân thẻ. */
  universeRules: { display: 'flex', flexDirection: 'column', gap: 6, marginTop: 4 },
  universeRule: { display: 'flex', alignItems: 'flex-start', gap: 8, fontSize: 12.5, lineHeight: 1.5, color: color['--color-fg-2'] },
  /**
   * §J câu 1 — dòng nhỏ ngay dưới thẻ nhắc cuối cùng ở cột phải (artboard: cao 28, chữ 12 px
   * `--kit-fg-icon`, không viền; ✓ 13 px + mũi tên xoay khi mở).
   */
  keptLine: { alignSelf: 'flex-start', display: 'flex', alignItems: 'center', gap: 6, height: 28, padding: '0 8px', margin: '-4px 0 0 4px', borderWidth: 0, borderRadius: 8, backgroundColor: 'transparent', color: kit['--kit-fg-icon'], fontSize: 12, cursor: 'pointer' },
  /** Danh sách mở tại chỗ trong cột phải, dùng lại thẻ phát hiện ở biến thể *đã giữ*. */
  keptList: { display: 'flex', flexDirection: 'column', gap: 8, marginTop: -4 },
  /** §J câu 1 — RecapCard biến thể *đã giữ*: viền trung tính 7%, nền `--kit-surface-18`, padding 12/16. */
  keptCard: { display: 'flex', flexDirection: 'column', gap: 6, padding: '12px 16px', borderColor: line['--line-2'], backgroundColor: kit['--kit-surface-18'] },
  keptHead: { display: 'flex', alignItems: 'center', gap: 8, fontSize: 13, fontWeight: 600, color: color['--color-fg-2'] },
  keptChip: { backgroundColor: kit['--kit-wash-6'], color: color['--color-fg-3'] },
  keptText: { fontSize: 12.5, color: kit['--kit-fg-icon'], lineHeight: 1.55 },
  keptReason: { fontSize: 12.5, color: color['--color-fg-3'], lineHeight: 1.5 },
  keptFoot: { display: 'flex', alignItems: 'center', gap: 12 },
  /** Artboard dùng `.btn2` viền none, chữ `--color-fg-3` cho “Bỏ giữ”. */
  unkeep: { borderWidth: 0, color: color['--color-fg-3'] },
  /** “Mở đoạn đó ›” ở cả hai biến thể thẻ nhắc (12,5 px, màu nhấn). */
  openLink: { fontSize: 12.5, color: color['--color-accent'] },
  stack: { display: 'flex', flexDirection: 'column', gap: 12 },
  entity: { display: 'flex', alignItems: 'center', gap: 10, minWidth: 0 },
  /** Avatar nhân vật trong bảng vũ trụ: nền tím 14%, chữ tím nhạt (artboard). */
  avatar: { width: 32, height: 32, borderRadius: '50%', backgroundColor: kit['--kit-acc-14'], color: kit['--kit-badge-run'], display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: 11, fontWeight: 700, flexShrink: 0 },
  /** Địa điểm: nền xanh ngọc 12%, icon xanh ngọc (artboard). */
  place: { borderRadius: 9, backgroundColor: projbrain['--projbrain-place-12'], color: brain['--brain-place'] },
  entityName: { fontSize: 13.5, fontWeight: 600, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' },
  /** Ô tên + loại ở bảng vũ trụ: không gap (artboard). */
  nameCell: { display: 'flex', flexDirection: 'column', minWidth: 0 },
  storyChips: { display: 'flex', flexWrap: 'wrap', gap: 4, minWidth: 0 },
  neutralChip: { backgroundColor: kit['--kit-wash-5'], color: color['--color-fg-2'] },
  body: { fontSize: 13, color: color['--color-fg-3'], lineHeight: 1.55 },
  searchField: { height: 42, display: 'flex', alignItems: 'center', gap: 10, padding: '0 14px', borderRadius: 12, backgroundColor: color['--color-surface-0'], borderWidth: 1, borderStyle: 'solid', borderColor: kit['--kit-acc-40'], maxWidth: 720, flexShrink: 0 },
  input: { flex: 1, minWidth: 0, borderWidth: 0, backgroundColor: 'transparent', color: color['--color-fg'], fontSize: 15, outline: 'none' },
  /** Số kết quả trong ô tìm: 12 px, `--color-fg-4` (artboard). */
  searchCount: { fontSize: 12, color: color['--color-fg-4'], whiteSpace: 'nowrap' },
  searchGrid: { display: 'grid', gridTemplateColumns: '320px minmax(0,1fr)', gap: 14, alignItems: 'start' },
  entityCard: { padding: '12px 14px', display: 'flex', flexDirection: 'column', gap: 4 },
  /** Thẻ đoạn văn: không gap — mỗi kết quả tự có đệm 10/8 (artboard). */
  passageCard: { padding: '12px 14px', display: 'flex', flexDirection: 'column' },
  label: { paddingBottom: 4, fontSize: 11, fontWeight: 600, letterSpacing: '.08em', color: color['--color-fg-4'], textTransform: 'uppercase' },
  labelPassages: { paddingBottom: 6 },
  /** Kết quả thực thể: đệm dọc 8, không bo, đường kẻ trên giữa các kết quả. */
  entityHit: { display: 'flex', flexDirection: 'column', gap: 2, padding: '8px 0' },
  /** Tên thực thể trong kết quả tìm: icon 15 + tên, cách 8 (artboard). */
  entityRow: { display: 'flex', alignItems: 'center', gap: 8, minWidth: 0 },
  /** Kết quả đoạn văn: đệm 10/8, bo 10 (artboard `.row`). */
  passageHit: { display: 'flex', flexDirection: 'column', gap: 4, padding: '10px 8px', borderRadius: 10 },
  /** Đường kẻ trên: thực thể 6%, đoạn văn 5% (artboard `bt`). */
  entityDivider: { borderTopWidth: 1, borderTopStyle: 'solid', borderTopColor: kit['--kit-wash-6'] },
  passageDivider: { borderTopWidth: 1, borderTopStyle: 'solid', borderTopColor: kit['--kit-wash-5'] },
  entityNameLoose: { fontSize: 13.5, fontWeight: 500 },
  entityIcon: { flexShrink: 0 },
  iconChar: { color: brain['--brain-char'] },
  iconPlace: { color: brain['--brain-place'] },
  iconObj: { color: brain['--brain-obj'] },
  iconEv: { color: brain['--brain-ev'] },
  iconFs: { color: brain['--brain-fs'] },
  iconRule: { color: brain['--brain-rule'] },
  /** Dòng “truyện · loại” dưới tên thực thể: 12 px, thụt 23 px bằng icon. */
  entityWhere: { paddingLeft: 23, fontSize: 12, color: kit['--kit-fg-icon'] },
  entityRel: { paddingLeft: 23, fontSize: 12.5, color: color['--color-fg-3'], lineHeight: 1.45 },
  /** Siêu dữ liệu đoạn văn: tên truyện đậm nhẹ, “· chương N” xám. */
  passageStory: { fontSize: 12, fontWeight: 500, color: color['--color-fg-2'] },
  passageChapter: { fontSize: 12, color: kit['--kit-fg-icon'] },
  quote: { fontFamily: font['--font-read'], fontSize: 14, color: color['--color-fg-2'], lineHeight: 1.6 },
  mark: { backgroundColor: projbrain['--projbrain-mark'], color: projbrain['--projbrain-mark-fg'], borderRadius: 3, padding: '0 2px' },
  /** Câu chân thẻ đoạn văn: 12 px, `--kit-fg-faint`, đệm 8/8/0. */
  resultsNote: { fontSize: 12, color: kit['--kit-fg-faint'], padding: '8px 8px 0' },
})
