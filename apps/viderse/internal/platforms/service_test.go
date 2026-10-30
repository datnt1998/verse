// Ca test cho mặt nghiệp vụ của mảng nền tảng: registry, suy trạng thái hiển
// thị (§24), mục cần xử lý cho chấm Cài đặt, và việc kiểm nền tảng trước khi
// gọi thao tác đăng nhập (§24.1).
package platforms_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/platforms"
)

func TestRegistryHasContractPlatforms(t *testing.T) {
	list := platforms.Registry()
	if len(list) != 3 {
		t.Fatalf("registry = %d nền tảng, muốn 3", len(list))
	}
	want := map[domain.PlatformID]struct {
		label  string
		height int
		login  domain.PlatformLoginType
	}{
		domain.PlatformYouTube:  {"YouTube", 360, domain.LoginBrowserCookie},
		domain.PlatformTwitch:   {"Twitch", 480, domain.LoginNone},
		domain.PlatformBilibili: {"Bilibili", 1080, domain.LoginQR},
	}
	for _, d := range list {
		w, ok := want[d.ID]
		if !ok {
			t.Fatalf("nền tảng lạ %q", d.ID)
		}
		if d.Label != w.label || d.TargetHeight != w.height || d.Login != w.login {
			t.Fatalf("%s = %+v", d.ID, d)
		}
	}
	// Registry trả bản sao: caller sửa không ảnh hưởng lần gọi sau.
	list[0].Label = "đổi"
	if platforms.Registry()[0].Label == "đổi" {
		t.Fatal("Registry phải trả bản sao")
	}
	if _, ok := platforms.Lookup("douyin"); ok {
		t.Fatal("Lookup nền tảng lạ phải false")
	}
}

// stubLogins ghi lại lời gọi và trả hình đăng nhập theo nền tảng.
type stubLogins struct {
	login      domain.PlatformLogin
	lastID     domain.PlatformID
	lastCookie string
	lastPause  int
}

func (s *stubLogins) Login(context.Context, domain.PlatformID) (domain.PlatformLogin, error) {
	return s.login, nil
}
func (s *stubLogins) Profiles(context.Context, bool) ([]domain.BrowserProfile, error) {
	return []domain.BrowserProfile{{Browser: "chrome", Profile: "Default", Name: "Cá nhân"}}, nil
}
func (s *stubLogins) SetCookieSource(_ context.Context, browser, profile string) (domain.PlatformLogin, error) {
	s.lastCookie = browser + ":" + profile
	return s.login, nil
}
func (s *stubLogins) SaveManualCookie(_ context.Context, cookie string) (domain.PlatformLogin, error) {
	s.lastCookie = cookie
	return s.login, nil
}
func (s *stubLogins) ClearLogin(_ context.Context, id domain.PlatformID) (domain.PlatformLogin, error) {
	s.lastID = id
	return domain.PlatformLogin{}, nil
}
func (s *stubLogins) StartQR(context.Context) (domain.QRSession, error) {
	return domain.QRSession{QRCodeKey: "k"}, nil
}
func (s *stubLogins) PollQR(context.Context, string) (domain.QRPoll, error) {
	return domain.QRPoll{Status: domain.QRSuccess}, nil
}
func (s *stubLogins) Refresh(context.Context) (domain.RefreshOutcome, domain.PlatformLogin, error) {
	return domain.RefreshRefreshed, s.login, nil
}
func (s *stubLogins) Check(_ context.Context, id domain.PlatformID) (domain.PlatformLogin, error) {
	s.lastID = id
	return s.login, nil
}

func newService(t *testing.T, store *memStore, logins platforms.LoginActions) *platforms.Service {
	t.Helper()
	return platforms.New(platforms.Deps{
		Store:  store,
		Logins: logins,
		Now:    func() time.Time { return time.Date(2026, 10, 30, 3, 0, 0, 0, time.UTC) },
	})
}

func TestStatusMapsLoginStateAndRegistry(t *testing.T) {
	store := newMemStore()
	until := time.Date(2026, 10, 30, 4, 10, 0, 0, time.UTC)
	reason := "Bilibili đang giới hạn yêu cầu"
	store.states[domain.PlatformBilibili] = domain.PlatformState{
		Platform: domain.PlatformBilibili, PausedUntil: &until, PauseReason: &reason, FailureCount: 2,
	}
	refreshErr := "Đăng nhập Bilibili đã hết hạn — quét mã QR lại"
	svc := newService(t, store, &stubLogins{login: domain.PlatformLogin{Saved: true, Method: "qr", RefreshError: &refreshErr}})
	list, err := svc.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 {
		t.Fatalf("status = %d", len(list))
	}
	for _, st := range list {
		if st.Login.Type == "" {
			t.Fatalf("%s thiếu login.type từ registry", st.ID)
		}
		switch st.ID {
		case domain.PlatformBilibili:
			if st.State != domain.PlatformPaused || st.FailureCount != 2 || st.PausedUntil == nil {
				t.Fatalf("bilibili = %+v", st)
			}
		default:
			if st.State != domain.PlatformNeedsLogin {
				// refreshError áp cho mọi nền tảng trong stub; chỉ cần một
				// nền tảng khác cũng cần đăng nhập lại là đủ kiểm mapping.
				t.Fatalf("%s state = %q", st.ID, st.State)
			}
		}
	}
}

func TestStatusNeedsLoginOnYouTubeCookieFailure(t *testing.T) {
	store := newMemStore()
	at := time.Date(2026, 10, 30, 2, 0, 0, 0, time.UTC)
	job := "job-1"
	store.states[domain.PlatformYouTube] = domain.PlatformState{
		Platform: domain.PlatformYouTube, LastErrorCode: "cookies_failed", LastErrorAt: &at, LastErrorJobID: &job,
	}
	svc := newService(t, store, &stubLogins{})
	list, err := svc.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, st := range list {
		if st.ID != domain.PlatformYouTube {
			continue
		}
		found = true
		if st.State != domain.PlatformNeedsLogin {
			t.Fatalf("state = %q", st.State)
		}
		if st.LastError == nil || st.LastError.Code != "cookies_failed" || st.LastError.JobID == nil || *st.LastError.JobID != "job-1" {
			t.Fatalf("lastError = %+v", st.LastError)
		}
	}
	if !found {
		t.Fatal("thiếu youtube")
	}
}

func TestAttentionOnlyForExpiredLogin(t *testing.T) {
	store := newMemStore()
	refreshErr := "Đăng nhập Bilibili đã hết hạn — quét mã QR lại"
	svc := newService(t, store, &stubLogins{login: domain.PlatformLogin{RefreshError: &refreshErr}})
	items, err := svc.Attention(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) == 0 {
		t.Fatal("đăng nhập hết hạn phải có mục cần xử lý")
	}
	for _, it := range items {
		if it.Kind != domain.AttentionLoginExpired {
			t.Fatalf("kind = %q", it.Kind)
		}
		if it.Href != "#/settings?section=accounts" {
			t.Fatalf("href = %q", it.Href)
		}
	}
	clean := newService(t, newMemStore(), &stubLogins{})
	items, err = clean.Attention(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("không có sự cố thì không có mục, được %+v", items)
	}
}

func TestLoginDispatchByRegistry(t *testing.T) {
	ctx := context.Background()
	store := newMemStore()
	logins := &stubLogins{}
	svc := newService(t, store, logins)

	if _, err := svc.SetLogin(ctx, domain.PlatformYouTube, platforms.LoginInput{Cookie: "x"}); !errors.Is(err, platforms.ErrInvalidLoginBody) {
		t.Fatalf("youtube nhận cookie phải invalid, được %v", err)
	}
	if _, err := svc.SetLogin(ctx, domain.PlatformYouTube, platforms.LoginInput{Browser: "chrome", Profile: "Default"}); err != nil {
		t.Fatalf("youtube hồ sơ: %v", err)
	}
	if logins.lastCookie != "chrome:Default" {
		t.Fatalf("hồ sơ = %q", logins.lastCookie)
	}
	if _, err := svc.SetLogin(ctx, domain.PlatformBilibili, platforms.LoginInput{Cookie: "SESSDATA=x"}); err != nil {
		t.Fatalf("bilibili cookie: %v", err)
	}
	if _, err := svc.SetLogin(ctx, domain.PlatformTwitch, platforms.LoginInput{Cookie: "x"}); !errors.Is(err, platforms.ErrLoginUnsupported) {
		t.Fatalf("twitch phải unsupported, được %v", err)
	}
	if _, err := svc.SetLogin(ctx, "douyin", platforms.LoginInput{}); !errors.Is(err, platforms.ErrPlatformUnknown) {
		t.Fatalf("nền tảng lạ phải unknown, được %v", err)
	}
	if _, err := svc.StartQR(ctx, domain.PlatformYouTube); !errors.Is(err, platforms.ErrLoginUnsupported) {
		t.Fatalf("QR youtube phải unsupported, được %v", err)
	}
	if _, _, err := svc.Refresh(ctx, domain.PlatformTwitch); !errors.Is(err, platforms.ErrLoginUnsupported) {
		t.Fatalf("refresh twitch phải unsupported, được %v", err)
	}
	if _, err := svc.Profiles(ctx, domain.PlatformBilibili, false); !errors.Is(err, platforms.ErrLoginUnsupported) {
		t.Fatalf("hồ sơ bilibili phải unsupported, được %v", err)
	}
}

// loginRejected là lỗi "nền tảng từ chối" để kiểm ghi lastError YouTube.
type loginRejected struct{}

func (loginRejected) Error() string       { return "bị từ chối" }
func (loginRejected) LoginReason() string { return "rejected" }

type rejectLogins struct{ *stubLogins }

func (rejectLogins) Check(context.Context, domain.PlatformID) (domain.PlatformLogin, error) {
	return domain.PlatformLogin{}, loginRejected{}
}

func TestYouTubeRejectionRecordsLastError(t *testing.T) {
	store := newMemStore()
	svc := newService(t, store, rejectLogins{&stubLogins{}})
	if _, err := svc.Check(context.Background(), domain.PlatformYouTube); err == nil {
		t.Fatal("muốn lỗi")
	}
	st, _, _ := store.One(context.Background(), domain.PlatformYouTube)
	if st.LastErrorCode != "cookies_failed" || st.LastErrorAt == nil {
		t.Fatalf("lastError = %+v", st)
	}
	// Kiểm tra thành công phải xoá lỗi.
	svc = newService(t, store, &stubLogins{})
	if _, err := svc.Check(context.Background(), domain.PlatformYouTube); err != nil {
		t.Fatal(err)
	}
	st, _, _ = store.One(context.Background(), domain.PlatformYouTube)
	if st.LastErrorCode != "" {
		t.Fatalf("lastError = %q, muốn rỗng", st.LastErrorCode)
	}
}

func TestSuccessfulBilibiliLoginClearsPause(t *testing.T) {
	store := newMemStore()
	until := time.Date(2026, 10, 30, 5, 0, 0, 0, time.UTC)
	store.states[domain.PlatformBilibili] = domain.PlatformState{
		Platform: domain.PlatformBilibili, PausedUntil: &until, FailureCount: 3,
	}
	svc := newService(t, store, &stubLogins{login: domain.PlatformLogin{Saved: true, Method: "qr"}})
	if _, err := svc.PollQR(context.Background(), domain.PlatformBilibili, "key"); err != nil {
		t.Fatal(err)
	}
	if st, _, _ := store.One(context.Background(), domain.PlatformBilibili); st.PausedUntil != nil || st.FailureCount != 0 {
		t.Fatalf("đăng nhập xong phải xoá giờ chờ, được %+v", st)
	}
}

func TestPauseMinutesEscalatesToCap(t *testing.T) {
	want := []float64{15, 30, 60, 120, 120, 120}
	for i, w := range want {
		if got := platforms.PauseMinutes(i + 1); got != w {
			t.Fatalf("PauseMinutes(%d) = %v, muốn %v", i+1, got, w)
		}
	}
	if got := platforms.PauseMinutes(0); got != 15 {
		t.Fatalf("PauseMinutes(0) = %v", got)
	}
}

// countingLogins đếm từng mặt của accounts để chắc GET không chạm mạng/scan.
type countingLogins struct {
	stubLogins
	profiles  int
	checks    int
	refreshes int
	qrs       int
}

func (c *countingLogins) Profiles(context.Context, bool) ([]domain.BrowserProfile, error) {
	c.profiles++
	return nil, nil
}
func (c *countingLogins) Check(context.Context, domain.PlatformID) (domain.PlatformLogin, error) {
	c.checks++
	return domain.PlatformLogin{}, nil
}
func (c *countingLogins) Refresh(context.Context) (domain.RefreshOutcome, domain.PlatformLogin, error) {
	c.refreshes++
	return "", domain.PlatformLogin{}, nil
}
func (c *countingLogins) StartQR(context.Context) (domain.QRSession, error) {
	c.qrs++
	return domain.QRSession{}, nil
}

// storeCalls đếm truy vấn để chốt luật "một lượt đọc trạng thái, không N+1".
type storeCalls struct {
	*memStore
	all int
}

func (s *storeCalls) All(ctx context.Context) ([]domain.PlatformState, error) {
	s.all++
	return s.memStore.All(ctx)
}

func TestStatusDoesOneStateQueryAndNoNetworkOrProfileScan(t *testing.T) {
	store := &storeCalls{memStore: newMemStore()}
	logins := &countingLogins{}
	svc := platforms.New(platforms.Deps{
		Store:  store,
		Logins: logins,
		Now:    func() time.Time { return time.Date(2026, 10, 30, 3, 0, 0, 0, time.UTC) },
	})
	for range 20 {
		if _, err := svc.Status(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if store.all != 20 {
		t.Fatalf("All = %d, muốn 20 (một truy vấn mỗi lượt)", store.all)
	}
	if logins.profiles != 0 || logins.checks != 0 || logins.refreshes != 0 || logins.qrs != 0 {
		t.Fatalf("GET không được gọi mạng/scan: profiles=%d checks=%d refreshes=%d qr=%d",
			logins.profiles, logins.checks, logins.refreshes, logins.qrs)
	}
}

// TestStatusNormalizesExpiredPause kiểm giờ chờ đã qua không còn hiện trên
// GET: nền tảng hiện "cần đăng nhập" (nếu refreshError) hoặc "sẵn sàng" ngay,
// không đợi một lượt quét nào dọn trường.
func TestStatusNormalizesExpiredPause(t *testing.T) {
	ctx := context.Background()
	deadline := time.Date(2026, 10, 30, 4, 0, 0, 0, time.UTC)
	refreshErr := "Đăng nhập Bilibili đã hết hạn — quét mã QR lại"
	reason := "Bilibili đang giới hạn yêu cầu"

	for _, tc := range []struct {
		name      string
		now       time.Time
		wantState domain.PlatformCondition
	}{
		{"before-deadline", deadline.Add(-time.Minute), domain.PlatformPaused},
		{"at-deadline", deadline, domain.PlatformNeedsLogin},
		{"after-deadline", deadline.Add(time.Minute), domain.PlatformNeedsLogin},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := newMemStore()
			until := deadline
			store.states[domain.PlatformBilibili] = domain.PlatformState{
				Platform: domain.PlatformBilibili, PausedUntil: &until, PauseReason: &reason, FailureCount: 3,
			}
			svc := platforms.New(platforms.Deps{
				Store:  store,
				Logins: &stubLogins{login: domain.PlatformLogin{RefreshError: &refreshErr}},
				Now:    func() time.Time { return tc.now },
			})
			list, err := svc.Status(ctx)
			if err != nil {
				t.Fatal(err)
			}
			for _, st := range list {
				if st.ID != domain.PlatformBilibili {
					continue
				}
				if st.State != tc.wantState {
					t.Fatalf("state = %q, muốn %q", st.State, tc.wantState)
				}
				if tc.wantState == domain.PlatformPaused {
					if st.PausedUntil == nil || st.PauseReason == nil {
						t.Fatalf("chưa hết hạn phải còn pause: %+v", st)
					}
				} else {
					if st.PausedUntil != nil || st.PauseReason != nil {
						t.Fatalf("hết hạn phải dọn: %+v", st)
					}
				}
			}
			// Read model không ghi ngược: failureCount vẫn còn trong store.
			if st, _, _ := store.One(ctx, domain.PlatformBilibili); st.FailureCount != 3 {
				t.Fatalf("failureCount = %d", st.FailureCount)
			}
		})
	}
}
