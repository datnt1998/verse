# verse

Monorepo for the AI writing apps that share one agent core.

| Path | What it is |
|---|---|
| `apps/noverse` | Noverse: novel/story writing CLI, TUI and web app (binary `ainovel-cli`). See [apps/noverse/README.md](apps/noverse/README.md). |
| `apps/viderse` | Viderse (Go): runs several YouTube channels by remaking rival videos through a 9-step machine pipeline (binary `viderse`). It is being rebuilt from yt-factory on the design in `apps/viderse/docs/design/v1/`; progress in [the rebuild plan](plans/261029-2242-viderse-rebuild-from-yt-factory/plan.md). See [apps/viderse/README.md](apps/viderse/README.md). |
| `core/` | Shared AI core: `agentcore` (agent runtime), `llm` (model interface and providers), `models`, `llmretry`, `llmcontract`, `llmconfig` (shared provider/model config schema), `auth` (OAuth and credentials), `roles` (role→model config). |
| `platform/` | App-agnostic infrastructure: `identity` (web accounts, sessions, pairing), `webguard` (trusted proxies, same-origin guard, client IP, secure-cookie decision), `webapp` (embedded SPA, first-run setup, login/logout/me, session cookie, authenticated `/api` routes), `events` (server-sent events hub: replay ring, `Last-Event-ID` resume, pluggable visibility), `pgmigrate` (embedded goose migrations), `pgtest` (isolated Postgres schemas for tests), `ui` (`@noverse/ui` web kit: tokens, components, icons, brand, Vietnamese labels; `@noverse/ui/data`: keyed resource cache and the SSE client). |
| `archtest/` | Import boundary test: `core/**` imports only the standard library, third-party modules and `core/**`; `platform/**` adds `platform/**`; an app never imports another app. |
| `plans/` | Monorepo-level plans, starting with [the move into this repository](plans/261029-1138-verse-monorepo/plan.md). |

One Go module, `github.com/datnt1998/verse`, rooted here. One npm workspace,
also rooted here: `platform/ui`, `apps/noverse/web` and `apps/viderse/web` share a
single `package-lock.json` and `node_modules`. Each app keeps its own Dockerfile,
migrations and docs inside its directory.

## Build and test

```sh
go build ./...
go test ./...                                   # Postgres/MinIO suites skip unless configured
(cd apps/noverse && go tool sqlc diff)          # generated SQL code is current
(cd apps/viderse && go tool sqlc diff)
npm ci && npm run typecheck --workspaces && npm test --workspaces   # web kit + web apps
npm run build -w apps/noverse/web               # rebuild an app's embedded dist
npm run test:sso                                # two-app shared-login E2E, see tests/sso/README.md
docker build -f apps/noverse/Dockerfile .       # hosted image; context is the repository root
```

Postgres and MinIO for the full suite: `apps/noverse/deploy/compose.test.yml`
(see `.github/workflows/ci.yml` for the exact environment).

## Adding an app

`apps/viderse` is the smallest complete example: Postgres, server-sent events, and a web UI on the shared kit. Copy its layout and wiring.

1. **Layout.** Put the code in `apps/<name>/` with `cmd/<binary>/`, `internal/` and a `README.md`. Add `apps/<name>/.gitignore` only if the app writes local artifacts. Everything stays in this Go module; there is no `go.work`.
2. **Imports.** An app may import the standard library, third-party modules, `core/**`, `platform/**` and itself. `archtest` classifies every `apps/<name>` automatically, so `go test ./archtest` covers a new app with no registration.
3. **Runtime state.** Each app keeps its own:
   - config dir: `~/.ainovel`, `~/.viderse`;
   - credentials: `core/auth` configured per app, see `apps/viderse/internal/bootstrap`;
   - Postgres data database and roles: `noverse_*`; `viderse_go` with `viderse_app`/`viderse_migrate`;
   - fixed default port: Noverse live 8790, Viderse 8789.

   Web accounts are the one shared thing. Noverse owns the account store (`users`, `sessions`, `app_grants` in its Postgres database, accessed through `platform/identity/postgres`). Every app reads it with its own least-privilege role, such as `viderse_identity` from `apps/noverse/deploy/postgres-init/02-identity-role.sql`, and the same `verse_session` cookie. A member opens an app only with an `app_grants` row for it; global admins open every app. Noverse with `STORE_DRIVER=file` stays standalone on `ainovel_session`.
4. **Web server.** Build the HTTP side on `platform/webapp`: `webapp.New(accounts, webapp.Options{AppName, App, UI})`. `accounts` wraps `identitypg.NewAccounts(pool, identitypg.Options{ProjectAccess: false})` over the shared store; proxy, cookie domain and sibling origins come from `webguard.ConfigFromEnv()` through `SetProxyConfig`. The package provides:
   - the SPA, health probes and the same-origin guard;
   - first-run setup with a one-time code and login/logout/me;
   - the app-grant and forced-password gates, and the user's own password and session routes.

   App routes register with `Handle`. Reference: `apps/viderse/internal/web/gateway.go` and `openAccounts` in `apps/viderse/cmd/viderse/serve.go`. A new app also needs its own `identity.App` value in `platform/identity/app-grants.go`, a migration widening the `app_grants.app` CHECK, a checkbox in Noverse's Members screen (`appIds`), and its own identity role.
5. **Web UI.** Put the SPA in `apps/<name>/web/`, add it to the root `package.json` workspaces, and depend on `@noverse/ui`. Copy from `apps/viderse/web`:
   - `vite.config.ts`: kit aliases and the StyleX plugin; `build.outDir` is the directory the Go side embeds;
   - `tsconfig.json` paths;
   - the `@source` line for the kit in `src/styles/base.css`;
   - `setKitLabels({ ...KIT_LABELS_VI, brand })` and the app brand in `src/main.tsx`.

   The kit's `boundary.test.ts` and `Icon.test.tsx` cover every app that depends on `@noverse/ui` automatically. Commit the built dist, because `go build` embeds it.
6. **Postgres.** Embed the goose migrations and apply them with `platform/pgmigrate`. Integration tests use `platform/pgtest`. sqlc config lives in `apps/<name>/sqlc.yaml`.
7. **CI.** Add the app's `go tool sqlc diff` step to `.github/workflows/ci.yml`. A workspace web app needs nothing more, because the `web` job runs every workspace.
8. **Sharing.** Move code into `core/` or `platform/` only once a second app needs it.

To bring in an existing repository:

- import it at a tag without history, as Noverse was at `pre-verse`;
- rewrite its module path to `github.com/datnt1998/verse/apps/<name>`;
- keep its runtime names (binary, config dir, cookie, database roles) so existing data and sessions keep working.

## Origin

Imported on 2026-10-29 from `datnt1998/noverse` at tag `pre-verse` without
history; that repository is frozen and keeps the full history. The module path
changed from `github.com/datnt1998/noverse` to `github.com/datnt1998/verse`
(Noverse code under `github.com/datnt1998/verse/apps/noverse/...`), superseding
the "keep the module path" notes in older design documents. Runtime names are
unchanged: binary `ainovel-cli`, config dir `~/.ainovel`, cookie
`ainovel_session`, database roles `noverse_*`.

Releases (`v*` tags) are Noverse releases, published by GoReleaser from this
repository.

## License

Apache-2.0, see [LICENSE](LICENSE) and [third_party/NOTICE](third_party/NOTICE).
