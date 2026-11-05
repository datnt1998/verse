package images

import (
	"context"

	"github.com/datnt1998/verse/apps/viderse/internal/autonomy"
	model "github.com/datnt1998/verse/apps/viderse/internal/pipeline/steps/images"
)

// AutoReviewer cho phần tự duyệt đọc điểm ảnh và duyệt qua đúng đường "Duyệt hết"
// của chủ kênh: cùng cổng ImagesReady/mutable và cùng đường chủ duyệt
// (CommitOwnerApproval khi còn ảnh chờ, StageDraftApproval khi bản dựng đã đỗ).
type AutoReviewer struct{ Service *Service }

var _ autonomy.Reviewer = AutoReviewer{}

// ReviewState đọc manifest ảnh: revision, điểm từng mục, và đã đủ để duyệt chưa.
func (r AutoReviewer) ReviewState(ctx context.Context, jobID string) (autonomy.ReviewState, error) {
	l, err := r.Service.load(ctx, jobID, nil, false)
	if err != nil {
		return autonomy.ReviewState{}, err
	}
	state := autonomy.ReviewState{Revision: l.manifest.Revision, Ready: model.ImagesReady(l.manifest) && r.Service.mutable(l) == nil}
	scene := 0
	for _, i := range l.manifest.Items {
		if i.Kind == "scene" {
			scene++
		}
		item := autonomy.ItemScore{ID: i.ID, Kind: i.Kind, Label: autonomy.ItemLabel(i.Kind, scene), Skipped: i.Status == "skipped"}
		if v := model.Displayed(i); v != nil && v.ScoreIssue == "" {
			item.Score = v.Score
		}
		state.Items = append(state.Items, item)
	}
	return state, nil
}

// ApproveAll duyệt toàn bộ ảnh ở đúng revision và chuyển video sang xuất.
func (r AutoReviewer) ApproveAll(ctx context.Context, jobID string, revision int) error {
	_, err := r.Service.ApproveAll(ctx, jobID, RevisionInput{ExpectedRevision: &revision, ApproveExport: true})
	return err
}
