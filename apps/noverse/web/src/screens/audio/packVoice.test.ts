// D11 — gói một giọng kể: nhận diện gói từ `packs[].audio` (goi.json › audio.che_do/giong) + chữ.
import { expect, it } from 'vitest'
import type { PackPreset } from '../../api/types'
import { t } from '../../i18n/vi'
import { packVoiceOf } from './packVoice'

const pack = (over: Partial<PackPreset>): PackPreset => ({
  id: 'goi-a', ten: 'Nữ tần trọng sinh báo thù, vả mặt', short: 'Nữ tần báo thù', so_chuong: 8, nguoi_nghe: 'Nữ 18–35', ...over,
})

it('gói có audio.che_do = single là gói một giọng kể (tên ngắn theo Câu 12)', () => {
  expect(packVoiceOf([pack({ audio: { che_do: 'single', giong: 'nu' } })], 'goi-a')).toEqual({ ten: 'Nữ tần báo thù', gender: 'nu' })
  expect(packVoiceOf([pack({ audio: { che_do: 'single' } })], 'goi-a')).toEqual({ ten: 'Nữ tần báo thù', gender: '' })
})

it('gói chia vai thì không khoá phân vai', () => {
  expect(packVoiceOf([pack({ audio: { che_do: 'cast' } })], 'goi-a')).toBeNull()
})

it('lựa chọn giọng của truyện thắng mặc định một giọng của gói', () => {
  for (const mode of ['cast', 'single', 'pack'] as const) {
    expect(packVoiceOf([pack({ audio: { che_do: 'single', giong: 'nu' } })], 'goi-a', mode)).toBeNull()
  }
})

it('chỉ có câu voice (máy chủ chưa trả audio) thì không đoán chế độ đọc', () => {
  expect(packVoiceOf([pack({ voice: 'Một giọng nữ trẻ kể cả truyện' })], 'goi-a')).toBeNull()
  expect(packVoiceOf([pack({ voice: 'Nhiều giọng, chia vai theo nhân vật' })], 'goi-a')).toBeNull()
})

it('giới tính lấy đúng trường audio.giong, không đọc câu voice', () => {
  expect(packVoiceOf([pack({ audio: { che_do: 'single', giong: 'nam' }, voice: 'Một giọng nữ kể cả truyện' })], 'goi-a'))
    .toEqual({ ten: 'Nữ tần báo thù', gender: 'nam' })
})

it('truyện dùng kiểu viết thường, gói đã bị xoá hoặc chưa có danh sách gói', () => {
  expect(packVoiceOf([pack({ audio: { che_do: 'single' } })], 'romance')).toBeNull()
  expect(packVoiceOf([], 'goi-a')).toBeNull()
  expect(packVoiceOf(undefined, 'goi-a')).toBeNull()
  expect(packVoiceOf([pack({ audio: { che_do: 'single' } })], undefined)).toBeNull()
})

it('câu mô tả và dòng phụ đúng chữ artboard', () => {
  expect(t.audio.pack.moTa('Nữ tần báo thù', 'nu')).toBe(
    'Gói Nữ tần báo thù dùng một giọng nữ trẻ cho cả truyện, kể cả lời thoại. Gói đo nhịp và thoại theo một giọng nên không phân vai.')
  expect(t.audio.pack.tomTat('Nữ tần báo thù', 'nu')).toBe('Gói Nữ tần báo thù · giọng nữ trẻ đọc cả truyện')
  expect(t.audio.pack.tomTat('Niên đại văn', 'nam')).toBe('Gói Niên đại văn · giọng nam đọc cả truyện')
})

it('mẫu giọng của bạn hiện “Bạn tải lên · 18 giây · nữ”, không có tên tệp', () => {
  expect(t.audio.pack.banTaiLen(18, ' · nữ')).toBe('Bạn tải lên · 18 giây · nữ')
})
