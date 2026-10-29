# ainovel web

React 19 + Vite + TypeScript + Tailwind CSS v4. Install with `npm install` inside `web/`.

- `npm run dev`: Vite on port 5173, proxying `/api` and `/auth` to `http://127.0.0.1:8686`. Set `AINOVEL_TOKEN` to inject bearer authorization for development.
- `npm run dev:mock`: standalone development UI with an in-memory mock API and a live SSE stream; no Go server required.
- `npm run typecheck`: strict TypeScript check.
- `npm run build`: typecheck and emit production files to `internal/entry/web/dist/` (committed for Go embedding).

The browser transport and server data shapes live in `src/api/`.
