// Hồi quy cho các lỗi gặp trên máy chủ thật: cấu hình rỗng, PUT ghi lẫn trường,
// model rỗng vẫn bấm được Tiếp tục, và bước Dự án chọn sai chế độ.
import { describe, expect, it } from 'vitest'
import type { ConfigResponse, ProjectSummary, ProviderConfig, ProviderPreset, User } from '../../api/types'
import {
  ADMIN_STEPS, FORMAT_LABEL, ONBOARD_FORMATS, PROVIDER_INTRO, PROVIDER_PRESENTATION, buildPreview, canAdvance, configFileOf, configuredProviderId, deviceCodeHint, draftProvider, libraryStoryCount, oauthChoices, oauthDescription, onboardProject,
  passwordStepCopy, projectDefaults, projectSlugError,
  providerConnection, providerStatusLabel, stepsFor, writableConfig, type AdvanceInput, type PreviewInput,
} from './model'

const user: User = {
  username: 'dat', name: 'Dat', role: 'admin', projects: {}, disabled: false,
  createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-01-01T00:00:00Z',
  onboardedAt: null, mustChangePassword: false, lastSeenAt: null,
}

const preset: ProviderPreset = {
  id: 'anthropic', label: 'Anthropic Claude', type: 'anthropic', baseURL: 'https://api.anthropic.com',
  keyRequired: true, oauth: false, api: 'chat', defaultModel: 'claude-sonnet-4',
}
const oauthPreset: ProviderPreset = { ...preset, id: 'codex', label: 'ChatGPT Plus/Pro', oauth: true, keyRequired: false }

const advance = (over: Partial<AdvanceInput>): AdvanceInput => ({
  busy: false, stepKey: 'model', name: 'Dat', pwCurrent: '', pwNext: '', pwRepeat: '', dirs: null,
  projMode: 'new', candidate: '', projName: '', slugError: '', changing: false, providerId: 'anthropic',
  preset, apiKey: '', existingKeySet: false, oauthConnected: false, model: '', ...over,
})

const preview = (over: Partial<PreviewInput>): PreviewInput => ({
  key: 'provider', user, isAdmin: true, name: 'Dat', providerLabel: '—', providerId: '',
  providerStatus: 'Chưa có', model: '', effort: '', shared: true,
  dirs: null, autoExport: true, formats: ['txt'], projectName: '', projectSlug: '', adopt: false, candidate: '',
  libraryDisplay: '', inviteUsername: '', inviteRole: 'Người viết', invitePassword: '', paired: false, pwChanged: false, ...over,
})

const project = (id: string): ProjectSummary => ({
  id, name: id, description: '', legacy: false, storyCount: 0, runningCount: 0, updatedAt: null, ruleCount: 0, style: '',
})

describe('E2EAUTH-5 · bước Dự án lấy trạng thái từ server', () => {
  const legacy = { ...project('_'), legacy: true, storyCount: 3 }
  it('tải lại trang (mất state cục bộ) vẫn thấy dự án đã tạo, bỏ dự án ẩn danh', () => {
    expect(onboardProject([legacy, project('kenh-thu-e2e')], null)?.id).toBe('kenh-thu-e2e')
  })
  it('ưu tiên dự án vừa tạo trong phiên', () => {
    expect(onboardProject([project('cu'), project('moi')], 'moi')?.id).toBe('moi')
  })
  it('chỉ có thư mục cũ ⇒ chưa có dự án', () => {
    expect(onboardProject([legacy], null)).toBeNull()
  })
  it('đã có dự án ⇒ Tiếp tục bật mà không cần form (không tạo trùng)', () => {
    expect(canAdvance(advance({ stepKey: 'project', projMode: 'new', projName: '', projectReady: true }))).toBe(true)
    expect(canAdvance(advance({ stepKey: 'project', projMode: 'new', projName: '' }))).toBe(false)
  })
})

describe('Gói A · bước Nơi lưu truyện không lộ thư mục', () => {
  it('đếm số truyện của mọi dự án (kể cả dự án ẩn danh)', () => {
    const legacy = { ...project('_'), legacy: true, storyCount: 3 }
    expect(libraryStoryCount({ projects: [legacy, { ...project('a'), storyCount: 2 }] })).toBe(5)
    expect(libraryStoryCount({ projects: [project('a')] })).toBe(0)
    expect(libraryStoryCount(null)).toBe(0)
  })
  it('cột tóm tắt nói “Mỗi dự án có: Ý tưởng · Tư liệu · Truyện · Bản xuất” và không lộ đường dẫn / tên thư mục', () => {
    const plain = buildPreview(preview({ key: 'library', libraryDisplay: 'Tài liệu › Truyện Noverse', dirs: { ideas: 'y-tuong', assets: 'tai-san', stories: 'kich_ban', exports: 'xuat-ban' } })).plain
    expect(plain).toContainEqual({ k: 'Lưu ở', v: 'Tài liệu › Truyện Noverse' })
    expect(plain).toContainEqual({ k: 'Mỗi dự án có', v: 'Ý tưởng · Tư liệu · Truyện · Bản xuất' })
    expect(JSON.stringify(plain)).not.toMatch(/y-tuong|tai-san|kich_ban|xuat-ban|output|\.json|\//)
  })
  it('bước Hoàn tất ghi “Nơi › Thư mục”, không ghi đường dẫn', () => {
    const plain = buildPreview(preview({ key: 'done', libraryDisplay: 'Tài liệu › Truyện Noverse', projectName: 'Kênh truyện audio' })).plain
    expect(plain).toContainEqual({ k: 'Nơi lưu & dự án', v: 'Tài liệu › Truyện Noverse · Kênh truyện audio' })
  })
})

describe('E2EAUTH-9/13 · chữ bước Mật khẩu theo mustChangePassword', () => {
  it('chỉ nói “mật khẩu tạm” khi server còn yêu cầu đổi', () => {
    expect(passwordStepCopy(true).currentLabel).toBe('Mật khẩu tạm')
    expect(passwordStepCopy(false).currentLabel).toBe('Mật khẩu hiện tại')
    expect(passwordStepCopy(false).lead).not.toMatch(/mật khẩu tạm/)
  })
})

describe('configFileOf', () => {
  it('lấy nội dung tệp (target), không lấy effective có provider/model rỗng', () => {
    const response: ConfigResponse = {
      target: { scope: 'global', path: '~/.ainovel/config.json', exists: false, config: {} },
      effective: { provider: '', model: '', style: 'satira' },
      chain: [], sources: {},
    }
    expect(configFileOf(response)).toEqual({})
  })
})

describe('writableConfig', () => {
  it('bỏ provider/model rỗng và providers rỗng khi PUT', () => {
    expect(writableConfig({ provider: '', model: '', providers: {}, style: 'satira' })).toEqual({ style: 'satira' })
  })

  it('giữ nguyên giá trị thật của provider/model', () => {
    const config = {
      provider: 'codex', model: 'gpt-5.4', providers: { codex: { type: 'openai' } }, style: 'satira',
    }
    expect(writableConfig(config)).toEqual(config)
  })
})

describe('configuredProviderId', () => {
  it('tệp rỗng hoặc provider không có trong providers ⇒ chưa cấu hình', () => {
    expect(configuredProviderId(null)).toBe('')
    expect(configuredProviderId({})).toBe('')
    expect(configuredProviderId({ provider: '', model: '' })).toBe('')
    expect(configuredProviderId({ provider: 'codex' })).toBe('')
  })

  it('chỉ tính khi có cả provider lẫn providers.<id>', () => {
    expect(configuredProviderId({ provider: 'codex', providers: { codex: { type: 'openai' } } })).toBe('codex')
  })
})

describe('QĐ 6 · bỏ bước Ngôn ngữ sáng tác (Onboard.dc.html)', () => {
  it('admin còn 7 bước, không còn bước ngôn ngữ', () => {
    expect(ADMIN_STEPS).toEqual(['profile', 'provider', 'model', 'library', 'project', 'mobile', 'done'])
    expect(ADMIN_STEPS).not.toContain('language')
  })
  it('tệp config.json vẫn ghi language "vi" nhưng không tô sáng dòng nào ở bước Model', () => {
    const result = buildPreview(preview({ key: 'model', providerId: 'anthropic', model: 'claude-sonnet-4' }))
    const line = result.lines.find((entry) => entry.segs.some((seg) => seg.text === '"language"'))
    expect(line?.segs.map((seg) => seg.text).join('')).toBe('"language": "vi",')
    expect(line?.hot).toBeFalsy()
    expect(result.plain.some((row) => row.k === 'Ngôn ngữ viết truyện')).toBe(false)
  })
  it('writableConfig không gửi lại language khác "vi" (tệp cũ còn zh)', () => {
    expect(writableConfig({ language: 'zh', style: 'default' })).toEqual({ style: 'default' })
    expect(writableConfig({ language: 'vi' })).toEqual({ language: 'vi' })
  })
})

describe('Bản xuất tự động · ba lựa chọn của artboard', () => {
  it('ba chip đúng thứ tự và nhãn tiếng Việt, mặc định bật Văn bản + Ebook', () => {
    expect(ONBOARD_FORMATS).toEqual(['txt', 'epub', 'narration'])
    expect(ONBOARD_FORMATS.map((format) => FORMAT_LABEL[format])).toEqual(['Văn bản', 'Ebook', 'Bản đọc TTS'])
  })
  it('dòng tóm tắt dùng nhãn tiếng Việt, không dùng mã kỹ thuật', () => {
    const rows = buildPreview(preview({ key: 'library', formats: ['txt', 'epub', 'narration'] })).plain
    expect(rows).toContainEqual({ k: 'Tự xuất bản khi xong một tập', v: 'Văn bản + Ebook + Bản đọc TTS' })
    const json = buildPreview(preview({ key: 'library', formats: ['narration'] })).lines.map((line) => line.segs.map((seg) => seg.text).join('')).join('\n')
    expect(json).toContain('"narration"')
  })
})

describe('Bước Mobile · người cùng viết', () => {
  it('hiện vai trò của người được mời, không phải vai trò của quản trị', () => {
    const plain = buildPreview(preview({ key: 'mobile', inviteUsername: 'linh', inviteRole: 'Người đọc' })).plain
    expect(plain).toContainEqual({ k: 'Người cùng viết', v: 'linh · Người đọc' })
  })
})

describe('providerConnection', () => {
  const base = {
    configuredProvider: '', providerId: '', changing: true,
    testOk: false, testFailed: false, oauthDone: false, accountConnected: false,
  }

  it('cấu hình rỗng ⇒ Chưa có', () => {
    expect(providerConnection(base)).toBe('none')
    expect(providerStatusLabel('none')).toBe('Chưa có')
  })

  it('vừa chọn provider nhưng chưa test ⇒ Chưa kết nối', () => {
    expect(providerConnection({ ...base, providerId: 'anthropic' })).toBe('untested')
    expect(providerStatusLabel('untested')).toBe('Chưa kết nối')
  })

  it('test thành công / OAuth xong / tài khoản đã liên kết ⇒ Đã kết nối', () => {
    expect(providerConnection({ ...base, providerId: 'anthropic', testOk: true })).toBe('connected')
    expect(providerConnection({ ...base, providerId: 'codex', oauthDone: true })).toBe('connected')
    expect(providerConnection({ ...base, providerId: 'codex', accountConnected: true })).toBe('connected')
    expect(providerStatusLabel('connected')).toBe('Đã kết nối')
  })

  it('provider đã lưu sẵn ⇒ Đã kết nối, nhưng đang đổi thì phải test lại', () => {
    expect(providerConnection({ ...base, changing: false, configuredProvider: 'codex', providerId: 'codex' })).toBe('connected')
    expect(providerConnection({ ...base, changing: true, configuredProvider: 'codex', providerId: 'codex' })).toBe('untested')
  })

  it('test thất bại thì không báo Đã kết nối dù provider đã lưu', () => {
    expect(providerConnection({
      ...base, changing: false, configuredProvider: 'codex', providerId: 'codex', testFailed: true,
    })).toBe('untested')
  })
})

describe('canAdvance', () => {
  it('bước Model bắt buộc có model chính', () => {
    expect(canAdvance(advance({ stepKey: 'model', model: '' }))).toBe(false)
    expect(canAdvance(advance({ stepKey: 'model', model: '   ' }))).toBe(false)
    expect(canAdvance(advance({ stepKey: 'model', model: 'gpt-5.4' }))).toBe(true)
  })

  it('bước Model cần provider đã chọn', () => {
    expect(canAdvance(advance({ stepKey: 'model', providerId: '', model: 'gpt-5.4' }))).toBe(false)
  })

  it('bước Provider khi đổi: phải chọn preset, khoá mới hoặc khoá cũ', () => {
    expect(canAdvance(advance({ stepKey: 'provider', changing: true, providerId: '', preset: null }))).toBe(false)
    expect(canAdvance(advance({ stepKey: 'provider', changing: true }))).toBe(false)
    expect(canAdvance(advance({ stepKey: 'provider', changing: true, apiKey: 'sk-new' }))).toBe(true)
    expect(canAdvance(advance({ stepKey: 'provider', changing: true, existingKeySet: true }))).toBe(true)
  })

  it('bước Provider OAuth: chỉ đi tiếp khi đã đăng nhập', () => {
    expect(canAdvance(advance({ stepKey: 'provider', changing: true, preset: oauthPreset }))).toBe(false)
    expect(canAdvance(advance({ stepKey: 'provider', changing: true, preset: oauthPreset, oauthConnected: true }))).toBe(true)
  })

  it('provider tự đặt phải có tên hợp lệ trước khi lưu', () => {
    const custom = { ...preset, id: 'custom', type: '' }
    const input = advance({ stepKey: 'provider', changing: true, providerId: 'custom', preset: custom, apiKey: 'sk-new' })
    expect(canAdvance({ ...input, customName: '' })).toBe(false)
    expect(canAdvance({ ...input, customName: 'proxy A' })).toBe(false)
    expect(canAdvance({ ...input, customName: 'my-proxy' })).toBe(true)
  })

  it('bước Dự án: nhận thư mục cần candidate, tạo mới cần tên và không trùng', () => {
    expect(canAdvance(advance({ stepKey: 'project', projMode: 'adopt', candidate: '' }))).toBe(false)
    expect(canAdvance(advance({ stepKey: 'project', projMode: 'adopt', candidate: 'kho-truyen-cu' }))).toBe(true)
    expect(canAdvance(advance({ stepKey: 'project', projMode: 'new', projName: '' }))).toBe(false)
    expect(canAdvance(advance({ stepKey: 'project', projMode: 'new', projName: 'Truyện mới' }))).toBe(true)
    expect(canAdvance(advance({
      stepKey: 'project', projMode: 'new', projName: 'Dit Dang Audio', slugError: 'Thư mục này đã có trong thư viện.',
    }))).toBe(false)
  })
})

describe('projectDefaults', () => {
  it('chưa đọc xong thư viện ⇒ không quyết định (không tự chuyển sang Tạo mới)', () => {
    expect(projectDefaults(undefined)).toBeNull()
  })

  it('có thư mục chờ nhận ⇒ Dùng thư mục có sẵn + thư mục đầu', () => {
    expect(projectDefaults(['dit-dang-audio', 'ban-thao-2024'])).toEqual({
      mode: 'adopt', candidate: 'dit-dang-audio',
    })
  })

  it('không có thư mục chờ nhận ⇒ Tạo mới, form rỗng', () => {
    expect(projectDefaults([])).toEqual({ mode: 'new', candidate: '' })
  })
})

describe('projectSlugError', () => {
  it('báo trùng dự án đã có', () => {
    expect(projectSlugError('Dit Dang Audio', [project('dit-dang-audio')], [])).toBe('Thư mục này đã có dự án trong thư viện.')
  })

  it('báo trùng thư mục đang chờ nhận', () => {
    expect(projectSlugError('Dit Dang Audio', [], ['dit-dang-audio'])).toBe('Thư mục này đã có trong thư viện.')
  })

  it('báo tên không còn ký tự dùng được', () => {
    expect(projectSlugError('!!!', [], [])).toBe('Tên cần có ít nhất một chữ cái hoặc số.')
  })

  it('không báo khi tên rỗng hoặc không trùng', () => {
    expect(projectSlugError('  ', [], ['dit-dang-audio'])).toBe('')
    expect(projectSlugError('Trà Xanh', [project('dit-dang-audio')], ['ban-thao-2024'])).toBe('')
  })
})

describe('draftProvider', () => {
  it('bỏ api_key_set/api_key_hint của bản GET trước khi gửi /api/config/test', () => {
    const base: ProviderConfig = {
      type: 'openai', base_url: 'https://api.openai.com/v1', api_key_set: true, api_key_hint: 'sk-…abcd',
      models: [{ name: 'gpt-5.4' }],
    }
    const draft = draftProvider(base, null, '', 'sk-new')
    expect(draft).not.toHaveProperty('api_key_set')
    expect(draft).not.toHaveProperty('api_key_hint')
    expect(draft.api_key).toBe('sk-new')
    expect(draft.base_url).toBe('https://api.openai.com/v1')
    expect(draft.models).toEqual([{ name: 'gpt-5.4' }])
  })
})

describe('buildPreview', () => {
  it('QĐ 6: admin 7 bước, không còn Ngôn ngữ sáng tác; người viết 4 bước', () => {
    expect(ADMIN_STEPS).toEqual(['profile', 'provider', 'model', 'library', 'project', 'mobile', 'done'])
    expect(stepsFor('member')).toEqual(['profile', 'password', 'mobile', 'done'])
  })

  it('bước Dự án không ghi language vào project.json', () => {
    const json = buildPreview(preview({ key: 'project', projectName: 'Trà Xanh', projectSlug: 'tra-xanh' })).lines
      .map((line) => line.segs.map((seg) => seg.text).join('')).join('\n')
    expect(json).toContain('"name": "Trà Xanh"')
    expect(json).not.toContain('"language"')
  })

  it('bước Provider hiển thị đúng trạng thái chưa có', () => {
    const rows = buildPreview(preview({ key: 'provider', providerId: '', providerStatus: 'Chưa có' })).plain
    expect(rows).toContainEqual({ k: 'Trạng thái', v: 'Chưa có' })
  })

  it('bước Dự án nhận thư mục hiển thị đúng thư mục chờ nhận', () => {
    const input = preview({ key: 'project', adopt: true, candidate: 'dit-dang-audio', projectName: 'Dit Dang Audio' })
    const previewResult = buildPreview(input)
    expect(previewResult.file).toBe('dit-dang-audio/project.json')
    expect(previewResult.plain).toContainEqual({ k: 'Thư mục', v: 'dit-dang-audio' })
  })
})

describe('Gói thuê bao SuperGrok / X Premium+ (xai-oauth)', () => {
  it('thẻ nhà cung cấp có nhãn + gợi ý riêng và đứng ngay sau ChatGPT Plus/Pro', () => {
    expect(PROVIDER_PRESENTATION['xai-oauth']).toEqual({ label: 'SuperGrok / X Premium+', hint: 'Gói thuê bao · đăng nhập OAuth' })
    const ids = Object.keys(PROVIDER_PRESENTATION)
    expect(ids.indexOf('xai-oauth')).toBe(ids.indexOf('openai-codex') + 1)
  })
  it('câu dẫn nhắc đăng nhập gói thuê bao chứ không chỉ ChatGPT', () => {
    expect(PROVIDER_INTRO).toContain('SuperGrok).')
  })
  it('xAI chỉ có mã thiết bị: không đưa lựa chọn “Mở trang đăng nhập” localhost', () => {
    const xai = { ...oauthPreset, id: 'xai-oauth', deviceOnly: true }
    expect(oauthChoices(xai).map((choice) => choice.device)).toEqual([true])
    expect(oauthChoices(oauthPreset).map((choice) => choice.device)).toEqual([false, true])
  })
  it('lời giải thích theo đúng dịch vụ, không nhắc ChatGPT cho xAI', () => {
    expect(oauthDescription({ id: 'xai-oauth' })).toContain('SuperGrok')
    expect(oauthDescription({ id: 'xai-oauth' })).not.toContain('ChatGPT')
    expect(oauthDescription({ id: 'openai-codex' })).toContain('ChatGPT')
    expect(deviceCodeHint('xai-oauth')).not.toContain('ChatGPT')
    expect(deviceCodeHint('openai-codex')).toContain('ChatGPT')
  })
})

describe('Gói thuê bao Claude Pro/Max (anthropic-oauth)', () => {
  it('thẻ nhà cung cấp có nhãn + gợi ý riêng, cạnh ChatGPT Plus/Pro và SuperGrok', () => {
    expect(PROVIDER_PRESENTATION['anthropic-oauth']).toEqual({ label: 'Claude Pro/Max', hint: 'Gói thuê bao · đăng nhập OAuth' })
    const ids = Object.keys(PROVIDER_PRESENTATION)
    expect(ids.indexOf('anthropic-oauth')).toBe(ids.indexOf('xai-oauth') + 1)
    expect(PROVIDER_PRESENTATION.anthropic).toEqual({ label: 'Anthropic', hint: 'API Key · Claude' })
  })
  it('câu dẫn nhắc cả ba gói thuê bao', () => {
    expect(PROVIDER_INTRO).toContain('hoặc đăng nhập gói thuê bao (ChatGPT Plus/Pro, Claude Pro/Max, SuperGrok).')
  })
  it('lời giải thích nhắc đúng Claude, không nhắc ChatGPT/xAI', () => {
    const text = oauthDescription({ id: 'anthropic-oauth' })
    expect(text).toContain('Claude Pro/Max')
    expect(text).not.toContain('ChatGPT')
    expect(text).not.toContain('xAI')
  })
  it('có cả “Mở trang đăng nhập” (cùng máy) và “Dùng mã thiết bị” (dán mã)', () => {
    const claude = { ...oauthPreset, id: 'anthropic-oauth', pasteCode: true }
    expect(oauthChoices(claude).map((choice) => choice.device)).toEqual([false, true])
  })
})
