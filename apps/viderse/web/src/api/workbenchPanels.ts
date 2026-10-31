import { z } from 'zod'
import { mutate, request } from './request'
import { cloneBackdropEvidenceSchema } from '@/lib/clone-view'

const reason = z.object({ agent: z.enum(['scout', 'director', 'artist', 'qa']), text: z.string(), facts: z.array(z.object({ label: z.string(), value: z.string(), source: z.string() })), fix: z.object({ label: z.string(), href: z.string() }).optional() })
const content = z.object({ title: z.string(), description: z.string(), tags: z.array(z.string()), editedBy: z.string().optional(), editedAt: z.string().optional(), edited: z.object({ title: z.boolean().optional(), description: z.boolean().optional(), tags: z.boolean().optional() }).optional(), reason: z.object({ title: reason.optional(), description: reason.optional(), tags: reason.optional() }).optional() })
const metadataView = z.object({ status: z.enum(['pending', 'writing', 'failed', 'ai']), metadata: content.nullable(), revision: z.number().int(), descriptionFooter: z.string().nullable().optional(), alwaysTags: z.array(z.string()).nullable().optional() })
const historyView = z.object({ history: z.array(content.extend({ at: z.string(), by: z.enum(['director', 'owner']), parts: z.array(z.enum(['title', 'description', 'tags'])) })).nullable() })
const backdropView = cloneBackdropEvidenceSchema.extend({ warnings: z.array(z.string()).nullable().transform(value => value ?? []), playbackSpeed: z.number().positive(), fixedItems: z.array(z.object({ id: z.string(), title: z.string().nullable(), startSec: z.number(), endSec: z.number(), playbackSec: z.number() })), sources: z.array(z.object({ channelId: z.string(), platform: z.string(), handle: z.string(), title: z.string().nullable() })) })
export type Metadata = z.infer<typeof content>
export type MetaResponse = z.infer<typeof metadataView>
export type History = NonNullable<z.infer<typeof historyView>['history']>[number]
export type BackdropView = z.infer<typeof backdropView>
const url = (id: string, suffix: string) => `/api/clone/${encodeURIComponent(id)}/${suffix}`
export async function getMetadata(id: string, signal?: AbortSignal) { return metadataView.parse(await request<unknown>(url(id, 'metadata'), { signal })) }
export async function mutateMetadata(id: string, action: '' | 'rewrite' | 'restore', body: object) { return metadataView.parse(await mutate<unknown>(action ? 'POST' : 'PATCH', url(id, `metadata${action ? `/${action}` : ''}`), body)) }
export async function getHistory(id: string) { return historyView.parse(await request<unknown>(url(id, 'metadata/history'))).history ?? [] }
export async function getBackdrop(id: string, signal?: AbortSignal) { return z.object({ backdrop: backdropView.nullable() }).parse(await request<unknown>(url(id, 'backdrop'), { signal })).backdrop }
export async function mutateBackdrop(id: string, body: object) { return mutate('POST', url(id, 'backdrop'), body) }
