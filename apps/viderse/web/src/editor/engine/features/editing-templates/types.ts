import type { CaptionWord } from '@editor/types/timeline'
import type { SerializedCommand } from '@editor/commands/types'

export interface EditingTemplateSection { title: string; from: number; durationInFrames: number; image?: { mediaId: string; src: string }; lowerThird?: string }
export interface EditingTemplateContext {
  idPrefix: string
  fps: number
  width: number
  height: number
  durationInFrames: number
  from?: number
  title?: string
  narration?: { mediaId: string; src: string }
  sections: EditingTemplateSection[]
  words?: CaptionWord[]
  wordsEstimated?: boolean
}
export interface EditingTemplate {
  id: string
  name: string
  description: string
  category: 'narration' | 'listicle' | 'documentary'
  resolve(context: EditingTemplateContext): SerializedCommand[]
}
