// @vitest-environment happy-dom
import { describe, expect, it } from 'vitest'
import { parseHash, routePath } from './router'

describe('headless editor request routes', () => {
  it.each(['render', 'measure'])('keeps scoped token when parsing and normalizing %s', (name) => {
    const parsed = parseHash(`#/${name}/request%2Fone?token=single%2Buse`)
    expect(parsed.route).toEqual({ name, requestId: 'request/one', token: 'single+use' })
    expect(routePath(parsed.route)).toBe(`/${name}/request%2Fone?token=single%2Buse`)
  })
  it('keeps editor project IDs escaped exactly once', () => {
    const parsed = parseHash('#/editor/project%252Fone')
    expect(parsed.route).toEqual({ name: 'editor', projectId: 'project%2Fone' })
    expect(routePath(parsed.route)).toBe('/editor/project%252Fone')
  })
})
