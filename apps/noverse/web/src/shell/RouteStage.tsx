import { useLayoutEffect, useRef, type ReactNode } from 'react'
import { KeepAlive } from '@noverse/ui'
import { endNav } from './navTransition'

const MAIN_ROUTES: Record<string, true | undefined> = { workspace: true, project: true, reader: true, audio: true, newstory: true }

/** Main tabs retain their DOM/state; hidden Activity trees suspend effects, never fade. */
export function RouteStage({ routeKey, children }: { routeKey: string; children: ReactNode }) {
  const visited = useRef(new Map<string, ReactNode>())
  // Keep one instance per main tab, replacing it when its project/story changes.
  const family = routeKey.split(':', 1)[0]
  for (const key of visited.current.keys()) {
    const name = key.split(':', 1)[0]
    if (key !== routeKey && (!MAIN_ROUTES[name] || name === family)) visited.current.delete(key)
  }
  visited.current.set(routeKey, children)
  useLayoutEffect(() => endNav(), [routeKey])

  return <div data-route-stage style={{ position: 'relative', flex: 1, minWidth: 0, minHeight: 0, display: 'flex' }}>
    {[...visited.current].map(([key, node]) => <KeepAlive key={key} active={key === routeKey}>
      <div data-route-layer={key === routeKey ? 'incoming' : 'retained'} aria-hidden={key !== routeKey || undefined}
        style={{ position: 'absolute', inset: 0, display: 'flex', minWidth: 0, minHeight: 0 }}>
        {node}
      </div>
    </KeepAlive>)}
  </div>
}
