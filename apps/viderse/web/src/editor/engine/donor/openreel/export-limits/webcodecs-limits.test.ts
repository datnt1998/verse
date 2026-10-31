import { describe, expect, it } from 'vitest'
import { checkBrowserExportCapability, getWebCodecsExportWarnings, resolveWebCodecsExportLimits } from './webcodecs-limits'

describe('browser export recommendations', () => {
  it('preserves portrait orientation and recommends HD for long video without modifying input', () => {
    const settings = { codec: 'h264', width: 2160, height: 3840, frameRate: 60 }
    expect(resolveWebCodecsExportLimits(settings, 600).settings).toEqual({ codec: 'h264', width: 1080, height: 1920, frameRate: 30 })
    expect(getWebCodecsExportWarnings(settings, 600)).toHaveLength(2)
    expect(settings.width).toBe(2160)
  })
  it('leaves YouTube 1080p30 and short 4K H264 unchanged', () => {
    expect(getWebCodecsExportWarnings({ codec: 'h264', width: 1920, height: 1080, frameRate: 30 }, 900)).toEqual([])
    expect(resolveWebCodecsExportLimits({ codec: 'h264', width: 3840, height: 2160, frameRate: 60 }, 30).wasAdjusted).toBe(false)
  })
  it('interprets supplied capability probes without touching browser globals', () => {
    const settings = { codec: 'h264', width: 1920, height: 1080, frameRate: 30 }
    expect(checkBrowserExportCapability(settings, { supportedContainerCodecs: ['h264'], encodableVideoCodecs: ['h264'], audioSupported: true }).supported).toBe(true)
    expect(checkBrowserExportCapability(settings, { supportedContainerCodecs: ['vp9'], encodableVideoCodecs: ['h264'], audioSupported: true }).supported).toBe(false)
  })
})
