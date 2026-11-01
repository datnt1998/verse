// Lệnh `/` của ô Nhắn trợ lý (04-components › Composer). Danh sách chỉ giúp gõ
// nhanh: chọn lệnh chỉ điền chữ vào ô, câu vẫn đi qua /api/assistant/plan như mọi
// câu khác. Lệnh chỉ hiện khi hợp lệ với video đang mở.
import { assistantCopy } from '../../i18n/assistant'

export const SLASH_IDS = ['redraw', 'provider', 'pause', 'resume', 'export-anyway', 'posted'] as const
export type SlashId = (typeof SLASH_IDS)[number]

export type SlashCommand = { id: SlashId; usage: string; text: string }

/** Phần của video đang mở mà việc chọn lệnh phụ thuộc vào. */
export type SlashJob = { stageGroup: string; activeStep: string | null; noImages: boolean }

const copy = assistantCopy.slash

const COMMANDS: Record<SlashId, SlashCommand> = {
  redraw: { id: 'redraw', usage: '/redraw <cảnh>', text: copy.redraw },
  provider: { id: 'provider', usage: '/provider <chatgpt|grok|gemini>', text: copy.provider },
  pause: { id: 'pause', usage: '/pause', text: copy.pause },
  resume: { id: 'resume', usage: '/resume', text: copy.resume },
  'export-anyway': { id: 'export-anyway', usage: '/export-anyway', text: copy.exportAnyway },
  posted: { id: 'posted', usage: '/posted [link]', text: copy.posted },
}

/** Các lệnh hợp lệ với video này, theo thứ tự của đặc tả. */
export function slashCommandsFor(job: SlashJob): SlashCommand[] {
  const valid: Record<SlashId, boolean> = {
    redraw: !job.noImages && job.stageGroup !== 'queued',
    provider: job.stageGroup !== 'posted',
    pause: job.stageGroup === 'working',
    resume: job.stageGroup === 'paused',
    'export-anyway': job.stageGroup === 'attention' && job.activeStep === 'score',
    posted: job.stageGroup === 'ready',
  }
  return SLASH_IDS.filter((id) => valid[id]).map((id) => COMMANDS[id])
}

/** Phần gõ sau `/` khi ô còn đang gõ tên lệnh (chưa có dấu cách); ngược lại null. */
export function slashQuery(draft: string): string | null {
  const match = /^\/(\S*)$/.exec(draft)
  return match ? match[1].toLowerCase() : null
}

export function filterSlash(commands: readonly SlashCommand[], query: string): SlashCommand[] {
  return commands.filter((command) => command.id.startsWith(query))
}

/** Lệnh đã gõ đủ và không có tham số: Enter gửi luôn, không còn gì để điền. */
export const isComplete = (command: SlashCommand, draft: string) => draft.toLowerCase() === `/${command.id}` && !/[<[]/.test(command.usage)
