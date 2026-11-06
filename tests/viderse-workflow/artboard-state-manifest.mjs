#!/usr/bin/env node
/**
 * Viderse workflow artboard state manifest.
 *
 * Read-only extractor for the published Viderse artboards. It parses the real
 * HTML `data-props` JSON footer of every board in
 * `apps/viderse/docs/design/v1/artboards/`, the README table, and the current
 * hash router. It then emits `reports/artboard-state-matrix.json` with one row
 * per enum/boolean value for the V36–V40 boards, plus bounded numeric/text
 * probes for values that live in the design contract rather than a tweak.
 * Each row also carries the parent-view `context` props needed for the tweak
 * to be visible (tab/section/screen/step/dialog/panel).
 *
 * This is an inventory. It never claims a screenshot, control ledger, or app
 * state was captured: every evidence path starts `null` and every row starts
 * `pending`.
 *
 * Usage:
 *   node tests/viderse-workflow/artboard-state-manifest.mjs
 *   node tests/viderse-workflow/artboard-state-manifest.mjs --stdout
 *   node tests/viderse-workflow/artboard-state-manifest.mjs --check
 *   node tests/viderse-workflow/artboard-state-manifest.mjs --out <path>
 *
 * `--check` compares the on-disk JSON with a fresh deterministic render.
 */

import { createHash } from 'node:crypto'
import { existsSync, mkdirSync, readFileSync, readdirSync, statSync, writeFileSync } from 'node:fs'
import { dirname, join, relative, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const HERE = dirname(fileURLToPath(import.meta.url))
const REPO = resolve(HERE, '..', '..')
const ARTBOARDS_REL = 'apps/viderse/docs/design/v1/artboards'
const ARTBOARDS_DIR = join(REPO, ARTBOARDS_REL)
const README_REL = `${ARTBOARDS_REL}/README.md`
const DEFAULT_OUT_REL = 'plans/261103-0752-viderse-workflow-rounds-40-44/reports/artboard-state-matrix.json'
const DEFAULT_OUT = join(REPO, DEFAULT_OUT_REL)

const PHASE_FILES = {
  plan: 'plans/261103-0752-viderse-workflow-rounds-40-44/plan.md',
  inventory: 'plans/261103-0752-viderse-workflow-rounds-40-44/reports/inventory.md',
  verification: 'plans/261103-0752-viderse-workflow-rounds-40-44/verification.md',
  phase01: 'plans/261103-0752-viderse-workflow-rounds-40-44/phase-01-inventory.md',
  phase03: 'plans/261103-0752-viderse-workflow-rounds-40-44/phase-03-v36-viewer-and-actions.md',
  phase04: 'plans/261103-0752-viderse-workflow-rounds-40-44/phase-04-v37-compact-studio.md',
  phase05: 'plans/261103-0752-viderse-workflow-rounds-40-44/phase-05-v38-channels-and-library.md',
  phase06: 'plans/261103-0752-viderse-workflow-rounds-40-44/phase-06-v39-quick-create-and-queue.md',
  phase07: 'plans/261103-0752-viderse-workflow-rounds-40-44/phase-07-v40-mobile-parity.md',
  workflow: 'apps/viderse/docs/design/v1/artboards/Workflow.dc.html',
  ia: 'apps/viderse/docs/design/v1/01-ia-flows.md',
  components: 'apps/viderse/docs/design/v1/04-components.md',
  kit: 'apps/viderse/docs/design/v1/05-kit.md',
  copy: 'apps/viderse/docs/design/v1/06-copy.md',
  data: 'apps/viderse/docs/design/v1/07-data-contract.md',
  dev: 'apps/viderse/docs/design/v1/08-dev-prompt.md',
  gap: 'apps/viderse/docs/design/v1/10-gap-closure.md',
  router: 'apps/viderse/web/src/router.ts',
  'artboards-readme': README_REL,
  support: `${ARTBOARDS_REL}/support.js`,
}

const QA_PREREQ =
  'Isolated QA stack only (HOME/.viderse/auth.json, app DB, identity DB, media/blob store, browser storage); seed through the existing API/DB, no DOM/state mock (verification.md §1).'
const QA_SEED =
  'Use a PR-owned QA fixture through existing API/DB constructors and record the seed receipt in reports/pr-NN/ before capture.'
const PENDING_EVIDENCE = () => ({
  reference: null,
  app: null,
  comparison: null,
  controlLedger: null,
  motionTrace: null,
})
const OWNER_PRS = new Set(['PR2', 'PR3', 'PR4', 'PR5', 'PR6'])

const baseline = (mapping, knownGap = null, exclusion = null) => ({ mapping, knownGap, exclusion })

/**
 * Included stateful boards. Every non-`$preview` property in the footer MUST
 * have an entry here; the extractor fails if an artboard gains or loses a
 * property without this inventory being updated. Empty `{}` means "use the
 * artboard default route/owner/baseline".
 */
const SCREEN_SPECS = {
  Review: {
    page: 'desktop',
    ownerPR: 'PR2',
    phases: ['V36', 'V40'],
    route: '#/review?video=<jobId>',
    baseline: baseline(
      'screens/review/ReviewScreen.tsx, ImageRail.tsx, imageView.ts, useImageEditing.ts; shared component/image-viewer/ImageViewer.tsx is absent at baseline.',
      'V36 creates the shared viewer and wires confirmCount/expectedRevision; Review still owns its image chrome at baseline.',
    ),
    prerequisite: 'Job in Chờ duyệt (or the exact state named by the tweak) with real images, versions, prompts, and provenance.',
    seed: 'Seed a review job with at least 7 images through the real clone/images API on the isolated QA DB.',
    expected: 'Review artboard state matches the footer tweak and the 04-components “Trình xem ảnh › Duyệt” contract.',
    props: {
      confirm: {
        valueNotes: {
          none: { expected: 'No confirmation layer; Review is directly interactive.' },
          redrawAll: {
            expected: 'Confirm “Vẽ lại cả 7 ảnh” shows the real image count and quota consequence.',
            prerequisite: 'Job with a known image count and a live image-quota readback.',
          },
        },
      },
      state: {
        valueNotes: {
          review: { expected: 'Review mode: image stage, versions, prompt, refs, feedback, and approve/redraw actions.' },
          redrawing: { expected: 'Redraw in progress keeps the selected image/version and shows progress, not a fake new image.' },
          reveal: { expected: 'Reveal shows the new version against the previous one with the real version id.' },
          failed: { expected: 'Failed draw shows a recoverable error tied to the real image/job, not a silent reset.' },
          batch: { expected: 'Batch review keeps all images in one video context and approves/redraws only eligible images.' },
          blocked: { expected: 'Blocked state explains the real permission/quota/ownership reason and keeps data.' },
          empty: { expected: 'Empty review explains there is nothing to review and offers a real route back.' },
        },
      },
      view: {
        valueNotes: {
          full: { expected: 'Full image layer; cutout is not applied.' },
          cutout: { expected: 'Cutout layer shown only when real cutout media exists; no CSS-only fake.' },
        },
      },
      score: {
        valueNotes: {
          true: { expected: 'Score/provenance is visible for the selected image with real score data; V40 removes “Vì sao?” but not the score itself.' },
          false: { expected: 'Score surface is absent without pretending the image passed/failed.' },
        },
      },
      prompt: {
        valueNotes: {
          view: { expected: 'Submitted prompt is read-only in its four parts.' },
          edit: { expected: 'Prompt edit stays per image and saves through the real prompt API.' },
          raw: { expected: 'Raw/source prompt is provenance, preserved in its original language.' },
        },
      },
      refs: {
        valueNotes: {
          view: { expected: 'Reference images are visible with the selected image.' },
          add: { expected: 'Add-reference opens the real picker and does not mutate until confirmed.' },
        },
      },
    },
  },

  Main: {
    page: 'desktop',
    ownerPR: 'PR3',
    phases: ['V36', 'V37', 'V40'],
    route: '#/?video=<jobId>',
    baseline: baseline(
      'screens/workbench/WorkbenchDesktop.tsx, LiveStudio.tsx, panels/WorkbenchPanels.tsx; shared ImageViewer absent.',
      'V36 viewer entrypoints and V37 inspector/progress/suggestion cutover still pending; recap/composer/plan/explain surfaces remain at baseline.',
    ),
    prerequisite: 'Seeded QA channels and videos at the lifecycle stage named by the tweak; select the video by stable job id.',
    seed: 'Seed workbench jobs across drawing/review/ready/issue/paused using the real pipeline/API on the isolated QA DB.',
    expected: 'Main artboard state matches the footer tweak; viewer opens the exact job/image id and never falls back to image 0.',
    props: {
      viewer: {
        ownerPR: 'PR2',
        valueNotes: {
          none: { expected: 'No viewer layer; Phòng vẽ is directly interactive.' },
          view: { expected: 'Shared viewer opens in Xem mode for the clicked image; ←/→ and Esc work; no background shortcuts.' },
          redraw: { expected: 'Shared viewer opens with Góp ý & vẽ lại and shows a real “tốn 1 ảnh” quota cost before sending.' },
        },
      },
      stage: {
        ownerPR: 'PR3',
        valueNotes: {
          images: { expected: 'Video in drawing/images stage with real tiles and progress.' },
          review: { expected: 'Video waiting for review; image click routes to Review at that image.' },
          render: { expected: 'Render/build state with real progress events, no fake percentage.' },
          issue: { expected: 'Cần xử lý state with one real corrective action and a reason.' },
          quality: { expected: 'Quality/measurement state with real score/check data.' },
          done: { expected: 'Sẵn sàng state with real MP4/track metadata and download.' },
          paused: { expected: 'Tạm dừng state keeps completed work and offers Chạy tiếp.' },
        },
      },
      panel: {
        valueNotes: {
          info: { expected: 'Right panel shows Thông tin and is the default on entry.' },
          content: { expected: 'Right panel shows Nội dung đăng with copy/tag controls backed by real data.' },
        },
      },
      meta: {
        context: { panel: 'content' },
        valueNotes: {
          pending: { expected: 'Post content pending; no invented title/description.' },
          writing: { expected: 'Director writing state with real progress/events.' },
          failed: { expected: 'Metadata failure with a recoverable action and preserved source title.' },
          ai: { expected: 'AI-authored metadata is visible and editable.' },
          edited: { expected: 'User-edited metadata is preserved after polling/reload.' },
          rewrite: { expected: 'Rewrite confirmation shows consequence and calls the real rewrite action.' },
          history: { expected: 'Metadata history reads persisted revisions, not a mock list.' },
        },
      },
      jobs: {
        valueNotes: {
          running: { expected: 'Header job pill shows running count/pending count and opens Việc nền.' },
          idle: { expected: 'Máy rảnh state with no false running count.' },
          paused: { expected: 'Paused state shows pause time and pending count; pause control is disabled when disconnected.' },
          offline: { expected: 'Mất kết nối retains last counts and timestamp and offers retry.' },
        },
      },
      select: {
        valueNotes: {
          true: { expected: 'Selection checkboxes plus sticky “Tải gói N video”; only ready videos are eligible.' },
          false: { expected: 'Normal list rows navigate by video; no selection bar.' },
        },
      },
      video: {
        valueNotes: {
          nurse: { expected: 'First seeded job selected; middle and right panels follow the job id.' },
          mall: { expected: 'Second seeded job selected; both panels change together.' },
          lastbus: { expected: 'Third seeded job selected; both panels change together.' },
          ferry: { expected: 'Fourth seeded job selected; both panels change together.' },
          hotel: { expected: 'Fifth seeded job selected; both panels change together.' },
          hem: { expected: 'Sixth seeded job selected; both panels change together.' },
        },
      },
      backdropShort: {
        valueNotes: {
          true: { expected: 'Backdrop shortage is explained with the real clip count and an action to add clips.' },
          false: { expected: 'Backdrop is sufficient; no shortage banner.' },
        },
      },
      scope: {
        valueNotes: {
          all: { expected: 'List scoped to all channels with the “Mọi kênh” switcher state.' },
          channel: { expected: 'List scoped to one real channel; channel switch preserves the selected video id.' },
        },
      },
    },
  },

  Editor: {
    page: 'desktop',
    ownerPR: 'PR3',
    phases: ['V36', 'V37'],
    route: '#/editor/<projectId>',
    baseline: baseline(
      'editor/v13/Editor.tsx, Inspector.tsx, SceneImagePanel.tsx, scene-images.ts; routed v13 exists.',
      'V36 must route the real v13 editor; V37 wires the 15 editor controls and vertical sidebar. No mobile Editor.',
    ),
    prerequisite: 'A real editor project/video with scenes, captions, backdrop, audio, and media.',
    seed: 'Seed a cloned video with a rendered project through the real editor pipeline on the isolated QA DB.',
    expected: 'Editor artboard state matches the footer tweak and the routed v13 editor; controls act on the real project.',
    props: {
      state: {
        valueNotes: {
          editing: { expected: 'Normal editing state with real timeline and inspector.' },
          'export-gate': { expected: 'Export gate shows real readiness checks and blocks invalid export.' },
          exporting: { expected: 'Export progress reflects real encoder events.' },
          'export-done': { expected: 'Export done exposes the real output file/link.' },
          conflict: { expected: 'Conflict state preserves both sides and offers a real resolution.' },
          narrow: { expected: 'Narrow desktop layout keeps controls reachable; not a mobile Editor design.' },
        },
      },
      fix: {
        valueNotes: {
          todo: { expected: 'Fix list has real unmet checks and one action per item.' },
          partial: { expected: 'Partially fixed state keeps unfinished items visible.' },
          done: { expected: 'All fixable items done; no fake pass.' },
          measuring: { expected: 'Measurement runs with real progress and cancellable state.' },
          passed: { expected: 'Passed state shows the real measured score.' },
        },
      },
      panel: {
        valueNotes: {
          fix: { expected: 'Left rail shows Cần sửa.' },
          scenes: { expected: 'Left rail shows scenes with real scene ids.' },
          subs: { expected: 'Left rail shows subtitles aligned to real cues.' },
          bg: { expected: 'Left rail shows backdrop clips from real media.' },
          audio: { expected: 'Left rail shows audio tracks and real mute/seek controls.' },
          media: { expected: 'Left rail shows media assets from the project.' },
        },
      },
      focus: {
        valueNotes: {
          caption: { expected: 'Caption selected in the timeline/inspector.' },
          scene: { expected: 'Scene selected; image cell can open the shared viewer in Xem + Thay mode.' },
          bg: { expected: 'Backdrop selected; real clip controls apply.' },
          none: { expected: 'Nothing selected; no inspector mutation target.' },
        },
      },
      img: {
        context: { focus: 'scene' },
        valueNotes: {
          view: { expected: 'Scene image opens in the shared viewer Xem mode.' },
          edit: { expected: 'Scene image prompt is editable through the real prompt API.' },
          drawing: { expected: 'Scene redraw is actually running; progress comes from real events.' },
          done: { expected: 'Redrawn image is available and undo/history works.' },
        },
      },
    },
  },

  Template: {
    page: 'desktop',
    ownerPR: 'PR3',
    phases: ['V36', 'V37', 'V38'],
    route: '#/templates/<templateId>?section=<section>',
    baseline: baseline(
      'screens/library/template/PreviewPane.tsx, CoverSection.tsx, ImagesSection.tsx and template detail routes exist.',
      'V37 floating selects/sliders and V38 single default-template “Kênh dùng làm mặc định” still pending at baseline.',
    ),
    prerequisite: 'A real template with image/cover/backdrop/output sections and at least one channel using it.',
    seed: 'Seed a template and channel refs through the real library API on the isolated QA DB.',
    expected: 'Template artboard state matches the footer tweak; previews/selects read real data and do not push layout.',
    props: {
      section: {
        ownerPR: 'PR4',
        valueNotes: {
          images: { expected: 'Images section with the real image composition.' },
          cover: { expected: 'Cover section with the real cover prompt/asset.' },
          backdrop: { expected: 'Backdrop section with real clips/effects.' },
          output: { expected: 'Output section with real export settings.' },
        },
      },
      preview: {
        valueNotes: {
          fresh: { expected: 'Preview is current for the template revision.' },
          stale: { expected: 'Stale preview is clearly marked and offers rebuild.' },
          building: { expected: 'Building preview reflects real builder state; no fake completed preview.' },
        },
      },
      fewClips: {
        ownerPR: 'PR4',
        context: { section: 'backdrop' },
        valueNotes: {
          true: { expected: 'Shortage of channel backdrop clips is explained with real counts and an add route.' },
          false: { expected: 'Enough clips; no shortage message.' },
        },
      },
      picker: {
        valueNotes: {
          none: { expected: 'No picker layer.' },
          add: { expected: 'Attach channel/video picker is a floating list that does not move content.' },
          choose: { expected: 'Choose-existing picker reads real channels/videos.' },
          video: { expected: 'Video select for the sample/cover build is a floating list.' },
        },
      },
    },
  },

  Library: {
    page: 'desktop',
    ownerPR: 'PR4',
    phases: ['V36', 'V38', 'V40'],
    route: '#/library?tab=<tab>&prompt=<prompt>&nguon=<nguon>&kenh=<kenh>',
    baseline: baseline(
      'screens/library/LibraryScreen.tsx, TemplatesTab.tsx, AttachChannelDialog.tsx; library side rail may still exist at baseline.',
      'V36 action wiring and V38 top-nav/default-template cutover pending; delete-default guard and shared-template refs must be real.',
    ),
    prerequisite: 'Real templates, image prompts, backdrop channels/videos, overlays, and channel default refs.',
    seed: 'Seed library assets and at least two channels sharing a template through the real library API.',
    expected: 'Library artboard state matches the footer tweak; every visible action is backed by a real mutation/readback.',
    props: {
      tab: {
        valueNotes: {
          templates: { expected: 'Templates tab; top nav Library active; no side rail.' },
          prompts: { expected: 'Image prompts tab with real prompt list.' },
          backdrops: { expected: 'Backdrop channels/videos tab.' },
          overlays: { expected: 'Overlays tab with real assets.' },
        },
      },
      prompt: {
        context: { tab: 'prompts' },
        valueNotes: {
          list: { expected: 'Prompt list including mine/builtin filters.' },
          mine: { expected: 'Only user prompts; no builtin leak.' },
          builtin: { expected: 'Only builtin prompts; no fake user prompt.' },
          new: { expected: 'New prompt editor; save through the real API.' },
        },
      },
      menu: {
        valueNotes: {
          true: { expected: 'Template ⋯ menu shows default/rename/duplicate/export/delete; delete is blocked with a real reason when in use.' },
          false: { expected: 'Menu closed; no orphan overlay.' },
        },
      },
      nguon: {
        context: { tab: 'backdrops' },
        valueNotes: {
          channels: { expected: 'Backdrop channel list from real subscriptions.' },
          videos: { expected: 'Backdrop video list from real media.' },
          adding: { expected: 'Add-channel flow validates the real handle/URL.' },
          addvideo: { expected: 'Add-video flow uses the real upload/URL path.' },
          unsupported: { expected: 'Unsupported source explains the real limitation without a fake add.' },
        },
      },
      import: {
        valueNotes: {
          none: { expected: 'No import dialog.' },
          ready: { expected: 'Import dialog ready with real file validation.' },
          drag: { expected: 'Drag-over state; dropping validates the real file.' },
          importing: { expected: 'Import progress from the real parser/transaction.' },
          done: { expected: 'Import completed and assets are readable.' },
          partial: { expected: 'Partial import keeps imported assets and reports failed items.' },
          zip: { expected: 'ZIP validation state with a real error/retry.' },
          offline: { expected: 'Offline import blocks writes and offers retry.' },
        },
      },
      kenh: {
        context: { tab: 'backdrops' },
        valueNotes: {
          slowroads: { expected: 'Slow Roads backdrop channel selected.' },
          nightdrivefm: { expected: 'Night Drive FM backdrop channel selected.' },
          bilibili: { expected: 'Bilibili backdrop source selected.' },
          lofilanes: { expected: 'Lofi Lanes backdrop channel selected.' },
        },
      },
    },
  },

  Settings: {
    page: 'desktop',
    ownerPR: 'PR6',
    phases: ['V36', 'V40'],
    route: '#/settings?section=<section>',
    baseline: baseline(
      'screens/settings/SettingsScreen.tsx and sections; screens/settings/connect/YoutubeDataKeyCard.tsx and api/performance.ts data-key calls still exist at baseline.',
      'V36 removes the YouTube Data API card and the real data-key surface on desktop and mobile; V40 rechecks the mobile branches.',
    ),
    prerequisite: 'Real local tools, AI providers, agents, platform accounts, storage mounts, and logs state; no production provider mutation.',
    seed: 'Use isolated QA config/storage and read-only provider metadata; never copy production credentials into the fixture.',
    expected: 'Settings artboard state matches the footer tweak; no YouTube Data API card remains after V36.',
    props: {
      section: {
        ownerPR: 'PR2',
        valueNotes: {
          tools: { expected: 'Tools section; data-key card absent.' },
          connect: { expected: 'AI connections section; data-key card absent.' },
          agents: { expected: 'Assistant models section.' },
          accounts: { expected: 'Platform accounts section.' },
          storage: { expected: 'Storage section.' },
          logs: { expected: 'Logs section.' },
        },
      },
      tools: {
        valueNotes: {
          ok: { expected: 'Tools ready; real binary/version metadata.' },
          update: { expected: 'Update available with a real version delta and install action.' },
          failed: { expected: 'Tool failure with a real reason and retry.' },
          installing: { expected: 'Install/update progress from the real installer.' },
        },
      },
      conn: {
        ownerPR: 'PR2',
        context: { section: 'connect' },
        valueNotes: {
          list: { expected: 'Provider list with no YouTube Data API card.' },
          add: { expected: 'Add-provider form uses the real provider catalogue.' },
          added: { expected: 'Provider added and readable after reload.' },
          models: { expected: 'Model list from the real provider.' },
          manual: { expected: 'Manual model entry validates.' },
          failed: { expected: 'Provider failure is actionable and does not delete existing credentials.' },
        },
      },
      grok: {
        context: { section: 'connect' },
        valueNotes: {
          signed: { expected: 'Grok signed-in state from real credential metadata.' },
          code: { expected: 'Device-code state; no fake success.' },
          out: { expected: 'Signed-out state; confirm explains consequences.' },
        },
      },
      ai: {
        context: { section: 'agents' },
        valueNotes: {
          none: { expected: 'No assistant model selected.' },
          director: { expected: 'Director model selected.' },
          artist: { expected: 'Artist model selected.' },
        },
      },
      bili: {
        context: { section: 'accounts' },
        valueNotes: {
          blocked: { expected: 'Bilibili blocked state with a real reason.' },
          saved: { expected: 'Bilibili credential saved (metadata only in evidence).' },
          manual: { expected: 'Manual cookie entry validates.' },
          expired: { expected: 'Expired credential offers reconnect.' },
          empty: { expected: 'No Bilibili account.' },
        },
      },
      clean: {
        context: { section: 'storage' },
        valueNotes: {
          idle: { expected: 'Cleanup idle with real usage numbers.' },
          preview: { expected: 'Cleanup preview lists real files/bytes before deletion.' },
          done: { expected: 'Cleanup done with real reclaimed bytes.' },
        },
      },
      store: {
        context: { section: 'storage' },
        valueNotes: {
          here: { expected: 'Local storage selected.' },
          pick: { expected: 'Folder picker reads a real path.' },
          lowspace: { expected: 'Low-space warning uses real free bytes.' },
          cloud: { expected: 'Cloud option reflects real configuration.' },
          moving: { expected: 'Move progress is real and resumable.' },
          done: { expected: 'Move completed; library still reads.' },
          donenew: { expected: 'Move completed to a new location.' },
          offline: { expected: 'Offline storage state offers retry.' },
          server: { expected: 'Server storage mode.' },
          s3pick: { expected: 'S3 provider picker.' },
          s3form: { expected: 'S3 form validates real fields.' },
          s3testing: { expected: 'S3 test running.' },
          s3testok: { expected: 'S3 test passed with real metadata.' },
          s3testfail: { expected: 'S3 test failed with a specific error class.' },
          s3envlocked: { expected: 'Env-locked fields are disabled with a reason.' },
          s3nokey: { expected: 'Missing-key state blocks save.' },
          s3confirm: { expected: 'S3 confirm explains consequences.' },
          s3moving: { expected: 'S3 move progress is real.' },
          s3moveerr: { expected: 'S3 move failure is recoverable.' },
          s3active: { expected: 'S3 active state.' },
          s3down: { expected: 'S3 unavailable state offers retry; no fake local fallback.' },
        },
      },
      yt: {
        context: { section: 'accounts' },
        valueNotes: {
          ok: { expected: 'YouTube account connected state (not the removed Data API key card).' },
          bot: { expected: 'YouTube bot/blocked state with a real reason.' },
        },
      },
      qr: {
        context: { section: 'accounts' },
        valueNotes: {
          none: { expected: 'No QR sheet.' },
          loading: { expected: 'QR loading state.' },
          waiting: { expected: 'QR waiting with real polling.' },
          scanned: { expected: 'QR scanned state.' },
          expired: { expected: 'QR expired with regenerate.' },
          rejected: { expected: 'QR rejected with reason.' },
          error: { expected: 'QR error with retry.' },
          success: { expected: 'QR success only after real provider confirmation.' },
        },
      },
      sprov: {
        context: { section: 'storage', store: 's3form' },
        valueNotes: {
          garage: { expected: 'Garage provider selected.' },
          r2: { expected: 'Cloudflare R2 selected.' },
          aws: { expected: 'AWS S3 selected.' },
          other: { expected: 'Other S3-compatible provider selected.' },
        },
      },
      sfail: {
        context: { section: 'storage', store: 's3testfail' },
        captureTarget: 'section[aria-labelledby="h-test"] [role="alert"]',
        valueNotes: {
          key: { expected: 'Invalid key failure.' },
          bucket: { expected: 'Bucket failure.' },
          perm: { expected: 'Permission failure.' },
          net: { expected: 'Network failure.' },
          clock: { expected: 'Clock skew failure.' },
          cors: { expected: 'CORS failure.' },
          reach: { expected: 'Unreachable endpoint failure.' },
        },
      },
      skey: {
        context: { section: 'storage', store: 's3form' },
        valueNotes: {
          new: { expected: 'New key entry.' },
          saved: { expected: 'Saved key mask/metadata; no secret in output.' },
        },
      },
      role: {
        context: { section: 'storage', store: 'server' },
        valueNotes: {
          admin: { expected: 'Admin view.' },
          member: { expected: 'Member view without admin actions.' },
        },
      },
      usage: {
        context: { section: 'storage', store: 'server' },
        valueNotes: {
          full: { expected: 'Full storage usage numbers.' },
          partial: { expected: 'Partial usage only where the source exists; no invented total.' },
        },
      },
    },
  },

  Channel: {
    page: 'desktop',
    ownerPR: 'PR4',
    phases: ['V38', 'V40'],
    route: '#/channels/<channelId>?tab=<tab>',
    baseline: baseline(
      'screens/channel/ChannelScreen.tsx plus VideosTab.tsx, ChannelAside.tsx, applied.ts, ScheduleCard.tsx; router CHANNEL_TABS still includes videos.',
      'V38 removes the Videos tab/aside/schedule, replaces rotation/autonomy with one default template and autoApprove/perDay; current baseline still has the old surfaces.',
    ),
    prerequisite: 'Three real QA channels with distinct identity/default-template state (MT/QH shared standard template, CL night-story template).',
    seed: 'Seed MT/CL/QH through the real channel API on the isolated QA DB; never overwrite user production channels.',
    expected: 'Channel artboard state matches the footer tweak; only Nguồn & tự động / Template mặc định / Hồ sơ remain.',
    props: {
      tab: {
        valueNotes: {
          sources: { expected: 'Sources & automation tab; default tab; four stats and real rival links.' },
          templates: { expected: 'Default-template tab with one template and Đổi/Sửa actions.' },
          settings: { expected: 'Channel profile tab with identity, language, tags, and delete.' },
        },
      },
      channel: {
        valueNotes: {
          MT: { expected: 'MT selected; stats, rivals, default template, and profile change together.' },
          CL: { expected: 'CL selected; stats, rivals, default template, and profile change together.' },
          QH: { expected: 'QH selected; stats, rivals, default template, and profile change together.' },
        },
      },
      autoApprove: {
        valueNotes: {
          true: { expected: 'Tự duyệt ảnh on; header chip “Tự duyệt ảnh ≥ 85”.' },
          false: { expected: 'Tự duyệt ảnh off; no approval chip; toggle persists after reload.' },
        },
      },
      tplMenu: {
        context: { tab: 'templates' },
        valueNotes: {
          true: { expected: 'Đổi template opens a floating list without pushing content.' },
          false: { expected: 'No template menu layer.' },
        },
      },
      identity: {
        context: { tab: 'settings' },
        valueNotes: {
          ready: { expected: 'Identity assets ready from real media.' },
          generating: { expected: 'Identity generation running with real progress.' },
          failed: { expected: 'Identity failure with a real retry; no fake avatar.' },
        },
      },
      link: {
        valueNotes: {
          none: { expected: 'No rival-link layer.' },
          picker: { expected: 'Rival picker lists real competitors.' },
          empty: { expected: 'No rivals to link; explains how to follow one.' },
        },
      },
      fresh: {
        valueNotes: {
          true: { expected: 'New channel with zero videos shows the empty workbench/default-template footer.' },
          false: { expected: 'Populated channel shows real stats/videos.' },
        },
      },
    },
  },

  CreateChannel: {
    page: 'desktop',
    ownerPR: 'PR4',
    phases: ['V38', 'V40'],
    route: '#/channels/new?step=<step>',
    baseline: baseline(
      'screens/channel/CreateStep.tsx, ChannelNewScreen.tsx, DoneStep.tsx, IdentityBlock.tsx.',
      'V38 replaces the old autonomy block with three rows (Template / Tự duyệt ảnh / Video mỗi ngày); V40 verifies the “Theo dõi & chọn” button.',
    ),
    prerequisite: 'Real followed-rival state and QA channel defaults; identity job can be seeded without production provider writes.',
    seed: 'Seed rivals and a QA identity job through the real channel/identity API.',
    expected: 'CreateChannel artboard state matches the footer tweak; template menu is a floating list and identity errors are real.',
    props: {
      step: {
        valueNotes: {
          create: { expected: 'Create step with three Cách kênh chạy rows and one primary action.' },
          done: { expected: 'Done step with real identity job status and next actions.' },
        },
      },
      rivals: {
        valueNotes: {
          some: { expected: 'Followed rivals selectable.' },
          none: { expected: 'No rivals followed; explains how to follow.' },
          empty: { expected: 'Empty rival search/result state.' },
        },
      },
      tplMenu: {
        valueNotes: {
          true: { expected: 'Template list opens as a floating list; does not push content; no link to Template page.' },
          false: { expected: 'Template list closed.' },
        },
      },
      identity: {
        context: { step: 'done' },
        valueNotes: {
          running: { expected: 'Identity generation running.' },
          failed: { expected: 'Artist identity failure with a real retry.' },
          director: { expected: 'Director could not name the channel; shows the real failed step.' },
        },
      },
    },
  },

  Onboard: {
    page: 'desktop',
    ownerPR: 'PR4',
    route: '#/setup',
    phases: ['V38'],
    routeByStep: { setup: '#/setup', create: '#/channels/new?step=create', done: '#/channels/new?step=done' },
    baseline: baseline(
      'screens/setup plus channel CreateStep/DoneStep.',
      'Onboard combines machine setup with the create-channel step; V38 keeps the three-row automation and template menu.',
    ),
    prerequisite: 'First-run QA state with tools installed/not installed and a real identity job.',
    seed: 'Use a fresh isolated QA HOME/config so no production tools/credentials are touched.',
    expected: 'Onboard artboard state matches the footer tweak; setup failures are real and retryable.',
    props: {
      step: {
        valueNotes: {
          setup: { expected: 'Machine setup step.' },
          create: { expected: 'Create-channel step with three automation rows.' },
          done: { expected: 'Done step with background identity status.' },
        },
      },
      setup: {
        valueNotes: {
          running: { expected: 'Tool setup running with real per-tool progress.' },
          login: { expected: 'ChatGPT login required; one primary action.' },
          failed: { expected: 'Tool failure with retry/settings path.' },
        },
      },
      tplMenu: {
        context: { step: 'create' },
        valueNotes: {
          true: { expected: 'Template list floating; no navigation away.' },
          false: { expected: 'Template list closed.' },
        },
      },
      identity: {
        context: { step: 'done' },
        valueNotes: {
          running: { expected: 'Identity running.' },
          failed: { expected: 'Artist identity failure.' },
          director: { expected: 'Director naming failure.' },
        },
      },
      rivals: {
        context: { step: 'create' },
        valueNotes: {
          some: { expected: 'Rivals available.' },
          none: { expected: 'No rivals followed.' },
          empty: { expected: 'Empty search state.' },
        },
      },
    },
  },

  NewVideo: {
    page: 'desktop',
    ownerPR: 'PR5',
    phases: ['V39'],
    route: '#/new?channel=<channelId> (legacy hash canonicalized to the Nhập nhanh dialog over the previous screen)',
    baseline: baseline(
      'screens/new/NewScreen.tsx, plan.ts, screens/RouteScreen.tsx and router #/new still render a separate page.',
      'V39 converts the page to one shared Nhập nhanh dialog; priority/select/review steps must disappear, not become an alias screen.',
    ),
    prerequisite: 'Real channel/default template, source URLs, duplicate detection, and image quota; no provider draw is needed for dialog states.',
    seed: 'Seed QA channels/templates and a known duplicate source through the real jobs/dedup API; do not enqueue production work.',
    expected: 'NewVideo artboard state matches the footer tweak; one primary action, real per-link validation, and no priority toggle.',
    props: {
      input: {
        valueNotes: {
          empty: { expected: 'Empty textarea; primary disabled until a valid link.' },
          pasted: { expected: 'One pasted link resolved with a real preview/title.' },
          duplicate: { expected: 'Duplicate source/channel returns the existing job/queue item, not a second enqueue.' },
          error: { expected: 'Invalid/unsupported link shows the real error and preserves other rows.' },
          multi: { expected: 'Multiple links show per-row status and “Làm N video” for valid rows.' },
        },
      },
      from: {
        valueNotes: {
          topbar: { expected: 'Opened from topbar/N; returns to the originating screen on close.' },
          explore: { expected: 'Opened from Explore with the selected videos; no paste field required.' },
        },
      },
      picker: {
        valueNotes: {
          none: { expected: 'No channel/template picker.' },
          channel: { expected: 'Channel picker is a floating list; changing channel resets template to that default.' },
          template: { expected: 'Template picker marks default vs “chỉ video này”; override does not PATCH the channel.' },
        },
      },
    },
  },

  Explore: {
    page: 'desktop',
    ownerPR: 'PR5',
    phases: ['V39'],
    route: '#/explore?rival=<rivalId>&video=<videoId>',
    baseline: baseline(
      'screens/explore/ExploreScreen.tsx, PreviewPanel.tsx, bulk.ts, RivalSidebar.tsx.',
      'V39 removes Vì sao?, makes filter/limit floating lists, and routes bulk selection through the same Nhập nhanh dialog.',
    ),
    prerequisite: 'Real followed rivals, scanned video history, filters, and a selected-video preview.',
    seed: 'Seed rival channels and scanned videos through the real explore API on isolated QA.',
    expected: 'Explore artboard state matches the footer tweak; bulk selection opens the canonical job dialog.',
    props: {
      select: {
        valueNotes: {
          true: { expected: 'Bulk selection checkboxes plus “Làm N video” opening the shared dialog.' },
          false: { expected: 'Normal browse; card click previews at right.' },
        },
      },
      rivalMenu: {
        valueNotes: {
          true: { expected: 'Rival ⋯ menu with real rescan/open/link/unfollow actions.' },
          false: { expected: 'Rival menu closed.' },
        },
      },
      preview: {
        valueNotes: {
          true: { expected: 'Right preview shows the selected real video, default template, and channel state.' },
          false: { expected: 'No preview selected.' },
        },
      },
      scan: {
        valueNotes: {
          running: { expected: 'Scan running with real progress.' },
          paused: { expected: 'Scan paused and resumable.' },
          limited: { expected: 'Scan limited by quota/plan with a real reason.' },
          done: { expected: 'Scan done with last-scan metadata.' },
        },
      },
      empty: {
        valueNotes: {
          true: { expected: 'No rivals followed; empty explanation and real add route.' },
          false: { expected: 'Rival list populated.' },
        },
      },
      search: {
        valueNotes: {
          none: { expected: 'No title search filter.' },
          match: { expected: 'Matching search results.' },
          nomatch: { expected: 'No-match state with clear-filter action.' },
        },
      },
      list: {
        valueNotes: {
          ok: { expected: 'Video list loads real rows.' },
          filterEmpty: { expected: 'Filters produce zero rows; clear filter restores list.' },
          firstScan: { expected: 'First scan state with progress/empty guidance.' },
          error: { expected: 'Load error with retry and preserved selection.' },
          offline: { expected: 'Offline state with cached data or clear retry.' },
        },
      },
    },
  },

  Queue: {
    page: 'desktop',
    ownerPR: 'PR5',
    phases: ['V39'],
    route: '#/queue',
    baseline: baseline(
      'screens/queue/QueueScreen.tsx, MobileQueue.tsx, queueView.ts; api/clone.ts and old priority/imageWorkers/rescanHours calls.',
      'V39 leaves only workers (1–6) + per-channel cadence; manual requests prepend and auto appends. Baseline still has old flags.',
    ),
    prerequisite: 'A real mixed queue: manual items, auto items, a running job, paused/busy/limited/empty variants.',
    seed: 'Seed queue items through the real queue/jobs API; do not pause a user queue.',
    expected: 'Queue artboard state matches the footer tweak; running job is not preempted and order is stable.',
    props: {
      state: {
        valueNotes: {
          running: { expected: 'Running queue with workers and channel cadence rules.' },
          paused: { expected: 'Paused queue; resume works and no job is silently restarted.' },
          limited: { expected: 'Image quota limited with a real reason and retry path.' },
          empty: { expected: 'Empty queue with a real create route.' },
        },
      },
    },
  },

  Activity: {
    page: 'desktop',
    ownerPR: 'PR3',
    phases: ['V37'],
    route: '#/ + Việc nền popover',
    baseline: baseline(
      'shell/ActivityPopover.tsx and screens/settings/NotificationSettings.tsx; recapOnReturn and per-kind off[] still exist at baseline.',
      'V37 removes Bản tin, collapses suggestions to one bool, and makes every popover control respond. Mixed legacy prefs must stop with a question, not be reset.',
    ),
    prerequisite: 'Real background tasks, notifications, notification prefs, and SSE reconnect state.',
    seed: 'Seed activity/notification rows through the real API; preflight legacy prefs separately.',
    expected: 'Activity artboard state matches the footer tweak; notification switches persist and no recap surface remains.',
    props: {
      view: {
        valueNotes: {
          running: { expected: 'Running background popover with real job/step counts.' },
          inbox: { expected: 'Notification inbox with real unread/read rows.' },
          toasts: { expected: 'Toast host examples; 4s default and undo only when a real inverse exists.' },
          system: { expected: 'System notifications with real links.' },
          settings: { expected: 'Notification settings: one suggestions bool, no recapOnReturn.' },
          empty: { expected: 'Empty activity without fake counts.' },
        },
      },
    },
  },

  Dialogs: {
    page: 'desktop',
    ownerPR: 'PR3',
    route: '#/ + dialog over the current screen',
    phases: ['V36', 'V37', 'V38', 'V39'],
    routeByDialog: {
      'delete-video': '#/?video=<jobId> + confirm',
      'delete-channel': '#/channels/<channelId> + confirm',
      'redraw-all': '#/review?video=<jobId> + confirm',
      restart: '#/?video=<jobId> + confirm',
      'clip-picker': '#/editor/<projectId> + clip picker',
      shortcuts: '#/ + shortcuts dialog',
      'channel-switch': '#/ + channel switch confirm',
      toasts: '#/ + toast host',
    },
    baseline: baseline(
      'components/confirm.tsx, Dialogs surfaces, and toast host exist; assistant-confirm/assistant-unknown and autonomy dialogs remain at baseline.',
      'V37 removes assistant dialogs and “template luân phiên” copy; V36 wires redraw-all confirm; V39 updates shortcut N to Nhập nhanh.',
    ),
    prerequisite: 'Real target entity for each dialog; no destructive mutation before confirmation.',
    seed: 'Seed one deletable video/channel and a review job with a known image count; use isolated QA only.',
    expected: 'Dialogs artboard state matches the footer tweak; Esc/×/Huỷ close without writing.',
    props: {
      dialog: {
        valueNotes: {
          'delete-video': { expected: 'Delete-video confirm names the real video and consequences.', ownerPR: 'PR3' },
          'delete-channel': { expected: 'Delete-channel confirm requires the real channel name; consequence count is real.', ownerPR: 'PR4' },
          'redraw-all': { expected: 'Redraw-all confirm shows the real image count and quota cost.', ownerPR: 'PR2' },
          restart: { expected: 'Restart confirm explains preserved work and restarts only the target.', ownerPR: 'PR3' },
          'clip-picker': { expected: 'Clip picker selects a real clip and returns it to the editor.', ownerPR: 'PR3' },
          shortcuts: { expected: 'Shortcuts dialog has N for Nhập nhanh and no assistant shortcut.', ownerPR: 'PR3' },
          'channel-switch': { expected: 'Channel-switch confirm preserves unsaved work or explains the loss.', ownerPR: 'PR3' },
          toasts: { expected: 'Toast variants render with real action/undo and 4s default.', ownerPR: 'PR3' },
        },
      },
      typed: {
        ownerPR: 'PR4',
        context: { dialog: 'delete-channel' },
        valueNotes: {
          true: { expected: 'Typed channel name matches; destructive button enabled.' },
          false: { expected: 'Typed name does not match; destructive button disabled with a reason.' },
        },
      },
    },
  },

  System: {
    page: 'desktop',
    ownerPR: 'PR3',
    phases: ['V37'],
    route: '#/',
    baseline: baseline(
      'System states in RouteScreen/System UI; shell still uses the old Main backdrop at baseline.',
      'V37 rebuilds loading/offline/not-found/empty-workbench on the new Main shell; retry must be real.',
    ),
    prerequisite: 'Real network/API failure and empty workbench data; do not fake errors by intercepting success.',
    seed: 'Use controlled QA API failure/empty data or a real offline toggle; record the mechanism.',
    expected: 'System artboard state matches the footer tweak; retry reconnects and empty workbench has four assistants/default-template footer.',
    props: {
      state: {
        valueNotes: {
          loading: { expected: 'Loading skeleton follows the new Main layout.' },
          offline: { expected: 'Disconnected state with “Đang kết nối lại…”, retry, and details.' },
          'not-found': { expected: 'Video not found explains the missing id and offers a valid destination.' },
          'empty-workbench': { expected: 'No videos: right column Bốn trợ lý, left footer Template mặc định, review badge hidden at 0.' },
        },
      },
    },
  },

  MReview: {
    page: 'mobile',
    ownerPR: 'PR6',
    phases: ['V36', 'V40'],
    route: '#/review?video=<jobId>',
    baseline: baseline(
      'screens/review/mobile/MReviewScreen.tsx and shared imageView.ts; MobileImageSheet remains the workbench image sheet.',
      'V40 removes Vì sao?/Ghi chú đã học and keeps the four prompt parts. Do not replace MReview with a round45 shared mobile viewer.',
      { id: 'mobile45-shared-image-viewer', note: 'Round45 mobile shared viewer is excluded; MReview/MobileImageSheet are kept and must not be expanded into it.' },
    ),
    prerequisite: 'A review job with real images, versions, submitted prompt, feedback, and approval state.',
    seed: 'Seed a review job through the real clone/images API on isolated QA; no provider smoke required for static states.',
    expected: 'MReview artboard state matches the footer tweak; actions use the real review APIs and preserve image/version.',
    props: {
      state: {
        valueNotes: {
          review: { expected: 'One large image, strip, versions, four-part prompt, sticky actions.' },
          prompt: { expected: 'Prompt sheet with only Cảnh/Phong cách/Tham chiếu/Tránh.' },
          feedback: { expected: 'Feedback sheet inserts/removes a phrase in the real feedback text.' },
          edit: { expected: 'Prompt edit saves per image; no learned note.' },
          redrawing: { expected: 'Redraw progress tied to the selected image/version.' },
          reveal: { expected: 'New version reveal with real version id.' },
          empty: { expected: 'Empty review state with a route back.' },
        },
      },
      score: {
        valueNotes: {
          true: { expected: 'Score surface visible for the image; no “Vì sao?” popover.' },
          false: { expected: 'Score surface absent.' },
        },
      },
    },
  },

  MMain: {
    page: 'mobile',
    ownerPR: 'PR6',
    route: '#/',
    phases: ['V40'],
    routeByScreen: {
      list: '#/',
      detail: '#/?video=<jobId>',
      jobs: '#/ + jobs sheet',
      inbox: '#/ + notifications tab',
      switch: '#/ + channel switch sheet',
    },
    baseline: baseline(
      'screens/workbench/mobile/MobileWorkbench.tsx, MobileWorkbenchList.tsx, MobileWorkbenchDetail.tsx, MobilePlanSheet.tsx, MobileRecap.tsx, MobileImageSheet.tsx.',
      'V40 removes recap/plan/composer/why, keeps MobileImageSheet without expanding it into a round45 viewer, and defaults detail to Nội dung đăng.',
      { id: 'mobile45-shared-image-viewer', note: 'Round45 mobile shared viewer is excluded; MReview/MobileImageSheet are kept and must not be expanded into it.' },
    ),
    prerequisite: 'Real mobile workbench data with list/detail/jobs/inbox/switch states.',
    seed: 'Seed channels/videos/notifications through the real API on isolated QA; mobile uses the same data as desktop.',
    expected: 'MMain artboard state matches the footer tweak; same data/actions as desktop and no recap/plan.',
    props: {
      screen: {
        valueNotes: {
          list: { expected: 'Workbench list with Chờ bạn rows and no recap card.' },
          detail: { expected: 'Video detail with Nội dung đăng default; Thông tin is a sheet.', captureTarget: 'section[aria-labelledby="pub-h"]' },
          jobs: { expected: 'Jobs/activity sheet with real counts and links.' },
          inbox: { expected: 'Notification tab with real unread/read rows.' },
          switch: { expected: 'Channel switch sheet with real channels and badges.' },
        },
      },
      stage: {
        context: { screen: 'detail' },
        valueNotes: {
          images: { expected: 'Images tab with real count; image sheet stays the existing MobileImageSheet.' },
          review: { expected: 'Review handoff opens the correct job/image in MReview.' },
        },
      },
      meta: {
        context: { screen: 'detail' },
        captureTarget: 'section[aria-labelledby="pub-h"]',
        valueNotes: {
          pending: { expected: 'Post content pending.' },
          writing: { expected: 'Writing progress from real events.' },
          failed: { expected: 'Metadata failure with recovery.' },
          ai: { expected: 'AI metadata visible/editable.' },
          edited: { expected: 'User edits survive reload.' },
          rewrite: { expected: 'Rewrite confirmation with real effect.', captureTarget: null },
          history: { expected: 'Persisted metadata history.', captureTarget: null },
        },
      },
      jobs: {
        context: { screen: 'jobs' },
        valueNotes: {
          running: { expected: 'Running job button with count.' },
          idle: { expected: 'Idle state.' },
          paused: { expected: 'Paused state with real time.' },
          offline: { expected: 'Offline state with retry.' },
        },
      },
      notice: {
        context: { screen: 'inbox' },
        valueNotes: {
          unread: { expected: 'Yellow dot + unread notification count.' },
          read: { expected: 'Read notification state.' },
          none: { expected: 'No notifications/dot.' },
        },
      },
      scope: {
        valueNotes: {
          all: { expected: 'All channels.' },
          channel: { expected: 'One channel; switcher preserves detail id.' },
        },
      },
    },
  },

  MChannel: {
    page: 'mobile',
    ownerPR: 'PR6',
    phases: ['V38', 'V40'],
    route: '#/channels/<channelId>?tab=<tab>',
    baseline: baseline(
      'screens/channel mobile branches plus mobile AutonomyMobile.tsx; mobile API consumers must already use the PR4 contract.',
      'V38 moves the mobile contract in PR4; V40 finishes layout, three automation rows, one default template, and sheets.',
    ),
    prerequisite: 'Real mobile channel data for MT/CL/QH with default templates and rival links.',
    seed: 'Use the same QA channels as desktop; no mobile-only feature set.',
    expected: 'MChannel artboard state matches the footer tweak; same options/defaults as desktop.',
    props: {
      tab: {
        valueNotes: {
          sources: { expected: 'Sources & automation tab with three automation rows.' },
          templates: { expected: 'Default template tab with sheet Đổi template.' },
          settings: { expected: 'Profile tab with identity/language/tags/delete.' },
        },
      },
      sheet: {
        contextByValue: { template: { tab: 'templates' } },
        valueNotes: {
          none: { expected: 'No sheet.' },
          rival: { expected: 'Rival sheet lists real competitors.' },
          unlink: { expected: 'Unlink confirm explains real affected videos/images.' },
          template: { expected: 'Template sheet with one default + Đổi/Sửa.' },
          link: { expected: 'Link-rival sheet validates a real handle/URL.' },
        },
      },
      fresh: {
        valueNotes: {
          true: { expected: 'New channel empty state.' },
          false: { expected: 'Populated channel.' },
        },
      },
      identity: {
        context: { tab: 'settings' },
        valueNotes: {
          ready: { expected: 'Identity ready.' },
          generating: { expected: 'Identity generating.' },
          failed: { expected: 'Identity failed with retry.' },
        },
      },
    },
  },

  MLibrary: {
    page: 'mobile',
    ownerPR: 'PR6',
    phases: ['V38', 'V40'],
    route: '#/library?tab=<tab>',
    baseline: baseline(
      'screens/library/mobile/MLibraryScreen.tsx and MTemplate/MPrompt/MOverlay/MBackdrop variants.',
      'PR4 moves the shared contract; V40 makes four equal tabs, counts in the description row, and ⋯/sheet actions.',
    ),
    prerequisite: 'Real library assets and channel-default refs shared with desktop.',
    seed: 'Use the same QA library seed as desktop.',
    expected: 'MLibrary artboard state matches the footer tweak; four tabs fit without horizontal scroll.',
    props: {
      tab: {
        valueNotes: {
          templates: { expected: 'Templates tab, four equal tabs.' },
          prompts: { expected: 'Prompts tab with real list.' },
          backdrops: { expected: 'Backdrops tab with real sources.' },
          overlays: { expected: 'Overlays tab with real assets.' },
        },
      },
      sheet: {
        valueNotes: {
          none: { expected: 'No sheet.' },
          actions: { expected: 'Asset/template actions sheet (rename/default/new) with real operations.' },
          new: { expected: 'New prompt/asset sheet with real save.' },
        },
      },
      empty: {
        valueNotes: {
          true: { expected: 'No templates: empty state with a real create route.' },
          false: { expected: 'Templates populated.' },
        },
      },
    },
  },

  MTemplate: {
    page: 'mobile',
    ownerPR: 'PR6',
    route: '#/templates/<templateId>',
    phases: ['V40'],
    routeByScreen: {
      list: '#/templates/<templateId>',
      images: '#/templates/<templateId>?section=images',
      cover: '#/templates/<templateId>?section=cover',
      backdrop: '#/templates/<templateId>?section=backdrop',
      output: '#/templates/<templateId>?section=output',
    },
    baseline: baseline(
      'screens/library/mobile/MTemplateScreen.tsx and template section components.',
      'V40 replaces rotation with “Kênh dùng làm mặc định”, keeps each template section as a child screen, and uses sheets for pickers.',
    ),
    prerequisite: 'Real template with section content and at least one default channel.',
    seed: 'Use the same QA template/channel refs as desktop.',
    expected: 'MTemplate artboard state matches the footer tweak; preview/save/rebuild are real.',
    props: {
      screen: {
        valueNotes: {
          list: { expected: 'Template list/child-screen entry.' },
          images: { expected: 'Images section.' },
          cover: { expected: 'Cover section.' },
          backdrop: { expected: 'Backdrop section.' },
          output: { expected: 'Output section.' },
        },
      },
      preview: {
        valueNotes: {
          fresh: { expected: 'Current preview.' },
          stale: { expected: 'Stale preview marked with rebuild.' },
          building: { expected: 'Real builder progress.' },
        },
      },
      sheet: {
        valueNotes: {
          none: { expected: 'No sheet.' },
          more: { expected: 'More sheet with real rebuild/sample/export/duplicate actions.' },
        },
      },
    },
  },

  MNew: {
    page: 'mobile',
    ownerPR: 'PR6',
    phases: ['V39', 'V40'],
    route: '#/new?channel=<channelId> (MNew overlay; legacy hash canonicalized to the shared Nhập nhanh flow)',
    baseline: baseline(
      'screens/new plus mobile MNew overlay; PR5 must move the mobile contract, PR6 finishes keyboard/layout.',
      'V39 canonical job/dedup/front contract; V40 keyboard-safe CTA and footer behavior. Do not create a separate mobile enqueue path.',
    ),
    prerequisite: 'Real channel/default template and source links; mobile keyboard/visualViewport behavior.',
    seed: 'Use the same QA jobs/dedup seed as desktop; no mobile-only job creation.',
    expected: 'MNew artboard state matches the footer tweak; keyboard does not cover the CTA/footer.',
    props: {
      input: {
        valueNotes: {
          empty: { expected: 'Empty input; disabled primary.' },
          pasted: { expected: 'Pasted single link resolved.' },
          duplicate: { expected: 'Duplicate returns existing job/queue item.' },
          error: { expected: 'Invalid link error with preserved rows.' },
          multi: { expected: 'Multi-link per-row status and “Làm N video”.' },
        },
      },
      from: {
        valueNotes: {
          topbar: { expected: 'Opened from mobile topbar; return to origin.' },
          explore: { expected: 'Opened from MExplore selection; carries selected videos.' },
        },
      },
      picker: {
        valueNotes: {
          none: { expected: 'No picker sheet.' },
          channel: { expected: 'Channel bottom sheet.' },
          template: { expected: 'Template bottom sheet with default/“chỉ video này”.' },
        },
      },
    },
  },

  MExplore: {
    page: 'mobile',
    ownerPR: 'PR6',
    phases: ['V39', 'V40'],
    route: '#/explore',
    baseline: baseline(
      'screens/explore/mobile/MExploreScreen.tsx, MVideoScreen.tsx.',
      'V39 moves the contract and bulk→MNew; V40 finishes sheets, filters, and 44px targets.',
    ),
    prerequisite: 'Real rivals/videos, selection, filters, and sheet states.',
    seed: 'Use the desktop explore QA seed; mobile reads the same data.',
    expected: 'MExplore artboard state matches the footer tweak; bulk selection opens MNew and chips open sheets.',
    props: {
      mode: {
        valueNotes: {
          browse: { expected: 'Browse list.' },
          select: { expected: 'Selection mode with “Làm N video”.' },
          info: { expected: 'Rival info sheet with ⋯ actions and scan pause/resume.' },
          add: { expected: 'Add rival flow/empty state with real follow.' },
        },
      },
      list: {
        valueNotes: {
          ok: { expected: 'Video list.' },
          noRivals: { expected: 'No rivals followed.' },
          filterEmpty: { expected: 'Filtered empty with clear action.' },
          firstScan: { expected: 'First scan progress.' },
          error: { expected: 'Load error with retry.' },
          offline: { expected: 'Offline state.' },
        },
      },
    },
  },

  MCompVideo: {
    page: 'mobile',
    ownerPR: 'PR6',
    phases: ['V39', 'V40'],
    route: '#/explore?rival=<rivalId>&video=<videoId>',
    baseline: baseline(
      'screens/explore/mobile/MVideoScreen.tsx includes the channel-sheet direct-add exception and “Vừa thêm” state.',
      '01 §6.2 and V39 preserve MCompVideo direct add with the channel default plus a secondary MNew link for override; do not force the generic dialog as the only path.',
    ),
    prerequisite: 'Rival video with chapters, channels, default templates, and a real duplicate/queue state.',
    seed: 'Seed one rival video and two channels with defaults through the real API.',
    expected: 'MCompVideo artboard state matches the footer tweak; direct add uses the channel default and updates “Vừa thêm”.',
    props: {
      sheet: {
        valueNotes: {
          none: { expected: 'Video detail; no sheet.' },
          picker: { expected: 'Channel picker sheet; choosing a channel adds directly with that channel default and marks “Vừa thêm”; secondary MNew link remains available.' },
        },
      },
    },
  },

  MQueue: {
    page: 'mobile',
    ownerPR: 'PR6',
    phases: ['V39', 'V40'],
    route: '#/queue',
    baseline: baseline(
      'screens/queue/MobileQueue.tsx; PR5 removes priority/imageWorkers/rescanHours controls and keeps workers/cadence.',
      'V40 final sheet/keyboard/44px layout; no why/priority controls.',
    ),
    prerequisite: 'Real queue states and workers/cadence config.',
    seed: 'Use the desktop queue QA seed; mobile reads the same queue.',
    expected: 'MQueue artboard state matches the footer tweak; only workers + per-channel cadence remain.',
    props: {
      state: {
        valueNotes: {
          running: { expected: 'Running queue.' },
          paused: { expected: 'Paused queue with resume.' },
          limited: { expected: 'Quota-limited queue.' },
          empty: { expected: 'Empty queue.' },
        },
      },
      sheet: {
        valueNotes: {
          none: { expected: 'No sheet.' },
          rules: { expected: 'Run-rules sheet: workers + channel cadence only.' },
          row: { expected: 'Row operations sheet with real pause/move/remove actions.' },
        },
      },
    },
  },

  MSystem: {
    page: 'mobile',
    ownerPR: 'PR6',
    route: '#/',
    phases: ['V35', 'V40'],
    routeByState: {
      loading: '#/',
      offline: '#/',
      notfound: '#/',
      empty: '#/',
      delVideo: '#/?video=<jobId> + confirm sheet',
      delChannel: '#/channels/<channelId> + confirm sheet',
      redrawAll: '#/?video=<jobId> + confirm sheet',
      restart: '#/?video=<jobId> + confirm sheet',
      clip: '#/ + clip sheet',
    },
    baseline: baseline(
      'MSystem system states and confirm sheets; old assistant states ask/unknown remain at baseline.',
      'V40 removes the two assistant sheets, changes “luân phiên” copy to default template, and keeps delete confirms with real consequences.',
    ),
    prerequisite: 'Real system failure/empty state and target entities for each confirm sheet.',
    seed: 'Use controlled QA API failure/empty data and the same entities as desktop/MMain.',
    expected: 'MSystem artboard state matches the footer tweak; every sheet is in-viewport and closeable.',
    props: {
      state: {
        valueNotes: {
          loading: { expected: 'Loading state.' },
          offline: { expected: 'Offline state with retry.' },
          notfound: { expected: 'Not-found state.' },
          empty: { expected: 'Empty workbench.' },
          delVideo: { expected: 'Delete-video confirm sheet.' },
          delChannel: { expected: 'Delete-channel confirm sheet with typed name.' },
          redrawAll: { expected: 'Redraw-all confirm sheet with image count.' },
          restart: { expected: 'Restart confirm sheet.' },
          clip: { expected: 'Clip confirm sheet.' },
        },
      },
      typed: {
        context: { state: 'delChannel' },
        valueNotes: {
          true: { expected: 'Typed channel name matches; destructive action enabled.' },
          false: { expected: 'Mismatch; action disabled.' },
        },
      },
    },
  },

  MSettings: {
    page: 'mobile',
    ownerPR: 'PR6',
    phases: ['V40'],
    route: '#/settings?section=<section>',
    baseline: baseline(
      'screens/settings mobile branches, MSettings storage/accounts/connect screens; data-key card still reachable at baseline.',
      'V40 removes the YouTube Data API card, adds provider logout confirm, and keeps model/storage/QR sheets in-viewport.',
    ),
    prerequisite: 'Real QA settings/storage/provider metadata; no production credential mutation.',
    seed: 'Use isolated QA config/storage; provider calls only with approved QA credentials.',
    expected: 'MSettings artboard state matches the footer tweak; no data-key card; sheets and 44px targets work.',
    props: {
      screen: {
        valueNotes: {
          list: { expected: 'Settings list.' },
          tools: { expected: 'Tools screen.' },
          connect: { expected: 'AI connections screen; no data-key card.' },
          agents: { expected: 'Assistant models screen.' },
          accounts: { expected: 'Platform accounts.' },
          storage: { expected: 'Storage screen.' },
        },
      },
      bili: {
        context: { screen: 'accounts' },
        valueNotes: {
          blocked: { expected: 'Bilibili blocked.' },
          saved: { expected: 'Bilibili saved.' },
          manual: { expected: 'Manual entry.' },
          expired: { expected: 'Expired with reconnect.' },
          empty: { expected: 'No account.' },
        },
      },
      qr: {
        context: { screen: 'accounts' },
        valueNotes: {
          none: { expected: 'No QR sheet.' },
          waiting: { expected: 'QR waiting.' },
          scanned: { expected: 'QR scanned.' },
          expired: { expected: 'QR expired.' },
          rejected: { expected: 'QR rejected.' },
          error: { expected: 'QR error.' },
          success: { expected: 'QR success only after real confirmation.' },
        },
      },
      store: {
        context: { screen: 'storage' },
        valueNotes: {
          here: { expected: 'Local storage.' },
          pick: { expected: 'Folder picker.' },
          lowspace: { expected: 'Low-space warning.' },
          moving: { expected: 'Move progress.' },
          done: { expected: 'Move done.' },
          offline: { expected: 'Offline storage.' },
          server: { expected: 'Server storage.' },
          s3active: { expected: 'S3 active.' },
          s3moving: { expected: 'S3 moving.' },
          s3down: { expected: 'S3 down with retry.' },
        },
      },
      sheet: {
        valueNotes: {
          none: { expected: 'No sheet.' },
          add: { expected: 'Add-provider/account sheet.' },
          models: { expected: 'Model catalogue sheet.' },
          director: { expected: 'Director model sheet.' },
          artist: { expected: 'Artist model sheet.' },
        },
      },
      usage: {
        context: { screen: 'storage', store: 'server' },
        valueNotes: {
          full: { expected: 'Full usage numbers.' },
          partial: { expected: 'Partial usage; no invented total.' },
        },
      },
    },
  },

  MCreate: {
    page: 'mobile',
    ownerPR: 'PR6',
    phases: ['V38', 'V40'],
    route: '#/channels/new?step=<step>',
    baseline: baseline(
      'screens/channel mobile create branches; PR4 moves the contract, PR6 finishes layout.',
      'V40 keeps the same fields/defaults as desktop; the follow-and-select button must call the real action and preserve selection after reload.',
    ),
    prerequisite: 'Real rivals, QA identity job, and three automation rows.',
    seed: 'Use the desktop CreateChannel QA seed.',
    expected: 'MCreate artboard state matches the footer tweak; same defaults as desktop.',
    props: {
      step: {
        valueNotes: {
          create: { expected: 'Create step.' },
          done: { expected: 'Done step with real identity status.' },
        },
      },
      rivals: {
        valueNotes: {
          some: { expected: 'Rivals available.' },
          none: { expected: 'No rivals.' },
          empty: { expected: 'Empty search.' },
        },
      },
      tplMenu: {
        valueNotes: {
          true: { expected: 'Template bottom sheet.' },
          false: { expected: 'Closed.' },
        },
      },
      identity: {
        context: { step: 'done' },
        valueNotes: {
          running: { expected: 'Identity running.' },
          failed: { expected: 'Identity failed with retry.' },
          director: { expected: 'Director naming failure.' },
        },
      },
    },
  },

  MAuth: {
    page: 'mobile',
    ownerPR: 'PR6',
    route: 'pre-session auth gate (no hash route)',
    phases: ['V40'],
    routeByScreen: {
      login: 'pre-session auth gate (no hash route)',
      denied: 'pre-session auth gate (no hash route)',
      password: 'pre-session auth gate (no hash route)',
      setup: 'pre-session auth gate (no hash route)',
      down: 'pre-session auth gate (no hash route)',
      account: '#/account',
    },
    baseline: baseline(
      'screens/auth mobile screens and account route; shared session store.',
      'V40 adds the eye toggle, logout-to-login with preserved username, retry states, and no data-key card. Uses the shared session store; do not alter identity DB.',
    ),
    prerequisite: 'Isolated identity DB and QA human session; no production account or provider credentials.',
    seed: 'Create a QA identity/user through the real identity store; record metadata only, never secrets.',
    expected: 'MAuth artboard state matches the footer tweak; session behavior is shared with desktop and no credentials leak.',
    props: {
      screen: {
        valueNotes: {
          login: { expected: 'Login gate.' },
          denied: { expected: 'No Viderse permission gate.' },
          password: { expected: 'Temporary-password change gate.' },
          setup: { expected: 'First-run admin setup gate.' },
          down: { expected: 'Server-down gate.' },
          account: { expected: 'Account page.' },
        },
      },
      kicked: {
        valueNotes: {
          none: { expected: 'No re-login reason block.' },
          self: { expected: 'Self logout reason.' },
          other: { expected: 'Other-device logout reason.' },
          password: { expected: 'Password changed reason.' },
          admin: { expected: 'Admin permission/reset reason.' },
          expired: { expected: '30-day expiry reason.' },
          unknown: { expected: 'Generic unknown reason.' },
        },
      },
      err: {
        valueNotes: {
          none: { expected: 'No login error.' },
          wrong: { expected: 'Wrong credentials error; fields preserved.' },
          limited: { expected: 'Rate-limited state with countdown.' },
        },
      },
      role: {
        context: { screen: 'account' },
        valueNotes: {
          admin: { expected: 'Admin role.' },
          member: { expected: 'Member role.' },
        },
      },
      other: {
        context: { screen: 'account', role: 'member' },
        valueNotes: {
          yes: { expected: 'Other app access present.' },
          no: { expected: 'Other app access absent; CTA follows the doc.' },
        },
      },
      app: {
        context: { screen: 'account' },
        valueNotes: {
          viderse: { expected: 'Viderse account menu/app context.' },
          noverse: { expected: 'Noverse tab context.' },
        },
      },
      step: {
        context: { screen: 'setup' },
        valueNotes: {
          code: { expected: 'Setup code step.' },
          account: { expected: 'Setup account step.' },
          recovery: { expected: 'Recovery-code step; confirmation required.' },
        },
      },
      down: {
        context: { screen: 'down' },
        valueNotes: {
          server: { expected: 'Viderse server down.' },
          accounts: { expected: 'Shared account store unreadable.' },
        },
      },
      pwerr: {
        context: { screen: 'password' },
        valueNotes: {
          none: { expected: 'No password error.' },
          mismatch: { expected: 'Mismatch error.' },
          wrong: { expected: 'Wrong current password.' },
        },
      },
      sheet: {
        context: { screen: 'account' },
        valueNotes: {
          none: { expected: 'No account sheet.' },
          password: { expected: 'Change-password sheet.' },
          devices: { expected: 'Devices sheet with real sessions.' },
        },
      },
      fail: {
        contextByValue: { copy: { screen: 'setup', step: 'recovery' }, norecovery: { screen: 'setup', step: 'recovery' } },
        valueNotes: {
          none: { expected: 'No failure.' },
          network: { expected: 'Network failure with retry.' },
          copy: { expected: 'Copy failure with retry.' },
          norecovery: { expected: 'Missing recovery confirmation blocks setup.' },
        },
      },
      busy: {
        valueNotes: {
          no: { expected: 'Primary idle.' },
          yes: { expected: 'Primary busy; prevents double submit.' },
        },
      },
    },
  },
}

/**
 * Property-level route overrides. `<value>` is replaced with the enum value;
 * `<channelId>`/`<templateId>`/`<jobId>` remain explicit fixture parameters.
 */
const PROP_ROUTES = {
  'Channel.tab': '#/channels/<channelId>?tab=<value>',
  'Channel.channel': '#/channels/<channelId>?tab=sources',
  'Channel.autoApprove': '#/channels/<channelId>?tab=sources',
  'Channel.tplMenu': '#/channels/<channelId>?tab=templates',
  'Channel.identity': '#/channels/<channelId>?tab=settings',
  'Channel.link': '#/channels/<channelId>?tab=sources',
  'Channel.fresh': '#/channels/<channelId>?tab=settings',
  'Library.tab': '#/library?tab=<value>',
  'Library.prompt': '#/library?tab=prompts&prompt=<value>',
  'Library.menu': '#/library?tab=templates',
  'Library.nguon': '#/library?tab=backdrops&nguon=<value>',
  'Library.import': '#/library?tab=templates',
  'Library.kenh': '#/library?tab=backdrops&nguon=channels&kenh=<value>',
  'Settings.section': '#/settings?section=<value>',
  'Settings.tools': '#/settings?section=tools',
  'Settings.conn': '#/settings?section=connect',
  'Settings.grok': '#/settings?section=connect',
  'Settings.ai': '#/settings?section=agents',
  'Settings.bili': '#/settings?section=accounts',
  'Settings.clean': '#/settings?section=storage',
  'Settings.store': '#/settings?section=storage',
  'Settings.yt': '#/settings?section=accounts',
  'Settings.qr': '#/settings?section=accounts',
  'Settings.sprov': '#/settings?section=storage',
  'Settings.sfail': '#/settings?section=storage',
  'Settings.skey': '#/settings?section=storage',
  'Settings.role': '#/settings?section=storage',
  'Settings.usage': '#/settings?section=storage',
  'Template.section': '#/templates/<templateId>?section=<value>',
  'Template.preview': '#/templates/<templateId>',
  'Template.fewClips': '#/templates/<templateId>?section=backdrop',
  'Template.picker': '#/templates/<templateId>',
  'CreateChannel.step': '#/channels/new?step=<value>',
  'CreateChannel.rivals': '#/channels/new?step=create',
  'CreateChannel.tplMenu': '#/channels/new?step=create',
  'CreateChannel.identity': '#/channels/new?step=done',
  'Onboard.setup': '#/setup',
  'Onboard.tplMenu': '#/channels/new?step=create',
  'Onboard.identity': '#/channels/new?step=done',
  'Onboard.rivals': '#/channels/new?step=create',
  'Dialogs.typed': '#/channels/<channelId> + confirm',
  'MChannel.tab': '#/channels/<channelId>?tab=<value>',
  'MChannel.sheet': '#/channels/<channelId>?tab=<value>',
  'MChannel.fresh': '#/channels/<channelId>?tab=settings',
  'MChannel.identity': '#/channels/<channelId>?tab=settings',
  'MLibrary.tab': '#/library?tab=<value>',
  'MLibrary.sheet': '#/library?tab=templates',
  'MLibrary.empty': '#/library?tab=templates',
  'MSettings.bili': '#/settings?section=accounts',
  'MSettings.qr': '#/settings?section=accounts',
  'MSettings.store': '#/settings?section=storage',
  'MSettings.usage': '#/settings?section=storage',
  'MSettings.sheet': '#/settings?section=connect',
  'MMain.stage': '#/?video=<jobId>',
  'MMain.meta': '#/?video=<jobId>',
  'MMain.jobs': '#/ + jobs sheet',
  'MMain.notice': '#/',
  'MMain.scope': '#/',
  'MSystem.typed': '#/channels/<channelId> + confirm sheet',
  'MCreate.step': '#/channels/new?step=<value>',
  'MCreate.rivals': '#/channels/new?step=create',
  'MCreate.tplMenu': '#/channels/new?step=create',
  'MCreate.identity': '#/channels/new?step=done',
  'MAuth.kicked': 'pre-session auth gate (no hash route)',
  'MAuth.err': 'pre-session auth gate (no hash route)',
  'MAuth.role': '#/account',
  'MAuth.other': '#/account',
  'MAuth.app': '#/account',
  'MAuth.step': 'pre-session auth gate (no hash route)',
  'MAuth.down': 'pre-session auth gate (no hash route)',
  'MAuth.pwerr': 'pre-session auth gate (no hash route)',
  'MAuth.sheet': '#/account',
  'MAuth.fail': 'pre-session auth gate (no hash route)',
  'MAuth.busy': 'pre-session auth gate (no hash route)',
}

const PROP_ROUTE_VALUES = {
  'Main.viewer.none': '#/?video=<jobId>',
  'Main.viewer.view': '#/?video=<jobId>',
  'Main.viewer.redraw': '#/?video=<jobId>',
  'Library.nguon.channels': '#/library?tab=backdrops&nguon=channels',
  'Library.nguon.videos': '#/library?tab=backdrops&nguon=videos',
  'Library.nguon.adding': '#/library?tab=backdrops&nguon=channels + add-channel dialog',
  'Library.nguon.addvideo': '#/library?tab=backdrops&nguon=videos + add-video dialog',
  'Library.nguon.unsupported': '#/library?tab=backdrops + unsupported-source error',
  'MQueue.sheet.none': '#/queue',
  'MQueue.sheet.rules': '#/queue?sheet=rules',
  'MQueue.sheet.row': '#/queue',
  'MChannel.sheet.none': '#/channels/<channelId>?tab=sources',
  'MChannel.sheet.rival': '#/channels/<channelId>?tab=sources',
  'MChannel.sheet.unlink': '#/channels/<channelId>?tab=sources',
  'MChannel.sheet.template': '#/channels/<channelId>?tab=templates',
  'MChannel.sheet.link': '#/channels/<channelId>?tab=sources',
  'MSettings.sheet.none': '#/settings',
  'MSettings.sheet.add': '#/settings?section=accounts',
  'MSettings.sheet.models': '#/settings?section=connect',
  'MSettings.sheet.director': '#/settings?section=agents',
  'MSettings.sheet.artist': '#/settings?section=agents',
  'MSettings.screen.list': '#/settings',
  'MSettings.screen.tools': '#/settings?section=tools',
  'MSettings.screen.connect': '#/settings?section=connect',
  'MSettings.screen.agents': '#/settings?section=agents',
  'MSettings.screen.accounts': '#/settings?section=accounts',
  'MSettings.screen.storage': '#/settings?section=storage',
  'MAuth.screen.login': 'pre-session auth gate (no hash route)',
  'MAuth.screen.denied': 'pre-session auth gate (no hash route)',
  'MAuth.screen.password': 'pre-session auth gate (no hash route)',
  'MAuth.screen.setup': 'pre-session auth gate (no hash route)',
  'MAuth.screen.down': 'pre-session auth gate (no hash route)',
  'MAuth.screen.account': '#/account',
}

function fillRoute(route, propKey, value) {
  return String(route)
    .replaceAll(`<${propKey}>`, String(value))
    .replaceAll('<value>', String(value))
}

/** Reference-only UI Kit boards: they carry only `$preview`, no tweaks. */
const REFERENCE_BOARDS = {
  UkOverview: ['V36', 'V37', 'V38', 'V39', 'V40'],
  UkFoundation: ['V36', 'V37', 'V38', 'V39', 'V40'],
  UkMotion: ['V37', 'V40'],
  UkControls: ['V37', 'V38', 'V39', 'V40'],
  UkInputs: ['V37', 'V38', 'V39', 'V40'],
  UkFeedback: ['V37', 'V40'],
  UkOverlays: ['V36', 'V37', 'V39', 'V40'],
  UkShell: ['V37', 'V40'],
  UkViderse: ['V36', 'V37', 'V38'],
}

const EXCLUDED_BOARDS = {
  Workflow: {
    reason:
      'Source authority for the workflow and the viewer/route rules, but it has no stateful tweak besides $preview. It is quoted, not enumerated as a screen row source.',
    sourceRefs: ['workflow', 'ia', 'dev'],
  },
  Auth: {
    reason:
      'Desktop auth/account is not in the V36–V40 owned board list; V40 lists MAuth only and phase-07 names MAuth. Auth is pre-session and is not a V36–V40 workflow state target.',
    sourceRefs: ['phase07', 'ia', 'plan'],
  },
  Brand: {
    reason:
      'Brand/logo board (UK.9) is not a workflow screen state; it does not own V36–V40 behavior. Its README frame also disagrees with its footer (recorded as a source note).',
    sourceRefs: ['ia', 'components'],
  },
  Kit: {
    reason: 'Archived old desktop kit; README explicitly says use UK.0–UK.8 instead.',
    sourceRefs: ['artboards-readme'],
  },
  MKit: {
    reason: 'Archived old mobile kit; README explicitly says use UK.0–UK.8 instead.',
    sourceRefs: ['artboards-readme'],
  },
}

const EXCLUSIONS = [
  {
    id: 'mobile45-shared-image-viewer',
    title: 'Trình xem ảnh dùng chung bản điện thoại (đợt 45)',
    status: 'excluded',
    appliesTo: ['MReview', 'MMain', 'MobileImageSheet'],
    reason:
      'Round 45 is explicitly outside V36–V40. Do not design or implement a shared mobile image viewer. MReview stays a review screen, and the existing MobileImageSheet is kept (V40 may edit its content/feedback) but is not expanded into round45.',
    sourceRefs: ['workflow', 'phase07', 'dev', 'inventory', 'ia'],
    notExcluded: ['MReview rows (included)', 'MMain rows (included)', 'MobileImageSheet current image/version/prompt behavior (kept, not expanded)'],
    mustStillVerify: ['MReview states and score', 'MobileImageSheet current image/version/prompt behavior', 'MMain detail image count'],
  },
  {
    id: 'mobile-editor',
    title: 'Mobile Editor',
    status: 'excluded',
    appliesTo: ['Editor (mobile variant only)'],
    reason:
      '01 §6.2 says Editor exists only on desktop; links on mobile show “Mở trên máy tính”. V40 does not invent a mobile Editor.',
    sourceRefs: ['ia', 'phase07'],
    notExcluded: ['Editor desktop states (included as a PR3 board)'],
    mustStillVerify: ['Desktop Editor states', 'Mobile “Mở trên máy tính” link'],
  },
  {
    id: 'noverse-artboards',
    title: 'Noverse artboards',
    status: 'excluded',
    appliesTo: ['apps/noverse/docs/design/v1/artboards/*'],
    reason:
      'The PR0 scope is Viderse workflow rounds 40–44. Noverse boards are a separate product and are not source for Viderse screen states.',
    sourceRefs: ['plan', 'inventory'],
    mustStillVerify: [],
  },
  {
    id: 'archived-kit-boards',
    title: 'Kit / MKit archived boards',
    status: 'excluded',
    appliesTo: ['Kit', 'MKit'],
    reason: 'README marks both as archived; UK.0–UK.8 are the reference kit.',
    sourceRefs: ['artboards-readme', 'ia'],
    mustStillVerify: [],
  },
]

const SCREENS_WITHOUT_ARTBOARD = [
  {
    id: 'MVideo',
    title: 'Mobile video detail',
    ownerPR: 'PR6',
    mappedTo: 'MMain screen=detail',
    reason: 'No dedicated MVideo.dc.html; 01 §6.2 describes it inside MMain detail.',
    sourceRefs: ['ia', 'phase07'],
    baseline: ['apps/viderse/web/src/screens/workbench/mobile/MobileWorkbenchDetail.tsx'],
  },
  {
    id: 'MMore',
    title: 'Mobile More/settings entry',
    ownerPR: 'PR6',
    mappedTo: 'MSettings screen=list + mobile shell',
    reason: 'No dedicated MMore.dc.html; 01 §6.2 and phase-07 describe four More rows; use MSettings/shell owners, do not create a phantom screen.',
    sourceRefs: ['ia', 'phase07', 'inventory'],
    baseline: ['apps/viderse/web/src/shell/MobileShell.tsx', 'apps/viderse/web/src/screens/settings/SettingsScreen.tsx'],
  },
  {
    id: 'MNotifications',
    title: 'Mobile notifications',
    ownerPR: 'PR6',
    mappedTo: 'MMain screen=inbox',
    reason: 'No dedicated MNotifications.dc.html; notifications are the inbox tab of the jobs sheet (01 §6.2).',
    sourceRefs: ['ia', 'phase07'],
    baseline: ['apps/viderse/web/src/shell/ActivityPopover.tsx'],
  },
  {
    id: 'MTop',
    title: 'Mobile top bar',
    ownerPR: 'PR6',
    mappedTo: 'mobile shell',
    reason: 'No dedicated MTop.dc.html; the actual owner is MobileShell, not a new phantom component.',
    sourceRefs: ['ia', 'phase07', 'inventory'],
    baseline: ['apps/viderse/web/src/shell/MobileShell.tsx'],
  },
]

const BASELINE_COMPONENTS = [
  {
    id: 'shared-desktop-image-viewer',
    path: 'apps/viderse/web/src/components/image-viewer/ImageViewer.tsx',
    status: 'missing-at-baseline',
    ownerPR: 'PR2',
    note: 'V36 creates one typed shared viewer; Review/Editor/Template/Library/Main must reuse it instead of keeping screen-local viewer chrome.',
    sourceRefs: ['inventory', 'phase03', 'components'],
  },
  {
    id: 'MobileImageSheet',
    path: 'apps/viderse/web/src/screens/workbench/mobile/MobileImageSheet.tsx',
    status: 'present-keep',
    ownerPR: 'PR6',
    note: 'Existing mobile image sheet is kept and may receive V40 content/feedback edits only; it MUST NOT be expanded into the round45 shared mobile viewer.',
    exclusionId: 'mobile45-shared-image-viewer',
    sourceRefs: ['inventory', 'phase07', 'ia'],
  },
  {
    id: 'YoutubeDataKeyCard',
    path: 'apps/viderse/web/src/screens/settings/connect/YoutubeDataKeyCard.tsx',
    status: 'present-remove',
    ownerPR: 'PR2',
    note: 'Remove the card and the actual /api/youtube/data-key surface from desktop and mobile; do not leave a hidden live read.',
    sourceRefs: ['inventory', 'phase03', 'phase07'],
  },
  {
    id: 'NewScreen-page',
    path: 'apps/viderse/web/src/screens/new/NewScreen.tsx',
    status: 'present-convert',
    ownerPR: 'PR5',
    note: 'Convert the page to the shared Nhập nhanh dialog; no functional alias screen/route remains.',
    sourceRefs: ['inventory', 'phase06'],
  },
  {
    id: 'channel-videos-tab',
    path: 'apps/viderse/web/src/screens/channel/VideosTab.tsx',
    status: 'present-remove',
    ownerPR: 'PR4',
    note: 'Remove the Channel Videos tab with ChannelAside/applied/ScheduleCard when the last consumer is migrated.',
    sourceRefs: ['inventory', 'phase05'],
  },
  {
    id: 'mobile-recap-plan',
    path: 'apps/viderse/web/src/screens/workbench/mobile/MobileRecap.tsx',
    status: 'present-remove',
    ownerPR: 'PR6',
    note: 'Remove recap/plan/composer/why mobile surfaces; keep image/review actions that V40 keeps.',
    sourceRefs: ['inventory', 'phase07'],
  },
]

const SOURCE_CONFLICTS = [
  {
    id: 'per-day-upper-bound',
    topic: 'Channel perDay upper bound',
    values: {
      '01-ia-flows §W2': '1–50 (default 5)',
      '08-dev-prompt V38': '1–20',
      'phase-05-v38-channels-and-library.md': '1–20',
    },
    resolution:
      'Use 1–20 as the PR4 implementation bound: the phase plan and 08 V38 are the owning sources; W2 is outside the cited authority block (01 §0.3/§6.2 Đợt44/W3–W13) and is recorded here rather than silently clamped.',
    ownerPR: 'PR4',
    sourceRefs: ['ia', 'dev', 'phase05'],
  },
  {
    id: 'readme-brand-frame',
    topic: 'README frame vs footer for Brand',
    values: {
      'README.md': '1440×2840',
      'Brand.dc.html data-props': '1440×3000',
    },
    resolution: 'Footer wins per source authority; Brand is excluded from V36–V40 state rows, and the mismatch is recorded for catalog hygiene.',
    ownerPR: null,
    sourceRefs: ['artboards-readme'],
  },
]

const PROBES = [
  {
    id: 'probe.channel.per-day',
    kind: 'numeric',
    artboard: 'Channel',
    page: 'desktop',
    property: 'perDay',
    domain: { min: 1, max: 20, default: 5, step: 1, unit: 'videos/day' },
    probes: [0, 1, 5, 20, 21],
    prerequisite: 'QA channel in Nguồn & tự động with a real perDay value and no production channel mutation.',
    route: '#/channels/<channelId>?tab=sources',
    ownerPR: 'PR4',
    observedVisible: null,
    expectedVisible:
      '0/21 are rejected with a user-visible range reason; 1/20 save and read back; the stepper disables at bounds. Existing >20 data must stop for a preflight question, never be clamped.',
    baseline: baseline('screens/channel/SourcesTab.tsx / PaceStepper.tsx; old autonomy/rotation path at baseline.', 'V38 introduces perDay 1–20; the 01 W2 1–50 text is recorded as a source conflict.'),
    sourceRefs: ['dev', 'phase05', 'ia'],
    dcPropNote: 'No perDay data-prop exists; Channel.tab=sources and Channel.autoApprove frame the automation block. Probe value derived from 08 V38/01 §W11, not invented as a DC prop.',
    dcPropRefs: ['Channel.tab', 'Channel.autoApprove'],
    evidence: PENDING_EVIDENCE(),
    status: 'pending',
    blockedReason: null,
  },
  {
    id: 'probe.channel.auto-approve-threshold',
    kind: 'numeric',
    artboard: 'Channel',
    page: 'desktop',
    property: 'autoApprove.score',
    domain: { min: 0, max: 100, threshold: 85, unit: 'score' },
    probes: [84, 85, 100],
    prerequisite: 'Real scored images on a QA video; one channel with autoApprove off and one on.',
    route: '#/channels/<channelId>?tab=sources',
    ownerPR: 'PR4',
    observedVisible: null,
    expectedVisible:
      '84 keeps the whole video in Chờ duyệt; 85 with every image ≥85 auto-approves only when the switch is on; 100 passes. Toggling off never auto-approves.',
    baseline: baseline('screens/channel/SourcesTab.tsx + components/autonomy/*; old autonomy modes at baseline.', 'V38 replaces autonomy with one bool at fixed 85.'),
    sourceRefs: ['dev', 'phase05', 'ia', 'data'],
    dcPropNote: 'Direct DC prop: Channel.autoApprove boolean; the 84/85/100 scores are derived boundary probes for the fixed threshold.',
    dcPropRefs: ['Channel.autoApprove'],
    evidence: PENDING_EVIDENCE(),
    status: 'pending',
    blockedReason: null,
  },
  {
    id: 'probe.channel.auto-approve-cases',
    kind: 'text',
    artboard: 'Channel',
    page: 'desktop',
    property: 'autoApprove.cases',
    probes: ['all-above', 'one-below', 'missing-score', 'drawing', 'failed', 'mixed-batch'],
    prerequisite: 'Six real image cases on one QA job; score/missing/drawing/failed states come from the pipeline, not CSS.',
    route: '#/channels/<channelId>?tab=sources',
    ownerPR: 'PR4',
    observedVisible: null,
    expectedVisible:
      'Only an all-scored ≥85 batch auto-approves when the switch is on; missing/drawing/failed/below-85 images keep the video waiting.',
    baseline: baseline('components/autonomy/scoreSummary and job image states exist; V38 must preserve all cases.', 'No fake “ready” for missing/drawing images.'),
    sourceRefs: ['phase05', 'dev'],
    dcPropNote: 'Direct DC prop: Channel.autoApprove boolean; case names are derived image-state probes, not DC props.',
    dcPropRefs: ['Channel.autoApprove'],
    evidence: PENDING_EVIDENCE(),
    status: 'pending',
    blockedReason: null,
  },
  {
    id: 'probe.queue.workers',
    kind: 'numeric',
    artboard: 'Queue',
    page: 'desktop',
    property: 'workers',
    domain: { min: 1, max: 6, step: 1, unit: 'workers' },
    probes: [0, 1, 6, 7],
    prerequisite: 'Real queue with running/manual/auto items; pause a QA queue only.',
    route: '#/queue?sheet=rules',
    ownerPR: 'PR5',
    observedVisible: null,
    expectedVisible:
      '0/7 are rejected or disabled with a reason; 1/6 save and apply without preempting a running job; imageConcurrency stays 3 and scan cycle 1h server-side.',
    baseline: baseline('screens/queue/QueueScreen.tsx, queueView.ts; old settings keys at baseline.', 'V39 removes imageWorkers/rescanHours/priority from public config.'),
    sourceRefs: ['dev', 'phase06', 'ia'],
    dcPropNote: 'No workers data-prop exists; Queue.state and MQueue.sheet frame the rules UI. Probe derived from 08 V39/01 §W12.',
    dcPropRefs: ['Queue.state', 'MQueue.sheet'],
    evidence: PENDING_EVIDENCE(),
    status: 'pending',
    blockedReason: null,
  },
  {
    id: 'probe.queue.fixed-server-values',
    kind: 'numeric',
    artboard: 'MQueue',
    page: 'mobile',
    property: 'imageConcurrency/rescanHours',
    domain: { imageConcurrency: 3, rescanHours: 1, editable: false },
    probes: [3, 1],
    prerequisite: 'Real queue endpoint response on QA; no client-side override.',
    route: '#/queue',
    ownerPR: 'PR5',
    observedVisible: null,
    expectedVisible:
      'Mobile run rules show only workers + per-channel cadence; image concurrency 3 and scan cycle 1h are described as fixed, not editable controls.',
    baseline: baseline('screens/queue/MobileQueue.tsx; old hidden setting reads at baseline.', 'V39/V40 remove the old editable keys.'),
    sourceRefs: ['dev', 'phase06', 'ia'],
    dcPropNote: 'No public fixed-value data-prop; Queue.state/MQueue.sheet frame the rules UI; values 3/1 come from 08 V39.',
    dcPropRefs: ['Queue.state', 'MQueue.sheet'],
    evidence: PENDING_EVIDENCE(),
    status: 'pending',
    blockedReason: null,
  },
  {
    id: 'probe.main.selection-count',
    kind: 'numeric',
    artboard: 'Main',
    page: 'desktop',
    property: 'select.count',
    domain: { min: 0, max: 6, step: 1, unit: 'videos' },
    probes: [0, 1, 2, 6],
    prerequisite: 'Real ready videos and one non-ready/issue video; QA workbench.',
    route: '#/',
    ownerPR: 'PR3',
    observedVisible: null,
    expectedVisible:
      '0 hides the sticky bar; 1/2/6 show “Tải gói N video” and only eligible ready files are included; no whole-library download.',
    baseline: baseline('screens/workbench/WorkbenchDesktop.tsx; selection/export logic exists but V37 must keep it real.', 'Bulk download ZIP must contain exactly the selected ready videos.'),
    sourceRefs: ['phase04', 'dev'],
    dcPropNote: 'Direct DC prop: Main.select boolean; counts 0/1/2/6 are derived selection-size probes.',
    dcPropRefs: ['Main.select'],
    evidence: PENDING_EVIDENCE(),
    status: 'pending',
    blockedReason: null,
  },
  {
    id: 'probe.new-video.link-count',
    kind: 'numeric',
    artboard: 'NewVideo',
    page: 'desktop',
    property: 'input.count',
    domain: { min: 0, max: null, step: 1, unit: 'links', note: 'No design upper bound; probe 0/1/multi-row behavior, not an invented cap.' },
    probes: [0, 1, 4],
    prerequisite: 'Real channels/templates and one known duplicate source; isolated QA.',
    route: '#/new?channel=<channelId>',
    ownerPR: 'PR5',
    observedVisible: null,
    expectedVisible:
      '0 disables the primary; 1 shows one row; 4 shows per-row status and “Làm N video”; duplicate rows return the existing job/queue item and are not enqueued twice.',
    baseline: baseline('screens/new/NewScreen.tsx; old one/many tabs and priority at baseline.', 'V39 canonical POST /api/jobs + dedup; no priority flag.'),
    sourceRefs: ['phase06', 'dev', 'data'],
    dcPropNote: 'Direct DC props: NewVideo.input and MNew.input enums; counts 0/1/4 are derived link-count probes.',
    dcPropRefs: ['NewVideo.input', 'MNew.input'],
    evidence: PENDING_EVIDENCE(),
    status: 'pending',
    blockedReason: null,
  },
  {
    id: 'probe.template.default-channel-count',
    kind: 'numeric',
    artboard: 'Template',
    page: 'desktop',
    property: 'defaultChannelCount',
    domain: { min: 0, max: 3, step: 1, unit: 'channels' },
    probes: [0, 1, 2],
    prerequisite: 'One shared standard template used by MT and QH, plus an unused template.',
    route: '#/templates/<templateId>',
    ownerPR: 'PR4',
    observedVisible: null,
    expectedVisible:
      '0 shows no default chip; 1 shows “Mặc định của <kênh>”; 2 shows “Mặc định của 2 kênh” with both real channel names. Delete/rename keeps shared refs intact.',
    baseline: baseline('screens/library/TemplatesTab.tsx, AttachChannelDialog.tsx; rotation helpers at baseline.', 'V38 single default template per channel; shared template refs allowed.'),
    sourceRefs: ['phase05', 'dev', 'inventory'],
    dcPropNote: 'No defaultChannelCount DC prop; Template.fewClips and Channel.tab frame the default-template UI; 0/1/2 derive from V38 channel refs.',
    dcPropRefs: ['Template.fewClips', 'Channel.tab'],
    evidence: PENDING_EVIDENCE(),
    status: 'pending',
    blockedReason: null,
  },
  {
    id: 'probe.viewer.frame',
    kind: 'numeric-pair',
    artboard: 'Main',
    page: 'desktop',
    property: 'viewer.frame',
    domain: { maxWidth: 1240, maxHeight: 800, contain: true, noUpscale: true },
    probes: ['1240x800', '1440x900', '390x844'],
    prerequisite: 'Real image at a known natural size; one portrait and one landscape image.',
    route: '#/?video=<jobId>',
    ownerPR: 'PR2',
    observedVisible: null,
    expectedVisible:
      'Viewer never exceeds 1240×800, image uses contain and is not upscaled past natural size. The 390×844 check is the mobile MReview/MMain image presentation, not a round45 shared viewer.',
    baseline: baseline('Shared ImageViewer missing; Review/Editor screen-local image chrome.', 'V36 creates one viewer with max 1240×800.'),
    sourceRefs: ['components', 'phase03', 'dev'],
    dcPropNote: 'Direct DC props: Main.viewer and Review.state; frame sizes are 04-components bounds, not props.',
    dcPropRefs: ['Main.viewer', 'Review.state'],
    evidence: PENDING_EVIDENCE(),
    status: 'pending',
    blockedReason: null,
  },
  {
    id: 'probe.mobile.touch-target',
    kind: 'numeric',
    artboard: 'MMain',
    page: 'mobile',
    property: 'interactive.minTouch',
    domain: { minPx: 44, appliesTo: 'buttons/links/tabs/radio/switch' },
    probes: [44, 43],
    prerequisite: 'Every visible mobile control in each M* tweak, including sheets and opened menus.',
    route: '#/',
    ownerPR: 'PR6',
    observedVisible: null,
    expectedVisible:
      'All interactive targets are ≥44px; 43 is a failure probe used to catch regressions. Visual 36px controls may have a 44px hit area.',
    baseline: baseline('MobileShell/MMain/MChannel controls; shared kit has 44px hit-area utilities.', 'V40 audits all 13 mobile boards.'),
    sourceRefs: ['verification', 'ia', 'kit'],
    dcPropNote: 'No touch-size DC prop; MMain.screen/MChannel.sheet/MSettings.sheet frame the audited controls; 44/43 derive from verification/04-components.',
    dcPropRefs: ['MMain.screen', 'MChannel.sheet', 'MSettings.sheet'],
    evidence: PENDING_EVIDENCE(),
    status: 'pending',
    blockedReason: null,
  },
  {
    id: 'probe.copy.language',
    kind: 'text',
    artboard: 'MReview',
    page: 'mobile',
    property: 'copy.language',
    probes: ['ui-vi', 'source-title-raw', 'submitted-prompt-raw'],
    prerequisite: 'Video with a non-Vietnamese source title and a raw submitted prompt.',
    route: '#/review?video=<jobId>',
    ownerPR: 'PR6',
    observedVisible: null,
    expectedVisible:
      'UI labels/errors/progress are Vietnamese; source video titles and the submitted/raw prompt keep their original language verbatim. Do not translate or stringify raw exceptions into the UI.',
    baseline: baseline('06-copy contract applies to Review/MReview; baseline copy is mixed.', 'V40 checks mobile labels/errors/feedback.'),
    sourceRefs: ['copy', 'phase07', 'verification'],
    dcPropNote: 'Direct DC props: MReview.state and Review.prompt; language cases derive from 06-copy, not props.',
    dcPropRefs: ['MReview.state', 'Review.prompt'],
    evidence: PENDING_EVIDENCE(),
    status: 'pending',
    blockedReason: null,
  },
  {
    id: 'probe.labels.long',
    kind: 'text',
    artboard: 'MMain',
    page: 'mobile',
    property: 'labels.long',
    probes: ['long-channel-name', 'long-template-name', 'long-video-title', 'long-prompt'],
    prerequisite: 'QA entities with long real labels; no CSS truncation-only fake data.',
    route: '#/',
    ownerPR: 'PR6',
    observedVisible: null,
    expectedVisible:
      'Long labels wrap or ellipsize according to the component contract without horizontal overflow, clipping avatars, or hiding the primary action; mobile title uses the documented 16px single-line ellipsis where specified.',
    baseline: baseline('Shell/card/list label styles at baseline; V40 audits 360/430px and long scroll.', 'No new color/type token for long labels.'),
    sourceRefs: ['verification', 'components', 'ia'],
    dcPropNote: 'No label-length DC prop; MMain.screen/MChannel.tab frame the audited lists; cases derive from verification long-label gate.',
    dcPropRefs: ['MMain.screen', 'MChannel.tab'],
    evidence: PENDING_EVIDENCE(),
    status: 'pending',
    blockedReason: null,
  },
]

// ---------------------------------------------------------------------------
// Parsing
// ---------------------------------------------------------------------------

function fail(message, details) {
  const error = new Error(message)
  if (details !== undefined) error.details = details
  throw error
}

function read(relPath) {
  const abs = join(REPO, relPath)
  if (!existsSync(abs)) fail(`Missing source file: ${relPath}`)
  return readFileSync(abs, 'utf8')
}

function sha256(text) {
  return createHash('sha256').update(text).digest('hex')
}

/**
 * Parse exactly one `data-props` HTML attribute. A naive regex would silently
 * truncate on quotes/apostrophes; this scanner tracks the HTML quote and JSON
 * backslash escapes, then JSON.parse validates the payload.
 */
function extractDataProps(html, relPath) {
  const token = 'data-props='
  const hits = []
  let from = 0
  for (;;) {
    const index = html.indexOf(token, from)
    if (index < 0) break
    const before = html[index - 1]
    const isBoundary = !before || !/[A-Za-z0-9_-]/.test(before)
    if (isBoundary) hits.push(index)
    from = index + token.length
  }
  if (hits.length !== 1) {
    fail(`Expected exactly one data-props attribute in ${relPath}, found ${hits.length}`, { hits })
  }
  const start = hits[0] + token.length
  const quote = html[start]
  if (quote !== "'" && quote !== '"') {
    fail(`data-props must be quoted in ${relPath}; found ${JSON.stringify(quote)}`)
  }
  let index = start + 1
  let payload = ''
  let escaped = false
  for (; index < html.length; index += 1) {
    const char = html[index]
    if (escaped) {
      payload += char
      escaped = false
      continue
    }
    if (char === '\\') {
      payload += char
      escaped = true
      continue
    }
    if (char === quote) break
    payload += char
  }
  if (index >= html.length) fail(`Unterminated data-props attribute in ${relPath}`)
  const rest = html[index + 1]
  if (rest && !/[\s>/]/.test(rest)) {
    fail(`data-props attribute is not followed by an attribute/tag boundary in ${relPath}: ${JSON.stringify(rest)}`)
  }
  let props
  try {
    props = JSON.parse(payload)
  } catch (error) {
    fail(`Invalid JSON in data-props for ${relPath}: ${error.message}`, { payloadStart: payload.slice(0, 160) })
  }
  if (!props || typeof props !== 'object' || Array.isArray(props)) {
    fail(`data-props must be a JSON object in ${relPath}`)
  }
  return { props, raw: payload, sha256: sha256(payload) }
}

function validatePreview(name, props) {
  const preview = props.$preview
  if (!preview || typeof preview !== 'object') fail(`${name}: missing $preview`)
  if (!Number.isInteger(preview.width) || !Number.isInteger(preview.height)) {
    fail(`${name}: $preview.width/height must be integers`, preview)
  }
  return preview
}

function validateProp(name, key, spec) {
  const where = `${name}.${key}`
  if (!spec || typeof spec !== 'object' || Array.isArray(spec)) fail(`${where}: property definition must be an object`)
  if (!spec.section || typeof spec.section !== 'string') fail(`${where}: missing section label`)
  const editor = spec.editor
  if (editor === 'enum') {
    if (!Array.isArray(spec.options) || spec.options.length === 0) fail(`${where}: enum needs options`)
    if (new Set(spec.options).size !== spec.options.length) fail(`${where}: duplicate enum options`)
    for (const option of spec.options) {
      if (typeof option !== 'string' || option.length === 0) fail(`${where}: enum options must be non-empty strings`, option)
    }
    if (!spec.options.includes(spec.default)) fail(`${where}: default is not an option`, spec.default)
    return 'enum'
  }
  if (editor === 'boolean') {
    if (typeof spec.default !== 'boolean') fail(`${where}: boolean default must be true/false`)
    return 'boolean'
  }
  if (editor === 'number') {
    if (typeof spec.default !== 'number' || !Number.isFinite(spec.default)) fail(`${where}: number default must be finite`)
    if (spec.min !== undefined && typeof spec.min !== 'number') fail(`${where}: number min must be numeric`)
    if (spec.max !== undefined && typeof spec.max !== 'number') fail(`${where}: number max must be numeric`)
    return 'number'
  }
  if (editor === 'text') {
    if (typeof spec.default !== 'string') fail(`${where}: text default must be a string`)
    return 'text'
  }
  fail(`${where}: unsupported editor ${JSON.stringify(editor)}; expected enum|boolean|number|text`)
}

/** Parse the README artboard table without silently dropping malformed rows. */
function parseReadme(readmeText) {
  const lines = readmeText.split(/\r?\n/)
  const header = lines.find((line) => line.startsWith('| Tệp | Màn |'))
  if (!header) fail('README artboard table header not found')
  const versionMatch = readmeText.match(/phiên bản\s+(\d+),\s*ngày\s*([0-9]{2}\/[0-9]{2}\/[0-9]{4})/)
  if (!versionMatch) fail('README version/date line not found')
  const rows = []
  for (const [lineNumber, line] of lines.entries()) {
    if (!line.startsWith('| `')) continue
    const match = line.match(/^\| `([^`]+)` \| (.*?) \| (.*?) \| (\d+)×(\d+) \| (.*) \|$/)
    if (!match) fail(`README row cannot be parsed at line ${lineNumber + 1}: ${line.slice(0, 120)}`)
    const [, file, screen, page, width, height, tweakCell] = match
    if (!file.endsWith('.dc.html')) fail(`README row ${file} is not a .dc.html file`)
    if (!['Máy tính', 'Điện thoại', 'UI Kit'].includes(page)) fail(`README row ${file} has unknown page ${JSON.stringify(page)}`)
    rows.push({
      file,
      name: file.slice(0, -'.dc.html'.length),
      screen,
      page,
      width: Number(width),
      height: Number(height),
      tweakCell,
      readmeTruncated: tweakCell.includes('…'),
      line: lineNumber + 1,
    })
  }
  if (rows.length !== 42) fail(`README must list 42 artboards, parsed ${rows.length}`)
  return { version: Number(versionMatch[1]), date: versionMatch[2], rows }
}

function pageLabel(page) {
  if (page === 'desktop') return 'Máy tính'
  if (page === 'mobile') return 'Điện thoại'
  if (page === 'uk') return 'UI Kit'
  return page
}

function valueAllowedValues(prop) {
  return prop.editor === 'boolean' ? [false, true] : prop.options
}

function enumBooleanCount(prop) {
  if (prop.editor === 'boolean') return 2
  if (prop.editor === 'enum') return prop.options.length
  return 0
}

function validateContext(name, propKey, context, board) {
  if (!context || typeof context !== 'object' || Array.isArray(context)) {
    fail(`${name}.${propKey}: context must be an object`)
  }
  const resolved = {}
  for (const [key, value] of Object.entries(context)) {
    if (key === propKey) fail(`${name}.${propKey}: context must not set the row property`)
    if (!Object.prototype.hasOwnProperty.call(board.props, key) || key === '$preview') {
      fail(`${name}.${propKey}: context references unknown data-props property ${key}`)
    }
    const allowed = valueAllowedValues(board.props[key])
    if (!Array.isArray(allowed) || !allowed.some((candidate) => Object.is(candidate, value))) {
      fail(`${name}.${propKey}: context ${key}=${JSON.stringify(value)} is not an allowed value`)
    }
    resolved[key] = value
  }
  return resolved
}

function genericExpected(screen, propKey, prop, value) {
  return `${screen} renders “${prop.section}” with ${propKey}=${String(value)} (footer default ${String(prop.default)}); compare the reference board at the same tweak with the seeded app state.`
}

function buildRows({ artboards, specs = SCREEN_SPECS }) {
  const rows = []
  const seen = new Set()
  const propertyInventory = []

  for (const [name, spec] of Object.entries(specs)) {
    const board = artboards[name]
    if (!board) fail(`Included artboard ${name} was not parsed`)
    const actualProps = Object.keys(board.props).filter((key) => key !== '$preview')
    const configuredProps = Object.keys(spec.props ?? {})
    const missing = actualProps.filter((key) => !configuredProps.includes(key))
    const extra = configuredProps.filter((key) => !actualProps.includes(key))
    if (missing.length || extra.length) {
      fail(`${name}: property inventory drift; update SCREEN_SPECS`, { missing, extra })
    }
    propertyInventory.push({ artboard: name, properties: actualProps.length, rows: 0 })

    const rowProps = Object.fromEntries(
      actualProps.map((key) => {
        const def = board.props[key]
        return [
          key,
          {
            editor: def.editor,
            ...(def.options ? { options: def.options } : {}),
            default: def.default,
            section: def.section,
          },
        ]
      }),
    )
    const rowDefaults = Object.fromEntries(actualProps.map((key) => [key, board.props[key].default]))

    for (const propKey of actualProps) {
      const prop = board.props[propKey]
      const editor = validateProp(name, propKey, prop)
      if (editor !== 'enum' && editor !== 'boolean') {
        fail(`${name}.${propKey}: ${editor} props cannot be tweak rows; add a probe instead`)
      }
      const propSpec = spec.props[propKey] ?? {}
      const values = valueAllowedValues(prop)
      const baseContext = validateContext(name, propKey, propSpec.context ?? {}, board)
      for (const key of Object.keys(propSpec.contextByValue ?? {})) {
        if (!values.map(String).includes(key)) {
          fail(`${name}.${propKey}: contextByValue key ${key} is not an allowed value`)
        }
      }
      const section = prop.section
      for (const value of values) {
        const context = {
          ...baseContext,
          ...validateContext(name, propKey, propSpec.contextByValue?.[String(value)] ?? {}, board),
        }
        const note = propSpec.valueNotes?.[String(value)] ?? {}
        const ownerPR = note.ownerPR ?? propSpec.ownerPRByValue?.[String(value)] ?? propSpec.ownerPR ?? spec.ownerPR
        if (!OWNER_PRS.has(ownerPR)) fail(`${name}.${propKey}.${String(value)}: invalid ownerPR ${ownerPR}`)
        const rawRoute =
          PROP_ROUTE_VALUES[`${name}.${propKey}.${String(value)}`] ??
          PROP_ROUTES[`${name}.${propKey}`] ??
          note.route ??
          propSpec.routeByValue?.[String(value)] ??
          propSpec.route ??
          spec.routeByStep?.[String(value)] ??
          spec.routeByScreen?.[String(value)] ??
          spec.routeByState?.[String(value)] ??
          spec.routeByDialog?.[String(value)] ??
          spec.route
        if (!rawRoute || typeof rawRoute !== 'string') fail(`${name}.${propKey}.${String(value)}: missing route`)
        const route = fillRoute(rawRoute, propKey, value)
        const prerequisiteParts = [
          QA_PREREQ,
          spec.prerequisite,
          propSpec.prerequisite,
          note.prerequisite,
        ].filter(Boolean)
        const seed = note.seed ?? propSpec.seed ?? spec.seed ?? QA_SEED
        const expected = note.expected ?? propSpec.expected ?? spec.expected ?? genericExpected(name, propKey, prop, value)
        const rowBaseline = {
          mapping: note.baseline?.mapping ?? propSpec.baseline?.mapping ?? spec.baseline.mapping,
          knownGap: note.baseline?.knownGap ?? propSpec.baseline?.knownGap ?? spec.baseline.knownGap ?? null,
          exclusion: note.baseline?.exclusion ?? propSpec.baseline?.exclusion ?? spec.baseline.exclusion ?? null,
        }
        const id = `${name}.${propKey}.${String(value)}`
        if (seen.has(id)) fail(`Duplicate row id ${id}`)
        seen.add(id)
        rows.push({
          id,
          kind: 'artboard-tweak',
          artboard: name,
          board: name,
          page: spec.page,
          file: `${ARTBOARDS_REL}/${name}.dc.html`,
          property: propKey,
          dcProp: `${name}.${propKey}`,
          section,
          valueType: editor,
          value,
          allowedValues: values,
          defaultValue: prop.default,
          isDefault: Object.is(prop.default, value),
          prerequisite: prerequisiteParts.join(' '),
          route,
          appRoute: route,
          seed,
          expectedVisible: expected,
          observedVisible: null,
          ownerPR,
          owner: ownerPR,
          props: rowProps,
          defaults: rowDefaults,
          context,
          patch: { [propKey]: value },
          captureTarget: Object.hasOwn(note, 'captureTarget') ? note.captureTarget : propSpec.captureTarget ?? null,
          baseline: rowBaseline,
          evidence: PENDING_EVIDENCE(),
          status: 'pending',
          blockedReason: null,
        })
      }
      const entry = propertyInventory[propertyInventory.length - 1]
      entry.rows += values.length
    }
  }
  return { rows, propertyInventory }
}

function validateProbes(probes, artboards) {
  const seen = new Set()
  for (const probe of probes) {
    if (seen.has(probe.id)) fail(`Duplicate probe id ${probe.id}`)
    seen.add(probe.id)
    if (!['numeric', 'numeric-pair', 'text'].includes(probe.kind)) fail(`${probe.id}: invalid probe kind`)
    if (!artboards[probe.artboard]) fail(`${probe.id}: unknown artboard ${probe.artboard}`)
    if (!OWNER_PRS.has(probe.ownerPR)) fail(`${probe.id}: invalid ownerPR`)
    if (!Array.isArray(probe.probes) || probe.probes.length === 0) fail(`${probe.id}: probes must be non-empty`)
    if (new Set(probe.probes.map((value) => JSON.stringify(value))).size !== probe.probes.length) {
      fail(`${probe.id}: duplicate probe values`)
    }
    if (!probe.route || typeof probe.route !== 'string') fail(`${probe.id}: missing route`)
    if (!Array.isArray(probe.dcPropRefs) || probe.dcPropRefs.length === 0) fail(`${probe.id}: missing dcPropRefs`)
    for (const ref of probe.dcPropRefs) {
      const [artboard, propKey] = ref.split('.')
      const board = artboards[artboard]
      if (!board) fail(`${probe.id}: dcPropRefs references unknown artboard ${artboard}`)
      if (!Object.prototype.hasOwnProperty.call(board.props, propKey) || propKey === '$preview') {
        fail(`${probe.id}: dcPropRefs references unknown data-props property ${ref}`)
      }
    }
    if (!probe.expectedVisible) fail(`${probe.id}: missing expectedVisible`)
    if (probe.observedVisible !== null) fail(`${probe.id}: observedVisible must start null`)
    if (!probe.baseline || !probe.baseline.mapping) fail(`${probe.id}: missing baseline mapping`)
    if (probe.status !== 'pending') fail(`${probe.id}: probes must start pending`)
    for (const key of ['reference', 'app', 'comparison', 'controlLedger', 'motionTrace']) {
      if (probe.evidence?.[key] !== null) fail(`${probe.id}: evidence.${key} must start null`)
    }
  }
}

// ---------------------------------------------------------------------------
// Build
// ---------------------------------------------------------------------------

function buildRuntimeProtocol() {
  const rel = `${ARTBOARDS_REL}/support.js`
  const text = read(rel)
  const checks = {
    hasHostRuntime: text.includes('window.__dcHostRuntime = true'),
    hasSetProps: text.includes('__dcSetProps:(c,_)=>n.setProps(c,_)'),
    hasRootName: text.includes('__dcRootName:()=>e'),
    hasBoot: text.includes('__dcBoot:()=>'),
    hasRegistry: text.includes('__dcRegistry:n.registry.entries'),
    hasBootedMessage: text.includes('type:"__dc_booted"'),
    hasRenderErrorsMessage: text.includes('type:"__dc_render_errors"'),
    hasDesignModeMessage: text.includes('type:"__dc_design_mode"'),
    hasThemeMessage: text.includes('d==="__dc_theme"'),
    hasProbeMessage: text.includes('d!=="__dc_probe"'),
    hasDataPropsParser: text.includes('script[data-dc-script]') && text.includes('data-props'),
    hasPropOverrides: text.includes('propOverrides'),
  }
  if (!Object.values(checks).every(Boolean)) {
    fail('support.js runtime protocol changed; update the PR0 protocol before parent captures', { checks })
  }
  return {
    source: rel,
    sourceHash: sha256(text),
    supported: true,
    runtimeVerified: false,
    verificationNote:
      'Static source inspection only. The API tokens exist in support.js; the parent capture harness must confirm boot/rootName/setProps at runtime before using this protocol as evidence.',
    checks,
    note:
      'Verified against the published support.js in this repo. Use this protocol only; do not invent props, add query params, or edit HTML/DOM to force a tweak.',
    propsApi: {
      sameOriginRequired: true,
      rootName: "iframe.contentWindow.__dcRootName() -> data-props basename with .dc.html/.html removed (e.g. 'Main', 'MReview')",
      rootNameSource: 'Set by the runtime from location.pathname via Pt()/It(); not necessarily the full file path.',
      setProps: 'iframe.contentWindow.__dcSetProps(rootName, propsObject)',
      setPropsSemantics:
        'sets registry[rootName].propOverrides = {...propsObject} (replaces the previous override object), then re-renders. The component receives {...data-props defaults, ...overrides}; unknown props are not validated and must not be used.',
      propsArgument:
        'Object of only keys from the board data-props footer excluding $preview; enum values must be one of options, boolean true|false. No invented props.',
      validateBeforeSet:
        'Assert each key exists in registry[rootName].propsMeta, each enum value is in that prop options, and each boolean is true|false; never pass $preview or an unknown key.',
      readMeta: 'iframe.contentWindow.__dcRegistry[rootName].propsMeta or the __dc_booted message .propsMeta (same footer metadata, $preview separate).',
      defaults: 'Use data-props[property].default; the runtime merges defaults under overrides.',
      boot: 'Runtime auto-boots on DOMContentLoaded. __dcBoot() re-parses the current document and is not a tweak API.',
      lowLevelUpdate:
        "iframe.contentWindow.__dcUpdate(rootName, 'props', JSON.stringify(propsObject), false, ...) exists, but prefer __dcSetProps for tweaks.",
      noDomMutation: 'Do not edit data-props, DOM, or HTML to apply a tweak.',
    },
    postMessage: {
      iframeToParent: [
        {
          type: '__dc_booted',
          when: 'iframe booted and parent !== self',
          payload: {
            type: '__dc_booted',
            rootName: 'string',
            mounted: 'boolean',
            propsMeta: 'object|null (data-props minus $preview)',
            preview: 'object|null',
            tellsErrors: true,
          },
        },
        {
          type: '__dc_render_errors',
          when: 'runtime render error batch',
          payload: { type: '__dc_render_errors', errors: 'array' },
        },
        {
          type: '__dc_design_mode',
          when: 'design mode changes or a __dc_probe is received',
          payload: { type: '__dc_design_mode', mode: 'string' },
        },
      ],
      parentToIframe: [
        {
          type: '__dc_theme',
          payload: { type: '__dc_theme', theme: 'light|dark' },
        },
        {
          type: '__dc_probe',
          payload: { type: '__dc_probe' },
          response: '__dc_design_mode',
        },
      ],
      propsAreNotPostMessage:
        'There is no postMessage type in support.js that sets props. Use __dcSetProps on the same-origin iframe window.',
    },
    usage: {
      load: 'Serve apps/viderse/docs/design/v1 statically and load artboards/<Board>.dc.html in a same-origin iframe/window.',
      wait: 'Wait for the __dc_booted message (rootName/mounted/propsMeta) or poll __dcRootName() until it is not the fallback Root and the registry entry exists.',
      oneTweak: "contentWindow.__dcSetProps(contentWindow.__dcRootName(), { stage: 'review' })",
      multipleTweaks: 'Pass all desired overrides in one object; __dcSetProps replaces the previous override object.',
      fullDefaultsPatch: 'Row capture: contentWindow.__dcSetProps(rootName, { ...row.defaults, ...row.context, ...row.patch }); reset with contentWindow.__dcSetProps(rootName, {}).',
      reset: 'contentWindow.__dcSetProps(rootName, {})',
      readBack: 'contentWindow.__dcRegistry[rootName].propOverrides',
      afterSet: 'Wait for the React commit/next paint and images/fonts; assert propOverrides equals the requested object before capture.',
      fonts: 'await contentWindow.document.fonts.ready before the screenshot; wait for images/finite animations per verification.md.',
      noQuery: 'There is no ?prop=value support; use __dcSetProps.',
    },
    query: {
      theme: '?theme=dark is parsed from location.search; no tweak value is parsed from the URL by support.js.',
    },
  }
}

function buildManifest() {
  const readmeText = read(README_REL)
  const readme = parseReadme(readmeText)
  const diskFiles = readdirSync(ARTBOARDS_DIR).filter((file) => file.endsWith('.dc.html')).sort()
  const readmeFiles = readme.rows.map((row) => row.file).sort()
  if (JSON.stringify(diskFiles) !== JSON.stringify(readmeFiles)) {
    fail('Artboard directory and README table disagree', {
      onlyOnDisk: diskFiles.filter((file) => !readmeFiles.includes(file)),
      onlyInReadme: readmeFiles.filter((file) => !diskFiles.includes(file)),
    })
  }

  const artboards = {}
  const coverage = []
  const previewFrameMismatches = []
  for (const row of readme.rows) {
    const rel = `${ARTBOARDS_REL}/${row.file}`
    const html = read(rel)
    const parsed = extractDataProps(html, rel)
    const preview = validatePreview(row.name, parsed.props)
    const propKeys = Object.keys(parsed.props).filter((key) => key !== '$preview')
    for (const key of propKeys) validateProp(row.name, key, parsed.props[key])
    artboards[row.name] = {
      ...row,
      rel,
      props: parsed.props,
      rawPropsSha256: parsed.sha256,
      htmlSha256: sha256(html),
      preview,
    }
    if (preview.width !== row.width || preview.height !== row.height) {
      previewFrameMismatches.push({
        artboard: row.name,
        readme: `${row.width}×${row.height}`,
        footer: `${preview.width}×${preview.height}`,
        included: Boolean(SCREEN_SPECS[row.name]),
        resolution: SCREEN_SPECS[row.name]
          ? 'FAIL: included artboard preview must match README'
          : 'recorded; board is excluded from V36–V40 state rows; footer wins per source authority',
      })
    }
  }

  for (const [name, spec] of Object.entries(SCREEN_SPECS)) {
    const board = artboards[name]
    if (!board) fail(`SCREEN_SPECS references missing board ${name}`)
    if (board.page !== pageLabel(spec.page)) {
      fail(`${name}: page mismatch; footer README says ${board.page}, spec says ${pageLabel(spec.page)}`)
    }
  }

  const { rows, propertyInventory } = buildRows({ artboards })
  const probes = PROBES.map((probe) => ({
    ...probe,
    board: probe.artboard,
    appRoute: probe.route,
    owner: probe.ownerPR,
  }))
  validateProbes(probes, artboards)

  for (const mismatch of previewFrameMismatches) {
    if (mismatch.included) fail(`Included artboard ${mismatch.artboard} preview does not match README`, mismatch)
  }

  const enumRows = rows.filter((row) => row.valueType === 'enum')
  const booleanRows = rows.filter((row) => row.valueType === 'boolean')
  const expectedValues = Object.entries(SCREEN_SPECS).reduce((sum, [name, spec]) => {
    const board = artboards[name]
    return (
      sum +
      Object.keys(spec.props ?? {}).reduce((inner, propKey) => inner + enumBooleanCount(board.props[propKey]), 0)
    )
  }, 0)
  if (rows.length !== expectedValues) fail(`Row count mismatch: ${rows.length} !== ${expectedValues}`)
  if (new Set(rows.map((row) => row.id)).size !== rows.length) fail('Duplicate row ids after build')

  for (const row of rows) {
    if (row.status !== 'pending') fail(`${row.id}: rows must start pending`)
    if (row.captureTarget !== null && typeof row.captureTarget !== 'string') fail(`${row.id}: captureTarget must be a source selector or null`)
    for (const key of ['reference', 'app', 'comparison', 'controlLedger', 'motionTrace']) {
      if (row.evidence[key] !== null) fail(`${row.id}: evidence.${key} must start null`)
    }
    if (row.observedVisible !== null) fail(`${row.id}: observedVisible must start null`)
    const board = artboards[row.artboard]
    const actualProps = Object.keys(board.props).filter((key) => key !== '$preview')
    if (Object.keys(row.defaults).sort().join('\u0000') !== actualProps.slice().sort().join('\u0000')) {
      fail(`${row.id}: defaults must cover every data-prop on ${row.artboard}`)
    }
    if (Object.keys(row.props).sort().join('\u0000') !== actualProps.slice().sort().join('\u0000')) {
      fail(`${row.id}: props must cover every data-prop on ${row.artboard}`)
    }
    for (const key of Object.keys(row.patch)) {
      if (!actualProps.includes(key)) fail(`${row.id}: patch key ${key} is not a data-prop`)
      if (!Object.is(row.patch[key], row.value)) fail(`${row.id}: patch value must equal row value`)
    }
    if (JSON.stringify(row.patch) !== JSON.stringify({ [row.property]: row.value })) {
      fail(`${row.id}: patch must contain exactly the row property`)
    }
    if (!Object.is(row.defaults[row.property], row.defaultValue)) fail(`${row.id}: defaults[property] must equal defaultValue`)
    for (const key of actualProps) {
      const def = board.props[key]
      if (row.props[key].editor !== def.editor) fail(`${row.id}: props.${key}.editor mismatch`)
      if (!Object.is(row.props[key].default, def.default)) fail(`${row.id}: props.${key}.default mismatch`)
      if (JSON.stringify(row.props[key].options) !== JSON.stringify(def.options)) fail(`${row.id}: props.${key}.options mismatch`)
      if (row.props[key].section !== def.section) fail(`${row.id}: props.${key}.section mismatch`)
    }
    if (!Object.is(row.patch[row.property], row.value)) fail(`${row.id}: patch does not reproduce value`)
  }

  // Coverage for every README board, including excluded/reference-only boards.
  for (const row of readme.rows) {
    const name = row.name
    const included = Boolean(SCREEN_SPECS[name])
    const reference = Boolean(REFERENCE_BOARDS[name])
    const excluded = Boolean(EXCLUDED_BOARDS[name])
    if ([included, reference, excluded].filter(Boolean).length !== 1) {
      fail(`${name}: must be exactly one of stateful/reference/excluded`)
    }
    const board = artboards[name]
    const actualProps = Object.keys(board.props).filter((key) => key !== '$preview')
    const valueCount = actualProps.reduce((sum, key) => sum + enumBooleanCount(board.props[key]), 0)
    coverage.push({
      artboard: name,
      board: name,
      file: `${ARTBOARDS_REL}/${row.file}`,
      page: row.page,
      frame: `${row.width}×${row.height}`,
      footerPreview: `${board.preview.width}×${board.preview.height}`,
      included: included || reference,
      role: included ? 'stateful' : reference ? 'reference-only' : 'excluded',
      ownerPR: included ? SCREEN_SPECS[name].ownerPR : reference ? 'PR6' : null,
      owner: included ? SCREEN_SPECS[name].ownerPR : reference ? 'PR6' : null,
      phases: included ? SCREEN_SPECS[name].phases : reference ? REFERENCE_BOARDS[name] : [],
      properties: actualProps.length,
      enumBooleanValues: valueCount,
      rows: included ? rows.filter((entry) => entry.artboard === name).length : 0,
      reason: included
        ? `V36–V40 stateful board; owner ${SCREEN_SPECS[name].ownerPR}; ${SCREEN_SPECS[name].phases.join(', ')}.`
        : reference
          ? `UI Kit reference for ${REFERENCE_BOARDS[name].join(', ')}; only $preview, no tweak rows.`
          : EXCLUDED_BOARDS[name].reason,
      readmeTruncated: row.readmeTruncated,
      readmeLine: row.line,
      rawPropsSha256: board.rawPropsSha256,
      htmlSha256: board.htmlSha256,
      sourceRefs: included
        ? ['artboards-readme', 'workflow', 'ia', 'dev', ...SCREEN_SPECS[name].phases.map((phase) => ({
            V36: 'phase03', V37: 'phase04', V38: 'phase05', V39: 'phase06', V40: 'phase07',
          }[phase])).filter(Boolean)]
        : reference
          ? ['artboards-readme', 'ia', 'components']
          : EXCLUDED_BOARDS[name].sourceRefs,
    })
  }

  const knownSourceRefs = new Set(Object.keys(PHASE_FILES))
  const referenced = new Set()
  for (const entry of [
    ...EXCLUSIONS,
    ...SCREENS_WITHOUT_ARTBOARD,
    ...BASELINE_COMPONENTS,
    ...SOURCE_CONFLICTS,
    ...coverage,
    ...probes,
  ]) {
    for (const ref of entry.sourceRefs ?? []) referenced.add(ref)
  }
  for (const ref of referenced) {
    if (!knownSourceRefs.has(ref)) fail(`Unknown sourceRef ${ref}; add it to PHASE_FILES`)
  }

  const validation = {
    ok: true,
    scope: 'PR0 structural/source validation only; target UI acceptance is not evaluated here.',
    targetAcceptanceEvaluated: false,
    readmeVersion: readme.version,
    readmeDate: readme.date,
    artboardsInReadme: readme.rows.length,
    artboardsOnDisk: diskFiles.length,
    parsedFooters: Object.keys(artboards).length,
    exactlyOneDataPropsPerFile: true,
    previewFramesChecked: Object.keys(artboards).length,
    previewFrameMismatches,
    includedStatefulBoards: Object.keys(SCREEN_SPECS).length,
    includedReferenceBoards: Object.keys(REFERENCE_BOARDS).length,
    excludedBoards: Object.keys(EXCLUDED_BOARDS).length,
    propertiesIncludedBoards: propertyInventory.reduce((sum, entry) => sum + entry.properties, 0),
    enumRows: enumRows.length,
    booleanRows: booleanRows.length,
    totalRows: rows.length,
    rowsWithContext: rows.filter((row) => Object.keys(row.context).length > 0).length,
    probes: probes.length,
    duplicateRowIds: [],
    duplicateProbeIds: [],
    unmappedProperties: [],
    allRowsPending: rows.every((row) => row.status === 'pending'),
    allEvidenceNull: rows.every((row) => Object.values(row.evidence).every((value) => value === null)),
    allProbesPending: probes.every((probe) => probe.status === 'pending'),
    allProbeEvidenceNull: probes.every((probe) => Object.values(probe.evidence).every((value) => value === null)),
    allObservedNull: rows.every((row) => row.observedVisible === null) && probes.every((probe) => probe.observedVisible === null),
    notes: [
      'Rows are an inventory only. No app/reference screenshot, comparison, click ledger, motion trace, or seed receipt is asserted.',
      'A row being pending is the expected PR0 state; it is not a pass and not a failure of the design.',
      'Footer data-props wins over README/kit when they differ; README truncation is recorded but never used to drop footer metadata.',
    ],
  }

  if (!validation.allRowsPending || !validation.allEvidenceNull || !validation.allProbesPending || !validation.allProbeEvidenceNull || !validation.allObservedNull) {
    fail('Validation state assertion failed', validation)
  }

  const sourceHashes = {}
  for (const rel of [
    README_REL,
    PHASE_FILES.workflow,
    PHASE_FILES.router,
    `${ARTBOARDS_REL}/support.js`,
    ...Object.values(PHASE_FILES).filter((rel) => rel.endsWith('.md')),
  ]) {
    sourceHashes[rel] = sha256(read(rel))
  }

  return {
    schemaVersion: 1,
    artifact: 'viderse-workflow-artboard-state-matrix',
    deterministic: true,
    generatedBy: 'tests/viderse-workflow/artboard-state-manifest.mjs',
    generatedAt: null,
    note:
      'PR0 inventory generated from the real artboard data-props footers. Every evidence path is null and every state row starts pending; do not treat this file as proof of a captured app state.',
    sourceRoot: '.',
    scope: {
      rounds: ['40', '41', '42', '43', '44'],
      desktopViewport: '1440x900',
      mobileViewport: '390x844',
      phases: ['V36', 'V37', 'V38', 'V39', 'V40'],
      ownerPRs: {
        PR2: 'V36',
        PR3: 'V37',
        PR4: 'V38',
        PR5: 'V39',
        PR6: 'V40',
      },
      authorityOrder: [
        'apps/viderse/docs/design/v1/artboards/Workflow.dc.html',
        '01-ia-flows.md §0.3 / §6.2 Đợt44 / W3–W13',
        '08-dev-prompt.md V36–V40',
        'individual artboard footer / README / kit',
      ],
    },
    sourceRefs: PHASE_FILES,
    sourceHashes,
    runtimeProtocol: buildRuntimeProtocol(),
    rowSemantics: {
      expectedVisible: 'Target acceptance for the owning PR; not an observed app state.',
      baseline: 'Current repository/artboard baseline at PR0. knownGap is an existing mismatch, not a target pass/fail.',
      status: 'pending = PR0 has not captured reference/app/comparison/control-ledger/motion evidence yet; it is neither pass nor fail.',
      exclusion: 'An exclusion object names a top-level out-of-scope item; it does not mark the row as passed or excluded.',
      props: 'Full data-props metadata for the row artboard (excluding $preview), for validating __dcSetProps input.',
      defaults: 'Default values for every data-prop on this artboard. Capture should call __dcSetProps(rootName, {...row.defaults, ...row.context, ...row.patch}).',
      context:
        'Parent-view/context values required for the tweak to be visible (tab, section, screen, step, dialog, panel). Merged between defaults and patch, so the row property in patch always wins. Empty object when the footer default already exposes the tweak.',
      patch: 'Exactly {property: value} for this row; merging it over defaults+context sets only this state while retaining the other defaults.',
    },
    evidencePolicy: {
      statuses: {
        pending: 'Not captured yet; expected in PR0 for every row/probe.',
        excluded: 'Explicitly outside V36–V40; must not be implemented or counted as pass.',
        blocked: 'A real prerequisite is missing; record the exact blocker instead of faking a result.',
      },
      pathsAreNullUntilCaptured: true,
      screenshotRule:
        'Each row needs reference/app/comparison images at 1440×900 and 390×844 (or a sourced not-applicable for the side without a design) under reports/pr-NN/{reference,app,comparison}/ with viewport, DPR, font status, revision, seed, and state in JSON.',
      controlOracleRule:
        'Every visible button/link/[role=tab|radio|switch], including opened menus/sheets, needs an accessible role/name, enabled/disabled reason, expected and observed effect, and API/readback. A click count or toast alone is not a pass.',
      motionRule: 'Capture opening→visible→closing→unmounted, rapid toggle, route change, focus return, scroll restore, normal and reduced motion; do not disable animation in the motion pass.',
      seedRule:
        'Use authored QA fixtures only as labelled inputs to real constructors/API/DB. They are not proof of real AI drawing/fallback. Never mock app state or DOM for screenshots.',
      isolationRule:
        'Before any mutation: separate HOME/.viderse/auth.json, app DB, identity DB, media/blob store, browser storage, and approved QA provider credentials. Missing any boundary blocks logout/draw/write smoke.',
    },
    exclusions: EXCLUSIONS,
    screensWithoutArtboard: SCREENS_WITHOUT_ARTBOARD,
    baselineComponents: BASELINE_COMPONENTS,
    sourceConflicts: SOURCE_CONFLICTS,
    coverage,
    rows,
    probes,
    validation,
  }
}

function buildProductionManifest() {
  const legacy = buildManifest()
  const groups = [
    { id: '01', title: 'Bàn dựng', boards: ['Main', 'MMain', 'Activity', 'Editor'] },
    { id: '02', title: 'Duyệt', boards: ['Review', 'MReview'] },
    { id: '03', title: 'Kênh', boards: ['Channel', 'MChannel'] },
    { id: '04', title: 'Thư viện + Template', boards: ['Library', 'Template', 'MLibrary', 'MTemplate'] },
    { id: '05', title: 'Làm video + Khám phá + Hàng chờ', boards: ['NewVideo', 'MNew', 'Explore', 'MExplore', 'MCompVideo', 'Queue', 'MQueue'] },
    { id: '06', title: 'Cài đặt', boards: ['Settings', 'MSettings'] },
    { id: '07', title: 'Đăng nhập/Tạo kênh/Hệ thống', boards: ['Auth', 'MAuth', 'Onboard', 'CreateChannel', 'MCreate', 'System', 'MSystem', 'Dialogs'] },
  ]
  const productBoards = groups.flatMap(group => group.boards)
  if (productBoards.length !== 29 || new Set(productBoards).size !== 29) fail('Production groups must own exactly 29 distinct product boards')

  const authFile = `${ARTBOARDS_REL}/Auth.dc.html`
  const authProps = extractDataProps(read(authFile), authFile).props
  const mobileAuth = SCREEN_SPECS.MAuth
  const authSpec = {
    ...mobileAuth,
    page: 'desktop',
    expected: 'Desktop Auth matches the real footer state; identity/session behavior and permissions stay unchanged.',
    props: {
      screen: { ...mobileAuth.props.screen, valueNotes: { ...mobileAuth.props.screen.valueNotes, menu: { expected: 'Account menu over the current app, not the account page.' } } },
      kicked: mobileAuth.props.kicked,
      err: mobileAuth.props.err,
      role: mobileAuth.props.role,
      other: mobileAuth.props.other,
      app: { ...mobileAuth.props.app, context: { screen: 'menu' } },
      step: mobileAuth.props.step,
      down: mobileAuth.props.down,
      pwerr: mobileAuth.props.pwerr,
      fail: mobileAuth.props.fail,
      busy: mobileAuth.props.busy,
    },
  }
  const auth = buildRows({ artboards: { Auth: { props: authProps } }, specs: { Auth: authSpec } })
  const routeFor = (row, state) => {
    if (row.board !== 'Auth') return row.appRoute
    return state.screen === 'account' ? '#/account' : state.screen === 'menu' ? '#/ + avatar menu' : 'pre-session auth gate (no hash route)'
  }
  const productionRow = row => {
    const state = { ...row.defaults, ...row.context, ...row.patch }
    let group = groups.find(entry => entry.boards.includes(row.board))?.id
    if (!group) fail(`No production group for ${row.id}`)
    const dialog = state.dialog ?? state.state ?? state.view
    if (row.board === 'Dialogs' || row.board === 'MSystem') {
      if (['delVideo', 'delete-video', 'restart', 'clip', 'clip-picker', 'shortcuts', 'channel-switch'].includes(dialog)) group = '01'
      else if (['redrawAll', 'redraw-all'].includes(dialog)) group = '02'
      else if (['delChannel', 'delete-channel', 'unlink'].includes(dialog)) group = '03'
    } else if (row.board === 'Activity' && state.view === 'settings') group = '06'
    const firstRun = row.board === 'Onboard' || (['Auth', 'MAuth'].includes(row.board) && state.screen === 'setup')
    const appRoute = routeFor(row, state)
    return {
      id: row.id,
      kind: row.kind,
      group,
      board: row.board,
      artboard: row.artboard,
      file: row.file,
      page: row.page,
      viewport: row.page === 'mobile' ? { width: 390, height: 844 } : { width: 1440, height: 900 },
      property: row.property,
      value: row.value,
      valueType: row.valueType,
      allowedValues: row.allowedValues,
      defaultValue: row.defaultValue,
      isDefault: row.isDefault,
      defaults: row.defaults,
      props: row.props,
      context: row.context,
      patch: row.patch,
      appRoute,
      captureTarget: row.captureTarget,
      expectedVisible: row.expectedVisible,
      recipe: {
        entity: null,
        steps: [
          'Use the authorized real account and supported UI/API; identify an approved acceptance entity before preparing this state.',
          'Before mounting the desktop shell, GET /api/setup/state; incomplete tool setup requires install approval before navigation.',
          `Open ${appRoute}; reach ${row.property}=${JSON.stringify(row.value)} with context ${JSON.stringify(row.context)} through real operations, never DOM/store injection or a QA seed on production.`,
          'Record each preparation action, its blast radius, permission, cost and authoritative readback; close automation lifetime after the final action.',
        ],
        prerequisite: row.expectedVisible,
        mutation: 'approval-required-before-state-preparation',
        firstRunExceptionEligible: firstRun ? 94 : null,
      },
      observation: {
        environment: null, url: null, capturedAt: null, deployedCommit: null,
        sourceCommit: null, fontReady: null, dpr: null, realEntity: null, observedVisible: null,
        controls: null, unresponsive: null, unverified: null, differences: null,
      },
      evidence: PENDING_EVIDENCE(),
      status: 'pending',
      blockedReason: null,
      exception: null,
    }
  }
  const rows = [...legacy.rows, ...auth.rows].map(productionRow)
  const coverage = productBoards.map(board => {
    const entries = rows.filter(row => row.board === board)
    if (!entries.length) fail(`Production board ${board} has no footer rows`)
    const reference = legacy.coverage.find(entry => entry.board === board)
    return {
      board, groups: [...new Set(entries.map(row => row.group))],
      properties: Object.keys(entries[0].props).length, rows: entries.length,
      rawPropsSha256: reference.rawPropsSha256, htmlSha256: reference.htmlSha256,
    }
  })
  const probes = legacy.probes.map(probe => ({
    ...probe,
    group: groups.find(group => group.boards.includes(probe.board))?.id ?? fail(`No production group for probe ${probe.id}`),
    seed: null,
    prerequisite: 'Use supported operations on authorized real entities; no QA fixture/DB seeding on production.',
    environment: null,
    deployedCommit: null,
    observedVisible: null,
    evidence: PENDING_EVIDENCE(),
    status: 'pending',
  }))
  const lifecycleProbes = [
    { key: 'indicator', scenarios: ['fonts-loaded', 'resize', 'label-change'], expected: 'For every Segmented/Tabs instance, each indicator edge is within2px of its target after fonts settle. Record measured edges and any non-applicable control explicitly.' },
    { key: 'overlay', scenarios: ['open', 'dismiss', 'cancel', 'confirm'], expected: 'Every popover/sheet stays within the viewport, does not push page content, has reachable cancel, and preserves focus/scroll correctly.' },
    { key: 'responsive', scenarios: ['360x844', '430x844', '767x900', '768x900', 'long-scroll', 'safe-area', 'keyboard'], expected: 'No horizontal overflow, obscured CTA, stuck dock or scroll lock; touch targets≥44px. Physical iPhone safe-area/keyboard evidence remains required by#93.' },
    { key: 'motion', scenarios: ['normal', 'reduced', 'rapid-reopen', 'route-change', 'focus-restore', 'scroll-restore'], expected: 'Exercise complete opening→closing transitions and interrupted exits for every applicable overlay/control. No stale follow-up action, duplicate active overlay or hidden focusable control.' },
  ]
  for (const group of groups) {
    for (const probe of lifecycleProbes) {
      probes.push({
        id: `production.group-${group.id}.${probe.key}`,
        kind: 'geometry-lifecycle', group: group.id, boards: group.boards,
        scope: 'Every applicable row and control of this group; examples do not replace full coverage.',
        scenarios: probe.scenarios, expectedVisible: probe.expected,
        prerequisite: 'Authorized real data/actions; do not mutate production DOM or inject fixture state.',
        environment: null, deployedCommit: null, observedVisible: null, measurements: null,
        evidence: PENDING_EVIDENCE(), status: 'pending',
      })
    }
  }
  const acceptanceContract = 'plans/261106-1303-viderse-production-artboard-parity/acceptance-contract.md'
  return {
    schemaVersion: 2,
    artifact: 'viderse-production-artboard-state-matrix',
    generatedBy: 'tests/viderse-workflow/artboard-state-manifest.mjs --production',
    deterministic: true,
    sourceCommit: null,
    trackedContentVerified: false,
    note: 'Pending inventory, not production/device proof. Record the exact source/build/deployed hashes at capture; no commit or artifact has been verified by inventory generation.',
    productionOrigin: 'https://vid.aarverse.studio',
    acceptanceContract,
    groups,
    sourceHashes: { ...legacy.sourceHashes, [acceptanceContract]: sha256(read(acceptanceContract)) },
    runtimeProtocol: legacy.runtimeProtocol,
    evidencePolicy: {
      statesStartPending: true,
      productionDOMOrNetworkMocksAllowed: false,
      actualControlsRequired: true,
      zeroUnresponsiveAndUnverifiedControls: true,
      sliderTolerancePx: 2,
      firstRunException: 'Only approved #94 first-run rows may use the isolated real instance of the same release. Eligibility is not verification; record verified-exception-94 with environment/hash/images, never production-pass.',
      secrets: 'Cookies/storage state stay outside the repository with mode0600. Never include credentials or raw traces in evidence.',
      lifecycle: 'After create/link/toggle, verify terminal automation configuration, stop future acceptance-channel admission when authorization ends, and reconcile queued/running work and receipts.',
      sharedDialogs: 'Dialogs and Activity instances belong to their parent action; one tested instance cannot pass the whole board.',
    },
    coverage,
    rows,
    probes,
    validation: {
      ok: true,
      targetAcceptanceEvaluated: false,
      totalRows: rows.length,
      probes: probes.length,
      includedStatefulBoards: coverage.length,
      previewFrameMismatches: legacy.validation.previewFrameMismatches,
      allRowsPending: rows.every(row => row.status === 'pending'),
      allEvidenceNull: rows.every(row => Object.values(row.evidence).every(value => value === null)),
    },
  }
}

// ---------------------------------------------------------------------------
// CLI
// ---------------------------------------------------------------------------

function parseArgs(argv) {
  const options = { out: DEFAULT_OUT, explicitOut: false, stdout: false, check: false, help: false, production: false }
  for (let index = 0; index < argv.length; index += 1) {
    const arg = argv[index]
    if (arg === '--stdout') options.stdout = true
    else if (arg === '--check') options.check = true
    else if (arg === '--production') options.production = true
    else if (arg === '--help' || arg === '-h') options.help = true
    else if (arg === '--out') {
      index += 1
      if (!argv[index]) fail('--out needs a path')
      options.out = resolve(process.cwd(), argv[index])
      options.explicitOut = true
    } else {
      fail(`Unknown argument: ${arg}\nUse --help for usage.`)
    }
  }
  if (options.production && !options.help) {
    if (!options.explicitOut && !options.stdout) fail('--production requires --out; the legacy receipt must not be overwritten')
    if (options.explicitOut) {
      const sameFile = existsSync(options.out) && existsSync(DEFAULT_OUT)
        && (() => {
          const target = statSync(options.out)
          const receipt = statSync(DEFAULT_OUT)
          return target.dev === receipt.dev && target.ino === receipt.ino
        })()
      if (options.out === DEFAULT_OUT || sameFile) fail('--production cannot target the legacy receipt')
    }
  }
  return options
}

function usage() {
  return [
    'Usage: node tests/viderse-workflow/artboard-state-manifest.mjs [options]',
    '',
    'Read-only extractor. Default output:',
    `  ${DEFAULT_OUT_REL}`,
    '',
    'Options:',
    '  --out <path>  Write to a different path',
    '  --stdout      Print the manifest instead of writing',
    '  --check       Compare on-disk JSON with a fresh render (exit 1 on drift)',
    '  --production Build the 29-board production inventory including desktop Auth; requires --out',
    '  --help        Show this help',
  ].join('\n')
}

function main() {
  const options = parseArgs(process.argv.slice(2))
  if (options.help) {
    process.stdout.write(`${usage()}\n`)
    return
  }
  const manifest = options.production ? buildProductionManifest() : buildManifest()
  const text = `${JSON.stringify(manifest, null, 2)}\n`
  if (options.stdout) {
    process.stdout.write(text)
    return
  }
  if (options.check) {
    if (!existsSync(options.out)) {
      process.stderr.write(`MISSING ${relative(REPO, options.out)}\n`)
      process.exitCode = 1
      return
    }
    const current = readFileSync(options.out, 'utf8')
    if (current !== text) {
      process.stderr.write(`DRIFT ${relative(REPO, options.out)}\n`)
      process.stderr.write(`expected ${text.length} bytes, found ${current.length} bytes\n`)
      process.exitCode = 1
      return
    }
    process.stdout.write(`OK ${relative(REPO, options.out)} (${manifest.validation.totalRows} rows, ${manifest.validation.probes} probes)\n`)
    return
  }
  mkdirSync(dirname(options.out), { recursive: true })
  writeFileSync(options.out, text)
  process.stdout.write(
    `Wrote ${relative(REPO, options.out)}: ${manifest.validation.totalRows} rows, ${manifest.validation.probes} probes, ` +
      `${manifest.validation.includedStatefulBoards} stateful boards, ${manifest.validation.previewFrameMismatches.length} README/footer note(s)\n`,
  )
}

try {
  main()
} catch (error) {
  if (error?.details !== undefined) {
    process.stderr.write(`${error.message}\n`)
    process.stderr.write(`${JSON.stringify(error.details, null, 2)}\n`)
  } else {
    process.stderr.write(`${error?.stack ?? error}\n`)
  }
  process.exitCode = 1
}
