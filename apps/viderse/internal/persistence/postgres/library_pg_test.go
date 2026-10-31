// Ca test Postgres của mảng Thư viện: CAS revision, chặn xoá khi kênh dùng,
// tên prompt duy nhất không phân biệt hoa thường, tombstone lớp phủ, và kiểm
// tham chiếu kênh/video nền. Chạy trên Postgres thật; bỏ qua khi thiếu
// TEST_DATABASE_URL (cùng provision với settings_pg_test).
package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/library"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
)

func newLibraryService(t *testing.T) (*library.Service, *pgxpool.Pool) {
	t.Helper()
	pool := provision(t)
	svc := library.New(library.Deps{
		Store: postgres.NewLibraryStore(pool),
		Now:   func() time.Time { return time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC) },
	})
	return svc, pool
}

func TestLibraryTemplateStoreRoundTrip(t *testing.T) {
	svc, _ := newLibraryService(t)
	ctx := context.Background()
	created, err := svc.Create(ctx, "Template Một")
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.Get(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Template Một" || got.Revision != 1 || got.Recipe.Scenes.Style != library.StyleCutout {
		t.Fatalf("đọc lại = %+v", got)
	}
	if got.PreviewState() != domain.PreviewNone {
		t.Fatalf("preview = %s", got.PreviewState())
	}
	list, err := svc.List(ctx)
	if err != nil || len(list) != 1 || list[0].ID != created.ID {
		t.Fatalf("list = %+v %v", list, err)
	}
	// PATCH hợp lệ: revision +1, updated_at mới
	updated, err := svc.Patch(ctx, created.ID, library.Patch{Revision: 1, Recipe: []byte(`{"scenes":{"images":{"mode":"fixed","count":8}}}`)})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Revision != 2 || updated.Recipe.Scenes.Images.Count != 8 {
		t.Fatalf("sau patch = %+v", updated)
	}
	// PATCH revision cũ: 409
	if _, err := svc.Patch(ctx, created.ID, library.Patch{Revision: 1, Recipe: []byte(`{}`)}); library.CodeOf(err) != library.CodeConflict {
		t.Fatalf("revision cũ = %v", err)
	}
	// RENAME giữ revision
	renamed, err := svc.Rename(ctx, created.ID, "Tên Khác", nil)
	if err != nil || renamed.Name != "Tên Khác" || renamed.Revision != 2 {
		t.Fatalf("rename = %+v %v", renamed, err)
	}
	// DUPLICATE rồi DELETE
	dup, err := svc.Duplicate(ctx, created.ID, "")
	if err != nil || dup.Name != "Tên Khác — Bản sao" {
		t.Fatalf("nhân bản = %+v %v", dup, err)
	}
	if err := svc.DeleteTemplate(ctx, dup.ID); err != nil {
		t.Fatalf("xoá bản sao: %v", err)
	}
	if _, err := svc.Get(ctx, dup.ID); library.CodeOf(err) != library.CodeNotFound {
		t.Fatalf("đã xoá: %v", err)
	}
}

func TestLibraryTemplateRevisionRaceOnPostgres(t *testing.T) {
	svc, _ := newLibraryService(t)
	ctx := context.Background()
	created, err := svc.Create(ctx, "Đua")
	if err != nil {
		t.Fatal(err)
	}
	type result struct{ err error }
	results := make(chan result, 2)
	for i := range 2 {
		go func(i int) {
			_, err := svc.Patch(ctx, created.ID, library.Patch{
				Revision: 1,
				Recipe:   []byte(fmt.Sprintf(`{"scenes":{"motion":{"kind":"wander","intensity":1,"speed":%d}}}`, i+1)),
			})
			results <- result{err: err}
		}(i)
	}
	wins, conflicts := 0, 0
	for range 2 {
		r := <-results
		switch {
		case r.err == nil:
			wins++
		case library.CodeOf(r.err) == library.CodeConflict:
			conflicts++
		default:
			t.Fatalf("lỗi lạ: %v", r.err)
		}
	}
	if wins != 1 || conflicts != 1 {
		t.Fatalf("wins=%d conflicts=%d", wins, conflicts)
	}
	stored, err := svc.Get(ctx, created.ID)
	if err != nil || stored.Revision != 2 {
		t.Fatalf("sau đua = %+v %v", stored, err)
	}
}

func TestLibraryTemplateDeleteBlockedWhenChannelUses(t *testing.T) {
	svc, pool := newLibraryService(t)
	ctx := context.Background()
	tpl, err := svc.Create(ctx, "Của Kênh")
	if err != nil {
		t.Fatal(err)
	}
	channelID := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	if _, err := pool.Exec(ctx, `INSERT INTO owner_channels (id) VALUES ($1)`, channelID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO owner_channel_templates (owner_channel_id, template_id, position) VALUES ($1, $2, 0)`, channelID, tpl.ID); err != nil {
		t.Fatal(err)
	}
	detail, err := svc.Get(ctx, tpl.ID)
	if err != nil || len(detail.Channels) != 1 {
		t.Fatalf("kênh dùng = %+v %v", detail, err)
	}
	err = svc.DeleteTemplate(ctx, tpl.ID)
	var apiErr *library.Error
	if !errors.As(err, &apiErr) || apiErr.Code != library.CodeConflict || !strings.Contains(apiErr.Message, "Đang dùng ở 1 kênh") {
		t.Fatalf("chặn xoá = %+v", err)
	}
	// Store tự nó cũng chặn (RESTRICT) khi service bị gọi thẳng.
	if err := postgres.NewLibraryStore(pool).DeleteTemplate(ctx, tpl.ID); !errors.Is(err, library.ErrInUse) {
		t.Fatalf("FK = %v", err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM owner_channel_templates WHERE template_id = $1`, tpl.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteTemplate(ctx, tpl.ID); err != nil {
		t.Fatalf("xoá sau khi bỏ khỏi kênh: %v", err)
	}
}

// Mẫu chuẩn tạo lúc kênh chưa có tên được đặt theo tên kênh khi kênh có tên;
// template dùng chung với kênh khác, đã đổi tên tay, hoặc không gắn kênh này
// giữ nguyên; revision không đổi (tên không nằm trong bản xem thử).
func TestLibraryNameStandardTemplatesOnlyOwnPlain(t *testing.T) {
	svc, pool := newLibraryService(t)
	ctx := context.Background()
	chA := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	chB := "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	for _, id := range []string{chA, chB} {
		if _, err := pool.Exec(ctx, `INSERT INTO owner_channels (id) VALUES ($1)`, id); err != nil {
			t.Fatal(err)
		}
	}
	create := func() string {
		id, err := svc.CreateStandardTemplate(ctx, "")
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	attach := func(channelID, templateID string, pos int) {
		if _, err := pool.Exec(ctx, `INSERT INTO owner_channel_templates (owner_channel_id, template_id, position) VALUES ($1, $2, $3)`, channelID, templateID, pos); err != nil {
			t.Fatal(err)
		}
	}
	own, shared, custom, other := create(), create(), create(), create()
	attach(chA, own, 0)
	attach(chA, shared, 1)
	attach(chB, shared, 0)
	attach(chA, custom, 2)
	attach(chB, other, 1)
	if _, err := svc.Rename(ctx, custom, "Của tôi", nil); err != nil {
		t.Fatal(err)
	}
	before, err := svc.Get(ctx, own)
	if err != nil {
		t.Fatal(err)
	}

	if err := svc.NameStandardTemplates(ctx, chA, "Insanity Inside"); err != nil {
		t.Fatalf("NameStandardTemplates: %v", err)
	}
	want := map[string]string{own: "Mẫu chuẩn · Insanity Inside", shared: "Mẫu chuẩn", custom: "Của tôi", other: "Mẫu chuẩn"}
	for id, name := range want {
		got, err := svc.Get(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if got.Name != name {
			t.Errorf("template %s tên = %q, muốn %q", id, got.Name, name)
		}
	}
	after, _ := svc.Get(ctx, own)
	if after.Revision != before.Revision {
		t.Errorf("revision %d → %d, đặt tên không được làm cũ bản xem thử", before.Revision, after.Revision)
	}
	// Lần sau (đổi tên kênh lần nữa) không đụng template đã mang tên kênh.
	if err := svc.NameStandardTemplates(ctx, chA, "Tên Khác"); err != nil {
		t.Fatal(err)
	}
	if got, _ := svc.Get(ctx, own); got.Name != "Mẫu chuẩn · Insanity Inside" {
		t.Errorf("đổi tên lần hai đã đụng template: %q", got.Name)
	}
}

func TestLibraryPromptStore(t *testing.T) {
	svc, _ := newLibraryService(t)
	ctx := context.Background()
	created, err := svc.CreatePrompt(ctx, library.PromptInput{
		Name: "Bìa Nhà Em", Kind: domain.PromptCover, Composition: "hook", Prompt: new("vẽ nhà"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreatePrompt(ctx, library.PromptInput{
		Name: "bìa nhà em", Kind: domain.PromptCover, Composition: "clean", Prompt: new("khác"),
	}); library.CodeOf(err) != library.CodeConflict {
		t.Fatalf("trùng hoa thường: %v", err)
	}
	// tìm không dấu trên prompt của bạn (fold_title trong SQL)
	items, err := svc.ListPrompts(ctx, "", "bia nha")
	if err != nil || len(items) != 1 || items[0].ID != created.ID {
		t.Fatalf("tìm = %+v %v", items, err)
	}
	// sửa tên + nội dung
	updated, err := svc.PatchPrompt(ctx, created.ID, library.PromptPatch{Name: new("Bìa Mới"), Prompt: new("nội dung mới")})
	if err != nil || updated.Name != "Bìa Mới" || updated.Prompt != "nội dung mới" {
		t.Fatalf("sửa = %+v %v", updated, err)
	}
	// dùng trong template rồi thử xoá → in_use
	tpl, _ := svc.Create(ctx, "Dùng prompt")
	if _, err := svc.Patch(ctx, tpl.ID, library.Patch{
		Revision: 1,
		Recipe:   []byte(fmt.Sprintf(`{"scenes":{"coverPromptId":%q}}`, created.ID)),
	}); err != nil {
		t.Fatal(err)
	}
	usage, err := svc.PromptUsage(ctx)
	if err != nil || len(usage[created.ID]) != 1 || usage[created.ID][0].ID != tpl.ID {
		t.Fatalf("usage = %+v %v", usage, err)
	}
	if err := svc.DeletePrompt(ctx, created.ID); library.CodeOf(err) != library.CodeInUse {
		t.Fatalf("xoá đang dùng = %v", err)
	}
	// bỏ khỏi template rồi xoá được
	if _, err := svc.Patch(ctx, tpl.ID, library.Patch{Revision: 2, Recipe: []byte(`{"scenes":{"coverPromptId":"builtin:cover-clean"}}`)}); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeletePrompt(ctx, created.ID); err != nil {
		t.Fatalf("xoá sau khi bỏ: %v", err)
	}
}

func TestLibraryOverlayStoreTombstone(t *testing.T) {
	_, pool := newLibraryService(t)
	ctx := context.Background()
	store := postgres.NewLibraryStore(pool)
	sha := strings.Repeat("c", 64)
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	duration := 4.5
	width, height := 1280, 720
	overlay := domain.Overlay{
		ID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", SHA256: sha, Name: "Mưa",
		FilePath: "library/overlays/mua.mp4", Mime: "video/mp4", Bytes: 100,
		DurationSec: &duration, Width: &width, Height: &height,
		Background: domain.BgBlack, CreatedAt: now,
	}
	if err := store.InsertOverlay(ctx, overlay); err != nil {
		t.Fatal(err)
	}
	// Cùng nội dung (sha) ở dòng khác đụng overlays_sha256_key, không phải pkey.
	dup := overlay
	dup.ID = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
	if err := store.InsertOverlay(ctx, dup); !errors.Is(err, library.ErrOverlayExists) {
		t.Fatalf("trùng sha = %v", err)
	}
	found, removed, err := store.OverlayBySHA(ctx, sha)
	if err != nil || found == nil || removed || found.DurationSec == nil || *found.DurationSec != 4.5 {
		t.Fatalf("theo sha = %+v removed=%v %v", found, removed, err)
	}
	if err := store.RemoveOverlay(ctx, overlay.ID); err != nil {
		t.Fatal(err)
	}
	if live, err := store.OverlayByID(ctx, overlay.ID); err != nil || live != nil {
		t.Fatalf("đã bỏ vẫn đọc được = %+v %v", live, err)
	}
	found, removed, err = store.OverlayBySHA(ctx, sha)
	if err != nil || found == nil || !removed {
		t.Fatalf("tombstone = %+v %v", found, removed)
	}
	if found, _, err := store.OverlayBySHA(ctx, strings.Repeat("d", 64)); err != nil || found != nil {
		t.Fatalf("sha lạ = %+v %v", found, err)
	}
	if list, err := store.ListOverlays(ctx); err != nil || len(list) != 0 {
		t.Fatalf("list = %+v %v", list, err)
	}
	if err := store.RestoreOverlay(ctx, overlay.ID, "Mưa 2"); err != nil {
		t.Fatal(err)
	}
	list, err := store.ListOverlays(ctx)
	if err != nil || len(list) != 1 || list[0].Name != "Mưa 2" {
		t.Fatalf("khôi phục = %+v %v", list, err)
	}
}

func TestLibraryBackdropReferenceChecks(t *testing.T) {
	_, pool := newLibraryService(t)
	ctx := context.Background()
	store := postgres.NewLibraryStore(pool)
	if _, err := pool.Exec(ctx, `INSERT INTO backdrop_channels (id, platform, handle, url) VALUES ('twitch:abc', 'twitch', 'abc', 'https://twitch.tv/abc')`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO backdrop_channel_videos (channel_id, video_id, kind, url, listed_at) VALUES ('twitch:abc', 'twitch:v1', 'vod', 'https://x/1', now())`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO backdrop_videos (id, platform, url, segment_to_sec) VALUES ('twitch:fixed', 'twitch', 'https://x/f', 30)`); err != nil {
		t.Fatal(err)
	}
	missing, err := store.BackdropChannels(ctx, []string{"twitch:abc", "twitch:nope", "bilibili:123"})
	if err != nil || len(missing) != 2 || missing[0] != "twitch:nope" || missing[1] != "bilibili:123" {
		t.Fatalf("kênh thiếu = %v %v", missing, err)
	}
	missing, err = store.BackdropVideos(ctx, "twitch:abc", []string{"twitch:v1", "twitch:v2"})
	if err != nil || len(missing) != 1 || missing[0] != "twitch:v2" {
		t.Fatalf("video thiếu = %v %v", missing, err)
	}
	if ok, err := store.BackdropVideoExists(ctx, "twitch:fixed"); err != nil || !ok {
		t.Fatalf("video lẻ = %v %v", ok, err)
	}
	if ok, err := store.BackdropVideoExists(ctx, "twitch:gone"); err != nil || ok {
		t.Fatalf("video lẻ thiếu = %v %v", ok, err)
	}
	if ok, err := store.OverlayExists(ctx, "cccccccc-cccc-4ccc-8ccc-cccccccccccc"); err != nil || ok {
		t.Fatalf("lớp phủ lạ = %v %v", ok, err)
	}
	if ok, err := store.PromptExists(ctx, library.BuiltinScene, domain.PromptScene); err != nil || ok {
		t.Fatalf("prompt có sẵn không có dòng: %v %v", ok, err)
	}
}

func TestLibraryOverlayUsageFromTemplates(t *testing.T) {
	svc, pool := newLibraryService(t)
	ctx := context.Background()
	store := postgres.NewLibraryStore(pool)
	sha := strings.Repeat("e", 64)
	overlay := domain.Overlay{
		ID: "dddddddd-dddd-4ddd-8ddd-dddddddddddd", SHA256: sha, Name: "Sương",
		FilePath: "library/overlays/suong.mp4", Mime: "video/mp4", Bytes: 10,
		Background: domain.BgNone, CreatedAt: time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC),
	}
	if err := store.InsertOverlay(ctx, overlay); err != nil {
		t.Fatal(err)
	}
	tpl, err := svc.Create(ctx, "Có lớp phủ")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Patch(ctx, tpl.ID, library.Patch{
		Revision: 1,
		Recipe:   []byte(fmt.Sprintf(`{"scenes":{"style":"cutout","images":{"mode":"fixed","count":5},"coverPromptId":"builtin:cover-hook","coverSource":"upload","referenceImages":true,"referenceStyle":"composition","sceneOpacity":1},"overlays":[%q]}`, overlay.ID)),
	}); err != nil {
		t.Fatal(err)
	}
	usage, err := svc.OverlayUsage(ctx)
	if err != nil || len(usage[overlay.ID]) != 1 || usage[overlay.ID][0].ID != tpl.ID {
		t.Fatalf("usage = %+v %v", usage, err)
	}
	if list, err := store.ListOverlays(ctx); err != nil || len(list) != 1 || list[0].Builtin != "" {
		t.Fatalf("list = %+v %v", list, err)
	}
	if builtins, err := store.ExistingBuiltins(ctx); err != nil || len(builtins) != 0 {
		t.Fatalf("builtins = %+v %v", builtins, err)
	}
}

// TestLibraryPromptRefLockedOnTemplateWrite kiểm khoá tham chiếu prompt:
// xoá/đổi loại prompt trực tiếp ở DB rồi ghi template trỏ tới nó phải bị chặn
// (đua giữa kiểm CheckRefs và UPDATE đã đóng bằng FOR UPDATE trong cùng tx).
func TestLibraryPromptRefLockedOnTemplateWrite(t *testing.T) {
	svc, pool := newLibraryService(t)
	ctx := context.Background()
	store := postgres.NewLibraryStore(pool)
	tpl, err := svc.Create(ctx, "Khoá prompt")
	if err != nil {
		t.Fatal(err)
	}
	prompt, err := svc.CreatePrompt(ctx, library.PromptInput{
		Name: "Cảnh khoá", Kind: domain.PromptScene, Prompt: new("x"),
	})
	if err != nil {
		t.Fatal(err)
	}
	// Prompt bị xoá thẳng ở DB → ghi template trỏ tới nó phải lỗi tham chiếu.
	if _, err := pool.Exec(ctx, `DELETE FROM image_prompts WHERE id = $1`, prompt.ID); err != nil {
		t.Fatal(err)
	}
	recipe := domain.Recipe{}
	recipe.Scenes.ScenePromptID = prompt.ID
	_, _, err = store.UpdateTemplate(ctx, tpl.ID, nil, recipe, tpl.Revision)
	if !errors.Is(err, library.ErrPromptRef) {
		t.Fatalf("ghi với prompt đã xoá = %v", err)
	}
	// Prompt đổi loại ở DB → ghi template trỏ cover vào scene cũng bị chặn.
	p2, err := svc.CreatePrompt(ctx, library.PromptInput{
		Name: "Bìa khoá", Kind: domain.PromptCover, Composition: "hook", Prompt: new("y"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE image_prompts SET kind='scene', composition=NULL WHERE id = $1`, p2.ID); err != nil {
		t.Fatal(err)
	}
	recipe.Scenes.CoverPromptID = p2.ID
	_, _, err = store.UpdateTemplate(ctx, tpl.ID, nil, recipe, tpl.Revision)
	if !errors.Is(err, library.ErrPromptRef) {
		t.Fatalf("ghi với prompt đổi loại = %v", err)
	}
	// Đường xoá phía kia: template còn trỏ prompt thì DeletePrompt bị chặn
	// ngay cả khi service bỏ qua bước kiểm trước.
	p3, err := svc.CreatePrompt(ctx, library.PromptInput{
		Name: "Scene dùng", Kind: domain.PromptScene, Prompt: new("z"),
	})
	if err != nil {
		t.Fatal(err)
	}
	recipe = domain.Recipe{}
	recipe.Scenes.ScenePromptID = p3.ID
	if _, _, err := store.UpdateTemplate(ctx, tpl.ID, nil, recipe, tpl.Revision); err != nil {
		t.Fatal(err)
	}
	if err := store.DeletePrompt(ctx, p3.ID); !errors.Is(err, library.ErrInUse) {
		t.Fatalf("xoá prompt còn dùng = %v", err)
	}
	// Đổi loại khi còn dùng cũng bị chặn ở store.
	if err := store.UpdatePrompt(ctx, domain.ImagePrompt{
		ID: p3.ID, Name: p3.Name, Kind: domain.PromptCover, Composition: "hook",
		Prompt: p3.Prompt, UpdatedAt: time.Now().UTC(),
	}); !errors.Is(err, library.ErrInUse) {
		t.Fatalf("đổi loại còn dùng = %v", err)
	}
}

// TestLibraryBackdropRefLockedOnTemplateWrite: tham chiếu video nền được kiểm
// dưới advisory lock trong cùng transaction ghi — kênh/video đã mất ở DB phải
// chặn lần ghi recipe dù CheckRefs phía trước đã qua.
func TestLibraryBackdropRefLockedOnTemplateWrite(t *testing.T) {
	svc, pool := newLibraryService(t)
	ctx := context.Background()
	store := postgres.NewLibraryStore(pool)
	tpl, err := svc.Create(ctx, "Khoá nền")
	if err != nil {
		t.Fatal(err)
	}
	// Kênh/video nền thật trong DB.
	if _, err := pool.Exec(ctx, `INSERT INTO backdrop_channels (id, platform, handle, url) VALUES ('twitch:lockchan', 'twitch', 'lockchan', 'https://twitch.tv/lockchan')`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO backdrop_channel_videos (channel_id, video_id, kind, url, listed_at) VALUES ('twitch:lockchan', 'v1', 'clip', 'https://x', now())`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO backdrop_videos (id, platform, url, segment_to_sec, state) VALUES ('twitch:vv', 'twitch', 'https://x', 30, 'ready')`); err != nil {
		t.Fatal(err)
	}
	recipe := domain.Recipe{Backdrop: &domain.BackdropRecipe{
		ClipSec: 60,
		Channels: []domain.BackdropChannelLink{{
			ChannelID: "twitch:lockchan", Kinds: []string{"clip"}, Pinned: []string{"v1"},
		}},
		FixedVideoIDs: []string{"twitch:vv"},
	}}
	if _, _, err := store.UpdateTemplate(ctx, tpl.ID, nil, recipe, tpl.Revision); err != nil {
		t.Fatalf("ghi recipe nền hợp lệ: %v", err)
	}
	// Xoá kênh thẳng ở DB (mô phỏng xoá thắng lock) → ghi lại phải bị chặn.
	if _, err := pool.Exec(ctx, `DELETE FROM backdrop_channels WHERE id='twitch:lockchan'`); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.UpdateTemplate(ctx, tpl.ID, nil, recipe, 2); !errors.Is(err, library.ErrBackdropRef) {
		t.Fatalf("ghi với kênh đã xoá = %v", err)
	}
	// Video lẻ đã xoá → cũng bị chặn.
	if _, err := pool.Exec(ctx, `DELETE FROM backdrop_videos WHERE id='twitch:vv'`); err != nil {
		t.Fatal(err)
	}
	recipe2 := domain.Recipe{Backdrop: &domain.BackdropRecipe{ClipSec: 60, FixedVideoIDs: []string{"twitch:vv"}}}
	if _, _, err := store.UpdateTemplate(ctx, tpl.ID, nil, recipe2, 2); !errors.Is(err, library.ErrBackdropRef) {
		t.Fatalf("ghi với video lẻ đã xoá = %v", err)
	}
	// Video ghim thiếu trong kênh còn sống → chặn.
	if _, err := pool.Exec(ctx, `INSERT INTO backdrop_channels (id, platform, handle, url) VALUES ('twitch:lockchan', 'twitch', 'lockchan', 'https://twitch.tv/lockchan')`); err != nil {
		t.Fatal(err)
	}
	recipe3 := domain.Recipe{Backdrop: &domain.BackdropRecipe{ClipSec: 60,
		Channels: []domain.BackdropChannelLink{{ChannelID: "twitch:lockchan", Kinds: []string{"clip"}, Pinned: []string{"v1"}}}}}
	if _, _, err := store.UpdateTemplate(ctx, tpl.ID, nil, recipe3, 2); !errors.Is(err, library.ErrBackdropRef) {
		t.Fatalf("ghi với video ghim thiếu = %v", err)
	}
}
