// Tuỳ chỉnh › Giọng đọc (Gói AT · 10-audio.md §8.1, §8.4) — thẻ Máy đọc năm trạng thái, chất lượng
// “Mặc định cho truyện mới”, tự tạo khi chốt, giọng có sẵn + nghe thử theo khả năng của máy đọc.
// Bản máy chủ ẩn Thiết bị/Bộ đọc; bản cá nhân cùng khung, tên “Máy này”.
// Dữ liệu TTS dùng cache stale-while-revalidate; việc cài chuyển sang dock toàn cục.
import { useCallback, useEffect, useRef, useState } from 'react'
import {
  getProjectVoices, getTtsEngines, getTtsStatus, getTtsVoices, previewTtsVoice,
} from '../../../api/client'
import type { AudioVoice } from '../../../api/types'
import { keys, useAction, useConnection, useResource, type Resource } from '../../../data'
import { Button } from '@noverse/ui'
import { Card } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { Switch } from '@noverse/ui'
import { Skeleton } from '@noverse/ui'
import { StaleBanner } from '@noverse/ui'
import { ErrorCard, InlineSegmented, ReadOnlyNotice, ReadOnlyValue, Rows, Section, SectionBadge, SettingRow } from '../components'
import type { SettingsContext } from '../types'
import { t } from '../../../i18n/vi'
import { unlockMedia } from '../../audio/useAudio'
import { PanelRefreshBar } from '../../../ui/PanelRefreshBar'
import { MachineCard, useMachineCardActions } from '../../audio/MachineCard'
import { machineStepLine, machineView, previewSupport, type MachineStatus } from '../../audio/machine'

const QUALITY = [
  { id: 'fast', label: t.audio.settings.ngheThuNhanh, hint: t.audio.settings.kiemTraTruocGiong },
  { id: 'standard', label: t.audio.settings.chuan, hint: t.audio.settings.duTotDeDang },
  { id: 'high', label: t.audio.settings.caoNhat, hint: t.audio.settings.muotVaTuNhien },
]

const errText = (error: unknown) => error instanceof Error ? error.message : t.audio.settings.khongKetNoiDuoc

function useDelayedNotice(active: boolean, delay: number) {
  const [visible, setVisible] = useState(false)
  useEffect(() => {
    setVisible(false)
    if (!active) return
    const timer = window.setTimeout(() => setVisible(true), delay)
    return () => window.clearTimeout(timer)
  }, [active, delay])
  return active && visible
}

function ResourceFeedback({ resource, label, rows = 1 }: { resource: Resource<unknown>; label: string; rows?: number }) {
  const loading = resource.data === undefined && resource.initialLoading
  const slow = useDelayedNotice(loading, 8000)
  return (
    <>
      <PanelRefreshBar resourceKeys={[resource.key]} updatedAt={resource.updatedAt} />
      {resource.data !== undefined
        ? <StaleBanner error={resource.error} onRetry={resource.refresh} />
        : resource.error != null && <div role="alert"><ErrorCard message={`${label}: ${errText(resource.error)}`} onRetry={() => void resource.refresh()} /></div>}
      {loading && (
        <div role="status" aria-label={`Đang tải ${label.toLocaleLowerCase('vi-VN')}`} aria-live="polite">
          <Card radius={16} padding="14px 18px" style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
            {Array.from({ length: rows }, (_, index) => (
              <div key={index} style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
                <Skeleton width={30} height={30} radius={15} />
                <div style={{ flex: 1, display: 'flex', flexDirection: 'column', gap: 8 }}>
                  <Skeleton width="45%" height={13} />
                  <Skeleton width="70%" height={11} />
                </div>
                <Skeleton width={70} height={28} />
              </div>
            ))}
            {slow && <span style={{ fontSize: 12, color: 'var(--color-fg-3)' }}>Đang chậm hơn bình thường…</span>}
          </Card>
        </div>
      )}
    </>
  )
}

/** Nghe thử một giọng (blob URL) — nút play nhỏ trong danh sách. */
function PreviewButton({ voiceId, label, hosted }: { voiceId: string; label: string; hosted: boolean }) {
  const connection = useConnection()
  const audio = useRef<HTMLAudioElement | null>(null)
  const objectURL = useRef<string | null>(null)
  const mounted = useRef(false)
  const release = useCallback(() => {
    if (audio.current) {
      audio.current.pause()
      audio.current.removeAttribute('src')
      audio.current.load()
      audio.current = null
    }
    if (objectURL.current) {
      URL.revokeObjectURL(objectURL.current)
      objectURL.current = null
    }
  }, [])
  useEffect(() => {
    mounted.current = true
    return () => { mounted.current = false; release() }
  }, [release])
  const preview = useAction(async () => {
    // iOS: thẻ audio phải được play() đồng bộ trong cử chỉ trước khi chờ API,
    // và dùng lại đúng thẻ đó cho lượt phát thật (thẻ mới sau await bị chặn).
    const element = audio.current ?? new Audio()
    element.setAttribute('playsinline', '')
    if (audio.current !== element) element.addEventListener('ended', () => { if (audio.current === element) release() }, { once: true })
    audio.current = element
    unlockMedia(element)
    const url = await previewTtsVoice(voiceId, t.audio.settings.xinChaoToiLa)
    if (!mounted.current) { URL.revokeObjectURL(url); return }
    release()
    audio.current = element
    objectURL.current = url
    element.src = url
    try {
      await element.play()
    } catch {
      release()
      if (mounted.current) throw new Error('Không phát được giọng nghe thử. Vui lòng thử lại.')
    }
  })
  const slow = useDelayedNotice(preview.busy, hosted ? 0 : 2000)
  return (
    <span style={{ display: 'flex', flexDirection: 'column', alignItems: 'flex-end', gap: 4 }}>
      <Button size="sm" variant="icon" aria-label={t.audio.settings.ngheThu(label)}
        title={connection.writable ? t.audio.settings.ngheThu(label) : 'Đang mất kết nối · Chờ kết nối lại để nghe thử'}
        pending={preview.pending} success={preview.success} pendingLabel={null} successLabel={null}
        disabled={preview.busy || !connection.writable} onClick={() => void preview.run()}
        style={{ width: 28, height: 28, borderRadius: '50%', border: '1px solid var(--line-4)' }}>
        <Icon name="play" size={11} />
      </Button>
      <span role="status" aria-live="polite" style={{ fontSize: 11, color: 'var(--color-fg-4)' }}>
        {slow ? (hosted ? 'Đang nhờ máy đọc…' : 'Đang tạo giọng nghe thử…') : preview.success ? 'Đang phát' : ''}
      </span>
    </span>
  )
}

function VoiceRow({ voice, first, preview, hosted }: { voice: AudioVoice; first?: boolean; preview: boolean; hosted: boolean }) {
  const desc = voice.builtIn
    ? t.audio.settings.giongCoSanCua
    : t.audio.settings.giay(voice.file || t.audio.settings.mauGiongDuAn, voice.durationSec ?? 0)
  return (
    <div style={{
      display: 'flex', alignItems: 'center', gap: 12, padding: '10px 18px',
      borderTop: first ? undefined : '1px solid var(--line-1)',
    }}>
      <Icon name={voice.builtIn ? 'sound' : 'microphone-2'} size={16} style={{ color: 'var(--color-fg-4)' }} />
      <span style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column' }}>
        <span style={{ fontSize: 13.5, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{voice.name}</span>
        <span style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>{desc}</span>
      </span>
      {preview && <PreviewButton voiceId={voice.id} label={voice.name} hosted={hosted} />}
    </div>
  )
}

export function VoiceSection({ ctx }: { ctx: SettingsContext }) {
  const statusResource = useResource([...keys.tts(), 'status'], ({ signal }) => getTtsStatus(signal))
  const enginesResource = useResource([...keys.tts(), 'engines'], ({ signal }) => getTtsEngines(signal))
  const status = statusResource.data
  const machineStatus = status as MachineStatus | undefined
  const machine = machineView(machineStatus)
  const machineActions = useMachineCardActions()
  const local = machine.mode === 'local'
  const canPreview = previewSupport(machineStatus) === 'ready'
  const engines = enginesResource.data
  // /projects/:p/voices đã gộp giọng có sẵn của bộ đọc (builtIn) với mẫu của dự án; chỉ dùng một nguồn.
  const voicesResource = useResource(ctx.project ? keys.project(ctx.project, 'voices') : [...keys.tts(), 'voices'],
    ({ signal }) => ctx.project ? getProjectVoices(ctx.project, signal) : getTtsVoices(signal),
    { enabled: Boolean(ctx.project) || machine.ready })
  const voices = voicesResource.data ?? []
  const builtInVoices = voices.filter((voice) => voice.builtIn).length
  // Đang chuẩn bị: làm mới trạng thái theo nhịp ngắn (luồng sự kiện có thể chưa tới).
  const refreshStatus = statusResource.refresh
  useEffect(() => {
    if (machine.kind !== 'provisioning') return
    const timer = window.setInterval(() => { void refreshStatus() }, 1500)
    return () => window.clearInterval(timer)
  }, [machine.kind, refreshStatus])
  // Máy đọc vừa sẵn sàng → giọng có sẵn mới xuất hiện trong danh sách.
  const refreshVoices = voicesResource.refresh
  const wasReady = useRef(machine.ready)
  useEffect(() => {
    if (machine.ready && !wasReady.current) void refreshVoices()
    wasReady.current = machine.ready
  }, [machine.ready, refreshVoices])

  const tts = { ...ctx.effective.tts, ...ctx.draft.tts }
  const quality = tts.quality ?? 'standard'
  const autoOnLock = ctx.audioSettings?.autoOnLock ?? true
  const engine = tts.engine ?? status?.engine ?? 'vieneu'
  const canEdit = ctx.canEdit

  return (
    <Section id="voice" title={t.audio.settings.giongDoc} icon="microphone-2"
      badge={<SectionBadge>{local ? t.audio.settings.docTrenMayCua : 'Máy đọc của thư viện'}</SectionBadge>}>
      {!canEdit && (
        <ReadOnlyNotice>
          <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6 }}>
            <Icon name="shield-tick" size={14} />{t.audio.settings.banChiCoQuyen}</span>
        </ReadOnlyNotice>
      )}

      <ResourceFeedback resource={statusResource} label="Trạng thái máy đọc" />

      {status && (
        <MachineCard variant="settings" view={machine} stepLine={machineStepLine(machineStatus)}
          tiles={[
            { label: 'Máy đọc', value: machine.machineName },
            { label: 'Mẫu giọng', value: builtInVoices > 0 ? `Tiếng Việt · ${builtInVoices} giọng` : 'Tiếng Việt' },
            { label: 'Tốc độ tạo', value: machine.speedLabel ?? 'Đang đo' },
            { label: 'Dung lượng đang dùng', value: machine.diskLabel ?? '—' },
          ]}
          onAction={(kind) => {
            if (!canEdit && (kind === 'retry' || kind === 'prepare')) return
            void machineActions.onAction(kind)
          }} />
      )}

      <Card radius={16} padding={0}>
        <ResourceFeedback resource={enginesResource} label="Danh sách bộ đọc" />
        <Rows>
          {local && <SettingRow first label={t.audio.settings.boDoc} desc={t.audio.settings.chayTrenMayNay}
            field={ctx.fieldState('tts.engine')}>
            {engines && engines.length > 0 ? (
              <InlineSegmented label={t.audio.settings.boDoc} small={false} disabled={!canEdit}
                value={engine}
                onChange={(id) => ctx.setField('tts.engine', id)}
                options={engines.map((item) => ({ id: item.id, label: item.name, disabled: !item.installed && item.id !== 'vieneu' }))} />
            ) : (
              <ReadOnlyValue mono={false}>{engines ? 'Chưa có bộ đọc khả dụng' : engine}</ReadOnlyValue>
            )}
          </SettingRow>}
          <SettingRow first={!local} label={t.audio.settings.chatLuongGiong}
            desc={local ? 'Mặc định cho truyện mới; từng truyện chỉnh ở Phòng thu.' : 'Mặc định cho truyện mới; từng truyện chỉnh ở Phòng thu. Máy đọc dùng mức của truyện.'}
            field={ctx.fieldState('tts.quality')}>
            <InlineSegmented label={t.audio.settings.chatLuongGiong} small={false} disabled={!canEdit}
              value={quality} onChange={(id) => ctx.setField('tts.quality', id)}
              options={QUALITY.map((item) => ({ id: item.id, label: item.label }))} />
          </SettingRow>
          {local && <SettingRow label="Chạy bằng"
            desc="Card đồ hoạ nhanh hơn nếu máy có."
            field={ctx.fieldState('tts.device')}>
            <InlineSegmented label="Chạy bằng" small={false} disabled={!canEdit}
              value={tts.device ?? 'auto'}
              onChange={(id) => { if (id === 'auto') ctx.resetField('tts.device'); else ctx.setField('tts.device', id) }}
              options={[{ id: 'auto', label: 'Tự chọn' }, { id: 'cpu', label: 'Bộ xử lý' }, { id: 'gpu', label: 'Card đồ hoạ' }]} />
          </SettingRow>}
          {ctx.scope === 'story' ? <SettingRow label={t.audio.settings.tuTaoGiongKhi}
            desc="Riêng truyện này · lưu vào audio/settings.json của truyện.">
            <Switch checked={autoOnLock} disabled={!canEdit || !ctx.audioSettings} label={t.audio.settings.tuTaoGiongKhi}
              onChange={ctx.setAutoOnLock} />
          </SettingRow> : <SettingRow label={t.audio.settings.tuTaoGiongKhi}
            desc="Chọn phạm vi Truyện ở trên để thay đổi thiết lập riêng của từng truyện.">
            <ReadOnlyValue mono={false}>Theo từng truyện</ReadOnlyValue>
          </SettingRow>}
        </Rows>
      </Card>
      {ctx.scope === 'story' && ctx.audioSettingsError && <div role="alert">
        <ErrorCard message={ctx.audioSettingsError} onRetry={ctx.refreshAudioSettings} />
      </div>}

      <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 2, padding: '0 2px' }}>
          <h3 style={{ margin: 0, fontSize: 13.5, fontWeight: 600 }}>{t.audio.settings.giongDocCoSan}</h3>
          <span style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>{t.audio.settings.giongRiengDuAn}</span>
        </div>
        {machine.ready ? (
          <>
            <ResourceFeedback resource={voicesResource} label={ctx.project ? 'Giọng của dự án' : 'Giọng có sẵn'} rows={2} />
            {voices.length > 0 && (
              <Card radius={16} padding="4px 0">
                {voices.map((voice, index) => <VoiceRow key={voice.id} voice={voice} first={index === 0} preview={canPreview} hosted={!local} />)}
              </Card>
            )}
          </>
        ) : (
          <Card radius={16} padding="16px 18px" style={{ fontSize: 13, color: 'var(--color-fg-3)' }}>
            {local ? 'Giọng có sẵn hiện ở đây khi bộ đọc sẵn sàng.' : 'Giọng có sẵn hiện ở đây khi máy đọc sẵn sàng.'}
          </Card>
        )}
        <span style={{ padding: '0 2px', fontSize: 12, color: 'var(--color-fg-4)' }}>
          {local ? 'Nghe thử chạy ngay trên máy này.' : canPreview ? 'Nghe thử nhờ máy đọc đọc một câu mẫu, mất vài giây.' : 'Nghe thử hiện khi máy đọc hỗ trợ; giọng vẫn dùng được cho truyện.'}
        </span>
      </div>
    </Section>
  )
}
