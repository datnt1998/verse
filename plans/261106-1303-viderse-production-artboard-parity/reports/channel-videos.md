# Kênh · danh sách Video

Status: candidate verified; production rollout pending. User decision08/11/2026 is recorded in design Q&A#96; artboards unchanged. This supersedes removal of the Video tab, not the rule that video details open in Bàn dựng.

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
