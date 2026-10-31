import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it } from 'vitest'
import type { SettingsContext } from '../types'
import { configForSave } from '../lib/config'
import { ProviderEditor } from './ProviderEditor'
import { ProvidersSection } from '../sections/providers'

describe('fixed provider protocols', () => {
  it('never sends forbidden endpoints from stale drafts', () => {
    const config = { providers: {
      'openai-codex': { api: 'responses' },
      subscription: { type: 'openai-codex', api: 'responses' },
      'xai-oauth': { api: 'chat' },
      anthropic: { api: 'chat' },
      proxy: { type: 'openai', api: 'responses' },
    } }
    const saved = configForSave(config)
    for (const id of ['openai-codex', 'subscription', 'xai-oauth', 'anthropic']) expect(saved.providers?.[id].api).toBeUndefined()
    expect(saved.providers?.proxy.api).toBe('responses')
    expect(config.providers.subscription.api).toBe('responses')
  })

  it.each(['openai-codex', 'xai-oauth', 'anthropic-oauth', 'anthropic', 'gemini'])('hides Endpoint for %s, including custom names', (type) => {
    const ctx = { draft: { providers: { subscription: { type, api: 'responses' } } }, effective: {}, canEdit: true, scope: 'global' } as unknown as SettingsContext
    expect(renderToStaticMarkup(<ProviderEditor ctx={ctx} id="subscription" removable />)).not.toContain('Endpoint')
  })

  it('marks an invalid entry and keeps the real reason in user-opened details', () => {
    const reason = 'Nhà cung cấp dùng giao thức cố định; lựa chọn điểm cuối chưa hợp lệ'
    const ctx = {
      scope: 'global', canEdit: false, admin: true,
      draft: { providers: { broken: { type: 'xai-oauth', models: [{ name: 'grok-4.7' }] } } },
      effective: {}, response: { providerErrors: { broken: reason } },
    } as unknown as SettingsContext
    const html = renderToStaticMarkup(<ProvidersSection ctx={ctx} />)
    expect(html).toContain('Bắt sửa · Thiết lập nhà cung cấp chưa hợp lệ')
    expect(html).toContain('<summary>Chi tiết kỹ thuật</summary>')
    expect(html).toContain(reason)
  })
})
