# Codebase evidence / filesystem cutover inventory

Đọc bằng glob/grep và ranged read, không chạy git/test/build. Paths dưới đây là repo-relative; prepend `/Users/dat/Projects/ainovel-cli/` để có full path. File đề xuất mới trong phase được đánh dấu Create, không giả đã tồn tại. Repo đã thay đổi account/security contracts trong lúc scout; facts cuối lấy từ lần đọc lại `store.go`/`members.go`/`password.go`/gateway. Không tin các trích dẫn sớm về bcrypt/global collaborator.

## 1. Luồng đã trace, không suy đoán từ tên file

1. Web bootstrap: `cmd/ainovel-cli/serve.go:149` tạo `webauth.NewStore`, `cmd/ainovel-cli/serve.go:65` truyền account store vào `NewGateway`; constructor `internal/entry/web/gateway.go:133-152` gắn project source vào account migration. Request `ServeHTTP` (`internal/entry/web/gateway.go:478`) xử lý non-API/static trước, login/pair/recover/setup ở `:495-506`, cookie authenticate ở `:508-520`, feedback wrapper `:523`, authorization `:528`, account/config/events/jobs/library/comments/projects/TTS handlers `:531-579`. `/healthz` mới phải đứng **trước static dispatch**, không để SPA index trả 200 giả health.
2. Login `internal/entry/web/auth.go:99-133` → limiter → `Store.Login` (`internal/webauth/store.go:738-762`) → verify Argon2 (`internal/webauth/password.go:48-74`) → recheck hash/user dưới write lock → new session → cookie. `clientIP` chỉ RemoteAddr (`auth.go:54-59`); `secureRequest` hiện tin X-Forwarded-Proto bất kỳ (`auth.go:63-64`). Đây là lỗ hổng proxy-trust cần sửa, không phải cookie chưa có Secure; cookie đã HttpOnly/SameSiteStrict (`auth.go:67-72`).
3. Authorization: `routePermission` (`internal/entry/web/authorization.go:14-83`) phân loại admin/project/minimum role; `requestAllowed` (`:85-104`) → `User.ProjectRole/CanRead/CanCollaborate/CanManage` (`internal/webauth/store.go:121-146`). `authorizeRequest` (`authorization.go:105-126`) chặn must-change trừ account allowlist. Stream `authenticatedStoryStream` (`:131-155`) reauthenticate mỗi giây. Không được bỏ recheck khi chuyển cache sang DB.
4. Story process: `internal/entry/web/gateway.go:715-722` gọi `exec.Command` với cwd=story dir và child env; `internal/entry/web/run_worker.go:24-36` load config/assets từ cwd rồi tạo Host. `internal/host/host.go:120-177` giữ lease/log trước tự tạo Store ở175. `internal/host/storyread.go:22-42` cố định `<story>/output/novel` và tạo Store35; LatestReview đọc directory/file ở518/532. Chỉ đổi gateway sẽ không chuyển engine/reader.
5. Tool commit: `internal/tools/commit_chapter.go:90-160` chuẩn bị/saga; `:309,527,777` gọi comments hook sau write. Rewrite `:776-790` viết prose rồi record/aggregate. `internal/tools/edit_chapter.go:32-35` dùng `agentcoretools.NewProseEdit(s.Dir())`, còn đọc/ghi file ẩn dù grep `os.` không thấy. `internal/tools/comments_hook.go:13-15` gọi `comments.AfterWrite` bằng engineDir. `rules.SiblingOpenings` (`commit_chapter.go:666`, `internal/revision/projector.go:216`) và ledger/library sibling lookup cũng bypass storage facade.
6. Audio: `internal/entry/web/audio.go:125-189` Stat story, resolve Layout, config, cache theo path, NewService rồi store vào sync.Map; composer `:164-181` mở `NewStoryReader`. `internal/audio/service.go:229` tạo dirs/load file state và queue; `internal/audio/queue.go:57-79` đọc queue JSON. `internal/audio/export_result_cache.go:34-50` cache dùng size/mtime chứ không content hash. Cần chuyển metadata vào DB, render/export binaries vào BlobStore; ffmpeg scratch vẫn được dùng.
7. Revision: `internal/revision/service.go:129-145` ưu tiên pending revision rồi Scan; `internal/revision/scan.go:41-42` đọc prose disk so với chapter record; accept/project/reanchor `service.go:193-215`. DB mode dùng explicit draft/version conflict, không coi temp scratch là người dùng sửa file. File mode vẫn giữ external-edit detector.

## 2. Direct filesystem operations: production, không tính `_test.go`

Các vị trí được grep tất cả variants ReadFile/WriteFile/ReadDir/Open/OpenFile/Create/CreateTemp/Mkdir/MkdirAll/MkdirTemp/Rename/Remove/RemoveAll/Stat/Lstat/Readlink/Symlink/Link/Chmod/Chtimes/Truncate/WalkDir. Line ranges trong một ô chỉ nhóm các callsite gần nhau, không khẳng định mọi dòng trong range là IO. `os.Getenv/Getwd/UserHomeDir/IsNotExist` không phải đọc/ghi nội dung nhưng vẫn cần phân loại path/config lifetime. Test setup không chuyển sang production repository.

### Tools / Host / Revision

| File và callsites | Chức năng | Đích |
|---|---|---|
| `internal/tools/edit_chapter.go:35`; `comments_hook.go:14`; `ledger_gate.go:13,17,28,43,48`; `retention.go:77`; `retention_context.go:21`; `save_foundation.go:177`; `commit_chapter.go:666` | Không direct os production trong tools nhưng IO gián tiếp | Repository prose edit/comments/siblings/ledger, phase 06 |
| `internal/host/book_lock.go:28` | Mkdir trước flock | File backend lease; PG lease advisory/fence |
| `internal/host/books.go:106,134` | Root/book updatedAt stat | Repository timestamps |
| `internal/host/storyread.go:518,532` | LatestReview ReadDir/ReadFile | Reviews repository |
| `internal/host/usage_replay.go:22` | Session JSONL Open | Agent session repo |
| `internal/host/web_live.go:350,357,364,369,377,384,435,448` | Rules/style samples read/write/remove/dirs | Typed documents + blobs |
| `internal/host/web_revision.go:52,104` | Session/prose ReadFile | PG draft/version; file mode giữ |
| `internal/host/web_simulate.go:39` | Source Stat | Uploaded source blob or CLI adapter |
| `internal/host/imp/analyze.go:122` | Remove analysis artifact | Import artifacts repo |
| `internal/host/imp/preview.go:46`, `runner.go:327`, `source.go:68` | Read uploaded source/path | Blob source reader, CLI file reader |
| `internal/host/imp/workspace.go:85,90,97,100,105,117,127,144,174,181,193,220,243,246,249,256,286` | Workspace checks, atomic files, rename publication | Typed import state/artifacts + immutable source blob |
| `internal/host/sim/import.go:27`, `scanner.go:29,41,51` | Profile/source reading/walk | Repo/blob source listing; CLI adapter |
| `internal/host/exp/exporter.go:134,146,215,240,243,248,265`; `inspect.go:34`; `pattern.go:113` | Collision handling/atomic output/count | Blob metadata conflict policy; scratch for render; offline file export giữ |
| `internal/revision/migration.go:158,181,203` | Legacy session/import baseline | File importer only |
| `internal/revision/scan.go:42` | External prose read | File detector only; PG draft/content version |

### Library / Comments

| File và callsites | Chức năng | Đích |
|---|---|---|
| `internal/library/drafts.go:28,33,39,64,68,80,85,90,117,124,127,133,138,141,144,183,187,209,212` | Co-create draft mkdir/stat/list/rename/cleanup | Durable document/job state |
| `internal/library/files.go:25,40,54,59,65,83,100,111,116,120,135,139,160,166,176,180,181,192,197` | Project/story JSON, common atomic write, style list | Library repo/file adapter |
| `internal/library/inspect.go:70` | Directory listing | Local-only picker |
| `internal/library/library.go:198` | Project path Stat | ID resolver for PG |
| `internal/library/ops.go:23,33,61,168,183,188,194,199,232,235,316,350,368,409,449,498` | Create/rename/content operations | projects/stories/project_documents |
| `internal/library/places.go:44,106,215,274,372,397,427` | Home/volumes browse | Local-only; not public VPS filesystem |
| `internal/library/scan.go:52,171,222,235,238,254,269,322` | Detect project/story layouts | Repo lists/import resolver |
| `internal/library/serveconfig.go:40,67` | Saved local root/network config | File CLI only; hosted env |
| `internal/library/validate.go:64,81,110,178,193,237,245,269,286` | Paths/rules/symlink validation | File/import confinement; field validator PG |
| `internal/library/writable_unix.go:13`; `writable_windows.go:17,22,24` | Writability probe | File backend only |
| `internal/comments/comments.go:173,185,187,268,296,349,428,449,477,511,532,536,539,544,895,913,937,946,951` | Per-chapter JSON, atomic temp/rename, candidate chapter scan, pending events JSON | Comments repo + event state; no JSONL invented |

### Audio

| File và callsites | Chức năng | Đích |
|---|---|---|
| `internal/audio/cache.go:182,195,212` | WAV segment cache read/existence/remove | BlobStore + audio_renders |
| `internal/audio/cast.go:319,344,375` | Cast and project/stories fallback | audio_cast/settings repo |
| `internal/audio/chapter.go:87,97,122,126,172,188,191,208,212,217` | Chapter manifest/output; temp peaks/merge | DB manifests+blob; scratch allowed |
| `internal/audio/cover.go:224,270,293,308` | Cover source/cache | Blob metadata/reader + scratch |
| `internal/audio/export.go:212,216,239,258,463,466,580,608,634,692,697,704,710,715,737,743,746,747,750,751` | Render/export/cache/retag/copy/case rename | Content-addressed cache/blob, temp ffmpeg preserved |
| `internal/audio/export_cache.go:46,59,84,91,113,121,146,154` | Manifest and mtime hash cache | export_cache input manifest/content hash |
| `internal/audio/export_files.go:50,56` | ZIP inputs/output | Streaming blob reads / bounded temp ZIP |
| `internal/audio/export_pending.go:81,169` | Durable request file/output conflict | jobs, BlobStore key conflict |
| `internal/audio/export_prefs.go:29` | Preferences | audio_settings / typed preference payload |
| `internal/audio/export_result_cache.go:35,49,62,73,121,215,240,263,300,304,305,310,315` | Cached results, retag and metadata scratch | export_cache/blob; render scratch allowed |
| `internal/audio/export_selection.go:78,91` | File collision policy | metadata transaction version/name policy |
| `internal/audio/ffmpeg.go:182,186,189,195,209,223,226,237,251,254,347,350,355,380,392` | Binary transform scratch and cover input | Keep scratch, stage blobs; bounded memory |
| `internal/audio/flags.go:29,51` | Flags | audio_settings/typed flags |
| `internal/audio/meta.go:418` | Metadata file helper | Blob reader for persistent content |
| `internal/audio/paths.go:128` | Ensure directory layout | File driver/scratch only |
| `internal/audio/queue.go:59` | Queue load, writes through helper | jobs/audio repo |
| `internal/audio/render.go:590,617,628,659,691,692,712` | Candidate clips/meta/segment lookup | audio_renders + blobs; candidates persisted not temp-only |
| `internal/audio/script.go:376,406,427,430,435,447` | Script and common atomic write helper | audio_scripts/segments; local driver retains helper |
| `internal/audio/settings.go:44,66,86,138` | Settings and pronunciation | audio_settings/project_documents |
| `internal/audio/voice_sample.go:26,30,39,47` | Voice sample ffmpeg scratch | Allowed temp scratch, source via blob |
| `internal/audio/voices.go:44,71,115,134,136,143,167,195` | Voice index/sample import/delete | audio_voices + sample blobs |

### Web

| File và callsites | Chức năng | Đích |
|---|---|---|
| `internal/entry/web/audio.go:129,1013,1018,1020,1349` | Story existence; voice upload temp; file stream | Repo existence, blob upload/GET; temp preprocessing |
| `internal/entry/web/config.go:147,510,1523,1617,1654,1659,1663,1697` | Scope configs/rules files | DB documents with inheritance, secrets encrypted |
| `internal/entry/web/fs.go:79,127,133` | Inspect/mkdir local user filesystem | File mode only; hosted returns explicit unsupported capability |
| `internal/entry/web/gateway.go:277` | launch story dir validation | PG UUID resolution, no dir requirement |
| `internal/entry/web/projects.go:472,553,561,568,572,814,1229,1247,1289` | Project/rules copying, card stats | Repo list/create/update/count/timestamps |
| `internal/entry/web/storyread.go:157,496,501,513,627,912,917` | Scan/cache fingerprint; export temp/list/download | Repo versions/cache; blob streaming |
| `internal/entry/web/watch.go:92,108` | Filesystem fingerprint polling | File backend watcher; PG commit events/reconnect revision |
| `internal/entry/web/worker.go:517,522,528` | Temp text export streaming | Blob export + ephemeral render scratch |
| `internal/entry/web/worker_live.go:154,169` | User file path resolution | Uploaded blob ID for hosted; CLI file path remains |
| `internal/entry/web/activity_history.go:192,204,227,266,280,325` | tui/autoexport JSONL and timestamps | Durable activity/events, stdout logs diagnostic only |
| `internal/entry/web/audio_resume.go:59,76` | queue/export request reads | jobs/audio repository |
| `internal/entry/web/cocreate_project.go:137,142,145,164,188` | idea/lost draft/rename publication | project_documents and transactional publish |
| `internal/entry/web/cover.go:39,45,50` | source assets paths | Blob source, scoped metadata |
| `internal/entry/web/exports_download.go:144,213,261,289,317,328,368` | exported result stat, temp staging and ZIP | BlobStore streaming and explicit scratch |
| `internal/entry/web/reveal.go:49`; `story_delete.go:22` | filesystem reveal and delete | Local capability; PG delete relational/soft-delete |
| `internal/entry/web/search_api.go:80,91,140,168` | asset/chapter markdown scan | Scoped Postgres trgm/document search |
| `internal/entry/web/universe_api.go:45,56,227,240` | ledger timeline/characters file scan | Typed world/characters/events repositories |
| `internal/entry/web/worker_cocreate.go:69`; `worker_job_registry.go:82,106`; `worker_results.go:118` | co-create rules/job persistence/result input | Durable jobs/documents + blob/scratch |

### Outside requested directories nhưng bắt buộc đi theo dependency

`internal/store/io.go:52-74,100-145` implements atomic JSON/Markdown writes; `internal/store/nao_vault.go:68`, `internal/store/revisions.go:93,127` still enumerate filesystem. `internal/bootstrap/configfile.go:20-36,98-165` resolves config by home/cwd/project; hosted config cannot silently read VPS home. `internal/webauth/members.go:98,178-185` reads/migrates accounts and backup; file mode remains. `internal/rules/openings.go` and `internal/ledger/project.go` are source owners reached through verified callers above; phase 06 must inspect exact helper internals before changing them. Embedded assets may stay read-only; custom rules/style/voice sources are persisted content, not “embedded assets”.

## 3. Existing on-disk layout

Root config `<root>/.ainovel/library.json`; `project.json`; directories chosen by config, defaults `y-tuong`, `tai-san`, `kich_ban`, `xuat-ban`, story-local `audio` (`internal/library/library.go:19-96`). Anonymous legacy project is `_`, not a physical underscore folder. Story has `story.json`, `.ainovel/config.json`, custom rules/style; reader uses `<story>/output/novel` (`internal/host/storyread.go:22-35`). CLI outside a library may have explicit engine root; importer must declare historical layout instead of probing arbitrary fallback.

Engine: `meta/book.json` plus derived book.md (`internal/store/book.go:18,38-41`), `meta/progress.json` (`progress.go:27,44`), `meta/run.json` (`run_meta.go:33,43`), `meta/chapter_records/%06d.json` (`chapter_records.go:14-21`), `chapters/%02d.md`, `drafts/%02d.draft.md`, plans (`drafts.go:20-26,37,84-89`), outlines/characters/world/summaries, `meta/snapshots`, reviews including rounds and `reviews/decisions.json` (`review_decisions.go:11`), `meta/checkpoints.jsonl` (`checkpoints.go:16`), `meta/decisions.jsonl` (`decisions.go:25`), runtime/session/usage logs, pending commit/revision and import workspace (`internal/host/imp/workspace.go:51-74`). `nao/` markdown is derived, JSON ledgers are facts.

Comments `<story>/.ainovel/gop-y/<n>.json`, `_events.json` (`internal/comments/comments.go:121-140,182-183,895-896`) with statuses pending/sent/fixed/lost/unsure/done (`:37-47`). Audio `<story>/audio/settings.json`, `vai.json`, `ban-doc/tap-NN/CC.json`, `chuong/tap-NN/CC`, `ung-vien/tap-NN/CC/<segment>`, `cache/<synth-key>.wav`, queue/export requests (`internal/audio/paths.go:21-35,81-117`, `internal/entry/web/audio_resume.go:59-76`). Project custom voices/pronunciation from resolved Layout. Import honors configured dirs; no invented scripts/tap or comments/events.jsonl layout.

## 4. Construction/callsite ownership and lifetime

- Existing `store.NewStore(dir)` stays explicit file constructor. Scoped production constructor set (8): `internal/host/host.go:175`, `storyread.go:35`, `books.go:96`; `internal/entry/web/projects.go:98,1194`; `ledger_api.go:51,193,198`. These require hosted factory injection or explicitly remain local-only; previous rough scout line list was superseded by re-grep.
- Facade shape stays `*store.Store`, existing tool constructor parameter shape is retained. Field concrete→interface and compound method implementation changes are owned by 02/05/06 sequentially; external callers do not need gratuitous constructor renames. All package-local `.io` usages must move behind file backend; do not export an `IO()` escape hatch.
- Reader callers:14 production (`NewStoryReader`, plus one WithStore at `storyread.go:34`). `internal/entry/web/audio.go:165,228,251,672`; `comments.go:60`; `lessons_api.go:71`; `notify.go:210`; `projects.go:939`; `quality_api.go:264`; `storyread.go:33`; `universe_api.go:126,143`; `watch.go:168`; `internal/host/autoexport.go:69`. All14 enumerated;14 refers NewStoryReader calls, the WithStore call is an additional branch in same factory. Keep file constructors, inject PG factory instead of rename shims.
- `NewGateway` production caller: `cmd/ainovel-cli/serve.go:65`. Test constructors observed: `internal/entry/web/auth_test.go:29`, `config_test.go:59`, `fs_test.go:237`, `gateway_test.go:63,98`, `audio_followup_test.go:154`, `setup_test.go:56,358,382,400` (10 calls, 7 test files). Keep constructor parameters stable by accepting an account facade backed by file/postgres; add backend options through one composition path, no duplicate gateway mode.
- `NewService` production construction `internal/entry/web/audio.go:164`; process-wide `audioServices` cache at `:94`, per-story create locks `:106`, configuration check `:145`, composer closes over story `:164-181`. Pool/blob client may share; actor/session/current request may not. Re-key PG cache by stable project/story UUID + backend identity/config version, not path. Remote worker run state is job/lease-scoped.
- Host `New` invoked from worker `internal/entry/web/run_worker.go:36`; per-run Host owns Store/models/lease. `internal/entry/web/gateway.go:715-722` supplies process env/cwd: PG DB env/story UUID pass without credentials in argv. TTS Mac is a new separate pulling worker, not the local engine child process.
