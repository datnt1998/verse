package main

import (
	"context"
	"fmt"
	"github.com/datnt1998/verse/apps/viderse/internal/insight"
	"github.com/datnt1998/verse/core/llm"
	"time"
)

// The model phrases the observed facts; insight rejects output that changes
// the verified sentence, and falls back to the deterministic DB summary.
func recapNarrator(ai *aiWiring) func(context.Context, insight.Recap) (string, error) {
	return func(ctx context.Context, recap insight.Recap) (string, error) {
		chain, err := ai.Service.ResolveDirector(ctx)
		if err != nil {
			return "", err
		}
		models := ai.Executors.textModels(ctx, chain)
		if len(models) == 0 {
			return "", fmt.Errorf("chưa kết nối model Đạo diễn")
		}
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		prompt := "Viết một câu Bản tin bằng đúng các dữ kiện sau, không thêm dữ kiện. Giữ nguyên số và tên trạng thái. Có thể mở đầu bằng 'Trong lúc bạn vắng' hoặc 'Khi bạn trở lại'. Chỉ trả câu đó: " + recap.Summary
		response, err := (cloneText{models}).Generate(ctx, []llm.Message{{Role: llm.RoleUser, Content: []llm.ContentBlock{llm.TextBlock(prompt)}}}, nil)
		if err != nil {
			return "", err
		}
		return response.Message.TextContent(), nil
	}
}
