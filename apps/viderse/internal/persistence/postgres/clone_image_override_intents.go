package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/datnt1998/verse/apps/viderse/internal/clone/jobedit"
	images "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/images"
)

// applyImageOverride projects the selected prompt and redraws only versions
// present when the owner confirmed the plan. Paid in-flight output is never
// replaced; draining waits until those flights finish.
func applyImageOverride(m *images.Manifest, desired images.Manifest) {
	if desired.ImageMode == "none" && m.ImageMode != "none" {
		items := m.Items[:0]
		for _, item := range m.Items {
			if item.Kind != "scene" { items = append(items, item) }
		}
		m.Items = items
		m.QueueOnly = nil
	} else if m.ImageMode == "none" && desired.ImageMode != "none" {
		items := make([]images.Item, 0, len(desired.Items))
		for _, wanted := range desired.Items {
			found := false
			for _, current := range m.Items {
				if current.ID == wanted.ID {
					items = append(items, current)
					found = true
					break
				}
			}
			if !found { items = append(items, wanted) }
		}
		m.Items = items
		m.QueueOnly = nil
	}
	m.ImageMode = desired.ImageMode
	m.Overrides = desired.Overrides
	m.TemplatePrompt = desired.TemplatePrompt
	m.ImagePromptMode = desired.ImagePromptMode
	m.CoverPrompt = desired.CoverPrompt
	m.Thumbnail = desired.Thumbnail
	m.ReviewOrigin = false
	if len(desired.QueueOnly) > 0 && len(m.QueueOnly) == 0 {
		for _, item := range m.Items {
			if item.Status == "pending" || item.Status == "running" {
				m.QueueOnly = append(m.QueueOnly, item.ID)
			}
		}
	}
	for _, id := range desired.QueueOnly {
		for i := range m.Items {
			item := &m.Items[i]
			if item.ID != id || item.InFlight != nil {
				continue
			}
			item.Status = "pending"
			item.CurrentVersion = nil
			item.ReviewVersion = nil
			item.SkipReason = ""
			item.Error = ""
			item.PromptVersion++
			found := false
			for _, queued := range m.QueueOnly { if queued == id { found = true; break } }
			if !found { m.QueueOnly = append(m.QueueOnly, id) }
			break
		}
	}
	m.Revision++
}

func overlayImageOverride(ctx context.Context, tx pgx.Tx, id any, m *images.Manifest, safe bool) (bool, error) {
	var raw []byte
	err := tx.QueryRow(ctx, `SELECT manifest FROM clone_image_override_intents WHERE job_id=$1`, id).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) { return false, nil }
	if err != nil { return false, err }
	if safe {
		for _, item := range m.Items { if item.InFlight != nil { return false, nil } }
	}
	var desired images.Manifest
	if err := json.Unmarshal(raw, &desired); err != nil { return false, err }
	applyImageOverride(m, desired)
	return true, nil
}

func enqueueImageOverride(ctx context.Context, tx pgx.Tx, id any, revision int, raw json.RawMessage) error {
	result, err := tx.Exec(ctx, `INSERT INTO clone_image_override_intents(job_id,expected_revision,manifest) VALUES($1,$2,$3::jsonb) ON CONFLICT DO NOTHING`, id, revision, raw)
	if err != nil { return err }
	if result.RowsAffected() != 1 { return jobedit.Conflict("Đang chờ áp dụng prompt ảnh trước đó") }
	return nil
}

func clearReviewOrigin(raw json.RawMessage) (json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil { return nil, err }
	delete(fields, "reviewOrigin")
	return json.Marshal(fields)
}

func startsImageFlight(previous, next json.RawMessage) bool {
	var old, current images.Manifest
	if json.Unmarshal(previous, &old) != nil || json.Unmarshal(next, &current) != nil { return false }
	for i, item := range current.Items {
		if item.InFlight == nil { continue }
		if i >= len(old.Items) || old.Items[i].ID != item.ID || old.Items[i].InFlight == nil { return true }
	}
	return false
}
