package web

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/activity"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/migrations"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/platform/identity"
	"github.com/datnt1998/verse/platform/pgmigrate"
	"github.com/datnt1998/verse/platform/pgtest"
)

func TestActivityHTTPScopesMemberStorageDataAndAcknowledgements(t *testing.T) {
	ctx := context.Background()
	p := newPGGateway(t)
	if _, err := p.fx.Admin.CreateUser(identity.NewUser{Username: "admin", Password: "admin-pass", Role: identity.RoleAdmin}); err != nil {
		t.Fatal(err)
	}
	p.fx.CreateMember(t, "member", "member-pass", false, identity.AppViderse)
	dsn := pgtest.Provision(t)
	db, err := pgmigrate.OpenDB(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pgmigrate.Up(ctx, db, migrations.FS); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	pool, err := postgres.OpenPool(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	svc := activity.New(postgres.NewActivityStore(pool), nil)
	MountActivity(p.g, svc)
	privateID, err := svc.NotifyScoped(ctx, activity.NotificationSpec{
		Kind: "needs_attention", Text: "private storage failure", Audience: activity.AudienceAdmin,
		Object: &domain.ObjectRef{Type: "storage", ID: "active"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Notify(ctx, "action_result", "public notice", "#/library", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Start(ctx, domain.TaskSpec{Kind: "storage_migrate", Title: "private migration", Object: domain.ObjectRef{Type: "storage", ID: "migration"}}); err != nil {
		t.Fatal(err)
	}
	memberCookie := sessionCookieOf(t, p.login("member", "member-pass"))
	adminCookie := sessionCookieOf(t, p.login("admin", "admin-pass"))
	for _, path := range []string{"/api/activity", "/api/errors"} {
		member := p.do(http.MethodGet, path, memberCookie, "")
		if member.Code != http.StatusOK || strings.Contains(member.Body.String(), "private") || !strings.Contains(member.Body.String(), "public notice") {
			t.Fatalf("member storage leak at %s: %d %s", path, member.Code, member.Body)
		}
		if member.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatalf("shared activity cache at %s", path)
		}
		admin := p.do(http.MethodGet, path, adminCookie, "")
		if admin.Code != http.StatusOK || !strings.Contains(admin.Body.String(), "private") {
			t.Fatalf("admin lost storage data: %d %s", admin.Code, admin.Body)
		}
	}
	for _, body := range []string{`{"ids":["` + privateID + `"]}`, `{"all":true}`} {
		read := p.do(http.MethodPost, "/api/notifications/read", memberCookie, body)
		if read.Code != http.StatusOK {
			t.Fatalf("read: %d %s", read.Code, read.Body)
		}
	}
	admin := p.do(http.MethodGet, "/api/activity", adminCookie, "")
	if admin.Code != http.StatusOK || !strings.Contains(admin.Body.String(), privateID) {
		t.Fatalf("member consumed admin notification: %d %s", admin.Code, admin.Body)
	}
}
