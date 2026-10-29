package postgres

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/noverse/internal/library"
	"github.com/datnt1998/noverse/internal/persistence/postgres/gen"
	"github.com/datnt1998/noverse/internal/webauth"
)

// projectBySlug resolves (or lazily anchors when create=true) a projects row.
// Story content arrives in phase 05; phase 04 rows are membership/blob anchors.
func projectBySlug(ctx context.Context, q *gen.Queries, slug string, create bool) (gen.Project, error) {
	p, err := q.GetProjectBySlug(ctx, slug)
	if err == nil {
		return p, nil
	}
	if !noRows(err) {
		return p, sanitizeErr(err)
	}
	if !create {
		return p, webauth.ErrNotFound
	}
	ensured, err := q.EnsureProject(ctx, gen.EnsureProjectParams{
		ID:   deterministicUUID("project", slug),
		Slug: slug,
		Name: slug,
	})
	if err != nil {
		return p, fmt.Errorf("neo dự án %s: %w", slug, sanitizeErr(err))
	}
	return gen.Project{
		ID: ensured.ID, Slug: ensured.Slug, Name: ensured.Name,
		Description: ensured.Description, Notes: ensured.Notes,
		UniverseName: ensured.UniverseName, Settings: ensured.Settings,
		DeletedAt: ensured.DeletedAt, CreatedAt: ensured.CreatedAt,
		UpdatedAt: ensured.UpdatedAt, Version: ensured.Version,
	}, nil
}

func projectID(ctx context.Context, q *gen.Queries, slug string) (pgtype.UUID, error) {
	p, err := q.GetProjectBySlug(ctx, slug)
	if err != nil {
		if noRows(err) {
			return pgtype.UUID{}, webauth.ErrNotFound
		}
		return pgtype.UUID{}, sanitizeErr(err)
	}
	return p.ID, nil
}

// checkOwnerInvariants enforces "đúng một owner active mỗi project" for a
// user patch: every project in before∪after must not drop to 0 or grow past
// 1 active owner. membershipMemberRows are locked per project.
func (a *Accounts) checkOwnerInvariants(ctx context.Context, q *gen.Queries, user gen.User, disabling bool, before, after map[string]webauth.ProjectRole) error {
	slugs := map[string]bool{}
	for s := range before {
		slugs[s] = true
	}
	for s := range after {
		slugs[s] = true
	}
	for slug := range slugs {
		p, err := q.GetProjectBySlug(ctx, slug)
		if err != nil {
			if noRows(err) {
				if after[slug] == webauth.ProjectOwner {
					return webauth.ErrNotFound
				}
				continue
			}
			return sanitizeErr(err)
		}
		// Serialize against concurrent member mutation on this project.
		members, err := q.LockProjectMembers(ctx, p.ID)
		if err != nil {
			return sanitizeErr(err)
		}
		othersActive := 0
		for _, m := range members {
			if m.UserID == user.ID || m.Role != string(webauth.ProjectOwner) {
				continue
			}
			if st, err := memberActive(ctx, q, m.UserID); err != nil {
				return err
			} else if st {
				othersActive++
			}
		}
		beforeOwner := before[slug] == webauth.ProjectOwner && user.Status == "active"
		hadOwner := othersActive > 0 || beforeOwner
		ownersAfter := othersActive
		if after[slug] == webauth.ProjectOwner && !disabling {
			ownersAfter++
		}
		if hadOwner && ownersAfter == 0 {
			return webauth.ErrLastOwner
		}
		if ownersAfter > 1 {
			return webauth.ErrOwnerTaken
		}
	}
	return nil
}

func memberActive(ctx context.Context, q *gen.Queries, userID pgtype.UUID) (bool, error) {
	u, err := q.GetUserByID(ctx, userID)
	if err != nil {
		return false, sanitizeErr(err)
	}
	return u.Status == "active", nil
}

// replaceMemberships diffs a user's project grants inside the tx: creates
// project anchors lazily, upserts/removes member rows and enforces the
// owner invariants. One summary audit line per patch (file parity).
func (a *Accounts) replaceMemberships(ctx context.Context, q *gen.Queries, user gen.User, before, after map[string]webauth.ProjectRole, actor *webauth.AuditActor, reason string) error {
	slugs := map[string]bool{}
	for s := range before {
		slugs[s] = true
	}
	for s := range after {
		slugs[s] = true
	}
	for slug := range slugs {
		b, bOK := before[slug]
		n, nOK := after[slug]
		if bOK && b == n || !bOK && !nOK {
			continue
		}
		p, err := projectBySlug(ctx, q, slug, nOK)
		if err != nil {
			return err
		}
		if !nOK {
			if err := q.RemoveMember(ctx, gen.RemoveMemberParams{ProjectID: p.ID, UserID: user.ID, RemovedAt: ts(a.now())}); err != nil {
				return sanitizeErr(err)
			}
			continue
		}
		if err := q.UpsertMember(ctx, gen.UpsertMemberParams{ProjectID: p.ID, UserID: user.ID, Role: string(n)}); err != nil {
			return constraintErr(err)
		}
	}
	return nil
}

func projectSummary(projects map[string]webauth.ProjectRole) string {
	keys := make([]string, 0, len(projects))
	for k := range projects {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(projects))
	for _, k := range keys {
		parts = append(parts, `"`+k+`":"`+string(projects[k])+`"`)
	}
	return "{" + strings.Join(parts, ",") + "}"
}

// --- scoped member APIs (owner/admin qua /api/projects/:p/members*) ---------

func (a *Accounts) ProjectMembers(project string) ([]webauth.ProjectMember, error) {
	ctx := context.Background()
	q := a.qp()
	p, err := q.GetProjectBySlug(ctx, project)
	if err != nil {
		if noRows(err) {
			return []webauth.ProjectMember{}, nil
		}
		return nil, sanitizeErr(err)
	}
	rows, err := q.ListProjectMembers(ctx, p.ID)
	if err != nil {
		return nil, sanitizeErr(err)
	}
	out := []webauth.ProjectMember{}
	for _, m := range rows {
		out = append(out, webauth.ProjectMember{
			Username: m.Username, Name: m.DisplayName,
			Role: webauth.ProjectRole(m.Role), Disabled: m.UserStatus == "locked",
		})
	}
	return out, nil
}

func (a *Accounts) SetProjectMember(project, username string, role webauth.ProjectRole, actor *webauth.AuditActor) (webauth.UserView, error) {
	if role != webauth.ProjectCollab && role != webauth.ProjectView {
		return webauth.UserView{}, fmt.Errorf("%w: vai trò dự án phải là collab hoặc view", webauth.ErrInvalidInput)
	}
	if !library.ValidProjectID(project) {
		return webauth.UserView{}, fmt.Errorf("%w: dự án không hợp lệ", webauth.ErrInvalidInput)
	}
	err := RunInTx(context.Background(), a.pool, func(tx pgx.Tx) error {
		ctx := context.Background()
		q := a.q(tx)
		row, err := q.GetUserForUpdate(ctx, username)
		if err != nil {
			if noRows(err) {
				return webauth.ErrNotFound
			}
			return sanitizeErr(err)
		}
		p, err := projectBySlug(ctx, q, project, true)
		if err != nil {
			return err
		}
		members, err := q.LockProjectMembers(ctx, p.ID)
		if err != nil {
			return sanitizeErr(err)
		}
		before := ""
		targetWasOwner := false
		for _, m := range members {
			if m.UserID == row.ID {
				before = m.Role
				targetWasOwner = m.Role == string(webauth.ProjectOwner)
			}
		}
		if targetWasOwner {
			othersActive := 0
			for _, m := range members {
				if m.UserID != row.ID && m.Role == string(webauth.ProjectOwner) {
					if active, err := memberActive(ctx, q, m.UserID); err != nil {
						return err
					} else if active {
						othersActive++
					}
				}
			}
			if othersActive == 0 && row.Status == "active" {
				return webauth.ErrLastOwner
			}
		}
		if err := q.UpsertMember(ctx, gen.UpsertMemberParams{ProjectID: p.ID, UserID: row.ID, Role: string(role)}); err != nil {
			return sanitizeErr(err)
		}
		if before != string(role) {
			return a.addAudit(ctx, q, actor, p.Slug, "perm", "Đổi quyền dự án", username, before, string(role), "")
		}
		return nil
	})
	if err != nil {
		return webauth.UserView{}, err
	}
	u, _, err := a.userByUsername(context.Background(), a.qp(), username)
	if err != nil {
		return webauth.UserView{}, err
	}
	return u.View(), nil
}

func (a *Accounts) RemoveProjectMember(project, username string, actor *webauth.AuditActor) error {
	return RunInTx(context.Background(), a.pool, func(tx pgx.Tx) error {
		ctx := context.Background()
		q := a.q(tx)
		row, err := q.GetUserForUpdate(ctx, username)
		if err != nil {
			if noRows(err) {
				return webauth.ErrNotFound
			}
			return sanitizeErr(err)
		}
		p, err := projectBySlug(ctx, q, project, false)
		if err != nil {
			return err
		}
		members, err := q.LockProjectMembers(ctx, p.ID)
		if err != nil {
			return sanitizeErr(err)
		}
		before := ""
		othersActive := 0
		for _, m := range members {
			if m.UserID == row.ID {
				before = m.Role
				continue
			}
			if m.Role == string(webauth.ProjectOwner) {
				if active, err := memberActive(ctx, q, m.UserID); err != nil {
					return err
				} else if active {
					othersActive++
				}
			}
		}
		if before == "" {
			return webauth.ErrNotFound
		}
		if before == string(webauth.ProjectOwner) && othersActive == 0 && row.Status == "active" {
			return webauth.ErrLastOwner
		}
		if err := q.RemoveMember(ctx, gen.RemoveMemberParams{ProjectID: p.ID, UserID: row.ID, RemovedAt: ts(a.now())}); err != nil {
			return sanitizeErr(err)
		}
		return a.addAudit(ctx, q, actor, p.Slug, "perm", "Gỡ quyền dự án", username, before, "", "")
	})
}

// TransferProjectOwner atomically demotes every active owner to collab and
// grants owner to the target user — serialized by member row locks, the
// partial-unique index additionally rejects a second owner at commit.
func (a *Accounts) TransferProjectOwner(project, username string, actor *webauth.AuditActor) error {
	return RunInTx(context.Background(), a.pool, func(tx pgx.Tx) error {
		ctx := context.Background()
		q := a.q(tx)
		row, err := q.GetUserForUpdate(ctx, username)
		if err != nil {
			if noRows(err) {
				return webauth.ErrNotFound
			}
			return sanitizeErr(err)
		}
		if row.Status == "locked" {
			return fmt.Errorf("%w: không thể chuyển quyền cho tài khoản đang bị khoá", webauth.ErrInvalidInput)
		}
		p, err := projectBySlug(ctx, q, project, true)
		if err != nil {
			return err
		}
		members, err := q.LockProjectMembers(ctx, p.ID)
		if err != nil {
			return sanitizeErr(err)
		}
		previous := []string{}
		for _, m := range members {
			if m.Role == string(webauth.ProjectOwner) && m.UserID != row.ID {
				if u, err := q.GetUserByID(ctx, m.UserID); err == nil && u.Status == "active" {
					previous = append(previous, u.Username)
				}
			}
		}
		if err := q.DemoteOwnersToCollab(ctx, gen.DemoteOwnersToCollabParams{ProjectID: p.ID, UpdatedAt: ts(a.now())}); err != nil {
			return sanitizeErr(err)
		}
		if err := q.UpsertMember(ctx, gen.UpsertMemberParams{ProjectID: p.ID, UserID: row.ID, Role: string(webauth.ProjectOwner)}); err != nil {
			return sanitizeErr(err)
		}
		sort.Strings(previous)
		return a.addAudit(ctx, q, actor, p.Slug, "perm", "Chuyển chủ dự án", username, strings.Join(previous, ","), string(webauth.ProjectOwner), "")
	})
}
