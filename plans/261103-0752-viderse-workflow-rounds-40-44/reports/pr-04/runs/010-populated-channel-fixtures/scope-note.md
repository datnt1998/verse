# V38 channel companion — synthetic fixture scope

**Prepared, not executed by the child. No pass is claimed.** This private, stateful QA artifact supplements rather than replaces the native evidence in `../channel-final-proof/results.json`. Product source, normal git index, credentials, worktrees and artboards were not modified.

## Parent execution

Run this controller sequentially, not alongside the other acceptance controllers:

```sh
node /var/folders/v7/cgy6bv_j1x18zxxk5l7dphw80000gn/T/verse-v38-acceptance-xxkx9151/parallel-acceptance-20261104/channel-fixture-proof/channel-fixture-proof.cjs
```

The executable owner is [channel-fixture-proof.cjs](./channel-fixture-proof.cjs). On execution it writes `results.json` and `SYNTHETIC-*.png` here. It uses the existing local QA origin, installed Playwright and Chrome, and the existing private cookie-file reference from the native controller; it does not install packages or copy credentials. Browser/context closure is in `finally` paths, including inventory and failed cases.

## Evidence boundary

The native results read during preparation record original channels with no linked sources and no avatar/banner; picker eligibility and rendered old identity were therefore unproven. Do not relabel that absence as a product defect or a native pass.

The companion explicitly supplies synthetic rival/source/scan/posting GET responses, anchored to existing owner-channel routes. One rival is linked here; another is linked to another original channel and remains selectable for sharing. Fixture names and full-page screenshots are labeled synthetic. The desktop 34px/two-row contract is distinct from the source-defined mobile 36px grid. Popup/Sheet interactions and normal/reduced-motion behavior are exercised without committing the selected association.

Identity fixtures reuse only a URL returned by the actual native prompt-sample GET inventory and successfully read/decoded from the same origin during the parent run. No sample URL or generated result is invented. `sampleInventory` preserves raw URL, prompt provenance, GET headers/status and decoded dimensions; external/download candidates are skipped without requesting them. If no usable existing sample is available, identity cases remain unproven. The reused sample is **synthetic channel identity**, not native generated branding.

Old-avatar continuity means the loaded old-image DOM node stays mounted and visible beneath the pending overlay during the observed GET-fixture pending interval. It does not mean a real backend ready→generating→ready transition was performed. Banner proof compares the same existing sample with the safe guide on/off; an image too dark to establish exterior dimming yields unproven rather than a manufactured pass.

All browser non-GET requests are aborted before networking; external/download routes are blocked. WebSockets are closed without connecting to the server. There is no APIRequestContext bypass. Download buttons, generation actions, link commits, scanning, follow, unlink and make-video actions are never invoked. Unexpected mutation attempts/downloads/page errors or unhandled synthetic GET routes are recorded and fail bounded fixture acceptance. Only GET fixture responses are synthesized; no mutation success or failure response is fabricated in this companion.

## Remains unproven regardless of fixture passes

- Native followed-rival records and native source associations; real sharing persistence.
- Native generated avatar/banner and real prior-media continuity across provider runs.
- Paid generation/admission, matching real job/activity/progress, generation completion and persisted identity.
- Successful brief save, reload after actual save, link/unlink/follow/create/default-template persistence, and downloads.
- Existing native brief-failure and template-artwork cases, which remain owned by the separate native controller. This companion does not erase or reinterpret those results. The prior brief pending failure was a duplicate exact-text locator issue; the actual feedback owner is `span[aria-live="polite"]`, not accessibility tooltip spans.

Parent acceptance must keep `NATIVE_READ_ONLY_GET_INVENTORY_NOT_UI_ACCEPTANCE` separate from `SYNTHETIC_BROWSER_GET_FIXTURE`. Even `PASS_SYNTHETIC_FIXTURES_ONLY` is not backend-persistence acceptance. Read each case's observations, screenshots, `unsupported`, `unproven`, safety events and closure fields before integrating evidence.
