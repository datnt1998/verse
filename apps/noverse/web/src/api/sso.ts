import type { AppLink } from './types'

export type KickedReason = 'self' | 'other' | 'password' | 'admin' | 'expired' | 'unknown'
export function kickedReason(payload: unknown): KickedReason {
  if (!payload || typeof payload !== 'object' || !('reason' in payload)) return 'unknown'
  switch (payload.reason) {
    case 'logout': return 'app' in payload ? payload.app === 'noverse' ? 'self' : payload.app === 'viderse' ? 'other' : 'unknown' : 'unknown'
    case 'password_changed': return 'password'
    case 'admin_changed': case 'locked': return 'admin'
    case 'expired': return 'expired'
    default: return 'unknown'
  }
}
export function readAppLinks(value: unknown): AppLink[] {
  if (!Array.isArray(value)) return []
  return value.flatMap((entry: unknown) => {
    if (!entry || typeof entry !== 'object' || !('id' in entry) || !('origin' in entry) || typeof entry.origin !== 'string') return []
    if (entry.id !== 'noverse' && entry.id !== 'viderse') return []
    try {
      const url = new URL(entry.origin)
      if (!['http:', 'https:'].includes(url.protocol) || url.origin !== entry.origin || url.username || url.password) return []
      return [{ id: entry.id, name: entry.id === 'noverse' ? 'Noverse' : 'Viderse', origin: url.origin } satisfies AppLink]
    } catch { return [] }
  })
}
export function deniedAppLinks(error: unknown): AppLink[] {
  if (!error || typeof error !== 'object' || !('payload' in error)) return []
  const payload = error.payload
  return payload && typeof payload === 'object' && 'apps' in payload ? readAppLinks(payload.apps) : []
}
