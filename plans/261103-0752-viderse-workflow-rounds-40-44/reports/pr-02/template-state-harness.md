# PR2 · V36 — Template viewer acceptance harness

- `tests/viderse-workflow/template-state-audit.pw.ts` có bốn native cases, kế thừa desktop/mobile và normal/reduced. Parent đã chạy lượt41:14pass/2fail; hai lỗi desktop là assertion HTML title sai với kit Tooltip. Sau bỏ assertion incidental, hai ca đó lượt42 qua2/2 bằng hover/visible-tooltip native. Không gọi hai lượt này là một lượt16/16 mới.
- Parent registered the file in `playwright.config.ts`. Actual origin setting is `WORKFLOW_APP_URL` (default8044), with `WORKFLOW_QA_COOKIE_FILE`, `WORKFLOW_QA_FIXTURE`, `PW_BROWSER_BINARY`, `WORKFLOW_EVIDENCE_DIR`; `WORKFLOW_QA_REAL_COVER=1` enables the already-generated real cover case. No credentials belong in the repository.
- **Gate nguồn tích hợp cuối:** [lượt54](runs/54-template-embedded-real-cover/interaction-results.json) chạy trực tiếp binary nhúng8043, **16/16 qua,0skip/0lỗi**. Native desktop/mobile và normal/reduced đủ cover tab, group label, source/settings, saved real-trial preview, decoded dimensions, Cancel/Esc/focus,0API mutation. Lượt53 trước đó12qua/4skip chỉ vì parent chưa bật cờ đọc trial đã có; không phải thiếu ảnh và không gọi thêm AI để sửa.

## Scope correction against the actual artboard

The parent delegation incorrectly named `cover=phrase/separate` and `tagline=none/art` probes. They are **not user requirements or current Template data-props** and are not missing backend features. No such controls/fields will be invented.

`Template.dc.html:1396` declares `section=images|cover|backdrop|output`, `preview=fresh|stale|building`, `fewClips`, `picker=none|add|choose|video`. The generated matrix assigns those Template tweak rows to PR3/PR4, not PR2. V36 owns the sample-viewer entrypoints on this screen; later Template cutover remains fully in scope. Prompt compositions currently use clean/hook/circle-arrow/hook-arrow/split/closeup and are data, not extra artboard tweak keys.

## Native cases

| Case | Observable behavior |
|---|---|
| Images → Cover → Back | URL, selected navigation item, correct heading/save status; phone returns to the list; template revision unchanged. |
| Cover prompt sample | Correct cover radiogroup name; sample eye opens the exact saved image without selecting another prompt, or exposes its real disabled reason. Desktop Esc restores focus; no round45 phone viewer. |
| Existing cover source | Saved AI/upload selection and actual default-media image or empty state match the persisted recipe; no upload/remove/write is performed. |
| Actual saved AI cover trial | Exact recorded URL and decoded dimensions, desktop dialog containment/motion and phone same-surface Back preserving source/radios/focus. No draw action is pressed. |

All browser API mutations are blocked and recorded; any attempt fails the case. The real cover trial is reused from [run22](runs/22-real-cover-preview-parity/interaction-results.json); genuine exhausted-quota behavior already passed [run31](runs/31-real-exhausted-quota/interaction-results.json). Exactly2/2 authorized images have been consumed; no third image is allowed.

## Finding and parent repair

`PromptPicker` used scene copy for both scene and cover groups. Parent added an explicit required scene/cover context, migrated both consumers, and uses existing section-specific copy for the accessible name/new-prompt hint. This fixes the cover group being announced as “Chỉ dẫn ảnh cảnh”. Native run41 verified the cover name; the desktop disabled-sample tooltip path passed run42 after the harness correction. Focused library+Review79/79 and typecheck passed on this source.

The earlier unsupported-variant absence assertions were removed: tests must not freeze an invented omission. Desktop composition-caption differences are not accepted as a product gap without an actual requirement in the owning artboard. No artboard or backend schema was changed.
