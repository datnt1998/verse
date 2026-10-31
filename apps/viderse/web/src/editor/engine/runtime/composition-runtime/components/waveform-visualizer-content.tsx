import React, { useContext, useMemo } from 'react'
import type { ShapeItem } from '@editor/types/timeline'
import { getEnvelopeBins } from '@editor/features/waveform-visualizer/envelope'
import { waveformReadiness, waveformSourceSeconds, writeWaveformGeometry } from '@editor/features/waveform-visualizer/geometry'
import { VideoConfigContext } from '@editor/runtime/player/video-config'

export function WaveformVisualizerContent({ item, frame, fps, width, height }: { item: ShapeItem; frame: number; fps?: number; width: number; height: number }) {
  const config = useContext(VideoConfigContext)
  const timelineFps = fps ?? config?.fps
  const payload = item.waveformVisualizer!
  const geometry = useMemo(() => new Float32Array(payload.bars * 3), [payload.bars])
  if (!timelineFps) throw new Error('No waveform video config found')
  const bins = getEnvelopeBins(payload)
  const error = waveformReadiness(item)
  if (error || !bins) return <div role="status" className="flex h-full w-full items-center justify-center border border-dashed border-muted-foreground bg-card p-3 text-center text-xs text-muted-foreground">{error}</div>
  if (!payload.visible) return null
  writeWaveformGeometry(bins, payload, waveformSourceSeconds(item, frame, timelineFps), width, height, geometry)
  let path = ''
  for (let i = 0; i < payload.bars; i++) path += `M${geometry[i * 3]} ${geometry[i * 3 + 1]}L${geometry[i * 3]} ${geometry[i * 3 + 2]}`
  return <svg role="img" aria-label="Sóng giọng" data-waveform-frame={frame} width={width} height={height} viewBox={`0 0 ${width} ${height}`}><path d={path} fill="none" stroke={payload.color} strokeWidth={payload.strokeWidth * width / (item.transform?.width ?? width)} strokeLinecap="round" /></svg>
}
