# Selected-version redraw backend (V36)

Status: Integrated and reviewed. Final focused/broader checks, embedded build, four-context native source proof and six actual HTTP rejection cases passed. Uses a private committed-source overlay for one unrelated generated-model worktree delta; the user's file remains untouched/unstaged. Not merged/deployed.

## Contract implemented

- `POST /api/clone/:id/images/redraw` accepts optional JSON `baseVersion: number` (`RedrawVersionsInput.BaseVersion *int`). It is a prompt/ref source selection, not an alias for the used version.
- `nil` (field absent) keeps the existing policy: draft parts overlay the used version, refs come from used version, then video defaults.
- Explicit `baseVersion`:
  - requires exactly one distinct image; otherwise `invalid_request` before any manifest write;
  - rejects `<= 0` (`invalid_request`), missing version (`not_found`), drawing/in-flight (`conflict`), cancelled (`invalid_request`), and stale source fingerprint (`stale`) before mutation;
  - requires the selected version to actually carry a saved `Prompt` or `Parts`; otherwise `invalid_request` instead of silently falling back;
  - stores `PromptDraft.SourceVersion` (JSON `sourceVersion`) in the item draft, so checkpoint encode/decode/restart and worker retries keep the requested source. `Flight.BaseVersion` and `usedVersionFor` are untouched (still only the used-version race guard).
- Drawing with an explicit source:
  - baseline parts are the selected version's saved `Parts` (or legacy `Prompt` as one scene part), including untouched scene/style/avoid provenance;
  - effective refs are the draft's edited refs if present, otherwise the selected version's refs verbatim — an empty selected refs list stays empty and never falls back to used refs or defaults;
  - unchanged attachment IDs/roles preserve the saved refs-description and its provenance, including custom edits; changed attachments rebuild the description from the effective staged refs, and `[]` remove-all drops it;
  - the trusted wrapper/tool block and provider capabilities are rebuilt through `BuildPrompt` for the current provider; only the wrapper is taken from the fresh build.
- `RedrawVersions` does not change `CurrentVersion`; normal worker completion rules still decide it. A failed selected version remains usable when its saved prompt/refs and source fingerprint are intact. Cancelled versions are rejected.
- Explicit `refs: []` is preserved as a non-nil empty slice (`append([]ImageRef{}, ...)`) so it survives JSON as `[]`, distinct from an absent field (`null`/nil = keep selected refs).
- Inherited selected refs are checked against the effective provider limit at the API boundary. The worker rechecks after provider configuration changes; it rejects rather than silently truncating the selected refs.
- The API and image View use the video's effective draw-provider override, retaining raw provider-card IDs (`chatgpt`, `grok`, `gemini-…`) instead of dropping unknown IDs into the global branch. An explicit request provider still takes precedence.
- Every redraw target with a live `InFlight` rejects with conflict before manifest/revision mutation. A failed version row does not make its still-live flight a valid new source.
- Before joining a selected source into the new request, the old embedded `Owner visual direction…` scene suffix is stripped, matching the existing viewer/used-version normalization. Only new feedback enters the fresh wrapper; history remains unchanged.
- Explicit refs changes on the omitted-base path also remove obsolete reference descriptions. Unchanged attachments preserve custom refs instructions on both paths.
- GET history keeps `parts` a non-null array even for an old cancelled attempt with no saved prompt. Returning `null` broke the strict client schema and silently forced the legacy DTO fallback, hiding versions/refs. Native59 and a new serialization regression failed before; final native61 reads the complete rich history after the fix.

## Files changed

- `apps/viderse/internal/clone/images/types.go` — `RedrawVersionsInput.BaseVersion`.
- `apps/viderse/internal/clone/images/versions.go` — base validation before commit, `SourceVersion` persisted in draft, explicit empty-refs slice.
- `apps/viderse/internal/clone/images/service.go` — image View exposes the same effective provider reference limit as redraw validation.
- `apps/viderse/internal/pipeline/steps/images/model.go` — `PromptDraft.SourceVersion`, `VersionParts`, `PromptSourceVersion` (missing/zero/drawing/stale/no-prompt validation).
- `apps/viderse/internal/pipeline/steps/images/draw.go` — plan uses selected-source parts/refs, rebuilds refs part, fresh wrapper; `requestedRefs` takes the selected source.
- `apps/viderse/internal/clone/images/versions_test.go` — selection persistence, failed retry, invalid base cases.
- `apps/viderse/internal/pipeline/steps/images/versions_test.go` — plan-level selected-vs-used prompt/ref assertions across JSON reload and used-version changes, explicit empty refs, omitted-base behavior, invalid source rejection, and an end-to-end `Handler.Run` request captured by `recorderDrawer`.

## Legacy semantics (not fabricated)

- A saved version without structured `Parts` keeps the existing one-scene legacy representation; no style/refs/avoid provenance is invented. The new request normalizes the historical direction suffix without changing the stored source. Full raw provider prompts remain read-only in the frontend.
- Versions whose `SourceFingerprint` no longer matches the item are rejected as stale for explicit bases; they are not silently replaced by the used version.

## Observed checks

- Final focused checks use `-overlay=<private committed-source map>`: clone/images0.446s and pipeline/images cached. The new empty-cancelled history regression fails before with `parts:null`, passes after0.682s.
- Final affected `web` + `pipeline/...` check passed (12 packages,2 without tests; web27.712s). This is not a full-backend/performance pass.
- One non-owned generated `postgres/gen/models.go` change adds unrelated template/trial fields. Go dependency inspection found no other unowned compiled source delta. Tests/build substitute that file's committed HEAD bytes via native Go overlay; no user changes are edited or published.
- `gofmt` applied to changed Go files; final `go build -overlay=<private map> -o <private QA binary> ./cmd/viderse` passed with the verified frontend.
- Worker regressions exercise persisted JSON reload, distinct selected-vs-used parts/refs, unchanged custom refs instructions, explicit empty refs, provider-limit changes, and captured draw requests.
- [Native61](runs/61-selected-source-final-empty-cancelled-history/selected-base-native-results.json) passed PC/mobile × normal/reduced with actual202/source2/current1/refs2; the empty-prompt cancelled3 remains disabled, not a fabricated third image. [Boundary62](runs/62-selected-source-final-api-boundaries/rejection-results.json) passed400/404/409/422 and no persistent mutation. Both restore the owned replay and preserve the original source.
- No additional paid image/model work. Native acceptance uses the owned replay with the worker paused; that proof does not claim a new generated image.
