// Viderse web — dev/test đọc thẳng source gói @noverse/ui (kit dùng chung
// platform/ui): StyleX biên dịch stylex.create/defineVars lúc build, gộp vào
// CSS chính. Mỗi màn hình lazy vào chunk riêng; scripts/size.mjs đo JS tải
// lần đầu từ manifest.
import { fileURLToPath } from 'node:url'
import { defineConfig, type Plugin } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

import stylex from '@stylexjs/unplugin'

// Kit dùng chung nằm ở platform/ui (npm workspace ở gốc repo).
const uiPkg = (rel: string) => fileURLToPath(new URL(`../../../platform/ui/src/${rel}`, import.meta.url))

// StyleX's own resolver can't see vite aliases — map the package to source.
// Vitest không có httpServer nên bỏ configureServer để vòng hẹn giờ HMR không kẹt.
function stylexPlugin(): Plugin {
  const plugin: Plugin = stylex.vite({
    useCSSLayers: true,
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

export default defineConfig({
  resolve: {
    alias: [
      { find: /^@noverse\/ui\/tokens\.stylex$/, replacement: uiPkg('tokens.stylex.ts') },
      { find: /^@noverse\/ui\/(.+\.css)$/, replacement: uiPkg('$1') },
      { find: /^@noverse\/ui\/data$/, replacement: uiPkg('data/index.ts') },
      { find: /^@noverse\/ui$/, replacement: uiPkg('index.ts') },
    ],
  },
  // Tailwind chỉ dùng preflight + @source (styles/base.css); StyleX nằm trên nó.
  plugins: [stylexPlugin(), react(), tailwindcss()],
  server: {
    port: 5181,
    proxy: {
      '/api': { target: process.env.VITE_API_TARGET ?? 'http://127.0.0.1:8789', changeOrigin: true },
    },
  },
  build: {
    // entry/web phục vụ dist như app nhúng (xem internal/web).
    outDir: '../internal/web/dist',
    emptyOutDir: true,
    manifest: true,
  },
})
