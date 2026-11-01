// Hộp thoại của màn Audio: đổi chất lượng giọng (tạo lại đoạn đã có) và thêm mẫu giọng.
import { useState } from 'react'
import type { AudioData } from './useAudio'
import { toast } from '@noverse/ui'
import { Dialog } from '@noverse/ui'
import { Button } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { FieldLabel } from '@noverse/ui'
import { KitSelect } from '@noverse/ui'
import { formatDuration } from './logic'
import { t } from '../../i18n/vi'
import { ConfirmDialog } from '@noverse/ui'

const QUALITY_LABEL = { fast: t.audio.dialogs.ngheThuNhanh, standard: t.audio.dialogs.chuan, high: t.audio.dialogs.caoNhat } as const

export function QualityDialog({ from, to, audioSeconds, onClose, onConfirm }: {
  from: 'fast' | 'standard' | 'high'; to: 'fast' | 'standard' | 'high'
  audioSeconds: number
  onClose: () => void; onConfirm: () => void | Promise<void>
}) {
  return <ConfirmDialog title={t.audio.dialogs.doiChatLuongGiong} tone="warn"
    body={<>{t.audio.dialogs.tu}<b>{QUALITY_LABEL[from]}</b>{t.audio.dialogs.sang}<b>{QUALITY_LABEL[to]}</b>{t.audio.dialogs.cacDoanDaCo}<br />{t.audio.dialogs.canTaoLaiAudio(formatDuration(audioSeconds))}</>}
    confirmLabel={t.audio.dialogs.doiTaoLai} pendingLabel="Đang đổi chất lượng…" onClose={onClose} onConfirm={onConfirm} />
}

export function AddVoiceDialog({ onClose, onUpload }: {
  onClose: () => void
  onUpload: (name: string, file: File, gender: string) => Promise<void>
}) {
  const [name, setName] = useState('')
  const [gender, setGender] = useState('nu')
  const [file, setFile] = useState<File | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  const submit = async () => {
    if (!file) { setError(t.audio.dialogs.chonTepMauGiong); return }
    if (!name.trim()) { setError(t.audio.dialogs.datTenChoMau); return }
    setBusy(true)
    setError('')
    try {
      await onUpload(name.trim(), file, gender)
      onClose()
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : t.audio.dialogs.khongTaiDuocMau)
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog padBody={false} title={t.audio.dialogs.themMauGiong} icon={<Icon name="microphone-2" size={16} />} onClose={onClose} width={460}
      footer={<>
        <Button variant="ghost" onClick={onClose}>{t.audio.dialogs.thoi}</Button>
        <Button variant="primary" onClick={() => void submit()} disabled={busy}>{busy ? t.audio.dialogs.dangTai : t.audio.dialogs.themMauGiong}</Button>
      </>}>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
        <p style={{ margin: 0, fontSize: 12.5, color: 'var(--color-fg-3)', lineHeight: 1.6 }}>
          {t.audio.dialogs.mauGiong1030}</p>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
          <FieldLabel htmlFor="voice-name">{t.audio.dialogs.tenMauGiong}</FieldLabel>
          <input id="voice-name" value={name} onChange={(event) => setName(event.target.value)} placeholder="vd: Mẫu giọng Tri Hạ"
            style={{
              width: '100%', padding: '8px 10px', borderRadius: 8, fontSize: 13,
              border: '1px solid var(--line-3)', background: 'var(--color-surface-0)', color: 'var(--color-fg)',
            }} />
        </div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
          <FieldLabel htmlFor="voice-gender">{t.audio.dialogs.gioiTinh}</FieldLabel>
          <KitSelect id="voice-gender" aria-label={t.audio.dialogs.gioiTinh} value={gender} height={38}
            options={[
              { value: 'nu', label: t.audio.dialogs.nu },
              { value: 'nam', label: t.audio.dialogs.nam },
              { value: 'khac', label: t.audio.dialogs.khac },
            ]}
            onChange={setGender} />
        </div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
          <FieldLabel htmlFor="voice-file">{t.audio.dialogs.tepMauGiong}</FieldLabel>
          <input id="voice-file" type="file" accept="audio/wav,audio/mpeg,audio/mp3,.wav,.mp3"
            onChange={(event) => setFile(event.target.files?.[0] ?? null)}
            style={{ fontSize: 12.5, color: 'var(--color-fg-3)' }} />
        </div>
        {error && <span role="alert" style={{ fontSize: 12, color: 'var(--color-err)' }}>{error}</span>}
      </div>
    </Dialog>
  )
}

type Quality = 'fast' | 'standard' | 'high'

/** Chip chất lượng trên thanh Audio → hộp thoại xác nhận (tạo lại các đoạn đã có giọng). */
export function useQualityChange(data: AudioData) {
  const [ask, setAsk] = useState<Quality | null>(null)
  const quality: Quality = data.settings?.quality ?? 'standard'
  const audioSeconds = (data.volume?.chapters ?? []).reduce((sum, item) => sum + (item.audio?.durationSec ?? 0), 0)
  const dialog = ask && (
    <QualityDialog from={quality} to={ask} audioSeconds={audioSeconds}
      onClose={() => setAsk(null)}
      onConfirm={async () => {
        if (!data.settings) return
        if (await data.saveSettings({ ...data.settings, quality: ask })) {
          setAsk(null)
          toast.ok(t.audio.dialogs.daDoiChatLuong)
        }
      }} />
  )
  return { quality, ask: setAsk, dialog }
}
