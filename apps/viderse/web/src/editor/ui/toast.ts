/**
 * Sonner-shaped toast adapter for the editor.
 *
 * Every call renders as an action-result pill through the @noverse/ui toast
 * host (`<ToastHost activity />` lives in App.tsx, so it covers the editor
 * route too — no separate host needed here). Sonner options map onto the kit:
 * `description` joins the message with " · " (pills are single-line), `action`
 * passes through, and `undo: true` keeps the 5-second "Hoàn tác · Ns" countdown.
 *
 * Importing this module also installs the engine sink so store actions under
 * `engine/` (which cannot import app modules) surface through the same host.
 */

import { toast as kitToast } from '@noverse/ui'
import { installToastSink } from '@editor/shared/ui/toast'

type ToastAction = { label: string; onClick: () => void | Promise<void> }
type Options = { description?: string; action?: ToastAction; undo?: boolean }

const text = (message: string, options?: Options) =>
  options?.description ? `${message} · ${options.description}` : message

const show = (tone: 'neutral' | 'ok' | 'warn' | 'err' | 'info') => (message: string, options?: Options) =>
  kitToast(text(message, options), tone, options?.action, { undo: options?.undo })

export const toast = Object.assign(
  (message: string, options?: Options) => show('neutral')(message, options),
  {
    error: show('err'),
    success: show('ok'),
    warning: show('warn'),
    info: show('info'),
  },
)

installToastSink(toast)
