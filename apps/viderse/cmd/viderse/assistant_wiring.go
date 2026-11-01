package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/datnt1998/verse/apps/viderse/internal/activity"
	"github.com/datnt1998/verse/apps/viderse/internal/assistant"
	"github.com/datnt1998/verse/apps/viderse/internal/channels"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/domain/quality"
	"github.com/datnt1998/verse/apps/viderse/internal/learn"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
	"github.com/datnt1998/verse/apps/viderse/internal/pipeline"
	"github.com/datnt1998/verse/apps/viderse/internal/render"
	"github.com/datnt1998/verse/apps/viderse/internal/rivals"
	"github.com/datnt1998/verse/apps/viderse/internal/web"
	"github.com/datnt1998/verse/core/llmretry"
)

// assistantSeams gom các service mà mountClonePipeline và
// mountCloneOwnerOperations dựng ra; trợ lý dùng ĐÚNG các instance đó, nên mỗi
// `run` đi cùng đường với nút bấm tương ứng. serve.go truyền một con trỏ trống,
// hai hàm ráp điền vào, rồi mountAssistant đọc.
type assistantSeams struct {
	Jobs       assistant.Jobs
	Images     assistant.Images
	Backdrops  assistant.Backdrops
	Metadata   assistant.Metadata
	Queue      assistant.Queue
	Autonomy   assistant.Autonomy
	Quality    assistant.Quality
	Posted     assistant.Posted
	ImagesLeft func(context.Context) (int, error)
	// Learn là Quy tắc đã học + Đề xuất (learn_wiring.go); serve.go dựng trước.
	Learn *learnRuntime
}

// assistantRules đưa quy tắc trợ lý vào Quy tắc đã học của kênh.
type assistantRules struct{ svc *learn.Service }

func (r assistantRules) AddRule(ctx context.Context, channelID, text, source string) (string, error) {
	return r.svc.AddRule(ctx, learn.RuleInput{ChannelID: channelID, Text: text, Source: source})
}

// directorModel trả model Đạo diễn hiện hành (chuỗi dự phòng như bước Viết
// metadata); lỗi thật khi chưa kết nối — trợ lý không tự đoán thay model.
func directorModel(ai *aiWiring) func(context.Context) (llmretry.Generator, error) {
	return func(ctx context.Context) (llmretry.Generator, error) {
		chain, err := ai.Service.ResolveDirector(ctx)
		if err != nil {
			return nil, err
		}
		// Thẻ đăng nhập gói chưa đăng nhập/hết hạn: client codex vẫn dựng được
		// rồi treo ở bước lấy token — loại ngay ở đây để báo lỗi tức thì.
		subs, err := ai.Service.Subscribed(ctx)
		if err != nil {
			return nil, err
		}
		notReady := map[string]bool{}
		for _, v := range subs {
			notReady[v.ID] = v.Status != domain.StatusReady
		}
		usable := chain[:0:0]
		for _, ref := range chain {
			if !notReady[ref.Provider] {
				usable = append(usable, ref)
			}
		}
		models := ai.Executors.textModels(ctx, usable)
		if len(models) == 0 {
			return nil, fmt.Errorf("chưa kết nối model Đạo diễn")
		}
		return cloneText{models}, nil
	}
}

// mountAssistant ráp /api/assistant/* và trả hàm dừng các lượt chạy nền.
func mountAssistant(g *web.Gateway, pool *pgxpool.Pool, ai *aiWiring, channelSvc *channels.Service, activities *activity.Service, s *assistantSeams) func() {
	svc := assistant.New(assistant.Deps{
		Director:   directorModel(ai),
		Jobs:       s.Jobs,
		Images:     s.Images,
		Backdrops:  s.Backdrops,
		Metadata:   s.Metadata,
		Channels:   channelSvc,
		Queue:      s.Queue,
		Rivals:     rivals.New(postgres.NewRivalsStore(pool)),
		Autonomy:   s.Autonomy,
		Rules:      assistantRules{svc: &learn.Service{Store: postgres.NewLearnStore(pool)}},
		Tasks:      activities,
		DrawSources: drawSource{ai: ai},
		Quality:     s.Quality,
		Posted:      s.Posted,
		ImagesLeft:  s.ImagesLeft,
		Diag:        func(err error) { fmt.Fprintln(os.Stderr, "Trợ lý:", err) },
	})
	web.MountAssistant(g, svc)
	return svc.Close
}

// drawSource tìm nguồn vẽ ảnh ĐÃ KẾT NỐI theo tên người dùng nói (/provider):
// thẻ nhà cung cấp đúng loại, có model ảnh đã chọn và có đường vẽ thật. Không
// thay bằng nguồn khác khi thiếu — trợ lý báo chỗ cần sửa.
type drawSource struct{ ai *aiWiring }

func (d drawSource) Ref(ctx context.Context, source string) (domain.ModelRef, bool, error) {
	kind := domain.ProviderKind(strings.ToLower(strings.TrimSpace(source)))
	switch kind {
	case domain.KindChatGPT, domain.KindGrok, domain.KindGemini:
	default:
		return domain.ModelRef{}, false, nil
	}
	providers, err := d.ai.Service.Providers(ctx)
	if err != nil {
		return domain.ModelRef{}, false, err
	}
	for _, p := range providers {
		if p.Kind != kind {
			continue
		}
		// Thẻ đăng nhập gói phải đang sẵn sàng; thẻ khoá API tự kiểm ở CanDraw.
		if p.Auth == domain.AuthSubscription && p.Status != domain.StatusReady {
			continue
		}
		models, err := d.ai.Service.Models(ctx, p.ID, "", "selected", "", 100)
		if err != nil {
			return domain.ModelRef{}, false, err
		}
		for _, m := range models.Models {
			if !hasCapability(m, domain.CapImage) {
				continue
			}
			ref := domain.ModelRef{Provider: p.ID, Model: m.ID}
			if !d.ai.Executors.CanDraw(ctx, ref) {
				continue
			}
			return ref, true, nil
		}
	}
	return domain.ModelRef{}, false, nil
}

// qualityGate dùng ĐÚNG cổng và đường xuất của nút “Vẫn xuất” ở Bàn dựng
// (POST /api/clone/:id/export {accept:true}).
type qualityGate struct{ svc *render.Service }

func (q qualityGate) Gate(ctx context.Context, jobID string) (quality.GateDecision, error) {
	return q.svc.Gate(ctx, jobID)
}

func (q qualityGate) ExportAnyway(ctx context.Context, jobID string) error {
	_, err := q.svc.BeginCloneExport(ctx, jobID, true)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, pipeline.ErrNotFound):
		return &domain.CloneError{Code: domain.CloneErrNotFound, Message: "Không tìm thấy video này."}
	case errors.Is(err, pipeline.ErrConflict):
		return &domain.CloneError{Code: domain.CloneErrConflict, Message: "Video đang bận. Chờ một lát rồi thử lại."}
	default:
		return err
	}
}
