// Logic thuần của Tuỳ chỉnh › Kết nối AI (Settings.dc.html renderVals `provs`).

/** Ba nền gradient của artboard (ChatGPT · OpenRouter · DeepSeek); nhà cung cấp thứ 4 trở đi quay vòng lại. */
export const AVATAR_GRADIENTS = [
  'linear-gradient(135deg,#E9FBEF,#A6EDC1)',
  'linear-gradient(135deg,#EEF4FF,#B9D0F7)',
  'linear-gradient(135deg,#FFF1DC,#F6CB8C)',
] as const

/** Chữ cái đầu + nền của ô 40px; `index` = vị trí thẻ trong danh sách. */
export function avatarOf(name: string, index: number): { ini: string; bg: string } {
  const ini = Array.from(name.trim())[0]?.toLocaleUpperCase('vi') ?? '?'
  const slot = ((index % AVATAR_GRADIENTS.length) + AVATAR_GRADIENTS.length) % AVATAR_GRADIENTS.length
  return { ini, bg: AVATAR_GRADIENTS[slot] }
}

/** Tên hiển thị: nhãn preset bỏ phần trong ngoặc (“ChatGPT Plus/Pro (Codex …)” → “ChatGPT Plus/Pro”); không có preset thì dùng id. */
/** Tên thương hiệu khi chưa tải xong presets — không bao giờ hiện mã nhà cung cấp thô trong câu lỗi. */
const KNOWN_PROVIDER_NAMES: Record<string, string> = {
  openai: 'ChatGPT', 'openai-codex': 'ChatGPT', deepseek: 'DeepSeek', gemini: 'Gemini',
  anthropic: 'Claude', ollama: 'Ollama', xai: 'Grok', openrouter: 'OpenRouter',
}

export function providerName(id: string, preset: { id: string; label: string } | null): string {
  if (!preset) return KNOWN_PROVIDER_NAMES[id] ?? id
  if (preset.id === 'openai-codex') return 'ChatGPT'
  return preset.label.split(' (')[0] || KNOWN_PROVIDER_NAMES[id] || id
}

/** `api_key_hint` của máy chủ (“sk-o…7890”, “…”, “oauth”) → “••••7890”; không có 4 ký tự cuối đọc được → “••••”. */
export function keyMask(hint: string | undefined): string {
  const tail = hint && hint !== 'oauth' && hint.includes('…') ? hint.slice(hint.indexOf('…') + 1) : ''
  return `••••${tail.length >= 4 ? tail.slice(-4) : ''}`
}

export type SubLine = { text: string; tone: 'ok' | 'mute' | 'warn' }

/** Màu dòng phụ: xanh khi đã đăng nhập, vàng khi chưa chọn model, xám còn lại (artboard `subFg`). */
export const SUB_COLOR: Record<SubLine['tone'], string> = { ok: '#A6EDC1', mute: '#8E8E99', warn: 'var(--color-warn)' }

export type SubInput = {
  oauth: boolean
  /** OAuth: tài khoản đã kết nối (null = chưa) */
  account: { name: string; plan: string } | null
  keySet: boolean
  keyHint?: string
  /** nhà cung cấp chạy trên máy này, không cần khoá */
  local: boolean
  models: number
}

/** Dòng phụ dưới tên: “Đã đăng nhập · email · gói Plus · 3 model đã chọn” / “Khoá API ••••abcd · 2 model đã chọn”.
 *  models = 0 → cảnh báo vàng “chưa chọn model, trợ lý chưa dùng được”. */
export function subLine(input: SubInput): SubLine {
  const warn = input.models === 0
  const tail = warn ? 'chưa chọn model, trợ lý chưa dùng được' : `${input.models} model đã chọn`
  const tone: SubLine['tone'] = warn ? 'warn' : 'mute'
  if (input.oauth) {
    if (!input.account) return { text: `Chưa đăng nhập · ${tail}`, tone }
    const parts = ['Đã đăng nhập', input.account.name, ...(input.account.plan ? [`gói ${input.account.plan}`] : []), tail]
    return { text: parts.filter(Boolean).join(' · '), tone: warn ? 'warn' : 'ok' }
  }
  if (input.keySet) return { text: `Khoá API ${keyMask(input.keyHint)} · ${tail}`, tone }
  if (input.local) return { text: `Chạy trên máy này · ${tail}`, tone }
  return { text: `Chưa có khoá API · ${tail}`, tone }
}

/** Nhãn nút “Thử kết nối” sau khi thử xong: “Ổn · 420 ms”. */
export const testedLabel = (latencyMs: number): string => `Ổn · ${Math.round(latencyMs)} ms`
