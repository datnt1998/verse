package revision

import (
	llmtypes "github.com/datnt1998/verse/core/llm"

	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/datnt1998/verse/apps/noverse/internal/chapterfacts"
	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/core/agentcore/schema"
	"github.com/datnt1998/verse/core/llmcontract"
)

var analysisContract = llmcontract.Contract{
	Name:        "chapter_revision_analysis",
	Description: "Phân tích bản sửa của người dùng trên chương đã hoàn thành và dựng lại đầy đủ sự kiện chương",
	Schema:      revisionAnalysisSchema(),
}

func revisionAnalysisSchema() map[string]any {
	textList := func(description string) map[string]any { return schema.Array(description, schema.String(description)) }
	voice := schema.Object(
		schema.Property("name", schema.String("Tên nhân vật")).Required(),
		schema.Property("rules", textList("Sở thích đối thoại")).Required(),
	)
	facts := schema.Object(chapterfacts.Properties(false)...)
	impact := schema.Object(
		schema.Property("deviation", schema.String("Thay đổi của tình tiết đã xảy ra so với kế hoạch hiện có")).Required(),
		schema.Property("suggestion", schema.String("Đề xuất điều chỉnh cho đại cương chưa hoàn thành")).Required(),
	)
	return schema.Object(
		schema.Property("change_summary", schema.String("Tóm tắt chỉnh sửa")).Required(),
		schema.Property("story_changed", schema.Bool("Có thay đổi sự kiện tình tiết không")).Required(),
		schema.Property("facts", facts).Required(),
		schema.Property("style_delta", schema.Object(
			schema.Property("prose", textList("Sở thích trần thuật xác nhận từ lần sửa này")).Required(),
			schema.Property("dialogue", schema.Array("Sở thích đối thoại của nhân vật", voice)).Required(),
			schema.Property("taboos", textList("Điều cấm thể hiện qua việc người dùng chủ động xoá/sửa")).Required(),
		)).Required(),
		schema.Property("outline_impact", llmcontract.Nullable(impact)).Required(),
		schema.Property("downstream_issues", textList("Xung đột tiềm ẩn với các chương đã hoàn thành phía sau")).Required(),
	)
}

func Analyze(ctx context.Context, model llmtypes.ChatModel, systemPrompt string, change Change, previous domain.ChapterRecord, downstream []domain.ChapterSummary) (domain.RevisionAnalysis, error) {
	if model == nil {
		return domain.RevisionAnalysis{}, fmt.Errorf("cần model cho revision")
	}
	if strings.TrimSpace(systemPrompt) == "" {
		return domain.RevisionAnalysis{}, fmt.Errorf("cần prompt cho revision")
	}
	payload, err := json.Marshal(map[string]any{
		"chapter": change.Chapter, "previous_facts": previous.Facts,
		"revised_content": change.After, "changed_excerpt": changedExcerpt(change.Before, change.After),
		"downstream_summaries": downstream,
	})
	if err != nil {
		return domain.RevisionAnalysis{}, err
	}
	analysis, err := llmcontract.Execute(ctx, model, llmcontract.Request[domain.RevisionAnalysis]{
		Contract: analysisContract, SystemPrompt: systemPrompt, Payload: string(payload), Agent: "revision",
		Validate: func(a *domain.RevisionAnalysis) error { return validateAnalysis(a, change.After) },
		Hooks: llmcontract.Hooks{
			Resolved: func(res llmcontract.Resolution) {
				slog.Debug("Chọn giao thức cấu trúc cho bản sửa chương", "mode", res.Mode, "provider", res.Provider, "model", res.Model)
			},
			Correction: func(ev llmcontract.Correction) {
				slog.Warn("Sửa đầu ra bản sửa chương", "attempt", ev.Attempt, "layer", ev.Layer, "err", ev.Err)
			},
		},
	})
	if err != nil {
		return domain.RevisionAnalysis{}, fmt.Errorf("phân tích bản sửa chương %d: %w", change.Chapter, err)
	}
	analysis.Facts.Feedback = analysis.OutlineImpact
	return analysis, nil
}

func validateAnalysis(analysis *domain.RevisionAnalysis, revisedText string) error {
	if strings.TrimSpace(analysis.ChangeSummary) == "" {
		return fmt.Errorf("change_summary không được để trống")
	}
	if err := chapterfacts.Validate(analysis.Facts); err != nil {
		return fmt.Errorf("facts: %w", err)
	}
	// 修订后的正文就是新的事实文本：模型附的 quote 必须能在其中定位，错误
	// 计入 Validate 失败，走 llmcontract 既有纠错重问。
	if err := chapterfacts.ValidateQuotes(revisedText, analysis.Facts); err != nil {
		return fmt.Errorf("facts: %v", err)
	}
	if analysis.OutlineImpact != nil && (strings.TrimSpace(analysis.OutlineImpact.Deviation) == "" || strings.TrimSpace(analysis.OutlineImpact.Suggestion) == "") {
		return fmt.Errorf("outline_impact cần deviation và suggestion")
	}
	if err := validateStyleDelta(analysis.StyleDelta); err != nil {
		return err
	}
	for i, issue := range analysis.DownstreamIssues {
		if strings.TrimSpace(issue) == "" {
			return fmt.Errorf("downstream_issues[%d] không được để trống", i)
		}
	}
	return nil
}

func validateStyleDelta(style domain.StyleDelta) error {
	for name, items := range map[string][]string{"prose": style.Prose, "taboos": style.Taboos} {
		for i, item := range items {
			if strings.TrimSpace(item) == "" {
				return fmt.Errorf("style_delta.%s[%d] không được để trống", name, i)
			}
		}
	}
	for i, voice := range style.Dialogue {
		if strings.TrimSpace(voice.Name) == "" || len(voice.Rules) == 0 {
			return fmt.Errorf("style_delta.dialogue[%d] cần name và rules", i)
		}
		for j, rule := range voice.Rules {
			if strings.TrimSpace(rule) == "" {
				return fmt.Errorf("style_delta.dialogue[%d].rules[%d] không được để trống", i, j)
			}
		}
	}
	return nil
}

type excerpt struct {
	BeforeStart int    `json:"before_start_line"`
	BeforeEnd   int    `json:"before_end_line"`
	Before      string `json:"before"`
	AfterStart  int    `json:"after_start_line"`
	AfterEnd    int    `json:"after_end_line"`
	After       string `json:"after"`
}

func changedExcerpt(before, after string) excerpt {
	oldLines := strings.Split(domain.NormalizeChapterContent(before), "\n")
	newLines := strings.Split(domain.NormalizeChapterContent(after), "\n")
	prefix := 0
	for prefix < len(oldLines) && prefix < len(newLines) && oldLines[prefix] == newLines[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < len(oldLines)-prefix && suffix < len(newLines)-prefix &&
		oldLines[len(oldLines)-1-suffix] == newLines[len(newLines)-1-suffix] {
		suffix++
	}
	oldChanged := oldLines[prefix : len(oldLines)-suffix]
	newChanged := newLines[prefix : len(newLines)-suffix]
	return excerpt{
		BeforeStart: prefix + 1, BeforeEnd: prefix + len(oldChanged), Before: strings.Join(oldChanged, "\n"),
		AfterStart: prefix + 1, AfterEnd: prefix + len(newChanged), After: strings.Join(newChanged, "\n"),
	}
}
