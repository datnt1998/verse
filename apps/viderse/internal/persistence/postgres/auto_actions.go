package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/insight"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type redrawItem struct {
	ID       string `json:"id"`
	Modest   bool   `json:"modest"`
	Status   string `json:"status"`
	InFlight *struct {
		GenerationID string `json:"generationId"`
	} `json:"inFlight"`
}
type redrawCheckpoint struct {
	Items []redrawItem `json:"items"`
}

func imageRedrawTransitions(previous, current json.RawMessage) []string {
	var old, next redrawCheckpoint
	if json.Unmarshal(previous, &old) != nil || json.Unmarshal(current, &next) != nil {
		return nil
	}
	byID := make(map[string]redrawItem, len(old.Items))
	for _, item := range old.Items {
		byID[item.ID] = item
	}
	var ids []string
	for _, item := range next.Items {
		before, ok := byID[item.ID]
		if ok && !before.Modest && item.Modest && item.Status == "running" && before.InFlight != nil && item.InFlight != nil && before.InFlight.GenerationID != "" && before.InFlight.GenerationID == item.InFlight.GenerationID {
			ids = append(ids, item.ID)
		}
	}
	return ids
}

func insertAutoAction(ctx context.Context, jobID pgtype.UUID, kind, text string, reason insight.Reason) error {
	tx, ok := ctx.Value(mediaTxKey{}).(pgx.Tx)
	if !ok {
		return fmt.Errorf("ghi hành động tự động cần giao dịch")
	}
	raw, err := json.Marshal(reason)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO clone_auto_actions(job_id,kind,text,reason) VALUES($1,$2,$3,$4::jsonb)`, jobID, kind, text, raw)
	return err
}

func recordImageAutoActions(ctx context.Context, jobID pgtype.UUID, previous, current json.RawMessage) error {
	for _, id := range imageRedrawTransitions(previous, current) {
		reason := insight.Reason{Agent: domain.AgentArtist, Text: "Bộ tạo ảnh từ chối yêu cầu trước; Hoạ sĩ điều chỉnh lời nhắc an toàn rồi thử lại.", Facts: []insight.Fact{
			{Label: "Ảnh", Value: id, Source: "Bước Ảnh"},
			{Label: "Kết quả", Value: "Bị từ chối", Source: "Bộ tạo ảnh"},
		}}
		if err := insertAutoAction(ctx, jobID, "safety_redraw", "Tự điều chỉnh và vẽ lại ảnh", reason); err != nil {
			return err
		}
	}
	return nil
}

func recordStepAutoRetry(ctx context.Context, jobID pgtype.UUID, step string) error {
	reason := insight.Reason{Agent: "system", Text: "Bước đã hết thời gian chờ thử lại tự động.", Facts: []insight.Fact{
		{Label: "Bước", Value: step, Source: "Tiến trình"},
		{Label: "Trạng thái trước", Value: "Chờ thử lại", Source: "Tiến trình"},
	}}
	return insertAutoAction(ctx, jobID, "step_retry", "Tự thử lại bước "+step, reason)
}
