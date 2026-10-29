import { useCallback } from 'react'
import { useMobileToast } from '../../mobile/primitives'
import { toast } from '@noverse/ui'

/** Toast của M7 theo artboard (.mtoast: dấu tick xanh). Điện thoại/máy tính bảng dùng MobileFeedback; máy tính dùng toast chung. */
export function useNotifyToast() {
  const mobile = useMobileToast()
  return useCallback((text: string) => {
    if (window.matchMedia('(max-width: 1099px)').matches) mobile(text)
    else toast.ok(text)
  }, [mobile])
}
