import { describe, expect, it } from 'vitest'
import type { ConfigResponse, PackPreset } from '../../api/types'
import { appliedRows } from './applied'

const PACKS: PackPreset[] = [{ id: 'nu-tan-bao-thu', ten: 'Nữ tần trọng sinh báo thù, vả mặt', so_chuong: 8, nguoi_nghe: 'Nữ 18–35' }]

const config = (style: string): ConfigResponse => ({
  target: { scope: 'story', config: { style } },
  effective: { style },
  sources: { style: 'story' },
} as unknown as ConfigResponse)

describe('D2 › cột phải “Văn phong” khi đang dùng gói thể loại', () => {
  it('gói đang chọn → “Gói <tên ngắn> · có chấm”; thiếu `short` thì bỏ phần sau dấu phẩy (câu 12)', () => {
    const row = appliedRows(config('nu-tan-bao-thu'), null, true, PACKS).find((item) => item.label === 'Văn phong')!
    expect(row.value).toBe('Gói Nữ tần trọng sinh báo thù · có chấm')
    expect(row.own).toBe(true)
    const short = appliedRows(config('nu-tan-bao-thu'), null, true, [{ ...PACKS[0], short: 'Nữ tần báo thù' }])
    expect(short.find((item) => item.label === 'Văn phong')!.value).toBe('Gói Nữ tần báo thù · có chấm')
  })

  it('kiểu viết thường giữ nguyên tên; chưa biết gói (presets chưa tải) không đổi giá trị', () => {
    expect(appliedRows(config('suspense'), null, true, PACKS).find((item) => item.label === 'Văn phong')!.value).toBe('suspense')
    expect(appliedRows(config('nu-tan-bao-thu'), null, true, undefined).find((item) => item.label === 'Văn phong')!.value)
      .toBe('nu-tan-bao-thu')
  })
})
