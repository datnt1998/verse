package webauth

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/datnt1998/verse/apps/noverse/internal/library"
)

type AuditActor struct {
	Username string `json:"username"`
	Name     string `json:"name"`
}
type AuditContext struct {
	Actor  *AuditActor
	Reason string
}
type AuditEntry struct {
	ID      string      `json:"id"`
	At      time.Time   `json:"at"`
	Actor   *AuditActor `json:"actor"`
	Type    string      `json:"type"`
	Action  string      `json:"action"`
	Target  string      `json:"target"`
	Project string      `json:"project,omitempty"`
	From    string      `json:"from,omitempty"`
	To      string      `json:"to,omitempty"`
	Reason  string      `json:"reason,omitempty"`
}
type AuditQuery struct {
	Type, Actor, Before string
	Limit               int
}
type AuditPage struct {
	Items []AuditEntry `json:"items"`
	Next  string       `json:"next"`
}

func auditContext(context []AuditContext) AuditContext {
	if len(context) > 0 {
		return context[0]
	}
	return AuditContext{}
}
func (s *Store) addAudit(d *data, actor *AuditActor, typ, action, target, from, to, reason string) {
	at := s.now()
	// IDs sort by insertion order even when multiple actions share the same clock tick.
	sequence := uint64(1)
	if len(d.Audit) > 0 {
		last := d.Audit[len(d.Audit)-1].ID
		if n, err := strconv.ParseUint(last, 10, 64); err == nil {
			sequence = n + 1
		}
	}
	d.Audit = append(d.Audit, AuditEntry{ID: fmt.Sprintf("%020d", sequence), At: at, Actor: actor, Type: typ, Action: action, Target: target, From: from, To: to, Reason: strings.TrimSpace(reason)})
}
func projectsEqual(a, b map[string]ProjectRole) bool { return maps.Equal(a, b) }
func projectSummary(projects map[string]ProjectRole) string {
	raw, _ := json.Marshal(projects)
	return string(raw)
}

// SetProjectSource supplies existing projects for the one-time wildcard migration.
// Call before any account read (the gateway does this before HasUsers).
func (s *Store) SetProjectSource(source func() ([]string, error)) {
	if s.repo != nil {
		// Postgres backend: memberships là bảng riêng, không cần nguồn project
		// cho migration đọc-ghi file — đây là file-mode migration hook.
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.projectIDs = source
	s.cache, s.stat = nil, nil
}

// Unknown roots must not resolve wildcard grants against an unrelated default
// library. CLI account commands leave these grants pending until serve supplies
// its effective --root through SetProjectSource.
var ErrProjectRootUnknown = errors.New("Chưa biết thư viện đang phục vụ.")

type legacyPermission struct {
	Role  string
	Books []string
}

func (s *Store) existingProjects() ([]string, error) {
	if s.projectIDs == nil {
		return nil, ErrProjectRootUnknown
	}
	return s.projectIDs()
}

func (s *Store) migrate(d *data) (bool, error) {
	// Chuỗi migration tuần tự: v2 legacy (books grants) → v3 → v4 (mật khẩu
	// "phải đổi" thành mật khẩu tạm một lần có TTL). File xuất phát ở v3 đi
	// thẳng bước v4; file v2 chạy hết cả chuỗi trong cùng một ghi.
	var raw []byte
	var err error
	if d.Version >= 3 {
		return s.migrateV4(d, true)
	}
	raw, err = os.ReadFile(s.path)
	if err != nil {
		return false, err
	}
	var legacy struct {
		Users []struct {
			Username string   `json:"username"`
			Role     string   `json:"role"`
			Books    []string `json:"books"`
		} `json:"users"`
	}
	if err := json.Unmarshal(raw, &legacy); err != nil {
		return false, err
	}
	d.legacyPermissions = make(map[string]legacyPermission, len(legacy.Users))
	var ids []string
	var sourceErr error
	resolved, complete := false, true
	for _, old := range legacy.Users {
		u := d.user(old.Username)
		if u == nil {
			continue
		}
		// Accounts created or explicitly re-granted while migration is pending
		// already carry the new model and must retain those current permissions.
		if old.Role == string(RoleMember) {
			if u.Projects == nil {
				u.Projects = map[string]ProjectRole{}
			}
			continue
		}
		d.legacyPermissions[old.Username] = legacyPermission{Role: old.Role, Books: old.Books}
		u.Projects = map[string]ProjectRole{}
		if old.Role == "admin" {
			u.Role = RoleAdmin
			continue
		}
		u.Role = RoleMember
		level := ProjectView
		if old.Role == "writer" {
			level = ProjectCollab
		}
		for _, book := range old.Books {
			if book == "" {
				continue
			}
			if book == "*" {
				if !resolved {
					ids, sourceErr = s.existingProjects()
					resolved = true
					if sourceErr != nil && !errors.Is(sourceErr, ErrProjectRootUnknown) {
						slog.Warn("Chưa thể di trú quyền tài khoản; giữ dữ liệu cũ để đăng nhập.", "lỗi", sourceErr)
					}
				}
				if sourceErr != nil {
					complete = false
					continue
				}
				for _, id := range ids {
					if library.ValidProjectID(id) {
						u.Projects[id] = level
					}
				}
			} else {
				project, _, ok := strings.Cut(book, "/")
				if !ok {
					project = library.LegacyProjectID
				}
				if library.ValidProjectID(project) {
					u.Projects[project] = level
				}
			}
		}
	}
	if !complete {
		return false, nil
	}
	// Preserve exact bytes before the first fully migrated write, never
	// overwriting the recovery copy on a retry or another process.
	backup := s.path + ".v2.bak"
	file, err := os.OpenFile(backup, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil && !errors.Is(err, os.ErrExist) {
		return false, err
	}
	if err == nil {
		if _, err = file.Write(raw); err != nil {
			file.Close()
			os.Remove(backup)
			return false, err
		}
		if err = file.Close(); err != nil {
			return false, err
		}
	}
	s.addAudit(d, nil, "perm", "Di trú quyền", "", "", "", "")
	// v2→v3 hoàn tất; tiếp tục bước v3→v4 trong cùng một ghi (không backup
	// .v3.bak vì byte gốc đã lưu ở .v2.bak).
	d.legacyPermissions = nil
	return s.migrateV4(d, false)
}

// migrateV4 đưa mật khẩu đang "phải đổi" thành TempPassword một lần có TTL
// (tính từ passwordChangedAt). backup=true khi file đĩa thật đang là v3 —
// byte gốc giữ một lần ở .v3.bak; khi đến từ chuỗi v2→v3→v4 bản sao đã có.
func (s *Store) migrateV4(d *data, backup bool) (bool, error) {
	for i := range d.Users {
		u := &d.Users[i]
		if u.MustChangePassword && u.TempPassword == nil && u.PasswordHash != "" {
			base := u.UpdatedAt
			if u.PasswordChangedAt != nil {
				base = *u.PasswordChangedAt
			}
			u.TempPassword = &TempPassword{CodeHash: u.PasswordHash, ExpiresAt: base.Add(TempPasswordTTL), CreatedAt: base}
			u.PasswordHash = ""
		}
	}
	if backup {
		raw, err := os.ReadFile(s.path)
		if err != nil {
			return false, err
		}
		path := s.path + ".v3.bak"
		if file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600); err == nil {
			if _, err = file.Write(raw); err != nil {
				file.Close()
				os.Remove(path)
				return false, err
			}
			if err = file.Close(); err != nil {
				return false, err
			}
		} else if !errors.Is(err, os.ErrExist) {
			return false, err
		}
	}
	d.Version = fileVersion
	return true, nil
}

// Pending migration may still update sessions/passwords. Only this transitional
// serializer retains the original permission fields; User/API have no aliases.
func marshalAccounts(d *data) ([]byte, error) {
	if len(d.legacyPermissions) == 0 {
		return json.MarshalIndent(d, "", "  ")
	}
	users := make([]json.RawMessage, 0, len(d.Users))
	for _, user := range d.Users {
		raw, err := json.Marshal(user)
		if err != nil {
			return nil, err
		}
		if legacy, ok := d.legacyPermissions[user.Username]; ok {
			var fields map[string]json.RawMessage
			if err = json.Unmarshal(raw, &fields); err != nil {
				return nil, err
			}
			fields["role"], err = json.Marshal(legacy.Role)
			if err != nil {
				return nil, err
			}
			fields["books"], err = json.Marshal(legacy.Books)
			if err != nil {
				return nil, err
			}
			delete(fields, "projects")
			raw, err = json.Marshal(fields)
			if err != nil {
				return nil, err
			}
		}
		users = append(users, raw)
	}
	return json.MarshalIndent(struct {
		Version  int               `json:"version"`
		Users    []json.RawMessage `json:"users"`
		Sessions []session         `json:"sessions"`
		Pairings []pairing         `json:"pairings"`
		Audit    []AuditEntry      `json:"audit"`
	}{d.Version, users, d.Sessions, d.Pairings, d.Audit}, "", "  ")
}
func (s *Store) Audit(query AuditQuery) (AuditPage, error) {
	if s.repo != nil {
		return s.repo.Audit(query)
	}
	if query.Type != "" && query.Type != "acct" && query.Type != "perm" && query.Type != "sec" {
		return AuditPage{}, fmt.Errorf("%w: loại nhật ký không hợp lệ", ErrInvalidInput)
	}
	if query.Limit <= 0 {
		query.Limit = 50
	}
	if query.Limit > 200 {
		query.Limit = 200
	}
	// Retention is applied on reads too, even if the server had no recent writes.
	d, err := s.snapshot()
	if err != nil {
		return AuditPage{}, err
	}
	cutoff := s.now().AddDate(-1, 0, 0)
	if slices.ContainsFunc(d.Audit, func(e AuditEntry) bool { return e.At.Before(cutoff) }) {
		if err = s.update(func(*data) error { return nil }); err != nil {
			return AuditPage{}, err
		}
		d, err = s.snapshot()
		if err != nil {
			return AuditPage{}, err
		}
	}
	page := AuditPage{Items: []AuditEntry{}}
	for i := len(d.Audit) - 1; i >= 0; i-- {
		e := d.Audit[i]
		if query.Type != "" && e.Type != query.Type {
			continue
		}
		actor := "system"
		if e.Actor != nil {
			actor = e.Actor.Username
		}
		if query.Actor != "" && query.Actor != actor {
			continue
		}
		if query.Before != "" && e.ID >= query.Before {
			continue
		}
		if len(page.Items) == query.Limit {
			page.Next = page.Items[len(page.Items)-1].ID
			break
		}
		page.Items = append(page.Items, e)
	}
	return page, nil
}
func (s *Store) RecordSecurityFailure(username string) error {
	username = strings.TrimSpace(username)
	if !ValidUsername(username) {
		return nil
	}
	if s.repo != nil {
		return s.repo.RecordSecurityFailure(username)
	}
	return s.update(func(d *data) error {
		if d.user(username) == nil {
			return nil
		}
		s.addAudit(d, nil, "sec", "Đăng nhập sai nhiều lần", username, "", "", "")
		return nil
	})
}
func (s *Store) RevokeAll(username, keep string, context AuditContext) error {
	if s.repo != nil {
		return s.repo.RevokeAll(username, keep, context)
	}
	return s.update(func(d *data) error {
		if d.user(username) == nil {
			return ErrNotFound
		}
		d.dropSessions(func(x session) bool { return x.Username != username || x.ID == keep })
		d.Pairings = slices.DeleteFunc(d.Pairings, func(x pairing) bool { return x.Username == username })
		s.addAudit(d, context.Actor, "sec", "Đăng xuất mọi thiết bị", username, "", "", context.Reason)
		return nil
	})
}

// ---- Quản lý membership phạm vi project (Repository contract) ----
// Owner hoặc admin gọi các hàm này qua API /api/projects/:p/members*; guard
// HTTP đã xác nhận quyền — repository chỉ thực thi invariant, không lặp quyền.

// addProjectAudit ghi audit gắn project (cột project trong AuditEntry).
func (s *Store) addProjectAudit(d *data, actor *AuditActor, project, typ, action, target, from, to, reason string) {
	s.addAudit(d, actor, typ, action, target, from, to, reason)
	d.Audit[len(d.Audit)-1].Project = project
}

// ProjectMembers liệt kê membership hiệu lực của project, sắp theo username.
func (s *Store) ProjectMembers(project string) ([]ProjectMember, error) {
	if s.repo != nil {
		return s.repo.ProjectMembers(project)
	}
	d, err := s.snapshot()
	if err != nil {
		return nil, err
	}
	out := []ProjectMember{}
	for _, u := range d.Users {
		if u.DeletedAt == nil {
			if role := u.Projects[project]; role != "" {
				out = append(out, ProjectMember{Username: u.Username, Name: u.Name, Role: role, Disabled: u.Disabled})
			}
		}
	}
	slices.SortFunc(out, func(a, b ProjectMember) int { return strings.Compare(a.Username, b.Username) })
	return out, nil
}

// SetProjectMember mời/đổi vai member; chỉ collab|view — owner chỉ qua
// TransferProjectOwner để giữ đúng một owner mỗi project.
func (s *Store) SetProjectMember(project, username string, role ProjectRole, actor *AuditActor) (UserView, error) {
	if s.repo != nil {
		return s.repo.SetProjectMember(project, username, role, actor)
	}
	if role != ProjectCollab && role != ProjectView {
		return UserView{}, fmt.Errorf("%w: vai trò dự án phải là collab hoặc view", ErrInvalidInput)
	}
	if !library.ValidProjectID(project) {
		return UserView{}, fmt.Errorf("%w: dự án không hợp lệ", ErrInvalidInput)
	}
	var updated User
	err := s.update(func(d *data) error {
		u := d.user(username)
		if u == nil {
			return ErrNotFound
		}
		before := u.Projects[project]
		if before == ProjectOwner && d.activeOwners(project) == 1 {
			return ErrLastOwner
		}
		if u.Projects == nil {
			u.Projects = map[string]ProjectRole{}
		}
		u.Projects[project] = role
		u.UpdatedAt = s.now()
		updated = *u
		if before != role {
			s.addProjectAudit(d, actor, project, "perm", "Đổi quyền dự án", username, string(before), string(role), "")
		}
		return nil
	})
	return updated.View(), err
}

// RemoveProjectMember gỡ một member; không gỡ owner cuối cùng.
func (s *Store) RemoveProjectMember(project, username string, actor *AuditActor) error {
	if s.repo != nil {
		return s.repo.RemoveProjectMember(project, username, actor)
	}
	return s.update(func(d *data) error {
		u := d.user(username)
		if u == nil {
			return ErrNotFound
		}
		before := u.Projects[project]
		if before == "" {
			return ErrNotFound
		}
		if before == ProjectOwner && d.activeOwners(project) == 1 {
			return ErrLastOwner
		}
		delete(u.Projects, project)
		u.UpdatedAt = s.now()
		s.addProjectAudit(d, actor, project, "perm", "Gỡ quyền dự án", username, string(before), "", "")
		return nil
	})
}

// TransferProjectOwner chuyển owner nguyên tử: mọi owner hiện tại về collab,
// username trở thành owner duy nhất — trong cùng một ghi (flock/tx).
func (s *Store) TransferProjectOwner(project, username string, actor *AuditActor) error {
	if s.repo != nil {
		return s.repo.TransferProjectOwner(project, username, actor)
	}
	return s.update(func(d *data) error {
		target := d.user(username)
		if target == nil {
			return ErrNotFound
		}
		if target.Disabled {
			return fmt.Errorf("%w: không thể chuyển quyền cho tài khoản đang bị khoá", ErrInvalidInput)
		}
		previous := []string{}
		for i := range d.Users {
			u := &d.Users[i]
			if u.DeletedAt == nil && !u.Disabled && u.Role != RoleAdmin && u.Projects[project] == ProjectOwner {
				previous = append(previous, u.Username)
				u.Projects[project] = ProjectCollab
				u.UpdatedAt = s.now()
			}
		}
		if target.Projects == nil {
			target.Projects = map[string]ProjectRole{}
		}
		target.Projects[project] = ProjectOwner
		target.UpdatedAt = s.now()
		s.addProjectAudit(d, actor, project, "perm", "Chuyển chủ dự án", username, strings.Join(previous, ","), string(ProjectOwner), "")
		return nil
	})
}
