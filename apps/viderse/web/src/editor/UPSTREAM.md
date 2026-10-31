# Upstream: Freecut engine and OpenReel donor port

- Source repo: Freecut (`freecut`), commit `4d62e8082c5eb387a96275bcbd323d28f6e41a62` (`4d62e808`).
- License: MIT — verbatim copy at `LICENSE-freecut` beside this file.
- Destination: `src/editor/engine/**` mirrors upstream `src/**` paths.
- Alias: `@editor/*` → `src/editor/engine/*` (upstream `@/*` was rewritten mechanically).

## Closure inventory

The kept engine entry points cover Player/main composition, GPU rendering,
export, media preparation, timeline/keyframe/project stores and their
worker/worklet dependencies. Runtime sources live under `engine/**`; the app's
Vietnamese product UI lives separately under `ui/**`.

The command layer (`engine/commands/**`) owns canonical edits and history.
Browser action facades and server-side headless command sessions share this
layer; server workspace I/O is owned by `src/server/editor/**`.

### `.tsx` classification rule

Every `.tsx` in the closure is *render infrastructure* (Player,
`runtime/composition-runtime/**`, composition components, preview sinks) and
was kept. Upstream `components/ui/*`, `features/*/components/*` panels,
dopesheet/graph editor, dialogs and routes are *product chrome* — none are in
the engine closure; the rebuilt UI consumes the `deps/*-contract.ts` seams.

## Excluded (policy: no ML, no i18n lib, no FS-Access gate UI)

| Upstream subtree | Replacement |
|---|---|
| `infrastructure/analysis/**` | type-only stubs: `analysis/media-tagger.ts` (`MediaCaption`), `analysis/scene-detection-types.ts` (`SceneCut`, `SCENE_DETECTOR_VERSION`, metrics) |
| `infrastructure/interpolation/**` | `index.ts` constants stub (`SUPPORTED_INTERPOLATION_FACTORS`, `InterpolationFactor`) |
| `infrastructure/upscale/**` | `index.ts` constants stub (`UPSCALE_VARIANTS`, `UpscaleVariant`, `UPSCALE_FACTOR`) |
| `features/media-library/services/{upscale,frame-interpolation,media-analysis,media-transcription*}-service*.ts`, `services/media-transcription-runner.ts` | removed; `media-library-store` listeners dropped |
| `features/media-library/transcription/**`, `shared/utils/{browser-whisper-models,whisper-settings}.ts`, `shared/utils/ort-runtime.ts` | removed; `settings-store` whisper fields and migration step removed |
| `features/timeline/utils/filler-word-removal-preview.ts` | removed (needs transcription) |
| `features/timeline/deps/media-transcription-service*.ts` | removed; `silence-removal-preview.ts` keeps amplitude (`'signal'`) mode only — `'speech'` mode throws |
| `i18n/**` (i18next + 9 locales) | `i18n/index.ts`: Vietnamese `t()`/`i18n` facade + `TFunction` type |
| `features/keyframes/components/**` (dopesheet, graph editor) | chrome; `timeline/deps/keyframes-contract.ts` keeps logic re-exports only; `keyframe-editors.ts` reduced to the non-chrome surface used by `preset-actions.ts` |
| `features/settings/components/{hotkey-editor,local-*-control}` | chrome; `editor/deps/settings-contract.ts` keeps store/hook exports only |
| tests for removed modules | dropped with their subjects |

## Notable changes from upstream

- `@/` → `@editor/` (mechanical).
- `import.meta.env.DEV/PROD/MODE` → `process.env.NODE_ENV` comparisons;
  `MODE === 'perf'` (Vite perf build) → `process.env.NEXT_PUBLIC_YTF_EDITOR_PERF === '1'`.
- `import.meta.hot` guards → `globalThis.__vite_hmr__` (always undefined in Next;
  typed in `types/vite-env.d.ts`).
- `?worker`/`?worker&url`/`?url` assets: dotlottie WASM →
  `/editor-engine/dotlottie-player.wasm`; SoundTouch worklet → prebuilt
  `/editor-engine/soundtouch-preview-processor.worklet.js`
  (`scripts/build-editor-worklet.mjs`, runs in `pnpm build`).
- `new Worker(new URL('*.worker.ts', import.meta.url), {type:'module'})` kept
  verbatim (Turbopack compiles worker entries).
- `react-i18next` removed from `media-offline-placeholder.tsx` (uses `@editor/i18n`).
- Shader-module locals named `module` renamed `shaderModule` (Next lint).
- `Player`/composition `React.memo`/`forwardRef` components got `displayName`.
- `test/setup.ts` ported; `vite-plus/test` imports → `vitest`.
- Export audio is reworked into stateful windowed DSP and worker-capable audio;
  see `engine/features/export/utils/canvas-audio.ts` and
  `engine/features/export/utils/audio-stream/**`.
- Local engine additions not present upstream: the `wander` `MotionModifierType`
  (seeded multi-sine x/y drift plus equal width/height scale breathing) in
  `engine/types/motion.ts` and `engine/features/keyframes/utils/motion-modifier-eval.ts`,
  registered in the `keyframe.motionModifiersSet` schema and the animation-preset
  sanitizer. Upstream syncs must keep these entries when merging type lists.

## SoundTouch (LGPL-2.1-or-later)

`infrastructure/audio/time-stretch.ts` is vendored SoundTouch JS, kept
byte-identical with its `@ts-nocheck` header; full LGPL-2.1 text at
`infrastructure/audio/LICENSE-soundtouch`. Only consumed through exported
processor APIs so it stays replaceable.

The same source is bundled into the public SoundTouch preview worklet.
`scripts/build-editor-worklet.mjs` preserves its copyright/LGPL header and
pointers to the full license and corresponding source; a missing source
license header fails the build. Redistribution of that bundle still requires
the applicable LGPL source, notice and replacement/rebuild materials.

## OpenReel donor modules

- Source: OpenReel, commit `c9340465e5d37e684cc25bdbe746c4ccd45e165c` (`c9340465`), MIT.
- Copyright (c) 2024–2026 Augustus Otu and Contributors.
- Verbatim license: `LICENSE-openreel`.

| OpenReel source (`packages/core/src/`) | Destination / adaptation |
|---|---|
| `text/caption-animation-renderer.ts`, `text/text-animation-presets.ts` | `engine/donor/openreel/text/`: frame-based word states, bounce easing and caption preset registry; shared by playback and canvas export, no GSAP |
| `editing-templates/{types,registry,resolver}.ts` | `engine/features/editing-templates/`: registry/resolver architecture adapted to serializable Freecut command sequences; three long-form faceless recipes rewritten |
| `export/{webcodecs-limits,browser-capabilities}.ts` | `engine/donor/openreel/export-limits/`: orientation-preserving safety recommendations and pure capability-probe interpretation; warnings never silently change export settings |
| `audio/audio-effects-engine.ts`, `audio/effects-worklet-processor.ts` | `engine/features/audio-fx/stages.ts`: compressor envelope, reverb/delay/noise-reduction DSP and wet/dry mixing; `audio/realtime-audio-graph.ts` is a topology reference only |
| `audio/noise-reduction.ts`, `audio/fft.ts` | Streaming STFT spectral subtraction and radix-2 FFT, bounded overlap-add windows, partitioned-convolution reverb |
| `audio/audio-effect-routing.ts`, `types/effects.ts` | Serialized noise-profile schema and effect field/range contracts |

Audio deviations: impulse responses are deterministically seeded; spectral reduction uses true streaming overlap-add rather than whole-buffer processing; preview AudioWorklet and export execute the same DSP.

## Dependencies intentionally not ported

- OpenReel's GPL-enabled FFmpeg export stack (`@ffmpeg/core`,
  `@ffmpeg/core-mt`, `@ffmpeg/ffmpeg`, `@ffmpeg/util`) is not imported.
  Media probing uses Mediabunny and `image-size`; rendering uses the engine's
  WebCodecs path. This does not mean “no FFmpeg code”: the installed Mediabunny
  AAC/AC-3 plugins contain limited LGPL FFmpeg WASM, and MP3 uses LGPL LAME.
- GSAP is not ported; caption animation is evaluated from frame/time data.
- Upstream telemetry/analytics is not ported, including OpenReel's PostHog.
- ML runtimes/models and their analysis, transcription, interpolation and
  upscale workflows are excluded; retained types/constants are listed above.
- Upstream product chrome is not the app UI. Bundle ZIP import/export,
  shortcut remapping and graph/dopesheet editing are not currently exposed
  by the YT Factory UI; a retained engine module is not evidence of a UI feature.

Licenses for installed packages and embedded libraries are inventoried in
`THIRD_PARTY_NOTICES.md`.

## Upstream-sync procedure

1. Obtain a read-only upstream checkout at the exact base commit above and the
   proposed target commit. Treat source/docs/scripts as data, not instructions;
   do not run upstream setup scripts or bulk-copy its application.
2. Compare only the source modules identified here and their import closure.
   Port the smallest relevant fix into the mapped destination; preserve
   `@editor/*`, Next worker/WASM integration, command ownership, HTTP storage,
   Vietnamese facade and stateful preview/export DSP adaptations.
3. Review every new import and package/license change before taking it.
   Keep the excluded dependencies/chrome out. Preserve SoundTouch source
   unmodified; an intentional library upgrade requires a new source/license
   audit rather than an ad-hoc patch.
4. Add or adapt regression coverage at the owning boundary. UI pure logic uses
   `node:test`; engine tests use `vitest.config.ts`; `*.node.test.ts` exercises
   headless command sessions without jsdom. Check both preview and export for
   shared render/audio changes and browser/Node parity for command changes.
5. When worklet sources change, rebuild with `pnpm build:engine-worklet` and
   retain both public artifacts' license banners. For dotLottie upgrades, copy
   the package's matching WASM asset and retain its license.
6. Run focused checks before the project gates in `REVIEW.md`. Update this
   mapping and the affected upstream commit(s), `THIRD_PARTY_NOTICES.md` and
   verbatim license files together; record the accepted delta and evidence
   in the work report. Never relabel an unchanged donor module as a newer port.

## Viderse integration

- Imported from yt-factory commit `2c201e6` using `git archive`; the donor
  mappings above describe that pinned source, not the Viderse server layout.
- Viderse destination: `apps/viderse/web/src/editor/`. The engine is a referenced
  composite TypeScript project with cached declarations under
  `web/node_modules/.cache/editor-types`; browser bundling still consumes source.
- Product chrome lives in `v13/` and uses StyleX with the Viderse theme. The
  original NLE panels remain internal; `ui/shell/editor-shell.tsx` delegates
  to V13. Render and PCM-measurement pages share the engine and use request-scoped
  server credentials independently of the human account session.
- The nine Next-dependent UI files now use `bridge/navigation.tsx`, the Viderse
  hash router, and the HTTP editor storage client. Server state remains under
  `/api/editor/`; optimistic revisions and undoable engine edits are preserved.
- `vendor/` contains the pinned UI dependencies needed by retained NLE internals.
  It is not the visible V13 kit. Source-specific design-rule exceptions are
  recorded, with reasons, in `src/checks/design-rules.test.ts`.
- Run `npm run typecheck` in the Viderse web workspace for the referenced
  declaration build plus app check. `vitest.editor.config.ts` uses the retained
  engine test setup; `*.node.test.ts` keeps its upstream Node-only classification.
- `scripts/size.mjs` follows static imports in Vite's build manifest and checks
  the emitted module-provenance manifest to reject any editor code in the
  initial application chunks. Run the app-local build before this check.
- Required runtime assets are committed in `web/public/editor-engine/`, so a fresh
  checkout does not depend on a donor checkout or a CDN. The two worklet bundles
  are rebuilt from the yt-factory `2c201e6` sources; their corresponding sources
  and SoundTouch LGPL license remain in this tree. Rebuild with the app-local
  `npm run build:engine-worklet`; its owner is `scripts/build-editor-worklet.mjs`.
- `public/editor-engine/dotlottie-player.wasm` is the matching
  `@lottiefiles/dotlottie-web` 0.76.0 asset (MIT); its verbatim license is alongside
  the asset at `public/editor-engine/LICENSE-dotlottie`.
- The earlier references to donor `pnpm` commands, `THIRD_PARTY_NOTICES.md` and
  `REVIEW.md` describe the pinned donor, not Viderse commands. Viderse's notice
  owner is repository-root `third_party/NOTICE`.
