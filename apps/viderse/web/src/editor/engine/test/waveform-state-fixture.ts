import { readFileSync } from 'node:fs'
import type { Project } from '@editor/types/project'
import type { ProjectState } from '@editor/commands'
import { VOICE_HASH } from './waveform-fixture'

export function waveformStateFixture(): ProjectState {
  const project = JSON.parse(readFileSync(new URL('../commands/fixtures/project.json', import.meta.url), 'utf8')) as Project
  return {
    items: [], tracks: [{ id: 'visual', name: 'V1', items: [], height: 60, order: 0, visible: true, muted: false, solo: false, locked: false }], transitions: [], keyframes: [], markers: [], compositions: [], topLevelSequenceIds: [], inPoint: null, outPoint: null, fps: 30, scrollPosition: 0, snapEnabled: true, currentFrame: 0, masterBusDb: 0, projectId: project.id, projectName: project.name, projectMetadata: project.metadata, stashStack: [], mainHolder: null,
    mediaItems: [{ id: 'voice-audio', storageType: 'workspace', contentHash: VOICE_HASH, fileName: 'voice.wav', fileSize: 100, mimeType: 'audio/wav', duration: 4, width: 0, height: 0, fps: 30, codec: 'pcm', bitrate: 0, tags: [], createdAt: 1, updatedAt: 1 }],
  }
}
