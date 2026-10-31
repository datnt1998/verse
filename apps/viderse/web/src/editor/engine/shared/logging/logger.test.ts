import { describe, expect, it, vi } from 'vitest'
import { createLogger } from './logger'

// Browser pages and module workers do not supply Node's process global.
// Creation exercises getDefaultLevel; startEvent exercises the environment tag.
describe('browser logger environment', () => {
  it('creates a development logger and records an event without process', () => {
    const debug = vi.spyOn(console, 'log').mockImplementation(() => {})
    const info = vi.spyOn(console, 'info').mockImplementation(() => {})
    try {
      vi.stubGlobal('process', undefined)
      try {
        const log = createLogger('browser')
        log.debug('ready')
        const event = log.startEvent('render', 'operation')
        event.set('frames', 30)
        event.success()
      } finally {
        vi.unstubAllGlobals()
      }
      expect(debug).toHaveBeenCalledWith('[browser] ready')
      expect(info).toHaveBeenCalledWith('[browser] render', expect.objectContaining({
        env: 'development', opId: 'operation', frames: 30, outcome: 'success',
      }))
    } finally {
      debug.mockRestore()
      info.mockRestore()
    }
  })
})
