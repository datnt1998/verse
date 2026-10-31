/**
 * Engine i18n seam.
 *
 * Freecut uses i18next with resource bundles. YT Factory ships a
 * Vietnamese-only UI, so the engine keeps a tiny `t()` facade with the
 * subset of keys the engine itself reads (project validation, relative
 * time, preview placeholders). Key lookups fall back to the key name so a
 * missing entry is visible instead of blank.
 */

export type TFunction = (
  key: string,
  options?: Record<string, unknown> | string,
) => string

const MESSAGES: Record<string, string> = {
  'projects.copySuffix': '{{name}} (Bản sao)',
  'projects.upgradeBackupName': '{{name}} (Bản sao lưu trước khi nâng cấp v{{from}} lên v{{to}})',
  'projects.relativeTime.justNow': 'vừa xong',
  'projects.relativeTime.minutesAgo': '{{count}} phút trước',
  'projects.relativeTime.hoursAgo': '{{count}} giờ trước',
  'projects.relativeTime.daysAgo': '{{count}} ngày trước',
  'projects.relativeTime.weeksAgo': '{{count}} tuần trước',
  'projects.relativeTime.monthsAgo': '{{count}} tháng trước',
  'projects.relativeTime.yearsAgo': '{{count}} năm trước',
  'projects.validation.nameRequired': 'Tên project là bắt buộc',
  'projects.validation.nameTooLong': 'Tên project phải ngắn hơn 100 ký tự',
  'projects.validation.nameWhitespace': 'Tên project không được chỉ gồm khoảng trắng',
  'projects.validation.descriptionTooLong': 'Mô tả phải ngắn hơn 500 ký tự',
  'projects.validation.widthInteger': 'Chiều rộng phải là số nguyên',
  'projects.validation.widthMin': 'Chiều rộng tối thiểu 320px',
  'projects.validation.widthMax': 'Chiều rộng tối đa 7680px (8K)',
  'projects.validation.heightInteger': 'Chiều cao phải là số nguyên',
  'projects.validation.heightMin': 'Chiều cao tối thiểu 240px',
  'projects.validation.heightMax': 'Chiều cao tối đa 4320px (8K)',
  'projects.validation.fpsInteger': 'FPS phải là số nguyên',
  'projects.validation.fpsMin': 'FPS tối thiểu 1',
  'projects.validation.fpsMax': 'FPS tối đa 240',
  'projects.validation.fpsUnsupported': 'FPS phải là một tốc độ khung hình được hỗ trợ',
  'projects.validation.invalidHexColor': 'Phải là mã màu hex hợp lệ (ví dụ #000000)',
  'preview.mediaOffline.title': 'Media ngoại tuyến',
  'preview.mediaOffline.relink': 'Liên kết lại trong panel Media',
  'preview.mediaOffline.loading': 'Đang tải…',
}

// Vietnamese has no plural inflection; the bare key already carries the
// "{{count}}" placeholder, so plural suffixes collapse onto the base key.
function t(key: string, options?: Record<string, unknown> | string): string {
  if (typeof options === 'string') options = { defaultValue: options }
  const found = MESSAGES[key] ?? MESSAGES[key.replace(/_(one|other|few|many|zero|two)$/, '')]
  let template =
    found ?? (typeof options?.defaultValue === 'string' ? options.defaultValue : key)
  if (options) {
    for (const [name, value] of Object.entries(options)) {
      if (value === undefined || value === null) continue
      template = template.replaceAll(`{{${name}}}`, String(value))
    }
  }
  return template
}


export const i18n = {
  t: t as TFunction,
  language: 'vi',
}

export { t }
export default i18n
