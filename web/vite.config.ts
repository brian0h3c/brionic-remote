import { defineConfig } from 'vite'

// During `npm run dev`, API and WebSocket calls are proxied to the Go backend
// (start it with `go run . --no-browser`). `npm run build` emits to web/dist,
// which the Go binary embeds.
export default defineConfig({
  build: {
    outDir: 'dist',
    // Keep emptyOutDir off so the committed .gitkeep (which keeps the Go embed
    // directory present on fresh clones) survives builds. The Makefile cleans
    // stale hashed assets before each build.
    emptyOutDir: false,
  },
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:8717',
        changeOrigin: true,
        ws: true,
      },
    },
  },
})
