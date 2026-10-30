import { useEffect, useState } from 'react'

/** Giá trị trễ `ms` sau lần đổi cuối — gõ nhanh không gọi API/lọc lại mỗi ký tự. */
export function useDebounced<T>(value: T, ms = 250): T {
  const [debounced, setDebounced] = useState(value)
  useEffect(() => {
    const id = setTimeout(() => setDebounced(value), ms)
    return () => clearTimeout(id)
  }, [value, ms])
  return debounced
}
