// Nâng cao › thẻ “Máy đọc · chi tiết kỹ thuật” (10-audio.md §8.5) — chỗ duy nhất trên giao diện
// được hiện tên công cụ, đường dẫn, mã lỗi và nhật ký. Một bước thu hồi quyền (19-gap-closure AT-4).
import { useState } from 'react'
import { getTtsStatus, revokeTtsMachine } from '../../../api/client'
import { invalidate, keys, useResource } from '../../../data'
import { useAction } from '../../../data/useAction'
import { Button, Icon, toast } from '@noverse/ui'
import { confirm } from '@noverse/ui'
import { copyText } from '../../../lib/clipboard'
import { machineAdvanced, type MachineAdvancedRow, type MachineStatus } from '../../audio/machine'

const MONO = "'JetBrains Mono', ui-monospace, monospace"

function StatusPill({ label }: { label: string }) {
  return (
    <span style={{
      display: 'inline-flex', alignItems: 'center', gap: 6, height: 22, padding: '0 8px', borderRadius: 7,
      background: 'var(--line-1)', color: 'var(--color-fg-3)', fontSize: 11.5, fontWeight: 500, whiteSpace: 'nowrap',
    }}>
      <span aria-hidden="true" style={{ width: 6, height: 6, borderRadius: '50%', background: 'currentColor' }} />
      {label}
    </span>
  )
}

/** Bảng nhãn · giá trị kỹ thuật (mono) · chú thích chữ thường bên cạnh. */
function DetailRows({ rows }: { rows: MachineAdvancedRow[] }) {
  return (
    <div style={{ display: 'grid', gridTemplateColumns: '190px minmax(0, 1fr)', columnGap: 14 }}>
      {rows.map((row) => (
        <div key={row.label} style={{ display: 'contents' }}>
          <span style={{ minHeight: 34, display: 'flex', alignItems: 'center', fontSize: 13, color: 'var(--color-fg-3)', borderTop: '1px solid var(--line-1)' }}>
            {row.label}
          </span>
          <span style={{ minHeight: 34, display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap', borderTop: '1px solid var(--line-1)' }}>
            <span className="mono" style={{ fontFamily: MONO, fontSize: 12, color: 'var(--color-fg)', overflowWrap: 'anywhere' }}>{row.value}</span>
            {row.note && <span style={{ fontSize: 12.5, color: 'var(--color-fg-4)' }}>{row.note}</span>}
          </span>
        </div>
      ))}
    </div>
  )
}

function LogBlock({ log }: { log: string }) {
  return (
    <pre style={{
      margin: 0, padding: '10px 12px', borderRadius: 10, background: 'var(--color-bg)', border: '1px solid var(--line-1)',
      fontFamily: MONO, fontSize: 11.5, lineHeight: 1.75, color: 'var(--color-fg-3)',
      whiteSpace: 'pre-wrap', overflowWrap: 'anywhere',
    }}>{log}</pre>
  )
}

export function MachineDetailCard() {
  const resource = useResource([...keys.tts(), 'status'], ({ signal }) => getTtsStatus(signal))
  const status = resource.data as MachineStatus | undefined
  const adv = machineAdvanced(status)
  const [copied, setCopied] = useState(false)
  const revoke = useAction(async () => { await revokeTtsMachine(); void invalidate(keys.tts()) })

  if (!status || adv.setup) {
    // Chưa có máy đọc (bản máy chủ): hướng dẫn + lệnh bật một lần.
    if (!adv.setup) return null
    return (
      <section className="card" aria-label="Máy đọc · chi tiết kỹ thuật" style={{
        borderRadius: 16, padding: '16px 18px', display: 'flex', flexDirection: 'column', gap: 12,
      }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
          <Icon name="headphone" size={17} style={{ color: 'var(--color-fg-3)' }} />
          <b style={{ flex: '1 1 auto', fontSize: 14, fontWeight: 600 }}>Máy đọc · chi tiết kỹ thuật</b>
          <StatusPill label={adv.statusLabel} />
        </div>
        <span style={{ fontSize: 13, color: 'var(--color-fg-3)', lineHeight: 1.55 }}>{adv.setupText}</span>
        <LogBlock log={adv.commands} />
      </section>
    )
  }
  if (!adv.has) return null

  const copyLog = async () => {
    try {
      await copyText(adv.log)
      setCopied(true)
      window.setTimeout(() => setCopied(false), 1600)
      toast.ok('Đã chép nhật ký máy đọc')
    } catch {
      toast.err('Trình duyệt chặn sao chép — hãy chọn và sao chép thủ công.')
    }
  }

  const askRevoke = () => {
    void confirm({
      title: `Thu hồi quyền của “${status.machine?.name?.trim() || 'Máy đọc'}”?`,
      body: 'Máy đọc này sẽ không còn nhận việc tạo giọng cho thư viện.',
      tone: 'danger',
      confirmLabel: 'Thu hồi quyền',
      pendingLabel: 'Đang thu hồi…',
      consequences: [
        'Máy đọc dừng nhận việc ngay; chương đang tạo trả về hàng chờ, không mất phần đã đọc.',
        'Giọng đã tạo, bản đọc và tệp xuất giữ nguyên.',
        'Muốn dùng lại phải cấp quyền mới trên máy đọc.',
      ],
      successMessage: `Đã thu hồi quyền máy đọc “${status.machine?.name?.trim() || 'Máy đọc'}”`,
      onConfirm: () => revoke.run(),
    })
  }

  return (
    <section className="card" aria-label="Máy đọc · chi tiết kỹ thuật" style={{
      borderRadius: 16, padding: '16px 18px', display: 'flex', flexDirection: 'column', gap: 12,
    }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
        <Icon name="headphone" size={17} style={{ color: 'var(--color-fg-3)' }} />
        <b style={{ flex: '1 1 auto', fontSize: 14, fontWeight: 600 }}>Máy đọc · chi tiết kỹ thuật</b>
        <StatusPill label={adv.statusLabel} />
      </div>
      <DetailRows rows={adv.rows} />
      {adv.log && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
          <span style={{ fontSize: 12.5, color: 'var(--color-fg-4)' }}>6 dòng nhật ký gần nhất</span>
          <LogBlock log={adv.log} />
        </div>
      )}
      <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
        <Button size="sm" disabled={!adv.log} onClick={() => void copyLog()} title={adv.log ? undefined : 'Chưa có dòng nhật ký nào'}>
          <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6 }}>
            <Icon name={copied ? 'tick-circle' : 'copy'} size={14} />{copied ? 'Đã chép' : 'Chép nhật ký'}
          </span>
        </Button>
        <span style={{ flex: 1 }} />
        {adv.canRevoke && (
          <Button size="sm" onClick={askRevoke} pending={revoke.pending} pendingLabel="Đang thu hồi…"
            style={{ borderColor: 'var(--kit-errb-35)', color: 'var(--color-err)' }}>
            Thu hồi quyền máy đọc
          </Button>
        )}
      </div>
    </section>
  )
}
