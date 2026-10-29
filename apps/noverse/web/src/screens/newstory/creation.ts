import { createStory, sendInput } from '../../api/client'
import type { StorySummary } from '../../api/types'

/** Keep the created story when starting fails, so retry never creates a second folder. */
export function quickStoryStarter() {
  let created: { key: string; story: StorySummary } | null = null
  const keyOf = (project: string, prompt: string, idea?: string) => JSON.stringify([project, prompt.trim(), idea ?? ''])
  return {
    /** Folder already created for this exact request, which the retry reuses. */
    reusable: (project: string, prompt: string, idea?: string) => created?.key === keyOf(project, prompt, idea) ? created.story.story : null,
    async start(project: string, prompt: string, idea?: string) {
      const text = prompt.trim()
      const key = keyOf(project, text, idea)
      if (created?.key !== key) {
        // Yêu cầu chỉ gửi vào /input — không phải tên truyện. Truyện khởi tạo không tên
        // (thư mục NN-truyen-moi, tempName) chờ engine đặt tên sách rồi backend đồng bộ.
        const story = await createStory(project, { idea })
        created = { key, story }
      }
      const story = created.story
      await sendInput(project, story.story, text)
      return story
    },
  }
}
