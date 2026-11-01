// GET /api/channels/summary — khung app (AppShell, MobileRouteStage) đọc ngay lần
// vẽ đầu, nên đọc bằng guard viết tay thay vì kéo zod vào entry.
import { bool, list, maybe, num, record, str } from './guard'
import { request } from './request'

export type ChannelSummary = {
  channelId: string
  name: string
  initials: string
  color: string
  avatarUrl?: string | null
  today: { done: number; cap: number }
  waitingYou: number
  ready: number
  postedThisWeek: number
  autoFetch: boolean
  templateIssue?: { templateId: string; text: string }
}

const readSummary = (value: unknown, path: string): ChannelSummary => {
  const o = record(value, path)
  const today = record(o.today, `${path}.today`)
  const issue = maybe(o.templateIssue, (entry, sub) => record(entry, sub), `${path}.templateIssue`)
  return {
    channelId: str(o.channelId, `${path}.channelId`),
    name: str(o.name, `${path}.name`),
    initials: str(o.initials, `${path}.initials`),
    color: str(o.color, `${path}.color`),
    // zod cũ: `.nullable().optional()` — giữ nguyên null/undefined như server gửi.
    avatarUrl: o.avatarUrl === null ? null : maybe(o.avatarUrl, str, `${path}.avatarUrl`),
    today: { done: num(today.done, `${path}.today.done`), cap: num(today.cap, `${path}.today.cap`) },
    waitingYou: num(o.waitingYou, `${path}.waitingYou`),
    ready: num(o.ready, `${path}.ready`),
    postedThisWeek: num(o.postedThisWeek, `${path}.postedThisWeek`),
    autoFetch: bool(o.autoFetch, `${path}.autoFetch`),
    ...(issue && { templateIssue: { templateId: str(issue.templateId, `${path}.templateIssue.templateId`), text: str(issue.text, `${path}.templateIssue.text`) } }),
  }
}

export const readChannelSummaries = (value: unknown): ChannelSummary[] => list(value, readSummary, 'channels')

export const getChannelSummaries = async (signal?: AbortSignal) =>
  readChannelSummaries(await request<unknown>('/api/channels/summary', { signal }))
