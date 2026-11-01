// Mock bộ đọc trên máy (GAP-21) — trạng thái + tiến độ cài + nghe thử.
// AINOVEL_MOCK_TTS=missing|installing|error để xem thẻ “Cài bộ đọc trên máy”;
// AINOVEL_MOCK_TTS=installfail để xem lượt cài hỏng (dock hiện lỗi + Thử lại);
// AINOVEL_MOCK_FFMPEG=0 để xem trạng thái thiếu ffmpeg.
import type { TtsEngine, TtsInstallProgress, TtsStatus } from '../src/api/types'
import { BUILTIN_VOICES } from './audio'
import { mirrorVoiceInstallJob } from './jobs'

const ENV_TTS = process.env.AINOVEL_MOCK_TTS ?? ''
const FFMPEG = process.env.AINOVEL_MOCK_FFMPEG !== '0'
/** Lượt cài hỏng ở 40% (mạng bị chặn) để thử nhánh lỗi + Thử lại của dock. */
const FAIL_INSTALL = ENV_TTS === 'installfail'

const seedStatus = (): TtsStatus => {
  const base: TtsStatus = {
    ready: true,
    engine: 'vieneu',
    model: 'VieNeu v3 Turbo',
    device: 'cpu',
    state: 'ready',
    error: null,
    rtf: 0.68,
    sampleRate: 48000,
    ffmpeg: FFMPEG,
    installedBytes: 1_536_000_000,
    mode: 'local',
    step: '',
    percent: null,
    bytesDone: 0,
    bytesTotal: 0,
    etaSec: 0,
    lastSeenAt: null,
    machine: null,
    version: '',
    diskBytes: 1_536_000_000,
    capabilities: { preview: true },
    queueWaiting: 0,
    installed: true,
  }
  if (ENV_TTS === 'missing' || FAIL_INSTALL) return { ...base, ready: false, state: 'missing', rtf: null, installedBytes: 0 }
  if (ENV_TTS === 'error') {
    return { ...base, ready: false, state: 'error', error: { code: 'start_failed', message: 'Không khởi động được bộ đọc trên máy' }, rtf: null }
  }
  if (ENV_TTS === 'installing') return { ...base, ready: false, state: 'provisioning', rtf: null, step: 'model', percent: 18, installed: false }
  return base
}

let status = seedStatus()
let timer: NodeJS.Timeout | null = null
let installState: TtsInstallProgress['state'] = ENV_TTS === 'installing' ? 'running' : 'idle'
let installError = ''
let installUpdatedAt = new Date().toISOString()

export const ttsStatus = () => status
/** ID hàng của dock — cùng ID với máy chủ thật (internal/entry/web/voice_jobs.go). */
export const VOICE_INSTALL_JOB = 'voice/install'

export const ttsInstallProgress = (): TtsInstallProgress => ({
  state: installState,
  step: installState === 'running' ? 'download' : installState,
  percent: installState === 'done' ? 100 : installState === 'idle' ? null : status.percent,
  message: installState === 'error'
    ? 'Cài bộ đọc thất bại'
    : installState === 'done'
      ? 'Đã cài xong bộ đọc'
      : status.step === 'model' ? 'Đang tải mẫu giọng…' : '',
  bytesDone: 0,
  bytesTotal: 0,
  etaSec: 0,
  ...(installError ? { error: installError } : {}),
  updated: installUpdatedAt,
})

const publish = () => {
  installUpdatedAt = new Date().toISOString()
  mirrorVoiceInstallJob(ttsInstallProgress(), (action) => {
    if (action === 'retry') installTts()
  })
}

const step = () => {
  const percent = Math.min(100, (status.percent ?? 0) + 9)
  if (FAIL_INSTALL && percent >= 40) {
    installState = 'error'
    installError = 'Không tải được mô hình đọc — kiểm tra mạng rồi thử lại'
    status = { ...status, state: 'missing', ready: false, percent, step: 'model' }
    clearInterval(timer ?? undefined)
    timer = null
    publish()
    return
  }
  if (percent >= 100) {
    status = {
      ...status, ready: true, state: 'ready', rtf: 0.68, installedBytes: 1_536_000_000,
      percent: 100, step: 'model',
    }
    installState = 'done'
    installError = ''
    clearInterval(timer ?? undefined)
    timer = null
    publish()
    return
  }
  status = {
    ...status,
    percent, step: 'model',
  }
  publish()
}

/** POST /api/tts/install — cài vào ~/.ainovel/voice/vieneu (mock: tiến dần trong ~6 giây). */
export const installTts = () => {
  if (installState === 'running' || status.ready) return
  status = { ...status, state: 'provisioning', ready: false, error: null, percent: 0, step: 'model' }
  installState = 'running'
  installError = ''
  clearInterval(timer ?? undefined)
  timer = setInterval(step, 520)
  publish()
}

export const ttsEngines = (): TtsEngine[] => [
  { id: 'vieneu', name: 'VieNeu', description: 'Chạy trên máy · giọng tiếng Việt · clone từ mẫu 10–30 giây', installed: status.state !== 'missing' },
  { id: 'openai-speech', name: 'Máy chủ giọng đọc khác', description: 'Máy chủ giọng đọc tương thích OpenAI Speech', installed: false },
]

export const ttsVoices = () => BUILTIN_VOICES.map(({ file: _file, durationSec: _durationSec, ...voice }) => ({ ...voice, source: 'builtin', builtIn: true }))

export const resetTts = () => {
  clearInterval(timer ?? undefined)
  timer = null
  status = seedStatus()
  installState = ENV_TTS === 'installing' ? 'running' : 'idle'
  installError = ''
  publish()
}
