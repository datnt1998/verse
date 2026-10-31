import test from "node:test";
import assert from "node:assert/strict";
import { controlAsCommand, defaultBindings, eventBinding, isReservedBinding, preservesNativeControlKey, resolveShortcut, shortcutLabel, type ShortcutAction } from "./index";

const event = { code: "KeyS", metaKey: true, ctrlKey: false, altKey: false, shiftKey: false };

test("physical shortcuts preserve Command and Control separately", () => {
  assert.equal(eventBinding(event), "mod+KeyS");
  assert.equal(eventBinding({ ...event, metaKey: false, ctrlKey: true, shiftKey: true }), "ctrl+shift+KeyS");
  assert.equal(eventBinding({ ...event, altKey: true, shiftKey: true }), "mod+alt+shift+KeyS");
});

test("reserved browser, operating system and VoiceOver keys cannot dispatch", () => {
  for (const binding of ["mod+KeyW", "mod+shift+KeyR", "mod+Digit3", "mod+Space", "ctrl+alt+KeyK", "mod+ctrl+alt+KeyS", "F5", "alt+ArrowLeft", "mod+shift+Digit4"]) assert.equal(isReservedBinding(binding), true, binding);
  for (const binding of ["mod+KeyS", "mod+shift+KeyZ", "ctrl+Digit1", "mod+alt+KeyK", "KeyK"]) assert.equal(isReservedBinding(binding), false, binding);
});

test("every shipped default avoids reserved bindings", () => {
  for (const [id, bindings] of Object.entries(defaultBindings)) for (const binding of bindings) assert.equal(isReservedBinding(binding), false, `${id}: ${binding}`);
});

test("focused scope wins over global and never crosses into another panel", () => {
  const run = () => {};
  const actions: ShortcutAction[] = [
    { id: "global", label: "Toàn cục", scope: "global", run, bindings: ["KeyK"] },
    { id: "preview", label: "Phát", scope: "preview", run, bindings: ["KeyK"] },
    { id: "inspector", label: "Keyframe", scope: "inspector", run, bindings: ["KeyK"] },
  ];
  assert.equal(resolveShortcut(actions, "KeyK", "preview")?.id, "preview");
  assert.equal(resolveShortcut(actions, "KeyK", "inspector")?.id, "inspector");
  assert.equal(resolveShortcut(actions, "KeyK", "library")?.id, "global");
  assert.equal(resolveShortcut(actions.slice(1), "KeyK", "library"), undefined);
});

test("disabled actions are skipped and an enabled global action remains available", () => {
  const run = () => {};
  const actions: ShortcutAction[] = [
    { id: "timeline", label: "Tách", scope: "timeline", run, bindings: ["KeyS"], enabled: () => false },
    { id: "global", label: "Lưu", scope: "global", run, bindings: ["KeyS"] },
  ];
  assert.equal(resolveShortcut(actions, "KeyS", "timeline")?.id, "global");
  actions[1].enabled = false;
  assert.equal(resolveShortcut(actions, "KeyS", "timeline"), undefined);
});

test("reserved bindings remain blocked even when a registration supplies one", () => {
  assert.equal(resolveShortcut([{ id: "bad", label: "Không được gán", scope: "global", run: () => {}, bindings: ["mod+KeyR"] }], "mod+KeyR", "global"), undefined);
  assert.equal(shortcutLabel("mod+alt+shift+ArrowLeft"), "⌘⌥⇧←");
});

test("native controls keep activation and navigation keys without disabling character actions", () => {
  for (const control of ["button", "summary", "tab", "combobox", "switch"]) assert.equal(preservesNativeControlKey(control, "Space"), true, control);
  for (const code of ["ArrowLeft", "ArrowRight", "Home", "End", "PageUp", "PageDown"]) assert.equal(preservesNativeControlKey("slider", code), true, code);
  assert.equal(preservesNativeControlKey("tab", "ArrowRight"), true);
  assert.equal(preservesNativeControlKey("button", "KeyK"), false);
  assert.equal(preservesNativeControlKey("slider", "KeyK"), false);
  assert.equal(preservesNativeControlKey("region", "ArrowRight"), false);
});

test("Control stands in for Command where there is no Command key, so Ctrl+Z undoes and Ctrl+Shift+Z redoes", () => {
  const undo: ShortcutAction = { id: "undo", label: "Hoàn tác", scope: "global", bindings: ["mod+KeyZ"], run: () => {} } as ShortcutAction;
  const redo: ShortcutAction = { id: "redo", label: "Làm lại", scope: "global", bindings: ["mod+shift+KeyZ"], run: () => {} } as ShortcutAction;
  const ctrlZ = eventBinding({ code: "KeyZ", metaKey: false, ctrlKey: true, altKey: false, shiftKey: false });
  const ctrlShiftZ = eventBinding({ code: "KeyZ", metaKey: false, ctrlKey: true, altKey: false, shiftKey: true });
  assert.equal(resolveShortcut([undo, redo], ctrlZ, "timeline"), undefined);
  assert.equal(resolveShortcut([undo, redo], controlAsCommand(ctrlZ), "timeline"), undo);
  assert.equal(resolveShortcut([undo, redo], controlAsCommand(ctrlShiftZ), "timeline"), redo);
});
