import type { Brain, BrainEntity, BrainEntityType } from '../src/api/types'

/** Live-sized regression fixture; the normal artboard story stays unchanged. */
export const denseBrainFixture = (): Brain => {
  const groups: [BrainEntityType, number, string][] = [
    ['char', 15, 'Nhân vật'], ['ev', 60, 'Sự kiện'], ['obj', 30, 'Vật phẩm'], ['place', 20, 'Địa điểm'],
  ]
  const entities = groups.flatMap(([type, count, label]) => Array.from({ length: count }, (_, index): BrainEntity => ({
    id: `${type}-${index}`, type, name: type === 'char' && index < 2 ? ['Lâm An', 'Lâm Khải'][index] : `${label} ${index + 1} với tên rất dài`,
    role: type === 'char' ? 'Nhân vật trong câu chuyện' : '', desc: '', fromChapter: 1,
    appears: [1], aliases: [], timeline: [],
  })))
  return {
    chapters: [{ n: 1, title: 'Khởi đầu', status: 'written' }], entities,
    edges: Array.from({ length: 150 }, (_, index) => ({
      a: entities[0].id, b: entities[1 + index % (entities.length - 1)].id,
      phases: [{ fromChapter: 1, type: 'rel', label: 'Liên quan' }],
    })),
  }
}
