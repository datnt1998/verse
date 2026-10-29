# ainovel-cli 评测体系

> 评测不是新造一套检查脚本，而是把项目**已有的事实诊断器（`diag`）、全书文体统计器（`stylestat`）、七维原生评审（`ReviewEntry`）当作评测器**，套一层离线批量 harness。一份事实定义，两处不再漂移。

---

## 0. 为什么需要重新设计

稳定性已经跑通：长篇 235 章 / 127 万字一次写完，滚动规划闭环成立（见 `architecture.md` §9.1）。瓶颈已经转移——**质量可迭代**：

- 改一个 prompt 后，流程是否仍稳定？工具链、状态推进、持久化事实是否还正确？
- 正文、大纲、评审质量是真的提升了，还是只是这一次随机抽到了好结果？
- 长篇里角色、时间线、伏笔、上下文是否持续可靠？
- **全书级的文体固化**（句式 tic 章均几十次、章末形态同构、跨章逐字复读）有没有变好或变坏？这是 196 章实证 6.5/10 的真凶，单章评审对它天然失明。

目前这些判断靠"凭感觉 + 人工抽读"。评测体系要把 prompt 改动从凭感觉变成**有回归、有证据、有人工读样**的工程流程。

但本项目不需要、也不应该照搬业界通用 eval 平台（dataset / experiment / scorer / 数据库 / Web UI）。原因很简单：**这些能力的核心——确定性检查与质量信号——项目里已经存在，且是 Go 写的、与运行时共享同一份事实模型。**

---

## 1. 核心论点：评测器已经存在

评测系统的四类评测器，三类在代码库里已经实现，只是从未被当作"评测器"调用：

| 评测器 | 项目已有能力 | 入口 | 产出 |
|---|---|---|---|
| **确定性事实诊断** | `internal/diag` 的一组工件规则 + 运行时规则 | `diag.Diagnose(store)` | `Report{Stats, Findings}`，Finding 带 Severity/Evidence |
| **全书级文体回归** | `internal/stylestat` | `stylestat.Compute(input)` | 句式模式章均、跨章重复句、章末短句占比、标题混用 |
| **质量裁定（rubric）** | 版本化 rubric（初始派生自 `editor.md` 七维） | LLM Judge（固定标尺做 A/B） | consistency/character/pacing/continuity/foreshadow/hook/aesthetic |
| **行为脱敏导出** | `internal/diag` 导出 | `diag.WriteExport(store, rep, rc)` | 行为骨架，供人工读样与归档 |

`diag.Analyze(s *store.Store)` 接收一个 Store 就能产出完整 `Report`——**它本来就能离线跑在任何产出目录上**。`stylestat.Compute` 是纯函数。这意味着评测系统要做的不是重新实现"章节是否落盘、progress 是否推进、checkpoint 是否存在、有没有 pending 残留、流程有没有死循环"——这些 diag 全做了，而且每条规则都对应一个踩过的真实坑（`PhaseFlowMismatch`、`OrphanedSteer`、`OutlineExhausted`、`repeatedErrors`/`stuckStep` 对应 idleResume / 大纲耗尽 livelock / 工具调用当文字打印等历史故障）。

> **评测系统的工作不是造检查，而是：批量驱动 + 把已有评测器跑在产出上 + 把 Finding/统计映射成门禁 + 聚合报告。**

---

## 2. 设计原则

### 2.1 评测器即诊断器，绝不重造确定性检查

确定性检查只调用 `diag.Diagnose`，不在评测层重新解析 `progress.json` / `checkpoints.jsonl` / `sessions/*.jsonl`。理由是这个项目的 DRY 铁律：**"什么是合法状态"只能有一份定义。** 如果评测用 Python 重新解析一遍 checkpoint 判断 commit 是否缺失，就有了两份"commit 完成"的定义，运行时改了 diag 规则、评测不跟着改，门禁立刻失真。

→ 评测 harness 用 **Go**，in-process 调用 `diag` 与 `stylestat`，与运行时共享 `internal/domain` 与 `internal/store`。这是本设计与上一版最根本的区别。

### 2.2 全书级文体回归是第一质量信号

单章 LLM Judge 看每一章都"正常"，但瓶颈恰恰是跨章固化。所以质量回归的**确定性骨干是 `stylestat`，不是 LLM Judge**。

**前提：`stylestat.Compute` 少于 5 章直接返回 nil**（`stylestat.go` `minChapters=5`，样本太小频率无意义）。因此文体回归**只在 ≥5 章的 Quality / Longform 层生效**，1 章的 Smoke 拿不到文体信号——这点决定了下文成本与默认策略。指标包括：

- variant 的句式模式章均次数 vs baseline（`patterns[].per_chapter`）
- 章末短句收尾占比（`ending.short_ratio` 逼近 1 是病）
- 跨章逐字重复句条数（`repeated_sentences`）
- 标题格式混用（`title_formats`）
- 开篇时间词率（`opening_time_rate`）

这些是零 LLM 成本、确定性、且正打在质量瓶颈上的指标。**LLM Judge 是补充，stylestat delta 是主线。**

### 2.3 LLM Judge 对齐七维原生 rubric，不另起炉灶

Judge 不发明新评分维度——维度严格等于 `domain.DimensionScore` 的七项，做 baseline/variant 比较。

**但 rubric 必须版本化、可固定**，存为 `evals/rubrics/*.json` 的快照，不是运行时实时读 `editor.md`。原因：当被测对象正是 `editor.md` 本身时，若裁判跟着 `editor.md` 一起变，评测基准就漂了——裁判和被测同源会让"改 editor 是好是坏"无从判断。所以 rubric 初始**派生**自 editor 七维（保证口径一致），之后**独立演进、显式 bump 版本**；report 里记录用的是哪版 rubric。

### 2.4 确定性 Finding 决定门禁，LLM 与人工只做质量裁定

对齐架构铁律"统计归代码，裁定归 LLM"：

- **能阻塞合入的只有确定性证据**：`diag` 的 `SevCritical` Finding、case 声明的契约断言失败。
- **LLM Judge 与人工读样产出 warning 与排序线索**，不单独决定合入。
- 一句话：`Finding.Severity` 直接映射门禁等级，不引入新的严重度分类法。

### 2.5 评测只观察，不介入控制流

评测复用 `diag`，但**丢弃 diag 的 `Action` 与 `Planner`**——那是运行时控制流的东西。在评测语境里 `diag.Report` 只取 `Stats` 与 `Findings`，Action 一律忽略。评测不自动修 prompt、不自动回滚、不续跑。这是观察者纪律（`architecture.md` §2.3）在评测语境的延伸。

### 2.6 失败显式暴露

不 mock 成功、不吞错误、不用模板假装通过。模型、工具、配置、文件系统、解析、judge 任一失败，报告显式记录原因。**失败本身就是评测结果**——一个 case 跑崩了，门禁就是 FAIL，不是"跳过"。

### 2.7 每次只验证一个变量

A/B 的硬约束：同需求、同配置、同模型/provider、同风格、隔离输出目录。Baseline = 当前正式 prompt，Variant = 只替换本次要验证的 prompt 文件。一次实验不要同时改 Writer/Architect/Editor/Arbiter。

---

## 3. 架构全景

```text
[Cases]  evals/cases/*.json —— 事实层断言集，不是通用 dataset 行
   │
[Runner]  internal/eval —— in-process 装配 host 驱动（按章数上限截停），bundle.Prompts 内存覆盖做 variant
   │       baseline run ┐
   │       variant  run ┘  各自隔离 output 目录
   ▼
[Collectors]  对每个产出目录采集：
   ├── diag.Diagnose(store)      → Report{Stats, Findings}      （事实 + 运行时）
   ├── stylestat.Compute(input)  → 全书文体统计                 （质量回归骨干）
   ├── case 契约断言             → 期望 checkpoint/phase/工具契约（diag 不覆盖的）
   ├── usage / cost / token      → 从 meta/usage.json 读
   └── tool_calls                → 从 meta/sessions/*.jsonl 读真实工具调用
   ▼
[Graders]
   ├── 确定性门禁：Finding.Severity + 契约断言 → hard_fail / regression
   ├── stylestat delta：variant vs baseline 文体指标差
   ├── LLM Judge（可选）：七维 rubric A/B 比较
   └── Human：人工读 baseline/variant 产物
   ▼
[Report]  report.json（机读）+ report.md（人读）+ 行为脱敏导出
   └── Gate: PASS / WARN / FAIL
```

依赖方向：`eval → host → agents → tools → store → domain`，横向复用 `diag` / `stylestat`。评测层**不反向依赖**运行时控制流，只读 Store 与只读评测器。

> **当前实现覆盖确定性主线**：无 `--variant` 时为 `mode=single`；传 `--variant` 时为 `mode=ab`，同一 case 隔离运行 baseline 与 variant，并生成 delta。Runner 有两条路径：整本 pipeline（按章数上限截停）与 **fixture 模式**（seed 固定书稿后只跑一条 editor/writer instruction，无章数上限），都直接 `host.New` 装配，**不复用无章数上限的 `headless.Run`**。Collectors 已接 `diag.Diagnose`、case 契约、`stylestat.Compute`、`meta/usage.json`、session 工具计数/错误计数、评审（含旧轮次）、章节正文与事实；确定性断言入口见 [case.go](../internal/eval/case.go)、[assert_review.go](../internal/eval/assert_review.go)、[assert_prose.go](../internal/eval/assert_prose.go) 与 [assert_context.go](../internal/eval/assert_context.go)。目标检查清单（`goal_status`）、复审闭环（`recheck_status`）及实际上下文（`context_contains`）已纳入契约门禁；LLM Judge 仍未接入，不参与当前确定性门禁。

---

## 4. 为什么是 Go in-process，不是 shell + Python

| 维度 | shell 拷源码 + Python 解析（旧路） | Go in-process（本设计） |
|---|---|---|
| 确定性检查 | Python 重新解析 JSON，与 diag 规则两份定义 | 直接 `diag.Diagnose(store)`，一份定义 |
| variant 切换 | 拷整个源码树 + 重新 `go build` 两个二进制 | `bundle.OverridePrompt(...)` 内存覆盖后装配 host，零拷贝零重编译 |
| 文体回归 | 需在 Python 重写 stylestat 的中文分句逻辑 | 直接 `stylestat.Compute` |
| Judge rubric | 维度散落在 Python | 复用 `domain.DimensionScore`，与线上同源 |
| 漂移风险 | 高：运行时改了事实模型，评测不跟 | 低：编译期就会暴露字段变更 |

旧 `prompt_ab.sh` 之所以要拷源码重编译，是因为 prompt 是嵌入二进制的（`go:embed`）。但 `assets.Bundle.Prompts` 是普通结构体，**runner 在内存里改一个字段就能做 variant**，根本不需要拷源码。这是用 Go 写 harness 顺带拿到的最大简化。

> **实现约束**：`assets.Load` 经 `loadPrompts` 给 Worker prompt（architect/writer/editor）统一追加 `WithSimulationGuidance` 后缀。若 variant 只把裸文本塞进 `bundle.Prompts.Writer`，就丢了 baseline 有的仿写画像后缀，A/B 不等价。
>
> 正确做法是通过 `assets.OverridePrompt` 覆盖，内部走与 `Load` 完全相同的包装；eval 不复制包装逻辑。

> 上一版文档保留 `prompt_ab.sh` / `prompt_ab_report.py` 并"逐步抽取能力"。本设计放弃这条路：它们解决的问题（隔离运行 + 指标汇总）在 in-process Go harness 里是子集，强行复用反而背上 shell/Python/Go 三语言的接口胶水。**Go harness 是唯一主路**；当前 Go harness 已覆盖 baseline/variant 隔离运行、repeat 汇总与确定性 delta。旧脚本（`scripts/prompt_ab.sh`、`scripts/prompt_ab_report.py`）及其操作手册 `docs/prompt-ab.md` 已随本设计落地一并删除，不再保留。

---

## 5. Case Manifest

Case 是评测输入的最小单位，也是一组**事实层断言**。用 JSON 描述，避免规则散落在命令行参数里。

```json
{
  "id": "writer_first_chapter_xianxia",
  "category": "smoke",
  "role": "writer",
  "description": "验证 Writer 第一章正文质量与工具链稳定性",
  "prompt": "写一本修仙长篇，主角从边城杂役起步，靠异常记忆破解宗门旧案卷入长生局。",
  "style": "fantasy",
  "max_chapters": 1,
  "target_prompts": ["writer.md"],
  "rubric": "writer_chapter",

  "expect": {
    "phase": "writing",
    "min_completed_chapters": 1,
    "required_checkpoints": ["chapter:1:plan", "chapter:1:draft", "chapter:1:commit"],
    "no_pending": ["pending_commit", "pending_steer"]
  },

  "gate": {
    "max_severity": "warning",
    "max_cost_delta_ratio": 0.3,
    "max_tool_call_delta_ratio": 0.3,
    "stylestat_regression": "warn"
  }
}
```

**字段语义**：

- `expect`：case 级契约断言，**只声明 diag 通用规则覆盖不到的、与本 case 强相关的预期**（比如"这个 smoke case 必须恰好产出 chapter:1:commit"）。通用的"无 pending 残留 / phase-flow 一致 / 无章节缺口"交给 diag，不在 case 里重复声明。
- `category`：评测层 ∈ `smoke` / `workflow` / `quality` / `longform` / `recovery` / `steering`。决定跑哪套门禁与默认是否开 stylestat/Judge。
- `role`：被测的角色 ∈ `writer` / `architect` / `editor`。与 `category` 正交——层决定"验到什么深度"，角色决定"验哪个 Worker"。Workflow 层按 `role` 选断言集。
- `max_severity`：diag Finding 允许的最高严重度。超过即 hard fail。
- `gate.max_cost_delta_ratio` / `gate.max_tool_call_delta_ratio`：variant 相对 baseline 的成本与工具调用增幅阈值；省略时默认 `0.3`，显式 `0` 表示不允许增长，负数表示关闭该项 delta gate。
- `rubric`：启用哪个版本化 LLM Judge 评分表。缺省则不跑 Judge。
- `gate.stylestat_regression`：`block` / `warn` / `off`，控制文体回归是否阻塞（仅 ≥5 章 case 生效）。

**Fixture case 追加字段**（全部可选，只有 `prompt` 的老 case 仍是合法 case）：

```json
{
  "id": "ed-timeline-contradiction",
  "category": "quality",
  "role": "editor",
  "fixture": "ed-timeline-contradiction",
  "instruction": { "agent": "editor", "task": "Thẩm duyệt chương 3: gọi novel_context(chapter=3), save_review …" },
  "control": false,
  "expect": {
    "review": {
      "verdict_in": ["polish", "rewrite"],
      "must_find": [{ "severity_min": "error", "quote_contains": "trời đã tối" }],
      "must_not_find": [{ "severity_min": "error", "type_contains": "timeline" }],
      "goal_status": { "rb1": "pass" },
      "recheck_status": { "trời đã tối": "unfixed" }
    },
    "prose": {
      "chapter": 3,
      "forbid_regex": ["(?i)sáng hôm sau"],
      "require_regex": ["đồng hồ"],
      "max_opening_overlap_runes": 60
    },
    "facts": { "chapter": 3, "forbid_contains": ["đã đến"] }
  }
}
```

- `fixture`：`evals/fixtures/<name>` 的目录名。只接受 `^[a-z0-9-]+$`，**不接受路径**（含 `/`、`..` 一律拒）。
- `instruction`：fixture 模式下唯一要跑的 Worker 任务，`agent ∈ editor|writer`。`task` 应照抄 Router 给该 Worker 的原话（换掉章号），评测的才是线上同一段指令。有 `fixture` 时 `instruction` 必填、`prompt` 忽略；没有 `fixture` 时 `instruction` 非法。
- `control`：对照 case——正确答案是**除 `must_find` 声明的缺陷外不报问题**：未被任何 `must_find` 命中的 error 以上 issue 都计为 `false_positive`（例如正文夹带指令的对照 case 仍要求抓到真实的时间线错误）。
- `expect.review`：对落盘 `ReviewEntry` 的断言。`severity_min` 的口径是 `warning < error < critical`（与 diag 的 info/warning/critical 不同，case 只认缺陷档），缺省 `error`。引文比对会做归一：小写、弯/直引号合一、空白压缩；标点仍参与比对。
- `expect.prose`：目标章正文。`forbid_regex` 命中即 FAIL、`require_regex` 未命中即 FAIL；`max_opening_overlap_runes > 0` 时，本章开篇与上一章结尾的最长连续重复 rune 数不得超过该值（口径见 `internal/rules/overlap.go`）。
- `expect.facts`：目标章 `ChapterFacts` 摊平后不得包含 `forbid_contains` 里的文字。
- **正则限制**：每个 case 最多 20 条（`forbid_regex` + `require_regex` 合计），每条 ≤ 200 字节，标准库 `regexp`（RE2），加载时编译。正则只用于**唯一词法标记**（专有地名、称呼、标记词），不要拿来判文笔——模型换一个同义词就会让它失效，这正是 case 变脆的常见原因。
- `expect.review.goal_status` / `recheck_status` 的断言权威是 [gradeReviewExpect](../internal/eval/assert_review.go)：分别核对最新 chapter review 的目标 ID，以及按 finding ID 或源轮次 quote 指定的检查结果；缺失或不符是 FAIL，不是 SKIP。Fixture 的源 review 和待检查队列由 [fixtureDoc](../internal/eval/fixture.go) 的 `reviews` / `pending_rechecks` 接入。
- `expect.context_contains` 由 [gradeContextExpect](../internal/eval/assert_context.go) 核对本次 session 中模型实际收到的 `novel_context` 结果，不从 Store 另造一份上下文；缺失即 hard fail。Eval 在 worker 启动前启用窄范围的原文记录；生产 session 仍按原策略压缩。旧工件的压缩占位符与失败 tool 结果不算真实 context，记入 `context_missing`。字段和加载约束以 [Expect](../internal/eval/case.go) 为准。

### 5.1 Fixture mode（固定书稿 + 单条 instruction）

fixture 模式把"让模型自由写一本书"换成"在一本已知的书上跑一条指令"：同一份书稿、同一条 instruction，重复运行只暴露模型侧差异。评测的是 Editor 的评审判断或 Writer 的单章产出，不是流水线能否跑通。

目录形状：

```text
evals/fixtures/<name>/           # <name> == case id
  fixture.json                   # ≤ 64 KB
  chapters/NN.md                 # 两位数章号，每章 ≤ 64 KB，最多 12 章
```

`fixture.json` 的键直接解码成 `internal/domain` 的既有类型（未知键报错，不静默忽略）：

```json
{
  "title": "…", "premise": "…", "total_chapters": 8,
  "outline": [], "characters": [], "world_rules": [],
  "completed": [{ "chapter": 1, "summary": {} }],
  "timeline": [], "foreshadow": [{ "chapter": 1, "updates": [] }],
  "state_changes": [], "chapter_plans": []
}
```

- Editor fixture：被评审的是**最后一个已完成章**。Writer fixture：目标章 = 最后完成章 + 1，它的 plan 放在 `chapter_plans`。
- 路径安全：fixture 目录经 `filepath.EvalSymlinks` 解析后必须仍在 fixture 根下；目录内出现 symlink 或非普通文件直接报错；fixture 目录与本次 `OutputDir` 互不包含。
- 落盘只用 store API（`Progress.Init/UpdatePhase/MarkChapterComplete`、`Outline.SavePremise/SaveOutline`、`Characters.Save`、`World.SaveWorldRules/AppendTimelineEvents/UpdateForeshadow/AppendStateChanges`、`Drafts.SaveChapterPlan/SaveFinalChapter`、`ChapterRecords.Save`（带 `ContentSHA256`）、`Summaries.SaveSummary`），因此 fixture 书稿与线上产物的形状完全一致。
- seed 之后立刻跑 `diag.Diagnose`：有任何 finding 比本 case 的 `gate.max_severity` 更严重就中止。**fixture 自己都不干净时，后续比较没有意义。**
- fixture 根目录默认 `evals/fixtures`，可用 `--fixtures <dir>` 覆盖；解析规则就是"相对当前工作目录，或显式给出绝对路径"，没有隐藏的搜索路径。
- 运行顺序：清空 `OutputDir` → seed → `host.New` → `RunInstructionOnce(agent, task)` → drain 到 `Done`。**不使用章数上限、不跑 `PrepareQuick`、不跑 Arbiter**：已经 seed 了 10 章也不会被"章数到了"截停。Worker 自己报错时引擎同样正常收场，所以 runner 在 `Done` 之后必须再读快照里的单指令结果，否则会把失败记成成功。

---

## 6. 评测分层

每一层明确**用哪个已有评测器**，避免"评测层自己又写一遍判断"。

### 6.1 Smoke（每次 prompt 改动必跑，最小集）

只判断系统是否还能稳定跑，不判文笔。1 章 / 规划阶段即可暴露。

| case | 目标 | 主要评测器 |
|---|---|---|
| `writer_first_chapter` | Writer 完成第一章并 commit | `expect.required_checkpoints` + diag |
| `architect_short` | 短篇规划存全 premise/outline/characters/world_rules | diag `MissingSummaries` 同源的 foundation 检查 + `expect` |
| `architect_long` | 长篇规划存 layered_outline/compass，首弧展开 | diag `OutlineExhausted`/`CompassDrift` + `expect` |
| `editor_review` | 到评审点 Editor 存 review（七维齐全） | `ReviewEntry` 字段断言 |

成本：1 章 × baseline+variant，秒级到分钟级，不开 Judge、不跑 stylestat（章数不足 5，`Compute` 返回 nil）。CI 默认只跑这层。

### 6.2 Workflow（验证 Agent 行为符合架构契约）

**关键纪律：断言契约，不断言精确工具序列。** 架构押注 LLM 自主决策流程（`architecture.md` §2.1），把工具顺序写死会在评测层重新引入被 §10.13 拒绝的"为 LLM 行为写硬编码"。所以这里只断言**必然事实**：

- Writer：`chapter:N:commit` checkpoint 存在；commit 后子代理本轮结束（无超长尾随正文）；draft checkpoint 先于 commit。**不**断言"必须 novel_context→read_chapter→plan→draft→check→commit 这个精确顺序**。
- Architect：写作期 outline 只增不全覆（`expand_arc`/`append_volume` 的 checkpoint，没有第二条 `layered_outline` 全量写）；展开后扁平 outline 与 layered 章节数一致。
- Editor：`ReviewEntry.Verdict` 合法（accept/polish/rewrite）；rewrite/polish 必须产出 affected chapters；弧末有 `arc_summary`、卷末有 `volume_summary` checkpoint。
- Engine 派发：Route 指令与实际运行的 Worker 匹配（从 session trace 读，diag `repeatedErrors` 兜底循环）；语义裁定核对 `meta/decisions.jsonl`。

这些大部分能直接由 diag 规则 + checkpoint 断言覆盖，少量（commit 后尾随正文）需在 collector 里加一条轻量 trace 检查。

### 6.3 Quality（流程通过后才跑，评内容质量）

两条腿：

1. **stylestat delta（确定性，主线）**：variant vs baseline 的文体指标差。这是质量回归的硬证据。**要求 case 跑满 ≥5 章**（否则 `Compute` 返回 nil，此项标 `insufficient_sample`），所以纯 1 章 Quality case 拿不到文体回归，需把 `max_chapters` 设到 5 以上。
2. **LLM Judge（辅助）**：七维 rubric A/B（见 §8）。

只有 §6.1/§6.2 通过的 case 才进 Quality——流程都不对，谈质量没意义。

### 6.4 Longform & Recovery（重大改动 / nightly）

不必每次跑。覆盖长篇稳定性与恢复能力，正是 diag 运行时规则与 context 规则的主场：

- 前 3 章 / 5 章连续写作 → diag `GhostCharacter`/`TimelineGaps`/`RelationshipStagnation`/`ChapterGaps` + stylestat 跨章重复。
- 弧末评审 + 下一弧展开 → `OutlineExhausted`/`StaleForeshadow`/`CompassDrift`。
- 用户中途干预（steering case）→ user_rules 是否落 `meta/user_rules.json`、是否被后续章节遵守。
- 崩溃恢复：跑到第 N 章 draft 后 kill → Resume → diag 确认 `checkpoints.jsonl` 无重复 step、不重写已落盘 draft、`pending_commit` 最终清零。
- 工具调用膨胀 / 成本异常 → diag `repeatedErrors`/`stuckStep`/`streamIdleStorm` + usage delta。

---

## 7. 确定性门禁

门禁等级由 **diag Finding 的 Severity** + **case 契约断言** 直接派生，不另立分类法。

### 7.1 Hard Fail（阻塞合入）

- 进程 panic / headless 返回 error。
- diag 产出 `SevCritical` Finding（`InvalidPendingRewrites` / `PhaseFlowMismatch` 等）。
- case `expect` 契约断言失败：缺 commit checkpoint、phase 未达预期、声明的 pending 未清零。
- variant 的错误数 / critical Finding 数多于 baseline（回归到更坏）。

### 7.2 Regression（默认 warning，是否阻塞由 case gate 决定）

- diag 新增 `SevWarning` Finding（variant 比 baseline 多）。
- tool calls / cost / input token / output token 增幅超过 case 阈值（默认 30%）。
- **stylestat 回归**：句式模式章均次数上升、章末短句占比上升、跨章重复句增多、标题混用出现——按 `gate.stylestat_regression` 决定 warn/block。
- 章节字数低于 baseline 60% 或高于 180%（diag `WordCountAnomaly` 同源阈值）。

### 7.3 Quality Gate（人工兜底）

- LLM Judge 只做辅助与排序。
- Judge 判 variant 明显更差 → 必须人工读样确认。
- 人工读样认定退化 → 阻塞。
- Judge 判 variant 更好但确定性有 hard fail → 仍阻塞。

### 7.4 推荐合入条件

日常改 prompt：Smoke 全过 + 目标角色 Workflow 全过（Smoke 1 章不含文体回归；若跑了 ≥5 章 Quality case，则 stylestat 无明显回归）。
重大改动：再加 2-3 个 Quality case + 1-2 个 Longform case + 人工读样。

### 7.5 Case 通过率与质量指标

`--repeat N` 时单个 case 的结论是**通过率 k/N**，而不只是最后一次的结果：`Outcome` 仍取 N 次里最坏的一次（CI 退出码需要"有一次炸了就算炸"），但 A/B 判断只看通过率。一次运行"通过"指没有 hard fail：WARN 运行的断言全部成立，计入 k；其 warning 仍单独计入 warning 次数（fixture 模式下 editor 正确判 `rewrite` 后，书稿诊断必然报"待重写"之类的 warning）。

报告的指标彼此独立，刻意不合成单一得分——一个总分既看不出漏报，也看不出误报：

| 指标 | 定义 |
|------|------|
| `mechanical_valid` | 声明了 `expect.review` 的运行里，真的落盘了可用 review 的比例 |
| `critical_hit` | `must_find` 条目被真正抓到的比例（漏报就是 FAIL） |
| `false_positive` | `must_not_find` 违例数 + 对照 case 上 error 级 issue 数（同一条 issue 只算一次） |
| `unknown_correct` | 金标 `goal_status` 要求 `unknown` 的目标项命中比例（不含 recheck）；没有这类期望时为 `N/A`，缺失 ID 仍计入分母。归属：[gradeReviewExpect](../internal/eval/assert_review.go)、[report.go](../internal/eval/report.go) |
| `writer_violations` | `expect.prose` / `expect.facts` 断言违例数 |
| `tool_errors` / `worker_runs` | 从 session 日志计数：工具结果标了 `is_error` 的条数 / 每个 agent 的运行文件数 |
| `context_trims` / `context_missing` | 前者统计实际 context 的 `_trimmed` 含 `previous_tail` 的次数；后者统计无法测量的结果（旧压缩占位符、空结果、工具失败）。有缺失时不能将 trims 的零值解读为未裁剪；报告标记 `N/A`。归属：[collect.go](../internal/eval/collect.go) |
| `quote_coverage` | 只统计本轮 session 中成功 `commit_chapter` 接纳的 foreshadow update 与 state change 携带 quote 的比例（不含 timeline）；排除 seeded 历史及 editor-only run。没有可统计条目时为 `N/A`。归属：[collectQuoteCoverage](../internal/eval/collect.go) |
| `tokens` / `cost_usd` / `usage_missing` | 从 `meta/usage.json` 累加；`usage_missing > 0` 时成本与 token 标注 `không kết luận được` |

A/B 决策规则（所有阶段通用）：

- 只有**在 dev 集上变好、且 held-out 集不变差**的整体指标才算改进；held-out 只在 baseline 与最终验收时各跑一次。
- 单个 case 只接受通过率差 ≥ 2/3 的变化（3 次里 3 胜 0 负、或 2 胜 0 负这类）；更小的差异写"không phân biệt được"，不要当结论。
- 功能在 baseline 提交上还不存在的指标，写"N/A ở baseline"，不参与 delta 计算。

---

## 8. LLM Judge

Judge 是质量辅助，本质是**用版本化 rubric（初始派生自 editor.md 七维）离线做 baseline/variant 比较**。rubric 是固定标尺、独立于线上 `editor.md` 演进（理由见 §2.3），report 记录所用 rubric 版本。

### 8.1 输入（控制大小，绝不塞整本书）

- 用户原始需求 + 当前章节大纲/契约。
- baseline 与 variant 的**同一章**正文。
- 最近 1-2 章摘要 + 角色状态摘要（从 store 读）。
- 该章的 stylestat 相关切片（让 Judge 看到"这句在全书重复了 7 次"这类事实）。

### 8.2 输出（结构化，对齐七维）

```json
{
  "scores": {
    "consistency": 8, "character": 7, "pacing": 8, "continuity": 8,
    "foreshadow": 7, "hook": 7, "aesthetic": 6
  },
  "winner": "variant",
  "confidence": "medium",
  "reasons": ["variant 行动推进更集中", "baseline 前情复述更重"],
  "risks": ["variant 配角动机铺垫略少"]
}
```

- 维度严格等于 `domain.DimensionScore` 七项，每项 0-10。
- `winner` ∈ baseline/variant/tie；`confidence` ∈ low/medium/high。
- `reasons`/`risks` 每条 ≤ 80 字，引用原文要短。

### 8.3 边界

Judge **不能**：决定流程是否通过、修改产物、自动改 prompt、作为唯一合入依据、生成长篇原文摘录。
Judge **可以**：给人工评审排序、标出明显退化、总结 A/B 差异、暴露 prompt 改动副作用。

---

## 9. 报告

每次实验生成 `report.json`（机读，可重生 markdown）+ `report.md`（人读）+ `artifacts/{case_id}/{baseline,variant}/`（原始产物）。`--repeat N` 时路径为 `artifacts/{case_id}/rN/{baseline,variant}/`。

### 9.1 指标 delta

报告显示 variant 相对 baseline 的差异，绝对值与比例并列：

```text
completed: baseline=5 variant=5   ← ≥5 章，文体指标才有意义
tool_calls: baseline=12 variant=16  +4 (+33.3%)
cost_usd: baseline=0.42 variant=0.55  +0.13 (+31.0%)
output_tokens: baseline=8200 variant=9100  +900 (+11.0%)
critical_findings: baseline=0 variant=0
warning_findings: baseline=1 variant=2  +1
stylestat.pattern_top_per_chapter: baseline=3.1 variant=5.4  +2.3   ← 文体回归
stylestat.ending_short_ratio: baseline=0.42 variant=0.71  +0.29     ← 章末同构加重
```

### 9.2 Repeat 汇总

`--repeat N` 时不只看最后一次，当前实现展示通过率、hard fail 次数、warning 次数、cost/tool_calls 的 min/avg/max。Judge 接入后再追加 winner 分布，避免在默认确定性报告里混入模型裁判噪声。

```text
writer_first_chapter_xianxia repeat=3
- pass_rate: 3/3
- cost_usd: avg=0.41 min=0.38 max=0.44
- tool_calls: avg=13 min=12 max=15
- stylestat.pattern_top_per_chapter: avg delta=+0.4（无显著回归）
```

### 9.3 最小可行报告

```text
Gate: FAIL

Hard Fail:
- writer_first_chapter_xianxia: missing checkpoint chapter:1:commit

Warnings:
- writer_dialogue_density: tool_calls +35%
- writer_anti_ai_tone: ending_short_ratio +0.28 (文体回归)

Quality:
- writer_anti_ai_tone: judge prefers variant, confidence=medium

Artifacts:
- workspace/evals/20260629-120000/report.json
```

### 9.4 Baseline 流程与安全上报

Baseline 是一次性的、写进仓库的参照点，后面每个阶段都拿它当分母：

```bash
go run ./cmd/ainovel-cli eval \
  --cases evals/cases/calibration \
  --repeat 3 \
  --config <cfg> \
  --out workspace/evals/baseline-<date>
```

- 原始报告（`report.json` / `report.md` / `artifacts/`）只落在 `workspace/`，该目录已被 `.gitignore` 忽略；**不提交**。
- 只提交 `plans/…/reports/baseline.md`，内容仅限：模型 ID、provider 名（不含 URL）、commit SHA、汇总指标表、每个 case 的通过率。**不得**包含原始错误消息、正文片段、prompt 或配置内容——这些可能夹带 provider 的 response body 与用户文本。
- 提交前跑一次 secret scan：

  ```bash
  rg -n -i 'api[_-]?key|bearer|sk-|https?://' plans/261027-1443-ai-core-quality-uplift/reports
  ```

- 后续新增 case 需要在**同一个 baseline commit** 上补跑，才能与历史比较：`git worktree add workspace/baseline-wt <sha>`，在那个 worktree 里跑新 case。功能在 baseline 上不存在时记"N/A ở baseline"，不算进 delta。

### 9.5 Calibration 集：dev / held-out

`evals/cases/calibration/` 分两套，**冻结**后再开始调 prompt：

- `dev/`：调 prompt 时随时跑，允许反复看。
- `heldout/`：只在 baseline 与最终验收各跑一次。用 held-out 结果反推 prompt 改动就等于把它变成第二份 dev 集。

Case 与 fixture 一一对应（`fixture` 名 == case id）。fixture 是手写越南语正文、每章 600–1200 字，必须带**唯一的词法标记**（专有地名、称呼、标记词），因为 `forbid_regex` / `require_regex` 只适合抓这类确定字符串。

已验证的离线检查：`go test ./internal/eval -run TestFixtureCasesLoadAndSeed` 会加载 `evals/cases/calibration` 与 `evals/cases/editor-goi-a` 下所有 case（必须都是 fixture 模式），并把它们的 fixture 铺成书稿、过 diag 门禁。

---

## 10. 目录结构与命令

```text
internal/eval/
  case.go           Case manifest 结构 + 加载（含 fixture/instruction/control 与 review/prose/facts 断言）
  eval.go           CLI 编排：single / A/B / repeat
  runner.go         装配 host 驱动（按章数上限截停 + drain 到 Done），fixture 模式跑单条 instruction
  fixture.go        fixture 路径校验 + seed（只用 store API）+ seed 后 diag 自检
  collect.go        diag.Diagnose + stylestat.Compute + usage/session + 评审/正文/事实采集
  assert_review.go  verdict / must_find / must_not_find / 对照 case 的假阳性
  assert_prose.go   prose 正则、开篇复读、facts forbid_contains
  grade.go          Finding→门禁映射 + baseline/variant delta + stylestat gate 决策
  report.go         report.json + report.md + 分项质量指标
  (rules/overlap.go 纯函数开篇重叠与去重)

cmd/ainovel-cli  eval 子命令入口

evals/
  cases/         smoke/ workflow/ quality/ longform/ recovery/ steering/
  cases/calibration/{dev,heldout}/   固定书稿的质量 case（fixture 名 == case id）
  fixtures/<name>/fixture.json + chapters/NN.md
  rubrics/       writer_chapter.json / architect_outline.json / editor_review.json
  variants/      writer-anti-ai-tone/writer.md 等（每目录只放要替换的 prompt）
  reports/       历史报告归档
```

命令：

```bash
# 多 case 批量（CI 默认只跑 smoke、不开 judge）
ainovel-cli eval --cases evals/cases/smoke \
  --variant evals/variants/writer-anti-ai-tone \
  --out workspace/evals/writer-anti-ai-tone --ci
```

**本期已实现的参数**：`--cases`（目录或单 manifest）、`--variant`（变体 prompt 目录；传入后自动跑 baseline+variant A/B）、`--repeat N`（每个 case 重复运行 N 次）、`--config`、`--out`、`--max-chapters N`（覆盖 case 默认；fixture 模式无关，该模式没有章数上限）、`--fixtures`（fixture 根目录，默认 `evals/fixtures`）、`--timeout`（单 case 墙钟上限）、`--ci`（抑制逐事件输出；退出码非 0 即 hard fail，不带也生效）。

**规划中（尚未实现，勿在命令行使用，否则报 flag 未定义）**：`--judge`/`--no-judge`（Phase 3 LLM Judge）。重大 prompt 改动当前可先用确定性 A/B + repeat：

```bash
# 重大 prompt 改动：A/B + repeat 降随机性
ainovel-cli eval --cases evals/cases/quality \
  --variant evals/variants/writer-anti-ai-tone \
  --repeat 3 --ci
```

---

## 11. 明确不做的事

违反即代表评测偏离定位。

1. **不在评测层复制 diag 的通用诊断逻辑** —— 通用判断（pending 残留、phase/flow 一致、章节缺口、死循环）一律走 `diag`，事实判断只有一份定义。case 级契约断言（`expect.required_checkpoints` 等）允许直接读 `store`/checkpoint API，但只做**薄断言**——验证本 case 强相关的具体预期，绝不重写一遍 diag 已有的通用规则。
2. **不重新实现确定性规则** —— diag 已有一组工件规则 + 运行时规则。缺规则就去 diag 加，评测层只消费。
3. **不在 Python 里重写 stylestat 的中文文体逻辑** —— 直接调 Go 包。
4. **不让 LLM Judge 决定流程是否通过** —— 门禁只认确定性证据。
5. **不让评测介入控制流** —— 丢弃 diag 的 Action/Planner，不自动修 prompt、不回滚、不续跑、不发布。
6. **不断言精确工具调用序列** —— 只断言契约（commit 发生、checkpoint 存在），保护"LLM 驱动流程"的押注。
7. **不引入数据库 / Web UI / 在线评测平台** —— 当前阶段需要的是可重复、可落地、低成本的本地回归。
8. **不拷源码重编译做 variant** —— 内存覆盖 `bundle.Prompts`。
9. **不 mock 成功、不吞错误** —— 任何环节失败显式记录，case 跑崩即 FAIL。
10. **case 不随 prompt 频繁改动** —— case 是稳定测试集；为了让 variant 通过去改 case 是作弊。

---

## 12. 分阶段落地

### Phase 1 · Runner + 确定性门禁（MVP，先证假设）

- `internal/eval`：Case 结构 + runner（in-process headless + bundle 覆盖）+ collect（调 `diag.Diagnose`）+ grade（Finding→门禁 + `expect` 契约）。
- `evals/cases/smoke/` 放 3-4 个 case。
- 报告先出 `report.json` + 最小 markdown。

**验收**：一条命令跑完 smoke；Writer 跳过 commit、pending 残留、checkpoint 缺失、phase 不符**都能被门禁拦下**（这些 diag 本就能查，验证的是 harness 把它接对了）。

### Phase 2 · A/B + repeat + stylestat 回归（已实现）

- `--variant` 自动跑 baseline 与 variant，输出隔离 artifacts。
- `--repeat N` 汇总 pass rate、hard fail runs、warning runs、cost/tool_calls min/avg/max。
- collect 加 `stylestat.Compute`，grade 加文体 delta。
- 报告展示句式章均 / 章末短句占比 / 跨章重复句 / 标题混用的 baseline-variant 对比。

**验收**：用一个 ≥5 章的 case + 一个"句式 tic 加重"的 variant，能被文体回归标出 warning；章数不足的 case 明确显示 `insufficient_sample` 而非误判通过。

### Phase 3 · LLM Judge

- `evals/rubrics/` + `judge.go`，七维 rubric A/B。
- Judge 失败（非法 JSON）→ 报告记失败，不影响确定性结果。

**验收**：Judge 输出进 json+md，且不污染确定性门禁。

### Phase 4 · Longform & Recovery

- 3-5 章连续 / 弧末评审 / 用户干预 / pending_commit replay / 上下文压缩压力 case。
- 复用 diag context+运行时规则。

**验收**：能发现重复时间线、pending 残留、弧末摘要缺失、工具循环。

---

## 13. Case 维护规范

- **数量克制**：Smoke 3-5、Workflow 各角色 3-5、Quality 2-4、Longform/Recovery 各 2-3。过多没人愿意跑。
- **好 case**：输入短而明确、覆盖真实风险、少章内暴露问题、不依赖模型生成固定句子、不把风格偏好写太细。
- **差 case**：输入过长、同时多目标、要跑几十章才判断、只能靠主观感受。
- **Variant 命名**：`writer-anti-ai-tone` / `architect-rolling-outline` / `editor-strict-review`，每目录只放要替换的 prompt。

---

## 14. 风险与边界

- **模型随机性**：同 prompt 多跑也会变。重要改动 `--repeat 3` 看趋势。
- **成本**：Judge 与 longform 烧钱。本地默认只跑 **smoke**（1 章 × baseline+variant，确定性 diag 门禁、不开 Judge、不跑 stylestat）；**stylestat 在 ≥5 章的 Quality/Longform 才启用**（smoke 章数不足，`Compute` 返回 nil，报告标 `insufficient_sample`）；完整 suite 留给重大改动。
- **Judge 偏差**：Judge 也是模型，偏好工整解释性文本，未必等于好看的小说——所以只做辅助，stylestat 是确定性主线。
- **过度指标化**：字数/工具次数/成本/文体统计都是信号不是目标。stylestat 数字是否成病由人按题材裁定，**阈值不写死**（与 editor.md 一致）。
- **不做线上自动回滚**：离线回归工具，不负责线上自动改 prompt / 发布。

---

## 15. 总结

这套评测体系的价值不是自动判断文学质量，而是把 prompt 改动从"凭感觉"变成"有回归、有证据、有人工读样"。

它与上一版设计的根本区别只有一句：**评测器已经在代码库里了。** `diag` 是确定性事实诊断器，`stylestat` 是全书文体回归器，`ReviewEntry` 七维是原生 rubric。评测系统要做的是一层薄薄的 Go harness——批量驱动、采集、把 Finding 与统计映射成门禁、聚合报告——而不是用另一种语言把这些事实判断重写一遍。

一份事实定义，永不漂移。这正是这个项目从架构到评测一以贯之的纪律：**最小 harness、最强复用、确定性归代码、裁定归 LLM 与人。**

---

## 16. 参考

业界 LLM eval 的通用结构（dataset / experiment / scorer / trace / regression gate）是本设计的思想来源，但**刻意不照搬**——本项目的"scorer"是已有的 `diag`/`stylestat`，"trace"是已有的 checkpoint/session 事实层，"dataset"是贴事实层断言的 case。

- OpenAI Evals · https://developers.openai.com/api/docs/guides/evals （注：其托管 Evals 平台已公布退役时间线，只引其结构化测试/自动评分/人工校准的**思想**，不作为未来依赖）
- Braintrust · https://www.braintrust.dev/foundations/what-is-an-eval
- LangSmith · https://docs.langchain.com/langsmith/evaluation-concepts
