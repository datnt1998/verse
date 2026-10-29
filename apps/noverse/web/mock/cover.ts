// §AF — bìa mock cho `GET S/cover?size=`: PNG vuông, deterministic theo kiểu bìa.
// 'up' = mô phỏng bìa đã tải lên (gradient truyện), 'gen' = bìa tự tạo (thương hiệu,
// có chấm Noverse ở chân — mô phỏng .cov.gen của AudioMeta.dc.html).
import { deflateSync } from 'zlib'

const crcTable = (() => {
  const table = new Uint32Array(256)
  for (let i = 0; i < 256; i++) {
    let c = i
    for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1
    table[i] = c >>> 0
  }
  return table
})()

const crc32 = (buf: Uint8Array) => {
  let c = 0xffffffff
  for (const byte of buf) c = crcTable[(c ^ byte) & 0xff] ^ (c >>> 8)
  return (c ^ 0xffffffff) >>> 0
}

const chunk = (type: string, data: Uint8Array) => {
  const name = Buffer.from(type, 'ascii')
  const head = Buffer.alloc(8 + data.length)
  head.writeUInt32BE(data.length, 0)
  name.copy(head, 4)
  head.set(data, 8)
  const crc = Buffer.alloc(4)
  crc.writeUInt32BE(crc32(new Uint8Array(head.subarray(4))), 0)
  return Buffer.concat([head, crc])
}

const mix = (a: [number, number, number], b: [number, number, number], t: number): [number, number, number] =>
  [a[0] + (b[0] - a[0]) * t, a[1] + (b[1] - a[1]) * t, a[2] + (b[2] - a[2]) * t]

const glow = (x: number, y: number, cx: number, cy: number, r: number) => Math.max(0, 1 - Math.hypot(x - cx, y - cy) / r)

/** PNG RGB: gradient chéo + hai đốm sáng theo đúng .cov.up / .cov.gen của artboard. */
export function mockCoverPng(size: number, kind: 'up' | 'gen'): Buffer {
  const n = Math.max(8, Math.min(1400, Math.round(size)))
  const [c1, c2] = kind === 'up' ? [[0x2b, 0x1a, 0x3a], [0x1c, 0x14, 0x26]] : [[0x24, 0x22, 0x4a], [0x2a, 0x18, 0x30]]
  const raw = Buffer.alloc(n * (1 + n * 3))
  for (let y = 0; y < n; y++) {
    const row = y * (1 + n * 3)
    raw[row] = 0
    for (let x = 0; x < n; x++) {
      const u = x / (n - 1), v = y / (n - 1)
      let px = mix(c1 as [number, number, number], c2 as [number, number, number], u * .4 + v * .6)
      if (kind === 'up') {
        px = mix(px, [0xf6, 0xcb, 0x8c], glow(u, v, .8, .1, .9) * .55)
        px = mix(px, [0xc9, 0x45, 0x7f], glow(u, v, .1, 1, .8) * .5)
      } else {
        px = mix(px, [0x78, 0x96, 0xf2], glow(u, v, 0, 0, .9) * .5)
        px = mix(px, [0xc9, 0x45, 0x7f], glow(u, v, 1, 1, .8) * .45)
      }
      // Chấm thương hiệu của bìa tự tạo (.cov .cb i) — góc trái dưới.
      if (kind === 'gen' && Math.hypot(u - .09, v - .91) < .045) {
        px = mix(px, [0xc9, 0xa6, 0xf5], .85)
      }
      const at = row + 1 + x * 3
      raw[at] = Math.round(px[0]); raw[at + 1] = Math.round(px[1]); raw[at + 2] = Math.round(px[2])
    }
  }
  const ihdr = Buffer.alloc(13)
  ihdr.writeUInt32BE(n, 0); ihdr.writeUInt32BE(n, 4)
  ihdr[8] = 8; ihdr[9] = 2 // 8-bit RGB
  return Buffer.concat([
    Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]),
    chunk('IHDR', ihdr),
    chunk('IDAT', deflateSync(raw)),
    chunk('IEND', new Uint8Array(0)),
  ])
}
