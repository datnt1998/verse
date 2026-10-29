import { describe, expect, it } from 'vitest'
import { parseAuthCode } from './authCode'

describe('parseAuthCode (dán mã đăng nhập Claude)', () => {
  it('mã thô code#state', () => {
    expect(parseAuthCode('abc#xyz')).toEqual({ code: 'abc', state: 'xyz' })
    expect(parseAuthCode('  abc#xyz\n')).toEqual({ code: 'abc', state: 'xyz' })
    expect(parseAuthCode('abc#xyz#thừa')).toEqual({ code: 'abc', state: 'xyz' })
  })
  it('mã thô không có state', () => {
    expect(parseAuthCode('abc')).toEqual({ code: 'abc', state: '' })
  })
  it('URL chuyển hướng đầy đủ', () => {
    expect(parseAuthCode('http://localhost:54545/callback?code=abc&state=xyz')).toEqual({ code: 'abc', state: 'xyz' })
    expect(parseAuthCode('https://platform.claude.com/oauth/code/callback?code=abc&state=xyz#frag')).toEqual({ code: 'abc', state: 'xyz' })
  })
  it('chuỗi query code=…&state=…', () => {
    expect(parseAuthCode('code=abc&state=xyz')).toEqual({ code: 'abc', state: 'xyz' })
    expect(parseAuthCode('?code=abc&state=xyz')).toEqual({ code: 'abc', state: 'xyz' })
  })
  it('trống hoặc URL không có code → null', () => {
    expect(parseAuthCode('')).toBeNull()
    expect(parseAuthCode('   ')).toBeNull()
    expect(parseAuthCode('https://claude.ai/oauth/authorize?state=xyz')).toBeNull()
  })
  it('mã có dấu hai chấm không bị hiểu nhầm là URL', () => {
    expect(parseAuthCode('ab:cd#xyz')).toEqual({ code: 'ab:cd', state: 'xyz' })
  })
})
