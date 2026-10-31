import type { WaveformStyle } from '@/editor/engine/features/waveform-visualizer/types'

export const WAVEFORM_SLIDERS = [
  { key: 'bars', label: 'Số cột', min: 4, max: 256, step: 1 },
  { key: 'amplitude', label: 'Độ cao', min: 0, max: 2, step: 0.05 },
  { key: 'windowSeconds', label: 'Cửa sổ sóng', min: 0.1, max: 10, step: 0.1 },
  { key: 'strokeWidth', label: 'Độ dày nét', min: 0.5, max: 20, step: 0.5 },
  { key: 'baseline', label: 'Đường cơ sở', min: 0, max: 1, step: 0.05 },
] as const
export function waveformStyleCommand(itemId: string, style: Partial<WaveformStyle>) {
  return { type: 'waveform.setStyle', params: { itemId, style } }
}
