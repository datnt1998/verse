// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { setTimeout as delay } from 'node:timers/promises'
import { invalidate } from '@noverse/ui/data'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import NotificationSettings from './NotificationSettings'
import type { NotificationSettings as Settings } from '../../api/activity'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
let root: Root
let el: HTMLDivElement
let failSave = false
let settings: Settings
const writes: Settings[] = []
const flush = async () => { await act(async () => { await delay(25) }) }

beforeEach(async () => {
  await invalidate()
  writes.length = 0
  failSave = false
  settings = {
    kinds: {
      review_ready: { inApp: true, system: false },
      needs_attention: { inApp: true, system: false },
      ready_to_post: { inApp: true, system: false },
      task_done: { inApp: true, system: false },
      action_result: { inApp: true, system: false },
    },
    doNotDisturb: { enabled: false, start: '22:00', end: '08:00' },
    badge: true, merge: false, recapOnReturn: true, suggestions: { off: [] },
  }
  vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input)
    if (url === '/api/notifications/settings') {
      if (init?.method === 'PUT') {
        writes.push(JSON.parse(String(init.body)) as Settings)
        if (failSave) return Response.json({ error: 'Save refused', reason: 'forbidden' }, { status: 403 })
        settings = writes.at(-1)!
      }
      return Response.json(settings)
    }
    if (url === '/api/channels') return Response.json({ channels: [] })
    return Response.json({ error: 'Not found' }, { status: 404 })
  }))
  el = document.createElement('div')
  document.body.append(el)
  root = createRoot(el)
  await act(async () => root.render(<NotificationSettings />))
  await flush()
})
afterEach(async () => {
  await act(async () => root.unmount())
  el.remove()
  await invalidate()
  vi.unstubAllGlobals()
})

it('saves the changed notification field and displays the persisted choice', async () => {
  const toggle = el.querySelector<HTMLButtonElement>('[role="switch"][aria-label="Gộp thông báo trong 10 giây"]')!
  expect(toggle).toBeTruthy()
  await act(async () => toggle.click())
  await flush()
  expect(writes).toHaveLength(1)
  expect(writes[0].merge).toBe(true)
  expect(writes[0].kinds.review_ready).toEqual({ inApp: true, system: false })
  expect(toggle.getAttribute('aria-checked')).toBe('true')
})

it('reports save rejection without presenting an unpersisted selection', async () => {
  failSave = true
  const toggle = el.querySelector<HTMLButtonElement>('[role="switch"][aria-label="Gộp thông báo trong 10 giây"]')!
  await act(async () => toggle.click())
  await flush()
  expect(writes).toHaveLength(1)
  expect(el.querySelector('[role="alert"]')?.textContent).toContain('Save refused')
  expect(toggle.getAttribute('aria-checked')).toBe('false')
})
