# MachineSetup — phase 02 (activity, tools, logs, setup)

Status: backend packages complete and tested; **serve.go wiring, web handler shape tests and
the real smoke run are NOT done** (run was stopped by budget at this point). Everything below
that is marked "verified" was actually executed.

## What changed (all new unless noted)

| File | Purpose |
|---|---|
| `internal/activity/activity.go` | Tasks contract (`Start/Update/Finish`), `Notify` (10 s merge), `Reconcile`, `Snapshot`, `MarkRead`; `Store` interface declared consumer-side |
| `internal/activity/errors.go` | `Errors`/`ErrorsCount` feed for Settings › Nhật ký ("Lỗi gần đây", 7 days) |
| `internal/activity/activity_test.go` | in-memory store tests: one-task-per-object, retry reset, counts, reconcile, merge, errors |
| `internal/persistence/postgres/queries/activity.sql` | sqlc queries for `background_tasks` + `notifications` (no new tables, no migration needed — `00001` already has both) |
| `internal/persistence/postgres/activity.go` | `ActivityStore`; merge under `pg_advisory_xact_lock(hashtext(kind))`; nil `ids` marshals as `[]` = all |
| `internal/persistence/postgres/activity_pg_test.go` | pgtest: lifecycle, reconcile, merge+read, 8 concurrent writers collapse to one row count 8 |
| `internal/logs/logs.go` | JSONL per-tool log (`<data>/logs/<tool>.log`), 256 KB cap → keep 128 KB tail; redaction table (Bearer/cookie/token/JSON secret keys/JWT/query params/home path); `Read(tool,level,limit,before)` cursor; `Writer` for `Spec.Log` |
| `internal/logs/bundle.go` | in-memory zip: `summary.json`, redacted `logs/*`, caller-supplied extras (also redacted) |
| `internal/logs/version.go` | `VersionChecker` (6 h TTL + forced `Check`) over a `VersionSource`; GitHub `/releases/latest` source |
| `internal/logs/logs_test.go` | redaction with real-looking samples, rotation, level/limit/cursor, writer partial flush, bundle contents, version cache |
| `internal/toolsvc/toolsvc.go` | `Service`, 6 rows, snapshot cache (`probeTTL` 5 min) + single-flight refresh, live overlay of running installs |
| `internal/toolsvc/probe.go` | per-job state mapping (yt-dlp/ffmpeg/transcription/chrome/codex/codex-login), update check (6 h TTL), friendly error text, managed-id mapping |
| `internal/toolsvc/install.go` | background install tasks (kind `setup`, object `tool/<id>`), 250 ms progress pump (≤4 writes/s, only on change), failure keeps row `failed` + `Thử lại`, cancel |
| `internal/toolsvc/report.go` | `StepChecks`, `LiveProgress`, `RequiredBytes`, `BaseDir`, `Attention` |
| `internal/toolsvc/toolsvc_test.go` | six-row order/states, update-available → `update`, failed install → task `failed` + row `failed`, attention, no-op cancel |
| `internal/setup/setup.go` | W1 orchestration: 5 installs in order, failure isolation, overall `running|login|failed|done`, disk block, single run + per-step retry |
| `internal/setup/setup_test.go` | order, failure isolation, single-step retry, Start joins |
| `internal/domain/tools.go` | `SetupStepID`s, `SetupCheck/SetupNeed`, `InstallProgress` shared shapes |
| `internal/tools/transcription.go` + `transcript-requirements.txt` | port of `transcription.ts`: embedded pinned requirements, uv → venv → models → Silero self-check; `Status` probe |
| `internal/tools/sys_unix.go` | exported `FreeDiskBytes` |
| `internal/tools/ytdlp.go` | added `(*YtDlpError).ErrorCode() string` (requested by Platforms) |
| `internal/web/activity.go` | `MountActivity`: GET `/api/activity`, POST `/api/notifications/read`, GET `/api/errors` |
| `internal/web/tools.go` | `MountTools`: GET `/api/tools`, POST `/api/tools/check`, POST `/api/tools/{id}/install|cancel` |
| `internal/web/setup.go` | `MountSetup`: GET `/api/setup`, POST `/api/setup/start`, POST `/api/setup/retry` |
| `internal/web/logs.go` | `MountLogs`: GET `/api/tools/logs` (cursor), GET `/api/tools/logs/export`, GET `/api/version`, POST `/api/version/check` |

## Wire shapes (sent to SettingsWebA)

1. `GET /api/activity` → `{running:[{id,kind,agent?,title,objectRef{type,id,label},phase,progress{done,total,groups?}|null,etaSec,state,action{label,href?,op?}}],videoJobs,queued,unread:[{id,kind,count,text,href,actionLabel,createdAt,readAt}]}`.
2. `GET /api/tools` → `{tools:[{id,state,hint,version?,update?{current,latest},progress?{phase,detail,bytesDone,bytesTotal,pct,startedAt},detail?,retryAt?}],checkedAt}` where `state ∈ ready|update|missing|failed|installing|needs_login`; `POST /{id}/install` → 202 `{started,taskId}`; `POST /{id}/cancel` → `{ok:true}`; `POST /api/tools/check` → same body, forced probe.
3. `GET /api/setup` → `{state:running|login|failed|done,complete,running,steps:[{id,state:done|pending|running|failed|needs_user,progress?,error?}],disk{freeBytes,requiredBytes,enough}}`; `POST /api/setup/start`, `POST /api/setup/retry {id}` → 202 same body.
4. `GET /api/tools/logs?tool&level&limit&before` → `{entries:[{ts,tool,level,message}],nextBefore}`; export → zip attachment.
5. `GET /api/errors` → `{items:[{id,time,what,why,status,tone,href?,link?,op?,object?}],count7d}`; `GET /api/version`, `POST /api/version/check` → `{version,latest,checkedAt}`.

## Tasks implementation notes

- One row per `(kind, object_ref)`: `Start` finds the existing row (in-process mutex + `FindTaskByObject`), resets it to `running` (retry) or reuses it; `Finish` deletes.
- `Reconcile` on startup: every `running|paused|waiting` row becomes `failed`, phase "Bị gián đoạn khi app khởi động lại", action `{label:"Thử lại", op:"retry"}`. It does not resume automatically; the owner/caller restarts and `Start` reuses the same id (verified in pgtest + memory tests).
- Notifications: latest row of same `kind` younger than 10 s is bumped (count+1, text/href refreshed, `created_at` kept); otherwise insert. Race safety via advisory xact lock per kind (`activity_pg_test` 8 writers → one row, count 8).
- `videoJobs`/`queued` count `kind=video` in `running`/`waiting` (0 until queue exists).

## Attention items (for the gear dot)

`toolsvc.Attention` returns kinds `tool_update` (yt-dlp newer release) and `tool_failed` (row `failed`), href `#/settings?section=tools`. **Not yet registered with `statusSvc.RegisterSource`** — the adapter was planned in `serve.go`; see open issues.

## Verification actually run

```
go test -p 2 ./apps/viderse/internal/logs/ ./apps/viderse/internal/activity/ \
  ./apps/viderse/internal/setup/ ./apps/viderse/internal/toolsvc/ ./apps/viderse/internal/web/
→ ok (logs 0.57s, activity 0.50s, setup 0.48s, toolsvc 0.52s, web 3.30s)

TEST_DATABASE_URL=postgres://dat@127.0.0.1:5432/ainovel_test \
go test -p 2 -run TestActivityStore ./apps/viderse/internal/persistence/postgres/
→ ok 0.95s

go tool sqlc generate && go tool sqlc diff   (only queries/activity.sql added) → generated
go build ./internal/{activity,logs,setup,toolsvc,web,domain,tools} ./internal/persistence/postgres → ok
```

`-race -p 1`, gofmt with go1.27.0, web handler shape tests and the port-18791 smoke were **not** run.

## Open issues / handoff

> The five items below were written before wiring landed; items 1 and 2 are now
> done (see “Review fixes”). Item 3–5: smoke ran with real installs but the
> kill/resume branch did not trigger; yt-dlp still has no resumable download;
> Cancel for yt-dlp remains best-effort; the Codex token type now exists (AIConn).

1. **serve.go wiring not written.** Needs: `activity.New(postgres.NewActivityStore(pool), Notify(hub,"activity"))`, `logs.New(<data>/logs, home)`, `tools.New/NewManaged`, `toolsvc.New(Deps{..., ChatGPT: <aiconn adapter>})`, transcription lifecycle registration, `setup.New(toolSvc, dataDir, Notify(hub,""))`, `status` source adapters, four `Mount*` calls, `Reconcile` + `Warm` at startup, bundle extras (settings minus secrets + versions + activity) for `/api/tools/logs/export`.
2. Handlers untested at the HTTP boundary (auth/shape); services and stores are tested.
3. Smoke (5 rows ready in temp HOME, kill/resume, orphan check) not executed; yt-dlp install does not resume in `internal/tools` (its own pipeline restarts the download) — managed installer resumes.
4. `toolsvc.Cancel` is best-effort for yt-dlp (its flight ignores ctx).
5. `internal/tools` lacks a `CodexAuthTokens` type at the time of writing (AIConn owns that file).

## Review fixes (Review-p2-machine.md)

Code (all with regression tests in the package named):

- **H4 — shutdown joins workers.** `toolsvc.Service.Shutdown(ctx)` waits on an install WaitGroup; `setup.Service.Shutdown(ctx)` waits on the run goroutine; `mountMachine` now returns `*machineRuntime{Shutdown}`; `serve.go` cancels `machineCtx`/`bg`, awaits both services (installer cancels its process group inside `Run`), then closes logs/hub and shuts HTTP down. Test: `TestShutdownWaitsForRunningStep` (setup).
- **H11 — backend auto-start.** `setup.EnsureStarted(ctx)` kicks the five installs once when no run exists, no step carries an error, and at least one install step is missing; ChatGPT `needs_user` never blocks. `GET /api/setup/state` calls it before rendering (no client POST needed). After a failed run it never auto-starts again; only `POST /api/setup/retry`. Tests: `TestEnsureStartedStartsOnCleanMachine`, `TestEnsureStartedDoesNotRetryFailedSteps`. SettingsWebA was told the exact shape.
- **H12 — immutable snapshots.** `cloneSnapshot` copies the row slice before overlays; cache is never mutated by `applyLive`; `Refresh` returns a clone. Test: `TestSnapshotDoesNotAliasCache`.
- **H13 — one run per tool.** `reserveInstall` publishes the run placeholder under the service mutex, fills `taskID` and signals `initDone`; `StartInstall` waits for `initDone` so joiners never see an empty id; the worker deletes only its own map entry; `Ensure` waits on the exact run handle. Test: `TestConcurrentStartInstallSingleRun`.
- **H14 — listener first, dead-owner reconcile.** `net.Listen` now happens before storage/machine mounts (a second serve on a busy port exits without touching live tasks). Migration `00006` adds `background_tasks.owner_pid`; `Reconcile` lists tasks, keeps rows whose owner PID is alive (`kill(pid,0)`; 0 = legacy = dead) and fails only the dead ones through `FailTasksByIDs`. Test: `TestReconcileOnlyDeadOwners`.
- **H16 — full credential redaction.** One shared `logs.Redact(line, home)` used by persistence and export; patterns now consume whole `Cookie:`/`Set-Cookie:` headers and named cookie fields (`SESSDATA`, `bili_jct`, `buvid3`, `HSID`, `SID`, `__Secure-*`, …) including JSON-escaped forms. Test: `TestRedactWholeCookieHeaders` checks every fragment is absent from disk and from the built zip.
- **M1 — read-only GET.** `GET /api/tools` and `GET /api/setup/state` ignore `fresh`; a cold tools cache returns an immediate `missing` snapshot and refreshes in the background; `VersionChecker.Get` is cache-only and `Check` does the network call (`GET /api/logs/version` uses `Get`). New `POST /api/setup/check` and the existing `POST /api/tools/check` are the explicit refresh operations. Test: `TestSnapshotNeverProbesOnColdCache`, plus the version test now asserts a cold `Get` makes no call.
- **M4 — live login readiness.** `applyLive` overlays the in-memory ChatGPT session on the `codex-login` row on every snapshot (no TTL wait); `rowCheck(installing)` now returns `NeedInstall`, so setup cannot call an in-progress install ready. Tests: `TestChatGPTLoginOverlayIgnoresCacheTTL`, `TestRowCheckInstallingIsNotReady`.
- **M6 — durable 7-day error history.** Migration `00006` adds `error_events` (kind, what, why, href, action_label, op, object_ref); `Update` records a `task_failed` event and `Notify` records `needs_attention`/`action_result`; `Errors` pages the durable history and `ErrorsCount` counts the window independently (no 20-item cap on `count7d`). Tests: `TestErrorHistorySurvivesReadAndRetry` (25 events → page 20, count 25) and the existing error test.
- **M7 — child output feeds the log store.** `toolsvc.NewLoggingRunner` wraps the shared `tools.Runner`: each invocation gets a `logs.Writer` for the tool (mapped from the binary: yt-dlp, ffmpeg/ffprobe, uv/python, chrome, codex), tees any existing `Spec.Log`, and flushes the trailing partial line after the process ends. Wired in `serve.go` before `tools.New`, so installs and runtime calls share it. Test: `TestLoggingRunnerFeedsLogStore`.
- **L1 — dead weight.** Removed `logs.Store.files`/no-op bookkeeping, `toolsvc.BaseDir/parentDir/transDirBase`, and the duplicated progress conversion is pending in `web` (the two handlers still each convert; tracked below).

Verification run (after the fixes):

```
TEST_DATABASE_URL=... go test -race -p 1 ./apps/viderse/internal/{activity,toolsvc,setup,logs}
activity ok 1.48s · toolsvc ok 1.55s · setup ok 1.57s · logs ok 3.08s
gofmt -l internal/{activity,toolsvc,setup,logs} → clean (go1.26.0; go1.27.0 not on this machine's PATH)
```

Still open / not done in this round:

- `web` package compile was blocked by peers' in-flight `platforms.go`/`aiconn`/`secretbox` edits; the setup/tools/logs handler changes were built only as part of those packages' tests earlier.
- The duplicated progress DTO conversion in `web/setup.go` + `web/tools.go` remains (L1 partial).
- **Smoke numbers from the earlier run are still the only real-install evidence**: ffmpeg + whisper env installed into the temp HOME, Chrome/Codex managed copies installed, no orphan process; the kill/resume branch was not actually triggered because transcription progress reports phase/detail without byte counts, so the harness never reached its >15 MB condition. `TestCancelResume`-style unit coverage exists for the managed installer, and yt-dlp's own flight restarts downloads (no resume).
- `yt-dlp` now prefers the managed copy over a system binary (`Resolve`: configured → managed → system) so the Cập nhật action actually changes the version the row reports; test renamed to `TestYtDlpResolveManagedBeforeSystem`.

## Design correction + follow-up evidence

**GET is side-effect-free (supersedes the earlier backend auto-start).**
`GET /api/setup/state` only renders. The snapshot now carries one server-computed state:
`idle | running | login | failed | done` (`idle` = clean machine that never started in this process).
`POST /api/setup/start` starts only from `idle` and returns the current snapshot otherwise
(202, idempotent); `POST /api/setup/retry` stays per-row. `EnsureStarted` was deleted.
`POST /api/setup/check` is the explicit refresh; GET ignores `fresh`.
SettingsWebA was sent the final shape: predicate is exactly `state === "idle"` → one POST.

- Regression tests: `TestIdleThenStartIsIdempotent`, `TestStartDoesNotRestartAfterFailure` (service),
  and `TestSetupHTTPFlowIdleStartAndNoRetry` in `web` — real gateway + real `setup.Service` over
  the real handlers: GET idle with zero installs, POST start → 5 Ensures → login, second POST no-op,
  failure → `failed` and neither GET nor POST restarts it.
- pgtest (real Postgres, migrations incl. 00006): `TestActivityStoreReconcileFailsOnlyDeadOwners`
  (live owner untouched, dead owner + legacy pid 0 failed) and `TestActivityStoreErrorHistoryDurable`
  (25 events → page 20, count7d 25, MarkRead and Finish do not erase history).
- L1 finished: one `web.installProgressWire` + `installProgressToWire` now feeds both `/api/tools`
  and `/api/setup` (identical JSON shape; duplicated converters deleted).

**verification**

```
go build ./apps/viderse/...                                   → ok
go test -race -p 1 ./internal/{activity,toolsvc,setup,logs}   → ok (cached) + postgres ok 3.89s
go test -run TestSetupHTTPFlow ./internal/web                 → ok 0.76s
go test ./archtest/                                           → ok
gofmt -l (go1.26.0 and go1.27.0) on all touched dirs          → clean
```

**Resume smoke (port 18791, throwaway DB `viderse_smoke_machinesetup2`, `HOME=/tmp/viderse-machinesetup2/home`).**
Real Chrome for Testing install forced through `POST /api/tools/chrome/install`; server `kill -9`
mid-download, then restarted and the same install re-posted:

```
admin ready
kill at chrome bytesDone=135921664 total=192617424
partial files >1MB: 1
first bytes after restart: 155058176
chrome final: ready | /Applications/Google Chrome.app/…/Google Chrome
resume: first=155058176 (killed at 135921664) continued=yes
setup state after: {"state":"idle", … yt-dlp/chrome/codex done, ffmpeg/transcription pending}
processes under temp HOME: none
viderse serve processes: none
cleanup done
```

The download resumed from the partial file (`~/.viderse/downloads`), finished, and no process
outlived the second server. The row’s detail points at the system Chrome because `Resolve()` prefers a
detected install over the managed copy for Chrome (existing convention, unchanged); on a machine
without system Chrome the managed copy under the temp HOME is what reports ready.
DB and temp HOME were dropped/removed; the first attempt at 43 MB and the 135 MB run both resumed.

**Remaining**: the web predicate itself and the M9 low-disk/skip surfaces belong to SettingsWebA;
attention for an unfinished setup is still not emitted by the backend.
