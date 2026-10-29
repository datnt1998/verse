import { describe, expect, it } from 'vitest'
import { canCreateStory, canManageProject, canReadProject, canReadStory, canWriteStory, projectRole } from './client'
import type { User } from './types'
const member: User = { username: 'linh', name: 'Linh', role: 'member', projects: { own: 'owner', work: 'collab', read: 'view' }, disabled: false, createdAt: '', updatedAt: '', onboardedAt: null, mustChangePassword: false, lastSeenAt: null }
describe('§AD · hai tầng quyền', () => {
  it.each(['own', 'work', 'read'])('chỉ các dự án được giao mới đọc được: %s', (id) => {
    expect(canReadProject(member, id)).toBe(true); expect(canReadStory(member, `${id}/story`)).toBe(true)
  })
  it('Chỉ xem không viết/tạo; Cộng tác viết/tạo nhưng không quản trị; Chủ dự án quản trị', () => {
    expect(canWriteStory(member, 'read/story')).toBe(false); expect(canCreateStory(member, 'read')).toBe(false)
    expect(canWriteStory(member, 'work/story')).toBe(true); expect(canCreateStory(member, 'work')).toBe(true); expect(canManageProject(member, 'work')).toBe(false)
    expect(canManageProject(member, 'own')).toBe(true)
  })
  it('dự án tạo sau mặc định Không vào; quản trị luôn là Chủ dự án', () => {
    expect(projectRole(member, 'future')).toBeNull(); expect(canReadProject(member, 'future')).toBe(false)
    expect(projectRole({ ...member, role: 'admin' }, 'future')).toBe('owner')
  })
  it('tài khoản bị khoá không giữ được quyền dù còn dữ liệu trong bộ nhớ', () => {
    const locked = { ...member, disabled: true }
    expect(canReadProject(locked, 'own')).toBe(false); expect(canManageProject(locked, 'own')).toBe(false); expect(canWriteStory(locked, 'work/story')).toBe(false)
  })
})
