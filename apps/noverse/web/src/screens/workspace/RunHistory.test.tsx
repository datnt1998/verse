// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { getLiveHistory } from '../../api/client'
import { RunHistory } from './RunHistory'

vi.mock('../../api/client', () => ({ getLiveHistory: vi.fn() }))
vi.mock('../../api/useStoryEvents', () => ({ useStoryEvents: () => ({ events: [] }) }))

const operation = { id: 'a', role: 'writer', round: 2, startedAt: '2026-01-01T10:00:00Z', durationMs: 1000, status: 'done' as const, error: '', key: 'read_chapter', params: '{"chapter":2,"source":"draft"}', result: '{}' }
let host: HTMLDivElement
let root: Root
beforeEach(() => {
  ;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
  vi.mocked(getLiveHistory).mockReset().mockResolvedValue({ rounds: [{ round: 2, roundId: 'round-2', role: 'writer', title: 'Viết chương 2', startedAt: operation.startedAt, operations: [operation] }], nextBeforeRound: 2 })
  host = document.createElement('div'); document.body.append(host); root = createRoot(host)
})
afterEach(async () => { await act(async () => root.unmount()); host.remove() })
describe('lịch sử luồng viết', () => {
  it('mở và đóng thẻ không tải lại; chi tiết kỹ thuật đóng sẵn', async () => {
    await act(async () => { root.render(<RunHistory project="p" story="s" />); await Promise.resolve() })
    expect(getLiveHistory).toHaveBeenCalledTimes(1)
    const card = host.querySelector('details')!
    expect(card).toBeTruthy()
    await act(async () => { card.open = true; card.dispatchEvent(new Event('toggle')) })
    expect(host.textContent).toContain('Chi tiết kỹ thuật')
    expect(host.textContent).not.toContain('read_chapter')
    await act(async () => { card.open = false; card.dispatchEvent(new Event('toggle')) })
    expect(getLiveHistory).toHaveBeenCalledTimes(1)
  })
  it('tải vòng cũ hơn với con trỏ độc quyền', async () => {
    await act(async () => { root.render(<RunHistory project="p" story="s" />); await Promise.resolve() })
    const button = [...host.querySelectorAll('button')].find(node => node.textContent?.includes('Tải thêm vòng cũ hơn'))!
    await act(async () => { button.click(); await Promise.resolve() })
    expect(getLiveHistory).toHaveBeenLastCalledWith('p', 's', { beforeRound: 2, limit: 20 })
  })
})
