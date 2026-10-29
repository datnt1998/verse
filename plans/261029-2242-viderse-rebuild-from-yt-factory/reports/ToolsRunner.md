# ToolsRunner — `apps/viderse/internal/tools`

Port of yt-factory `src/server/tools/` (commit `2c201e6`) to Go. Package owns tool
manifests, the atomic installer, process-group runner, per-tool probes, Codex
isolation preflight and Chrome CDP probing. No globals; every external dependency
(HTTP client, target dir, process runner, path store, clock, process table) is injected.

## Files

| File | Purpose |
|---|---|
| `manifest.go` | Pinned versions + SHA-256 per platform (ffmpeg 6.0/b6.1.1, Chrome for Testing 155.0.8059.79, uv 0.13.0, Codex 0.162.1), `ParseSha256Sums`, `PinnedAssets` (copy), pin accessor funcs `FfmpegPin()/ChromePin()/UvPin()/CodexPin()` |
| `process.go` | `Runner` interface + `ExecRunner` (Setpgid, SIGTERM→grace→SIGKILL on group, serialized `OnLine`/`Log` under one mutex, `MaxLineBytes` cap, tail-capped capture, full drain before `Wait` returns), `tailBuffer`, `FindOnPath`, `FileVerificationCache[T]`, `spawnDetached` |
| `lifecycle.go` | `TerminateGroup` policy (identity re-proof before SIGKILL), `Registry` for orphan reaping (`process-groups/<pid>.json`), `ProcessOps`/`Clock` seams |
| `sys_unix.go` / `sys_other.go` | `kill(-pgid)`, group liveness, `Statfs` free space, file identity, `Setpriority` |
| `installer.go` | `Installer`: download→verify SHA-256→`os.Root`-safe unpack→prepare→self-check→atomic rename→flip `current`; shared in-flight runs; cancel sentinel (mid-download interrupt, pre-start abort); `install-state.json`; `DownloadTimeout` |
| `managed.go` | `Managed` registry over the managed tools (install/cancel/state/set path), lifecycle hook for transcription |
| `deps.go` | `Deps` (Runner, PathStore, BaseDir, Client, Home, PathEnv, SystemDirs, Registry, CDPSpawn seam), `Tools` bundle + `New`, resolution order |
| `ffmpeg.go`, `uv.go`, `ytdlp.go`, `codex.go`, `codex_isolation.go`, `chrome.go` | per-tool probes/status/install specs/run helpers; login serialization; CDP via registry-tracked spawn |
| `machineload.go` | machine load line (memory pressure, swap, thermal, Chrome RSS) |
| `tempdirs.go` | stale temp-dir sweep (Chrome probe profiles etc.) |
| `errors.go` | `ToolError` with stable code + `RetryAt` |

## Exported API for later consumers

### Runner (inject a fake in tests)
```go
type Runner interface {
    Run(ctx context.Context, file string, args []string, spec Spec) (Result, error)
}
type Spec struct {
    Dir, Env map[string]string, EnvExact map[string]string
    Timeout time.Duration; LogLimit int
    OnLine func(line string, stream Stream)   // serialized, lines ≤ LineLimit
    Log io.Writer; LineLimit int; OKCodes []int; Stdin []byte; Background bool
    Match, Label string
}
type Result struct{ Code int; Stdout, Stderr string; Duration time.Duration }
type Stream string // StreamStdout | StreamStderr
var ErrCancelled error
type ProcessFailedError struct{ Message string; Code int; Stdout, Stderr string; TimedOut bool; Err error }
type ExecRunner struct{ Grace time.Duration; Registry *Registry }
const MaxLineBytes = 4<<20
```

### Lifecycle
```go
type Signal int // SigTerm, SigKill
type GroupTarget struct{ PGID int; Identity []string }
type TerminateOptions struct{ Grace, Reap time.Duration; Clock Clock; Ops ProcessOps } // nil → system defaults
func TerminateGroup(ctx, GroupTarget, TerminateOptions) (bool, error)
type Registry; func NewRegistry(baseDir string, ops ProcessOps) *Registry
func (r *Registry) Track/UntrackForget/Prune/ReapOwner/ReapOrphaned(...)
```

### Resolver / Deps
```go
type Deps struct{ Runner Runner; Paths PathStore; BaseDir string; Client *http.Client;
    Home, PathEnv string; SystemDirs []string; Registry *Registry;
    CDPSpawn func(*exec.Cmd) (pgid int, err error) }
type PathStore interface{ Get(key) (string,bool); Set(key,path) error }
type FilePathStore struct{ Dir string }
type ToolSource string // SourceConfigured | SourceSystem | SourceManaged
type Tools struct{ Ffmpeg *Ffmpeg; Uv *Uv; YtDlp *YtDlp; Codex *Codex; Chrome *Chrome }
func New(Deps) *Tools
```
Resolution order everywhere: **configured path → system binary → app-managed copy**.
Setting keys: `FfmpegSettingKey`, `UvSettingKey`, `CodexSettingKey`, `ChromeSettingKey`.

### Installer
```go
type Installer struct{ Client *http.Client; BaseDir string; DownloadTimeout time.Duration } // 0 → 30m per download
type InstallSpec struct{ ToolID string; Pin ToolPin; Prepare, SelfCheck, Finalize func(...) }
func (i *Installer) Install(ctx, InstallSpec) (InstallResult, error)
func (i *Installer) RunLifecycle(ctx, toolID, version string, run func(runCtx context.Context, lc *LifecycleContext) error) error
func (i *Installer) RequestCancel(toolID string); CancelRequested; ReadInstallState; InstallInProgress
func ToolRoot/ToolVersionDir/ActiveVersionDir(baseDir, toolID[, version]) string
type InstallPhase string // checking|downloading|installing|self-check|activating|done|failed|cancelled
type ChecksumError struct{ Asset, Expected, Actual string }
func (m *Managed) RegisterLifecycle(id ManagedToolID, run func(ctx context.Context, lc *LifecycleContext) error)
```
Extraction goes through `os.Root`: `..` members, absolute and escaping symlinks are
rejected (explicit target check — `Root.Symlink` only blocks traversal on resolve).

### Probes
- `Ffmpeg.Probe/Resolve/Status/Candidates/ConfiguredPath/SetConfiguredPath/ManagedPath/InstallSpec`;
  pure: `ParseFfmpegVersion`, `ParseFfmpegArch`, `ParseFfmpegCaps`, `MissingFfmpegCaps`.
- `Uv.Probe/Resolve/Status/Env/EnvInfo/EnvBuiltFingerprint/EnsurePythonEnv/InstallSpec`; `ParseUvVersion`.
- `YtDlp.Status/Resolve/IsInstalled/Run/LatestVersion/Install`; `RunYtDlpOptions{Timeout, LogLimit, OnLine, Log, CookieFile, JSRuntime}`;
  `ClassifyYtDlpError` → `YtDlpError{Code, Message, StderrTail}`; `YtDlpErrorCodes()` copy.
- `Codex.Status/Resolve/LoginStatus/Isolation/IsolationPreflight/StartLogin/CancelLogin/InstallSpec`;
  `ParseCodexVersion`, `CompareVersions`. Login ops serialized: concurrent `StartLogin`
  yields one spawn; `CancelLogin` holds the lock through group termination; wait-goroutine
  clears the state file only if the PID still matches.
- `CodexIsolation`: `RestrictImageModelCatalog`, `RestrictTextModelCatalog`,
  `InspectImageToolAllowlist`, `InspectAdvertisementRequest` (concatenates all
  `additional_tools` entries), `InspectTextAdvertisementRequest`,
  `IsolationConfigOverrides`, `SandboxProbeArgs`, `CodexExecArgs`, `CodexTextExecArgs`,
  `CreateIsolatedCodexHome`, `CodexDisabledFeatures()/CodexEnabledFeatures()/CodexTextEnabledFeatures()` copies.
- `Chrome.Resolve/Status/ExportCaps/InstallSpec`; `ParseChromeVersion`, `ParseProbeDom`, `ChromeCapsOk`.
  `evaluateInHeadlessPage` spawns through `Deps.CDPSpawn` (default `cmd.Start`) and tracks
  the group in `Deps.Registry` until `TerminateGroup` completes.
- `MachineLoad.Line`; `ParseSwapUsedGb`, `ParseThermalWarnings`, `ParseMemoryFreePercent`, `ParseVmStatFreeGb`, `SumRssGb`.
- Pin/slice tables now private vars + copy-returning functions (`FfmpegPin()`,
  `ManagedToolIDs()`, `YtDlpErrorCodes()`, `AppTempPrefixes()`, `CodexDisabledFeatures()`…).

### Error codes (`errors.go`)
`ErrCodeToolMissing = "tool_missing"`, `ErrCodeInternal = "internal_error"`;
`ToolError{Code, Message, Recovery, RetryAt *time.Time, RetryAtSource}`,
`MissingToolError`, `AsToolError`.
yt-dlp codes: `login_required`, `unavailable`, `cookies_failed`, `no_subtitles`, `rate_limited`, `failed`.

## Review-fix round (post initial delivery)

- `Run`: `Stdout`/`Stderr` writer adapters instead of `StdoutPipe`+goroutines — `Wait` now
  guarantees the child has closed its pipes before `Run` returns; `OnLine`/`Log` serialized
  under one mutex; trailing partial line flushed; `MaxLineBytes` fails explicitly; kill
  sequence runs on a detached context so caller-cancel mid-grace cannot skip SIGKILL;
  caller ctx `DeadlineExceeded` → `ProcessFailedError{TimedOut:true}`.
- Installer: `DownloadTimeout` (default 30 min/download); `cancel-request` watcher aborts
  in-flight downloads via ctx, not only at chunk boundaries; `bytesDone` accumulates per
  completed asset (no multiplicative overcount); pre-start sentinel aborts the run as
  cancelled (upstream semantics restored); `runLifecycle` gets `runCtx` in `run`,
  writes `PhaseDone`, honors pre-start sentinel; `InstallBusyError` and `lifecycle` flag removed.
- `os.Root`-based extraction; absolute/`..` symlink targets rejected explicitly; codex
  linux-x64 manifest member fixed (`codex-x86_64-unknown-linux-musl`).
- `advertisementTools` concatenates all `additional_tools` entries; preflight loopback
  server tracks conns via `ConnState` so the verdict is read only after handlers drain;
  `inspectPromptInput` `/private` prefix strip fixed (ImageGen report).
- `Codex` login ops under one mutex with spawn seam; `evaluateInHeadlessPage` spawn is
  registry-tracked and injectable via `Deps.CDPSpawn`.
- Exported mutable package vars (pins, feature lists, `ManagedToolIDs`, `YtDlpErrorCodes`,
  `AppTempPrefixes`, `SystemClock`) → private vars + copy-returning funcs or unexported default.

## Verification

```
GOTOOLCHAIN=go1.26.0 go build ./apps/viderse/internal/tools/      # ok
GOTOOLCHAIN=go1.26.0 go vet   ./apps/viderse/internal/tools/      # ok
go test -p 2 -count=1 ./apps/viderse/internal/tools/              # ok 27.0s
go test -race -p 1 ./apps/viderse/internal/tools/                 # ok 28.6s
GOTOOLCHAIN=go1.26.0 gofmt -l ... ; GOTOOLCHAIN=go1.27.0 gofmt -l ...  # clean
pgrep -fl hang.sh                                                 # none
VIDEROSE_TOOLS_SMOKE=1 go test -run TestSmoke -v                  # real binaries
```

Smoke output on this machine: ffmpeg 9.0.2 (arch arm64, `missing=[encoder:libwebp]` — the
Homebrew build lacks libwebp; real observation, not a bug), ffprobe 9.0.2, yt-dlp 2026.08.19,
uv 0.10.7, codex 0.162.1.

## Notes / open issues

- Installer uses an in-process in-flight lock (single-process app); cross-process install
  serialisation (old SQLite lease) was intentionally dropped as out of scope.
- `codex-code-mode-host` and Codex CLI archives share a hash source; the values carried over
  from yt-factory were not re-downloaded (no large downloads allowed here). If a checksum ever
  fails at install time, re-pin from the release `SHA256SUMS`.
- Transcription's Python-env lifecycle registers through `Managed.RegisterLifecycle`
  (signature: `func(ctx context.Context, lc *LifecycleContext) error`); the transcription
  package owner should call `RunLifecycle` via that hook.
- `imagegen` is the only external package consumer so far (`tools.New`, `Deps`, `Runner`);
  the pin→accessor change does not touch it, but `Managed.RegisterLifecycle`'s new
  signature is the one to use when wiring transcription.
