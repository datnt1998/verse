import type { AdminAuditItem, User } from '../src/api/types'
const state = globalThis as typeof globalThis & { __ainovelAudit?: AdminAuditItem[] }
export const auditItems = state.__ainovelAudit ??= []
export function validProjects(value: unknown): value is User['projects'] {
  return !!value && typeof value === 'object' && !Array.isArray(value) && Object.values(value).every((role) => ['owner', 'collab', 'view'].includes(String(role)))
}
export function appendAudit(actor: User | null, type: AdminAuditItem['type'], action: string, target: string, detail: Partial<Pick<AdminAuditItem, 'project' | 'from' | 'to' | 'reason'>> = {}) {
  const item: AdminAuditItem = { id: crypto.randomUUID(), at: new Date().toISOString(), actor: actor ? { username: actor.username, name: actor.name } : null, type, action, target, ...detail }
  auditItems.unshift(item)
  const cutoff = Date.now() - 365 * 86400000
  for (let index = auditItems.length - 1; index >= 0; index--) if (Date.parse(auditItems[index].at) < cutoff) auditItems.splice(index, 1)
  return item
}
export function readAudit(query: URLSearchParams) {
  const type = query.get('type'), actor = query.get('actor'), before = query.get('before')
  const limit = Math.min(200, Math.max(1, Number(query.get('limit')) || 50))
  const filtered = auditItems.filter((item) => (!type || item.type === type) && (!actor || (actor === 'system' ? item.actor === null : item.actor?.username === actor)))
  const offset = before ? filtered.findIndex((item) => item.id === before) + 1 : 0
  const items = filtered.slice(offset, offset + limit)
  return { items, next: offset + limit < filtered.length ? items[items.length - 1]?.id ?? '' : '' }
}
