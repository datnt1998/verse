// Package storageimport migrates a frozen file-mode library (plus its
// accounts file) into the Postgres + BlobStore stores, reconciles the result
// and can write the data back out as a file-mode library.
//
// The design constraints come from plans/261028-1340-db-vps-storage/
// phase-08-import-reconcile.md:
//
//   - the source is read-only: every file is opened through an os.Root
//     confined to the frozen snapshot, external symlinks are refused, and
//     nothing in the source tree is ever written;
//   - every path gets exactly one disposition (imported / blob / derived /
//     excluded / quarantined) recorded in import_items, with the raw SHA-256
//     of the source bytes. Unknown files are quarantined, never silently
//     skipped;
//   - target ids are deterministic (UUIDv5 of source id + domain + logical
//     key), so a rerun of the same manifest writes zero new rows and zero new
//     blobs. An existing row that disagrees with the source is a loud
//     conflict, never an overwrite;
//   - two hashes per record: the raw source bytes (drift detection) and a
//     semantic digest of the normalized record (reconciliation);
//   - blobs are uploaded with an expected digest and only referenced from
//     the database once the driver returned a verified object.
//
// The package never parses the source with the file-mode Store: account and
// project files are decoded by the pure decoders in decode.go, so importing
// can never trigger the file backend's migration-on-read.
package storageimport

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/datnt1998/noverse/internal/blobkey"
)

// ImporterVersion stamps import_runs so a schema/decoder change is visible in
// the ledger even when the source manifest is unchanged.
const ImporterVersion = "storageimport/1"

// Path classes, mirroring internal/store/testdata/storage-contract/manifest.json.
const (
	KindFacts   = "facts"
	KindBlob    = "blob"
	KindDerived = "derived"
	KindScratch = "scratch"
	KindLocal   = "local-only"
	KindExclude = "excluded"
)

// Dispositions recorded in import_items.
const (
	DispImported = "imported"
	DispBlob     = "blob"
	DispDerived  = "derived"
	DispExcluded = "excluded"
	// DispTolerated marks a known, reported imperfection that does not block
	// publish (a truncated trailing JSONL line from a partial write).
	DispTolerated   = "tolerated"
	DispQuarantined = "quarantined"
)

// Domains group records for the reconciliation report.
const (
	DomainLibrary  = "library"
	DomainAccounts = "accounts"
	DomainProject  = "project"
	DomainStory    = "story"
	DomainEngine   = "engine"
	DomainComment  = "comment"
	DomainReview   = "review"
	DomainAudio    = "audio"
	DomainStream   = "stream"
	DomainAsset    = "asset"
)

// ManifestFile is one classified source path.
type ManifestFile struct {
	// Path is slash-separated and relative to the frozen root; the accounts
	// file is recorded as PathAccounts.
	Path string `json:"path"`
	Kind string `json:"kind"`
	// Domain and Role tell the importer which decoder owns the file. Role is
	// the stable role name (e.g. "chapter_record", "comment", "audio_cast").
	Domain string `json:"domain"`
	Role   string `json:"role"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
	// Reason documents every derived/excluded/quarantined path.
	Reason string `json:"reason,omitempty"`
}

// PathAccounts is the manifest path used for the accounts file, which lives
// outside the library root.
const PathAccounts = "@accounts"

// Manifest is the frozen inventory of one source snapshot.
type Manifest struct {
	Version     int            `json:"version"`
	SourceID    string         `json:"sourceId"`
	SourceLabel string         `json:"sourceLabel"`
	Files       []ManifestFile `json:"files"`
	// SHA256 covers Files (path, kind, size, digest) in path order and the
	// accounts digest; generated time is deliberately excluded so the same
	// bytes always yield the same hash.
	SHA256 string `json:"sha256"`
	// Stats counts dispositions/classes for the dry-run summary.
	Stats map[string]int `json:"stats"`
}

// CanonicalJSON marshals v with sorted object keys and no HTML escaping, the
// single normalization used for semantic digests.
func CanonicalJSON(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	var generic any
	if err := dec.Decode(&generic); err != nil {
		return nil, err
	}
	var buf strings.Builder
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(generic); err != nil {
		return nil, err
	}
	return []byte(strings.TrimSpace(buf.String())), nil
}

// SemanticDigest is SHA-256 over the canonical JSON of v after normalizing
// time values to UTC with microsecond precision. Without this a timestamp that
// round-tripped through timestamptz (+07 vs Z, ns vs µs) would hash
// differently from the source even though the instant is identical.
func SemanticDigest(v any) (string, error) {
	data, err := CanonicalJSON(normalizeDigestValue(reflect.ValueOf(v)).Interface())
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// TextDigest is SHA-256 over text with CRLF normalized and trailing spaces
// stripped per line — the normalization applied to markdown payloads.
func TextDigest(text string) string {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " \t")
	}
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(sum[:])
}

// RawDigest is the plain SHA-256 of bytes.
func RawDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// DigestOfStrings hashes a list of already-derived digests in order; used for
// per-domain roll-ups in the report.
func DigestOfStrings(values []string) string {
	sorted := append([]string(nil), values...)
	sort.Strings(sorted)
	h := sha256.New()
	for _, v := range sorted {
		_, _ = h.Write([]byte(v))
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// namespaceSalt pins the import UUID recipe. This is a PINNED LEGACY
// ALGORITHM, not a standards-compliant RFC 4122 UUIDv5: the hash is SHA-1
// over a textual salt (not a UUID namespace) plus NUL-separated
// source/domain/key, and the result is stamped with version-5/variant bits.
// Changing the salt or hash silently re-keys every imported identity, so it is
// a constant. New mappings that need true UUIDv5 must use one UUID namespace
// algorithm behind a deliberate mapping-version boundary.
var namespaceSalt = []byte("noverse/storageimport/v1")

// DeterministicUUID is the pinned legacy UUID recipe described above. Same
// inputs always produce the same id, which is what makes the import
// idempotent; preserve IDs across slug renames.
func DeterministicUUID(sourceID, domain, key string) string {
	h := sha1.New()
	_, _ = h.Write(namespaceSalt)
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(sourceID))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(domain))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(key))
	sum := h.Sum(nil)
	var u [16]byte
	copy(u[:], sum[:16])
	u[6] = (u[6] & 0x0f) | 0x50 // version 5
	u[8] = (u[8] & 0x3f) | 0x80 // RFC 4122 variant
	return formatUUID(u)
}

func formatUUID(u [16]byte) string {
	var b strings.Builder
	b.WriteString(hex.EncodeToString(u[0:4]))
	b.WriteByte('-')
	b.WriteString(hex.EncodeToString(u[4:6]))
	b.WriteByte('-')
	b.WriteString(hex.EncodeToString(u[6:8]))
	b.WriteByte('-')
	b.WriteString(hex.EncodeToString(u[8:10]))
	b.WriteByte('-')
	b.WriteString(hex.EncodeToString(u[10:16]))
	return b.String()
}

// BlobObjectKey builds the immutable, tenant-scoped object key for an
// imported file. The key scheme lives in internal/blobkey: UUIDs only,
// content-addressed by digest, story-scoped when the file belongs to a story.
// storyID may be empty for project-level files (project assets, project
// exports) — those keys live directly under the project.
func BlobObjectKey(keys blobkey.Keys, projectID, storyID, digest, sourcePath string) (string, error) {
	return keys.Import(projectID, storyID, digest, sourcePath)
}

// ContentType guesses a media type from the extension; unknown files stay
// application/octet-stream rather than claiming a type.
func ContentType(name string) string {
	switch strings.ToLower(extOf(name)) {
	case ".json":
		return "application/json"
	case ".jsonl", ".ndjson":
		return "application/x-ndjson"
	case ".md":
		return "text/markdown; charset=utf-8"
	case ".txt":
		return "text/plain; charset=utf-8"
	case ".srt":
		return "application/x-subrip"
	case ".epub":
		return "application/epub+zip"
	case ".mp3":
		return "audio/mpeg"
	case ".m4b":
		return "audio/mp4"
	case ".wav":
		return "audio/wav"
	case ".zip":
		return "application/zip"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}

func extOf(name string) string {
	if i := strings.LastIndexByte(name, '.'); i >= 0 {
		return name[i:]
	}
	return ""
}

// lockKey mirrors import_lock_key() in migration 00005 so the Go side and the
// SQL side agree on the advisory-lock namespace.
func lockKey(sourceID string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(sourceID))
	var out [8]byte
	binary.BigEndian.PutUint64(out[:], h.Sum64())
	return int64(binary.BigEndian.Uint64(out[:]) & 0x0fff_ffff_ffff_ffff)
}

// FormatSize renders byte counts for the Vietnamese summary.
func FormatSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for n/div >= unit && exp < 3 {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

// TimeOrZero keeps struct digests stable when a source timestamp is absent.
// Timestamps are truncated to microseconds because that is Postgres'
// resolution: a digest must survive a round-trip through timestamptz.
func TimeOrZero(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return t.UTC().Truncate(time.Microsecond)
}

// normalizeDigestValue deep-copies v with every time.Time in UTC at
// microsecond precision (Postgres' resolution) so a digest is stable across a
// database round-trip.
func normalizeDigestValue(v reflect.Value) reflect.Value {
	if !v.IsValid() {
		return v
	}
	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			return v
		}
		inner := normalizeDigestValue(v.Elem())
		out := reflect.New(inner.Type())
		out.Elem().Set(inner)
		return out
	case reflect.Interface:
		if v.IsNil() {
			return v
		}
		inner := normalizeDigestValue(v.Elem())
		out := reflect.New(v.Type()).Elem()
		out.Set(inner)
		return out
	case reflect.Struct:
		if t, ok := v.Interface().(time.Time); ok {
			out := reflect.New(v.Type()).Elem()
			out.Set(reflect.ValueOf(t.UTC().Truncate(time.Microsecond)))
			return out
		}
		out := reflect.New(v.Type()).Elem()
		out.Set(v)
		for i := 0; i < v.NumField(); i++ {
			if !out.Field(i).CanSet() {
				continue
			}
			out.Field(i).Set(normalizeDigestValue(v.Field(i)))
		}
		return out
	case reflect.Slice:
		if v.IsNil() {
			return v
		}
		out := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
		for i := 0; i < v.Len(); i++ {
			out.Index(i).Set(normalizeDigestValue(v.Index(i)))
		}
		return out
	case reflect.Array:
		out := reflect.New(v.Type()).Elem()
		for i := 0; i < v.Len(); i++ {
			out.Index(i).Set(normalizeDigestValue(v.Index(i)))
		}
		return out
	case reflect.Map:
		if v.IsNil() {
			return v
		}
		out := reflect.MakeMapWithSize(v.Type(), v.Len())
		iter := v.MapRange()
		for iter.Next() {
			out.SetMapIndex(iter.Key(), normalizeDigestValue(iter.Value()))
		}
		return out
	}
	return v
}
