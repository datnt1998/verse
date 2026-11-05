// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { ExportDoneDialog, ExportingDialog } from './ExportDialogs'

afterEach(() => { cleanup(); vi.restoreAllMocks() })

describe('actual export dialog behavior', () => {
  it('delegates cancellation and leaves unknown progress indeterminate', () => {
    const cancel = vi.fn()
    render(<ExportingDialog seconds={6} fps={30} onCancel={cancel} />)
    expect(screen.getByRole('progressbar').hasAttribute('aria-valuenow')).toBe(false)
    fireEvent.click(screen.getByRole('button', { name: 'Huỷ xuất' }))
    expect(cancel).toHaveBeenCalledOnce()
  })

  it('plays the actual exported source through the overlay and restores the overlay when it ends', () => {
    const play = vi.spyOn(HTMLMediaElement.prototype, 'play').mockImplementation(function (this: HTMLMediaElement) { this.dispatchEvent(new Event('play')); return Promise.resolve() })
    render(<ExportDoneDialog seconds={6} bytes={621793} download="/api/editor/projects/p/exports/e" onClose={vi.fn()} onBack={vi.fn()} />)
    expect(screen.getByRole('link', { name: 'Tải MP4' }).getAttribute('href')).toBe('/api/editor/projects/p/exports/e')
    fireEvent.click(screen.getByRole('button', { name: 'Phát bản vừa xuất' }))
    expect(play).toHaveBeenCalledOnce()
    const video = document.querySelector('video')!
    expect(video.controls).toBe(true)
    expect(screen.queryByRole('button', { name: 'Phát bản vừa xuất' })).toBeNull()
    fireEvent.ended(video)
    expect(screen.getByRole('button', { name: 'Phát bản vừa xuất' })).toBeTruthy()
  })

  it('reports a real playback rejection rather than displaying a fake successful preview', async () => {
    vi.spyOn(HTMLMediaElement.prototype, 'play').mockRejectedValue(new Error('Playback denied'))
    render(<ExportDoneDialog seconds={6} download="/api/editor/projects/p/exports/e" onClose={vi.fn()} onBack={vi.fn()} />)
    fireEvent.click(screen.getByRole('button', { name: 'Phát bản vừa xuất' }))
    expect((await screen.findByRole('alert')).textContent).toBe('Playback denied')
    expect(screen.getByRole('button', { name: 'Phát bản vừa xuất' })).toBeTruthy()
  })
})
