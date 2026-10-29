import { describe, expect, it } from 'vitest'
import { parseHash, paths } from '../router'

describe('route Hướng dẫn lệnh (13-commands.md §2)', () => {
  it('đọc #/help/commands/:name?', () => {
    expect(parseHash('#/help/commands').route).toEqual({ name: 'help', cmd: null })
    expect(parseHash('#/help/commands/export').route).toEqual({ name: 'help', cmd: 'export' })
    expect(parseHash('#/help/commands/cocreate?x=1').route).toEqual({ name: 'help', cmd: 'cocreate' })
    expect(parseHash('#/help').route).toEqual({ name: 'help', cmd: null })
  })

  it('dựng lại đúng hash', () => {
    expect(paths.help()).toBe('/help/commands')
    expect(paths.help('export')).toBe('/help/commands/export')
    expect(parseHash(`#${paths.help('export')}`).route).toEqual({ name: 'help', cmd: 'export' })
  })
})
