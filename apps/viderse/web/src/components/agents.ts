// Bốn trợ lý Viderse → orb của kit (design 02 §3): tên hiển thị luôn là tên
// Viderse, chỉ màu/orb dùng chung bốn agent của kit Noverse.
//   Trinh sát → architect (xanh) · Đạo diễn → writer (tím)
//   Hoạ sĩ → editor (vàng cam) · Kiểm định → arbiter (mint)
import type { AgentKind } from '@noverse/ui'
import { t } from '../i18n/vi'

export type Assistant = 'scout' | 'director' | 'artist' | 'qa'

export const ASSISTANT_ORB: Record<Assistant, AgentKind> = {
  scout: 'architect',
  director: 'writer',
  artist: 'editor',
  qa: 'arbiter',
}

/** Token màu chữ riêng của từng trợ lý (03-tokens › Trợ lý). */
export const ASSISTANT_INK: Record<Assistant, string> = {
  scout: 'var(--vv-scout-ink)',
  director: 'var(--vv-director-ink)',
  artist: 'var(--vv-artist-ink)',
  qa: 'var(--vv-qa-ink)',
}

export const assistantName = (assistant: Assistant) => t.agents[assistant].name

export const ASSISTANTS = Object.keys(ASSISTANT_ORB) as Assistant[]
