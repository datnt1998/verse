/**
 * Whether every pixel of a decoded image is fully opaque. Occlusion culling may
 * only skip the layers beneath an image that paints every pixel: a cut-out PNG
 * stretched over the canvas still shows the clip under it. Decided once per load
 * from the full-resolution alpha channel. When the pixels cannot be read (no
 * OffscreenCanvas, tainted source) the image counts as transparent, which only
 * costs drawing the layers below.
 */
export function isFullyOpaqueImage(source: CanvasImageSource, width: number, height: number): boolean {
  if (typeof OffscreenCanvas === 'undefined' || width < 1 || height < 1) return false
  const context = new OffscreenCanvas(width, height).getContext('2d', { willReadFrequently: true })
  if (!context) return false
  try {
    context.drawImage(source, 0, 0)
    const { data } = context.getImageData(0, 0, width, height)
    for (let index = 3; index < data.length; index += 4) {
      if (data[index] !== 255) return false
    }
    return true
  } catch {
    return false
  }
}
