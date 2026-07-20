# gocrawl web UI

The browser frontend for `gocrawl serve` — a small React + TypeScript SPA (Vite) that talks
to the API in [`internal/webserver`](../internal/webserver). See
[`docs/web.md`](../docs/web.md) for the full guide.

```sh
npm install
npm run dev      # dev server on :5173, proxies /api to gocrawl serve on :8080
npm run build    # builds straight into ../internal/webserver/webui/dist (embedded by Go)
```
