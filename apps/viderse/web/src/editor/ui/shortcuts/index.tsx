"use client";

import { createContext, useCallback, useContext, useEffect, useLayoutEffect, useRef, useSyncExternalStore, type ReactNode } from "react";

export type ShortcutScope = "global" | "timeline" | "preview" | "inspector" | "library";
export interface ShortcutAction {
  id: string;
  label: string;
  scope: ShortcutScope;
  run: (event: KeyboardEvent) => void;
  enabled?: boolean | (() => boolean);
  bindings?: readonly string[];
  onRelease?: (event: KeyboardEvent) => void;
}

export const defaultBindings: Readonly<Record<string, readonly string[]>> = {
  save: ["mod+KeyS"], undo: ["mod+KeyZ"], redo: ["mod+shift+KeyZ"], export: ["mod+shift+KeyE"],
  shortcuts: ["shift+Slash", "mod+alt+KeyK"], rename: ["F2"],
  play: ["Space", "KeyK"], reverse: ["KeyJ"], forward: ["KeyL"],
  "frame-back": ["ArrowLeft"], "frame-forward": ["ArrowRight"],
  "second-back": ["shift+ArrowLeft"], "second-forward": ["shift+ArrowRight"],
  start: ["Home"], end: ["End"], "set-in": ["KeyI"], "set-out": ["KeyO"], "clear-in-out": ["alt+KeyX"],
  select: ["KeyV"], trim: ["KeyT"], razor: ["KeyC"], split: ["KeyS"],
  "ripple-head": ["KeyQ"], "ripple-tail": ["KeyW"], snap: ["KeyN"],
  delete: ["Delete", "Backspace"], "ripple-delete": ["shift+Delete", "mod+Delete"], duplicate: ["shift+KeyD"],
  "select-all": ["mod+KeyA"], deselect: ["Escape"], copy: ["mod+KeyC"], cut: ["mod+KeyX"], paste: ["mod+KeyV"],
  "zoom-in": ["Equal", "shift+Equal"], "zoom-out": ["Minus"], "zoom-fit": ["Backslash"], "zoom-reset": ["shift+Backslash"],
  marker: ["KeyM"], "delete-marker": ["shift+KeyM"], "marker-previous": ["BracketLeft"], "marker-next": ["BracketRight"],
  "link": ["mod+alt+KeyL"], "unlink": ["alt+shift+KeyL"], "linked-selection": ["shift+KeyL"],
  "item-previous": ["PageUp"], "item-next": ["PageDown"],
  "nudge-left": ["mod+alt+ArrowLeft"], "nudge-right": ["mod+alt+ArrowRight"],
  "nudge-second-left": ["mod+alt+shift+ArrowLeft"], "nudge-second-right": ["mod+alt+shift+ArrowRight"],
  "track-up": ["alt+shift+ArrowUp"], "track-down": ["alt+shift+ArrowDown"],
  import: ["mod+KeyI"], "toggle-library": ["mod+KeyB"], "toggle-inspector": ["mod+alt+KeyB"], focus: ["Backquote"],
  keyframe: ["KeyK"], "keyframe-previous": ["alt+BracketLeft"], "keyframe-next": ["alt+BracketRight"],
};

let actions: readonly ShortcutAction[] = [];
const subscribers = new Set<() => void>();
const notify = () => subscribers.forEach((listener) => listener());
const subscribe = (listener: () => void) => { subscribers.add(listener); return () => { subscribers.delete(listener); }; };
const emptyActions: readonly ShortcutAction[] = [];
export function useRegisteredShortcuts() { return useSyncExternalStore(subscribe, () => actions, () => emptyActions); }

export function useShortcutAction(action: ShortcutAction) {
  const current = useRef(action);
  useLayoutEffect(() => { current.current = action; }, [action]);
  const bindingsKey = (action.bindings ?? defaultBindings[action.id] ?? []).join("|");
  useEffect(() => {
    const registered: ShortcutAction = {
      id: action.id, label: action.label, scope: action.scope, bindings: bindingsKey ? bindingsKey.split("|") : [],
      run: (event) => current.current.run(event),
      onRelease: (event) => current.current.onRelease?.(event),
      enabled: () => typeof current.current.enabled === "function" ? current.current.enabled() : current.current.enabled !== false,
    };
    actions = [...actions, registered]; notify();
    return () => { actions = actions.filter((entry) => entry !== registered); notify(); };
  }, [action.id, action.label, action.scope, bindingsKey]);
}

const reservedKeys: Readonly<Record<string, readonly string[]>> = {
  mod: ["keyr", "keyt", "keyw", "keyn", "keyl", "keyd", "keyp", "keyf", "keyq", "keyh", "keym", "space", "tab", "equal", "minus", "digit0", "digit1", "digit2", "digit3", "digit4", "digit5", "digit6", "digit7", "digit8", "digit9", "arrowleft", "arrowright"],
  "mod+shift": ["keyr", "keyt", "keyw", "keyn", "keya", "keyo", "equal", "digit3", "digit4", "digit5"],
  "mod+alt": ["keyi", "keyj"],
  alt: ["arrowleft", "arrowright"],
};

export function isReservedBinding(binding: string): boolean {
  const normalized = binding.toLowerCase();
  if (normalized.includes("ctrl+") && normalized.includes("alt+")) return true;
  const split = normalized.lastIndexOf("+");
  const key = normalized.slice(split + 1);
  if (key === "f5" || key === "f11" || key === "f12") return true;
  const modifiers = split < 0 ? "" : normalized.slice(0, split);
  return reservedKeys[modifiers]?.includes(key) ?? false;
}

export function eventBinding(event: Pick<KeyboardEvent, "code" | "metaKey" | "ctrlKey" | "altKey" | "shiftKey">): string {
  let binding = "";
  if (event.metaKey) binding += "mod+";
  if (event.ctrlKey) binding += "ctrl+";
  if (event.altKey) binding += "alt+";
  if (event.shiftKey) binding += "shift+";
  return binding + event.code;
}

/**
 * Outside Apple platforms there is no Command key: people press Control for Undo, Save and friends.
 * Shortcuts are written with the Command-style `mod+` prefix, so Control must be tried as that.
 */
export function controlAsCommand(binding: string): string {
  return binding.replace("ctrl+", "mod+");
}

function isApplePlatform(): boolean {
  return /Mac|iPhone|iPad/i.test(navigator.platform);
}

const controlKeys: Readonly<Record<string, readonly string[]>> = {
  button: ["Space", "Enter"], summary: ["Space", "Enter"], checkbox: ["Space"], switch: ["Space", "Enter"],
  radio: ["Space", "ArrowLeft", "ArrowRight", "ArrowUp", "ArrowDown"],
  slider: ["ArrowLeft", "ArrowRight", "ArrowUp", "ArrowDown", "Home", "End", "PageUp", "PageDown"],
  tab: ["Space", "Enter", "ArrowLeft", "ArrowRight", "ArrowUp", "ArrowDown", "Home", "End"],
  combobox: ["Space", "Enter", "ArrowLeft", "ArrowRight", "ArrowUp", "ArrowDown", "Home", "End"],
};

export function preservesNativeControlKey(control: string, code: string): boolean {
  return controlKeys[control]?.includes(code) ?? false;
}

export function resolveShortcut(registered: readonly ShortcutAction[], binding: string, scope: ShortcutScope): ShortcutAction | undefined {
  if (isReservedBinding(binding)) return undefined;
  let globalAction: ShortcutAction | undefined;
  for (const action of registered) {
    if (action.scope !== scope && action.scope !== "global") continue;
    if (!action.bindings?.includes(binding)) continue;
    if (typeof action.enabled === "function" ? !action.enabled() : action.enabled === false) continue;
    if (action.scope === scope) return action;
    globalAction ??= action;
  }
  return globalAction;
}

export function shortcutLabel(binding: string): string {
  return binding.replaceAll("mod+", "⌘").replaceAll("ctrl+", "⌃").replaceAll("alt+", "⌥").replaceAll("shift+", "⇧")
    .replace(/Key([A-Z])/g, "$1").replace(/Digit(\d)/g, "$1")
    .replace("ArrowLeft", "←").replace("ArrowRight", "→").replace("ArrowUp", "↑").replace("ArrowDown", "↓")
    .replace("Space", "Space").replace("Slash", "/").replace("Backslash", "\\").replace("Backquote", "`");
}

const ScopeContext = createContext<(scope: ShortcutScope) => void>(() => {});
export function useShortcutScope(scope: ShortcutScope) { const setScope = useContext(ScopeContext); return () => setScope(scope); }
export function ShortcutProvider({ children }: { children: ReactNode }) {
  const activeScope = useRef<ShortcutScope>("preview");
  const setScope = useCallback((scope: ShortcutScope) => { activeScope.current = scope; }, []);
  useEffect(() => {
    const held = new Map<string, ShortcutAction>();
    const keydown = (event: KeyboardEvent) => {
      const root = document.querySelector<HTMLElement>("[data-editor-root]");
      if (!root || root.getClientRects().length === 0) return;
      const target = event.target instanceof Element ? event.target : null;
      if (event.defaultPrevented || target?.closest("input, textarea, select, [contenteditable]:not([contenteditable='false']), [role='textbox']") || document.querySelector("[aria-modal='true'], [role='menu'][data-state='open']")) return;
      const control = target?.closest<HTMLElement>("button, summary, [role]");
      const controlKind = control?.getAttribute("role") ?? control?.tagName.toLowerCase() ?? "";
      if (!event.metaKey && !event.ctrlKey && !event.altKey && preservesNativeControlKey(controlKind, event.code)) return;
      const binding = eventBinding(event);
      const focused = target?.closest<HTMLElement>("[data-shortcut-scope]")?.dataset.shortcutScope as ShortcutScope | undefined;
      const chosenScope = focused ?? activeScope.current;
      const action = resolveShortcut(actions, binding, chosenScope)
        ?? (event.ctrlKey && !event.metaKey && !isApplePlatform() ? resolveShortcut(actions, controlAsCommand(binding), chosenScope) : undefined);
      if (!action) return;
      // Character shortcuts require an editor region, never a bare document focus.
      if (!event.metaKey && !event.ctrlKey && !event.altKey && !target?.closest("[data-editor-root]")) return;
      event.preventDefault(); held.set(event.code, action); action.run(event);
    };
    const keyup = (event: KeyboardEvent) => { held.get(event.code)?.onRelease?.(event); held.delete(event.code); };
    const blur = () => {
      for (const [code, action] of held) action.onRelease?.(new KeyboardEvent("keyup", { code }));
      held.clear();
    };
    document.addEventListener("keydown", keydown); document.addEventListener("keyup", keyup); window.addEventListener("blur", blur);
    return () => { blur(); document.removeEventListener("keydown", keydown); document.removeEventListener("keyup", keyup); window.removeEventListener("blur", blur); };
  }, []);
  const trackScope = (event: React.SyntheticEvent) => {
    const target = event.target instanceof Element ? event.target.closest<HTMLElement>("[data-shortcut-scope]") : null;
    if (target?.dataset.shortcutScope) setScope(target.dataset.shortcutScope as ShortcutScope);
  };
  return <ScopeContext value={setScope}><div className="contents" onFocusCapture={trackScope} onPointerDownCapture={trackScope}>{children}</div></ScopeContext>;
}
