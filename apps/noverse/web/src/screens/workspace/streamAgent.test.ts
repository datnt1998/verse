import { describe, expect, it } from 'vitest'
import type { AgentSnapshot, Snapshot } from '../../api/types'
import { streamAgentName, streamAgentOf, streamKindOf } from './streamAgent'

const agent = (name: string, state: string, tool = ''): AgentSnapshot => ({
  name, state, taskId: '', taskKind: '', summary: '', tool, turn: 0,
  context: { tokens: 0, contextWindow: 0, percent: 0, scope: '', strategy: '', activeMessages: 0, summaryMessages: 0, compactedCount: 0, keptCount: 0 },
  updatedAt: '',
})
const snapshot = (phase: string, agents: AgentSnapshot[] | null): Snapshot =>
  ({ phase, agents }) as Snapshot

describe('streamAgentOf — trợ lý đang chạy là chủ khung', () => {
  it('Kiến trúc sư đang làm thì thắng, không về Người viết', () => {
    const agents = [agent('writer', 'idle'), agent('architect_long', 'working', 'save_foundation')]
    expect(streamAgentOf(agents, 'save_foundation')?.name).toBe('architect_long')
  })

  it('agent bận có tool khớp liveTool thắng trước', () => {
    const agents = [agent('writer', 'running', ''), agent('editor', 'running', 'save_review')]
    expect(streamAgentOf(agents, 'save_review')?.name).toBe('editor')
  })

  it('không agent nào bận → null', () => {
    expect(streamAgentOf([agent('writer', 'idle'), agent('architect', 'standby')], '')).toBeNull()
    expect(streamAgentOf(null, 'x')).toBeNull()
  })
})

describe('streamKindOf — màu khung theo trợ lý đang chạy', () => {
  it('architect_long → architect; pha outline không agent → architect; còn lại → writer', () => {
    expect(streamKindOf(null, agent('architect_long', 'working'))).toBe('architect')
    expect(streamKindOf(snapshot('outline', null), null)).toBe('architect')
    expect(streamKindOf(snapshot('writing', null), null)).toBe('writer')
  })
})

describe('streamAgentName — nhãn tiếng Việt', () => {
  it('architect_long → “Kiến trúc sư”; không agent thì theo màu chủ khung', () => {
    expect(streamAgentName(agent('architect_long', 'working'), 'writer')).toBe('Kiến trúc sư')
    expect(streamAgentName(null, 'architect')).toBe('Kiến trúc sư')
    expect(streamAgentName(null, 'writer')).toBe('Người viết')
  })
})
