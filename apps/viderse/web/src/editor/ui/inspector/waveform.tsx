"use client";

import { useEffect, useRef, useState } from 'react'
import { RefreshCw, Eye, EyeOff } from 'lucide-react'
import { toast } from 'sonner'
import type { ShapeItem, TimelineItem } from '@editor/types/timeline'
import { executeCommands, type SerializedCommand } from '@editor/commands'
import { useMediaLibraryStore } from '@editor/features/media-library/stores/media-library-store'
import { waveformEnvelopeSchema, type WaveformStyle } from '@editor/features/waveform-visualizer/types'
import { waveformReadiness } from '@editor/features/waveform-visualizer/geometry'
import { ColorField, EditorIconButton, EditorSelect, PropertyRow, SectionHeader, SliderField } from '@/editor/ui/controls'
import { announce } from '@/editor/ui/shell/announcer'
import { WAVEFORM_SLIDERS, waveformStyleCommand } from './waveform-settings'

function run(command: SerializedCommand): void {
  const result = executeCommands([command])
  if (!result.ok) { const message = result.issues.map(issue => issue.message).join('; '); toast.error(message); announce(message, 'assertive') }
  else announce('Đã cập nhật Sóng giọng')
}
function WaveformEditor({ item }: { item: ShapeItem }) {
  const payload = item.waveformVisualizer!
  const records = useMediaLibraryStore(state => state.mediaItems)
  const [chosen, setChosen] = useState(item.mediaId ?? '')
  const [busy, setBusy] = useState(false)
  const abort = useRef<AbortController | null>(null)
  useEffect(() => () => abort.current?.abort(), [])
  const style = (style: Partial<WaveformStyle>) => run(waveformStyleCommand(item.id, style))
  const source = records.find(media => media.id === item.mediaId)
  const error = waveformReadiness(item, source?.contentHash)
  const regenerate = async () => {
    if (!chosen || busy) return
    const controller = new AbortController()
    abort.current = controller
    setBusy(true)
    try {
      const response = await fetch(`/api/editor/media/${encodeURIComponent(chosen)}/waveform`, { method: 'POST', signal: controller.signal })
      const result = await response.json()
      if (!response.ok) throw new Error(result.error ?? 'Không thể tạo sóng giọng')
      run({ type: 'waveform.setEnvelope', params: { itemId: item.id, mediaId: chosen, envelope: waveformEnvelopeSchema.parse(result.envelope) } })
    } catch (error) {
      if (!controller.signal.aborted) { const message = error instanceof Error ? error.message : 'Không thể tạo sóng giọng'; toast.error(message); announce(message, 'assertive') }
    } finally { if (!controller.signal.aborted) setBusy(false); abort.current = null }
  }
  return <div data-tour="editor-waveform-inspector"><SectionHeader title="Sóng giọng">
    <PropertyRow label="Nguồn giọng" action={<EditorIconButton icon={RefreshCw} label={busy ? 'Đang tạo sóng giọng' : 'Tạo lại sóng giọng'} disabled={busy || !chosen} onClick={() => void regenerate()} />}>
      <EditorSelect label="Nguồn giọng" value={chosen} disabled={busy} options={records.filter(media => media.mimeType.startsWith('audio/')).map(media => ({ value: media.id, label: media.fileName }))} onValueChange={setChosen} />
    </PropertyRow>
    {chosen !== item.mediaId && <p className="text-xs text-muted-foreground">Bấm tạo lại để áp dụng nguồn giọng đã chọn.</p>}
    {error && <p role="status" className="text-xs text-muted-foreground">{error}</p>}
    <PropertyRow label="Màu sóng"><ColorField label="Màu sóng giọng" value={payload.color} onChange={color => style({ color })} /></PropertyRow>
    {WAVEFORM_SLIDERS.map(setting => <div key={setting.key} className="space-y-1"><p className="text-xs text-muted-foreground">{setting.label}</p><SliderField label={setting.label} value={payload[setting.key]} min={setting.min} max={setting.max} step={setting.step} onChange={value => style({ [setting.key]: value })} /></div>)}
    <PropertyRow label="Hiển thị"><EditorIconButton icon={payload.visible ? Eye : EyeOff} label="Hiển thị sóng giọng" aria-pressed={payload.visible} onClick={() => style({ visible: !payload.visible })} /></PropertyRow>
  </SectionHeader></div>
}
export function WaveformSection({ item }: { item: TimelineItem }) {
  return item.type === 'shape' && item.waveformVisualizer ? <WaveformEditor key={item.id} item={item} /> : null
}
