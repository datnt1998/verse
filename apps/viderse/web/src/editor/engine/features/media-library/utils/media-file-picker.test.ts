// @vitest-environment node

import { afterEach, describe, expect, it, vi } from 'vitest'
import {
  getSupportedMediaFormatLabels,
  MEDIA_FILE_PICKER_TYPES,
  pickMediaFiles,
} from './media-file-picker'

type Listener = () => void

class FakeInput {
  type = ''
  multiple = false
  accept = ''
  hidden = false
  files: File[] | null = null
  removed = false
  clicked = false
  private listeners = new Map<string, Listener>()

  addEventListener(name: string, listener: Listener): void {
    this.listeners.set(name, listener)
  }
  remove(): void {
    this.removed = true
  }
  click(): void {
    this.clicked = true
  }
  fire(name: string): void {
    this.listeners.get(name)?.()
  }
}

function stubDocument(): FakeInput {
  const input = new FakeInput()
  vi.stubGlobal('document', {
    createElement: vi.fn(() => input),
    body: { append: vi.fn() },
  })
  return input
}

describe('media-file-picker', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('opens a hidden multi-file input restricted to the supported media extensions', async () => {
    const input = stubDocument()

    const picked = pickMediaFiles()
    input.files = [new File(['x'], 'a.mp4')]
    input.fire('change')
    await picked

    expect(input.type).toBe('file')
    expect(input.hidden).toBe(true)
    expect(input.multiple).toBe(true)
    expect(input.clicked).toBe(true)
    expect(input.accept.split(',')).toEqual(
      Object.values(MEDIA_FILE_PICKER_TYPES[0].accept).flat(),
    )
  })

  it('honours multiple: false', async () => {
    const input = stubDocument()

    const picked = pickMediaFiles({ multiple: false })
    input.files = [new File(['x'], 'a.mp4')]
    input.fire('change')
    await picked

    expect(input.multiple).toBe(false)
  })

  it('resolves with the selected File objects and removes the input', async () => {
    const input = stubDocument()
    const file = new File(['x'], 'clip.mp4', { type: 'video/mp4' })

    const picked = pickMediaFiles()
    input.files = [file]
    input.fire('change')

    await expect(picked).resolves.toEqual([file])
    expect(input.removed).toBe(true)
  })

  it('rejects with an AbortError when the dialog is cancelled', async () => {
    const input = stubDocument()

    const picked = pickMediaFiles()
    input.fire('cancel')

    await expect(picked).rejects.toMatchObject({ name: 'AbortError' })
    expect(input.removed).toBe(true)
  })

  it('treats an empty selection as a cancel', async () => {
    const input = stubDocument()

    const picked = pickMediaFiles()
    input.files = []
    input.fire('change')

    await expect(picked).rejects.toMatchObject({ name: 'AbortError' })
  })

  it('includes every accepted media extension in display order', () => {
    expect(getSupportedMediaFormatLabels()).toEqual([
      'MP4',
      'WebM',
      'MOV',
      'AVI',
      'MKV',
      'MP3',
      'WAV',
      'OGG',
      'M4A',
      'AAC',
      'JPG',
      'JPEG',
      'PNG',
      'GIF',
      'WebP',
      'SVG',
      'JSON',
      'LOTTIE',
    ])
  })
})
