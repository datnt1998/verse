package web

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/audio"
	"github.com/datnt1998/verse/apps/noverse/internal/blobkey"
	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/apps/noverse/internal/host"
	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
	"github.com/datnt1998/verse/apps/noverse/internal/version"
	"github.com/datnt1998/verse/platform/blobstore"
	"github.com/datnt1998/verse/platform/identity"
	"github.com/datnt1998/verse/platform/secheaders"
	"github.com/datnt1998/verse/platform/webguard"
)

//go:embed all:dist
var ui embed.FS

type launchedWorker struct {
	addr    string
	stdin   io.Closer
	done    <-chan struct{}
	kill    func() error
	noModel bool
}
type storyWorker struct {
	mu       sync.Mutex
	worker   *launchedWorker
	running  bool
	deleted  bool
	starting chan struct{}
	active   int
	deleting bool
}

// beginOperation protects story removal without holding a mutex during a proxy
// request or model call. Deletion refuses active operations instead of queuing.
func (b *storyWorker) beginOperation() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.deleted || b.deleting {
		return false
	}
	b.active++
	return true
}

func (b *storyWorker) endOperation() {
	b.mu.Lock()
	b.active--
	b.mu.Unlock()
}

// accountBackend selects which session contract the Gateway runs. It is
// declared at construction — never guessed from a file path — because cookie
// name and app-grant enforcement differ per backend:
//
//	backendFile     → `ainovel_session`, Noverse grants always effective
//	backendPostgres → `verse_session`, per-app grants enforced per request
type accountBackend int

const (
	backendFile accountBackend = iota
	backendPostgres
)

// Gateway owns one serialized launcher per story, without holding the registry lock during startup.
type Gateway struct {
	// Root 是书库根；运行时可用 PUT /api/library 换，读写一律走 RootPath。
	// 为空表示尚未选择（全新安装）：依赖书库的接口一律回 409 errNoLibrary，不会落到当前目录。
	Root string
	// backend 是账号后端选择（serve 传 STORE_DRIVER）：决定 session cookie
	// 名与 app-grant gate。构造后不可变，不受请求影响。
	backend     accountBackend
	accounts    *identity.Store
	limiter     identity.LoginLimiter
	workerToken string
	rootMu      sync.RWMutex
	mu          sync.Mutex
	workers     map[string]*storyWorker
	launch      func(id, dir, root, token string) (*launchedWorker, error)
	// launchHosted 是 hosted 故事的 worker 启动缝：进程由 spawnWorker 建，
	// scope/存储经 env 传入（绝不在 argv 放 secret）。测试 stub 它跑进程内
	// worker —— 与 launch 分两条缝，文件模式测试不受 hosted 变化影响。
	launchHosted func(key, project, story string) (*launchedWorker, error)
	// transport 是测试注入 outbound HTTP 的缝（worker/代理/上下文同步共用）。
	// 测试会在 watcher 已运行时替换它，所以读写都必须经 workerTransport/
	// setTransport 过 transportMu，否则 -race 会把中间态的赋值抓出来。
	transportMu sync.RWMutex
	transport   http.RoundTripper
	// eventHub 是全局事件流 /api/events（events.go），经 events() 惰性取用；
	// jobs 汇总全部故事的后台任务（GAP-26，jobs.go），经 jobsHub() 惰性取用。
	eventHub      *EventHub
	eventsOnce    sync.Once
	jobs          *jobsHub
	jobsOnce      sync.Once
	versionLocks  sync.Map // resource identity → *sync.Mutex; never a gateway-wide I/O lock
	watchMu       sync.Mutex
	watcher       *fileWatcher
	notifications *gatewayNotifications
	closeOnce     sync.Once
	storageTests  map[string][]time.Time
	// recoverLimiter 限制 POST /api/recover（每 IP 每小时 5 次，recovery.go）。
	recoverLimiter identity.RecoveryLimiter
	// build 是 ldflags 构建身份（/api/system 的 version/commit/built）；
	// latest 是被动检查新版本的结果，由后台 goroutine 写入。
	build    version.Info
	latestMu sync.Mutex
	latest   version.CheckResult
	// setup 是 GAP-41 的首次设置状态，只在启动时还没有任何账号时创建；
	// setupCode 是码的明文，serve 打印一次后清空（之后内存里只剩哈希）。
	setup     *identity.SetupState
	setupCode string
	setupMu   sync.Mutex
	// factStore 是故事事实存储的工厂接缝：nil = file backend 根在
	// storyOutputDir(st)；serve 在 STORE_DRIVER=postgres 时注入 postgres
	// factory（scoped backend + capability gate）。没有任何静默回退：
	// 工厂报错时调用方返回错误，不读目录。user 是当前请求账号 —— nil =
	// 内部后台路径；postgres factory 必须拒绝无 actor 的请求面调用。
	factStore func(st library.Story, user *identity.User) (*store.Store, error)
	// libRepo 是 hosted 的 thư viện repository（projects/stories/ideas 在
	// Postgres）；nil = file backend theo --root。
	libRepo library.Repository
	// workerStore 是 hosted 引擎子进程的存储契约（STORE_DRIVER=postgres 的
	// env 解析结果）：launchHostedWorker 把它转成子进程 env；nil = hosted
	// worker 未配置，此时 startDir 报 capability 错而不是退回目录启动。
	workerStore *bootstrap.StorageConfig
	// pgAudio 是 hosted 音频后端接缝（phase 07）：nil = file backend。
	// 启动顺序是 NewGateway 再 SetAudioStore，而后台 watcher 在构造时就已
	// 启动，所以字段读写一律走 pgAudioMu + audioStoreCh：注入完成（关闭
	// channel）是 watcher 从本地后端切到 hosted 的同步点。
	pgAudio *PGAudioDeps
	// audioStoreCh 在 SetAudioStore 注入后关闭；audioStoreClosed 让重复注入
	// 不会重复关闭。两者都由 pgAudioMu 保护。
	audioStoreCh     chan struct{}
	audioStoreClosed bool
	// pgAudioSvcs 每个 story 一个 PGService（缓存 hub 与已解析的 scope）；
	// 挂 Gateway 而不是包级 map —— 不同 Gateway（比如测试里连不同 schema
	// 的池）拿同一个 slug 绝不能共用服务。
	pgAudioMu   sync.Mutex
	pgAudioSvcs map[string]*audio.PGService
	// remoteVoices caches the worker's published catalogue so render/view paths
	// that ask PGService.Voices repeatedly do not list workers each time.
	remoteVoicesMu   sync.Mutex
	remoteVoicesAt   time.Time
	remoteVoicesList []audio.ProjectVoice
	// Hosted "AI sẽ soạn bản đọc" runner (audio_prepare.go): one sequential
	// compose loop per story, woken by the audio service (enqueue/retry/resume)
	// or by startup recovery. It never polls when idle; a live preparation
	// lease is waited out with a timer instead.
	prepMu      sync.Mutex
	prepWake    chan struct{}
	prepCancel  context.CancelFunc
	prepStarted bool
	prepActive  map[string]bool
	prepWG      sync.WaitGroup
	// proxy 是 trusted-proxy 边界（platform/webguard）；nil = 未配置，沿用 loopback 规则。
	proxy *webguard.Config
	// sec 是 bộ header bảo mật dùng chung (platform/secheaders): HSTS chỉ khi
	// PUBLIC_URL là https.
	sec secheaders.Policy
	// health 是 /readyz 的依赖探针（health.go）；nil = 无外部依赖（file 模式）。
	// blobProbedAt/blobOK 缓存 BlobStore 探测，避免每次探活都打网络；
	// readyMu/readyAt/readyBody/readyStatus 缓存整个探活结果（cooldown 2s），
	// 让公网 /readyz 不能当成压库的杠杆。
	healthMu     sync.Mutex
	health       *HealthDeps
	blobProbedAt time.Time
	blobOK       bool
	readyMu      sync.Mutex
	readyAt      time.Time
	readyBody    readinessBody
	readyStatus  int
}

// workerTransport trả transport hiện hành cho outbound HTTP tới worker; đọc
// dưới RWMutex vì test có thể đổi nó khi watcher đang chạy.
func (g *Gateway) workerTransport() http.RoundTripper {
	g.transportMu.RLock()
	defer g.transportMu.RUnlock()
	return g.transport
}

// setTransport thay transport của outbound HTTP (test dùng; nil = mặc định).
func (g *Gateway) setTransport(rt http.RoundTripper) {
	g.transportMu.Lock()
	g.transport = rt
	g.transportMu.Unlock()
}

// PGAudioDeps 是 Postgres 音频后端的所有依赖（serve 注入一次）。
type PGAudioDeps struct {
	Jobs    *postgres.Jobs
	Audio   *postgres.Audio
	Cache   *postgres.ExportCache
	Nar     *postgres.Narrative
	Blobs   blobstore.BlobStore
	Storage *bootstrap.ManagedBlobs
	// Keys builds BlobStore object keys for the process STORAGE_ENV; serve
	// passes blobkey.FromEnv(). The zero value is the dev environment.
	Keys   blobkey.Keys
	Engine string
	Model  string
}

// SetAudioStore 注入 hosted 音频后端依赖（启动期一次，postgres 模式）。
// 构造先于注入（NewGateway → SetAudioStore），所以这里除了写入还要关闭
// audioStoreCh：已经在跑的 jobs watcher 据此从本地安装状态切到 hosted
// worker 状态，不会永远停在构造时选中的本地后端。
func (g *Gateway) SetAudioStore(deps *PGAudioDeps) {
	g.pgAudioMu.Lock()
	defer g.pgAudioMu.Unlock()
	g.pgAudio = deps
	if g.audioStoreCh == nil {
		g.audioStoreCh = make(chan struct{})
	}
	if !g.audioStoreClosed {
		g.audioStoreClosed = true
		close(g.audioStoreCh)
	}
}

// audioDeps 返回 hosted 音频依赖与“已注入”信号。watcher 用它决定后端；
// 请求面不要用它读字段，沿用各自路径（启动期注入后只读）。
func (g *Gateway) audioDeps() (*PGAudioDeps, bool, <-chan struct{}) {
	g.pgAudioMu.Lock()
	defer g.pgAudioMu.Unlock()
	return g.pgAudio, g.audioStoreClosed, g.audioStoreCh
}

// SetFactStore 注入故事事实存储的工厂（启动期一次，postgres 模式）。
func (g *Gateway) SetFactStore(fn func(st library.Story, user *identity.User) (*store.Store, error)) {
	g.factStore = fn
}

// openFactStore 打开一个故事的事实存储：postgres 工厂优先，否则 file
// backend。PG 模式下绝不回退到 storyOutputDir。user 必须非 nil —— 请求面
// 一律带真实账号，内部调用走 openFactStoreInternal。
func (g *Gateway) openFactStore(st library.Story, user *identity.User) (*store.Store, error) {
	if g.factStore != nil {
		return g.factStore(st, user)
	}
	return store.NewStore(g.storyOutputDir(st)), nil
}

// openFactStoreInternal 是后台（watcher 等没有请求账号的路径）的入口；
// 语义与 openFactStore 相同，只是显式声明 actor 缺席。
func (g *Gateway) openFactStoreInternal(st library.Story) (*store.Store, error) {
	return g.openFactStore(st, nil)
}

// SetLibraryRepository 注入 hosted 的 thư viện repository（启动期一次，
// postgres 模式）。注入后 g.lib() 不再打开 --root 目录。
func (g *Gateway) SetLibraryRepository(repo library.Repository) {
	g.libRepo = repo
}

// SetWorkerStorage injects the storage contract handed to hosted engine
// workers (STORE_DRIVER=postgres): the env-scoped DSN a child process needs
// to open its own scoped store. nil keeps file behavior.
func (g *Gateway) SetWorkerStorage(cfg *bootstrap.StorageConfig) {
	g.workerStore = cfg
}

// RootPath 返回当前书库根（切换时持有读锁）。
func (g *Gateway) RootPath() string {
	g.rootMu.RLock()
	defer g.rootMu.RUnlock()
	return g.Root
}

func (g *Gateway) setRoot(root string) {
	g.rootMu.Lock()
	g.Root = root
	g.rootMu.Unlock()
}

// checkLatestRelease 是指向 version.CheckLatest 的测试接缝；生产路径始终用它。
var checkLatestRelease = version.CheckLatest

// NewGateway 创建网关；workerToken 每次启动随机生成，只在网关与本机 worker 之间使用。
// build 为 ldflags 构建身份（serve 传入 versionInfo()）；正式版本会在后台跑一次
// 被动检查新版本，结果经 /api/system 的 latestVersion/hasUpdate 暴露。
// 文件后端契约（独立 ainovel_session）；共享 Postgres 部署用 NewPostgresGateway。
func NewGateway(root string, accounts *identity.Store, build version.Info) *Gateway {
	return newGateway(root, accounts, backendFile, build)
}

// NewPostgresGateway is the constructor for the shared-account deployment:
// the caller must declare the backend explicitly — the cookie name and the
// app-grant gate are derived from it, never guessed from paths or env.
func NewPostgresGateway(root string, accounts *identity.Store, build version.Info) *Gateway {
	return newGateway(root, accounts, backendPostgres, build)
}

func newGateway(root string, accounts *identity.Store, backend accountBackend, build version.Info) *Gateway {
	g := &Gateway{Root: root, backend: backend, accounts: accounts, limiter: accounts.LoginLimiter(), recoverLimiter: accounts.RecoveryLimiter(recoverMaxFails, recoverWindow), workerToken: rand.Text(), workers: make(map[string]*storyWorker), launch: launchWorker, build: version.Resolve(build), audioStoreCh: make(chan struct{}), prepWake: make(chan struct{}, 1), prepActive: map[string]bool{}}
	g.launchHosted = g.launchHostedWorker
	accounts.SetProjectSource(func() ([]string, error) {
		if g.libRepo == nil && g.RootPath() == "" {
			return nil, identity.ErrProjectRootUnknown
		}
		lib, err := g.lib(context.Background(), nil)
		if err != nil {
			return nil, err
		}
		scan, err := lib.Scan(context.Background(), nil)
		if err != nil {
			return nil, err
		}
		ids := make([]string, 0, len(scan.Projects))
		for _, p := range scan.Projects {
			ids = append(ids, p.ID)
		}
		return ids, nil
	})
	// GAP-41：启动时还没有任何账号 → 生成设置码（只留哈希）；serve 打印一次明文。
	if has, err := accounts.HasUsers(); err == nil && !has {
		if state, code, err := identity.NewSetupState(time.Now); err == nil {
			g.setup, g.setupCode = state, code
		}
	}
	g.initNotifications()
	g.jobsHub().addProvider(newAudioJobsProvider(g))
	g.jobsHub().addProvider(newVoiceJobsProvider(g))
	g.resumeAudioQueues()
	if g.build.Version != "dev" {
		// 后台检查不阻塞启动；网络错误由 CheckLatest 静默吞掉。
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			res, err := checkLatestRelease(ctx, version.CheckOptions{
				CurrentVersion: g.build.Version,
				CacheDir:       bootstrap.DefaultConfigDir(),
			})
			if err != nil || res == nil {
				return
			}
			g.latestMu.Lock()
			g.latest = *res
			g.latestMu.Unlock()
		}()
	}
	return g
}

// latestRelease 返回最近一次被动检查的已知结果；未完成时为零值（无更新）。
func (g *Gateway) latestRelease() version.CheckResult {
	g.latestMu.Lock()
	defer g.latestMu.Unlock()
	return g.latest
}

func (g *Gateway) slot(id string) *storyWorker {
	g.mu.Lock()
	defer g.mu.Unlock()
	b := g.workers[id]
	if b == nil {
		b = &storyWorker{}
		g.workers[id] = b
	}
	return b
}

// resolveStoryID 把 "p/s" 解析为实际故事位置。
func (g *Gateway) resolveStoryID(id string) (library.Story, error) {
	p, s, ok := strings.Cut(id, "/")
	if !ok {
		return library.Story{}, os.ErrNotExist
	}
	lib, err := g.lib(context.Background(), nil)
	if err != nil {
		return library.Story{}, err
	}
	st, found := lib.ResolveStory(context.Background(), nil, p, s)
	if !found {
		return library.Story{}, os.ErrNotExist
	}
	return st, nil
}

// workerState 返回该故事的 worker 是否存活、引擎是否在跑；worker 已死时顺手清理。
func (g *Gateway) workerState(id string) (open, running bool) {
	b := g.slot(id)
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.worker != nil && workerDead(b.worker) {
		b.worker = nil
		b.running = false
	}
	if b.worker == nil {
		return false, false
	}
	return true, b.running
}

func workerDead(w *launchedWorker) bool {
	select {
	case <-w.done:
		return true
	default:
		return false
	}
}

func (g *Gateway) start(id string) (*launchedWorker, error) {
	st, err := g.resolveStoryID(id)
	if err != nil {
		return nil, err
	}
	return g.startDir(id, st.Dir)
}

// startDir 在 dir 内启动（或复用）key 对应的 worker；草稿共创复用同一套槽位管理。
func (g *Gateway) startDir(key, dir string) (*launchedWorker, error) {
	b := g.slot(key)
	for {
		b.mu.Lock()
		if b.deleted || b.deleting {
			b.mu.Unlock()
			return nil, os.ErrNotExist
		}
		if b.worker != nil && !workerDead(b.worker) {
			worker := b.worker
			b.mu.Unlock()
			return worker, nil
		}
		if pending := b.starting; pending != nil {
			b.mu.Unlock()
			<-pending
			continue
		}
		b.worker, b.running = nil, false
		b.starting = make(chan struct{})
		b.mu.Unlock()

		// Process startup may wait on the worker's readiness pipe. Readers of
		// the slot (including /api/library) must not wait on that I/O.
		var worker *launchedWorker
		var err error
		if dir == "" && g.factStore != nil {
			// Hosted story: no engine directory exists. The child gets the
			// env-scoped store and claims its own lease; the slot key carries
			// the "project/story" scope it resolves after connecting.
			project, story, ok := splitStoryKey(key)
			if strings.HasPrefix(key, "draft//") {
				project, story, ok = strings.Cut(strings.TrimPrefix(key, "draft//"), "/")
				story = "draft/" + story
			}
			if !ok {
				err = os.ErrNotExist
			} else {
				worker, err = g.launchHosted(key, project, story)
			}
		} else if info, statErr := os.Stat(dir); statErr != nil {
			err = statErr
		} else if !info.IsDir() {
			err = os.ErrNotExist
		} else {
			worker, err = g.launch(key, dir, g.RootPath(), g.workerToken)
		}
		b.mu.Lock()
		b.worker = worker
		close(b.starting)
		b.starting = nil
		b.mu.Unlock()
		if err != nil {
			return nil, err
		}
		if project, story, ok := splitStoryKey(key); ok {
			g.jobsHub().allowStory(project, story)
		}
		g.watchWorkerJobs(key, worker)
		return worker, nil
	}
}

func stopWorker(worker *launchedWorker) {
	_ = worker.stdin.Close()
	select {
	case <-worker.done:
	case <-time.After(10 * time.Second):
		_ = worker.kill()
		<-worker.done
	}
}

func (g *Gateway) closeBook(id string) {
	b := g.slot(id)
	for {
		b.mu.Lock()
		if pending := b.starting; pending != nil {
			b.mu.Unlock()
			<-pending
			continue
		}
		worker := b.worker
		b.worker, b.running = nil, false
		b.starting = make(chan struct{})
		b.mu.Unlock()
		if worker != nil {
			stopWorker(worker)
		}
		b.mu.Lock()
		close(b.starting)
		b.starting = nil
		b.mu.Unlock()
		return
	}
}

// dropSlot 停止并移除一个 worker 槽（草稿提交/取消后不再需要）。
func (g *Gateway) dropSlot(key string) {
	g.closeBook(key)
	g.mu.Lock()
	defer g.mu.Unlock()
	if b := g.workers[key]; b != nil {
		b.mu.Lock()
		idle := b.worker == nil
		b.mu.Unlock()
		if idle {
			delete(g.workers, key)
		}
	}
}

// workerRunning 直接问 worker 引擎是否在跑（不依赖最近一次 snapshot 代理）。
func (g *Gateway) workerRunning(worker *launchedWorker) bool {
	if worker == nil || workerDead(worker) {
		return false
	}
	status, body, err := g.workerCall(context.Background(), worker, http.MethodGet, "snapshot", nil)
	if err != nil || status != http.StatusOK {
		return false
	}
	var v struct {
		IsRunning bool `json:"isRunning"`
	}
	return json.Unmarshal(body, &v) == nil && v.IsRunning
}

// runningWorkers 返回当前有引擎在跑的全部槽位键。
func (g *Gateway) runningWorkers() []string {
	g.mu.Lock()
	keys := make([]string, 0, len(g.workers))
	for key := range g.workers {
		keys = append(keys, key)
	}
	g.mu.Unlock()
	out := []string{}
	for _, key := range keys {
		b := g.slot(key)
		b.mu.Lock()
		worker := b.worker
		if worker != nil && workerDead(worker) {
			b.worker, b.running = nil, false
		}
		known := b.running
		b.mu.Unlock()
		if worker == nil {
			continue
		}
		if known || g.workerRunning(worker) {
			out = append(out, key)
		}
	}
	return out
}

// workerCall 用内部令牌调用 worker 端点（path 不含 /api/），返回状态码与响应体。
func (g *Gateway) workerCall(ctx context.Context, worker *launchedWorker, method, path string, body any) (int, []byte, error) {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, "http://"+worker.addr+"/api/"+path, reader)
	if err != nil {
		return 0, nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("X-Ainovel-Worker-Token", g.workerToken)
	client := &http.Client{}
	if rt := g.workerTransport(); rt != nil {
		client.Transport = rt
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, data, nil
}

// workerError 从 worker 的 {"error": "..."} 响应里取消息，缺省用状态码文案。
func workerError(status int, body []byte) string {
	var v struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(body, &v) == nil && v.Error != "" {
		return v.Error
	}
	return fmt.Sprintf("worker trả về %d", status)
}

func (g *Gateway) closeWorkers() {
	g.mu.Lock()
	ids := make([]string, 0, len(g.workers))
	for id := range g.workers {
		ids = append(ids, id)
	}
	g.mu.Unlock()
	var wg sync.WaitGroup
	for _, id := range ids {
		wg.Add(1)
		go func() { defer wg.Done(); g.closeBook(id) }()
	}
	wg.Wait()
	g.closeAudioServices()
}

func (g *Gateway) Close() {
	g.closeOnce.Do(func() {
		g.stopWatcher()
		// Stop composing before the audio services/hub go away: the runner
		// publishes through the per-story service.
		g.stopAudioPreparation()
		g.closeWorkers()
		g.jobsHub().close()
		g.closeNotifications()
		g.events().close()
		g.closePGAudioServices()
	})
}

// projectUpload permits only the documented project and story multipart upload routes.
// cookie là SameSite=Strict nên ngoại lệ hẹp này không mở CSRF.
func projectUpload(r *http.Request) bool {
	if r.Method != "POST" {
		return false
	}
	typ, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || typ != "multipart/form-data" {
		return false
	}
	segs := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(segs) == 6 && segs[0] == "api" && segs[1] == "projects" && segs[2] != "" && segs[3] == "stories" && segs[4] != "" && segs[5] == "style-samples" {
		return true
	}
	return len(segs) == 4 && segs[0] == "api" && segs[1] == "projects" && segs[2] != "" && (segs[3] == "voices" || segs[3] == "assets")
}

// SetProxyConfig installs the trusted-proxy boundary parsed at startup. A nil
// config keeps the legacy loopback rule (CLI/offline unchanged).
func (g *Gateway) SetProxyConfig(cfg *webguard.Config) {
	g.proxy = cfg
	g.sec = secheaders.FromProxy(cfg)
}

func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 归一化必须在所有安全判断之前（限流键、Secure cookie、CSRF origin）。
	r = g.proxy.Normalize(r)
	// Header bảo mật đứng trước mọi nhánh: health, 401, tệp tĩnh và lỗi API đều mang header.
	g.sec.Apply(w)
	if g.sec.Robots(w, r) {
		return
	}
	// 健康探针先于静态路由：/healthz 绝不能落到 SPA 的 index.html。
	switch r.URL.Path {
	case "/healthz":
		g.healthz(w, r)
		return
	case "/readyz":
		g.readyz(w, r)
		return
	}
	if pairRedirect(w, r) {
		return
	}
	if g.proxy.CrossSiteRequest(r) {
		jsonError(w, http.StatusForbidden, "yêu cầu khác origin bị từ chối")
		return
	}
	if r.URL.Path == "/api" || !strings.HasPrefix(r.URL.Path, "/api/") {
		if r.URL.Path == "/api" {
			jsonError(w, 404, "không tìm thấy")
			return
		}
		g.static(w, r)
		return
	}
	if webguard.JSONBodyRequired(r) && !webguard.JSONContent(r) && !projectUpload(r) {
		jsonError(w, 400, "Content-Type phải là application/json")
		return
	}
	switch {
	case r.URL.Path == "/api/login" && r.Method == "POST":
		g.login(w, r)
		return
	case r.URL.Path == "/api/pair" && r.Method == "POST":
		g.pair(w, r)
		return
	case r.URL.Path == "/api/recover" && r.Method == "POST":
		g.recoverPassword(w, r)
		return
	case g.serveSetup(w, r):
		// GAP-41：/api/setup* 与 /api/login 同层，不需要登录（还没有账号时也要能进）。
		return
	}
	cookie, err := r.Cookie(g.cookiePolicy().Name)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, identity.EndedBody(identity.ErrUnauthenticated))
		return
	}
	user, current, err := g.accounts.Authenticate(cookie.Value)
	if err != nil {
		if errors.Is(err, identity.ErrUnauthenticated) {
			var ended *identity.SessionEnded
			if g.backend != backendPostgres || !errors.As(err, &ended) {
				g.clearSession(w, r)
			}
			writeJSON(w, http.StatusUnauthorized, identity.EndedBody(err))
			return
		}
		accountError(w, err)
		return
	}
	// App-grant gate (shared Postgres store): a valid session without a
	// Noverse grant gets 403 — never 401, never a cleared cookie — and only
	// the exact auth-neutral endpoints (logout, own password, own sessions)
	// still run. Everything else stops before feedback or any handler.
	if !appAccessGranted(user.Grants) {
		g.serveSessionOnly(w, r, user, current)
		return
	}
	// Order matters: the body may name the project (rules files), so it is
	// read first; authorization then runs before the version layer, because a
	// stale If-Match answers 409 with the current resource and a request that
	// is not allowed — forced password change, missing project role — must
	// stop first.
	res, ok := g.prepareFeedback(w, r, user)
	if !ok {
		return
	}
	if !authorizeRequest(w, r, user) {
		return
	}
	var finish func()
	var proceed bool
	w, finish, proceed = g.beginFeedback(w, r, user, current, res)
	if !proceed {
		return
	}
	defer finish()
	if g.serveNotify(w, r, user, current) {
		return
	}
	if g.serveAccount(w, r, user, current) {
		return
	}
	if g.serveRecovery(w, r, user) {
		return
	}
	if g.serveStorageMetrics(w, r, user) || g.serveLessons(w, r, user) {
		return
	}
	if g.serveConfig(w, r, user) {
		return
	}
	if r.URL.Path == "/api/events" && r.Method == http.MethodGet {
		g.serveEvents(w, r, user)
		return
	}
	if r.URL.Path == "/api/events/watch" && r.Method == http.MethodPost {
		g.serveEventsWatch(w, r, user)
		return
	}
	if g.serveJobs(w, r, user) {
		return
	}
	if g.serveCommands(w, r) {
		return
	}
	if g.libraryUnconfigured(w, r) {
		return
	}
	if g.serveReveal(w, r, user) {
		return
	}
	if g.serveFS(w, r, user) {
		return
	}
	if g.serveLibrary(w, r, user) {
		return
	}
	if g.serveComments(w, r, user) {
		return
	}
	if g.serveProjects(w, r, user) {
		return
	}
	if g.serveTTS(w, r, user) {
		return
	}
	if g.serveBlobDelivery(w, r, user) {
		return
	}
	jsonError(w, 404, "không tìm thấy")
}

func (g *Gateway) static(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		jsonError(w, 404, "không tìm thấy")
		return
	}
	path := strings.TrimPrefix(filepath.ToSlash(filepath.Clean("/"+r.URL.Path)), "/")
	if path == "" {
		path = "index.html"
	}
	f, err := fs.Sub(ui, "dist")
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	data, err := fs.ReadFile(f, path)
	if err != nil {
		// Tệp băm trong assets/ không có thì là 404 thật — trả index.html sẽ bị trình duyệt cache nhầm thành icon/script.
		if strings.HasPrefix(path, "assets/") {
			jsonError(w, 404, "không tìm thấy")
			return
		}
		path = "index.html"
		data, err = fs.ReadFile(f, path)
	}
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	if typ := mime.TypeByExtension(filepath.Ext(path)); typ != "" {
		w.Header().Set("Content-Type", typ)
	}
	if strings.HasPrefix(path, "assets/") {
		// Tên tệp có mã băm nội dung (Vite) → giữ vĩnh viễn; không có header này trình duyệt tải lại icon mỗi lần vẽ.
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		// index.html (và tệp gốc khác) phải kiểm lại để nhận bản mới sau khi cập nhật.
		w.Header().Set("Cache-Control", "no-cache")
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(data)
}

// proxyToWorker 把故事子路径转发给该故事的 worker 进程（内部路径 /api/<rest>）。
func (g *Gateway) proxyToWorker(w http.ResponseWriter, r *http.Request, id, rest string) {
	st, err := g.resolveStoryID(id)
	if err != nil {
		jsonError(w, 404, err.Error())
		return
	}
	g.proxyWorker(w, r, id, st.Dir, rest)
}

// proxyWorker 把 /api/<rest> 转发给 key 对应的 worker（dir 为其 cwd）。
func (g *Gateway) proxyWorker(w http.ResponseWriter, r *http.Request, key, dir, rest string) {
	worker, err := g.startDir(key, dir)
	if err != nil {
		switch {
		case errors.Is(err, errBookLocked):
			jsonError(w, http.StatusConflict, err.Error())
		case errors.Is(err, os.ErrNotExist):
			jsonMappedError(w, err)
		default:
			// Starting the story writer failed for an internal reason: log the
			// cause, never ship it (paths, ports, child process output).
			slog.Error("không mở được máy viết của truyện", "module", "web", "story_slug", key, "err", err)
			jsonError(w, http.StatusBadGateway, errWorkerUnavailableMessage)
		}
		return
	}
	target := &url.URL{Scheme: "http", Host: worker.addr}
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.FlushInterval = -1
	if rt := g.workerTransport(); rt != nil {
		proxy.Transport = rt
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, err error) {
		slog.Warn("máy viết của truyện không phản hồi", "module", "web", "story_slug", key, "err", err)
		jsonError(w, http.StatusBadGateway, errWorkerUnreachableMessage)
	}
	original := proxy.Director
	proxy.Director = func(req *http.Request) {
		original(req)
		req.URL.Path = "/api/" + rest
		req.Header.Set("X-Ainovel-Worker-Token", g.workerToken)
		req.Header.Del("Cookie")
	}
	if rest == "snapshot" {
		proxy.ModifyResponse = func(resp *http.Response) error {
			if resp.StatusCode != 200 {
				return nil
			}
			var v struct {
				IsRunning bool `json:"isRunning"`
			}
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			resp.Body.Close()
			if json.Unmarshal(body, &v) == nil {
				b := g.slot(key)
				b.mu.Lock()
				if b.worker == worker {
					b.running = v.IsRunning
				}
				b.mu.Unlock()
			}
			resp.Body = io.NopCloser(bytes.NewReader(body))
			resp.ContentLength = int64(len(body))
			return nil
		}
	}
	proxy.ServeHTTP(w, r)
}

var errBookLocked = errors.New("truyện đang bị tiến trình khác giữ")

// workerScopeEnv 把 "project/story" 范围传给 hosted 引擎子进程；子进程用
// 它 resolve 出自己的 scoped store 并申领 story lease。
const workerScopeEnv = "AINOVEL_STORY_SCOPE"

type tailBuffer struct {
	mu sync.Mutex
	s  string
}

func (t *tailBuffer) add(s string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.s += s
	if len(t.s) > 8192 {
		t.s = t.s[len(t.s)-8192:]
	}
}
func (t *tailBuffer) text() string { t.mu.Lock(); defer t.mu.Unlock(); return t.s }

// workerEnvPassthrough lists the only parent environment a worker child may
// inherit. Anything else — database DSNs, API keys the operator exported,
// migration credentials — is filtered out: the child must never receive an
// account credential through the environment by accident.
var workerEnvPassthrough = []string{
	"HOME", "PATH", "TMPDIR", "TMP", "TEMP", "USER", "USERNAME", "LOGNAME", "SHELL",
	"LANG", "LC_ALL", "LC_CTYPE", "TZ", "TERM",
	// Connectivity the engine needs for outbound LLM/media calls.
	"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY",
	"http_proxy", "https_proxy", "all_proxy", "no_proxy",
	"SSL_CERT_FILE", "SSL_CERT_DIR",
	// Runtime tuning, safe to inherit.
	"GOMAXPROCS", "GOTRACEBACK", "GODEBUG",
}

// workerEnv builds the minimal environment for a worker child: only the
// passthrough keys above plus the explicit additions the caller supplies.
// An empty value in extra is skipped rather than exported empty, so optional
// keys never shadow a meaningful default.
func workerEnv(extra ...string) []string {
	env := make([]string, 0, len(workerEnvPassthrough)+len(extra))
	for _, key := range workerEnvPassthrough {
		if value, ok := os.LookupEnv(key); ok {
			env = append(env, key+"="+value)
		}
	}
	for _, kv := range extra {
		if _, value, found := strings.Cut(kv, "="); found && value != "" {
			env = append(env, kv)
		}
	}
	return env
}

// launchWorker 在故事目录内启动 worker 子进程；dir 即引擎的书目录（worker cwd）。
// AINOVEL_LIBRARY_ROOT 让 worker 找到书库配置（自动导出等）。
func launchWorker(id, dir, root, token string) (*launchedWorker, error) {
	return spawnWorker(id, dir, "",
		workerEnv("AINOVEL_WORKER_TOKEN="+token, library.RootEnv+"="+root))
}

// launchHostedWorker 启动 hosted 故事的 worker 子进程：cwd 是一块私有
// scratch（不存在的故事目录绝不伪造），scope/存储经 env 传入——worker 只拿到
// NOVERSE_WORKER_DATABASE_URL 的受限角色，绝不拿 app DSN；secret 只在 env，不在 argv。
// 子进程连库后用 scope 自领 story lease。
func (g *Gateway) launchHostedWorker(key, project, story string) (*launchedWorker, error) {
	if g.workerStore == nil || g.workerStore.Driver != bootstrap.StoreDriverPostgres {
		return nil, fmt.Errorf("worker lưu trữ hosted chưa cấu hình: %w", errs.ErrStorageUnsupported)
	}
	env, err := hostedWorkerEnv(g.workerStore, g.workerToken, project, story)
	if err != nil {
		return nil, err
	}
	scratch, err := os.MkdirTemp("", "ainovel-worker-")
	if err != nil {
		return nil, err
	}
	worker, err := spawnWorker(key, scratch, scratch, env)
	if err != nil {
		_ = os.RemoveAll(scratch)
		return nil, err
	}
	return worker, nil
}

// hostedWorkerEnv builds the hosted child's environment from the parent
// storage contract. It fails closed when WorkerURL is unset — a hosted
// worker never inherits DATABASE_URL — and everything not on the passthrough
// list is dropped, so no account/migration/app DSN or operator secret can
// leak through the child's environment.
func hostedWorkerEnv(cfg *bootstrap.StorageConfig, token, project, story string) ([]string, error) {
	if cfg.WorkerURL == "" {
		return nil, fmt.Errorf("NOVERSE_WORKER_DATABASE_URL chưa cấu hình — worker hosted không bao giờ nhận DATABASE_URL của app: %w", errs.ErrConfig)
	}
	env := workerEnv(
		"AINOVEL_WORKER_TOKEN="+token,
		workerScopeEnv+"="+project+"/"+story,
		"STORE_DRIVER="+string(cfg.Driver),
		// The child contract stays single-keyed: its DATABASE_URL is the
		// scoped worker role, never the parent app DSN.
		"DATABASE_URL="+cfg.WorkerURL,
	)
	if cfg.MaxConns > 0 {
		env = append(env, "DB_MAX_CONNS="+strconv.FormatInt(int64(cfg.MaxConns), 10))
	}
	if cfg.ConnectTimeout > 0 {
		env = append(env, "DB_CONNECT_TIMEOUT="+cfg.ConnectTimeout.String())
	}
	return env, nil
}

// spawnWorker 建 __web-worker 子进程并等它就绪。dir 是 worker 的 cwd
// （文件模式=书目录；hosted=临时 scratch）；cleanupDir 非空时在进程退出后
// 删除该目录。存储与身份范围经 env 传入，绝不在 argv 放 secret。
func spawnWorker(id, dir, cleanupDir string, env []string) (*launchedWorker, error) {
	binary, err := os.Executable()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(binary, "__web-worker")
	cmd.Dir = dir
	cmd.Env = env
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err = cmd.Start(); err != nil {
		return nil, err
	}
	tail := &tailBuffer{}
	stderrDone := make(chan struct{})
	go func() {
		scanner := bufio.NewScanner(stderr)
		scanner.Buffer(make([]byte, 4096), 1<<20)
		for scanner.Scan() {
			line := scanner.Text()
			tail.add(line + "\n")
			fmt.Fprintf(os.Stderr, "[%s] %s\n", id, line)
		}
		close(stderrDone)
	}()
	done := make(chan struct{})
	go func() { _ = cmd.Wait(); <-stderrDone; close(done) }()
	type ready struct {
		addr    string
		err     error
		noModel bool
	}
	result := make(chan ready, 1)
	go func() {
		line, err := bufio.NewReader(stdout).ReadString('\n')
		if err != nil {
			result <- ready{err: err}
			return
		}
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "AINOVEL_WORKER_LISTEN ") {
			result <- ready{err: fmt.Errorf("worker trả lời sai giao thức: %s", line)}
			return
		}
		fields := strings.Fields(line)
		if len(fields) < 2 || len(fields) > 3 || (len(fields) == 3 && fields[2] != "readonly") {
			result <- ready{err: fmt.Errorf("worker trả lời sai giao thức")}
			return
		}
		addr := fields[1]
		host, _, err := net.SplitHostPort(addr)
		if err != nil || host != "127.0.0.1" {
			result <- ready{err: fmt.Errorf("địa chỉ worker không hợp lệ: %s", addr)}
			return
		}
		result <- ready{addr: addr, noModel: len(fields) == 3}
	}()
	select {
	case r := <-result:
		if r.err == nil {
			return &launchedWorker{addr: r.addr, stdin: stdin, done: done, kill: cmd.Process.Kill, noModel: r.noModel}, nil
		}
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		<-done
	case <-done:
		_ = stdin.Close()
	case <-time.After(60 * time.Second):
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		<-done
	}
	return nil, workerStartError(tail.text())
}

// workerStartError 把 worker 启动失败时的 stderr 尾部转成错误。Worker 与 gateway 是同一可执行文件，
// 按 host.ErrBookInUse 的文案识别目录占用：生产者改词时匹配随之同步。
func workerStartError(stderrTail string) error {
	msg := strings.TrimSpace(stderrTail)
	if msg == "" {
		msg = "worker không khởi động được"
	}
	if strings.Contains(msg, host.ErrBookInUse.Error()) || strings.Contains(msg, postgres.ErrLeaseLost.Error()) {
		return fmt.Errorf("%w: %s", errBookLocked, msg)
	}
	return errors.New(msg)
}
