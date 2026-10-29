package blobkey

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestKeysBuildExpectedScheme pins the locked key layout for every kind. The
// expected strings are written out literally on purpose: if a builder drifts,
// the rollout of existing objects (and every GC/reconcile rule) silently breaks.
func TestKeysBuildExpectedScheme(t *testing.T) {
	keys, err := New(EnvProd)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	const (
		project = "11111111-1111-4111-8111-111111111111"
		story   = "22222222-2222-4222-8222-222222222222"
		user    = "33333333-3333-4333-8333-333333333333"
		upload  = "44444444-4444-4444-8444-444444444444"
		sha     = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	)
	cases := []struct {
		name string
		got  func() (string, error)
		want string
	}{
		{"audio segment", func() (string, error) { return keys.AudioSegment(project, story, sha, "wav") },
			"prod/p/" + project + "/s/" + story + "/audio/seg/01/" + sha + ".wav"},
		{"audio chapter", func() (string, error) { return keys.AudioChapter(project, story, 2, 7, sha, "mp3") },
			"prod/p/" + project + "/s/" + story + "/audio/ch/v2/c7/" + sha + ".mp3"},
		{"export", func() (string, error) { return keys.Export(project, story, sha, "KHI SẾP HỌC — Tập 1.mp3") },
			"prod/p/" + project + "/s/" + story + "/export/" + sha + "/KHI-SẾP-HỌC-Tập-1.mp3"},
		{"cover", func() (string, error) { return keys.Cover(project, story, sha, "png") },
			"prod/p/" + project + "/s/" + story + "/cover/" + sha + ".png"},
		{"music", func() (string, error) { return keys.Music(project, story, sha, "mp3") },
			"prod/p/" + project + "/s/" + story + "/music/" + sha + ".mp3"},
		{"import", func() (string, error) {
			return keys.Import(project, story, sha, "kich_ban/truyen/audio/bản đọc.json")
		}, "prod/p/" + project + "/s/" + story + "/import/01/" + sha + "-" + pathDigest("kich_ban/truyen/audio/bản đọc.json")[:12] + "-b-n-c.json"},
		{"asset", func() (string, error) { return keys.Asset(project, sha, "jpg") },
			"prod/p/" + project + "/assets/" + sha + ".jpg"},
		{"avatar", func() (string, error) { return keys.Avatar(user, sha, "webp") },
			"prod/u/" + user + "/avatar/" + sha + ".webp"},
		{"staging", func() (string, error) { return keys.Staging(upload) },
			"prod/staging/" + upload},
		{"backup", func() (string, error) {
			return keys.BackupPG(time.Date(2026, 10, 29, 3, 15, 0, 0, time.UTC)), nil
		}, "prod/backups/pg/2026/10/29/20261029T031500Z.dump"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.got()
			if err != nil {
				t.Fatalf("build: %v", err)
			}
			if got != tc.want {
				t.Fatalf("key = %q\nwant    %q", got, tc.want)
			}
		})
	}
}

// TestBuildersRejectBadInput: validation is part of the contract — a slug, a
// truncated digest or an unvalidated UUID must never become a key.
func TestBuildersRejectBadInput(t *testing.T) {
	keys, err := New(EnvDev)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	const (
		project = "11111111-1111-4111-8111-111111111111"
		story   = "22222222-2222-4222-8222-222222222222"
		sha     = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	)
	bad := []struct {
		name string
		fn   func() (string, error)
	}{
		{"slug project", func() (string, error) { return keys.AudioSegment("tron-audio", story, sha, "wav") }},
		{"empty story", func() (string, error) { return keys.AudioSegment(project, "", sha, "wav") }},
		{"short digest", func() (string, error) { return keys.AudioSegment(project, story, sha[:16], "wav") }},
		{"uppercase digest", func() (string, error) { return keys.AudioSegment(project, story, strings.ToUpper(sha), "wav") }},
		{"bad ext", func() (string, error) { return keys.AudioSegment(project, story, sha, "../wav") }},
		{"zero volume", func() (string, error) { return keys.AudioChapter(project, story, 0, 1, sha, "mp3") }},
		{"zero chapter", func() (string, error) { return keys.AudioChapter(project, story, 1, 0, sha, "mp3") }},
		{"empty export name", func() (string, error) { return keys.Export(project, story, sha, "//") }},
		{"slug user", func() (string, error) { return keys.Avatar("admin", sha, "png") }},
		{"slug upload", func() (string, error) { return keys.Staging("upload-1") }},
		{"empty import path", func() (string, error) { return keys.Import(project, story, sha, "  ") }},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			if key, err := tc.fn(); err == nil {
				t.Fatalf("accepted %q", key)
			}
		})
	}
}

// TestParseRoundTrip: every builder output parses back to the same fields, so
// reconcile/GC can classify an object without touching the database.
func TestParseRoundTrip(t *testing.T) {
	keys, err := New(EnvStaging)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	const (
		project = "11111111-1111-4111-8111-111111111111"
		story   = "22222222-2222-4222-8222-222222222222"
		user    = "33333333-3333-4333-8333-333333333333"
		upload  = "44444444-4444-4444-8444-444444444444"
		sha     = "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
	)
	seg, _ := keys.AudioSegment(project, story, sha, "wav")
	ch, _ := keys.AudioChapter(project, story, 3, 12, sha, "m4b")
	ex, _ := keys.Export(project, story, sha, "Truyện.epub")
	cov, _ := keys.Cover(project, story, sha, "png")
	mus, _ := keys.Music(project, story, sha, "ogg")
	imp, _ := keys.Import(project, story, sha, "a/b/c.txt")
	asset, _ := keys.Asset(project, sha, "jpg")
	avatar, _ := keys.Avatar(user, sha, "webp")
	staging, _ := keys.Staging(upload)
	backup := keys.BackupPG(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))

	check := func(key string, want Object) {
		t.Helper()
		got, err := Parse(key)
		if err != nil {
			t.Fatalf("Parse(%q): %v", key, err)
		}
		want.Env, want.Raw = EnvStaging, key
		if got != want {
			t.Fatalf("Parse(%q)\n got %+v\nwant %+v", key, got, want)
		}
	}
	check(seg, Object{Kind: KindAudioSegment, ProjectID: project, StoryID: story, SHA256: sha, Ext: "wav"})
	check(ch, Object{Kind: KindAudioChapter, ProjectID: project, StoryID: story, Volume: 3, Chapter: 12, SHA256: sha, Ext: "m4b"})
	check(ex, Object{Kind: KindExport, ProjectID: project, StoryID: story, SHA256: sha, Name: "Truyện.epub"})
	check(cov, Object{Kind: KindCover, ProjectID: project, StoryID: story, SHA256: sha, Ext: "png"})
	check(mus, Object{Kind: KindMusic, ProjectID: project, StoryID: story, SHA256: sha, Ext: "ogg"})
	check(imp, Object{Kind: KindImport, ProjectID: project, StoryID: story, SHA256: sha,
		PathHash: pathDigest("a/b/c.txt")[:12], Name: "c.txt"})
	check(asset, Object{Kind: KindAsset, ProjectID: project, SHA256: sha, Ext: "jpg"})
	check(avatar, Object{Kind: KindAvatar, UserID: user, SHA256: sha, Ext: "webp"})
	check(staging, Object{Kind: KindStaging, UploadID: upload})
	check(backup, Object{Kind: KindBackupPG, BackupAt: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)})
}

// TestParseRejectsForeignKeys: anything outside the scheme (old slug keys,
// wrong shard, wrong env, path traversal) is refused instead of guessed.
func TestParseRejectsForeignKeys(t *testing.T) {
	const (
		project = "11111111-1111-4111-8111-111111111111"
		story   = "22222222-2222-4222-8222-222222222222"
		sha     = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	)
	bad := []string{
		"",
		"/dev/staging/x",
		"projects/" + project + "/import/01/" + sha + "-0123456789ab-x",
		"prod/p/tron-audio/s/" + story + "/cover/" + sha + ".png",
		"prod/p/" + project + "/s/" + story + "/audio/seg/ff/" + sha + ".wav",
		"prod/p/" + project + "/s/" + story + "/audio/ch/v0/c1/" + sha + ".mp3",
		"prod/p/" + project + "/s/" + story + "/export/nothex/name.mp3",
		"prod/p/" + project + "/s/" + story + "/export/" + sha + "/a/b.mp3",
		"prod/p/" + project + "/s/" + story + "/cover/../" + sha + ".png",
		"prod/staging/not-a-uuid",
		"prod/backups/pg/2026/13/40/20261340T000000Z.dump",
		"prod/backups/pg/2026/10/29/20261029T031500Z.sql",
		"prod/p/" + project + "/s/" + story + "/import/01/" + sha + "-zzzz-c.txt",
		"qa/p/" + project + "/assets/" + sha + ".jpg",
	}
	for _, key := range bad {
		if obj, err := Parse(key); err == nil {
			t.Fatalf("Parse(%q) accepted as %+v", key, obj)
		}
	}
}

// isKeyLiteral lists the sub-strings that only an object-key builder may emit.
// API routes (/api/projects/…, …/stories/…) are not keys and are ignored: a
// key root never starts with a slash.
func isKeyLiteral(v string) bool {
	for _, root := range []string{"projects/", "stories/", "staging/", "backups/"} {
		if strings.HasPrefix(v, root) {
			return true
		}
	}
	for _, marker := range []string{"audio/seg/", "audio/ch/"} {
		if strings.Contains(v, marker) {
			return true
		}
	}
	return false
}

// TestNoKeyLiteralsOutsideBlobkey is the ratchet: object keys may only be
// assembled in this package. It parses (not greps) every Go file so comments
// and API routes are ignored, and it fails on both string concatenation
// ("projects/" + …) and fmt.Sprintf formats that embed key segments.
func TestNoKeyLiteralsOutsideBlobkey(t *testing.T) {
	root := appRoot(t)
	skip := map[string]bool{
		"internal/blobkey": true, ".git": true, "node_modules": true, "workspace": true,
		"third_party": true, "web": true,
	}
	var violations []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if skip[rel] || skip[d.Name()] && rel != "." && !strings.Contains(rel, "/") {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(rel, ".go") {
			return nil
		}
		src, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, p, src, 0)
		if err != nil {
			return err
		}
		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.BasicLit:
				if node.Kind != token.STRING {
					return true
				}
				v, err := strconv.Unquote(node.Value)
				if err != nil {
					return true
				}
				if isKeyLiteral(v) {
					violations = append(violations, fset.Position(node.Pos()).String()+": literal "+node.Value)
				}
			case *ast.CallExpr:
				sel, ok := node.Fun.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "Sprintf" || len(node.Args) == 0 {
					return true
				}
				lit, ok := node.Args[0].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return true
				}
				format, err := strconv.Unquote(lit.Value)
				if err != nil || !strings.Contains(format, "%s") {
					return true
				}
				for _, marker := range []string{"projects/", "stories/", "exports/", "staging/", "backups/"} {
					if strings.Contains(format, marker) {
						violations = append(violations, fset.Position(node.Pos()).String()+": Sprintf "+lit.Value)
						break
					}
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if len(violations) > 0 {
		t.Fatalf("object keys may only be built in internal/blobkey (%d violation(s)):\n%s",
			len(violations), strings.Join(violations, "\n"))
	}
}

// appRoot finds the Noverse app root (apps/noverse in the monorepo) from this
// file's location (internal/blobkey); the go.mod lives further up, so the
// command directory is the marker.
func appRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root := filepath.Dir(filepath.Dir(wd))
	if _, err := os.Stat(filepath.Join(root, "cmd", "ainovel-cli")); err != nil {
		t.Fatalf("app root %s: %v", root, err)
	}
	return root
}
