// @vitest-environment happy-dom
// Banner học (LearnSlot): lần sửa thứ 3 trả `learn` → hiện banner → Lưu quy tắc → banner đã lưu có Hoàn tác;
// Không bỏ qua; Xem N lần sửa mở ví dụ. Danh sách Quy tắc đã học: công tắc, Xoá có xác nhận, ví dụ,
// trạng thái. Hàng Đề xuất của trợ lý trong Cài đặt thông báo ghi `suggestions.off`.
import { useState } from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { confirm, toast } from '@noverse/ui'
import { Switch } from '@noverse/ui'
import { LearnSlot } from './LearnSlot'
import { LearnedRulesList } from './LearnedRulesList'
import { SuggestionSettingsRow } from './SuggestionSettingsRow'
import { button, click, json, learnSuggestion, mount, rule, unmountAll, type Call } from '../suggestions/testFixtures'
import type { Suggestion } from '../../api/suggestions'

vi.mock('@noverse/ui', async (importOriginal) => ({ ...(await importOriginal<object>()), confirm: vi.fn(), toast: Object.assign(vi.fn(), { err: vi.fn(), ok: vi.fn() }) }))
vi.mock('../../data/status', () => ({ useOnline: () => true }))
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

let calls: Call[] = []
let reply: (call: Call) => Response
beforeEach(() => {
  calls = []
  vi.mocked(confirm).mockReset().mockResolvedValue(true)
  vi.mocked(toast).mockReset()
  vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const call = { url: String(input), method: init?.method ?? 'GET', body: init?.body ? JSON.parse(String(init.body)) : undefined }
    calls.push(call)
    return reply(call)
  }))
})
afterEach(async () => { await unmountAll(); vi.unstubAllGlobals() })

const posts = (suffix: string) => calls.filter((call) => call.method === 'POST' && call.url.endsWith(suffix))

describe('LearnSlot', () => {
  it('không có `learn` → không vẽ gì', async () => {
    reply = () => json({})
    const el = await mount(<LearnSlot ownerKey="test" learn={null} />)
    expect(el.textContent).toBe('')
  })

  it('lần sửa thứ 3 trả learn → banner → Lưu quy tắc → banner đã lưu + Hoàn tác gỡ quy tắc', async () => {
    reply = (call) => call.url.endsWith('/accept')
      ? json({ ok: true, summary: 'Đã lưu', ruleId: 'r1', undo: { op: 'remove_rule', params: { id: 'r1' } } })
      : json({ ok: true })
    // Phản hồi PATCH metadata của lần sửa thứ 3 mang `learn`.
    const Harness = () => {
      const [learn, setLearn] = useState<Suggestion | null>(null)
      return <>
        <button type="button" onClick={() => setLearn(learnSuggestion('l1') as unknown as Suggestion)}>sửa lần 3</button>
        <LearnSlot ownerKey="test" learn={learn} />
      </>
    }
    const el = await mount(<Harness />)
    expect(el.textContent).not.toContain('Đạo diễn nhận ra')
    await click(button(el, 'sửa lần 3'))
    expect(el.textContent).toContain('Đạo diễn nhận ra: lần thứ 3 bạn bỏ phần trong ngoặc ở cuối tiêu đề')
    await click(button(el, 'Lưu quy tắc'))
    expect(posts('/accept').map((call) => call.url)).toEqual(['/api/suggestions/l1/accept'])
    expect(el.textContent).toContain('Đã lưu vào Hồ sơ kênh › Quy tắc đã học')
    expect(el.textContent).not.toContain('Đạo diễn nhận ra')
    await click(button(el, 'Hoàn tác'))
    expect(calls.some((call) => call.url === '/api/undo' && JSON.stringify(call.body) === JSON.stringify({ op: 'remove_rule', params: { id: 'r1' } }))).toBe(true)
    expect(el.textContent).not.toContain('Đã lưu vào Hồ sơ kênh')
  })

  it('banner Hoạ sĩ lưu xong nêu tên prompt', async () => {
    reply = () => json({ ok: true, summary: 'Đã lưu', ruleId: 'r2', undo: { op: 'remove_rule', params: { id: 'r2' } } })
    const learn = learnSuggestion('l2', { agent: 'artist', banner: 'Hoạ sĩ nhận ra: góp ý lần thứ 2.', primary: { label: 'Thêm vào Ghi chú đã học', op: 'add_rule', params: { promptName: 'Đêm xanh rêu', learnedFrom: { examples: [] } } } }) as unknown as Suggestion
    const el = await mount(<LearnSlot ownerKey="test" learn={learn} />)
    await click(button(el, 'Thêm vào Ghi chú đã học'))
    expect(el.textContent).toContain('Thư viện › Prompt ảnh › Đêm xanh rêu › Ghi chú đã học')
  })

  it('Không → dismiss rồi banner biến mất, không toast', async () => {
    reply = () => json({ ok: true })
    const el = await mount(<LearnSlot ownerKey="test" learn={learnSuggestion('l3') as unknown as Suggestion} />)
    await click(button(el, 'Không'))
    expect(posts('/dismiss')).toHaveLength(1)
    expect(el.textContent).toBe('')
    expect(toast).not.toHaveBeenCalled()
  })

  it('Xem 3 lần sửa mở danh sách Trước/Sau tại chỗ', async () => {
    reply = () => json({ ok: true })
    const el = await mount(<LearnSlot ownerKey="test" learn={learnSuggestion('l4') as unknown as Suggestion} />)
    expect(el.querySelector('li')).toBeNull()
    await click(button(el, 'Xem 3 lần sửa'))
    expect([...el.querySelectorAll('li')].map((li) => li.textContent)).toEqual(['TrướcA (1)SauA', 'TrướcB (2)SauB', 'TrướcC (3)SauC'])
  })

  it('lưu lỗi → báo ngay trong banner, banner còn để thử lại', async () => {
    reply = () => json({ error: 'đã đủ 20 quy tắc', code: 'limit' }, 409)
    const el = await mount(<LearnSlot ownerKey="test" learn={learnSuggestion('l5') as unknown as Suggestion} />)
    await click(button(el, 'Lưu quy tắc'))
    expect(el.textContent).toContain('Đạo diễn nhận ra')
    expect(el.querySelector('[role="alert"]')).toBeTruthy()
  })

  it('giữ Hoàn tác cho hành động vừa lưu khi đề xuất kế tiếp đến, rồi mới hiện đề xuất mới sau hoàn tác', async () => {
    reply = call => call.url.endsWith('/accept')
      ? json({ ok: true, summary: 'Đã lưu', ruleId: 'r-saved', undo: { op: 'remove_rule', params: { id: 'r-saved' } } })
      : json({ ok: true })
    const Harness = () => {
      const [learn, setLearn] = useState<Suggestion | null>(learnSuggestion('first') as unknown as Suggestion)
      return <><button onClick={() => setLearn(null)}>máy hết đề xuất</button><button onClick={() => setLearn(learnSuggestion('next', { banner: 'Đề xuất tiếp theo' }) as unknown as Suggestion)}>máy gửi tiếp</button><LearnSlot ownerKey="job-one" learn={learn} /></>
    }
    const el = await mount(<Harness />)
    await click(button(el, 'Lưu quy tắc'))
    await click(button(el, 'máy hết đề xuất'))
    expect(el.textContent).toContain('Hoàn tác')
    await click(button(el, 'máy gửi tiếp'))
    expect(el.textContent).toContain('Đã lưu vào Hồ sơ kênh')
    expect(el.textContent).not.toContain('Đề xuất tiếp theo')
    await click(button(el, 'Hoàn tác'))
    expect(posts('/api/undo').map(call => call.body)).toEqual([{ op: 'remove_rule', params: { id: 'r-saved' } }])
    expect(el.textContent).toContain('Đề xuất tiếp theo')
  })

  it('không mang xác nhận hoặc Hoàn tác của video/kênh trước sang chủ sở hữu mới', async () => {
    reply = () => json({ ok: true, summary: 'Đã lưu', ruleId: 'r-old', undo: { op: 'remove_rule', params: { id: 'r-old' } } })
    const Harness = () => {
      const [other, setOther] = useState(false)
      const learn = learnSuggestion(other ? 'other' : 'original', { channelId: other ? 'channel-two' : 'channel-one', banner: other ? 'Quy tắc của kênh mới' : 'Quy tắc của kênh cũ' }) as unknown as Suggestion
      return <><button onClick={() => setOther(true)}>đổi video</button><LearnSlot ownerKey={other ? 'channel-two:job-two' : 'channel-one:job-one'} learn={learn} /></>
    }
    const el = await mount(<Harness />)
    await click(button(el, 'Lưu quy tắc'))
    await click(button(el, 'đổi video'))
    expect(el.textContent).not.toContain('Đã lưu vào Hồ sơ kênh')
    expect(el.textContent).not.toContain('Hoàn tác')
    expect(el.textContent).toContain('Quy tắc của kênh mới')
  })
})

describe('LearnedRulesList', () => {
  const list = (rules: unknown[]) => json({ rules, limit: 20 })

  it('hiện quy tắc, nguồn, đếm n / 20; công tắc PATCH enabled', async () => {
    let rules = [rule('r1'), rule('r2', { enabled: false, source: 'Từ số liệu 8 video', learnedFrom: { kind: 'perf', count: 8, from: 'a', to: 'b', examples: [] } })]
    reply = (call) => {
      if (call.method === 'PATCH') { rules = rules.map((item) => (item.id === 'r1' ? { ...item, enabled: false } : item)); return json(rules[0]) }
      return list(rules)
    }
    const el = await mount(<LearnedRulesList scope={{ channelId: 'c-toggle' }} />)
    expect(el.textContent).toContain('Quy tắc đã học')
    expect(el.textContent).toContain('2 / 20')
    expect(el.textContent).toContain('Học từ 3 lần bạn sửa · 26–28/10')
    expect(el.textContent).toContain('Từ số liệu 8 video · đang tắt')
    const toggles = [...el.querySelectorAll('[role="switch"]')]
    expect(toggles.map((node) => node.getAttribute('aria-checked'))).toEqual(['true', 'false'])
    await click(toggles[0])
    expect(calls.find((call) => call.method === 'PATCH')).toMatchObject({ url: '/api/learned-rules/r1', body: { enabled: false } })
  })

  it('Xoá hỏi xác nhận (đỏ); Huỷ không xoá, Xoá thì DELETE', async () => {
    reply = (call) => (call.method === 'DELETE' ? json({ ok: true }) : list([rule('r1', { text: 'Bỏ ngoặc' })]))
    const el = await mount(<LearnedRulesList scope={{ channelId: 'c-delete' }} />)
    vi.mocked(confirm).mockResolvedValueOnce(false)
    await click(el.querySelector('[aria-label="Xoá quy tắc Bỏ ngoặc"]'))
    expect(calls.some((call) => call.method === 'DELETE')).toBe(false)
    await click(el.querySelector('[aria-label="Xoá quy tắc Bỏ ngoặc"]'))
    expect(vi.mocked(confirm).mock.calls[1][0]).toMatchObject({ tone: 'danger', title: 'Xoá quy tắc “Bỏ ngoặc”?' })
    expect(calls.filter((call) => call.method === 'DELETE').map((call) => call.url)).toEqual(['/api/learned-rules/r1'])
  })

  it('Xem ví dụ mở Trước/Sau; quy tắc không có ví dụ không có nút', async () => {
    reply = () => list([rule('r1'), rule('r2', { learnedFrom: { kind: 'perf', count: 8, from: 'a', to: 'b', examples: [] } })])
    const el = await mount(<LearnedRulesList scope={{ channelId: 'c-example' }} />)
    expect([...el.querySelectorAll('button')].filter((node) => node.textContent === 'Xem ví dụ')).toHaveLength(1)
    await click(button(el, 'Xem ví dụ'))
    expect(el.textContent).toContain('Ga tàu (Phần 2)')
    expect(button(el, 'Ẩn ví dụ')).toBeTruthy()
  })

  it('trống / lỗi / đang tải', async () => {
    reply = () => list([])
    const empty = await mount(<LearnedRulesList scope={{ channelId: 'c-empty' }} />)
    expect(empty.textContent).toContain('Chưa có quy tắc nào')
    reply = () => json({ error: 'lỗi' }, 500)
    const bad = await mount(<LearnedRulesList scope={{ channelId: 'c-bad' }} />)
    expect(bad.textContent).toContain('Không tải được quy tắc đã học')
    expect(button(bad, 'Thử lại')).toBeTruthy()
  })

  it('Ghi chú đã học: cùng danh sách, tiêu đề và chữ riêng của prompt ảnh', async () => {
    reply = () => list([rule('n1', { scope: { imagePromptId: 'p1' }, agent: 'artist', text: 'Không chữ trong ảnh' })])
    const el = await mount(<LearnedRulesList scope={{ imagePromptId: 'p1' }} title="Ghi chú đã học" help="Hoạ sĩ học từ góp ý vẽ lại." empty="Chưa có ghi chú nào." errorText="Không tải được ghi chú đã học." />)
    expect(el.textContent).toContain('Ghi chú đã học')
    expect(el.textContent).toContain('Không chữ trong ảnh')
    expect(calls[0].url).toBe('/api/learned-rules?imagePromptId=p1')
  })
})

describe('SuggestionSettingsRow', () => {
  type Off = { channelId: string | null; kind: 'hot_video' | 'cleanup' | 'pace' | 'template_failing' | 'learn_rule' | 'raise_autonomy' | 'perf_insight' }
  const Harness = ({ initial, spy }: { initial: Off[]; spy: (next: Off[]) => void }) => {
    const [off, setOff] = useState(initial)
    return <SuggestionSettingsRow off={off} channelNames={{ c1: 'Midnight Tales' }} offline={false}
      renderSwitch={(label, checked, onChange) => <Switch label={label} checked={checked} onChange={onChange} />}
      onChange={(next) => { setOff(next); spy(next) }} />
  }

  it('Chọn loại mở 7 loại; gạt một loại tắt ghi {channelId:null, kind} vào suggestions.off', async () => {
    const spy = vi.fn()
    const el = await mount(<Harness initial={[]} spy={spy} />)
    expect(el.textContent).toContain('Bật 7/7 loại')
    await click(button(el, 'Chọn loại'))
    const cleanup = el.querySelector<HTMLElement>('[aria-label="Đề xuất “Dọn MP4 đã đăng”"]')
    expect(cleanup).toBeTruthy()
    await click(cleanup)
    expect(spy).toHaveBeenLastCalledWith([{ channelId: null, kind: 'cleanup' }])
    expect(el.textContent).toContain('Bật 6/7 loại')
    await click(el.querySelector('[aria-label="Đề xuất “Dọn MP4 đã đăng”"]'))
    expect(spy).toHaveBeenLastCalledWith([])
  })

  it('loại tắt riêng cho kênh có "Bật lại"; hàng chỉ có chữ và nút Chọn loại (không công tắc tổng)', async () => {
    const spy = vi.fn()
    const el = await mount(<Harness initial={[{ channelId: 'c1', kind: 'pace' }]} spy={spy} />)
    expect(el.querySelector('[aria-label="Đề xuất của trợ lý"]')).toBeNull()
    await click(button(el, 'Chọn loại'))
    expect(el.textContent).toContain('“Đổi nhịp đăng” ở Midnight Tales')
    await click(button(el, 'Bật lại'))
    expect(spy.mock.calls.at(-1)![0]).toEqual([])
  })
})
