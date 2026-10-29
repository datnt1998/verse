// Pack voice defaults apply only until the story has an explicit voice choice.
// The cast resource is shared by desktop and mobile, including mode overrides.
import { useMemo } from 'react'
import { getAudioCast, getConfig, getConfigPresets } from '../../api/client'
import type { AudioCast, PackPreset } from '../../api/types'
import { keys, useResource } from '../../data'
import { packLabel } from '../settings/lib/packs'

/** Gói một giọng kể: tên gói để ghép câu + giới tính giọng gói đề xuất ('' = gói không khai). */
export type PackVoice = { ten: string; gender: 'nu' | 'nam' | '' }

/** Gói một giọng kể của truyện; null khi truyện dùng kiểu viết thường hoặc gói chia vai.
 *  Chỉ đọc `packs[].audio` (goi.json › `audio.che_do`/`audio.giong`); câu `packs[].voice` chỉ để hiển thị. */
export function packVoiceOf(packs: PackPreset[] | undefined, style: string | undefined, voiceMode?: AudioCast['voiceMode']): PackVoice | null {
  if (!style || voiceMode) return null
  const pack = (packs ?? []).find((item) => item.id === style)
  if (pack?.audio?.che_do !== 'single') return null
  const gender = pack.audio.giong
  return { ten: packLabel(pack), gender: gender === 'nu' || gender === 'nam' ? gender : '' }
}

/** Gói của truyện đang mở (dùng chung cache với Tuỳ chỉnh nên không gọi lại khi đã có). */
export function usePackVoice(project: string, story: string): PackVoice | null {
  const config = useResource(keys.config('story', project, story), ({ signal }) => getConfig('story', project, story, signal))
  const presets = useResource(['config-presets'], ({ signal }) => getConfigPresets(signal), { staleTime: 60000 })
  const cast = useResource(keys.story(project, story, 'audio', 'cast'), ({ signal }) => getAudioCast(project, story, signal))
  return useMemo(() => packVoiceOf(presets.data?.packs, config.data?.effective?.style, cast.data?.voiceMode), [presets.data, config.data, cast.data])
}
