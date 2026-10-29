import { readdirSync, readFileSync } from 'node:fs'
import { join, relative } from 'node:path'
import { fileURLToPath } from 'node:url'
import { parse, type Rule } from 'postcss'
import { expect, it } from 'vitest'

// A zero-radius blur still creates a filter/compositing layer. Entry motion may
// blur, but a visible filled endpoint must release that layer (especially iOS).
it('releases the filter layer at every visible blur-entry endpoint', () => {
  const src = fileURLToPath(new URL('..', import.meta.url))
  const failures: string[] = []
  for (const entry of readdirSync(src, { recursive: true, withFileTypes: true })) {
    if (!entry.isFile() || !entry.name.endsWith('.css')) continue
    const file = join(entry.parentPath, entry.name)
    // Parse animation blocks independently; legacy declarations elsewhere in a
    // stylesheet must not hide a bad visible endpoint from this guard.
    const animations = [...readFileSync(file, 'utf8').matchAll(/@keyframes\s+[\w-]+\s*\{(?:[^{}]|\{[^{}]*\})*\}/g)].map(match => match[0]).join('\n')
    parse(animations).walkAtRules('keyframes', (frames) => {
      let hasBlur = false
      frames.walkDecls('filter', (decl) => { if (decl.value.includes('blur(')) hasBlur = true })
      if (!hasBlur) return
      const endpoint = (frames.nodes ?? []).find((node): node is Rule => node.type === 'rule' && /(?:^|,)\s*(?:to|100%)\s*(?:,|$)/.test(node.selector))
      if (!endpoint) return
      const declarations = Object.fromEntries(endpoint.nodes.filter(node => node.type === 'decl').map(node => [node.prop, node.value]))
      // Exit animations intentionally finish invisible and are removed by their owners.
      if (declarations.opacity === '0') return
      if (declarations.filter !== 'none') failures.push(`${relative(src, file)}: ${frames.params} ends with filter ${declarations.filter ?? '(implicit)'}`)
    })
  }
  expect(failures, 'Visible text must settle at filter: none, not blur(0)').toEqual([])
})
