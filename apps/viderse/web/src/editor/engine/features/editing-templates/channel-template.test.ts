// @vitest-environment node
import { describe, expect, it } from 'vitest'
import { makeTimelineAudioItem, makeTimelineTrack, makeTimelineVideoItem } from '@editor/features/timeline/test-helpers'
import type { TemplateBackdrop } from '@editor/types/timeline'
import type { TemplateProjectShape } from './channel-template'
import { suggestTemplateRoleCommands, validateChannelTemplate } from './channel-template'

function template(): TemplateProjectShape {
  return {
    metadata: { width: 1920, height: 1080, fps: 30 },
    tracks: [makeTimelineTrack({ id: 'scenes', name: 'Hình ảnh cảnh', order: 0, kind: 'video', templateSlot: 'scenes' }), makeTimelineTrack({ id: 'captions', name: 'Phụ đề', order: 1, kind: 'video', templateSlot: 'captions' }), makeTimelineTrack({ id: 'audio', name: 'Âm thanh gốc', order: 2, kind: 'audio', templateSlot: 'source-audio' })],
    items: [{ id: 'image', type: 'image', trackId: 'scenes', from: 0, durationInFrames: 90, label: 'Mẫu', src: '/image', mediaId: 'image-media' }, makeTimelineAudioItem({ id: 'audio-item', trackId: 'audio', mediaId: 'audio-media' })],
  }
}
const mediaIds = new Set(['image-media', 'audio-media', 'video-media'])

describe('channel templates', () => {
  it('accepts a valid template, with captions optional', () => {
    const project = template()
    expect(validateChannelTemplate(project, mediaIds)).toEqual([])
    expect(validateChannelTemplate({ ...project, tracks: project.tracks.filter(track => track.templateSlot !== 'captions') }, mediaIds)).toEqual([])
  })
  it('reports resolution, missing slots, duplicate slots and missing media', () => {
    const project = template()
    expect(validateChannelTemplate({ ...project, metadata: { width: 1280, height: 720, fps: 24 }, tracks: [] }, mediaIds).filter(issue => issue.severity === 'error')).toHaveLength(3)
    const duplicate = makeTimelineTrack({ id: 'duplicate', name: 'Trùng', kind: 'video', order: 3, templateSlot: 'scenes' })
    expect(validateChannelTemplate({ ...project, tracks: [...project.tracks, duplicate] }, mediaIds).some(issue => issue.trackId === 'duplicate')).toBe(true)
    expect(validateChannelTemplate(project, new Set()).filter(issue => issue.itemId)).toHaveLength(2)
  })
  it('allows an empty scenes prototype and reports its full-frame default', () => {
    const project = template()
    const issues = validateChannelTemplate({ ...project, items: project.items.filter(item => item.type !== 'image') }, mediaIds)
    expect(issues.map(issue => issue.severity)).toEqual(['info'])
  })
  it('warns about an empty background track unless backdrop channels feed it', () => {
    const project = template()
    const withBackground = (templateBackdrop?: TemplateBackdrop) => ({ ...project, tracks: [...project.tracks, makeTimelineTrack({ id: 'bg', name: 'Nền video', order: 3, kind: 'video', templateSlot: 'background-video', templateBackdrop })] })
    const emptyIssues = validateChannelTemplate(withBackground(), mediaIds)
    expect(emptyIssues.map(issue => issue.trackId)).toEqual(['bg'])
    expect(emptyIssues[0]?.severity).toBe('info')
    const backdrop: TemplateBackdrop = { channels: [{ channelId: 'twitch:xqc', kinds: ['clip'], pinned: [] }], clipCount: 10, clipSec: 60 }
    expect(validateChannelTemplate(withBackground(backdrop), mediaIds)).toEqual([])
  })
  it('refuses full videos that cannot loop, including linked audio', () => {
    const project = template()
    const video = makeTimelineVideoItem({ id: 'overlay', trackId: 'captions', mediaId: 'video-media', templateAnchor: 'full', embeddedAudioMuted: true, sourceStart: 0, sourceEnd: 60, linkedGroupId: 'linked' })
    const audio = makeTimelineAudioItem({ id: 'linked', trackId: 'audio', linkedGroupId: 'linked' })
    expect(validateChannelTemplate({ ...project, items: [...project.items, video, audio] }, mediaIds).some(issue => issue.itemId === 'overlay')).toBe(true)
    expect(validateChannelTemplate({ ...project, items: [...project.items, { ...video, linkedGroupId: undefined }] }, mediaIds)).toEqual([])
  })
  it('suggests all three legacy roles in one batch, never overrides existing roles', () => {
    const tracks = template().tracks.map(track => ({ ...track, templateSlot: undefined }))
    const commands = suggestTemplateRoleCommands(tracks)
    expect(commands.map(command => command.params)).toEqual([{ id: 'scenes', updates: { templateSlot: 'scenes' } }, { id: 'captions', updates: { templateSlot: 'captions' } }, { id: 'audio', updates: { templateSlot: 'source-audio' } }])
    expect(suggestTemplateRoleCommands(template().tracks)).toEqual([])
    expect(suggestTemplateRoleCommands(tracks.slice(1))).toEqual([])
  })
})
