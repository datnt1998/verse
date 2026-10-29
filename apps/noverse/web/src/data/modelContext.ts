import { getConfig, getConfigModels, putConfig } from '../api/client'
import { configForSave } from '../screens/settings/lib/config'
import { invalidate } from '@noverse/ui/data'
import { useResource } from './useResource'

export const modelContextKey = (provider: string, model: string) => ['model-context', provider, model] as const
/** Every context editor subscribes to this exact per-provider/model cache entry. */
export function useModelContext(provider: string, model: string) {
  return useResource(modelContextKey(provider, model), async () => (await getConfigModels(provider, [model])).models.find(entry => entry.name === model), { enabled: !!provider && !!model })
}
export type ContextChange = { provider: string; model: string; window: number | null }
/** Use the latest global version, preserving unrelated settings and model metadata. */
export async function saveModelContexts(changes: ContextChange[], admin: boolean) {
  if (!admin || !changes.length) return
  const current = await getConfig('global')
  const config = configForSave(current.target.config)
  const providers = { ...config.providers }
  for (const change of changes) {
    const pc = providers[change.provider] ?? {}
    const models = [...(pc.models ?? [])]
    const index = models.findIndex(entry => entry.name === change.model)
    const entry = { ...(index >= 0 ? models[index] : { name: change.model }) }
    if (change.window === null) delete entry.context_window
    else entry.context_window = change.window
    if (index >= 0) models[index] = entry
    else models.push(entry)
    providers[change.provider] = { ...pc, models }
  }
  await putConfig('global', { ...config, providers }, undefined, undefined, current.version)
  await Promise.all([invalidate(['model-context']), invalidate(['settings-model-catalog']), invalidate(['config'])])
}
