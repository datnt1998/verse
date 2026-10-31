import assert from 'node:assert/strict'
import { describe, it } from 'node:test'
import { EXPORT_PRESETS, EXPORT_QUALITY_OPTIONS, exportSettings, exportTechnicalSummary, formatBytes, formatDuration, preflightMessage } from './export-settings'

describe('export settings', () => {
  it('keeps every preset at 30 fps and disables smart-copy for render exports', () => {
    for (const preset of EXPORT_PRESETS) {
      const settings = exportSettings(preset.id, 'high', true)
      assert.deepEqual(settings.resolution, { width: preset.width, height: preset.height })
      assert.equal(settings.fps, 30)
      assert.equal(settings.smartCopy, false)
      assert.equal(settings.videoContainer, 'mp4')
    }
    assert.equal(exportSettings('shorts', 'medium', false).renderWholeProject, false)
  })
  it('formats byte sizes and duration with Vietnamese compact values', () => {
    assert.equal(formatBytes(1024), '1 KB')
    assert.equal(formatBytes(1024 ** 2), '1 MB')
    assert.equal(formatBytes(1024 ** 3), '1 GB')
    assert.equal(formatDuration(61.2), '01:02')
    assert.equal(formatDuration(-1), '00:00')
  })
  it('explains slow-render and missing-media problems in plain words rather than exposing translation keys', () => {
    assert.match(preflightMessage({ id: 'worker-animated-image-fallback', severity: 'warning', titleKey: '', detailKey: '' }), /chậm hơn/)
    assert.match(preflightMessage({ id: 'missing-media-blocks-export', severity: 'error', titleKey: '', detailKey: '', detailParams: { count: 2 } }), /Thiếu 2 file/)
  })
  it('keeps engineering terms out of every visible preflight message, preset and quality label', () => {
    const technical = /codec|worker|h\.?264|aac|fps|luồng chính|\bmp4\b/i
    const ids = ['empty-range', 'export-range-ready', 'missing-media-blocks-export', 'waveform-envelope-blocks-export', 'video-codec-unavailable', 'video-codec-fallback', 'video-codec-supported', 'audio-codec-unavailable', 'audio-codec-supported', 'worker-unavailable-fallback', 'worker-animated-image-fallback', 'worker-export-ready', 'large-file-risk', 'long-export-risk', 'unknown']
    for (const id of ids) assert.doesNotMatch(preflightMessage({ id, severity: 'warning', titleKey: '', detailKey: '', detailParams: { codec: 'avc', container: 'mp4', count: 1 } } as never), technical, id)
    for (const preset of EXPORT_PRESETS) assert.doesNotMatch(preset.label, technical, preset.id)
    for (const option of EXPORT_QUALITY_OPTIONS) assert.doesNotMatch(option.label, technical, option.value)
    assert.match(exportTechnicalSummary({ resolution: { width: 1920, height: 1080 }, fps: 30 }), /1920×1080/)
  })
})
