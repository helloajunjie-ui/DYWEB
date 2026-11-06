import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'

export default defineConfig({
  plugins: [vue()],
  base: '/',
  build: {
    outDir: '../server/webdist',
    emptyOutDir: true,
    rollupOptions: {
      input: {
        index: resolve(__dirname, 'index.html'),
        overlay: resolve(__dirname, 'overlay.html'),
        panel: resolve(__dirname, 'panel.html')
      }
    }
  },
  server: {
    proxy: {
      '/api': 'http://127.0.0.1:9101',
      '/push': 'http://127.0.0.1:9101',
      '/sfx': 'http://127.0.0.1:9101',
      '/ws': { target: 'ws://127.0.0.1:9101', ws: true }
    }
  }
})
