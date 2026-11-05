# V40 settings acceptance

Status: DONE_WITH_CONCERNS

## Summary

Executed real headless Google Chrome against the V40 native binary at `http://127.0.0.1:8043/#/settings?section=connect`, after `V40_NATIVE_READY`. Covered 1440×900 desktop and 390×844 phone, each with normal and reduced motion. No repository files changed, no server/build/lint/unit suites started. Browser contexts and browsers closed in finally blocks.

**UI_FIXTURE qualification:** provider/agent GET responses were explicit local response fixtures. Exercised PATCH and DELETE requests were intercepted and fulfilled locally; no mutation was continued to the native server or a provider. No actual credential was read, saved, printed, or uploaded; no logout, provider test/login, image generation, quota change, or DB write was performed. Authentication cookies were loaded privately and remapped to 127.0.0.1 with secure=false, never emitted.

## Observed acceptance

- Desktop key editor is inline (zero modal); phone key/address editors are a single Sheet with no duplicate inline editor.
- Phone password visibility toggle is 44×44; Save, Cancel, and Remove are 358×44 in both motion modes.
- Changed key/address drafts are editable. Cancel closes them; reopening a key editor starts empty. Local-address fixture correctly shows `http://127.0.0.1:11435` instead of a password field.
- Phone Cancel, Escape, scrim click, and native Sheet drag-close all dismiss the editor under normal and reduced motion. The Sheet has **no separate Close button**; its built-in close gesture is the drag handle. No design changes were made to invent one.
- Phone Remove fully unmounts the editor before the danger confirmation. A requestAnimationFrame sampler measured a maximum of **one visible modal** through that handoff, in both motion modes. Escape dismisses the resulting danger confirmation. Cancel issues no DELETE.
- Reproduced the real deferred response path: start first fixture PATCH, cancel editor while pending, reopen and type a second unsaved draft, release old PATCH. The second draft remains present and its Sheet remains open in both motion modes. This is browser response-fixture evidence, not persistence evidence.
- DELETE fixture returns HTTP 500 with `UI_FIXTURE_upstream_internal_exception_marker`. Rendered body never exposes that marker; confirmation shows `Có lỗi xảy ra. Thử lại.` and retains the provider card. No raw upstream text was visible outside advanced (or anywhere in the rendered body).
- Logout confirmation explicitly says new calls stop until sign-in, in-flight images may still finish, fallback occurs only when an enabled usable source exists, otherwise it waits for the user. Existing video/images are preserved. Confirmation was canceled; logout was not performed.
- All settled editor, address, deferred-save, removal, error, and logout checks stayed at at most one visible modal and had no horizontal document/body overflow: exactly 1440px or 390px for their corresponding viewport.

## Source qualification

- `ProviderCard.tsx:362-392`: close increments editSession; save captures revision and only closes the same session after update/invalidation.
- `ConnectSection.test.tsx:371-398`: inspected the existing deferred PATCH gate and reopened-draft assertion; browser controller follows that path. Unit tests were not run.
- `ProviderCard.tsx:395-430`: provider removal wraps untrusted errors into friendly copy before confirmation renders errors.
- `ProviderCard.tsx:565-569` plus kit `Sheet.tsx:60-113`: phone removal handoff is onExited after unmount/scroll-lock release; reduced motion unmounts without waiting for animation.
- `components/confirm.tsx:73-88` uses shared logout copy without a fake fallback option.
- `DevKitScreen.tsx:183-205` uses the same helper; its demo callback only records the action and no fabricated fallback configuration is passed. DevKit was source-inspected, not browser-executed.

## Evidence

- `results.json`: main run, 110 recorded assertions. Two failed assertions were a **controller assumption**, not a discovered product regression: it expected a separate accessible Close button. The adjacent original `close button exits` entries actually used Escape and must not be treated as Close-button evidence.
- `dismissal-supplement.json`: 12/12 passing assertions against the actual Sheet contract, including native drag dismissal and frame-sampled one-modal handoff. No mutation requests occurred during this supplement.
- Safety: main run fulfilled 2 PATCH and 4 DELETE fixture responses locally; zero native/provider mutations were continued; no logout request occurred.
- `desktop-normal-key-editor.png`, `desktop-reduced-key-editor.png`: desktop inline editor.
- `phone-normal-key-editor.png`, `phone-reduced-key-editor.png`: phone Sheet with fixture draft.
- `phone-normal-static-icons-editor.png`, `phone-reduced-static-icons-editor.png`: supplemental phone evidence with local static SVG icons allowed.
- `*-remove-confirm.png`, `*-logout.png`: confirmation evidence for both viewports and both motion preferences.

## Concerns / limits

1. There is no dedicated Close button on kit Sheet. Cancel, scrim, Escape, and drag-close pass. If acceptance literally requires a new explicit Close button, that is not present; no out-of-scope design expansion was attempted. The existing drag area is 390×14, while all editor buttons meet 44px.
2. All provider-state/write/error paths are response-fixture-qualified, not native persistence, real-provider, credential-validity, successful logout, or live upstream-failure proof.
3. The initial run intentionally blocked all image/media requests, which also hid static SVG icon assets in initial screenshots. The focused supplement allows only local static SVGs and still blocks external/media/provider traffic. This does not qualify image generation or media behavior.
4. Desktop Remove keeps its inline editor mounted behind the modal; it never creates a second modal. The edit-unmounted-before-confirm guarantee was observed for the changed phone Sheet path.
