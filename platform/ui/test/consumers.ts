// App trong monorepo dùng kit: mọi apps/<tên>/web có dependency '@noverse/ui'.
// Test ranh giới và test tên biểu tượng của kit quét hết các app này (giống archtest
// quét import Go), nên app mới dùng kit được canh ngay mà không phải đăng ký thêm.
import { existsSync, readdirSync, readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

/** Thư mục gói: platform/ui. */
export const PKG = dirname(dirname(fileURLToPath(import.meta.url)))
export const PKG_SRC = join(PKG, 'src')
const REPO = dirname(dirname(PKG))

export type Consumer = { app: string; src: string }

export function consumers(): Consumer[] {
  const apps = join(REPO, 'apps')
  return readdirSync(apps).sort().flatMap((app) => {
    const manifest = join(apps, app, 'web', 'package.json')
    if (!existsSync(manifest)) return []
    const pkg = JSON.parse(readFileSync(manifest, 'utf8')) as { dependencies?: Record<string, string> }
    return pkg.dependencies?.['@noverse/ui'] ? [{ app, src: join(apps, app, 'web', 'src') }] : []
  })
}

const SOURCE = /\.tsx?$/

export function* sourceFiles(root: string): Generator<string> {
  if (!existsSync(root)) return
  for (const entry of readdirSync(root, { recursive: true, withFileTypes: true })) {
    if (entry.isFile() && SOURCE.test(entry.name)) yield join(entry.parentPath, entry.name)
  }
}
