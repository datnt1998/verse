// D13 — dữ liệu Chất lượng truyện. Khoá nằm dưới `reviews` nên mọi `story.changed` có `reviews`
// (lần chấm mới, giữ/bỏ giữ) hoặc `chapters` (nộp chương → đổi trạng thái chương) đều làm mới.
import { getQuality } from '../../api/client'
import { keys, useResource } from '../../data'

export const qualityKey = (project: string, story: string) => keys.story(project, story, 'reviews', 'quality')

export const useQuality = (project: string, story: string, enabled = true) =>
  useResource(qualityKey(project, story), ({ signal }) => getQuality(project, story, signal), { enabled })
