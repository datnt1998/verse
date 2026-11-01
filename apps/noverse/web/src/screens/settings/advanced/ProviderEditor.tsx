// Bộ sửa kỹ thuật của một nhà cung cấp (Nâng cao › Tinh chỉnh model): giao thức, endpoint, base URL, danh sách model
// (cửa sổ ngữ cảnh, json_schema) và xoá nhà cung cấp. Đây là phần đã chuyển khỏi Kết nối AI.
// Thêm/bớt model chỉ làm ở Kết nối AI (bảng chọn trong thẻ); ở đây chỉ sửa phần kỹ thuật của model đã chọn.
import type { ProviderConfig } from '../../../api/types'
import { agentLabel } from '../../../lib/labels'
import { Button } from '@noverse/ui'
import { Checkbox } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { EmptyState } from '@noverse/ui'
import { TextField } from '@noverse/ui'
import { KitSelect } from '@noverse/ui'
import { InlineSegmented } from '../components'
import { providerTypeOptions, rolesUsing } from '../lib/config'
import type { SettingsContext } from '../types'

const mono = { fontFamily: 'var(--font-mono)', fontSize: 12 } as const
const caption = { flex: '0 0 120px', fontSize: 13, color: '#C9C9D1' } as const

export function ProviderEditor({ ctx, id, removable }: { ctx: SettingsContext; id: string; removable: boolean }) {
  const preset = ctx.presets?.providers.find((entry) => entry.id === id) ?? null
  const provider: ProviderConfig = ctx.draft.providers?.[id] ?? {}
  const models = provider.models ?? []
  const type = provider.type || id
  const set = (key: keyof ProviderConfig, value: unknown) => ctx.setField(`providers.${id}.${key}`, value)
  const canEdit = ctx.canEdit
  // Giao thức và Base URL quyết định nơi khoá/token được gửi tới: chỉ đặt ở cài đặt chung.
  const canRoute = canEdit && ctx.scope === 'global'

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 10, padding: '4px 0 12px' }}>
      {!preset && (
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <span style={caption}>Giao thức</span>
          <KitSelect aria-label={`Giao thức của ${id}`} value={provider.type ?? ''} disabled={!canRoute} onChange={(value) => {
            ctx.setField(`providers.${id}`, { ...provider, type: value || undefined, ...((value || id) !== 'openai' ? { api: undefined } : {}) })
          }}
            options={[{ value: '', label: 'Theo tên provider' }, ...providerTypeOptions(provider.type ?? '', (ctx.presets?.providers ?? []).map((entry) => entry.type)).map((value) => ({ value, label: value }))]}
            style={{ width: 200 }} />
        </div>
      )}
      {!preset && type === 'openai' && (
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <span style={caption}>Endpoint</span>
          <InlineSegmented label={`Endpoint của ${id}`} mono small value={provider.api ?? ''} disabled={!canEdit} onChange={(value) => set('api', value)}
            options={[{ id: 'chat', label: 'chat' }, { id: 'responses', label: 'responses' }]} />
        </div>
      )}
      <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
        <label htmlFor={`purl-${id}`} style={caption}>Base URL</label>
        <TextField id={`purl-${id}`} value={provider.base_url ?? ''} disabled={!canRoute} placeholder={preset?.baseURL || undefined}
          onChange={(value) => set('base_url', value)} style={{ flex: '1 1 260px', maxWidth: 360, height: 34, ...mono }} />
      </div>
      <div style={{ display: 'flex', alignItems: 'flex-start', gap: 8 }}>
        <span style={{ ...caption, paddingTop: 6 }}>Model</span>
        <div style={{ flex: '1 1 360px', display: 'flex', flexDirection: 'column', gap: 8, minWidth: 0 }}>
          {models.map((model, index) => {
            const roles = rolesUsing(ctx.effective, id, model.name)
            const update = (patch: Partial<typeof model>) => set('models', models.map((entry, position) => position === index ? { ...entry, ...patch } : entry))
            return <div key={index} style={{ display: 'flex', flexDirection: 'column', gap: 8, padding: '10px 12px', borderRadius: 12, background: 'rgba(255,255,255,.02)', border: '1px solid rgba(255,255,255,0.06)' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                {/* Tên model chọn/bỏ ở Kết nối AI; ở đây chỉ hiện, không đổi/xoá. */}
                <span className="mono" style={{ flex: 1, minWidth: 90, fontSize: 12, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{model.name}</span>
                <TextField aria-label={`Cửa sổ ngữ cảnh ${model.name || index + 1}`} type="number" value={String(model.context_window ?? '')} disabled={!canEdit} placeholder="200000" onChange={(value) => update({ context_window: value === '' ? undefined : Number(value) })} style={{ width: 80, height: 26, padding: 0, fontSize: 11, fontFamily: 'var(--font-mono)', textAlign: 'right', background: 'transparent', border: '1px solid transparent', color: '#A0A0AB' }} />
                <Checkbox checked={!!model.json_schema} disabled={!canEdit} onChange={(value) => update({ json_schema: value })} label={<span className="mono" style={{ fontSize: 11, color: '#A0A0AB' }}>json_schema</span>} />
              </div>
              <div style={{ display: 'flex', alignItems: 'center', gap: 5, flexWrap: 'wrap' }}>
                {roles.length ? roles.map((role) => <span key={role} style={{ padding: '2px 6px', borderRadius: 5, background: 'rgba(255,255,255,.04)', border: '1px solid rgba(255,255,255,0.06)', fontSize: 10, color: '#A0A0AB' }}>{agentLabel(role)}</span>) : <span style={{ fontSize: 11, color: '#8E8E99' }}>Chưa gán vai trò</span>}
              </div>
            </div>
          })}
          {!models.length && <EmptyState icon="cpu" title="Chưa chọn model nào" />}
          <button type="button" className="hov-ghost" onClick={() => ctx.goSection('connect', id)}
            style={{ alignSelf: 'flex-start', height: 28, padding: '0 10px', borderRadius: 7, border: '1px dashed rgba(255,255,255,0.14)', background: 'transparent', color: '#C9C9D1', fontSize: 12, display: 'inline-flex', alignItems: 'center', gap: 5 }}>
            <Icon name="cpu" size={13} />Chọn/bỏ model ở Kết nối AI
          </button>
        </div>
      </div>
      {ctx.scope === 'global' && (
        <div>
          <Button size="sm" disabled={!canEdit || !removable} title={removable ? undefined : 'Không xoá được nhà cung cấp đang dùng hoặc nhà cung cấp cuối cùng'}
            onClick={() => ctx.setField(`providers.${id}`, undefined)} style={{ height: 28, padding: '0 10px', borderRadius: 7, fontSize: 12, color: 'var(--color-err)' }}>
            <Icon name="trash" size={13} />Xoá nhà cung cấp
          </Button>
        </div>
      )}
    </div>
  )
}
