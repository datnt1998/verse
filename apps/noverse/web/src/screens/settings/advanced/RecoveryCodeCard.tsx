// Thẻ “Mã khôi phục của quản trị” — Onboard.dc.html (bước Hoàn tất, showRecovery). Mã chỉ hiện một lần.
import { useEffect, useRef, useState } from 'react'
import { t } from '../../../i18n/vi'
import { Icon } from '@noverse/ui'
import { toast } from '@noverse/ui'
import { recoveryFile } from './logic'
import { copyText } from '../../../lib/clipboard'

const ghostBtn = {
  height: 42, padding: '0 14px', borderRadius: 10, border: '1px solid rgba(255,255,255,0.12)',
  background: 'transparent', color: '#EDEDEF', fontSize: 13, display: 'flex', alignItems: 'center', gap: 6,
} as const

export function RecoveryCodeCard({ code }: { code: string }) {
  const [copied, setCopied] = useState(false)
  const timer = useRef<number | null>(null)
  useEffect(() => () => { if (timer.current !== null) window.clearTimeout(timer.current) }, [])

  const copy = async () => {
    try {
      await copyText(code)
      setCopied(true)
      if (timer.current !== null) window.clearTimeout(timer.current)
      timer.current = window.setTimeout(() => setCopied(false), 1600)
    } catch {
      toast.err(t.adv.recovery.copyBlocked)
    }
  }

  const download = () => {
    const file = recoveryFile(code)
    const url = URL.createObjectURL(new Blob([file.text], { type: 'text/plain;charset=utf-8' }))
    const link = document.createElement('a')
    link.href = url
    link.download = file.name
    document.body.appendChild(link)
    link.click()
    link.remove()
    URL.revokeObjectURL(url)
  }

  return (
    <div className="card" role="group" aria-label={t.adv.recovery.title} style={{
      borderRadius: 16, padding: 16, display: 'flex', flexDirection: 'column', gap: 10,
      borderColor: 'rgba(246,203,140,0.28)',
    }}>
      <span style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 14, fontWeight: 600 }}>
        <Icon name="key" size={17} style={{ color: '#F6CB8C' }} />{t.adv.recovery.title}
      </span>
      <span style={{ fontSize: 13, color: '#A0A0AB', lineHeight: 1.5 }}>{t.adv.recovery.body}</span>
      <div style={{ display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: 8 }}>
        <span style={{
          flex: '1 1 220px', minWidth: 0, height: 42, padding: '0 14px', borderRadius: 10, background: '#0F0F11',
          border: '1px dashed rgba(246,203,140,0.35)', display: 'flex', alignItems: 'center', fontSize: 17,
          fontWeight: 600, letterSpacing: '.12em', color: '#FBE3BD', userSelect: 'all',
        }}>{code}</span>
        <button type="button" className="hov-ghost" onClick={() => void copy()} style={ghostBtn}>
          <Icon name={copied ? 'tick-circle' : 'copy'} size={15} />{copied ? t.adv.recovery.copied : t.adv.recovery.copy}
        </button>
        <button type="button" className="hov-ghost" onClick={download} style={ghostBtn}>
          <Icon name="document-download" size={15} />{t.adv.recovery.download}
        </button>
      </div>
    </div>
  )
}
