import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  build: {
    // `make web-build` runs `vite build` from here; the output is embedded directly by
    // internal/webserver/assets.go, so it lands there rather than under web/dist.
    outDir: '../internal/webserver/webui/dist',
    emptyOutDir: true,
  },
  server: {
    // Lets `npm run dev` (served from :5173) hit the Go API without CORS wrangling; run
    // `gocrawl serve` on :8080 alongside it during frontend development.
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
})
