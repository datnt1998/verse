---
phase: 1
title: "Khóa inventory, parity và schema"
status: completed
priority: P1
effort: 20h
dependencies: []
---

# Phase 01 — Chốt dữ liệu và hợp đồng trước khi đổi backend

## Context và yêu cầu

Đọc [plan](./plan.md), [source inventory](./reports/codebase-evidence.md), [DDL/contracts](./reports/schema-and-contracts.md), `AGENTS.md:1-150`, `docs/architecture.md:20-58,103-120`. Repo yêu cầu preserve deterministic Engine/Flow, tools là cửa ghi dữ kiện và không biến observer thành writer. Không đổi quyết định văn học/routing khi đổi storage.

Nguồn đã xác minh: `Store` có 19 concrete substore (`internal/store/store.go:16-39,53-79`); compound outline/progress hiện dùng mutex + file IO chứ không transaction (`:270-321`). File IO atomic một file (`internal/store/io.go:100-145`), comments JSON và pending event JSON (`internal/comments/comments.go:121-140,895-951`), account hiện role admin/member và grants theo project (`internal/webauth/store.go:39-41,63-95,113-146`), password Argon2id (`internal/webauth/password.go:14-23,35-74`).

Yêu cầu: inventory tất cả persistent input/output; fixture sạch và anonymous/custom-directory variants; SQL model/authority/transaction boundaries được khóa; file mode không bị bỏ. Không sao chép dữ liệu thật/secret trong testdata. Schema report là starting design bắt buộc cho implementer, không viết schema mới ở mỗi domain.

## Kiến trúc / luồng

Source root + account file → read-only inventory manifest (relative path, type, size, SHA-256, schema/kind) → domain mapping → expected count/hash fixture. Fixture phản ánh cả accepted chapter record và prose đã sửa chưa accept; không chỉ snapshot happy-path. Không tự sửa source khi discover/validate.

Actor matrix: global admin/member × project none/view/collab/owner × active/disabled/must-change × sessions live/revoked/expired. Content matrix: old/current project format, pending commit/revision/import, flat/layered outline, Unicode LF/CRLF, partially written JSONL, missing/corrupt blob, custom library dirs, renamed/adopted project.

## Files / quyền sở hữu

Root viết tắt `R=/Users/dat/Projects/ainovel-cli`; mọi đường dẫn dưới là đầy đủ khi ghép `R/`.

- Modify `R/docs/architecture.md` (facts store/backend boundary) và `R/README.md` (migration scope/assumptions); chỉ phase 01 sở hữu đến handoff.
- Create `R/internal/store/contract_test.go`, `R/internal/store/testdata/storage-contract/` (sanitized fixtures), `R/internal/webauth/contract_test.go`, `R/internal/library/contract_test.go`; đây là file mới.
- Existing code nguồn, chưa đổi behavior: `R/internal/store/store.go:16`, `R/internal/store/chapter_records.go:24-45`, `R/internal/host/storyread.go:30-80`, `R/internal/library/library.go:47-96`, `R/internal/webauth/store.go:477-551`, `R/internal/comments/comments.go:50-106`.
- Không đổi Docker/production wiring trong phase này. Không có file Delete.

## Các bước

1. Đọc lại source inventory trước implement vì account files đã có thay đổi trong planning. Bổ sung manifest các persistent dependencies ngoài scope direct `os.`: `agentcoretools.NewProseEdit` (`internal/tools/edit_chapter.go:35`), comments hook (`internal/tools/comments_hook.go:14`), ledger (`internal/tools/ledger_gate.go:13-48`), sibling openings (`internal/tools/commit_chapter.go:666`), source/config/style loader. Mỗi path xếp loại facts/blob/derived/scratch/local-only/excluded có lý do; không dùng “tất cả callers” thay danh sách.
2. Tạo fixture từ formats thực: identity v3 map Projects và v1/v2 legacy books; project/story config; chapter record + workspace prose lệch; import manifest+intent+analyses; audio settings/cast/script/candidate/queue/export result; source symlink confined và escape. File cá nhân production chỉ đọc tại rehearsal, không thêm vào repo.
3. Viết parity contract trước extraction: absent vs corrupt/permission denied, deterministic ordering, JSON envelopes, Vietnamese error strings, same-input idempotence, recovery semantics. Contract có backend factories; chưa bắt buộc PG implementation ở phase 01 nhưng không tạo fake PG pass.
4. Khóa schema report: case-insensitive unique identifiers, two rolesets hiện có, exact-one-owner bằng lock+deferred checks, at-least-one-admin, immutable revisions with author, explicit scope/composite FK, snapshots vs append streams, CAS version, soft-delete/retention. Chapter text trong Postgres, không nhét vào blob rồi coi DB là index.
5. Ghi authority map: `meta/book.json` title/synopsis là facts; story title có title_renamed rule (`internal/library/library.go:79-96`); markdown/nao là derived. Audio cache key phải dùng input content hash thay size/mtime. Mỗi domain có import/reverse-export mapping.
6. Chốt fail-before names/lệnh ở các phase sau và dependency gate: 02 chưa được bắt đầu nếu fixture/authority/schema còn hở. Không dựng live dual-write để “giảm rủi ro”.

## Ma trận test (fail-before)

| Lớp | Test phải viết trước | Chứng minh thất bại trước thay đổi |
|---|---|---|
| Unit | `TestStorageContractFile`, `TestStorageAuthority`, absent/corrupt distinction | Regression bắt empty fallback, sai canonical title/record |
| Unit | `TestAccountContractFile` gồm create/update/temp/change/revoke/grants | Snapshot role/admin/member và envelope không biến mất |
| Integration | `TestLibraryLayoutContract` với dirs override/legacy `_`/renamed roots | Hardcoded kich_ban/output fails fixture |
| E2E contract | Current HTTP fixtures của `members_ad_test.go:27`, `path_permissions_test.go`, `auth_test.go:156` | Ghi baseline bằng real handlers, không mock server UI |

## Lệnh kiểm chứng cho implementer

```sh
go test ./internal/store -run 'TestStorageContractFile|TestStorageAuthority' -count=1
go test ./internal/webauth -run 'TestAccountContractFile|TestMustChangePasswordLifecycle' -count=1
go test ./internal/library -run TestLibraryLayoutContract -count=1
go test ./internal/entry/web -run 'TestAD|TestMustChangePasswordViaAPI' -count=1
```

Tên mới phải được tạo đúng như trên; các lệnh không được xem là pass nếu không có test nào chạy. Không chạy chúng trong lượt planning này.

## Rủi ro, bảo mật, rollback và gate

- **High (L cao × I cao):** bỏ sót IO ẩn → VPS lại cần home folder. Mitigation: inventory có source edges, fixture DB mode sau này chạy read-only/no-library mount; allowlist scratch chỉ ffmpeg/local import adapters.
- **High (L vừa × I cao):** fixture rò password/token/chapter cá nhân. Mitigation: nội dung tổng hợp, hash mẫu đúng thuật toán, scans dữ liệu fixture trước commit của implementer.
- **Medium (L vừa × I vừa):** hai backend lệch semantics. Mitigation: một contract suite, cùng expectation; không “PG expected khác” trừ atomicity file saga đã ghi rõ.

Rollback chỉ docs/tests fixture mới; chưa đổi production data. Gate observable: manifest phân loại không còn path unknown, fixture tái tạo mọi layout được liệt kê, contract baseline file có assertion thực; schema invariants/owner mapping rõ. Handoff quyền file cho 02, trạng thái 01 chỉ completed sau bằng chứng implementer, không phải sau khi đọc plan.

## Implementation evidence — 2026-10-28

- **Manifest**: `internal/store/testdata/storage-contract/manifest.json` classifies 60+ persistent paths into facts/blob/derived/scratch/local-only/excluded with reasons; no `unknown` class exists (enforced by `manifest_classifies_every_path`).
- **Fixtures**: sanitized synthetic trees under the same directory — `library/` (project + legacy `_` story + adoptable `cu-dir` candidate), `library-custom/` (dirs.stories=`truyen` override), `library-renamed/` (slug≠name, titleRenamed), `accounts/` (v3, v2-legacy books[]/writer/reader, corrupt). Source symlinks are created at test time (confined + escape). No real data or secrets; Argon2id hashes are of documented fixture passwords only.
- **Contract tests** (all assert current file behavior, green): `TestStorageContractFile`, `TestStorageAuthority` (`internal/store/contract_test.go`), `TestAccountContractFile` (`internal/webauth/contract_test.go`), `TestLibraryLayoutContract` (`internal/library/contract_test.go`).
- **Mutation proof** (temporarily mutated → observed failure → reverted):
  1. `io.go` corrupt-JSON→`os.ErrNotExist` → `absent_vs_corrupt` FAIL: `corrupt book.json must error, got <nil> <nil>` (reverted).
  2. `book.go` Load reads `book.md` → `book_json_is_facts_book_md_is_derived`, `save_regenerates_projection`, `absent_vs_corrupt` FAIL (reverted).
  3. `scan.go` Stories hardcodes `kich_ban` → `dirs_overrides_honored` FAIL: `Stories must list under dirs.stories: <nil> []` (reverted). A weaker mutation in `Scan`/`isLegacyStoryDir` did not fail — proof targets the `Stories` parent join.
  4. `webauth/store.go` validRole accepts `collaborator` → `envelope_and_roles` FAIL: `global role 'collaborator' must be rejected: <nil>` (reverted).
  5. `webauth/members.go` writer→view → `v2_legacy_migrates_grants` FAIL: `Projects:map[du-an:view truyen-cu:view]` (reverted).
- **Verification** (`-v` counts confirm tests ran): `go test ./internal/store -run 'TestStorageContractFile|TestStorageAuthority' -count=1` → 2 tests / 14 subtests PASS; `go test ./internal/webauth -run 'TestAccountContractFile|TestMustChangePasswordLifecycle'` → 2 tests / 7 subtests PASS; `go test ./internal/library -run TestLibraryLayoutContract` → 1 test / 6 subtests PASS; `go test ./internal/entry/web -run 'TestAD|TestMustChangePasswordViaAPI'` → 6 baseline tests PASS. `gofmt -l` clean, `go vet` clean on touched packages; full `go test` on touched packages green after reverts.
- **Docs**: `docs/architecture.md` §4.5 records authority/absent-vs-corrupt/append-log/library-layout contract; `README.md` §9 records the locked layout and the 2026-10-28 decisions (PG primary, file mode kept, TTS home worker).
- **Schema/authority/fail-before chốt**: schema report §9 ghi 4 quyết định ma trận; phase 02 có sqlc/queries/gen + `go tool sqlc vet|diff`; phase 04 có steps 10–11 + matrix cases cho 2 behavior changes (owner-scoped members API, collab DELETE keep). No PG code, no fake PG passes.
