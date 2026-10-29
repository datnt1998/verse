// Canh tệp tĩnh của PWA: sw.js nạp gì, MIME của biểu tượng, manifest và kích thước PNG thật.
// sw.js là tệp tĩnh chạy thẳng trên trình duyệt nên bài kiểm nạp mã nguồn vào một hộp cát `self` giả.
import { readFileSync, existsSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'

const root = new URL('../../', import.meta.url).pathname
const publicDir = join(root, 'public')
const read = (path: string) => readFileSync(join(root, path), 'utf8')

type SwApi = {
  APP_ASSETS: string[]
  STATIC_CACHE: string
  expectedTypes: (url: URL) => string[] | null
}

function loadServiceWorker(): SwApi {
  const source = read('public/sw.js')
  const self = {
    location: { origin: 'https://noverse.test' },
    addEventListener: () => {},
    skipWaiting: async () => {},
    clients: { claim: async () => {}, matchAll: async () => [] },
  }
  const factory = new Function('self', 'caches', `${source}\nreturn { APP_ASSETS, STATIC_CACHE, expectedTypes }`)
  return factory(self, {}) as SwApi
}

const sw = loadServiceWorker()
const types = (path: string) => sw.expectedTypes(new URL(path, 'https://noverse.test'))

describe('service worker công khai', () => {
  it('chỉ nạp bộ biểu tượng PNG hiện có', () => {
    const { APP_ASSETS } = sw
    expect(APP_ASSETS).toEqual([
      '/manifest.webmanifest', '/icons/icon-192.png', '/icons/icon-512.png',
      '/icons/icon-maskable-512.png', '/icons/apple-touch-icon.png', '/icons/favicon.svg',
    ])
    for (const asset of APP_ASSETS) expect(existsSync(join(publicDir, asset)), asset).toBe(true)
  })

  it('nhận MIME của bộ biểu tượng mới và bỏ SVG khai báo cũ', () => {
    expect(types('/icons/icon-192.png')).toEqual(['image/png'])
    expect(types('/icons/icon-512.png')).toEqual(['image/png'])
    expect(types('/icons/icon-maskable-512.png')).toEqual(['image/png'])
    expect(types('/icons/apple-touch-icon.png')).toEqual(['image/png'])
    expect(types('/icons/favicon-16.png')).toEqual(['image/png'])
    expect(types('/icons/noverse-og-image.png')).toEqual(['image/png'])
    expect(types('/icons/favicon.svg')).toEqual(['image/svg+xml'])
    expect(types('/icons/favicon.ico')).toEqual(['image/x-icon', 'image/vnd.microsoft.icon'])
    expect(types('/icons/icon-192.svg')).toBeNull()
  })

  it('dọn cache theo tiền tố tên kỹ thuật ainovel và đã tăng phiên bản', () => {
    expect(sw.STATIC_CACHE).toBe('ainovel-mobile-static-v3')
  })
})

/** Kích thước từ khối IHDR của PNG (byte 16..23). */
function pngSize(path: string): string {
  const buffer = readFileSync(join(publicDir, path))
  expect(buffer.subarray(1, 4).toString('latin1')).toBe('PNG')
  return `${buffer.readUInt32BE(16)}x${buffer.readUInt32BE(20)}`
}

describe('manifest và biểu tượng', () => {
  const manifest = JSON.parse(read('public/manifest.webmanifest')) as {
    name: string; short_name: string
    icons: { src: string; sizes: string; type: string; purpose: string }[]
  }

  it('mang tên Noverse và icon PNG any + maskable', () => {
    expect(manifest.name).toBe('Noverse · Xưởng viết truyện')
    expect(manifest.short_name).toBe('Noverse')
    expect(manifest.icons).toEqual([
      { src: '/icons/icon-192.png', sizes: '192x192', type: 'image/png', purpose: 'any' },
      { src: '/icons/icon-512.png', sizes: '512x512', type: 'image/png', purpose: 'any' },
      { src: '/icons/icon-maskable-512.png', sizes: '512x512', type: 'image/png', purpose: 'maskable' },
    ])
  })

  it('mỗi biểu tượng có mặt và đúng số điểm ảnh', () => {
    for (const icon of manifest.icons) expect(pngSize(icon.src), icon.src).toBe(icon.sizes)
    expect(pngSize('/icons/apple-touch-icon.png')).toBe('180x180')
    expect(pngSize('/icons/favicon-16.png')).toBe('16x16')
  })
})
