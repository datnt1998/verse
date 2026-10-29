// Avatar nhân vật & dòng tên/vai — luật duy nhất của app (04-components.md › “Avatar nhân vật”,
// chốt 26/10). Mọi danh sách Nhân vật (Bàn viết, Dự án & truyện, Não truyện, Não dự án,
// người nói ở Audio) phải qua các hàm ở đây; không sao chép luật ra từng màn.

/**
 * Danh xưng đứng đầu tên (04 › “Bắt đầu bằng danh xưng”: Ông, Bà, Cô, Chú, Bác, Dì,
 * Cậu, Mợ, Thầy, Luật sư, Bác sĩ, Giám đốc…). So sánh không phân biệt hoa/thường.
 */
const HONORIFICS: readonly string[] = [
  'luật sư', 'bác sĩ', 'giám đốc', 'giáo sư', 'tiến sĩ',
  'ông', 'bà', 'cô', 'chú', 'bác', 'dì', 'cậu', 'mợ', 'thầy', 'cụ',
].sort((a, b) => b.split(' ').length - a.split(' ').length)

/** Chữ đầu (đã viết hoa, giữ dấu: Đ) của một từ. */
const head = (word: string) => (word[0] ?? '').toUpperCase()

/**
 * Chữ trong avatar tròn — 04-components.md:104-109:
 * - `ini` người dùng tự đặt (nếu có) thắng mọi luật.
 * - 1 chữ → chữ cái đầu (“Lụa” → L).
 * - 2 chữ → chữ đầu cả hai (“Lâm An” → LA).
 * - ≥3 chữ → bỏ họ, lấy chữ đầu 2 chữ cuối (“Lâm Tri Hạ” → TH).
 * - Bắt đầu bằng danh xưng → chữ đầu danh xưng + chữ đầu chữ cuối
 *   (“Luật sư Hàn” → LH, “Cô lao công Lụa” → CL, “Bà Tư” → BT).
 */
export function charInitials(name: string, ini?: string | null): string {
  const custom = (ini ?? '').trim()
  if (custom) return custom
  const words = name.trim().split(/\s+/).filter(Boolean)
  if (words.length === 0) return '?'
  const lower = words.map((word) => word.toLowerCase())
  // Danh xưng: còn ít nhất 1 chữ sau danh xưng thì lấy đầu danh xưng + đầu chữ cuối.
  for (const honorific of HONORIFICS) {
    const parts = honorific.split(' ')
    if (words.length > parts.length && parts.every((part, index) => lower[index] === part)) {
      return head(words[0]) + head(words[words.length - 1])
    }
  }
  if (words.length === 1) return head(words[0])
  return head(words[words.length - 2]) + head(words[words.length - 1])
}

/**
 * Tách dòng nhân vật cũ “Tên (vai; …)” thành { name, role } — backend snapshot
 * vẫn ghép vai vào trong ngoặc (chuỗi này còn phục vụ TUI, xem host.go).
 * Nhận cả ngoặc thường `(…)` và ngoặc rộng `（…）`; trong `role` đổi “;” thành “ · ”.
 */
export function splitCastLabel(raw: string): { name: string; role: string } {
  const text = (raw ?? '').trim()
  const match = /^(.*)\s*[（(]([^（）()]*)\s*[）)]\s*$/.exec(text)
  if (!match) return { name: text, role: '' }
  const name = match[1].trim()
  if (!name) return { name: text, role: '' }
  return { name, role: roleText(match[2]) }
}

/** Vai hiển thị một dòng: các ý nối bằng “ · ”, “;” (dữ liệu cũ) đổi thành “ · ”. */
export function roleText(role: string): string {
  return (role ?? '')
    .split(/[;·]/)
    .map((part) => part.trim())
    .filter(Boolean)
    .join(' · ')
}

/**
 * Nhãn tiếng Việt của vai — bảng ROLE của artboard (04-components.md › dòng tên/vai).
 * Máy chủ gửi slug tiếng Anh (`main`, `antagonist`…) hoặc tiếng Việt; khóa theo ý đầu.
 * Trả `''` khi vai ngoài bảng để nơi gọi tự quyết chữ dự phòng.
 */
const ROLE_LABEL: Record<string, string> = {
  'nữ chính': 'Nữ chính',
  'phản diện chính': 'Phản diện chính',
  'đồng phạm': 'Đồng phạm',
  'đồng minh hài': 'Đồng minh hài',
  'nam chính phụ trợ': 'Nam chính phụ trợ',
  'phụ chức năng': 'Phụ chức năng',
  main: 'Nhân vật chính',
  protagonist: 'Nhân vật chính',
  antagonist: 'Phản diện chính',
  villain: 'Phản diện',
  supporting: 'Phụ trợ',
  mentor: 'Dẫn dắt',
  minor: 'Phụ chức năng',
  // Ba vai của bản đọc (audio.Speaker.Role): máy chủ gửi slug tiếng Anh.
  narrator: 'Dẫn truyện',
  support: 'Phụ trợ',
}

export function roleLabel(role: string): string {
  const first = roleText(role).split(' · ')[0] ?? ''
  return ROLE_LABEL[first.toLowerCase()] ?? ''
}

/**
 * Vai hiển thị của một người nói trong bản đọc. Máy chủ gửi slug (`narrator`,
 * `main`, `support` — internal/audio/cast.go), dữ liệu cũ gửi câu tiếng Việt
 * (“Nữ chính · lời thoại”). Chỉ dịch khi CẢ chuỗi là một slug đã biết, còn lại
 * giữ nguyên câu tiếng Việt để không mất phần sau dấu “ · ”.
 */
export function speakerRoleLabel(role: string): string {
  const text = roleText(role)
  return ROLE_LABEL[text.toLowerCase()] ?? text
}
