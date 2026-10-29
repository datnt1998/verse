// Package store 提供存储后端契约与 file backend 的具体实现。
//
// 架构（phase 02）：contracts（contracts.go 的 19 个领域接口）+ Backend
// （backend.go）+ file 具体实现（file_backend.go + file*Store）+ 业务外观
// Store（store.go）。外部调用方只依赖接口，不依赖具体子存储。
//
// file backend：每个具体子存储（file*Store）持有独立的 IO 实例和独立的
// sync.RWMutex；主要领域（Progress、Outline、Drafts、Summaries 等）的读写
// 互不阻塞；fileWorldStore 将多个低频小领域合并共享一把锁。
//
// 组合根 Store 持有后端提供的子存储接口，并将跨域操作
// （ExpandArc、AppendVolume、ClearHandledSteer、ReviseOutline、
// InvalidateChapterAggregates）委托给 Backend。file backend 以
// crossMu + 每个文件的写锁串行协调；多个文件不构成事务原子提交，
// 调用依靠安全写入顺序、显式错误与同参数幂等重放恢复。postgres
// backend（内部 persistence/postgres）在后续阶段用真事务实现同一
// 契约，由 NewFromBackend 的能力门在启动期校验完整性。
//
// 子存储划分（接口名 = 领域，具体实现 = file*Store）：
//   - ProgressStore: 进度主状态（meta/progress.json）
//   - OutlineStore: 前提、大纲（扁平/分层）、指南针
//   - DraftStore: 章节构思、草稿、终稿
//   - SummaryStore: 章/弧/卷摘要
//   - RunMetaStore: 运行元数据（模型、干预历史）
//   - SignalStore: 一次性信号文件（PendingCommit 恢复）
//   - CheckpointStore: step 级 checkpoint（meta/checkpoints.jsonl）
//   - RuntimeStore: 运行时事件队列（meta/runtime/*.jsonl）
//   - CharacterStore: 角色档案、状态快照
//   - WorldStore: 时间线、伏笔、关系、状态变化、世界规则、风格规则、审阅
//   - 其余领域接口见 contracts.go（Book/UserRules/Sessions/Usage/
//     Simulation/Decisions/ChapterRecords/Revisions）
package store
