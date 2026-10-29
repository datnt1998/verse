// D2 — gói thể loại: `GET /api/config/presets` › `packs`. Dùng chung cho Tuỳ chỉnh › Cách viết,
// Bắt đầu nhanh (chip “Văn phong”), màn thiết lập truyện trên điện thoại và dòng “Văn phong” ở cột phải Dự án.
import type { PackLayer, PackParam, PackPreset, PackSection } from '../../../api/types'

export type PackCard = { id: string; label: string; meta: string; pack: PackPreset }

/**
 * Tên gói hiển thị (Câu 12): `short` (`goi.json` › `ten_ngan`); gói cũ thiếu `short` thì bỏ tiền tố
 * “Gói ” và phần sau dấu phẩy của `ten` (“Nữ tần trọng sinh báo thù, vả mặt” → “Nữ tần trọng sinh báo thù”).
 */
export function packLabel(pack: Pick<PackPreset, 'short' | 'ten'>): string {
  const short = (pack.short ?? '').trim()
  if (short) return short
  return (pack.ten ?? '').replace(/^Gói\s+/i, '').split(',')[0].trim()
}

/** Gói → thẻ: tên ngắn + “8 chương · thoại 40–55%” (máy chủ soạn `meta` từ dữ liệu gói). */
export function packCards(packs: PackPreset[] | undefined): PackCard[] {
  return (packs ?? []).map((pack) => ({
    id: pack.id, label: packLabel(pack), meta: (pack.meta ?? '').trim(), pack,
  }))
}

/** Gói đang chọn trong `style` (null khi đang dùng kiểu viết hoặc gói đã bị xoá khỏi máy chủ). */
export function findPack(packs: PackPreset[] | undefined, id: string | undefined): PackPreset | null {
  return (packs ?? []).find((pack) => pack.id === id) ?? null
}

/** Số theo cách viết tiếng Việt: 1400 → “1.400”. */
export const viNumber = (value: number): string => value.toLocaleString('vi-VN')

/**
 * Câu 1 — độ dài chương của gói. Settings viết đủ “1.400–1.600 tiếng mỗi chương”; khối gói ở MMore bỏ
 * “mỗi chương” (`perChapter: false`) vì đã nằm trong câu “8 chương · 1.400–1.600 tiếng · thoại 40–55%”.
 * Gói không khai `chapterWords` → chuỗi rỗng: không bịa số.
 */
export function chapterWordsText(words: PackPreset['chapterWords'], { perChapter = true }: { perChapter?: boolean } = {}): string {
  if (!words || typeof words.min !== 'number' || typeof words.max !== 'number') return ''
  const range = `${viNumber(words.min)}–${viNumber(words.max)} tiếng`
  return perChapter ? `${range} mỗi chương` : range
}

/**
 * Hàng chip “Số đo dùng để chấm” của chi tiết gói: `params` máy chủ trả, thêm chip độ dài chương ngay sau
 * “N chương” (Settings.dc.html › `pk.params`). Gói không khai độ dài → đúng `params`, không chip rỗng.
 */
export function packParamChips(pack: Pick<PackPreset, 'params' | 'chapterWords'>): PackParam[] {
  const chips = pack.params ?? []
  const text = chapterWordsText(pack.chapterWords)
  if (!text) return [...chips]
  const after = chips.findIndex((chip) => chip.key === 'chapters') + 1
  return [...chips.slice(0, after), { key: 'chapterWords', text }, ...chips.slice(after)]
}

/**
 * Dòng 2 khối gói ở MMore (MMore.dc.html): “8 chương · 1.400–1.600 tiếng · thoại 40–55%” — độ dài chương
 * chèn giữa số chương và tỷ lệ thoại; gói không khai thì giữ nguyên `meta` máy chủ soạn.
 */
export function packInfoMeta(pack: PackPreset): string {
  const parts = [`${pack.so_chuong} chương`]
  const words = chapterWordsText(pack.chapterWords, { perChapter: false })
  if (words) parts.push(words)
  const dialogue = (pack.params ?? []).find((param) => param.key === 'dialogue')?.text
  // “Thoại 40–55%” → “thoại 40–55%” giữa câu (artboard viết thường).
  if (dialogue) parts.push(dialogue.charAt(0).toLocaleLowerCase('vi') + dialogue.slice(1))
  return parts.length > 1 ? parts.join(' · ') : (pack.meta ?? '').trim()
}

/** Badge trên thẻ gói: gói chưa dùng được = “Sắp có”, có bộ luật = “Có chấm” (artboard `packs[].badge`). */
export function packBadge(pack: PackPreset): { label: string; ok: boolean } {
  return pack.available === false || pack.hasRules === false
    ? { label: 'Sắp có', ok: false }
    : { label: 'Có chấm', ok: true }
}

/** Dòng mô tả dưới tên gói; máy chủ trả trùng `ten` thì không hiện (tránh lặp chữ). */
export function packDesc(pack: PackPreset): string {
  const desc = (pack.desc ?? '').trim()
  return desc && desc !== pack.ten ? desc : ''
}

export type LayerCount = { text: string; tone: 'err' | 'wrn' | 'neu' }

/** Hai nhãn đếm của một tầng luật; tầng không có mục nào ghi “dùng khi chấm tự động” (artboard `.cnt`). */
export function layerCounts(layer: PackLayer): LayerCount[] {
  const out: LayerCount[] = []
  if (layer.errors > 0) out.push({ text: `${layer.errors} bắt sửa`, tone: 'err' })
  if (layer.warnings > 0) out.push({ text: `${layer.warnings} cảnh báo`, tone: 'wrn' })
  return out.length > 0 ? out : [{ text: 'dùng khi chấm tự động', tone: 'neu' }]
}

/**
 * Câu 10 — “Trợ lý đọc đủ Master + Bộ A · 65 mục bắt sửa, 61 mục cảnh báo”: tên các tầng luật có mục
 * chấm (Master + bộ của gói, bỏ tầng Cách đo) và số mục cộng từ `layers` máy chủ trả. Không số phiên bản;
 * gói không có mục nào thì không có vế số.
 */
export function packRuleSummary(pack: PackPreset): { layers: string; counts: string } {
  const rules = (pack.layers ?? []).filter((layer) => layer.key === 'master' || layer.key === 'pack')
  const errors = rules.reduce((sum, layer) => sum + (layer.errors ?? 0), 0)
  const warnings = rules.reduce((sum, layer) => sum + (layer.warnings ?? 0), 0)
  return {
    // “Luật chung · Master” → “Master”; “Luật thể loại · Bộ A” → “Bộ A”.
    layers: rules.map((layer) => layer.title.split('·').pop()?.trim() ?? '').filter(Boolean).join(' + '),
    counts: errors + warnings > 0 ? `${errors} mục bắt sửa, ${warnings} mục cảnh báo` : '',
  }
}

/** Số mục của một §: “3 / 1” (bắt sửa / cảnh báo) như artboard; không có mục nào → “—”. */
export function sectionCount(section: PackSection): { text: string; tone: LayerCount['tone'] } {
  const errors = section.errors ?? 0
  const warnings = section.warnings ?? 0
  if (errors === 0 && warnings === 0) return { text: '—', tone: 'neu' }
  return { text: `${errors} / ${warnings}`, tone: errors > 0 ? 'err' : 'wrn' }
}

/** “Master §1” → “§1” (cột số hiệu của danh sách mục, artboard `.secl`). */
export function sectionRef(ref: string): string {
  const at = ref.lastIndexOf('§')
  return at >= 0 ? ref.slice(at) : ref
}

/** Icon của từng chip thông số (Settings.dc.html › `pk.params`): book-1 · messages-2 · edit-2 · timer-1 · note-text · copy. */
export const PARAM_ICON: Record<string, string> = {
  chapters: 'book-1', chapterWords: 'document-text', dialogue: 'messages-2', sentence: 'edit-2',
  quiet: 'timer-1', procedural: 'note-text', opening: 'copy',
}
