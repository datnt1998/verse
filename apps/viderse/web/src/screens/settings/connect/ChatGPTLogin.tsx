// ChatGPTLogin — hành động đăng nhập ChatGPT dùng lại được ở Cài đặt › Kết nối
// AI và màn Chuẩn bị máy (V9 codex-login): variant 'primary' (nút chính duy
// nhất "Đăng nhập ChatGPT") hoặc 'ghost' (nhỏ "Đăng nhập ngay"). Phiên đăng nhập
// poll GET /api/ai/providers/chatgpt/login 2 s tuần tự; xong → invalidate.
// Cửa sổ authorize giữ chỗ NGAY trong cú click rồi điều hướng khi có authUrl;
// popup bị chặn → link "Mở trang đăng nhập" hiện cạnh trạng thái chờ.
// ChatGPTLoginAction không điều hướng — màn setup tự theo dõi GET /api/setup.
import { useEffect, useRef, useState } from 'react'
import { Button, Spinner, toast } from '@noverse/ui'
import { invalidate } from '@noverse/ui/data'
import { cancelProviderLogin, getProviderLogin, startProviderLogin, type AiLogin } from '../../../api/ai'
import { keys } from '../../../data/keys'
import { sequentialPoll } from '../poll'
import { t } from '../../../i18n/vi'
import { aiCopy } from '../../../i18n/ai'

/** Provider id của ChatGPT trong danh sách /api/ai/providers. */
export const CHATGPT_PROVIDER_ID = 'chatgpt'

export function ChatGPTLoginAction({ variant = 'primary', size = 'compact', onDone }: {
  variant?: 'primary' | 'ghost'
  /** Cỡ nút chính. Cài đặt › Kết nối là `compact` (artboard `.primary` 32);
   *  màn Chuẩn bị máy dùng `md` (artboard Onboard `primary` 36/r10/13.5). */
  size?: 'compact' | 'md'
  onDone?: () => void
}) {
  const [pending, setPending] = useState(false)
  const [starting, setStarting] = useState(false)
  const [login, setLogin] = useState<AiLogin | null>(null)
  const win = useRef<Window | null>(null)
  const navigated = useRef('')
  const onDoneRef = useRef(onDone)
  onDoneRef.current = onDone

  useEffect(() => {
    if (!pending) return
    const stop = sequentialPoll(2000, async (signal) => {
      let next: AiLogin
      try {
        next = await getProviderLogin(CHATGPT_PROVIDER_ID, signal)
      } catch {
        return !signal.aborted
      }
      setLogin(next)
      const url = next.phase === 'authorize' ? next.authUrl : next.phase === 'code' ? next.verifyUrl : undefined
      if (url && url !== navigated.current && win.current && !win.current.closed) {
        navigated.current = url
        win.current.location.href = url
      }
      const done = next.phase === 'done' || next.phase === 'failed' || next.phase === 'cancelled' || next.phase === 'idle'
      if (done) {
        setPending(false)
        await invalidate(keys.aiProviders())
        if (next.phase === 'done') onDoneRef.current?.()
      }
      return !done
    })
    return stop
  }, [pending])

  async function start() {
    setStarting(true)
    // Giữ chỗ cửa sổ trong gesture — popup bị chặn vẫn còn link fallback.
    if (typeof window.open === 'function') {
      try { win.current = window.open('', '_blank', 'noopener') } catch { win.current = null }
    }
    try {
      await startProviderLogin(CHATGPT_PROVIDER_ID)
      navigated.current = ''
      setLogin(null)
      setPending(true)
    } catch (error) {
      toast.err(error instanceof Error ? error.message : t.errors.failed)
    } finally {
      setStarting(false)
    }
  }

  async function cancel() {
    try { await cancelProviderLogin(CHATGPT_PROVIDER_ID) } catch { /* phiên đã chết */ }
    setPending(false)
  }

  if (pending) {
    const url = login?.phase === 'authorize' ? login.authUrl : login?.phase === 'code' ? login.verifyUrl : undefined
    return (
      <span className="conn-code__wait">
        <Spinner size={14} color="var(--color-fg-3)" />
        {aiCopy.connect.chatgptWait}
        {url && (
          <a className="conn-link" href={url} target="_blank" rel="noopener noreferrer">
            {aiCopy.connect.openAuth}
          </a>
        )}
        <Button variant="quiet" size="compact" onClick={() => void cancel()}>{t.common.cancel}</Button>
      </span>
    )
  }
  if (variant === 'ghost') {
    return (
      <Button size="compact" variant="ghost" pending={starting} onClick={() => void start()}>
        {aiCopy.connect.loginNow}
      </Button>
    )
  }
  return (
    <Button size={size} variant="primary" pending={starting} onClick={() => void start()}>
      {aiCopy.connect.loginChatGPT}
    </Button>
  )
}
