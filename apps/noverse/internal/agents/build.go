package agents

import (
	llmtypes "github.com/datnt1998/verse/core/llm"

	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/datnt1998/verse/apps/noverse/assets"
	"github.com/datnt1998/verse/apps/noverse/internal/agents/ctxpack"
	"github.com/datnt1998/verse/apps/noverse/internal/agents/guard"
	"github.com/datnt1998/verse/apps/noverse/internal/bootstrap"
	"github.com/datnt1998/verse/apps/noverse/internal/rules"
	"github.com/datnt1998/verse/apps/noverse/internal/store"
	"github.com/datnt1998/verse/apps/noverse/internal/tools"
	"github.com/datnt1998/verse/apps/noverse/internal/userrules"
	"github.com/datnt1998/verse/core/agentcore"
	corecontext "github.com/datnt1998/verse/core/agentcore/context"
	"github.com/datnt1998/verse/core/agentcore/subagent"
)

// agentToRole 把 subagent name 归一为 ModelSet 认得的 role 名。
// architect_short / architect_long 都共用同一个 architect role 配置。
// 跟 host.agentRoleName 同义，因为 build 与 host 互不依赖故各持一份。
func agentToRole(name string) string {
	if strings.HasPrefix(name, "architect_") {
		return "architect"
	}
	return name
}

// promptCacheBase 从书目录派生稳定短哈希，作为提示词缓存身份前缀：同一本书
// 跨进程重启共享路由桶，且不向 provider 泄露本地路径。角色后缀由调用方拼接，
// subagent 每次 spawn 再追加 "#seq"（一次会话一个键）。
func promptCacheBase(bookDir string) string {
	sum := sha256.Sum256([]byte(bookDir))
	return "nvl-" + hex.EncodeToString(sum[:6])
}

// subagentMaxRetries 是所有 Worker 的 LLM retry 上限。
// 退避策略：指数退避（受 maxDelay 上限约束），优先服从 server Retry-After。
// 工具只在完整 Assistant 消息提交后启动，因此 stream-idle / 503 /
// 短暂网络抖动可以在 Worker 内安全重试，不会重放工具副作用。
const subagentMaxRetries = 7

// UsageRecorder 是 BuildWorkers 可选的用量回调；签名与 OnMessage 一致，
// 每条 agent 消息都会调一次，由 Host 层负责聚合。task 是本次 spawn 的任务文本
// 作为会话身份，供缓存链断裂检测按会话重置基线。
// nil 表示不追踪。
type UsageRecorder func(agentName, task string, msg llmtypes.AgentMessage)

// ApplyThinking 把某具体角色的推理强度应用到 Worker（运行时 /model 调整用）。
// architect → 两个 architect_* 子代理；writer/editor → 对应子代理。
// 空 level = 沿用模型/provider 默认。其它 role 名忽略。
type ApplyThinking func(role string, level llmtypes.ThinkingLevel)

func ResolveThinkingForModel(model llmtypes.ChatModel, level llmtypes.ThinkingLevel) (llmtypes.ThinkingLevel, bool) {
	level = llmtypes.NormalizeThinkingLevel(level)
	// 对不支持 thinking 的普通 chat 模型，显式 off 不是 no-op，而是非法参数。
	if cp, ok := model.(llmtypes.CapabilityProvider); ok && cp.Capabilities().Thinking.Supported == llmtypes.SupportNo {
		return llmtypes.ThinkingAuto, level == llmtypes.ThinkingAuto
	}
	return llmtypes.ThinkingPolicyFor(model).Resolve(level)
}

func AvailableThinkingForModel(model llmtypes.ChatModel) []llmtypes.ThinkingLevel {
	if cp, ok := model.(llmtypes.CapabilityProvider); ok && cp.Capabilities().Thinking.Supported == llmtypes.SupportNo {
		return []llmtypes.ThinkingLevel{llmtypes.ThinkingAuto}
	}
	return llmtypes.ThinkingPolicyFor(model).Available
}

// roleThinking 解析某角色生效的推理强度；非法值降级为空（不覆盖）并 warn。
func roleThinking(cfg bootstrap.Config, role string) llmtypes.ThinkingLevel {
	lv, err := llmtypes.ParseThinkingLevel(cfg.ResolveReasoningEffort(role))
	if err != nil {
		slog.Warn("bỏ qua cấu hình mức suy luận không hợp lệ", "module", "agent", "role", role, "err", err)
		return ""
	}
	return lv
}

func resolvedRoleThinking(model llmtypes.ChatModel, cfg bootstrap.Config, role string) llmtypes.ThinkingLevel {
	resolved, _ := ResolveThinkingForModel(model, roleThinking(cfg, role))
	return resolved
}

// BuildWorkers 组装三个 Worker(architect_short/long、writer、editor)为可程序化
// 调用的 subagent.Runner。Engine 直接调用其类型化入口，无 LLM 工具层
// (docs/engine-rfc.md §1)。
// 返回 Runner、WriterRestorePack 与 ApplyThinking(运行时 /model 联动各角色推理强度;
// writer/architect/editor 的 ContextManager 走工厂自动重建)。
// onGuardBlock 可选(nil 安全):各 Worker StopGuard 的拦截/升级审计回调。
func BuildWorkers(
	cfg bootstrap.Config,
	store *store.Store,
	styleStats *tools.StyleStatsIndex,
	models *bootstrap.ModelSet,
	bundle assets.Bundle,
	recordUsage UsageRecorder,
	onGuardBlock guard.BlockHook,
	packDefaults ...rules.Candidate,
) (*subagent.Runner, *ctxpack.WriterRestorePack, ApplyThinking) {
	// 共享工具
	contextTool := tools.NewContextTool(store, bundle.References, cfg.Style, styleStats)
	readChapter := tools.NewReadChapterTool(store)
	saveReview := tools.NewSaveReviewTool(store, bundle.Pack)
	var packRules rules.Candidate
	if len(packDefaults) > 0 {
		packRules = packDefaults[0]
	}

	architectTools := []agentcore.Tool{
		contextTool,
		readChapter,
		tools.NewSaveBookTool(store),
		tools.NewSaveFoundationTool(store, bundle.Pack).WithLedgerModel(models.ForRole("architect")),
		tools.NewReviseOutlineTool(store, bundle.Pack),
		// 返工队列里的章节只能靠 chapter_contract 承接"该重写成什么样"：
		// revise_outline 不许碰已写章节，save_foundation(outline) 写作期禁止全量覆盖。
		tools.NewPlanChapterTool(store),
		tools.NewResolveOutlineFeedbackTool(store),
		tools.NewAuditFoundationTool(store),
	}
	writerTools := []agentcore.Tool{
		contextTool,
		readChapter,
		tools.NewPlanChapterTool(store),
		tools.NewDraftChapterTool(store),
		tools.NewEditChapterTool(store),
		tools.NewCheckConsistencyTool(store),
		tools.NewCommitChapterTool(store, styleStats, packRules),
	}
	editorTools := []agentcore.Tool{
		contextTool,
		readChapter,
		saveReview,
		tools.NewSaveArcSummaryTool(store),
		tools.NewSaveVolumeSummaryTool(store),
	}

	// Provider failover 只记日志,不通知宿主
	reportFailover := func(ev bootstrap.FailoverEvent) {
		slog.Warn("chuyển provider",
			"module", "agent",
			"role", ev.Role,
			"reason", ev.Reason,
			"from", fmt.Sprintf("%s/%s", ev.FromProvider, ev.FromModel),
			"to", fmt.Sprintf("%s/%s", ev.ToProvider, ev.ToModel),
			"err", ev.Err,
		)
	}

	architectModel := models.ForRoleWithFailover("architect", reportFailover)
	writerModel := models.ForRoleWithFailover("writer", reportFailover)
	editorModel := models.ForRoleWithFailover("editor", reportFailover)

	// P0-3/P0-6: gói thể loại được chọn qua cfg.Style (bundle.Pack != nil)。
	// Ba vai đều nhận nguyên văn luat/*.md ở cuối system prompt（不经过
	// userrules 归一化）。Writer 的 style 段换成 gói 的 van-phong.md；
	// gói 没有 van-phong.md 时 style 段为空，不回退 styles/<name>.md。
	pack := bundle.Pack
	writerStyle := bundle.Styles[cfg.Style]
	if pack != nil {
		writerStyle = pack.VanPhong
	}
	architectShortPrompt := assets.AppendPackRules(bundle.Prompts.ArchitectShort, pack)
	architectLongPrompt := assets.AppendPackRules(bundle.Prompts.ArchitectLong, pack)
	lessonContext, lessonErr := userrules.LessonContext(store, cfg.Style)
	if lessonErr != nil {
		slog.Error("Không đọc được bài học đã giữ", "module", "architect", "err", lessonErr)
	}
	architectShortPrompt += lessonContext
	architectLongPrompt += lessonContext
	editorPrompt := assets.AppendPackRules(bundle.Prompts.Editor, pack) + saveReview.RuleContext()
	// 唯一组装路径:协议模板 {{VOICE}} 原位回填文风段,再追加风格预设。
	// eval 的 voice A/B 走同一函数,保证两臂等价(docs/voice-layer.md §3.2)。
	writerPrompt := assets.AppendPackRules(assets.BuildWriterPrompt(bundle.Prompts.Writer, bundle.Voice, writerStyle), pack)
	architectShortPrompt += guard.AmbiguityPrompt
	architectLongPrompt += guard.AmbiguityPrompt
	writerPrompt += guard.AmbiguityPrompt
	editorPrompt += guard.AmbiguityPrompt

	// Quyết định 2: system prompt 每角色 ≤ 40% 所挂模型的上下文窗口，
	// 超出只记 warning（不拆包）。估算复用 corecontext.EstimateTokens。
	for _, sp := range []struct {
		agent, role, prompt string
	}{
		{"architect_short", "architect", architectShortPrompt},
		{"architect_long", "architect", architectLongPrompt},
		{"writer", "writer", writerPrompt},
		{"editor", "editor", editorPrompt},
	} {
		provider, model, _ := models.CurrentSelection(sp.role)
		window, _ := models.ResolveContextWindow(provider, model)
		tokens := corecontext.EstimateTokens(llmtypes.SystemMsg(sp.prompt))
		if window > 0 && tokens*10 > window*4 {
			slog.Warn("system prompt của vai chiếm hơn 40% cửa sổ ngữ cảnh",
				"module", "agent",
				"role", sp.role,
				"agent", sp.agent,
				"model", model,
				"prompt_tokens", tokens,
				"context_window", window,
			)
		}
	}

	// Writer 的 ContextManager 由工厂每次调用重建，窗口随模型 swap 动态跟随（见下方工厂）。
	writerProvider, writerModelName, _ := models.CurrentSelection("writer")
	writerContextWindow, writerSource := cfg.ResolveContextWindow(writerProvider, writerModelName)
	bootstrap.LogContextWindowChoice("writer", writerModelName, writerContextWindow, writerSource)
	// modelLookup 写入 session 时给每条 assistant 消息附 _meta:{provider,model}，
	// 让 replay 不再依赖"当前 ModelSet"来反推历史 cost，运行中切换模型也能精确算。
	modelLookup := func(agentName string) (string, string) {
		role := agentToRole(agentName)
		provider, name, _ := models.CurrentSelection(role)
		return provider, name
	}
	baseOnMsg := store.Sessions.SubAgentLogger(modelLookup)
	onMsg := func(agentName, task string, msg llmtypes.AgentMessage) {
		baseOnMsg(agentName, task, msg)
		if recordUsage != nil {
			recordUsage(agentName, task, msg)
		}
	}
	// onEvent 把 core 信号事件（retry/stream_idle/tool_blocked/circuit_open/
	// compaction/subagent_exit）写进同一份 session 文件，供 eval 采集。
	onEvent := SubAgentEventLogger(store)

	// 提示词缓存：一书一基、一角色一名、一会话一键（subagent spawn 追加 #seq）。
	// OpenAI 系用 prompt_cache_key 做路由亲和；Claude 系用 cache_control 滚动断点
	//（system 地板 + 末消息尖端）。provider 不支持时由 agentcore 按能力静默丢弃，
	// 多轮会话下读缓存收益恒为正，故不设开关。
	cacheBase := promptCacheBase(store.Dir())

	architectStopGuardFactory := func(_, _ string) agentcore.StopGuard {
		return guard.NewArchitectStopGuard(store, onGuardBlock)
	}
	architectThinking, _ := ResolveThinkingForModel(architectModel, roleThinking(cfg, "architect"))
	// Chặn vòng "gọi lại y nguyên lời gọi vừa lỗi" (guard chung của core,
	// Noverse giữ câu chặn riêng với gợi ý đổi tham số đọc chương).
	repeatFailure := func() agentcore.ToolMiddleware {
		return agentcore.NewRepeatFailureMiddleware(&agentcore.RepeatFailureOptions{
			Refusal: func(call llmtypes.ToolCall, failed int) error {
				return fmt.Errorf("lời gọi %s với đúng tham số này đã lỗi %d lần liền và lặp lại sẽ không thành công. Hãy đổi tham số (ví dụ bỏ chapter, chọn chương khác, đổi source) hoặc dùng thông tin đã có để làm bước tiếp theo", call.Name, failed)
			},
		})
	}
	architectMiddleware := repeatFailure()
	writerMiddleware := repeatFailure()
	editorMiddleware := repeatFailure()

	architectShort := subagent.Config{
		Name:             "architect_short",
		Description:      "Nhà quy hoạch truyện ngắn: tạo thiết lập gọn và đại cương phẳng cho truyện một tập, một xung đột, mật độ cao",
		Model:            architectModel,
		SystemPrompt:     architectShortPrompt,
		Tools:            architectTools,
		MaxTurns:         15,
		MaxRetries:       subagentMaxRetries,
		ThinkingLevel:    architectThinking,
		OnMessage:        onMsg,
		OnEvent:          onEvent,
		CacheLastMessage: "ephemeral",
		PromptCacheKey:   cacheBase + "-architect_short",
		StopAfterToolResult: func(toolName string, result json.RawMessage) bool {
			return foundationReadyResult(toolName, result)
		},
		StopGuardFactory: architectStopGuardFactory,
		Middlewares:      []agentcore.ToolMiddleware{architectMiddleware},
	}
	architectLong := subagent.Config{
		Name:                "architect_long",
		Description:         "Nhà quy hoạch truyện dài: tạo thiết lập phân tầng và đại cương tập/cung cho truyện nhiều kỳ, nâng cấp bền vững",
		Model:               architectModel,
		SystemPrompt:        architectLongPrompt,
		Tools:               architectTools,
		MaxTurns:            20,
		MaxRetries:          subagentMaxRetries,
		ThinkingLevel:       architectThinking,
		OnMessage:           onMsg,
		OnEvent:             onEvent,
		CacheLastMessage:    "ephemeral",
		PromptCacheKey:      cacheBase + "-architect_long",
		StopAfterToolResult: architectLongShouldStopAfterToolResult,
		StopGuardFactory:    architectStopGuardFactory,
		Middlewares:         []agentcore.ToolMiddleware{architectMiddleware},
	}

	restore := &ctxpack.WriterRestorePack{}
	restore.Refresh(store)

	writer := subagent.Config{
		Name:             "writer",
		Description:      "Người sáng tác: tự hoàn thành ý tưởng, viết, tự thẩm định và nộp một chương",
		Model:            writerModel,
		SystemPrompt:     writerPrompt,
		Tools:            writerTools,
		MaxTurns:         30,
		MaxRetries:       subagentMaxRetries,
		ThinkingLevel:    resolvedRoleThinking(writerModel, cfg, "writer"),
		StopAfterTools:   []string{"commit_chapter"},
		OnMessage:        onMsg,
		OnEvent:          onEvent,
		CacheLastMessage: "ephemeral",
		PromptCacheKey:   cacheBase + "-writer",
		StopGuardFactory: func(_, _ string) agentcore.StopGuard {
			return guard.NewWriterStopGuard(store, onGuardBlock)
		},
		Middlewares: []agentcore.ToolMiddleware{writerMiddleware},
		ContextManagerFactory: func(model llmtypes.ChatModel) agentcore.ContextManager {
			// 每章按当前 writer 模型重建上下文管理器。
			window, _ := models.ResolveContextWindow(bootstrap.ModelProvider(model), bootstrap.ModelName(model))
			return newContextManager(contextManagerConfig{
				Model:         model,
				ContextWindow: window,
				ReserveTokens: bootstrap.CompactReserveTokens(window),
				Agent:         "writer",
				// 提交投影，避免后续轮次反复改写请求前缀。
				CommitProjected: true,
				ToolMicrocompact: &corecontext.ToolResultMicrocompactConfig{
					MinResultTokens: 200,
				},
				ExtraStrategies: []corecontext.Strategy{
					ctxpack.NewStoreSummaryCompact(ctxpack.StoreSummaryCompactConfig{
						Store:            store,
						KeepRecentTokens: 20000,
					}),
				},
				Summary: &corecontext.FullSummaryConfig{
					PostSummaryHooks:    []corecontext.PostSummaryHook{restore.Hook()},
					SystemPrompt:        ctxpack.WriterSummarySystemPrompt,
					SummaryPrompt:       ctxpack.WriterSummaryPrompt,
					UpdateSummaryPrompt: ctxpack.WriterUpdateSummaryPrompt,
					TurnPrefixPrompt:    ctxpack.WriterTurnPrefixPrompt,
				},
			})
		},
	}

	editor := subagent.Config{
		Name:             "editor",
		Description:      "Người thẩm duyệt: đọc nguyên văn, phát hiện vấn đề ở cả tầng cấu trúc và tầng thẩm mỹ",
		Model:            editorModel,
		SystemPrompt:     editorPrompt,
		Tools:            editorTools,
		MaxTurns:         20,
		MaxRetries:       subagentMaxRetries,
		ThinkingLevel:    resolvedRoleThinking(editorModel, cfg, "editor"),
		OnMessage:        onMsg,
		OnEvent:          onEvent,
		CacheLastMessage: "ephemeral",
		PromptCacheKey:   cacheBase + "-editor",
		// 终态产物命中即停。终态退出仍会咨询 StopGuard（契约测试 TestContract_
		// TerminalToolExitConsultsStopGuard），任务感知的 NewEditorStopGuard 负责
		// 否决"被派生成摘要却只做了复核"的提前退出，所以 save_review 可以安全硬停。
		StopAfterToolResult: func(toolName string, _ json.RawMessage) bool {
			return toolName == "save_review" || toolName == "save_arc_summary" || toolName == "save_volume_summary"
		},
		StopGuardFactory: func(_, task string) agentcore.StopGuard {
			return guard.NewEditorStopGuard(store, task, onGuardBlock)
		},
		Middlewares: []agentcore.ToolMiddleware{editorMiddleware},
	}

	runner := subagent.NewRunner(architectShort, architectLong, writer, editor)

	// 运行时联动各角色推理强度（/model 调整用）。
	applyThinking := func(role string, level llmtypes.ThinkingLevel) {
		switch role {
		case "architect":
			level, _ = ResolveThinkingForModel(models.ForRole("architect"), level)
			runner.SetThinkingLevel("architect_short", level)
			runner.SetThinkingLevel("architect_long", level)
		case "writer", "editor":
			level, _ = ResolveThinkingForModel(models.ForRole(role), level)
			runner.SetThinkingLevel(role, level)
		}
	}

	return runner, restore, applyThinking
}

type saveFoundationResult struct {
	Type            string `json:"type"`
	FoundationReady bool   `json:"foundation_ready"`
}

func decodeSaveFoundationResult(toolName string, result json.RawMessage) saveFoundationResult {
	if toolName != "save_foundation" {
		return saveFoundationResult{}
	}
	var r saveFoundationResult
	_ = json.Unmarshal(result, &r)
	return r
}

func architectLongShouldStopAfterToolResult(toolName string, result json.RawMessage) bool {
	if foundationReadyResult(toolName, result) {
		return true
	}
	r := decodeSaveFoundationResult(toolName, result)
	switch r.Type {
	case "expand_arc", "complete_book":
		return true
	default:
		return false
	}
}

func foundationReadyResult(toolName string, result json.RawMessage) bool {
	if toolName != "audit_foundation" {
		return false
	}
	var r struct {
		FoundationReady bool `json:"foundation_ready"`
	}
	return json.Unmarshal(result, &r) == nil && r.FoundationReady
}
