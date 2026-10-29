import { defineConfig } from 'vite'
import stylex from '@stylexjs/unplugin'

// Build thư viện: StyleX biên dịch stylex.create/defineVars vào dist/style.css (consumer không cần plugin).
export default defineConfig({
  plugins: [stylex.vite({ useCSSLayers: false })],
  build: {
    lib: {
      entry: {
        index: 'src/index.ts',
        'tokens.stylex': 'src/tokens.stylex.ts',
        brand: 'src/brand.ts',
      },
      cssFileName: 'style',
      formats: ['es'],
    },
    rollupOptions: {
      external: ['react', 'react-dom', 'react-dom/client', 'react/jsx-runtime', 'react/jsx-dev-runtime', '@stylexjs/stylex', 'qrcode-generator'],
    },
  },
})
