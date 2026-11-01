# Editor targeted scene-swap undo

Status: DONE

## Outcome

The scene-swap notice now retains the exact discrete command-history entry and a media-only restoration command. If that entry is still newest, the notice uses ordinary executor undo, preserving the existing redo/CmdZ contract. If later edits exist, the notice executes a discrete compensating `item.update`, restoring only fields changed by the swap, including cleared source fields. It never replays the original whole-item inverse. Later opacity, motion, keyframes, timing and other-clip edits remain untouched. A stale notice whose entry is absent from the active history cannot undo unrelated work.

The compensating restore is itself normally undoable; the original swap and intervening edits remain in history. No shared executor/history API changes were necessary. The existing scene-version commit observer also synchronizes traced media changes from execute commits, so targeted restores send `use { n: previousVersion, project: false }` and reload version labels just like ordinary undo/redo.

## Files

- `apps/viderse/web/src/editor/v13/scene-images.ts`: bind notices to their history entry, retain only changed media fields, select ordinary versus targeted undo, and synchronize targeted version changes.
- `apps/viderse/web/src/editor/v13/scene-images.test.tsx`: add deterministic opacity/other-clip regressions, newest-entry undo/redo coverage, and cleared-history stale-notice coverage. Existing ordinary CmdZ synchronization coverage remains.

## Exercised verification

All commands ran with isolated `HOME=/tmp/verse-editor-undo.CcsONx` and one Vitest worker. No dependencies were installed and no browser, protected ports, production fixtures or real user configuration were used.

1. Fail-before: `npm exec -- vitest run --config .tmp-undo-baseline.config.ts src/editor/v13/scene-images.test.tsx -t 'toast undo restores only media' --maxWorkers=1`.
   - A throwaway Vite pre-load plugin loaded the original `HEAD` scene implementation without changing working source.
   - Both regressions failed: media stayed `m2`; the opacity case also reverted opacity from `0.35` to `1`.
2. Pass-after: `npm exec -- vitest run --config vitest.editor.config.ts src/editor/v13/scene-images.test.tsx --maxWorkers=1`.
   - 15/15 tests passed, including original swap → CmdZ server synchronization, both intervening-edit cases, targeted restore → CmdZ, newest-entry undo/redo and stale-history protection.
3. `npm run typecheck -w apps/viderse/web`.
   - Passed (`tsc -b src/editor/tsconfig.json && tsc --noEmit`).
4. Narrow state smoke: `npm exec -- vitest run --config vitest.editor.config.ts src/editor/v13/.tmp-undo-state-smoke.test.ts --maxWorkers=1`.
   - Passed, one smoke covering both edited-scene and edited-neighbour paths.
   - Exercised actual consumer functions, command executor, Zustand domain stores and `buildTimelineFromStores`; wrote serialized timeline JSON into isolated temporary files and read it back. Exact persisted timeline equality retained later opacity/label, motion, timing, keyframes and all other clips while restoring `m1`, its source URL and original dimensions.
   - HTTP/media-loading were isolated test boundaries, not a live backend. Both paths verified use-version requests `2 → 1` with `project: false`.

Both throwaway runners and their persisted smoke files were removed. No permanent wiring/source-echo tests were added.

## Scope and remaining concerns

No API-images, review, pipeline, storage or shared history implementation was edited. No merge was performed. Live browser/backend persistence was not exercised in this assignment; the parent can run its integrated runtime smoke after cherry-picking this change.
