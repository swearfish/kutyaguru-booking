import {defineConfig} from 'vite'
import react from '@vitejs/plugin-react'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],
  // Pin the dev server port and HMR socket so the Wails webview (macOS WKWebView)
  // can connect for hot-module reload. wails.json points dev:serverUrl here.
  server: {
    port: 5173,
    strictPort: true,
    hmr: { protocol: 'ws', host: 'localhost', port: 5173 },
  },
})
