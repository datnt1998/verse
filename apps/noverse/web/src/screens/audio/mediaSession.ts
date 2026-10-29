// A7 · Gói AF — Media Session cho trình phát web (màn khoá iPhone, thông báo
// Android, trình duyệt máy tính, xe/Bluetooth). Một bộ thông tin giống thẻ tệp
// xuất: artist luôn "Noverse Audio", album "Tên truyện · Tập N", bìa truyện từ
// GET S/cover?size= — không bao giờ dùng trạng thái giọng hay chữ viết tắt.
import { useEffect, useRef } from 'react'
import { NOVERSE_ARTIST } from '../../lib/audioName'
import type { Player } from './useAudio'

const COVER_SIZES = [96, 128, 192, 256, 384, 512]

/** Bìa truyện cho Media Session / thẻ ảnh: `volume` để bìa tự tạo ghi đúng tập. */
export const storyCoverURL = (p: string, s: string, size: number, volume: number) =>
  `/api/projects/${encodeURIComponent(p)}/stories/${encodeURIComponent(s)}/cover?size=${size}&volume=${volume}`

export type MediaTrack = {
  project: string
  story: string
  volume: number
  /** "Chương N · tên" · bản thử thêm " (bản thử)" · tệp cả tập đổi theo mốc chương. */
  title: string
  /** "Tên truyện · Tập N". */
  album: string
  /** Thời lượng cho thanh tua màn khoá; 0 = chưa biết, ẩn vị trí. */
  duration: number
}

const ACTIONS = ['play', 'pause', 'seekbackward', 'seekforward', 'seekto', 'stop', 'previoustrack', 'nexttrack'] as const

/**
 * Đồng bộ trình phát với OS: metadata đổi theo `track` (đổi chương/mốc chương),
 * vị trí phát cập nhật mỗi khung đồng hồ, nút lùi/tới 10 giây và ⏮ ⏭ gọi
 * `onPrevious`/`onNext` (chương trước/sau, hoặc mốc chương của tệp cả tập).
 * `track = null` (chưa chọn/chưa phát được) gỡ metadata; unmount gỡ sạch.
 */
export function useMediaSession(track: MediaTrack | null, player: Player, actions: { onPrevious?: (() => void) | null; onNext?: (() => void) | null } = {}) {
  const latest = useRef(actions)
  latest.current = actions

  // Metadata + nút điều khiển.
  useEffect(() => {
    if (!('mediaSession' in navigator)) return
    if (!track || !track.title) {
      navigator.mediaSession.metadata = null
      return
    }
    if (typeof MediaMetadata === 'function') {
      navigator.mediaSession.metadata = new MediaMetadata({
        title: track.title,
        artist: NOVERSE_ARTIST,
        album: track.album,
        artwork: COVER_SIZES.map((size) => ({ src: storyCoverURL(track.project, track.story, size, track.volume), sizes: `${size}x${size}`, type: 'image/png' })),
      })
    }
    const handlers: Record<(typeof ACTIONS)[number], MediaSessionActionHandler | null> = {
      play: () => { void player.play().catch(() => {}) },
      pause: player.pause,
      seekbackward: (details) => player.skip(-(details?.seekOffset ?? 10)),
      seekforward: (details) => player.skip(details?.seekOffset ?? 10),
      seekto: (details) => { if (details?.seekTime !== undefined) player.seek(details.seekTime) },
      stop: () => { player.pause(); player.seek(0) },
      previoustrack: latest.current.onPrevious ? () => latest.current.onPrevious?.() : null,
      nexttrack: latest.current.onNext ? () => latest.current.onNext?.() : null,
    }
    for (const [action, handler] of Object.entries(handlers)) {
      try { navigator.mediaSession.setActionHandler(action as MediaSessionAction, handler) } catch { /* Action không bắt buộc. */ }
    }
    return () => {
      navigator.mediaSession.metadata = null
      for (const action of ACTIONS) {
        try { navigator.mediaSession.setActionHandler(action, null) } catch { /* Action không bắt buộc. */ }
      }
    }
  }, [track?.project, track?.story, track?.volume, track?.title, track?.album, !!track,
      player.play, player.pause, player.seek, player.skip,
      !!actions.onPrevious, !!actions.onNext])

  // Trạng thái + vị trí phát (thanh tua màn khoá) — cập nhật cả khi đổi tốc độ.
  useEffect(() => {
    if (!('mediaSession' in navigator)) return
    navigator.mediaSession.playbackState = player.playing ? 'playing' : 'paused'
    const duration = track?.duration ?? 0
    if (duration > 0 && 'setPositionState' in navigator.mediaSession) {
      navigator.mediaSession.setPositionState({ duration, playbackRate: player.rate, position: Math.min(duration, Math.max(0, player.time)) })
    }
  }, [player.playing, player.time, player.rate, track?.duration])

  // Gỡ trạng thái phát khi rời màn.
  useEffect(() => () => {
    if ('mediaSession' in navigator) navigator.mediaSession.playbackState = 'none'
  }, [])
}
