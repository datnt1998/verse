import type { ActivityEvent, ActivityHistory } from '../src/api/types'

// Read-only source: ~/NoverseLive/tron-audio/kich_ban/01-viet-kich-ban-dan-y/
// output/novel/logs/tui.log, 27/10/2026 13:10–13:26; chapter titles from
// meta/chapter_records/000038..000040.json. A compact historical snapshot,
// not a generated 38–41 story. Raw wrong cursor 41 is intentionally retained.
const base = { failed: false, kind: '', level: 'info', depth: 0, durationNs: 0, retryAt: '0001-01-01T00:00:00Z', sessionId: 's1', canRetryNow: false }
const rows: ActivityEvent[] = [
  { ...base, id: 'e1793082382813985000-440', time: '2026-10-27T13:26:22.81399+07:00', finishedAt: '2026-10-27T13:26:23.005883+07:00', category: 'TOOL', agent: 'writer', summary: 'novel_context(chương 40)', chapter: 40, tool: 'novel_context', args: '{"chapter":40}', durationNs: 191889834, durationMs: 191, status: 'ok' },
  { ...base, id: 'e1793082380689436000-439', time: '2026-10-27T13:26:20.689437+07:00', finishedAt: '0001-01-01T00:00:00Z', category: 'DISPATCH', agent: 'writer', summary: 'writer (Viết lại chương 40)', chapter: 41 },
  { ...base, id: 'e1793082270624460000-432', time: '2026-10-27T13:24:30.624461+07:00', finishedAt: '2026-10-27T13:26:20.671399+07:00', category: 'DISPATCH', agent: 'writer', summary: 'writer (Viết lại chương 39)', chapter: 41, durationNs: 110044841667 },
  { ...base, id: 'e1793082270150992000-431', time: '2026-10-27T13:24:30.151021+07:00', finishedAt: '2026-10-27T13:24:30.594634+07:00', category: 'TOOL', agent: 'editor', summary: 'save_review(cung này·rewrite)', chapter: 40, tool: 'save_review', result: '{"affected_chapters":[39,40],"chapter":40,"issues":7,"next_chapter":41,"next_flow":"rewriting","saved":true,"scope":"arc","verdict":"rewrite"}', durationNs: 443604458, durationMs: 443, status: 'ok' },
  { ...base, id: 'e1793082077840455000-422', time: '2026-10-27T13:21:17.840457+07:00', finishedAt: '2026-10-27T13:21:18.176139+07:00', category: 'TOOL', agent: 'editor', summary: 'novel_context(chương 40)', chapter: 40, tool: 'novel_context', args: '{"chapter":40}', durationNs: 335675625, status: 'ok' },
  { ...base, id: 'e1793082074490144000-421', time: '2026-10-27T13:21:14.490145+07:00', finishedAt: '2026-10-27T13:24:30.606478+07:00', category: 'DISPATCH', agent: 'editor', summary: 'editor (Thẩm duyệt cung 2 của tập 1 (chương 33-40): gọi novel_context(chapter=40), save_review dùng scope=arc, chapter=40)', chapter: 41, durationNs: 196112595167 },
  { ...base, id: 'e1793081902378334000-412', time: '2026-10-27T13:18:22.378335+07:00', finishedAt: '2026-10-27T13:21:14.478284+07:00', category: 'DISPATCH', agent: 'writer', summary: 'writer (Viết chương 40)', chapter: 41, durationNs: 172055871291 },
  { ...base, id: 'e1793081404864888000-388', time: '2026-10-27T13:10:04.864889+07:00', finishedAt: '2026-10-27T13:13:49.181342+07:00', category: 'DISPATCH', agent: 'writer', summary: 'writer (Viết chương 38)', chapter: 41, durationNs: 224313654500 },
]
export const mobileLogFixture: ActivityHistory = {
  events: rows, nextCursor: null,
  chapters: [
    { chapter: 38, title: 'Mẹ không làm bằng chứng', eventCount: 1 },
    { chapter: 39, title: 'Không ai phải theo tôi', eventCount: 1 },
    { chapter: 40, title: 'Mỗi giọng một chỗ ký', eventCount: 4 },
  ],
  counts: { total: rows.length, errors: 0, mine: 0, byAgent: { writer: 5, editor: 3 } },
}
