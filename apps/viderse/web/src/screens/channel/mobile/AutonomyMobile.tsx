// Mức tự chủ + Ưu tiên kênh trên điện thoại (artboard MChannel › tab Nguồn):
// nhóm chọn 3 mức cao 40 trong khung 48, bốn dòng 56 mở sheet chọn, ngưỡng
// điểm ảnh 70–95 bước 5. Dùng chung useAutonomySave với bản desktop nên luồng
// xác nhận Tự chạy / Tự duyệt hết vẫn y nguyên.
import * as stylex from '@stylexjs/stylex'
import { useState } from 'react'
import { Icon, Segmented, Sheet } from '@noverse/ui'
import { Orb } from '../../../components/Orb'
import { t } from '../../../i18n/autonomy'
import {
  APPROVE_MAX, APPROVE_MIN, APPROVE_STEP, applyPreset, presetIds, presetOf, readChannelAutonomy, withRow, withThreshold,
  type Autonomy, type AutonomyRow, type Priority,
} from '../../../api/autonomy'
import type { Channel } from '../../../api/channels'
import { useAutonomySave } from '../../../components/autonomy/AutonomyCard'
import { mch } from './mchannel.stylex'

const tt = t.autonomy
const OPTIONS: Record<AutonomyRow, readonly string[]> = {
  pick: ['suggest', 'score2', 'all'],
  approve: ['ask', 'score', 'all'],
  onError: ['stop', 'retry_once'],
  belowQuality: ['ask', 'fix_then_ask'],
}
const ROWS: readonly AutonomyRow[] = ['pick', 'approve', 'onError', 'belowQuality']

function optionLabel(row: AutonomyRow, value: string, min: number): string {
  if (row === 'approve' && value === 'score') return tt.rows.approve.score(min)
  const options = tt.rows[row].options as Record<string, string>
  return options[value] ?? value
}

/** Ưu tiên kênh: Đều đặn / Lượt xem — nhóm chọn 40 trong khung 48 của điện thoại. */
export function PrioritySegMobile({ channel }: { channel: Channel }) {
  const value = readChannelAutonomy(channel)
  const { saving, online, error, savePriority } = useAutonomySave(channel)
  if (!value) return null
  const p = tt.pacing
  return (
    <div {...stylex.props(mch.runCol)}>
      <span {...stylex.props(mch.prioLabel)}>{p.priority}</span>
      <Segmented
        label={p.priority}
        value={value.priority}
        size="xl"
        radius={10}
        fit="fill"
        options={[
          { id: 'steady', label: p.steady, disabled: saving || !online },
          { id: 'views', label: p.views, disabled: saving || !online },
        ]}
        onChange={(id) => { if (id !== value.priority) void savePriority(id as Priority) }}
      />
      <span {...stylex.props(mch.help)}>{value.priority === 'views' ? p.viewsHint : p.steadyHint}</span>
      {error && <span role="alert" {...stylex.props(mch.error)}>{error}</span>}
    </div>
  )
}

export function AutonomyCardMobile({ channel }: { channel: Channel }) {
  const value = readChannelAutonomy(channel)
  const { saving, error, online, saveAutonomy } = useAutonomySave(channel)
  const [openRow, setOpenRow] = useState<AutonomyRow | null>(null)
  if (!value) return null
  const { autonomy } = value
  const preset = presetOf(autonomy)
  const disabled = saving || !online
  const min = autonomy.approve.min

  const setRow = (row: AutonomyRow, next: string) => {
    const after = row === 'approve'
      ? withRow(autonomy, 'approve', next as Autonomy['approve']['mode'])
      : row === 'pick'
        ? withRow(autonomy, 'pick', next as Autonomy['pick'])
        : row === 'onError'
          ? withRow(autonomy, 'onError', next as Autonomy['onError'])
          : withRow(autonomy, 'belowQuality', next as Autonomy['belowQuality'])
    void saveAutonomy(autonomy, after)
    setOpenRow(null)
  }

  return (
    <section aria-label={tt.title} {...stylex.props(mch.card16)}>
      <div {...stylex.props(mch.headRow)}>
        <span {...stylex.props(mch.orbs)}><Orb of="qa" size={22} standalone /></span>
        <div {...stylex.props(mch.headText)}>
          <div {...stylex.props(mch.row6)}>
            <span {...stylex.props(mch.cardTitle)}>{tt.title}</span>
            {preset === 'custom' && <span {...stylex.props(mch.tag)}>{tt.custom}</span>}
          </div>
          <span {...stylex.props(mch.cardSub)}>{tt.sub}</span>
        </div>
      </div>

      <Segmented
        label={tt.title}
        value={preset}
        size="xl"
        radius={10}
        fit="fill"
        options={presetIds.map((id) => ({ id, label: tt.presets[id], disabled }))}
        onChange={(id) => { if (id !== preset) void saveAutonomy(autonomy, applyPreset(autonomy, id as (typeof presetIds)[number])) }}
      />

      <div>
        {ROWS.map((row) => {
          const rowValue = row === 'approve' ? autonomy.approve.mode : autonomy[row]
          const copy = tt.rows[row]
          return (
            <div key={row}>
              <button
                type="button"
                aria-label={copy.label}
                aria-haspopup="dialog"
                disabled={disabled}
                onClick={() => setOpenRow(row)}
                {...stylex.props(mch.autonomyRow)}
              >
                <span {...stylex.props(mch.autonomyRowText)}>
                  <span {...stylex.props(mch.autonomyLabel)}>{copy.label}</span>
                  <span {...stylex.props(mch.autonomyCur)}>{optionLabel(row, rowValue, min)}</span>
                </span>
                <Icon name="arrow-right-2" size={16} xstyle={mch.chev} />
              </button>
              {row === 'approve' && autonomy.approve.mode === 'score' && (
                <div {...stylex.props(mch.thrRow)}>
                  <span {...stylex.props(mch.paceLabel)}>{tt.threshold}</span>
                  <span role="group" aria-label={tt.threshold} {...stylex.props(mch.thrStepper)}>
                    <button type="button" aria-label={tt.thresholdDown} disabled={disabled || min <= APPROVE_MIN}
                      onClick={() => void saveAutonomy(autonomy, withThreshold(autonomy, min - APPROVE_STEP))}
                      {...stylex.props(mch.iconBtn32)}>
                      <Icon name="minus" size={14} />
                    </button>
                    <output aria-live="polite" className={`num ${stylex.props(mch.thresholdValue).className}`}>{min}</output>
                    <button type="button" aria-label={tt.thresholdUp} disabled={disabled || min >= APPROVE_MAX}
                      onClick={() => void saveAutonomy(autonomy, withThreshold(autonomy, min + APPROVE_STEP))}
                      {...stylex.props(mch.iconBtn32)}>
                      <Icon name="add" size={14} />
                    </button>
                  </span>
                </div>
              )}
            </div>
          )
        })}
      </div>

      {(error || !online) && <p role="alert" {...stylex.props(mch.error)}>{!online ? tt.offline : error}</p>}

      <div {...stylex.props(mch.help, mch.guardRow)}>
        <Icon name="info-circle" size={15} />
        <span>{tt.guard}</span>
      </div>

      <Sheet
        open={openRow !== null}
        onClose={() => setOpenRow(null)}
        title={openRow ? tt.rows[openRow].label : undefined}
        subtitle={openRow ? tt.rows[openRow].hint : undefined}
        size="md"
      >
        <div role="radiogroup" aria-label={openRow ? tt.rows[openRow].label : tt.title} {...stylex.props(mch.choiceList)}>
          {openRow && OPTIONS[openRow].map((option) => {
            const current = openRow === 'approve' ? autonomy.approve.mode : autonomy[openRow]
            const on = option === current
            return (
              <button
                key={option}
                type="button"
                role="radio"
                aria-checked={on}
                disabled={disabled}
                onClick={() => setRow(openRow, option)}
                {...stylex.props(mch.choice, on && mch.choiceOn)}
              >
                <span aria-hidden="true" {...stylex.props(mch.radio, on && mch.radioOn)}>{on && <span {...stylex.props(mch.radioDot)} />}</span>
                <span>{optionLabel(openRow, option, min)}</span>
              </button>
            )
          })}
        </div>
      </Sheet>
    </section>
  )
}
