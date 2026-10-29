import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it } from 'vitest'
import { Screen } from './App'

describe('missing desktop route', () => {
  it('shows a missing page instead of an unrelated empty library', () => {
    const html = renderToStaticMarkup(<Screen parsed={{ route: { name: 'missing' }, dialog: null, search: new URLSearchParams() }} />)
    expect(html).toContain('Không có trang này')
    expect(html).toContain('Về thư viện')
  })
})
