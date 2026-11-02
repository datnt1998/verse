import { act, type ReactNode } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cache } from '@noverse/ui/data'
import { DEFAULT_AUTONOMY } from '../../api/autonomy'
import { getChannel, type Channel } from '../../api/channels'
import type { PostingSchedule } from '../../api/rivals'
import { keys } from '../../data/keys'
import { useResource } from '../../data/useResource'
import { t } from '../../i18n/channels'
import { useRoute } from '../../router'
import { ChannelScreen } from './ChannelScreen'
import { ScheduleCard } from './ScheduleCard'
import { AutonomyCardMobile } from './mobile/AutonomyMobile'

vi.mock('../../data/status', () => ({ useOnline: () => true, useStatus: () => ({ data: {}, status: 'ready' }) }))
const channel: Channel = {
  id: 'controls-channel', name: 'Control channel', youtubeUrl: null, language: 'vi',
  styleNote: null, descriptionFooter: null, alwaysTags: [], autoPick: true, pacePerDay: 3, templateCursor: 0,
  sources: [{ rivalId: 'reference', title: 'Reference channel', handle: null, linkedAt: '2026-01-01T00:00:00Z', reference: true }], templates: [],
  autonomy: DEFAULT_AUTONOMY, priority: 'steady',
  branding: { brief: '', summary: '', look: '', avatar: '', banner: '' }, avatarUrl: null, bannerUrl: null,
  identity: { state: 'ready' }, createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-01-01T00:00:00Z',
}
const reply = (value: unknown, status = 200) => new Response(JSON.stringify(value), { status, headers: { 'Content-Type': 'application/json' } })
const roots: Root[] = []
async function mount(node: ReactNode) {
  const element = document.createElement('div')
  document.body.appendChild(element)
  const root = createRoot(element)
  roots.push(root)
  await act(async () => { root.render(node) })
  return element
}
function RoutedChannel() {
  const { route } = useRoute()
  return route.name === 'channel' ? <ChannelScreen route={route} /> : null
}
function CachedAutonomy() {
  const value = useResource(keys.channel(channel.id), ({ signal }) => getChannel(channel.id, signal))
  return value.data ? <AutonomyCardMobile channel={value.data} /> : null
}

beforeEach(() => {
  vi.stubGlobal('IS_REACT_ACT_ENVIRONMENT', true)
  vi.stubGlobal('matchMedia', vi.fn((query: string) => ({ matches: false, media: query, addEventListener: vi.fn(), removeEventListener: vi.fn() })))
  cache.clear()
  window.location.hash = `#/channels/${channel.id}`
  cache.setData(keys.channel(channel.id), channel)
  cache.setData(keys.channels(), { channels: [channel] })
  vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL) => {
    const url = String(input)
    if (url === `/api/owner-channels/${channel.id}`) return reply(channel)
    if (url === '/api/owner-channels') return reply({ channels: [channel] })
    if (url === '/api/rivals') return reply({ rivals: [], totalVideos: 0, newOutliers: 0 })
    if (url.startsWith('/api/clone?channel=')) return reply({ jobs: [], queuedTotal: 0, bulkEligibility: { retry: [], cancel: [], approve: [], delete: [], bundle: [], posted: [] } })
    if (url.startsWith('/api/backdrop/')) return reply([])
    if (url.includes('/posting-slots')) return reply({ slots: [], total: 0, pending: 0, perWeek: null })
    return reply({ items: [] })
  }))
})
afterEach(async () => {
  await act(async () => { for (const root of roots) root.unmount() })
  roots.length = 0
  document.body.replaceChildren()
  cache.clear()
  vi.unstubAllGlobals()
})

describe('channel control integration', () => {
  it('routes the underline tabs and shows the selected content with roving focus', async () => {
    const element = await mount(<RoutedChannel />)
    const tabs = element.querySelectorAll<HTMLButtonElement>('[role="tab"]')
    expect(tabs).toHaveLength(4)
    await act(async () => { tabs[0].focus(); tabs[0].dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true })) })
    await act(async () => { window.dispatchEvent(new HashChangeEvent('hashchange')) })
    expect(window.location.hash).toContain('tab=sources')
    const selected = element.querySelector<HTMLButtonElement>('[role="tab"][aria-selected="true"]')!
    expect(selected.textContent).toContain(t.channels.page.tabs.sources)
    expect(selected.tabIndex).toBe(0)
    expect(document.activeElement).toBe(selected)
    const panel = element.querySelector('[role="tabpanel"]')!
    expect(panel.getAttribute('aria-labelledby')).toBe(selected.id)
    expect(panel.textContent).toContain(t.channels.sources.rivalsTitle)
    expect([...tabs].filter((tab) => tab.tabIndex === 0)).toHaveLength(1)
  })

  it('loads schedule data matching the selected desktop zone', async () => {
    const fetcher = vi.fn(async (input: RequestInfo | URL) => {
      const zone = String(input).endsWith('zone=ny') ? 'ny' : 'vn'
      const schedule: PostingSchedule = {
        channelId: 'reference', zone, sample: 2, total: 2, pending: 0, pausedUntil: null, perWeek: 2,
        weeksTotal: 1, offSlot: 0, grid: [], slots: [{ weekdays: [0], start: zone === 'ny' ? '08:00' : '20:00', end: '21:00', count: 2, weeksHit: 1, weeksTotal: 1, sinceWeeks: 1, rhythm: 'weekly', isTop: true }],
      }
      return reply(schedule)
    })
    vi.stubGlobal('fetch', fetcher)
    const element = await mount(<ScheduleCard channel={channel} variant="tab" />)
    const choices = element.querySelectorAll<HTMLButtonElement>('[role="radio"]')
    expect(choices).toHaveLength(2)
    expect(element.textContent).toContain('20:00')
    await act(async () => { choices[0].focus(); choices[0].dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true })) })
    expect(choices[1].getAttribute('aria-checked')).toBe('true')
    expect(choices[0].getAttribute('aria-checked')).toBe('false')
    expect(element.textContent).toContain('08:00')
    expect(fetcher.mock.calls.some(([url]) => String(url) === '/api/rivals/reference/posting-slots?zone=ny')).toBe(true)
  })

  it.each([true, false])('keeps mobile autonomy selection consistent with its save response (success: %s)', async (success) => {
    let persisted = channel
    vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      if (String(input) === '/api/owner-channels') return reply({ channels: [persisted] })
      if (String(input) !== `/api/owner-channels/${channel.id}`) throw new Error('Unexpected autonomy request')
      if (init?.method === 'PATCH') {
        if (!success) return reply({ error: 'Save rejected', code: 'conflict' }, 409)
        const body = JSON.parse(String(init.body)) as { autonomy: Channel['autonomy'] }
        persisted = { ...persisted, autonomy: body.autonomy }
      }
      return reply(persisted)
    }))
    const element = await mount(<CachedAutonomy />)
    const trigger = element.querySelector<HTMLButtonElement>('[aria-haspopup="dialog"]')!
    await act(async () => { trigger.click() })
    const choices = document.querySelectorAll<HTMLElement>('[role="dialog"] [role="radio"]')
    const original = [...choices].findIndex((choice) => choice.getAttribute('aria-checked') === 'true')
    const target = (original + 1) % choices.length
    await act(async () => { choices[target].click() })
    if (!success) expect(element.querySelector('[role="alert"]')?.textContent).toContain('Save rejected')
    await act(async () => { trigger.click() })
    const selected = document.querySelectorAll<HTMLElement>('[role="dialog"] [role="radio"]')
    expect(selected[success ? target : original].getAttribute('aria-checked')).toBe('true')
  })
})
