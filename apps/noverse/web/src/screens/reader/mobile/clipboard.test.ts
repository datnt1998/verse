import { afterEach, describe, expect, it, vi } from 'vitest'
import { copy } from '../copy'
import { copyToClipboard } from './clipboard'

afterEach(() => { vi.unstubAllGlobals() })

describe('copyToClipboard (E2EM-17)', () => {
  it('replaces the browser error with the Vietnamese copy failure', async () => {
    vi.stubGlobal('navigator', { clipboard: { writeText: vi.fn().mockRejectedValue(new DOMException("Failed to execute 'writeText' on 'Clipboard': Document is not focused.", 'NotAllowedError')) } })
    await expect(copyToClipboard('/x')).rejects.toThrow(copy.export.copyFailed)
  })

  it('also covers browsers without a clipboard', async () => {
    vi.stubGlobal('navigator', {})
    await expect(copyToClipboard('/x')).rejects.toThrow(copy.export.copyFailed)
  })

  it('writes the text when allowed', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    vi.stubGlobal('navigator', { clipboard: { writeText } })
    await copyToClipboard('/x')
    expect(writeText).toHaveBeenCalledWith('/x')
  })
})
