// @vitest-environment happy-dom
import { readFileSync } from 'node:fs'
import { URL as NodeURL } from 'node:url'
import { parse, type Rule, type AtRule } from 'postcss'
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, expect, it, vi } from 'vitest'
const state = vi.hoisted(() => ({ streamRenders: 0, streamText: 'A paragraph in the stream.', snapshot: { isRunning: true, runtimeState: 'running', phase: 'writing', flow: 'writing', totalChapters: 8, currentChapter: 2, inProgressChapter: 2, completedCount: 1, agents: [], outline: [] }, events: [] }))
vi.mock('../../../api/session', () => ({ useSession: () => ({ user: { username: 'dat', role: 'admin', projects: {} } }) }))
vi.mock('../../../api/useStoryEvents', () => ({ useStoryEvents: () => ({ snapshot: state.snapshot, events: state.events, streamText: state.streamText, connected: true, blocked: null, updatedAt: null }) }))
vi.mock('../../../data', () => ({ keys: { story: (...key: unknown[]) => key }, invalidate: vi.fn(), useResource: () => ({ data: null }), useAction: (fn: (...args: unknown[]) => unknown) => ({ run: fn, busy: false, pending: false, error: null }) }))
vi.mock('../../../mobile/runtime', () => ({ useMobileRuntime: () => ({ keyboard: 0, offline: false, seconds: 0 }), useStoryConnection: vi.fn(), useMobileScroll: () => ({ down: false }) }))
vi.mock('../../../mobile/primitives', () => ({ useMobileToast: () => vi.fn(), ActionSheet: () => null }))
vi.mock('../../../mobile/outbox', () => ({ enqueueInput: vi.fn(), useInputOutbox: () => [] }))
vi.mock('../../notify/useNotify', () => ({ useNotificationInbox: () => ({ data: null }) }))
vi.mock('../../../shell/jobs/useJobs', () => ({ useActiveJobs: () => [] }))
vi.mock('../../../shell/useNames', () => ({ useStoryName: () => 'Story' }))
vi.mock('../../../lib/StreamMarkdown', () => ({ StreamMarkdown: () => { state.streamRenders++; return <p>stream</p> } }))
vi.mock('./CommandSheet', () => ({ CommandSheet: () => null }))
vi.mock('./JobsSheet', () => ({ JobsSheet: () => null }))
vi.mock('./PauseSheet', () => ({ PauseSheet: () => null }))
vi.mock('../../../tablet/TabletSlashPopover', () => ({ TabletSlashPopover: () => null }))
vi.mock('../../../commands/store', () => ({ handleSlashKey: vi.fn() }))
import { MobileWorkspace } from './index'
const workspaceCss = readFileSync(new NodeURL('./workspace.css', import.meta.url), 'utf8')
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
afterEach(() => { state.streamRenders = 0; state.streamText = 'A paragraph in the stream.'; state.snapshot.flow = 'writing'; state.snapshot.isRunning = true })
it('composer input does not render the stream subtree', async () => {
  const container = document.createElement('div'); document.body.append(container)
  const root = createRoot(container)
  try {
    await act(async () => root.render(<MobileWorkspace project="p" story="s" />))
    const renders = state.streamRenders
    const input = container.querySelector('textarea')!
    await act(async () => {
      Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value')!.set!.call(input, 'A new draft')
      input.dispatchEvent(new Event('input', { bubbles: true }))
    })
    expect(input.value).toBe('A new draft')
    expect(state.streamRenders).toBe(renders)
  } finally { await act(async () => root.unmount()); container.remove() }
})

it('labels the running assistant with its current action and running state', async () => {
  const container = document.createElement('div'); document.body.append(container)
  const root = createRoot(container)
  try {
    await act(async () => root.render(<MobileWorkspace project="p" story="s" />))
    expect(container.querySelector('.m-agent.active')?.getAttribute('aria-label')).toBe('Người viết · Viết bản nháp · đang chạy')
  } finally { await act(async () => root.unmount()); container.remove() }
})

it('keeps the running action element and shimmer class across step changes', async () => {
  const container = document.createElement('div'); document.body.append(container)
  const root = createRoot(container)
  try {
    await act(async () => root.render(<MobileWorkspace project="p" story="s" />))
    const action = container.querySelector('.m-agent.active small')!
    expect(action.classList.contains('mw-agent-action-live')).toBe(true)
    state.snapshot.flow = 'reviewing'
    await act(async () => root.render(<MobileWorkspace project="p" story="s" />))
    const updated = container.querySelector('.m-agent.active small')!
    expect(updated).toBe(action)
    expect(updated.textContent).toBe('Soát nhất quán')
    expect(updated.className).toBe(action.className)
  } finally { await act(async () => root.unmount()); container.remove() }
})

it('mounts moving orb layers only for running cards and removes them at idle', async () => {
  const container = document.createElement('div'); document.body.append(container)
  const root = createRoot(container)
  try {
    await act(async () => root.render(<MobileWorkspace project="p" story="s" />))
    expect(container.querySelectorAll('.mw-agent-halo')).toHaveLength(1)
    expect(container.querySelectorAll('.mw-agent-highlight')).toHaveLength(1)
    expect(container.querySelectorAll('.m-agent:not(.active) .mw-agent-action-live, .m-agent:not(.active) .mw-agent-halo, .m-agent:not(.active) .mw-agent-highlight')).toHaveLength(0)
    state.snapshot.isRunning = false
    await act(async () => root.render(<MobileWorkspace project="p" story="s" />))
    expect(container.querySelectorAll('.mw-agent-action-live, .mw-agent-halo, .mw-agent-highlight')).toHaveLength(0)
  } finally { await act(async () => root.unmount()); container.remove() }
})

it('has reduced-motion branches for a half-opacity halo and static coloured action', () => {
  const rules: Rule[] = []
  parse(workspaceCss).walkAtRules('media', (media) => {
    if (media.params.includes('prefers-reduced-motion')) media.walkRules((rule) => { rules.push(rule) })
  })
  const value = (rule: Rule | undefined, property: string) => rule?.nodes.find((node) => node.type === 'decl' && node.prop === property)
  const stopped = value(rules.find((rule) => rule.selector.includes('.mw-workspace *')), 'animation')
  expect(stopped?.type === 'decl' ? stopped.value : undefined).toBe('none')
  const halo = value(rules.find((rule) => rule.selector === '.mw-agent-halo'), 'opacity')
  expect(halo?.type === 'decl' ? halo.value : undefined).toBe('.5')
  const action = rules.find((rule) => rule.selector === '.mw-workspace .m-agent .mw-agent-action-live')
  const background = value(action, 'background-image'), colour = value(action, 'color')
  expect(background?.type === 'decl' ? background.value : undefined).toBe('none')
  expect(colour?.type === 'decl' ? colour.value : undefined).toBe('var(--mw-agent-text)')
})

it('moves the 250% shimmer exactly one image period without a backward loop jump', () => {
  let frames: AtRule | undefined
  parse(workspaceCss).walkAtRules('keyframes', (rule) => { if (rule.params === 'mw-agent-shimmer') frames = rule })
  expect(frames).toBeDefined()
  const positions: string[] = []
  frames!.walkDecls('background-position', (decl) => { positions.push(decl.value) })
  const start = parseFloat(positions[0]) / 100
  const endPosition = positions[1]
  const fraction = endPosition.match(/calc\(([-\d.]+)%\s*\/\s*([\d.]+)\)/)
  const end = fraction ? Number(fraction[1]) / Number(fraction[2]) / 100 : parseFloat(endPosition) / 100
  const width = 100, image = 2.5 * width
  const startOffset = (width - image) * start
  const endOffset = (width - image) * end
  expect(endOffset).toBeGreaterThan(startOffset)
  expect(endOffset - startOffset).toBeCloseTo(image, 3)
  expect(((endOffset - startOffset) % image) / image).toBeCloseTo(0, 5)
})

it.each([false, true])('does not fade the preparation text or orb (tablet=%s)', async (tablet) => {
  state.streamText = ''
  const container = document.createElement('div')
  container.className = tablet ? 'tablet-left' : ''
  document.body.append(container)
  const root = createRoot(container)
  try {
    await act(async () => root.render(<MobileWorkspace project="p" story="s" tablet={tablet} />))
    const stream = container.querySelector('.m-stream')!
    expect(stream.textContent).toContain('Người viết đang chuẩn bị')
    expect(stream.querySelector('canvas')).not.toBeNull()
    const tabletCss = readFileSync(new NodeURL('../../../tablet/tablet.css', import.meta.url), 'utf8')
    const overlays: string[] = []
    parse(`${workspaceCss}\n${tabletCss}`).walkRules((rule) => {
      if (!rule.selector.endsWith(':after')) return
      const selector = rule.selector.replace(/::?after$/, '')
      if (stream.matches(selector)) overlays.push(rule.selector)
    })
    expect(overlays, 'Preparation text and its orb must not sit under a stream fade').toEqual([])
  } finally { await act(async () => root.unmount()); container.remove() }
})

it('keeps the header line boxes on whole pixels without scaling text on press', () => {
  const declarations = (selector: string) => {
    const values: Record<string, string> = {}
    parse(workspaceCss).walkRules((rule) => {
      if (rule.selector === selector) rule.walkDecls((decl) => { values[decl.prop] = decl.value })
    })
    return values
  }
  expect(declarations('.mw-story-copy strong')['line-height']).toBe('20px')
  expect(declarations('.mw-story-copy small')['line-height']).toBe('16px')
  expect(declarations('.mw-story-link')['transform']).toBe('none')
  expect(declarations('.mw-story-link:active')['transform']).toBe('none')
})
