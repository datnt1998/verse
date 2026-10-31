// Dòng phụ của một template trong danh sách luân phiên. Backend đã tóm tắt
// recipe thành câu "Prompt <cảnh> · <cách ghép> · bìa <bìa>" (TemplateSummary.
// prompts.summary) — dùng nguyên câu đó rồi nối số ảnh, thay vì lặp lại tên mục
// tab. Không tự ghép lại style/bìa để khỏi lệch với màn Thư viện.
import type { TemplateSummary } from '../../api/library'
import { t } from '../../i18n/vi'

export function templateLine(template: TemplateSummary): string {
  const tt = t.channels.templates.line
  return [
    template.prompts.summary,
    template.images.count ? tt.images(template.images.count) : '',
  ].filter(Boolean).join(tt.sep)
}
