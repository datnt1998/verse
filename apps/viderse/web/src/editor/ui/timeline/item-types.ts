import { AudioLines, Film, ImageIcon, Layers, PanelTop, Type } from 'lucide-react';
import type { TimelineItem } from '@editor/types/timeline';

export const TYPE_LABEL: Record<TimelineItem['type'], string> = {
  video: 'Video', audio: 'Âm thanh', image: 'Ảnh', text: 'Chữ', subtitle: 'Phụ đề', composition: 'Composition', shape: 'Hình', lottie: 'Lottie', adjustment: 'Hiệu ứng', controller: 'Điều khiển',
};
export const TYPE_COLOR: Record<TimelineItem['type'], string> = {
  video: 'video', audio: 'audio', text: 'text', subtitle: 'text', image: 'image', shape: 'image', lottie: 'image', composition: 'video', adjustment: 'effect', controller: 'effect',
};
export const TYPE_ICON = {
  video: Film, audio: AudioLines, text: Type, subtitle: Type, image: ImageIcon, shape: ImageIcon, lottie: ImageIcon, composition: Layers, adjustment: Layers, controller: PanelTop,
};
