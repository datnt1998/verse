// D10 · Não truyện — chương đang xem đi qua `?ch=` (giữ được khi mở từ trang Đọc).
// Câu 20: đổi `?ch=` trong địa chỉ lúc màn đang mở phải chọn lại chương, không chỉ đọc một lần.
// `?ch=` là nguồn một chiều như các màn khác (Quality `?view=`, SysBrain `?tab=`): màn chỉ đọc, không ghi
// lại vào địa chỉ, nên URL và state không đẩy qua lại nhau.
import { useEffect, useRef, useState } from 'react'
import type { Brain } from '../../api/types'
import { useRoute } from '../../router'
import { initialChapter } from './model'

/**
 * Chương đang xem của Não truyện (bản máy tính): `?ch=` khi hợp lệ, không thì chương đang viết.
 * Chọn tay trên thanh chương chỉ đổi state — dữ liệu được làm mới không kéo người xem về chương cũ.
 */
export function useChapterQuery(chapters: Brain['chapters'] | null): [number | null, (n: number) => void] {
  const raw = Number.parseInt(useRoute().search.get('ch') ?? '', 10)
  const query = Number.isFinite(raw) && raw > 0 ? raw : null
  const [chapter, setChapter] = useState<number | null>(null)
  const last = chapters?.[chapters.length - 1]?.n ?? null
  // Effect chọn lại chỉ chạy khi `?ch=` đổi, nên đọc số chương mới nhất qua ref thay vì thêm vào phụ thuộc.
  const limit = useRef<number | null>(null)
  limit.current = last

  // Mở màn (và mỗi lần dữ liệu về): giữ lựa chọn tay, chưa chọn thì lấy `?ch=`, không thì chương đang viết.
  useEffect(() => {
    if (!chapters || last === null) return
    const wanted = query !== null && query <= last ? query : null
    setChapter((value) => value ?? wanted ?? initialChapter(chapters))
  }, [chapters, last, query])

  // Câu 20: `?ch=` đổi trong địa chỉ → chọn lại đúng chương đó. Chương ngoài dàn ý thì bỏ qua.
  useEffect(() => {
    const max = limit.current
    if (query === null || max === null || query > max) return
    setChapter(query)
  }, [query])

  return [chapter, setChapter]
}
