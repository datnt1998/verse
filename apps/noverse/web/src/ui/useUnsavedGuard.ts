import { useEffect, useRef } from 'react'
import { addNavigationBlocker } from '../router'
import { confirmUnsavedLeave } from './confirmPresets'

type LeaveOptions = { save?: () => Promise<boolean>; allow?: (url: URL) => boolean }

export function confirmDiscardChanges(message = 'Bạn còn thay đổi chưa lưu. Rời trang và bỏ thay đổi?') {
  return confirmUnsavedLeave({
    body: message, confirmLabel: 'Bỏ thay đổi', pendingLabel: 'Đang rời đi…',
    successMessage: undefined, onConfirm: () => {}, secondary: undefined,
  })
}

/** Liên kết, navigate() và back/forward dùng kit async; đóng tab vẫn dùng beforeunload native. */
export function useUnsavedGuard(dirty: boolean, message = 'Bạn còn thay đổi chưa lưu. Rời trang và bỏ thay đổi?', options: LeaveOptions = {}) {
  const latest = useRef(options)
  latest.current = options
  const mounted = useRef(true)
  useEffect(() => { mounted.current = true; return () => { mounted.current = false } }, [])
  useEffect(() => {
    if (!dirty) return
    let acceptedURL = location.href
    let prompting = false
    const allowed = (url: URL) => url.hash.startsWith('#/login') || latest.current.allow?.(url)
    const leave = async (url: string) => {
      if (prompting) return
      prompting = true
      const save = latest.current.save
      const accepted = save ? await confirmUnsavedLeave({
        body: message,
        onConfirm: async () => { if (!await save()) throw new Error('Chưa lưu được thay đổi. Hãy thử lại hoặc chọn Bỏ thay đổi.') },
        secondary: { label: 'Bỏ thay đổi', danger: true, onConfirm: () => {} },
      }) : await confirmDiscardChanges(message)
      prompting = false
      if (!accepted || !mounted.current) return
      acceptedURL = url
      location.assign(url)
    }
    const beforeUnload = (event: BeforeUnloadEvent) => { event.preventDefault(); event.returnValue = '' }
    const click = (event: MouseEvent) => {
      if (event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return
      const anchor = (event.target as Element | null)?.closest('a[href]') as HTMLAnchorElement | null
      if (!anchor || anchor.href === location.href || anchor.target === '_blank' || anchor.hasAttribute('download')) return
      if (allowed(new URL(anchor.href))) { acceptedURL = anchor.href; return }
      event.preventDefault(); event.stopImmediatePropagation()
      void leave(anchor.href)
    }
    // Router hỏi trước khi đổi route: Chrome gọi listener của window theo thứ tự đăng ký, capture không chen trước được.
    const blockHashChange = () => {
      if (location.href === acceptedURL || allowed(new URL(location.href))) { acceptedURL = location.href; return false }
      const nextURL = location.href
      history.replaceState(null, '', acceptedURL)
      void leave(nextURL)
      return true
    }
    window.addEventListener('beforeunload', beforeUnload)
    document.addEventListener('click', click, true)
    const removeBlocker = addNavigationBlocker(blockHashChange)
    return () => {
      window.removeEventListener('beforeunload', beforeUnload)
      document.removeEventListener('click', click, true)
      removeBlocker()
    }
  }, [dirty, message])
}
