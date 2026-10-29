// Nhãn kit của Viderse — một nguồn duy nhất cho main.tsx và test (test render
// App phải đi qua đúng đường production, không tự gắn nhãn khác).
import { KIT_LABELS_VI, setKitLabels } from '@noverse/ui'
import { t } from './i18n/vi'

export function applyViderseLabels(): void {
  setKitLabels({ ...KIT_LABELS_VI, brand: t.brand })
}
