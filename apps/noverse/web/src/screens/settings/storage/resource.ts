import { useEffect } from 'react'
import { getStorage } from '../../../api/client'
import { useSession } from '../../../api/session'
import { useResource } from '../../../data'
import { cache } from '@noverse/ui/data'
import { migrating, startVisiblePolling } from './logic'

export const storageKey = ['system', 'storage'] as const
const leases = new Map<number, number>()
let pollingInterval = 0
let stopPolling: (() => void) | undefined
function updatePolling() {
  let interval = 0
  for (const value of leases.keys()) if (!interval || value < interval) interval = value
  if (interval === pollingInterval) return
  stopPolling?.(); pollingInterval = interval
  // Cache.refresh owns request failures and publishes them to every subscriber.
  stopPolling = interval ? startVisiblePolling(interval, () => cache.refresh(storageKey)) : undefined
}
export function useStorage(detail = false, enabled = true, watchHealth = false) {
  const { user } = useSession()
  const admin = user?.role === 'admin' && enabled
  const resource = useResource(storageKey, ({ signal }) => getStorage(signal), { enabled: admin, staleTime: 30000 })
  const active = migrating(resource.data?.migration)
  useEffect(() => {
    if (!admin || (!watchHealth && !(detail && active))) return
    const interval = detail && active ? 5000 : 30000
    leases.set(interval, (leases.get(interval) ?? 0) + 1); updatePolling()
    return () => {
      const count = (leases.get(interval) ?? 1) - 1
      if (count) leases.set(interval, count); else leases.delete(interval)
      updatePolling()
    }
  }, [admin, detail, watchHealth, active])
  return resource
}
