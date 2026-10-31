import { describe, expect, it } from 'vitest'
import { displayValue, fieldChildren, fieldValue, looseText, parseNodes, parseStream } from './streamRounds'

// Chuỗi mẫu lấy từ internal/host/stream_extract_test.go (định dạng thật của host; header Việt).
const EDIT = [
  '✻ Gọt giũa',
  'chapter: 4',
  'old_string: Kiều My đặt đũa xuống trước tiên.',
  'Cô nhận ra người vắng mặt.',
  'new_string: Kiều My đặt đũa xuống trước tiên, rồi nhìn quanh.',
  'replace_all: false',
].join('\n')

const COMMIT = [
  '✻ Nộp chương',
  'chapter: 4',
  'hook_type: crisis',
  'dominant_strand: quest',
  'feedback: null',
  'characters:',
  '  - Kiều My',
  '  - Trần Nga',
  'foreshadow_updates:',
  '  - id: fs-1',
  '    action: plant',
  '    description: Phòng quy hoạch ghi nhiều hơn mức cần.',
  '  - id: fs-2',
  '    action: resolve',
  '    description: null',
  'key_events:',
  '  - Kiều My bác thẻ nhường.',
].join('\n')

describe('parseStream — round ✻ Gọt giũa → diff', () => {
  it('dịch nhãn, giữ giá trị nhiều dòng và cờ replace_all', () => {
    const round = parseStream(EDIT, true).rounds[0]
    expect(round).toMatchObject({ labelVi: 'Gọt giũa', tool: 'edit_chapter', kind: 'diff', live: true, defaultOpen: true })
    expect(fieldValue(round.nodes, 'chapter')).toBe('4')
    expect(fieldValue(round.nodes, 'old_string')).toBe('Kiều My đặt đũa xuống trước tiên.\nCô nhận ra người vắng mặt.')
    expect(fieldValue(round.nodes, 'new_string')).toBe('Kiều My đặt đũa xuống trước tiên, rồi nhìn quanh.')
    expect(fieldValue(round.nodes, 'replace_all')).toBe('false')
  })
})

describe('parseStream — round ✻ Nộp chương → thẻ phục bút', () => {
  it('parse mảng primitive và mảng object lồng nhau', () => {
    const round = parseStream(COMMIT, true).rounds[0]
    expect(round).toMatchObject({ labelVi: 'Lưu chương', tool: 'commit_chapter', kind: 'commit', defaultOpen: true })
    expect(fieldChildren(round.nodes, 'characters').map((node) => node.value)).toEqual(['Kiều My', 'Trần Nga'])
    const updates = fieldChildren(round.nodes, 'foreshadow_updates')
    expect(updates).toHaveLength(2)
    expect(updates[0]).toMatchObject({ key: 'id', value: 'fs-1', item: true })
    expect(fieldValue(updates[0].children, 'action')).toBe('plant')
    expect(fieldValue(updates[0].children, 'description')).toBe('Phòng quy hoạch ghi nhiều hơn mức cần.')
    expect(displayValue(fieldValue(updates[1].children, 'description'))).toBe('—')
    expect(fieldChildren(round.nodes, 'key_events').map((node) => node.value)).toEqual(['Kiều My bác thẻ nhường.'])
    expect(displayValue(fieldValue(round.nodes, 'feedback'))).toBe('—')
  })
})

// streamText/sự kiện lưu từ bản cũ còn header tiếng Trung — phải ra cùng nhãn/tool/kind.
describe('parseStream — header ✻ tiếng Trung cũ', () => {
  it('打磨 / 章节提交 / 查询上下文 parse như header Việt', () => {
    const legacy = parseStream([EDIT.replace('✻ Gọt giũa', '✻ 打磨'), COMMIT.replace('✻ Nộp chương', '✻ 章节提交'), '✻ 查询上下文\nchapter: 6'].join('\n'), false)
    expect(legacy.rounds.map((round) => [round.labelVi, round.tool, round.kind])).toEqual([
      ['Gọt giũa', 'edit_chapter', 'diff'],
      ['Lưu chương', 'commit_chapter', 'commit'],
      ['Đọc bối cảnh', 'novel_context', 'kv'],
    ])
    expect(fieldValue(legacy.rounds[0].nodes, 'old_string')).toBe('Kiều My đặt đũa xuống trước tiên.\nCô nhận ra người vắng mặt.')
  })
})

describe('parseStream — văn xuôi & suy nghĩ', () => {
  const LIVE = ['Bữa trưa hôm ấy bàn dài đặt giữa phòng ăn.', '\x02Đang cân nhắc nhịp chương 4\x02', '', '✻ Đọc bối cảnh', 'chapter: 6'].join('\n')

  it('tách \x02 thành khối suy nghĩ và giữ văn xuôi ngoài round', () => {
    const parsed = parseStream(LIVE, true)
    expect(parsed.prose).toBe('Bữa trưa hôm ấy bàn dài đặt giữa phòng ăn.')
    expect(parsed.thinking).toBe('Đang cân nhắc nhịp chương 4')
    expect(parsed.blocks.map((block) => block.kind)).toEqual(['prose', 'thinking'])
    expect(parsed.rounds).toHaveLength(1)
    expect(parsed.rounds[0]).toMatchObject({ labelVi: 'Đọc bối cảnh', tool: 'novel_context', kind: 'kv' })
  })

  it('round cuối chỉ live khi agent đang chạy', () => {
    expect(parseStream(LIVE, true).liveTool).toBe('novel_context')
    const stopped = parseStream(LIVE, false)
    expect(stopped.liveTool).toBe('')
    expect(stopped.rounds[0].live).toBe(false)
    expect(stopped.rounds[0].defaultOpen).toBe(false)
  })
})

describe('parseStream — giới hạn & định dạng lạ', () => {
  it('giữ toàn bộ round thay vì cắt mất lịch sử sau vòng 32', () => {
    const many = Array.from({ length: 40 }, (_, i) => `✻ Đọc chương\nchapter: ${i + 1}`).join('\n')
    const rounds = parseStream(many, true).rounds
    expect(rounds).toHaveLength(40)
    expect(fieldValue(rounds[0].nodes, 'chapter')).toBe('1')
    expect(rounds[39].live).toBe(true)
  })

  it('header lạ → nhãn nguyên văn, kind raw', () => {
    const round = parseStream('✻ abc_tool\nfoo: bar', false).rounds[0]
    expect(round).toMatchObject({ labelVi: 'abc tool', tool: 'abc_tool', kind: 'raw' })
    expect(fieldValue(round.nodes, 'foo')).toBe('bar')
  })

  it('chuẩn hoá CRLF và chuỗi rỗng', () => {
    expect(parseStream('', true)).toMatchObject({ rounds: [], prose: '', thinking: '', liveTool: '' })
    const round = parseStream('✻ Gọt giũa\r\nchapter: 4\r\n', false).rounds[0]
    expect(fieldValue(round.nodes, 'chapter')).toBe('4')
  })

  it('parseNodes: object rỗng + phần tử mảng có key inline', () => {
    const nodes = parseNodes('cast_intros:\n  - name: Trần Nga\n    brief_role: giữ chìa khoá')
    expect(nodes).toHaveLength(1)
    expect(nodes[0].children[0]).toMatchObject({ key: 'name', value: 'Trần Nga', item: true })
    expect(fieldValue(nodes[0].children[0].children, 'brief_role')).toBe('giữ chìa khoá')
  })

  it('đoạn văn sau YAML là văn xuôi của round, không nối vào field cuối', () => {
    const text = ['✻ Đọc chương', 'chapter: 5', 'character: null', 'Bữa trưa hôm ấy bàn dài.', ' Quách ngồi đầu bàn.'].join('\n')
    const round = parseStream(text, true).rounds[0]
    expect(fieldValue(round.nodes, 'character')).toBe(null)
    expect(looseText(round.nodes)).toBe('Bữa trưa hôm ấy bàn dài.\n\nQuách ngồi đầu bàn.')
    expect(fieldValue(round.nodes, 'chapter')).toBe('5')
  })

  it('items giữ văn xuôi đầu luồng và toàn bộ vòng đã xong', () => {
    const many = Array.from({ length: 40 }, (_, i) => `✻ Đọc chương\nchapter: ${i + 1}`).join('\n')
    const parsed = parseStream(`Mở đầu.\n${many}`, true)
    expect(parsed.items[0]).toEqual({ kind: 'prose', text: 'Mở đầu.' })
    expect(parsed.items.filter((item) => item.kind === 'round')).toHaveLength(40)
    expect(parsed.items[1].kind).toBe('round')
  })

  it('ranh giới round giữ suy nghĩ, bước cũ và đoạn văn sau cùng', () => {
    const boundary = '\n\x1e\n'
    const parsed = parseStream(`\x02Cân nhắc\x02${boundary}✻ Đọc chương\nchapter: 1${boundary}Đã đọc đầy đủ.`, false)
    expect(parsed.thinking).toBe('Cân nhắc')
    expect(parsed.rounds).toHaveLength(1)
    expect(parsed.items.at(-1)).toEqual({ kind: 'prose', text: 'Đã đọc đầy đủ.' })
  })
})
