// Ca test Postgres của mảng Kênh của bạn: CRUD, liên kết + tham chiếu, danh
// template mặc định, đoán ngôn ngữ từ đối thủ tham chiếu, và lease
// nhận diện (một run mỗi kênh, fence theo token + tên).
//
// Chạy trên Postgres thật (pgtest + migration nhúng); bỏ qua khi thiếu
// TEST_DATABASE_URL.
package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/datnt1998/verse/apps/viderse/internal/channels"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
)

const (
	rivalA = "UCaaaaaaaaaaaaaaaaaaaaaa"
	rivalB = "UCbbbbbbbbbbbbbbbbbbbbbb"
	chOne  = "11111111-1111-4111-8111-111111111111"
	chTwo  = "22222222-2222-4222-8222-222222222222"
	tplA   = "aaaaaaaa-1111-4111-8111-111111111111"
	tplB   = "bbbbbbbb-2222-4222-8222-222222222222"
)

func seedChannelRival(t *testing.T, pool *pgxpool.Pool, id, title string, avatar *string) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO rival_channels (id, title, description, avatar_url, url)
		 VALUES ($1, $2, $3, $4, $5)`,
		id, title, "Mô tả "+title, avatar, "https://www.youtube.com/@"+id)
	if err != nil {
		t.Fatal(err)
	}
}

func seedChannelTemplate(t *testing.T, pool *pgxpool.Pool, id, name string) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO templates (id, name, recipe) VALUES ($1, $2, '{}'::jsonb)`, id, name)
	if err != nil {
		t.Fatal(err)
	}
}

func seedChannelVideo(t *testing.T, pool *pgxpool.Pool, id, channelID, title string, position int64) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO rival_videos (id, channel_id, title, position, origin)
		 VALUES ($1, $2, $3, $4, 'baseline')`, id, channelID, title, position)
	if err != nil {
		t.Fatal(err)
	}
}

func TestChannelCreateListAndDelete(t *testing.T) {
	pool := provision(t)
	ctx := context.Background()
	seedChannelRival(t, pool, rivalA, "Chuyện Ma", nil)
	seedChannelRival(t, pool, rivalB, "Kinh Dị", nil)
	seedChannelTemplate(t, pool, tplA, "Mẫu chuẩn")
	seedFixtureTemplate(t, pool)
	store := postgres.NewChannelStore(pool)

	if _, err := store.CreateChannel(ctx, channels.CreateInput{
		ID: chOne, Name: "Kênh A", Language: domain.LangVI, AutoFetch: true,
		PerDay: 5, CompetitorIDs: []string{rivalA, rivalB}, DefaultTemplateID: tplA,
	}); err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}
	if _, err := store.CreateChannel(ctx, channels.CreateInput{
		ID: chTwo, Name: "", Language: domain.LangAuto, AutoFetch: true,
		PerDay: 5, DefaultTemplateID: tplA,
	}); err != nil {
		t.Fatalf("CreateChannel 2: %v", err)
	}

	list, err := store.ListChannels(ctx)
	if err != nil {
		t.Fatalf("ListChannels: %v", err)
	}
	if len(list) != 2 || list[0].ID != chOne || list[1].ID != chTwo {
		t.Fatalf("thứ tự = %+v, muốn kênh có tên trước, tên rỗng cuối", list)
	}
	one := list[0]
	if len(one.Sources) != 2 || !one.Sources[0].Reference || one.Sources[0].RivalID != rivalA {
		t.Fatalf("sources = %+v, muốn rivalA tham chiếu trước", one.Sources)
	}
	if one.Sources[0].Title != "Chuyện Ma" {
		t.Errorf("title = %q", one.Sources[0].Title)
	}
	if one.DefaultTemplate == nil || one.DefaultTemplate.ID != tplA || one.DefaultTemplateID != tplA {
		t.Fatalf("default = %+v", one.DefaultTemplate)
	}

	// Xoá kênh xoá link + template theo (cascade), không để gì treo.
	if ok, err := store.DeleteChannel(ctx, chOne); err != nil || !ok {
		t.Fatalf("DeleteChannel = %v %v", ok, err)
	}
	var links int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM owner_channel_sources WHERE owner_channel_id = $1`, chOne).Scan(&links); err != nil {
		t.Fatal(err)
	}
	if links != 0 {
		t.Fatalf("dangling source links=%d", links)
	}
	var exists bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM templates WHERE id=$1)`, tplA).Scan(&exists); err != nil || !exists {
		t.Fatalf("deleting a channel must preserve its library template: %v", err)
	}
	if _, found, err := store.GetChannel(ctx, chOne); err != nil || found {
		t.Fatalf("Get sau xoá = found=%v err=%v", found, err)
	}
}

func TestChannelCreateUntrackedCompetitorFails(t *testing.T) {
	pool := provision(t)
	seedChannelTemplate(t, pool, tplA, "Mẫu chuẩn")
	seedFixtureTemplate(t, pool)
	store := postgres.NewChannelStore(pool)
	_, err := store.CreateChannel(context.Background(), channels.CreateInput{
		ID: chOne, Language: domain.LangAuto, AutoFetch: true, PerDay: 5, DefaultTemplateID: tplA,
		CompetitorIDs: []string{"UCzzzzzzzzzzzzzzzzzzzzzz"},
	})
	if channels.CodeOf(err) != channels.CodeNotFound {
		t.Fatalf("code = %q, muốn not_found", channels.CodeOf(err))
	}
	if _, found, _ := store.GetChannel(context.Background(), chOne); found {
		t.Fatal("kênh phải không được tạo khi rival không tồn tại")
	}
}

func TestChannelUnlinkLastPausesAndReferenceMoves(t *testing.T) {
	pool := provision(t)
	ctx := context.Background()
	seedChannelRival(t, pool, rivalA, "A", nil)
	seedChannelRival(t, pool, rivalB, "B", nil)
	seedFixtureTemplate(t, pool)
	store := postgres.NewChannelStore(pool)
	if _, err := store.CreateChannel(ctx, channels.CreateInput{
		ID: chOne, Name: "Kênh", Language: domain.LangVI, AutoFetch: true, PerDay: 5, DefaultTemplateID: fixtureTemplate,
		CompetitorIDs: []string{rivalA, rivalB},
	}); err != nil {
		t.Fatal(err)
	}

	removed, paused, err := store.UnlinkSource(ctx, chOne, rivalA)
	if err != nil || !removed || paused {
		t.Fatalf("unlink = %v %v %v", removed, paused, err)
	}
	view, _, err := store.GetChannel(ctx, chOne)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Sources) != 1 || view.Sources[0].RivalID != rivalB || !view.Sources[0].Reference {
		t.Fatalf("tham chiếu không dời: %+v", view.Sources)
	}

	removed, paused, err = store.UnlinkSource(ctx, chOne, rivalB)
	if err != nil || !removed || !paused {
		t.Fatalf("unlink cuối = %v %v %v", removed, paused, err)
	}
	view, _, _ = store.GetChannel(ctx, chOne)
	if view.AutoFetch {
		t.Fatal("bỏ đối thủ cuối phải tắt Tự lấy")
	}
}

func TestChannelLanguageSampleUsesReference(t *testing.T) {
	pool := provision(t)
	ctx := context.Background()
	avatar := "https://yt3.googleusercontent.com/avatar.png"
	seedChannelRival(t, pool, rivalA, "Chuyện Ma Đêm", &avatar)
	seedChannelRival(t, pool, rivalB, "English Stories", nil)
	seedChannelVideo(t, pool, "vid00000001", rivalA, "Đêm đầu tiên ở nhà hoang", 0)
	seedChannelVideo(t, pool, "vid00000002", rivalA, "Tiếng gọi lúc nửa đêm", 1)
	seedFixtureTemplate(t, pool)
	store := postgres.NewChannelStore(pool)
	if _, err := store.CreateChannel(ctx, channels.CreateInput{
		ID: chOne, Name: "Kênh", Language: domain.LangAuto, AutoFetch: true, PerDay: 5, DefaultTemplateID: fixtureTemplate,
		CompetitorIDs: []string{rivalA, rivalB},
	}); err != nil {
		t.Fatal(err)
	}
	sample, err := store.ReferenceLanguageText(ctx, chOne)
	if err != nil {
		t.Fatalf("ReferenceLanguageText: %v", err)
	}
	if sample.Title != "Chuyện Ma Đêm" || len(sample.Titles) != 2 || sample.Titles[0] != "Đêm đầu tiên ở nhà hoang" {
		t.Fatalf("sample = %+v", sample)
	}
	comps, err := store.Competitors(ctx, chOne)
	if err != nil || len(comps) != 2 || comps[0].AvatarURL == nil {
		t.Fatalf("competitors = %+v err=%v", comps, err)
	}
}

func TestChannelIdentityLeaseAndPublishFence(t *testing.T) {
	pool := provision(t)
	ctx := context.Background()
	seedChannelRival(t, pool, rivalA, "A", nil)
	seedFixtureTemplate(t, pool)
	store := postgres.NewChannelStore(pool)
	if _, err := store.CreateChannel(ctx, channels.CreateInput{
		ID: chOne, Name: "Tên Chủ", Language: domain.LangVI, AutoFetch: true, PerDay: 5, DefaultTemplateID: fixtureTemplate,
		CompetitorIDs: []string{rivalA},
	}); err != nil {
		t.Fatal(err)
	}
	token := "33333333-3333-4333-8333-333333333333"
	until := time.Now().Add(time.Minute)
	ok, err := store.AcquireIdentityLease(ctx, channels.LeaseInput{
		ChannelID: chOne, Part: domain.IdentityAll, Owner: "test", Token: token, Until: until,
	})
	if err != nil || !ok {
		t.Fatalf("acquire = %v %v", ok, err)
	}
	// Run thứ hai bị từ chối khi lease còn sống.
	ok, err = store.AcquireIdentityLease(ctx, channels.LeaseInput{
		ChannelID: chOne, Part: domain.IdentityAll, Owner: "khác",
		Token: "44444444-4444-4444-8444-444444444444", Until: until,
	})
	if err != nil || ok {
		t.Fatalf("acquire lần hai = %v %v, muốn bị từ chối", ok, err)
	}
	live, err := store.IdentityLeaseLive(ctx, chOne)
	if err != nil || !live {
		t.Fatalf("lease live = %v %v", live, err)
	}
	if ok, err := store.HeartbeatIdentityLease(ctx, chOne, token, time.Now().Add(2*time.Minute)); err != nil || !ok {
		t.Fatalf("heartbeat = %v %v", ok, err)
	}

	// Đổi tên giữa chừng: publish đọc tên lúc bắt đầu nên bị từ chối.
	ok, err = store.PublishIdentity(ctx, channels.PublishInput{
		ChannelID: chOne, Token: token, Name: "Tên Mới", ExpectedName: "Tên Cũ",
		Branding:   domain.Branding{Brief: "b", Summary: "s", Look: "l", Avatar: "a", Banner: "n"},
		AvatarPath: "channels/x/1-avatar.png", BannerPath: "channels/x/1-banner.jpg",
	})
	if err != nil || ok {
		t.Fatalf("publish với tên cũ = %v %v, muốn bị từ chối", ok, err)
	}

	ok, err = store.PublishIdentity(ctx, channels.PublishInput{
		ChannelID: chOne, Token: token, Name: "Tên Mới", ExpectedName: "Tên Chủ",
		Branding:   domain.Branding{Brief: "b", Summary: "s", Look: "l", Avatar: "a", Banner: "n"},
		AvatarPath: "channels/x/1-avatar.png", BannerPath: "channels/x/1-banner.jpg",
	})
	if err != nil || !ok {
		t.Fatalf("publish = %v %v", ok, err)
	}
	view, _, _ := store.GetChannel(ctx, chOne)
	if view.Name != "Tên Mới" || view.AvatarPath == nil || view.IdentityState != domain.IdentityIdle {
		t.Fatalf("sau publish = %+v", view.Channel)
	}
	if live, _ := store.IdentityLeaseLive(ctx, chOne); live {
		t.Fatal("publish phải nhả lease")
	}
}

func TestChannelIdentityFailAndInterruptedRecovery(t *testing.T) {
	pool := provision(t)
	ctx := context.Background()
	seedFixtureTemplate(t, pool)
	store := postgres.NewChannelStore(pool)
	if _, err := store.CreateChannel(ctx, channels.CreateInput{
		ID: chOne, Name: "", Language: domain.LangAuto, AutoFetch: true, PerDay: 5, DefaultTemplateID: fixtureTemplate,
	}); err != nil {
		t.Fatal(err)
	}
	token := "55555555-5555-4555-8555-555555555555"
	if ok, err := store.AcquireIdentityLease(ctx, channels.LeaseInput{
		ChannelID: chOne, Part: domain.IdentityAvatar, Owner: "test", Token: token,
		Until: time.Now().Add(time.Minute),
	}); err != nil || !ok {
		t.Fatalf("acquire = %v %v", ok, err)
	}
	retry := time.Now().Add(2 * time.Hour).UTC().Truncate(time.Second)
	if ok, err := store.FailIdentity(ctx, channels.FailInput{
		ChannelID: chOne, Token: token, Code: "quota", Message: "Hết hạn mức.", RetryAt: &retry,
		Step: domain.IdentityStepImages,
	}); err != nil || !ok {
		t.Fatalf("fail = %v %v", ok, err)
	}
	view, _, _ := store.GetChannel(ctx, chOne)
	if view.IdentityState != domain.IdentityFailed || view.IdentityErrorCode == nil || *view.IdentityErrorCode != "quota" {
		t.Fatalf("identity = %+v", view.Channel)
	}
	if view.IdentityRetryAt == nil || !view.IdentityRetryAt.Equal(retry) {
		t.Fatalf("retryAt = %v, muốn %v", view.IdentityRetryAt, retry)
	}
	if view.IdentityFailedStep == nil || *view.IdentityFailedStep != domain.IdentityStepImages {
		t.Fatalf("failed step = %v, muốn images", view.IdentityFailedStep)
	}
	// Fail nhả lease: run sau giành lại được ngay.
	if ok, err := store.AcquireIdentityLease(ctx, channels.LeaseInput{
		ChannelID: chOne, Part: domain.IdentityAll, Owner: "test",
		Token: "66666666-6666-4666-8666-666666666666", Until: time.Now().Add(time.Minute),
	}); err != nil || !ok {
		t.Fatalf("acquire sau fail = %v %v", ok, err)
	}

	// Lease hết hạn (run chết): phục hồi đánh dấu interrupted, không tự vẽ lại.
	if _, err := pool.Exec(ctx, `UPDATE owner_channels SET identity_lease_until = now() - interval '1 minute' WHERE id = $1`, chOne); err != nil {
		t.Fatal(err)
	}
	ids, err := store.MarkInterruptedIdentities(ctx)
	if err != nil || len(ids) != 1 || ids[0] != chOne {
		t.Fatalf("mark interrupted = %v %v", ids, err)
	}
	view, _, _ = store.GetChannel(ctx, chOne)
	if view.IdentityState != domain.IdentityFailed || *view.IdentityErrorCode != domain.IdentityErrInterrupted {
		t.Fatalf("identity = %+v", view.Channel)
	}
	// Run mới xoá bước hỏng cũ; run chết giữa chừng không rõ bước.
	if view.IdentityFailedStep != nil {
		t.Fatalf("failed step = %q, muốn nil sau interrupted", *view.IdentityFailedStep)
	}
}

func TestChannelPatchFieldsAndBrief(t *testing.T) {
	pool := provision(t)
	ctx := context.Background()
	seedFixtureTemplate(t, pool)
	store := postgres.NewChannelStore(pool)
	if _, err := store.CreateChannel(ctx, channels.CreateInput{
		ID: chOne, Name: "Tên Cũ", Language: domain.LangAuto, AutoFetch: true, PerDay: 5, DefaultTemplateID: fixtureTemplate,
	}); err != nil {
		t.Fatal(err)
	}
	style := "kể chậm"
	footer := "Nguồn: Viderse"
	url := "https://www.youtube.com/@kenh"
	ok, err := store.PatchChannel(ctx, chOne, channels.Patch{
		Name:              channels.Field[string]{Set: true, Value: "Tên Mới"},
		YoutubeURL:        channels.Field[*string]{Set: true, Value: &url},
		Language:          channels.Field[string]{Set: true, Value: domain.LangVI},
		StyleNote:         channels.Field[*string]{Set: true, Value: &style},
		DescriptionFooter: channels.Field[*string]{Set: true, Value: &footer},
		AlwaysTags:        channels.Field[[]string]{Set: true, Value: []string{"ma", "kinh dị"}},
		AutoFetch:         channels.Field[bool]{Set: true, Value: false},
		PerDay:            channels.Field[int]{Set: true, Value: 9},
		Brief:             channels.Field[string]{Set: true, Value: "mô tả nhận diện"},
	})
	if err != nil || !ok {
		t.Fatalf("PatchChannel = %v %v", ok, err)
	}
	view, _, err := store.GetChannel(ctx, chOne)
	if err != nil {
		t.Fatal(err)
	}
	c := view.Channel
	if c.Name != "Tên Mới" || c.Language != domain.LangVI || c.AutoFetch || c.PerDay != 9 {
		t.Fatalf("trường cơ bản = %+v", c)
	}
	if c.YoutubeURL == nil || *c.YoutubeURL != url || c.StyleNote == nil || *c.StyleNote != style ||
		c.DescriptionFooter == nil || *c.DescriptionFooter != footer {
		t.Fatalf("trường chuỗi = %+v", c)
	}
	if len(c.AlwaysTags) != 2 || c.Branding.Brief != "mô tả nhận diện" {
		t.Fatalf("tags/brief = %+v / %+v", c.AlwaysTags, c.Branding)
	}

	// Patch chỉ brief không được xoá các trường khác hay branding còn lại.
	c.Branding.Look = "look giữ nguyên"
	if _, err := pool.Exec(ctx, `UPDATE owner_channels SET branding = $2 WHERE id = $1`, chOne,
		`{"brief":"mô tả nhận diện","look":"look giữ nguyên"}`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.PatchChannel(ctx, chOne, channels.Patch{
		Brief: channels.Field[string]{Set: true, Value: "mô tả mới"},
	}); err != nil {
		t.Fatal(err)
	}
	view, _, _ = store.GetChannel(ctx, chOne)
	if view.Name != "Tên Mới" || view.Branding.Look != "look giữ nguyên" || view.Branding.Brief != "mô tả mới" {
		t.Fatalf("patch brief làm đổi trường khác: %+v / %+v", view.Channel, view.Branding)
	}
}

// TestChannelConcurrentUnlinkPausesOnce: hai bỏ liên kết đối thủ cuối chạy đồng
// thời — khoá hàng kênh serialize, đúng một lệnh thấy "đã tạm dừng Tự lấy".
func TestChannelConcurrentUnlinkPausesOnce(t *testing.T) {
	pool := provision(t)
	ctx := context.Background()
	seedChannelRival(t, pool, rivalA, "A", nil)
	seedFixtureTemplate(t, pool)
	store := postgres.NewChannelStore(pool)
	if _, err := store.CreateChannel(ctx, channels.CreateInput{
		ID: chOne, Name: "Kênh", Language: domain.LangVI, AutoFetch: true, PerDay: 5, DefaultTemplateID: fixtureTemplate,
		CompetitorIDs: []string{rivalA},
	}); err != nil {
		t.Fatal(err)
	}
	pauses := make(chan bool, 2)
	errs := make(chan error, 2)
	for range 2 {
		go func() {
			_, paused, err := store.UnlinkSource(ctx, chOne, rivalA)
			pauses <- paused
			errs <- err
		}()
	}
	pausedA, pausedB := <-pauses, <-pauses
	if err := <-errs; err != nil {
		t.Fatal(err)
	}
	_ = <-errs
	if pausedA == pausedB {
		t.Fatalf("paused = %v %v, muốn đúng một lệnh true", pausedA, pausedB)
	}
	view, _, _ := store.GetChannel(ctx, chOne)
	if view.AutoFetch || len(view.Sources) != 0 {
		t.Fatalf("sau đua: autopick=%v sources=%d", view.AutoFetch, len(view.Sources))
	}
}
