import { useEffect, useMemo, useRef, useState } from 'react'
import { toast } from '@noverse/ui'
import { getQueue, patchQueue, type QueuePatch, type QueueView } from '../../api/clone'
import { listChannels, type Channel, type ChannelList } from '../../api/channels'
import type { Status } from '../../api/status'
import { useResource, type Resource } from '../../data/useResource'
import { keys } from '../../data/keys'
import { useOnline, useStatus } from '../../data/status'
import { t } from '../../i18n/queue'
import { sortItems } from './queueView'

/** One mutation at a time: never roll back a later edit with an older response. */
export function useQueue(): QueueController {
  const res = useResource(keys.queue(), ({ signal }) => getQueue(signal))
  const channelsRes = useResource(keys.channels(), ({ signal }) => listChannels(signal))
  const status = useStatus()
  const online = useOnline()
  const [view, setView] = useState<QueueView | undefined>(res.data)
  const [busy, setBusy] = useState(false)
  const inFlight = useRef(false)
  const latest = useRef(view)
  latest.current = view
  useEffect(() => { if (res.data && !inFlight.current) setView(res.data) }, [res.data])
  const channels = useMemo(() => new Map((channelsRes.data?.channels ?? []).map((channel) => [channel.id, channel])), [channelsRes.data])
  const items = useMemo(() => sortItems(view?.items ?? []), [view?.items])

  const patch = async (body: QueuePatch, optimistic?: (old: QueueView) => QueueView): Promise<boolean> => {
    if (inFlight.current || !online) return false
    inFlight.current = true
    setBusy(true)
    const before = latest.current
    if (before && optimistic) setView(optimistic(before))
    try {
      const next = await patchQueue(body)
      latest.current = next
      setView(next)
      return true
    } catch {
      if (before) setView(before)
      toast.err(t.screens.queue.toasts.failed)
      return false
    } finally {
      inFlight.current = false
      setBusy(false)
    }
  }
  return { res, status, online, view, channels, channelsRes, items, patch, busy }
}

export interface QueueController {
  res: Resource<QueueView>
  status: Resource<Status>
  online: boolean
  view: QueueView | undefined
  channels: Map<string, Channel>
  channelsRes: Resource<ChannelList>
  items: QueueView['items']
  patch: (body: QueuePatch, optimistic?: (old: QueueView) => QueueView) => Promise<boolean>
  busy: boolean
}
