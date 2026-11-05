export type QuickCreateRequest = { channelId?: string; urls?: string[]; fromSelection?: boolean }
export const QUICK_CREATE_EVENT = 'viderse:quick-create'

/** Shell owns the only composer; opening it never changes the current route. */
export function openQuickCreate(detail: QuickCreateRequest = {}) {
  window.dispatchEvent(new CustomEvent<QuickCreateRequest>(QUICK_CREATE_EVENT, { detail }))
}

export function blocksQuickCreate(target: EventTarget | null): boolean {
  return target instanceof Element && !!target.closest('input, textarea, select, [contenteditable]:not([contenteditable="false"])')
}
