# PR2 · quota-0 acceptance + production confirm-chunk failure — exact map

Status: **DONE_WITH_CONCERNS** (read-only discovery; no source edit, no test/browser/provider run, no extra image draw).

Scope: the two remaining runtime claims of PR2 —
1. a real paid-image quota of `0` is represented honestly by the template-cover and prompt-sample trial surfaces;
2. the production lazy `ConfirmDialog` chunk can fail for real from **Account → Đăng xuất**, with no provider involved and no logout request sent.

---

## 1. Discovered files and owners

| Area | File | Relevant boundary |
|---|---|---|
| Prompt sample surface | `apps/viderse/web/src/screens/library/PromptEditor.tsx` | `trialLockedReason` L88–94; `askSample` guard L107–109; draw button L352–371 (`lockedReason` L368); viewer footer draw L427 |
| Template cover surface | `apps/viderse/web/src/screens/library/template/CoverSection.tsx` | `drawLockedReason` L77; `quotaText` L79–84; `draw` guard L86–87; draw button L220–228; visible hint L232–236; viewer footer L327 |
| Prompt trial data | `apps/viderse/web/src/screens/library/usePromptTrial.ts`; `.../template/useCoverTrial.ts` | `start()` POSTs `confirm:true` only after acceptance; status/ready is server-owned |
| Wire types | `apps/viderse/web/src/api/cover-trials.ts` | `TrialQuota` L23–31, `CoverTrialStatus` L58–67, `PromptSampleStatus` L81–90 |
| Server status builder | `apps/viderse/internal/covertrial/service.go` | quota guard L299–305 / L514–520; `coverBlocked` L859–876; `sampleBlocked` L878–893; `refillRecovery` L906–911 |
| Quota wiring | `apps/viderse/cmd/viderse/wire_ai.go` L441–449; `.../covertrial_wiring.go` L119–148 | env `VIDERSE_DAILY_PAID_IMAGE_LIMIT` 1–50; `Left=max(0,limit-charged)` |
| Shared confirm store | `platform/ui/src/confirm-store.ts` | dynamic imports L31; catch/toast L34–37; `onConfirm` never runs on load failure |
| Confirm export surface | `platform/ui/src/index.ts:18`; `apps/viderse/web/src/components/confirm.tsx:9`; `.../components/index.ts:12–13` | static re-exports exist, but are tree-shaken out of the eager entry (see §4) |
| Logout caller | `apps/viderse/web/src/screens/account/AccountScreen.tsx:3,44–47`; `.../api/session.tsx:218–231` | `signOut()` calls `confirm()` before `logout()` |
| Production chunk rule | `apps/viderse/web/scripts/mobile-route-chunks.ts:5–15,47–49` | `ConfirmDialog.tsx` is forced into the `shared-controls` manual chunk |
| Existing browser harness | `tests/viderse-workflow/playwright.config.ts`; `viewer-audit.pw.ts` | config requires `WORKFLOW_EVIDENCE_DIR`, loopback `WORKFLOW_APP_URL`, `PW_BROWSER_BINARY`; existing logout test at `viewer-audit.pw.ts:544–570` |

---

## 2. Required runtime state for genuine quota `0`

No ledger edit, no extra draw. All conditions must hold together; otherwise the visible reason is a **different** real blocker, not quota.

1. Embedded server env: `VIDERSE_DAILY_PAID_IMAGE_LIMIT=2` (accepted range 1–50, `wire_ai.go:442–447`).
2. Same local day, journal charged exactly **2** paid images for the primary provider (`chatgpt`, label `ChatGPT`). Current evidence: one native cover trial (`reports/pr-02/runs/21-real-cover-native-trial/native-cover-ready.json`, provider `chatgpt`, `paid:true`) and one scene image version (`provider-budget-and-cleanup.json`, `sceneImageVersion:2`).
3. `resolveDrawer` succeeds: provider access restored for **read-only readiness**, no draw. If provider access is absent, `resErr` wins over quota and the reason becomes `Chưa sẵn sàng vẽ ảnh` / provider copy.
4. At least one succeeded video source exists. Evidence: job `3225b629-51c8-4f81-8456-9ecacde664ac`, status `succeeded` (`real-source-completed.json`). `sources.length===0` wins over quota in both `coverBlocked` and `sampleBlocked`.
5. Run before `resetAt` (current evidence `2026-11-05T00:00:00+07:00`). After local midnight `Left` returns to `2` and this state is gone.

Expected API shape, both endpoints:

- `GET /api/templates/<templateId>/cover-trials`
- `GET /api/image-prompts/<promptId>/samples`

```json
{
  "sources": [ { "jobId": "3225b629-…", "title": "…", "durationSec": 6 } ],
  "quota": { "provider": "chatgpt", "label": "ChatGPT", "paid": true, "left": 0, "total": 2, "resetAt": "…" },
  "cost": { "images": 1, "provider": "codex", "label": "ChatGPT", "quotaLeft": 0 },
  "ready": false,
  "blockedReason": "Đã hết hạn mức ảnh hôm nay · Chờ đến HH:MM DD/MM rồi thử lại"
}
```

Exact reason pattern (`covertrial/service.go:872–873,889–890` + `refillRecovery` time format `15:04 02/01`):

```js
/^Đã hết hạn mức ảnh hôm nay · Chờ đến \d{2}:\d{2} \d{2}\/\d{2} rồi thử lại$/
```

Server-side guards independently reject a bypassed start with the same reason (`service.go:299–305`, `:514–520`), but the UI never reaches either POST while `ready:false`.

---

## 3. Exact UI controls / state boundaries at quota `0`

`ActionButton` with `lockedReason` renders a real `<button aria-disabled="true" aria-describedby="…">` plus an `sr-only` reason span; `Button` replaces `onClick` with `preventDefault` (`components/ActionButton.tsx:34–40`, `platform/ui/src/Button.tsx:98–100,143`). Therefore clicks are accepted by Playwright but perform nothing.

### 3.1 Template › Ảnh bìa (`/#/templates/<id>?section=cover`)

- Region: `getByRole('region', { name: 'Ảnh bìa', exact: true })` (desktop uses `aria-labelledby="vt-cover-title"`, mobile `hideTitle` uses `aria-label`; `CoverSection.tsx:160`).
- Draw button: `region.getByRole('button', { name: /^Vẽ thử · 1 ảnh / })` (`copy.trialCost`, `i18n/library.ts:499`).
- State:
  - `aria-disabled="true"`;
  - accessible description = blocked reason above (`aria-describedby` / tooltip);
  - **visible** quota state = the hint `<p class="vt-hint">` with the same reason (`CoverSection.tsx:232–236`). The text `Hạn mức ChatGPT còn 0/2 ảnh hôm nay.` is **not** rendered when blocked because `blockedReason` branches first.
- Click result: `draw()` returns false before `confirm()` (`CoverSection.tsx:86–87`), so `alertdialog` stays `0`, no POST.
- Optional sample viewer: `region.getByRole('button', { name: 'Xem ảnh', exact: true }).first()` opens `getByRole('dialog')`; its footer draw is also `aria-disabled` with the same description (`CoverSection.tsx:327`). Clicking it closes then reopens the viewer (`drawFromViewer` restores on `false`) and opens no confirmation.

### 3.2 Library › Prompt editor (`/#/library?tab=prompts`, open the QA card)

- Open the existing card: `page.getByRole('button').filter({ hasText: 'Ảnh mẫu QA tự tạo · 1' }).click()` (same path used by `viewer-audit.pw.ts:492–494`).
- Draw button (no viewer open): `page.getByRole('button', { name: /^Vẽ thử · 1 ảnh / })` (`PromptEditor.tsx:352–371`).
- State:
  - `aria-disabled="true"`;
  - accessible description = blocked reason;
  - **no visible quota/reason paragraph** in the editor: the adjacent `p` remains `copy.tryNote` (`PromptEditor.tsx:373`). This is a real copy gap, not a selector problem.
- Critical boundary: do **not** edit the name/prompt/kind first. `trialLockedReason` prioritises `dirty` → `Lưu chỉ dẫn trước khi vẽ thử.` (`PromptEditor.tsx:88–92`), which would mask the quota reason.
- Click result: `askSample()` returns false before `confirm()` (`PromptEditor.tsx:107–109`), no `alertdialog`, no POST.
- Optional viewer: `page.getByRole('button', { name: 'Xem ảnh mẫu', exact: true }).first()`; footer draw `dialog.getByRole('button', { name: /^Vẽ thử · 1 ảnh / })` is locked and likewise cannot open a confirmation (`PromptEditor.tsx:427`).

### 3.3 Existing tests that must NOT run in this state

`WORKFLOW_QA_CONFIRM_CANCEL=1` (`viewer-audit.pw.ts:483–526`) clicks the prompt/cover **viewer** draw button and expects `alertdialog` to open. At `ready:false` the button is locked, so this test must be skipped in the quota-0 run. The review “Vẽ lại” confirmation test (`viewer-audit.pw.ts:68–73`) is a different confirm source and is unaffected because it only cancels.

---

## 4. Is `ConfirmDialog` already statically loaded through AccountScreen?

**No, not in the current production build.** It is inside `shared-controls`, which is a lazy dynamic chunk and is not in AccountScreen's static dependency closure.

Production proof (current `apps/viderse/internal/web/dist/.vite/manifest.json`, build 2026-11-04 04:25; newer than `confirm-store.ts` 03:29):

- Entry: `index.html → assets/index-ifYkv6GT.js`. Its static closure is only itself; `shared-controls` appears only under `dynamicImports`.
- `_shared-controls-DboAEz_g.js` → `assets/shared-controls-DboAEz_g.js`, `isDynamicEntry:true`, imports only the entry + `_Selects`; it contains the ConfirmDialog code (`alertdialog`).
- `src/screens/account/AccountScreen.tsx → assets/AccountScreen-CSSpWWIj.js`; static imports are `index.html`, `_parts`, `ChangePasswordScreen`, `_Banner`. No `shared-controls`, no confirm module.
- The entry's compiled `confirm()` does: `load-full().then(() => import("./shared-controls-DboAEz_g.js"))` and the catch is the toast path (`confirm-store.ts:31–37`). There is no `<link rel="modulepreload">` in `dist/index.html`.
- Static re-exports (`platform/ui/src/index.ts:18`, `components/confirm.tsx:9`, `components/index.ts:12–13`) are tree-shaken out of the entry today. `manualChunks` deliberately places `ConfirmDialog.tsx` in `shared-controls` (`scripts/mobile-route-chunks.ts:5–15`).

**Parent follow-up — `_parts-PVQTzGKN.js` does not preload it.** `AccountScreen` imports `_parts`, but:

- manifest record `_parts-PVQTzGKN.js` has `imports: ["index.html"]`, no `shared-controls`, no dynamic imports;
- the built `assets/parts-PVQTzGKN.js` imports only `./index-ifYkv6GT.js`, contains **0** `shared-controls` references and **0** `alertdialog` occurrences;
- recursive static closure of `src/screens/account/AccountScreen.tsx` is exactly 5 modules (`index.html`, `_parts`, `ChangePasswordScreen`, `_Banner`, self), with no shared/confirm module.

The manifest has no separate `ConfirmDialog` key by design: `ConfirmDialog.tsx` was folded into the `shared-controls` manual chunk, and that chunk is the dynamic import target seen in the entry's compiled `confirm()` (`import("./shared-controls-DboAEz_g.js")`). The embedded logout-cancel run 30 proves the confirm works, but it did not record whether the chunk request happened before the click; only the static graph below distinguishes preloaded vs lazy. There is no loader abstraction to add.

**Reachability constraint:** 41 lazy chunks statically import `shared-controls` (LibraryScreen, TemplateScreen, SettingsScreen, WorkbenchDesktop, ReviewScreen, auth/machine SetupScreen, etc.). Any of those routes loads the chunk first and destroys the failure scenario. Therefore the smoke must boot **directly to `/#/account`** in a fresh context, with a valid `workflow.qa` cookie, and assert:

- the entry script is `/assets/index-<hash>.js` (production build, not Vite dev);
- `location.hash === '#/account'` and the Account screen's `Đăng xuất` button is visible;
- **zero** `/assets/shared-controls-*.js` requests before the click.

Do **not** copy the existing logout test's `#/setup → Cài đặt nâng cao → /#/account` fallback (`viewer-audit.pw.ts:557–562`): both SetupScreen chunks statically import `shared-controls`, so that fallback preloads the chunk and invalidates the test.

---

## 5. Deterministic production chunk-failure smoke (no provider)

Proposed test location: `tests/viderse-workflow/viewer-audit.pw.ts`, immediately after the existing logout test. No `playwright.config.ts` change is needed (`viewer-audit.pw.ts` is already in `testMatch`).

Gate: `test.skip(process.env.WORKFLOW_QA_CONFIRM_CHUNK !== '1', …)`.

**Decision under current evidence.** Do not add another loader or force a fault through a route that preloads `shared-controls`. The current build's direct `/#/account` path is the only reachable route and is conditional on the pre-click assertion; if that assertion ever sees a `shared-controls` request, mark the runtime smoke unreachable and rely on the existing unit boundary regression (`platform/ui/src/ConfirmDialog.test.tsx:223–251`: rejected dynamic module import → `false`, no `onConfirm`, no root, error toast with `Tải lại`). Run the browser smoke only if a real production fault is still required after the native logout-cancel smoke.

Procedure:

1. Fresh test context + real cookie; `page.goto('/#/account')`.
2. Record `page.on('request')` for `/assets/shared-controls-\w+\.js` and every non-GET `/api/**`; record `page.on('console')` errors.
3. Assert production entry, account hash, visible `getByRole('button', { name: 'Đăng xuất', exact: true })`, and `shared.length === 0`.
4. Install `page.route('**/assets/shared-controls-*.js', route => route.abort('failed'))`.
5. Click logout. Expected, all real:
   - one intercepted `shared-controls` request (`shared.length === 1` after `expect.poll`);
   - `page.getByRole('alertdialog')` count `0` (never mounted);
   - toast `getByRole('alert').filter({ hasText: 'Không mở được hộp thoại xác nhận' })` visible, with button `Tải lại`; hover the toast immediately to pause its 4 s timer;
   - console error containing `[confirm] Không tải được hộp thoại xác nhận`;
   - `writes` empty (no `POST /api/logout`), URL still `#/account`, logout button re-enabled after `signOut()` returns false;
   - `context.request.get('/api/me')` still `200` / `workflow.qa`.
6. Do not click `Tải lại`. Attach `{ sharedChunks, alertdialogs:0, toastCopy, writes:[], consoleErrors, session:'workflow.qa' }`.

Why this is not a provider/API mock: only the production lazy asset request is failed; the app's real `confirm()` path, real toast, real session and real account screen run unchanged. `page.on('request')` proving the exact chunk request keeps the test self-validating if a future build inlines `ConfirmDialog`.

If the test sees `shared.length > 0` before the click or `0` after it, fail with “ConfirmDialog no longer live in an isolated lazy chunk” rather than letting the dialog open.

---

## 6. Narrow commands (parent runs; not run here)

Preflight, production split (read-only):

```sh
cd apps/viderse/internal/web/dist
python3 - <<'PY'
import json
m=json.load(open('.vite/manifest.json'))
def closure(k):
    seen=set(); todo=[k]
    while todo:
        x=todo.pop()
        if x in seen: continue
        seen.add(x)
        todo += (m.get(x) or {}).get('imports') or []
    return seen
assert not any('shared-controls' in k for k in closure('src/screens/account/AccountScreen.tsx'))
assert any('shared-controls' in k for k in (m['index.html'].get('dynamicImports') or []))
print('OK: AccountScreen does not statically load shared-controls')
PY
```

Quota-0 acceptance only:

```sh
cd /Users/dat/Projects/verse
WORKFLOW_EVIDENCE_DIR="$PWD/plans/261103-0752-viderse-workflow-rounds-40-44/reports/pr-02/runs/30-quota0" \
WORKFLOW_APP_URL="http://127.0.0.1:<embedded-port>" \
WORKFLOW_QA_FIXTURE="/outside/repo/workflow-qa-fixture.json" \
WORKFLOW_QA_COOKIE_FILE="/outside/repo/workflow-qa-cookies.json" \
PW_BROWSER_BINARY="/path/to/Chrome for Testing" \
WORKFLOW_QA_QUOTA_EXHAUSTED=1 \
npx playwright test -c tests/viderse-workflow/playwright.config.ts viewer-audit.pw.ts \
  --grep "quota exhausted" --project=desktop
```

Production confirm-chunk smoke only:

```sh
cd /Users/dat/Projects/verse
WORKFLOW_EVIDENCE_DIR="$PWD/plans/261103-0752-viderse-workflow-rounds-40-44/reports/pr-02/runs/31-confirm-chunk" \
WORKFLOW_APP_URL="http://127.0.0.1:<embedded-port>" \
WORKFLOW_QA_FIXTURE="/outside/repo/workflow-qa-fixture.json" \
WORKFLOW_QA_COOKIE_FILE="/outside/repo/workflow-qa-cookies.json" \
PW_BROWSER_BINARY="/path/to/Chrome for Testing" \
WORKFLOW_QA_CONFIRM_CHUNK=1 \
npx playwright test -c tests/viderse-workflow/playwright.config.ts viewer-audit.pw.ts \
  --grep "confirm dialog lazy chunk" --project=desktop
```

Both tests must use the **embedded production origin** (served `assets/index-<hash>.js`), not the Vite dev server on 5181/8044.

---

## 7. Concerns / blockers

1. **Prompt editor exposes quota exhaustion only through accessible description/tooltip**, not visible text (`PromptEditor.tsx:373` still shows generic `tryNote`). If acceptance requires a visible `Hạn mức … 0/2`, the current UI does not meet it. CoverSection shows only `Đã hết hạn mức …`, not the numeric `0/2` either.
2. `WORKFLOW_QA_CONFIRM_CANCEL=1` is incompatible with quota `0` (expects a confirmation that the locked viewer footer cannot open). Keep it unset in the quota run.
3. The state expires at local midnight (current `resetAt 2026-11-05T00:00+07:00`). After that, `limit=2` with no new charges yields `left=2`, not `0`; no additional images are authorized to recreate it.
4. Provider access must be restored enough for `resolveDrawer`/`trialQuota`; otherwise `ready:false` is caused by provider readiness, not quota. The test must assert the exact quota object and `blockedReason`, not merely the button state.
5. `sources` must be non-empty; the succeeded job evidence is from the same QA store, but if it is absent, `coverBlocked`/`sampleBlocked` return the source reason before quota.
6. Chunk smoke is coupled to the production chunking decision. It is deterministic only while `ConfirmDialog` remains in a separately fetched `shared-controls-*.js` and no prior route/chunk preload occurs. The test's pre-click zero-request assertion and post-click one-request assertion make that coupling explicit.
7. Run 30 (`30-reviewed-embedded-logout-native-setup-exit`, 4/4 pass) exercised the real logout confirmation but recorded no asset requests. It therefore proves the dialog works, not whether the chunk was fetched lazily; do not read it as “already loaded”.
8. No tests were added and no browser/build/runtime was run by this task; production `dist` must be rebuilt if any PR2 source changes after the observed build.
