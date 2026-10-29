import { describe, expect, it } from 'vitest'
import { parseHash, paths, routePath, type Route } from './router'

describe('parseHash', () => {
  it('parse mọi route đúng đường dẫn do paths sinh ra (round trip)', () => {
    const cases: [Route, string][] = [
      [{ name: 'home' }, paths.home()],
      [{ name: 'account' }, paths.account()],
    ]
    for (const [route, path] of cases) {
      expect(routePath(route)).toBe(path)
      expect(parseHash(`#${path}`).route).toEqual(route)
    }
  })

  it('hash rỗng và #/ đều là home', () => {
    expect(parseHash('').route).toEqual({ name: 'home' })
    expect(parseHash('#').route).toEqual({ name: 'home' })
    expect(parseHash('#/').route).toEqual({ name: 'home' })
  })

  it('route lạ rơi về home', () => {
    for (const hash of ['#/khong-co', '#/account/extra', '#/setup', '#/login', '#/p/x']) {
      expect(parseHash(hash).route, hash).toEqual({ name: 'home' })
    }
  })

  it('giữ query trong search', () => {
    const parsed = parseHash('#/account?x=1&y=a b')
    expect(parsed.route).toEqual({ name: 'account' })
    expect(parsed.search.get('x')).toBe('1')
    expect(parsed.search.get('y')).toBe('a b')
  })
})
