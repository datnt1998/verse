// Máy đọc — một nguồn câu chữ + bộ chuyển trạng thái sang khung nhìn (10-audio.md §8).
// Dùng chung cho Tuỳ chỉnh › Giọng đọc/Nâng cao, thẻ đầu màn Bản đọc · Phòng thu · Nghe,
// hàng “Chuẩn bị bộ đọc” của dock và bản điện thoại (MAudio/MVoiceGen import trực tiếp).
//
// Quy ước: ngoài Nâng cao không bao giờ lộ mã lỗi, tên công cụ hay đường dẫn — mọi thứ
// người dùng đọc đều đi qua file này. `machineView` thuần (không React, không I/O).

/** Trạng thái máy đọc theo 06-data-contract › §AT (thêm `stopped` của bản cá nhân). */
export type MachineState = 'missing' | 'provisioning' | 'starting' | 'ready' | 'error' | 'offline' | 'stopped'

/** Biến thể thẻ: 5 trạng thái §8.1 + `missingFfmpeg` (§8.3) + `ready`. */
export type MachineKind = 'noMachine' | 'offline' | 'provisioning' | 'provisionFailed' | 'ready' | 'missingFfmpeg'

/** Bản máy chủ (máy đọc riêng) hay bản cá nhân (bộ đọc chạy trên máy này). */
export type MachineMode = 'server' | 'local'

export type MachineTone = 'neutral' | 'info' | 'danger' | 'warn' | 'ok'

/** Nút trên thẻ — máy tính và điện thoại tự vẽ theo `kind`. */
export type MachineActionKind = 'retry' | 'advanced' | 'prepare' | 'settings' | 'link'
export type MachineAction = { label: string; kind: MachineActionKind }

/** Trường máy đọc UI đọc được. Mọi trường đều tuỳ chọn để chịu được bản máy chủ cũ. */
export type MachineStatus = {
  state?: MachineState | string
  /** bước đang chạy: tools · python · source · deps · model */
  step?: string
  /** 0…100; `null`/âm = bước không đo được (UI không vẽ %) */
  percent?: number | null
  bytesDone?: number
  bytesTotal?: number
  etaSec?: number | null
  error?: { code?: string; message?: string; retryInSec?: number; diskFreeBytes?: number } | string | null
  lastSeenAt?: string | null
  /** bản máy chủ: `mode: 'server'`; bản cá nhân: `'local'` */
  mode?: string
  machine?: { name?: string; revokedAt?: string | null; grantedAt?: string | null; capabilities?: string[] } | null
  version?: string
  diskBytes?: number
  rtf?: number | null
  capabilities?: { preview?: boolean } | null
  /** số chương đang xếp hàng chờ máy đọc */
  queueWaiting?: number
  ffmpeg?: boolean
  /** chỉ Nâng cao: kênh nối + 6 dòng nhật ký gần nhất */
  channel?: string
  logTail?: string[]
  /** bản cá nhân: bộ đọc đã cài trên chính máy chủ này */
  engine?: string
  ready?: boolean
  installedBytes?: number
}

export type MachineStepView = {
  key: string
  label: string
  state: 'done' | 'now' | 'wait'
  /** chỉ bước đo được mới có số */
  percent: number | null
  /** dòng phải của danh sách bước: “xong” · “42 %” · rỗng */
  right: string
  note: string
}

export type MachineProgress = { percent: number | null; label: string }

export type MachineView = {
  kind: MachineKind
  mode: MachineMode
  tone: MachineTone
  icon: string
  /** tiêu đề thẻ đầu màn (Transcript · Phòng thu · Nghe) */
  title: string
  /** câu giải thích của thẻ Tuỳ chỉnh › Giọng đọc, không mã lỗi */
  sentence: string
  /** câu giải thích của thẻ đầu màn (Transcript · Phòng thu · Nghe) */
  cardText: string
  /** tiêu đề của thẻ Tuỳ chỉnh (null = dùng `machineName`) */
  settingsTitle: string | null
  /** nhãn trạng thái cạnh tên máy đọc (Tuỳ chỉnh) */
  statusLabel: string
  machineName: string
  chip: { text: string; tone: 'warn' | 'error' } | null
  progress: MachineProgress | null
  spinning: boolean
  steps: MachineStepView[] | null
  actions: MachineAction[]
  /** nút của thẻ đầu màn — bản máy chủ mở thẳng trang Giọng đọc */
  cardActions: MachineAction[]
  queueWaiting: number
  ready: boolean
  /** “1 phút audio ≈ 25 giây” khi đã đo được tốc độ */
  speedLabel: string | null
  diskLabel: string | null
  /** lần kết nối cuối, đã thành câu ngắn (“22:14 hôm qua”) */
  lastSeenLabel: string | null
  /** ba bước “Máy đọc là gì” khi chưa có máy đọc (bản máy chủ) */
  how: string[] | null
  /** dòng phụ của khung “n chương đang chờ máy đọc” ở Phòng thu */
  waitSub: string
}

/** Sáu bước chuẩn bị bộ đọc — thứ tự cố định của máy đọc (10-audio.md §8.1). */
export const MACHINE_STEPS: readonly { key: string; label: string; measurable: boolean }[] = [
  { key: 'tools', label: 'Tải công cụ cài đặt', measurable: false },
  { key: 'python', label: 'Chuẩn bị môi trường chạy', measurable: false },
  { key: 'source', label: 'Tải bộ đọc', measurable: false },
  { key: 'deps', label: 'Cài thư viện', measurable: true },
  { key: 'model', label: 'Tải mẫu giọng', measurable: true },
  { key: 'start', label: 'Khởi động bộ đọc', measurable: false },
]

export type MachineFailure = { title: string; sentence: string; action: MachineActionKind; label: string }

/** Lý do lỗi công khai (10-audio.md §8.2) — mã không ra UI. */
export const MACHINE_FAILURES: Record<string, MachineFailure> = {
  no_disk: {
    title: 'Máy đọc không đủ chỗ trống',
    sentence: 'Cần ít nhất 4 GB trống để chuẩn bị bộ đọc, máy đọc chỉ còn {x} GB. Dọn bớt tệp trên máy đọc rồi thử lại.',
    action: 'retry', label: 'Thử lại',
  },
  download_failed: {
    title: 'Không tải được bộ đọc',
    sentence: 'Mạng của máy đọc bị ngắt giữa chừng. Máy đọc tự thử lại sau {n} phút; bấm Thử lại để thử ngay.',
    action: 'retry', label: 'Thử lại',
  },
  uv_failed: {
    title: 'Không tải được bộ đọc',
    sentence: 'Mạng của máy đọc bị ngắt giữa chừng. Máy đọc tự thử lại sau {n} phút; bấm Thử lại để thử ngay.',
    action: 'retry', label: 'Thử lại',
  },
  hash_mismatch: {
    title: 'Tệp tải về không khớp bản gốc',
    sentence: 'Tệp vừa tải khác bản Noverse đã kiểm nên đã bị bỏ để giữ an toàn. Máy đọc không tự thử tiếp; thử lại khi mạng ổn định, vẫn lỗi thì báo quản trị.',
    action: 'retry', label: 'Thử lại',
  },
  interrupted: {
    title: 'Lượt chuẩn bị bị gián đoạn',
    sentence: 'Máy đọc dừng giữa chừng khi chuẩn bị bộ đọc nên chưa rõ kết quả. Phần đã xong được giữ nguyên; bấm Thử lại để làm tiếp.',
    action: 'retry', label: 'Thử lại',
  },
  deps_failed: {
    title: 'Không chuẩn bị được thư viện',
    sentence: 'Bước cài thư viện cho bộ đọc dừng giữa chừng, thường do mạng chập chờn. Phần đã tải được giữ lại nên lần sau nhanh hơn.',
    action: 'retry', label: 'Thử lại',
  },
  model_failed: {
    title: 'Không tải được mẫu giọng',
    sentence: 'Mẫu giọng (khoảng 580 MB) mới tải được một phần. Phần đã tải được giữ, bấm Thử lại để tải tiếp.',
    action: 'retry', label: 'Thử lại',
  },
  start_failed: {
    title: 'Bộ đọc không khởi động được',
    sentence: 'Bộ đọc đã chuẩn bị xong nhưng không bật lên được, thường do máy đọc thiếu bộ nhớ. Đóng bớt ứng dụng trên máy đọc rồi thử lại.',
    action: 'retry', label: 'Thử lại',
  },
  token_rejected: {
    title: 'Máy đọc đã bị thu hồi quyền',
    sentence: 'Máy đọc này không còn được nhận việc tạo giọng. Quản trị cấp lại quyền ở Nâng cao.',
    action: 'advanced', label: 'Mở Nâng cao',
  },
}

const FALLBACK_FAILURE: MachineFailure = {
  title: 'Chuẩn bị bộ đọc chưa xong',
  sentence: 'Máy đọc dừng giữa chừng khi chuẩn bị bộ đọc. Phần đã xong được giữ nguyên; bấm Thử lại để làm tiếp.',
  action: 'retry', label: 'Thử lại',
}

export const machineFailure = (code: string | null | undefined): MachineFailure =>
  (code && MACHINE_FAILURES[code]) || FALLBACK_FAILURE

/** Mã lỗi từ status — chuỗi cũ (bản máy chủ trước §AT) không phải mã thì bỏ qua. */
export function machineErrorCode(status: MachineStatus | null | undefined): string | null {
  const error = status?.error
  if (!error) return null
  if (typeof error === 'string') return null
  return error.code ?? null
}

/** Bản máy chủ khi status nói về một máy đọc riêng; bản cá nhân khi bộ đọc chạy tại chỗ. */
export function machineMode(status: MachineStatus | null | undefined): MachineMode {
  if (!status) return 'server'
  if (status.mode === 'server') return 'server'
  if (status.mode === 'local') return 'local'
  if (status.machine != null || status.channel != null) return 'server'
  if (status.engine === 'remote') return 'server'
  return 'local'
}

export function machineKind(status: MachineStatus | null | undefined, mode = machineMode(status)): MachineKind {
  const state = status?.state
  if (!status || !state || state === 'missing') return 'noMachine'
  if (state === 'offline') return 'offline'
  if (state === 'provisioning' || state === 'starting' || state === 'installing') return 'provisioning'
  if (state === 'error') return 'provisionFailed'
  // Bản cá nhân: bộ đọc tự tắt sau 10 phút rảnh, vẫn coi là sẵn sàng.
  if (status.ffmpeg === false) return 'missingFfmpeg'
  return 'ready'
}

export function machineName(status: MachineStatus | null | undefined, mode = machineMode(status)): string {
  if (mode === 'local') return 'Máy này'
  return status?.machine?.name?.trim() || 'Máy đọc'
}

const usablePercent = (value: number | null | undefined): number | null => {
  if (value === null || value === undefined) return null
  if (!Number.isFinite(value) || value < 0) return null
  return Math.max(0, Math.min(100, Math.round(value)))
}

const decimal = (value: number) => value.toFixed(1).replace('.', ',')

/** “244 / 581 MB” · “1,4 / 1,4 GB” */
export function formatMachineBytes(done: number | null | undefined, total: number | null | undefined): string {
  if (!done || !total) return ''
  if (total >= 1_073_741_824) return `${decimal(done / 1_073_741_824)} / ${decimal(total / 1_073_741_824)} GB`
  return `${Math.round(done / 1_048_576)} / ${Math.round(total / 1_048_576)} MB`
}

/** Dung lượng đang dùng của bộ đọc. */
export function formatMachineDisk(bytes: number | null | undefined): string | null {
  if (!bytes || bytes <= 0) return null
  return bytes >= 1_073_741_824 ? `${decimal(bytes / 1_073_741_824)} GB` : `${Math.round(bytes / 1_048_576)} MB`
}

/** Tốc độ tạo: “1 phút audio ≈ 25 giây”. */
export function formatMachineSpeed(rtf: number | null | undefined): string | null {
  if (!rtf || rtf <= 0) return null
  return `1 phút audio ≈ ${Math.max(1, Math.round(rtf * 60))} giây`
}

/** Lần kết nối cuối thành câu ngắn; giờ máy người đọc. */
export function formatMachineLastSeen(iso: string | null | undefined, now = new Date()): string | null {
  if (!iso) return null
  const at = new Date(iso)
  if (Number.isNaN(at.getTime())) return null
  const clock = `${String(at.getHours()).padStart(2, '0')}:${String(at.getMinutes()).padStart(2, '0')}`
  const startOfDay = (date: Date) => new Date(date.getFullYear(), date.getMonth(), date.getDate()).getTime()
  const days = Math.round((startOfDay(now) - startOfDay(at)) / 86_400_000)
  if (days <= 0) return clock
  if (days === 1) return `${clock} hôm qua`
  return `${clock} · ${at.getDate()}/${at.getMonth() + 1}`
}

/** Chỉ số bước đang chạy 0…5; -1 khi chưa rõ. */
export function machineStepIndex(status: MachineStatus | null | undefined): number {
  const state = status?.state
  if (state === 'starting') return MACHINE_STEPS.length - 1
  const index = MACHINE_STEPS.findIndex((step) => step.key === status?.step)
  return index
}

function stepNote(status: MachineStatus, index: number, percent: number | null): string {
  const step = MACHINE_STEPS[index]
  if (!step) return ''
  if (!step.measurable) {
    if (step.key === 'start') return 'Đang bật bộ đọc lên…'
    if (step.key === 'source') return 'Đang tải bộ đọc từ nguồn đã kiểm…'
    return `Đang ${step.label.toLocaleLowerCase('vi-VN')}…`
  }
  const parts: string[] = []
  if (percent !== null) parts.push(`${percent}%`)
  if (step.key === 'model') {
    const bytes = formatMachineBytes(status.bytesDone, status.bytesTotal)
    if (bytes) parts.push(bytes)
    if (status.etaSec && status.etaSec > 0) parts.push(`còn khoảng ${Math.max(1, Math.round(status.etaSec / 60))} phút`)
  }
  return parts.join(' · ')
}

/** Danh sách 6 bước khi đang chuẩn bị — bước không đo được thì chỉ có vòng xoay. */
export function machineSteps(status: MachineStatus | null | undefined): MachineStepView[] {
  const at = machineStepIndex(status)
  const percent = usablePercent(status?.percent)
  return MACHINE_STEPS.map((step, index) => {
    const state: MachineStepView['state'] = at < 0 ? 'wait' : index < at ? 'done' : index === at ? 'now' : 'wait'
    const measured = step.measurable && state === 'now' ? percent : null
    return {
      key: step.key,
      label: step.label,
      state,
      percent: measured,
      right: state === 'done' ? 'xong' : measured !== null ? `${measured}%` : '',
      note: state === 'now' ? stepNote(status ?? {}, index, measured) : '',
    }
  })
}

/** Dòng tiến độ của thẻ đầu màn: % chỉ hiện ở bước đo được. */
export function machineProgress(status: MachineStatus | null | undefined): MachineProgress | null {
  const at = machineStepIndex(status)
  if (at < 0) return null
  const step = MACHINE_STEPS[at]
  if (!step.measurable) return null
  const percent = usablePercent(status?.percent)
  if (percent === null) return null
  return { percent, label: stepNote(status ?? {}, at, percent) }
}

/** “Bước 5/6 · Tải mẫu giọng · 42%” — dòng phụ của Tuỳ chỉnh và của dock. */
export function machineStepLine(status: MachineStatus | null | undefined): string {
  const at = machineStepIndex(status)
  if (at < 0) return 'Đang chuẩn bị bộ đọc…'
  const step = MACHINE_STEPS[at]
  const percent = step.measurable ? usablePercent(status?.percent) : null
  return [`Bước ${at + 1}/${MACHINE_STEPS.length}`, step.label, percent !== null ? `${percent}%` : ''].filter(Boolean).join(' · ')
}

/**
 * Hàng “Chuẩn bị bộ đọc” của dock: chỉ bước đo được mới có %, bước còn lại là vòng xoay
 * và một dòng chữ (10-audio.md §8.6). `step` là khoá bước thô từ job.
 */
export function prepLine(step: string | undefined, percent: number | null | undefined, bytesDone?: number, bytesTotal?: number): {
  line: string
  percent: number | null
} {
  const index = MACHINE_STEPS.findIndex((item) => item.key === step)
  if (index < 0) return { line: 'Đang chuẩn bị bộ đọc…', percent: null }
  const meta = MACHINE_STEPS[index]
  const measured = meta.measurable ? usablePercent(percent) : null
  const bytes = meta.key === 'model' ? formatMachineBytes(bytesDone, bytesTotal) : ''
  return {
    line: [`Bước ${index + 1}/${MACHINE_STEPS.length}`, meta.label, measured !== null ? `${measured}%` : '', bytes].filter(Boolean).join(' · '),
    percent: measured,
  }
}

const EMPTY_VIEW: Omit<MachineView, 'kind' | 'mode' | 'tone' | 'title' | 'sentence' | 'cardText' | 'settingsTitle' | 'statusLabel' | 'actions' | 'cardActions'> = {
  icon: 'headphone', machineName: 'Máy đọc', chip: null, progress: null, spinning: false,
  steps: null, queueWaiting: 0, ready: false, speedLabel: null, diskLabel: null, lastSeenLabel: null, how: null,
  waitSub: '',
}

/** Dòng phụ của khung “n chương đang chờ máy đọc” (Phòng thu, §8.6). */
function waitingSub(kind: MachineKind, mode: MachineMode, lastSeen: string | null, progress: MachineProgress | null): string {
  if (kind === 'noMachine') {
    return mode === 'local'
      ? 'Máy này chưa có bộ đọc. Chương đã chốt bản đọc, chỉ còn chờ máy tạo giọng.'
      : 'Chưa có máy đọc. Chương đã chốt bản đọc, chỉ còn chờ máy đọc tạo giọng.'
  }
  if (kind === 'offline') {
    return `${lastSeen ? `Máy đọc chưa kết nối từ ${lastSeen}.` : 'Máy đọc chưa kết nối.'} Máy thức dậy là các chương tự chạy theo thứ tự.`
  }
  if (kind === 'provisioning') {
    return `Máy đọc đang chuẩn bị bộ đọc lần đầu${progress ? ` · ${progress.percent}%` : ''}. Xong là các chương tự chạy.`
  }
  if (kind === 'provisionFailed') return 'Chuẩn bị bộ đọc chưa xong. Bấm Thử lại ở thẻ phía trên; các chương vẫn giữ chỗ.'
  return ''
}

/**
 * Trạng thái → khung nhìn. `options.waitN` ghi đè số chương đang chờ khi nơi gọi biết rõ hơn
 * status (Phòng thu đếm từ hàng đợi).
 */
export function machineView(status: MachineStatus | null | undefined, options: { waitN?: number } = {}): MachineView {
  const mode = machineMode(status)
  const kind = machineKind(status, mode)
  const name = machineName(status, mode)
  const waiting = Math.max(0, Math.round(options.waitN ?? status?.queueWaiting ?? 0))
  const view: MachineView = {
    ...EMPTY_VIEW,
    kind, mode, tone: 'neutral', title: '', sentence: '', cardText: '', settingsTitle: null, statusLabel: '', actions: [], cardActions: [],
    machineName: name, queueWaiting: waiting,
    speedLabel: formatMachineSpeed(status?.rtf),
    diskLabel: formatMachineDisk(status?.diskBytes ?? status?.installedBytes),
    lastSeenLabel: formatMachineLastSeen(status?.lastSeenAt),
    waitSub: waitingSub(kind, mode, formatMachineLastSeen(status?.lastSeenAt), machineProgress(status)),
  }

  if (kind === 'noMachine') {
    const local = mode === 'local'
    return {
      ...view,
      tone: 'neutral',
      title: local ? 'Máy này chưa có bộ đọc' : 'Chưa có máy đọc để tạo giọng',
      sentence: local
        ? 'Chuẩn bị một lần để tạo giọng ngay trên máy này: cần khoảng 1,5 GB, lúc chuẩn bị cần 4 GB trống. Bản đọc vẫn soạn được trong lúc chờ.'
        : 'Máy đọc là một máy tính luôn bật (ví dụ Mac ở nhà) đọc thành tiếng cho mọi truyện trong thư viện. Bật một lần ở Nâng cao; máy đọc tự chuẩn bị bộ đọc khi chưa có.',
      cardText: local
        ? 'Bạn vẫn soạn và chốt bản đọc được. Chương bấm Tạo giọng sẽ xếp hàng và tự chạy khi máy này có bộ đọc.'
        : 'Bạn vẫn nghe được chương đã có giọng. Chương bấm Tạo giọng sẽ chờ tới khi quản trị bật máy đọc.',
      settingsTitle: local ? 'Máy này chưa có bộ đọc' : 'Chưa có máy đọc nào',
      statusLabel: local ? 'Chưa chuẩn bị' : 'Chưa có máy đọc',
      actions: local ? [{ label: 'Chuẩn bị bộ đọc', kind: 'prepare' }] : [{ label: 'Cách bật máy đọc', kind: 'advanced' }],
      cardActions: local ? [{ label: 'Chuẩn bị bộ đọc', kind: 'prepare' }] : [{ label: 'Xem Giọng đọc', kind: 'settings' }],
      how: local ? null : [
        'Quản trị cấp quyền máy đọc ở Nâng cao (một lần).',
        'Máy đọc tự chuẩn bị bộ đọc, khoảng 10 phút lần đầu.',
        'Xong là chương đang chờ tự tạo giọng.',
      ],
    }
  }

  if (kind === 'offline') {
    const last = view.lastSeenLabel
    return {
      ...view,
      tone: 'neutral',
      title: 'Máy đọc chưa kết nối',
      sentence: `${last ? `Lần cuối kết nối ${last}. ` : ''}Có thể máy đang ngủ hoặc tắt — không phải lỗi. Chương đang chờ sẽ tự chạy khi máy đọc kết nối lại.`,
      cardText: `${last ? `Lần cuối ${last}. ` : ''}Có thể máy đang ngủ hoặc tắt; chương đang chờ sẽ tự chạy khi máy đọc kết nối lại.`,
      statusLabel: 'Chưa kết nối',
      chip: waiting > 0 ? { text: `${waiting} chương đang chờ`, tone: 'warn' } : null,
    }
  }

  if (kind === 'provisioning') {
    const at = machineStepIndex(status)
    const step = at >= 0 ? MACHINE_STEPS[at] : null
    const progress = machineProgress(status)
    return {
      ...view,
      tone: 'info',
      title: step ? `Đang chuẩn bị bộ đọc · ${step.label}` : 'Đang chuẩn bị bộ đọc',
      sentence: mode === 'local'
        ? 'Lần đầu mất khoảng 10 phút. Bạn vẫn soạn bản đọc được; chương bấm Tạo giọng sẽ chờ tới khi xong.'
        : 'Lần đầu mất khoảng 10 phút. Bạn vẫn soạn bản đọc được; chương bấm Tạo giọng sẽ chờ tới khi xong.',
      cardText: 'Lần đầu mất khoảng 10 phút. Chương bấm Tạo giọng sẽ chờ tới khi xong.',
      statusLabel: 'Đang chuẩn bị',
      progress,
      spinning: progress === null,
      steps: machineSteps(status),
    }
  }

  if (kind === 'provisionFailed') {
    const failure = machineFailure(machineErrorCode(status))
    const error = typeof status?.error === 'object' && status.error ? status.error : null
    const disk = formatMachineDisk(error?.diskFreeBytes)
    const retryMin = error?.retryInSec && error.retryInSec > 0 ? Math.max(1, Math.round(error.retryInSec / 60)) : null
    // Số liệu máy chủ chưa gửi thì bỏ mệnh đề số, không bịa — câu vẫn đúng 8.2.
    const sentence = failure.sentence
      .replace(disk ? '{x}' : ', máy đọc chỉ còn {x} GB', disk ?? '')
      .replace('{n}', retryMin !== null ? String(retryMin) : 'vài')
    return {
      ...view,
      tone: 'danger',
      icon: 'warning-2',
      title: failure.title,
      sentence,
      cardText: sentence,
      settingsTitle: failure.title,
      statusLabel: 'Chuẩn bị chưa xong',
      chip: { text: 'Bắt sửa', tone: 'error' },
      actions: [{ label: failure.label, kind: failure.action }],
      cardActions: [{ label: failure.label, kind: failure.action }],
    }
  }

  if (kind === 'missingFfmpeg') {
    return {
      ...view,
      tone: 'warn',
      icon: 'export-square',
      title: 'Chưa ghép và xuất tập được',
      sentence: 'Máy chủ đang thiếu công cụ ghép âm thanh. Tạo giọng và nghe từng chương vẫn chạy bình thường; nhờ quản trị bổ sung để ghép tập.',
      cardText: 'Máy chủ đang thiếu công cụ ghép âm thanh. Tạo giọng và nghe từng chương vẫn chạy bình thường; nhờ quản trị bổ sung để ghép tập.',
      statusLabel: 'Thiếu công cụ ghép',
      chip: { text: 'Cảnh báo', tone: 'warn' },
    }
  }

  return {
    ...view,
    tone: 'ok',
    title: name,
    sentence: mode === 'local'
      ? 'Bộ đọc chạy trên máy này, chữ không gửi đi đâu. Tự tắt sau 10 phút rảnh để nhả bộ nhớ.'
      : 'Máy đọc nhận việc tạo giọng của mọi truyện. Chữ chỉ đi giữa máy chủ và máy đọc của bạn.',
    statusLabel: 'Sẵn sàng',
    ready: true,
  }
}

/** Nghe thử giọng: bản cá nhân luôn có; bản máy chủ chỉ khi máy đọc báo hỗ trợ. */
export function previewSupport(status: MachineStatus | null | undefined): 'ready' | 'hidden' | 'waiting' {
  if (machineMode(status) === 'local') return 'ready'
  if (!status?.capabilities?.preview) return 'hidden'
  return 'ready'
}

// ─── Nâng cao › Máy đọc · chi tiết kỹ thuật (10-audio.md §8.5) ───
// Chỗ duy nhất được hiện tên công cụ, đường dẫn, mã lỗi và nhật ký.

export type MachineAdvancedRow = { label: string; value: string; note?: string }

export type MachineAdvanced = {
  /** nhãn cạnh tiêu đề thẻ */
  statusLabel: string
  /** có máy đọc để hiện bảng chi tiết */
  has: boolean
  /** chưa có máy đọc (bản máy chủ): hướng dẫn + lệnh bật một lần */
  setup: boolean
  canRevoke: boolean
  rows: MachineAdvancedRow[]
  /** 6 dòng nhật ký gần nhất, mỗi dòng một dòng chữ */
  log: string
  setupText: string
  commands: string
}

/** Thời điểm tuyệt đối “2026-11-01 21:52:07” của Nâng cao. */
export function formatMachineStamp(iso: string | null | undefined): string {
  if (!iso) return ''
  const at = new Date(iso)
  if (Number.isNaN(at.getTime())) return ''
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${at.getFullYear()}-${pad(at.getMonth() + 1)}-${pad(at.getDate())} ${pad(at.getHours())}:${pad(at.getMinutes())}:${pad(at.getSeconds())}`
}

/** Khoảng cách tới lần kết nối cuối bằng lời (“12 giây trước” · “hôm qua”). */
export function machineSeenNote(iso: string | null | undefined, now = new Date()): string {
  if (!iso) return ''
  const at = new Date(iso)
  if (Number.isNaN(at.getTime())) return ''
  const seconds = Math.round((now.getTime() - at.getTime()) / 1000)
  if (seconds < 0) return ''
  if (seconds < 60) return `${seconds} giây trước`
  const minutes = Math.round(seconds / 60)
  if (minutes < 60) return `${minutes} phút trước`
  const startOfDay = (date: Date) => new Date(date.getFullYear(), date.getMonth(), date.getDate()).getTime()
  const days = Math.round((startOfDay(now) - startOfDay(at)) / 86_400_000)
  if (days <= 0) return `${Math.round(minutes / 60)} giờ trước`
  if (days === 1) return 'hôm qua'
  return `${days} ngày trước`
}

/** Giá trị kỹ thuật gửi về — mono, chú thích tiếng Việt chữ thường bên cạnh. */
function reportedLine(status: MachineStatus): string {
  const state = status.state || 'missing'
  if (state === 'provisioning' || state === 'starting') {
    const at = machineStepIndex(status)
    const percent = usablePercent(status.percent)
    const step = at >= 0 ? MACHINE_STEPS[at].key : ''
    return [state, step, percent !== null ? `${percent}%` : ''].filter(Boolean).join(' · ')
  }
  if (state === 'error') return ['error', machineErrorCode(status) ?? ''].filter(Boolean).join(' · ')
  if (state === 'offline') return 'ready'
  if (status.rtf) return `ready · rtf ${String(Math.round(status.rtf * 100) / 100)}`
  return 'ready'
}

const REVOKED_LABEL = 'Đã thu hồi'

export function machineAdvanced(status: MachineStatus | null | undefined, now = new Date()): MachineAdvanced {
  const mode = machineMode(status)
  const view = machineView(status, {})
  const revoked = Boolean(status?.machine?.revokedAt)
  const local = mode === 'local'
  const kind = view.kind
  const has = Boolean(status) && kind !== 'noMachine' && !local
  const setup = Boolean(status) && kind === 'noMachine' && !local
  const disk = formatMachineDisk(status?.diskBytes ?? status?.installedBytes)
  const rows: MachineAdvancedRow[] = []
  if (has && status) {
    const name = status.machine?.name?.trim() || 'Máy đọc'
    const seen = formatMachineStamp(status.lastSeenAt)
    const seenNote = machineSeenNote(status.lastSeenAt, now)
    rows.push({ label: 'Tên máy đọc', value: name })
    rows.push({ label: 'Lần cuối kết nối', value: seen || '—', note: seen ? seenNote : 'chưa lần nào' })
    rows.push({ label: 'Phiên bản bộ đọc', value: status.version?.trim() || '—' })
    rows.push({ label: 'Dung lượng', value: disk ?? '—', note: local ? 'trên máy này' : '' })
    if (status.channel?.trim()) rows.push({ label: 'Kênh nối', value: status.channel.trim(), note: 'chỉ trong máy chủ' })
    const caps = status.machine?.capabilities?.filter(Boolean).join(' · ') || 'tts'
    const granted = formatMachineStamp(status.machine?.grantedAt)
    const revokedAt = formatMachineStamp(status.machine?.revokedAt)
    rows.push({
      label: 'Quyền', value: caps,
      note: revoked ? `đã thu hồi ${revokedAt || ''}`.trim() : granted ? `cấp ${granted}` : '',
    })
    rows.push({
      label: 'Trạng thái gửi về', value: reportedLine(status),
      note: kind === 'offline' && !revoked ? 'cũ · offline suy từ lease' : '',
    })
  }
  return {
    statusLabel: revoked ? REVOKED_LABEL : view.statusLabel,
    has,
    setup,
    canRevoke: has && !revoked,
    rows,
    log: (status?.logTail ?? []).slice(0, 6).join('\n'),
    setupText: 'Chưa có máy đọc nào kết nối. Chạy trên máy đọc (một lần), rồi máy đọc tự chuẩn bị bộ đọc:',
    commands: [
      'ainovel-cli worker token create --cap tts mac-mini',
      '# lưu vào ~/server/apps/noverse/tts-worker.env (quyền 0600)',
      'launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/com.dat.noverse-tts.plist',
    ].join('\n'),
  }
}
