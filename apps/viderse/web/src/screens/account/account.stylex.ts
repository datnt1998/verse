import * as stylex from '@stylexjs/stylex'
import { color, kit, line } from '@noverse/ui/tokens.stylex'
const MOBILE = '@media (max-width: 767px)'
export const accountStyles = stylex.create({
  // Màn Tài khoản dựng ngoài MobileShell nên tự giữ vùng an toàn: đỉnh theo
  // `--m-top` (tai thỏ), đáy theo `--m-safe-bottom` (home indicator).
  page: { height: '100%', flexGrow: 1, minHeight: 0, width: '100%', display: 'flex', flexDirection: 'column', paddingTop: { default: 0, [MOBILE]: 'var(--m-top)' }, lineHeight: { default: 1.55, [MOBILE]: 1.5 } },
  header: { flexShrink: 0, height: 52, display: 'flex', alignItems: 'center', gap: 2, paddingInline: 6, borderBottomWidth: 1, borderBottomStyle: 'solid', borderBottomColor: line['--line-2'] },
  back: { width: 44, height: 44, padding: 0, gap: 0, borderRadius: 12, fontSize: 14, fontWeight: 400, lineHeight: 1.5, color: color['--color-fg-2'] },
  mobileTitle: { fontSize: 16, fontWeight: 600, margin: 0, flexGrow: 1 },
  main: { flexGrow: 1, minHeight: 0, overflowY: 'auto', padding: { default: '28px 32px 48px', [MOBILE]: '4px 16px max(20px, var(--m-safe-bottom))' } },
  column: { width: '100%', maxWidth: 680, marginInline: 'auto', display: 'flex', flexDirection: 'column', gap: { default: 16, [MOBILE]: 14 } },
  mobileNote: { fontSize: 13, color: color['--color-fg-3'], paddingInline: 4, lineHeight: 1.55 },
  mobileLabel: { fontSize: 12.5, color: kit['--kit-fg-sub'], paddingInline: 4 },
  mobileCard: { borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-2'], borderRadius: 16, backgroundColor: color['--color-surface-0'], overflow: 'hidden' },
  row: { width: '100%', height: 56, display: 'flex', alignItems: 'center', gap: 12, padding: '6px 14px', borderWidth: 0, backgroundColor: 'transparent', textAlign: 'left', color: color['--color-fg'], fontSize: 14, textDecoration: 'none' },
  rowDivider: { borderBottomWidth: 1, borderBottomStyle: 'solid', borderBottomColor: 'var(--vv-hairline)' },
  rowText: { flexGrow: 1, minWidth: 0, fontSize: 15 },
  secondary: { display: 'block', fontSize: 12, lineHeight: 1.35, color: kit['--kit-fg-meta'] },
  mutedIcon: { color: color['--color-fg-3'] },
  count: { fontSize: 13, color: kit['--kit-fg-meta'] },
  logout: { height: 48, width: '100%', justifyContent: 'center', borderRadius: 12, fontSize: 14, paddingInline: 16, borderColor: 'var(--vv-warn-bd)' },
  logoutNote: { fontSize: 12.5, color: kit['--kit-fg-sub'], textAlign: 'center' },
  sheetHeader: { display: 'flex', flexDirection: 'column', gap: 4, padding: '6px 16px 8px' },
  sheetTitle: { fontSize: 18, fontWeight: 600, lineHeight: 1.35 },
  sheetSubtitle: { fontSize: 13, lineHeight: 1.5, color: color['--color-fg-3'] },
  sheetBody: { padding: '4px 16px 10px', display: 'flex', flexDirection: 'column', gap: 12 },
})
