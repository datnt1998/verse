import { createStory, putConfig, sendInput } from '../../api/client'
import type { Config, ModelRef, StorySummary } from '../../api/types'
import { amWritingCopy, t } from '../../i18n/vi'
import { writerConfig } from '../settings/sections/style'

export type QuickStoryChoices = { style: string | null; writer: ModelRef | null }

/** Lý do nút *Bắt đầu viết* bị khoá — máy tính và M11 dùng chung một luật. */
export type StartBlock = 'offline' | 'empty' | 'blocked'

export function startBlock({ offline, prompt, blocked }: { offline: boolean; prompt: string; blocked: boolean }): StartBlock | null {
  if (offline) return 'offline'
  if (!prompt.trim()) return 'empty'
  return blocked ? 'blocked' : null
}

/** Câu hiện ngay trên nút. Bản điện thoại dùng câu ngắn của 07-copy.md › Gói AR. */
export function startBlockedReason(block: StartBlock | null, variant: 'desktop' | 'phone' = 'desktop'): string | null {
  if (block === 'offline') return 'Mất kết nối — chờ kết nối lại để bắt đầu.'
  if (block === 'empty') return 'Nhập ý tưởng hoặc yêu cầu sáng tác để bắt đầu.'
  if (block === 'blocked') return variant === 'phone' ? t.newStory.blockedBar : t.newStoryCheck.blockedHint
  return null
}

/** Tên truyện tạm khi bắt đầu từ tệp: bỏ thư mục và đuôi tệp (Kiến trúc sư đặt lại tên sau). */
export function storyNameFromFile(path: string): string {
  return path.split('/').pop()?.replace(/\.[^.]+$/, '') ?? ''
}

/** Only explicit choices that differ from the project's current defaults become story config. */
export function quickStoryConfig(choices: QuickStoryChoices, project?: Config): Config {
  let config: Config = {}
  if (choices.style && choices.style !== project?.style) config.style = choices.style
  const provider = project?.roles?.writer?.provider || project?.roles?.default?.provider || project?.provider
  const model = project?.roles?.writer?.model || project?.roles?.default?.model || project?.model
  if (choices.writer && (choices.writer.provider !== provider || choices.writer.model !== model)) {
    config = writerConfig(config, choices.writer)
  }
  return config
}

/** Keep the created story when starting fails, so retry never creates a second folder. */
export function quickStoryStarter() {
  let created: { key: string; story: StorySummary; configKey: string | null } | null = null
  const keyOf = (project: string, prompt: string, idea?: string) => JSON.stringify([project, prompt.trim(), idea ?? ''])
  return {
    /** Folder already created for this exact request, which the retry reuses. */
    reusable: (project: string, prompt: string, idea?: string) => created?.key === keyOf(project, prompt, idea) ? created.story.story : null,
    async start(project: string, prompt: string, idea?: string, config: Config = {}) {
      const text = prompt.trim()
      const key = keyOf(project, text, idea)
      if (created?.key !== key) {
        // Yêu cầu chỉ gửi vào /input — không phải tên truyện. Truyện khởi tạo không tên
        // (thư mục NN-truyen-moi, tempName) chờ engine đặt tên sách rồi backend đồng bộ.
        const story = await createStory(project, { idea })
        created = { key, story, configKey: '{}' }
      }
      const story = created.story
      const configKey = JSON.stringify(config)
      if (created.configKey !== configKey) {
        // A lost response can still mean the server saved it; the next choice must be written again.
        created.configKey = null
        try {
          await putConfig('story', config, project, story.story)
          created.configKey = configKey
        } catch (cause) {
          throw new Error(amWritingCopy.configSaveFailed, { cause })
        }
      }
      await sendInput(project, story.story, text)
      return story
    },
  }
}
