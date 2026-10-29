import { beforeEach, describe, expect, it, vi } from 'vitest'
import { appendAudit, auditItems, readAudit, validProjects } from './members'
import type { User } from '../src/api/types'
const actor = { username: 'dat', name: 'Dat' } as User
beforeEach(() => { auditItems.length = 0; vi.useRealTimers() })
describe('§AD mock audit', () => {
  it('không bắt buộc lý do; lý do có nhập được giữ nguyên, không ghi mật khẩu', () => {
    appendAudit(actor, 'acct', 'khoá tài khoản', 'linh')
    appendAudit(actor, 'acct', 'khoá tài khoản', 'ha', { reason: 'Nghỉ cộng tác' })
    const page = readAudit(new URLSearchParams('type=acct&actor=dat'))
    expect(page.items).toHaveLength(2); expect(page.items[0].reason).toBe('Nghỉ cộng tác'); expect(page.items[1].reason).toBeUndefined()
    expect(JSON.stringify(page)).not.toContain('password')
  })
  it('lọc và phân trang bằng before không lặp hay bỏ dòng', () => {
    for (let n = 0; n < 4; n++) appendAudit(actor, 'perm', 'đổi vai trò', `u${n}`, { project: 'p', from: 'view', to: 'collab' })
    appendAudit(null, 'sec', 'đăng nhập sai', 'linh')
    const first = readAudit(new URLSearchParams('type=perm&limit=2'))
    const second = readAudit(new URLSearchParams(`type=perm&limit=2&before=${first.next}`))
    expect([...first.items, ...second.items].map((item) => item.target)).toEqual(['u3', 'u2', 'u1', 'u0'])
    expect(second.next).toBe(''); expect(readAudit(new URLSearchParams('actor=system')).items).toHaveLength(1)
  })
  it('chỉ giữ một năm nhật ký và chỉ chấp nhận bản đồ ba vai trò dự án', () => {
    auditItems.push({ id: 'old', at: '2000-01-01', actor: null, type: 'sec', action: 'cũ', target: 'linh' })
    appendAudit(actor, 'acct', 'tạo', 'ngoc')
    expect(auditItems.some((item) => item.id === 'old')).toBe(false)
    expect(validProjects({ p: 'owner', q: 'collab', r: 'view' })).toBe(true)
    expect(validProjects(['p'])).toBe(false); expect(validProjects({ p: 'reader' })).toBe(false); expect(validProjects({})).toBe(true)
  })
})
