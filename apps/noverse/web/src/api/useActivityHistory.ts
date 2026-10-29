import { useEffect, useMemo, useRef, useState } from 'react'
import { getStoryEvents } from './client'
import type { ActivityEvent, ActivityHistory, ActivityHistoryQuery, HostEvent } from './types'
import { keys, useAction, useResource } from '../data'
import { deduplicateUserEvents, isInternalEvent } from '../lib/activity'

const EMPTY: ActivityEvent[] = []
const EMPTY_CHAPTERS: ActivityHistory['chapters'] = []

/** Older pages, refreshed head, then live updates: one row per stable ID, newest first. */
export function mergeLogEvents(...sources: readonly HostEvent[][]): ActivityEvent[] {
  const events = new Map<string, ActivityEvent>()
  for (const source of sources) for (const event of source) {
    events.set(event.id, { chapter: 0, ...events.get(event.id), ...event })
  }
  return deduplicateUserEvents([...events.values()].filter((event) => !isInternalEvent(event))).sort((a, b) => Date.parse(b.time) - Date.parse(a.time) || b.id.localeCompare(a.id))
}

/** GAP-31 history and cursor lifetime shared by desktop, writing desk and phone. */
export function useActivityHistory(project: string, story: string, query: ActivityHistoryQuery = {}, live: HostEvent[] = EMPTY) {
  const queryKey = JSON.stringify([query.chapter, query.agent, query.level, query.mine, query.q, query.limit ?? 100])
  const scope = `${project}/${story}/${queryKey}`
  const history = useResource([...keys.story(project, story), 'activity', queryKey], ({ signal }) => getStoryEvents(project, story, query, signal))
  const [pages, setPages] = useState<{ scope: string; events: ActivityEvent[]; chapters: ActivityHistory['chapters']; cursor: string | null } | null>(null)
  const request = useRef<AbortController | null>(null)
  useEffect(() => () => { request.current?.abort(); request.current = null }, [scope])
  const older = pages?.scope === scope ? pages.events : EMPTY
  const olderChapters = pages?.scope === scope ? pages.chapters : EMPTY_CHAPTERS
  const more = pages?.scope === scope ? pages.cursor : history.data?.nextCursor
  const first = history.data?.events ?? EMPTY
  const events = useMemo(() => mergeLogEvents(older, first, live), [older, first, live])
  const chapters = useMemo(() => [...olderChapters, ...(history.data?.chapters ?? EMPTY_CHAPTERS)], [olderChapters, history.data?.chapters])
  const loadOlder = useAction(async () => {
    if (!more || request.current) return
    const controller = new AbortController()
    request.current = controller
    try {
      const page = await getStoryEvents(project, story, { ...query, before: more }, controller.signal)
      if (controller.signal.aborted) return
      setPages((previous) => ({ scope,
        events: mergeLogEvents(previous?.scope === scope ? previous.events : EMPTY, page.events),
        chapters: [...(previous?.scope === scope ? previous.chapters : EMPTY_CHAPTERS), ...page.chapters],
        cursor: page.nextCursor,
      }))
      return page
    } finally {
      if (request.current === controller) request.current = null
    }
  })
  return { ...history, events, chapters, more, loadOlder }
}
