// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { Sheet } from './Sheet'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
let root: Root | undefined

afterEach(async () => {
  if (root) await act(() => root!.unmount())
  root = undefined
  document.body.replaceChildren()
  vi.useRealTimers()
  vi.unstubAllGlobals()
})

async function setup(reduced = false) {
  vi.useFakeTimers()
  vi.stubGlobal('matchMedia', () => ({ matches: reduced }))
  const trigger = document.createElement('button')
  trigger.textContent = 'Mở lựa chọn'
  const host = document.createElement('div')
  document.body.append(trigger, host)
  trigger.focus()
  root = createRoot(host)
  const exited = vi.fn(() => ({
    hasSheet: !!document.querySelector('.m-sheet'),
    overflow: document.body.style.overflow,
    focusRestored: document.activeElement === trigger,
  }))
  const render = async (open: boolean) => act(() => root!.render(<Sheet open={open} onClose={() => {}} onExited={exited} title="Chọn kênh"><button>Midnight Tales</button></Sheet>))
  return { render, exited }
}

describe('sheet exit handoff', () => {
  it('does not report an exit before opening; hands off only after releasing the closing layer', async () => {
    const { render, exited } = await setup()
    await render(false)
    expect(exited).not.toHaveBeenCalled()
    await render(true)
    await render(false)
    await act(() => vi.advanceTimersByTime(259))
    expect(exited).not.toHaveBeenCalled()
    expect(document.body.style.overflow).toBe('hidden')
    await act(() => vi.advanceTimersByTime(1))
    expect(exited).toHaveBeenCalledTimes(1)
    expect(exited.mock.results[0].value).toEqual({ hasSheet: false, overflow: '', focusRestored: true })
  })

  it('hands off immediately with reduced motion after removing the sheet and its lock', async () => {
    const { render, exited } = await setup(true)
    await render(true)
    await render(false)
    expect(exited).toHaveBeenCalledTimes(1)
    expect(exited.mock.results[0].value).toEqual({ hasSheet: false, overflow: '', focusRestored: true })
  })

  it('cancels an interrupted exit when reopened rather than running a queued action behind it', async () => {
    const { render, exited } = await setup()
    await render(true)
    await render(false)
    await act(() => vi.advanceTimersByTime(100))
    await render(true)
    await act(() => vi.advanceTimersByTime(300))
    expect(exited).not.toHaveBeenCalled()
    expect(document.querySelector('.m-sheet')).not.toBeNull()
    await render(false)
    await act(() => vi.advanceTimersByTime(260))
    expect(exited).toHaveBeenCalledTimes(1)
  })

  it('discards a closing handoff when its owning route or session unmounts', async () => {
    const { render, exited } = await setup()
    await render(true)
    await render(false)
    await act(() => root!.unmount())
    root = undefined
    await act(() => vi.advanceTimersByTime(300))
    expect(exited).not.toHaveBeenCalled()
    expect(document.body.style.overflow).toBe('')
  })

  it('keeps portal chrome and scrim clicks out of the clickable background row', async () => {
    await setup()
    const background = vi.fn()
    const close = vi.fn()
    await act(() => root!.render(<div onClick={background}><Sheet open onClose={close} title="Thao tác"><button>Chọn</button></Sheet></div>))
    await act(() => (document.querySelector('.m-sheet-title') as HTMLElement).click())
    expect(background).not.toHaveBeenCalled()
    expect(close).not.toHaveBeenCalled()
    await act(() => (document.querySelector('.m-scrim') as HTMLElement).click())
    expect(close).toHaveBeenCalledTimes(1)
    expect(background).not.toHaveBeenCalled()
  })
})
