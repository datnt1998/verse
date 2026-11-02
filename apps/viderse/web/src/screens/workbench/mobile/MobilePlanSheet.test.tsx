// @vitest-environment happy-dom
// Sheet Kế hoạch trên điện thoại: cùng máy trạng thái với ô nhắn ở máy tính —
// đọc câu, kế hoạch chưa chạy gì, chặn gửi/chạy trùng, tiến độ thật, kết quả thật,
// lỗi Việt hoá và không cho đóng khi lượt chạy còn chạy.
import { act, type ReactNode } from 'react'
import type * as NoverseUI from '@noverse/ui'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { MobilePlanSheet } from './MobilePlanSheet'
import { assistantCopy } from '../../../i18n/assistant'
import { screenWorkbench } from '../../../i18n/screens'

vi.mock('@noverse/ui', async (importOriginal) => {
  const actual = await importOriginal<typeof NoverseUI>()
  return { ...actual, Sheet: ({ children }: { children: ReactNode }) => <div>{children}</div> }
})

const c = screenWorkbench.mobile.plan
const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status, headers: { 'content-type': 'application/json' } })
const step = (id: string) => ({ id, intent: 'pause', params: {}, title: `Việc ${id}`, sub: `Phụ ${id}`, agent: 'director', enqueuesVideo: false, detail: '', reruns: [] })
const plan = (steps: unknown[]) => ({ ok: true, planId: 'p1', steps, total: { images: 0 } })

let calls: { url: string; body: unknown }[] = []
let roots: Root[] = []
const flush = (ms = 20) => act(async () => {
  const { promise, resolve } = Promise.withResolvers<void>()
  setTimeout(resolve, ms)
  await promise
})

const mount = async (onClose = vi.fn(), onSettled = vi.fn()) => {
  const host = document.createElement('div')
  document.body.append(host)
  const root = createRoot(host)
  roots.push(root)
  await act(async () => { root.render(<MobilePlanSheet open text="tạm dừng video này" scope={{ jobId: 'j1' }} onClose={onClose} onSettled={onSettled} />) })
  return { host, onClose, onSettled }
}
const button = (host: ParentNode, text: string) => {
  const node = [...host.querySelectorAll('button')].find((candidate) => candidate.textContent?.includes(text))
  return node instanceof HTMLButtonElement ? node : undefined
}
const click = async (target: Element | undefined | null) => {
  expect(target).toBeTruthy()
  if (!(target instanceof HTMLElement)) throw new Error('thiếu nút để bấm')
  await act(async () => { target.click() })
  await flush()
}

beforeEach(() => {
  const actEnvironment = globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }
  actEnvironment.IS_REACT_ACT_ENVIRONMENT = true
  calls = []
  roots = []
})
afterEach(async () => {
  await act(async () => { for (const root of roots.splice(0)) root.unmount() })
  document.body.replaceChildren()
  vi.unstubAllGlobals()
})

describe('Sheet Kế hoạch', () => {
  it('mở sheet là đọc câu một lần, chỉ hiện kế hoạch, chưa chạy gì', async () => {
    let resolvePlan: (value: Response) => void = () => {}
    vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input)
      calls.push({ url, body: init?.body ? JSON.parse(String(init.body)) : undefined })
      if (url === '/api/assistant/plan') {
        const { promise, resolve } = Promise.withResolvers<Response>()
        resolvePlan = resolve
        return promise
      }
      return json({ error: 'not found' }, 404)
    }))
    const { host } = await mount()
    expect(calls.filter((call) => call.url === '/api/assistant/plan')).toHaveLength(1)
    expect(host.textContent).toContain(assistantCopy.states.planningWho)
    await act(async () => { resolvePlan(json(plan([step('a')]))) })
    await flush()
    expect(host.textContent).toContain(c.head(1))
    expect(host.textContent).toContain('Việc a')
    expect(calls.filter((call) => call.url === '/api/assistant/run-plan')).toHaveLength(0)
    expect(button(host, c.run(1))).toBeTruthy()
  })

  it('bấm Làm hai lần sát nhau chỉ chạy một lượt, tiến độ là dữ liệu thật', async () => {
    vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input)
      calls.push({ url, body: init?.body ? JSON.parse(String(init.body)) : undefined })
      if (url === '/api/assistant/plan') return json(plan([step('a')]))
      if (url === '/api/assistant/run-plan') return json({ runId: 'r1', state: 'running', steps: [{ id: 'a', state: 'queued' }] }, 202)
      return json({ runId: 'r1', state: 'running', steps: [{ id: 'a', state: 'queued' }] })
    }))
    const { host, onClose } = await mount()
    const run = button(host, c.run(1))!
    await act(async () => { run.click(); run.click() })
    await flush()
    expect(calls.filter((call) => call.url === '/api/assistant/run-plan')).toHaveLength(1)
    expect(host.textContent).toContain(assistantCopy.progress.queued)
    expect(host.textContent).toContain(assistantCopy.states.keepOpenWhileRunning)
    expect(onClose).not.toHaveBeenCalled()
    expect(button(host, c.close)).toBeUndefined()
  })

  it('lượt chạy xong hiện câu kết quả thật và cho đóng', async () => {
    vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input)
      calls.push({ url, body: init?.body ? JSON.parse(String(init.body)) : undefined })
      if (url === '/api/assistant/plan') return json(plan([step('a')]))
      if (url === '/api/assistant/run-plan') return json({ runId: 'r1', state: 'done', steps: [{ id: 'a', state: 'done', message: 'Đã tạm dừng video này.' }] })
      return json({ error: 'not found' }, 404)
    }))
    const { host, onClose } = await mount()
    await click(button(host, c.run(1)))
    expect(host.textContent).toContain('Đã tạm dừng video này.')
    await click(button(host, c.close))
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('lỗi tiếng Anh không lộ ra; Thử lại lập kế hoạch lại từ đúng câu', async () => {
    let fail = true
    vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input)
      calls.push({ url, body: init?.body ? JSON.parse(String(init.body)) : undefined })
      if (url !== '/api/assistant/plan') return json({ error: 'not found' }, 404)
      return fail ? json({ error: 'internal failure' }, 500) : json(plan([step('a')]))
    }))
    const { host } = await mount()
    expect(host.textContent).toContain(assistantCopy.states.errorTitle)
    expect(host.textContent).toContain(assistantCopy.states.failed)
    expect(host.textContent).not.toContain('internal failure')
    fail = false
    await click(button(host, assistantCopy.states.retry))
    expect(calls.filter((call) => call.url === '/api/assistant/plan')).toHaveLength(2)
    expect(host.textContent).toContain('Việc a')
  })
})
