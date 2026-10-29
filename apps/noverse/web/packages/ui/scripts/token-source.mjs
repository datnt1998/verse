// Parse src/tokens.stylex.ts thành map nhóm → biến — nguồn duy nhất của design token.
// Không import module (stylex.defineVars là compile-time) nên đọc văn bản.
export function parseTokens(text) {
  const groups = {}
  const groupRe = /export const (\w+) = stylex\.defineVars\(\{/g
  let match
  while ((match = groupRe.exec(text))) {
    const name = match[1]
    const bodyStart = match.index + match[0].length
    const bodyEnd = text.indexOf('\n})', bodyStart)
    if (bodyEnd === -1) throw new Error(`Không tìm thấy cuối defineVars của ${name}`)
    const body = text.slice(bodyStart, bodyEnd).replace(/\/\*[\s\S]*?\*\//g, '')
    const entries = {}
    const entryRe = /'(--[\w-]+)':\s*('(?:[^'\\]|\\.)*'|`(?:[^`\\]|\\.)*`)/g
    let e
    while ((e = entryRe.exec(body))) {
      const raw = e[2].slice(1, -1)
      if (entries[e[1]]) throw new Error(`Trùng khoá ${e[1]} trong nhóm ${name}`)
      entries[e[1]] = raw
    }
    groups[name] = entries
  }
  return groups
}

/** Trả về map phẳng biến → giá trị (mọi nhóm gộp lại). */
export function flatTokens(text) {
  return Object.assign({}, ...Object.values(parseTokens(text)))
}
