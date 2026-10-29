package auth

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type fakeProvider struct {
	refreshes int
	next      Credential
	err       error
}

func (f *fakeProvider) ID() string   { return "fake" }
func (f *fakeProvider) Name() string { return "Fake" }
func (f *fakeProvider) Login(context.Context, Controller, LoginOptions) (Credential, error) {
	return Credential{}, errors.New("unused")
}
func (f *fakeProvider) Refresh(_ context.Context, cred Credential) (Credential, error) {
	f.refreshes++
	if f.err != nil {
		return Credential{}, f.err
	}
	return f.next, nil
}

func newTestManager(t *testing.T, provider *fakeProvider) (*Manager, *Store, time.Time) {
	t.Helper()
	store := NewStore(filepath.Join(t.TempDir(), "auth.json"))
	now := time.Unix(1_700_000_000, 0)
	m := NewManager(store)
	m.now = func() time.Time { return now }
	m.lookup = func(id string) (Provider, bool) { return provider, id == "fake" }
	return m, store, now
}

func TestManagerReturnsFreshCredentialWithoutRefresh(t *testing.T) {
	provider := &fakeProvider{}
	m, store, now := newTestManager(t, provider)
	cred := Credential{Access: "a1", Refresh: "r1", Expires: now.Add(time.Hour).UnixMilli()}
	if err := store.Put("fake", cred); err != nil {
		t.Fatal(err)
	}
	got, err := m.Credential(context.Background(), "fake")
	if err != nil || got != cred || provider.refreshes != 0 {
		t.Fatalf("got=%+v err=%v refreshes=%d", got, err, provider.refreshes)
	}
}

func TestManagerRefreshesInsideSkewAndPersistsRotation(t *testing.T) {
	provider := &fakeProvider{}
	m, store, now := newTestManager(t, provider)
	// Còn 1 phút < skew 5 phút: phải refresh trước khi request dài chạy.
	_ = store.Put("fake", Credential{Access: "a1", Refresh: "r1", Expires: now.Add(time.Minute).UnixMilli()})
	provider.next = Credential{Access: "a2", Refresh: "r2", Expires: now.Add(time.Hour).UnixMilli()}

	got, err := m.Credential(context.Background(), "fake")
	if err != nil || got.Access != "a2" || provider.refreshes != 1 {
		t.Fatalf("got=%+v err=%v refreshes=%d", got, err, provider.refreshes)
	}
	stored, _, _ := store.Get("fake")
	if stored.Refresh != "r2" {
		t.Fatalf("refresh token xoay vòng phải được lưu, kho có %q", stored.Refresh)
	}
}

func TestManagerInvalidateSkipsRefreshWhenAnotherProcessRotated(t *testing.T) {
	provider := &fakeProvider{}
	m, store, now := newTestManager(t, provider)
	// Tiến trình khác đã refresh: token trong kho khác token stale vừa bị 401.
	current := Credential{Access: "a-new", Refresh: "r-new", Expires: now.Add(time.Hour).UnixMilli()}
	_ = store.Put("fake", current)

	got, err := m.Invalidate(context.Background(), "fake", "a-old")
	if err != nil || got != current || provider.refreshes != 0 {
		t.Fatalf("got=%+v err=%v refreshes=%d (refresh lại sẽ dùng token đã bị thu hồi)", got, err, provider.refreshes)
	}

	provider.next = Credential{Access: "a3", Refresh: "r3", Expires: now.Add(time.Hour).UnixMilli()}
	got, err = m.Invalidate(context.Background(), "fake", "a-new")
	if err != nil || got.Access != "a3" || provider.refreshes != 1 {
		t.Fatalf("token stale trùng kho phải refresh: got=%+v err=%v refreshes=%d", got, err, provider.refreshes)
	}
}

func TestManagerRefreshFailureKeepsStoredCredential(t *testing.T) {
	provider := &fakeProvider{err: errors.New("invalid_grant")}
	m, store, now := newTestManager(t, provider)
	cred := Credential{Access: "a1", Refresh: "r1", Expires: now.Add(-time.Minute).UnixMilli()}
	_ = store.Put("fake", cred)

	if _, err := m.Credential(context.Background(), "fake"); err == nil {
		t.Fatal("refresh lỗi phải được trả về")
	}
	stored, ok, _ := store.Get("fake")
	if !ok || stored != cred {
		t.Fatalf("refresh lỗi không được xoá/ghi đè credential: ok=%v stored=%+v", ok, stored)
	}
}

func TestManagerNotLoggedIn(t *testing.T) {
	m, _, _ := newTestManager(t, &fakeProvider{})
	if _, err := m.Credential(context.Background(), "fake"); !errors.Is(err, ErrNotLoggedIn) {
		t.Fatalf("err = %v, want ErrNotLoggedIn", err)
	}
}

func TestStoreWritesOwnerOnlyAndDeletes(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "nested", "auth.json"))
	if err := store.Put("a", Credential{Access: "x"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Put("b", Credential{Access: "y"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(store.Path())
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("quyền file credential = %o, want 600", perm)
	}
	if existed, err := store.Delete("a"); err != nil || !existed {
		t.Fatalf("Delete a: existed=%v err=%v", existed, err)
	}
	if _, ok, _ := store.Get("a"); ok {
		t.Fatal("a vẫn còn sau Delete")
	}
	if got, ok, _ := store.Get("b"); !ok || got.Access != "y" {
		t.Fatalf("Delete a làm mất b: ok=%v got=%+v", ok, got)
	}
}
