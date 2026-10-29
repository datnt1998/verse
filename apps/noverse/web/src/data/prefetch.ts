// Nạp trước dữ liệu của màn đích khi rê chuột / focus vào mục điều hướng (Gói Q › mục 6).
//
// Mỗi màn tự `useResource` với khoá riêng; ở đây chỉ gọi **đúng khoá + fetcher** đó vào bộ đệm
// chung (`cache.prefetch`) nên lần mở thật không phải chờ mạng. Khoá lệch nghĩa là nạp trước
// không có tác dụng (không sai gì), không phải lỗi dữ liệu.
import {
  getAudioCast, getAudioReady, getAudioSettings,
  ApiError, getChapters, getConfig, getOutline, getProject, getSnapshot, listStories,
} from '../api/client'
import { cache } from '@noverse/ui/data'
import { keys } from './keys'

export type PrefetchTarget = 'workspace' | 'project' | 'reader' | 'audio' | 'newstory'

/** Dữ liệu nạp trước còn dùng được trong 15 s (vượt xa khoảng rê chuột → bấm). */
const PREFETCH_STALE = 15000

const storyKeys = (p: string, s: string) => {
  if (!p || !s) return
  cache.prefetch(keys.story(p, s, 'snapshot'), ({ signal }) => getSnapshot(p, s, signal), { staleTime: PREFETCH_STALE })
  cache.prefetch(keys.story(p, s, 'chapters'), ({ signal }) => getChapters(p, s, signal), { staleTime: PREFETCH_STALE })
  cache.prefetch(keys.story(p, s, 'outline'), ({ signal }) => getOutline(p, s, signal).catch(error => { if (error instanceof ApiError && error.status === 404) return null; throw error }), { staleTime: PREFETCH_STALE })
}

/**
 * Nạp trước màn đích. Gọi được nhiều lần: `cache.prefetch` bỏ qua khoá đang bay hoặc còn mới.
 * Không có project/truyện (mục bị khoá) → không làm gì.
 */
export function prefetchScreen(target: PrefetchTarget, project: string | null, story: string | null) {
  if (!project) return
  switch (target) {
    case 'workspace':
      if (!story) return
      storyKeys(project, story)
      cache.prefetch(keys.project(project, 'stories'), () => listStories(project), { staleTime: PREFETCH_STALE })
      return
    case 'project':
      cache.prefetch(keys.project(project, 'stories'), () => listStories(project), { staleTime: PREFETCH_STALE })
      cache.prefetch(keys.project(project, 'meta'), () => getProject(project), { staleTime: PREFETCH_STALE })
      return
    case 'reader':
      if (!story) return
      cache.prefetch(keys.story(project, story, 'chapters'), ({ signal }) => getChapters(project, story, signal), { staleTime: PREFETCH_STALE })
      cache.prefetch(keys.story(project, story, 'outline'), ({ signal }) => getOutline(project, story, signal).catch(error => { if (error instanceof ApiError && error.status === 404) return null; throw error }), { staleTime: PREFETCH_STALE })
      return
    case 'audio':
      if (!story) return
      // KHÔNG nạp trước `audio/volume`: trên máy chủ thật đường đọc này có thể chờ máy đọc TTS nạp
      // mô hình (hàng chục giây) và giữ một trong 6 kết nối HTTP/1.1 — màn Audio thật sự sẽ thiếu
      // kết nối để tải. Ba khoá nhẹ dưới đây đủ làm màn mở nhanh.
      cache.prefetch(keys.story(project, story, 'audio', 'cast'), ({ signal }) => getAudioCast(project, story, signal), { staleTime: PREFETCH_STALE })
      cache.prefetch(keys.story(project, story, 'audio', 'settings'), ({ signal }) => getAudioSettings(project, story, signal), { staleTime: PREFETCH_STALE })
      cache.prefetch(keys.story(project, story, 'audio', 'ready'), ({ signal }) => getAudioReady(project, story, signal), { staleTime: PREFETCH_STALE })
      return
    case 'newstory':
      cache.prefetch(keys.config('project', project), ({ signal }) => getConfig('project', project, undefined, signal), { staleTime: PREFETCH_STALE })
      return
  }
}
