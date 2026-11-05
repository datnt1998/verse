# V38 frontend final source review

Status: DONE_WITH_CONCERNS

## Summary

Read-only review of the assigned Library/Template/mobile and Channel profile/identity/source files against accepted phase05 and design decisions #86–90. Also followed the directly imported AttachChannelDialog, PreviewPane, BackdropSection and the named preview API helper where necessary to resolve consumer behavior. No source, index, product data or artboard edits; no commands, tests, browser, server or provider calls.

Three concrete recovery/parity findings remain. These are source-traced findings, not newly exercised native failures. They warrant parent triage before declaring the frontend recovery surface accepted. This report is not a global pixel-parity or physical-Safari approval.

Paths below are relative to `/Users/dat/Projects/verse`; frontend paths share prefix `apps/viderse/web/src/`.

## Findings

### F1 — High: a successful conflict reload silently discards edits made while its GET is pending, then reports a clean saved state

- `screens/library/template/useTemplateDraft.ts:138–149`: `reload()` sets `busy.current = true`, waits for GET, then unconditionally replaces `draft.current` with `fresh`, clears `pending.current`, and publishes `dirty: false` with no error.
- The same hook still accepts recipe edits during that interval (`122–126`): it changes `draft.current` and enqueues a save. `flush()` immediately returns because `busy.current` is true (`48–49`); nothing persists that new edit before reload clears it.
- Desktop exposes Reload from the conflict banner (`screens/library/TemplateScreen.tsx:231–235`) while continuing to render active ImagesSection controls with `onSave={save}` and no reloading lock (`252–254`). Mobile does the same (`screens/library/mobile/MTemplateScreen.tsx:175–182,234–236`). ImagesSection directly sends selection/stepper/switch changes (`screens/library/template/ImagesSection.tsx:27,53–56,107–116,180–181`).

**Consumer path:** get a 409 → choose Reload → before GET finishes, change composition or image count → GET resolves → that *new* choice disappears without a PATCH or retained pending draft. The page then displays the ordinary clean saved state (`TemplateScreen.tsx:109–110`; `MTemplateScreen.tsx:83,191–194`). The warning authorizes discarding the draft existing when Reload was chosen, not silently accepting and discarding fresh edits afterward.

**Required correction boundary:** do not accept editable changes while a reload will overwrite them, or preserve/version changes made after reload starts. Apply the same rule on PC/mobile; no redesign is required.

### F2 — Medium: desktop template-name Escape commits the value it is trying to cancel

- `screens/library/TemplateScreen.tsx:139–143`: `commitName()` closes over the currently rendered `name` and calls `draft.saveName(name)` when it differs from `detail.name`.
- The input saves through `onBlur` (`195–197`). Escape first schedules `setName(detail.name)` and synchronously calls `event.currentTarget.blur()` in the same handler (`198–201`).

**Consumer path [INFERENCE from React event/state semantics]:** with saved name A, type B, press Escape. The scheduled state reset has not produced a new render when synchronous blur invokes the current `commitName` closure, so that closure submits B. If PATCH succeeds, B is persisted despite Escape and returns as the saved name. This is a write-side cancellation bug, not merely an incorrect label. No native write was performed for this review.

**Required correction boundary:** make Escape suppress that blur's commit or commit/cancel against an immediately updated ref; preserve the normal Enter/blur autosave path.

### F3 — Medium: desktop preview can rebuild while its displayed template has unsaved/failed/conflicting changes; mobile correctly blocks this

- `screens/library/template/PreviewPane.tsx:19–29` receives `dirty`, but `canBuild` considers only whether *any* source is buildable and whether the preview request is busy. The rebuild button invokes `build` under that condition (`36,103–110`).
- Desktop supplies only `detail` and `dirty` to PreviewPane (`screens/library/TemplateScreen.tsx:268–269`), not saving/reloading state. Thus a failed PATCH/409 leaves `dirty: true` but does not lock the rebuild action.
- `screens/library/template/useTemplatePreview.ts:67–74` starts the request without checking dirty; the API posts only `{jobId, minutes}` (`api/library.ts:280–281`), not the visible draft recipe or revision.
- Mobile explicitly requires `!draft.saving && !draft.reloading && !draft.dirty` and an enabled selected source before permitting the same operation (`screens/library/mobile/MTemplateScreen.tsx:104–108,300–305`).

**Consumer effect:** after changing settings and encountering save failure/conflict, PC offers an actionable rebuild for the saved server template, not the changes being shown. Mobile explains and blocks that operation. The dirty-state badge remains stale (`useTemplatePreview.ts:56–58`), so this is not a claim that the hook falsely marks that draft fresh; it is an incorrect enabled consumer action and PC/mobile parity gap. A second related weakness is that PC checks any buildable source rather than verifying the currently picked source is still enabled after a recipe/status refresh (`PreviewPane.tsx:25–29`), unlike mobile's `selectedSource` check.

**Required correction boundary:** use the same saved/selected-source eligibility on PC as mobile. Do not solve this by sending draft settings or bypassing revision/conflict behavior.

## Bounded positive observations

- Ordinary failed template writes retain the queued name/recipe (`useTemplateDraft.ts:81–107`); 409 stops further flushing until explicit reload (`48–49,93–95`). The exception is F1's edits arriving during reload.
- Fresh preview status is guarded against dirty state and older built revision (`useTemplatePreview.ts:49–58`); a saved revision triggers source/status refresh (`42–47`). This is useful source evidence, not proof of every native backend preview transition.
- TemplateArtwork chooses only the sample belonging to the attached scene prompt and falls back to a labelled neutral placeholder for missing/failed URLs (`screens/library/TemplateArtwork.tsx:13–18`). Desktop/mobile template cards reuse it (`TemplatesTab.tsx:87–88`; `mobile/MLibraryScreen.tsx:530–532`). Prompt cards likewise fall back instead of copying a fake generated image (`PromptsTab.tsx:36–50`; `mobile/MLibraryScreen.tsx:368–370`). This is consistent with #90.
- The latest static composition illustrations in ImagesSection are option illustrations (`ImagesSection.tsx:51–65`), separate from data-driven template artwork. Their presence is not evidence that a template generated an image. This review did not inspect/rasterize the assets or verify the final styling visually.
- Default-template usage/deletion restrictions use channel references on both desktop/mobile (`TemplatesTab.tsx:42–43,109–117,134–136`; `mobile/MLibraryScreen.tsx:237–270`). The “set default for channel” dialog deliberately navigates to that channel's Template tab rather than claiming an immediate save (`AttachChannelDialog.tsx:4–5,25–30`). No wrong-channel target was found on those inspected paths; this does not independently accept the channel default-picker implementation outside the assigned files.
- The mobile profile entry calls the channel-specific instructions callback separately from global AI settings (`screens/channel/SettingsTab.tsx:375–383`), matching #89 at the inspected entry point. The composer/Sheet implementation itself is outside this review's named source set.
- Identity regeneration checks saved brief state synchronously before the paid call and retains direction on request failure (`screens/channel/IdentityBlock.tsx:73–91`); loading/progress values use actual activity data (`35–49`). No generation was called here.
- BackdropPicker pins its modal shell choice across breakpoint changes and defers sheet unmount to exit (`screens/library/template/BackdropPicker.tsx:242–252`). Mobile asset-detail confirmation replaces its detail Sheet rather than rendering a second steady-state modal (`mobile/MLibraryScreen.tsx:668–681`). Source inspection does not prove animation-time focus/hit-testing or physical Safari behavior.

## Evidence boundary

The following are **parent-supplied results**, not checks performed by this reviewer:

- Library controller: 52 cases, zero failure/page error.
- Channel native controller: six cases; 13 further cases explicitly lack native populated data.
- Populated Channel response-override fixtures: 12 cases, zero failure; synthetic consumer proof only, not native persistence evidence.
- Explicit recovery response-override fixtures: 19 cases, zero failure and zero actual writes; these do not establish real successful PATCH persistence or cover every asynchronous interleaving in F1–F3.
- Four instruction recipients saved on PC/mobile with actual PATCH/readback and original data restored: real native persistence evidence supplied by parent, not synthetic proof and not a new write by this reviewer.
- Full frontend suite: 441 files, 3,659 pass / four skip **before** the last ImagesSection style-only patch. No post-patch result is claimed here; the requested parent build/native visual work was pending when this assignment was issued.

The checked-in integration report remains explicitly non-accepting (`plans/261103-0752-viderse-workflow-rounds-40-44/reports/pr-04/integration-state.md:1,18–23`). Its older test totals are not silently combined with the newer parent-supplied total. Accepted #90 records only two native prompts with real sample images and neutral placeholders for the missing default-template imagery (`apps/viderse/docs/design/v1/11-hoi-dap.md:661–668`); this report does not claim complete native sample/built-in asset availability. No physical Safari, complete overlay-focus audit, native provider/model behavior, global pixel parity or final build was proved by this review.

## Concerns/Blockers

F1 is silent draft loss plus a false clean-save state in a reachable recovery interleaving. F2 is unintended persistence on explicit Escape. F3 is a concrete PC/mobile recovery eligibility gap. Parent owns severity/acceptance and any implementation or verification; no source fixes were authorized or made in this slice.
