// Hợp đồng dữ liệu giữa SettingsScreen (index.tsx) và các section.
import type {
  AudioSettings, Config, ConfigPresets, ConfigResponse, ConfigScope, LibraryConfig, LibraryResponse, Snapshot,
} from '../../api/types'
import type { FieldState } from './components'
import type { Ovr } from './lib/labels'
import type { SectionId } from './lib/target'

/** Phần việc riêng của một mục (Thông tin, Quy tắc…) góp vào thanh nổi đáy: Lưu/Áp dụng và Huỷ gọi lần lượt. */
export type PendingExtra = {
  count: number
  /** Resolve bằng chuỗi → dùng làm nội dung toast thành công. */
  apply: () => Promise<string | void>
  cancel: () => void
  /** Phần đuôi của câu ở thanh nổi, vd “có hiệu lực từ chương 26”. */
  hint?: string
}

export type SettingsContext = {
  /** cấp của tệp cấu hình đang sửa: mục Hệ thống luôn là `global` */
  scope: ConfigScope
  /** cấp người dùng chọn ở thanh cấp; mục Hệ thống vẫn giữ nó (dòng “cấp đang chọn” ở Nâng cao) dù `scope` là global. */
  viewScope: ConfigScope
  project: string
  story: string
  /** tên hiển thị (không slug) của dự án / truyện đang chọn */
  projectName: string
  storyTitle: string
  admin: boolean
  /** được sửa tệp cấu hình của cấp đang chọn */
  canEdit: boolean
  canEditScope: (scope: ConfigScope) => boolean
  response: ConfigResponse | null
  /** bản nháp tệp của cấp đang chọn (chỉ chứa khoá đã ghi đè) */
  draft: Config
  /** cấu hình hiệu lực sau khi gộp chuỗi */
  effective: Config
  presets: ConfigPresets | null
  /** ảnh chụp trạng thái truyện (chỉ ở cấp truyện) — chi phí đã dùng, chế độ duyệt, chương đang viết */
  snapshot: Snapshot | null
  setField: (path: string, value: unknown) => void
  resetField: (path: string) => void
  /** `undefined` ở cấp toàn cục — không có cấp kế thừa để hiển thị badge */
  fieldState: (path: string) => FieldState | undefined
  /** Nhãn kế thừa của một nhóm khoá (Settings.dc.html `ovr`): Riêng … / Theo dự án / Theo “Mọi truyện” / Mặc định. */
  ovr: (paths: string[]) => Ovr
  refreshConfig: () => void
  audioSettings: AudioSettings | null
  audioSettingsError: string
  setAutoOnLock: (value: boolean) => void
  refreshAudioSettings: () => void
  library: LibraryResponse | null
  libDraft: LibraryConfig | null
  setLibDraft: (next: LibraryConfig) => void
  /** Tải lại thư viện sau khi đổi nơi lưu truyện (LibraryRootRows tự PUT {root}). */
  reloadLibrary: () => void
  /** G1 — máy chủ chạy trên chính máy này: hiện “Mở thư mục”, ngược lại hiện “Chép đường dẫn” */
  canReveal: boolean
  /** truyện mẫu để xem trước slug (truyện đầu của dự án đang chọn) */
  sampleStoryTitle: string
  sampleStoryIndex: number
  /** Mục nào cũng đăng ký phần việc riêng bằng khoá của mình; `null` = không còn thay đổi. */
  registerExtra: (key: string, extra: PendingExtra | null) => void
  /** Chuyển mục (giữ nguyên cấp/ngữ cảnh), vd nút “Đổi mẫu tên trong Nâng cao”. */
  goSection: (id: SectionId, open?: string) => void
}
