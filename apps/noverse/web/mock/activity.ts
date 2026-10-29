import type { ActivityEvent, ActivityHistory, HostEvent, User } from '../src/api/types'

/** Mock-side equivalent of host.ContextualizeEvent; never derive arc bounds in UI. */
export function mockActivityContext(replay: HostEvent[], currentChapter: number): ActivityEvent[] {
  const events = replay.map((row) => {
    const event = { ...row, chapter: row.chapter ?? currentChapter } as ActivityEvent
    let args: Record<string, unknown> = {}
    let result: Record<string, unknown> = {}
    try { args = JSON.parse(event.args ?? '{}') } catch { /* Historical truncated arguments. */ }
    try { result = JSON.parse(event.result ?? '{}') } catch { /* Results can also be truncated. */ }
    if (typeof args.chapter === 'number') event.chapter = args.chapter
    else if (typeof result.chapter === 'number') event.chapter = result.chapter
    else {
      const target = /(?:Viết(?: lại)?|Gọt giũa|Thẩm duyệt|Đọc) chương\s+(\d+)|(?:chương|Ch\.)\s*(\d+)/i.exec(event.summary)
      if (target) event.chapter = Number(target[1] ?? target[2])
    }
    if (event.tool === 'novel_context' && (!('chapter' in args) || typeof args.chapter === 'number' && args.chapter <= 0)) event.chapter = 0
    if (typeof args.scope === 'string' || typeof result.scope === 'string') event.scope = String(args.scope ?? result.scope)
    if (event.category === 'DISPATCH' && /thẩm duyệt.*cung/i.test(event.summary)) {
      const span = /chương\s+(\d+)[–-](\d+)/i.exec(event.summary)
      event.scope = 'arc'
      event.arc = Number(/cung\s+(\d+)/i.exec(event.summary)?.[1]) || undefined
      event.volume = Number(/tập\s+(\d+)/i.exec(event.summary)?.[1]) || undefined
      if (span) { event.chapterFrom = Number(span[1]); event.chapterTo = event.chapter = Number(span[2]) }
    }
    return event
  })
  for (const event of events) if (event.scope === 'arc' && !event.chapterFrom) {
    const dispatch = events.find(row => row.category === 'DISPATCH' && row.scope === 'arc' && row.chapterFrom && row.agent === event.agent && Date.parse(row.time) <= Date.parse(event.time) && Date.parse(event.time) <= Date.parse(row.finishedAt))
    if (dispatch) Object.assign(event, { arc: dispatch.arc, volume: dispatch.volume, chapterFrom: dispatch.chapterFrom, chapterTo: dispatch.chapterTo })
  }
  return events
}

export function mockActivityHistory(replay: HostEvent[], currentChapter: number, chaptersByNumber: Record<number, { title: string }>, user: User, url: URL, source?: ActivityHistory): ActivityHistory {
  const fold = (value: string) => value.normalize('NFD').replace(/[\u0300-\u036f]/g, '').replace(/đ/g, 'd').replace(/Đ/g, 'D').toLowerCase()
  const now = Date.now()
  // The focused stream regression fixture must not be paired with synthetic read calls.
  const fixture: ActivityEvent[] = Array.from({ length: source || replay.some((event) => event.id === 'ask-surface-read') ? 0 : 64 }, (_, index) => {
    const chapter = Math.max(1, currentChapter - Math.floor(index / 8))
    const failed = index === 3 || index === 19
    const agent = ['writer', 'architect', 'editor', 'arbiter'][index % 4]!
    return {
      id: `activity-${index}`, time: new Date(now - index * 17000).toISOString(), finishedAt: index === 0 ? '' : new Date(now - index * 17000).toISOString(),
      category: index === 5 ? 'USER' : 'TOOL', agent: index === 5 ? 'user' : agent,
      summary: index === 5 ? 'Giữ giọng văn, thêm đối thoại ở chương này' : failed ? 'Không đọc được bản nháp: kết nối tạm thời gián đoạn' : `${index % 2 ? 'Đã đọc bản nháp' : 'Lưu chương'} ${chapter} · giữ nhịp kể và giọng văn`,
      level: failed ? 'error' : 'info', failed, kind: '', depth: 1, durationNs: 1370000000, retryAt: '', canRetryNow: false, sessionId: 'activity-session',
      chapter, username: index === 5 ? user.username : undefined,
      tool: index === 5 ? undefined : index % 2 ? 'read_chapter' : 'draft_chapter',
      args: index === 5 ? undefined : JSON.stringify({ chapter, instruction: 'Giữ giọng văn, tăng đối thoại' }),
      result: index === 0 ? 'Đang viết · 1.840 chữ' : index === 5 ? undefined : failed ? 'Kết nối bị ngắt' : JSON.stringify({ ok: true, words: 1840, chapter }),
      durationMs: 1370, status: index === 0 ? 'running' : failed ? 'error' : 'ok',
      retries: index === 7 ? [{ atSec: 2, error: 'Kết nối mạng tạm thời gián đoạn', waitSec: 8, ok: true }] : [],
    }
  })
  const events = [...mockActivityContext(source?.events ?? replay, currentChapter), ...fixture].sort((a, b) => Date.parse(b.time) - Date.parse(a.time))
  const params = url.searchParams
  const base = events.filter((event) => (!params.has('chapter') || event.chapter === Number(params.get('chapter'))) && (!params.get('q') || fold(`${event.summary} ${event.args ?? ''} ${event.result ?? ''}`).includes(fold(params.get('q')!))))
  const counts: ActivityHistory['counts'] = { total: base.length, byAgent: {}, errors: 0, mine: 0 }
  for (const event of base) {
    counts.byAgent[event.agent] = (counts.byAgent[event.agent] ?? 0) + 1
    if (event.failed || event.level === 'error' || event.retries?.length) counts.errors++
    if (event.username === user.username) counts.mine++
  }
  const filtered = base.filter((event) => (!params.get('agent') || event.agent === params.get('agent')) && (params.get('level') !== 'error' || event.failed || event.level === 'error' || event.retries?.length) && (params.get('mine') !== '1' || event.username === user.username))
  const before = params.get('before')
  const start = before ? Math.max(0, filtered.findIndex((event) => event.id === atob(before)) + 1) : 0
  const limit = Number(params.get('limit')) || 100
  const page = filtered.slice(start, start + limit)
  const chapters = [...new Set(filtered.map((event) => event.chapter))].sort((a, b) => b - a).map((chapter) => ({
    chapter, title: source?.chapters.find(row => row.chapter === chapter)?.title ?? chaptersByNumber[chapter]?.title.replace(/^Chương\s+\d+\s*[:·.-]?\s*/i, '') ?? '',
    score: source ? source.chapters.find(row => row.chapter === chapter)?.score : chapter < currentChapter ? 81 : undefined,
    eventCount: filtered.filter((event) => event.chapter === chapter && event.scope !== 'arc').length,
  }))
  return { events: page, counts, chapters, nextCursor: start + page.length < filtered.length ? btoa(page.at(-1)!.id) : null }
}
