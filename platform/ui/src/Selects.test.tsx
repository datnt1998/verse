// @vitest-environment happy-dom
import { act, type ReactNode } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { renderToStaticMarkup } from 'react-dom/server'
import { afterEach, describe, expect, it } from 'vitest'
import * as stylex from '@stylexjs/stylex'
import { Combobox, KitSelect, MultiCombobox, RichSelect, filterOptionGroups, toggleMultiValue, typeAheadIndex, type Option } from './Selects'

// Vitest extracts StyleX CSS without attaching it to happy-dom. Compare the
// compiled atomic classes against semantic contracts, not hashed class names.
const styleContract = stylex.create({
  shrinkable: { minWidth: 0 },
  ellipsis: { overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' },
  arrow: { flexShrink: 0 },
})

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
const roots: Root[] = []
afterEach(async () => { for (const root of roots.splice(0)) await act(() => root.unmount()); document.body.replaceChildren() })

async function mount(node: ReactNode) {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  roots.push(root)
  await act(async () => { root.render(node) })
  return container
}

/** Bấm/gõ như người dùng: sự kiện nổi bọt tới vùng bàn phím của KitSelect. */
async function press(target: Element, key: string) {
  await act(async () => { target.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true })) })
}

const options: readonly Option[] = [
  { value: 'viet', label: 'Người viết cân bằng', group: 'Trong nước', meta: '400K · cân bằng' },
  { value: 'outline', label: 'Đại cương nhanh', group: 'Trong nước', meta: '200K · nhanh' },
  { value: 'voice', label: 'Giọng đọc dịu dàng', group: 'Quốc tế' },
  { value: 'balanced', label: 'Người viết nâng cao', group: 'Quốc tế' },
]

describe('filterOptionGroups', () => {
  it('lọc chữ Việt không dấu, không phân biệt hoa thường và khoảng trắng ngoài', () => {
    const groups = filterOptionGroups(options, '  NGUOI VIET  ')
    expect(groups.map((group) => [group.label, group.options.map((option) => option.value)])).toEqual([
      ['Trong nước', ['viet']],
      ['Quốc tế', ['balanced']],
    ])
  })

  it('coi đ là d và giữ nguyên nhãn có dấu cho phần tô sáng', () => {
    const groups = filterOptionGroups(options, 'dai cuong')
    expect(groups).toEqual([{ label: 'Trong nước', options: [options[1]] }])
    expect(groups[0].options[0]).toBe(options[1])
    expect(groups[0].options[0].label).toBe('Đại cương nhanh')
  })

  it('hỗ trợ cả dấu Unicode tách rời', () => {
    const decomposed: Option = { value: 'decomposed', label: 'Đại cương'.normalize('NFD') }
    expect(filterOptionGroups([decomposed], 'đẠI CƯƠNG')[0].options).toEqual([decomposed])
  })

  it('đếm theo kết quả đang lọc, bỏ nhóm rỗng và giữ đúng thứ tự nhóm', () => {
    const source: Option[] = [
      { value: 'a', label: 'Model một', group: 'A' },
      { value: 'b', label: 'Không khớp', group: 'B' },
      { value: 'c', label: 'Model hai', group: 'A' },
      { value: 'd', label: 'Model ba', group: 'C' },
    ]
    expect(filterOptionGroups(source, 'model').map((group) => ({ label: group.label, count: group.options.length }))).toEqual([
      { label: 'A', count: 2 },
      { label: 'C', count: 1 },
    ])
  })

  it('gom nhóm rời nhau trong nguồn mà vẫn giữ thứ tự lựa chọn trong nhóm', () => {
    const source = [options[0], options[2], options[1], options[3]]
    expect(filterOptionGroups(source, '').map((group) => group.options.map((option) => option.value))).toEqual([
      ['viet', 'outline'], ['voice', 'balanced'],
    ])
    expect(source).toEqual([options[0], options[2], options[1], options[3]])
  })

  it('giữ lựa chọn không có nhóm và lựa chọn vô hiệu trong danh sách', () => {
    const loose: Option = { value: 'loose', label: 'Không nhóm', disabled: true }
    expect(filterOptionGroups([loose], '')).toEqual([{ label: '', options: [loose] }])
  })

  it('trả danh sách rỗng khi không khớp, không lọc bằng mã hoặc meta ẩn', () => {
    expect(filterOptionGroups(options, 'không tồn tại')).toEqual([])
    expect(filterOptionGroups(options, '400K')).toEqual([])
    expect(filterOptionGroups([], '')).toEqual([])
  })
})

describe('toggleMultiValue', () => {
  const option = (value: string): Option => ({ value, label: value })

  it('cho phép thêm mục thứ năm nhưng không thêm mục thứ sáu theo mặc định', () => {
    const four = ['a', 'b', 'c', 'd']
    const five = toggleMultiValue(four, option('e'))
    expect(five).toEqual(['a', 'b', 'c', 'd', 'e'])
    expect(toggleMultiValue(five, option('f'))).toEqual(five)
    expect(four).toEqual(['a', 'b', 'c', 'd'])
  })

  it('dùng giới hạn cấu hình thay vì luôn khoá ở năm', () => {
    expect(toggleMultiValue(['a'], option('b'), 2)).toEqual(['a', 'b'])
    expect(toggleMultiValue(['a', 'b'], option('c'), 2)).toEqual(['a', 'b'])
    expect(toggleMultiValue(['a', 'b', 'c', 'd', 'e'], option('f'), 6)).toHaveLength(6)
    expect(toggleMultiValue([], option('a'), 0)).toEqual([])
  })

  it('vẫn bỏ được mục đã chọn khi đầy hoặc khi giới hạn vừa giảm', () => {
    const selected = ['a', 'b', 'c', 'd', 'e']
    expect(toggleMultiValue(selected, option('c'))).toEqual(['a', 'b', 'd', 'e'])
    expect(toggleMultiValue(selected, option('a'), 2)).toEqual(['b', 'c', 'd', 'e'])
  })

  it('không tạo bản sao mục đã chọn và không đổi thứ tự mục còn lại', () => {
    expect(toggleMultiValue(['a', 'b', 'c'], option('b'))).toEqual(['a', 'c'])
    expect(toggleMultiValue(['a', 'c'], option('b'))).toEqual(['a', 'c', 'b'])
  })

  it('không thay đổi lựa chọn vô hiệu, kể cả khi đang được chọn', () => {
    expect(toggleMultiValue(['a'], { ...option('b'), disabled: true })).toEqual(['a'])
    expect(toggleMultiValue(['a'], { ...option('a'), disabled: true })).toEqual(['a'])
  })
})

describe('typeAheadIndex', () => {
  const list: Option[] = [{ value: 'a', label: 'Ngôn tình' }, { value: 'b', label: 'Ngọc Lan', disabled: true }, { value: 'c', label: 'Nam' }, { value: 'd', label: 'Đại cương' }, { value: 'e', label: 'Ngọc Hân' }]

  it('nhảy tới mục bắt đầu bằng chữ gõ, bỏ dấu và bỏ qua mục vô hiệu', () => {
    expect(typeAheadIndex(list, 0, 'n')).toBe(2)
    expect(typeAheadIndex(list, -1, 'ngoc')).toBe(4)
    expect(typeAheadIndex(list, 0, 'dai')).toBe(3)
  })

  it('gõ lặp một chữ thì xoay vòng, gõ cụm thì ở lại mục đang sáng nếu còn khớp', () => {
    expect(typeAheadIndex(list, 2, 'nn')).toBe(4)
    expect(typeAheadIndex(list, 4, 'nn')).toBe(0)
    expect(typeAheadIndex(list, 0, 'ngo')).toBe(0)
  })

  it('trả -1 khi không khớp hoặc chuỗi rỗng', () => {
    expect(typeAheadIndex(list, 0, 'z')).toBe(-1)
    expect(typeAheadIndex(list, 0, '')).toBe(-1)
    expect(typeAheadIndex([], 0, 'a')).toBe(-1)
  })
})

describe('giao diện select được điều khiển', () => {
  it('nhãn dài cắt dấu ba chấm, không ép co mũi tên và giữ title đầy đủ', async () => {
    const label = 'Tên lựa chọn rất dài cần cắt bằng dấu ba chấm'
    const container = await mount(<KitSelect value="long" options={[{ value: 'long', label }]} onChange={() => {}} />)
    const trigger = container.querySelector<HTMLButtonElement>('[role="combobox"]')!
    const value = trigger.firstElementChild!.firstElementChild!
    expect(trigger.title).toBe(label)
    for (const token of stylex.props(styleContract.shrinkable).className!.split(' ')) expect(trigger.firstElementChild!.classList.contains(token)).toBe(true)
    for (const token of stylex.props(styleContract.ellipsis).className!.split(' ')) expect(value.classList.contains(token)).toBe(true)
    for (const token of stylex.props(styleContract.arrow).className!.split(' ')) expect(trigger.lastElementChild!.classList.contains(token)).toBe(true)
  })
  it('KitSelect liên kết nhãn, lỗi và giá trị biểu mẫu mà không đổi native Select', () => {
    const html = renderToStaticMarkup(<KitSelect id="language" name="language" label="Ngôn ngữ" value="vi" options={[{ value: 'vi', label: 'Tiếng Việt', code: 'vi' }]} onChange={() => {}} error="Hãy chọn ngôn ngữ khác" />)
    expect(html).toContain('for="language"')
    expect(html).toContain('role="combobox"')
    expect(html).toContain('aria-expanded="false"')
    expect(html).toContain('aria-invalid="true"')
    expect(html).toContain('aria-describedby="language-error"')
    expect(html).toContain('type="hidden" name="language" value="vi"')
    expect(html).toContain('Tiếng Việt')
    expect(html).toContain('title="Tiếng Việt"')
    expect(html).toContain('Hãy chọn ngôn ngữ khác')
    const empty = renderToStaticMarkup(<KitSelect aria-label="Model" value="" options={[]} placeholder="Nhãn đầy đủ rất dài" onChange={() => {}} />)
    expect(empty).toContain('title="Nhãn đầy đủ rất dài"')
  })

  it('RichSelect hiển thị ảnh và mô tả lựa chọn được điều khiển', () => {
    const html = renderToStaticMarkup(<RichSelect label="Giọng người kể" value="voice" options={[{ value: 'voice', label: 'Mai', avatar: <img src="/mai.png" alt="" />, description: 'Miền Bắc · ấm áp' }]} onChange={() => {}} />)
    expect(html).toContain('src="/mai.png"')
    expect(html).toContain('Miền Bắc · ấm áp')
    expect(html).toContain('height:48px')
  })

  it('Combobox không làm mất lựa chọn cũ khi nguồn đang lỗi hoặc đang tải', () => {
    const html = renderToStaticMarkup(<Combobox label="Model" value="saved-model" options={[]} loading loadError="Nhà cung cấp không phản hồi." onChange={() => {}} />)
    expect(html).toContain('value="saved-model"')
    expect(html).toContain('aria-autocomplete="list"')
  })

  it('Combobox inlineDescription giữ tên model đầy đủ + mô tả phụ trên nút đóng, chỉ mở ô tìm sau khi bấm', async () => {
    const container = await mount(<Combobox aria-label="Model" inlineDescription value="openai/gpt-5.2" options={[{ value: 'openai/gpt-5.2', label: 'openai/gpt-5.2', description: 'OpenAI · khoá API' }]} onChange={() => {}} />)
    const trigger = container.querySelector<HTMLButtonElement>('[role="combobox"]')!
    expect(trigger.tagName).toBe('BUTTON')
    expect(trigger.textContent).toContain('openai/gpt-5.2')
    expect(trigger.textContent).toContain('OpenAI · khoá API')
    expect(container.querySelector('input[role="combobox"]')).toBeNull()
    await act(async () => { trigger.click() })
    const search = container.querySelector<HTMLInputElement>('input[role="combobox"]')
    expect(search).toBeTruthy()
    expect(document.activeElement).toBe(search)
  })

  it('Combobox inlineDescription đóng khi focus chuyển ra ngoài mà không giành lại focus', async () => {
    const container = await mount(<>
      <Combobox aria-label="Model" inlineDescription value="alpha" options={[{ value: 'alpha', label: 'Alpha', description: 'Provider' }]} onChange={() => {}} />
      <button id="next-control" type="button">Tiếp theo</button>
    </>)
    const trigger = container.querySelector<HTMLButtonElement>('[role="combobox"]')!
    await act(async () => { trigger.click() })
    const next = container.querySelector<HTMLButtonElement>('#next-control')!
    await act(async () => { next.focus() })
    expect(container.querySelector('input[role="combobox"]')).toBeNull()
    expect(document.activeElement).toBe(next)
    expect(container.querySelector('[role="combobox"]')?.getAttribute('aria-expanded')).toBe('false')
  })

  it('MultiCombobox hiển thị chip, nút bỏ và số lượng theo giới hạn cấu hình', () => {
    const html = renderToStaticMarkup(<MultiCombobox id="genres" name="genres" label="Thể loại" value={['viet', 'outline']} options={options} limit={2} onChange={() => {}} />)
    expect(html).toContain('2/2')
    expect(html).toContain('aria-label="Remove tag Người viết cân bằng"')
    expect(html).toContain('aria-label="Remove tag Đại cương nhanh"')
    expect(html).toContain('aria-describedby="genres-count"')
    expect(html).toContain('type="hidden" name="genres" value="viet"')
    expect(html).toContain('type="hidden" name="genres" value="outline"')
  })
})

const plain: Option[] = [
  { value: 'a', label: 'Alpha' },
  { value: 'b', label: 'Beta' },
  { value: 'c', label: 'Gamma' },
]

describe('KitSelect: bàn phím và chọn', () => {
  it('mở bằng ArrowDown, đi bằng ArrowDown/Home/End, chọn bằng Enter, Esc đóng và trả focus', async () => {
    const picked: string[] = []
    await mount(<KitSelect aria-label="Model" value="a" options={plain} onChange={(value) => picked.push(value)} />)
    const trigger = document.querySelector<HTMLButtonElement>('[role="combobox"]')!
    trigger.focus()
    await press(trigger, 'ArrowDown')
    expect(trigger.getAttribute('aria-expanded')).toBe('true')
    expect(trigger.getAttribute('aria-activedescendant')).toMatch(/-option-0$/)
    await press(trigger, 'ArrowDown')
    expect(trigger.getAttribute('aria-activedescendant')).toMatch(/-option-1$/)
    await press(trigger, 'End')
    expect(trigger.getAttribute('aria-activedescendant')).toMatch(/-option-2$/)
    await press(trigger, 'Home')
    expect(trigger.getAttribute('aria-activedescendant')).toMatch(/-option-0$/)
    await press(trigger, 'ArrowDown')
    await press(trigger, 'Enter')
    expect(picked).toEqual(['b'])
    expect(trigger.getAttribute('aria-expanded')).toBe('false')
    expect(document.activeElement).toBe(trigger)
    await press(trigger, 'ArrowDown')
    await press(trigger, 'Escape')
    expect(trigger.getAttribute('aria-expanded')).toBe('false')
    expect(document.activeElement).toBe(trigger)
  })

  it('bỏ qua mục bị khoá và gõ chữ để nhảy tới mục khớp', async () => {
    const options: Option[] = [plain[0], { ...plain[1], disabled: true }, plain[2]]
    await mount(<KitSelect aria-label="Model" value="a" options={options} onChange={() => {}} />)
    const trigger = document.querySelector<HTMLButtonElement>('[role="combobox"]')!
    trigger.focus()
    await press(trigger, 'ArrowDown')
    await press(trigger, 'ArrowDown')
    expect(trigger.getAttribute('aria-activedescendant')).toMatch(/-option-2$/)
    await press(trigger, 'Escape')
    await press(trigger, 'g')
    expect(trigger.getAttribute('aria-expanded')).toBe('true')
    expect(trigger.getAttribute('aria-activedescendant')).toMatch(/-option-2$/)
  })

  it('bấm một dòng chọn giá trị, đóng danh sách và giữ focus ở ô chọn', async () => {
    const picked: string[] = []
    await mount(<KitSelect aria-label="Model" value="a" options={plain} onChange={(value) => picked.push(value)} />)
    const trigger = document.querySelector<HTMLButtonElement>('[role="combobox"]')!
    await act(async () => { trigger.click() })
    const rows = [...document.querySelectorAll<HTMLElement>('[role="option"]')]
    expect(rows).toHaveLength(3)
    await act(async () => { rows[2].click() })
    expect(picked).toEqual(['c'])
    expect(trigger.getAttribute('aria-expanded')).toBe('false')
    expect(document.activeElement).toBe(trigger)
  })
})
