import { expect, it } from 'vitest'
import { fatigueRows } from './fatigue'
import type { RuleEntry } from './logic'
const entry = (word: string, limit: number): RuleEntry => ({ key: word, text: '', chips: [{ kind: 'max', text: word, limit }], uncertain: false, note: '', options: [], status: 'ready' })

it('nới ngưỡng bằng số đã dùng xoá huy hiệu vượt ngay, không chờ chấm lại', () => {
  const chapters = [{ chapter: 3, at: '', violations: [{ rule: 'fatigue_words', target: 'dường như', actual: 2, limit: 1, severity: 'warning' as const }] }]
  const before = fatigueRows('story', undefined, [], [entry('dường như', 1)], [], chapters).find((row) => row.word === 'dường như')!
  expect(before).toMatchObject({ source: 'learned', limit: 1, over: 3 })
  const after = fatigueRows('story', undefined, [entry('dường như', 2)], [], [], chapters).find((row) => row.word === 'dường như')!
  expect(after.over).toBeUndefined()
})

it('quy tắc truyện đè dự án, đặt lại quay về mức dự án chứ không mặc định', () => {
  const project = [entry('như một', 5)]
  const own = fatigueRows('story', undefined, [entry('như một', 6)], [], project).find((row) => row.word === 'như một')!
  expect(own).toMatchObject({ limit: 6, baseline: 5, changed: true, project: true })
  const reset = fatigueRows('story', undefined, [], [], project).find((row) => row.word === 'như một')!
  expect(reset).toMatchObject({ limit: 5, baseline: 5, source: 'project', changed: false })
})

it('đọc ngưỡng gói AP2 theo chương, không quy đổi lần thứ hai', () => {
  const rows = fatigueRows('story', { snapshot: null, items: [], fatigueWords: [{ word: 'như một', per1000: 4, perChapter: 6, source: 'pack' }] }, [], [], [])
  expect(rows.find((row) => row.word === 'như một')).toMatchObject({ limit: 6, source: 'pack' })
})

it('đặt lại một luật đã lưu trở về ngưỡng gói bên dưới, kể cả ngưỡng nền bằng 0', () => {
  const own = entry('như một', 7)
  const data = { snapshot: null, items: [{ text: own.text, chips: own.chips, scope: 'project' as const, uncertain: false, uncertainNote: '', status: 'ready' as const }], fatigueWords: [{ word: 'như một', per1000: 4.67, perChapter: 7, source: 'project' as const, basePerChapter: 0, baseSource: 'pack' as const }] }
  expect(fatigueRows('project', data, [own], [], []).find((row) => row.word === 'như một')).toMatchObject({ limit: 7, baseline: 0 })
  expect(fatigueRows('project', data, [], [], []).find((row) => row.word === 'như một')).toMatchObject({ limit: 0, source: 'pack', changed: false })
})

it('snapshot cũ không chứng minh được nguồn vẫn giữ đúng số, không tự gán nhãn', () => {
  const data = { snapshot: null, items: [], fatigueWords: [{ word: 'như một', per1000: 4.67, perChapter: 7, basePerChapter: 3 }] }
  expect(fatigueRows('story', data, [], [], []).find((row) => row.word === 'như một')).toMatchObject({ limit: 7, source: undefined })
})
