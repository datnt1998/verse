type StageJob = { stageGroup: string }
type GroupKey = 'waiting' | 'working' | 'ready' | 'queued' | 'posted'
const definitions: { key: GroupKey; label: string; stages: string[] }[] = [
  { key: 'waiting', label: 'Chờ bạn', stages: ['attention', 'review', 'paused'] },
  { key: 'working', label: 'Đang làm', stages: ['working'] },
  { key: 'ready', label: 'Sẵn sàng đăng', stages: ['ready'] },
  { key: 'queued', label: 'Chờ lượt', stages: ['queued'] },
  { key: 'posted', label: 'Đã đăng', stages: ['posted'] },
]
export function groupJobs<T extends StageJob>(jobs: T[], all: boolean) {
  return definitions.flatMap(({ key, label, stages }) => {
    if (all && key === 'posted') return []
    const matches = jobs.filter(job => stages.includes(job.stageGroup))
    const items = key === 'queued' ? matches.slice(0, all ? 3 : 2) : key === 'posted' ? matches.slice(0, 1) : matches
    return items.length ? [{ key, label, items, count: matches.length }] : []
  })
}
export function readScope(search: URLSearchParams, saved: string | null) {
  return search.get('channel') || (search.get('scope') === 'all' ? 'all' : saved || 'all')
}
export function scheduleScopeRestore(saved: string, readHash: () => string, restore: (scope: string) => void) {
  queueMicrotask(() => {
    if (readHash() === '#/') restore(saved)
  })
}
// Mục hàng chờ đang chọn được nhận thành job mới (id khác): đi theo job cùng nguồn, thế hệ cao nhất.
export function followAdmitted<T extends { id: string; sourceKey: string; generation: number }>(jobs: T[], followed: { id: string; sourceKey: string } | null) {
  if (!followed?.sourceKey || jobs.some(job => job.id === followed.id)) return undefined
  return jobs.filter(job => job.sourceKey === followed.sourceKey).sort((a, b) => b.generation - a.generation)[0]?.id
}
