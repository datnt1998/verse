# F6 — confirm() lazy chunk failure fails closed

Status: DONE

## Files

- `platform/ui/src/confirm-store.ts` — catch module-load rejection around the two dynamic imports; show an existing error toast; resolve `false`; no root/container; never call `onConfirm`.
- `platform/ui/src/ConfirmDialog.test.tsx` — two regressions: broken module load fails closed with the toast; a session boundary crossed while the chunk is loading emits no stale-session toast.
- This report.

## Exact failure boundary

Only `Promise.all([import('react-dom/client'), import('./ConfirmDialog')])` is wrapped.

- `.catch()` is chained directly on that `Promise.all`, before the `.then()` that mounts the dialog. It therefore catches only a rejected module load (asset 404 after deploy/hash change, offline fetch, import machinery failure). A synchronous throw or rejection raised later by `onConfirm` is not reachable here: `onConfirm` is invoked by `ConfirmDialog` after mount, and its existing inline-error/retry semantics are untouched. The remembered-answer path is untouched.
- On rejection: `console.error` keeps the cause diagnosable; the promise resolves `false` (contract preserved, callers' `if (!accepted) return` still fires); no `div`/`createRoot` is created, so no dialog, no root, no `activeConfirmations` entry, no hanging UI.
- Visible result: `toast('Không mở được hộp thoại xác nhận. Kiểm tra kết nối rồi tải lại trang để thử lại.', 'err', { label: 'Tải lại', onClick: () => location.reload() })` using the existing Toast host. The action is user-initiated reload, not an automatic import retry; the text states the required action when offline. `onConfirm` never runs, so no trial/start, no mutation.
- Generation boundary: the toast is emitted only when `generation === confirmationGeneration`. If `dismissConfirmations()` runs while the chunk is loading, the store returns `false` without a toast — no stale-session UI is surfaced after an account/session change. The existing post-import generation check before mounting is unchanged.

## Regressions added (not run, per integration contract)

In `ConfirmDialog.test.tsx`, `describe('lỗi tải chunk hộp thoại — đóng, không đoán')`:

1. Mocks the dynamic boundary (`ConfirmDialog` module factory rejects; Toast/`react-dom/client` stubbed), asserts `confirm()` resolves `false`, `onConfirm` is not called, no `[role="alertdialog"]`, no body children, and one `err` toast whose text contains `tải lại trang` with a `Tải lại` action.
2. Calls `dismissConfirmations()` synchronously while the chunk import is pending; asserts the promise resolves `false`, no toast, no `onConfirm`.

Test module reloading is intentional and commented: static imports cannot replace the module graph at the load boundary.

## Proof path for the browser (controlled failure; no provider mock)

1. Build the app for production and inspect `dist/assets`: the dynamic import of `ConfirmDialog` must be emitted as a separately fetched chunk for a network fault to be possible. Note: `platform/ui/src/index.ts:18` still statically re-exports `ConfirmDialog`, and Viderse re-exports it in `apps/viderse/web/src/components/confirm.tsx`, so today the module is already in the boot graph and the dynamic import may resolve from the module registry without a request. Verify the split in the build output first; do not assume it.
2. If split: load the app, open the CoverSection/mobile sample surface, then in DevTools Network block the ConfirmDialog asset (or go offline after boot) and click **Vẽ thử**. Expected: no dialog, error toast with **Tải lại**, no trial/start request, `confirm()` settles false, no console unhandled rejection.
3. If not split: the failure path is not reachable through that click in this build; the unit regression above covers the same boundary. This is a build/chunking question for the deferred gate, not a behavior change here.

## Concerns

- No build/test/lint was run here per the integration contract; the two tests are written but their execution is the integration owner's gate.
- The toast copy is Vietnamese and hardcoded because the kit label set has no key for this failure and `labels.ts` was out of scope; the reload action uses `location.reload()` (user gesture only).
- The fix cannot make the chunk failure more reachable; it is defensive at the correct boundary, so F6 may remain latent if the production build never splits `ConfirmDialog` from the boot chunk.
