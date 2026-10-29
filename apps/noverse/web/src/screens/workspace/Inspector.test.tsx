import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it } from 'vitest'
import { snapshotTraXanh } from '../../../mock/data'
import { Inspector } from './Inspector'
import type { AgentSnapshot } from '../../api/types'

const agent = (name: string, state: string): AgentSnapshot => ({
  name, state, taskId: '', taskKind: '', summary: '', tool: '', turn: 0, updatedAt: '',
  context: { tokens: 0, contextWindow: 0, percent: 0, scope: '', strategy: '', activeMessages: 0, summaryMessages: 0, compactedCount: 0, keptCount: 0 },
})
function markup(agents: AgentSnapshot[] | null, paused = false) {
  return renderToStaticMarkup(<Inspector snapshot={{ ...snapshotTraXanh, agents, cachePerAgent: null }}
    project="p" story="s" tab="agent" onTab={() => {}} running={!paused} paused={paused} />)
}
describe('Inspector assistant roster', () => {
  it('shows all four roles with real observed state and explicit missing data, not invented idle or zero usage', () => {
    const html = markup([agent('architect_long', 'working')])
    for (const name of ['Kiến trúc sư', 'Người viết', 'Biên tập viên', 'Trọng tài']) expect(html).toContain(`>${name}<`)
    expect(html).toContain('>Đang chạy<')
    expect(html.match(/>Chưa có dữ liệu</g)).toHaveLength(3)
    expect(html).not.toContain('>Chờ lệnh<')
    expect(html).not.toContain('0 token')
  })
  it('merges architect variants into one card, preferring the active snapshot; preserves other states', () => {
    const html = markup([agent('architect_short', 'idle'), agent('writer', 'failed'), agent('architect_long', 'working'), agent('editor', 'idle'), agent('arbiter', 'standby')], true)
    expect(html.match(/title="Kiến trúc sư">Kiến trúc sư<\/div>/g)).toHaveLength(1)
    for (const label of ['Tạm dừng', 'Lỗi', 'Chờ lệnh', 'Gọi khi cần']) expect(html).toContain(`>${label}<`)
  })
})
