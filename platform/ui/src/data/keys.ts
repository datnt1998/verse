// Khoá cache dùng chung. Khoá là mảng; `invalidate(prefix)` khớp mọi khoá bắt đầu
// bằng prefix, nên app đặt phần chung lên trước phần chi tiết. Không có khoá của app nào ở đây.
export type KeyPart = string | number | boolean | null
export type Key = readonly KeyPart[]

/** Băm khoá ổn định (JSON) — hai khoá cùng nội dung → cùng mục cache. */
export const keyHash = (key: Key) => JSON.stringify(key)

/** `key` bắt đầu bằng `prefix`? Prefix rỗng khớp mọi khoá. */
export function matchesPrefix(key: Key, prefix: Key): boolean {
  if (prefix.length > key.length) return false
  for (let i = 0; i < prefix.length; i++) if (key[i] !== prefix[i]) return false
  return true
}
