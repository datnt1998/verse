import { golden } from './golden'
import type { Lesson, LessonAction, LessonState, LessonsResponse, ViolationsResponse } from '../src/api/types'

// Bài học phát lại golden do Go test ghi (internal/entry/web + internal/lessons):
// gom nhóm/quan sát/văn bản tạo đều do backend tính. Mock chỉ giữ ba chuyển
// trạng thái mà POST /api/system/lessons/{id} phải phản ánh: Giữ | Bỏ | Sửa
// (đề xuất đã "Đã giữ"/"Đã bỏ" rồi chọn lại quay về Chờ duyệt như Decide thật).

function allLessons(): Lesson[] {
  return [
    ...golden<LessonsResponse>('lessons-pending.json').lessons,
    ...golden<LessonsResponse>('lessons-kept.json').lessons,
    ...golden<LessonsResponse>('lessons-dropped.json').lessons,
  ]
}

const lessons: Lesson[] = allLessons()

export function mockLessons(state: LessonState): LessonsResponse {
  const base = golden<LessonsResponse>(`lessons-${state}.json`)
  return {
    lessons: lessons.filter((l) => l.state === state),
    counts: {
      pending: lessons.filter((l) => l.state === 'pending').length,
      kept: lessons.filter((l) => l.state === 'kept').length,
      dropped: lessons.filter((l) => l.state === 'dropped').length,
    },
    stats: base.stats,
    packs: base.packs,
  }
}

export function mockViolations(pack: string): ViolationsResponse | null {
  const base = golden<LessonsResponse>('lessons-pending.json')
  const entry = base.packs.find((p) => p.id === pack)
  if (!entry) return null
  return golden<ViolationsResponse>(`violations-${pack}.json`)
}

export function mockPendingLessons(): number {
  return lessons.filter((l) => l.state === 'pending').length
}

export function mockDecideLesson(id: string, input: LessonAction): { status: number; body: Lesson | { error: string } } {
  const lesson = lessons.find((l) => l.id === id)
  if (!lesson) return { status: 404, body: { error: 'Bài học không tồn tại' } }
  switch (input.action) {
    case 'keep':
    case 'drop': {
      if (lesson.state !== 'pending') return { status: 400, body: { error: 'Bài học không ở trạng thái chờ duyệt' } }
      if (input.scope && input.scope !== 'pack' && input.scope !== 'all') {
        return { status: 400, body: { error: 'Phạm vi không hợp lệ' } }
      }
      if (input.scope) lesson.scope = input.scope
      lesson.state = input.action === 'keep' ? 'kept' : 'dropped'
      lesson.decidedAt = new Date().toISOString()
      return { status: 200, body: lesson }
    }
    case 'undo': {
      if (lesson.state === 'pending') return { status: 400, body: { error: 'Bài học chưa được duyệt' } }
      lesson.state = 'pending'
      lesson.decidedAt = undefined
      return { status: 200, body: lesson }
    }
    case 'edit': {
      const text = (input.text ?? '').trim()
      if (lesson.state !== 'pending' || !text) return { status: 400, body: { error: 'Bài học không ở trạng thái chờ duyệt hoặc thiếu câu chữ' } }
      lesson.text = text
      return { status: 200, body: lesson }
    }
    default:
      return { status: 400, body: { error: 'Hành động không hợp lệ' } }
  }
}
