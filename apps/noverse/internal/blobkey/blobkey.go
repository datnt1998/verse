// Package blobkey owns the single object-key scheme of the hosted BlobStore.
//
// Every object key is built here (typed builders per kind, all validated) and
// read back by Parse; no other package may assemble a key from parts. The
// scheme is:
//
//	{env}/p/{projectUUID}/s/{storyUUID}/
//	    audio/seg/{sha[:2]}/{sha}.{ext}
//	    audio/ch/v{vol}/c{ch}/{sha}.{ext}
//	    export/{sha}/{download-name}
//	    cover/{sha}.{ext}
//	    music/{sha}.{ext}
//	    import/{sha[:2]}/{sha}-{pathhash12}-{name}
//	{env}/p/{projectUUID}/assets/{sha}.{ext}
//	{env}/p/{projectUUID}/import/{sha[:2]}/{sha}-{pathhash12}-{name}
//	{env}/u/{userUUID}/avatar/{sha}.{ext}
//	{env}/staging/{uploadUUID}
//	{env}/backups/pg/{YYYY}/{MM}/{DD}/{ts}.dump
//
// Rules that make the scheme safe:
//
//   - The env prefix comes from STORAGE_ENV (dev|staging|prod, default dev) and
//     is the first path segment, so one bucket can host several environments
//     and a GC run can never touch another environment's objects.
//   - Identifiers are UUIDs. Slugs, titles and usernames never appear in a key:
//     renaming a project/story must not move a single byte.
//   - Content-addressed kinds embed the full lowercase-hex SHA-256 (64 chars),
//     so the same bytes always address the same object and different bytes can
//     never collide under one key.
//   - Friendly names (download names) only exist in Content-Disposition, with
//     the single exception of the export sub-key, which keeps the download name
//     for operator legibility (sanitized, never a path separator).
//   - The two project-level import/assets keys exist only for blobs that are
//     not owned by a story (project assets and project-level exports); anything
//     story-scoped uses the /s/{storyUUID}/ branch above.
//
// Staging: the hot path does NOT use a staging hop. Uploads stream straight to
// the final content-addressed key with PutOptions.ExpectedSHA256 — the driver
// aborts the object on a digest mismatch (S3: multipart abort/delete; local:
// temp file + atomic rename), so a partial or corrupted object is never
// visible at a final key, and the key itself already commits to the digest.
// The {env}/staging/ prefix stays reserved for future resumable/multipart
// uploads; `storage gc` expires anything left there after 24h.
package blobkey

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Errors returned by the package. Callers use errors.Is.
var (
	// ErrInvalidEnv: STORAGE_ENV (or the first key segment) is not dev/staging/prod.
	ErrInvalidEnv = errors.New("môi trường lưu trữ không hợp lệ")
	// ErrInvalidKey: a key does not match the scheme (Parse).
	ErrInvalidKey = errors.New("khoá đối tượng không theo quy ước")
	// ErrInvalidPart: a builder input is empty or malformed.
	ErrInvalidPart = errors.New("tham số khoá đối tượng không hợp lệ")
)

// Env is the deployment environment prefix of every key.
type Env string

const (
	EnvDev     Env = "dev"
	EnvStaging Env = "staging"
	EnvProd    Env = "prod"

	// EnvVar is the environment variable naming the deployment environment.
	EnvVar = "STORAGE_ENV"
)

// ParseEnv validates a raw STORAGE_ENV value. The empty string means dev: a
// missing variable is a dev workstation, never production by accident.
func ParseEnv(raw string) (Env, error) {
	switch Env(strings.TrimSpace(raw)) {
	case "", EnvDev:
		return EnvDev, nil
	case EnvStaging:
		return EnvStaging, nil
	case EnvProd:
		return EnvProd, nil
	}
	return "", fmt.Errorf("%s=%q (dev|staging|prod): %w", EnvVar, raw, ErrInvalidEnv)
}

// EnvFromOS parses STORAGE_ENV from the process environment.
func EnvFromOS() (Env, error) { return ParseEnv(os.Getenv(EnvVar)) }

// Kind names one key family. It is stored in Object.Kind by Parse so GC and
// reconcile can reason about an object without re-splitting its key.
type Kind string

const (
	KindAudioSegment Kind = "audio/seg"
	KindAudioChapter Kind = "audio/ch"
	KindExport       Kind = "export"
	KindCover        Kind = "cover"
	KindMusic        Kind = "music"
	KindImport       Kind = "import"
	KindAsset        Kind = "asset"
	KindAvatar       Kind = "avatar"
	KindStaging      Kind = "staging"
	KindBackupPG     Kind = "backup/pg"
)

// Keys builds keys for one environment. The zero value is the dev environment,
// so a missing wiring step can never emit a key without an env prefix.
type Keys struct {
	env Env
}

// New validates env and returns its builder. The empty env means dev.
func New(env Env) (Keys, error) {
	parsed, err := ParseEnv(string(env))
	if err != nil {
		return Keys{}, err
	}
	return Keys{env: parsed}, nil
}

// FromEnv builds the keys for the process environment (STORAGE_ENV).
func FromEnv() (Keys, error) {
	env, err := EnvFromOS()
	if err != nil {
		return Keys{}, err
	}
	return Keys{env: env}, nil
}

// Env reports the environment prefix of the keys this builder emits.
func (k Keys) Env() Env {
	if k.env == "" {
		return EnvDev
	}
	return k.env
}

// Prefix is the environment-scoped root of every key this builder emits. GC
// lists under it and never touches another environment.
func (k Keys) Prefix() string { return string(k.Env()) + "/" }

// StagingPrefix is the prefix of the staging area for this environment.
func (k Keys) StagingPrefix() string { return k.Prefix() + "staging/" }

// BackupPrefix is the prefix of database backups for this environment.
func (k Keys) BackupPrefix() string { return k.Prefix() + "backups/pg/" }

// ── builders ────────────────────────────────────────────────────────────────

func (k Keys) projectStory(projectID, storyID string) (string, error) {
	if err := validUUID(projectID); err != nil {
		return "", fmt.Errorf("projectID %q: %w", projectID, err)
	}
	if err := validUUID(storyID); err != nil {
		return "", fmt.Errorf("storyID %q: %w", storyID, err)
	}
	return k.Prefix() + "p/" + projectID + "/s/" + storyID, nil
}

// AudioSegment is one rendered TTS segment, content-addressed and shared by
// every job that produces the same bytes.
func (k Keys) AudioSegment(projectID, storyID, sha256Hex, ext string) (string, error) {
	base, err := k.projectStory(projectID, storyID)
	if err != nil {
		return "", err
	}
	if err := validSHA(sha256Hex); err != nil {
		return "", err
	}
	ext, err = normExt(ext)
	if err != nil {
		return "", err
	}
	return base + "/audio/seg/" + sha256Hex[:2] + "/" + sha256Hex + "." + ext, nil
}

// AudioChapter is one assembled chapter audio file of a volume.
func (k Keys) AudioChapter(projectID, storyID string, volume, chapter int, sha256Hex, ext string) (string, error) {
	base, err := k.projectStory(projectID, storyID)
	if err != nil {
		return "", err
	}
	if err := validSHA(sha256Hex); err != nil {
		return "", err
	}
	if volume <= 0 || volume > 9999 {
		return "", fmt.Errorf("volume %d: %w", volume, ErrInvalidPart)
	}
	if chapter <= 0 || chapter > 99999 {
		return "", fmt.Errorf("chapter %d: %w", chapter, ErrInvalidPart)
	}
	ext, err = normExt(ext)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/audio/ch/v%d/c%d/%s.%s", base, volume, chapter, sha256Hex, ext), nil
}

// Export is one produced export artifact. The download name is kept for
// operator legibility only; it is sanitized and never contains a separator.
func (k Keys) Export(projectID, storyID, sha256Hex, downloadName string) (string, error) {
	base, err := k.projectStory(projectID, storyID)
	if err != nil {
		return "", err
	}
	if err := validSHA(sha256Hex); err != nil {
		return "", err
	}
	name := sanitizeExportName(downloadName)
	if name == "" {
		return "", fmt.Errorf("downloadName %q: %w", downloadName, ErrInvalidPart)
	}
	return base + "/export/" + sha256Hex + "/" + name, nil
}

// Cover is the story cover image (content-addressed).
func (k Keys) Cover(projectID, storyID, sha256Hex, ext string) (string, error) {
	return k.storyMedia("cover", projectID, storyID, sha256Hex, ext)
}

// Music is a story-level music bed (content-addressed).
func (k Keys) Music(projectID, storyID, sha256Hex, ext string) (string, error) {
	return k.storyMedia("music", projectID, storyID, sha256Hex, ext)
}

func (k Keys) storyMedia(kind Kind, projectID, storyID, sha256Hex, ext string) (string, error) {
	base, err := k.projectStory(projectID, storyID)
	if err != nil {
		return "", err
	}
	if err := validSHA(sha256Hex); err != nil {
		return "", err
	}
	ext, err = normExt(ext)
	if err != nil {
		return "", err
	}
	return base + "/" + string(kind) + "/" + sha256Hex + "." + ext, nil
}

// Import is the key of one imported source file. Keys are content-addressed
// and disambiguated by the source path hash, so two identical files at
// different source paths stay two objects (one ledger row per path).
// storyID may be empty only for project-level files; story-scoped imports must
// pass their story UUID.
func (k Keys) Import(projectID, storyID, sha256Hex, sourcePath string) (string, error) {
	if err := validUUID(projectID); err != nil {
		return "", fmt.Errorf("projectID %q: %w", projectID, err)
	}
	if err := validSHA(sha256Hex); err != nil {
		return "", err
	}
	if strings.TrimSpace(sourcePath) == "" {
		return "", fmt.Errorf("sourcePath rỗng: %w", ErrInvalidPart)
	}
	prefix := k.Prefix() + "p/" + projectID
	if storyID != "" {
		if err := validUUID(storyID); err != nil {
			return "", fmt.Errorf("storyID %q: %w", storyID, err)
		}
		prefix += "/s/" + storyID
	}
	name := sanitizeImportName(sourcePath)
	pathHash := pathDigest(sourcePath)[:12]
	return prefix + "/import/" + sha256Hex[:2] + "/" + sha256Hex + "-" + pathHash + "-" + name, nil
}

// Asset is a project-level asset (not owned by one story).
func (k Keys) Asset(projectID, sha256Hex, ext string) (string, error) {
	if err := validUUID(projectID); err != nil {
		return "", fmt.Errorf("projectID %q: %w", projectID, err)
	}
	if err := validSHA(sha256Hex); err != nil {
		return "", err
	}
	ext, err := normExt(ext)
	if err != nil {
		return "", err
	}
	return k.Prefix() + "p/" + projectID + "/assets/" + sha256Hex + "." + ext, nil
}

// Avatar is one user's avatar image (content-addressed).
func (k Keys) Avatar(userID, sha256Hex, ext string) (string, error) {
	if err := validUUID(userID); err != nil {
		return "", fmt.Errorf("userID %q: %w", userID, err)
	}
	if err := validSHA(sha256Hex); err != nil {
		return "", err
	}
	ext, err := normExt(ext)
	if err != nil {
		return "", err
	}
	return k.Prefix() + "u/" + userID + "/avatar/" + sha256Hex + "." + ext, nil
}

// Staging is one in-flight upload slot. Only used by streamed/resumable
// uploads that need a temporary object; the direct Put-with-hash path skips it.
func (k Keys) Staging(uploadID string) (string, error) {
	if err := validUUID(uploadID); err != nil {
		return "", fmt.Errorf("uploadID %q: %w", uploadID, err)
	}
	return k.StagingPrefix() + uploadID, nil
}

// BackupPG is the nightly database dump key of one run (UTC).
func (k Keys) BackupPG(at time.Time) string {
	at = at.UTC()
	return k.BackupPrefix() + at.Format("2006/01/02") + "/" + at.Format("20060102T150405Z") + ".dump"
}

// ── reverse direction ───────────────────────────────────────────────────────

// Object is the parsed form of one key. Fields not carried by a kind are empty
// (or zero), so callers must switch on Kind.
type Object struct {
	Env  Env
	Kind Kind
	Raw  string

	ProjectID string
	StoryID   string
	UserID    string

	SHA256   string
	Ext      string
	Name     string
	PathHash string

	Volume  int
	Chapter int

	UploadID string
	BackupAt time.Time
}

// Parse reverses a key produced by any builder. It validates every segment
// (env, UUIDs, digests, shards, extensions) and rejects anything that is not
// exactly the scheme — reconcilers and GC must never guess.
func Parse(key string) (Object, error) {
	bad := func(format string, args ...any) (Object, error) {
		return Object{}, fmt.Errorf("%w: %s", ErrInvalidKey, fmt.Sprintf(format, args...))
	}
	if key == "" || strings.HasPrefix(key, "/") || strings.HasSuffix(key, "/") {
		return bad("rỗng hoặc có / ở đầu/cuối")
	}
	parts := strings.Split(key, "/")
	env, err := ParseEnv(parts[0])
	if err != nil || string(env) != parts[0] {
		return bad("tiền tố môi trường %q", parts[0])
	}
	obj := Object{Env: env, Raw: key}
	switch {
	case parts[1] == "staging" && len(parts) == 3:
		if err := validUUID(parts[2]); err != nil {
			return bad("uploadUUID %q", parts[2])
		}
		obj.Kind, obj.UploadID = KindStaging, parts[2]
		return obj, nil

	case parts[1] == "backups" && len(parts) == 7 && parts[2] == "pg":
		at, err := time.Parse("2006/01/02", parts[3]+"/"+parts[4]+"/"+parts[5])
		if err != nil {
			return bad("ngày sao lưu %q", strings.Join(parts[3:6], "/"))
		}
		stamp, ok := strings.CutSuffix(parts[6], ".dump")
		if !ok {
			return bad("tên dump %q", parts[6])
		}
		if _, err := time.Parse("20060102T150405Z", stamp); err != nil {
			return bad("mốc thời gian dump %q", stamp)
		}
		obj.Kind, obj.BackupAt = KindBackupPG, at
		return obj, nil

	case parts[1] == "u" && len(parts) == 5 && parts[3] == "avatar":
		if err := validUUID(parts[2]); err != nil {
			return bad("userUUID %q", parts[2])
		}
		sha, ext, err := splitDigestExt(parts[4])
		if err != nil {
			return bad("%v", err)
		}
		obj.Kind, obj.UserID, obj.SHA256, obj.Ext = KindAvatar, parts[2], sha, ext
		return obj, nil

	case parts[1] == "p" && len(parts) >= 5:
		if err := validUUID(parts[2]); err != nil {
			return bad("projectUUID %q", parts[2])
		}
		obj.ProjectID = parts[2]

		// Project-level namespaces.
		if parts[3] != "s" {
			switch {
			case parts[3] == "assets" && len(parts) == 5:
				sha, ext, err := splitDigestExt(parts[4])
				if err != nil {
					return bad("%v", err)
				}
				obj.Kind, obj.SHA256, obj.Ext = KindAsset, sha, ext
				return obj, nil
			case parts[3] == "import" && len(parts) == 6:
				sha, pathHash, name, err := splitImportName(parts[5])
				if err != nil {
					return bad("%v", err)
				}
				if parts[4] != sha[:2] {
					return bad("shard %q không khớp sha %q", parts[4], sha)
				}
				obj.Kind, obj.SHA256, obj.PathHash, obj.Name = KindImport, sha, pathHash, name
				return obj, nil
			}
			return bad("namespace dự án %q", strings.Join(parts[3:], "/"))
		}

		if len(parts) < 7 {
			return bad("thiếu phần loại dưới /s/{story}")
		}
		if err := validUUID(parts[4]); err != nil {
			return bad("storyUUID %q", parts[4])
		}
		obj.StoryID = parts[4]
		switch parts[5] {
		case "audio":
			if len(parts) < 8 {
				return bad("audio thiếu loại")
			}
			switch parts[6] {
			case "seg":
				if len(parts) != 9 {
					return bad("audio/seg cần đúng 9 đoạn")
				}
				sha, ext, err := splitDigestExt(parts[8])
				if err != nil {
					return bad("%v", err)
				}
				if parts[7] != sha[:2] {
					return bad("shard %q không khớp sha", parts[7])
				}
				obj.Kind, obj.SHA256, obj.Ext = KindAudioSegment, sha, ext
				return obj, nil
			case "ch":
				if len(parts) != 10 {
					return bad("audio/ch cần đúng 10 đoạn")
				}
				volume, ok := parseDecorated(parts[7], "v")
				if !ok {
					return bad("volume %q", parts[7])
				}
				chapter, ok := parseDecorated(parts[8], "c")
				if !ok {
					return bad("chapter %q", parts[8])
				}
				sha, ext, err := splitDigestExt(parts[9])
				if err != nil {
					return bad("%v", err)
				}
				obj.Kind, obj.Volume, obj.Chapter, obj.SHA256, obj.Ext = KindAudioChapter, volume, chapter, sha, ext
				return obj, nil
			}
			return bad("audio/%s", parts[6])
		case "export":
			if len(parts) != 8 {
				return bad("export cần đúng 8 đoạn")
			}
			if err := validSHA(parts[6]); err != nil {
				return bad("export sha %q", parts[6])
			}
			obj.Kind, obj.SHA256, obj.Name = KindExport, parts[6], parts[7]
			return obj, nil
		case "cover", "music":
			if len(parts) != 7 {
				return bad("%s cần đúng 7 đoạn", parts[5])
			}
			sha, ext, err := splitDigestExt(parts[6])
			if err != nil {
				return bad("%v", err)
			}
			obj.Kind, obj.SHA256, obj.Ext = Kind(parts[5]), sha, ext
			return obj, nil
		case "import":
			if len(parts) != 8 {
				return bad("import cần đúng 8 đoạn")
			}
			sha, pathHash, name, err := splitImportName(parts[7])
			if err != nil {
				return bad("%v", err)
			}
			if parts[6] != sha[:2] {
				return bad("shard %q không khớp sha", parts[6])
			}
			obj.Kind, obj.SHA256, obj.PathHash, obj.Name = KindImport, sha, pathHash, name
			return obj, nil
		}
		return bad("loại %q", parts[5])
	}
	return bad("không khớp khung khoá")
}

// ── validation helpers ──────────────────────────────────────────────────────

var (
	uuidRe  = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	shaRe   = regexp.MustCompile(`^[0-9a-f]{64}$`)
	extRe   = regexp.MustCompile(`^[a-z0-9]{1,8}$`)
	hashRe  = regexp.MustCompile(`^[0-9a-f]{12}$`)
	digitRe = regexp.MustCompile(`^[0-9]{1,5}$`)
)

func validUUID(v string) error {
	if !uuidRe.MatchString(v) {
		return fmt.Errorf("UUID %q: %w", v, ErrInvalidPart)
	}
	return nil
}

func validSHA(v string) error {
	if !shaRe.MatchString(v) {
		return fmt.Errorf("sha256 %q: %w", v, ErrInvalidPart)
	}
	return nil
}

func normExt(ext string) (string, error) {
	ext = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(ext), "."))
	if !extRe.MatchString(ext) {
		return "", fmt.Errorf("phần mở rộng %q: %w", ext, ErrInvalidPart)
	}
	return ext, nil
}

func splitDigestExt(seg string) (sha, ext string, err error) {
	i := strings.LastIndexByte(seg, '.')
	if i < 0 {
		return "", "", fmt.Errorf("%q thiếu phần mở rộng", seg)
	}
	sha, ext = seg[:i], seg[i+1:]
	if !shaRe.MatchString(sha) {
		return "", "", fmt.Errorf("sha256 %q không hợp lệ", sha)
	}
	if !extRe.MatchString(ext) {
		return "", "", fmt.Errorf("phần mở rộng %q không hợp lệ", ext)
	}
	return sha, ext, nil
}

// splitImportName splits "{sha}-{pathhash12}-{name}".
func splitImportName(seg string) (sha, pathHash, name string, err error) {
	parts := strings.SplitN(seg, "-", 3)
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("%q không có dạng sha-hash-tên", seg)
	}
	if !shaRe.MatchString(parts[0]) {
		return "", "", "", fmt.Errorf("sha256 %q không hợp lệ", parts[0])
	}
	if !hashRe.MatchString(parts[1]) {
		return "", "", "", fmt.Errorf("path hash %q không hợp lệ", parts[1])
	}
	if parts[2] == "" {
		return "", "", "", fmt.Errorf("%q thiếu tên tệp", seg)
	}
	return parts[0], parts[1], parts[2], nil
}

func parseDecorated(seg, prefix string) (int, bool) {
	rest, ok := strings.CutPrefix(seg, prefix)
	if !ok || !digitRe.MatchString(rest) {
		return 0, false
	}
	n := 0
	for _, r := range rest {
		n = n*10 + int(r-'0')
	}
	if n <= 0 {
		return 0, false
	}
	return n, true
}

// pathDigest is the lowercase-hex SHA-256 of a source path (path hash input).
func pathDigest(sourcePath string) string {
	sum := sha256.Sum256([]byte(sourcePath))
	return hex.EncodeToString(sum[:])
}

// sanitizeImportName keeps ASCII letters/digits/._- and replaces everything
// else with '-', so the key stays portable across every driver.
func sanitizeImportName(sourcePath string) string {
	base := path.Base(strings.ReplaceAll(sourcePath, "\\", "/"))
	var b strings.Builder
	lastDash := false
	for _, r := range base {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '_':
			b.WriteRune(r)
			lastDash = false
		case r == '-':
			b.WriteRune(r)
			lastDash = true
		default:
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-.")
	if out == "" {
		return "file"
	}
	if len(out) > 80 {
		out = out[:80]
	}
	return out
}

// sanitizeExportName keeps a human-readable download name (Unicode letters
// included, so Vietnamese file names stay legible) while removing anything a
// key must never carry: separators, punctuation, quotes, control characters.
// It returns "" when nothing legible is left, so Export can refuse the key.
func sanitizeExportName(name string) string {
	name = path.Base(strings.ReplaceAll(name, "\\", "/"))
	var b strings.Builder
	lastDash := false
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '_':
			b.WriteRune(r)
			lastDash = false
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-.")
	if len(out) > 80 {
		cut := 80
		for cut > 0 && !utf8.RuneStart(out[cut]) {
			cut--
		}
		out = strings.TrimRight(out[:cut], "-.")
	}
	return out
}
