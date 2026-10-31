import type { HostEvent, LiveHistoryOperation, LiveHistoryRound } from '../api/types'

/** Snapshot và SSE cùng định danh vòng/thao tác; trạng thái đã kết thúc không lùi về bắt đầu. */
export function mergeRunHistory(current: LiveHistoryRound[], incoming: LiveHistoryRound[]): LiveHistoryRound[] {
  const rounds = new Map(current.map(round => [round.roundId, round]))
  for (const next of incoming) {
    const previous = rounds.get(next.roundId)
    if (!previous) { rounds.set(next.roundId, next); continue }
    const operations = new Map(previous.operations.map(operation => [operation.id, operation]))
    for (const operation of next.operations) {
      const old = operations.get(operation.id)
      if (!old || old.status === 'running' || operation.status !== 'running') operations.set(operation.id, operation)
    }
    rounds.set(next.roundId, { ...previous, ...next, title: next.title || previous.title, operations: [...operations.values()].map(operation => operation.round === next.round ? operation : { ...operation, round: next.round }) })
  }
  return [...rounds.values()].sort((a, b) => a.round - b.round)
}

/** Ghép các frame TOOL mới nhất vào snapshot, dựa trên id thực của lần gọi và vòng. */
export function mergeRunEvents(current: LiveHistoryRound[], events: HostEvent[]): LiveHistoryRound[] {
  let result = current
  let newest = current.at(-1)
  for (const event of events) {
    if (event.category !== 'TOOL' || !event.id || !event.roundId || !event.tool || !event.roundStartedAt) continue
    const existing = result.find(round => round.roundId === event.roundId)
    if (!existing && newest && event.roundStartedAt < newest.startedAt) continue
    const number = existing?.round ?? (newest?.round ?? 0) + 1
    const failed = event.failed || event.status === 'error'
    const operation: LiveHistoryOperation = { id: event.id, round: number, roundId: event.roundId, role: event.role || event.agent, startedAt: event.time, durationMs: event.durationMs ?? Math.floor(event.durationNs / 1_000_000), status: failed ? 'error' : event.status === 'ok' ? 'done' : 'running', error: failed ? event.result || event.summary : '', key: event.tool, params: event.args || '', result: event.result || '' }
    const round: LiveHistoryRound = { round: number, roundId: event.roundId, role: event.role || event.agent, title: event.roundTitle, startedAt: event.roundStartedAt, operations: [operation] }
    result = mergeRunHistory(result, [round]); newest = result.at(-1)
  }
  return result
}
