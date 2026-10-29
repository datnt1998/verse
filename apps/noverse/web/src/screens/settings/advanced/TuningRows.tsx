// Nhóm “Tinh chỉnh model” — Settings.dc.html advGroups[2]: Cửa sổ ngữ cảnh mặc định · Ngắt khi model im lặng · Model khi nhập truyện,
// cộng một hàng cho mỗi nhà cung cấp (giao thức, base URL, model — phần đã chuyển khỏi Kết nối AI). Ghi vào bản nháp config.json toàn cục.
import { useState } from 'react'
import { getSystem } from '../../../api/client'
import { keys, useResource } from '../../../data'
import { KitSelect } from '@noverse/ui'
import { TextField } from '@noverse/ui'
import { modelOptionId, modelOptions } from '../lib/config'
import { providerName } from '../sections/connectLogic'
import type { SettingsContext } from '../types'
import { AdvButton, AdvRow } from './AdvGroup'
import {
  DEFAULT_IDLE, IMPORT_ROLES, contextWindowValue, humanDuration, idleSummary, importModelsValue, importRoleModel, isDuration, parseContextWindow,
} from './logic'
import { ProviderEditor } from './ProviderEditor'

type Panel = 'context' | 'idle' | 'import' | `provider:${string}` | null

const editor = { display: 'flex', flexDirection: 'column', gap: 8, padding: '4px 0 12px' } as const
const caption = { flex: '0 0 150px', fontSize: 13, color: '#C9C9D1' } as const
const hint = { fontSize: 12, color: '#8E8E99', lineHeight: 1.5 } as const

export function TuningRows({ ctx }: { ctx: SettingsContext }) {
  const system = useResource(keys.system(), ({ signal }) => getSystem(signal), { staleTime: 60000 })
  const [open, setOpen] = useState<Panel>(null)
  const [windowText, setWindowText] = useState(() => String(ctx.draft.context_window ?? ''))
  const toggle = (panel: Exclude<Panel, null>) => setOpen((current) => (current === panel ? null : panel))

  const providers = ctx.draft.providers ?? {}
  const ids = Object.keys(providers)
  const primary = ctx.draft.provider ?? ctx.effective.provider ?? ''
  const options = modelOptions(ctx.effective)
  const parsedWindow = parseContextWindow(windowText)

  return <>
    <AdvRow label="Cửa sổ ngữ cảnh mặc định" value={contextWindowValue(ctx.draft.context_window, system.data?.defaultContextWindow)}
      below={open === 'context' ? (
        <div style={editor}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <label htmlFor="adv-ctx" style={caption}>Số token</label>
            <TextField id="adv-ctx" inputMode="numeric" value={windowText} disabled={!ctx.canEdit} placeholder="200000"
              aria-invalid={parsedWindow === null || undefined}
              onChange={(value) => {
                setWindowText(value)
                const parsed = parseContextWindow(value)
                if (parsed !== null) ctx.setField('context_window', parsed)
              }} style={{ width: 140, height: 34, fontFamily: 'var(--font-mono)', fontSize: 12 }} />
          </div>
          <span style={hint}>
            {parsedWindow === null ? 'Nhập số nguyên dương, hoặc để trống để dùng mặc định.' : 'Dùng khi model không tự khai báo cửa sổ ngữ cảnh. Để trống = mặc định của máy chủ.'}
          </span>
        </div>
      ) : undefined}>
      <AdvButton expanded={open === 'context'} disabled={!ctx.canEdit} onClick={() => toggle('context')}>Đổi</AdvButton>
    </AdvRow>

    <AdvRow label="Ngắt khi model im lặng" value={idleSummary(ids.map((id) => providers[id]?.stream_idle_timeout))}
      below={open === 'idle' ? (
        <div style={editor}>
          {ids.length === 0 && <span style={hint}>Chưa có nhà cung cấp nào.</span>}
          {ids.map((id) => {
            const value = providers[id]?.stream_idle_timeout ?? ''
            return (
              <div key={id} style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
                <label htmlFor={`pidle-${id}`} className="mono" style={{ ...caption, fontSize: 12 }}>{id}</label>
                <TextField id={`pidle-${id}`} value={value} disabled={!ctx.canEdit} placeholder={DEFAULT_IDLE} aria-invalid={!isDuration(value) || undefined}
                  onChange={(next) => ctx.setField(`providers.${id}.stream_idle_timeout`, next)} style={{ width: 100, height: 34, fontFamily: 'var(--font-mono)', fontSize: 12 }} />
                <span style={hint}>{!isDuration(value) ? 'Dùng dạng 900s, 15m, 1h' : value.trim() ? humanDuration(value) : 'Mặc định'}</span>
              </div>
            )
          })}
          <span style={hint}>Để trống = {DEFAULT_IDLE}. Dùng dạng 900s, 15m.</span>
        </div>
      ) : undefined}>
      <AdvButton expanded={open === 'idle'} disabled={!ctx.canEdit} onClick={() => toggle('idle')}>Đổi</AdvButton>
    </AdvRow>

    <AdvRow label="Model khi nhập truyện" value={importModelsValue(ctx.draft.roles)}
      below={open === 'import' ? (
        <div style={editor}>
          {IMPORT_ROLES.map((role) => {
            const current = importRoleModel(ctx.draft.roles, role.key)
            const known = options.some((option) => modelOptionId(option) === current)
            return (
              <div key={role.key} style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
                <label htmlFor={`m-${role.key}`} style={caption} title={role.desc}>{role.name}</label>
                <KitSelect id={`m-${role.key}`} aria-label={role.name} value={current} disabled={!ctx.canEdit} height={34}
                  options={[
                    { value: '', label: 'Theo Mặc định' },
                    ...options.map((option) => ({ value: modelOptionId(option), label: modelOptionId(option) })),
                    ...(current && !known ? [{ value: current, label: current }] : []),
                  ]}
                  onChange={(value) => {
                    if (!value) { ctx.resetField(`roles.${role.key}`); return }
                    const [provider, ...rest] = value.split('/')
                    ctx.setField(`roles.${role.key}.provider`, provider)
                    ctx.setField(`roles.${role.key}.model`, rest.join('/'))
                  }}
                  style={{ width: 240 }} />
                <span style={hint}>{role.desc}</span>
              </div>
            )
          })}
        </div>
      ) : undefined}>
      <AdvButton expanded={open === 'import'} disabled={!ctx.canEdit} onClick={() => toggle('import')}>Đổi</AdvButton>
    </AdvRow>

    {ids.map((id) => {
      const preset = ctx.presets?.providers.find((entry) => entry.id === id) ?? null
      const provider = providers[id] ?? {}
      const panel = `provider:${id}` as const
      const summary = [provider.type || id, provider.base_url || 'URL mặc định', `${provider.models?.length ?? 0} model`].join(' · ')
      return (
        <AdvRow key={id} label={`Nhà cung cấp · ${providerName(id, preset)}`} value={summary} valueTitle={summary}
          below={open === panel ? <ProviderEditor ctx={ctx} id={id} removable={ids.length > 1 && id !== primary} /> : undefined}>
          <AdvButton expanded={open === panel} disabled={!ctx.canEdit} onClick={() => toggle(panel)}>Đổi</AdvButton>
        </AdvRow>
      )
    })}

  </>
}
