// Package notify persists the per-user inbox and delivers encrypted Web Push.
package notify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/gofrs/flock"
)

var ErrConflict = errors.New("Đã có thay đổi từ nơi khác")
var ErrInvalid = errors.New("Thiết lập thông báo không hợp lệ")

type Types struct {
	Gate     bool `json:"gate"`
	Error    bool `json:"error"`
	Job      bool `json:"job"`
	Done     bool `json:"done"`
	Security bool `json:"security"`
}
type Quiet struct {
	From string `json:"from"`
	To   string `json:"to"`
}
type Prefs struct {
	Enabled bool   `json:"enabled"`
	Types   Types  `json:"types"`
	Digest  string `json:"digest"`
	Quiet   Quiet  `json:"quiet"`
	Version string `json:"version"`
}
type Keys struct {
	P256dh string `json:"p256dh"`
	Auth   string `json:"auth"`
}
type Subscription struct {
	Endpoint  string    `json:"endpoint"`
	Keys      Keys      `json:"keys"`
	Device    string    `json:"device"`
	Session   string    `json:"session"`
	CreatedAt time.Time `json:"createdAt"`
}
type Action struct {
	Action string `json:"action"`
	Title  string `json:"title"`
	URL    string `json:"url,omitempty"`
}
type PushNotification struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Story     string    `json:"story"`
	Chapter   int       `json:"chapter,omitempty"`
	Job       string    `json:"job,omitempty"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	URL       string    `json:"url"`
	Actions   []Action  `json:"actions"`
	CreatedAt time.Time `json:"createdAt"`
	Read      bool      `json:"read"`
	Session   string    `json:"-"`
	Score     *float64  `json:"score,omitempty"`
}

// Session is private in push/inbox payloads but persisted for security actions.
type storedNotification struct {
	PushNotification
	TargetSession string `json:"targetSession,omitempty"`
}
type digest struct {
	Items []PushNotification `json:"items"`
	Due   time.Time          `json:"due"`
}
type userData struct {
	Prefs         Prefs                `json:"prefs"`
	Subscriptions []Subscription       `json:"subscriptions"`
	Inbox         []storedNotification `json:"inbox"`
	Digests       map[string]digest    `json:"digests"`
}
type disk struct {
	Users    map[string]*userData `json:"users"`
	Sequence uint64               `json:"sequence"`
}
type vapid struct {
	Public  string `json:"publicKey"`
	Private string `json:"privateKey"`
}
type delivery struct {
	User string
	Item PushNotification
}
type Options struct {
	// Directory defaults to ~/.ainovel/push. Clock defaults to time.Now.
	Directory  string
	Clock      func() time.Time
	HTTPClient *http.Client
	// Allowed is checked both when recording and immediately before delivery.
	Allowed       func(user, story string) bool
	SessionActive func(user, session string) bool
	Changed       func(user string, unread int)
}
type Store struct {
	mu     sync.Mutex
	closed bool
	dir    string
	keys   vapid
	data   disk
	opts   Options
	queue  chan delivery
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func Open(opts Options) (*Store, error) {
	if opts.Directory == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		opts.Directory = filepath.Join(home, ".ainovel", "push")
	}
	if opts.Clock == nil {
		opts.Clock = time.Now
	}
	if opts.HTTPClient == nil {
		opts.HTTPClient = &http.Client{Timeout: 15 * time.Second}
	}
	if err := os.MkdirAll(opts.Directory, 0700); err != nil {
		return nil, err
	}
	keyLock := flock.New(filepath.Join(opts.Directory, "vapid.lock"))
	if err := keyLock.Lock(); err != nil {
		return nil, err
	}
	defer keyLock.Unlock()
	s := &Store{dir: opts.Directory, opts: opts, queue: make(chan delivery, 128), data: disk{Users: map[string]*userData{}}}
	if err := load(filepath.Join(s.dir, "vapid.json"), &s.keys); errors.Is(err, os.ErrNotExist) {
		private, public, e := webpush.GenerateVAPIDKeys()
		if e != nil {
			return nil, e
		}
		s.keys = vapid{Public: public, Private: private}
		// Exclusive creation ensures concurrent gateway starts never replace a key.
		f, e := os.OpenFile(filepath.Join(s.dir, "vapid.json"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if errors.Is(e, os.ErrExist) {
			if e = load(filepath.Join(s.dir, "vapid.json"), &s.keys); e != nil {
				return nil, e
			}
		} else if e != nil {
			return nil, e
		} else {
			e = json.NewEncoder(f).Encode(s.keys)
			ce := f.Close()
			if e != nil {
				return nil, e
			}
			if ce != nil {
				return nil, ce
			}
		}
	} else if err != nil {
		return nil, err
	}
	if s.keys.Public == "" || s.keys.Private == "" {
		return nil, errors.New("Khoá Web Push không hợp lệ")
	}
	if err := os.Chmod(filepath.Join(s.dir, "vapid.json"), 0600); err != nil {
		return nil, err
	}
	if err := load(filepath.Join(s.dir, "state.json"), &s.data); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if s.data.Users == nil {
		s.data.Users = map[string]*userData{}
	}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.wg.Add(1)
	go s.run()
	return s, nil
}
func load(path string, v any) error {
	b, e := os.ReadFile(path)
	if e != nil {
		return e
	}
	return json.Unmarshal(b, v)
}
func (s *Store) save() error {
	if s.closed {
		return errors.New("Kho thông báo đã đóng")
	}
	b, e := json.Marshal(s.data)
	if e != nil {
		return e
	}
	f, e := os.CreateTemp(s.dir, ".state-*")
	if e != nil {
		return e
	}
	name := f.Name()
	defer os.Remove(name)
	if e = f.Chmod(0600); e == nil {
		_, e = f.Write(b)
	}
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e != nil {
		return e
	}
	if ce != nil {
		return ce
	}
	return os.Rename(name, filepath.Join(s.dir, "state.json"))
}
func DefaultPrefs() Prefs {
	return Prefs{Enabled: true, Types: Types{Gate: true, Error: true, Job: true, Security: true}, Digest: "five", Quiet: Quiet{"22:00", "07:00"}, Version: "1"}
}
func (s *Store) user(name string) *userData {
	u := s.data.Users[name]
	if u == nil {
		u = &userData{Prefs: DefaultPrefs(), Subscriptions: []Subscription{}, Inbox: []storedNotification{}, Digests: map[string]digest{}}
		s.data.Users[name] = u
	}
	if u.Digests == nil {
		u.Digests = map[string]digest{}
	}
	return u
}
func (s *Store) PublicKey() string       { return s.keys.Public }
func (s *Store) Prefs(user string) Prefs { s.mu.Lock(); defer s.mu.Unlock(); return s.user(user).Prefs }
func minute(v string) (int, bool) {
	t, e := time.Parse("15:04", v)
	return t.Hour()*60 + t.Minute(), e == nil
}

// PutPrefs lưu cài đặt. Chuyển sang “mỗi chương” thì các chương đang chờ gộp được
// trả ngay thành một mục hộp thư, không để kẹt trong nhóm không còn ai gom.
func (s *Store) PutPrefs(user string, p Prefs, match string) (Prefs, error) {
	saved, e := s.putPrefs(user, p, match)
	if e == nil && saved.Digest == "each" {
		e = s.flushUser(user)
	}
	return saved, e
}
func (s *Store) putPrefs(user string, p Prefs, match string) (Prefs, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.user(user)
	if match != "" && match != "*" && strings.Trim(match, "\"") != u.Prefs.Version {
		return u.Prefs, ErrConflict
	}
	if p.Digest != "each" && p.Digest != "five" && p.Digest != "day" {
		return u.Prefs, ErrInvalid
	}
	_, a := minute(p.Quiet.From)
	_, b := minute(p.Quiet.To)
	if !(p.Quiet.From == "" && p.Quiet.To == "") && (!a || !b) {
		return u.Prefs, ErrInvalid
	}
	p.Types.Security = true
	p.Types.Error = true
	v, _ := strconv.Atoi(u.Prefs.Version)
	p.Version = strconv.Itoa(v + 1)
	old := u.Prefs
	u.Prefs = p
	if e := s.save(); e != nil {
		u.Prefs = old
		return old, e
	}
	return p, nil
}
func permits(p Prefs, kind string, now time.Time) bool {
	if kind == "security" || kind == "error" {
		return true
	}
	if !p.Enabled {
		return false
	}
	on := false
	switch kind {
	case "gate":
		on = p.Types.Gate
	case "done":
		on = p.Types.Done
	case "job", "import", "sync", "cocreate":
		on = p.Types.Job
	}
	if !on {
		return false
	}
	from, a := minute(p.Quiet.From)
	to, b := minute(p.Quiet.To)
	if a && b && from != to {
		m := now.Hour()*60 + now.Minute()
		if from < to && m >= from && m < to || from > to && (m >= from || m < to) {
			return false
		}
	}
	return true
}
func (s *Store) Subscribe(user, session string, sub Subscription) error {
	u, e := url.Parse(sub.Endpoint)
	if e != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || sub.Keys.P256dh == "" || sub.Keys.Auth == "" {
		return ErrInvalid
	}
	s.mu.Lock()
	sub.Session = session
	sub.CreatedAt = s.opts.Clock()
	for _, owner := range s.data.Users {
		owner.Subscriptions = slices.DeleteFunc(owner.Subscriptions, func(v Subscription) bool { return v.Endpoint == sub.Endpoint })
	}
	owner := s.user(user)
	owner.Subscriptions = append(owner.Subscriptions, sub)
	e = s.save()
	count := s.unread(user, owner)
	s.mu.Unlock()
	if e == nil {
		s.changed(user, count)
	}
	return e
}
func (s *Store) Subscriptions(user string) []Subscription {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Subscription{}, s.user(user).Subscriptions...)
}
func (s *Store) Unsubscribe(user, endpoint string) error {
	s.mu.Lock()
	u := s.user(user)
	u.Subscriptions = slices.DeleteFunc(u.Subscriptions, func(v Subscription) bool { return v.Endpoint == endpoint })
	e := s.save()
	count := s.unread(user, u)
	s.mu.Unlock()
	if e == nil {
		s.changed(user, count)
	}
	return e
}
func (s *Store) Revoke(user, session string) error {
	s.mu.Lock()
	u := s.user(user)
	u.Subscriptions = slices.DeleteFunc(u.Subscriptions, func(v Subscription) bool { return v.Session == session })
	e := s.save()
	count := s.unread(user, u)
	s.mu.Unlock()
	if e == nil {
		s.changed(user, count)
	}
	return e
}
func (s *Store) unread(user string, u *userData) int {
	n := 0
	access := map[string]bool{}
	for _, v := range u.Inbox {
		if v.Read {
			continue
		}
		allowed, known := access[v.Story]
		if !known {
			allowed = s.allowed(user, v.Story)
			access[v.Story] = allowed
		}
		if allowed {
			n++
		}
	}
	return n
}
func (s *Store) allowed(user, story string) bool {
	return s.opts.Allowed == nil || s.opts.Allowed(user, story)
}
func (s *Store) Record(user string, n PushNotification) error {
	if !s.allowed(user, n.Story) {
		return nil
	}
	s.mu.Lock()
	u := s.user(user)
	now := s.opts.Clock()
	n.CreatedAt = now
	if s.duplicate(u, n, now) {
		s.mu.Unlock()
		return nil
	}
	if n.Actions == nil {
		n.Actions = []Action{}
	}
	if n.Type == "done" && u.Prefs.Digest != "each" {
		d := u.Digests[n.Story]
		d.Items = append(d.Items, n)
		if d.Due.IsZero() {
			d.Due = time.Date(now.Year(), now.Month(), now.Day(), 21, 0, 0, 0, now.Location())
			if !d.Due.After(now) {
				d.Due = d.Due.AddDate(0, 0, 1)
			}
		}
		u.Digests[n.Story] = d
		if u.Prefs.Digest == "day" || len(d.Items) < 5 {
			e := s.save()
			s.mu.Unlock()
			return e
		}
		n = group(d.Items)
		delete(u.Digests, n.Story)
	}
	s.append(u, &n)
	e := s.save()
	count := s.unread(user, u)
	send := permits(u.Prefs, n.Type, now)
	s.mu.Unlock()
	if e != nil {
		return e
	}
	s.changed(user, count)
	if send {
		s.enqueue(user, n)
	}
	return nil
}

// ScoreText hiển thị điểm 0–100 theo thang 10 kiểu Việt Nam (84 → “8,4”), cùng cách
// màn Đọc và Hoạt động đang dùng.
func ScoreText(score float64) string {
	return strings.Replace(strconv.FormatFloat(score/10, 'f', 1, 64), ".", ",", 1)
}
func group(items []PushNotification) PushNotification {
	if len(items) == 1 {
		return items[0]
	}
	n := items[0]
	last := items[len(items)-1]
	n.Title = fmt.Sprintf("Xong %d chương (%d–%d)", len(items), n.Chapter, last.Chapter)
	n.Body = fmt.Sprintf("Đã hoàn thành %d chương · %s", len(items), last.Body)
	var total float64
	count := 0
	for _, item := range items {
		if item.Score != nil {
			total += *item.Score
			count++
		}
	}
	if count > 0 {
		average := total / float64(count)
		n.Score = &average
		n.Body = "Điểm trung bình " + ScoreText(average)
	}
	return n
}

// dedupeWindow là khoảng chặn thông báo trùng (phát lại sự kiện sau khi worker khởi động lại).
const dedupeWindow = 10 * time.Minute

// dedupeKey nhận diện một sự kiện nghiệp vụ; rỗng = không chống trùng (thử, lỗi, thông báo không định danh).
func dedupeKey(n PushNotification, session string) string {
	switch n.Type {
	case "gate", "done":
		return fmt.Sprintf("%s|%s|%d", n.Type, n.Story, n.Chapter)
	case "job", "import", "sync", "cocreate":
		if n.Job != "" {
			return "job|" + n.Job
		}
	case "security":
		if session != "" {
			return "security|" + session
		}
	}
	return ""
}

// duplicate: cùng sự kiện đã nằm trong nhóm chờ gộp, đã nằm trong hộp thư trong cửa sổ chống trùng,
// hoặc (cổng duyệt) vẫn chưa đọc. Người dùng đã xử lý cổng cũ rồi thì lần duyệt sau được báo lại.
func (s *Store) duplicate(u *userData, n PushNotification, now time.Time) bool {
	key := dedupeKey(n, n.Session)
	if key == "" {
		return false
	}
	for _, v := range u.Digests[n.Story].Items {
		if dedupeKey(v, "") == key {
			return true
		}
	}
	for i := len(u.Inbox) - 1; i >= 0; i-- {
		v := u.Inbox[i]
		if dedupeKey(v.PushNotification, v.TargetSession) != key {
			continue
		}
		if now.Sub(v.CreatedAt) < dedupeWindow || (v.Type == "gate" && !v.Read) {
			return true
		}
	}
	return false
}
func (s *Store) append(u *userData, n *PushNotification) {
	s.data.Sequence++
	n.ID = strconv.FormatUint(s.data.Sequence, 10)
	u.Inbox = append(u.Inbox, storedNotification{*n, n.Session})
	if len(u.Inbox) > 500 {
		u.Inbox = append([]storedNotification(nil), u.Inbox[len(u.Inbox)-500:]...)
	}
}
func (s *Store) changed(user string, n int) {
	if s.opts.Changed != nil {
		s.opts.Changed(user, n)
	}
}
func (s *Store) enqueue(user string, n PushNotification) {
	select {
	case s.queue <- delivery{user, n}:
	default: /* The inbox is durable even when the bounded push queue is full. */
		slog.Warn("Hàng đợi Web Push đầy; thông báo vẫn được lưu trong hộp thư", "user", user)
	}
}

// flushUser trả mọi nhóm đang chờ của một tài khoản thành mục hộp thư ngay (đổi sang “mỗi chương”).
func (s *Store) flushUser(user string) error {
	s.mu.Lock()
	u := s.user(user)
	if len(u.Digests) == 0 {
		s.mu.Unlock()
		return nil
	}
	now := s.opts.Clock()
	stories := make([]string, 0, len(u.Digests))
	for story := range u.Digests {
		stories = append(stories, story)
	}
	slices.Sort(stories)
	var out []PushNotification
	for _, story := range stories {
		d := u.Digests[story]
		delete(u.Digests, story)
		if len(d.Items) == 0 || !s.allowed(user, story) {
			continue
		}
		n := group(d.Items)
		n.CreatedAt = now
		s.append(u, &n)
		if permits(u.Prefs, n.Type, now) {
			out = append(out, n)
		}
	}
	e := s.save()
	count := s.unread(user, u)
	s.mu.Unlock()
	if e != nil {
		return e
	}
	s.changed(user, count)
	for _, n := range out {
		s.enqueue(user, n)
	}
	return nil
}
func (s *Store) FlushDigests() error {
	s.mu.Lock()
	now := s.opts.Clock()
	pending := []delivery{}
	counts := map[string]int{}
	dirty := false
	for name, u := range s.data.Users {
		for key, d := range u.Digests {
			if u.Prefs.Digest != "day" {
				continue
			}
			if !s.allowed(name, key) {
				delete(u.Digests, key)
				dirty = true
				continue
			}
			if len(d.Items) == 0 || now.Before(d.Due) {
				continue
			}
			n := group(d.Items)
			n.CreatedAt = now
			s.append(u, &n)
			delete(u.Digests, key)
			dirty = true
			counts[name] = s.unread(name, u)
			if permits(u.Prefs, n.Type, now) {
				pending = append(pending, delivery{name, n})
			}
		}
	}
	if !dirty {
		s.mu.Unlock()
		return nil
	}
	e := s.save()
	s.mu.Unlock()
	if e != nil {
		return e
	}
	for user, n := range counts {
		s.changed(user, n)
	}
	for _, v := range pending {
		s.enqueue(v.User, v.Item)
	}
	return nil
}
func (s *Store) Inbox(user, before string) ([]PushNotification, int, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := s.user(user)
	out := []PushNotification{}
	count := 0
	cursor, _ := strconv.ParseUint(before, 10, 64)
	next := ""
	access := map[string]bool{}
	for i := len(u.Inbox) - 1; i >= 0; i-- {
		v := u.Inbox[i]
		allowed, known := access[v.Story]
		if !known {
			allowed = s.allowed(user, v.Story)
			access[v.Story] = allowed
		}
		if !allowed {
			continue
		}
		if !v.Read {
			count++
		}
		id, _ := strconv.ParseUint(v.ID, 10, 64)
		if cursor > 0 && id >= cursor {
			continue
		}
		if len(out) < 50 {
			out = append(out, v.PushNotification)
		} else {
			next = out[len(out)-1].ID
		}
	}
	return out, count, next
}
func (s *Store) Read(user string, ids []string, all bool) (int, error) {
	s.mu.Lock()
	u := s.user(user)
	for i := range u.Inbox {
		if all || slices.Contains(ids, u.Inbox[i].ID) {
			u.Inbox[i].Read = true
		}
	}
	e := s.save()
	n := s.unread(user, u)
	s.mu.Unlock()
	if e == nil {
		s.changed(user, n)
	}
	return n, e
}
func (s *Store) Find(user, id string) (PushNotification, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, v := range s.user(user).Inbox {
		if v.ID == id && s.allowed(user, v.Story) {
			n := v.PushNotification
			n.Session = v.TargetSession
			return n, true
		}
	}
	return PushNotification{}, false
}
func (s *Store) run() {
	defer s.wg.Done()
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-tick.C:
			_ = s.FlushDigests()
		case v := <-s.queue:
			s.send(v)
		}
	}
}
func (s *Store) send(v delivery) {
	if !s.allowed(v.User, v.Item.Story) || !permits(s.Prefs(v.User), v.Item.Type, s.opts.Clock()) {
		return
	}
	payload, e := json.Marshal(v.Item)
	if e != nil {
		return
	}
	for _, sub := range s.Subscriptions(v.User) {
		if s.opts.SessionActive != nil && !s.opts.SessionActive(v.User, sub.Session) {
			_ = s.Revoke(v.User, sub.Session)
			continue
		}
		res, e := webpush.SendNotificationWithContext(s.ctx, payload, &webpush.Subscription{Endpoint: sub.Endpoint, Keys: webpush.Keys{P256dh: sub.Keys.P256dh, Auth: sub.Keys.Auth}}, &webpush.Options{HTTPClient: s.opts.HTTPClient, Subscriber: "mailto:push@ainovel.local", VAPIDPublicKey: s.keys.Public, VAPIDPrivateKey: s.keys.Private, TTL: 3600})
		if e != nil {
			if !errors.Is(e, context.Canceled) {
				slog.Warn("Không gửi được Web Push", "user", v.User, "err", e)
			}
			continue
		}
		_, _ = io.Copy(io.Discard, res.Body)
		_ = res.Body.Close()
		if res.StatusCode >= 400 && res.StatusCode != 404 && res.StatusCode != 410 {
			slog.Warn("Máy chủ Web Push từ chối thông báo", "user", v.User, "status", res.StatusCode)
		}
		if res.StatusCode == 404 || res.StatusCode == 410 {
			_ = s.Unsubscribe(v.User, sub.Endpoint)
		}
	}
}
func (s *Store) Close() {
	s.mu.Lock()
	s.closed = true
	s.cancel()
	s.mu.Unlock()
	s.wg.Wait()
}
