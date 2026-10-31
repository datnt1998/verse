#!/usr/bin/env node
// Ported from yt-factory 2c201e6. Rebuild the committed public AudioWorklets
// after changing their sources; audioWorklet.addModule loads plain JavaScript.
import { build } from 'esbuild'
import { fileURLToPath } from 'node:url'
import { dirname, resolve } from 'node:path'
import { readFileSync } from 'node:fs'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')

// Keep SoundTouch's LGPL copyright and source/license pointers in the bundle.
const soundTouchSource = readFileSync(
  resolve(root, 'src/editor/engine/infrastructure/audio/time-stretch.ts'),
  'utf8',
)
const soundTouchNotice = soundTouchSource.match(/\/\*[\s\S]*?\*\//)?.[0]
if (!soundTouchNotice?.includes('GNU Lesser General Public')) {
  throw new Error('SoundTouch license header not found in time-stretch.ts')
}
const soundTouchBanner = `${soundTouchNotice}\n/* Full license text: src/editor/engine/infrastructure/audio/LICENSE-soundtouch. Corresponding source: src/editor/engine/infrastructure/audio/time-stretch.ts. */`

await build({
  entryPoints: [
    resolve(root, 'src/editor/engine/runtime/composition-runtime/worklets/soundtouch-preview-processor.worklet.ts'),
  ],
  alias: { '@editor': resolve(root, 'src/editor/engine') },
  outfile: resolve(root, 'public/editor-engine/soundtouch-preview-processor.worklet.js'),
  banner: { js: soundTouchBanner },
  bundle: true,
  format: 'iife',
  target: 'es2022',
  minify: false,
  logLevel: 'info',
})

await build({
  entryPoints: [
    resolve(root, 'src/editor/engine/runtime/composition-runtime/worklets/audio-fx-processor.worklet.ts'),
  ],
  alias: { '@editor': resolve(root, 'src/editor/engine') },
  outfile: resolve(root, 'public/editor-engine/audio-fx-processor.worklet.js'),
  banner: { js: `/*\n${readFileSync(resolve(root, 'src/editor/LICENSE-openreel'), 'utf8')}\n*/` },
  bundle: true,
  format: 'iife',
  target: 'es2022',
  minify: false,
  logLevel: 'info',
})
