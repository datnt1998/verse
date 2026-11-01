// Mock máy đọc (Gói AT · 10-audio.md §8) — trạng thái, 6 bước chuẩn bị, hàng dock “Chuẩn bị bộ đọc”.
// AINOVEL_MOCK_TTS:
//   (rỗng)      máy đọc riêng đã sẵn sàng (bản máy chủ)
//   local       bản cá nhân đã sẵn sàng — tên “Máy này”
//   missing     chưa có máy đọc · localmissing  chưa chuẩn bị (bản cá nhân)
//   offline     máy đọc tắt, 5 chương đang chờ
//   installing  đang chuẩn bị, bước Tải mẫu giọng 42 % (đo được)
//   provsource  đang chuẩn bị, bước Tải bộ đọc (không đo được → vòng xoay)
//   error[:mã]  chuẩn bị chưa xong với mã 8.2 (mặc định model_failed)
//   revoked     máy đọc đã bị thu hồi quyền (token_rejected)
//   installfail lượt chuẩn bị hỏng ở 40 % (dock: Thử lại + Huỷ)
//   prep · prepstart · prepdone · preperr  bốn kịch bản hàng dock (Jobs.dc.html)
//   nopreview   máy đọc không hỗ trợ nghe thử (capabilities.preview=false)
// AINOVEL_MOCK_FFMPEG=0 → thiếu công cụ ghép (thẻ missingFfmpeg).
import type { TtsEngine, TtsInstallProgress, TtsStatus } from '../src/api/types'
import { BUILTIN_VOICES } from './audio'
import { mirrorVoiceInstallJob } from './jobs'

/** Kịch bản máy đọc — đổi lúc chạy bằng POST /api/__mock/tts để chụp đủ trạng thái. */
let scenario = process.env.AINOVEL_MOCK_TTS ?? ''
const FFMPEG = process.env.AINOVEL_MOCK_FFMPEG !== '0'
const failCode = () => (scenario.startsWith('error:') ? scenario.slice('error:'.length) : '')

const MACHINE_NAME = 'Mac mini ở nhà'
const MODEL_BYTES = 581 * 1_048_576

/** Sáu bước chuẩn bị bộ đọc — cùng thứ tự với máy chủ thật. */
const STEPS: { key: string; label: string; measured: boolean }[] = [
  { key: 'tools', label: 'Tải công cụ cài đặt', measured: false },
  { key: 'python', label: 'Chuẩn bị môi trường chạy', measured: false },
  { key: 'source', label: 'Tải bộ đọc', measured: false },
  { key: 'deps', label: 'Cài thư viện', measured: true },
  { key: 'model', label: 'Tải mẫu giọng', measured: true },
  { key: 'start', label: 'Khởi động bộ đọc', measured: false },
]

const humanBytes = (bytes: number) =>
  bytes >= 1 << 30 ? `${Math.round(bytes / (1 << 30))} GB` : `${Math.round(bytes / (1 << 20))} MB`

/** Dòng bước của dock — “Bước 5/6 · Tải mẫu giọng · 299/581 MB”; % nằm ở cột phải. */
export function prepStepLine(progress: Pick<TtsInstallProgress, 'step' | 'percent' | 'bytesDone' | 'bytesTotal'>): string {
  const index = STEPS.findIndex((step) => step.key === progress.step)
  if (index < 0) return ''
  const step = STEPS[index]
  const line = [`Bước ${index + 1}/${STEPS.length}`, step.label]
  if (step.key === 'model' && progress.bytesDone > 0 && progress.bytesTotal > 0) {
    line.push(`${humanBytes(progress.bytesDone)}/${humanBytes(progress.bytesTotal)}`)
  }
  return line.join(' · ')
}

type Prep = Pick<TtsInstallProgress, 'step' | 'percent' | 'bytesDone' | 'bytesTotal' | 'etaSec'>

/** Tiến độ 0..1 → bước đang chạy; ba bước đầu và bước cuối không đo được (không có % giả). */
function prepAt(value: number): Prep {
  const p = Math.max(0, Math.min(1, value))
  if (p < 0.08) return { step: 'tools', percent: null, bytesDone: 0, bytesTotal: 0, etaSec: 0 }
  if (p < 0.15) return { step: 'python', percent: null, bytesDone: 0, bytesTotal: 0, etaSec: 0 }
  if (p < 0.25) return { step: 'source', percent: null, bytesDone: 0, bytesTotal: 0, etaSec: 0 }
  if (p < 0.65) {
    const done = (p - 0.25) / 0.4
    return { step: 'deps', percent: Math.round(done * 100), bytesDone: 0, bytesTotal: 0, etaSec: Math.round((1 - p) * 600) }
  }
  if (p < 1) {
    const done = (p - 0.65) / 0.35
    return {
      step: 'model', percent: Math.round(done * 100),
      bytesDone: Math.round(done * MODEL_BYTES), bytesTotal: MODEL_BYTES,
      etaSec: Math.max(30, Math.round((1 - p) * 600)),
    }
  }
  return { step: 'start', percent: null, bytesDone: MODEL_BYTES, bytesTotal: MODEL_BYTES, etaSec: 20 }
}

/** Kịch bản tĩnh: bốn hàng dock + các trạng thái thẻ. */
const STATIC_PREP: Record<string, { prep: Prep; state: TtsInstallProgress['state']; code?: string }> = {
  prep: { prep: { step: 'model', percent: 79, bytesDone: 232 * 1_048_576, bytesTotal: MODEL_BYTES, etaSec: 120 }, state: 'running' },
  prepstart: { prep: { step: 'source', percent: null, bytesDone: 0, bytesTotal: 0, etaSec: 0 }, state: 'running' },
  prepdone: { prep: { step: 'start', percent: null, bytesDone: MODEL_BYTES, bytesTotal: MODEL_BYTES, etaSec: 0 }, state: 'done' },
  preperr: { prep: { step: 'model', percent: 42, bytesDone: 244 * 1_048_576, bytesTotal: MODEL_BYTES, etaSec: 240 }, state: 'error', code: 'model_failed' },
  installfail: { prep: { step: 'model', percent: 40, bytesDone: 232 * 1_048_576, bytesTotal: MODEL_BYTES, etaSec: 300 }, state: 'error', code: 'model_failed' },
}

const serverMachine = (revoked = false): NonNullable<TtsStatus['machine']> => ({
  name: MACHINE_NAME,
  revokedAt: revoked ? new Date().toISOString() : null,
  grantedAt: '2026-10-28T09:12:00+07:00',
  capabilities: ['tts'],
})

/** §AT — kịch bản máy đọc bản máy chủ cho thẻ đầu màn (MAudio `machine` · MVoiceGen `gk=waiting|waitPrep`).
 * Mọi trường đều là trường thật của DTO §AT; kịch bản địa phương (`installing`, `missing`, `ready`) không đổi. */
export function applyMachineDemo(next: string): boolean {
  const hosted: Partial<TtsStatus> = {
    channel: 'hosted', machine: serverMachine(), queueWaiting: 3,
    lastSeenAt: null, logTail: undefined, version: '', diskBytes: 0,
    capabilities: { preview: false }, ffmpeg: true,
  }
  switch (next) {
    case 'mobileMachineNone':
      status = { ...status, ready: false, state: 'missing', rtf: null, installedBytes: 0, error: null, ...hosted, machine: null, queueWaiting: 0 }
      return true
    case 'mobileMachineOffline':
    case 'mobileWait':
      status = { ...status, ready: false, state: 'offline', rtf: null, error: null, ...hosted,
        lastSeenAt: new Date(Date.now() - 26 * 3_600_000).toISOString(), queueWaiting: 3 }
      return true
    case 'mobileMachinePrep':
    case 'mobileWaitPrep':
      status = { ...status, ready: false, state: 'provisioning', step: 'model', percent: 42, bytesDone: 244 * 1_048_576, bytesTotal: MODEL_BYTES, etaSec: 240, error: null, ...hosted }
      return true
    case 'mobileMachineFail':
      status = { ...status, ready: false, state: 'error', rtf: null,
        error: { code: 'no_disk', message: 'Máy đọc không đủ chỗ trống để chuẩn bị bộ đọc' }, ...hosted }
      return true
    default:
      return false
  }
}

/** Gỡ kịch bản máy đọc của lượt demo trước: dựng lại đúng trạng thái gốc của máy chủ mock. */
export const resetMachineDemo = () => {
  status = seedStatus()
}

const seedStatus = (): TtsStatus => {
  const base: TtsStatus = {
    ready: true, engine: 'vieneu', model: 'VieNeu v3 Turbo', device: 'cpu', quality: 'standard',
    cloning: true, qualities: ['fast', 'standard', 'high'],
    state: 'ready', error: null, rtf: 0.68, sampleRate: 48000, ffmpeg: FFMPEG, installedBytes: 1_536_000_000,
    mode: 'server', step: '', percent: null, bytesDone: 0, bytesTotal: 0, etaSec: 0,
    lastSeenAt: new Date().toISOString(), machine: serverMachine(), version: 'VieNeu-TTS v3.8.3 · ainovel-cli 0.9.3',
    diskBytes: 1_526_000_000, capabilities: { preview: scenario !== 'nopreview' }, queueWaiting: 0,
    installed: true, channel: 'http://127.0.0.1:8081',
    logTail: [
      '21:52:07  heartbeat ok · state=ready · rtf=0.41',
      '21:49:51  claim job=voice/ch4 · 46 segments',
      '21:49:50  engine up · port=52114 · onnx cpu fp32',
      '21:48:12  idle 10m → engine stopped',
      '21:31:02  ack job=voice/ch3 · 17:12 audio',
      '21:30:58  upload ch3.m4a · 15.8 MB',
    ],
  }
  const local: TtsStatus = {
    ...base, mode: 'local', machine: null, channel: undefined, logTail: undefined, version: '',
    lastSeenAt: null,
  }
  if (scenario === 'local') return local
  if (scenario === 'localmissing') {
    return { ...local, ready: false, state: 'missing', rtf: null, installedBytes: 0, diskBytes: 0, installed: false }
  }
  if (scenario === 'missing') {
    return { ...base, ready: false, state: 'missing', rtf: null, installedBytes: 0, diskBytes: 0, machine: null, lastSeenAt: null, installed: false }
  }
  if (scenario === 'offline') {
    return {
      ...base, ready: false, state: 'offline', rtf: null, queueWaiting: 5,
      lastSeenAt: '2026-10-31T22:14:03+07:00',
      logTail: [
        '22:14:03  heartbeat ok · state=ready',
        '22:13:33  heartbeat ok · state=ready',
        '22:12:51  ack job=voice/ch2 · 18:05 audio',
        '22:02:40  claim job=voice/ch2',
        '21:52:07  heartbeat ok · rtf=0.41',
        '--:--:--  no heartbeat since 22:14:03 (lease 90s)',
      ],
    }
  }
  if (scenario === 'installing') {
    return { ...base, ready: false, state: 'provisioning', rtf: null, installed: false, queueWaiting: 3, ...prepAt(0.74) }
  }
  if (scenario === 'provsource') {
    return { ...base, ready: false, state: 'provisioning', rtf: null, installed: false, ...prepAt(0.18) }
  }
  if (scenario === 'revoked') {
    return {
      ...base, ready: false, state: 'error', rtf: null, installed: false,
      error: { code: 'token_rejected', message: 'Máy đọc đã bị thu hồi quyền' },
      machine: serverMachine(true),
    }
  }
  if (scenario === 'error' || failCode()) {
    const code = failCode() || 'model_failed'
    return {
      ...base, ready: false, state: 'error', rtf: null, installed: false, diskBytes: 640 * 1_048_576,
      error: { code, message: 'Chuẩn bị bộ đọc chưa xong' },
      ...prepAt(0.74),
    }
  }
  if (scenario === 'installfail') {
    return {
      ...base, ready: false, state: 'error', rtf: null, installed: false,
      error: { code: 'model_failed', message: 'Không tải được mẫu giọng' },
      ...prepAt(0.74),
    }
  }
  if (scenario in STATIC_PREP) {
    const staticPrep = STATIC_PREP[scenario]
    if (staticPrep.state === 'running') {
      return { ...base, ready: false, state: 'provisioning', rtf: null, installed: false, ...staticPrep.prep }
    }
    if (staticPrep.state === 'error') {
      return {
        ...base, ready: false, state: 'error', rtf: null, installed: false,
        error: { code: staticPrep.code ?? 'model_failed', message: 'Chuẩn bị bộ đọc chưa xong' },
        ...staticPrep.prep,
      }
    }
    return base
  }
  return base
}

let status = seedStatus()
let timer: NodeJS.Timeout | null = null
let installProgress = 0
let installState: TtsInstallProgress['state'] =
  scenario in STATIC_PREP ? STATIC_PREP[scenario].state : scenario === 'installing' ? 'running' : 'idle'
let installError = ''
let installErrorCode = scenario in STATIC_PREP ? STATIC_PREP[scenario].code ?? '' : ''
let installUpdatedAt = new Date().toISOString()

export const ttsStatus = () => status
/** ID hàng của dock — cùng ID với máy chủ thật (internal/entry/web/voice_jobs.go). */
export const VOICE_INSTALL_JOB = 'voice/install'

export const ttsInstallProgress = (): TtsInstallProgress => {
  const prep = scenario in STATIC_PREP ? STATIC_PREP[scenario].prep : prepAt(installProgress)
  return {
    state: installState,
    step: prep.step,
    percent: installState === 'done' ? 100 : prep.percent,
    bytesDone: prep.bytesDone,
    bytesTotal: prep.bytesTotal,
    etaSec: prep.etaSec ?? 0,
    message: installState === 'error'
      ? 'Chuẩn bị bộ đọc chưa xong'
      : installState === 'done'
        ? `Bộ đọc đã sẵn sàng trên ${MACHINE_NAME}`
        : '',
    ...(installError ? { error: installError } : {}),
    ...(installErrorCode ? { errorCode: installErrorCode } : {}),
    updated: installUpdatedAt,
  }
}

const publish = () => {
  installUpdatedAt = new Date().toISOString()
  const progress = ttsInstallProgress()
  mirrorVoiceInstallJob(progress, (action) => {
    if (action === 'retry') installTts()
  }, prepStepLine(progress))
}

const tick = () => {
  installProgress = Math.min(1, installProgress + 0.035)
  const prep = prepAt(installProgress)
  status = { ...status, state: 'provisioning', ready: false, ...prep }
  if (installProgress >= 1) {
    status = {
      ...status, ready: true, state: 'ready', rtf: 0.68, installedBytes: 1_536_000_000,
      diskBytes: 1_526_000_000, percent: null, step: 'start', installed: true,
      lastSeenAt: new Date().toISOString(),
    }
    installState = 'done'
    installError = ''
    installErrorCode = ''
    clearInterval(timer ?? undefined)
    timer = null
  }
  publish()
}

/** POST /api/tts/install — máy đọc chuẩn bị bộ đọc (mock: chạy hết 6 bước trong ~12 giây). */
export const installTts = () => {
  if (installState === 'running' || status.ready) return
  installProgress = 0
  status = { ...status, state: 'provisioning', ready: false, error: null, ...prepAt(0) }
  installState = 'running'
  installError = ''
  installErrorCode = ''
  clearInterval(timer ?? undefined)
  timer = setInterval(tick, 420)
  publish()
}

/** §8.5 — thu hồi quyền máy đọc (một bước, không mất dữ liệu). */
export const revokeMachine = () => {
  if (!status.machine) return null
  const name = status.machine.name
  status = {
    ...status, ready: false, state: 'error', rtf: null,
    error: { code: 'token_rejected', message: 'Máy đọc đã bị thu hồi quyền' },
    machine: { ...status.machine, revokedAt: new Date().toISOString() },
  }
  return name
}

export const ttsEngines = (): TtsEngine[] => [
  { id: 'vieneu', name: 'VieNeu', description: 'Chạy trên máy đọc · giọng tiếng Việt · clone từ mẫu 10–30 giây', installed: status.state !== 'missing' },
  { id: 'openai-speech', name: 'Máy chủ giọng đọc khác', description: 'Máy chủ giọng đọc tương thích OpenAI Speech', installed: false },
]

export const ttsVoices = () => BUILTIN_VOICES.map(({ file: _file, durationSec: _durationSec, ...voice }) => ({ ...voice, source: 'builtin', builtIn: true }))

export const resetTts = () => {
  clearInterval(timer ?? undefined)
  timer = null
  status = seedStatus()
  installProgress = scenario === 'installing' ? 0.74 : 0
  installState = scenario in STATIC_PREP ? STATIC_PREP[scenario].state : scenario === 'installing' ? 'running' : 'idle'
  installError = ''
  installErrorCode = scenario in STATIC_PREP ? STATIC_PREP[scenario].code ?? '' : ''
  publish()
}

/** POST /api/__mock/tts {scenario} — đổi trạng thái máy đọc mà không cần dựng lại máy chủ. */
export const setTtsScenario = (next: string) => {
  scenario = next
  resetTts()
  return status
}

// Kịch bản tĩnh có sẵn hàng dock ngay khi mở app.
if (scenario in STATIC_PREP) publish()
