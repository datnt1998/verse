import { golden } from './golden'
import type {
  LedgerCheckResult, LedgerSuggestion, ProjectLedger, ProjectSearch, ProjectUniverse, ProjectUniverseEntity,
  UniverseCheckItem, UniverseKeptItem,
} from '../src/api/types'

// Brief đã qua “Fix một lần” của artboard: kết quả chấm lại là ledger-check-fixed.json.
export const MOCK_LEDGER_FIXED_BRIEF = 'Chị Dâu Bắt Tôi May Váy Cưới Cho Em Gái Nuôi'

export const MOCK_NEW_STORY_BRIEF =
  'Thợ May Trọng Sinh: Mẹ Kế Cướp Tiệm May Của Tôi, Tôi Lấy Lại Từng Đường Kim'

// Mock phát lại golden do Go test ghi (web/mock/golden/*.json); code mock chỉ
// còn ba phần thuần túy: lọc theo quyền xem (canView), chọn biến thể theo brief,
// và áp các chuyển trạng thái người dùng vừa bấm (bỏ giữ mục §19).

// Golden ghi id truyện theo dự án fixture "proj"; mock đổi nhánh "proj/" thành
// dự án được hỏi để giữ quan hệ canView/storyId của frontend.
function withProject<T>(value: T, project: string): T {
  return JSON.parse(JSON.stringify(value).replaceAll('proj/', `${project}/`)) as T
}

function see(canView: ((id: string) => boolean) | undefined, project: string, story: string) {
  return !canView || canView(`${project}/${story}`)
}

export function mockLedger(project: string, canView?: (id: string) => boolean): ProjectLedger {
  const g = golden<ProjectLedger>('ledger.json')
  const stories = g.stories.filter((s) => see(canView, project, s.id.slice(s.id.indexOf('/') + 1)))
  return withProject({
    axes: g.axes,
    stories,
    candidate: see(canView, project, '02-chua-co-ten') ? g.candidate : null,
    channelMedianDay7: g.channelMedianDay7,
  }, project)
}

export function mockLedgerCheck(project: string, brief: string, canView?: (id: string) => boolean): LedgerCheckResult {
  // Trùng tổ hợp nằm ở me-ke-tiem-may; overuse trục 3 cần cả me-ke + em-gai
  // trong tầm nhìn → chọn golden ứng với thành phần truyện còn thấy, không tính
  // lại check nào. brief đã sửa (Fix một lần / brief mới) → golden fixed.
  let name = 'ledger-check.json'
  if (brief.includes('Chị Dâu') || brief === MOCK_NEW_STORY_BRIEF || brief.includes('Chủ tiệm')) name = 'ledger-check-fixed.json'
  else if (!see(canView, project, 'me-ke-tiem-may')) name = 'ledger-check-dup-hidden.json'
  return withProject(golden<LedgerCheckResult>(name), project)
}

export function mockLedgerSuggest(): LedgerSuggestion {
  return golden('ledger-suggest.json')
}

// ─── Vũ trụ chung ───────────────────────────────────────────────────────────

type StoryVerdicts = {
  story: string
  evaluated: boolean
  checks: UniverseCheckItem[]
  kept: UniverseKeptItem[]
}

// Mục §19 người dùng vừa "Bỏ giữ" ở phiên mock: về lại checks cho đến khi tải
// lại, giống xoá quyết định trong reviews/decisions.json.
const unkeptItems = new Set<string>()

export function mockUniverse(project: string, canView?: (id: string) => boolean, name = project): ProjectUniverse {
  const g = golden<ProjectUniverse>('universe.json')
  const verdicts = golden<{ stories: StoryVerdicts[] }>('universe-verdicts.json')
  const viewable = (story: string) => see(canView, project, story)
  // lastRole là vai trò ở truyện mới nhất còn thấy; stories[] chỉ mang id/title
  // nên mock giữ lastRole của golden — xấp xỉ đủ cho màn hình.
  const entities = g.entities
    .map((e): ProjectUniverseEntity => ({ ...e, stories: e.stories.filter((s) => viewable(s.id)) }))
    .filter((e) => e.stories.length >= 2)
  const shared = new Set(entities.flatMap((e) => e.stories.map((s) => s.id)))
  const checks: UniverseCheckItem[] = []
  const kept: UniverseKeptItem[] = []
  let evaluated = false
  for (const v of verdicts.stories) {
    if (!viewable(v.story) || !shared.has(v.story)) continue
    if (v.evaluated) evaluated = true
    checks.push(...v.checks)
    for (const k of v.kept) {
      if (unkeptItems.has(k.quoteId)) checks.push({ ref: k.ref, story: k.story, ch: k.ch, text: k.text, quoteId: k.quoteId })
      else kept.push(k)
    }
  }
  return {
    name,
    counts: { stories: shared.size, chars: entities.filter((e) => e.type === 'char').length, places: entities.filter((e) => e.type === 'place').length },
    entities,
    checks: evaluated ? checks : null,
    keptCount: kept.length,
    kept,
  }
}

export function mockUnkeep(_project: string, _story: string, itemId: string): boolean {
  const verdicts = golden<{ stories: StoryVerdicts[] }>('universe-verdicts.json')
  if (!verdicts.stories.some((v) => v.kept.some((k) => k.quoteId === itemId))) return false
  unkeptItems.add(itemId)
  return true
}

// Ô Tìm trong dự án: lọc thực thể của golden theo tên; trích đoạn là bảng tĩnh
// (mock không đọc chương thật).
const SEARCH_PASSAGES: { story: string; storyTitle: string; ch: number; quote: string }[] = [
  { story: '01-tra-xanh-cuop-kich-ban', storyTitle: 'Trà Xanh Cướp Kịch Bản', ch: 3, quote: 'Bản ghi âm gốc nằm trong túi áo bà Tư, tiếng của cả đoàn phim.' },
  { story: 'me-chong', storyTitle: 'Mẹ Chồng Đuổi Tôi Đi', ch: 5, quote: 'Ghi âm buổi tối hôm ấy vẫn còn trong điện thoại tôi.' },
  { story: 'dong-nghiep', storyTitle: 'Đồng Nghiệp Cướp Công', ch: 2, quote: 'Email gốc đính kèm bản ghi âm cuộc họp, ai cũng nghe được.' },
  { story: 'chong-cu-quan-pho', storyTitle: 'Chồng Cũ Hối Hận', ch: 4, quote: 'Tôi mở bản ghi âm giữa đám giỗ, cả nhà im bặt.' },
  { story: 'hidden-story', storyTitle: 'Truyện riêng không được xem', ch: 1, quote: 'Đoạn ghi âm riêng không ai được nghe.' },
]

export function mockProjectSearch(project: string, q: string, canView?: (id: string) => boolean): ProjectSearch {
  const fold = (v: string) => v.normalize('NFD').replace(/[\u0300-\u036f]/g, '').replace(/đ/g, 'd').replace(/Đ/g, 'D').toLowerCase()
  const needle = fold(q.trim())
  const entities = mockUniverse(project, canView).entities.filter((e) => fold(e.name).includes(needle))
  const passages = SEARCH_PASSAGES
    .filter((p) => fold(p.quote).includes(needle) && see(canView, project, p.story))
    .map((p) => {
      const at = fold(p.quote).indexOf(needle)
      return { ...p, hl: [[at, at + needle.length]] as [number, number][] }
    })
  return { entities, passages, total: entities.length + passages.length }
}
