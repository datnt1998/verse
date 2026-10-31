import { describe, expect, it, vi } from 'vitest'
import { ensureFontsLoaded, loadFont } from './font-loader'

describe('browser font loading', () => {
  it('resolves platform and generic fonts locally instead of requesting a nonexistent Google stylesheet', async () => {
    const append = vi.spyOn(document.head, 'appendChild')
    const ready = document.fonts
    Object.defineProperty(document, 'fonts', {
      configurable: true,
      value: { ready: Promise.resolve(), load: vi.fn().mockResolvedValue([]) },
    })
    try {
      expect(loadFont('Arial')).toBe('Arial')
      expect(loadFont('Times New Roman')).toBe('Times New Roman')
      expect(loadFont('monospace')).toBe('monospace')
      await ensureFontsLoaded(['Arial', 'Times New Roman', 'monospace'])
      expect(document.head.querySelector('link[href*="family=Arial"]')).toBeNull()
      expect(append).not.toHaveBeenCalled()
    } finally {
      Object.defineProperty(document, 'fonts', { configurable: true, value: ready })
      append.mockRestore()
    }
  })
})
