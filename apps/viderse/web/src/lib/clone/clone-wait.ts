import { progressCopy } from '../../i18n/screens'

export type CloneWaitReason = 'source_auth' | 'source' | 'storage' | 'transient' | 'image_quota'
type WaitError = { code: string; message: string; waitReason?: string }

/** Read old stored waits without displaying their raw diagnostics. Mirrors domain.CloneWaitReason. */
export function cloneWaitReason(step: string, error: WaitError | null | undefined): CloneWaitReason | null {
  if (error?.code !== 'quota') return null
  if (step === 'images') return 'image_quota'
  switch (error.waitReason) {
    case 'source_auth': case 'source': case 'storage': case 'transient': return error.waitReason
  }
  if (step === 'download') {
    const text = error.message.toLowerCase()
    if (text.includes('not a bot') || text.includes('sign in') || text.includes('cookie')) return 'source_auth'
    if (text.includes('không đủ chỗ') || text.includes('no space left') || text.includes('disk quota')) return 'storage'
    if (text.includes('tải nguồn ngừng tiến triển') || text.includes('tải nguồn vượt giới hạn thời gian') || text.includes('transcode nguồn bị timeout')) return 'transient'
    return 'source'
  }
  return step === 'backdrop' ? 'source' : 'transient'
}

export function cloneWaitText(reason: CloneWaitReason): string {
  return progressCopy.waitReasons[reason]
}
