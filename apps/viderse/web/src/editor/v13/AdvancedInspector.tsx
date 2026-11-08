import { useEffect, useId, useRef, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import type { TimelineItem } from '@editor/types/timeline'
import { AudioSection, MixerSection } from '../ui/inspector/audio'
import { CaptionsSection } from '../ui/inspector/captions'
import { ColorSection } from '../ui/inspector/color'
import { EffectsSection } from '../ui/inspector/effects'
import { KeyframesSection } from '../ui/inspector/keyframes'
import { ObjectPropertiesSection } from '../ui/inspector/object-properties'
import { ShapeSection } from '../ui/inspector/shape'
import { ShapePathSection } from '../ui/inspector/shape-path'
import { TemplateAnchorSection } from '../ui/inspector/template-anchor'
import { TextSection } from '../ui/inspector/text'
import { TransformSection } from '../ui/inspector/transform'
import { WaveformSection } from '../ui/inspector/waveform'
import { EffectsTab } from '../ui/library/effects-tab'
import { isCaption } from './ContentPanels'
import { inspectorStyles as sx } from './Inspector.stylex'

export const ITEM_TYPE_LABELS: Record<TimelineItem['type'], string> = {
  video: 'Video',
  audio: 'Âm thanh',
  image: 'Cảnh',
  text: 'Chữ',
  subtitle: 'Phụ đề',
  shape: 'Hình dạng',
  lottie: 'Hoạt ảnh',
  adjustment: 'Lớp điều chỉnh',
  controller: 'Điều khiển',
  composition: 'Nhóm cảnh',
}

/** Presentation only: every section reads and edits the current timeline through existing commands. */
export function AdvancedInspector({ item }: { item: TimelineItem }) {
  const [open, setOpen] = useState(false)
  const [choosingEffect, setChoosingEffect] = useState(false)
  const [query, setQuery] = useState('')
  const chooserRef = useRef<HTMLInputElement>(null)
  const chooserId = useId()
  const text = item.type === 'text' || item.type === 'subtitle'
  const audio = item.type === 'audio' || item.type === 'video' || item.type === 'composition'
  const grading = item.type === 'video' || item.type === 'image' || item.type === 'lottie' || item.type === 'shape' || item.type === 'adjustment' || item.type === 'composition'
  useEffect(() => {
    if (open && choosingEffect) chooserRef.current?.focus()
  }, [open, choosingEffect])

  return <details {...stylex.props(sx.section)} open={open} onToggle={event => setOpen(event.currentTarget.open)}>
    <summary {...stylex.props(sx.subheading)}>Nâng cao</summary>
    <p {...stylex.props(sx.help)}>Vị trí, điểm mốc và các thuộc tính của {ITEM_TYPE_LABELS[item.type].toLocaleLowerCase('vi')} đang chọn.</p>
    {open && <div className="min-w-0 text-foreground">
      <ObjectPropertiesSection item={item} />
      {item.type === 'shape' && <><ShapeSection item={item} /><ShapePathSection item={item} /></>}
      {text && <TextSection item={item} styleOnly={isCaption(item)} />}
      {item.type === 'shape' && item.waveformVisualizer && <WaveformSection item={item} />}
      {item.type !== 'audio' && <TransformSection item={item} />}
      {text && <CaptionsSection item={item} />}
      <KeyframesSection item={item} />
      {grading && <ColorSection item={item} />}
      {audio && <AudioSection item={item} />}
      {item.type !== 'audio' && <>
        <EffectsSection item={item} onChooseEffect={() => setChoosingEffect(true)} />
        <details className="mt-3 border-t border-border pt-2" open={choosingEffect} onToggle={event => setChoosingEffect(event.currentTarget.open)}>
          <summary className="cursor-pointer text-xs font-semibold">Thư viện hiệu ứng</summary>
          {choosingEffect && <div className="space-y-3 pt-3">
            <label htmlFor={chooserId} className="block text-xs text-muted-foreground">Tìm hiệu ứng</label>
            <input ref={chooserRef} id={chooserId} type="search" {...stylex.props(sx.input)} value={query} onChange={event => setQuery(event.currentTarget.value)} placeholder="Tên hiệu ứng…" />
            <EffectsTab query={query} />
          </div>}
        </details>
      </>}
      <TemplateAnchorSection item={item} />
    </div>}
  </details>
}

export function ProjectAudioInspector() {
  const [open, setOpen] = useState(false)
  return <details {...stylex.props(sx.section)} open={open} onToggle={event => setOpen(event.currentTarget.open)}>
    <summary {...stylex.props(sx.subheading)}>Âm thanh toàn video</summary>
    <p {...stylex.props(sx.help)}>Âm lượng tổng và trộn âm của các track, áp dụng cho cả video.</p>
    {open && <div className="min-w-0 text-foreground"><MixerSection /></div>}
  </details>
}
