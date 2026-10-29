import stylex from '@stylexjs/unplugin'
import type { Plugin } from 'vite'
import { defineConfig } from 'vitest/config'

// Test của kit biên dịch stylex.create/defineVars như lúc build. Vitest không có httpServer
// nên vòng hẹn giờ HMR trong configureServer của plugin không bao giờ dừng → bỏ hook đó.
function stylexPlugin(): Plugin {
  const plugin: Plugin = stylex.vite({ useCSSLayers: true })
  delete plugin.configureServer
  return plugin
}

export default defineConfig({ plugins: [stylexPlugin()] })
