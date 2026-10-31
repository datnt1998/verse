// Màu avatar kênh của tôi: kênh không có cột màu trong DB nên chọn tint ổn
// định theo id (5 màu phẳng của artboard, dùng chung với đối thủ ở Khám phá).
const TINTS = ['var(--vv-rav-1)', 'var(--vv-rav-2)', 'var(--vv-rav-3)', 'var(--vv-rav-4)', 'var(--vv-rav-5)']

export function channelTint(id: string): string {
  let hash = 0
  for (let i = 0; i < id.length; i++) hash = (hash * 31 + id.charCodeAt(i)) >>> 0
  return TINTS[hash % TINTS.length]
}
