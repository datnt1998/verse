import type { CommandSpec, HostEvent, Snapshot, User } from '../../../api/types'
import { canWriteStory } from '../../../api/client'
import { initialValues, type CommandValues } from '../../../commands/model'
import { isCompleted, isGate } from '../GateCard'

export type WorkState = 'running' | 'gate' | 'idle' | 'paused' | 'done' | 'error'

/** Tiêu đề lệnh ngắn (`SHORT` trong MWork.dc.html): chip lệnh nhanh và danh sách lệnh trong sheet. */
export const COMMAND_SHORT: Record<string, string> = {
  help: 'Hướng dẫn lệnh', model: 'Đổi model trợ lý', review: 'Duyệt từng chương', next: 'Viết chương tiếp',
  import: 'Nhập truyện có sẵn', reopen: 'Mở lại truyện đã xong', cocreate: 'Bàn hướng đi tới',
  sync: 'Nhận chỗ sửa tay', diag: 'Khám sức khoẻ truyện', export: 'Xuất TXT / EPUB',
}

/** Năm bước của một chương, tên theo `steps` của MWork.dc.html (`flow` của snapshot → bước). Phần trăm = bước × 20, dùng chung với thẻ “Chương sau” của Đọc. */
export const WRITING_STEPS: Record<string, { number: number; label: string }> = {
  planning: { number: 1, label: 'Đọc bối cảnh' }, writing: { number: 2, label: 'Viết bản nháp' },
  reviewing: { number: 3, label: 'Soát nhất quán' }, rewriting: { number: 4, label: 'Biên tập chấm điểm' }, polishing: { number: 5, label: 'Lưu chương' },
}
export function writingPercent(flow: string | undefined): number {
  return (WRITING_STEPS[flow ?? '']?.number ?? 1) * 20
}

export type AgentRole = 'writer' | 'architect' | 'editor' | 'arbiter'

/**
 * Dòng trạng thái dưới tên trợ lý trong dải M1 (MWork.dc.html › `AGS`): Người viết theo trạng thái truyện
 * (đang chạy → bước hiện tại, mất mạng → “—”), ba trợ lý còn lại chờ lệnh; đang chạy thật thì ghi việc đang làm.
 */
export function agentStatus(role: AgentRole, ctx: { state: WorkState | null; stale: boolean; active: boolean; task: string; nextChapter: number; queued: boolean; providerStop?: string }): string {
  if (ctx.providerStop) return ctx.providerStop === 'quota' ? 'Chờ lượt, sẽ tự thử lại' : 'Dừng do lỗi'
  if (role === 'writer') {
    if (ctx.state === null) return '—'
    if (ctx.state === 'running') return ctx.stale ? '—' : ctx.task
    return { gate: 'Chờ bạn duyệt', idle: 'Đang chờ', paused: 'Đã tạm dừng', done: 'Đã xong', error: 'Dừng do lỗi' }[ctx.state]
  }
  if (ctx.active) return ctx.task
  if (role === 'architect') return ctx.state === 'done' ? 'Đã xong' : 'Chờ lệnh'
  if (role === 'editor') return ctx.state === 'done' ? 'Đã xong' : `Chờ chương ${ctx.nextChapter}`
  return ctx.queued ? 'Chờ góp ý' : 'Gọi khi cần'
}

/** Snapshot không gắn lỗi nhà cung cấp vào một vai: ưu tiên trợ lý báo lỗi, rồi trợ lý hoạt động gần nhất. */
export function providerStoppedRole(snapshot: Snapshot | null | undefined): AgentRole | null {
  if (!snapshot?.providerError || snapshot.isRunning) return null
  let selected: NonNullable<Snapshot['agents']>[number] | undefined
  let selectedPriority = 0
  for (const agent of snapshot.agents ?? []) {
    const priority = ['error', 'failed'].includes(agent.state) ? 2 : ['running', 'working', 'thinking', 'tool'].includes(agent.state) ? 1 : 0
    if (!priority || (selected && (priority < selectedPriority || (priority === selectedPriority && agent.updatedAt <= selected.updatedAt)))) continue
    selected = agent
    selectedPriority = priority
  }
  const name = selected?.name.toLowerCase()
  return name ? (['writer', 'architect', 'editor', 'arbiter'] as const).find((role) => name.includes(role)) ?? null : null
}

/** Băng mất kết nối (MWork.dc.html › `retryText`). */
export const retryText = (seconds: number) => `Thử lại sau ${seconds} giây · máy vẫn chạy`

/** Ô chat tối đa 4 dòng: 4 × (16px × 1.4) + 2 × 8px đệm (18-mobile §Bàn phím mở). */
export const MAX_COMPOSER = 4 * 22.4 + 16

const PHONE_QUICK = ['next', 'diag', 'export', 'review'].map((name) => ({ name, label: COMMAND_SHORT[name] }))
const TABLET_QUICK = [{ name: 'cocreate', label: 'Bàn hướng đi' }, { name: 'diag', label: 'Khám truyện' }, { name: 'export', label: 'Xuất' }]

/** Export writes into the library (`POST /exports` needs write access, 11-api-contract), so like rewrite it is hidden from readers (18-mobile §M2). */
export function workspaceActions(user: User | null, storyId: string, tablet = false) {
  const write = !!user && canWriteStory(user, storyId)
  return { write, export: write, quickCommands: write ? (tablet ? TABLET_QUICK : PHONE_QUICK) : [] }
}

/** The i18n `init` values are artboard samples (`to=30`, `tra-xanh.epub`); on a real story use its chapter count and slug. */
export function mobileInitialValues(spec: CommandSpec, story: string, completedCount: number): CommandValues {
  const out = initialValues(spec)
  if (spec.name === 'export') {
    if (spec.params.some((param) => param.key === 'path')) out.values.path = `exports/${story}.epub`
    if (spec.params.some((param) => param.key === 'to')) out.values.to = completedCount > 0 ? String(completedCount) : ''
  }
  return out
}

/** Engine state wins over historical log failures; only an unrecovered stop is an error. */
export function workState(snapshot: Snapshot, events: HostEvent[]): WorkState {
  if (isCompleted(snapshot)) return 'done'
  if (snapshot.isRunning) return 'running'
  if (snapshot.waitingFor === 'gate' && !snapshot.inProgressChapter) return 'gate'
  if (snapshot.runtimeState === 'paused' || snapshot.runtimeState === 'pausing') return 'paused'
  const last = events.at(-1)
  if (snapshot.runtimeState === 'error' || snapshot.runtimeState === 'failed' || last?.failed || last?.level === 'error') return 'error'
  if (isGate(snapshot)) return 'gate'
  return 'idle'
}

export function resultHref(project: string, story: string, line: string, job = 'new'): string {
  return `/settings/mobile?${new URLSearchParams({ result: job, command: line, project, story })}`
}

/** Consume a guide's prefill once, without treating M5 navigation as a new composer draft. */
export function composerPrefill(hash: string, workspacePath: string): { command: string; hash: string } | null {
  const path = `#${workspacePath}`
  if (hash !== path && !hash.startsWith(`${path}?`)) return null
  const query = new URLSearchParams(hash.split('?')[1] ?? '')
  const command = query.get('command')
  if (!command) return null
  query.delete('command')
  const rest = query.toString()
  return { command, hash: `${path}${rest ? `?${rest}` : ''}` }
}

const RESULT_COMMANDS: Record<string, true> = { cocreate: true, import: true, diag: true, sync: true, reopen: true }
export function hasMobileResult(line: string): boolean {
  return !!RESULT_COMMANDS[line.trim().split(/\s+/)[0].replace(/^\//, '')]
}

export function reviewScore(dimensions?: { score: number }[]): string | null {
  if (!dimensions?.length) return null
  return (dimensions.reduce((sum, item) => sum + item.score, 0) / dimensions.length).toLocaleString('vi', { maximumFractionDigits: 1 })
}
