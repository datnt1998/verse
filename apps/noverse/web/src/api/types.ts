// Kiểu dữ liệu bám sát docs/design/v1/11-api-contract.md.
// Struct engine giữ nguyên tag gốc (snake_case); kiểu API mới dùng camelCase.

// DTO của thẩm duyệt, chất lượng, sổ cái, vũ trụ, bài học và vi phạm sinh từ
// struct Go (internal/entry/web/api_dto.go) — một định nghĩa duy nhất cho JSON
// của API. Sinh lại: go generate ./internal/entry/web
import type { CharacterKind, LessonScope, ProjectUniverseEntity, ReviewItem } from './types.gen'
export * from './types.gen'
export type LiveHistoryOperation = {
  id: string; role: string; round: number; startedAt: string; durationMs: number
  status: 'done' | 'running' | 'error'; error: string
  key: string; params: string; result: string
}
export type LiveHistoryRound = {
  round: number; role: string; title?: string; startedAt: string
  operations: LiveHistoryOperation[]
}
export type LiveHistory = { rounds: LiveHistoryRound[]; nextBeforeRound: number | null }


// ─── §3 Thư viện & dự án ────────────────────────────────────────────────

export type AutoExport = {
  onVolume: boolean
  onBook: boolean
  onChapter: boolean
  /** Bản đọc TTS (`narration`) không nằm trong tự xuất mặc định nhưng vẫn chọn được (D3). */
  formats: ExportFormat[]
  pattern: string
  onExists: 'replace' | 'keep'
}
export type LibraryConfig = {
  version: 1
  dirs: { ideas: string; assets: string; stories: string; exports: string }
  slugWords: number
  autoExport: AutoExport
}
export type ProjectSummary = {
  id: string; name: string; description: string
  version?: string
  ruleCount: number; style: string
  legacy: boolean; storyCount: number; runningCount: number; updatedAt: string | null
}
export type ProjectFile = {
  name: string; description: string; notes: string; createdAt: string | null
  version?: string
}
export type Project = ProjectSummary & {
  notes: string; createdAt: string | null; path: string
  /** §H câu 3 — tên vũ trụ tuỳ chọn; "" = dùng tên dự án (dự án ẩn danh thì Não dự án ẩn dòng tên). */
  universeName: string
}
export type StoryStatus = 'idea' | 'writing' | 'paused' | 'done' | 'published'
export type StoryFile = {
  title: string; status: StoryStatus; idea: string; tags: string[]
  version?: string
  publishedAt: string | null; createdAt: string | null
  /** §AF — tên ngắn (≤32) cho tệp audio tải về/metadata; thiếu thì suy ra từ title. */
  shortTitle?: string
  /** Đã đổi "Tên truyện" tay: đồng bộ tự động theo meta/book.json tắt khi true. */
  titleRenamed?: boolean
  /** D1 — khoảng chương do mỗi bản Noverse viết ra (gộp các chương liền nhau cùng bản). */
  builds?: BuildRange[]
}
/** D1 — “Viết bằng phiên bản”: `from`/`to` là số chương (bao gồm hai đầu), `version` rỗng khi bản ghi cũ thiếu. */
export type BuildRange = {
  from: number; to: number
  /** hiển thị được: không có tiền tố `v`, không kèm commit; rỗng = bản ghi cũ */
  version: string
  /** 7 ký tự đầu của commit đã viết ra khoảng chương này; rỗng = không rõ */
  commit: string
  /** RFC3339 của chương đầu khoảng (khoảng gộp giữ ngày chương đầu); rỗng = bản ghi cũ */
  at: string
}
/** D1 — một bản trong “Có gì mới”: `notes` là thân release trên GitHub (markdown nguyên văn, có thể rỗng). */
export type ReleaseNote = { version: string; date: string; notes: string }
export type StorySummary = {
  id: string            // "<p>/<s>"
  project: string; story: string
  folder: string        // tương đối dự án, vd "kich_ban/01-tra-xanh"
  title: string
  status: StoryStatus
  tags: string[]; idea: string
  phase: string; completedChapters: number; totalChapters: number; totalWords: number
  volumes: number
  open: boolean; running: boolean
  locked: boolean
  updatedAt: string | null
  exportsCount: number
  tempName: boolean
  /** §AF — tên ngắn (≤32) cho tệp audio tải về/metadata; thiếu thì suy ra từ title. */
  shortTitle?: string
  /** D13 · GAP-42 — số mục Bắt sửa còn mở (nút “Chất lượng” ở đầu trang truyện); vắng ở máy chủ cũ */
  openErrors?: number
}
export type Idea = {
  id: string; title: string; excerpt: string
  createdAt: string | null; updatedAt: string | null; usedBy: string[]
}
export type Asset = { path: string; kind: 'file' | 'dir'; size: number; updatedAt: string | null }
export type FolderNode = {
  name: string; path: string; kind: 'file' | 'dir'
  role: 'project' | 'config' | 'rules' | 'ideas' | 'assets' | 'stories' | 'story' | 'storyMeta' | 'engine' | 'exports' | 'other'
  status: 'ok' | 'engine' | 'optional' | 'missing' | 'invalid'
  note?: string; children?: FolderNode[]
}
export type ValidationIssue = {
  path: string; level: 'error' | 'warn' | 'info'; code: string; message: string; fixable: boolean
}
export type LibraryResponse = {
  version?: string
  /** GAP-34 — false khi cài mới chưa chọn nơi lưu truyện (không có thư viện nào). */
  configured: boolean
  /** Native folder picker, hosted database, or an image-managed data volume. */
  storageMode?: 'file' | 'postgres' | 'container'
  storageWarning?: string
  /** “Nơi › … › Thư mục” đã dịch sẵn; "" khi chưa chọn. Chỉ chuỗi này được hiện trên UI. */
  rootDisplay: string
  /** Đường dẫn tuyệt đối — chỉ quản trị nhận được, chỉ dùng cho tooltip / sao chép. */
  rootPath?: string
  config: LibraryConfig
  projects: ProjectSummary[]
  candidates: string[]
}
export type ProjectDetail = { version?: string; project: Project; stories: StorySummary[]; ideas: Idea[]; assets: Asset[] }
export type ValidateResult = { ok: boolean; issues: ValidationIssue[]; tree: FolderNode }

// ─── §4 Truyện — đọc trực tiếp ──────────────────────────────────────────

export type ChapterInfo = {
  chapter: number; title: string; completed: boolean; words: number
  hasDraft: boolean; volume: number; arc: number
  /** §AI — số góp ý trên hàng chương (03b): need = lost+unsure, pending = chờ gửi. */
  comments?: { need: number; pending: number } | null
}
export type ChapterText = { chapter: number; title: string; text: string; source: string; words: number }

// Engine structs (nguyên tag snake_case — internal/domain)
export type ChapterContract = {
  required_beats?: string[]; forbidden_moves?: string[]; continuity_checks?: string[]
  evaluation_focus?: string[]; emotion_target?: string; payoff_points?: string[]; hook_goal?: string
}
export type ChapterPlan = {
  chapter: number; title: string; goal: string; conflict: string; hook: string
  emotion_arc?: string; opening_state?: string; notes?: string; contract?: ChapterContract
}
export type TimelineEvent = { chapter: number; time: string; event: string; characters?: string[] }
export type ForeshadowUpdate = { id: string; action: 'plant' | 'advance' | 'resolve' | string; description?: string }
export type RelationshipEntry = { character_a: string; character_b: string; relation: string; chapter: number }
export type StateChange = { character: string; field: string; from?: string; to?: string }
export type CastIntro = { name: string; brief_role: string }
export type OutlineFeedback = { deviation: string; suggestion: string }
export type ChapterFacts = {
  title: string; summary: string; characters: string[]; key_events: string[]
  timeline_events: TimelineEvent[]; foreshadow_updates: ForeshadowUpdate[]
  relationship_changes: RelationshipEntry[]; state_changes: StateChange[]
  cast_intros: CastIntro[]; hook_type?: string; dominant_strand?: string; feedback?: OutlineFeedback
}
export type Character = {
  name: string; aliases?: string[]; role: string; description: string; arc: string
  traits: string[]; tier?: string
  /** Chữ viết tắt người dùng tự đặt cho avatar (04 › Avatar nhân vật — thắng luật tính). */
  ini?: string
  /** Loại vai tô màu avatar (Gói S, `06 › Bổ sung §S`); thiếu thì coi là `support`. */
  kind?: CharacterKind
  /** Chương lộ vai thật; `null`/thiếu = không có bí mật (viền avatar nét liền). */
  revealAt?: number | null
}
export type CastEntry = {
  name: string; aliases?: string[]; brief_role?: string
  first_seen_chapter: number; last_seen_chapter: number
  appearance_count: number; appearance_chapters: number[]; promoted?: boolean
}
export type ForeshadowEntry = {
  id: string; description: string; planted_at: number
  status: 'planted' | 'advanced' | 'resolved' | 'abandoned' | string; resolved_at?: number
  due_by?: number; last_touched?: number; abandoned_at?: number; abandon_reason?: string
}
export type RevisionEntry = { time: string | null; kind: string; old: string; new: string; replaceAll: boolean }

export type OutlineChapter = {
  chapter: number; title: string; coreEvent: string; summary: string
  completed: boolean; words: number
  status: 'done' | 'writing' | 'pending' | 'rewrite' | 'rewriting'
  hasPlan: boolean; hasReview: boolean
}
export type Outline = {
  layered: boolean; path: string
  volumes: {
    index: number; title: string; summary: string
    arcs: { index: number; title: string; summary: string; chapters: number[]; estimatedChapters?: number }[]
    chapters: OutlineChapter[]
  }[]
  nextVolumeTitle: string; compassDirection: string; compassScale: string
}
/** D3 — ba định dạng xuất: Văn bản .txt · Ebook .epub · Bản đọc TTS (narration). */
export type ExportFormat = 'txt' | 'epub' | 'narration'
export type ExportFile = {
  path: string; name: string; format: 'txt' | 'epub' | 'audio' | 'other'
  size: number; updatedAt: string | null; volume: number | null
  chapter?: number; durationSec?: number; sizeBytes?: number; url?: string
}
export type ExportRow = {
  volume: number; title: string; from: number; to: number; complete: boolean
  txt: ExportFile | null; epub: ExportFile | null; audio: ExportFile[]
  /** D3 — bản đọc TTS của tập; null/thiếu = chưa xuất (Bản đọc TTS không nằm trong tự xuất) */
  narration?: ExportFile | null
}
export type ExportResult = { path: string; bytes: number; chapters: number; skipped: number[] }
export type ExportsResponse = { dir: string; files: ExportFile[]; matrix: ExportRow[]; autoExport: AutoExport }

// ─── §5 Truyện — qua worker (live) ──────────────────────────────────────

export type AgentSnapshot = {
  name: string; state: string; taskId: string; taskKind: string; summary: string; tool: string
  turn: number
  context: {
    tokens: number; contextWindow: number; percent: number; scope: string; strategy: string
    activeMessages: number; summaryMessages: number; compactedCount: number; keptCount: number
  }
  updatedAt: string
}
export type AgentCacheStat = {
  role: string; model: string; input: number; output: number
  cacheRead: number; cacheWrite: number; cost: number; saved: number; cacheCapable: boolean
  recentCacheRead: number; recentInput: number; recentSamples: number
}
// Gói P · P1: “Kế hoạch truyện” — snapshot.plan khi truyện đang lập kế hoạch.
export type PlanChapter = { chapter: number; title: string; hook?: string }
export type PlanStepDetail =
  | { kind: 'book'; title: string; synopsis: string }
  | { kind: 'premise'; text: string }
  | { kind: 'characters'; characters: { name: string; role?: string; kind?: CharacterKind; revealAt?: number | null }[] }
  | { kind: 'worldRules'; rules: { rule: string; boundary?: string }[] }
  | { kind: 'outline'; saved: number; total: number; chapters: PlanChapter[] }
export type PlanStep = {
  key: string; status: 'done' | 'running' | 'paused' | 'pending' | string
  summary: string; durationMs: number; startedAt?: string; detail?: PlanStepDetail
}
export type PlanSnapshot = { runningStep?: string; done: number; total: number; steps: PlanStep[] }
export type OutlineItem = { chapter: number; title: string; coreEvent: string }
/** Gói P · P4 — Trọng tài đang chờ người dùng chọn (snapshot.pendingAsk). */
export type AskOption = { label: string; reply?: string; isDefault?: boolean }
export type PendingAsk = { question: string; options?: AskOption[]; chapter?: number }
export type Snapshot = {
  streamText?: string
  readOnlyReason?: string
  provider: string; bookTitle: string; modelName: string; modelContextWindow: number
  thinkingLevel: string; style: string
  runtimeState: 'idle' | 'running' | 'pausing' | 'paused' | 'completed' | string
  /** G5 (14-gap-closure.md): '' · 'gate' · 'cocreate' · 'confirm' · 'input' → chip “Đang chờ bạn” */
  waitingFor: '' | 'gate' | 'cocreate' | 'confirm' | 'input' | string
  statusLabel: string; phase: string; flow: string
  currentChapter: number; totalChapters: number; completedCount: number; totalWordCount: number
  inProgressChapter: number; pendingRewrites: number[] | null; rewriteReason: string; pendingSteer: string
  advanceMode: string; advancePermitChapter: number; hasAdvanceHold: boolean; advanceHoldReason: string
  /** Gói P · P4: chương mục tiêu của một lần tạm dừng (after=chapter) — tô cam nhạt trong Đại cương. */
  advanceHoldChapter?: number
  /** Gói P · P4: Trọng tài đang dừng chờ người dùng trả lời; null/undefined = không có. */
  pendingAsk?: PendingAsk | null
  recoveryLabel: string; isRunning: boolean; agents: AgentSnapshot[] | null
  totalInputTokens: number; totalOutputTokens: number; totalCacheReadTokens: number; totalCacheWriteTokens: number
  totalCostUSD: number; totalSavedUSD: number; budgetLimitUSD: number
  overallCacheCapable: boolean; overallRecentCacheRead: number; overallRecentInput: number; overallRecentSamples: number
  totalCacheBreaks: number; missingAssistantUsage: number
  cachePerAgent: AgentCacheStat[] | null; cachePerModel: AgentCacheStat[] | null
  synopsis: string; premise: string; outline: OutlineItem[] | null; characters: string[] | null
  supportingCount: number; recentSupporting: string[] | null; layered: boolean
  currentVolumeArc: string; nextVolumeTitle: string; compassDirection: string; compassScale: string
  openThreads?: string[]
  planningTier?: 'short' | 'mid' | 'long' | ''
  lastCommitSummary: string; lastReviewSummary: string; lastCheckpointName: string; recentSummaries: string[] | null
  /** Gói P · P1: khung “Kế hoạch truyện”; chỉ có khi truyện đang lập kế hoạch. */
  plan?: PlanSnapshot | null
}
export type HostEvent = {
  id: string; time: string; finishedAt: string; failed: boolean
  category: string; agent: string; summary: string; kind: string
  level: 'info' | 'warn' | 'error' | 'success' | string
  depth: number; durationNs: number; retryAt: string
  /** Authoritative event target/range from host details or the persisted outline. */
  chapter?: number; scope?: string; volume?: number; arc?: number; chapterFrom?: number; chapterTo?: number
  /** G3: tăng mỗi lần engine mở phiên (start/resume/continue/reopen/import); '' ở dữ liệu cũ */
  sessionId: string
  /** G4: true = lượt chờ backoff đánh thức được bằng POST …/retry (agentcore mid-stream → false) */
  canRetryNow: boolean
  /** GAP-31: chỉ có ở đầu vào/lệnh của người dùng. */
  username?: string
  /** User action audit data, separate from its Vietnamese presentation. */
  command?: string
  input?: string
}
export type CoCreateState = {
  active: boolean
  messages: { role: 'user' | 'assistant'; content: string }[]
  draft: string; ready: boolean; suggestions: string[]
  schemaVersion?: 1; proposal?: CoCreateProposal | null; committed?: boolean; nextChapter?: number | null
}
export type CoCreateProposal = {
  schemaVersion: 1; title: string; revision: number
  chapters: { schemaVersion: 1; chapter: number; title: string; summary: string }[]
  impacts: string[]
}
export type CoCreateReply = { reply: string; draft: string; ready: boolean; suggestions: string[]; schemaVersion?: 1; proposal?: CoCreateProposal | null; committed?: boolean; nextChapter?: number | null }
export type CoCreateCommit = { ok: boolean; action: 'start' | 'resume'; schemaVersion?: 1; proposal?: CoCreateProposal | null; committed?: boolean; nextChapter?: number | null }
export type ReopenChecks = {
  schemaVersion: 1; title: string; chapters: number; words: number; lockedAt: string | null; closedAt: string | null
  checks: { id: string; ok: boolean; label: string; note: string; chapters: number[] }[]
  allowed: boolean
}
export type JobEvent = {
  job: string; kind: 'import' | 'simulate' | 'importsim' | 'sync' | 'export' | 'diag' | string
  stage: string; current: number; total: number
  message: string; level: 'info' | 'warn' | 'error' | 'success'
  retryAt: string | null; done: boolean; error: string
  awaiting: '' | 'confirm' | 'storyStatus'
  /** kind 'export' — có khi xong (GAP-27) */
  export?: ExportJobResult
  /** kind 'sync' — có khi xong (GAP-29) */
  sync?: SyncResult
  /** kind 'simulate' — sources/words có từ bước quét; synthesis khi xong */
  simulate?: SimulateJobResult
  import?: ImportProgress
}

// ─── GAP-28 — /diag: Host.Diagnose() → diag.Report (14-jobs-dock §2) ────
export type DiagSeverity = 'critical' | 'warning' | 'info'
export type DiagCategory = 'flow' | 'quality' | 'planning' | 'context'
export type DiagStats = {
  completedChapters: number; totalChapters: number; totalWords: number; avgWordsPerChapter: number
  phase: string; flow: string; planningTier: string
  reviewCount: number; rewriteCount: number
  /** 0 = chưa có điểm */
  avgReviewScore: number
  foreshadowOpen: number; foreshadowStale: number
  /** ngày bắt đầu truyện; null = chưa rõ */
  startedAt: string | null
  currentVolume: number; totalVolumes: number
}
export type DiagFinding = {
  fingerprint: string; rule: string; category: DiagCategory; severity: DiagSeverity
  confidence: 'high' | 'medium' | 'low'; autoLevel: 'none' | 'suggest' | 'safe'
  target: string; title: string; evidence: string; suggestion: string
  /** Chỉ hiển thị trong phần Nâng cao do người dùng chủ động mở. */
  technical?: string
}
export type DiagAction = {
  fingerprint: string; sourceRule: string; kind: string; severity: DiagSeverity
  summary: string; message: string
  /** đã gửi cho trợ lý trong phiên worker này */
  sent: boolean
}
export type DiagReport = { generatedAt: string; stats: DiagStats; findings: DiagFinding[]; actions: DiagAction[] }

// ─── GAP-29 — /sync: số chữ thêm/bớt cho --check + revision.Result ──────
export type SyncChanged = {
  chapter: number; title: string; kind: 'edit' | 'resume' | string
  /** số chữ thêm/bớt (cùng đơn vị ChapterInfo.words) */
  added: number; removed: number
  modifiedAt: string | null
}
export type SyncIssue = { text: string; chapter: number | null; issueIndex?: number; sent?: boolean }
export type SyncAnalysis = {
  chapter: number; title: string; changeSummary: string; storyChanged: boolean
  schemaVersion?: 1; classification?: 'facts' | 'wording' | 'conflicts'; conflictCount?: number
  downstreamIssues: SyncIssue[]
}
export type SyncResult = { schemaVersion?: 1; applied: number[]; analyses: SyncAnalysis[] }
/** Toàn văn chương đã tiếp nhận (before) và bản sửa ngoài app (after), đã chuẩn hoá. */
export type SyncDiff = { chapter: number; title: string; before: string; after: string }

// ─── GAP-27 — /export là việc nền + xử lý tệp trùng ─────────────────────

/** Gói AE — lỗi việc ghép audio: lời người dùng + chương dừng + mã nguyên nhân. */
export type JobErrorInfo = {
  message: string
  chapter?: number
  retryable?: boolean
  /** "intro_missing" | "outro_missing" | "music_decode" | … — quyết định nút Ghép không nhạc */
  cause?: string
}
/** Kết quả việc ghép & xuất audio (`job.result.data` khi kind 'export' của audio). */
export type AudioExportJobResult = {
  files?: { name: string; sizeBytes?: number; url?: string; path?: string; durationSec?: number; chapter?: number }[]
  zipUrl?: string
}
export type ExportOnExists = 'replace' | 'keep' | 'fail'
export type ExportJobResult = {
  /** tương đối thư mục truyện — không hiện trên UI (bất biến #7) */
  path: string; name: string; title: string; format: ExportFormat
  bytes: number; chapters: number; skipped: number[]
}
export type ExportConflict = {
  path: string; format: ExportFormat
  existing: { path: string; size: number; modTime: string | null; bytes: number; chapters: number | null; modifiedAt: string | null }
  /** bytesEstimate = kích thước thật của bản dựng thử; null khi dựng thử lỗi */
  incoming: { path: string; size: number; chapters: number; skipped: number[]; bytesEstimate: number | null }
}

// ─── /simulate — engine SimulationSynthesis giữ json tag snake_case ─────
export type SimulationSynthesis = {
  style?: { narrative_voice?: string[]; sentence_rhythm?: string[]; prose_texture?: string[]; perspective?: string[]; mood?: string[] }
  lexicon?: { signature_phrases?: string[] }
  pacing_density?: { dialogue_action_ratio?: string[] }
}
export type SimulateJobResult = {
  sourceDir: string; sources: number; words: number; profilePath: string
  synthesis?: SimulationSynthesis
}

export type ModelRoleKey = 'default' | 'architect' | 'writer' | 'editor'
export type ModelRole = {
  role: ModelRoleKey
  provider: string; model: string; thinking: string; thinkingOptions: string[]
}
/** Một model trong bảng /model: `thinking` là các mức model hỗ trợ ('' = Mặc định). */
export type ContextWindowSource = 'model_config' | 'config' | 'registry' | 'default'
export type ModelCatalogModel = { name: string; contextWindow: number; contextSource: ContextWindowSource; registryWindow: number; thinking: string[] }
export type ModelCatalogProvider = { provider: string; label: string; ready: boolean; models: ModelCatalogModel[] }
export type ModelsResponse = { roles: ModelRole[]; providers: ModelCatalogProvider[] }
export type ImportRequest = {
  path: string; storyStatus?: 'open' | 'closed'; continueAfter?: boolean
  guide?: string; autoConfirm?: boolean
}

// ─── §6 Cấu hình & quy tắc ──────────────────────────────────────────────

export type ConfigScope = 'global' | 'project' | 'story'
// Config = bootstrap.Config theo json tag gốc; api_key không bao giờ trả về.
export type ModelConfig = { name: string; context_window?: number; json_schema?: boolean }
export type ProviderConfig = {
  type?: string; api?: string; base_url?: string; models?: ModelConfig[]
  extra_body?: Record<string, unknown>; extra?: Record<string, unknown>
  stream_idle_timeout?: string
  // server-side only: api_key_set/api_key_hint thay cho api_key khi GET
  api_key?: string; api_key_set?: boolean; api_key_hint?: string; clear_api_key?: boolean
}
export type ModelRef = { provider: string; model: string }
export type RoleConfig = { provider: string; model: string; fallbacks?: ModelRef[]; reasoning_effort?: string }
export type BudgetConfig = { book_usd?: number; warn_ratio?: number; hard_stop?: boolean }
export type NotifyConfig = { enabled?: boolean; command?: string; events?: string[] }
export type Config = {
  provider?: string; model?: string; reasoning_effort?: string
  providers?: Record<string, ProviderConfig>; roles?: Record<string, RoleConfig>
  style?: string; context_window?: number
  /** Ngôn ngữ sáng tác; hiện chỉ "vi" (bước Ngôn ngữ của Onboard khoá Tiếng Trung). */
  language?: string
  budget?: BudgetConfig; notify?: NotifyConfig; advance_mode?: 'auto' | 'review'
  /** Gói P · P4 — cho Trọng tài tạm dừng hỏi người dùng (mặc định tắt). */
  ask_user?: boolean
  /** GAP-21 — bộ đọc trên máy (15-audio-impl.md §1) */
  tts?: TtsConfig
}
/** GAP-21 — `tts` trong config.json, kế thừa 3 cấp như mọi khoá khác. */
export type TtsConfig = {
  engine?: string
  model?: string
  device?: 'cpu' | 'gpu'
  quality?: 'fast' | 'standard' | 'high'
  sampleRate?: number
  endpoint?: string
  python?: string
}

export type ConfigDoc = { scope: ConfigScope; path: string; exists: boolean; config: Config }
export type ConfigResponse = {
  version?: string
  target: ConfigDoc; effective: Config
  chain: ConfigDoc[]; sources: Record<string, ConfigScope>
  /** G7 — tài khoản OAuth đã kết nối theo provider (đọc từ auth.json); chỉ admin nhận dữ liệu thật. */
  accounts?: Record<string, ConfigAccount>
  /** Cảnh báo cấu hình theo nhà cung cấp; không chặn các nhà cung cấp khác. */
  providerErrors?: Record<string, string>
}
export type ProviderPreset = {
  id: string; label: string; type: string; baseURL: string
  keyRequired: boolean; oauth: boolean; api: 'chat' | 'responses' | ''
  /** OAuth chỉ có device-code (xAI): không có luồng mở trang đăng nhập localhost. */
  deviceOnly?: boolean
  /** OAuth nhận mã dán tay `code#state` / URL chuyển hướng (Claude Pro/Max): dùng được khi mở web từ thiết bị khác. */
  pasteCode?: boolean
  /** G6 — model mặc định của preset; server dùng khi /api/config/test không có `model`. */
  defaultModel: string
}
/** D2 — gói thể loại (GAP: `/api/config/presets` › `packs`). Gói có bộ luật, Biên tập viên chấm từng chương. */
export type PackLayer = {
  key: 'master' | 'pack' | 'measure' | string
  title: string
  /** Mô tả cố định theo loại tầng (Câu 7): Master đếm mục từ dữ liệu, Bộ/Định dạng là câu cố định. */
  desc?: string
  /** số mục [LỖI] / [CẢNH BÁO] của tầng luật (đếm lúc máy chủ trả, bỏ dòng chú giải) */
  errors: number; warnings: number
}
export type PackSection = {
  ref: string; title: string
  /** error khi errors>0, warning khi warnings>0, còn lại rỗng */
  level: 'error' | 'warning' | '' | string
  /** số mục [LỖI] / [CẢNH BÁO] của riêng mục này; tổng đúng bằng số của tầng */
  errors: number; warnings: number
}
export type PackMetric = { key: string; name: string; limit: string }
/** Một chip của hàng thông số trong chi tiết gói: `key` để chọn icon, `text` do máy chủ soạn từ `do_do`. */
export type PackParam = { key: 'chapters' | 'dialogue' | 'sentence' | 'quiet' | 'procedural' | 'opening' | string; text: string }
/** Một dòng “Thay mặc định văn phong”: chỉ giá trị mới của khoá `voice.*` (Câu 6). */
export type PackOverrideRow = { key: string; value: string }
export type PackPreset = {
  id: string; ten: string; so_chuong: number; nguoi_nghe: string
  /** `goi.json` › `ten_ngan` — tên ngắn dùng ở mọi chỗ hiển thị (Câu 7, 12). */
  short?: string
  /** `goi.json` › `mo_ta` — dòng mô tả dưới tên gói trên thẻ (rỗng/trùng `ten` thì không hiện). */
  desc?: string
  /** “8 chương · thoại 40–55%” do máy chủ soạn từ `so_chuong` + `thoai` (Câu 7). */
  meta?: string
  /** Câu 1 — độ dài chương (`goi.json` › `tieng_moi_chuong`); null khi gói không khai (không bịa số). */
  chapterWords?: { min: number; max: number } | null
  /** Gói có bộ luật để chấm từng chương → badge “Có chấm”. */
  hasRules?: boolean
  /** Gói dùng được ngay; false = “Sắp có” (không chọn được). */
  available?: boolean
  layers?: PackLayer[]
  sections?: PackSection[]
  /** Chín số đo của gói (`internal/rules/metrics.go`) — dùng ở màn khác, không phải hàng chip của chi tiết gói. */
  metrics?: PackMetric[]
  /** Hàng thông số “Số đo dùng để chấm” của chi tiết gói, chỉ lấy từ `goi.json` › `do_do` (Câu 7). */
  params?: PackParam[]
  /** Khối “Thay mặc định văn phong”: chỉ có mục khi gói ghi đè ít nhất một khoá `voice.*` (Câu 6). */
  overrides?: { count: number; keys: string[]; rows: PackOverrideRow[] }
  /** Câu “Giọng đọc theo gói: …” do máy chủ soạn (packdetail.go › packVoiceLine). */
  voice?: string
  /** D11 — chế độ đọc của gói (`goi.json` › `audio`): single = khoá phân vai, một giọng kể.
   *  Máy chủ đọc thẳng `goi.json`; gói không khai thì cả hai trường là "". */
  audio?: { che_do?: 'single' | 'cast' | ''; giong?: 'nu' | 'nam' | '' }
}
export type ConfigPresets = {
  providers: ProviderPreset[]; styles: string[]; reasoning: string[]
  /** Gói thể loại đang có trên máy chủ (rỗng/thiếu = máy chủ chưa trả). */
  packs?: PackPreset[]
}
export type ConfigModelInfo = { name: string; displayName: string; description?: string; contextWindow: number; contextSource: ContextWindowSource; registryWindow: number; summaryThreshold: number; automaticWindow: number; automaticSource: ContextWindowSource }

// ─── POST /api/config/models/list (06-data-contract) ──────

/** Nguồn cửa sổ ngữ cảnh của danh sách model: như ContextWindowSource + 'provider' (API nhà cung cấp báo). */
export type ModelListContextSource = 'model_config' | 'provider' | 'registry' | 'default'
export type ProviderModel = {
  /** id lưu vào providers.<id>.models[].name */
  name: string
  displayName: string
  description?: string
  /** đã giải, không bao giờ 0 (dự phòng 200000) */
  contextWindow: number
  contextSource: ModelListContextSource
  /** cửa sổ trong danh mục Noverse; 0 = danh mục không biết model */
  registryWindow: number
  /** USD mỗi 1 triệu token, chỉ khi nhà cung cấp trả giá (OpenRouter) */
  price?: { input: number; output: number }
  /** danh mục Noverse có model này hoặc là defaultModel của preset */
  recommended: boolean
}
export type ProviderModelList = {
  /** theo thứ tự nhà cung cấp; [] khi lỗi/không hỗ trợ */
  models: ProviderModel[]
  total: number
  /** RFC3339 lúc lấy danh sách (cache giữ mốc gốc); "" khi lỗi */
  fetchedAt: string
  /** nhà cung cấp không có API danh sách (bedrock) → models: [] */
  unsupported?: true
  error?: 'key_rejected' | 'unreachable' | 'missing_key' | 'not_logged_in'
  /** tiếng Việt, luôn có khi error có */
  message?: string
  /** lỗi gốc từ nhà cung cấp, không bắt buộc */
  detail?: string
}
export type ConfigAccount = { connected: boolean; label: string; email: string; plan: string; expiresAt: string | null }
export type OAuthStart = {
  id: string; mode: 'browser' | 'device' | 'paste'
  url: string; userCode: string; verifyURL: string; expiresAt: string
}
export type OAuthStatus = {
  state: 'pending' | 'done' | 'failed'; mode: 'browser' | 'device' | 'paste'
  account: { label: string; email: string; plan: string } | null
  error: string; models: ConfigModelInfo[]
}
export type RulesScope = { scope: ConfigScope; dir: string; exists: boolean; files: { name: string; size: number; updatedAt: string | null }[] }
export type RulesResponse = { chain: RulesScope[] }
export type RuleFile = { name: string; text: string; version?: string }
/** GAP-35 — văn mẫu cho /simulate: `id` là tên mẫu (không bao giờ là đường dẫn máy chủ). */
export type StyleSample = { id: string; title: string; chars: number; addedAt: string }
export type StyleSamples = { samples: StyleSample[]; total: { count: number; chars: number } }
export type SystemInfo = {
  version: string; root: string; rootDisplay: string; retries: number; deadlock: number[]
  compactThreshold: number; promptCache: boolean; outputDir: string; defaultContextWindow: number
  /** G1: máy chủ chạy ngay trên máy người dùng (loopback) → UI hiện “Mở thư mục”. */
  canReveal: boolean
  /** Đang mở cho thiết bị cùng Wi-Fi (GET /api/system.lan). */
  lan?: boolean
  /** D1 — mã build (commit) và thời điểm build do máy chủ trả; rỗng khi bản dev không có. */
  commit?: string; built?: string
  /** D1 — bản mới nhất đã biết (rỗng khi chưa kiểm tra xong) + có bản mới hay không. */
  latestVersion?: string; hasUpdate?: boolean
  /** D1 — nhật ký thay đổi: bản mới trước (khi có bản mới), rồi bản đang chạy; ≥1 mục khi máy chủ trả. */
  releases?: ReleaseNote[]
}

// ─── §7 Tài khoản (webauth) ─────────────────────────────────────────────

export type Role = 'admin' | 'member'
/** Kho tài khoản chung: tên app là giá trị lưu trữ, không phải nhãn hiển thị. */
export type AppId = 'noverse' | 'viderse'
/**
 * Quyền vào từng app của tài khoản (map app → vai trò hiệu lực).
 * Kho chung Postgres luôn trả object — `{}` = thành viên chưa được cấp app nào;
 * backend file độc lập trả `null` (không có khái niệm grant liên app).
 */
export type AppGrants = Partial<Record<AppId, Role>>
export type AppLink = { id: AppId; name: 'Noverse' | 'Viderse'; origin: string }
export type ProjectRole = 'owner' | 'collab' | 'view'
export type ProjectPermissions = Record<string, ProjectRole>
export type AdminAuditItem = {
  id: string; at: string; actor: { username: string; name: string } | null
  type: 'acct' | 'perm' | 'sec'; action: string; target: string
  project?: string; from?: string; to?: string; reason?: string
}
export type User = {
  version?: string
  username: string; name: string; role: Role; projects: ProjectPermissions
  /** Quyền vào từng app trên kho tài khoản chung; `null` = backend file độc lập. */
  grants?: AppGrants | null
  disabled: boolean; createdAt: string; updatedAt: string
  onboardedAt: string | null; mustChangePassword: boolean
  lastSeenAt: string | null; sessionCount?: number
  disabledAt?: string; disabledBy?: string; disabledReason?: string; passwordChangedAt?: string
}
export type Session = {
  id: string; username: string; device: string; ip: string
  createdAt: string; lastSeen: string; expiresAt: string; current: boolean
}
export type Pairing = {
  code: string; username: string; expiresAt: string
  links: { label: string; url: string; svg: string }[]
}
export type RememberedAccount = {
  username: string; name: string; role: Role; lastLoginAt: string; device?: string
}

// ─── GAP-41 · Lần chạy đầu (A0 Setup) — không cần phiên ──────────────────
/** `needed=false` khi đã có ít nhất một tài khoản; mã thiết lập không bao giờ nằm trong phản hồi. */
export type SetupState = {
  needed: boolean
  sharedAccounts?: boolean
  expiresAt?: string
  /** Mã đã hết hạn (GAP-41 bổ sung): hết hạn thắng khoá, UI mở trạng thái codeExpired. */
  expired?: boolean
  attemptsLeft: number
  lockedUntil?: string
  /** Phiên bản Noverse — chỉ có khi `needed=true` (chân trang A0). */
  version?: string
}
export type SetupVerifyResult = { setupToken: string; expiresAt: string }
export type SetupAdminResult = { user: User; recoveryCode: string }

// ─── §8 Bổ sung 14-gap-closure.md ───────────────────────────────────────

/** G1 — thư mục mở được bằng trình quản lý tệp trên máy chủ. */
export type RevealTarget = 'library' | 'project' | 'story' | 'exports' | 'activity' | 'audio' | 'rules' | 'config'
export type RevealRequest = {
  target: RevealTarget
  project?: string
  story?: string
  /** chỉ dùng cho target `rules` và `config` */
  scope?: ConfigScope
}
export type RevealResult = { ok: true; path: string }

/** G9 — xem trước cách chia chương; bản có guide được Kiến trúc sư lưu để xác nhận. */
export type ImportPreviewChapter = { n: number; title: string; words: number; firstLine: string; newlySplit?: boolean; warning?: string }
export type ImportPreview = { schemaVersion?: 1; filename?: string; size?: number; words?: number; splitHow?: string; chapters: ImportPreviewChapter[]; warnings: string[]; encoding: string }
export type ImportOptions = { schemaVersion: 1; storyStatus: 'open' | 'closed' | ''; continueAfter: boolean; guide: string }
export type ImportProgress = {
  schemaVersion: 1
  steps: { schemaVersion: 1; id: string; title: string; current: number; total: number; status: 'pending' | 'running' | 'done' }[]
  etaSec?: number | null
  summary?: { schemaVersion: 1; chapters: number; characters: number; foreshadows: number } | null
  preview?: ImportPreview | null; continued?: boolean
}
export type ImportState = { schemaVersion?: 1; resumeHint: string; job: JobEvent | null; path?: string; preview?: ImportPreview | null; options?: ImportOptions }

/** G10 — revision đổi khi bất kỳ chương/bản nháp/thẩm duyệt đổi (mtime + kích thước). */
export type ChaptersResponse = { chapters: ChapterInfo[]; revision: string }

/** G1 — GET /api/reveal: chỉ trả đường dẫn, không mở thư mục. */
export type RevealPath = { path: string; canReveal: boolean }

/** G8 — đồng sáng tác ở cấp dự án (chưa có truyện); `draftId` = id phiên nháp, `draft` = bản nháp AI tổng hợp. */
export type ProjectCoCreateState = CoCreateState & { draftId: string }
export type ProjectCoCreateReply = CoCreateReply & { draftId: string }

/** G4 — đánh thức vòng chờ backoff ngay (409 khi không có lượt nào đang chờ). */
export type RetryResult = { ok: boolean }

/** G13 — lệnh gõ trong ô chat web; `message` là kết quả server muốn hiện trong nhật ký,
 *  `result` cho UI biết mở bảng nào / theo dõi việc nền nào (14-jobs-dock §2). */
export type CommandResult = { ok: boolean; message?: string; result?: CommandOutcome }
export type CommandOutcome =
  | { kind: 'panel'; panel: 'model' | 'diag'; role?: string }
  /** `path` (chỉ export) = đường dẫn tương đối đã phân giải, kể cả tên mặc định */
  | { kind: 'job'; job: string; jobKind: 'export' | 'sync' | 'simulate' | 'import' | 'importsim'; path?: string }
  /** chỉ đi kèm 409 — chưa ghi gì */
  | { kind: 'exportExists'; exists: ExportConflict }
  | { kind: 'syncCheck'; changed: SyncChanged[] }

// ─── §8 Lệnh — registry dùng chung (13-commands.md › GAP-23, G11) ───────
/** Cách nhận giá trị của một tham số: đường dẫn · nhóm chọn · công tắc · số · chữ tự do. */
export type CommandParamKind = 'path' | 'enum' | 'flag' | 'num' | 'text'
export type CommandParam = {
  key: string
  /** phần hiện trong cú pháp, vd '<đường-dẫn>', 'on|off', 'from=N', '--overwrite' */
  token: string
  kind: CommandParamKind
  required: boolean
  /** tiền tố dính liền giá trị, đã gồm dấu '=', vd 'from=', '--story=' */
  prefix?: string
  /** công tắc đứng riêng, vd '--yes' */
  flag?: string
  options?: { value: string; note: string }[]
}
export type CommandSpec = {
  /** tên không có dấu '/', vd 'export' */
  name: string
  /** tên khác, vd ['plan'] cho cocreate; rỗng là [] */
  aliases: string[]
  /** cú pháp đầy đủ, giữ dấu '/' ở đầu */
  usage: string
  /** begin · write · style · end · sys */
  group: string
  autoExecute: boolean
  needsIdle: boolean
  only?: 'welcome' | 'running'
  params: CommandParam[]
}


/** G2 — PUT /api/library {root}: đổi thư mục thư viện lúc chạy; 409 khi đang có truyện chạy. */
export type LibraryRootResult = { ok: boolean; root: string; rootDisplay: string; config: LibraryConfig }

/** G2 — POST /api/system/pick-folder: hộp chọn thư mục của hệ điều hành; 501 khi không hỗ trợ. */
export type PickFolderResult = { path?: string; cancelled?: boolean }

/** G6 — POST /api/config/test: `model` là model máy chủ thực sự dùng (preset defaultModel khi thiếu).
 *  `message` luôn tiếng Việt; `detail` là lỗi gốc (mạng/nhà cung cấp) khi có, chỉ để chẩn đoán.
 *  Khi ok: contextWindow/contextSource giải cho cặp provider+model vừa thử (model_config > provider > registry > default). */
export type ConfigTestResult = { ok: boolean; latencyMs: number; message: string; model: string; detail?: string; contextWindow?: number; contextSource?: ModelListContextSource }

// ─── §9 Audio — bản đọc → tạo giọng → nghe → xuất tập (10-audio.md, GAP-17…21) ───

/** Trạng thái bản đọc một chương (GAP-17). `draft` = AI đang soạn. */
export type AudioScriptStatus = 'todo' | 'draft' | 'review' | 'locked' | 'stale'

/** Đoạn đã có giọng: `ready` dùng luôn khi xuất · `stale` chữ đã đổi, cần đọc lại. */
export type AudioSegmentAudio = { url: string; state: 'none' | 'ready' | 'stale'; durationSec: number }

/** Chỗ chữ đã được chuẩn hoá: `90 giây` → `chín mươi giây` (gạch chân chấm xanh, hover xem gốc). */
export type AudioNormalized = { from: string; to: string; kind: string }

/** Gợi ý người nói khi AI chưa chắc — chỉ có ở đoạn `unknown_speaker`. */
export type AudioSpeakerGuess = { who: string; p: number }

export type AudioSegment = {
  id: string
  speaker: string
  text: string
  /** chữ gốc trước khi chuẩn hoá (hiện khi bật “So với chương gốc”) */
  source?: string
  pauseMs: number
  tone: string
  normalized?: AudioNormalized[]
  speakerGuess?: AudioSpeakerGuess[]
  /** long_sentence · unknown_speaker · english · no_voice */
  flags?: string[]
  /** chữ đổi sau khi chốt → chấm cam “cần đọc lại” */
  changed?: boolean
  /** AI còn đang soạn đoạn này (chương `draft`) — hiện mờ, chưa nghe được */
  pending?: boolean
  audio?: AudioSegmentAudio | null
}

export type AudioScript = {
  version?: string
  chapter: number
  volume: number
  status: 'review' | 'locked'
  lockedAt: string | null
  sourceHash: string
  segments: AudioSegment[]
  chapterChanged?: boolean
  scriptChanged?: boolean
  staleReason?: string
}

export type AudioChapterAudio = {
  segmentsDone: number
  segments: number
  durationSec: number | null
  rendering: boolean
  percent: number
  /** Dấu vân tay audio cả chương khi đã đủ giọng — gắn vào URL phát để tải lại sau khi dựng lại. */
  version?: string
}

export type AudioChapter = {
  chapter: number
  title: string
  status: AudioScriptStatus
  issues: number
  durationSec: number | null
  lockedAt: string | null
  segments: number
  speakers: number
  audio: AudioChapterAudio | null
  changedSegments: number
  chapterChanged?: boolean
  scriptChanged?: boolean
  staleReason?: string
  /** §AK — ước tính giây chỉ đọc lại các đoạn đổi (có khi bộ đọc đã đo được tốc độ; thiếu = chưa đo). */
  updateEtaSec?: number
}

export type AudioVolume = { chapters: AudioChapter[] }

/** Giọng dự án (tai-san/giong) + giọng có sẵn của bộ đọc. */
export type AudioVoice = {
  id: string
  name: string
  file?: string
  durationSec?: number
  gender: string
  builtIn: boolean
  sampleUrl?: string
}

export type AudioSingleSettings = {
  voiceId: string
  dialogueShift: boolean
  keepTones: boolean
  dialoguePauseMs: number
}

export type AudioCastSpeaker = {
  id: string
  name: string
  role: string
  voiceId: string | null
  /** Chữ viết tắt avatar người dùng tự đặt (04 › Avatar nhân vật). */
  ini?: string
  /** Loại vai của nhân vật (Gói S) — màu avatar người nói; dẫn truyện và quần chúng bỏ trống. */
  kind?: CharacterKind
  /** dùng chung giọng với vai khác (Dẫn truyện ↔ nhân vật ngôi thứ nhất) */
  sameAs?: string
  oldSegments?: number
  oldEtaSec?: number
}

export type AudioCast = {
  version?: string
  mode: 'cast' | 'single'
  voiceMode?: 'cast' | 'single' | 'pack'
  narratorCharacterId?: string
  single: AudioSingleSettings
  speakers: AudioCastSpeaker[]
}

/** Cách đọc: từ điển + công tắc tự chuẩn hoá (`cach-doc.json`). */
export type AudioPronunciation = { written: string; spoken: string }

export type AudioNormalize = {
  numbers: boolean
  dates: boolean
  money: boolean
  symbols: boolean
  abbreviations: boolean
  chapterTitles: boolean
  dropBrackets: boolean
}

export type AudioSettings = {
  version?: string
  quality: 'fast' | 'standard' | 'high'
  autoOnLock: boolean
  normalize: AudioNormalize
}

export type AudioJobState = 'queued' | 'rendering' | 'paused' | 'error' | 'done' | 'canceled'

export type AudioQueueJob = {
  jobId: string
  chapter: number
  /** §AK — create: tạo mới · update: chỉ đọc lại đoạn đổi · redo: đọc lại từ đầu. */
  kind?: 'create' | 'update' | 'redo'
  /** §AK — thứ tự 1…n trong các job `queued` (1 = Tiếp theo); trạng thái khác không có. */
  position?: number
  state: AudioJobState
  quality?: AudioSettings['quality']
  step?: 'compose' | 'render'
  needsScript?: boolean
  segmentsDone: number
  segments: number
  etaSec: number
  speaker: string
  /** câu đang đọc (Literata 19px trong phòng thu) */
  currentText?: string
  /** tiến độ trong đoạn hiện tại 0…1 — chia 4 bước của đoạn */
  segmentPercent?: number
  error?: string
  errorAt?: string
  percent: number
}

export type AudioTimelineChapter = {
  chapter: number
  title: string
  state: 'done' | 'rendering' | 'queued' | 'unlocked' | 'error'
  sec: number | null
  estSec: number
  pct: number
  segment?: number
  segments?: number
  etaSec?: number
  quality?: AudioSettings['quality']
  reason?: string
  at?: string
  chapterChanged?: boolean
  scriptChanged?: boolean
  staleReason?: string
  /** §AK — copy từ job: `queued` + cờ đổi = “chờ cập nhật”. */
  kind?: 'create' | 'update' | 'redo'
  position?: number
}

export type AudioQueue = {
  jobs: AudioQueueJob[]
  chapters?: AudioTimelineChapter[]
  madeSec?: number
  estTotalSec?: number
  qualityEtaSec?: Partial<Record<AudioSettings['quality'], number>>
}

/** Đỉnh sóng tính sẵn phía server (≈ 2 giá trị/giây). */
export type AudioPeaks = { peaks: number[]; durationSec: number }

export type AudioChapterMark = { atSec: number; label: string }

export type AudioExportOptions = {
  format: 'm4b' | 'mp3' | 'wav'
  chapterMarks: boolean
  subtitles: boolean
  whenReady: boolean
  /** Chỉ ghép các chương đã có giọng được chọn; bỏ trống để ghép cả tập. */
  chapters?: number[]
  split?: boolean
  pack?: 'files' | 'zip'
  trial?: boolean
  onExists?: ExportOnExists
  intro: boolean
  chapterTitles: boolean
  loudness: boolean
  gapSec: number
}
export type AudioExportPrefs = Pick<AudioExportOptions, 'format' | 'intro' | 'chapterTitles' | 'loudness' | 'gapSec' | 'chapterMarks' | 'subtitles'> & { pack: 'files' | 'zip' }
export type AudioExportResult = {
  files: ExportFile[]
  chapterMarks: AudioChapterMark[]
  durationSec: number
  pending?: boolean
  chapterFiles?: ExportFile[]
  split?: boolean
  zipUrl?: string
}

/** Thành phẩm đã ghép của một tập (màn “Đã xuất”). */
export type AudioExportState = {
  volume: number
  files: ExportFile[]
  chapterMarks: AudioChapterMark[]
  exportedAt: string | null
  durationSec: number
  voices: number
  chapterCount: number
  dir: string
  totalBytes: number
  peaks?: number[]
  chapterFiles?: ExportFile[]
  split?: boolean
  zipUrl?: string
}

/** Cờ “cần nghe lại” của trình nghe (A5). */
export type AudioFlag = {
  id: string
  chapter: number
  segmentId?: string
  atSec: number
  reason: string
  speaker?: string
  durationSec?: number
  resolved?: boolean
}

/** Ứng viên đọc lại đoạn — so sánh Bản cũ / Bản mới trước khi dùng. */
export type AudioCandidate = {
  candidateId: string
  url: string
  durationSec: number
  previousUrl?: string
  previousDurationSec?: number
}

export type TtsState = 'missing' | 'installing' | 'stopped' | 'starting' | 'ready' | 'error'

/** GAP-21 — bộ đọc trên máy (`tts` trong config.json). */
export type AudioEngineStatus = {
  ready: boolean
  engine: string
  model: string
  device: string
  state: TtsState
  error: string
  note?: string
  rtf: number | null
}

export type TtsStatus = {
  ready: boolean
  engine: string
  model: string
  device: string
  state: TtsState
  error: string
  rtf: number | null
  sampleRate: number
  /** ffmpeg có trên máy chủ — thiếu thì chỉ soạn/duyệt bản đọc được */
  ffmpeg: boolean
  installedBytes?: number
  /** tiến độ cài khi `state: 'installing'` (A1Voice: bổ sung cho SSE `audio.install`) */
  install?: { percent: number; message: string } | null
}

export type TtsEngine = { id: string; name: string; description: string; installed: boolean }

/** GET /api/tts/install — mốc của lượt cài bộ đọc trên máy (voice.InstallProgress).
 * `state: 'idle'` = chưa từng cài; 'error' kèm `error` là câu lỗi cho người dùng. */
export type TtsInstallProgress = {
  state: 'idle' | 'running' | 'done' | 'error'
  step: string
  percent: number
  message: string
  bytes: number
  error?: string
  updated: string
}

/** Gợi ý trạng thái cho màn Audio: thiếu ffmpeg / bộ đọc chưa cài (A2Audio: GET S/audio/ready). */
export type AudioReady = {
  ffmpeg: boolean
  engine: AudioEngineStatus
  ready: boolean
  message: string
}

/** SSE `audio.render` (S/audio/events). */
export type AudioRenderEvent = {
  jobId: string
  chapter: number
  segment: number
  segmentId?: string
  segments: number
  speaker: string
  percent: number
  etaSec: number
  state: AudioJobState
  text?: string
  segmentPercent?: number
}

/** SSE `audio.queue` / `audio.export` / `audio.compose`. */
export type AudioQueueEvent = { jobs: AudioQueueJob[] }
export type AudioExportEvent = { volume: number; percent: number; state: 'drafting' | 'done' | 'error'; error?: string }
export type AudioComposeEvent = { chapter: number; state: 'drafting' | 'done' | 'error'; segments: number; percent: number; error?: string }

// ─── §10 Việc chạy nền (14-jobs-dock.md §1.7 · GAP-26) ──────────────────

export type JobKind = 'write' | 'audio' | 'voice' | 'export' | 'sync' | 'simulate' | 'import' | 'diag'
export type JobStatus = 'running' | 'paused' | 'waiting' | 'failed' | 'done'
export type JobAgent = 'architect' | 'writer' | 'editor' | 'arbiter'
/** Nút kết quả: `download` + path → tải tệp xuất · `reveal` → POST /api/reveal · `open` + href → route SPA. */
export type JobAction = {
  id?: string; label: string; primary?: boolean
  kind?: 'download' | 'reveal' | 'open'; path?: string; href?: string
  /** tên icon (docs/design/v1/icons); thiếu → theo kind */
  icon?: string
}
export type Job = {
  /** "<p>/<s>/<kind>/<n>" */
  id: string
  kind: JobKind
  title: string
  status: JobStatus
  /** 0..1; null = không rõ % */
  progress: number | null
  /** nhãn phụ dựng sẵn ("Chọn tuyến chính"); ở `waiting` là lời nhắc chờ bạn */
  step?: string
  unit?: { done: number; total: number; label: string }
  etaSec?: number
  /** Lời lỗi; việc ghép audio (§AE) gửi bản đầy đủ — đọc bằng `jobError(job)`. */
  error?: string | JobErrorInfo
  result?: {
    title: string; sub: string; actions: JobAction[]
    data?: ExportJobResult | SyncResult | SimulateJobResult | ImportProgress | AudioExportJobResult
    /** §AE — với việc audio, máy chủ đặt thẳng files/zipUrl trên result */
    files?: AudioExportJobResult['files']
    zipUrl?: string
  }
  /** route SPA của màn liên quan */
  href: string
  project: string
  story: string
  /** chỉ có ở kind `write` khi một trợ lý đang chạy */
  agent?: JobAgent
  startedAt: string
  finishedAt?: string
}
/** SSE /api/events: `job.created|job.progress|job.waiting|job.failed|job.done` mang Job đầy đủ; `job.removed` mang {id}. */
export type JobsResponse = { jobs: Job[] }
export type JobDetail = { schemaVersion: 1; project: string; story: string; job: Job; event: JobEvent | null }

/** M2 · Gói AI — vị trí đoạn bắt đầu từ 0; null = góp ý cả chương.
 * `paragraph` + `quote` là vị trí gốc (nội bộ, không hiện số đoạn); sau mỗi lần ghi chương
 * máy chủ giải lại mốc neo và trả `status`/`newQuote`/`candidates` (06-data-contract › Bổ sung §AI). */
export type ParagraphCommentStatus = 'pending' | 'sent' | 'fixed' | 'lost' | 'unsure' | 'done'
export type ParagraphCommentCandidate = { paragraph: number; snippet: string }
export type ParagraphComment = {
  id: string; paragraph: number | null; quote: string; chips: string[]; text: string
  author: string; createdAt: string; appliedAt?: string
  /** Vòng đời; thiếu (bản cũ) → suy ra từ appliedAt (sent) hay pending. */
  status?: ParagraphCommentStatus
  sentAt?: string; fixedAt?: string; resolvedAt?: string
  /** `agent` = Người viết báo đã sửa · `quote_gone` = câu trích cũ mất, tìm được đoạn thay. */
  fixedBy?: 'agent' | 'quote_gone'
  /** Người viết không báo đã sửa và câu trích vẫn còn → về Chờ gửi. */
  unfixed?: boolean
  /** Máy tự gắn sang đoạn khác (gạch chân nét đứt + “đã dời”). */
  moved?: boolean
  /** Người dùng vừa gắn lại thủ công. */
  relinked?: boolean
  /** Gửi cho cả chương, kèm câu trích cũ. */
  whole?: boolean
  /** Câu sau khi sửa (thẻ Đã sửa: Trước/Sau). */
  newQuote?: string
  /** Đoạn gợi ý khi status = unsure (tối đa 3). */
  candidates?: ParagraphCommentCandidate[]
  /** Mốc neo đã giải (0-based): máy chủ điền sau mỗi lần ghi chương; badge/gạch chân
   *  gắn theo `anchor.paragraph`/`anchor.quote`. Vắng khi chưa giải được. */
  anchor?: { paragraph: number; quote: string; prefix?: string; suffix?: string } | null
}
export type ParagraphCommentInput = Pick<ParagraphComment, 'paragraph' | 'quote' | 'chips' | 'text'>
/** PATCH …/chapters/:n/comments/:id — hành động của người đọc lên góp ý. */
export type ParagraphCommentPatch =
  | { action: 'resolve' }
  | { action: 'reopen' }
  | { action: 'relink'; paragraph: number; quote?: string }
  | { action: 'resend_whole' }
/** Khung `live`/SSE `comments.reanchored {chapter, fixed, unfixed, need}` sau lần viết lại. */
export type CommentsReanchored = { chapter: number; fixed: number; unfixed: number; need: number }
export const PARAGRAPH_COMMENT_CHIPS = ['Ngắn lại', 'Thêm thoại', 'Kịch tính hơn', 'Giữ giọng văn', 'Chậm nhịp lại'] as const

/** GAP-30 — thông báo đẩy và hộp thư theo tài khoản. */
export type NotifyPrefs = {
  enabled: boolean
  types: { gate: boolean; error: boolean; job: boolean; done: boolean; security: boolean }
  digest: 'each' | 'five' | 'day'
  quiet: { from: string; to: string }
  version: string
}
export type PushSubscriptionInfo = {
  endpoint: string; keys: { p256dh: string; auth: string }; device: string; session: string; createdAt: string
}
export type InboxNotification = {
  id: string; type: 'gate' | 'error' | 'job' | 'done' | 'security' | 'import' | 'sync' | 'cocreate'
  story: string; chapter?: number; job?: string; title: string; body: string; url: string
  actions: { action: string; title: string; url?: string }[]
  createdAt: string; read: boolean; score?: number
}
export type NotificationsResponse = { notifications: InboxNotification[]; unread: number; nextBefore?: string }

/** GAP-31 — lịch sử từ đĩa, cùng chi tiết như sự kiện trực tiếp. */
export type ActivityEvent = HostEvent & {
  chapter: number; username?: string; tool?: string; args?: string; result?: string
  /** Vài con số người đọc cần, trích từ kết quả gốc trước khi `result` bị cắt 800 ký tự. */
  resultHint?: string
  durationMs?: number; status?: 'running' | 'ok' | 'error'
  retries?: { atSec: number; error: string; waitSec: number; ok: boolean }[]
  /** GAP-36: mục thẩm duyệt của lần chấm này (chỉ dòng Biên tập viên) — BE cần gửi kèm sự kiện. */
  review?: { round?: number; items: ReviewItem[] }
}
export type ActivityHistoryQuery = {
  chapter?: number; agent?: string; level?: 'error'; mine?: boolean
  q?: string; before?: string; limit?: number
}
export type ActivityHistory = {
  events: ActivityEvent[]; nextCursor: string | null
  chapters: { chapter: number; title: string; score?: number; eventCount: number }[]
  counts: { total: number; byAgent: Record<string, number>; errors: number; mine: number }
}

// ─── GAP-34 — Chọn nơi lưu truyện (09-auth-onboard.md §10) ─────────────────

/** GET /api/fs/places — Nơi trên máy chủ (admin). `path` chỉ dùng để duyệt/tooltip, không hiện ra UI. */
export type FsPlace = { id: string; label: string; path: string; icon: string }
/** GET /api/fs/list?path= — chỉ thư mục. `projects` chỉ có khi kind = library. */
export type FsFolder = { name: string; path: string; kind: 'dir' | 'library' | 'readonly'; items: number; writable: boolean; projects?: number }
export type FsMkdirResult = { name: string; path: string; display: string }
/** GET /api/library/inspect?path= — kết luận kiểm tra một thư mục (§10.4). */
export type LibraryInspect = {
  kind: 'found' | 'empty' | 'other' | 'readonly' | 'missing'
  projects: number; stories: number; items: number
  place: string; display: string; name: string
  /** Tên dự án đầu tiên khi kind = found. */
  projectName?: string
  storageWarning?: string
}

// ─── Quy tắc viết (GAP-32, 11-api-contract.md · “Quy tắc của truyện”) ────
export type RuleChipKind = 'ban' | 'chr' | 'max' | 'genre' | 'pref'
export type RuleChip = { kind: RuleChipKind; text: string; /** chỉ 'max': tối đa N lần/chương */ limit?: number }
export type RuleStatus = 'ready' | 'degraded'
/** Một dòng của `quy-tac.md` đã được chuẩn hoá (theo cấp). `options`: 0–3 cách hiểu viết lại cho khung “chưa chắc”. */
export type RuleItem = {
  text: string; scope: ConfigScope; chips: RuleChip[]
  /** GAP-32 bổ sung (Câu 26a): máy chủ đánh dấu quy tắc còn nghi ngờ. */
  uncertain: boolean
  /** Câu lý do chưa chắc (rỗng khi `uncertain` false) — chữ của khung cam. */
  uncertainNote: string
  options?: string[]; status: RuleStatus
}
/** Luật “Học từ góp ý” do Trọng tài thêm lúc viết. */
export type RuntimeRule = {
  id: string; text: string; chapter: number; createdAt: string; chips: RuleChip[]
  /** Dữ liệu đã lưu: danh sách lý do (rỗng = rõ ràng). */
  uncertain: string[]
  /** Câu đầu của `uncertain` để hiện trong khung cam. */
  uncertainNote?: string
  options?: string[]; status: RuleStatus
}
export type RuleStructured = { genre?: string; forbidden_chars?: string[]; forbidden_phrases?: string[]; fatigue_words?: Record<string, number> }
export type RuleSnapshot = { version: number; status: RuleStatus; structured: RuleStructured; preferences: string; sources: string[]; uncertain: string[] }
export type RulesTiming = { effectiveFrom: number; inProgress: number; completed: number }
/** Effective project/story rules. Historical snapshots may omit unprovable word provenance. */
export type FatigueWord = {
  word: string; per1000: number; perChapter: number
  source?: 'default' | 'pack' | 'project' | 'story' | 'learned'
  basePerChapter?: number
  baseSource?: 'default' | 'pack' | 'project'
}
export type RulesEffective = {
  snapshot: RuleSnapshot | null; items: RuleItem[]; runtime?: RuntimeRule[]; timing?: RulesTiming
  fatigueWords?: FatigueWord[]; forbiddenDefaults?: string[]
}
export type RuleUnderstanding = {
  structured: RuleStructured; preferences: string; uncertain: string[]
  /** Câu đầu của `uncertain` (Câu 26a) — chữ của khung cam. */
  uncertainNote?: string
  options?: string[]; chips: RuleChip[]; status: RuleStatus
}
export type RuleApplyResult = { snapshot: RuleSnapshot; effectiveFrom: number; pinnedChapter?: number; jobId?: string }

// ─── Xưng hô & quan hệ (GAP-38, 22-brief-2510.md D7/D8) ─────────────────
/** Một dòng bảng xưng hô = một chiều (from gọi to). `untilChapter` khác null = dòng đã hết hiệu lực. */
export type AddressTerm = {
  id: string; from: string; fromName: string; to: string; toName: string
  call: string; self: string; fromChapter: number; untilChapter: number | null; reason: string
}
/**
 * Một dòng “Lịch sử quan hệ theo chương”. `type` là kiểu quan hệ engine ghi (D10), dòng cũ đọc là `rel`;
 * màu chip của D7 gom theo ba nhóm của kiểu này (Câu 20).
 */
export type RelationChange = {
  chapter: number; a: string; aName: string; b: string; bName: string
  before: string; after: string; reason: string; type: BrainEdgeType
}
export type AddressTerms = {
  terms: AddressTerm[]; relations: RelationChange[]; latestWritten: number
  /**
   * Trợ lý đang viết → mọi thao tác sửa trả 423. `state` = đang viết lần đầu hay viết lại (Câu 23),
   * để banner khoá đọc đúng “đang viết chương N” / “đang viết lại chương N”.
   */
  writing: { chapter: number; role: string; state: 'writing' | 'rewriting' } | null
}
/**
 * `apply` khi “từ chương” chạm chương đã viết: `next` = chỉ ghi nhận cho chương chưa viết ·
 * `mark` = đưa chương đã viết vào hàng chờ sửa. Câu 21 đổi tên `forward` → `next`.
 */
export type AddressApply = 'next' | 'mark'
/** Sửa trên web bắt buộc lý do (Câu 20/21) — máy chủ trả 400 `{field:"reason"}` khi thiếu. */
export type AddressTermInput = {
  term: { id?: string; from: string; to: string; call: string; self: string; fromChapter: number; reason: string }
  apply?: AddressApply
}
export type RelationInput = {
  a: string; b: string; after: string; reason: string; fromChapter: number
  /** Kiểu quan hệ engine ghi; bỏ trống thì máy chủ ghi `rel`. */
  type?: BrainEdgeType
  apply?: AddressApply
}

// ─── Não truyện (GAP-39, D10) — màn D7 dùng `chapters` + `entities[].appears` ─
export type BrainEntityType = 'char' | 'place' | 'obj' | 'ev' | 'fs' | 'rule'
export type BrainEdgeType = 'enemy' | 'ally' | 'work' | 'own' | 'secret' | 'rel'
export type BrainEntity = {
  id: string; type: BrainEntityType; name: string; role: string; desc: string
  fromChapter: number; appears: number[]
  /** Quy tắc thế giới giữ nguyên nhóm lưu trong nền móng. */
  category?: string
  /** Chữ viết tắt avatar người dùng tự đặt (04 › Avatar nhân vật). */
  ini?: string
  /** Loại vai của nhân vật (Gói S) — màu avatar/nút đồ thị; thực thể khác bỏ trống. */
  kind?: CharacterKind
  /** Chương lộ vai thật; thiếu/`null` = không có bí mật. */
  revealAt?: number | null
  /** Tên khác dùng trong chính văn — Trình đọc gạch chấm mọi tên này (M8 `state=person`). */
  aliases: string[]
  /** Diễn biến theo chương — sheet Não truyện (M8) và cột phải bản máy tính. */
  timeline: { chapter: number; text: string }[]
}
export type BrainEdge = { a: string; b: string; phases: { fromChapter: number; type: BrainEdgeType; label: string }[] }
export type Brain = {
  /** `writing` = đang viết lần đầu (Câu 23); thanh chương vẽ như `rewriting` nhưng tooltip “đang viết”. */
  chapters: { n: number; title: string; status: 'written' | 'rewriting' | 'writing' | 'unwritten' }[]
  entities: BrainEntity[]; edges: BrainEdge[]
}

// ─── Khung truyện & bản đồ giữ chân (GAP-37, D5) ────────────────────────
/** Một nhóm người nghe của Master §1.2 — nhãn và “họ muốn…” lấy nguyên từ gói thể loại. */
export type FrameAudience = { n: number; label: string; wants: string }
/** Ai đặt khung truyện: gói thể loại điền sẵn · Kiến trúc sư ghi khi lập dàn ý · người dùng lưu từ web. */
export type FrameSource = 'pack' | 'architect' | 'user'
export type Frame = {
  /** chỉ số trong `audiences[]` đã chọn — gói C mặc định `[2, 1]`, đã chọn biến thể thì còn một số (Câu 18) */
  audience: number[]
  difference: string; sequelSeed: string
  source: FrameSource; audiences: FrameAudience[]
}
export type FrameInput = { audience?: number[]; difference?: string; sequelSeed?: string }

/** Bốn kiểu móc của Master §7 + `seed` cho chương cuối (Câu 19) — không có kiểu khác. */
export type RetentionHookType = 'info' | 'threat' | 'decision' | 'line' | 'seed'
export type RetentionPayoffTag = 'promise' | 'midpoint' | 'climax'
export type RetentionChapter = {
  n: number; title: string; status: 'written' | 'writing' | 'unwritten'
  questions: string[]; promise: string
  payoff: { tag: RetentionPayoffTag | ''; text: string } | null
  hook: { type: RetentionHookType; waiting: string } | null
}
export type RetentionCheckRef = 'Master §2' | 'Master §3' | 'Master §7' | 'Master §8'
export type RetentionCheck = { ref: RetentionCheckRef; ok: boolean; text: string; chapters: number[] }
/** Vị trí lời hứa/cú lật là 0..1 của tổng số chương (mốc cuối chương); null khi chưa đánh dấu. */
export type Retention = {
  /** Dàn ý cũ chưa có trường giữ chân: không chạy 4 phép kiểm (Câu 19) — UI hiện trạng thái rỗng. */
  empty: boolean
  chapters: RetentionChapter[]
  promiseAt: number | null; twistAt: number | null
  checks: RetentionCheck[]
}

// ─── Đo bằng máy (D6) ───────────────────────────────────────────────────
export type MetricSeverity = 'error' | 'warning'
/**
 * Một số đo của gói thể loại (`internal/rules/metrics.go`): `value` là số trần,
 * `unit` và `limit` là chữ máy chủ đã viết sẵn ("tiếng", "≤ 40", "40–55%") — UI
 * hiện nguyên văn, không tự đặt chuẩn. `value` null khi số đo không áp cho chương
 * (mở đầu trùng truyện cũ ngoài chương 1) → UI hiện “—”. `severity`/`ref` chỉ có
 * khi tệp luật của gói ghi mức và mục luật cho số đo đó.
 */
export type ChapterMetric = {
  key: string; name: string; value: number | null
  unit: string; limit: string; ok: boolean
  /** chi tiết thêm khi số > 0 (“dài nhất 96 tiếng”) — Nhật ký hiện thay chuẩn */
  detail?: string
  severity?: MetricSeverity; ref?: string
}
/** Từ lặp có mật độ cao nhất chương — dòng “Từ mệt nhiều nhất · “x”” của D6. */
export type TiredWord = { word: string; per1000: number; limit: number; ok: boolean }
/** `GET …/metrics?chapters=last3|all` — chương đã viết, tăng dần theo số chương. */
export type StoryMetrics = { chapters: { n: number; metrics: ChapterMetric[]; tiredWord: TiredWord | null }[] }
export type MetricsScope = 'last3' | 'all'

// ─── Não dự án (D14 · GAP-43…45) ──────────────────────────────────────
// Sổ cái (LedgerAxis/LedgerCheck/LedgerCell/LedgerCheckResult/LedgerSuggestion/
// ProjectLedger) và Vũ trụ chung (ProjectUniverseEntity/UniverseCheckItem/
// UniverseKeptItem/ProjectUniverse) sinh từ Go — xem đầu tệp.
export type ProjectSearch = { entities: ProjectUniverseEntity[]; passages: { story: string; storyTitle: string; ch: number; quote: string; hl: [number, number][] }[]; total: number }

// ─── Não hệ thống (D15 · GAP-46, chỉ quản trị) ─────────────────────────
// Lesson, LessonEvidence, LessonPack, LessonsResponse, ViolationRow và
// ViolationsResponse (kèm các union trạng thái) sinh từ Go — xem đầu tệp.
export type LessonAction = { action: 'keep' | 'drop' | 'edit' | 'undo'; text?: string; scope?: LessonScope }

// ─── Chất lượng truyện (D13 · GAP-42) ──────────────────────────────────
// Quality, QualityChapter, QualityRewrite, QualityIssue, QualityCell,
// QualityChapterStatus và QualityIssueState sinh từ Go — xem đầu tệp.
