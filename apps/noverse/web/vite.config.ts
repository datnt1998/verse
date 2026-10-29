import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import stylex from '@stylexjs/unplugin'
import { mockApiPlugin } from './mock/server'
import type { Plugin } from 'vite'
import { fileURLToPath } from 'node:url'

// Kit dùng chung nằm ở platform/ui (npm workspace ở gốc repo).
const uiPkg = (rel: string) => fileURLToPath(new URL(`../../../platform/ui/src/${rel}`, import.meta.url))

// StyleX biên dịch `stylex.create`/`defineVars` lúc build (cả vitest), gộp CSS vào tệp CSS chính.
// useCSSLayers: StyleX nằm trong @layer (trên preflight Tailwind) nên CSS cũ không layer vẫn đè được
// trong lúc chuyển dần — không dùng mẹo :not(#\#) vốn làm mọi rule ghi đè ở màn khác mất tác dụng.
// Vitest không có httpServer nên vòng hẹn giờ HMR trong configureServer không bao giờ dừng → bỏ hook đó khi test.
function stylexPlugin(): Plugin {
  const plugin: Plugin = stylex.vite({
    useCSSLayers: true,
    // StyleX's own resolver can't see vite aliases — map the package to source.
    aliases: {
      '@noverse/ui/tokens.stylex': [uiPkg('tokens.stylex.js')],
      '@noverse/ui/*': [uiPkg('*')],
    },
    unstable_moduleResolution: {
      type: 'commonJS',
      rootDir: fileURLToPath(new URL('.', import.meta.url)),
    },
  })
  if (process.env.VITEST) delete plugin.configureServer
  return plugin
}

// `npm run dev:mock` (--mode mock) → API mock trong bộ nhớ; `npm run dev` → proxy sang `ainovel-cli serve` (:8686).
export default defineConfig(({ mode }) => ({
  resolve: {
    // Dev/test đọc thẳng source gói @noverse/ui (HMR giữ nguyên); consumer khác nhận dist qua npm pack.
    alias: [
      { find: /^@noverse\/ui\/tokens\.stylex$/, replacement: uiPkg('tokens.stylex.ts') },
      { find: /^@noverse\/ui\/(.+\.css)$/, replacement: uiPkg('$1') },
      { find: /^@noverse\/ui$/, replacement: uiPkg('index.ts') },
    ],
  },
  plugins: [stylexPlugin(), react(), tailwindcss(), ...(mode === 'mock' ? [mockApiPlugin()] : [])],
  server: {
    proxy: mode === 'mock' ? undefined : { '/api': 'http://127.0.0.1:8686' },
  },
  // 产物嵌入 Go 二进制（internal/entry/web/embed）
  build: { outDir: '../internal/entry/web/dist', emptyOutDir: true },
}))
