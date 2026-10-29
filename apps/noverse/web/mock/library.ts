// Mock GAP-34 — hệ tệp ảo cho bộ chọn nơi lưu truyện (09-auth-onboard.md §10, Onboard.dc.html › lib).
// Dữ liệu lấy đúng từ artboard: Nơi (Thư mục nhà, Tài liệu, Màn hình, iCloud Drive, Samsung T7) + các dòng của từng Nơi.
// Chọn kịch bản bằng AINOVEL_MOCK_LIBRARY: existing (mặc định) · fresh (cài mới, chưa có nơi lưu) · readonly.
import type { LibraryInspect, FsFolder, FsPlace } from '../src/api/types'

type Node = { kind: 'dir' | 'library' | 'readonly'; items: number; projects?: number; stories?: number }

const HOME = '/mock/home'
const PLACES: FsPlace[] = [
  { id: 'home', label: 'Thư mục nhà', path: HOME, icon: 'user' },
  { id: 'documents', label: 'Tài liệu', path: `${HOME}/Documents`, icon: 'document-text' },
  { id: 'desktop', label: 'Màn hình', path: `${HOME}/Desktop`, icon: 'monitor' },
  { id: 'icloud', label: 'iCloud Drive', path: `${HOME}/Library/Mobile Documents/com~apple~CloudDocs`, icon: 'cloud-connection' },
  { id: 'volume:Samsung T7', label: 'Samsung T7', path: '/Volumes/Samsung T7', icon: 'box' },
]

/** Thư mục con theo thư mục cha (tên → nút). */
const TREE = new Map<string, Map<string, Node>>([
  [HOME, new Map<string, Node>([
    ['Truyen', { kind: 'library', items: 3, projects: 1, stories: 1 }],
    ['Tài liệu', { kind: 'dir', items: 5 }],
    ['Màn hình', { kind: 'dir', items: 2 }],
    ['Tải về', { kind: 'dir', items: 12 }],
  ])],
  [`${HOME}/Documents`, new Map<string, Node>([
    ['Bản thảo cũ', { kind: 'dir', items: 24 }],
    ['Hoá đơn', { kind: 'dir', items: 112 }],
    ['Truyện Noverse', { kind: 'dir', items: 0 }],
  ])],
  [`${HOME}/Desktop`, new Map<string, Node>([['Ảnh chụp', { kind: 'dir', items: 8 }]])],
  [PLACES[3].path, new Map<string, Node>([
    ['Truyện của Dat', { kind: 'dir', items: 0 }],
    ['Ghi chú', { kind: 'dir', items: 31 }],
  ])],
  ['/Volumes/Samsung T7', new Map<string, Node>([['Sao lưu cũ', { kind: 'readonly', items: 7 }]])],
])

const scenario = () => process.env.AINOVEL_MOCK_LIBRARY ?? 'existing'

/** Gốc thư viện hiện tại; '' = chưa chọn (cài mới). */
let root = scenario() === 'fresh' ? '' : `${HOME}/Truyen`

export const mockLibraryRoot = () => root
export const setMockLibraryRoot = (path: string) => { root = path }
export const mockLibraryConfigured = () => root !== ''

const split = (path: string) => path.replace(/\/+$/, '')
const parentOf = (path: string) => split(path).slice(0, split(path).lastIndexOf('/')) || '/'
const nameOf = (path: string) => split(path).slice(split(path).lastIndexOf('/') + 1)

const placeOf = (path: string) => PLACES.filter((p) => path === p.path || path.startsWith(`${p.path}/`)).sort((a, b) => b.path.length - a.path.length)[0]

export function mockDisplay(path: string): string {
  if (!path) return ''
  const place = placeOf(path)
  const rel = place ? split(path).slice(place.path.length).split('/').filter(Boolean) : split(path).split('/').filter(Boolean)
  const segs = rel.length > 3 ? [rel[0], '…', rel[rel.length - 1]] : rel
  return [place?.label ?? 'Máy chủ', ...segs].join(' › ')
}

function nodeAt(path: string): Node | null {
  const parent = parentOf(path)
  return TREE.get(parent)?.get(nameOf(path)) ?? null
}

export const mockPlaces = () => PLACES

export function mockList(path: string): { status: number; body: FsFolder[] | { error: string } } {
  const clean = split(path)
  if (!placeOf(clean)) return { status: 403, body: { error: 'Không duyệt được thư mục này' } }
  const children = TREE.get(clean)
  const here = clean === placeOf(clean)?.path || nodeAt(clean)
  if (!here) return { status: 404, body: { error: 'Không tìm thấy thư mục' } }
  const rows: FsFolder[] = [...(children ?? [])].map(([name, node]) => ({
    name, path: `${clean}/${name}`, kind: node.kind, items: node.items, writable: node.kind !== 'readonly',
    ...(node.kind === 'library' ? { projects: node.projects } : {}),
  }))
  return { status: 200, body: rows }
}

export function mockMkdir(parent: string, name: string): { status: number; body: unknown } {
  const clean = split(parent)
  if (!placeOf(clean)) return { status: 403, body: { error: 'Không duyệt được thư mục này' } }
  if (!name.trim() || /[/\\:*?"<>|]/.test(name) || name.startsWith('.')) return { status: 400, body: { error: 'Tên thư mục không hợp lệ' } }
  const siblings = TREE.get(clean) ?? new Map<string, Node>()
  if (siblings.has(name)) return { status: 409, body: { error: 'Đã có thư mục tên này' } }
  siblings.set(name, { kind: 'dir', items: 0 })
  TREE.set(clean, siblings)
  const path = `${clean}/${name}`
  return { status: 201, body: { name, path, display: mockDisplay(path) } }
}

export function mockInspect(path: string): LibraryInspect {
  const clean = split(path)
  const base = { place: placeOf(clean)?.label ?? '', display: mockDisplay(clean), name: nameOf(clean) }
  const node = nodeAt(clean)
  if (!node) return { kind: 'missing', projects: 0, stories: 0, items: 0, ...base }
  if (node.kind === 'readonly') return { kind: 'readonly', projects: 0, stories: 0, items: node.items, ...base }
  if (node.kind === 'library') return { kind: 'found', projects: node.projects ?? 1, stories: node.stories ?? 1, items: node.items, projectName: 'Kênh truyện audio', ...base }
  return { kind: node.items === 0 ? 'empty' : 'other', projects: 0, stories: 0, items: node.items, ...base }
}

/** PUT /api/library {root} — kiểm tra như CheckRoot rồi mới đổi. */
export function mockSwitchRoot(path: string): { status: number; body: unknown } {
  const clean = split(path)
  const node = nodeAt(clean)
  if (!node) return { status: 400, body: { error: 'thư mục thư viện không hợp lệ: không tìm thấy thư mục' } }
  if (node.kind === 'readonly') return { status: 400, body: { error: 'thư mục thư viện không hợp lệ: không ghi được vào thư mục' } }
  root = clean
  return { status: 200, body: null }
}
