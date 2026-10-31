import { afterEach, describe, expect, it, vi } from 'vitest'
import { getExternalImageSourceDimensions } from './effects-pipeline'
import { lut3d } from './effects/lut'
import { encodeLutData, parseCubeLut } from './lut/cube-lut'

afterEach(() => vi.unstubAllGlobals())

describe('GPU effect source dimensions', () => {
  it('reads worker canvases without referencing unavailable DOM constructors', () => {
    vi.stubGlobal('HTMLVideoElement', undefined)
    vi.stubGlobal('HTMLImageElement', undefined)
    vi.stubGlobal('VideoFrame', undefined)
    const source = { width: 1920, height: 1080 } as OffscreenCanvas
    expect(getExternalImageSourceDimensions(source)).toEqual({ width: 1920, height: 1080 })
  })

  it('uses display dimensions for a decoded worker video frame', () => {
    class WorkerFrame { displayWidth = 1280; displayHeight = 720 }
    vi.stubGlobal('HTMLVideoElement', undefined)
    vi.stubGlobal('HTMLImageElement', undefined)
    vi.stubGlobal('VideoFrame', WorkerFrame)
    expect(getExternalImageSourceDimensions(new WorkerFrame() as VideoFrame)).toEqual({ width: 1280, height: 720 })
  })

  it('keeps intrinsic dimensions for DOM media in preview', () => {
    class Video { videoWidth = 640; videoHeight = 360; width = 100; height = 100 }
    class Image { naturalWidth = 320; naturalHeight = 180; width = 100; height = 100 }
    vi.stubGlobal('HTMLVideoElement', Video)
    vi.stubGlobal('HTMLImageElement', Image)
    expect(getExternalImageSourceDimensions(new Video() as HTMLVideoElement)).toEqual({ width: 640, height: 360 })
    expect(getExternalImageSourceDimensions(new Image() as HTMLImageElement)).toEqual({ width: 320, height: 180 })
  })
})

describe('embedded warm LUT upload', () => {
  it('maps imported red-fastest cube data and size into texture payload and uniforms', () => {
    const lut = parseCubeLut('LUT_3D_SIZE 2\n0.1 0 0\n1 0 0\n0.1 0.95 0\n1 0.95 0\n0.1 0 0.65\n1 0 0.65\n0.1 0.95 0.65\n1 0.95 0.65')
    const params = { intensity: 1, lutSize: String(lut.size), lutData: encodeLutData(lut.data) }
    const texture = lut3d.dataTexture!.build(params)
    expect([texture.width, texture.height, texture.depth]).toEqual([2, 2, 2])
    expect(Array.from(texture.data.slice(24, 28))).toEqual([26, 242, 166, 255])
    expect(Array.from(lut3d.packUniforms(params, 1920, 1080)!)).toEqual([1, 2, 0, 0])
  })
})
