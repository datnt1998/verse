package web

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/datnt1998/verse/platform/identity"
)

// routePermission is the shared request boundary for all project/story/audio
// handlers, including worker-proxied routes. Unknown writes are never read-only.
func routePermission(r *http.Request) (project string, minimum identity.ProjectRole, admin bool) {
	path := strings.Trim(r.URL.Path, "/")
	seg := strings.Split(path, "/")
	read := r.Method == http.MethodGet || r.Method == http.MethodHead
	if strings.HasPrefix(path, "api/jobs/"+voiceInstallJobID+"/") {
		return "", "", true
	}
	if strings.HasPrefix(path, "api/admin/") || path == "api/users" || strings.HasPrefix(path, "api/users/") || strings.HasPrefix(path, "api/fs/") || path == "api/system/network" || path == "api/system/pick-folder" || path == "api/tts/install" {
		return "", "", true
	}
	if path == "api/projects" || path == "api/projects/adopt" {
		return "", "", true
	}
	if len(seg) >= 3 && seg[0] == "api" && seg[1] == "projects" {
		project = seg[2]
		minimum = identity.ProjectView
		if !read {
			minimum = identity.ProjectCollab
		}
		// Owner-scoped member API (ma trận §9, quyết định 2026-10-28): chỉ
		// chủ dự án (admin = owner mọi project) mời/đổi vai/gỡ/chuyển chủ.
		if !read && len(seg) > 3 && (seg[3] == "members" || seg[3] == "owner") {
			minimum = identity.ProjectOwner
		}
		if !read && (len(seg) == 3 || (len(seg) > 3 && (seg[3] == "ledger" || seg[3] == "validate"))) {
			minimum = identity.ProjectOwner
		}
	} else if len(seg) >= 4 && seg[0] == "api" && seg[1] == "jobs" {
		project = seg[2]
		minimum = identity.ProjectView
		if !read {
			minimum = identity.ProjectCollab
		}
	} else if path == "api/config" || strings.HasPrefix(path, "api/config/") || path == "api/rules" || strings.HasPrefix(path, "api/rules/") {
		scope := r.URL.Query().Get("scope")
		projectID := r.URL.Query().Get("project")
		if resolved, ok := r.Context().Value(mutationScopeKey{}).(mutationScope); ok {
			scope, projectID = resolved.scope, resolved.project
		}
		if scope == "project" || scope == "story" {
			project = projectID
			minimum = identity.ProjectView
			if !read {
				minimum = identity.ProjectCollab
				if scope == "project" {
					minimum = identity.ProjectOwner
				}
			}
		} else if !read || strings.HasPrefix(path, "api/config/oauth/") {
			return "", "", true
		}
	} else if path == "api/library" && !read {
		return "", "", true
	}
	if project != "" && !read {
		if r.Method == http.MethodDelete {
			minimum = identity.ProjectOwner
			// Collab unkeep Cảnh báo (quyết định 2026-10-28): DELETE đối xứng
			// với POST trên …/review/items/{id}/keep — các DELETE khác giữ owner.
			if len(seg) >= 9 && seg[1] == "projects" && seg[3] == "stories" &&
				seg[5] == "review" && seg[6] == "items" && seg[8] == "keep" {
				minimum = identity.ProjectCollab
			}
		}
		start := 2
		if len(seg) >= 3 && seg[1] == "projects" {
			start = 3
			if len(seg) > 3 && seg[3] == "stories" {
				start = 5
			}
		} else if len(seg) >= 4 && seg[1] == "jobs" {
			start = 4
		}
		if len(seg) > start {
			action, last := seg[start], seg[len(seg)-1]
			if action == "share" || last == "delete" || last == "remove" || last == "share" {
				minimum = identity.ProjectOwner
			}
		}
	}
	return
}
func requestAllowed(user identity.User, r *http.Request) bool {
	project, minimum, admin := routePermission(r)
	if user.Disabled {
		return false
	}
	if admin {
		return isAdmin(user)
	}
	if project == "" {
		return true
	}
	switch minimum {
	case identity.ProjectOwner:
		return user.CanManageProject(project)
	case identity.ProjectCollab:
		return user.CanCollaborateProject(project)
	default:
		return user.CanReadProject(project)
	}
}
func authorizeRequest(w http.ResponseWriter, r *http.Request, user identity.User) bool {
	if user.MustChangePassword && !passwordNeutralRoute(r.Method, r.URL.Path) {
		writeJSON(w, 403, map[string]any{"error": "Bạn cần đổi mật khẩu tạm trước khi tiếp tục.", "mustChangePassword": true})
		return false
	}
	if requestAllowed(user, r) {
		return true
	}
	project, _, admin := routePermission(r)
	message := "Bạn không có quyền vào dự án này."
	if admin {
		message = "Chỉ quản trị viên được thực hiện thao tác này."
	} else {
		switch user.ProjectRole(project) {
		case identity.ProjectView:
			message = "Bạn chỉ có quyền Chỉ xem ở dự án này"
		case identity.ProjectCollab:
			message = "Thao tác này cần quyền Chủ dự án."
		}
	}
	jsonError(w, 403, message)
	return false
}

// passwordNeutralRoute enumerates — method and path exactly — the only
// requests an account that must change its temporary password may make.
// This mirrors the no-grant auth-neutral set plus the profile/status reads
// the forced-change screens need; anything else, including SSE, uploads and
// unknown writes, is refused before any handler runs.
func passwordNeutralRoute(method, path string) bool {
	switch path {
	case "/api/me":
		return method == http.MethodGet || method == http.MethodPost || method == http.MethodPut
	case "/api/me/onboarded":
		return method == http.MethodPost
	case "/api/me/sessions":
		return method == http.MethodGet
	}
	return sessionOnlyRoute(method, path)
}

// Keep long-lived story responses tied to current stored permissions and sessions,
// including changes made by another CLI/server process which cannot notify this hub.
func (g *Gateway) authenticatedStoryStream(r *http.Request, story string) (*http.Request, context.CancelFunc) {
	ctx, cancel := context.WithCancel(r.Context())
	cookie, err := r.Cookie(g.cookiePolicy().Name)
	if err != nil {
		cancel()
		return r.WithContext(ctx), cancel
	}
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				user, _, err := g.accounts.Authenticate(cookie.Value)
				if err != nil || !user.CanRead(story) || !appAccessGranted(user.Grants) {
					cancel()
					return
				}
			}
		}
	}()
	return r.WithContext(ctx), cancel
}
