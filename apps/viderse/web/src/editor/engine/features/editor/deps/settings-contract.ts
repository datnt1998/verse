/**
 * Adapter exports for settings dependencies.
 * Editor modules should import settings stores/services from here.
 * Settings UI (hotkey editor, model cache controls) is product chrome and is
 * rebuilt later; only store/logic exports live on the engine side.
 */

export {
  useSettingsStore,
  CAPTIONING_INTERVAL_BOUNDS,
  DEFAULT_CAPTIONING_INTERVAL_SECONDS,
  resolveCaptioningIntervalSec,
} from '@editor/features/settings/stores/settings-store'
export type { CaptioningIntervalUnit } from '@editor/features/settings/stores/settings-store'
export { useResolvedHotkeys } from '@editor/features/settings/hooks/use-resolved-hotkeys'
