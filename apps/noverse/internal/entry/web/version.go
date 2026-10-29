package web

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/datnt1998/verse/apps/noverse/internal/library"
	"github.com/datnt1998/verse/platform/identity"
)

// File versions include metadata as well as content: saving an identical document
// still invalidates a previously read version, including atomic file replacements.
func fileVersion(path string) string {
	h := sha256.New()
	if info, err := os.Stat(path); err == nil {
		fmt.Fprintf(h, "%d:%d:", info.ModTime().UnixNano(), info.Size())
		if !info.IsDir() {
			if data, err := os.ReadFile(path); err == nil {
				_, _ = h.Write(data)
			}
		}
	} else {
		_, _ = h.Write([]byte("missing"))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func valueVersion(v any) string {
	b, _ := json.Marshal(v)
	var normalized any
	if json.Unmarshal(b, &normalized) == nil {
		b, _ = json.Marshal(normalized)
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

type versionResource struct {
	path, username string
	users          bool
	writable       bool
	query          url.Values
}

// writeVersionToken 把已算好的 ETag 写到响应头并返回 token。
func writeVersionToken(w http.ResponseWriter, token string) string {
	w.Header().Set("ETag", `"`+token+`"`)
	return token
}

// checkIfMatchV 与 checkIfMatch 一样执行 If-Match/If-None-Match，但拿的是
// 后端直接给的 version token（Postgres 后端没有文件可 stat）。语义必须与
// checkIfMatch 一致：裸 token 与带引号都接受，失配回 409 + conflict 载荷
// （前端"Ghi đè"流程依赖这个形状）。
func checkIfMatchV(w http.ResponseWriter, r *http.Request, token string, resource func() (any, error)) bool {
	ifMatch := strings.TrimSpace(r.Header.Get("If-Match"))
	ifNoneMatch := strings.TrimSpace(r.Header.Get("If-None-Match"))
	writeVersionToken(w, token)
	if ifMatch != "" && ifMatch != "*" {
		if unq, err := strconv.Unquote(ifMatch); err == nil {
			ifMatch = unq
		}
		if ifMatch != token {
			value, err := resource()
			if err != nil {
				audioError(w, err)
				return false
			}
			writeJSON(w, http.StatusConflict, map[string]any{
				"error": "Đã có thay đổi từ nơi khác", "conflict": true,
				"version": token, "current": value,
			})
			return false
		}
	}
	if ifMatch == "" && ifNoneMatch != "" {
		matched := ifNoneMatch == "*"
		for _, part := range strings.Split(ifNoneMatch, ",") {
			part = strings.TrimSpace(part)
			if unq, err := strconv.Unquote(part); err == nil {
				part = unq
			}
			if part == token || part == "*" {
				matched = true
			}
		}
		if matched {
			if r.Method == http.MethodGet || r.Method == http.MethodHead {
				w.WriteHeader(http.StatusNotModified)
			} else {
				jsonError(w, http.StatusConflict, "Đã có thay đổi từ nơi khác")
			}
			return false
		}
	}
	_, err := resource()
	if err != nil {
		audioError(w, err)
		return false
	}
	return true
}

func (g *Gateway) resourceFor(r *http.Request, user identity.User, body map[string]json.RawMessage) *versionResource {
	path := strings.TrimSuffix(r.URL.Path, "/")
	q := r.URL.Query()
	if strings.HasPrefix(path, "/api/rules/file") {
		for _, key := range []string{"scope", "project", "story", "name"} {
			var s string
			if json.Unmarshal(body[key], &s) == nil {
				q.Set(key, s)
			}
		}
	}
	switch path {
	case "/api/config", "/api/rules/file", "/api/rules/file/delete":
		t, err := g.resolveConfigScope(q.Get("scope"), q.Get("project"), q.Get("story"))
		if err != nil {
			return nil
		}
		p := t.path
		if path != "/api/config" {
			if !validRuleName(q.Get("name")) {
				return nil
			}
			p = filepath.Join(t.rulesTargetDir(), q.Get("name"))
		}
		return &versionResource{path: p, writable: checkConfigWrite(user, t) == nil, query: q}
	case "/api/library":
		if g.RootPath() == "" {
			return nil
		}
		return &versionResource{path: library.ConfigPath(g.RootPath()), writable: isAdmin(user)}
	case "/api/me", "/api/me/password", "/api/me/onboarded":
		return &versionResource{username: user.Username, writable: true}
	case "/api/users":
		if !isAdmin(user) {
			return nil
		}
		return &versionResource{users: true, writable: true}
	}
	segs := strings.Split(strings.Trim(path, "/"), "/")
	if len(segs) >= 3 && segs[0] == "api" && segs[1] == "users" && isAdmin(user) {
		return &versionResource{username: segs[2], writable: true}
	}
	if len(segs) < 3 || segs[0] != "api" || segs[1] != "projects" {
		return nil
	}
	lib, err := g.lib(r.Context(), actorOf(user))
	if err != nil {
		return nil
	}
	p := segs[2]
	if len(segs) == 3 && user.CanReadProject(p) {
		ref, ok := lib.ResolveProject(r.Context(), actorOf(user), p)
		if !ok || ref.Dir == "" {
			return nil
		}
		return &versionResource{path: filepath.Join(ref.Dir, "project.json"), writable: user.CanManageProject(p)}
	}
	if len(segs) == 6 && segs[3] == "stories" && segs[5] == "meta" && user.CanRead(p+"/"+segs[4]) {
		st, ok := lib.ResolveStory(r.Context(), actorOf(user), p, segs[4])
		if !ok || st.Dir == "" {
			return nil
		}
		return &versionResource{path: filepath.Join(st.Dir, "story.json"), writable: user.CanWrite(st.ID)}
	}
	return nil
}

func (g *Gateway) resourceVersion(res *versionResource) string {
	if res.path != "" {
		return fileVersion(res.path)
	}
	users, err := g.accounts.Users()
	if err != nil {
		return valueVersion(nil)
	}
	views := make([]identity.UserView, 0, len(users))
	for _, u := range users {
		if res.username != "" && u.Username == res.username {
			return valueVersion(u.UserView)
		}
		views = append(views, u.UserView)
	}
	if res.username != "" {
		return valueVersion(nil)
	}
	return valueVersion(views)
}

type resourceBuffer struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func newResourceBuffer() *resourceBuffer      { return &resourceBuffer{header: make(http.Header)} }
func (b *resourceBuffer) Header() http.Header { return b.header }
func (b *resourceBuffer) WriteHeader(status int) {
	if b.status == 0 {
		b.status = status
	}
}
func (b *resourceBuffer) Write(p []byte) (int, error) {
	if b.status == 0 {
		b.status = 200
	}
	return b.body.Write(p)
}

// currentResource goes through the same permission-aware readers as GET. It is
// never called until resourceFor has established the caller's write permission.
func (g *Gateway) currentResource(r *http.Request, user identity.User, current identity.Session, res *versionResource) any {
	if res.username != "" {
		users, _ := g.accounts.Users()
		for _, u := range users {
			if u.Username == res.username {
				data, _ := json.Marshal(accountUserView{UserView: u.UserView, LastSeenAt: u.LastSeenAt})
				var view map[string]any
				_ = json.Unmarshal(data, &view)
				view["version"] = valueVersion(u.UserView)
				if strings.HasPrefix(r.URL.Path, "/api/me") {
					return map[string]any{"user": view, "session": current}
				}
				return view
			}
		}
		return nil
	}
	get := r.Clone(r.Context())
	get.Method = http.MethodGet
	u := *r.URL
	get.URL = &u
	if res.query != nil {
		get.URL.RawQuery = res.query.Encode()
	}
	get.URL.Path = strings.TrimSuffix(get.URL.Path, "/delete")
	b := newResourceBuffer()
	if !g.serveAccount(b, get, user, current) && !g.serveConfig(b, get, user) && !g.serveLibrary(b, get, user) {
		g.serveProjects(b, get, user)
	}
	var v any
	if b.status >= 200 && b.status < 300 {
		_ = json.Unmarshal(b.body.Bytes(), &v)
	}
	return v
}

// versionLock serializes only one persisted resource. Worker actions have no
// versioned resource and never acquire one of these locks.
func (g *Gateway) versionLock(res *versionResource) *sync.Mutex {
	key := res.path
	if key == "" {
		key = "accounts"
	}
	lock, _ := g.versionLocks.LoadOrStore(key, &sync.Mutex{})
	return lock.(*sync.Mutex)
}

// beginFeedback keeps version compare/write atomic for the same resource only.
// Network response writes and event publication happen after releasing its lock.
func (g *Gateway) beginFeedback(w http.ResponseWriter, r *http.Request, user identity.User, current identity.Session) (http.ResponseWriter, func(), bool) {
	mutating := r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodDelete || r.Method == http.MethodPatch
	var body map[string]json.RawMessage
	// Only versioned JSON resources need their body inspected; uploads and worker
	// streams retain their original reader and flushing semantics.
	candidate := r.URL.Path == "/api/library" || r.URL.Path == "/api/config" || strings.HasPrefix(r.URL.Path, "/api/rules/file") || r.URL.Path == "/api/me" || strings.HasPrefix(r.URL.Path, "/api/me/") || strings.HasPrefix(r.URL.Path, "/api/users") || r.Method == http.MethodPut
	if mutating && candidate && jsonContent(r) {
		data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err != nil {
			jsonError(w, 400, "JSON không hợp lệ")
			return w, func() {}, false
		}
		r.Body = io.NopCloser(bytes.NewReader(data))
		if json.Unmarshal(data, &body) == nil {
			var version string
			if len(body["version"]) > 0 && json.Unmarshal(body["version"], &version) == nil {
				if r.Header.Get("If-Match") == "" {
					r.Header.Set("If-Match", version)
				}
				delete(body, "version")
				clean, _ := json.Marshal(body)
				r.Body = io.NopCloser(bytes.NewReader(clean))
				r.ContentLength = int64(len(clean))
			}
		}
	}
	res := g.resourceFor(r, user, body)
	if res != nil && res.query != nil {
		*r = *r.WithContext(context.WithValue(r.Context(), mutationScopeKey{}, mutationScope{res.query.Get("scope"), res.query.Get("project"), res.query.Get("story")}))
	}
	if mutating && strings.HasPrefix(r.URL.Path, "/api/sessions/") {
		id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/sessions/"), "/revoke")
		sessions, _ := g.accounts.Sessions("", current.ID)
		for _, session := range sessions {
			if session.ID == id {
				*r = *r.WithContext(context.WithValue(r.Context(), revokedOwnerKey{}, session.Username))
				break
			}
		}
	}
	if mutating && res != nil && res.username != "" {
		sessions, _ := g.accounts.Sessions(res.username, current.ID)
		*r = *r.WithContext(context.WithValue(r.Context(), previousSessionsKey{}, sessions))
	}
	unlock := func() {}
	if res != nil {
		lock := g.versionLock(res)
		lock.Lock()
		unlock = lock.Unlock
	}
	if res != nil && mutating && res.writable {
		version := g.resourceVersion(res)
		match := strings.Trim(r.Header.Get("If-Match"), "\"")
		if match != "" && match != "*" && match != version {
			latest := g.currentResource(r, user, current, res)
			if m, ok := latest.(map[string]any); ok {
				m["version"] = version
			}
			unlock()
			writeJSON(w, 409, map[string]any{"error": "Đã có thay đổi từ nơi khác", "conflict": true, "version": version, "current": latest})
			return w, func() {}, false
		}
	}
	out := w
	var mutation *mutationResponse
	if mutating {
		mutation = &mutationResponse{ResponseWriter: w}
		out = mutation
	}
	var buffered *resourceBuffer
	if res != nil {
		buffered = newResourceBuffer()
		out = buffered
	}
	finish := func() {
		locked := true
		release := func() {
			if locked {
				unlock()
				locked = false
			}
		}
		defer release()
		if buffered != nil {
			status := buffered.status
			if status == 0 {
				status = 200
			}
			data := buffered.body.Bytes()
			if status >= 200 && status < 300 {
				var object map[string]any
				if json.Unmarshal(data, &object) == nil && object != nil {
					if r.URL.Path == "/api/library" {
						if _, directConfig := object["dirs"]; directConfig {
							object = map[string]any{"ok": true, "config": object}
						}
						res.path = g.libraryConfigPath()
					}
					if res.users && mutating {
						if username, ok := object["username"].(string); ok {
							res.username = username
						}
					}
					version := g.resourceVersion(res)
					object["version"] = version
					if p, ok := object["project"].(map[string]any); ok {
						p["version"] = version
					}
					if u, ok := object["user"].(map[string]any); ok {
						u["version"] = version
					}
					if users, ok := object["users"].([]any); ok {
						for _, item := range users {
							if u, ok := item.(map[string]any); ok {
								view := map[string]any{}
								for k, v := range u {
									if k != "lastSeenAt" && k != "sessionCount" {
										view[k] = v
									}
								}
								u["version"] = valueVersion(view)
							}
						}
					}
					data, _ = json.Marshal(object)
					buffered.header.Set("ETag", "\""+version+"\"")
				}
			}
			release()
			dst := w
			if mutation != nil {
				dst = mutation
			}
			for k, v := range buffered.header {
				dst.Header()[k] = v
			}
			dst.Header().Del("Content-Length")
			dst.WriteHeader(status)
			_, _ = dst.Write(data)
		}
		release()
		if mutation != nil && mutation.status >= 200 && mutation.status < 300 {
			g.publishMutation(r, user, current, mutation.body.Bytes())
		}
	}
	return out, finish, true
}
