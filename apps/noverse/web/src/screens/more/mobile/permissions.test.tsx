import { beforeEach, describe, expect, it, vi } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import type { Role } from '../../../api/types'
import { ApiError } from '../../../api/client'

const state = vi.hoisted(() => ({ role: 'member' as Role, permission: 'view' as 'view' | 'collab', appConfig: undefined as unknown, appConfigError: null as unknown, sessionError: null as unknown, resources: vi.fn() }))
vi.mock('../../../api/session', () => ({ useSession: () => ({ user: { username: 'lan', name: 'Lan', role: state.role, projects: { p: state.permission } }, session: { id: 'current', device: 'Điện thoại', expiresAt: '2099-01-01' }, setUser: vi.fn() }) }))
vi.mock('../../../data/useResource', () => ({ useResource: (...args: unknown[]) => state.resources(...args) }))
vi.mock('../../../data/useAction', () => ({ useAction: () => ({ busy: false, pending: false, error: null, run: vi.fn() }) }))
vi.mock('../../../mobile', () => ({ MobilePage: ({ children }: { children: React.ReactNode }) => <main>{children}</main>, ConfirmSheet: () => null, useMobileToast: () => vi.fn() }))
vi.mock('../../../mobile/runtime', () => ({ useTabMode: vi.fn() }))
vi.mock('@noverse/ui', async (importOriginal) => ({ ...(await importOriginal<object>()),  Icon: () => null,  AgentOrb: () => null }))
vi.mock('./Accounts', () => ({ AccountsSheet: () => null, AccountAvatar: () => null, RoleBadge: () => null }))
vi.mock('./Password', () => ({ MobilePassword: () => null, MobileFirstPassword: () => null }))
vi.mock('./Guide', () => ({ MobileGuide: () => null, desktopCommand: () => false }))
vi.mock('./Models', () => ({ Models: () => <div>TRANG_MODEL</div> }))
import { MobileMore } from './index'

function snapshot(data: unknown, error: unknown = null) {
  return { data, error, status: error ? 'error' : data === undefined ? 'loading' : 'ready', refreshing: false, refresh: vi.fn() }
}
beforeEach(() => {
  vi.stubGlobal('navigator', { onLine: true })
  state.role = 'member'; state.permission = 'view'; state.appConfig = undefined
  state.appConfigError = null
  state.sessionError = null
  state.resources.mockReset().mockImplementation((key: string[]) => {
    if (key[0] === 'config' && key[1] === 'global') return snapshot(state.appConfig, state.appConfigError)
    if (key[0] === 'config') return snapshot(undefined, new ApiError(403, 'Bạn không có quyền xem cấu hình'))
    if (key[0] === 'sessions') return snapshot(state.sessionError ? undefined : { sessions: [] }, state.sessionError)
    if (key[0] === 'library') return snapshot({ projects: [] })
    if (key[0] === 'rules') return snapshot({ snapshot: null, items: [], runtime: [] })
    return snapshot({ stories: [{ story: 's', title: 'Truyện đang đọc', volumes: 1 }] })
  })
})

const appConfig = (config: unknown) => ({ target: { config }, effective: config, version: 'v', chain: [], sources: {} })

describe('M4 không biến lỗi quyền thành trạng thái tải vô hạn', () => {
  it('người đọc vẫn thấy model dùng chung, không bị khoá theo quyền viết truyện', () => {
    state.appConfig = appConfig({ provider: 'openai', model: 'gpt-live' })
    const html = renderToStaticMarkup(<MobileMore project="p" story="s" />)
    expect(html).toContain('Model cho từng trợ lý')
    expect(html).toContain('Dùng chung cho cả app')
    expect(html).toContain('Model cho từng trợ lý')
    expect(html).not.toContain('Đang tải')
    expect(state.resources.mock.calls.filter(([key]) => key[0] === 'config' && key[1] === 'global')).toHaveLength(1)
    const direct = renderToStaticMarkup(<MobileMore project="p" story="s" initialPage="model" />)
    expect(direct).toContain('TRANG_MODEL')
  })

  it('tóm tắt model người viết đọc từ cấu hình chung, có nhãn “Dùng chung cho cả app”', () => {
    state.appConfig = appConfig({ provider: 'openai', model: 'gpt-6.1-sol', roles: { writer: { provider: 'openai', model: 'gpt-live' } } })
    const html = renderToStaticMarkup(<MobileMore project="p" story="s" />)
    const row = html.match(/<button[^>]*>[\s\S]*?Model cho từng trợ lý[\s\S]*?<\/button>/)?.[0] ?? ''
    expect(row).toContain('gpt-live')
    expect(row).toContain('Dùng chung cho cả app')
    expect(row).toContain('Dùng chung cho cả app')
  })

  it('lỗi model/thiết bị kết thúc bằng phản hồi thử lại, không giữ nhãn đang tải hay dấu ba chấm', () => {
    state.permission = 'collab'; state.appConfigError = new ApiError(403, 'Không có quyền đọc cấu hình'); state.sessionError = new ApiError(403, 'Không có quyền đọc thiết bị')
    const html = renderToStaticMarkup(<MobileMore project="p" story="s" />)
    expect(html).toContain('Không tải được model')
    expect(html).toContain('Không tải được thiết bị')
    expect(html).toContain('Thử lại')
    expect(html).not.toContain('Đang tải')
    expect(html).not.toContain('…')
  })
})
