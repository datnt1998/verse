import type { AppLink, SessionEnded } from './types'

export function readAppLinks(payload: unknown): AppLink[] {
  if (!payload || typeof payload !== 'object' || !('apps' in payload) || !Array.isArray(payload.apps)) return []
  const links: AppLink[] = []
  for (const value of payload.apps) {
    if (!value || typeof value !== 'object' || (value.id !== 'noverse' && value.id !== 'viderse') || typeof value.origin !== 'string') continue
    try {
      const url = new URL(value.origin)
      if (!['https:', 'http:'].includes(url.protocol) || url.username || url.password || url.origin !== value.origin || links.some((app) => app.id === value.id)) continue
      const app: AppLink = { id: value.id, name: value.id === 'noverse' ? 'Noverse' : 'Viderse', origin: url.origin }
      if (app.id === 'viderse') links.unshift(app)
      else links.push(app)
    } catch { /* Malformed origins cannot become navigation destinations. */ }
  }
  return links
}

export function readSessionEnded(payload: unknown): SessionEnded {
  if (!payload || typeof payload !== 'object') return { reason: 'unknown' }
  return {
    reason: 'reason' in payload && typeof payload.reason === 'string' ? payload.reason : 'unknown',
    ...('app' in payload && (payload.app === 'viderse' || payload.app === 'noverse') ? { app: payload.app } : {}),
  }
}

export function membersUrl(apps: readonly AppLink[]): string | undefined {
  const noverse = apps.find((app) => app.id === 'noverse')
  return noverse ? `${noverse.origin}/#/settings/members` : undefined
}
