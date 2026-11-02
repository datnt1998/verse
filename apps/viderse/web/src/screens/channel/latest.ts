// "Làm video mới nhất" của một đối thủ: đọc video mới nhất thật rồi xếp hàng
// cho kênh. Dùng chung cho bước Xong (tạo kênh) và từng dòng Nguồn, để hai nơi
// không lệch hành vi (ưu tiên pick, mở đúng job vừa tạo).
import { enqueueClone } from '../../api/clone'
import { listRivalVideos } from '../../api/rivals'
import { t } from '../../i18n/channels'

export type LatestResult = { jobId: string | null; title: string }

export async function makeLatestRivalVideo(channelId: string, rivalId: string): Promise<LatestResult> {
  const page = await listRivalVideos(rivalId, { sort: 'newest', limit: 1 })
  const video = page.videos[0]
  if (!video) throw new Error(t.channels.latestEmpty)
  const result = await enqueueClone({ channelId, urls: [video.url], priority: 'pick' })
  return { jobId: result.items[0]?.jobId ?? null, title: video.title }
}
