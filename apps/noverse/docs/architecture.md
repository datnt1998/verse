# ainovel-cli 运行时架构

> 事实层确定，语义层自主：一个串行确定性 Engine、三个自主 Worker、少数几个按需 Arbiter 函数、一个文件系统事实层。
>
> 2026-07-12 控制面更替完成：Coordinator LLM 长循环退役，由 Engine（确定性循环）+ Arbiter（语义裁定函数）接管。设计决策与评审记录见 `docs/engine-arbiter.md`，RFC 见 `docs/engine-rfc.md`。

---

## 1. 目标（按优先级）

1. **稳定性**：一句话输入，稳定写完整本小说（200~500 章）。中间不因架构问题自行中断。
2. **质量可迭代**：prompt / 参考资料 / 评审维度 / 上下文策略可独立调整，不牵连架构。
3. **可恢复**：崩溃、断网、暂停后能从最近 checkpoint 继续。
4. **可观测**：每章每 step 的进度、产物、用时可查。

"稳定"是前提，"质量"是上层。每个架构决策优先服务稳定性。

---

## 2. 核心原则

### 2.1 三分法：决策按性质归位

- **可枚举的状态迁移 → 代码**。"写完一章后派谁"是读事实查表：`flow.Route` 纯函数 + 万级组合穷举规格测试，错误率趋近 0、零 LLM 开销。
- **边界清晰的语义判断 → LLM 函数（Arbiter）**。选规划师、用户干预分诊、失败/僵局出路：事实进、结构化决策出、机械校验兜底、每次裁定落盘可回放。
- **开放式创作 → LLM 循环（Worker）**。一章、一次评审、一次规划之内，architect/writer/editor 完全自主。

两平面对称是贯穿性纪律——未来任何新决策点照此形状，不发明新模式：

```
确定性平面:  flow.LoadState   → flow.Route     → Instruction   (穷举规格测试)
语义平面:    arbiter.Collect* → arbiter.Decide* → XxxDecision   (decisions.jsonl + eval 回归)
              └── 事实采集(IO) ──┘└── 核心(可离线重放) ──┘└── Engine 执行 ──┘
```

### 2.2 工具是事实层唯一接口

所有与文件系统、Progress、Checkpoint 的交互都由工具完成。单个文件使用 `temp + fsync + rename` 原子替换；跨文件顺序写入不冒充数据库事务：章节提交使用持久化 `PendingCommit` Saga，结构写入使用确定性幂等重放并显式暴露失败。每一步都必须检查错误；只有已持久化恢复意图的流程才能承诺跨重启按原载荷恢复。

### 2.3 观察层只观察

UI、诊断、事件日志都是从事件流 / 只读工件投影出来的被动消费者。读事实，不产生事实，不影响控制流。

观测数据严格分三层：`agentcore.ProgressPayload` 是传输层，错误文本必须完整且不得包含 UI 截断策略；`host.Event.Summary` 是短展示语义，`Detail` 是完整诊断；文件日志优先写完整 `Detail`，TUI 只读 `Summary` 并在最终渲染时按终端宽度截断。文件 logger 由 `Host` 持有：先取得小说目录租约，再建立日志会话，随后才装配 Store、模型和 Engine；这样既不会绕过单书独占，也能覆盖全部装配和关闭日志。所谓“完整日志”指错误链、原始非法参数和生命周期元数据不丢失，不是把成功生成的小说正文重复转录到 `tui.log`；大内容仍由 Store 工件和 `meta/sessions` 承载。

**`internal/diag` 是引擎唯一的可观测性子系统**——一等支撑设施，但不是产品核心。它跨读几乎所有工件 + session + log + checkpoint，承担两职：① **创作质量诊断**（规则 → Finding，`/diag` 屏上报告）；② **运行时排错 + 脱敏导出**（行为骨架剥正文 + 循环聚合 → 覆盖式 `meta/diag-export.md`）。

**观察者纪律（不可松动）**：diag 可以诊断、可以建议，但**永不自己动手**——不自动修复、不续跑、不改流程（历史教训见 §10 第 5 条）。

### 2.4 事实层扁平

只有三类事实：

- **Progress** — 进度索引（写到第几章、待重写列表）
- **Checkpoint** — step 级推进记录（plan / draft / commit / review / arc_summary）
- **Artifact** — 章节正文、大纲、角色、摘要等产物

不引入 WorkflowInstance / TaskInstance / Command 等抽象。附属事实（大纲反馈池、机械违规记录、裁定审计）同样是扁平 jsonl，各有唯一生产者与消费者。

### 2.5 四铁律

**铁律一：工具只返事实，不返跨调度指令**。`commit_chapter` 返回 `arc_end` / `needs_expansion` 等结构化字段；不夹带 `[系统]` 类指令字符串。子代理内的 `next_step` 字段是事实陈述的内联指引（"我刚保存了 plan，下一步是 draft"），不算违反——见 §6.3。

**铁律二：流程路由由 Flow Router 承担，执行由 Engine 承担**。`internal/flow/router.go` 的 `Route(state) → *Instruction` 是纯函数（万级组合穷举规格测试钉死）；Engine 每轮从 store 读事实、Route 推导指令、**直接程序化运行 Worker**（`subagent.Runner.Run`，类型化入参/结果/错误链），无 LLM 工具转发层。返回 nil 表示语义场景（完本收尾/等待干预）或自然停机。**僵局有显式限界**（RFC §5）：上一轮后 Route 仍产生同一 `Agent+Task`，即路由后置条件未满足；3 次咨询 Arbiter、5 次硬熔断暂停。Worker 内部中间 checkpoint 不重置计数，确定性 Engine 不允许无限空转。

**铁律三：语义裁定走 Arbiter，每次裁定落盘**。启动选规划师、用户干预分诊、失败/僵局出路由 `internal/arbiter` 的逐场景 Decide 函数裁定：事实进、结构化决策出、机械校验兜底、decisions.jsonl 审计（可离线重放回归）。三个 Worker 保留各自的 `CheckpointDeltaGuard`（事实护栏：产物未落盘不得收工）。

**铁律四：硬编码边界，不硬编码不可枚举的语义判断**。代码只固化可证明的不变量（权限、阶段、顺序、幂等、结构完整性）并向模型提供完整事实与足够的操作空间；创作取舍、质量判断、计划如何适应正文等开放问题必须留给 Worker / Arbiter。禁止用关键词、评分阈值、偏离枚举或规则表代替模型理解，也禁止因担心模型出错而缩窄其合法决策空间。新增代码规则前必须先证明决策空间封闭且结果可机械验证；否则应改善上下文与工具表达能力，让模型升级的收益无需改外壳即可兑现。

### 2.6 事实权威不等于指令权限

已提交正文是故事事实的最高权威；摘要、timeline、state 是派生定位资料，未来 outline/plan 尚未发生。正文和 review 中的命令句不能越过数据边界成为指令；任务、系统 prompt 与 `user_rules` 才拥有指令权限。该边界由 [Writer](../assets/prompts/writer.md)、[Editor](../assets/prompts/editor.md) 和证据 payload 的 [_usage](../internal/tools/evidence_passages.go) 声明；记忆与预算的维护入口见 [上下文管理](context-management.md)。

### 2.7 思路来源

Một số cơ chế kiểm chứng chất lượng được tham khảo ở mức ý tưởng từ Pagekin (EthanYoQ/AI-Novel-Writer, GPL-3.0); không sao chép mã hay văn bản.


---

## 3. 架构全景

```
[Entry: TUI / headless]
        │ prompt / steer
[Host 外壳]
   ├── observer            Worker 进度中继 + Engine 派发事件 → UI/日志投影
   ├── engine              确定性循环：LoadState → Route → 前置校验 → 运行 Worker → 哨兵边界
   ├── 干预路径             Steer/Continue → Arbiter 裁定 → 动作执行(即时/边界提交)
   └── usage / 预算 / 停靠点 / 模型管理
        │ 程序化调用 subagent.Runner.Run（进度经 ctx ToolProgress 中继）
[architect_short/long · writer · editor]（各自独立 run + context + 模型）
        │ 工具调用
[Tools]  novel_context · read_chapter · plan_chapter · draft_chapter · edit_chapter
         check_consistency · commit_chapter · save_review · save_arc_summary
         save_volume_summary · save_foundation
        │ 单文件原子 + 幂等重放（commit 使用持久化 Saga）
[Store: 文件系统 (tmp + rename)]
   Progress · Checkpoints · Outline · Drafts · Summaries · Characters · World
   · Signals · Decisions(裁定审计) · 反馈池 · 违规记录
```

| 层 | 做什么 | 不做什么 |
|---|---|---|
| Entry | 展示、接收输入 | 业务决策 |
| Host/Engine | 生命周期、Route 执行、Worker 运行、哨兵边界、干预编排 | 文学判断；写创作事实（控制态动作经工具内核） |
| Arbiter | 语义裁定（结构化决策） | 亲自创作；执行动作 |
| Workers | 思考、写作、审阅 | 直接读写 Store（必须经工具） |
| Tools | 单文件原子 IO + 显式错误 + 幂等；commit 使用 Saga | 跨子代理调度指令 |
| Store | 文件系统落盘 | 业务逻辑 |

依赖单向：`entry → host → agents/arbiter → tools → store → domain`；`flow` 为顶层纯策略包（store 之上、host 之下）。横向独立：`errs/` 可被任何层引用，`diag/` 订阅 host 事件流 + 只读 `store/`。

---

## 4. 数据模型

### 4.1 BookMetadata 与 Progress

`BookMetadata` 是书名和面向读者简介的唯一事实源，持久化到 `meta/book.json`；`book.md` 只是可读投影。Premise 不重复保存书名，Progress 也不承载作品信息。

```go
type BookMetadata struct {
    Title    string
    Synopsis string
}
```

Progress（`internal/domain/runtime.go`）只记录运行状态：

```go
type Progress struct {
    Phase             Phase           // init / premise / outline / writing / complete
    CurrentChapter    int
    TotalChapters     int
    CompletedChapters []int
    TotalWordCount    int
    ChapterWordCounts map[int]int
    InProgressChapter int             // 正在写作的章节
    Flow              FlowState       // writing / reviewing / rewriting / polishing / steering
    PendingRewrites   []int
    PendingRechecks   []int
    StrandHistory     []string        // dominant_strand 序列
    HookHistory       []string        // hook_type 序列
    CurrentVolume, CurrentArc int     // 长篇分层
    Layered           bool
}
```

控制逻辑只读上述事实字段，不依赖任何"更新时间戳"——时间信息由 checkpoint 的 `OccurredAt` 承载。

RunMeta（`meta/run.json`）承载**用户运行意图**（非创作事实）：PlanningTier、PlanStart（启动裁定固化，规划期崩溃恢复的唯一依据）、PendingSteer（干预崩溃保护，单在途槽位）、AdvanceMode / AdvancePermitChapter（逐章验收政策与精确章节许可）、AdvanceHold（干预签署的一次性暂停）。`RunMeta.Init` 跨重启保留全部意图字段。

### 4.2 Checkpoint（`internal/domain/checkpoint.go`）

```go
type Scope      struct { Kind ScopeKind; Chapter, Volume, Arc int }
type Checkpoint struct {
    Seq        int64       // 单调自增
    Scope      Scope       // chapter / arc / volume / global
    Step       string      // plan / draft / commit / review / arc_summary / ...
    Artifact   string
    Digest     string
    OccurredAt time.Time
}
```

存储：`meta/checkpoints.jsonl`，只追加。重复写入相同 `Scope+Step+Digest` 视为幂等不产生新行。

### 4.3 Artifact 与附属事实

Artifact 在 `store/outline.go` `drafts.go` `summaries.go` `characters.go` `world.go`。

- **Signals**：`PendingCommit`（commit 中断恢复）。启动/恢复时读，运行时不读。
- **Decisions**（`meta/decisions.jsonl`）：每次 Arbiter 裁定的审计记录（facts+input+decision），可离线重放；**不是恢复数据源**（恢复只依赖 Progress/Checkpoint/RunMeta）。
- **增长型世界事实**：时间线与角色状态变化分别以 `timeline.jsonl`、`meta/state_changes.jsonl` 追加；进程内维护去重索引，正常提交只写本章增量。旧版 JSON 数组在下一次追加时按“先原子写新日志、后删除旧文件”的幂等协议迁移，`timeline.md` 是可重建的人类可读投影。
- **大纲反馈池**（`meta/outline_feedback.jsonl`）：writer 的普通反馈在下一次结构操作中消费；外部正文修订若影响剧情，则在继续写作前优先交给 architect，处理后清空。
- **机械违规记录**（`meta/rule_violations.jsonl`）：commit 时按 user_rules 检查的结果，editor 评审经 `novel_context(chapter=N)` 消费；best-effort 质量元数据，非与提交同级强一致。

### 4.4 分层大纲与完本收敛（收官卷）

滚动规划（compass 锚点 + 卷骨架 + 弧按需展开）解决"开与滚"，但让"何时结束"从一个数字变成每卷末的开放裁定——完本收敛必须显式设计，否则出现两类僵局：账面写完收不了尾（越界续写死循环，已由结构兜底修复）与叙事写完账面不让停（estimated_scale 高估 + 完结门槛硬否决 → 注水或熔断）。

**收官卷是收敛的一等概念**，完本 = 一次方向裁定 + 一段确定性滑行：

- **宣告（LLM 语义裁定）**：架构师在卷末三选一——append_volume（继续）/ append_volume 带 `"final": true`（收官卷）/ complete_book（条件当下全满足）。estimated_scale 在完结判定里是**证据不是否决权**。
- **执行（代码事实查表）**：收官事实 = `domain.FinaleVolume`。终卷结构写完（`layeredStructurallyComplete`）**且卷末收尾三连齐备（弧评审/弧摘要/卷摘要）**即自动 MarkComplete——完结不抢在 editor 质量闸之前。未宣告的书仍走质量级 `layeredBookComplete`（伏笔+长线归零）。
- **解除（数据推导，无撤销工具）**：宣告后又追加未标记新卷 → 收束态自然解除。状态永远可从 layered_outline 推导。
- **完结判定的派发**：卷末由 Route 分支 10 派 architect_long 走完结判定清单——完结裁定权在架构师（一个 Worker），不在控制面。


### 4.5 存储契约（file backend baseline, 2026-10-28 锁定）

为未来 Postgres backend 做准备，事实权威与边界已在 phase-01 契约测试锁定：

- **权威归属**：`meta/book.json` 是书名/简介唯一事实源，`book.md` 只是可重建投影（篡改或删除投影不得改变事实）；`meta/chapter_records/%06d.json` 是已接纳正文的基线（content_sha256 校验），`chapters/*.md`/`drafts/` 是用户可编辑的工作区，两者字节可不一致；`story.json` 的 `titleRenamed` 决定显示名优先于引擎书名。
- **absent vs corrupt**：缺失工件按契约返回 nil（如 `Book.Load`、`LoadPending`、chapter record），损坏工件必须报错——绝不把坏数据当“还没有”继续覆盖。`meta/format.json` 缺失意为格式 v1。
- **追加日志**：`checkpoints.jsonl`/`decisions.jsonl` 只追加；末尾未换行结束的残行视为未提交，读取时截断，已提交的损坏行必须报错。
- **library 布局**：`<root>/.ainovel/library.json` 的 `dirs` 覆盖必须生效（不写死 `kich_ban`/`output`）；根级 `output/` 目录聚合为匿名项目 `_`；有 `<dirs.stories>/` 无 `project.json` 的是可认领候选目录。
- **契约基线**：`internal/store/testdata/storage-contract/`（manifest 分类所有持久路径 + sanitized fixtures）与 `contract_test.go`（store/webauth/library 三包）在引入第二个 backend 前先钉死当前行为；Postgres backend 必须过同一套断言。

**Phase 02 contracts/backend 切分（2026-10-28 落锤）**：

- `internal/store/contracts.go` — 19 个领域契约接口（`BookStore`/`ProgressStore`/`OutlineStore`/`DraftStore`/`SummaryStore`/`RunMetaStore`/`UserRulesStore`/`SignalStore`/`RuntimeStore`/`CharacterStore`/`CastStore`/`WorldStore`/`CheckpointStore`/`SessionStore`/`UsageStore`/`SimulationStore`/`DecisionStore`/`ChapterRecordStore`/`RevisionStore`），按调用方实际使用的方法抽取——不是泛型 CRUD。具体实现改名 `file*Store`，接口名保持 `XxxStore`（外部调用面不变）。
- `internal/store/backend.go` — `Backend` 契约：`Kind()`、`Dir()`（file 才有，postgres 返回 `""`）、`Substores()`、跨域复合操作、`Close()`；`Substores` 任一域缺失 → `NewFromBackend` 返回 `*CapabilityError`——**启动期失败，不允许半成品 backend**。`internal/errs` 新增存储哨兵 `ErrStorageNotFound`/`Conflict`/`Forbidden`/`Unsupported`，跨后端状态码映射的契约。
- `internal/store/file_backend.go` — file backend：持有 19 个 `file*Store`、`crossMu` 串行化、复合方法（`Init`/`FoundationFingerprint`/`ProjectFormatVersion`/`ExpandArc`/`AppendVolume`/`ReviseOutline`/`ClearHandledSteer`/`InvalidateChapterAggregates`）。仍是单文件原子写 + 有序幂等重放的 saga 语义，不是多文件事务。`NewStore(dir)` = file 真实构造器；`NewFromBackend(b)` = 能力门控入口。
- Postgres plumbing（开发/迁移可用，**非 production-ready**）：`internal/persistence/postgres`（`Pool`+ping+`CheckSchema` 兼容钩子、`RunInTx` 真事务、`ExecCAS`→`ErrStorageConflict`）；`internal/persistence/migrate`+`migrations`（goose/v3 嵌入式 SQL，`00001_foundation`=citext/pg_trgm/`noverse_meta`/`schema.format=1`；唯一 SQL 写路径是迁移，应用层 `sql.DB` 只在 migrate 包内）；`sqlc.yaml`（schema=migrations，queries=`postgres/queries/`，pgx/v5，`gen/` 提交）+ `go tool sqlc vet`/`diff` 门。
- `STORE_DRIVER`（`file` 默认 / `postgres`）+ `DATABASE_URL` + `MIGRATION_DATABASE_URL` + `DB_MAX_CONNS` + `DB_CONNECT_TIMEOUT`：严格解析，错即在启动期失败，postgres 不回落 file，DSN 永不打印。`storage migrate up|status|version` 是唯一 schema 管理入口。`platform/pgtest` 按测试建独立 schema（`TEST_DATABASE_URL` 必配，`REQUIRE_POSTGRES_TEST=1` 让缺失变 fail）。

**Phase 05 document surface（2026-10-29，仍非 production-ready）**：

- `internal/store/io.go` 的 `IO` 不再直接碰磁盘：所有读写经 `IOOps`（`Read/Write/Append/Sync/Remove/RemoveTree/Truncate/ListDir/Exists/MkdirAll/BeginWrite/EndWrite`）。file backend 用 `fileOps`（tmp+rename 原子写、删除缺省不报错、`RemoveAll` 语义）；`IO.mu` 仍是唯一锁，`Lock` 时额外通知 ops 开启“写区域”。
- `internal/persistence/postgres/docstore.go` 是第二个 `IOOps`：把 file backend 的相对路径**映射到已设计好的关系表**，不建通用 KV：`meta/*.json` → `story_state`（kind）、`outline*.json`/`premise.md`/`compass`/`retention` → `outlines`、`summaries/*`/`snapshots/*`/`drafts/*.plan.json`/`chapters/*.md`/派生 `.md` → `story_artifacts`（kind+key）、`*.jsonl` → `story_events`（stream+log_key）、`meta/checkpoints.jsonl` → `checkpoints`、`meta/sessions/agents/*` → `agent_sessions`、`drafts/*.draft.md` → `chapters.draft_text`。未映射路径报错——静默丢写会丢事实。
- 一个写区域（`io.mu.Lock`）在 postgres 上钉住**一个真实事务**，因此每个 substore 方法的原子性与 file 的单文件锁一致；`docstore` 的 `finErr` 只上报一次，避免一次提交失败永久毒化实例。
- `postgres.Backend`（`backend.go`）用 `store.FileBackendOver` 复用全部 file 域规则，只覆盖两处无法用文档表达的事实：`ChapterRecords` 走不可变 `chapter_versions`（接受时盖 `legacy_build`、revision>1 时同事务失效 `reviews`/`review_decisions`）；`World` 的 review 家族走 `reviews` 行（rounds + kept decisions），其余世界事实仍是文档。复合操作（`ExpandArc`/`AppendVolume`/`ReviseOutline`/`ClearHandledSteer`/`InvalidateChapterAggregates`）在**一个事务**内用事务绑定的临时 substore 集执行同一套领域逻辑。
- 镜像：`characters.json` 写入同事务刷新 `characters` 行（搜索/外键），`layered_outline.json` 同事务刷新 `volumes` 行（章节复合外键）。文档仍是权威，行是派生镜像。
- `00004_story_documents` 扩展 `story_state.kind`/`story_artifacts.kind`/`story_events.stream` 并加 `log_key`（主键变 `(project,story,stream,log_key,seq)`），使每个 file 事实都有落点；append-only 表的删除只发生在文档层（`SET LOCAL noverse.allow_immutable_write`）。
- `bootstrap.OpenStoryStore` 解析 project/story scope 后返回完整 facade；`serve` 在 `STORE_DRIVER=postgres` 时把 `Gateway.SetFactStore` 指向它（`projects.go`/`ledger_api.go` 的 5 处 `NewStore` 已改走 seam），工厂失败**不回退目录**。
- **Hosted library（2026-10-29）**：`internal/library/repository.go` 的 `Repository` 是 project/story/idea 的唯一入口；`internal/persistence/postgres/library.go` 把它映射到关系行——项目 → `projects`（`ProjectFile` = name/description/notes/universe_name/created_at），故事 → `stories`（`StoryFile` = title/short_title/title_renamed/status/tags/published_at/created_at，`idea` 字段是 `idea_document_id` 指向的 `project_documents.logical_name`），想法 → `project_documents kind='idea'`（沿用 file 的 `YYYYMMDD-HHmm-<slug>` 名称与 `# Title` 正文），编号仍由 `LockProjectRow` + `SplitNumbered` 在事务内推导 `NN-<slug>`（并发建故事不会拿到同一编号）。`Dir`/`ProjectDir`/`Root` 在 hosted 上恒为空——没有文件镜像，引擎路径因此显式失败而非读目录。actor nil = 内部分径（调用前已授权），非 nil 时 membership 与 mutation 同事务校验：非成员与不存在的项目都返回 `ErrStorageNotFound`（不区分，防 IDOR），写要求 owner|collab → `ErrStorageForbidden`。HTTP 映射：`ErrStorageNotFound`→404、`Forbidden`→403、`Conflict`↔`library.ErrExists`→409、`ErrStorageUnsupported`（draft 协作/assets/validate-fix/adopt，仅 file 有）→501；项目总览把 unsupported assets 呈现为空数组，`/assets` 端点仍 501。`REQUIRE_POSTGRES_TEST=1` 的 `TestLibraryBackendIDOROverHTTP` 覆盖成员/外部/伪造授权三种身份。
- **项目列表同步**：桌面 `LibraryProvider` 独立于共享 resource cache，显式合并 `library.changed` / `project.changed` 事件，并在 SSE 重连或可见页面重新聚焦时重新读取列表；重连同时刷新 `/api/me`。服务端权限仍是唯一授权边界：管理员可读所有有效项目，成员只读显式成员项目，不因客户端刷新放宽权限。


---

**Phase 06 engine cutover（2026-10-29）**：

- `internal/host/storage.go`：`WithStorage(Storage{Store, Lease})` 是 hosted 运行唯一的注入点；Host 拿到后不再创建/加锁/写日志进 `OutputDir`，`Scratch`/`EngineScratchDir` 提供显式私有临时区。file 模式保留 flock + `tui.log` 原链路。
- `internal/entry/web/{gateway,run_worker}.go`：hosted worker 没有故事目录——cwd 是 `os.MkdirTemp` 私有 scratch，scope/DSN 只走 env（`AINOVEL_STORY_SCOPE`、`STORE_DRIVER`、`DATABASE_URL`），子进程自己 resolve scope 并申领 story lease；`postgres.ErrLeaseLost` 映射为 409 而不是 502。
- 读者/工具：`NewStoryReaderWithStore`、`edit_chapter`、comment hook、ledger 全部走 backend 面；`Dir()==""` 且没有 backend 面时返回显式能力错误（501），绝不回退目录。
- `docstore` 的 `meta/chapter_records/%06d.json` 是**只读派生视图**（读 = 当前 revision 行的 JSON，写/删一律拒绝）：`commit_chapter` 的 checkpoint 摘要按 file 路径读取该工件，行后端必须能回答，否则 hosted 提交在 checkpoint 步失败并留下 `progress_marked` 的 PendingCommit。
- `docstore` 的 `reviews/NN[-global].json` 同样是**只读派生视图**（读 = 该 scope_key 最新未作废 review 轮的、与 file backend 相同缩进的 JSON；写/删拒绝）：`save_review` 先写 typed review 轮与 progress，再按 file 路径 digest 该工件；缺映射会让已落库的评审在 checkpoint 步报错并让调用方误以为保存失败。轮次只经 `Narrative.SaveReview*` 追加，文档层绝不成为第二个评审源。
- 门禁：`internal/entry/web/storage_cutover_test.go` 在 PG 上用空且 `0555` 的书库根、空 cwd 跑完整 HTTP 流程（建故事 → 放行一章 → 真实 engine 写 plan/draft/check/commit → 读章节 → 写/读 góp ý），并断言根与 cwd 一个文件都没有；`internal/persistence/postgres` 的 `chapter_record_document_view` 子测试与 `TestPostgresReviewDocumentView` 钉住这两个只读视图。

---

**Phase 07 audio/jobs（2026-10-29）**：

- `internal/persistence/migrations/00007_jobs_audio.sql`：`workers`（SHA-256 token、capability `tts`）、`jobs`（`FOR UPDATE SKIP LOCKED` 队列：state/lease/fencing_token/idempotency key）、append-only `job_events`、`audio_*`（settings/voices/cast/scripts/segments/renders，renders 按 synth key 存候选并只允许一个 `selected`）、`export_cache`（cache key = 内容哈希 + manifest）。冻结/版本触发器沿用 00003（`prevent_immutable_change`、`enforce_row_version_guard`）。
- `internal/persistence/postgres/jobs.go`：`Enqueue` 按 `(story, kind, idempotency_key)` 幂等；`Claim` 在短事务里 SKIP LOCKED，用**服务器时间**给 120 s lease 并 `fencing_token+1`；`Heartbeat/Complete/Fail/Release` 都必须匹配 `(worker_id, fence, state=running, lease_until>now())`，因此被杀/分区的 worker 无法发布。`Cancel/Pause/Retry` 会移动 fence，正在跑的 worker 立刻失效（不必等 lease 过期）。`RequeueExpired` 只在启动时把**过期** running 归位 queued，活 lease 不受影响。`CompleteWith` 让渲染行与 job 结果在同一事务落库。
- `internal/persistence/postgres/audio.go` / `export_cache.go`：settings/voices（项目级）/cast/script（绑定 accepted revision + sha256）/segment（script revision + 稳定 id）/render（synth key = `audio.SynthKey.Hash()`，文本+voice+tone+speed+pitch+quality+engine+model）/export cache（按 cache key，缺失或过期即 miss，不碰 mtime）。二进制只存 BlobStore；上传/登记走下文 Hosted S3 的统一发布协议。
- `internal/entry/web/worker_api.go`：**私有 listener**（`/internal/worker/*`，不得挂到 :8080 公共 mux、不得被 Traefik 路由）上的 claim → snapshot → upload → ACK 协议。Bearer token 只存 hash，必须带 `tts` capability；upload 的 object key 由 `internal/blobkey` 按环境/项目 UUID/故事 UUID/内容 SHA 生成（worker 不能选 key/跨租户覆盖）；complete 在 fence 事务里核对 blob 的 hash/size 并写 `audio_renders`，job id 不属于当前 worker 一律 404（不暴露他人 job 是否存在）。
- `internal/ttsworker`：Mac 侧只拉不听的 worker（`ainovel-cli worker tts --api <tailnet-url>`，token 仅来自 `TTS_WORKER_TOKEN`）。心跳 30 s；发布被 409/404 拒绝即停（stale worker 不重试）；中断时 release 把 job 立刻放回队列；合成串行（Mac 容量下限），engine 走 `internal/voice.NewEngine`。
- 接线入口：`cmd/ainovel-cli/serve.go` 的 `WORKER_LISTEN_ADDR` 私有 listener；token 签发/吊销入口为 `worker token` CLI。Hosted API 使用 `audio.PGService`，file API 保留 `audio.Service`。

**Hosted 补全（2026-10-31）**：

- 配置：`internal/entry/web/config_pg.go` + `internal/persistence/postgres/config.go` 把项目/故事配置存为 `project_documents`/`story_state` 的 `config` 文档，GET 返回逐字段来源，PUT 用 ETag 做事务内 CAS（冲突 409）。hosted worker 由 `bootstrap/hosted_config.go` 按 global → project → story 合并后再加载文风资产并建 engine。
- 规则与文风样本：`hosted_docs.go`（三级规则 CRUD + 继承、样本 multipart/上限）与 `hosted_document_sources.go`（engine 的规则/样本来源适配器，`postgres/rule_sources.go`、`text_documents.go`）。
- 媒体与导出：`pg_media.go` 实现封面渲染缓存（`blobkey` cover 键 + `export_cache`）、项目资产上传/列表、四个文本导出 handler（`export-file/<rel>` 注册行 + export blob），`processExportJob` 完成事务内为每个音频产物登记 `export-file/` 行；`audio.PGService.Peaks` 走 blob → scratch → PCM。剩余 501 只有文件模式专属的 fs/reveal 与 worker 能力桩。
- 模型就绪：`ValidateBase` 保持严格；`Host.New` 只对默认/各 role/fallback 实际引用的 provider 做 `ValidateModelReady`，未被引用的坏 provider 只在设置页提示，不再让整个 app 只读；保存含坏 provider 的配置返回 400。no-model worker 退役时持有 `slot.starting`，直到旧 worker 释放 story lease 才允许新 worker 启动。
- 容器：`NOVERSE_CONTAINER=1` 时书库根限定于 `LIBRARY_BROWSE_ROOTS`（缺省 `/var/lib/noverse/library`），并用 `/proc/self/mountinfo` 检查书库是否在持久挂载上（overlay/tmpfs 不算），否则启动日志与 web 显示警告。
- 项目级共创：`00014_cocreate_drafts` 以 `status='draft'` 的隐藏 story 行承接真实事实 FK，不复制事实；普通解析、列表、搜索、计数与反向导出均排除草稿。worker 用显式 `project/draft/<id>` scope、项目配置/规则和原有 fenced story lease，网页会话存 `story_state.cocreate_session`，无目录镜像。提交先停 worker，再在项目行锁内分配编号、改 slug/status（同一事务，UUID/会话不变）；撤销/过期清理撤销 lease 并删除草稿事实和 blob 引用，物理字节仍由 maintenance `storage gc` 按 grace/共享引用规则回收，应用角色的历史/blob 权限不放宽。
- 声音与读法：`/api/projects/{p}/voices`（列表/上传 WAV 48 kHz、10–30 秒校验/`sample`/删除）走 `audio_voices` + 项目级资产 blob（`sample_blob_id`）；`/pronunciations` 存为 `project_documents` 的 `.ainovel/cach-doc.json`（kind `config`），ETag/If-Match 在锁住项目行的事务内比较。两者不要求项目有故事或目录。
- 元数据版本：项目 `/api/projects/{p}` 与故事 `/meta` 在两种后端都返回 `version` + `ETag`，PUT 接受 `If-Match` 或 `version`，过期返回 `409 {conflict:true, version, current}`；hosted 由 `postgres/library_meta.go` 在同一事务里比较并写入。

#### Hosted S3-compatible blob storage

事实在 Postgres、二进制在 private BlobStore；切换 Garage/R2/AWS S3 不改业务
key 布局。`internal/blobkey` 保留 `{env}/p/{projectUUID}/s/{storyUUID}/…`，
segment 内容寻址只在该 tenant 分支复用，不做全局 dedupe。故事 soft-delete
不是物理 purge；现存 FK/缓存/工件引用仍保留字节。

- **配置与在线迁移**：`bootstrap/blob_settings.go` 是 serve 和 maintenance 的
  同一解析入口：任何 blob env 覆盖并锁住整份配置；否则用 DB，最后才是持久
  local 默认。`00016_storage_settings.sql` 的 singleton CAS 和独立 store
  identity 支持 S3→S3；credentials 用 `CONFIG_ENCRYPTION_KEY` 的
  AES-256-GCM 加密，DB 同存 key id，未提供密钥不允许保存 credentials。
  管理接口只接受 Verse 全局 admin，沿用共享 identity/app-grant/scoped-request
  授权与 `verse_session`，项目 owner 不等于管理员。保留 SSO 00012/00013，
  导入迁移顺序为 00014–00018；新表不扩大 identity role 的权限。
  app 默认 DML grant 被显式收窄：storage_settings 和迁移历史无 DELETE，
  只有 blob_stores 的未引用 probe 配置在 00018 恢复 FK 保护的 DELETE。
  `bootstrap/blob_router.go` 按 blob 行的 store id 读，publication 在上传至
  提交期间持共享 fence，并复用同一条连接解析 active/config 和提交 registry，
  不嵌套获取 pool 连接；物理 S3 identity 是 canonical endpoint + bucket，
  不含 signing region。迁移复用 `blobstore.Copier.CopyKey` 的 SHA/size
  read-back，网络 copy 在 fence 外，最终 delta 检查与 config/ready 行切换
  在短事务内完成。进度、暂停、取消和错误在 DB，重启可续；旧对象不自动删除。
  环境已有数据须显式运行 `storage settings adopt-env` 完成加密与 store
  identity 交接，之后才移除 blob env；没有对应配置的非 local ready 行会
  fail-closed，不隐式 seed。使用量分类由 `blobkey.UsageKind` 随 publication
  写入；旧行通过 maintenance 命令 `storage settings backfill-usage` 分页补齐，
  未分类仍计总量，已知 avatar/PG backup 不计入库文件六类。
- **交付模式**：`delivery=proxy` 复用 app 的 Range/ETag/If-Range 流式路径，
  内网 HTTP/私有 IP/无点 hostname 默认 proxy；云端默认 redirect。
  `presignEndpoint` 对应独立 SigV4 signer，不能签完后替换 host。Garage
  不可靠实现 If-Range，proxy 由 app 处理；redirect 依赖不变 SHA key。
  `blob_probe.go` 用响应 Date 检测超过五分钟的时钟偏差，不依赖 provider
  拒绝未来签名。redirect probe 的 CORS policy 使用小写 header，符合
  Garage 的大小写限制，也适用于 R2/AWS；浏览器自行完成真实 preflight。
- **检查与健康**：`POST /api/system/storage/test`（候选配置或 `{"current":true}`，envlocked 时也可用）和 PUT 内的检查共享每 admin 每分钟 6 次配额；过期 version 在网络检查前拒绝，提交仍在 fence 下 CAS。`Accept: application/x-ndjson` 时每完成一步立即 flush 一行 `{step,ok,ms,code,reason}`，最后一行 `{done:true,…}`；客户端断开即停止后续步骤但仍清理 probe 对象。JSON/NDJSON 都是 `no-store`；`corsProbeUrl` 是含完整 access-key ID 的短期 signed URL，不可记录日志或缓存。Probe 过期后回收无 active/pending/blob/migration/probe 引用的配置，不将 probe-only client 留在路由缓存。后台健康检查在 serve 启动后立即运行，之后每 30 秒一次（3 秒超时），GET 只读其结果，不做远端 listing。

- **数据面**：`platform/blobstore/s3.go`（与 Viderse 共用）用同一个 SDK client/transport。
  Put 先在 TMPDIR 限额 spool 并验证 SHA，成功后才写最终 key，防止坏流覆盖
  已有内容；正常 Open 从一次 GET 取元数据/Range 字节，空范围/空 object
  的边界处理见 provider tests。对象默认 private/no-store + inline，
  不因 SHA 不变而变成 public cache。
  S3 dialer 对 DNS 解析后的实际 IP 拒绝 link-local 和 `fd00:ec2::254`，
  允许内网 Garage；禁用 HTTP redirect 和环境代理，防止重绑定或转发绕过。
- **发布与删除**：`postgres/blob_publish.go` 的 `PublishBlob` 在 key 行锁内
  接受 exact ready duplicate、恢复匹配的 staged；内容/tenant/driver 不匹配
  拒绝。`UploadAndPublishBlob` 在提交前持锁检查 storage，必要时从保留源重传，
  与 `storagegc` 的 `staged/ready → deleting → 删 object → 删行` 协议共同
  封住跨 DB/storage 的 race。失败 tombstone 留待下次 GC，不能中途复活。
  批量删除和 multipart reap 是 `blobstore/batch.go` 的可选能力；维护角色
  才能删 registry 行，app 不获得 DELETE 权限。
- **私有直送**：`entry/web/blob.go`、`audio_backend.go`、`pg_media.go` 在 app
  验权后给短期 signed GET redirect；本地 driver 保留鉴权流式 fallback。
  URL 是临时 bearer credential，不能日志记录；播放 inline，下载 attachment。
  Validator 有明确边界：app SHA ETag 用于本地流及签名前 `If-None-Match` 的
  `304` 判断；redirect 可带 SHA，但最终 S3 representation 及后续直连
  `If-Range`/`If-None-Match` 使用 provider ETag。`307` 不能改写浏览器转发的
  请求 header；不宣称 app SHA 覆盖 S3（尤其 multipart）的 ETag。
- **本地物化**：`audio/blobcache.go` 为 export/peaks 提供 tenant+driver+SHA
  隔离的 verified disk cache、bounded GET/single-flight 和带锁 work dir；
  peaks 缓存按源 SHA/算法/bucket width 区分，PCM 流式扫描。
  `BLOB_CACHE_DIR`/`BLOB_CACHE_MAX_BYTES` 只约束 cache，不是整卷 scratch quota；
  具体 sizing 与 mount 在部署 runbook。`pg_export.go` 维护导出 lease heartbeat，
  fence 丢失时取消工作，完成仍经原 fenced transaction。
- **可观测性**：`blobstore/metrics.go` 在 `serve` 统一装饰共享 store，保留
  可选 batch/multipart/TTL 能力；`GET /api/system/storage` 复用 session/admin
  权限，提供有界 op/driver/outcome 的次数/字节/时长聚合。慢日志只含有界标签、
  数值和可用的 SDK request ID，不含 key、用户、错误原文或签名 query。
  直送的 provider byte/retry 指标不在 app wrapper 的观测范围内。

Bucket CORS、abort-incomplete-only lifecycle、独立 media backup/restore、缓存
盘规划与 provider smoke 前置条件统一见 [部署 runbook](../deploy/README.md)。


## 5. 工具规约

工具是事实层与 Agent 的唯一交互点。

### 5.1 读类工具

`novel_context(scope)` / `read_chapter(n)` —— 任何时候可调用，不依赖前置状态，返回数据足够 LLM 独立决策。`novel_context(chapter=N)` 额外注入该章机械违规（如有）；architect 路径注入已完成卷/当前卷弧摘要、角色快照、大纲反馈池与 foundation 状态。扩弧时，已发生内容是事实，骨架只是计划；Architect 可在 `expand_arc` 中同步修订目标弧的 title/goal 并展开章节。

### 5.2 写类工具（单文件原子 + 分级恢复语义）

单文件写入原子；跨文件步骤不承诺数据库式原子性。`commit_chapter` 的普通提交与返工提交共用 `PendingCommit`，按“完整意图 → artifact/状态 → Progress → checkpoint → 清除意图”推进；恢复只使用首次落盘的规范化 payload 与正文快照，禁止采用重启后模型重新生成的参数或被覆盖的 draft。`expand_arc` / `append_volume` 等结构操作没有持久化意图，只承诺同一参数的幂等重放、派生视图修复和错误显式返回。

| 工具 | Artifact | Step |
|---|---|---|
| `save_book` | meta/book.json + book.md | book |
| `plan_chapter` | drafts/chXX.plan.json | plan |
| `draft_chapter` | drafts/chXX.draft.md | draft |
| `edit_chapter` | drafts/chXX.draft.md | edit |
| `check_consistency` | 无（只读，inline 返回） | consistency_check |
| `commit_chapter` | chapters/chXX.md + Progress（+ 反馈池/违规记录 best-effort） | commit |
| `save_review` | reviews/chXX.json（global 为 chXX-global.json） | review |
| `save_arc_summary` | summaries/arc-vNNaNN.json | arc_summary |
| `save_volume_summary` | summaries/vol-vNN.json | volume_summary |
| `save_foundation` | foundation/*.json（expand_arc/append_volume/update_compass 成功即消费反馈池） | premise / outline / layered_outline / characters / world_rules / expand_arc / append_volume / update_compass / complete_book |

`commit_chapter` 承担弧/卷/全书完成检测，返回结构化事实；`save_review` 不做文学阈值裁定。目标清单与 `contract_status` 的唯一推导源是 [domain.GoalsFromContract / DeriveContractStatus](../internal/domain/goals.go)，引文定位归 [domain.LocateQuote](../internal/domain/quote.go)，提交校验归 [SaveReviewTool](../internal/tools/save_review.go)。审阅结果通过 [ProgressStore.ApplyReviewOutcome](../internal/store/progress.go) 原子合并返工队列；“已重写”不是“已修复”，定向检查走同一 `save_review`，推进边界见 [Chapter Advance Gate](chapter-advance-gate.md)。

`edit_chapter` 是 `agentcore.EditTool` 的薄封装，仅允许编辑已完成且位于 `PendingRewrites` 的章节；新章初稿需通过 `draft_chapter(mode="write")` 整章覆盖。

### 5.3 错误分层

| 错误类型 | 处理层 | 动作 |
|---|---|---|
| 网络超时 / 流式 EOF | Tools | 重试 3 次 |
| provider 429/503 | litellm | failover 到备用 provider |
| 鉴权 / 模型不存在 | Tools | terminal 上抛 |
| 缺前置 artifact | Tools | conflict 上抛，LLM 调 `novel_context` 后重试 |
| 工具参数非法 | Tools | validation 上抛，LLM 改参数 |
| retryable（stream-idle 等） | subagent 层 | MaxRetries=7 就近重试，不出 Worker |
| Worker 失败（guard 升级/hard_stop 等） | Engine | 确定性错误直接暂停；其余同指令重试一次 → Arbiter 裁定 retry/reroute/abort |
| 僵局（同一路由指令连续重现） | Engine | 3 次咨询 Arbiter，5 次硬熔断暂停 |
| 流式空响应 / 长思考 | litellm (`StreamIdleTimeout=5min`) | watchdog 触发重试 |

### 5.4 幂等

每个写类工具执行前先检查 checkpoint：如果当前 scope 最新 checkpoint 的 `Step+Digest` 与本次相同，直接返回已有产物。重试与崩溃恢复后的重复派发都是安全的——这也是 Engine 恢复模型（读 store 续跑）成立的根基。

### 5.5 正文接纳与质量信号

机械事故应在接纳新正文时拒绝，而不是交给文学评分兜底；可疑的复述则保留为审阅事实。各边界只维护一份可执行定义：

| 边界 | 唯一实现入口 |
|---|---|
| thinking 标签、code fence、首尾 meta、独立剧本场景标题（`CẢNH n` / `NỘI.` / `NGOẠI.` / `INT.` / `EXT.`）；终稿另查截断 | [rules.Admission](../internal/rules/integrity.go)，草稿、定点编辑和新终稿接入 |
| 前章结尾重演 `opening_replay`，只报 lint、不由接纳门禁阻断 | [appendOpeningReplay](../internal/rules/lint.go) |
| append 重叠裁剪与 `trimmed_overlap_runes`；整段已存在则拒收 | [DraftChapterTool.Execute](../internal/tools/draft_chapter.go)、[rules.TrimLeadingOverlap](../internal/rules/overlap.go) |
| review 导致的全章重写异常缩水保护 | [validateRewriteDraft / rewriteShrunkBelowFloor](../internal/tools/commit_chapter.go) |

恢复必须尊重冻结载荷，不能追溯套用新门槛。新提交的 admission 与可选事实 quote 校验边界归 [CommitChapterTool.Execute](../internal/tools/commit_chapter.go)；导入实例的 `WithAuthorSource` 保留作者原文，跳过 admission，lint 仍报告。quote 校验不要求导入补证据：导入事实没有 quote。修订分析的非空 quote 则归 [revision.validateAnalysis](../internal/revision/analyze.go)。

新提交的 quote 校验覆盖初稿和 rewrite/polish 两条路径，且均在冻结 PendingCommit 之前执行。错误 quote 不得污染最终正文、chapter record 或冻结状态；已冻结的历史提交仍按原契约恢复。


---

## 6. Worker 装配

> 单一超大 Prompt + 单一 Agent 跑完一本书理论可行，但三件事会阻塞稳定性：**上下文爆炸**（200 章再强压缩也退化）、**职责干扰**（规划严谨 / 写作想象 / 审阅批判在同一 prompt 互相冲淡）、**模型异构红利损失**（规划/写作/审阅独立选模型是显著的成本/质量优化空间）。多 Worker 拓扑因此必要。

### 6.1 装配与运行

`agents.BuildWorkers`（`internal/agents/build.go`）把三类 Worker 装配为一个 `subagent.Runner`：Engine 直接调用 `Run(agent, task)`，每次调用是一个完整的 `agentcore.AgentLoop`（独立 context、独立模型、独立重试）。全部装配一次生效：角色模型 + failover、prompt cache key（每 spawn 自增 #seq）、ThinkingLevel、UsageRecorder/SessionLogger（OnMessage）、Writer ContextManagerFactory（窗口随 /model 切换自动重建）、RestorePack、StopGuardFactory、StopAfterTools。

Worker 进度中继走 **ctx 的 ToolProgress 回调**：Engine 以 `agentcore.WithToolProgress(ctx, relay)` 调 `Runner.Run`，子代理的工具调用/流式正文/thinking/retry/context 事件经 relay 进入 observer——与 Coordinator 时代同一 ProgressPayload 形态，观察层复用。

```
Engine ── Runner.Run(agent, task) ──▶ architect_short/long · writer · editor
                                          │ 工具调用
                                        Store（协作媒介，Worker 之间不直接通信）
```

`bootstrap.ModelSet` 支持角色级模型：architect/writer/editor 各自独立配置 + provider failover。Writer 跑 Sonnet 而不是 Opus 在 200 章长篇上能省一个数量级成本。Arbiter 统一使用 Default 模型（经 usageTrackedModel 计费），当前不开放独立角色配置。

### 6.2 三类协作模式

Worker 之间不直接通信，所有信息流经 Store 中的结构化工件：

**模式 A · 串行移交（主干）**：Route 派 Architect 规划 → Writer 章 1..N → Editor 弧末评审 → Writer 重写。每一步"下一个派谁"由 Route 从事实推导。

**模式 B · 反馈闭环**：Writer 在 commit 中报告大纲偏离 → 反馈池落盘（仅分层书）→ Architect 下次结构操作经 novel_context 参考 → 操作成功即消费清空。Writer 不直接呼叫 Architect，反馈经事实层流转。

**模式 C · 骨架展开（滚动规划）**：commit 后事实显示下一弧仍是骨架 → Route（或 Engine precheck）派 architect_long 展开 → Writer 继续。长篇"滚动规划"能力就是这个闭环。

### 6.3 Worker 流程的代码约束（不靠 prompt 拐杖）

> 早期 writer 流程靠 `writer.md` 的"严格按以下顺序推进"约束。LLM 经常违反——跳过 plan 直接 draft、把正文只写到聊天里不落盘。**提示词约束流程不稳定**，模型升级反而可能让它"创造性地不遵守"。

四层代码约束（同时生效）：

| 层 | 落点 | 作用 |
|---|---|---|
| `StopAfterTools` / `StopAfterToolResult` | `agents/build.go` SubAgentConfig | 关键工具成功即退出 Worker run（终态退出仍咨询 StopGuard，见契约测试）。Writer `commit_chapter` 命中即停；Editor 的 `save_review`/`save_arc_summary`/`save_volume_summary`、Architect 弧/卷收尾走 `StopAfterToolResult` |
| `CheckpointDeltaGuard` | `agents/guard/subagent_guards.go` | 以 baseline checkpoint 为分界，本轮结束前必须看到对应 step 的新 checkpoint，否则拒绝 `end_turn`；连续拦 3 次升级 terminate（弱模型死循环兜底）。Editor 的 guard 任务感知：被派生成摘要时仅复核不算完成 |
| 工具内联 `next_step` | 各工具返回值字段 | 每个事实自带"下一步建议"，LLM 看到事实就知道下一步 |
| 工具内归属/前置检查 | `edit_chapter` `commit_chapter` 等 | 数据层物理拦截：初稿定点编辑、改未入队的已完成章、空提交均被拒，`ConcurrencySafe=false` 阻止并发竞态 |

writer.md 只承担：执行协议、断点续跑认知模型、章节契约解读；写作标准在文风层（`{{VOICE}}` 占位回填，用户可覆盖，见 `docs/voice-layer.md`）。**这正是文风层敢开放给用户的前提：不变量住在工具层，prompt 随便改坏不了状态机。**

### 6.4 agentcore 依赖

`../agentcore` 是本项目自有的通用 Agent 库（go.work 关联）。Engine 用到的原语：`subagent.Runner.Run`（程序化直调，类型化结果与错误链——`errors.Is(err, subagent.ErrUnknownAgent)` 等分类不依赖错误文案）、ctx `ToolProgress`（事件中继）、`subagent.Config`、`StopGuard`/`StopAfterTools`。`subagent.Tool` 只供需要把 Runner 暴露给模型的宿主通过 `Runner.AsTool()` 使用，AINovel 不经过这层。

**修改边界**：可进 agentcore——新 ContextManager 策略、新 provider 适配、新事件类型；不进 agentcore——业务模型与业务工具。判断准则：假设 agentcore 未来会被 coding agent / 客服 agent 引入，新能力在那个场景依然有意义才允许进。**禁止在应用层写兜底补丁**——缺能力直接改上游。

**已知上游缺陷与两个例外落点**（2026-10-26，provider `openai-codex`）：litellm v1.8.10 的 Responses 解析在**并行 function_call** 下会丢工具名/错配 ID——`response.output_item.done` 被当作空事件丢弃，`arguments.done` 的 `name` 只在"没有 delta"时才转发；若 `call_id` 在两条 delta 之间才解析出来，agentcore 还会把 name 与 arguments 拆进两个 block。表现：`tool "" not found`、工具参数缺字段、下一请求 `tool use "call_…" missing name` → Arbiter 判死。两处落点都在**wire 适配层**而非业务层：

| 落点 | 职责 |
|---|---|
| `core/llm/provider/openai/toolstream.go` | 与 `transport.go` 同层：缓冲每个 item 的 ToolUse 事件，在 `output_item.done`（或流结束时）发**唯一**一组 `Start/Delta/Done`，name/args/ID 取自 item，call_id 优先于 item_id |
| `internal/agents/replay.go`（`replayConvert`） | 历史不变量：`ConvertToLLM` 之后**不得存在 name 为空的 tool_use**；缺失时改名为 `unknown_tool` 并保留 call/result 配对，让模型读到 "tool not found" 后自行重试，而不是永久毒化会话 |

升级 litellm 修复上游后应删除 `toolstream.go` 并保留 `replayConvert`（后者是历史不变量，与 provider 无关）。

**契约测试**（`internal/agents/agentcore_contract_test.go`，6 条，全部经 `Runner.Run` 驱动）：把本项目依赖的框架行为钉成可执行断言（终态退出咨询 StopGuard、Error/Aborted 不触达 guard、Escalate 错误链可 `errors.Is` 匹配、`Run` 的类型化 `ErrUnknownAgent`、工具错误进度完整且为纯文本）。**bump agentcore 前必须全绿**——注释会过时，测试不会（这条纪律已经抓到过一次失效假设并省下一个 workaround）。

### 6.5 提示词缓存

长跑成本的第二杠杆（第一是模型选型）。完整讲解版见 `docs/prompt-cache-design.md`。三层分工：**litellm 只做协议翻译**，**agentcore 决定缓存放置与身份**，**ainovel 一行配置接入**。

缓存收益的前提是**请求前缀字节稳定**，由三条纪律保证（都在 agentcore）：

1. **tools 字节确定性** — Description/Schema 每次重建，任何 map 迭代都先排序
2. **历史 append-only** — 消息只追加不改写；上下文压缩是"付一次全 miss 换窗口"的显式交易，投影必须 `CommitOnProject`
3. **动态内容进尾部** — 信封/指令全部尾部追加，永不回写早期消息

配置为「一书一基、一角色一名、一会话一键」：OpenAI 系 `PromptCacheKey = nvl-<书哈希>-<角色>#<spawn序号>` 做路由亲和（默认只对官方端发送，中转可显式开启）；Claude 系 `CacheLastMessage: "ephemeral"` 滚动断点 + system 地板断点。**闩锁红线**：一切进缓存键的量会话内首算即冻结，宁陈旧不破缓存。断裂检测（`host/usage.go noteCacheBreak`）纯观测不修复，计数进 `usage.json cache_breaks` 与 TUI 缓存面板。

---

## 7. Engine 与 Arbiter

### 7.1 Engine 循环（`internal/host/engine.go`）

```
for {
    应用干预控制态动作(排空;hold+dispatch 先建立返工事实)
    advanceGate.HandleBoundary() // hold 消费 + review 许可对账
    inst := 干预派单 ?? Route(LoadState) ?? planStartFallback
    inst == nil → return          // 完本 / 语义停机,等 Continue
    precheck(inst)                // 原 ToolGate 的确定性化身:完本期丢弃派发;
                                  // writer 目标章未展开 → 改派 architect 展开
    advanceGate.Allow(inst)       // 仅阻断未获许可的正向新章
    trackDeadlock(inst)           // 同一 Agent+Task 连续重现:3 次问 Arbiter,5 次熔断
    runWorker(inst)               // subagent.Runner.Run + 进度中继 + DISPATCH 事件
    错误分类:确定性错误→暂停;首败重试一次;再败→Arbiter(retry/reroute/abort)
    政策边界:budget → advanceGate
}
```

单 goroutine 串行；`ctx` cancel = 暂停（checkpoint 保证无损）。**控制状态只在循环边界变更**：干预的 hold/reopen/dispatch 排队至边界提交（hold+dispatch 组合先执行派单建队列，再允许 Gate 消费 hold）；answer/rules 即时执行。`review` 模式只约束正向新章，不阻断返工、评审、结构维护与提交恢复。Arbiter 派单执行前做 Expect 对账（Phase/Flow/QueueHead 语义字段；CheckpointSeq 只审计不对账——干预时 worker 多在跑，seq 必变），不符则丢弃并把原始干预**同步**送回完整裁定路径重询。

返工相关 Route 优先级（完整顺序见 `internal/flow/router.go:Route`）：

| 优先级 | 事实 | 工作 |
|---|---|---|
| 3 | `PendingRewrites` 非空 | Writer 改写 / 打磨 |
| 3b | `PendingRechecks` 非空 | Editor 用 `save_review(recheck=true)` 定向核查 |
| 后续 | 两队列均空 | 聚合工件、规划及正向新章 |

改过文字不等于改好了问题，因此 review 引起的首次改写后必须检查原 finding。每条 review 链最多一次检查、两次改写；仍未修好的 finding 保留为现行 review 的 open 项，不再自动循环。完成与 `rewrites_drained` 共用双队列边界，见 `internal/tools/completion.go` 和 `internal/domain/runtime.go:ReviewWorkDrained`。用户改文导致源 review 失效时，不得继续派发无对象的检查。


### 7.2 Arbiter（`internal/arbiter/`）

四个场景，每场景一对 `Collect*Facts`（IO 边界）/ `Decide*`（除统一执行器管理的模型请求外无 IO，可离线重放）+ 专属 Decision 类型（场景不匹配的动作在类型上不可表达）：

| 场景 | 触发 | 决策类型 |
|---|---|---|
| `plan_start` | 新书启动 | 选 short/long 规划师 + 扩充过短需求 |
| `intervention` | 用户干预 | answer/rules/hold/reopen/dispatch 组合（执行顺序由 Engine 固定） |
| `worker_failure` | Worker 报错且确定性分类无出路 | retry / reroute / abort |
| `deadlock` | 同指令反复无进展 | retry / reroute / abort |

失败路径：统一结构化执行器按能力选择原生 JSON Schema 或提示词契约；提示词模式的格式/Schema 错误与两种模式的业务校验错误会把精确原因反馈给模型继续修正，直至成功或 `context` 结束，不设置次数上限。原生契约违约、拒答、截断、错误终止及不可重试请求错误立即显式返回；干预不产生写入，启动显式报错，failure/deadlock 保守暂停。**Arbiter 输出与一切 LLM 输出同样不可信**——JSON Schema 校验后，`Validate` 继续按事实做机械校验（phase 约束、reopen 仅限完本、章节越界）。用量经 `usageTrackedModel` 进预算与 usage 系统。

### 7.3 Host 外壳（`internal/host/host.go`）

生命周期（`StartPrepared`/`Resume`/`Continue`/`Steer`/`Abort`/`Close`）、干预编排（FIFO 串行 + PendingSteer 崩溃保护）、事件投影、模型管理。观察通道 `Events`/`Stream`/`Done`，UI 聚合 `Snapshot()`，扩展入口（导入/导出/共创/仿写/模型切换）。

`runEnded`（engine.onDone 回调）按 store 事实定终态：Phase=Complete → completed + 确定性完本总结（不花 LLM 调用）；其余 → idle/paused。**禁止任何"自动续跑"逻辑出现在此**（历史教训 §10 第 5 条）。

---

## 8. 启动、恢复与干预

### 8.1 新建

```
User: "一句话需求"
  → StartPrepared(raw)
    → Progress.Init / Checkpoints.Reset
    → StartPrompt 固化进 RunMeta(输入事实先于裁定落盘)
    → Arbiter plan_start 裁定(选规划师+扩充需求) → 失败显式报错(审计带 error)
    → PlanStartRecord 固化进 RunMeta(裁定先落事实,再起执行)
    → engine.start(首条派单指令)
```

裁定失败不是死局:StartPrompt 已在,之后任何一次恢复/继续都会由引擎补裁(见 §8.2)。

### 8.2 恢复（崩溃后重启）

```
进程启动 → resumeLabel(纯 UI 标签) → 一致性告警 → AdvanceGate 对账
  → PendingSteer 存在 → 同步走干预裁定路径(干预先于续跑生效)后拉起引擎
  → 否则 engine.start(nil):只恢复事实,Route 从 store 重算续跑
```

没有会话需要恢复。规划期崩溃（裁定已落盘、首个 foundation 未落盘）由 `planStartFallback` 按 PlanStartRecord 续派，不重做已有裁定。若启动裁定**从未完成**（启动时模型故障），`planStartFallback` 依据 StartPrompt 现场补裁——这是首次裁定的重试，不违反"恢复不重新裁定"；补裁失败显式暂停告知，不允许无声停机。重复派发安全由工具幂等保证（§5.4）。

### 8.3 用户干预

`Steer`/`Continue` 统一走 Arbiter 裁定路径（`doIntervention`）：

```
持久化 PendingSteer(崩溃保护) → Collect facts → Decide(秒级)
  → 落 decisions.jsonl → answer 回显 / rules 即时落盘
  → hold/reopen/dispatch 入队边界提交(引擎停机时立即执行并视意图拉起引擎)
  → 全部动作成功 → 原子清除 PendingSteer(ClearHandledSteer)
```

崩溃保护是 **best-effort 单在途持久化**：首次 `SetPendingSteer` 失败会显式报错并停止裁定，绝不在无恢复记录时继续执行；裁定期、动作失败（保留待重放）、正常退出/Abort（defer 回存残留派单）受保护。仍有两个明确不保证的窗口——派单转入内存执行队列后被硬杀（毫秒级）、interMu 等待中的并发输入。用户在场可感知，重发成本秒级。

**长效干预的持久层**：写作风格/质量规则由裁定的 `rules` 动作经 `userrules.Service` 归一化进本书规则快照，`novel_context` 注入 `working_memory.user_rules`——跨压缩、跨重启生效（详见 [用户规则快照](user-rules-runtime.md)）。其余出路本就落 store（篇幅/剧情→architect 派单，改旧章→editor 入队 PendingRewrites，完本返工→reopen）。

### 8.4 章节推进控制

`ChapterAdvanceGate` 统一执行两种不同时间尺度的用户意图：

| 意图 | 来源 | 语义 |
|---|---|---|
| `AdvanceMode=review` + 精确 permit | `/review on`、`/next` | 持久政策：每个正向新章必须单独放行 |
| `AdvanceHold` | Arbiter intervention | 一次性意图：当前边界、返工排空或目标章节稳定提交后暂停 |

许可绑定章节号。只有目标章进入 CompletedChapters、PendingCommit 清空且 commit checkpoint 存在才消费，因此提交 saga 任一窗口崩溃都不会把同一许可用于下一章。详细不变量见 [Chapter Advance Gate](chapter-advance-gate.md)。

---

## 9. 目录结构

```
internal/
  domain/         纯数据：Phase / FlowState / Progress / Checkpoint / Scope / Story / Plan /
                  Review / StateChange / Phase-Flow 迁移规则
  store/          文件系统持久化（tmp+rename + 幂等协调；commit 有 Saga 阶段事实）：progress / checkpoints / outline /
                  drafts / summaries / characters / world / signals / run_meta / runtime /
                  session / decisions(裁定审计)
  tools/          11 个 Agent 工具，写类单文件原子 + 显式错误 + 幂等；commit 额外使用持久化 Saga
  flow/           路由策略（纯函数 + IO 边界）：router.go (Route 决策表) + state.go (LoadState)
                  + pause.go (停靠点裁定)
  arbiter/        语义裁定层（LLM-as-function）：plan_start / intervention / failure(deadlock)
                  逐场景 Collect/Decide 函数对 + 逐场景 Decision 类型 + 机械校验
  agents/         build.go 装配三个 Worker(subagent.Runner,Engine 程序化直调)；ctxpack/ Writer 上下文压缩策略
    guard/        subagent_guards.go (CheckpointDeltaGuard ×3,Worker 事实护栏)
  host/           host.go (生命周期/干预编排) + engine.go (确定性执行循环) + observer*.go
                  + events.go + usage*.go + budget.go + advance_gate.go + resume.go + cocreate.go
    imp/          外部小说语义编译导入：ingest → segment → analyze → synthesize → publish（纯状态推导 + LLM 作函数）
    exp/          已完成章节导出：TXT / EPUB 3；纯只读
  entry/          tui (Bubble Tea) / headless / startup
  bootstrap/      config + ModelSet + provider failover + setup 向导
  eval/           离线评测（prompt/voice A/B、回归）
  diag/ errs/ models/ notify/ rules/ userrules/ stylestat/ ...

assets/
  prompts/        arbiter-plan-start / arbiter-intervention / arbiter-failure / architect-short|long
                  / writer(协议模板,{{VOICE}} 占位) / editor / import-* / simulation-*
  voice.md        写作标准(文风层内置默认;三层覆盖见 docs/voice-layer.md)
  references/     写作技巧 + anti-ai-tone + 体裁模板等
  styles/         默认/奇幻/言情/悬疑(用户可覆盖/新增)

../../core/      verse 仓库共享的 AI core：agentcore（通用 Agent 框架）、llm（LLM 接口 + provider）、
                 models / llmretry / llmcontract、auth（OAuth 与凭据）。只依赖 core 自身与第三方库，
                 不 import 任何 app（`archtest/boundary_test.go` 强制）；可加通用能力，不加业务
```

### 9.1 演进里程碑

| 时间 | 重构 | 净效果 |
|---|---|---|
| 2026-04-10 | `internal/orchestrator/` (6342 行) → `host/` + `agents/` | 运行时核心 -74% |
| 2026-04-20 | Hybrid Coordinator：新建 `host/flow/`，路由收归纯函数 | 路由错误率趋近 0 |
| 2026-05-02 | agentcore 慢思考/流式修复；删除 `idleResumeCount` 续跑补丁 | mimo / 慢思考流式跑通 |
| 2026-06-05 | 滚动规划闭环 + `/import` 反推续写 | 200+ 章首次跑通 |
| 2026-07-12 | **Engine + Arbiter 控制面更替**：Coordinator 长循环及七项补丁生态退役；文风层三层覆盖；五轮对抗评审加固 | 每边界省一次 LLM 转发；控制面 100% 离线可测；语义裁定可回放 |
| 2026-07-15 | **`/import` 语义编译管线**：硬编码切分规则退役，改为 ingest→segment→analyze→synthesize→publish 分阶段编译；纯状态推导（`NextAction(Facts)`）+ 输入指纹绑定工件，全程可恢复幂等 | 切分随模型能力自然增强；无漂移阶段枚举；中断可续、控制面离线可测 |

实测：hy3-preview free 12 章 / 73 分钟、mimo-v2.5-pro 10 章 / 8.4 万字，均一次跑完；长篇 gpt-5.4《凡骨》235 章 / 127 万字滚动规划闭环跑通（Coordinator 时代数据，Engine 时代首跑待补）。

---

## 10. 明确不做的事

违反即代表架构偏离。

1. **不引入 Task / Job / WorkItem 概念**。UI 显示的"当前任务"是事件流投影，不是事实。
2. **不在 Route 之外发明第二个调度器**。所有"下一步派谁"必须经 Route 决策表（穷举规格钉死）或 Arbiter 裁定（落盘审计），不允许散落的 if-else 派发。
3. **不做"空闲续跑"机制**。Engine 循环结束 = Host 进入终态；再动起来只有用户 `Continue` 或重启 `Resume`。
4. **不给 prompt 加行为规训**。需要行为护栏说明分层错了——不变量进工具前置条件，判断进 Arbiter，流程进 Route。
5. **不在 Host 为异常停机加自动续跑补丁**。曾经的 `idleResumeCount` 在唯一一次实际触发的长跑里 100% 没救场，反而掩盖了 agentcore 层真因（详见 `feedback_no_host_resilience.md`）。
6. **不基于"tool exec end"推断任务完成**。完成的唯一证据是 checkpoint 写入。
7. **不做 WorkflowInstance / Command + Apply 等四层模型**。事实层只有 Progress + Checkpoint + Artifact。
8. **不支持并行 Worker**。单活跃 Engine 循环，单本书串行推进。多本小说请用多进程。
9. **不在工具层做 LLM 调用**（除 Agent 工具自身）。纯 IO + 校验 + 幂等。
10. **不让 UI 直接读 Store**。只能订阅事件或读 Host `Snapshot()`。
11. **不写 Host 端的 Flow 状态机**。Flow 标签只由工具更新，Route 只读不写。
12. **不为"LLM 幻觉"写兜底硬编码**。优化 prompt、改进工具返回值、让 novel_context 更清楚地呈现事实。
13. **不让 diag / 观察层介入控制流**。诊断只读；自动修复 / 续跑 / 改流程一律不做。
14. **预算与章节推进政策不进 Route/工具层**。`BudgetSentinel` / `ChapterAdvanceGate` 是 Engine 边界的政策组件（执行用户预先签署的指令，不评估文学行为）；`notify` 纯观察。
15. **控制面改动必须先改穷举规格再改实现**；**bump agentcore 前必须过契约测试**。
16. **不做通用工作流 DSL、事件溯源、全局 State Digest**。Route 是一个领域一张表，泛化即过度设计。

---

## 11. 验证策略

### 11.1 测试资产清单

| 层 | 资产 | 覆盖 |
|---|---|---|
| 控制面规格 | `flow/router_exhaustive_test.go` | Route 决策表 12 万组合穷举 + 纯函数/确定性/守恒性质 |
| 框架契约 | `agents/agentcore_contract_test.go` | 5 条 agentcore 行为假设，经 `Runner.Run` 驱动（升级前必跑） |
| 引擎端到端 | `host/engine_test.go` | fake 模型 + 真实工具:写完整书 / 失败裁定 / 僵局裁定 / 返工验收时序 / boundary hold 即停 / 退出竞态保全 / 单许可单章节 |
| 裁定 | `arbiter/arbiter_test.go` | 解析/反馈重试/逐场景校验矩阵/事实采集 |
| 事实管道契约 | store/tools 测试 | 反馈池跨重启、违规记录 latest-wins/重写清除/novel_context 注入、PlanStart 跨 Init 保留 |
| 文风层 | `assets/load_test.go` | 拆分逐字节一致 / 三层覆盖语义 / eval 同组装路径 |
| 语义质量 | `internal/eval` + decisions.jsonl | prompt/voice A/B、裁定离线重放（回归集建设中） |

### 11.2 稳定性场景

- **A 长跑**：80~200 章一次跑完，Phase=complete。允许 provider failover、重试；禁止任何自动续跑。
- **B 崩溃恢复**：任意 step 后 kill 进程 → Resume → Route 从事实续跑，不重写已落盘产物，checkpoints 无重复 step。规划期崩溃走 PlanStartRecord。
- **C provider 抖动**：间歇 503 → litellm failover，Worker 无感知。
- **D 用户干预**：运行中 Steer → 秒级裁定回显、动作边界提交；停机 Steer → 裁定后按意图拉起；崩溃 → PendingSteer 重放。

### 11.3 合规性（可写成 linter / test）

- `flow.Route` 必须纯函数：禁止读 Store / 任何 IO
- `runEnded` 函数体内不允许出现任何启动引擎的调用
- 新裁定场景必须成对新增 Collect/Decide + Decision 类型 + 落盘
- recovery 相关代码只能出现在 `host/resume.go` 与 `engine.planStartFallback`

### 11.4 质量迭代

改文风 → 改 `<书目录>/style/`（用户级）或 assets/voice.md（内置），文风评测集 A/B 验证；新增评审维度 → 改 editor.md（save_review 结构化接收）；新增参考资料 → 三处显式接线（`tools.References` + `loadReferences` + novel_context 注入映射）。

**全书级风格统计（`internal/stylestat`）**：Host 为每本书创建唯一 `StyleStatsIndex`，并显式注入 `novel_context` 与 `commit_chapter`。首次启动从全部已完成章节恢复索引，后续对新增/重写章节增量更新（句式模式/高频短语/跨章重复句/章末形态），相同书状态下复用快照并注入 `episodic_memory.style_stats`：editor 按数字裁定，writer 据此自避免。离线 eval 仍可直接调用纯函数 `Compute`。**统计归代码，裁定归 LLM**。

---

## 12. 部署形态（hosted）

- **两种 facts 后端**：`STORE_DRIVER=file`（CLI/离线，目录即书库）与
  `STORE_DRIVER=postgres`（hosted，actor + membership 在事务内解析）。没有
  隐式回退；hosted 启动即校验 schema 版本，缺少迁移直接失败。
- **健康检查**：`/healthz` 是 liveness（进程能响应 HTTP，不碰依赖）；
  `/readyz` 检查数据库/schema 版本与 BlobStore 可达性，失败返回 503。
  `/healthz` 先于 SPA 静态路由处理，避免 index.html 造成 200 假象。家中
  Mac 的 TTS worker 不在 readiness 内：掉线是队列退化，重启容器无益。
- **代理边界**：只有 `TRUSTED_PROXIES` CIDR 内的 peer 可以设置
  `X-Forwarded-For/Proto`；client IP 从右向左穿过可信 hop。`PUBLIC_URL`
  是绝对链接与允许 origin 的来源；`SECURE_COOKIES=true` 时 cookie 恒带
  Secure，跨站写请求返回 403。
- **两个监听**：公开 mux 由 Traefik 转发（容器 8080 不发布）；worker mux
  绑定 `WORKER_LISTEN_ADDR`（host 只发布 Tailscale IP），token 在数据库
  校验，公开路由永远不注册 worker handler。
- **Blob**：`STORAGE_DRIVER=local|s3` 与 facts 后端独立；private bucket 只通过
  授权 + presign redirect（S3）或鉴权流式（local）交付，没有公开 URL 通道。
- 运维手册（Dokploy、备份/恢复、cutover 与回滚）在 `deploy/README.md`。

---

## 13. 总结

> **事实层确定，语义层自主。**模型自由在验证不可能的地方（写什么、怎么写、怎么判），被约束在验证可能的地方（顺序、幂等、阶段）。

没有 task queue，没有 policy engine，没有常驻会话。有的只是：

- 一个串行确定性 Engine 循环（~500 行，六条端到端路径钉死）
- 一张 Route 决策表（纯函数，12 万组合穷举规格）
- 四个 Arbiter 裁定函数（事实进、结构化决策出、落盘可回放）
- 三类职能 Worker（context 与模型独立，事实护栏零打扰）
- 11 个单文件原子、跨文件显式失败/幂等恢复的工具；其中 commit 使用持久化 Saga + 一个 jsonl checkpoint 文件

模型升级的收益流向何处一目了然：创作更好（Writer/Architect/Editor 的全部输出）、裁定更准（Arbiter 四场景）、摘要更好（ctxpack）——全部换模型即得，外壳一行不改。控制面不吃模型红利，因为**查表不需要智力**；它需要的是被证明正确，而它已经被证明了。

流程刚性是有意的、标了价的、留了门的：想放开 writer 的工具顺序 → 松一段协议 prompt（不变量在工具层兜底）；想按弧派发 → Route 加一行分支；想扩裁定能力 → 加一对 Collect/Decide。每一次松绑都有裁判（穷举规格、文风评测、decisions 回放）——**用证据决定给模型多少绳子，而不是用信仰**。

唯一的纪律：**有人想加一个决策点时，先过三分法——可枚举的进 Route，边界清晰的进 Arbiter，开放式的进 Worker**。三者都不是的决策，重新想清楚它是不是真的存在。
