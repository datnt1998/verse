// @vitest-environment node
import { readFileSync } from 'node:fs'
import { runInNewContext } from 'node:vm'
import { expect, it, vi } from 'vitest'

interface ClickEvent { notification: { close: () => void; data: { href: string } }; waitUntil: (promise: Promise<void>) => void }
it('focuses an existing app window at the notification video', async () => {
  const handlers: Record<string, (event: ClickEvent) => void> = {}
  const navigate = vi.fn().mockResolvedValue(undefined)
  const focus = vi.fn().mockResolvedValue(undefined)
  const close = vi.fn()
  const self = { location: { origin: 'http://localhost:3000' }, clients: { matchAll: vi.fn().mockResolvedValue([{ navigate, focus }]), openWindow: vi.fn() }, addEventListener: (name: string, handler: (event: ClickEvent) => void) => { handlers[name] = handler } }
  runInNewContext(readFileSync(new URL('../../public/sw.js', import.meta.url), 'utf8'), { self, URL })
  let operation: Promise<void> | undefined
  handlers.notificationclick({ notification: { close, data: { href: '/?video=job-42' } }, waitUntil: (promise: Promise<void>) => { operation = promise } })
  await operation
  expect(close).toHaveBeenCalledOnce()
  expect(navigate).toHaveBeenCalledWith('http://localhost:3000/?video=job-42')
  expect(focus).toHaveBeenCalledOnce()
})
