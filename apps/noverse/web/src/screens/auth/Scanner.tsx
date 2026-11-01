// A2b › scan · scanFound · scanWrong — camera quét QR đăng nhập (Phone.dc.html).
// BarcodeDetector nếu có; không có (hoặc không bảo mật/không có camera) → PairScreen hiện cameraBlocked.
// Không tự gọi /api/pair: chỉ mở bước xác nhận sau khi người dùng quét được mã.
import { useEffect, useRef, useState } from 'react'
import { Icon } from '@noverse/ui'
import { Spinner } from '@noverse/ui'

type DetectedBarcode = { rawValue: string }
type BarcodeDetectorLike = { detect: (source: CanvasImageSource) => Promise<DetectedBarcode[]> }
type BarcodeDetectorCtor = new (options?: { formats?: string[] }) => BarcodeDetectorLike

declare global {
  interface Window { BarcodeDetector?: BarcodeDetectorCtor }
}

/** Mã Noverse hợp lệ: cùng máy chủ, dạng /p/<code> (mã QR hiện tại) hoặc /#/pair/<code> (liên kết cũ). */
function parsePairCode(raw: string): string | null {
  try {
    const url = new URL(raw, location.href)
    if (url.origin !== location.origin) return null
    const match = /^\/p\/([^/?#]+)$/.exec(url.pathname) ?? /#\/pair\/([^/?#]+)/.exec(url.hash)
    return match ? decodeURIComponent(match[1]) : null
  } catch { return null }
}

export function Scanner({ onClose, onPassword, onCode, onBlocked }: {
  onClose: () => void
  onPassword: () => void
  onCode: (code: string) => void
  onBlocked: () => void
}) {
  const videoRef = useRef<HTMLVideoElement>(null)
  const streamRef = useRef<MediaStream | null>(null)
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const [found, setFound] = useState(false)
  const [lastFrame, setLastFrame] = useState('')
  const [wrong, setWrong] = useState(false)
  const [torch, setTorch] = useState(false)
  const [torchable, setTorchable] = useState(false)

  useEffect(() => {
    // Trang http (không bảo mật): xin quyền cũng vô ích → đi thẳng cameraBlocked.
    const Detector = window.BarcodeDetector
    if (!window.isSecureContext || !Detector) { onBlocked(); return }
    let cancelled = false
    const detector = new Detector({ formats: ['qr_code'] })
    let wrongTimer: ReturnType<typeof setTimeout> | null = null
    let foundTimer: ReturnType<typeof setTimeout> | null = null
    let accepted = false

    const stop = () => {
      clearInterval(timerRef.current ?? undefined)
      timerRef.current = null
      streamRef.current?.getTracks().forEach((track) => track.stop())
      streamRef.current = null
    }

    const start = async () => {
      try {
        const stream = await navigator.mediaDevices.getUserMedia({
          audio: false,
          video: { facingMode: { ideal: 'environment' } },
        })
        if (cancelled) { stream.getTracks().forEach((track) => track.stop()); return }
        streamRef.current = stream
        const video = videoRef.current
        if (video) {
          video.srcObject = stream
          await video.play().catch(() => { /* autoplay có thể bị chặn; vẫn thử detect */ })
        }
        const track = stream.getVideoTracks()[0]
        const caps = (track.getCapabilities?.() ?? {}) as { torch?: boolean }
        setTorchable(caps.torch === true)
        timerRef.current = setInterval(() => {
          const node = videoRef.current
          if (!node || node.readyState < 2) return
          void detector.detect(node).then((codes) => {
            if (cancelled || accepted) return
            const raw = codes[0]?.rawValue
            if (!raw) return
            const code = parsePairCode(raw)
            if (code) {
              accepted = true
              setWrong(false)
              clearTimeout(wrongTimer ?? undefined)
              const frame = document.createElement('canvas')
              frame.width = node.videoWidth; frame.height = node.videoHeight
              frame.getContext('2d')?.drawImage(node, 0, 0)
              setLastFrame(frame.toDataURL('image/jpeg', .85))
              clearInterval(timerRef.current ?? undefined)
              timerRef.current = null
              stop()
              setFound(true)
              navigator.vibrate?.(30)
              foundTimer = setTimeout(() => { if (!cancelled) onCode(code) }, 1000)
            } else {
              setWrong(true)
              clearTimeout(wrongTimer ?? undefined)
              wrongTimer = setTimeout(() => setWrong(false), 2600)
            }
          }).catch(() => { /* khung hình chưa sẵn sàng */ })
        }, 125)
      } catch {
        if (!cancelled) onBlocked()
      }
    }
    void start()
    return () => {
      cancelled = true
      clearTimeout(wrongTimer ?? undefined)
      clearTimeout(foundTimer ?? undefined)
      stop()
    }
  }, [onBlocked, onCode])

  const toggleTorch = () => {
    const track = streamRef.current?.getVideoTracks()[0]
    if (!track) return
    const next = !torch
    // torch có trong spec MediaTrackConstraintSet nhưng chưa có trong lib.dom của TS.
    const constraints = { advanced: [{ torch: next }] } as unknown as MediaTrackConstraints
    void track.applyConstraints(constraints)
      .then(() => setTorch(next)).catch(() => setTorchable(false))
  }

  const frameColor = found ? 'var(--color-ok)' : wrong ? 'var(--color-warn)' : 'white'

  return (
    <div style={{ position: 'relative', flex: 1, minHeight: 0, overflow: 'hidden', background: 'var(--color-bg)' }}>
      <video ref={videoRef} muted playsInline aria-hidden="true"
        style={{ position: 'absolute', inset: 0, width: '100%', height: '100%', objectFit: 'cover', background: 'var(--color-surface-0)' }} />
      {lastFrame && <img src={lastFrame} alt="" aria-hidden="true" style={{ position: 'absolute', inset: 0, width: '100%', height: '100%', objectFit: 'cover' }} />}
      <div aria-hidden="true" style={{
        position: 'absolute', left: '50%', top: '36%', width: 236, height: 236,
        transform: 'translate(-50%, -50%)', borderRadius: 26,
        boxShadow: '0 0 0 999px rgba(5,5,7,0.58)',
        animation: found ? 'ai-lock-in .35s ease-out both' : undefined,
      }}>
        {[
          { pos: { top: -3, left: -3 }, radius: '22px 0 0 0', width: '4px 0 0 4px' },
          { pos: { top: -3, right: -3 }, radius: '0 22px 0 0', width: '4px 4px 0 0' },
          { pos: { bottom: -3, left: -3 }, radius: '0 0 0 22px', width: '0 0 4px 4px' },
          { pos: { bottom: -3, right: -3 }, radius: '0 0 22px 0', width: '0 4px 4px 0' },
        ].map((corner, index) => (
          <span key={index} aria-hidden="true" style={{
            position: 'absolute', width: 38, height: 38, ...corner.pos,
            borderColor: frameColor, borderStyle: 'solid', borderWidth: corner.width, borderRadius: corner.radius,
          }} />
        ))}
        {!found && !wrong && <span className="scanline" aria-hidden="true" />}
      </div>

      <div style={{ position: 'absolute', top: 0, left: 0, right: 0, display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: 14 }}>
        <button type="button" className="hov-ghost" onClick={onClose} aria-label="Đóng camera"
          style={{ width: 40, height: 40, borderRadius: '50%', border: 'none', background: 'rgba(20,20,24,0.7)', color: 'var(--color-fg)', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
          <Icon name="close-circle-bulk" size={18} />
        </button>
        <span style={{ fontSize: 15, fontWeight: 600, textShadow: '0 1px 8px rgba(0,0,0,0.6)' }}>Quét mã đăng nhập</span>
        <button type="button" className="hov-ghost" onClick={toggleTorch} aria-pressed={torch} aria-label="Đèn flash" disabled={!torchable}
          style={{
            width: 40, height: 40, borderRadius: '50%', border: 'none',
            background: torch ? 'var(--color-fg)' : 'rgba(20,20,24,0.7)',
            color: torch ? 'var(--color-bg)' : 'var(--color-fg)',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            opacity: torchable ? 1 : 0.5,
          }}>
          <Icon name="flash" size={18} />
        </button>
      </div>

      <div style={{
        position: 'absolute', left: 0, right: 0, bottom: 0, padding: '18px 16px 26px',
        borderRadius: '24px 24px 0 0', background: 'var(--color-surface-1)',
        borderTop: '1px solid var(--line-3)', display: 'flex', flexDirection: 'column', gap: 14,
      }}>
        <div aria-hidden="true" style={{ alignSelf: 'center', width: 36, height: 4, borderRadius: 4, background: 'var(--color-surface-7)' }} />
        {!found && !wrong && (
          <>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 4, textAlign: 'center' }}>
              <span style={{ fontSize: 16, fontWeight: 600 }}>Hướng camera vào mã QR trên máy tính</span>
              <span style={{ fontSize: 13, color: 'var(--color-fg-3)' }}>Giữ điện thoại cách màn hình khoảng 20–30 cm</span>
            </div>
            <div style={{ display: 'flex', gap: 12, padding: 12, borderRadius: 14, background: 'var(--color-surface-0)', border: '1px solid var(--line-1)' }}>
              <span aria-hidden="true" style={{
                width: 36, height: 36, flex: '0 0 auto', borderRadius: 10, background: 'var(--color-surface-5)',
                display: 'flex', alignItems: 'center', justifyContent: 'center', color: 'var(--color-accent)',
              }}><Icon name="monitor" size={18} /></span>
              <span style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
                <span style={{ fontSize: 13, fontWeight: 500 }}>Chưa thấy mã trên máy tính?</span>
                <span style={{ fontSize: 12.5, color: 'var(--color-fg-3)' }}>
                  Mở Noverse trên máy tính → bấm ảnh đại diện → <span style={{ color: 'var(--color-fg)' }}>Đăng nhập điện thoại bằng QR</span>
                </span>
              </span>
            </div>
            <span role="status" style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 8, fontSize: 13, color: 'var(--color-fg-3)' }}>
              <Spinner size={14} />Đang tìm mã…
            </span>
          </>
        )}
        {found && (
          <div role="status" style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '4px 0' }}>
            <span aria-hidden="true" style={{
              width: 44, height: 44, borderRadius: '50%', background: 'var(--tint-ok)',
              boxShadow: '0 0 0 1px rgba(166,237,193,0.3)', display: 'flex', alignItems: 'center', justifyContent: 'center', color: 'var(--color-ok)',
            }}><Icon name="tick-circle-bulk" size={24} /></span>
            <span style={{ display: 'flex', flexDirection: 'column' }}>
              <span style={{ fontSize: 16, fontWeight: 600 }}>Đã nhận mã đăng nhập</span>
              <span style={{ fontSize: 13, color: 'var(--color-fg-3)' }}>Đang mở bước xác nhận…</span>
            </span>
          </div>
        )}
        {wrong && (
          <div role="alert" style={{ display: 'flex', gap: 12, padding: 12, borderRadius: 14, background: 'rgba(246,203,140,0.06)', border: '1px solid rgba(246,203,140,0.22)' }}>
            <Icon name="warning-2" size={18} style={{ color: 'var(--color-warn)', marginTop: 1 }} />
            <span style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
              <span style={{ fontSize: 14, fontWeight: 600 }}>Đây không phải mã đăng nhập Noverse</span>
              <span style={{ fontSize: 13, color: 'var(--color-fg-2)' }}>Hãy quét mã trong hộp thoại “Đăng nhập điện thoại bằng QR” trên máy tính của bạn.</span>
            </span>
          </div>
        )}
        {wrong && <span role="status" style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 8, fontSize: 13, color: 'var(--color-fg-3)' }}><Spinner size={14} />Vẫn đang tìm mã…</span>}
        <button type="button" className="hov-ghost" onClick={onPassword} style={{
          height: 44, borderRadius: 14, border: '1px solid var(--line-4)', background: 'transparent',
          color: 'var(--color-fg)', fontSize: 15,
        }}>Đăng nhập bằng mật khẩu</button>
      </div>
    </div>
  )
}
