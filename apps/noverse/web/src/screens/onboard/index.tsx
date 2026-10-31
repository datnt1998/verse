// A3 · Thiết lập lần đầu (#/welcome/:step). Mỗi bước ghi ngay khi bấm Tiếp tục;
// admin 7 bước, writer/reader 4 bước (09-auth-onboard.md §5). Ghi cuối: POST /api/me/onboarded.
import { createElement, useEffect, useRef, useState, type KeyboardEvent, type ReactNode } from 'react'
import {
  ApiError, ConflictError, adoptProject, canReadStory, cancelOAuth, changePassword, completeOAuth, createProject, createUser,
  getConfig, getConfigModels, getConfigPresets, getLibrary, getOAuthStatus, getProject, issueRecovery, logoutOAuth,
  markOnboarded, myPairing, mySessions, putConfig, putLibrary, setLanAccess, startOAuth, testConfig, updateMe,
} from '../../api/client'
import { PASTE_CODE_EMPTY_ERROR, parseAuthCode } from '../../lib/authCode'
import { useSession } from '../../api/session'
import type {
  Config, ConfigAccount, ConfigModelInfo, ConfigResponse, LibraryResponse, Pairing, ProviderConfig,
  ProviderPreset, ProjectPermissions, ProjectRole, Session,
} from '../../api/types'
import { navigate } from '../../router'
import { useMediaQuery } from '../../shell/AppShell'
import { BrandLockup } from '@noverse/ui'
import { Avatar, Kbd } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { StepBody } from './OnboardSteps'
import { WritePreview } from './WritePreview'
import {
  USERNAME_RE, OPTIONAL_STEPS, PROVIDER_PRESENTATION, buildPreview, canAdvance, clearFromSetup, configFileOf, configuredProviderId, draftProvider, fromSetup, humanizeSlug,
  makePassword, libraryStoryCount, onboardProject, passwordStepCopy, projectDefaults, projectSlugError, providerConnection, providerStatusLabel, stepLabel, stepsFor, takeSetupRecovery,
  writableConfig, type OAuthUi, type OnboardFormat, type OnboardModel, type PreviewInput, type RoleModelDraft, type StepKey,
} from './model'
import { slugifyOrEmpty } from '../../lib/slug'
import { keys, useAction, useConnection, useResource } from '../../data'
import { Button, toast } from '@noverse/ui'
import { useLibraryChoice } from './libraryChoice'
import { ConflictBar } from '@noverse/ui'
import { StaleBanner } from '@noverse/ui'
import { useSaveShortcut } from '@noverse/ui'
import { PanelRefreshBar } from '../../ui/PanelRefreshBar'
import { useUnsavedGuard } from '../../ui/useUnsavedGuard'

export function OnboardScreen({ step }: { step: string | null }) {
  const { user, session, refresh, endSession } = useSession()
  const libraryResource = useResource(keys.library(), getLibrary)
  const library = libraryResource.data ?? null
  const libraryLoading = !library && libraryResource.initialLoading
  const live = useConnection()
  const configResource = useResource(keys.config('global'), () => getConfig('global'), { enabled: user?.role === 'admin' })
  const presetsResource = useResource(['config', 'presets'], () => getConfigPresets(), { enabled: user?.role === 'admin' })
  const [configVersion, setConfigVersion] = useState<string | undefined>()
  const profileBaseline = useRef(user)
  const initialConfig = useRef(false)
  const libraryBaseline = useRef<LibraryResponse | null>(null)
  const narrow = useMediaQuery('(max-width: 1100px)')

  const isAdmin = user?.role === 'admin'
  const steps = user ? stepsFor(user.role) : stepsFor('admin')
  const requested = steps.includes(step as StepKey) ? step as StepKey : steps[0]
  const stepKey = requested
  const idx = steps.indexOf(stepKey)
  const libChoice = useLibraryChoice({ library, enabled: isAdmin && stepKey === 'library' })

  const [name, setName] = useState(user?.name ?? '')
  // Rail “Lần chạy đầu ✓✓” chỉ hiện khi vừa tạo tài khoản quản trị ở A0 (#/setup).
  const [firstRun] = useState(() => fromSetup())
  // Mã khôi phục của quản trị: cấp một lần khi tới bước Hoàn tất (POST /api/me/recovery), không lưu ở đâu khác.
  const [recovery, setRecovery] = useState<OnboardModel['recovery']>({ status: 'idle', code: '', error: '' })
  const recoveryRequested = useRef(false)
  useEffect(() => {
    if (stepKey !== 'done' || !isAdmin || recoveryRequested.current) return
    recoveryRequested.current = true
    // A0 vừa tạo quản trị: mã khôi phục đã cấp kèm lúc tạo, không gọi lại API (sẽ 409 vì còn mã chưa dùng).
    const fromSetupCode = takeSetupRecovery()
    if (fromSetupCode) { setRecovery({ status: 'ready', code: fromSetupCode, error: '' }); return }
    setRecovery({ status: 'loading', code: '', error: '' })
    issueRecovery().then(
      (issued) => setRecovery({ status: 'ready', code: issued.code, error: '' }),
      (cause) => setRecovery(cause instanceof ConflictError || cause instanceof ApiError && cause.status === 409
        ? { status: 'exists', code: '', error: '' }
        : { status: 'error', code: '', error: cause instanceof Error ? cause.message : String(cause) }),
    )
  }, [stepKey, isAdmin])
  const [config, setConfig] = useState<Config | null>(null)
  // G7 — tài khoản OAuth đã kết nối do GET /api/config trả về (persist), không phải hỏi lại từng provider.
  const [accounts, setAccounts] = useState<Record<string, ConfigAccount>>({})
  const [presets, setPresets] = useState<ProviderPreset[]>([])
  const [providerId, setProviderId] = useState('')
  const [baseURL, setBaseURL] = useState('')
  const [apiKey, setApiKey] = useState('')
  const [customName, setCustomName] = useState('')
  const [customType, setCustomType] = useState('openai')
  const [changing, setChanging] = useState(false)
  const [testState, setTestState] = useState<'idle' | 'run' | 'ok' | 'fail'>('idle')
  const [testLatency, setTestLatency] = useState(0)
  const [testMessage, setTestMessage] = useState('')
  // G6 — model máy chủ thực sự dùng khi thử kết nối (preset defaultModel nếu chưa chọn model).
  const [testModel, setTestModel] = useState('')
  const [oauth, setOauth] = useState<OAuthUi>({ phase: 'idle' })
  const [oauthCode, setOauthCode] = useState('')
  const [oauthCodeError, setOauthCodeError] = useState('')
  const [models, setModels] = useState<ConfigModelInfo[]>([])
  const [model, setModel] = useState('')
  const [effort, setEffort] = useState('')
  const [shared, setShared] = useState(true)
  const [roleModels, setRoleModels] = useState<RoleModelDraft>({
    architect: { model: '', effort: 'high' },
    writer: { model: '', effort: 'medium' },
    editor: { model: '', effort: 'high' },
  })
  const dirs = library?.config.dirs ?? null
  const [autoExport, setAutoExport] = useState(true)
  const [formats, setFormats] = useState<OnboardFormat[]>(['txt', 'epub'])
  const [projMode, setProjMode] = useState<'adopt' | 'new'>('adopt')
  const [candidate, setCandidate] = useState('')
  const [projName, setProjName] = useState('')
  const [projDesc, setProjDesc] = useState('')
  const [createdProject, setCreatedProject] = useState<string | null>(null)
  const [addingProject, setAddingProject] = useState(false)
  const [pwCurrent, setPwCurrent] = useState('')
  const [pwNext, setPwNext] = useState('')
  const [pwRepeat, setPwRepeat] = useState('')
  const [dismissedConflict, setDismissedConflict] = useState<unknown>(null)
  const [pwChanged, setPwChanged] = useState(false)
  const [pairing, setPairing] = useState<Pairing | null>(null)
  const sessionsResource = useResource(keys.sessions(), mySessions, { enabled: stepKey === 'mobile' })
  const [pairedSession, setPairedSession] = useState<Session | null>(null)
  const [inviteUsername, setInviteUsername] = useState('')
  const [inviteRole, setInviteRole] = useState<ProjectRole>('collab')
  const [inviteProjects, setInviteProjects] = useState<ProjectPermissions>({})
  const [inviteTouched, setInviteTouched] = useState(false)
  const [invitePassword, setInvitePassword] = useState(makePassword)
  const [inviteError, setInviteError] = useState('')
  const [invited, setInvited] = useState<{ username: string; password: string } | null>(null)
  const assignedResource = useResource([...keys.library(), 'assigned', user?.username ?? ''], async () => {
    const details = await Promise.all((library?.projects ?? []).map((project) => getProject(project.id)))
    return details.flatMap((detail) => detail.stories.filter((story) => user && canReadStory(user, `${detail.project.id}/${story.story}`)).map((story) => ({ project: detail.project, story })))
  }, { enabled: !!user && user.role !== 'admin' && !!library })
  const inviteStoriesResource = useResource([...keys.library(), 'invite-stories'], async () => {
    const details = await Promise.all((library?.projects ?? []).map((project) => getProject(project.id)))
    return details.flatMap((detail) => detail.stories.map((story) => ({ project: detail.project, story })))
  }, { enabled: !!user && user.role === 'admin' && !!library && stepKey === 'mobile' })
  const assigned = assignedResource.data ?? []
  const [error, setError] = useState('')
  const baseline = useRef<Set<string>>(new Set())
  const projectInit = useRef(false)

  const projects = library?.projects ?? []
  const candidates = library?.candidates ?? []
  // E2EAUTH-5: trạng thái bước Dự án lấy từ server (tải lại trang vẫn thấy dự án đã tạo).
  const serverProject = onboardProject(projects, createdProject)
  const projectReady = Boolean(serverProject) && !addingProject
  const preset = presets.find((entry) => entry.id === providerId) ?? null
  const providerLabel = PROVIDER_PRESENTATION[providerId]?.label || preset?.label || providerId || 'Chưa có nhà cung cấp'
  const providerDetail = preset
    ? [preset.api || preset.type, baseURL || preset.baseURL].filter(Boolean).join(' · ')
    : config?.providers?.[providerId]?.base_url ?? ''
  const configuredProvider = configuredProviderId(config)
  const accountInfo = accounts[providerId]?.connected ? accounts[providerId] : null
  const connection = providerConnection({
    configuredProvider,
    providerId,
    changing,
    testOk: testState === 'ok',
    testFailed: testState === 'fail',
    oauthDone: oauth.phase === 'done',
    accountConnected: Boolean(accountInfo?.connected),
  })
  const providerStatusText = providerStatusLabel(connection)
  const projSlug = slugifyOrEmpty(projName)
  const slugError = projMode === 'new' ? projectSlugError(projName, projects, candidates) : ''

  useEffect(() => {
    if (!configResource.data || initialConfig.current) return
    initialConfig.current = true
    const response = configResource.data
    const fileConfig = configFileOf(response)
    setConfig(fileConfig); setConfigVersion(response.version)
    setAccounts(response.accounts ?? {})
    setProviderId(fileConfig.provider ?? '')
    setModel(fileConfig.model ?? '')
    setEffort(fileConfig.reasoning_effort ?? '')
    setChanging(!configuredProviderId(fileConfig))
    setRoleModels({
      architect: { model: fileConfig.roles?.architect?.model ?? '', effort: fileConfig.roles?.architect?.reasoning_effort ?? 'high' },
      writer: { model: fileConfig.roles?.writer?.model ?? '', effort: fileConfig.roles?.writer?.reasoning_effort ?? 'medium' },
      editor: { model: fileConfig.roles?.editor?.model ?? '', effort: fileConfig.roles?.editor?.reasoning_effort ?? 'high' },
    })
  }, [configResource.data])
  useEffect(() => { if (presetsResource.data) setPresets(presetsResource.data.providers) }, [presetsResource.data])
  const modelsResource = useResource(['config', 'models', providerId], () => getConfigModels(providerId), { enabled: !!providerId && providerId !== 'custom' })
  useEffect(() => {
    if (!modelsResource.data) return
    setModels(modelsResource.data.models)
    setModel((current) => current || modelsResource.data?.models[0]?.name || preset?.defaultModel || '')
  }, [modelsResource.data, preset?.defaultModel])

  const oauthId = oauth.phase === 'browser' || oauth.phase === 'device' || oauth.phase === 'paste' ? oauth.id : null
  const oauthResource = useResource(['config', 'global', 'oauth', oauthId], () => getOAuthStatus(oauthId!), { enabled: !!oauthId, staleTime: 0 })
  useEffect(() => {
    const status = oauthResource.data
    if (!status) return
    if (status.state === 'done' && status.account) {
      const account = status.account
      setOauth({ phase: 'done', account, models: status.models, via: status.mode })
      setAccounts((prev) => ({ ...prev, [providerId]: { connected: true, label: account.label, email: account.email, plan: account.plan, expiresAt: null } }))
      if (status.models.length) { setModels(status.models); setModel((current) => current || status.models[0].name) }
    } else if (status.state === 'failed') setOauth({ phase: 'failed', message: status.error || 'Nhà cung cấp từ chối đăng nhập' })
  }, [oauthResource.data, providerId])
  useEffect(() => {
    if (!oauthId) return
    const deadline = Date.now() + 10 * 60_000
    const timer = setInterval(() => {
      if (Date.now() >= deadline) {
        setOauth({ phase: 'failed', message: 'Phiên đăng nhập đã hết hạn. Vui lòng thử lại.' })
        return
      }
      void oauthResource.refresh()
    }, 2000)
    return () => clearInterval(timer)
  }, [oauthId, oauthResource.refresh])

  useEffect(() => {
    if (!library || libraryBaseline.current) return
    libraryBaseline.current = library
    setAutoExport(library.config.autoExport.onVolume)
    setFormats(library.config.autoExport.formats)
  }, [library])

  useEffect(() => {
    if (projectInit.current) return
    const defaults = projectDefaults(library?.candidates)
    if (!defaults) return
    projectInit.current = true
    setProjMode(defaults.mode)
    setCandidate(defaults.candidate)
    if (defaults.mode === 'adopt' && defaults.candidate) {
      setProjName((current) => current || humanizeSlug(defaults.candidate))
    }
  }, [library])

  useEffect(() => {
    if (inviteTouched || projects.length === 0) return
    setInviteProjects({ [projects[0].id]: 'collab' })
  }, [projects, inviteTouched])

  const pairingAction = useAction(async () => {
    baseline.current = new Set((sessionsResource.data?.sessions ?? []).map((session) => session.id))
    const created = await myPairing()
    setPairing(created)
    return created
  }, { error: false })
  const pairingLoading = pairingAction.busy || !sessionsResource.data
  const generatedPairing = useRef(false)
  useEffect(() => {
    if (stepKey !== 'mobile' || !sessionsResource.data || generatedPairing.current) return
    generatedPairing.current = true
    void pairingAction.run()
  }, [stepKey, sessionsResource.data, pairingAction.run])
  useEffect(() => {
    if (!pairing) return
    const fresh = sessionsResource.data?.sessions.find((session) => !baseline.current.has(session.id) && !session.current)
    if (fresh) setPairedSession(fresh)
  }, [sessionsResource.data, pairing])

  const go = (index: number) => {
    const target = steps[Math.max(0, Math.min(steps.length - 1, index))]
    navigate(`/welcome/${target}`, index < idx)
  }

  const testAction = useAction(async () => {
    setTestMessage('')
    const targetId = providerId === 'custom' ? customName.trim() : providerId
    if (!targetId) throw new Error('Nhập tên provider')
    const existing = config?.providers?.[targetId]
    const draft = targetId && (apiKey || changing && !preset?.oauth)
      ? { provider: targetId, model: model || undefined, providers: { [targetId]: draftProvider(existing, preset?.id === 'custom' ? { ...preset, type: customType } : preset, baseURL, apiKey) } }
      : undefined
    const result = await testConfig(targetId, model || undefined, draft)
    setTestLatency(result.latencyMs); setTestModel(result.model)
    setTestState(result.ok ? 'ok' : 'fail')
    setTestMessage(result.ok ? '' : result.message || 'Không kết nối được')
    if (!result.ok) throw new Error(result.message || 'Không kết nối được')
    return true
  }, { error: false })
  useEffect(() => {
    if (testAction.pending) setTestState('run')
    else if (testAction.error) { setTestState('fail'); setTestMessage(testAction.error instanceof Error ? testAction.error.message : 'Không kết nối được') }
  }, [testAction.pending, testAction.error])

  const chooseProvider = (id: string) => {
    const next = presets.find((entry) => entry.id === id)
    setProviderId(id)
    setBaseURL(next?.baseURL ?? '')
    setApiKey('')
    setModel('')
    setModels([])
    setTestState('idle')
    setTestMessage('')
    setTestModel('')
    setOauth({ phase: 'idle' })
  }

  const oauthStartAction = useAction(async (device: boolean) => {
    if (!preset) throw new Error('Chọn nhà cung cấp AI')
    const started = await startOAuth(preset.id, device)
    if (started.mode === 'browser' && started.url) window.open(started.url, '_blank', 'noopener')
    setOauthCode(''); setOauthCodeError('')
    if (started.mode === 'paste') setOauth({ phase: 'paste', id: started.id, url: started.url, expiresAt: started.expiresAt })
    else setOauth({ phase: started.mode === 'device' ? 'device' : 'browser', id: started.id, url: started.url, userCode: started.userCode, verifyURL: started.verifyURL, expiresAt: started.expiresAt })
    return started
  }, { error: false })
  useEffect(() => {
    if (oauthStartAction.error) setOauth({ phase: 'failed', message: oauthStartAction.error instanceof Error ? oauthStartAction.error.message : 'Không bắt đầu được đăng nhập OAuth' })
  }, [oauthStartAction.error])
  const oauthCancelAction = useAction(async () => {
    if (oauth.phase === 'browser' || oauth.phase === 'device' || oauth.phase === 'paste') await cancelOAuth(oauth.id)
    setOauthCode(''); setOauthCodeError('')
    setOauth({ phase: 'idle' })
    return true
  }, { error: false })
  const oauthCodeAction = useAction(async () => {
    if (oauth.phase !== 'paste') return false
    if (!parseAuthCode(oauthCode)) { setOauthCodeError(PASTE_CODE_EMPTY_ERROR); return false }
    try {
      await completeOAuth(oauth.id, oauthCode)
      setOauthCode('')
      setOauthCodeError('')
      void oauthResource.refresh()
      return true
    } catch (error) {
      setOauthCodeError(error instanceof Error ? error.message : 'Mã không dùng được')
      return false
    }
  }, { error: false })
  const disconnectAction = useAction(async () => {
    if (!preset) throw new Error('Chọn nhà cung cấp AI')
    await logoutOAuth(preset.id)
    setAccounts((prev) => { const next = { ...prev }; delete next[preset.id]; return next })
    setOauth({ phase: 'idle' })
    return true
  }, { success: 'Đã ngắt kết nối', error: false, invalidate: [keys.config('global')] })

  const saveProvider = async (version = configVersion) => {
    if (!preset) throw new Error('Chọn nhà cung cấp AI')
    const targetId = preset.id === 'custom' ? customName.trim() : preset.id
    if (!/^[a-z0-9][a-z0-9_-]*$/.test(targetId)) throw new Error('Tên provider chỉ dùng chữ thường, số, “-”, “_”')
    const existing = config?.providers?.[targetId]
    if (preset.keyRequired && !preset.oauth && !apiKey && !existing?.api_key_set) throw new Error('Nhập API Key cho nhà cung cấp này')
    const providerConfig = draftProvider(existing, preset.id === 'custom' ? { ...preset, type: customType } : preset, baseURL, apiKey)
    const response = await putConfig('global', {
      ...writableConfig(config),
      provider: targetId,
      model: model || existing?.models?.[0]?.name || models[0]?.name || undefined,
      providers: { ...(config?.providers ?? {}), [targetId]: providerConfig },
    }, undefined, undefined, version)
    setConfig(configFileOf(response)); setConfigVersion(response.version)
    setProviderId(targetId)
    setChanging(false)
    setApiKey('')
  }

  const saveModel = async (version = configVersion) => {
    if (!providerId) throw new Error('Chọn nhà cung cấp AI')
    if (!model.trim()) throw new Error('Chọn model chính')
    const next: Config = { ...writableConfig(config), provider: providerId, model: model.trim(), reasoning_effort: effort || undefined }
    next.roles = shared ? {} : {
      architect: {
        provider: providerId, model: roleModels.architect.model || model.trim(),
        reasoning_effort: roleModels.architect.effort || undefined,
      },
      writer: {
        provider: providerId, model: roleModels.writer.model || model.trim(),
        reasoning_effort: roleModels.writer.effort || undefined,
      },
      editor: {
        provider: providerId, model: roleModels.editor.model || model.trim(),
        reasoning_effort: roleModels.editor.effort || undefined,
      },
    }
    const response = await putConfig('global', next, undefined, undefined, version)
    setConfig(configFileOf(response)); setConfigVersion(response.version)
  }

  const saveLibrary = async (version = libraryBaseline.current?.version) => {
    if (!library) throw new Error('Chưa đọc được cấu hình thư viện')
    if (library.storageMode === 'postgres' || library.storageMode === 'container') return
    const result = await putLibrary({
      ...(libraryBaseline.current?.config ?? library.config),
      autoExport: { ...(libraryBaseline.current?.config.autoExport ?? library.config.autoExport), onVolume: autoExport, formats },
    }, version)
    libraryBaseline.current = { ...(libraryBaseline.current ?? library), config: result.config, version: result.version }
  }

  const saveProject = async () => {
    if (projectReady) return
    if (projMode === 'adopt') {
      if (!candidate) throw new Error('Chọn thư mục để nhận')
      const project = await adoptProject(candidate, projName.trim() || undefined)
      setCreatedProject(project.id)
    } else {
      if (!projName.trim()) throw new Error('Đặt tên cho dự án')
      if (!projSlug) throw new Error('Tên cần có ít nhất một chữ cái hoặc số.')
      if (slugError) throw new Error(slugError)
      const project = await createProject({
        slug: projSlug, name: projName.trim(), description: projDesc.trim(), notes: '',
      })
      setCreatedProject(project.id)
    }
    setAddingProject(false)
  }

  const inviteAction = useAction(async () => {
    setInviteError('')
    if (!USERNAME_RE.test(inviteUsername)) throw new Error('Tên đăng nhập: chữ thường không dấu, số và . _ - · 2–32 ký tự')
    if (invitePassword.length < 8) throw new Error('Mật khẩu tạm cần ít nhất 8 ký tự')
    const created = await createUser(inviteUsername, inviteUsername, 'member', Object.fromEntries(Object.keys(inviteProjects).map((id) => [id, inviteRole])), invitePassword)
    setInvited({ username: created.username, password: invitePassword })
    return created
  }, { success: 'Đã tạo tài khoản', error: false, invalidate: [keys.users(), keys.library()] })
  useEffect(() => { if (inviteAction.error instanceof Error) setInviteError(inviteAction.error.message) }, [inviteAction.error])
  const lanAction = useAction(async () => {
    await setLanAccess(true)
    await pairingAction.run()
  }, { error: false })

  const primaryProject = serverProject ?? projects.find((project) => !project.legacy) ?? projects[0] ?? null
  const primaryTarget = isAdmin
    ? (primaryProject ? `/p/${primaryProject.id}/new` : '/')
    : (assigned[0] ? `/p/${assigned[0].project.id}/s/${assigned[0].story.story}/work` : '/')
  const workspaceTarget = isAdmin && primaryProject
    ? `/p/${primaryProject.id}`
    : primaryTarget

  const finishAction = useAction(async (target: string) => {
    await markOnboarded()
    clearFromSetup()
    await refresh()
    navigate(target, true)
    return true
  }, { error: false, invalidate: [keys.me()] })
  const advanceAction = useAction(async (version?: string) => {
    if (stepKey === 'profile') {
      if (!name.trim()) throw new Error('Tên hiển thị không được để trống')
      const result = await updateMe(name.trim(), version ?? profileBaseline.current?.version)
      profileBaseline.current = result.user
      await refresh()
    } else if (stepKey === 'password') {
      if (!pwCurrent) throw new Error(passwordStepCopy(user?.mustChangePassword === true && !pwChanged).missingCurrent)
      if (pwNext.length < 8) throw new Error('Mật khẩu mới cần ít nhất 8 ký tự')
      if (pwNext !== pwRepeat) throw new Error('Hai lần nhập mật khẩu chưa khớp')
      const result = await changePassword(pwCurrent, pwNext, version ?? profileBaseline.current?.version)
      // Đổi xong: máy chủ thu hồi mọi phiên kể cả phiên đang chạy onboard này —
      // báo đổi thành công rồi đưa về đăng nhập; không refresh /api/me (sẽ 401).
      if (result.reauthenticate) {
        toast('Đã đổi mật khẩu. Vui lòng đăng nhập lại.')
        if (user?.grants != null) endSession('password'); else endSession()
        return true
      }
      await refresh()
      setPwChanged(true); setPwCurrent(''); setPwNext(''); setPwRepeat('')
    } else if (stepKey === 'provider') {
      if (changing) await saveProvider(version)
      else if (!await testAction.run()) throw new Error('Chưa kết nối được nhà cung cấp — xem chi tiết bên dưới')
    } else if (stepKey === 'model') await saveModel(version)
    else if (stepKey === 'library') {
      // Đổi nơi lưu trước (tạo thư mục gợi ý nếu cần): thư viện mới có library.json/phiên bản riêng nên làm mốc lại.
      const switched = await libChoice.commit()
      if (switched) libraryBaseline.current = switched
      await saveLibrary(switched ? switched.version : version)
    }
    else if (stepKey === 'project') await saveProject()
    go(idx + 1)
    return true
  }, { error: false, invalidate: [keys.me(), keys.config('global'), keys.library()] })
  const busy = advanceAction.busy || finishAction.busy || inviteAction.busy || oauthStartAction.busy || oauthCancelAction.busy || oauthCodeAction.busy || disconnectAction.busy || testAction.busy || !live.writable
  const actionError = advanceAction.error || finishAction.error || (stepKey === 'provider' ? disconnectAction.error || oauthCancelAction.error : null) || (stepKey === 'mobile' ? pairingAction.error : null)
  const conflict = actionError instanceof ConflictError && actionError !== dismissedConflict ? actionError : null
  // Một bề mặt lỗi cho mỗi thao tác (16 §3): sai mật khẩu hiện tại là lỗi theo trường → dưới ô, không lặp ở đầu trang.
  const pwCurrentError = stepKey === 'password' && actionError instanceof ApiError && actionError.status === 403 ? actionError.message : ''
  const advance = (skip = false) => {
    if (busy) return
    setError('')
    if (stepKey === 'done') { void finishAction.run(primaryTarget); return }
    if (skip) { go(idx + 1); return }
    void advanceAction.run()
  }
  const dirty = stepKey === 'profile' ? name !== profileBaseline.current?.name
    : stepKey === 'password' ? Boolean(pwCurrent || pwNext || pwRepeat)
    : stepKey === 'provider' ? Boolean(apiKey || changing && providerId !== config?.provider)
    : stepKey === 'model' ? Boolean(config && (model !== config.model || effort !== (config.reasoning_effort ?? '') || !shared && JSON.stringify(roleModels) !== JSON.stringify(config.roles)))
    : stepKey === 'library' ? Boolean(libraryBaseline.current && (autoExport !== libraryBaseline.current.config.autoExport.onVolume || formats.join() !== libraryBaseline.current.config.autoExport.formats.join()))
    : stepKey === 'project' ? projectReady ? false : projMode === 'new' ? Boolean(projName || projDesc) : candidate !== (library?.candidates?.[0] ?? '')
    : false
  useUnsavedGuard(dirty && !advanceAction.busy && !finishAction.busy)
  useSaveShortcut(() => advance(), !busy && stepKey !== 'mobile')

  // E2EAUTH-6: mỗi lần vào bước (Tiếp tục, Quay lại, bấm rail, tải lại trang) focus ô nhập đầu tiên
  // khi ô đó đã hiện (bước chờ dữ liệu thì focus ngay khi dữ liệu về).
  const bodyRef = useRef<HTMLDivElement>(null)
  const focusStep = useRef<{ step: StepKey | null; pending: boolean }>({ step: null, pending: false })
  useEffect(() => {
    const state = focusStep.current
    if (state.step !== stepKey) { state.step = stepKey; state.pending = true }
    if (!state.pending) return
    const body = bodyRef.current
    if (!body || body.contains(document.activeElement) && document.activeElement !== body) { if (body) state.pending = false; return }
    const field = body.querySelector<HTMLElement>('input:not([readonly]):not([disabled]):not([type="hidden"]), textarea:not([readonly]):not([disabled])')
    if (!field) return
    state.pending = false
    field.focus()
  })
  // Lỗi theo trường: focus vào trường lỗi (16 §3).
  useEffect(() => { if (pwCurrentError) document.getElementById('ob-pw-cur')?.focus() }, [pwCurrentError])

  const onKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.key !== 'Enter' || event.shiftKey || busy || stepKey === 'done') return
    const tag = (event.target as HTMLElement).tagName
    if (tag === 'TEXTAREA' || tag === 'BUTTON' || tag === 'SELECT' || tag === 'A') return
    event.preventDefault()
    void advance(false)
  }
  if (!user) return null

  const modelValue: OnboardModel = {
    user, isAdmin, steps, stepKey, idx, sessionExpiresAt: session?.expiresAt ?? null, pwChanged,
    name, setName,
    pwCurrent, pwNext, pwRepeat, setPwCurrent, setPwNext, setPwRepeat,
    config, presets, providerId, setProviderId: chooseProvider,
    providerLabel, providerDetail, configuredProvider, providerStatus: connection, preset, accountInfo,
    changing, setChanging: (value) => {
      setChanging(value)
      if (value) {
        setBaseURL(preset?.baseURL ?? config?.providers?.[providerId]?.base_url ?? '')
        setTestState('idle'); setTestMessage(''); setTestModel(''); setOauth({ phase: 'idle' })
      } else {
        const back = configuredProvider
        const backPreset = presets.find((entry) => entry.id === back) ?? null
        setProviderId(back)
        setBaseURL(backPreset?.baseURL ?? config?.providers?.[back]?.base_url ?? '')
        setApiKey('')
        setTestState('idle'); setTestMessage(''); setTestModel(''); setOauth({ phase: 'idle' })
      }
    },
    apiKey, setApiKey, baseURL, setBaseURL, customName, setCustomName, customType, setCustomType,
    testState, testLatency, testMessage, testModel, runTest: async () => Boolean(await testAction.run()),
    oauth, oauthPending: oauthStartAction.pending, oauthBusy: oauthStartAction.busy, oauthMutationPending: oauthCancelAction.pending || disconnectAction.pending, startOauth: (device) => { void oauthStartAction.run(device) }, cancelOauth: () => { void oauthCancelAction.run() }, disconnectProvider: () => { void disconnectAction.run() },
    oauthCode, setOauthCode: (value) => { setOauthCode(value); setOauthCodeError('') }, oauthCodeError, oauthCodePending: oauthCodeAction.pending, submitOauthCode: () => { void oauthCodeAction.run() },
    models, model, setModel, effort, setEffort, shared, setShared, roleModels,
    setRoleModel: (role, patch) => setRoleModels((current) => ({ ...current, [role]: { ...current[role], ...patch } })),
    library, libraryLoading, libChoice,
    dirs,
    autoExport, setAutoExport,
    formats,
    toggleFormat: (format) => setFormats((current) => current.includes(format) ? current.filter((entry) => entry !== format) : [...current, format]),
    projects, candidates, projMode, project: projectReady ? serverProject : null, addingProject: addingProject && Boolean(serverProject),
    setAddingProject: (value) => {
      setAddingProject(value)
      if (value) { setProjMode('new'); setProjName(''); setProjDesc('') }
    },
    setProjMode: (value) => {
      setProjMode(value)
      if (value === 'new') {
        setProjName('')
        setProjDesc('')
      } else {
        setProjName((current) => current || (candidate ? humanizeSlug(candidate) : ''))
      }
    },
    candidate,
    setCandidate: (value) => {
      if (projMode === 'adopt') {
        setProjName((current) => (!current || current === humanizeSlug(candidate) ? humanizeSlug(value) : current))
      }
      setCandidate(value)
    },
    projName, setProjName, projDesc, setProjDesc, projSlug, slugError,
    pairing, pairingLoading, pairedSession, renewPairing: () => { void pairingAction.run() },
    inviteUsername,
    // Sửa tên sau khi đã tạo = bắt đầu lời mời mới: mật khẩu tạm cũ đã gắn với tài khoản trước.
    setInviteUsername: (value) => { if (invited) { setInvited(null); setInvitePassword(makePassword()) }; setInviteUsername(value) },
    inviteRole,
    setInviteRole: (role) => setInviteRole(role),
    inviteProjects,
    setInviteProjects: (value) => { setInviteTouched(true); setInviteProjects(value) },
    invitePassword,
    refreshPassword: () => setInvitePassword(makePassword()),
    createInvite: async () => { await inviteAction.run() },
    inviteError, invited,
    inviteStories: inviteStoriesResource.data ?? [],
    enableLan: () => { void lanAction.run() },
    lanBusy: lanAction.busy,
    lanError: lanAction.error instanceof Error ? lanAction.error.message : '',
    assigned, assignedLoading: !assignedResource.data && assignedResource.initialLoading,
    go,
    mustChangePassword: user.mustChangePassword && !pwChanged,
    pwCurrentError,
    storyCount: libraryStoryCount(library),
    recovery,
    busy, error: error || (actionError instanceof Error && actionError !== dismissedConflict ? actionError.message : ''), invitePending: inviteAction.pending, inviteSuccess: inviteAction.success,
  }

  const previewInput: PreviewInput = {
    key: stepKey, user, isAdmin, name, providerLabel, providerId, providerStatus: providerStatusText, model, effort, shared,
    dirs, autoExport, formats,
    projectName: projectReady ? serverProject?.name || serverProject?.id || '' : projName,
    projectSlug: projectReady ? serverProject?.id ?? '' : projSlug,
    adopt: !projectReady && projMode === 'adopt', projectExists: projectReady, candidate,
    storageMode: library?.storageMode,
    libraryDisplay: libChoice.display,
    inviteUsername: inviteUsername || invited?.username || '',
    invitePassword: invited?.password ?? invitePassword,
    paired: Boolean(pairedSession), pwChanged: pwChanged || !user.mustChangePassword,
    baseURL: changing ? baseURL : config?.providers?.[providerId]?.base_url,
    oauthProvider: Boolean(presets.find((entry) => entry.id === providerId)?.oauth),
    inviteRole: inviteRole === 'view' ? 'Chỉ xem' : inviteRole === 'owner' ? 'Chủ dự án' : 'Cộng tác',
  }
  const preview = buildPreview(previewInput)

  const primaryLabel = stepKey === 'done'
    ? (isAdmin ? 'Viết truyện đầu tiên' : 'Mở truyện được giao')
    : ''
  const primaryDisabled = !canAdvance({
    busy, stepKey, name, pwCurrent, pwNext, pwRepeat, dirs, projMode, candidate, projName, slugError,
    changing, providerId, preset, apiKey, customName,
    libraryBlocked: libChoice.blocked || libChoice.empty,
    existingKeySet: Boolean(config?.providers?.[providerId === 'custom' ? customName : providerId]?.api_key_set),
    oauthConnected: oauth.phase === 'done' || Boolean(accountInfo?.connected),
    model, projectReady,
  })

  const railTitle = isAdmin ? 'Thiết lập thư viện' : 'Bắt đầu'
  const stepsLeft = steps.length - 1 - idx
  const railNote = isAdmin
    ? 'Đã dùng Noverse trước đây? Thiết lập cũ được giữ nguyên — chỉ cần kiểm tra lại.'
    : 'Nhà cung cấp AI, model và thư viện do quản trị viên quản lý.'
  const last = stepKey === 'done'
  const nextLabel = stepKey === 'provider' && !changing ? 'Dùng cấu hình này' : 'Tiếp tục'
  const row = (gap: number, ...children: ReactNode[]) => createElement('span', { style: { display: 'flex', alignItems: 'center', gap } }, ...children)

  return (
    <div onKeyDown={onKeyDown} style={{ height: '100vh', minHeight: '100vh', display: 'flex', flexDirection: 'column', background: 'var(--color-bg)', color: 'var(--color-fg)' }}>
      <header style={{ flex: '0 0 auto', display: 'flex', alignItems: 'center', gap: 14, padding: '12px 20px', borderBottom: '1px solid rgba(255,255,255,0.06)' }}>
        <BrandLockup size={22} gap={14} />
        <span style={{ color: '#3A3A40' }}>/</span>
        <span style={{ fontSize: 13, color: '#A0A0AB' }}>Thiết lập lần đầu</span>
        <span style={{ flex: 1 }} />
        <span className="num" style={{ fontSize: 12, color: '#7E7E8A' }}>Bước {idx + 1} / {steps.length}</span>
        {!narrow && (
          <div aria-hidden="true" style={{ width: 120, height: 4, borderRadius: 4, background: '#1E1E21' }}>
            <div style={{ width: `${Math.round((idx + 1) / steps.length * 100)}%`, height: '100%', borderRadius: 4, background: '#EDEDEF', transition: 'width .3s' }} />
          </div>
        )}
        <Avatar name={name || user.username} size={30} label={`Tài khoản: ${name || user.name || user.username}`} style={{ marginLeft: 8 }} />
      </header>

      <div style={{ flex: 1, minHeight: 0, display: 'flex' }}>
        {!narrow && (
          <nav aria-label="Các bước thiết lập" className="scroll" style={{
            flex: '0 0 272px', padding: '18px 14px', borderRight: '1px solid rgba(255,255,255,0.06)',
            display: 'flex', flexDirection: 'column', gap: 14, overflowY: 'auto',
          }}>
            {firstRun && isAdmin && (
              <div aria-label="Lần chạy đầu: đã xong" style={{
                display: 'flex', flexDirection: 'column', gap: 6, padding: '10px 12px', borderRadius: 14,
                border: '1px solid var(--kit-ok-16)', background: 'var(--kit-ok-4)',
              }}>
                <span style={{ fontSize: 12, color: 'var(--color-fg-3)' }}>Lần chạy đầu</span>
                {['Mã thiết lập', 'Tài khoản quản trị'].map((line) => (
                  <span key={line} style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 13, color: 'var(--color-ok)' }}>
                    <Icon name="tick-circle-bulk" size={15} aria-hidden="true" />{line}
                  </span>
                ))}
              </div>
            )}
            <div className="card" style={{ borderRadius: 16, padding: '12px 10px 10px' }}>
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '0 6px 8px' }}>
                <span style={{ fontSize: 13, fontWeight: 600 }}>{railTitle}</span>
                <span className="num" style={{ fontSize: 12, color: '#7E7E8A' }}>{stepsLeft ? `${stepsLeft} bước còn lại` : 'xong'}</span>
              </div>
              <ol style={{ listStyle: 'none', margin: 0, padding: 0, display: 'flex', flexDirection: 'column', gap: 2 }}>
                {steps.map((key, index) => {
                  const current = index === idx
                  const done = index < idx
                  const todo = index > idx
                  const value = done ? stepValue(key) : current ? 'đang thiết lập' : ''
                  return (
                    <li key={key}>
                      <button type="button" disabled={todo} onClick={() => go(index)} aria-current={current ? 'step' : undefined} style={{
                        width: '100%', display: 'flex', alignItems: 'flex-start', gap: 10, padding: 8, borderRadius: 10,
                        border: `1px solid ${current ? 'rgba(201,166,245,0.35)' : 'transparent'}`, background: current ? '#1E1E21' : 'transparent',
                        color: todo ? '#7E7E8A' : '#EDEDEF', textAlign: 'left',
                      }}>
                        <span aria-hidden="true" style={{ width: 18, height: 18, flex: '0 0 auto', marginTop: 1, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                          {done && <Icon name="tick-circle-bulk" size={18} style={{ color: '#A6EDC1' }} />}
                          {current && <span style={{ width: 16, height: 16, borderRadius: '50%', border: '2px solid #C9A6F5', display: 'flex', alignItems: 'center', justifyContent: 'center' }}><span style={{ width: 6, height: 6, borderRadius: '50%', background: '#C9A6F5' }} /></span>}
                          {todo && <span style={{ width: 16, height: 16, borderRadius: '50%', border: '1.5px dashed rgba(255,255,255,0.25)' }} />}
                        </span>
                        <span style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column' }}>
                          <span style={{ fontSize: 13, fontWeight: current ? 600 : 400, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{stepLabel(key, user.role)}</span>
                          <span style={{ fontSize: 11.5, color: '#7E7E8A', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{value}</span>
                        </span>
                        {todo && OPTIONAL_STEPS.includes(key) && (
                          <span style={{ flex: '0 0 auto', fontSize: 10.5, color: '#7E7E8A', marginTop: 2, whiteSpace: 'nowrap' }}>tùy chọn</span>
                        )}
                      </button>
                    </li>
                  )
                })}
              </ol>
            </div>
            <div style={{ padding: 12, borderRadius: 14, border: '1px solid rgba(255,255,255,0.07)', display: 'flex', gap: 10, fontSize: 12, color: '#A0A0AB' }}>
              <Icon name="info-circle" size={15} style={{ marginTop: 1 }} />
              <span>{railNote}</span>
            </div>
          </nav>
        )}

        <main style={{ flex: '999 1 560px', minWidth: 0, display: 'flex', flexDirection: 'column' }}>
          <div className="scroll" style={{ flex: 1, minHeight: 0, overflowY: 'auto', padding: narrow ? '20px 16px' : '30px 40px 24px' }}>
            <div ref={bodyRef} style={{ maxWidth: 680, margin: '0 auto', display: 'flex', flexDirection: 'column', gap: 22 }}>
              {narrow && (
                <div style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 11.5, color: '#7E7E8A' }}>
                  <span style={{ flex: 1, height: 4, borderRadius: 4, background: '#1E1E21' }}>
                    <span style={{ display: 'block', width: `${Math.round((idx + 1) / steps.length * 100)}%`, height: '100%', borderRadius: 4, background: '#EDEDEF' }} />
                  </span>
                  <span>{stepLabel(stepKey, user.role)}</span>
                </div>
              )}
              <PanelRefreshBar resourceKeys={[keys.config('global'), keys.library(), ['config', 'models', providerId], keys.sessions()]} updatedAt={configResource.updatedAt ?? libraryResource.updatedAt} />
              <StaleBanner error={configResource.error || presetsResource.error || libraryResource.error || modelsResource.error || oauthResource.error || assignedResource.error || sessionsResource.error || (!live.writable ? new Error('Mất kết nối') : null)} onRetry={() => { void configResource.refresh(); void libraryResource.refresh(); void modelsResource.refresh(); void oauthResource.refresh(); void assignedResource.refresh(); void sessionsResource.refresh() }} />
              {conflict && <ConflictBar pending={advanceAction.pending} onView={() => {
                setDismissedConflict(conflict)
                if (stepKey === 'provider' || stepKey === 'model') { const current = conflict.current as ConfigResponse; setConfig(configFileOf(current)); setConfigVersion(conflict.version); initialConfig.current = false; void configResource.refresh() }
                else if (stepKey === 'library') { libraryBaseline.current = null; void libraryResource.refresh() }
                else { profileBaseline.current = user; setName(user.name); void refresh() }
              }} onOverwrite={() => { void advanceAction.run(conflict.version) }} />}
              {actionError instanceof Error && !conflict && !pwCurrentError && actionError !== dismissedConflict && <span role="alert" style={{ color: 'var(--color-err)', fontSize: 12 }}>{actionError.message}</span>}
              <StepBody model={modelValue} />
              {error && (
                <div role="alert" style={{
                  display: 'flex', alignItems: 'center', gap: 8, padding: '10px 12px', borderRadius: 10,
                  background: 'var(--tint-err)', border: '1px solid rgba(255,154,162,0.3)', fontSize: 12.5, color: 'var(--color-fg-2)',
                }}>
                  <Icon name="warning-2" size={15} style={{ color: 'var(--color-err)' }} />{error}
                </div>
              )}
            </div>
          </div>

          <div style={{
            flex: '0 0 auto', display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap',
            padding: narrow ? '10px 16px calc(10px + env(safe-area-inset-bottom))' : '12px 40px',
            borderTop: '1px solid rgba(255,255,255,0.06)', background: '#0B0B0D',
          }}>
            {idx > 0 && (
              <Button variant="ghost" onClick={() => go(idx - 1)} style={{ height: 38, padding: '0 14px 0 10px', borderRadius: 10, border: '1px solid rgba(255,255,255,0.1)', fontSize: 13.5 }}>
                {row(6, <Icon name="arrow-left-2" size={16} />, 'Quay lại')}
              </Button>
            )}
            <span style={{ flex: 1 }} />
            {!narrow && <span style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 12, color: '#7E7E8A' }}><Kbd>Enter</Kbd>tiếp tục</span>}
            {OPTIONAL_STEPS.includes(stepKey) && (
              <Button variant="ghost" onClick={() => void advance(true)} disabled={busy} style={{ height: 38, padding: '0 14px', borderRadius: 10, border: 'none', color: '#A0A0AB', fontSize: 13.5 }}>Để sau</Button>
            )}
            {last && (
              <Button variant="ghost" onClick={() => { void finishAction.run(workspaceTarget) }} disabled={busy} pending={finishAction.pending} style={{ height: 38, padding: '0 14px', borderRadius: 10, border: '1px solid rgba(255,255,255,0.1)', fontSize: 13.5 }}>Vào Bàn viết</Button>
            )}
            <Button variant="primary" disabled={primaryDisabled} pending={advanceAction.pending || finishAction.pending} style={{ height: 38, padding: last ? '0 16px 0 12px' : '0 12px 0 16px', borderRadius: 10, fontSize: 13.5, fontWeight: 600 }} onClick={() => advance()}>
              {last ? row(8, <Icon name="magic-star" size={16} />, primaryLabel) : row(6, nextLabel, <Icon name="arrow-right-2" size={16} />)}
            </Button>
          </div>
        </main>

        {!narrow && (
          <aside aria-label="Thiết lập sẽ lưu" className="scroll" style={{
            flex: '0 0 340px', padding: '18px 16px', borderLeft: '1px solid rgba(255,255,255,0.06)',
            display: 'flex', flexDirection: 'column', gap: 12, overflowY: 'auto',
          }}>
            <WritePreview preview={preview} showTech={isAdmin && !!preview.file} />
          </aside>
        )}
      </div>
    </div>
  )

  function stepValue(key: StepKey): string {
    if (key === 'profile') return `${name || user?.username || ''} · @${user?.username ?? ''}`
    if (key === 'password') return pwChanged ? 'đã đổi' : user?.mustChangePassword ? 'nên đổi mật khẩu tạm' : 'đã đặt mật khẩu riêng'
    if (key === 'provider') return configuredProvider ? providerLabel : 'chưa có'
    if (key === 'model') return `${model || '—'}${shared ? ' · dùng chung' : ' · riêng từng trợ lý'}`
    if (key === 'library') return library?.storageMode && library.storageMode !== 'file'
      ? libChoice.display : `${libChoice.display || '—'} · tự xuất bản ${autoExport ? 'bật' : 'tắt'}`
    if (key === 'project') return serverProject ? serverProject.name || serverProject.id : 'chưa tạo'
    if (key === 'mobile') return isAdmin ? (invited ? 'QR · mời 1 người' : 'QR') : 'QR'
    return ''
  }
}
