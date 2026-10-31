// Package backdrop implements resumable channel-backed background clips.
package backdrop

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
)

const MaxItems = 40
const ManifestFile = "backdrop/backdrop.json"

type ItemError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
type Item struct {
	ID        string     `json:"id"`
	ChannelID string     `json:"channelId"`
	VideoID   string     `json:"videoId"`
	Kind      string     `json:"kind"`
	URL       string     `json:"url"`
	Title     *string    `json:"title"`
	StartSec  int        `json:"startSec"`
	EndSec    int        `json:"endSec"`
	Source    string     `json:"source"`
	Status    string     `json:"status"`
	MediaID   *string    `json:"mediaId"`
	Error     *ItemError `json:"error"`
}
type Manifest struct {
	Version     int      `json:"version"`
	Revision    string   `json:"revision"`
	Salt        uint32   `json:"salt"`
	WantedCount int      `json:"wantedCount"`
	TargetSec   *int     `json:"targetSec"`
	Fallback    string   `json:"fallback"`
	Warnings    []string `json:"warnings"`
	Items       []Item   `json:"items"`
}

// Checkpoint includes unpublished local files so a retry need not fetch them again.
type Checkpoint struct {
	Manifest  Manifest         `json:"manifest"`
	Artifacts []media.Artifact `json:"artifacts"`
}

func Seal(m Manifest) Manifest {
	m.Version = 1
	if m.Items == nil {
		m.Items = []Item{}
	}
	if m.Warnings == nil {
		m.Warnings = []string{}
	}
	m.Revision = ""
	b, _ := json.Marshal(m)
	var value map[string]any
	_ = json.Unmarshal(b, &value)
	delete(value, "revision")
	b, _ = json.Marshal(value)
	sum := sha256.Sum256(b)
	m.Revision = hex.EncodeToString(sum[:8])
	return m
}
func Empty(wanted int, target *int) Manifest {
	return Seal(Manifest{WantedCount: wanted, TargetSec: target, Fallback: "none"})
}
func invalidManifest() error {
	return &domain.CloneError{Code: domain.CloneErrInternal, Message: "Manifest nền video hỏng"}
}
func Parse(b []byte) (Manifest, error) {
	var m Manifest
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(b, &fields); err != nil {
		return m, invalidManifest()
	}
	for _, key := range []string{"version", "revision", "salt", "wantedCount", "fallback", "warnings", "items"} {
		raw, ok := fields[key]
		if !ok || string(raw) == "null" {
			return m, invalidManifest()
		}
	}
	var rawItems []map[string]json.RawMessage
	if err := json.Unmarshal(fields["items"], &rawItems); err != nil {
		return m, invalidManifest()
	}
	for _, raw := range rawItems {
		for _, key := range []string{"id", "channelId", "videoId", "kind", "url", "title", "startSec", "endSec", "source", "status", "mediaId", "error"} {
			if _, ok := raw[key]; !ok {
				return m, invalidManifest()
			}
		}
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return m, invalidManifest()
	}
	if m.Version != 1 || m.Revision == "" || m.WantedCount < 0 || m.WantedCount > MaxItems || len(m.Items) > MaxItems || m.TargetSec != nil && *m.TargetSec < 0 || m.Fallback != "none" && m.Fallback != "template-clips" {
		return m, invalidManifest()
	}
	seen := map[string]bool{}
	for _, v := range m.Items {
		if v.ID == "" || seen[v.ID] || v.ChannelID == "" || v.VideoID == "" || v.URL == "" || v.StartSec < 0 || v.EndSec <= v.StartSec || v.Source != "random" && v.Source != "owner" || v.Status != "pending" && v.Status != "ready" && v.Status != "failed" {
			return m, invalidManifest()
		}
		switch v.Kind {
		case "clip", "vod", "highlight", "video":
		default:
			return m, invalidManifest()
		}
		seen[v.ID] = true
	}
	if m.Items == nil {
		m.Items = []Item{}
	}
	if m.Warnings == nil {
		m.Warnings = []string{}
	}
	return m, nil
}
func ParseCheckpoint(b []byte) (Checkpoint, error) {
	var c Checkpoint
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(b, &fields); err != nil {
		return c, invalidManifest()
	}
	if raw, ok := fields["manifest"]; ok {
		m, err := Parse(raw)
		if err != nil {
			return c, err
		}
		c.Manifest = m
		if raw := fields["artifacts"]; len(raw) > 0 {
			if err := json.Unmarshal(raw, &c.Artifacts); err != nil {
				return c, invalidManifest()
			}
		}
		return c, nil
	}
	m, err := Parse(b)
	c.Manifest = m
	return c, err
}
func NextItemID(items []Item) string {
	max := 0
	for _, v := range items {
		if strings.HasPrefix(v.ID, "b") {
			n, err := strconv.Atoi(v.ID[1:])
			if err == nil && n > max {
				max = n
			}
		}
	}
	return fmt.Sprintf("b%d", max+1)
}
func ItemSec(v Item) int { return v.EndSec - v.StartSec }
func PlannedSec(items []Item) int {
	n := 0
	for _, v := range items {
		if v.Status != "failed" {
			n += ItemSec(v)
		}
	}
	return n
}
func ReadyItems(m Manifest) []Item {
	out := []Item{}
	for _, v := range m.Items {
		if v.Status == "ready" && v.MediaID != nil {
			out = append(out, v)
		}
	}
	return out
}
func ShortfallWarnings(warnings []string, items []Item, wanted int, target *int) []string {
	out := []string{}
	for _, s := range warnings {
		if !strings.HasPrefix(s, "Lấy được ") {
			out = append(out, s)
		}
	}
	if target != nil {
		return out
	}
	seen := map[string]bool{}
	for _, v := range items {
		if v.Status == "pending" {
			return out
		}
		if v.Status == "ready" && v.MediaID != nil {
			seen[*v.MediaID] = true
		}
	}
	if wanted > len(seen) {
		out = append(out, fmt.Sprintf("Lấy được %d/%d clip nền", len(seen), wanted))
	}
	return out
}
func StepData(m Manifest) map[string]any {
	ids := []string{}
	seen := map[string]bool{}
	for _, v := range ReadyItems(m) {
		if !seen[*v.MediaID] {
			ids = append(ids, *v.MediaID)
			seen[*v.MediaID] = true
		}
	}
	return map[string]any{"mediaIds": ids, "readyCount": len(ids), "wantedCount": m.WantedCount, "targetSec": m.TargetSec, "fallback": m.Fallback, "warnings": m.Warnings, "manifest": m}
}
