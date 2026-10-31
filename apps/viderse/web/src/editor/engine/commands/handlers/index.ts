import { registerProjectCommands } from './project'
import { registerTrackCommands } from './track'
import { registerCompositionCommands } from './composition'
import { registerMarkerCommands } from './marker'
import { registerItemCommands } from './item'
import { registerEditCommands } from './edit'
import { registerKeyframeCommands } from './keyframe'
import { registerTransitionCommands } from './transition'
import { registerEffectCommands } from './effect'
import { registerAudioCommands } from './audio'
import { registerTextCommands } from './text'
import { registerSubtitleCommands } from './subtitle'
import { registerMediaCommands } from './media'
import { registerAudioFxCommands } from './audio-fx'
import { registerCaptionCommands } from './caption'
import { registerTemplateCommands } from './template'
import { registerWaveformCommands } from './waveform'
export function registerAllCommands(): void {
  registerProjectCommands()
  registerTrackCommands()
  registerCompositionCommands()
  registerMarkerCommands()
  registerItemCommands()
  registerEditCommands()
  registerKeyframeCommands()
  registerTransitionCommands()
  registerEffectCommands()
  registerAudioCommands()
  registerTextCommands()
  registerSubtitleCommands()
  registerMediaCommands()
  registerAudioFxCommands()
  registerCaptionCommands()
  registerTemplateCommands()
  registerWaveformCommands()
}
