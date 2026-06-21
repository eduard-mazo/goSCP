import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// The build output is written straight into the Go embed package so that
// `go build` bundles the latest frontend into the binary.
export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  build: {
    outDir: '../internal/assets/dist',
    emptyOutDir: true,
  },
  server: {
    port: 5173,
    // Proxy API calls to the Go backend during development so there is no CORS
    // friction and the dev experience mirrors production (same-origin).
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
})
