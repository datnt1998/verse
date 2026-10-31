import { describe, expect, it } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import * as stylex from '@stylexjs/stylex'
import { space, corners } from '../styles/viderse.stylex'
import { ActionButton } from './ActionButton'

const styles = stylex.create({
  action: { paddingInline: space['--vv-s-20'], borderRadius: corners['--vv-r-7'] },
  selected: { paddingInline: space['--vv-s-24'] },
})

describe('ActionButton StyleX composition', () => {
  it('composes app styles after kit defaults and forwards ordinary DOM props', () => {
    const html = renderToStaticMarkup(<ActionButton data-testid="action" className="action-hook" xstyle={styles.action}>Làm video</ActionButton>)
    const className = html.match(/class="([^"]+)"/)?.[1].split(/\s+/) ?? []
    for (const cls of stylex.props(styles.action).className!.split(/\s+/)) expect(className).toContain(cls)
    expect(className).toContain('action-hook')
    expect(html).toContain('data-testid="action"')
    expect(html).not.toContain('xstyle=')
  })

  it('merges conditional app styles without retaining a conflicting earlier atomic class', () => {
    const html = renderToStaticMarkup(<ActionButton xstyle={[styles.action, styles.selected]}>Làm video</ActionButton>)
    const className = html.match(/class="([^"]+)"/)?.[1].split(/\s+/) ?? []
    const expected = stylex.props(styles.action, styles.selected).className!.split(/\s+/)
    const removed = stylex.props(styles.action).className!.split(/\s+/).filter((cls) => !expected.includes(cls))
    expect(removed.length).toBeGreaterThan(0)
    for (const cls of expected) expect(className).toContain(cls)
    for (const cls of removed) expect(className).not.toContain(cls)
  })
})
