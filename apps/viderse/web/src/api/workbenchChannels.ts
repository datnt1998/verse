import { z } from 'zod'
import { request } from './request'

const channelSummarySchema = z.object({ channelId: z.string(), name: z.string(), initials: z.string(), color: z.string(), avatarUrl: z.string().nullable().optional(), today: z.object({ done: z.number(), cap: z.number() }), waitingYou: z.number(), ready: z.number(), postedThisWeek: z.number(), autoFetch: z.boolean(), templateIssue: z.object({ templateId: z.string(), text: z.string() }).optional() })
export const getChannelSummaries = async (signal?: AbortSignal) => z.array(channelSummarySchema).parse(await request<unknown>('/api/channels/summary', { signal }))
