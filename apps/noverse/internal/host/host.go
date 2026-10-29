package host

import (
	llmtypes "github.com/datnt1998/noverse/internal/llm"

	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/datnt1998/noverse/assets"
	"github.com/datnt1998/noverse/internal/agent"
	"github.com/datnt1998/noverse/internal/agents"
	"github.com/datnt1998/noverse/internal/agents/ctxpack"
	"github.com/datnt1998/noverse/internal/arbiter"
	"github.com/datnt1998/noverse/internal/bootstrap"
	"github.com/datnt1998/noverse/internal/comments"
	"github.com/datnt1998/noverse/internal/domain"
	"github.com/datnt1998/noverse/internal/flow"
	"github.com/datnt1998/noverse/internal/host/exp"
	"github.com/datnt1998/noverse/internal/host/imp"
	"github.com/datnt1998/noverse/internal/host/sim"
	"github.com/datnt1998/noverse/internal/llmretry"
	runtimelog "github.com/datnt1998/noverse/internal/logger"
	modelreg "github.com/datnt1998/noverse/internal/models"
	"github.com/datnt1998/noverse/internal/notify"
	"github.com/datnt1998/noverse/internal/revision"
	"github.com/datnt1998/noverse/internal/rules"
	storepkg "github.com/datnt1998/noverse/internal/store"
	"github.com/datnt1998/noverse/internal/tools"
	"github.com/datnt1998/noverse/internal/userrules"
)

// Host 是运行时外壳:生命周期/干预入口/事件投影/模型管理。
// 调度与执行在 engine(确定性循环);语义裁定在 arbiter(LLM-as-function)。
type Host struct {
	cfg                bootstrap.Config
	interventionPrompt string
	bundle             assets.Bundle
	packRules          rules.Candidate
	store              *storepkg.Store
	bookLease          StoryLease
	styleStats         *tools.StyleStatsIndex
	models             *bootstrap.ModelSet
	engine             *engine
	thinkingApplier    agents.ApplyThinking // /model 调推理强度时联动各 Worker
	writerRestore      *ctxpack.WriterRestorePack
	userRules          *userrules.Service
	observer           *observer
	usage              *UsageTracker
	usageCancel        context.CancelFunc  // 停掉 autoSaveLoop 并触发最后一次 flush
	budget             *BudgetSentinel     // 预算政策；未启用为 nil（方法 nil 安全）
	gate               *ChapterAdvanceGate // 章节许可与一次性暂停的统一政策组件
	notifier           *notify.Notifier    // 无人值守告警；未启用为 nil（Send nil 安全）
	configPath         string              // 配置写盘目标：/config、/model 就近写当前生效的那份（项目级存在则写它，否则全局）
	logCleanup         func()
	fileLogErr         error

	events   chan Event
	streamCh chan string
	done     chan struct{}

	mu         sync.Mutex
	lifecycle  lifecycle
	cocreating bool // 阶段共创占用：paused 窗口内堵住 import/simulate/continue 的并发介入
	// gateWaiting：最近一次暂停由章节推进闸门（逐章验收/advance hold）触发，
	// Snapshot 据此把 waitingFor 报成 "gate"（G5）。引擎重新开跑即清除。
	gateWaiting bool
	exclusive   string // 后台独占作业占用（导入/仿写/修订）：非空表示某作业在跑，堵住并发独占入口
	// exclusiveCancel 是当前独占作业的取消函数：预算硬停/手动暂停须能停掉正在烧钱的
	// 导入，而不仅是 Engine——abortWithEvent 在 Engine 未运行时取消它（预算哨兵的
	// abort 回调与手动 Abort 共用同一停机机制）。releaseExclusive 一并清空。
	exclusiveCancel context.CancelFunc
	closeOnce       sync.Once
	asyncWG         sync.WaitGroup
	closing         bool
	readOnly        bool

	// actionCancels: bounded host-side model calls of user actions (rules,
	// Arbiter); Abort cancels them before the engine loop exists.
	actionCancels map[*context.Context]context.CancelFunc

	interMu sync.Mutex // 干预裁定 FIFO 串行(同一时刻至多一次在途咨询)

	// sessionMu 保护引擎会话计数（G3）：emitEvent 会给每条事件打上当前会话 ID，
	// 单独用一把锁避免与 h.mu 的既有持锁路径嵌套。
	sessionMu  sync.RWMutex
	sessionSeq int
	sessionID  string

	// retryGate 是请求退避的可唤醒闸门（G4）：导入等直接模型调用在退避等待时
	// 注册为等待者，RetryNow 提前放行；无人等待时唤醒返回 false（409）。
	retryGate *llmretry.Gate

	outputMu     sync.RWMutex
	outputClosed bool

	// runCtx 约束宿主侧的 LLM 裁定调用(启动裁定/干预分诊);Close 取消,
	// 避免退出时仍有裁定在途且无法中断。
	runCtx    context.Context
	runCancel context.CancelFunc
}

type lifecycle string

const (
	lifecycleIdle      lifecycle = "idle"
	lifecycleRunning   lifecycle = "running"
	lifecyclePaused    lifecycle = "paused"
	lifecycleCompleted lifecycle = "completed"
)

// New 创建 Host。
func New(cfg bootstrap.Config, bundle assets.Bundle, options ...NewOption) (*Host, error) {
	cfg.FillDefaults()
	packRules, err := rules.PackDefaults(bundle.Pack)
	if err != nil {
		return nil, fmt.Errorf("đọc phép đo của gói: %w", err)
	}
	var opts newOptions
	for _, option := range options {
		if option != nil {
			option(&opts)
		}
	}
	injected := opts.storage
	if injected != nil {
		if err := injected.Validate(); err != nil {
			return nil, err
		}
	}
	if err := cfg.ValidateBase(); err != nil {
		if !opts.allowUnconfiguredReads {
			return nil, err
		}
		if injected != nil {
			return newReadHostWithStore(cfg, injected.Store, injected.Lease), nil
		}
		lease, leaseErr := acquireBookLease(cfg.OutputDir)
		if leaseErr != nil {
			return nil, leaseErr
		}
		return newReadHost(cfg, lease), nil
	}

	// Hosted runs receive their store and lease from the composition root: no
	// directory is created, locked or logged into (phase 06 engine cutover).
	var bookLease StoryLease
	if injected != nil {
		bookLease = injected.Lease
	} else {
		fileLease, leaseErr := acquireBookLease(cfg.OutputDir)
		if leaseErr != nil {
			return nil, leaseErr
		}
		bookLease = fileLease
	}
	keepBookLease := false
	var logCleanup func()
	defer func() {
		if keepBookLease {
			return
		}
		if err := closeLease(bookLease); err != nil {
			slog.Error("Nhả khoá thư mục truyện thất bại", "module", "host", "dir", cfg.OutputDir, "err", err)
		}
		if logCleanup != nil {
			logCleanup()
		}
	}()

	var fileLogErr error
	if opts.logFile != "" {
		if injected != nil {
			// The hosted process logs to stdout; a durable activity trail comes
			// from repository events, never from a file beside the engine.
			slog.Warn("Chế độ hosted không ghi log tệp; log đi ra stdout", "module", "host", "file", opts.logFile)
		} else {
			logCleanup, fileLogErr = runtimelog.SetupFile(cfg.OutputDir, opts.logFile, opts.logAlsoStderr, opts.logAttrs...)
			if fileLogErr != nil {
				logCleanup = nil
				slog.Warn("Log file không khả dụng, tiếp tục dùng log tiến trình hiện tại", "module", "host", "file", opts.logFile, "err", fileLogErr)
			}
		}
	}

	slog.Info("Khởi động", "module", "boot", "provider", cfg.Provider, "model", cfg.ModelName, "output", cfg.OutputDir)

	// 起后台 goroutine 从 OpenRouter 刷新模型元数据（窗口/价格），磁盘缓存 24h。
	modelreg.StartPricingRefresh(modelreg.DefaultRegistry(), bootstrap.DefaultConfigDir())

	store := storepkg.NewStore(cfg.OutputDir)
	if injected != nil {
		store = injected.Store
	}
	if err := store.Init(); err != nil {
		return nil, fmt.Errorf("khởi tạo kho dữ liệu: %w", err)
	}
	// RunMeta 是所有控制语义的事实源，必须在构造模型/后台任务之前完成校验。
	// 未知 advance mode 直接返回结构化错误；禁止猜测降级后继续写盘。
	// 新建的书（尚无 meta/run.json）按配置 advance_mode 初始化推进模式；
	// 已存在的书保留持久化模式，不被配置覆盖。
	freshBook := false
	if meta, err := store.RunMeta.Load(); err != nil {
		return nil, fmt.Errorf("đọc run meta: %w", err)
	} else {
		freshBook = meta == nil
	}
	if err := store.RunMeta.Init(cfg.Style, cfg.Provider, cfg.ModelName); err != nil {
		return nil, fmt.Errorf("khởi tạo run meta: %w", err)
	}
	if freshBook && cfg.AdvanceMode == "review" {
		if err := store.RunMeta.SetAdvanceMode(domain.ChapterAdvanceReview); err != nil {
			return nil, fmt.Errorf("khởi tạo chế độ đi tiếp: %w", err)
		}
	}

	// G4：agent 模型的开流前重试与导入重试共用同一闸门，Host.RetryNow 一并放行。
	retryGate := llmretry.NewGate()
	models, err := bootstrap.NewModelSet(cfg)
	if err != nil {
		if opts.allowUnconfiguredReads {
			var h *Host
			if injected != nil {
				h = newReadHostWithStore(cfg, store, bookLease)
			} else {
				h = newReadHostWithStore(cfg, storepkg.NewStore(cfg.OutputDir), bookLease)
			}
			h.logCleanup, h.fileLogErr = logCleanup, fileLogErr
			keepBookLease = true
			return h, nil
		}
		return nil, fmt.Errorf("khởi tạo model: %w", err)
	}
	models.SetRetryGate(retryGate)
	slog.Info("Model sẵn sàng", "module", "boot", "summary", models.Summary())

	usage := NewUsageTracker(models, store)
	// 优先读 meta/usage.json；以下情况都走 sessions/*.jsonl 一次性回填：
	//   - 文件不存在（首次持久化前）
	//   - schema 版本不匹配（未来升级后丢弃旧格式）
	//   - 文件存在但损坏 / IO 错误（不能让坏数据让累计永久归零）
	// 回填完立即 SaveNow，把结果固化下来，下次启动直接 Load 命中。
	loaded, loadErr := usage.LoadFromStore()
	if loadErr != nil {
		slog.Warn("Tải usage thất bại, sẽ thử backfill từ sessions", "module", "usage", "err", loadErr)
	}
	if !loaded {
		// Session replay is a file-backend backfill; hosted stores already serve
		// usage facts from rows and must not scan a directory.
		if dir, ok := store.FileRoot(); ok && dir != "" {
			if n, err := usage.ReplaySessions(dir); err != nil {
				slog.Warn("Replay usage thất bại", "module", "usage", "err", err)
			} else if n > 0 {
				slog.Info("Backfill usage từ session hoàn tất", "module", "usage", "messages", n)
				if err := usage.SaveNow(); err != nil {
					slog.Warn("Lưu sau khi backfill usage thất bại", "module", "usage", "err", err)
				}
			}
		}
	}
	usageCtx, usageCancel := context.WithCancel(context.Background())
	usage.StartAutoSave(usageCtx)

	// onGuardBlock 前置声明:h 构造后才能挂事件浮出闭包。
	var onGuardBlock func(agent, reason string, consecutive int32)
	styleStats := tools.NewStyleStatsIndex(store)
	if len(opts.recordToolResults) > 0 {
		store.Sessions.KeepToolResults(opts.recordToolResults...)
	}
	workers, restore, applyThinking := agents.BuildWorkers(cfg, store, styleStats, models, bundle, usage.Record,
		func(agent, reason string, consecutive int32) {
			if onGuardBlock != nil {
				onGuardBlock(agent, reason, consecutive)
			}
		}, packRules)
	store.Signals.ClearStaleSignals()

	h := &Host{
		cfg:             cfg,
		bundle:          bundle,
		packRules:       packRules,
		store:           store,
		bookLease:       bookLease,
		styleStats:      styleStats,
		models:          models,
		thinkingApplier: applyThinking,
		writerRestore:   restore,
		userRules:       userrules.NewService(store, models.Default, rules.DefaultOptions(), packRules),
		usage:           usage,
		usageCancel:     usageCancel,
		configPath:      bootstrap.EffectiveConfigPath(),
		logCleanup:      logCleanup,
		fileLogErr:      fileLogErr,
		events:          make(chan Event, 100),
		streamCh:        make(chan string, 256),
		done:            make(chan struct{}, 4),
		lifecycle:       lifecycleIdle,
		retryGate:       retryGate,
		// ask_user tắt (mặc định) → hợp đồng can thiệp không còn luật `ask`.
		interventionPrompt: askRulePrompt(bundle.Prompts.ArbiterIntervention, cfg.ArbiterAskUser()),
	}
	h.runCtx, h.runCancel = context.WithCancel(context.Background())
	h.observer = newObserver(store, h.emitEvent, h.emitDelta, h.emitClear, h.contextWindowForAgent)
	workers.SetEventObserver(h.observer.captureToolResult)
	// 宿主侧 Arbiter 与 Worker 共用同一条 ToolProgress → observer → 工作台链路。
	h.runCtx = agentcore.WithToolProgress(h.runCtx, h.observer.workerProgress)
	if cfg.Notify.IsEnabled() {
		h.notifier = notify.New(cfg.Notify.Command, cfg.Notify.Events)
	}
	// 预算哨兵:Engine 在每轮循环边界直接调用 HandleBoundary(不再经事件订阅)。
	if sentinel := NewBudgetSentinel(cfg.Budget,
		func() float64 { c, _, _, _, _ := usage.Totals(); return c },
		func(reason string) { h.abortWithEvent(reason, "error") },
		func(level, summary string) {
			h.emitEvent(Event{Time: time.Now(), Category: "SYSTEM", Summary: summary, Level: level})
			h.notifier.Send(notify.Notification{Kind: notify.KindBudget, Level: level, Title: "Noverse: ngân sách", Body: summary})
		},
	); sentinel != nil {
		h.budget = sentinel
		usage.SetOnCost(sentinel.OnCost)
		// 计费盲区告警：模型不报 usage 时成本恒 0，预算永不触发——保险丝没接上必须喊人。
		usage.SetOnMissingUsage(func() {
			const blind = "Không đo được ngân sách: dịch vụ mô hình không trả thông tin sử dụng nên chi phí đang được tính bằng 0 và giới hạn ngân sách chưa thể bảo vệ bạn. Hãy kiểm tra dịch vụ và bảng giá trong cài đặt trước khi tiếp tục."
			h.emitEvent(Event{Time: time.Now(), Category: "SYSTEM", Summary: blind, Level: "warn"})
			h.notifier.Send(notify.Notification{Kind: notify.KindBudget, Level: "warn", Title: "Noverse: ngân sách", Body: blind})
		})
	}
	// 统一前进闸门：执行一次性 hold，并阻止 review 模式下无许可的新章。
	h.gate = NewChapterAdvanceGate(store,
		func(reason string) {
			h.mu.Lock()
			h.gateWaiting = true
			h.mu.Unlock()
			h.abortWithEvent(reason, "info")
			h.notifier.Send(notify.Notification{Kind: notify.KindAdvanceGate, Level: "info", Title: "Noverse: chờ nghiệm thu", Body: reason})
		},
		func(level, summary string) {
			h.emitEvent(Event{Time: time.Now(), Category: "SYSTEM", Summary: summary, Level: level})
			h.notifier.Send(notify.Notification{Kind: notify.KindAdvanceGate, Level: level, Title: "Noverse: đẩy tiến chương", Body: summary})
		},
	)
	// StopGuard 拦截浮出：blocked 是高频自愈动作，只进屏内事件流（推送会刷屏）；
	// escalated / hard_stop 意味着本轮子任务报废，事件+notify 成对发出（架构 §2.3）。
	onGuardBlock = func(agent, reason string, n int32) {
		switch reason {
		case "escalated":
			body := fmt.Sprintf("%s quay rỗng %d lần liên tiếp mà không ghi ra đĩa các sản phẩm bắt buộc, nhiệm vụ lượt này chấm dứt, giao lại cho Engine xử lý", RoleLabel(agent), n)
			h.emitEvent(Event{Time: time.Now(), Category: "SYSTEM", Agent: agent, Summary: "Bộ chống lặp leo thang: " + body, Level: "warn"})
			h.notifier.Send(notify.Notification{Kind: notify.KindStopGuard, Level: "warn", Title: "Noverse: bộ chống lặp", Body: body})
		case "hard_stop":
			body := fmt.Sprintf("%s bị nhà cung cấp từ chối trả lời (chặn kiểm duyệt nội dung), nhiệm vụ lượt này kết thúc ngay", RoleLabel(agent))
			h.emitEvent(Event{Time: time.Now(), Category: "SYSTEM", Agent: agent, Summary: "Bộ chống lặp leo thang: " + body, Level: "warn"})
			h.notifier.Send(notify.Notification{Kind: notify.KindStopGuard, Level: "warn", Title: "Noverse: bộ chống lặp", Body: body})
		default: // blocked
			h.emitEvent(Event{Time: time.Now(), Category: "SYSTEM", Agent: agent,
				Summary: fmt.Sprintf("Bộ chống lặp: %s định kết thúc khi chưa tạo đủ sản phẩm bắt buộc, đã chặn lệnh thúc giục (lần thứ %d liên tiếp)", RoleLabel(agent), n), Level: "info"})
		}
	}
	// Engine:确定性执行引擎(docs/engine-rfc.md)。arbiter 用 Default 模型(过渡限制,
	// 见 engine-arbiter.md §4.2)。
	h.engine = &engine{
		store:           store,
		workers:         workers,
		arbiterModel:    newUsageTrackedModel(models.Default, "arbiter", usage.Record),
		failurePrompt:   bundle.Prompts.ArbiterFailure,
		planStartPrompt: bundle.Prompts.ArbiterPlanStart,
		style:           cfg.Style,
		// 同步重询:阻塞引擎循环一次裁定(数秒),换取"干预先于后续创作生效"。
		reconsult:       h.handleIntervention,
		resolveQuestion: h.resolveWorkerQuestion,
		observer:        h.observer,
		budget:          h.budget,
		gate:            h.gate,
		refresh:         h.refreshWriterRestore,
		emitEvent:       h.emitEvent,
		notify: func(kind, level, title, body string) {
			h.notifier.Send(notify.Notification{Kind: kind, Level: level, Title: title, Body: body})
		},
		onPause: func(summary string) { h.abortWithEvent(summary, "warn") },
		onDone:  h.runEnded,
	}
	// 27/10：ask_user tắt → thẻ “Đang chờ bạn” không bao giờ xuất hiện； cờ vừa
	// bị tắt trong khi còn câu hỏi treo thì dọn luôn trạng thái chờ khi mở truyện.
	if !h.cfg.ArbiterAskUser() {
		if meta, err := h.store.RunMeta.Load(); err == nil && meta != nil && meta.PendingAsk != nil {
			if err := h.store.RunMeta.ClearPendingAsk(*meta.PendingAsk); err != nil {
				h.emitEvent(Event{Time: time.Now(), Category: "ERROR", Level: "error",
					Summary: "Dọn câu hỏi đang chờ khi tắt hỏi khi phân vân thất bại: " + err.Error()})
			}
		}
	}

	keepBookLease = true
	return h, nil
}

// ── 生命周期 ──

// PrepareUserRules 在新建模式下生成本书用户规则快照（启动侧确定性，不进主创作 Run）。
//
// 入参是用户的**原始**创作要求（未经 BuildStartPrompt 包装）——归一化要的是用户规则本身，
// 不是启动脚手架。入口须在 StartPrepared 之前调用一次（quick/cocreate 两条新建路径都走这里）。
//
// 归一化失败只降级不报错（增强路径）；只有快照无法落盘才返回 error 中止开书——
// 后续运行将没有稳定事实源（见设计 §失败与降级）。
func (h *Host) PrepareUserRules(rawPrompt string) error {
	if err := h.refuseNewBookOverExisting(); err != nil {
		return err
	}
	// The user is waiting on this action: bound it and fail fast when the
	// provider is unreachable (normalization then degrades as designed).
	ctx, cancel := h.actionContext()
	defer cancel()
	svc := userrules.NewService(h.store, interactiveModel{ChatModel: h.models.Default}, rules.DefaultOptions(), h.packRules)
	snap, err := svc.Build(ctx, rawPrompt)
	if err != nil {
		return fmt.Errorf("Ghi snapshot quy tắc người dùng ra đĩa thất bại, không thể tiếp tục: %w", err)
	}
	logUserRulesSnapshot(snap)
	return nil
}

// ensureUserRules 在恢复路径确保快照存在；缺失时按
// system_defaults + rules 文件生成。
func (h *Host) ensureUserRules() {
	svc := userrules.NewService(h.store, h.models.Default, rules.DefaultOptions(), h.packRules)
	snap, err := svc.GetOrBuild(context.Background())
	if err != nil {
		slog.Warn("Đọc/tạo snapshot quy tắc người dùng thất bại, runtime sẽ lùi về mặc định tích hợp", "module", "rules", "err", err)
		return
	}
	logUserRulesSnapshot(snap)
}

// logUserRulesSnapshot 启动回显：让用户看到系统把规则理解成了什么（复用日志，不新增机制）。
func logUserRulesSnapshot(snap *rules.Snapshot) {
	if snap == nil {
		return
	}
	slog.Info("Snapshot quy tắc người dùng",
		"module", "rules",
		"status", string(snap.Status),
		"Nguồn", snap.Sources,
		"Cụm từ bị cấm", len(snap.Structured.ForbiddenPhrases),
		"Từ lặp", len(snap.Structured.FatigueWords),
	)
	if snap.Status == rules.StatusDegraded {
		slog.Warn("Một số quy tắc không phân tích được, đã chạy theo raw preferences (có thể tái tạo snapshot)",
			"module", "rules", "uncertain", snap.Uncertain)
	}
}

// StartPrepared 用用户的**原始**创作要求开始创作:plan_start 裁定选规划师并扩充
// 需求，裁定结果先固化为
// 事实(PlanStartRecord)再启动 Engine——恢复永远依赖已落盘事实,不重做已有裁定。
// 输入事实(StartPrompt)在裁定之前落盘:裁定失败时它是引擎补裁的依据,
// 启动失败可从任何恢复入口(Resume/继续)自愈,不是死局。
func (h *Host) StartPrepared(rawRequirement string) error {
	h.mu.Lock()
	if h.lifecycle == lifecycleRunning {
		h.mu.Unlock()
		return fmt.Errorf("Truyện đang chạy")
	}
	if h.cocreating {
		h.mu.Unlock()
		return fmt.Errorf("Đang đồng sáng tác theo giai đoạn, hãy kết thúc trước")
	}
	h.mu.Unlock()

	rawRequirement = strings.TrimSpace(rawRequirement)
	if rawRequirement == "" {
		return fmt.Errorf("Yêu cầu sáng tác không được để trống")
	}
	if err := h.refuseNewBookOverExisting(); err != nil {
		return err
	}
	if err := upgradeProject(h.store); err != nil {
		return err
	}
	if err := h.budget.Refuse(); err != nil {
		return err
	}
	if err := h.store.Checkpoints.Reset(); err != nil {
		return fmt.Errorf("đặt lại điểm kiểm soát: %w", err)
	}
	if err := h.store.Progress.Init(0); err != nil {
		return fmt.Errorf("khởi tạo tiến độ: %w", err)
	}
	// 输入事实先于裁定落盘:裁定失败(模型故障等)后 StartPrompt 仍在,
	// 恢复/继续时引擎据此补裁(planStartFallback),启动失败不再是死局。
	if err := h.store.RunMeta.SetStartPrompt(rawRequirement); err != nil {
		return fmt.Errorf("Ghi nhận nhu cầu sáng tác: %w", err)
	}

	// 启动裁定:失败显式报错中止(启动期用户在场,报错优于猜测)。
	start := time.Now()
	actionCtx, cancelAction := h.actionContext()
	decision, derr := runObservedDecision(h.observer, "Phán định khởi động", func() (arbiter.PlanStartDecision, error) {
		return arbiter.DecidePlanStart(actionCtx, h.actionArbiterModel(),
			h.bundle.Prompts.ArbiterPlanStart, rawRequirement, h.cfg.Style)
	})
	cancelAction()
	derr = userFacingProviderError(derr)
	rec := storepkg.DecisionRecord{Kind: "plan_start", Decider: "arbiter", Input: rawRequirement,
		Reason: decision.Reason, DurationMs: time.Since(start).Milliseconds()}
	if derr == nil {
		if data, err := json.Marshal(decision); err == nil {
			rec.Decision = data
		}
	} else {
		rec.Error = derr.Error()
	}
	var recErr error
	if rec, recErr = h.store.Decisions.Append(rec); recErr != nil {
		slog.Warn("Ghi audit phán định khởi động xuống đĩa thất bại", "module", "host", "err", recErr)
	}
	if derr != nil {
		return fmt.Errorf("Phán định khởi động thất bại: %w", derr)
	}
	if err := h.store.RunMeta.SetPlanStart(domain.PlanStartRecord{
		RawPrompt: rawRequirement, Planner: decision.Planner, PlannerTask: decision.Task, DecisionID: rec.ID,
	}); err != nil {
		return fmt.Errorf("Ghi lại phán định khởi động: %w", err)
	}

	h.emitEvent(Event{Time: time.Now(), Category: "SYSTEM",
		Summary: fmt.Sprintf("Bắt đầu sáng tác (%s — %s)", PlannerStartLabel(decision.Planner), decision.Reason), Level: "info"})
	if !h.startEngine(&flow.Instruction{Agent: decision.Planner, Task: decision.Task, Reason: decision.Reason}) {
		return fmt.Errorf("Engine đang chạy hoặc đang dừng, không thể khởi động truyện mới")
	}
	return nil
}

// refuseNewBookOverExisting 拒绝在已有成章的书目录里开新书：StartPrepared 会重置
// checkpoints 与 progress，误触即静默清掉整本书的进度链（导入完成后停在欢迎页
// 误按 Enter 是最典型场景）。只看已完成章数——规划阶段/启动失败的残留没有成章，
// 放行以保留共创 Ctrl+S 同会话重试与恢复补裁的自愈路径。
func (h *Host) refuseNewBookOverExisting() error {
	progress, err := h.store.Progress.Load()
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if progress == nil || len(progress.CompletedChapters) == 0 {
		return nil
	}
	book, err := h.store.Book.Load()
	if err != nil {
		return err
	}
	if book == nil {
		return fmt.Errorf("Thư mục đầu ra đã có chương, nhưng thông tin tác phẩm không tồn tại")
	}
	name := book.Title
	return fmt.Errorf("Thư mục đầu ra đã có tiến độ của «%s» (%d chương); tạo mới sẽ đặt lại tiến độ và checkpoint: muốn viết tiếp hãy khôi phục (mở lại ứng dụng sẽ tự khôi phục), truyện mới hãy đổi thư mục đầu ra",
		name, len(progress.CompletedChapters))
}

// startEngine 统一的引擎启动入口(Start/Resume/Continue/干预重启共用)。
// lifecycle 必须先于 goroutine 启动置为 running:引擎可能立即结束(完本/无路由),
// runEnded 会把 lifecycle 落到终态;若顺序颠倒,runEnded 先跑、这里再写 running,
// UI 将永远显示"运行中"而引擎实际已停。
func (h *Host) startEngine(initial *flow.Instruction) bool {
	// 跨重启门禁：存在未完成导入工作区时，禁止普通 Engine 消费半发布状态（RFC §12.5）。
	active, done, importErr := imp.ResumeStatus(h.store)
	if importErr != nil {
		h.emitEvent(Event{Time: time.Now(), Category: "ERROR", Level: "error",
			Summary: "Đọc trạng thái nhập thất bại, đã chặn sáng tác thường ghi đè lên artifact hiện có:" + importErr.Error()})
		return false
	}
	if active && !done {
		h.emitEvent(Event{Time: time.Now(), Category: "SYSTEM", Level: "warn",
			Summary: "Tồn tại một lượt nhập truyện từ bên ngoài chưa hoàn tất, vui lòng chạy /import để khôi phục hoàn tất rồi mới tiếp tục sáng tác"})
		return false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closing {
		return false
	}
	// 后台独占作业（导入/仿写）进行中时，引擎不得抢跑，避免与其写入竞争。这是所有引擎启动路径
	// （Resume/Continue 重启/自动接力/next）的统一 backstop——入口守卫是第一道，这里是最后一道。
	if h.exclusive != "" {
		return false
	}
	// lifecycle 可能已经是 paused，但旧 Engine goroutine 仍在执行退出 defer。
	// 必须同时核对 Engine 真状态；否则会把 lifecycle 改回 running，而 start
	// 实际 no-op，随后旧 runEnded 又把它落成 idle。
	if h.engine.isRunning() {
		return false
	}
	h.observer.setAborting(false)
	previous := h.lifecycle
	previousGate := h.gateWaiting
	h.lifecycle = lifecycleRunning
	h.gateWaiting = false
	if !h.engine.start(initial) {
		h.lifecycle = previous
		h.gateWaiting = previousGate
		return false
	}
	// 引擎真正开跑才算新会话（G3）：启动失败不得凭空多出一个会话。
	h.beginSession()
	return true
}

// RunInstructionOnce 在已落盘的书上只运行一条指令:派发给定 agent 执行一次
// task,其 worker 返回后引擎立刻按正常停机收尾(Done 照常触发)。供 eval 在
// 预置书上回放 editor/writer 的单步行为——不走 StartPrepared,不咨询 Arbiter。
// agent 仅接受 editor/writer;引擎已在运行或宿主正关闭时报错。
func (h *Host) RunInstructionOnce(agent, task string) error {
	if agent != "editor" && agent != "writer" {
		return fmt.Errorf("vai trò chỉ được là editor hoặc writer, nhận được %q", agent)
	}
	if strings.TrimSpace(task) == "" {
		return fmt.Errorf("nhiệm vụ không được để trống")
	}
	if h.engine == nil {
		return fmt.Errorf("engine chưa được khởi tạo")
	}
	// 与 startEngine 同规:未完成导入工作区存在时,普通派发不得消费半发布状态。
	active, done, importErr := imp.ResumeStatus(h.store)
	if importErr != nil {
		return fmt.Errorf("đọc trạng thái nhập thất bại: %w", importErr)
	}
	if active && !done {
		return fmt.Errorf("tồn tại lượt nhập truyện chưa hoàn tất, vui lòng chạy /import để khôi phục hoàn tất trước")
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closing {
		return fmt.Errorf("host đang đóng")
	}
	if h.exclusive != "" {
		return fmt.Errorf("đang có tác vụ độc quyền chạy nền (%s)", h.exclusive)
	}
	if h.engine.isRunning() {
		return fmt.Errorf("engine đang chạy, không thể chạy một lệnh riêng lẻ")
	}
	// lifecycle 必须先于 goroutine 置 running(同 startEngine):单指令 run 可能
	// 立即结束,顺序颠倒会让 runEnded 落终态后再被这里覆写成 running。
	h.observer.setAborting(false)
	previous := h.lifecycle
	previousGate := h.gateWaiting
	h.lifecycle = lifecycleRunning
	h.gateWaiting = false
	if !h.engine.startOnce(&flow.Instruction{Agent: agent, Task: task}) {
		h.lifecycle = previous
		h.gateWaiting = previousGate
		return fmt.Errorf("engine không thể khởi động lượt chạy đơn lệnh")
	}
	h.beginSession()
	return nil
}

// Reopen 把已完结的书强制重开为创作态。完本与重开都是重决策：完本可由架构师裁定，
// 重开只能由用户显式发起（/reopen），不经模型裁定。direction 非空时登记为待处理干预，
// 恢复时先经 Arbiter 裁定注入（与停机期干预同通道），再续跑引擎（卷末路由派发续卷）。
func (h *Host) Reopen(direction string) error {
	checks, err := h.ReopenPreconditions()
	if err != nil {
		return err
	}
	for _, check := range checks.Checks {
		if !check.OK {
			return fmt.Errorf("Không thể mở lại: %s. %s", check.Label, check.Note)
		}
	}

	if err := h.store.Progress.ReopenContinue(); err != nil {
		return err
	}
	// 重开是显式的用户动作，本身开启一个会话（G3）。
	h.beginSession()
	reopenEvent := Event{Time: time.Now(), Category: "SYSTEM", Summary: "Đã mở lại truyện để sáng tác tiếp (bạn hủy phán định hoàn thành)", Level: "info"}
	if d := strings.TrimSpace(direction); d != "" {
		reopenEvent.Detail = reopenEvent.Summary + "\nHướng viết tiếp: " + d
	}
	h.emitEvent(reopenEvent)
	if d := strings.TrimSpace(direction); d != "" {
		if err := h.store.RunMeta.SetPendingSteer(d); err != nil {
			return fmt.Errorf("Đã mở lại, nhưng đăng ký hướng viết tiếp thất bại: %v, vui lòng nhập lại hướng trực tiếp trong ô nhập", err)
		}
	}
	return nil
}

// Resume 恢复模式：从 checkpoint + progress 生成 resume prompt 并启动。
func (h *Host) Resume() (string, error) {
	h.mu.Lock()
	if h.lifecycle == lifecycleRunning {
		h.mu.Unlock()
		return "", fmt.Errorf("Truyện đang chạy")
	}
	if h.cocreating {
		h.mu.Unlock()
		return "", fmt.Errorf("Đang đồng sáng tác theo giai đoạn, hãy kết thúc trước")
	}
	if h.exclusive != "" {
		ex := h.exclusive
		h.mu.Unlock()
		return "", fmt.Errorf("Đang %s, hãy chờ xong rồi tiếp tục sáng tác", ex)
	}
	h.mu.Unlock()
	if err := upgradeProject(h.store); err != nil {
		return "", err
	}

	label, err := resumeLabel(h.store)
	if err != nil {
		return "", err
	}
	if label == "" {
		return "", nil // 新建模式，无恢复
	}
	if err := h.requireCleanChapters(); err != nil {
		return label, err
	}
	if err := h.budget.Refuse(); err != nil {
		return "", err
	}

	h.emitEvent(Event{Time: time.Now(), Category: "SYSTEM", Summary: "Tiếp tục sáng tác: " + label, Level: "info"})
	for _, w := range h.store.CheckConsistency() {
		h.emitEvent(Event{Time: time.Now(), Category: "SYSTEM", Summary: "Cảnh báo nhất quán: " + w, Level: "warn"})
	}
	// 确保用户规则快照存在；已有则廉价读取。
	h.ensureUserRules()
	h.refreshWriterRestore()
	// 待处理干预(停机期留下的/裁定期崩溃残留的)必须先于引擎续跑裁定——
	// 否则引擎可能抢在裁定前继续写出与干预相悖的章节。同步执行(阻塞数秒可接受,
	// UI 已显示"恢复创作");doIntervention 成功后自行清除 PendingSteer 并按
	// restart=true 拉起引擎。无待处理干预 → 直接续跑。
	meta, err := h.store.RunMeta.Load()
	if err != nil {
		return label, fmt.Errorf("Đọc can thiệp đang chờ xử lý: %w", err)
	}
	if meta != nil && meta.PendingSteer != "" {
		if err := h.doIntervention(meta.PendingSteer, true); err != nil {
			return label, err
		}
	} else {
		// 只恢复事实,不恢复会话(RFC §6):Engine 从 store 重算路由续跑。
		if !h.startEngine(nil) {
			return label, fmt.Errorf("Engine đang hoàn tất lượt dừng trước đó, vui lòng thử khôi phục lại sau")
		}
	}
	// lifecycle 由 startEngine / runEnded 管理,此处不再覆写——
	// 引擎立即结束(完本等)时覆写会把终态改回 running。
	return label, nil
}

// handleIntervention 适配 Engine 的无返回值重询回调；错误已由 doIntervention 发出事件。
func (h *Host) handleIntervention(text string) {
	_ = h.doIntervention(text, false)
}

// doIntervention 是用户干预的统一裁定路径:Collect → Decide → 执行。
// FIFO 串行(同一时刻至多一次在途咨询);answer/rules 即时执行,控制态动作
// (hold/reopen/dispatch)引擎运行中排队边界提交、停机时立即执行。
// restart=true(Continue 语义)时干预处理完确保引擎运行。
func (h *Host) doIntervention(text string, restart bool) error {
	if strings.HasPrefix(text, workerQuestionPrefix) {
		return h.resolveWorkerQuestion(text)
	}
	return h.intervene(text, restart, h.interventionPrompt)
}

func (h *Host) intervene(text string, restart bool, prompt string) error {
	h.interMu.Lock()
	defer h.interMu.Unlock()

	// 崩溃保护:裁定前先持久化(PendingSteer),成功应用或已当面回显失败后原子清除
	// (ClearHandledSteer 同时复位 FlowSteering)。裁定期间崩溃 → 下次 Resume 重放。
	if err := h.store.RunMeta.SetPendingSteer(text); err != nil {
		wrapped := fmt.Errorf("Ghi lưu can thiệp thất bại, đã dừng phán định: %w", err)
		h.emitEvent(Event{Time: time.Now(), Category: "ERROR", Agent: "arbiter",
			Summary: wrapped.Error(), Detail: wrapped.Error(), Level: "error"})
		return wrapped
	}
	clearPending := func() error {
		if err := h.store.ClearHandledSteer(); err != nil {
			return fmt.Errorf("Xoá can thiệp đã xử lý thất bại: %w", err)
		}
		return nil
	}

	facts, err := arbiter.CollectInterventionFacts(h.store)
	if err != nil {
		wrapped := fmt.Errorf("Thu thập dữ kiện can thiệp thất bại, chưa gọi Arbiter: %w", err)
		h.emitEvent(Event{Time: time.Now(), Category: "ERROR", Agent: "arbiter",
			Summary: wrapped.Error(), Detail: wrapped.Error(), Level: "error"})
		return wrapped
	}
	facts.Running = h.engine.isRunning()
	facts.AskUserEnabled = h.cfg.ArbiterAskUser()
	facts.WorkerQuestion = strings.HasPrefix(text, workerQuestionPrefix)

	start := time.Now()
	actionCtx, cancelAction := h.actionContext()
	defer cancelAction()
	// 27/10：autoPicks 是 ask_user tắt 时服务器自答的轨迹；空切片表示无自答。
	type interventionOutcome struct {
		decision  arbiter.InterventionDecision
		autoPicks []arbiter.AskAutoPick
	}
	outcome, derr := runObservedDecision(h.observer, "Phán định can thiệp", func() (interventionOutcome, error) {
		d, picks, err := arbiter.DecideIntervention(actionCtx, h.actionArbiterModel(),
			prompt, facts, text)
		return interventionOutcome{decision: d, autoPicks: picks}, err
	})
	decision := outcome.decision
	derr = userFacingProviderError(derr)

	rec := storepkg.DecisionRecord{Kind: "intervention", Decider: "arbiter", Input: text,
		Reason: decision.Reason, DurationMs: time.Since(start).Milliseconds()}
	if cp := h.store.Checkpoints.LatestGlobal(); cp != nil {
		rec.CheckpointSeq = cp.Seq
	}
	if data, err := json.Marshal(facts); err == nil {
		rec.Facts = data
	}
	if derr == nil {
		if data, err := json.Marshal(decision); err == nil {
			rec.Decision = data
		}
	} else {
		rec.Error = derr.Error()
	}
	if _, err := h.store.Decisions.Append(rec); err != nil {
		wrapped := fmt.Errorf("Ghi audit phán định can thiệp xuống đĩa thất bại, từ chối thực thi hành động: %w", err)
		h.emitEvent(Event{Time: time.Now(), Category: "ERROR", Agent: "arbiter",
			Summary: wrapped.Error(), Detail: wrapped.Error(), Level: "error"})
		return wrapped
	}

	if derr != nil {
		// 宁可不动,不可误动:不产生任何写入。调用错误与
		// 输出校验错误共用同一 error 通道,必须原样回显,不得统一伪装成"未能理解"。
		// 已当面告知 → 清除 pending(否则下次 Resume 会自动重放同一条失败干预)。
		h.emitEvent(newInterventionFailureEvent(derr))
		if err := clearPending(); err != nil {
			return fmt.Errorf("%v; %w", derr, err)
		}
		return derr
	}

	h.emitEvent(Event{Time: time.Now(), Category: "SYSTEM", Agent: "arbiter", Kind: "internal", Summary: "Phán định: " + decision.Reason, Level: "info"})
	if decision.Answer != "" {
		h.emitEvent(Event{Time: time.Now(), Category: "SYSTEM", Summary: decision.Answer, Level: "info"})
	}
	// 27/10：ask_user tắt 时服务器替用户挑了 is_default 项——留一行 nhật ký
	// 让用户知道 Trọng tài 为何没停着等回答。
	for _, pick := range outcome.autoPicks {
		h.emitEvent(Event{Time: time.Now(), Category: "SYSTEM", Agent: "arbiter",
			Summary: fmt.Sprintf("Trọng tài tự chọn: %s (vì bạn đã tắt Hỏi khi phân vân)", pick.Choice), Level: "info"})
	}
	// 任一动作持久化失败 → 保留 PendingSteer(恢复时整条重放重新裁定;
	// hold/reopen 幂等、dispatch 经新事实重询,重放安全)。
	var actionErr error
	if decision.Rules != "" {
		if snap, _, err := h.userRules.AddRuntimeRule(actionCtx, decision.Rules); err != nil {
			h.emitEvent(Event{Time: time.Now(), Category: "ERROR", Summary: "Ghi quy tắc viết xuống đĩa thất bại: " + err.Error(), Level: "error"})
			actionErr = err
		} else if snap != nil {
			h.emitEvent(Event{Time: time.Now(), Category: "SYSTEM", Summary: "Quy tắc viết đã cập nhật và lưu lại", Level: "info"})
		}
	}

	if decision.Hold != nil || decision.Reopen != nil || decision.Ask != nil || decision.Dispatch != nil {
		op := controlOp{hold: decision.Hold, reopen: decision.Reopen, ask: decision.Ask, dispatch: decision.Dispatch, text: text, facts: facts}
		if !h.engine.enqueue(op) {
			// 引擎未运行:立即执行;持久化失败 → 保留 PendingSteer,恢复时重放整条干预。
			if err := h.engine.applyControlOp(context.Background(), op); err != nil {
				h.emitEvent(Event{Time: time.Now(), Category: "SYSTEM", Level: "warn",
					Summary: "Thực thi hành động can thiệp thất bại, đã giữ lại; sẽ tự thử lại khi khôi phục/tiếp tục"})
				return err
			}
			// reopen/dispatch 表达了继续创作的意图,拉起引擎。
			if decision.Reopen != nil || decision.Dispatch != nil {
				restart = true
			}
		}
	}
	if actionErr != nil {
		// 保留 PendingSteer:恢复/继续时整条重放重新裁定。
		h.emitEvent(Event{Time: time.Now(), Category: "SYSTEM", Level: "warn",
			Summary: "Một số hành động can thiệp chưa thành công, can thiệp đã được giữ lại; tự thử lại khi khôi phục/tiếp tục"})
		return actionErr
	}
	// 动作已成功应用/入队,清除崩溃保护(入队后引擎侧失败或退出竞态由 engine
	// 回存 PendingSteer 兜底)。
	if err := clearPending(); err != nil {
		h.emitEvent(Event{Time: time.Now(), Category: "ERROR", Level: "error", Summary: err.Error()})
		return err
	}
	// 本轮干预被采纳即视为已回答在途提问：结清 PendingAsk（按 Collect 时刻读到的
	// 同一提问对账，防误删）。新 ask 会在 controlOp 应用时再登记，顺序后于结清。
	if facts.PendingAsk != nil && decision.Ask == nil {
		if err := h.store.RunMeta.ClearPendingAsk(*facts.PendingAsk); err != nil {
			h.emitEvent(Event{Time: time.Now(), Category: "ERROR", Level: "error",
				Summary: "Kết thúc câu hỏi đang chờ thất bại: " + err.Error()})
			return fmt.Errorf("Kết thúc câu hỏi đang chờ thất bại: %w", err)
		}
	}
	// ask 意味着引擎应停着等回答：即使 Continue 拉起语义也不许立即续跑。
	if decision.Ask != nil {
		restart = false
	}

	if restart && !h.engine.isRunning() {
		if err := h.budget.Refuse(); err != nil {
			h.emitEvent(Event{Time: time.Now(), Category: "SYSTEM", Summary: err.Error(), Level: "warn"})
			return err
		}
		h.refreshWriterRestore()
		if !h.startEngine(nil) {
			// 此时干预动作已生效并清除 PendingSteer，只是引擎未能立即拉起——不能谎称"已保存"。
			h.emitEvent(Event{Time: time.Now(), Category: "SYSTEM", Level: "warn",
				Summary: "Can thiệp đã có hiệu lực, nhưng Engine chưa thể chạy tiếp ngay; vui lòng tiếp tục trong ô nhập lúc khác hoặc khởi động lại ứng dụng để khôi phục"})
			return fmt.Errorf("Can thiệp đã có hiệu lực, nhưng Engine chưa thể chạy tiếp ngay")
		}
	}
	return nil
}

func newInterventionFailureEvent(err error) Event {
	detail := err.Error()
	return Event{
		Time:     time.Now(),
		Category: "ERROR",
		Agent:    "arbiter",
		Summary:  "Phán định can thiệp thất bại: " + detail + " (chưa thay đổi gì)",
		Detail:   detail,
		Kind:     errorKind(err, detail),
		Level:    "error",
	}
}

// arbiterModel 返回带用量追踪的裁定模型(token/成本进预算与 usage 系统)。
func (h *Host) arbiterModel() llmtypes.ChatModel {
	return newUsageTrackedModel(h.models.Default, "arbiter", h.usage.Record)
}

// actionArbiterModel is the Arbiter model for decisions a user waits on:
// unreachable providers fail immediately instead of retrying until timeout.
func (h *Host) actionArbiterModel() llmtypes.ChatModel {
	return interactiveModel{ChatModel: h.arbiterModel()}
}

// Continue 停机后用户在输入框输入时调用:干预裁定 + 确保引擎重新运行。
func (h *Host) Continue(text, username string) error {
	return h.continueWriting(text, username, true)
}

// Internal co-create instructions are not a second user action.
func (h *Host) continueWriting(text, username string, recordInput bool) error {
	rawText := text
	text = strings.TrimSpace(text)
	if text == "" {
		return fmt.Errorf("Nội dung không được để trống")
	}
	h.mu.Lock()
	if h.cocreating {
		h.mu.Unlock()
		return fmt.Errorf("Đang đồng sáng tác theo giai đoạn, hãy kết thúc trước")
	}
	if h.exclusive != "" {
		ex := h.exclusive
		h.mu.Unlock()
		// 独占作业期间必须在裁定前挡住：否则 Arbiter 已改 PendingSteer/规则/控制态，引擎才被门禁拦下。
		return fmt.Errorf("Đang %s, hãy chờ xong rồi tiếp tục sáng tác", ex)
	}
	h.mu.Unlock()
	if err := h.requireCleanChapters(); err != nil {
		return err
	}
	if err := h.budget.Refuse(); err != nil {
		return err
	}

	err, launched := h.runAsync(func() error {
		if recordInput {
			h.RecordUserEvent(username, "continue", rawText)
		}
		return h.doIntervention(text, true)
	})
	if !launched {
		return fmt.Errorf("Host đang đóng, không thể tiếp tục sáng tác")
	}
	return err
}

// SetAdvanceMode 确定性切换章节推进模式。它只写入用户运行意图，
// 不调用 Arbiter，也不隐式启动已经暂停的 Engine。
func (h *Host) SetAdvanceMode(mode domain.ChapterAdvanceMode) error {
	h.interMu.Lock()
	defer h.interMu.Unlock()
	if err := h.store.RunMeta.SetAdvanceMode(mode); err != nil {
		return err
	}
	label := "Tự động"
	if mode == domain.ChapterAdvanceReview {
		label = "Nghiệm thu từng chương"
	}
	summary := "Đã chuyển cách đẩy chương: " + label
	h.mu.Lock()
	state := h.lifecycle
	h.mu.Unlock()
	if mode == domain.ChapterAdvanceAuto && state != lifecycleRunning && state != lifecycleCompleted {
		summary += "; sáng tác đang tạm dừng, bấm Tiếp tục để chạy lại"
	}
	h.emitEvent(Event{Time: time.Now(), Category: "SYSTEM", Summary: summary, Level: "info"})
	return nil
}

// AdvanceOneChapter 在逐章验收模式下授权一个精确章节并启动 Engine。
func (h *Host) AdvanceOneChapter() error {
	return h.advanceOneChapter(nil)
}

func (h *Host) advanceOneChapter(review *domain.CommentReview) error {
	h.interMu.Lock()
	defer h.interMu.Unlock()

	h.mu.Lock()
	running, cocreating, ex := h.lifecycle == lifecycleRunning, h.cocreating, h.exclusive
	h.mu.Unlock()
	if running || h.engine.isRunning() {
		return fmt.Errorf("Việc sáng tác vẫn đang chạy hoặc đang hoàn tất tạm dừng, hãy thử lại /next sau")
	}
	if cocreating {
		return fmt.Errorf("Đang đồng sáng tác theo giai đoạn, hãy kết thúc trước")
	}
	if ex != "" {
		return fmt.Errorf("Đang %s, hãy chờ xong rồi chạy /next", ex)
	}
	if err := h.requireCleanChapters(); err != nil {
		return err
	}
	meta, err := h.store.RunMeta.Load()
	if err != nil {
		return err
	}
	if meta == nil {
		return fmt.Errorf("RunMeta chưa khởi tạo")
	}
	if meta.AdvanceMode != domain.ChapterAdvanceReview {
		return fmt.Errorf("/next chỉ dùng cho chế độ nghiệm thu từng chương, hãy chạy /review on trước")
	}
	if meta.AdvanceHold != nil {
		return fmt.Errorf("Vẫn còn ý định tạm dừng một lần đang chờ xử lý (%s), hãy khôi phục hoặc hoàn tất can thiệp hiện tại trước", meta.AdvanceHold.Reason)
	}
	if err := h.budget.Refuse(); err != nil {
		return err
	}
	progress, err := h.store.Progress.Load()
	if err != nil {
		return err
	}
	if progress == nil || progress.Phase != domain.PhaseWriting {
		return fmt.Errorf("Truyện chưa ở giai đoạn sáng tác nên chưa thể duyệt chương mới")
	}
	target := progress.NextChapter()
	if target <= 0 {
		return fmt.Errorf("Không thể suy ra chương tiếp theo từ tiến độ hiện tại")
	}
	if review != nil {
		completed := false
		for _, chapter := range progress.CompletedChapters {
			if chapter == review.Chapter {
				completed = true
				break
			}
		}
		if !completed {
			return fmt.Errorf("Chương %d chưa hoàn thành để áp dụng góp ý", review.Chapter)
		}
		if err := h.store.RunMeta.GrantCommentReview(target, *review); err != nil {
			return err
		}
	} else if err := h.store.RunMeta.GrantAdvancePermit(target); err != nil {
		return err
	}
	h.emitEvent(Event{Time: time.Now(), Category: "SYSTEM",
		Summary: fmt.Sprintf("Đã duyệt chương %d; sau khi nộp chương này sẽ hoàn tất thẩm duyệt và cập nhật cấu trúc cung/tập cần thiết, rồi lại chờ duyệt", target), Level: "info"})
	h.refreshWriterRestore()
	if !h.startEngine(nil) {
		// 许可按章节号持久化且同目标幂等，调用方稍后重试不会重复授权。
		return fmt.Errorf("Giấy phép chương đã được lưu, nhưng Engine vẫn đang hoàn tất lượt dừng trước; hãy thử lại /next sau")
	}
	return nil
}

// Steer 提交用户干预（运行中随时可用；停机时裁定后视动作决定是否拉起引擎）。
// TUI 通过 tea.Cmd 等待结果，因此能收到真实裁定/持久化错误而不会阻塞界面。
func (h *Host) Steer(text, username string) error {
	err, launched := h.runAsync(func() error {
		h.RecordUserEvent(username, "steer", text)
		return h.doIntervention(text, false)
	})
	if !launched {
		return fmt.Errorf("Host đang đóng, không thể gửi can thiệp")
	}
	return err
}

// Abort 暂停当前引擎循环。
func (h *Host) Abort() bool {
	return h.abortWithEvent("Bạn đã tạm dừng sáng tác", "warn")
}

// abortWithEvent 以指定原因事件执行暂停。预算停机与手动暂停共用同一停机机制，
// 仅事件文案不同（预算停机=用户预先签署的 Abort 指令，语义等同手动暂停）。
func (h *Host) abortWithEvent(summary, level string) bool {
	h.mu.Lock()
	running := h.lifecycle == lifecycleRunning
	if running {
		h.lifecycle = lifecyclePaused
	}
	cancelExclusive := h.exclusiveCancel
	h.mu.Unlock()
	actionsCancelled := h.cancelActions()
	if running {
		// 置位必须在 engine.abort 之前：cancel 传播会立刻引发 stream init / worker
		// 失败事件，observer 凭此标志识别为 abort 衍生噪声并抑制。
		h.observer.setAborting(true)
		h.engine.abort()
		h.emitEvent(Event{Time: time.Now(), Category: "SYSTEM", Summary: summary, Level: level})
		return true
	}
	// Engine 未运行但独占作业（导入等）在跑：它同样在烧钱，预算硬停/手动暂停必须
	// 能停掉它——否则预算政策对导入形同虚设（docs/import-pipeline.md §13.1）。
	if cancelExclusive != nil {
		cancelExclusive()
		h.emitEvent(Event{Time: time.Now(), Category: "SYSTEM", Summary: summary, Level: level})
		return true
	}
	if actionsCancelled {
		h.emitEvent(Event{Time: time.Now(), Category: "SYSTEM", Summary: summary, Level: level})
		return true
	}
	return false
}

// Close 终止引擎并关闭事件通道。
//
// Usage 持久化语义：先取消 autoSaveLoop（它自行 flush 最后一次 dirty 状态），
// 再补一次同步 SaveNow 收尾。终止后 in-flight LLM 调用的最末几百 token
// 丢失由下次启动时 session jsonl replay 自动补回。
func (h *Host) Close() {
	h.closeOnce.Do(func() {
		h.mu.Lock()
		h.closing = true
		cancelExclusive := h.exclusiveCancel
		h.mu.Unlock()
		if h.readOnly {
			h.closeOutputChannels()
			_ = closeLease(h.bookLease)
			if h.logCleanup != nil {
				h.logCleanup()
				h.logCleanup = nil
			}
			return
		}

		h.observer.setAborting(true)
		if h.runCancel != nil {
			h.runCancel() // 中断在途的宿主侧裁定调用与 supervisor 转发
		}
		if cancelExclusive != nil {
			cancelExclusive()
		}
		h.engine.abort()
		h.engine.wait()
		h.asyncWG.Wait()

		if h.usageCancel != nil {
			h.usageCancel()
			h.usageCancel = nil
		}
		h.usage.WaitAutoSave()
		if err := h.usage.SaveNow(); err != nil {
			slog.Warn("usage ghi xuống đĩa thất bại trước khi thoát", "module", "usage", "err", err)
		}
		h.closeOutputChannels()
		if err := closeLease(h.bookLease); err != nil {
			slog.Error("Nhả khoá thư mục truyện thất bại", "module", "host", "dir", h.cfg.OutputDir, "err", err)
		}
		if h.logCleanup != nil {
			h.logCleanup()
			h.logCleanup = nil
		}
	})
}

// FileLogError 返回构造阶段的文件日志初始化错误；Host 生命周期内不会变化。
func (h *Host) FileLogError() error {
	return h.fileLogErr
}

// runEnded 引擎循环结束(任何原因)时由 engine.onDone 回调:按 store 事实定终态。
//   - Phase=Complete  → 标记 completed，发"创作完成"事件
//   - 其它            → 标记 idle/paused，发"创作停止"事件
func (h *Host) runEnded() {
	h.observer.finalize()

	h.mu.Lock()
	progress, err := h.store.Progress.Load()
	if err != nil {
		if h.lifecycle == lifecycleRunning {
			h.lifecycle = lifecycleIdle
		}
		h.mu.Unlock()
		h.emitEvent(Event{Time: time.Now(), Category: "ERROR", Level: "error",
			Summary: "Đọc tiến độ khi engine kết thúc thất bại: " + err.Error()})
		select {
		case h.done <- struct{}{}:
		default:
		}
		return
	}
	book, err := h.store.Book.Load()
	if err != nil {
		h.lifecycle = lifecycleIdle
		h.mu.Unlock()
		h.emitEvent(Event{Time: time.Now(), Category: "ERROR", Level: "error",
			Summary: "Đọc thông tin tác phẩm khi engine kết thúc thất bại: " + err.Error()})
		select {
		case h.done <- struct{}{}:
		default:
		}
		return
	}
	if progress != nil && progress.Phase == domain.PhaseComplete {
		if book == nil {
			h.lifecycle = lifecycleIdle
			h.mu.Unlock()
			h.emitEvent(Event{Time: time.Now(), Category: "ERROR", Level: "error",
				Summary: "Thông tin tác phẩm không tồn tại khi engine kết thúc"})
			select {
			case h.done <- struct{}{}:
			default:
			}
			return
		}
		h.lifecycle = lifecycleCompleted
		// 完本收尾:确定性生成(store 已有全部事实,不花 LLM 调用;RFC 末节)。
		summary := completionSummary(*progress, *book)
		h.mu.Unlock()
		h.emitEvent(Event{Time: time.Now(), Category: "SYSTEM", Summary: summary, Level: "success"})
		h.notifier.Send(notify.Notification{
			Kind: notify.KindRunEnd, Level: "info", Title: "Noverse: sáng tác hoàn tất",
			Body: h.runEndBody("", summary),
		})
	} else {
		wasRunning := h.lifecycle == lifecycleRunning
		if wasRunning {
			h.lifecycle = lifecycleIdle
		}
		completed := 0
		title := ""
		if progress != nil {
			completed = len(progress.CompletedChapters)
		}
		if book != nil {
			title = book.Title
		}
		h.mu.Unlock()
		if wasRunning {
			summary := fmt.Sprintf("Engine dừng (đã xong %d chương)", completed)
			h.emitEvent(Event{Time: time.Now(), Category: "SYSTEM", Summary: summary, Level: "warn"})
			h.notifier.Send(notify.Notification{
				Kind: notify.KindRunEnd, Level: "warn", Title: "Noverse: sáng tác đã dừng",
				Body: h.runEndBody(title, summary),
			})
		}
	}

	select {
	case h.done <- struct{}{}:
	default:
	}
}

// runEndBody 组装 run_end 通知正文：书名 + 进度摘要 + 累计花费。
func (h *Host) runEndBody(title, summary string) string {
	if name := strings.TrimSpace(title); name != "" {
		summary = "«" + name + "»" + summary
	}
	cost, _, _, _, _ := h.usage.Totals()
	if cost > 0 {
		summary += fmt.Sprintf(" · chi phí $%.2f", cost)
	}
	return summary
}

// ── 通道 ──

// StreamClearSentinel 通过 streamCh 单条发送以示意"清空当前流式 round"。
// 不再用独立 clearCh —— 双通道无序导致 ✻ header 时常落到上一个 round 末尾。
const StreamClearSentinel = "\x00\x00CLEAR\x00\x00"

func (h *Host) Events() <-chan Event  { return h.events }
func (h *Host) Stream() <-chan string { return h.streamCh }
func (h *Host) Done() <-chan struct{} { return h.done }
func (h *Host) Dir() string           { return h.store.Dir() }

// CommentStore exposes the backend-bound comment lifecycle (file rows under
// the story directory, scoped rows on hosted) so worker endpoints never
// derive a directory from Dir() == "".
func (h *Host) CommentStore() *comments.Store {
	if h.store == nil {
		return nil
	}
	return h.store.Comments()
}

// ── 事件发射 ──

func (h *Host) emitEvent(ev Event) {
	if ev.ID == "" {
		ev.ID = nextEventID()
	}
	if ev.SessionID == "" {
		ev.SessionID = h.currentSessionID()
	}
	ev = ContextualizeEvent(ev, nil)
	if ev.Scope == "arc" && ev.ChapterFrom == 0 && h.store != nil && h.store.Outline != nil {
		volumes, _ := h.store.Outline.LoadLayeredOutline()
		ev = ContextualizeEvent(ev, volumes)
	}
	if ev.Chapter == 0 && ev.Tool != "novel_context" && h.store != nil && h.store.Progress != nil {
		if progress, err := h.store.Progress.Load(); err == nil && progress != nil {
			ev.Chapter = progress.CurrentChapter
		}
	}
	if ev.Category == "USER" && ev.Username == "" {
		ev.Username = os.Getenv("USER")
		ev.Agent = "user"
	}
	h.outputMu.RLock()
	defer h.outputMu.RUnlock()
	if h.outputClosed {
		return
	}
	// 读锁保证关闭前的事件完整写完；关闭后的事件直接拒绝。
	LogEvent(ev)
	select {
	case h.events <- ev:
	default:
		select {
		case <-h.events:
		default:
		}
		select {
		case h.events <- ev:
		default:
		}
	}
}

// beginSession 开启新的引擎会话并返回其 ID（G3）。每次引擎开跑
// （start/resume/continue/reopen/import）调用一次；emitEvent 统一打上当前会话 ID。
func (h *Host) beginSession() string {
	h.sessionMu.Lock()
	defer h.sessionMu.Unlock()
	h.sessionSeq++
	h.sessionID = fmt.Sprintf("s%d", h.sessionSeq)
	return h.sessionID
}

// currentSessionID 返回当前会话 ID；尚未开启任何会话时为空串。
func (h *Host) currentSessionID() string {
	h.sessionMu.RLock()
	defer h.sessionMu.RUnlock()
	return h.sessionID
}

// RetryNow 立即放行正在退避等待的模型请求（G4）。没有等待者时返回 false，
// 上层据此回 409「Không có lượt nào đang chờ thử lại」。
func (h *Host) RetryNow() bool {
	return h.retryGate != nil && h.retryGate.Wake()
}

func (h *Host) emitDelta(delta string) {
	h.outputMu.RLock()
	defer h.outputMu.RUnlock()
	if h.outputClosed {
		return
	}
	select {
	case h.streamCh <- delta:
	default:
		select {
		case <-h.streamCh:
		default:
		}
		select {
		case h.streamCh <- delta:
		default:
		}
	}
}

func (h *Host) closeOutputChannels() {
	h.outputMu.Lock()
	defer h.outputMu.Unlock()
	if h.outputClosed {
		return
	}
	h.outputClosed = true
	close(h.done)
	close(h.events)
	close(h.streamCh)
}

func (h *Host) emitClear() {
	// 通过 streamCh 走"sentinel"，保证与 emitDelta 在同一条通道里有序送达 TUI。
	h.emitDelta(StreamClearSentinel)
}

// ── Snapshot (TUI 状态聚合) ──

func (h *Host) Snapshot() UISnapshot {
	h.mu.Lock()
	state := h.lifecycle
	cocreating := h.cocreating
	gateWaiting := h.gateWaiting
	exclusive := h.exclusive != ""
	provider, model := h.cfg.Provider, h.cfg.ModelName
	if h.models != nil {
		provider, model, _ = h.models.CurrentSelection("default")
	}
	modelWindow, _ := h.cfg.ResolveContextWindow(provider, model)
	thinkingLevel := h.cfg.ResolveReasoningEffort("default")
	style := h.cfg.Style
	h.mu.Unlock()

	// 动态解析当前模型的上下文窗口，/model 或 /config 切换后下一次 Snapshot 自动反映。
	cost, tokIn, tokOut, cacheRead, cacheWrite := h.usage.Totals()
	saved := h.usage.SavedUSD()
	overallCapable := h.usage.OverallCacheCapable()
	recentRead, recentInput, recentSamples := h.usage.OverallRecent()
	perAgent := h.usage.PerAgent()
	cacheStats := make([]AgentCacheStat, 0, len(perAgent))
	for _, a := range perAgent {
		cacheStats = append(cacheStats, AgentCacheStat{
			Role:            a.Role,
			Input:           a.Input,
			Output:          a.Output,
			CacheRead:       a.CacheRead,
			CacheWrite:      a.CacheWrite,
			Cost:            a.Cost,
			Saved:           a.Saved,
			CacheCapable:    a.CacheCapable,
			RecentCacheRead: a.RecentCacheRead,
			RecentInput:     a.RecentInput,
			RecentSamples:   a.RecentSamples,
		})
	}
	perModel := h.usage.PerModel()
	modelStats := make([]AgentCacheStat, 0, len(perModel))
	for _, a := range perModel {
		modelStats = append(modelStats, AgentCacheStat{
			Model:        a.Model,
			Input:        a.Input,
			Output:       a.Output,
			CacheRead:    a.CacheRead,
			CacheWrite:   a.CacheWrite,
			Cost:         a.Cost,
			Saved:        a.Saved,
			CacheCapable: a.CacheCapable,
		})
	}

	snap := UISnapshot{
		Provider:               provider,
		ModelName:              model,
		ModelContextWindow:     modelWindow,
		ThinkingLevel:          thinkingLevel,
		Style:                  style,
		RuntimeState:           string(state),
		IsRunning:              state == lifecycleRunning,
		TotalInputTokens:       tokIn,
		TotalOutputTokens:      tokOut,
		TotalCacheReadTokens:   cacheRead,
		TotalCacheWriteTokens:  cacheWrite,
		TotalCostUSD:           cost,
		TotalSavedUSD:          saved,
		BudgetLimitUSD:         h.budget.Limit(),
		OverallCacheCapable:    overallCapable,
		OverallRecentCacheRead: recentRead,
		OverallRecentInput:     recentInput,
		OverallRecentSamples:   recentSamples,
		TotalCacheBreaks:       h.usage.OverallCacheBreaks(),
		CachePerAgent:          cacheStats,
		CachePerModel:          modelStats,
		MissingAssistantUsage:  h.usage.MissingAssistantUsage(),
	}

	// 单指令运行结果:engine 存在时才可能有;正常 run 恒为 nil。
	if h.engine != nil {
		snap.OnceResult = h.engine.onceResultSnapshot()
	}

	if book, _ := h.store.Book.Load(); book != nil {
		snap.BookTitle = book.Title
		snap.Synopsis = truncate(book.Synopsis, 200)
	}
	progress, _ := h.store.Progress.Load()
	if progress != nil {
		snap.Phase = string(progress.Phase)
		snap.Flow = string(progress.Flow)
		snap.CurrentChapter = progress.CurrentChapter
		snap.TotalChapters = progress.TotalChapters
		snap.CompletedCount = len(progress.CompletedChapters)
		snap.TotalWordCount = progress.TotalWordCount
		snap.InProgressChapter = progress.InProgressChapter
		snap.PendingRewrites = progress.PendingRewrites
		snap.RewriteReason = progress.RewriteReason
		snap.Layered = progress.Layered
		if progress.CurrentVolume > 0 {
			snap.CurrentVolumeArc = fmt.Sprintf("Tập %d · Cung %d", progress.CurrentVolume, progress.CurrentArc)
		}
	}
	if meta, _ := h.store.RunMeta.Load(); meta != nil {
		snap.PendingSteer = meta.PendingSteer
		snap.AdvanceMode = string(meta.AdvanceMode)
		snap.AdvancePermitChapter = meta.AdvancePermitChapter
		if meta.AdvanceHold != nil {
			snap.HasAdvanceHold = true
			snap.AdvanceHoldReason = meta.AdvanceHold.Reason
			snap.AdvanceHoldChapter = meta.AdvanceHold.TargetChapter
		}
		// ask_user tắt không chiếu PendingAsk ra snapshot：thẻ “Đang chờ bạn”
		// không bao giờ xuất hiện khi công tắc tắt (27/10)。NewHost 已清扫，
		// 这里再兜底一次防运行期标志/状态错位。
		if meta.PendingAsk != nil && h.cfg.ArbiterAskUser() {
			ask := &PendingAskSnapshot{Question: meta.PendingAsk.Question, Chapter: meta.PendingAsk.Chapter}
			for _, o := range meta.PendingAsk.Options {
				ask.Options = append(ask.Options, AskOptionItem{Label: o.Label, Reply: o.Reply, IsDefault: o.IsDefault})
			}
			snap.PendingAsk = ask
		}
	}

	snap.Agents = h.observer.agentSnapshots()
	snap.StatusLabel = deriveStatusLabel(snap)
	snap.WaitingFor = deriveWaitingFor(state, snap, cocreating, gateWaiting, exclusive)

	// 恢复标签
	if label, err := resumeLabel(h.store); err == nil && label != "" {
		snap.RecoveryLabel = label
	}

	h.fillDetails(&snap, progress)

	return snap
}

// fillDetails 填充详情区:设定、角色、最近 commit/review/摘要。
func (h *Host) fillDetails(snap *UISnapshot, progress *domain.Progress) {
	if premise, _ := h.store.Outline.LoadPremise(); premise != "" {
		snap.Premise = truncate(premise, 80)
	}
	if outline, _ := h.store.Outline.LoadOutline(); len(outline) > 0 {
		completed := make(map[int]struct{})
		if progress != nil {
			completed = make(map[int]struct{}, len(progress.CompletedChapters))
			for _, chapter := range progress.CompletedChapters {
				completed[chapter] = struct{}{}
			}
		}
		for _, e := range outline {
			title := e.Title
			if _, ok := completed[e.Chapter]; ok {
				committedTitle, err := h.store.Summaries.LoadSummaryTitle(e.Chapter)
				if err != nil {
					slog.Warn("Chiếu tiêu đề chương thất bại", "module", "host.snapshot", "chapter", e.Chapter, "err", err)
				} else if strings.TrimSpace(committedTitle) != "" {
					title = committedTitle
				}
			}
			snap.Outline = append(snap.Outline, OutlineSnapshot{
				Chapter: e.Chapter, Title: title, CoreEvent: e.CoreEvent,
			})
		}
	}
	h.fillPlan(snap, progress)
	if progress != nil && progress.Layered {
		if compass, _ := h.store.Outline.LoadCompass(); compass != nil {
			snap.CompassDirection = compass.EndingDirection
			snap.CompassScale = compass.EstimatedScale
		}
		if volumes, _ := h.store.Outline.LoadLayeredOutline(); len(volumes) > 0 {
			for _, v := range volumes {
				if v.Index > progress.CurrentVolume {
					snap.NextVolumeTitle = v.Title
					break
				}
			}
		}
	}
	if chars, _ := h.store.Characters.Load(); len(chars) > 0 {
		for _, c := range chars {
			label := c.Name
			if c.Role != "" {
				label += " (" + c.Role + ")"
			}
			snap.Characters = append(snap.Characters, label)
		}
	}
	if ledger, _ := h.store.Cast.Load(); len(ledger) > 0 {
		snap.SupportingCount = len(ledger)
		recent, _ := h.store.Cast.RecentActive(5)
		for _, e := range recent {
			label := e.Name
			if e.BriefRole != "" {
				label += " (" + e.BriefRole + ")"
			}
			snap.RecentSupporting = append(snap.RecentSupporting, label)
		}
	}
	if progress != nil && len(progress.CompletedChapters) > 0 {
		lastCh := progress.CompletedChapters[len(progress.CompletedChapters)-1]
		wc := progress.ChapterWordCounts[lastCh]
		snap.LastCommitSummary = fmt.Sprintf("Chương %d · %s chữ", lastCh, formatCount(wc))
	}
	currentCh := 1
	if progress != nil && len(progress.CompletedChapters) > 0 {
		currentCh = progress.CompletedChapters[len(progress.CompletedChapters)-1]
	}
	if review, err := h.store.World.LoadLastReview(currentCh); err == nil && review != nil {
		snap.LastReviewSummary = fmt.Sprintf("%s · %d vấn đề", verdictLabel(review.Verdict), len(review.Issues))
		if len(review.AffectedChapters) > 0 {
			snap.LastReviewSummary += " · ảnh hưởng chương " + joinInts(review.AffectedChapters, ", ")
		}
	}
	if cp := h.store.Checkpoints.LatestGlobal(); cp != nil {
		snap.LastCheckpointName = fmt.Sprintf("%s.%s", cp.Scope, cp.Step)
	}
	if progress != nil {
		for i := len(progress.CompletedChapters) - 1; i >= 0 && len(snap.RecentSummaries) < 2; i-- {
			ch := progress.CompletedChapters[i]
			if summary, err := h.store.Summaries.LoadSummary(ch); err == nil && summary != nil {
				snap.RecentSummaries = append(snap.RecentSummaries,
					fmt.Sprintf("Chương %d: %s", ch, truncate(summary.Summary, 50)))
			}
		}
	}
}

// formatCount 按 vi-VN 千位分隔（8766 → "8.766"），与 web 端 Intl.NumberFormat('vi-VN') 一致。
func formatCount(n int) string {
	s := strconv.Itoa(n)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	var b strings.Builder
	if neg {
		b.WriteByte('-')
	}
	for i := range len(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// verdictLabel 把审阅 verdict 枚举转成界面文案；未知值原样返回。与 web/src/i18n/vi.ts verdict 表一致。
func verdictLabel(verdict string) string {
	switch verdict {
	case "accept":
		return "Đạt"
	case "polish", "revise":
		return "Cần sửa"
	case "rewrite":
		return "Viết lại"
	case "reject":
		return "Không đạt"
	}
	return verdict
}

func joinInts(values []int, sep string) string {
	parts := make([]string, len(values))
	for i, v := range values {
		parts[i] = strconv.Itoa(v)
	}
	return strings.Join(parts, sep)
}

// deriveWaitingFor 计算工作台状态芯片的等待原因（G5）：
// 共创占用 → cocreate；运行中或独占作业（导入/仿写/同步）进行中 → ""（芯片显示"Đang chạy"）；
// 完本 → ""；章节推进闸门（逐章验收 / advance hold）→ gate；其余空闲或暂停 → input。
// 导入待确认（confirm）由 worker 依据任务事件叠加，Host 无从得知。
func deriveWaitingFor(state lifecycle, snap UISnapshot, cocreating, gateWaiting, exclusive bool) string {
	switch {
	case cocreating:
		return "cocreate"
	case snap.IsRunning || exclusive:
		return ""
	case state == lifecycleCompleted:
		return ""
	case gateWaiting || snap.HasAdvanceHold:
		return "gate"
	default:
		return "input"
	}
}

func deriveStatusLabel(s UISnapshot) string {
	switch {
	case s.Phase == string(domain.PhaseComplete):
		return "COMPLETE"
	case s.Flow == string(domain.FlowReviewing):
		return "REVIEW"
	case s.Flow == string(domain.FlowRewriting) || s.Flow == string(domain.FlowPolishing):
		return "REWRITE"
	case s.RuntimeState == "running":
		return "RUNNING"
	default:
		return "READY"
	}
}

// statusLabelText is the Vietnamese text for each StatusLabel code; the TUI and the web jobs dock share it.
var statusLabelText = map[string]string{
	"READY":    "Sẵn sàng",
	"RUNNING":  "Đang chạy",
	"REVIEW":   "Thẩm duyệt",
	"REWRITE":  "Viết lại",
	"COMPLETE": "Hoàn thành",
	"PAUSED":   "Tạm dừng",
	"PAUSING":  "Đang tạm dừng",
	"ERROR":    "Lỗi",
}

// StatusLabelText returns the user-facing text for a StatusLabel code; empty → "Sẵn sàng".
// An unknown code is returned lowercased so an English code is never shown as-is.
func StatusLabelText(code string) string {
	if code == "" {
		code = "READY"
	}
	if text, ok := statusLabelText[code]; ok {
		return text
	}
	return strings.ToLower(code)
}

// ── 模型管理 ──

func (h *Host) ConfiguredProviders() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	providers := make([]string, 0, len(h.cfg.Providers))
	for name := range h.cfg.Providers {
		providers = append(providers, name)
	}
	sort.Strings(providers)
	return providers
}

func (h *Host) CurrentModelSelection(role string) (string, string, bool) {
	if h.models == nil {
		return h.cfg.Provider, h.cfg.ModelName, false
	}
	return h.models.CurrentSelection(role)
}

func (h *Host) SwitchModel(role, provider, model string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if provider == "" || model == "" {
		return fmt.Errorf("Cần chọn cả nhà cung cấp và model")
	}
	if err := h.models.Swap(role, provider, model); err != nil {
		return err
	}
	if role == "" || role == "default" {
		h.cfg.Provider = provider
		h.cfg.ModelName = model
	} else {
		if h.cfg.Roles == nil {
			h.cfg.Roles = make(map[string]bootstrap.RoleConfig)
		}
		rc := h.cfg.Roles[role]
		rc.Provider = provider
		rc.Model = model
		h.cfg.Roles[role] = rc
	}
	// 换模型不改动已存的推理强度意图：只在下发时按新模型能力钳制。
	if h.configPath != "" {
		if err := bootstrap.SaveConfig(h.configPath, h.cfg); err != nil {
			slog.Warn("Lưu cấu hình thất bại", "module", "host", "err", err)
		}
	}
	h.applyThinkingLocked(role)
	// 切到未登记模型时打一行 warn，提示用户走了 128k 兜底——长篇容易被提前压缩。
	logRole := role
	if logRole == "" {
		logRole = "default"
	}
	window, source := h.cfg.ResolveContextWindow(provider, model)
	bootstrap.LogContextWindowChoice(logRole, model, window, source)

	// 无常驻上下文需要联动:writer/architect/editor 的 ContextManager 走
	// ContextManagerFactory,下次 spawn 自动按新模型窗口重建。

	h.emitEvent(Event{
		Time:     time.Now(),
		Category: "SYSTEM",
		Summary:  fmt.Sprintf("Đã đổi mô hình cho %s → %s/%s", RoleLabel(role), provider, model),
		Level:    "info",
	})
	return nil
}

// concreteThinkingRoles 是可应用推理强度的具体角色（与 agents.ApplyThinking 路由一致）。
// 调 default 时按各角色 ResolveReasoningEffort 逐个重新应用。
var concreteThinkingRoles = []string{"architect", "writer", "editor"}

// CurrentThinking 返回某角色当前生效的推理强度原始串（供 /model 面板同步当前值）。
func (h *Host) CurrentThinking(role string) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.cfg.ResolveReasoningEffort(strings.ToLower(strings.TrimSpace(role)))
}

func (h *Host) AvailableThinking(role string) []llmtypes.ThinkingLevel {
	h.mu.Lock()
	if h.models == nil {
		h.mu.Unlock()
		return nil
	}
	model := h.models.ForRole(strings.ToLower(strings.TrimSpace(role)))
	h.mu.Unlock()
	return agents.AvailableThinkingForModel(model)
}

// resolveThinkingForRoleLocked 计算某角色实际生效的推理强度：取其原始意图
// （ResolveReasoningEffort：角色级 → 顶层默认），再按该角色当前模型的能力钳制。
// 钳制只发生在这条“生效路径”上，不回写配置——存储始终保留用户的原始意图。
func (h *Host) resolveThinkingForRoleLocked(role string) llmtypes.ThinkingLevel {
	parsed, _ := agents.ParseThinkingLevel(h.cfg.ResolveReasoningEffort(role))
	resolved, _ := agents.ResolveThinkingForModel(h.models.ForRole(role), parsed)
	return resolved
}

// applyThinkingLocked 把生效强度下发给 live agent；每个角色各按自己的模型钳制。
func (h *Host) applyThinkingLocked(role string) {
	if h.thinkingApplier == nil {
		return
	}
	role = strings.ToLower(strings.TrimSpace(role))
	if role == "" || role == "default" {
		for _, r := range concreteThinkingRoles {
			h.thinkingApplier(r, h.resolveThinkingForRoleLocked(r))
		}
		return
	}
	h.thinkingApplier(role, h.resolveThinkingForRoleLocked(role))
}

// SetRoleThinking 设置某角色（或 default）的推理强度：校验→持久化→联动 live agent→事件。
// 镜像 SwitchModel 的结构；与模型选择正交，可单独调整。level 为空 = 不覆盖（继承）。
func (h *Host) SetRoleThinking(role, level string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	parsed, err := agents.ParseThinkingLevel(level)
	if err != nil {
		return err
	}
	role = strings.ToLower(strings.TrimSpace(role))
	// 存储保留原始意图：直接持久化用户选定的强度，钳制只在下发(applyThinkingLocked)时按模型能力发生。
	if role == "" || role == "default" {
		h.cfg.ReasoningEffort = string(parsed)
	} else {
		if h.cfg.Roles == nil {
			h.cfg.Roles = make(map[string]bootstrap.RoleConfig)
		}
		rc := h.cfg.Roles[role]
		rc.ReasoningEffort = string(parsed)
		h.cfg.Roles[role] = rc
	}
	if h.configPath != "" {
		if err := bootstrap.SaveConfig(h.configPath, h.cfg); err != nil {
			slog.Warn("Lưu cấu hình thất bại", "module", "host", "err", err)
		}
	}

	// 联动 live：具体角色直接应用；default 则遍历各具体角色按 ResolveReasoningEffort 重新应用
	// （已被角色级覆盖的保留自身，未覆盖的吃上新默认）。
	h.applyThinkingLocked(role)

	logRole := role
	if logRole == "" {
		logRole = "default"
	}
	h.emitEvent(Event{
		Time:     time.Now(),
		Category: "SYSTEM",
		Summary:  fmt.Sprintf("Đã đổi mức suy luận: %s → %s", RoleLabel(logRole), ThinkingLabel(string(parsed))),
		Level:    "info",
	})
	return nil
}

// ── 事件回放 ──

func (h *Host) ReplayQueue(afterSeq int64) ([]domain.RuntimeQueueItem, error) {
	if h.store == nil || h.store.Runtime == nil {
		return nil, nil
	}
	return h.store.Runtime.LoadQueueAfter(afterSeq)
}

// ── 共创 ──

// CoCreateStream 冷启动共创：从零澄清需求，产出整本书的创作指令。
func (h *Host) CoCreateStream(ctx context.Context, history []CoCreateMessage, onProgress func(kind, text string)) (CoCreateReply, error) {
	return coCreateStream(ctx, h.models, h.store.Sessions, coCreateSystemPrompt, history, onProgress)
}

// StageCoCreateStream 阶段共创：在已写内容的基础上规划后续方向。
// 系统提示 = 阶段 prompt + 当前故事状态摘要，让助手知道"已经写了什么"。
func (h *Host) StageCoCreateStream(ctx context.Context, history []CoCreateMessage, onProgress func(kind, text string)) (CoCreateReply, error) {
	return coCreateStream(ctx, h.models, h.store.Sessions, stageSystemPrompt(h.store), history, onProgress)
}

// stagePlanPrefix 把共创产出的"后续方向 brief"包装成一条阶段规划干预，交 Arbiter 裁定。
// 只贴 [阶段规划] 事实标记 + 中性陈述，不写死"怎么落地"——具体路由（compass / architect /
// user_rules）交给 arbiter-intervention.md 的「阶段规划」判据，避免与 prompt 形成第二真相源、
// 也不堵死风格类要求走 user_rules（守"分类裁定归 LLM"）。Continue 再叠加 [用户干预] 前缀。
const stagePlanPrefix = "[Lập kế hoạch theo giai đoạn] Tôi đã tạm dừng sáng tác, cùng trợ lý đồng sáng tác sắp xếp các hướng tiếp theo dưới đây, hãy phán định cách triển khai theo phân loại can thiệp của bạn, rồi tiếp tục sáng tác. Các hướng tiếp theo như sau:\n\n"

// PauseForCoCreate 进入阶段共创：置共创占用标记，运行中则一并暂停 Engine。
// 返回 false 表示无法进入（全书已完成或已在共创中），调用方忽略即可。
// 占用标记在共创窗口内堵住 import/simulate/start/resume/continue 的并发介入——
// 运行中暂停后 lifecycle=paused，现有 ==running 互斥失效，靠该标记补缺；
// 已停止（idle/paused）也允许进入，规划完经 Continue 续跑。
func (h *Host) PauseForCoCreate() bool {
	h.mu.Lock()
	if h.cocreating || h.lifecycle == lifecycleCompleted {
		h.mu.Unlock()
		return false
	}
	h.cocreating = true
	running := h.lifecycle == lifecycleRunning
	h.mu.Unlock()

	// 运行中复用 abortWithEvent 停机（running→paused + setAborting + Abort + 事件），与手动
	// 暂停同序、不另抄一遍；已停止（idle/paused）只置标记，规划完经 Continue 续跑。
	if running {
		h.abortWithEvent("Vào đồng sáng tác theo giai đoạn, sáng tác đã tạm dừng", "info")
	} else {
		h.emitEvent(Event{Time: time.Now(), Category: "SYSTEM", Summary: "Vào đồng sáng tác theo giai đoạn", Level: "info"})
	}
	return true
}

// ResumeFromCoCreate 结束阶段共创：把共创产出的后续方向作为干预注入并恢复创作。
// 清占用标记后复用 Continue 的停机注入路径（受预算前置约束）。
// 注：draft 为空时提前返回、不清标记是有意的（共创尚未结束）；TUI 侧 canStart() 守卫
// 与此处用同一"非空"判据，保证该路径不可达，cocreating 不会因此泄漏。
func (h *Host) ResumeFromCoCreate(draft string) error {
	draft = strings.TrimSpace(draft)
	if draft == "" {
		return fmt.Errorf("Bản nháp không được để trống")
	}
	h.mu.Lock()
	if !h.cocreating {
		h.mu.Unlock()
		return fmt.Errorf("Không ở chế độ đồng sáng tác")
	}
	h.cocreating = false
	h.mu.Unlock()

	// PauseForCoCreate 的 abort 是异步的:等引擎循环真正收敛再继续,回到与手动
	// 暂停后 Continue 一致的"真停机"前提。共创窗口是人机交互时间尺度,短轮询无感。
	for h.engine.isRunning() {
		time.Sleep(20 * time.Millisecond)
	}

	h.emitEvent(Event{Time: time.Now(), Category: "SYSTEM", Summary: "Đồng sáng tác theo giai đoạn hoàn tất, đã nạp hướng tiếp theo và khôi phục sáng tác", Level: "info"})
	return h.continueWriting(stagePlanPrefix+draft, "", false)
}

// CancelCoCreate 放弃阶段共创：清占用标记，保持暂停态（用户可在输入框继续或重启 Resume）。
func (h *Host) CancelCoCreate() {
	h.mu.Lock()
	if !h.cocreating {
		h.mu.Unlock()
		return
	}
	h.cocreating = false
	h.mu.Unlock()
	h.emitEvent(Event{Time: time.Now(), Category: "SYSTEM", Summary: "Đã thoát đồng sáng tác theo giai đoạn, sáng tác vẫn tạm dừng (có thể tiếp tục trong ô nhập)", Level: "info"})
}

// ── 工具 ──

func (h *Host) refreshWriterRestore() {
	if h.writerRestore != nil {
		h.writerRestore.Refresh(h.store)
	}
}

func (h *Host) CheckChapterRevisions() ([]int, error) {
	pending, err := h.store.Revisions.LoadPending()
	if err != nil {
		return nil, fmt.Errorf("Đọc bản ghi khôi phục sửa đổi: %w", err)
	}
	if pending != nil {
		chapters := make([]int, 0, len(pending.Items))
		for _, item := range pending.Items {
			chapters = append(chapters, item.Chapter)
		}
		return chapters, nil
	}
	changes, err := revision.Scan(h.store)
	if err != nil {
		return nil, err
	}
	return revision.ChangedChapters(changes), nil
}

func (h *Host) SyncChapterRevisions(ctx context.Context) (*revision.Result, error) {
	if err := h.acquireExclusive("đồng bộ bản sửa chương"); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(ctx)
	h.mu.Lock()
	h.exclusiveCancel = cancel
	h.mu.Unlock()
	defer h.releaseExclusive()

	pending, err := h.store.Revisions.LoadPending()
	if err != nil {
		return nil, err
	}
	if pending == nil {
		changes, err := revision.Scan(h.store)
		if err != nil {
			return nil, err
		}
		if len(changes) == 0 {
			return &revision.Result{}, nil
		}
		if err := h.budget.Refuse(); err != nil {
			return nil, err
		}
	}
	model := h.models.ForRoleWithFailover("editor", func(ev bootstrap.FailoverEvent) {
		slog.Warn("Chuyển provider sửa đổi chương", "module", "revision", "role", ev.Role,
			"reason", ev.Reason, "from", fmt.Sprintf("%s/%s", ev.FromProvider, ev.FromModel),
			"to", fmt.Sprintf("%s/%s", ev.ToProvider, ev.ToModel), "err", ev.Err)
	})
	model = newUsageTrackedModel(model, "editor", h.usage.Record)
	service := revision.NewService(h.store, model, h.bundle.Prompts.RevisionAnalyze, h.styleStats, h.packRules)
	return service.Sync(ctx)
}

func (h *Host) requireCleanChapters() error {
	chapters, err := h.CheckChapterRevisions()
	if err != nil {
		return fmt.Errorf("Kiểm tra sửa đổi bên ngoài của chương thất bại: %w", err)
	}
	if len(chapters) > 0 {
		return fmt.Errorf("Phát hiện nội dung chương đã bị sửa đổi từ bên ngoài: %v; hãy chạy /sync trước", chapters)
	}
	return nil
}

func truncate(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + "..."
}

// ImportFrom 启动一次外部小说语义编译导入：ingest → segment → analyze → synthesize → publish。
// 模型只裁定开放语义（边界/事实/综合），Go 掌管坐标/覆盖/幂等；与 Engine 运行互斥，
// 导入完成后由 AdvanceHold 决定是否续写。
// 返回的事件通道由 imp.Run 关闭，调用方负责消费（满则丢弃以防阻塞管线协程）。
func (h *Host) ImportFrom(ctx context.Context, opts imp.Options) (<-chan imp.Event, error) {
	// 预算启动前置检查与 Start/Resume/Continue 同一纪律：导入是全流程模型调用，
	// 预算已超时不得启动（§13.1「纳入现有预算哨兵」）。
	if err := h.budget.Refuse(); err != nil {
		return nil, err
	}
	if err := h.acquireExclusive("nhập"); err != nil {
		return nil, err
	}
	// 登记取消函数：预算硬停/手动暂停经 abortWithEvent 取消导入自己的 context
	//（否则哨兵只会去暂停并未运行的 Engine，导入继续烧钱）。
	ctx, cancel := context.WithCancel(ctx)
	h.mu.Lock()
	h.exclusiveCancel = cancel
	h.mu.Unlock()

	deps := h.importDeps()
	ch, err := imp.Run(ctx, deps, opts)
	if err != nil {
		h.releaseExclusive()
		return nil, err
	}
	// 导入是独立于 Engine 的一条工作会话（G3）：其事件按导入会话分组。
	h.beginSession()
	return h.superviseImport(ch, opts), nil
}

// ImportResumeHint 返回未完成导入的一行提示（无则空串），供 TUI 启动时主动告知（RFC §18.2）。
// 只在启动时调用一次：内部会重算工作区各工件的 InputDigest，不适合放进快照轮询。
func (h *Host) ImportResumeHint() string {
	return imp.ResumeSummary(h.store)
}

// importRole 解析一个导入语义函数的模型档位（RFC §13.1）：roles 配置存在 import_<fn>
// 则用该档位（用量也记该角色的账），否则落 architect。这是调用配置，不改任何语义契约。
func (h *Host) importRole(fn string) string {
	role := "import_" + fn
	if _, _, explicit := h.models.CurrentSelection(role); !explicit {
		return "architect"
	}
	return role
}

// importCaller 为后台导入取带 fallback 与可唤醒重试的模型。
func (h *Host) importCaller(fn string) imp.Caller {
	role := h.importRole(fn)
	model := h.models.ForRoleWithFailover(role, func(ev bootstrap.FailoverEvent) {
		slog.Warn("Chuyển provider nhập", "module", "import", "role", ev.Role,
			"reason", ev.Reason,
			"from", fmt.Sprintf("%s/%s", ev.FromProvider, ev.FromModel),
			"to", fmt.Sprintf("%s/%s", ev.ToProvider, ev.ToModel),
			"err", ev.Err)
	})
	model = newUsageTrackedModel(model, role, h.usage.Record)
	return imp.Caller{Model: model, Runtime: h.importModelRuntime(role, model)}
}

// importModelRuntime 探测所选档位角色模型的调用能力，供 imp 双预算 / thinking 自适应使用（RFC §13/§21）。
// 探测失败的字段留零值，imp 侧回退保守默认，保证无能力信息也能正确运行。
// 结构化输出由 imp 的 llmcontract 在每次请求前现读模型事实，不在 Runtime 重复缓存。
func (h *Host) importModelRuntime(role string, model llmtypes.ChatModel) imp.ModelRuntime {
	var rt imp.ModelRuntime
	provider, name, _ := h.models.CurrentSelection(role)
	if name == "" {
		name = bootstrap.ModelName(model)
		provider = bootstrap.ModelProvider(model)
	}
	// context / completion 上限：registry 是唯一可信来源（被包装模型的 Info() 不含窗口）。
	rt.ContextTokens, _ = h.cfg.ResolveContextWindow(provider, name)
	if entry, ok := modelreg.DefaultRegistry().Resolve(name); ok {
		rt.MaxOutputTokens = entry.MaxTokens
	}
	// thinking：按角色 reasoning effort 与模型能力 resolve；不支持则不发（与 arbiter 同策略）。
	if level, err := agents.ParseThinkingLevel(h.cfg.ResolveReasoningEffort(role)); err == nil {
		if resolved, ok := agents.ResolveThinkingForModel(model, level); ok {
			rt.Thinking = resolved
		}
	}
	return rt
}

// Simulate 读取 simulate 目录并生成或增量更新仿写画像。
func (h *Host) Simulate(ctx context.Context) (<-chan sim.Event, error) {
	if err := h.acquireExclusive("tạo hồ sơ mô phỏng văn phong"); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(ctx)
	h.mu.Lock()
	h.exclusiveCancel = cancel
	h.mu.Unlock()

	wd, err := os.Getwd()
	if err != nil {
		h.releaseExclusive()
		return nil, fmt.Errorf("đọc thư mục làm việc: %w", err)
	}
	deps := sim.Deps{
		Store: h.store,
		LLM:   h.models.ForRole("architect"),
		Prompts: sim.Prompts{
			Source: h.bundle.Prompts.SimulationSource,
			Merge:  h.bundle.Prompts.SimulationMerge,
		},
	}
	ch, err := sim.Run(ctx, deps, sim.Options{SourceDir: filepath.Join(wd, "simulate")})
	if err != nil {
		h.releaseExclusive()
		return nil, err
	}
	return superviseExclusive(h, ch), nil
}

// ImportSimulationProfile 导入此前生成的仿写画像。
func (h *Host) ImportSimulationProfile(ctx context.Context, path string) (<-chan sim.Event, error) {
	if err := h.acquireExclusive("nhập hồ sơ mô phỏng văn phong"); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(ctx)
	h.mu.Lock()
	h.exclusiveCancel = cancel
	h.mu.Unlock()
	ch, err := sim.RunImport(ctx, h.store, path)
	if err != nil {
		h.releaseExclusive()
		return nil, err
	}
	return superviseExclusive(h, ch), nil
}

// acquireExclusive 原子占用后台独占作业槽（import/simulate/revision）：Engine 运行中、阶段共创窗口内、
// 或已有独占作业在跑时拒绝。成功即登记占用，作业结束须调 releaseExclusive 释放——否则两个导入
// 或导入+仿写会并发抢改同一状态。补上此前只查 ==running/cocreating、不登记作业本身的缺口。
func (h *Host) acquireExclusive(action string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	switch {
	case h.closing:
		return fmt.Errorf("Host đang đóng, không thể %s", action)
	// engine.isRunning() 必查：Abort 先置 lifecycle=paused 再异步等 goroutine 退出，
	// 该窗口内 lifecycle 已非 running 但引擎仍可能在写 store（与启动门禁同一纪律）。
	case h.lifecycle == lifecycleRunning || h.engine.isRunning():
		return fmt.Errorf("Engine đang chạy hoặc đang dừng, hãy đợi rồi %s", action)
	case h.cocreating:
		return fmt.Errorf("Đang đồng sáng tác theo giai đoạn, hãy kết thúc rồi %s", action)
	case h.exclusive != "":
		return fmt.Errorf("Đang %s, hãy chờ xong rồi %s", h.exclusive, action)
	}
	h.exclusive = action
	return nil
}

// releaseExclusive 释放后台独占作业槽（连同已登记的取消函数）。
func (h *Host) releaseExclusive() {
	h.mu.Lock()
	cancel := h.exclusiveCancel
	h.exclusive = ""
	h.exclusiveCancel = nil
	h.mu.Unlock()
	if cancel != nil {
		cancel() // 作业已结束：释放派生 context；对已退出的 runner 无副作用
	}
}

// superviseExclusive 转发独占作业事件，通道关闭（作业结束）时释放占用槽。
func superviseExclusive[T any](h *Host, src <-chan T) <-chan T {
	out := make(chan T, 32)
	if !h.launchAsync(func() {
		defer close(out)
		defer h.releaseExclusive()
		for ev := range src {
			select {
			case out <- ev:
			case <-h.runCtx.Done():
				// 关闭期继续排空源通道，避免 producer 因终态事件阻塞而无法退出。
				for range src {
				}
				return
			}
		}
	}) {
		close(out)
		h.releaseExclusive()
	}
	return out
}

// superviseImport 是"导入完成后是否接力"的唯一所有者：转发导入事件，成功完成时先释放独占槽、
// 再决定并执行接力，最后把真实接力结果写进 StageDone 事件的 Continued 字段。TUI 只据此渲染，
// 不再用本地 --continue 标志臆测运行态（消除 Runner/Host/TUI 三方各自解释导致的时序竞态）。
func (h *Host) superviseImport(src <-chan imp.Event, opts imp.Options) <-chan imp.Event {
	out := make(chan imp.Event, 32)
	if !h.launchAsync(func() {
		defer close(out)
		released := false
		release := func() {
			if !released {
				released = true
				h.releaseExclusive()
			}
		}
		defer release()
		for ev := range src {
			if ev.Stage == imp.StageDone {
				release() // 先释放独占槽，接力的 startEngine 才能通过独占门禁
				ev.Continued = h.continueAfterImport(opts)
			}
			select {
			case out <- ev:
			case <-h.runCtx.Done():
				for range src {
				}
				return
			}
		}
	}) {
		close(out)
		h.releaseExclusive()
	}
	return out
}

// launchAsync 在 Host 生命周期内登记一个后台任务。closing 与 WaitGroup.Add 受同一
// 把锁保护，保证 Close 开始 Wait 后不会再出现新的 Add。
func (h *Host) launchAsync(fn func()) bool {
	h.mu.Lock()
	if h.closing {
		h.mu.Unlock()
		return false
	}
	h.asyncWG.Add(1)
	h.mu.Unlock()
	go func() {
		defer h.asyncWG.Done()
		fn()
	}()
	return true
}

// runAsync 复用 Host 已有的后台任务登记，同时把业务错误交还调用方。
func (h *Host) runAsync(fn func() error) (error, bool) {
	result := make(chan error, 1)
	if !h.launchAsync(func() { result <- fn() }) {
		return nil, false
	}
	return <-result, true
}

// continueAfterImport 决定并执行 --continue 的真正自动接力，返回 Engine 是否已启动。
// 有效接力意图 = 本次 opts 或工作区持久化 intent（覆盖崩溃后无参数 /import 恢复的场景）；
// 仅 auto 推进模式接力，由自适应扩弧规划承接开放故事、或让已完结故事收尾；review 交用户 /next。
func (h *Host) continueAfterImport(opts imp.Options) bool {
	want := opts.ContinueAfter
	if !want {
		in, err := imp.OpenWorkspace(h.store.Dir()).LoadIntent()
		if err != nil {
			h.emitEvent(Event{Time: time.Now(), Category: "SYSTEM", Level: "warn",
				Summary: "Đã nhập xong, nhưng đọc ý định tự động tiếp nối thất bại: " + err.Error()})
		} else if in != nil {
			want = in.ContinueAfterImport
		}
	}
	if !want {
		return false
	}
	meta, err := h.store.RunMeta.Load()
	if err != nil || meta == nil {
		slog.Warn("Đọc RunMeta cho tự động tiếp nối sau nhập thất bại", "module", "host", "err", err)
		return false
	}
	if meta.AdvanceMode != domain.ChapterAdvanceAuto {
		h.emitEvent(Event{Time: time.Now(), Category: "SYSTEM", Level: "info",
			Summary: "Đã nhập xong; hiện đang ở chế độ nghiệm thu từng chương, nhập tiếp tục hoặc /next để tiếp nối viết tiếp"})
		return false
	}
	h.emitEvent(Event{Time: time.Now(), Category: "SYSTEM", Level: "info", Summary: "Đã nhập xong, tự động tiếp nối viết tiếp"})
	if !h.startEngine(nil) {
		h.emitEvent(Event{Time: time.Now(), Category: "SYSTEM", Level: "warn",
			Summary: "Khởi động tự động tiếp nối thất bại, hãy nhập lệnh tiếp tục để khôi phục thủ công"})
		return false
	}
	return true
}

// Export 导出已完成章节为外部文件（当前仅支持 TXT）。
//
// 与 ImportFrom 不同：导出是只读操作（不动 Progress / Checkpoint），
// 因此**不要求 Engine 停机**——写作中途也可以随时导出"现阶段成品"。
// 只读到 Progress.CompletedChapters + 章节终稿 + 大纲 + premise 的一致快照。
func (h *Host) Export(ctx context.Context, opts exp.Options) (*exp.Result, error) {
	return exp.Run(ctx, exp.Deps{Store: h.store}, opts)
}
