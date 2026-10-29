import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import stylex from '@stylexjs/unplugin'
import type { Plugin } from 'vite'
import { fileURLToPath } from 'node:url'

// Kit dùng chung nằm ở platform/ui (npm workspace ở gốc repo).
const uiPkg = (rel: string) => fileURLToPath(new URL(`../../../platform/ui/src/${rel}`, import.meta.url))

// StyleX biên dịch `stylex.create`/`defineVars` lúc build (cả vitest), gộp CSS vào tệp CSS chính.
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

// `npm run dev` → proxy /api sang `vidverse serve` (:8788).
export default defineConfig({
  resolve: {
    // Dev/test đọc thẳng source gói @noverse/ui (HMR giữ nguyên); consumer khác nhận dist qua npm pack.
    alias: [
      { find: /^@noverse\/ui\/tokens\.stylex$/, replacement: uiPkg('tokens.stylex.ts') },
      { find: /^@noverse\/ui\/(.+\.css)$/, replacement: uiPkg('$1') },
      { find: /^@noverse\/ui$/, replacement: uiPkg('index.ts') },
    ],
  },
  plugins: [stylexPlugin(), react(), tailwindcss()],
  server: {
    proxy: { '/api': 'http://127.0.0.1:8788' },
  },
  // SPA nhúng vào nhị phân Go (apps/vidverse/internal/web — //go:embed all:dist).
  build: { outDir: '../internal/web/dist', emptyOutDir: true },
})
