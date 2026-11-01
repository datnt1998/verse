import { describe, expect, it } from 'vitest'
import type { ProjectSummary, User } from '../../api/types'
import { PROJECTS_PAGE, newStoryEntry, newStoryProject } from './entry'

const user: User = { username: 'dat', name: 'Dat', role: 'member', projects: { own: 'owner', work: 'collab', read: 'view' }, disabled: false, createdAt: '', updatedAt: '', onboardedAt: null, mustChangePassword: false, lastSeenAt: null }
const project = (id: string): ProjectSummary => ({ id, name: id, description: '', ruleCount: 0, style: 'default', legacy: false, storyCount: 0, runningCount: 0, updatedAt: null })

describe('M11 · lối vào Truyện mới giữ dự án đang mở', () => {
  const projects = [project('read'), project('work'), project('own')]

  it('dự án đang mở tạo được truyện → mở đúng dự án đó', () => {
    expect(newStoryProject(projects, user, 'work')).toBe('work')
    expect(newStoryEntry(projects, user, 'work')).toBe('/p/work/new')
  })

  it('dự án đang mở chỉ có quyền xem → lấy dự án đầu tiên tạo được, không mở nhầm dự án chỉ xem', () => {
    expect(newStoryProject(projects, user, 'read')).toBe('work')
    expect(newStoryEntry(projects, user, 'read')).toBe('/p/work/new')
  })

  it('không có dự án nào tạo được → về Dự án & truyện thay vì mở M11 không có đích', () => {
    expect(newStoryProject([project('read')], user, 'read')).toBeNull()
    expect(newStoryEntry([project('read')], user)).toBe(PROJECTS_PAGE)
    expect(newStoryEntry([], null, 'work')).toBe(PROJECTS_PAGE)
  })
})
