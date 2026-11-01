// Thẻ "Mức tự chủ" ở tab Nguồn & tự động (artboard Channel › tab=sources):
// nhóm chọn 3 mức + 4 dòng (Lấy video mới · Duyệt ảnh · Khi gặp lỗi · Video
// chưa đạt 90 điểm), nhãn Tuỳ chỉnh khi bốn dòng không khớp mức nào, ngưỡng điểm
// ảnh 70–95 bước 5 ở dòng Duyệt theo điểm, khung "Không bao giờ tự làm".
// Lên Tự chạy / Tự duyệt hết hỏi xác nhận (confirmAutonomy) rồi mới PATCH
// với confirmed:true. Ưu tiên kênh (Đều đặn / Lượt xem) nằm cùng thẻ Nhịp.
import * as stylex from '@stylexjs/stylex'
import { useState } from 'react'
import { Icon, KitSelect } from '@noverse/ui'
import { Segmented } from '../Segmented'
import { invalidate, setData } from '@noverse/ui/data'
import { Orb } from '../Orb'
import { confirmAutonomy } from '../confirm'
import { t } from '../../i18n/vi'
import { keys } from '../../data/keys'
import { useOnline } from '../../data/status'
import {
  APPROVE_MAX, APPROVE_MIN, APPROVE_STEP, applyPreset, confirmKind, isConfirmRequired, patchAutonomy, presetIds, presetOf,
  readChannelAutonomy, withRow, withThreshold, type Autonomy, type AutonomyRow, type Priority,
} from '../../api/autonomy'
import { ApiError, request } from '../../api/request'
import type { Channel } from '../../api/channels'
import { channelStyles } from '../../screens/channel/channel.stylex'
import { autonomyStyles as sx } from './autonomy.stylex'

const tt = t.autonomy

const OPTIONS: Record<AutonomyRow, readonly string[]> = {
  pick: ['suggest', 'score2', 'all'],
  approve: ['ask', 'score', 'all'],
  onError: ['stop', 'retry_once'],
  belowQuality: ['ask', 'fix_then_ask'],
}
const ROWS: readonly AutonomyRow[] = ['pick', 'approve', 'onError', 'belowQuality']

/** Nhãn một lựa chọn của một dòng; dòng Duyệt ảnh có ngưỡng trong nhãn. */
function optionLabel(row: AutonomyRow, value: string, min: number): string {
  if (row === 'approve' && value === 'score') return tt.rows.approve.score(min)
  const options = tt.rows[row].options as Record<string, string>
  return options[value] ?? value
}

const autonomyValue = (autonomy: Autonomy, row: AutonomyRow): string => (row === 'approve' ? autonomy.approve.mode : autonomy[row])

/** Số video của kênh đang chờ duyệt; không đọc được thì coi như 0 (câu xác nhận bỏ vế đó). */
async function waitingCount(channelId: string): Promise<number> {
  try {
    const result = await request<{ jobs: { stageGroup: string }[] }>(`/api/clone?channel=${encodeURIComponent(channelId)}`)
    return result.jobs.filter((job) => job.stageGroup === 'review').length
  } catch {
    return 0
  }
}

/** Lưu Mức tự chủ / Ưu tiên của một kênh; hỏi xác nhận trước khi tăng quyền tự duyệt. */
export function useAutonomySave(channel: Channel) {
  const online = useOnline()
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const commit = async (patch: { autonomy?: Autonomy; priority?: Priority }, confirmed: boolean) => {
    setSaving(true)
    setError(null)
    try {
      const next = await patchAutonomy(channel.id, confirmed ? { ...patch, confirmed: true } : patch)
      setData(keys.channel(channel.id), next)
      void invalidate(keys.channels())
    } catch (err) {
      setError(err instanceof ApiError && err.status === 0 ? tt.offline : isConfirmRequired(err) ? tt.saveError : err instanceof Error && err.message ? err.message : tt.saveError)
      throw err
    } finally {
      setSaving(false)
    }
  }

  const saveAutonomy = async (autonomy: Autonomy, next: Autonomy) => {
    if (!online) { setError(tt.offline); return }
    const kind = confirmKind(autonomy, next)
    if (kind) {
      const waiting = await waitingCount(channel.id)
      await confirmAutonomy({
        channel: channel.name || t.channels.page.noName,
        kind,
        min: next.approve.min,
        waiting,
        onConfirm: () => commit({ autonomy: next }, true),
      })
      return
    }
    await commit({ autonomy: next }, false).catch(() => undefined)
  }

  const savePriority = async (priority: Priority) => {
    if (!online) { setError(tt.offline); return }
    await commit({ priority }, false).catch(() => undefined)
  }

  return { saving, error, online, saveAutonomy, savePriority }
}

export function AutonomyCard({ channel }: { channel: Channel }) {
  const value = readChannelAutonomy(channel)
  const { saving, error, online, saveAutonomy } = useAutonomySave(channel)

  if (!value) {
    // Máy chủ chưa trả Mức tự chủ (bản cũ) hoặc trả sai hình dạng: không bịa mức.
    return (
      <section {...stylex.props(channelStyles.card)} aria-label={tt.title}>
        <p {...stylex.props(channelStyles.fieldError)} role="alert">{tt.loadError}</p>
      </section>
    )
  }

  const { autonomy } = value
  const preset = presetOf({ pick: autonomy.pick, onError: autonomy.onError, belowQuality: autonomy.belowQuality, approve: autonomy.approve })
  const custom = preset === 'custom'
  const selected = custom ? '' : preset
  const disabled = saving || !online
  const min = autonomy.approve.min

  const setRow = (row: AutonomyRow, next: string) => {
    if (next === autonomyValue(autonomy, row)) return
    void saveAutonomy(autonomy, withRow(autonomy, row, next as never))
  }

  return (
    <section {...stylex.props(channelStyles.card)} aria-label={tt.title} aria-busy={saving}>
      <div {...stylex.props(sx.head)}>
        <span {...stylex.props(sx.orbs)} aria-hidden="true">
          <Orb of="qa" size={14} className={stylex.props(sx.orbQa).className} />
          <Orb of="artist" size={14} className={stylex.props(sx.orbArtist).className} />
        </span>
        <div {...stylex.props(sx.headText)}>
          <span {...stylex.props(sx.titleRow)}>
            <h2 {...stylex.props(sx.title)}>{tt.title}</h2>
            {custom && <span {...stylex.props(sx.tag)}>{tt.custom}</span>}
          </span>
          <span {...stylex.props(sx.sub)}>{tt.sub}</span>
        </div>
      </div>
      <div {...stylex.props(sx.seg)}>
        <Segmented
          label={tt.title}
          value={selected}
         
          options={presetIds.map((id) => ({ id, label: tt.presets[id], disabled, style: { paddingInline: 16 } }))}
          onChange={(id) => {
            if (id === preset) return
            void saveAutonomy(autonomy, applyPreset(autonomy, id as (typeof presetIds)[number]))
          }}
        />
      </div>
      <div {...stylex.props(sx.rows)}>
        {ROWS.map((row) => {
          const copy = tt.rows[row]
          const rowValue = autonomyValue(autonomy, row)
          return (
            <div key={row} {...stylex.props(sx.row)}>
              <div>
                <div {...stylex.props(sx.rowLabel)}>{copy.label}</div>
                <div {...stylex.props(sx.rowHint)}>{copy.hint}</div>
              </div>
              <KitSelect
                aria-label={copy.label}
                value={rowValue}
                disabled={disabled}
                options={OPTIONS[row].map((option) => ({ value: option, label: optionLabel(row, option, min) }))}
                onChange={(next) => setRow(row, next)}
              />
              {row === 'approve' && autonomy.approve.mode === 'score' && (
                <div {...stylex.props(sx.thr)}>
                  <span {...stylex.props(sx.help)}>{tt.threshold}</span>
                  <span {...stylex.props(sx.thrStepper)} role="group" aria-label={tt.threshold}>
                    <button {...stylex.props(sx.thrButton)} type="button" aria-label={tt.thresholdDown} disabled={disabled || min <= APPROVE_MIN}
                      onClick={() => void saveAutonomy(autonomy, withThreshold(autonomy, min - APPROVE_STEP))}>
                      <Icon name="minus" size={13} />
                    </button>
                    <output {...stylex.props(sx.thrValue)} aria-live="polite">{min}</output>
                    <button {...stylex.props(sx.thrButton)} type="button" aria-label={tt.thresholdUp} disabled={disabled || min >= APPROVE_MAX}
                      onClick={() => void saveAutonomy(autonomy, withThreshold(autonomy, min + APPROVE_STEP))}>
                      <Icon name="add" size={13} />
                    </button>
                  </span>
                  <span {...stylex.props(sx.help)}>{tt.thresholdRange}</span>
                </div>
              )}
            </div>
          )
        })}
      </div>
      {(error || !online) && <p {...stylex.props(channelStyles.fieldError)} role="alert">{!online ? tt.offline : error}</p>}
      <div {...stylex.props(sx.guard)}>
        <span {...stylex.props(sx.guardIcon)}><Icon name="info-circle" size={15} /></span>
        <span>{tt.guard}</span>
      </div>
    </section>
  )
}

/** Ưu tiên kênh: Đều đặn / Lượt xem (artboard Channel › thẻ Nhịp). */
export function PrioritySeg({ channel }: { channel: Channel }) {
  const value = readChannelAutonomy(channel)
  const { saving, online, error, savePriority } = useAutonomySave(channel)
  if (!value) return null
  const { priority } = value
  const p = tt.pacing
  return (
    <div {...stylex.props(sx.prio)}>
      <span id={`prio-${channel.id}`} {...stylex.props(sx.prioLabel)}>{p.priority}</span>
      <div {...stylex.props(sx.seg)}>
        <Segmented
          label={p.priority}
          value={priority}
          options={[
            { id: 'steady', label: p.steady, disabled: saving || !online },
            { id: 'views', label: p.views, disabled: saving || !online },
          ]}
          onChange={(id) => { if (id !== priority) void savePriority(id as Priority) }}
        />
      </div>
      <span {...stylex.props(sx.help)}>{priority === 'views' ? p.viewsHint : p.steadyHint}</span>
      {error && <span {...stylex.props(channelStyles.fieldError)} role="alert">{error}</span>}
    </div>
  )
}
