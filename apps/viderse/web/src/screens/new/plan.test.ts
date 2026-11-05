import { describe, expect, it } from 'vitest'
import { parseUrls } from './plan'

describe('quick-create URL input', () => {
  it('preserves input order for a manual front block', () => {
    expect(parseUrls('  https://youtu.be/a\nhttps://youtu.be/b  ')).toEqual(['https://youtu.be/a', 'https://youtu.be/b'])
  })
  it('retains invalid and repeated lines for truthful server feedback', () => {
    expect(parseUrls('bad-link\nhttps://youtu.be/a\nhttps://youtu.be/a')).toEqual(['bad-link', 'https://youtu.be/a', 'https://youtu.be/a'])
  })
  it('does not manufacture input from whitespace', () => { expect(parseUrls(' \n\t ')).toEqual([]) })
})
