import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// `npm run dev` → proxy /api sang `viderse serve` (:8789).
// 产物嵌入 Go 二进制（internal/web/dist, //go:embed all:dist）。
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: { '/api': 'http://127.0.0.1:8789' },
  },
  build: { outDir: '../internal/web/dist', emptyOutDir: true },
  test: {
    environment: 'happy-dom',
    include: ['src/**/*.test.{ts,tsx}'],
  },
})
