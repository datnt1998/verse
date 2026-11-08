# Kênh · danh sách Video

Status: deployed to https://vid.aarverse.studio at main `1491dddb1ba653841feaba1c1f025f9cb797ae2e` (PR#26),08/11/2026. User decision#96 supersedes removal of the Video tab, not the rule that video details open in Bàn dựng. Artboards unchanged.

## Delivered contract

- Video is the default channel tab on desktop and phone; explicit Nguồn/Template/Hồ sơ routes remain valid.
- Reuse GET `/api/clone?channel=<id>` and its existing event-invalidated cache key. Show all returned channel videos with real source thumbnail/title, Vietnamese stage label and update date; no new mutation, paid action, video copy or backend schema.
- Every row is a native link to Bàn dựng with channel and video IDs. Removing cross-key keepPrevious for channel jobs prevents old channel rows and links appearing while another channel loads.
- Loading, genuinely empty, failed read with retry, and stale refresh error are distinct. Tokens reuse the shared UI/channel palette and spacing/type scales.

## Verification

- Channel/router suite:53tests passed across9files. The existing explore fixture emits an empty-image-src warning; not suppressed. Final new regression tests2/2passed after fixture/type corrections.
- TypeScript, production build and size gates passed:entry142.6KiB,phone179.8KiB,CSS33.9KiB gzip. Existing Rollup annotation/chunk warnings remain visible. Native build/help passed.
- Authenticated production API contains4videos for the real channel, all ready, with createdAt/updatedAt and the existing envelope fields.
- Actual production responses and events with candidate built assets:1440×900and390×844 both show4rows,no horizontal overflow. Mobile rows110px tall. First desktop row and last mobile row open the exact owning video/channel URL. All four mobile tabs change to their intended routes/panels.
- Candidate screenshots: `/tmp/viderse-channel-videos-candidate-desktop.webp`, `/tmp/viderse-channel-videos-candidate-mobile.webp`; candidate assets with real production data, not deployed or artboard acceptance evidence. Some desktop thumbnails were still loading in the capture; no synthetic images used.
- Initial checks caught an accidentally removed style import, incomplete API test fixtures, and loading/error React prop types; corrected before final verification. No server data was changed for this smoke.

No full artboard parity, physical Safari, successful new export, or production deployment is claimed by the candidate proof above.

## Production rollout and loaded-view proof

- Safe rollout completed2026-11-08T14:20:27Z; backup and restored-database rehearsal passed, schema41unchanged. Commit marker and native SHA256 `9ec87a5a920ed4a62ec3c12a44fc754ab48657759e198c75fd4b6114ca6e1f3b` match the tested release. Startup connection refusals/timeouts preceded successful readiness.
- Public `/readyz`200; served `index-DzYWJTFl.js` SHA256 `d5a0d0375bd251d2715d7d719f28c5e95fa5b0ecac857613e6df746b10c78d64` matches the tested build.
- Unsubstituted production1440×900and390×844 show4real videos, no horizontal overflow; phone row targets110px. Existing four mobile tabs select their correct routes. All4video links on each viewport were clicked and loaded a visible video element in Bàn dựng at the expected channel/video hash:8loaded-view checks,0unresponsive row links.
- Production captures: [desktop](channel-videos-production-desktop.webp), [phone](channel-videos-production-mobile.webp). These prove this feature, not full artboard or physical Safari acceptance.
- Expanded smoke initially hit a cached404 for a new lazy chunk in the test profile, left by candidate-origin request interception before rollout. Public assets were200with correct JavaScript while that profile returned404. Cache-disabled, network-backed browser verification loaded all8destinations. Independent read-only counsel found no row-anchor defect; no application retry/preload workaround was added. Current service worker has no fetch handler. Temporary asset substitution, preview service and owned test tabs/browser were closed.
