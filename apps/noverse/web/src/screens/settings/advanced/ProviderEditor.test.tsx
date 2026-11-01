import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it } from 'vitest'
import type { SettingsContext } from '../types'
import { configForSave } from '../lib/config'
import { ProviderEditor } from './ProviderEditor'

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

  it.each([['global', false], ['project', true], ['story', true]])('locks Base URL outside global (%s)', (scope, locked) => {
    const ctx = { draft: { providers: { relay: { base_url: 'https://models.example/v1' } } }, effective: {}, canEdit: true, scope } as unknown as SettingsContext
    const html = renderToStaticMarkup(<ProviderEditor ctx={ctx} id="relay" removable />)
    const input = html.match(/<input[^>]*id="purl-relay"[^>]*>/)?.[0] ?? ''
    expect(input).not.toBe('')
    expect(/\sdisabled(=|\s|>)/.test(input)).toBe(locked)
  })

})
