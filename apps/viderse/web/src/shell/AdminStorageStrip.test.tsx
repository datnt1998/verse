// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { expect, it } from 'vitest'
import { AdminStorageStrip } from './AdminStorageStrip'
import type { User } from '../api/types'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
it.each<{ user: User; visible: boolean }>([
  { user: { username: 'admin', name: '', role: 'admin' }, visible: true },
  { user: { username: 'app-admin', name: '', role: 'member', grants: { viderse: 'admin' } }, visible: true },
  { user: { username: 'member', name: '', role: 'member', grants: { noverse: 'admin', viderse: 'member' } }, visible: false },
])('filters the global strip using effective Viderse role: $user.username', async ({ user, visible }) => {
  const element = document.createElement('div'); document.body.append(element)
  const root = createRoot(element)
  try {
    await act(async () => root.render(<AdminStorageStrip user={user} status={{ running: 0, queued: 0, paused: false, reviewCount: 0, attention: [], storageDown: { label: 'Garage', since: '2026-10-31T21:42:00Z' } }} />))
    expect(!!element.querySelector('[role="alert"]')).toBe(visible)
    if (visible) {
      expect(element.querySelector('[inert]')).toBeNull()
      await act(async () => element.querySelector('button')!.click())
      expect(location.hash).toBe('#/settings?section=storage')
    }
  } finally { await act(async () => root.unmount()); element.remove() }
})
