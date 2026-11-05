// Resolve the actual newest source, then hand it to the shell-owned composer.
import { openQuickCreate } from '../new/quickCreate'
import { listRivalVideos } from '../../api/rivals'
import { t } from '../../i18n/channels'

export async function makeLatestRivalVideo(channelId: string, rivalId: string): Promise<void> {
  const page = await listRivalVideos(rivalId, { sort: 'newest', limit: 1 })
  const video = page.videos[0]
  if (!video) throw new Error(t.channels.latestEmpty)
  openQuickCreate({ channelId, urls: [video.url], fromSelection: true })
}
