import { describe, expect, it } from 'vitest'
import { COMMANDS } from '../../../../mock/data'
import type { HostEvent, Snapshot, User } from '../../../api/types'
import { agentStatus, COMMAND_SHORT, composerPrefill, hasMobileResult, MAX_COMPOSER, providerStoppedRole, retryText, writingPercent, WRITING_STEPS, mobileInitialValues, resultHref, reviewScore, workState, workspaceActions } from './logic'

const snapshot = (patch: Partial<Snapshot> = {}) => ({ runtimeState: 'idle', phase: 'writing', isRunning: false, waitingFor: '', advanceMode: 'auto', advancePermitChapter: 25, completedCount: 24, currentChapter: 25, ...patch }) as Snapshot
const failed = { failed: true, level: 'error' } as HostEvent

describe('M1 state and navigation', () => {
  it('projects classified provider failure to the actual failed or latest active agent, never always the writer', () => {
    const agents = [{ name: 'writer', state: 'running', updatedAt: '2026-11-02T04:00:00Z' }, { name: 'editor', state: 'error', updatedAt: '2026-11-02T03:59:00Z' }] as Snapshot['agents']
    const stopped = snapshot({ agents, providerError: { provider: 'openai', kind: 'expired', message: '' } })
    expect(providerStoppedRole(stopped)).toBe('editor')
    expect(providerStoppedRole(snapshot({ ...stopped, isRunning: true }))).toBeNull()
    expect(providerStoppedRole(snapshot({ agents }))).toBeNull()
    expect(providerStoppedRole(snapshot({ ...stopped, agents: agents!.map((agent) => ({ ...agent, state: 'running' })) }))).toBe('writer')
    const context = { state: 'idle' as const, stale: false, active: false, task: '', nextChapter: 4, queued: false }
    expect(agentStatus('editor', { ...context, providerStop: 'expired' })).toBe('Dừng do lỗi')
    expect(agentStatus('editor', { ...context, providerStop: 'quota' })).toBe('Chờ lượt, sẽ tự thử lại')
    expect(agentStatus('writer', context)).toBe('Đang chờ')
  })

  it('hides writer and export actions for a story reader on phone and tablet (18-mobile §M2, POST /exports needs write)', () => {
    const reader: User = { username: 'lan', name: 'Lan', role: 'member', projects: { p: 'view' }, disabled: false, createdAt: '', updatedAt: '', onboardedAt: null, mustChangePassword: false, lastSeenAt: null }
    for (const tablet of [false, true]) {
      const actions = workspaceActions(reader, 'p/s', tablet)
      expect(actions.write).toBe(false)
      expect(actions.export).toBe(false)
      expect(actions.quickCommands).toEqual([])
    }
    expect(workspaceActions({ ...reader, projects: { p: 'collab' } }, 'p/s').quickCommands.map((item) => item.name)).toEqual(['next', 'diag', 'export', 'review'])
    expect(workspaceActions({ ...reader, projects: { p: 'collab' } }, 'p/s', true).quickCommands.map((item) => item.name)).toEqual(['cocreate', 'diag', 'export'])
    expect(workspaceActions({ ...reader, projects: { p: 'collab' } }, 'other/s')).toMatchObject({ write: false, export: false, quickCommands: [] })
    expect(workspaceActions(null, 'p/s')).toMatchObject({ write: false, export: false, quickCommands: [] })
    expect(workspaceActions({ ...reader, role: 'admin', projects: {} }, 'p/other')).toMatchObject({ write: true, export: true })
  })
  it('defaults /export to the real chapter count and story slug, not the artboard sample', () => {
    const exportSpec = COMMANDS.find((entry) => entry.name === 'export')!
    expect(mobileInitialValues(exportSpec, '01-tra-xanh', 8).values).toMatchObject({ from: '1', to: '8', path: 'exports/01-tra-xanh.epub' })
    expect(mobileInitialValues(exportSpec, 'x', 0).values.to).toBe('')
    const diagSpec = COMMANDS.find((entry) => entry.name === 'diag')!
    expect(mobileInitialValues(diagSpec, 'x', 8).values).toEqual(mobileInitialValues(diagSpec, 'y', 3).values)
  })
  it('maps six engine states, without reviving historical errors while running or paused', () => {
    expect(workState(snapshot({ isRunning: true }), [failed])).toBe('running')
    expect(workState(snapshot({ waitingFor: 'gate' }), [])).toBe('gate')
    expect(workState(snapshot({ runtimeState: 'paused', waitingFor: 'gate' }), [])).toBe('gate')
    expect(workState(snapshot({ runtimeState: 'paused', waitingFor: 'gate', inProgressChapter: 28, completedCount: 27 }), [])).toBe('paused')
    expect(workState(snapshot(), [])).toBe('idle')
    expect(workState(snapshot({ runtimeState: 'paused' }), [failed])).toBe('paused')
    expect(workState(snapshot({ phase: 'complete' }), [failed])).toBe('done')
    expect(workState(snapshot(), [failed])).toBe('error')
    expect(workState(snapshot(), [failed, { failed: false, level: 'success' } as HostEvent])).toBe('idle')
  })
  it('routes every dedicated result command to M5, not local-only commands or command prefixes', () => {
    for (const name of ['cocreate', 'import', 'diag', 'sync', 'reopen']) expect(hasMobileResult(`/${name} --check`)).toBe(true)
    for (const line of ['/export epub', '/review on', '/model writer', '/syncing', '/help']) expect(hasMobileResult(line)).toBe(false)
  })
  it('preserves full command and selected story in push navigation, including Vietnamese and spaces', () => {
    const href = resultHref('dự án', 'trà xanh', '/import nhap/truyen.txt --guide=Chương hồi riêng')
    expect(href.split('?')[0]).toBe('/settings/mobile')
    const params = new URLSearchParams(href.split('?')[1])
    expect(Object.fromEntries(params)).toEqual({ result: 'new', project: 'dự án', story: 'trà xanh', command: '/import nhap/truyen.txt --guide=Chương hồi riêng' })
    expect(new URLSearchParams(resultHref('p', 's', '/sync', 'job-1').split('?')[1]).get('result')).toBe('job-1')
  })
  it('consumes guide prefill once while preserving other query fields and edited drafts on return', () => {
    const path = '/p/p/s/s/work'
    const prefill = composerPrefill(`#${path}?command=%2Freview+on&focus=writer`, path)
    expect(prefill).toEqual({ command: '/review on', hash: `#${path}?focus=writer` })
    expect(composerPrefill(prefill!.hash, path)).toBeNull()
    expect(composerPrefill('#/settings/mobile?command=%2Fdiag&result=new', path)).toBeNull()
    expect(composerPrefill('#/p/p/s/other/work?command=%2Fdiag', path)).toBeNull()
    expect(composerPrefill(`#${path}?command=`, path)).toBeNull()
  })
  it('shows actual review average in Vietnamese and omits missing scores rather than fabricating one', () => {
    expect(reviewScore([{ score: 8 }, { score: 8.2 }])).toBe('8,1')
    expect(reviewScore([])).toBeNull()
    expect(reviewScore()).toBeNull()
    expect(reviewScore([{ score: 0 }])).toBe('0')
  })
})

describe('MWork parity helpers', () => {
  it('writingPercent = step × 20 (MWork 3/5 = 60%), unknown flow = 20%', () => {
    expect(writingPercent('reviewing')).toBe(60)
    expect(writingPercent('polishing')).toBe(100)
    expect(writingPercent(undefined)).toBe(20)
    expect(Object.values(WRITING_STEPS).map((step) => step.label)).toEqual(['Đọc bối cảnh', 'Viết bản nháp', 'Soát nhất quán', 'Biên tập chấm điểm', 'Lưu chương'])
  })
  it('agentStatus follows the artboard AGS rows', () => {
    const ctx = { state: 'idle' as const, stale: false, active: false, task: 'Soát nhất quán', nextChapter: 25, queued: false }
    expect(agentStatus('writer', ctx)).toBe('Đang chờ')
    expect(agentStatus('writer', { ...ctx, state: 'error' })).toBe('Dừng do lỗi')
    expect(agentStatus('writer', { ...ctx, state: 'running', active: true })).toBe('Soát nhất quán')
    expect(agentStatus('writer', { ...ctx, state: 'running', stale: true })).toBe('—')
    expect(agentStatus('editor', ctx)).toBe('Chờ chương 25')
    expect(agentStatus('arbiter', { ...ctx, queued: true })).toBe('Chờ góp ý')
    expect(agentStatus('architect', { ...ctx, state: 'done' })).toBe('Đã xong')
    expect(agentStatus('writer', { ...ctx, state: 'gate' })).toBe('Chờ bạn duyệt')
    expect(agentStatus('writer', { ...ctx, state: 'paused' })).toBe('Đã tạm dừng')
    expect(agentStatus('writer', { ...ctx, state: null })).toBe('—')
    expect(agentStatus('architect', ctx)).toBe('Chờ lệnh')
    expect(agentStatus('arbiter', ctx)).toBe('Gọi khi cần')
  })
  it('SHORT command titles, retry banner text and 4-line composer cap', () => {
    expect(COMMAND_SHORT.next).toBe('Viết chương tiếp')
    expect(COMMAND_SHORT.diag).toBe('Khám sức khoẻ truyện')
    expect(retryText(3)).toBe('Thử lại sau 3 giây · máy vẫn chạy')
    expect(MAX_COMPOSER).toBeCloseTo(105.6)
  })
})
