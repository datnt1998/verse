package storageimport

// Shared-account cutover: imports the account files of the two apps into the
// shared Postgres store under owner supervision. The rules below are the
// whole safety contract: never merge accounts silently, never write before
// the owner approves a manifest digest, never emit secret material into the
// report, and preserve credentials verbatim (no re-hash, no TTL extension).

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/datnt1998/verse/platform/identity"
)

// identityFileVersion is the only account-file schema the cutover accepts.
// Older files must be upgraded on a copy by the app's own store first (it
// writes its own .vN.bak next to the copy); newer or corrupt files stop the
// whole run with an explicit error.
const identityFileVersion = 4

// App names of the two cutover sources; they double as app_grants.app values.
const (
	SourceNoverse = "noverse"
	SourceViderse = "viderse"
)

// IdentitySource is one frozen account file the owner hands to the command,
// or the explicitly declared absence of that file.
type IdentitySource struct {
	App      string // SourceNoverse | SourceViderse
	SourceID string // stable UUID; deterministic ids are namespaced by it
	Path     string // absolute path to the frozen web-accounts.json
	Empty    bool   // owner-declared "this app never had an account file"
	// Raw, when set, is the content of Path already read by the caller (the
	// library scan in file mode). Decoding those bytes instead of reading
	// the file a second time makes the library pass and the identity pass
	// consume one snapshot by construction.
	Raw []byte
	// Label is the only import_runs.source_label besides "<app>-identity"
	// this source id may legitimately carry — the library-pass label in
	// file mode. Any other label already recorded under this id means the
	// id was recycled from a different source and preflight refuses.
	Label string
}

// RenameEntry is one owner-approved mapping decision. SourceUsername is kept
// verbatim everywhere for provenance (ledger keys, audit details); the
// TargetUsername is what lands in the shared store. Role is optional:
// "member" demotes a source global admin — an explicit owner decision that
// travels inside the approved manifest digest.
type RenameEntry struct {
	SourceApp      string `json:"sourceApp"`
	SourceUsername string `json:"sourceUsername"`
	TargetUsername string `json:"targetUsername,omitempty"`
	Role           string `json:"role,omitempty"`
}

// CutoverCollision is one unresolved conflict found by preflight (or the
// in-transaction recheck). It is the machine-readable explanation of a stop.
type CutoverCollision struct {
	Kind           string `json:"kind"` // two_sources|target_occupied|id_conflict|changed_snapshot|viderse_projects|source_id_reuse
	SourceApp      string `json:"sourceApp,omitempty"`
	SourceUsername string `json:"sourceUsername,omitempty"`
	TargetUsername string `json:"targetUsername,omitempty"`
	Detail         string `json:"detail"`
}

// CutoverPlan is the preflight outcome: decoded and mapped sources, every
// unresolved collision, and the facts an owner must read before approving.
type CutoverPlan struct {
	Fatal      string             `json:"fatal,omitempty"`
	Collisions []CutoverCollision `json:"collisions,omitempty"`
	Renames    []RenameEntry      `json:"renamesApplied,omitempty"`
	// GlobalAdmins lists every account that will hold platform-wide admin
	// (global_role='admin') after apply: existing target admins plus the
	// admins each source file contributes after rename-map renames and
	// demotions. GlobalAdminsDigest covers that exact list — --apply
	// always requires --accept-global-admins equal to it.
	GlobalAdmins       []GlobalAdmin `json:"globalAdmins,omitempty"`
	GlobalAdminsDigest string        `json:"globalAdminsDigest"`
	// LiveUsersWithoutGrants counts live target users with NO app membership
	// at all — an access anomaly worth a manual look (a deliberately revoked
	// grant stays revoked; the tool reports, never re-grants). A live member
	// of the other app alone legitimately lacks the noverse row.
	LiveUsersWithoutGrants int `json:"liveUsersWithoutGrants"`
	// ActiveAdminCountAfter is the post-import platform-admin floor
	// (tombstones/locked never count), checked again inside the tx.
	ActiveAdminCountAfter int `json:"activeAdminCountAfter"`

	sources        []*decodedSource
	targets        map[string]string // canonical username -> user uuid in target
	existingAdmins []GlobalAdmin     // target rows already holding global_role='admin'
	manifestSHA    string
}

// Failed reports whether preflight stopped the run.
func (p *CutoverPlan) Failed() bool {
	return p.Fatal != "" || len(p.Collisions) > 0
}

// Sources exposes the decoded sources for reporting.
func (p *CutoverPlan) Sources() []SourceReport {
	out := make([]SourceReport, 0, len(p.sources))
	for _, s := range p.sources {
		out = append(out, s.report())
	}
	return out
}

// SourceReport is the secret-free per-source summary the report renders.
type SourceReport struct {
	App           string `json:"app"`
	SourceID      string `json:"sourceId"`
	Path          string `json:"path,omitempty"`
	Empty         bool   `json:"empty,omitempty"`
	SHA256        string `json:"sha256,omitempty"`
	Users         int    `json:"users"`
	SessionsDrop  int    `json:"sessionsInvalidated"`
	PairingsDrop  int    `json:"pairingsInvalidated"`
	Audit         int    `json:"auditEntries"`
	ResumeSkips   int    `json:"resumeSkips"`
	BackupPath    string `json:"backupPath,omitempty"`
	BackupCreated bool   `json:"backupCreated,omitempty"`
}

func (s *decodedSource) report() SourceReport {
	r := SourceReport{
		App: s.spec.App, SourceID: s.spec.SourceID, Path: s.spec.Path,
		Empty: s.spec.Empty, SHA256: s.sha256,
		SessionsDrop: s.sessions, PairingsDrop: s.pairings,
	}
	if s.file != nil {
		r.Users = len(s.file.Users)
		r.Audit = len(s.file.Audit)
	}
	for _, mu := range s.mapped {
		if mu.ledgerOK {
			r.ResumeSkips++
		}
	}
	return r
}

// decodedSource is one account file after strict decode + mapping.
type decodedSource struct {
	spec     IdentitySource
	sha256   string
	file     *AccountFile
	mapped   []mappedUser
	ledger   map[string]ledgerRowProbe // record_key -> probe, loaded from target
	sessions int
	pairings int
}

// ledgerRowProbe is the resume evidence one committed record carries.
type ledgerRowProbe struct {
	semantic string
	targetID string
	kind     string
}

// mappedUser is one source account after owner mapping.
type mappedUser struct {
	srcApp   string      // owning source app, for collision reporting
	src      AccountUser // verbatim source record
	target   AccountUser // post-mapping row to write
	targetID string
	skip     bool // a ledger row for this source user exists
	ledgerOK bool // ledger row exists AND matches snapshot+mapping
}

// GlobalAdmin is one account holding platform-wide admin after cutover and
// where that power came from: an existing target row, or the admin role a
// Noverse/Viderse file grants under the approved mapping.
type GlobalAdmin struct {
	Username string `json:"username"`
	Origin   string `json:"origin"` // existing | noverse | viderse
	Locked   bool   `json:"locked,omitempty"`
}

// ── strict decode ────────────────────────────────────────────────────────────

// DecodeIdentitySource reads and validates one frozen v4 account file. Empty
// sources decode to an empty file — never to an implicit empty store — and a
// missing path is always an error.
func DecodeIdentitySource(spec IdentitySource) (*decodedSource, error) {
	if spec.App != SourceNoverse && spec.App != SourceViderse {
		return nil, fmt.Errorf("nguồn %q không hợp lệ", spec.App)
	}
	canonicalID, err := CanonicalSourceID(spec.SourceID)
	if err != nil {
		return nil, fmt.Errorf("nguồn %s: source id %q không phải UUID", spec.App, spec.SourceID)
	}
	spec.SourceID = canonicalID
	src := &decodedSource{spec: spec, ledger: map[string]ledgerRowProbe{}}
	if spec.Empty {
		if spec.Path != "" {
			return nil, fmt.Errorf("nguồn %s: không thể vừa khai empty vừa chỉ file", spec.App)
		}
		src.file = &AccountFile{Version: identityFileVersion}
		return src, nil
	}
	if !filepath.IsAbs(spec.Path) {
		return nil, fmt.Errorf("nguồn %s: đường dẫn phải tuyệt đối (%q)", spec.App, spec.Path)
	}
	data := spec.Raw
	if data == nil {
		read, err := os.ReadFile(spec.Path)
		if err != nil {
			return nil, fmt.Errorf("nguồn %s: đọc %s: %w (không coi file thiếu là kho trống)", spec.App, spec.Path, err)
		}
		data = read
	}
	sum := sha256.Sum256(data)
	src.sha256 = hex.EncodeToString(sum[:])
	var file AccountFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("nguồn %s: web-accounts.json hỏng: %w", spec.App, err)
	}
	switch {
	case file.Version == identityFileVersion:
	case file.Version >= 2 && file.Version < identityFileVersion:
		return nil, fmt.Errorf("nguồn %s: file phiên bản %d — nâng một BẢN SAO lên v%d bằng chính store của app "+
			"(store tự ghi .v%d.bak cạnh bản sao) rồi trỏ lại vào bản sao; không mở file live, không đoán trường cũ",
			spec.App, file.Version, identityFileVersion, file.Version)
	case file.Version > identityFileVersion:
		return nil, fmt.Errorf("nguồn %s: file phiên bản %d mới hơn %d mà công cụ hiểu — cần binary mới hơn",
			spec.App, file.Version, identityFileVersion)
	default:
		return nil, fmt.Errorf("nguồn %s: file thiếu hoặc phiên bản lạ (%d) — chỉ chấp nhận v%d",
			spec.App, file.Version, identityFileVersion)
	}
	if len(file.Users) == 0 {
		return nil, fmt.Errorf("nguồn %s: web-accounts.json không có tài khoản nào; nếu app chưa từng có file hãy chọn chế độ nguồn trống",
			spec.App)
	}
	src.file = &file
	src.sessions = len(file.Sessions)
	src.pairings = len(file.Pairings)
	if err := validateIdentityUsers(spec.App, &file); err != nil {
		return nil, err
	}
	return src, nil
}

// canonicalUsername is the citext/trim/lowercase key used for every
// comparison. The stored value must already be canonical — anything else is a
// structural defect, never something the tool rewrites behind the owner.
func canonicalUsername(name string) (string, bool) {
	c := strings.ToLower(strings.TrimSpace(name))
	return c, c == name && identity.ValidUsername(c)
}

func validateIdentityUsers(app string, file *AccountFile) error {
	known := map[string]bool{}
	for _, u := range file.Users {
		if _, ok := canonicalUsername(u.Username); !ok {
			return fmt.Errorf("nguồn %s: username %q không hợp lệ (canonical a-z0-9._- dài 2..32, không khoảng trắng/hoa)", app, u.Username)
		}
		if known[u.Username] {
			return fmt.Errorf("nguồn %s: username %q lặp trong file", app, u.Username)
		}
		known[u.Username] = true
		if u.Role != "admin" && u.Role != "member" {
			return fmt.Errorf("nguồn %s: tài khoản %q có role lạ %q", app, u.Username, u.Role)
		}
		if u.PasswordHash != "" && !strings.HasPrefix(u.PasswordHash, "$argon2id$") {
			return fmt.Errorf("nguồn %s: tài khoản %q có hash mật khẩu không phải Argon2id", app, u.Username)
		}
		if t := u.TempPassword; t != nil {
			if !strings.HasPrefix(t.CodeHash, "$argon2id$") {
				return fmt.Errorf("nguồn %s: temp password của %q không phải hash Argon2id", app, u.Username)
			}
			if t.CreatedAt.IsZero() || !t.ExpiresAt.After(t.CreatedAt) {
				return fmt.Errorf("nguồn %s: temp password của %q có TTL lạ (expiresAt phải sau createdAt)", app, u.Username)
			}
			if t.UsedAt != nil && !t.UsedAt.After(t.CreatedAt) {
				return fmt.Errorf("nguồn %s: temp password của %q có usedAt trước createdAt", app, u.Username)
			}
		}
		if r := u.Recovery; r != nil && !strings.HasPrefix(r.CodeHash, "$argon2id$") {
			return fmt.Errorf("nguồn %s: recovery code của %q không phải hash Argon2id", app, u.Username)
		}
	}
	for _, u := range file.Users {
		if u.DisabledBy != "" && !known[u.DisabledBy] {
			return fmt.Errorf("nguồn %s: tài khoản %q có disabledBy trỏ tới %q không có trong file", app, u.Username, u.DisabledBy)
		}
		if t := u.TempPassword; t != nil && t.CreatedBy != "" && !known[t.CreatedBy] {
			return fmt.Errorf("nguồn %s: temp password của %q có createdBy trỏ tới %q không có trong file", app, u.Username, t.CreatedBy)
		}
	}
	auditIDs := map[string]bool{}
	for _, a := range file.Audit {
		if a.ID == "" || auditIDs[a.ID] {
			return fmt.Errorf("nguồn %s: audit id %q rỗng hoặc lặp", app, a.ID)
		}
		auditIDs[a.ID] = true
	}
	return nil
}

// LoadRenameMap reads the owner-provided rename/role mapping file. An empty
// path means no mapping; a present path must be an absolute JSON file — an
// absent map is never guessed.
func LoadRenameMap(path string) ([]RenameEntry, error) {
	if path == "" {
		return nil, nil
	}
	if !filepath.IsAbs(path) {
		return nil, fmt.Errorf("--rename-map phải là đường dẫn tuyệt đối (%q)", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("đọc rename map: %w", err)
	}
	var entries []RenameEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("rename map hỏng: %w", err)
	}
	return entries, nil
}

// ── preflight ────────────────────────────────────────────────────────────────

// PlanCutover decodes both sources, applies the owner mapping, inventories
// every username of both sources plus the whole target database (including
// tombstones/locked rows) and classifies every conflict. pool may be nil for
// pure file-side checks; the command always passes the migration pool
// because dry-run must see target collisions too.
func PlanCutover(ctx context.Context, pool *postgres.Pool, noverse, viderse IdentitySource, renames []RenameEntry) (*CutoverPlan, error) {
	plan := &CutoverPlan{targets: map[string]string{}}

	for _, spec := range []IdentitySource{noverse, viderse} {
		if spec.App == "" {
			continue
		}
		src, err := DecodeIdentitySource(spec)
		if err != nil {
			plan.Fatal = err.Error()
			return plan, nil
		}
		plan.sources = append(plan.sources, src)
	}

	// Two distinct files may never share one source id: deterministic
	// user ids are namespaced by it, so reuse across apps would forge the
	// resume ledger of the other source.
	for i, a := range plan.sources {
		if a.spec.Empty {
			continue
		}
		for _, b := range plan.sources[i+1:] {
			if b.spec.Empty {
				continue
			}
			// Decode already canonicalized both ids, so equal strings here are
			// the same UUID even when the owner typed different cases.
			if a.spec.SourceID == b.spec.SourceID {
				plan.Fatal = fmt.Sprintf("hai nguồn %s và %s dùng chung source id %s — mỗi app phải có UUID riêng",
					a.spec.App, b.spec.App, a.spec.SourceID)
				return plan, nil
			}
		}
	}

	// Ledger evidence: the only legal target overlap is a user this exact
	// source+snapshot+mapping already committed.
	if pool != nil {
		if err := plan.loadTarget(ctx, pool); err != nil {
			plan.Fatal = err.Error()
			return plan, nil
		}
		for _, src := range plan.sources {
			if err := src.loadLedger(ctx, pool.Pool()); err != nil {
				plan.Fatal = err.Error()
				return plan, nil
			}
		}
	}

	// Source-id provenance: an id import_runs already recorded under a
	// label this source would never write belongs to a different source —
	// refuse before any write rather than merge two histories.
	if pool != nil {
		for _, src := range plan.sources {
			if src.spec.Empty {
				continue
			}
			plan.checkSourceIDReuse(ctx, pool, src)
			if plan.Fatal != "" {
				return plan, nil
			}
		}
	}

	// Apply the rename map. Unknown apps/users or invalid targets are
	// structural map errors — fix the map file, never silently adjust.
	renamed := map[string]string{}
	roles := map[string]string{}
	plan.Renames = renames
	for _, e := range renames {
		if e.SourceApp != SourceNoverse && e.SourceApp != SourceViderse {
			plan.Fatal = fmt.Sprintf("rename map: app %q không hợp lệ", e.SourceApp)
			return plan, nil
		}
		src := findSource(plan.sources, e.SourceApp)
		if src == nil || userOf(src.file, e.SourceUsername) == nil {
			plan.Fatal = fmt.Sprintf("rename map: %q không có trong nguồn %s", e.SourceUsername, e.SourceApp)
			return plan, nil
		}
		key := e.SourceApp + "\x00" + e.SourceUsername
		if e.TargetUsername != "" {
			if _, ok := canonicalUsername(e.TargetUsername); !ok {
				plan.Fatal = fmt.Sprintf("rename map: target %q không phải username canonical hợp lệ", e.TargetUsername)
				return plan, nil
			}
			renamed[key] = e.TargetUsername
		}
		switch e.Role {
		case "", "admin":
			// empty keeps the source role; admin is an explicit no-op.
		case "member":
			if userOf(src.file, e.SourceUsername).Role != "admin" {
				plan.Fatal = fmt.Sprintf("rename map: role=member chỉ áp dụng để hạ admin (%q đã là member)", e.SourceUsername)
				return plan, nil
			}
			roles[key] = "member"
		default:
			plan.Fatal = fmt.Sprintf("rename map: role %q không hợp lệ (chỉ member để hạ admin)", e.Role)
			return plan, nil
		}
	}

	for _, src := range plan.sources {
		for i := range src.file.Users {
			u := src.file.Users[i]
			target := u
			if t, ok := renamed[src.spec.App+"\x00"+u.Username]; ok {
				target.Username = t
			}
			if r, ok := roles[src.spec.App+"\x00"+u.Username]; ok {
				target.Role = r
			}
			src.mapped = append(src.mapped, mappedUser{
				srcApp:   src.spec.App,
				src:      u,
				target:   target,
				targetID: DeterministicUUID(src.spec.SourceID, "user", u.Username),
				skip:     src.ledgerSkip(u.Username),
			})
		}
	}

	plan.detectCollisions()
	if pool != nil && !plan.Failed() {
		if err := plan.checkTargetGrants(ctx, pool); err != nil {
			plan.Fatal = err.Error()
			return plan, nil
		}
	}
	plan.adminReport()
	return plan, nil
}

func findSource(sources []*decodedSource, app string) *decodedSource {
	for _, s := range sources {
		if s.spec.App == app {
			return s
		}
	}
	return nil
}

func userOf(file *AccountFile, username string) *AccountUser {
	if file == nil {
		return nil
	}
	for i := range file.Users {
		if file.Users[i].Username == username {
			return &file.Users[i]
		}
	}
	return nil
}

// loadTarget inventories every username in the shared store — locked and
// soft-deleted rows included (a tombstone still reserves its name).
func (p *CutoverPlan) loadTarget(ctx context.Context, pool *postgres.Pool) error {
	rows, err := pool.Pool().Query(ctx, `SELECT id, username FROM users`)
	if err != nil {
		return fmt.Errorf("đọc users đích: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id pgtype.UUID
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return err
		}
		p.targets[strings.ToLower(name)] = formatUUIDBytes(id)
	}
	return rows.Err()
}

func (s *decodedSource) loadLedger(ctx context.Context, q querier) error {
	rows, err := q.Query(ctx, `
SELECT record_key, target_kind, target_id, semantic_sha256
FROM import_items WHERE source_id = $1 AND relative_path = $2`,
		uuidArg(s.spec.SourceID), PathAccounts)
	if err != nil {
		return fmt.Errorf("đọc ledger nguồn %s: %w", s.spec.App, err)
	}
	defer rows.Close()
	for rows.Next() {
		var p ledgerRowProbe
		var key string
		if err := rows.Scan(&key, &p.kind, &p.targetID, &p.semantic); err != nil {
			return err
		}
		s.ledger[key] = p
	}
	return rows.Err()
}

// ledgerSkip reports whether the ledger proves this source user was already
// committed under the deterministic id of this exact source.
func (s *decodedSource) ledgerSkip(username string) bool {
	p, ok := s.ledger[username]
	if !ok || p.kind != "user" {
		return false
	}
	return p.targetID == DeterministicUUID(s.spec.SourceID, "user", username)
}

// detectCollisions classifies every unresolved username conflict before any
// write. Sameness of hash/email/display name is never merge evidence; the
// only legal overlap is the ledger-proven resume skip.
func (p *CutoverPlan) detectCollisions() {
	owner := map[string]*mappedUser{}
	for _, src := range p.sources {
		for i := range src.mapped {
			mu := &src.mapped[i]
			if probe, hasLedger := src.ledger[mu.src.Username]; hasLedger && !mu.skip {
				p.Collisions = append(p.Collisions, CutoverCollision{
					Kind:           "id_conflict",
					SourceApp:      src.spec.App,
					SourceUsername: mu.src.Username,
					Detail:         "ledger có bản ghi nguồn này nhưng target id khác — provenance lạ, đối soát tay",
				})
				_ = probe
				continue
			}
			if mu.skip {
				want, err := ItemDigest("user", mu.target)
				if err != nil {
					p.Fatal = err.Error()
					return
				}
				if src.ledger[mu.src.Username].semantic != want {
					p.Collisions = append(p.Collisions, CutoverCollision{
						Kind:           "changed_snapshot",
						SourceApp:      src.spec.App,
						SourceUsername: mu.src.Username,
						TargetUsername: mu.target.Username,
						Detail:         "snapshot hoặc mapping đổi sau lần apply đã commit — không nhập lại dưới cùng source-id/manifest",
					})
					continue
				}
				mu.ledgerOK = true
				continue
			}
			key, _ := canonicalUsername(mu.target.Username)
			if prev, ok := owner[key]; ok {
				p.Collisions = append(p.Collisions, CutoverCollision{
					Kind:           "two_sources",
					SourceApp:      src.spec.App,
					SourceUsername: mu.src.Username,
					TargetUsername: key,
					Detail: fmt.Sprintf("%s:%q và %s:%q cùng chiếm username %q — đổi tên một bên qua rename map, không merge âm thầm",
						prev.srcApp, prev.src.Username, src.spec.App, mu.src.Username, key),
				})
				continue
			}
			owner[key] = mu
			if existingID, ok := p.targets[key]; ok {
				kind := "target_occupied"
				detail := fmt.Sprintf("username %q đã có trong kho đích (kể cả tombstone/locked) — rename map hoặc đối soát tay", key)
				if existingID == mu.targetID {
					kind = "id_conflict"
					detail = "id deterministic đã tồn tại nhưng ledger chưa ghi nhận lần nhập này — đối soát tay, không nhập lại"
				}
				p.Collisions = append(p.Collisions, CutoverCollision{
					Kind:           kind,
					SourceApp:      src.spec.App,
					SourceUsername: mu.src.Username,
					TargetUsername: key,
					Detail:         detail,
				})
			}
		}
		// Audit records are append-only provenance: a ledger row with a
		// different semantic means the source file was edited post-import.
		if src.file != nil {
			for _, a := range src.file.Audit {
				probe, ok := src.ledger[a.ID]
				if !ok {
					continue
				}
				want, err := ItemDigest("audit", a)
				if err != nil {
					p.Fatal = err.Error()
					return
				}
				if probe.semantic != want {
					p.Collisions = append(p.Collisions, CutoverCollision{
						Kind:      "changed_snapshot",
						SourceApp: src.spec.App,
						Detail:    fmt.Sprintf("audit %q đổi sau lần apply — snapshot không đóng băng", a.ID),
					})
				}
			}
		}
	}
	// A Viderse projects map has no domain in the shared store: nonempty is
	// reported and stops the import — never silently dropped or converted.
	if src := findSource(p.sources, SourceViderse); src != nil && src.file != nil {
		for i := range src.mapped {
			if len(src.mapped[i].src.Projects) > 0 {
				p.Collisions = append(p.Collisions, CutoverCollision{
					Kind:           "viderse_projects",
					SourceApp:      src.spec.App,
					SourceUsername: src.mapped[i].src.Username,
					Detail:         "tài khoản Viderse có map dự án không rỗng — kho chung không có domain này; chủ máy kiểm tra trước khi tiếp tục",
				})
			}
		}
	}
}

// checkSourceIDReuse refuses a source id import_runs already recorded under
// a label this run would never write: the identity pass labels its runs
// "<app>-identity" and the library pass labels them after the frozen root.
// Any other label means the UUID was recycled from another app or snapshot.
func (p *CutoverPlan) checkSourceIDReuse(ctx context.Context, pool *postgres.Pool, src *decodedSource) {
	rows, err := pool.Pool().Query(ctx, `SELECT DISTINCT source_label FROM import_runs WHERE source_id = $1`,
		uuidArg(src.spec.SourceID))
	if err != nil {
		p.Fatal = fmt.Sprintf("kiểm tra import_runs cho nguồn %s: %v", src.spec.App, err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var label string
		if err := rows.Scan(&label); err != nil {
			p.Fatal = fmt.Sprintf("kiểm tra import_runs cho nguồn %s: %v", src.spec.App, err)
			return
		}
		if label == src.spec.App+"-identity" || label == src.spec.Label {
			continue
		}
		p.Collisions = append(p.Collisions, CutoverCollision{
			Kind:      "source_id_reuse",
			SourceApp: src.spec.App,
			Detail: fmt.Sprintf("source id %s đã có import_runs với nhãn %q — UUID này thuộc nguồn khác; mỗi app phải có UUID riêng",
				src.spec.SourceID, label),
		})
	}
	if err := rows.Err(); err != nil {
		p.Fatal = fmt.Sprintf("kiểm tra import_runs cho nguồn %s: %v", src.spec.App, err)
	}
}

// checkTargetGrants fills the read-only checks that need the target: live
// users missing the noverse:member row (reported, never auto-granted), the
// committed platform admins (they enter the global-admin approval list with
// origin "existing") and the active-admin floor already in the store.
func (p *CutoverPlan) checkTargetGrants(ctx context.Context, pool *postgres.Pool) error {
	if err := pool.Pool().QueryRow(ctx, `
SELECT count(*) FROM users u
WHERE u.deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM app_grants g WHERE g.user_id = u.id)`,
	).Scan(&p.LiveUsersWithoutGrants); err != nil {
		return fmt.Errorf("kiểm tra app_grants: %w", err)
	}
	rows, err := pool.Pool().Query(ctx, `
SELECT username, status <> 'active' OR deleted_at IS NOT NULL
FROM users WHERE global_role = 'admin' ORDER BY username`)
	if err != nil {
		return fmt.Errorf("kiểm tra admin đích: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var a GlobalAdmin
		if err := rows.Scan(&a.Username, &a.Locked); err != nil {
			return fmt.Errorf("kiểm tra admin đích: %w", err)
		}
		a.Origin = "existing"
		p.existingAdmins = append(p.existingAdmins, a)

	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("kiểm tra admin đích: %w", err)
	}
	return nil
}

// mergedGlobalAdmins folds the committed target admins into the admins the
// source files claim under the approved mapping: a source keeps the origin
// even for a row already committed by an earlier run of this same cutover,
// so the approval list stays identical across resume. Returns the sorted
// list plus the count of ACTIVE admins (tombstones/locked never count).
// preflight and the in-transaction recheck share this function so they can
// never disagree about the digest's meaning.
func mergedGlobalAdmins(sources []*decodedSource, existing []GlobalAdmin) (entries []GlobalAdmin, active int) {
	claimed := map[string]bool{}
	for _, src := range sources {
		for i := range src.mapped {
			mu := &src.mapped[i]
			if mu.target.Role != "admin" {
				continue
			}
			if !claimed[mu.target.Username] {
				claimed[mu.target.Username] = true
				entries = append(entries, GlobalAdmin{
					Username: mu.target.Username,
					Origin:   src.spec.App,
					Locked:   mu.src.Disabled || mu.src.DeletedAt != nil,
				})
			}
			if !mu.src.Disabled && mu.src.DeletedAt == nil {
				active++
			}
		}
	}
	for _, a := range existing {
		if claimed[a.Username] {
			continue // the mapped source claims the origin
		}
		entries = append(entries, a)
		if !a.Locked {
			active++
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Username != entries[j].Username {
			return entries[i].Username < entries[j].Username
		}
		return entries[i].Origin < entries[j].Origin
	})
	return entries, active
}

// adminReport lists every account holding platform admin after apply and
// derives the approval digest plus the post-import active-admin floor. The
// digest is always computed, even over an empty list, so --apply always has
// a concrete approval string to compare.
func (p *CutoverPlan) adminReport() {
	entries, active := mergedGlobalAdmins(p.sources, p.existingAdmins)
	p.ActiveAdminCountAfter = active
	p.GlobalAdmins = entries
	digest, err := SemanticDigest(entries)
	if err != nil {
		p.Fatal = err.Error()
		return
	}
	p.GlobalAdminsDigest = digest
}

// ── manifest / backup ────────────────────────────────────────────────────────

// CutoverManifest is the file the owner approves. Every byte that decides the
// import is inside: both source ids and digests, the rename map, and the
// library manifest of the file-mode library pass.
type CutoverManifest struct {
	Version         int               `json:"version"`
	NoverseStore    string            `json:"noverseStore"` // file|postgres
	NoverseSourceID string            `json:"noverseSourceId,omitempty"`
	NoverseAccounts string            `json:"noverseAccountsPath,omitempty"`
	NoverseSHA256   string            `json:"noverseAccountsSha256,omitempty"`
	NoverseEmpty    bool              `json:"noverseEmpty,omitempty"`
	ViderseSourceID string            `json:"viderseSourceId"`
	ViderseAccounts string            `json:"viderseAccountsPath,omitempty"`
	ViderseSHA256   string            `json:"viderseAccountsSha256,omitempty"`
	ViderseEmpty    bool              `json:"viderseEmpty,omitempty"`
	RenameMap       []RenameEntry     `json:"renameMap,omitempty"`
	LibraryRoot     string            `json:"libraryRoot,omitempty"`
	LibrarySHA256   string            `json:"libraryManifestSha256,omitempty"`
	OwnerMap        map[string]string `json:"ownerMap,omitempty"`
}

// ManifestBytes renders the manifest deterministically; the accepted digest
// is sha256 over exactly these bytes — plain `sha256sum <file>` reproduces
// what --accept-manifest expects.
func ManifestBytes(m *CutoverManifest) ([]byte, error) {
	data, err := CanonicalJSON(m)
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// ManifestSHA256 returns the digest of the rendered manifest.
func ManifestSHA256(m *CutoverManifest) (string, []byte, error) {
	data, err := ManifestBytes(m)
	if err != nil {
		return "", nil, err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), data, nil
}

// ManifestForPlan projects the plan into an approvable manifest.
func ManifestForPlan(plan *CutoverPlan, store, libSHA, libRoot string, ownerMap map[string]string) *CutoverManifest {
	m := &CutoverManifest{
		Version: 1, NoverseStore: store,
		LibraryRoot: libRoot, LibrarySHA256: libSHA, OwnerMap: ownerMap,
	}
	for _, src := range plan.sources {
		switch src.spec.App {
		case SourceNoverse:
			m.NoverseSourceID = src.spec.SourceID
			m.NoverseAccounts = src.spec.Path
			m.NoverseSHA256 = src.sha256
			m.NoverseEmpty = src.spec.Empty
		case SourceViderse:
			m.ViderseSourceID = src.spec.SourceID
			m.ViderseAccounts = src.spec.Path
			m.ViderseSHA256 = src.sha256
			m.ViderseEmpty = src.spec.Empty
		}
	}
	m.RenameMap = plan.Renames
	return m
}

// BackupReceipt is one verified or freshly created .v4.bak entry.
type BackupReceipt struct {
	SourcePath   string `json:"sourcePath"`
	BackupPath   string `json:"backupPath"`
	SourceSHA256 string `json:"sourceSha256"`
	Created      bool   `json:"created"`
}

// BackupSource names one account file that needs a .v4.bak twin.
type BackupSource struct {
	Path   string
	SHA256 string
}

// BackupSources returns the account files of this plan that carry bytes
// (empty sources have no file and no backup).
func (p *CutoverPlan) BackupSources() []BackupSource {
	var out []BackupSource
	for _, src := range p.sources {
		if src == nil || src.spec.Empty || src.spec.Path == "" {
			continue
		}
		out = append(out, BackupSource{Path: src.spec.Path, SHA256: src.sha256})
	}
	return out
}

// BackupSourceFor decodes one source and returns its backup descriptor.
// The strict decode runs first so backup-only mode refuses v2/v3/corrupt
// sources exactly like an import.
func BackupSourceFor(spec IdentitySource) (BackupSource, error) {
	src, err := DecodeIdentitySource(spec)
	if err != nil {
		return BackupSource{}, err
	}
	return BackupSource{Path: src.spec.Path, SHA256: src.sha256}, nil
}

// EnsureSourceBackups verifies or creates <source>.v4.bak next to every
// account file: O_EXCL, mode 0600, byte-identical to the source, fsynced
// file and directory. An existing backup is opened once with O_NOFOLLOW and
// every check happens on that handle — regular file, owned by the current
// user, perm&0o077==0 — and the bytes compared are read from it, so a
// symlink swap or a raced file substitution lands in the checks instead of
// slipping between them. A byte-different backup stops the run — it is
// never overwritten. A source whose digest drifted from the approved
// manifest stops before any write.
func EnsureSourceBackups(sources []BackupSource) ([]BackupReceipt, error) {
	var receipts []BackupReceipt
	for _, src := range sources {
		if src.Path == "" {
			continue
		}
		want, err := os.ReadFile(src.Path)
		if err != nil {
			return nil, fmt.Errorf("đọc lại nguồn %s trước backup: %w", src.Path, err)
		}
		sum := sha256.Sum256(want)
		if hex.EncodeToString(sum[:]) != src.SHA256 {
			return nil, fmt.Errorf("nguồn %s đổi sau preflight — snapshot không đóng băng; chạy lại dry-run", src.Path)
		}
		backup := src.Path + ".v4.bak"
		r := BackupReceipt{SourcePath: src.Path, BackupPath: backup, SourceSHA256: src.SHA256}
		bf, openErr := openNoFollow(backup)
		switch {
		case openErr == nil:
			info, err := bf.Stat()
			if err != nil {
				bf.Close()
				return nil, fmt.Errorf("fstat backup %s: %w", backup, err)
			}
			if !info.Mode().IsRegular() {
				bf.Close()
				return nil, fmt.Errorf("backup %s không phải file thường (symlink/đặc biệt) — dừng, xoá thủ công sau khi kiểm tra", backup)
			}
			if info.Mode().Perm()&0o077 != 0 {
				bf.Close()
				return nil, fmt.Errorf("backup %s có quyền %o lộ group/other — backup chứa hash tài khoản phải là 0600; dừng, xoá thủ công sau khi kiểm tra",
					backup, info.Mode().Perm())
			}
			if !ownedByCurrentUser(info) {
				bf.Close()
				return nil, fmt.Errorf("backup %s thuộc uid khác — không tin backup người khác đặt; dừng, xoá thủ công sau khi kiểm tra", backup)
			}
			existing, err := io.ReadAll(bf)
			bf.Close()
			if err != nil {
				return nil, fmt.Errorf("đọc backup %s: %w", backup, err)
			}
			if !bytes.Equal(want, existing) {
				got := sha256.Sum256(existing)
				return nil, fmt.Errorf("backup %s đã tồn tại với digest %x khác nguồn %s — dừng, không ghi đè",
					backup, got[:8], src.SHA256[:8])
			}
			receipts = append(receipts, r)
			continue
		case symlinkOpenError(openErr):
			return nil, fmt.Errorf("backup %s là symlink — dừng, xoá thủ công sau khi kiểm tra", backup)
		case !errors.Is(openErr, os.ErrNotExist):
			return nil, fmt.Errorf("kiểm tra backup %s: %w", backup, openErr)
		}
		f, err := os.OpenFile(backup, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			return nil, fmt.Errorf("tạo backup %s: %w", backup, err)
		}
		if _, err := f.Write(want); err != nil {
			f.Close()
			os.Remove(backup)
			return nil, fmt.Errorf("ghi backup %s: %w", backup, err)
		}
		if err := f.Sync(); err != nil {
			f.Close()
			os.Remove(backup)
			return nil, fmt.Errorf("fsync backup %s: %w", backup, err)
		}
		if err := f.Close(); err != nil {
			os.Remove(backup)
			return nil, fmt.Errorf("đóng backup %s: %w", backup, err)
		}
		if dir, err := os.Open(filepath.Dir(backup)); err == nil {
			_ = dir.Sync()
			_ = dir.Close()
		}
		r.Created = true
		receipts = append(receipts, r)
	}
	return receipts, nil
}

// pgDumpMagic opens every pg_dump custom-format archive — the only format
// accepted as pre-cutover database backup.
const pgDumpMagic = "PGDMP"

// errSymlinkRefused marks an open that stopped at a symlink: raised by
// O_NOFOLLOW (ELOOP) on unix and by the lstat guard elsewhere.
var errSymlinkRefused = errors.New("symlink không được theo")

// VerifiedBackup is the dump archive apply accepted as the pre-cutover
// database backup.
type VerifiedBackup struct {
	Path          string `json:"path"`
	SHA256        string `json:"sha256"`
	Format        string `json:"format"`
	PGRestore     string `json:"pgRestore"`
	PGRestorePath string `json:"pgRestorePath"`

	stats *backupStats
}

// CheckPrivateBackupDir checks the owner-only directory used for backup
// evidence and immutable inventories. It never changes its permissions.
func CheckPrivateBackupDir(dir string) error {
	if !filepath.IsAbs(dir) {
		return fmt.Errorf("--backup-dir phải tuyệt đối (%q)", dir)
	}
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("đọc --backup-dir: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("--backup-dir %s không phải thư mục", dir)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("--backup-dir %s có quyền group/other, không private — chạy chmod 700 trước khi tiếp tục", dir)
	}
	if !ownedByCurrentUser(info) {
		return fmt.Errorf("--backup-dir %s thuộc uid khác — không tin thư mục backup người khác kiểm soát", dir)
	}
	return nil
}

// CheckBackupDir verifies custom archives from one no-follow handle each.
// Every base table must have TABLE DATA and COPY data matching the live row
// count and COPY timestamptz maxima. No wall-clock timestamp is evidence.
// Passing candidates are ordered by fstat mtime, then ascending path.
func CheckBackupDir(ctx context.Context, pool *postgres.Pool, dir string) (*VerifiedBackup, error) {
	if dir == "" {
		return nil, errors.New("thiếu --backup-dir")
	}
	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("phân giải --backup-dir: %w", err)
	}
	if err := CheckPrivateBackupDir(dir); err != nil {
		return nil, err
	}
	pgRestore, err := exec.LookPath("pg_restore")
	if err != nil {
		return nil, errors.New("không tìm thấy pg_restore trên PATH — không thể kiểm chứng backup nên apply từ chối (cài postgresql-client rồi chạy lại)")
	}
	pgRestore, err = filepath.Abs(pgRestore)
	if err == nil {
		pgRestore, err = filepath.EvalSymlinks(pgRestore)
	}
	if err != nil {
		return nil, fmt.Errorf("phân giải đường dẫn pg_restore: %w", err)
	}
	versionCmd := exec.CommandContext(ctx, pgRestore, "--version")
	versionCmd.Env = pgRestoreEnv()
	versionOut, err := versionCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("chạy pg_restore --version: %v", execError(err))
	}
	pgRestoreVersion := strings.TrimSpace(string(versionOut))

	var dbName, usersSchema string
	if err := pool.Pool().QueryRow(ctx, `SELECT current_database()`).Scan(&dbName); err != nil {
		return nil, fmt.Errorf("đọc tên database đích: %w", err)
	}
	if err := pool.Pool().QueryRow(ctx, `
SELECT n.nspname FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE c.oid = 'users'::regclass`).Scan(&usersSchema); err != nil {
		return nil, fmt.Errorf("phân giải schema của bảng users: %w", err)
	}
	tables, err := backupTables(ctx, pool.Pool(), usersSchema)
	if err != nil {
		return nil, err
	}
	if len(tables) == 0 {
		return nil, fmt.Errorf("schema %s không có bảng base nào — kho đích chưa được migrate?", usersSchema)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("đọc --backup-dir: %w", err)
	}
	type candidate struct {
		path string
		f    *os.File // the single O_NOFOLLOW handle; every check uses it
		fi   os.FileInfo
	}
	var candidates []candidate
	var reasons []string
	for _, e := range entries {
		p := filepath.Join(dir, e.Name())
		if e.IsDir() {
			reasons = append(reasons, p+": directory không được chấp nhận — chỉ nhận file pg_dump --format=custom")
			continue
		}
		if e.Type()&os.ModeSymlink != 0 {
			reasons = append(reasons, p+": symlink không được theo — backup phải là file thường")
			continue
		}
		if !e.Type().IsRegular() && e.Type() != 0 {
			reasons = append(reasons, p+": không phải file thường")
			continue
		}
		f, err := openNoFollow(p)
		if err != nil {
			reasons = append(reasons, fmt.Sprintf("%s: mở an toàn thất bại (%v)", p, err))
			continue
		}
		fi, err := f.Stat()
		if err != nil {
			f.Close()
			reasons = append(reasons, fmt.Sprintf("%s: fstat thất bại (%v)", p, err))
			continue
		}
		switch {
		case !fi.Mode().IsRegular():
			f.Close()
			reasons = append(reasons, p+": không phải file thường")
			continue
		case fi.Mode().Perm()&0o077 != 0:
			f.Close()
			reasons = append(reasons, fmt.Sprintf("%s: file backup không private — chạy chmod 600 trước khi tiếp tục", p))
			continue
		case !ownedByCurrentUser(fi):
			f.Close()
			reasons = append(reasons, p+": thuộc uid khác")
			continue
		}
		head := make([]byte, len(pgDumpMagic))
		if _, err := io.ReadFull(f, head); err != nil {
			f.Close()
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				continue // too small to be an archive — not a candidate
			}
			reasons = append(reasons, fmt.Sprintf("%s: đọc magic thất bại (%v)", p, err))
			continue
		}
		if string(head) != pgDumpMagic {
			f.Close()
			continue // not a pg_dump archive — not a candidate
		}
		candidates = append(candidates, candidate{path: p, f: f, fi: fi})
	}
	defer func() {
		for _, c := range candidates {
			c.f.Close()
		}
	}()
	if len(candidates) == 0 {
		detail := ""
		if len(reasons) > 0 {
			detail = " — " + strings.Join(reasons, "; ")
		}
		return nil, fmt.Errorf("--backup-dir %s không có bản dump pg_dump --format=custom nào — chạy pg_dump --format=custom vào thư mục này trước%s", dir, detail)
	}
	var best *VerifiedBackup
	var bestInfo os.FileInfo
	for _, c := range candidates {
		vb, err := verifyDump(ctx, pgRestore, pgRestoreVersion, c.f, c.path, c.fi, dbName, usersSchema, tables)
		if err == nil {
			err = vb.stats.compareLive(ctx, pool.Pool())
		}
		if err != nil {
			reasons = append(reasons, fmt.Sprintf("%s: %v", c.path, err))
			continue
		}
		if best == nil || c.fi.ModTime().After(bestInfo.ModTime()) ||
			(c.fi.ModTime().Equal(bestInfo.ModTime()) && c.path < best.Path) {
			best = vb
			bestInfo = c.fi
		}
	}
	if best == nil {
		return nil, fmt.Errorf("không có bản dump nào trong --backup-dir %s đạt kiểm chứng — %s",
			dir, strings.Join(reasons, "; "))
	}
	return best, nil
}

// verifyDump checks TOC coverage and streams every data block through the
// COPY aggregate parser. The full restore pass also reads non-data entries;
// both use the same inherited handle and a minimal environment.
func verifyDump(ctx context.Context, pgRestore, pgRestoreVersion string, f *os.File, path string, fi os.FileInfo,
	dbName, usersSchema string, tables []backupTable) (*VerifiedBackup, error) {
	arg := dumpRestorePath(path)
	run := func(args ...string) *exec.Cmd {
		// Explicit format avoids autodetection reopening /dev/fd/3 after
		// consuming its shared offset on Darwin.
		cmd := exec.CommandContext(ctx, pgRestore, append([]string{"--format=custom"}, args...)...)
		cmd.ExtraFiles = []*os.File{f}
		cmd.Env = pgRestoreEnv()
		return cmd
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek về đầu archive: %w", err)
	}
	list, err := run("--list", arg).Output()
	if err != nil {
		return nil, fmt.Errorf("pg_restore --list thất bại: %v", execError(err))
	}
	var listedDB string
	tableData := map[string]bool{}
	for _, line := range strings.Split(string(list), "\n") {
		comment := strings.TrimSpace(strings.TrimPrefix(line, ";"))
		if name, ok := strings.CutPrefix(comment, "dbname:"); ok {
			listedDB = unquote(strings.TrimSpace(name))
			continue
		}
		if i := strings.Index(line, " TABLE DATA "); i >= 0 {
			rest := strings.TrimSpace(line[i+len(" TABLE DATA "):])
			for _, table := range tables {
				// TOC names may be emitted raw or SQL-quoted, depending
				// on the identifier and client version.
				for _, names := range []string{
					usersSchema + " " + table.name + " ",
					pgx.Identifier{usersSchema}.Sanitize() + " " + pgx.Identifier{table.name}.Sanitize() + " ",
				} {
					if strings.HasPrefix(rest, names) {
						tableData[table.name] = true
					}
				}
			}
		}
	}
	if listedDB == "" {
		return nil, errors.New("archive thiếu header dbname — không phải dump hợp lệ")
	}
	if listedDB != dbName {
		return nil, fmt.Errorf("dump của database %q, không phải đích %q", listedDB, dbName)
	}
	var missing []string
	for _, table := range tables {
		if !tableData[table.name] {
			missing = append(missing, table.name)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("TOC thiếu TABLE DATA cho %s.%s — dump phải chứa MỌI bảng base của schema chứa users, không phải bản -t chọn lọc",
			usersSchema, strings.Join(missing, ", "+usersSchema+"."))
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek về đầu archive: %w", err)
	}
	dataCmd := run("--data-only", "-f", "-", arg)
	stream, err := dataCmd.StdoutPipe()
	if err != nil {
		return nil, errors.New("mở luồng COPY từ archive thất bại")
	}
	// Do not retain stderr: a corrupt archive can print row material.
	if err := dataCmd.Start(); err != nil {
		return nil, errors.New("khởi chạy pg_restore đọc COPY thất bại")
	}
	stats, parseErr := readDumpStats(stream, usersSchema, tables)
	if parseErr != nil {
		_ = dataCmd.Process.Kill()
	}
	waitErr := dataCmd.Wait()
	if parseErr != nil {
		return nil, parseErr
	}
	if waitErr != nil {
		return nil, errors.New("pg_restore không đọc được toàn bộ dữ liệu archive")
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek về đầu archive: %w", err)
	}
	if err := run("--file="+os.DevNull, arg).Run(); err != nil {
		return nil, errors.New("pg_restore không đọc được toàn bộ archive")
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek về đầu archive: %w", err)
	}
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return nil, fmt.Errorf("băm %s: %w", path, err)
	}
	after, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("fstat lại %s: %w", path, err)
	}
	if !backupHandleUnchanged(fi, after) {
		return nil, errors.New("file đổi size/mtime/ctime/inode/dev trong lúc kiểm chứng — archive bị thay giữa chừng")
	}
	return &VerifiedBackup{
		Path: path, SHA256: hex.EncodeToString(h.Sum(nil)), Format: "custom",
		PGRestore: pgRestoreVersion, PGRestorePath: pgRestore, stats: stats,
	}, nil
}

// pgRestoreEnv gives pg_restore a minimal environment: only PATH, LANG,
// LC_ALL and TMPDIR propagate, and only when set. No PG* variable may steer
// the verification.
func pgRestoreEnv() []string {
	var env []string
	for _, k := range []string{"PATH", "LANG", "LC_ALL", "TMPDIR"} {
		if v, ok := os.LookupEnv(k); ok {
			env = append(env, k+"="+v)
		}
	}
	return env
}

// unquote strips one layer of double quoting pg_restore uses for non-simple
// identifiers.
func unquote(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return strings.ReplaceAll(s[1:len(s)-1], `""`, `"`)
	}
	return s
}

func execError(err error) error {
	if ee, ok := err.(*exec.ExitError); ok {
		if stderr := strings.TrimSpace(string(ee.Stderr)); stderr != "" {
			return fmt.Errorf("%v — %s", err, stderr)
		}
	}
	return err
}

// ── apply ────────────────────────────────────────────────────────────────────

// cutoverLockKey serializes the whole identity phase across both sources.
// Per-source locks additionally guard each source id against the legacy
// importer, which takes the same lockKey advisory lock.
func cutoverLockKey() int64 { return lockKey("verse/sso-cutover/identity") }

// ApplyResult is the committed outcome of the identity transaction.
type ApplyResult struct {
	Inserted        int    `json:"usersInserted"`
	Skipped         int    `json:"usersSkipped"`
	AuditInserted   int    `json:"auditInserted"`
	Grants          int    `json:"grantsInserted"`
	SessionsRevoked int    `json:"sessionsRevoked"`
	CutoverRunID    string `json:"cutoverRunId"`
	PairingsRevoked int    `json:"pairingsRevoked"`
}

// ApplyCutover commits both sources inside ONE identity transaction: the
// advisory cutover lock plus per-source locks and auth_invariant_lock in the
// order normal account mutations use, a collision recheck against the
// now-locked target, strict inserts (a unique violation is a conflict, never
// an ON CONFLICT swallow), per-app grants, provenance ledger rows and the
// revocation of every live Postgres session/pairing. Any failure rolls back
// both sources. manifestSHA is the owner-approved manifest digest; inventoryPath
// identifies this apply's immutable backup inventory. Both are required.
func ApplyCutover(ctx context.Context, pool *postgres.Pool, plan *CutoverPlan, backup *VerifiedBackup, manifestSHA, inventoryPath string, now time.Time) (*ApplyResult, error) {
	if plan.Failed() {
		return nil, fmt.Errorf("preflight chưa sạch (%d xung đột, fatal=%q) — không apply",
			len(plan.Collisions), plan.Fatal)
	}
	if manifestSHA == "" {
		return nil, errors.New("thiếu manifest digest đã duyệt — apply không được bỏ qua bước phê duyệt")
	}
	if backup == nil || backup.stats == nil {
		return nil, errors.New("thiếu backup đã đối chiếu thống kê với kho đích — không apply")
	}
	if inventoryPath == "" {
		return nil, errors.New("thiếu đường dẫn backup inventory — không apply")
	}
	inventoryPath, err := filepath.Abs(inventoryPath)
	if err != nil {
		return nil, fmt.Errorf("phân giải đường backup inventory: %w", err)
	}
	dumpPath, err := filepath.Abs(backup.Path)
	if err != nil {
		return nil, fmt.Errorf("phân giải đường backup dump: %w", err)
	}
	plan.manifestSHA = manifestSHA
	cutoverSourceID := DeterministicUUID("verse", "sso-cutover-identity", inventoryPath)
	res := &ApplyResult{CutoverRunID: RunIDFor(cutoverSourceID, manifestSHA)}
	err = postgres.RunInTx(ctx, pool.Pool(), func(tx pgx.Tx) error {
		// FOR UPDATE NOWAIT covers only the row lock; a statement can still
		// queue behind a table lock (open DDL, VACUUM FULL, manual LOCK
		// TABLE). The timeout turns that wait into the same 55P03 refusal.
		if _, err := tx.Exec(ctx, `SET LOCAL lock_timeout = '1s'`); err != nil {
			return fmt.Errorf("đặt lock_timeout cho cutover: %w", err)
		}
		// Lock order: cutover → per-source (stable decode order) → invariant
		// row → all base tables → user rows.
		var ok bool
		if err := tx.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock($1)`, cutoverLockKey()).Scan(&ok); err != nil {
			return fmt.Errorf("khoá cutover: %w", err)
		}
		if !ok {
			return errors.New("đã có một tiến trình cutover khác đang chạy — không nhập hai lượt")
		}
		for _, src := range plan.sources {
			if src.spec.Empty {
				continue
			}
			if err := tx.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock($1)`, lockKey(src.spec.SourceID)).Scan(&ok); err != nil {
				return fmt.Errorf("khoá nguồn %s: %w", src.spec.App, err)
			}
			if !ok {
				return fmt.Errorf("đã có importer khác chạy cho nguồn %s", src.spec.App)
			}
		}
		invariantTable := pgx.Identifier{backup.stats.schema, "auth_invariant_lock"}.Sanitize()
		if _, err := tx.Exec(ctx, "SELECT id FROM "+invariantTable+" WHERE id = 1 FOR UPDATE NOWAIT"); err != nil {
			return cutoverLockError(invariantTable, err)
		}
		// READ COMMITTED sees every write committed before these locks.
		// EXCLUSIVE blocks later writers and holds through commit, so no
		// repeatable-read snapshot is needed for the aggregate recheck.
		if err := backup.stats.lockLiveTables(ctx, tx); err != nil {
			return err
		}
		// The approved admin set must still describe reality: recompute the
		// digest over the now-locked store plus the mapped sources and roll
		// back if an admin row was granted/revoked since preflight.
		txAdminsRows, err := tx.Query(ctx, `
SELECT username, status <> 'active' OR deleted_at IS NOT NULL
FROM users WHERE global_role = 'admin' ORDER BY username`)
		if err != nil {
			return fmt.Errorf("kiểm tra lại admin trong tx: %w", err)
		}
		var txExisting []GlobalAdmin
		for txAdminsRows.Next() {
			var a GlobalAdmin
			if err := txAdminsRows.Scan(&a.Username, &a.Locked); err != nil {
				txAdminsRows.Close()
				return fmt.Errorf("kiểm tra lại admin trong tx: %w", err)
			}
			a.Origin = "existing"
			txExisting = append(txExisting, a)
		}
		txAdminsRows.Close()
		if err := txAdminsRows.Err(); err != nil {
			return fmt.Errorf("kiểm tra lại admin trong tx: %w", err)
		}
		txEntries, _ := mergedGlobalAdmins(plan.sources, txExisting)
		txDigest, err := SemanticDigest(txEntries)
		if err != nil {
			return err
		}
		baselineDigest, err := SemanticDigest(plan.existingAdmins)
		if err != nil {
			return err
		}
		currentDigest, err := SemanticDigest(txExisting)
		if err != nil {
			return err
		}
		// Source origins intentionally hide resumed rows in the merged list;
		// compare target evidence too so a resumed admin's demotion or lock
		// cannot be masked by its old source role.
		if txDigest != plan.GlobalAdminsDigest || currentDigest != baselineDigest {
			return errors.New("danh sách admin nền tảng trong kho đã đổi sau preflight — rollback; chạy lại dry-run rồi duyệt digest mới")
		}
		if err := backup.stats.compareLive(ctx, tx); err != nil {
			return fmt.Errorf("backup không còn khớp kho đích trong giao dịch — rollback: %w", err)
		}

		// Recheck every non-skipped target username against the locked store:
		// a rename/create that raced preflight still fails the whole tx.
		for _, src := range plan.sources {
			for i := range src.mapped {
				mu := &src.mapped[i]
				if mu.skip {
					continue
				}
				key, _ := canonicalUsername(mu.target.Username)
				var existing string
				err := tx.QueryRow(ctx, `SELECT id::text FROM users WHERE username = $1`, key).Scan(&existing)
				if err == nil {
					return fmt.Errorf("username %q vừa xuất hiện trong kho đích sau preflight — dừng cả cutover", key)
				}
				if !errors.Is(err, pgx.ErrNoRows) {
					return err
				}
			}
		}
		// Users first without disabled_by / temp created_by, then resolve the
		// references so a disabler imported later in the same tx still binds.
		for _, src := range plan.sources {
			if src.spec.Empty {
				continue
			}
			if err := src.applyUsers(ctx, tx, plan, res, now); err != nil {
				return err
			}
		}
		for _, src := range plan.sources {
			if src.spec.Empty {
				continue
			}
			if err := src.applyUserRefs(ctx, tx); err != nil {
				return err
			}
			if err := src.applyAudit(ctx, tx, res); err != nil {
				return err
			}
			if err := src.writeLedger(ctx, tx, plan); err != nil {
				return err
			}
		}
		// Every device logs in again under the shared store: file bearer rows
		// were never imported, and any live Postgres session or pairing dies
		// inside the same transaction.
		tag, err := tx.Exec(ctx, `UPDATE sessions SET revoked_at = now(), updated_at = now(), version = version + 1 WHERE revoked_at IS NULL`)
		if err != nil {
			return fmt.Errorf("thu hồi sessions: %w", err)
		}
		res.SessionsRevoked = int(tag.RowsAffected())
		tag, err = tx.Exec(ctx, `UPDATE device_pairings SET revoked_at = now(), updated_at = now(), version = version + 1 WHERE revoked_at IS NULL`)
		if err != nil {
			return fmt.Errorf("thu hồi pairings: %w", err)
		}
		res.PairingsRevoked = int(tag.RowsAffected())
		// At least one active platform admin must exist after the mapping;
		// tombstones and locked accounts never count.
		var admins int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM users WHERE global_role='admin' AND status='active' AND deleted_at IS NULL`).Scan(&admins); err != nil {
			return err
		}
		if admins == 0 {
			return errors.New("mapping để lại 0 admin nền tảng đang hoạt động — tombstone/locked không được tính")
		}
		// This receipt exists exactly when this identity transaction commits.
		// Keep one per apply, including empty sources and library resumes.
		summary := map[string]any{
			"cutoverBackup": map[string]string{
				"inventoryPath": inventoryPath,
				"dumpPath":      dumpPath,
				"dumpSha256":    backup.SHA256,
			},
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO import_runs (id, source_id, manifest_sha256, status, importer_version, source_label, summary, started_at, verified_at, finished_at)
VALUES ($1, $2, $3, 'verified', $4, 'sso-cutover-identity', $5, $6, $6, $6)`,
			uuidArg(res.CutoverRunID), uuidArg(cutoverSourceID), manifestSHA, ImporterVersion, mustJSON(summary), now); err != nil {
			return fmt.Errorf("ghi bản ghi backup cho identity transaction — rollback: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// applyUsers inserts one source's users + credentials + grants. Inserts are
// strict: a collision is an error the transaction surfaces, never a skip.
func (s *decodedSource) applyUsers(ctx context.Context, tx pgx.Tx, plan *CutoverPlan, res *ApplyResult, now time.Time) error {
	runID := RunIDFor(s.spec.SourceID, plan.manifestSHA)
	// The row exists only if this transaction commits, so it is written as
	// 'verified' directly; a rolled-back run leaves no 'running' ghost.
	if _, err := tx.Exec(ctx, `
INSERT INTO import_runs (id, source_id, manifest_sha256, status, importer_version, source_label, started_at, verified_at, finished_at)
VALUES ($1, $2, $3, 'verified', $4, $5, $6, $6, $6)
ON CONFLICT (id) DO NOTHING`,
		uuidArg(runID), uuidArg(s.spec.SourceID), plan.manifestSHA, ImporterVersion, s.spec.App+"-identity", now); err != nil {
		return fmt.Errorf("tạo import_run %s: %w", s.spec.App, err)
	}
	for i := range s.mapped {
		mu := &s.mapped[i]
		if mu.skip {
			res.Skipped++
			continue
		}
		u := mu.target
		status := "active"
		if u.Disabled || u.DeletedAt != nil {
			status = "locked"
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO users (id, username, email, display_name, global_role, status, must_change_password,
    onboarded_at, disabled_at, disabled_reason, deleted_at, created_at, updated_at)
VALUES ($1,$2,NULL,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
			uuidArg(mu.targetID), u.Username, u.Name, u.Role, status, u.MustChangePassword,
			u.OnboardedAt, u.DisabledAt, u.DisabledReason, u.DeletedAt,
			tsOr(u.CreatedAt), tsOr(u.UpdatedAt)); err != nil {
			return fmt.Errorf("%s:%q: %w (username/id đụng dòng khác — không ghi đè)", s.spec.App, mu.src.Username, err)
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO user_credentials (user_id, password_hash, password_changed_at) VALUES ($1,$2,$3)`,
			uuidArg(mu.targetID), nullableText(u.PasswordHash), u.PasswordChangedAt); err != nil {
			return fmt.Errorf("%s:%q credential: %w", s.spec.App, mu.src.Username, err)
		}
		if r := u.Recovery; r != nil {
			if _, err := tx.Exec(ctx, `
INSERT INTO recovery_codes (id, user_id, code_hash, mask, used_at, created_at) VALUES ($1,$2,$3,$4,$5,$6)`,
				uuidArg(DeterministicUUID(s.spec.SourceID, "recovery", mu.src.Username)),
				uuidArg(mu.targetID), r.CodeHash, nullableText(r.Mask), r.UsedAt, tsOr(r.CreatedAt)); err != nil {
				return fmt.Errorf("%s:%q recovery: %w", s.spec.App, mu.src.Username, err)
			}
		}
		// Per-app grant: a live account gains membership of ITS OWN app only,
		// including locked accounts (unlock must not lose access); tombstones
		// get nothing and no app ever grants the other side.
		if u.DeletedAt == nil {
			if _, err := tx.Exec(ctx, `
INSERT INTO app_grants (user_id, app, role) VALUES ($1,$2,'member')`,
				uuidArg(mu.targetID), s.spec.App); err != nil {
				return fmt.Errorf("%s:%q grant: %w", s.spec.App, mu.src.Username, err)
			}
			res.Grants++
		}
		res.Inserted++
	}
	return nil
}

// applyUserRefs resolves disabled_by and temp created_by now that every user
// of this source exists. References keep the SOURCE username namespace;
// rename targets live only in the target column.
func (s *decodedSource) applyUserRefs(ctx context.Context, tx pgx.Tx) error {
	for i := range s.mapped {
		mu := &s.mapped[i]
		if mu.skip {
			continue
		}
		if mu.src.DisabledBy != "" {
			byID := DeterministicUUID(s.spec.SourceID, "user", mu.src.DisabledBy)
			if _, err := tx.Exec(ctx, `UPDATE users SET disabled_by = $1, updated_at = now(), version = version + 1 WHERE id = $2`,
				uuidArg(byID), uuidArg(mu.targetID)); err != nil {
				return fmt.Errorf("%s:%q disabled_by: %w", s.spec.App, mu.src.Username, err)
			}
		}
		if t := mu.src.TempPassword; t != nil {
			// Inserted in the refs phase so created_by binds directly; the
			// source TTL/used state is preserved verbatim — a used or expired
			// one-shot code never comes back as an open password.
			var createdBy pgtype.UUID
			if t.CreatedBy != "" {
				createdBy = uuidArg(DeterministicUUID(s.spec.SourceID, "user", t.CreatedBy))
			}
			if _, err := tx.Exec(ctx, `
INSERT INTO temporary_passwords (id, user_id, password_hash, expires_at, used_at, created_by, created_at)
VALUES ($1,$2,$3,$4,$5,$6,$7)`,
				uuidArg(DeterministicUUID(s.spec.SourceID, "temp_password", mu.src.Username)),
				uuidArg(mu.targetID), t.CodeHash, t.ExpiresAt, t.UsedAt, createdBy, t.CreatedAt); err != nil {
				return fmt.Errorf("%s:%q temp password: %w", s.spec.App, mu.src.Username, err)
			}
		}
	}
	return nil
}

// applyAudit copies the source audit trail with source-namespaced ids. The
// file audit records actors/actions/targets only — hashes and tokens never
// appear in entries, so the details copy stays secret-free.
func (s *decodedSource) applyAudit(ctx context.Context, tx pgx.Tx, res *ApplyResult) error {
	for _, a := range s.file.Audit {
		if _, ok := s.ledger[a.ID]; ok {
			continue // ledger-proven resume
		}
		var actorUser pgtype.UUID
		actorKind := "import"
		var actorName string
		if len(a.Actor) > 0 && string(a.Actor) != "null" {
			var actor struct {
				Username string `json:"username"`
			}
			if err := json.Unmarshal(a.Actor, &actor); err == nil {
				actorName = actor.Username
				if userOf(s.file, actor.Username) != nil {
					actorUser = uuidArg(DeterministicUUID(s.spec.SourceID, "user", actor.Username))
					actorKind = "user"
				}
			}
		}
		details := map[string]any{"sourceType": a.Type, "sourceApp": s.spec.App, "entry": a}
		if actorName != "" {
			details["sourceActor"] = actorName
		}
		// project_id stays NULL inside the identity transaction: the project
		// row lands later in the same source's library pass, and the source
		// slug survives in details.
		projectID := pgtype.UUID{}
		if a.Project != "" {
			details["sourceProject"] = a.Project
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO audit_logs (id, project_id, actor_user_id, actor_kind, action, target_kind, target_id, details, occurred_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			uuidArg(DeterministicUUID(s.spec.SourceID, "audit", a.ID)), projectID, actorUser, actorKind,
			firstNonEmpty(a.Action, a.Type), firstNonEmpty(a.Type, "account"), a.Target,
			mustJSON(details), tsOr(a.At)); err != nil {
			return fmt.Errorf("%s audit %q: %w", s.spec.App, a.ID, err)
		}
		res.AuditInserted++
	}
	return nil
}

// writeLedger records the provenance rows the resume contract reads back:
// path @accounts, record key = source username / audit id, raw = the account
// file digest (matching the library importer), semantic = digest of the row
// actually written. A changed source or mapping under the same ids is a
// conflict at preflight, never a silent second import.
func (s *decodedSource) writeLedger(ctx context.Context, tx pgx.Tx, plan *CutoverPlan) error {
	runID := RunIDFor(s.spec.SourceID, plan.manifestSHA)
	for i := range s.mapped {
		mu := &s.mapped[i]
		if mu.ledgerOK {
			continue
		}
		semantic, err := ItemDigest("user", mu.target)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO import_items (run_id, source_id, domain, relative_path, record_key, disposition,
    target_kind, target_id, raw_sha256, semantic_sha256, size_bytes, reason)
VALUES ($1,$2,$3,$4,$5,$6,'user',$7,$8,$9,0,'')
ON CONFLICT DO NOTHING`,
			uuidArg(runID), uuidArg(s.spec.SourceID), DomainAccounts, PathAccounts, mu.src.Username,
			DispImported, mu.targetID, s.sha256, semantic); err != nil {
			return fmt.Errorf("ledger %s:%q: %w", s.spec.App, mu.src.Username, err)
		}
	}
	for _, a := range s.file.Audit {
		if _, ok := s.ledger[a.ID]; ok {
			continue
		}
		semantic, err := ItemDigest("audit", a)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO import_items (run_id, source_id, domain, relative_path, record_key, disposition,
    target_kind, target_id, raw_sha256, semantic_sha256, size_bytes, reason)
VALUES ($1,$2,$3,$4,$5,$6,'audit',$7,$8,$9,0,'')
ON CONFLICT DO NOTHING`,
			uuidArg(runID), uuidArg(s.spec.SourceID), DomainAccounts, PathAccounts, a.ID,
			DispImported, DeterministicUUID(s.spec.SourceID, "audit", a.ID), s.sha256, semantic); err != nil {
			return fmt.Errorf("ledger %s audit %q: %w", s.spec.App, a.ID, err)
		}
	}
	return nil
}

func formatUUIDBytes(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	return formatUUID(u.Bytes)
}
