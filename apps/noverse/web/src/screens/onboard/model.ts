// Mô hình dữ liệu + tiện ích thuần cho A3 · Thiết lập lần đầu (Onboard.dc.html).
// Bước theo vai trò, slug thư mục, mật khẩu tạm, mức suy luận và “Thiết lập sẽ lưu”.
import type {
  Config, ConfigAccount, ConfigModelInfo, ConfigResponse, LibraryConfig, LibraryResponse, Pairing, ProjectSummary,
  ProviderConfig, ProviderPreset, ProjectPermissions, ProjectRole, Role, Session, StorySummary, User,
} from '../../api/types'
import { slugifyOrEmpty } from '../../lib/slug'
import type { LibraryChoice } from './libraryChoice'
import { t } from '../../i18n/vi'

export type StepKey = 'profile' | 'password' | 'provider' | 'model' | 'library' | 'project' | 'mobile' | 'done'

// Onboard.dc.html ADMIN / WRITER: admin 7 bước (QĐ 6 — bỏ bước Ngôn ngữ sáng tác), người viết 4 bước.
export const ADMIN_STEPS: StepKey[] = ['profile', 'provider', 'model', 'library', 'project', 'mobile', 'done']
export const BASIC_STEPS: StepKey[] = ['profile', 'password', 'mobile', 'done']
export const OPTIONAL_STEPS: StepKey[] = ['password', 'project', 'mobile']

/** Bản xuất tự động khi xong một tập (Onboard.dc.html › formats) — `narration` là bản đọc TTS. */
export type OnboardFormat = 'txt' | 'epub' | 'narration'
export const ONBOARD_FORMATS: OnboardFormat[] = ['txt', 'epub', 'narration']
export const FORMAT_LABEL: Record<OnboardFormat, string> = { txt: 'Văn bản', epub: 'Ebook', narration: 'Bản đọc TTS' }

export const STEP_LABEL: Record<StepKey, string> = {
  profile: 'Hồ sơ',
  password: 'Mật khẩu riêng',
  provider: 'Nhà cung cấp AI',
  model: 'Model & trợ lý',
  library: 'Nơi lưu truyện',
  project: 'Dự án đầu tiên',
  mobile: 'Điện thoại & nhóm',
  done: 'Hoàn tất',
}

/** Visible provider-card copy from Onboard.dc.html; IDs stay server-owned. */
export const PROVIDER_PRESENTATION: Record<string, { label: string; hint: string }> = {
  anthropic: { label: 'Anthropic', hint: 'API Key · Claude' },
  openai: { label: 'OpenAI', hint: 'API Key · GPT' },
  'openai-codex': { label: 'ChatGPT Plus/Pro', hint: 'Gói thuê bao · đăng nhập OAuth' },
  'xai-oauth': { label: 'SuperGrok / X Premium+', hint: 'Gói thuê bao · đăng nhập OAuth' },
  'anthropic-oauth': { label: 'Claude Pro/Max', hint: 'Gói thuê bao · đăng nhập OAuth' },
  deepseek: { label: 'DeepSeek', hint: 'API Key' },
  gemini: { label: 'Gemini', hint: 'API Key' },
  groq: { label: 'Groq', hint: 'API Key' },
  mistral: { label: 'Mistral', hint: 'API Key' },
  xai: { label: 'xAI (Grok)', hint: 'API Key' },
  openrouter: { label: 'OpenRouter', hint: 'API Key · nhiều model' },
  'opencode-go': { label: 'OpenCode Go', hint: 'Gói thuê bao · API Key · Kimi, DeepSeek, GLM…' },
  ollama: { label: 'Ollama', hint: 'Cục bộ / offline' },
  lmstudio: { label: 'LM Studio', hint: 'Cục bộ / OpenAI API' },
  custom: { label: 'Khác / proxy', hint: 'Base URL tuỳ chỉnh' },
}

/** Câu dẫn của bước nhà cung cấp khi chưa có thiết lập sẵn. */
export const PROVIDER_INTRO = 'Chọn một nhà cung cấp bên dưới rồi nhập khoá, hoặc đăng nhập gói thuê bao (ChatGPT Plus/Pro, Claude Pro/Max, SuperGrok).'

/** Mô tả thẻ đăng nhập OAuth theo gói thuê bao của preset. */
export const oauthDescription = (preset: Pick<ProviderPreset, 'id'>): string =>
  preset.id === 'xai-oauth'
    ? 'Dùng hạn mức gói SuperGrok / X Premium+ bạn đang có, không cần API key xAI. Noverse tự giữ kết nối luôn còn hiệu lực.'
    : preset.id === 'anthropic-oauth'
      ? 'Dùng hạn mức gói Claude Pro/Max bạn đang có, không cần API key Anthropic. Noverse tự giữ kết nối luôn còn hiệu lực.'
      : 'Dùng hạn mức gói ChatGPT bạn đang có, không cần mua API riêng. Noverse tự giữ kết nối luôn còn hiệu lực.'

/** Cách đăng nhập OAuth một preset cho phép: provider chỉ có device-code (xAI) không có “Mở trang đăng nhập” localhost. */
export type OAuthChoice = { device: boolean; icon: string; title: string; desc: string }
export const oauthChoices = (preset: Pick<ProviderPreset, 'deviceOnly'>): OAuthChoice[] => {
  const device: OAuthChoice = preset.deviceOnly
    ? { device: true, icon: 'keyboard', title: 'Đăng nhập bằng mã thiết bị', desc: 'Mở link xác nhận trên bất kỳ thiết bị nào, rồi nhập mã hiển thị ở đây' }
    : { device: true, icon: 'keyboard', title: 'Dùng mã thiết bị', desc: 'Khi bạn ở máy khác hoặc trên điện thoại' }
  if (preset.deviceOnly) return [device]
  return [{ device: false, icon: 'global', title: 'Mở trang đăng nhập', desc: 'Khi bạn đang dùng chính máy cài Noverse' }, device]
}

/** Hướng dẫn dưới mã thiết bị: tên dịch vụ phải đúng provider. */
export const deviceCodeHint = (presetId: string): string =>
  `trên điện thoại hoặc máy bất kỳ, đăng nhập ${presetId === 'xai-oauth' ? 'tài khoản X / xAI' : 'ChatGPT'} rồi nhập mã này.`

/** Hướng dẫn trong khung dán mã (Claude Pro/Max): mở link, đăng nhập, dán mã trang Claude hiển thị. */
export const PASTE_CODE_HINT = 'trên điện thoại hoặc máy bất kỳ, đăng nhập Claude rồi dán mã hiển thị (dạng code#state) hoặc cả URL chuyển hướng vào ô bên dưới.'

export const stepsFor = (role: Role): StepKey[] => (role === 'admin' ? ADMIN_STEPS : BASIC_STEPS)

export const stepLabel = (key: StepKey, role: Role): string =>
  key === 'mobile' && role !== 'admin' ? 'Điện thoại' : STEP_LABEL[key]

/** Khối “Lần chạy đầu ✓✓” ở đầu rail chỉ hiện khi vừa tạo tài khoản quản trị ở A0 (Setup.dc.html). */
const FROM_SETUP_KEY = 'ainovel.fromSetup'
export const markFromSetup = () => { try { sessionStorage.setItem(FROM_SETUP_KEY, '1') } catch { /* chế độ riêng tư */ } }
export const fromSetup = () => { try { return sessionStorage.getItem(FROM_SETUP_KEY) === '1' } catch { return false } }
export const clearFromSetup = () => { try { sessionStorage.removeItem(FROM_SETUP_KEY) } catch { /* chế độ riêng tư */ } }

/**
 * Mã khôi phục cấp kèm lúc tạo quản trị ở A0 (POST /api/setup/admin) — bước Hoàn tất hiện đúng mã đó
 * thay vì gọi POST /api/me/recovery (máy chủ đã có mã chưa dùng nên sẽ trả 409).
 */
const SETUP_RECOVERY_KEY = 'ainovel.setupRecovery'
export const markSetupRecovery = (code: string) => { try { sessionStorage.setItem(SETUP_RECOVERY_KEY, code) } catch { /* chế độ riêng tư */ } }
export function takeSetupRecovery(): string {
  try {
    const code = sessionStorage.getItem(SETUP_RECOVERY_KEY) ?? ''
    sessionStorage.removeItem(SETUP_RECOVERY_KEY)
    return code
  } catch { return '' }
}

export const USERNAME_RE = /^[a-z0-9._-]{2,32}$/

/** “kho-truyen-cu” → “Kho Truyen Cu” (tên gợi ý cho dự án nhận từ thư mục). */
export const humanizeSlug = (slug: string) =>
  slug.split(/[-_]+/).filter(Boolean).map((part) => part.charAt(0).toUpperCase() + part.slice(1)).join(' ')

/**
 * Provider đang có thật trong tệp cấu hình: cần cả `provider` lẫn mục `providers.<id>`.
 * Tệp rỗng (hoặc chỉ có chuỗi rỗng do lần ghi lỗi trước) ⇒ '' = chưa cấu hình.
 */
export const configuredProviderId = (config: Config | null): string => {
  const id = config?.provider ?? ''
  return id && config?.providers?.[id] ? id : ''
}

export type ProviderConnection = 'connected' | 'untested' | 'none'

/**
 * Trạng thái kết nối của provider đang chọn (Onboard.dc.html › bước provider).
 * “Đã kết nối” chỉ khi test vừa thành công, OAuth vừa xong, tài khoản đã liên kết,
 * hoặc đang xem đúng provider đã lưu sẵn trong cấu hình. Vừa chọn provider mới ⇒ “Chưa kết nối”.
 */
export function providerConnection(input: {
  configuredProvider: string
  providerId: string
  changing: boolean
  testOk: boolean
  testFailed: boolean
  oauthDone: boolean
  accountConnected: boolean
}): ProviderConnection {
  if (!input.providerId) return 'none'
  if (input.testFailed) return 'untested'
  if (input.testOk || input.oauthDone || input.accountConnected) return 'connected'
  if (!input.changing && input.configuredProvider === input.providerId) return 'connected'
  return 'untested'
}

export const providerStatusLabel = (state: ProviderConnection): string =>
  state === 'connected' ? 'Đã kết nối' : state === 'untested' ? 'Chưa kết nối' : 'Chưa có'

/** PUT luôn ghi đè toàn bộ tệp của cấp đó ⇒ lấy nội dung tệp (target), không lấy `effective` đã trộn. */
export const configFileOf = (response: ConfigResponse): Config => response.target.config

/**
 * Bản ghi gửi lên: bỏ khoá rỗng/chưa đặt ở cấp cao nhất (`provider: ""`, `model: ""`,
 * `providers: {}`…) để mỗi bước không ghi đè giá trị rỗng cho trường mình không sở hữu.
 */
export const writableConfig = (config: Config | null): Config =>
  Object.fromEntries(Object.entries(config ?? {}).filter(([key, value]) => {
    if (value === '' || value === null || value === undefined) return false
    if (key === 'language' && value !== 'vi') return false // zh chưa hỗ trợ: tệp cũ còn zh thì không gửi lại
    if (key === 'providers' && typeof value === 'object' && Object.keys(value).length === 0) return false
    return true
  })) as Config

/**
 * Provider gửi kèm /api/config/test: bỏ các trường chỉ tồn tại ở phản hồi GET
 * (`api_key_set`, `api_key_hint`) vì server giải mã với DisallowUnknownFields.
 */
export function draftProvider(
  base: ProviderConfig | undefined, preset: ProviderPreset | null, baseURL: string, apiKey: string,
): ProviderConfig {
  const provider: ProviderConfig = {}
  const type = preset?.type || base?.type
  const api = preset?.api || base?.api
  const url = baseURL || preset?.baseURL || base?.base_url
  if (type) provider.type = type
  if (api) provider.api = api
  if (url) provider.base_url = url
  if (base?.models) provider.models = base.models
  if (base?.extra_body) provider.extra_body = base.extra_body
  if (base?.extra) provider.extra = base.extra
  if (base?.stream_idle_timeout) provider.stream_idle_timeout = base.stream_idle_timeout
  if (apiKey) provider.api_key = apiKey
  return provider
}

export type AdvanceInput = {
  busy: boolean
  stepKey: StepKey
  name: string
  pwCurrent: string
  pwNext: string
  pwRepeat: string
  dirs: LibraryConfig['dirs'] | null
  projMode: 'adopt' | 'new'
  candidate: string
  projName: string
  slugError: string
  changing: boolean
  providerId: string
  preset: ProviderPreset | null
  customName?: string
  apiKey: string
  existingKeySet: boolean
  /** Nơi lưu truyện đã chọn không ghi được (hoặc chưa chọn được gì) → khoá Tiếp tục. */
  libraryBlocked?: boolean
  oauthConnected: boolean
  model: string
  /** Bước Dự án: server đã có dự án và người dùng không tạo thêm ⇒ chỉ đi tiếp. */
  projectReady?: boolean
}

/** Điều kiện bật “Tiếp tục” — cùng một nguồn cho nút và cho thông báo lỗi ở advance(). */
export function canAdvance(input: AdvanceInput): boolean {
  if (input.busy) return false
  switch (input.stepKey) {
    case 'profile':
      return input.name.trim().length > 0
    case 'password':
      return input.pwCurrent.length > 0 && input.pwNext.length >= 8 && input.pwNext === input.pwRepeat
    case 'provider':
      if (!input.changing) return true
      if (!input.providerId || !input.preset) return false
      if (input.providerId === 'custom' && !/^[a-z0-9][a-z0-9_-]*$/.test(input.customName ?? '')) return false
      if (input.preset.oauth) return input.oauthConnected
      return !input.preset.keyRequired || input.apiKey.length > 0 || input.existingKeySet
    case 'model':
      return Boolean(input.providerId) && input.model.trim().length > 0
    case 'library':
      return Boolean(input.dirs) && !input.libraryBlocked
    case 'project':
      if (input.projectReady) return true
      return input.projMode === 'adopt'
        ? Boolean(input.candidate)
        : Boolean(input.projName.trim()) && !input.slugError
    default:
      return true
  }
}

/**
 * Mặc định bước Dự án: chỉ quyết định khi đã đọc xong thư viện (`undefined` = chưa đọc).
 * Có thư mục chờ nhận ⇒ “Dùng thư mục có sẵn” + thư mục đầu; không có ⇒ form tạo mới rỗng.
 */
export function projectDefaults(candidates: string[] | undefined): { mode: 'adopt' | 'new'; candidate: string } | null {
  if (!candidates) return null
  if (candidates.length === 0) return { mode: 'new', candidate: '' }
  return { mode: 'adopt', candidate: candidates[0] }
}

/** Lỗi slug của form tạo mới: trùng dự án đã có hoặc trùng thư mục đang chờ nhận. */
export function projectSlugError(name: string, projects: ProjectSummary[], candidates: string[]): string {
  if (!name.trim()) return ''
  const slug = slugifyOrEmpty(name)
  if (!slug) return 'Tên cần có ít nhất một chữ cái hoặc số.'
  if (projects.some((project) => project.id === slug)) return 'Thư mục này đã có dự án trong thư viện.'
  if (candidates.includes(slug)) return 'Thư mục này đã có trong thư viện.'
  return ''
}

/**
 * Dự án của bước “Dự án đầu tiên” lấy từ server, không từ state cục bộ: tải lại trang vẫn thấy
 * dự án đã tạo/nhận (không tạo trùng). Ưu tiên dự án vừa tạo; không có thì dự án thật đầu tiên
 * (bỏ dự án ẩn danh `_` gom thư mục cũ).
 */
export function onboardProject(projects: ProjectSummary[], createdId: string | null): ProjectSummary | null {
  return (createdId ? projects.find((project) => project.id === createdId) : undefined)
    ?? projects.find((project) => !project.legacy) ?? null
}

/** “Đã có N truyện” ở bước Nơi lưu truyện: tổng số truyện của mọi dự án (kể cả dự án ẩn danh gom truyện cũ). */
export const libraryStoryCount = (library: Pick<LibraryResponse, 'projects'> | null): number =>
  (library?.projects ?? []).reduce((sum, project) => sum + project.storyCount, 0)


/**
 * Bước Mật khẩu riêng: chỉ nói “mật khẩu tạm” khi server còn đánh dấu `mustChangePassword`
 * (admin vừa tạo/đặt lại). Đã có mật khẩu riêng ⇒ đổi tuỳ chọn với mật khẩu hiện tại.
 */
export function passwordStepCopy(mustChange: boolean, shared = false): { title: string; lead: string; currentLabel: string; missingCurrent: string } {
  return mustChange
    ? {
        title: 'Đặt mật khẩu của riêng bạn',
        lead: shared ? 'Quản trị viên đã tạo tài khoản với mật khẩu tạm. Đổi ngay để chỉ mình bạn biết. Đổi xong, mọi thiết bị phải đăng nhập lại ở cả Noverse và Viderse, kể cả máy này.' : 'Quản trị viên đã tạo tài khoản với mật khẩu tạm. Đổi ngay để chỉ mình bạn biết — các thiết bị khác đang dùng mật khẩu tạm sẽ bị đăng xuất.',
        currentLabel: 'Mật khẩu tạm',
        missingCurrent: 'Nhập mật khẩu tạm đang dùng',
      }
    : {
        title: 'Mật khẩu riêng',
        lead: shared ? 'Bạn đã có mật khẩu của riêng mình. Đổi tại đây nếu muốn. Đổi xong, mọi thiết bị phải đăng nhập lại ở cả Noverse và Viderse, kể cả máy này.' : 'Bạn đã có mật khẩu của riêng mình. Đổi tại đây nếu muốn — các thiết bị khác sẽ bị đăng xuất, máy này giữ nguyên phiên.',
        currentLabel: 'Mật khẩu hiện tại',
        missingCurrent: 'Nhập mật khẩu hiện tại',
      }
}

const PASSWORD_WORDS = ['vang', 'may', 'mua', 'gio', 'nang', 'sang', 'toi', 'bien', 'nui', 'hoa', 'tra', 'moc']
export function makePassword(): string {
  const pick = () => PASSWORD_WORDS[Math.floor(Math.random() * PASSWORD_WORDS.length)]
  return `${pick()}-${pick()}-${1000 + Math.floor(Math.random() * 9000)}`
}

export const EFFORTS: { key: string; label: string }[] = [
  { key: '', label: 'Mặc định' },
  { key: 'off', label: 'Tắt' },
  { key: 'low', label: 'Thấp' },
  { key: 'medium', label: 'Vừa' },
  { key: 'high', label: 'Cao' },
  { key: 'xhigh', label: 'Rất cao' },
  { key: 'max', label: 'Tối đa' },
]
export const effortLabel = (key: string) => EFFORTS.find((entry) => entry.key === key)?.label ?? 'Mặc định'

export const formatDate = (date: Date) =>
  `${String(date.getDate()).padStart(2, '0')}/${String(date.getMonth() + 1).padStart(2, '0')}/${date.getFullYear()}`

export type OAuthUi =
  | { phase: 'idle' }
  | { phase: 'browser' | 'device'; id: string; url: string; userCode: string; verifyURL: string; expiresAt: string }
  | { phase: 'paste'; id: string; url: string; expiresAt: string }
  | { phase: 'done'; account: { label: string; email: string; plan: string }; models: ConfigModelInfo[]; via: 'browser' | 'device' | 'paste' }
  | { phase: 'failed'; message: string }

export type RoleModelDraft = Record<'architect' | 'writer' | 'editor', { model: string; effort: string }>

export type OnboardModel = {
  user: User
  isAdmin: boolean
  steps: StepKey[]
  stepKey: StepKey
  idx: number
  sessionExpiresAt: string | null
  pwChanged: boolean
  // hồ sơ
  name: string
  setName: (value: string) => void
  // mật khẩu riêng
  pwCurrent: string
  pwNext: string
  pwRepeat: string
  setPwCurrent: (value: string) => void
  setPwNext: (value: string) => void
  setPwRepeat: (value: string) => void
  // nhà cung cấp
  config: Config | null
  presets: ProviderPreset[]
  providerId: string
  setProviderId: (value: string) => void
  providerLabel: string
  providerDetail: string
  configuredProvider: string
  providerStatus: ProviderConnection
  preset: ProviderPreset | null
  accountInfo: ConfigAccount | null
  changing: boolean
  setChanging: (value: boolean) => void
  apiKey: string
  setApiKey: (value: string) => void
  baseURL: string
  setBaseURL: (value: string) => void
  customName: string
  setCustomName: (value: string) => void
  customType: string
  setCustomType: (value: string) => void
  testState: 'idle' | 'run' | 'ok' | 'fail'
  testLatency: number
  testMessage: string
  /** G6 — model máy chủ đã dùng khi thử kết nối */
  testModel: string
  runTest: () => Promise<boolean>
  oauth: OAuthUi
  oauthPending: boolean
  oauthBusy: boolean
  oauthMutationPending: boolean
  startOauth: (device: boolean) => void
  cancelOauth: () => void
  disconnectProvider: () => void
  /** Luồng dán mã (Claude Pro/Max): nội dung ô nhập, lỗi lần dán gần nhất và hành động gửi. */
  oauthCode: string
  setOauthCode: (value: string) => void
  oauthCodeError: string
  oauthCodePending: boolean
  submitOauthCode: () => void
  // model & agent
  models: ConfigModelInfo[]
  model: string
  setModel: (value: string) => void
  effort: string
  setEffort: (value: string) => void
  shared: boolean
  setShared: (value: boolean) => void
  roleModels: RoleModelDraft
  setRoleModel: (role: keyof RoleModelDraft, patch: Partial<{ model: string; effort: string }>) => void
  // thư viện
  library: LibraryResponse | null
  libraryLoading: boolean
  /** Bước Nơi lưu truyện: lựa chọn thư mục + bộ chọn (libraryChoice.ts). */
  libChoice: LibraryChoice
  dirs: LibraryConfig['dirs'] | null
  autoExport: boolean
  setAutoExport: (value: boolean) => void
  formats: OnboardFormat[]
  toggleFormat: (format: OnboardFormat) => void
  // dự án
  projects: ProjectSummary[]
  candidates: string[]
  projMode: 'adopt' | 'new'
  setProjMode: (value: 'adopt' | 'new') => void
  candidate: string
  setCandidate: (value: string) => void
  projName: string
  setProjName: (value: string) => void
  projDesc: string
  setProjDesc: (value: string) => void
  projSlug: string
  slugError: string
  /** Dự án đã có trên server (onboardProject) — bước Dự án hiện trạng thái “đã tạo”. */
  project: ProjectSummary | null
  /** Đã có dự án nhưng người dùng chọn tạo thêm một dự án khác. */
  addingProject: boolean
  setAddingProject: (value: boolean) => void
  // điện thoại & thành viên
  pairing: Pairing | null
  pairingLoading: boolean
  pairedSession: Session | null
  renewPairing: () => void
  inviteUsername: string
  setInviteUsername: (value: string) => void
  inviteRole: ProjectRole
  setInviteRole: (value: ProjectRole) => void
  inviteProjects: ProjectPermissions
  setInviteProjects: (value: ProjectPermissions) => void
  invitePassword: string
  refreshPassword: () => void
  createInvite: () => Promise<void>
  invitePending: boolean
  inviteSuccess: boolean
  inviteError: string
  invited: { username: string; password: string } | null
  /** Truyện của mọi dự án — để chọn “Được viết” theo tên (chỉ quản trị). */
  inviteStories: { project: ProjectSummary; story: StorySummary }[]
  // truyện được giao (writer/reader)
  assigned: { project: ProjectSummary; story: StorySummary }[]
  assignedLoading: boolean
  // điều hướng
  go: (index: number) => void
  mustChangePassword: boolean
  /** Lỗi theo trường của ô mật khẩu hiện tại (403 “Mật khẩu hiện tại không đúng”). */
  pwCurrentError: string
  busy: boolean
  error: string
  /** Tổng số truyện đang có trong thư viện (mô tả thẻ “Dùng thư mục có sẵn”). */
  storyCount: number
  /** Mã khôi phục quản trị ở bước Hoàn tất: hiện đúng một lần. 'exists' = còn mã chưa dùng, không hiện lại được. */
  recovery: { status: 'idle' | 'loading' | 'ready' | 'exists' | 'error'; code: string; error: string }
}

export type Seg = { text: string; color: string }
export type CodeLine = { pad: number; segs: Seg[]; hot?: boolean }
export type Preview = {
  title: string
  file: string
  fileNote: string
  lines: CodeLine[]
  plain: { k: string; v: string }[]
}

// Màu code theo artboard (Onboard.dc.html › const C).
const COLORS = {
  key: '#B9D0F7',
  string: '#A6EDC1',
  number: '#F6CB8C',
  punct: '#7E7E8A',
  comment: '#5E5E68',
  plain: '#C9C9D1',
}

function makeBuilder() {
  const lines: CodeLine[] = []
  const line = (depth: number, segs: Seg[], hot?: boolean) => {
    lines.push({ pad: 12 + depth * 14, segs, hot })
  }
  const key = (depth: number, name: string, value: string | number | boolean, hot?: boolean, comma = true) => {
    const isPrimitive = typeof value === 'number' || typeof value === 'boolean'
    line(depth, [
      { text: JSON.stringify(name), color: COLORS.key },
      { text: ': ', color: COLORS.punct },
      { text: isPrimitive ? String(value) : JSON.stringify(value), color: isPrimitive ? COLORS.number : COLORS.string },
      { text: comma ? ',' : '', color: COLORS.punct },
    ], hot)
  }
  const open = (depth: number, name: string, bracket: string, hot?: boolean) => {
    line(depth, name
      ? [{ text: JSON.stringify(name), color: COLORS.key }, { text: `: ${bracket}`, color: COLORS.punct }]
      : [{ text: bracket, color: COLORS.punct }], hot)
  }
  const close = (depth: number, bracket: string) => line(depth, [{ text: bracket, color: COLORS.punct }])
  const comment = (depth: number, text: string) => line(depth, [{ text: `// ${text}`, color: COLORS.comment }])
  return { lines, line, key, open, close, comment }
}

export type PreviewInput = {
  key: StepKey
  user: User
  isAdmin: boolean
  name: string
  providerLabel: string
  providerId: string
  providerStatus: string
  model: string
  effort: string
  shared: boolean
  dirs: LibraryConfig['dirs'] | null
  autoExport: boolean
  formats: OnboardFormat[]
  projectName: string
  projectSlug: string
  adopt: boolean
  /** Dự án đã có trên server — preview không hứa tạo thêm. */
  projectExists?: boolean
  candidate: string
  /** “Nơi › … › Thư mục” — không bao giờ là tên thư mục trần hay đường dẫn tuyệt đối. */
  libraryDisplay: string
  storageMode?: LibraryResponse['storageMode']
  inviteUsername: string
  /** Nhãn vai trò của người được mời (“Người viết” / “Người đọc”), không phải vai trò của quản trị. */
  inviteRole: string
  invitePassword: string
  paired: boolean
  pwChanged: boolean
  /** Địa chỉ gốc của provider đang chọn (dòng base_url trong tệp sẽ ghi). */
  baseURL?: string
  /** Provider đăng nhập OAuth: tệp không có api_key / base_url. */
  oauthProvider?: boolean
}

/** Panel “Thiết lập sẽ lưu” — bản bằng lời thường + JSON trong “Chi tiết kỹ thuật”. */
export function buildPreview(input: PreviewInput): Preview {
  const b = makeBuilder()
  const roleLabel = input.isAdmin ? 'Quản trị' : 'Thành viên'
  let file = ''
  let fileNote = ''
  let note = ''
  const plain: { k: string; v: string }[] = []

  if (input.storageMode && input.storageMode !== 'file' && (input.key === 'library' || input.key === 'project')) {
    return {
      title: input.key === 'library' ? t.libraryStorage.title : 'Thiết lập sẽ lưu',
      file: '', fileNote: '', lines: [],
      plain: input.key === 'library'
        ? [{ k: t.libraryStorage.title, v: input.libraryDisplay }]
        : [{ k: 'Dự án', v: input.projectName || '—' }, { k: t.libraryStorage.title, v: input.libraryDisplay }],
    }
  }
  if (input.key === 'profile') {
    file = '~/.ainovel/web-accounts.json'
    fileNote = 'users[]'
    b.open(0, '', '{')
    b.key(1, 'username', input.user.username)
    b.key(1, 'name', input.name, true)
    b.key(1, 'role', input.user.role)
    b.open(1, 'projects', '{')
    for (const [index, [project, role]] of Object.entries(input.user.projects).entries()) b.key(2, project, role, index < Object.keys(input.user.projects).length - 1)
    b.close(1, '},')
    b.key(1, 'disabled', false, false)
    b.comment(1, 'passwordHash, createdAt… giữ nguyên')
    b.close(0, '}')
    plain.push({ k: 'Tên hiển thị', v: input.name || '—' }, { k: 'Vai trò', v: roleLabel })
  } else if (input.key === 'password') {
    file = 'POST /api/me/password'
    fileNote = 'API có sẵn'
    b.open(0, '', '{')
    b.key(1, 'current', '••••••••')
    b.key(1, 'new', input.pwChanged ? '••••••••••••' : '', true, false)
    b.close(0, '}')
    b.comment(0, 'giữ phiên hiện tại, đăng xuất phiên khác')
    // (không còn ghi chú dành cho dev: tên API / số GAP không hiện trên UI)
    plain.push(
      { k: 'Mật khẩu', v: input.pwChanged ? 'Mật khẩu mới của riêng bạn' : 'Chưa đổi' },
      { k: 'Thiết bị khác', v: 'Sẽ được đăng xuất để an toàn' },
    )
  } else if (input.key === 'provider' || input.key === 'model') {
    // Onboard.dc.html: một tệp config.json chung cho hai bước, dòng của bước đang đứng được tô sáng.
    file = '~/.ainovel/config.json'
    fileNote = 'toàn cục'
    const isKey = (step: StepKey) => input.key === step
    b.open(0, '', '{')
    if (input.providerId) b.key(1, 'provider', input.providerId, isKey('provider'))
    if (input.providerId) b.key(1, 'model', input.model, isKey('model'))
    if (input.providerId) b.key(1, 'reasoning_effort', input.effort, isKey('model'))
    // QĐ 6: không còn bước chọn ngôn ngữ — app chỉ viết tiếng Việt nên dòng này luôn là "vi".
    b.key(1, 'language', 'vi')
    b.key(1, 'style', 'default', false, Boolean(input.providerId))
    if (input.providerId) {
      b.open(1, 'providers', '{')
      b.open(2, input.providerId, '{', isKey('provider'))
      if (!input.oauthProvider) {
        b.key(3, 'api_key', 'sk-…', isKey('provider'))
        if (input.baseURL) b.key(3, 'base_url', input.baseURL, isKey('provider'))
      }
      b.open(3, 'models', '[')
      b.line(4, [
        { text: '{ ', color: COLORS.punct }, { text: '"name"', color: COLORS.key }, { text: ': ', color: COLORS.punct },
        { text: JSON.stringify(input.model), color: COLORS.string }, { text: ' }', color: COLORS.punct },
      ], isKey('model'))
      b.close(3, ']')
      b.close(2, '}')
      b.close(1, '},')
      if (input.shared) b.line(1, [{ text: '"roles"', color: COLORS.key }, { text: ': {}', color: COLORS.punct }])
      else {
        b.open(1, 'roles', '{')
        b.line(2, [{ text: '"architect"', color: COLORS.key }, { text: ': { "reasoning_effort": "high" },', color: COLORS.punct }], isKey('model'))
        b.line(2, [{ text: '"editor"', color: COLORS.key }, { text: ': { "reasoning_effort": "high" }', color: COLORS.punct }], isKey('model'))
        b.close(1, '}')
      }
    }
    b.close(0, '}')
    if (input.key === 'provider') plain.push({ k: 'Nhà cung cấp AI', v: input.providerLabel }, { k: 'Trạng thái', v: input.providerStatus })
    if (input.key === 'model') {
      plain.push(
        { k: 'Model chính', v: input.model || '—' },
        { k: 'Mức suy luận', v: effortLabel(input.effort) },
        { k: 'Các trợ lý', v: input.shared ? 'Dùng chung model chính' : 'Kiến trúc sư và Biên tập viên đặt riêng' },
      )
    }
  } else if (input.key === 'library') {
    file = '.ainovel/library.json'
    fileNote = 'thư viện'
    const dirs = input.dirs ?? { ideas: '', assets: '', stories: '', exports: '' }
    b.open(0, '', '{')
    b.open(1, 'dirs', '{')
    b.key(2, 'ideas', dirs.ideas, true)
    b.key(2, 'assets', dirs.assets, true)
    b.key(2, 'stories', dirs.stories, true)
    b.key(2, 'exports', dirs.exports, true, false)
    b.close(1, '},')
    b.open(1, 'autoExport', '{')
    b.key(2, 'onVolume', input.autoExport, true)
    b.line(2, [
      { text: '"formats": [', color: COLORS.punct },
      { text: input.formats.map((format) => JSON.stringify(format)).join(', '), color: COLORS.string },
      { text: '],', color: COLORS.punct },
    ])
    b.key(2, 'pattern', '', false, false)
    b.close(1, '}')
    b.close(0, '}')
    plain.push(
      { k: 'Lưu ở', v: input.libraryDisplay || '—' },
      { k: 'Mỗi dự án có', v: 'Ý tưởng · Tư liệu · Truyện · Bản xuất' },
      { k: 'Tự xuất bản khi xong một tập', v: input.autoExport ? input.formats.map((format) => FORMAT_LABEL[format]).join(' + ') || 'Bật' : 'Tắt' },
    )
  } else if (input.key === 'project') {
    const folder = input.adopt ? input.candidate : input.projectSlug
    file = `${folder || 'ten-thu-muc'}/project.json`
    fileNote = input.projectExists ? 'đã có' : input.adopt ? 'thêm mới' : 'tạo mới'
    b.open(0, '', '{')
    b.key(1, 'name', input.projectName, true)
    b.key(1, 'notes', '', false, false)
    b.close(0, '}')
    b.comment(0, input.projectExists ? 'đã lưu trên máy chủ, không tạo lại' : input.adopt ? `giữ nguyên nội dung trong ${input.candidate}` : 'tạo .ainovel/ và các thư mục con')
    plain.push(
      { k: 'Dự án', v: input.projectName || '—' },
      { k: 'Thư mục', v: folder || '—' },
      { k: 'Cách tạo', v: input.projectExists ? 'Đã tạo — tiếp tục không tạo thêm' : input.adopt ? 'Dùng thư mục có sẵn, giữ nguyên nội dung' : 'Tạo thư mục mới' },
    )
  } else if (input.key === 'mobile') {
    file = 'POST /api/me/pairings'
    fileNote = 'API có sẵn'
    b.open(0, '', '{')
    b.key(1, 'code', '••••••')
    b.key(1, 'username', input.user.username)
    b.open(1, 'links', '[')
    b.line(2, [{ text: input.paired ? '{ "label": "nov.example.com", "url", "svg" }' : '[]', color: COLORS.string }])
    b.close(1, ']')
    b.close(0, '}')
    if (input.inviteUsername) {
      b.comment(0, 'POST /api/users')
      b.open(0, '', '{')
      b.key(1, 'username', input.inviteUsername, true)
      b.key(1, 'role', 'member', true)
      b.key(1, 'password', '••••••••', false, false)
      b.close(0, '}')
    }
    plain.push(
      { k: 'Điện thoại', v: input.paired ? 'Đã đăng nhập' : 'Mã QR dùng một lần, hết hạn sau 5 phút' },
      ...(input.inviteUsername ? [{ k: 'Người cùng viết', v: `${input.inviteUsername} · ${input.inviteRole}` }] : []),
    )
  } else {
    file = 'Đã ghi'
    fileNote = input.isAdmin ? '4 tệp' : '2 thay đổi'
    const written = input.isAdmin
      ? [
        ['~/.ainovel/web-accounts.json', 'tên hiển thị'],
        ['~/.ainovel/config.json', 'provider · model'],
        ['.ainovel/library.json', 'thư mục · xuất bản'],
        [`${input.projectSlug || 'du-an'}/project.json`, 'dự án'],
      ]
      : [['web-accounts.json', 'tên hiển thị'], ['/api/me/password', 'mật khẩu mới']]
    written.forEach(([path, what]) => b.line(0, [
      { text: '✓ ', color: 'var(--color-ok)' },
      { text: path, color: 'var(--color-fg-2)' },
      { text: `  ${what}`, color: COLORS.comment },
    ]))
    plain.push(...(input.isAdmin
      ? [
        { k: 'Tài khoản', v: input.name },
        { k: 'Nhà cung cấp & model', v: `${input.providerLabel} · ${input.model || '—'}` },
        { k: 'Nơi lưu & dự án', v: `${input.libraryDisplay || '—'} · ${input.projectName || 'chưa tạo'}` },
      ]
      : [
        { k: 'Tài khoản', v: input.name },
        { k: 'Mật khẩu', v: input.pwChanged ? 'Đã đổi' : 'Vẫn là mật khẩu tạm' },
      ]))
  }

  return {
    title: input.key === 'done' ? 'Đã lưu' : 'Thiết lập sẽ lưu',
    file, fileNote, lines: b.lines, plain,
  }
}
