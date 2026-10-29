// Tuỳ chỉnh › Giọng đọc (GAP-21) — trạng thái bộ đọc trên máy, cài đặt, engine, chất lượng,
// thiết bị, tự tạo khi chốt, giọng có sẵn + nghe thử (15-audio-impl.md §1, §3).
// Bộ đọc dùng cấu hình kế thừa; tự tạo khi chốt dùng audio/settings của từng truyện.
// Dữ liệu TTS dùng cache stale-while-revalidate; việc cài chuyển sang dock toàn cục.
import { useCallback, useEffect, useRef, useState } from 'react'
import {
  getProjectVoices, getTtsEngines, getTtsStatus, getTtsVoices, installTts, previewTtsVoice,
} from '../../../api/client'
import type { AudioVoice, TtsStatus } from '../../../api/types'
import { keys, useAction, useConnection, useResource, type Resource } from '../../../data'
import { dispatchDock } from '../../../shell/jobs/store'
import { Button } from '@noverse/ui'
import { Card } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { Switch } from '@noverse/ui'
import { Skeleton } from '@noverse/ui'
import { StaleBanner } from '@noverse/ui'
import { toast } from '@noverse/ui'
import { ErrorCard, InlineSegmented, ReadOnlyNotice, ReadOnlyValue, Rows, Section, SectionBadge, SettingRow } from '../components'
import type { SettingsContext } from '../types'
import { t } from '../../../i18n/vi'
import { unlockMedia } from '../../audio/useAudio'
import { PanelRefreshBar } from '../../../ui/PanelRefreshBar'

const QUALITY = [
  { id: 'fast', label: t.audio.settings.ngheThuNhanh, hint: t.audio.settings.kiemTraTruocGiong },
  { id: 'standard', label: t.audio.settings.chuan, hint: t.audio.settings.duTotDeDang },
  { id: 'high', label: t.audio.settings.caoNhat, hint: t.audio.settings.muotVaTuNhien },
]

const STATE_LABEL: Record<TtsStatus['state'], string> = {
  missing: t.audio.settings.chuaCai,
  installing: t.audio.settings.dangCai2,
  stopped: t.audio.settings.dangTat,
  starting: t.audio.settings.dangKhoiDong,
  ready: t.audio.settings.sanSang,
  error: t.audio.settings.loi,
}

const errText = (error: unknown) => error instanceof Error ? error.message : t.audio.settings.khongKetNoiDuoc

const formatBytes = (bytes: number) => `${(bytes / 1_073_741_824).toFixed(1).replace('.', ',')} GB`

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
function PreviewButton({ voiceId, label }: { voiceId: string; label: string }) {
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
  const slow = useDelayedNotice(preview.busy, 2000)
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
        {slow ? 'Đang tạo giọng nghe thử…' : preview.success ? 'Đang phát' : ''}
      </span>
    </span>
  )
}

function VoiceRow({ voice, first }: { voice: AudioVoice; first?: boolean }) {
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
      <PreviewButton voiceId={voice.id} label={voice.name} />
    </div>
  )
}

export function VoiceSection({ ctx }: { ctx: SettingsContext }) {
  const connection = useConnection()
  const statusResource = useResource([...keys.tts(), 'status'], ({ signal }) => getTtsStatus(signal))
  const enginesResource = useResource([...keys.tts(), 'engines'], ({ signal }) => getTtsEngines(signal))
  const status = statusResource.data
  const engines = enginesResource.data
  // Chưa cài bộ đọc: /tts/voices trả 503 kèm lời nhắc mở chính trang này — dùng trạng thái trống + nút Cài ở trên.
  const engineMissing = !ctx.project && status?.state === 'missing'
  // /projects/:p/voices đã gộp giọng có sẵn của bộ đọc (builtIn) với mẫu của dự án; chỉ dùng một nguồn.
  const voicesResource = useResource(ctx.project ? keys.project(ctx.project, 'voices') : [...keys.tts(), 'voices'],
    ({ signal }) => ctx.project ? getProjectVoices(ctx.project, signal) : getTtsVoices(signal),
    { enabled: Boolean(ctx.project) || (status !== undefined && status.state !== 'missing') })
  const voices = voicesResource.data ?? []
  const install = useAction(async () => {
    const result = await installTts()
    dispatchDock({ type: 'open' })
    toast(t.audio.settings.dangCaiBoDoc, 'ok', {
      label: 'Mở việc chạy nền', onClick: () => dispatchDock({ type: 'open' }),
    })
    return result
  }, { invalidate: [keys.tts(), keys.jobs()] })
  const installSlow = useDelayedNotice(install.busy, 2000)
  const refreshStatus = statusResource.refresh
  useEffect(() => {
    if (status?.state !== 'installing' && status?.state !== 'starting') return
    const timer = window.setInterval(() => { void refreshStatus() }, 1500)
    return () => window.clearInterval(timer)
  }, [status?.state, refreshStatus])
  // Bộ đọc vừa sẵn sàng → giọng có sẵn mới xuất hiện trong danh sách.
  const refreshVoices = voicesResource.refresh
  const engineReady = status?.ready === true
  const wasReady = useRef(engineReady)
  useEffect(() => {
    if (engineReady && !wasReady.current) void refreshVoices()
    wasReady.current = engineReady
  }, [engineReady, refreshVoices])

  const tts = { ...ctx.effective.tts, ...ctx.draft.tts }
  const quality = tts.quality ?? 'standard'
  const device = tts.device ?? 'cpu'
  const autoOnLock = ctx.audioSettings?.autoOnLock ?? true
  const engine = tts.engine ?? status?.engine ?? 'vieneu'
  const canEdit = ctx.canEdit
  const installingNow = status?.state === 'installing' || install.busy

  return (
    <Section id="voice" title={t.audio.settings.giongDoc} icon="microphone-2" keyName="tts"
      badge={<SectionBadge>{t.audio.settings.docTrenMayCua}</SectionBadge>}>
      {!canEdit && (
        <ReadOnlyNotice>
          <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6 }}>
            <Icon name="shield-tick" size={14} />{t.audio.settings.banChiCoQuyen}</span>
        </ReadOnlyNotice>
      )}

      <ResourceFeedback resource={statusResource} label="Trạng thái bộ đọc" />

      {status && (
        <Card radius={16} padding="14px 18px" style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
          <div style={{ display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: 10 }}>
            <Icon name="cpu" size={18} style={{ color: status.ready ? 'var(--color-ok)' : 'var(--color-fg-4)' }} />
            <span style={{ flex: 1, minWidth: 180, display: 'flex', flexDirection: 'column' }}>
              <span style={{ fontSize: 14, fontWeight: 500 }}>
                {t.audio.settings.boDocTrenMay(STATE_LABEL[status.state])}
              </span>
              <span style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>
                {status.state === 'missing'
                  ? t.audio.settings.caiMotLanSau
                  : status.state === 'error'
                    ? status.error
                    : `${status.model} · ${status.device === 'gpu' ? t.audio.settings.cardDoHoa : 'CPU'} · ${Math.round(status.sampleRate / 1000)} kHz${status.rtf !== null ? t.audio.settings.n1PhutAudioGiay(Math.round(status.rtf * 60)) : ''}`}
              </span>
            </span>
            {status.state === 'missing' || status.state === 'error' || install.busy || install.success ? (
              <Button variant="primary" size="sm" icon={<Icon name="export-square" size={14} />}
                pending={install.pending} success={install.success}
                pendingLabel="Đang bắt đầu cài…" successLabel="Đã giao việc"
                title={connection.writable ? t.audio.settings.caiBoDocTren : 'Đang mất kết nối · Chờ kết nối lại để cài bộ đọc'}
                disabled={!canEdit || installingNow || !connection.writable} onClick={() => void install.run()}>
                {t.audio.settings.caiBoDocTren}
              </Button>
            ) : (
              <ReadOnlyValue mono={false}>
                {status.state === 'ready' ? t.audio.settings.dangDung : STATE_LABEL[status.state]}
              </ReadOnlyValue>
            )}
          </div>
          <span role="status" aria-live="polite" style={{ fontSize: 12, color: 'var(--color-fg-3)' }}>
            {installSlow ? 'Đang chuẩn bị bộ đọc và tạo việc cài đặt…' : install.success ? 'Đã chuyển việc cài bộ đọc sang việc chạy nền.' : ''}
          </span>
          {(status.state === 'installing' || status.state === 'starting') && (
            <Button size="sm" icon={<Icon name="arrow-right-2" size={14} />}
              onClick={() => dispatchDock({ type: 'open' })} style={{ alignSelf: 'flex-start' }}>
              Xem tiến độ trong việc chạy nền
            </Button>
          )}
          {!status.ffmpeg && (
            <div style={{
              display: 'flex', gap: 8, padding: '10px 12px', borderRadius: 10,
              background: 'var(--tint-warn)', border: '1px solid rgba(246,203,140,.3)', fontSize: 12.5, color: 'var(--color-fg-2)',
            }}>
              <Icon name="warning-2" size={15} style={{ color: 'var(--color-warn)', marginTop: 2 }} />
              <span>
                {t.audio.settings.mayChuaCo}<b style={{ fontWeight: 500 }}>{t.audio.settings.ffmpeg}</b> {t.audio.settings.vanSoanVaDuyet}</span>
            </div>
          )}
        </Card>
      )}

      <Card radius={16} padding={0}>
        <ResourceFeedback resource={enginesResource} label="Danh sách bộ đọc" />
        <Rows>
          <SettingRow first label={t.audio.settings.boDoc} keyName="tts.engine" desc={t.audio.settings.chayTrenMayNay}
            field={ctx.fieldState('tts.engine')}>
            {engines && engines.length > 0 ? (
              <InlineSegmented label={t.audio.settings.boDoc} small={false} disabled={!canEdit}
                value={engine}
                onChange={(id) => ctx.setField('tts.engine', id)}
                options={engines.map((item) => ({ id: item.id, label: item.name, disabled: !item.installed && item.id !== 'vieneu' }))} />
            ) : (
              <ReadOnlyValue mono={false}>{engines ? 'Chưa có bộ đọc khả dụng' : engine}</ReadOnlyValue>
            )}
          </SettingRow>
          <SettingRow label={t.audio.settings.chatLuongGiong} keyName="tts.quality"
            desc={QUALITY.find((item) => item.id === quality)?.hint}
            field={ctx.fieldState('tts.quality')}>
            <InlineSegmented label={t.audio.settings.chatLuongGiong} small={false} disabled={!canEdit}
              value={quality} onChange={(id) => ctx.setField('tts.quality', id)}
              options={QUALITY.map((item) => ({ id: item.id, label: item.label }))} />
          </SettingRow>
          <SettingRow label={t.audio.settings.thietBi} keyName="tts.device"
            desc={device === 'gpu' ? t.audio.settings.dungCardDoHoa : t.audio.settings.chayBangCpuCham}
            field={ctx.fieldState('tts.device')}>
            <InlineSegmented label={t.audio.settings.thietBi} small={false} disabled={!canEdit}
              value={device} onChange={(id) => ctx.setField('tts.device', id)}
              options={[{ id: 'cpu', label: 'CPU' }, { id: 'gpu', label: t.audio.settings.cardDoHoa }]} />
          </SettingRow>
          {ctx.scope === 'story' ? <SettingRow label={t.audio.settings.tuTaoGiongKhi} keyName="autoOnLock"
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
        <div style={{ display: 'flex', alignItems: 'baseline', gap: 8, padding: '0 2px' }}>
          <h3 style={{ margin: 0, fontSize: 13.5, fontWeight: 600 }}>{t.audio.settings.giongDocCoSan}</h3>
          <span style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>
            {t.audio.settings.mauGiongCuaDu(voices.filter((voice) => !voice.builtIn).length, voices.filter((voice) => voice.builtIn).length)}</span>
        </div>
        <ResourceFeedback resource={voicesResource} label={ctx.project ? 'Giọng của dự án' : 'Giọng có sẵn'} rows={2} />
        {voices.length > 0 ? (
          <Card radius={16} padding="4px 0">
            {voices.map((voice, index) => <VoiceRow key={voice.id} voice={voice} first={index === 0} />)}
          </Card>
        ) : voicesResource.data !== undefined || engineMissing ? (
          <Card radius={16} padding="16px 18px" style={{ fontSize: 13, color: 'var(--color-fg-3)' }}>
            Chưa có giọng đọc. Cài bộ đọc hoặc thêm mẫu giọng trong tài sản dự án.
          </Card>
        ) : null}
        <span style={{ padding: '0 2px', fontSize: 12, color: 'var(--color-fg-4)' }}>
          {t.audio.settings.ngheThuODay(status?.installedBytes ? t.audio.settings.boDocDangDung(formatBytes(status.installedBytes)) : '')}</span>
      </div>
    </Section>
  )
}
