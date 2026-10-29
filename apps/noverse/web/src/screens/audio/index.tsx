// Màn Audio (#/p/:p/s/:s/audio/:v/:mode?) — Transcript.dc.html (review) +
// AudioExport.dc.html (make: rendering · paused · renderError · rendered; listen; exported = listen?xem=da-xuat).
// `background` (rời màn khi đang tạo) do dock việc chạy nền ở AppShell đảm nhận (14-jobs-dock.md).
import { useRoute, type AudioMode } from '../../router'
import { TranscriptScreen } from './Transcript'
import { StudioScreen } from './Studio'
import { EXPORTED_VIEW, ExportedScreen, ListenScreen } from './Listen'

export function AudioScreen({ project, story, volume, mode }: {
  project: string; story: string; volume: number | null; mode: AudioMode
}) {
  const { search } = useRoute()
  const v = volume ?? 1
  if (mode === 'make') return <StudioScreen project={project} story={story} volume={v} />
  if (mode === 'listen' && search.get('xem') === EXPORTED_VIEW) return <ExportedScreen project={project} story={story} volume={v} />
  if (mode === 'listen') return <ListenScreen project={project} story={story} volume={v} />
  return <TranscriptScreen project={project} story={story} volume={v} />
}
