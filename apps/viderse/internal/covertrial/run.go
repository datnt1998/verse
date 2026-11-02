package covertrial

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"image"
	_ "image/png"
	"os"
	"path/filepath"
	"time"

	"github.com/datnt1998/verse/apps/viderse/internal/imagegen"
	"github.com/datnt1998/verse/apps/viderse/internal/library"
	"github.com/datnt1998/verse/apps/viderse/internal/media"
)

// trialKeep là số lần vẽ thử giữ lại cho mỗi template/prompt.
const trialKeep = 12

// terminalTimeout là trần ghi trạng thái cuối khi ctx đã bị huỷ.
const terminalTimeout = 10 * time.Second

// waitFlightsTimeout là trần chờ hàng lịch sử chốt xong khi xoá template/prompt
// (mỗi lần vẽ tự chốt trong terminalTimeout, cộng biên cho lần chốt cuối).
const waitFlightsTimeout = terminalTimeout + 5*time.Second

// runCover vẽ và lưu MỘT ảnh bìa thử. Mọi chuyển trạng thái nằm ở đây để
// Cancel chỉ đánh dấu huỷ; hàng lịch sử luôn kết thúc running → ready/failed/
// cancelled, và huỷ đã nhận thì không bao giờ ghi ready.
func (s *Service) runCover(ctx context.Context, f *flight, templateID string, row library.CoverTrial, taskID string, res DrawerResolution) {
	defer s.endFlight(flightKey("t", templateID, row.ID))
	defer s.finishTask(ctx, taskID)

	s.phase(ctx, taskID, "Đang đọc video nguồn")
	src, err := s.loadSource(ctx, row.JobID)
	if err != nil {
		s.concludeCover(ctx, f, row, err, false, nil)
		return
	}
	prompt, err := s.opts.Prompts.GetPrompt(ctx, row.PromptID)
	if err != nil {
		s.concludeCover(ctx, f, row, promptError(err), false, nil)
		return
	}
	s.phase(ctx, taskID, "Đang ghép chỉ dẫn")
	promptText, width, height, files, release, err := s.composeCover(ctx, src, prompt, res.Provider, res.RefLimit)
	if err != nil {
		s.concludeCover(ctx, f, row, err, false, nil)
		return
	}
	defer release()

	s.phase(ctx, taskID, "Đang vẽ ảnh")
	out := outputPath(row.ID)
	sent, receipt, err := s.draw(ctx, taskID, res, imagegen.Request{
		Key:        "cover-trial:" + row.ID,
		Prompt:     promptText,
		Output:     out,
		Width:      width,
		Height:     height,
		References: files,
		OnPhase: func(phase string) {
			if phase == "drawing" {
				s.phase(ctx, taskID, "Nhà cung cấp đang vẽ")
			}
		},
	})
	if err != nil {
		s.concludeCover(ctx, f, row, err, sent, nil)
		return
	}
	defer os.Remove(out)
	imgWidth, imgHeight, err := pngSize(out)
	if err != nil {
		s.concludeCover(ctx, f, row, err, true, nil)
		return
	}
	path := coverTrialPath(templateID, row.ID)
	now := s.now()
	row.State = library.TrialReady
	row.MediaPath = path
	row.Provider = firstNonEmpty(receipt.Provider, res.Provider)
	row.Model = firstNonEmpty(receipt.Model, res.Model)
	row.Width, row.Height = imgWidth, imgHeight
	row.CostImages = 1
	row.FinishedAt = &now
	row.Error = nil

	s.phase(ctx, taskID, "Đang lưu ảnh")
	ready := func(txctx context.Context) error {
		return s.opts.Media.Publish(txctx, []media.Artifact{{
			Path: path, File: out, ContentType: "image/png", Filename: "anh-thu.png",
		}}, func(commitCtx context.Context) error {
			return s.opts.Store.UpdateCoverTrial(commitCtx, row)
		})
	}
	if s.concludeCover(ctx, f, row, nil, true, ready) {
		s.pruneCover(ctx, templateID)
	}
}

// runPromptSample vẽ và lưu MỘT ảnh mẫu của prompt.
func (s *Service) runPromptSample(ctx context.Context, f *flight, promptID string, row library.PromptSample, taskID string, res DrawerResolution) {
	defer s.endFlight(flightKey("s", promptID, row.ID))
	defer s.finishTask(ctx, taskID)

	s.phase(ctx, taskID, "Đang đọc video nguồn")
	src, err := s.loadSource(ctx, row.VideoID)
	if err != nil {
		s.concludeSample(ctx, f, row, err, false, nil)
		return
	}
	prompt, err := s.opts.Prompts.GetPrompt(ctx, row.PromptID)
	if err != nil {
		s.concludeSample(ctx, f, row, promptError(err), false, nil)
		return
	}
	s.phase(ctx, taskID, "Đang ghép chỉ dẫn")
	var promptText string
	var width, height int
	var files []string
	var release func()
	if row.PromptKind == "cover" {
		promptText, width, height, files, release, err = s.composeCover(ctx, src, prompt, res.Provider, res.RefLimit)
	} else {
		promptText, width, height, files, release, err = s.composeScene(ctx, src, prompt, row.SceneID, res.Provider, res.RefLimit)
	}
	if err != nil {
		s.concludeSample(ctx, f, row, err, false, nil)
		return
	}
	defer release()

	s.phase(ctx, taskID, "Đang vẽ ảnh")
	out := outputPath(row.ID)
	sent, receipt, err := s.draw(ctx, taskID, res, imagegen.Request{
		Key:        "prompt-sample:" + row.ID,
		Prompt:     promptText,
		Output:     out,
		Width:      width,
		Height:     height,
		References: files,
		OnPhase: func(phase string) {
			if phase == "drawing" {
				s.phase(ctx, taskID, "Nhà cung cấp đang vẽ")
			}
		},
	})
	if err != nil {
		s.concludeSample(ctx, f, row, err, sent, nil)
		return
	}
	defer os.Remove(out)
	imgWidth, imgHeight, err := pngSize(out)
	if err != nil {
		s.concludeSample(ctx, f, row, err, true, nil)
		return
	}
	path := promptSamplePath(promptID, row.ID)
	now := s.now()
	row.State = library.TrialReady
	row.MediaPath = path
	row.Provider = firstNonEmpty(receipt.Provider, res.Provider)
	row.Model = firstNonEmpty(receipt.Model, res.Model)
	row.Width, row.Height = imgWidth, imgHeight
	row.CostImages = 1
	row.FinishedAt = &now
	row.Error = nil

	s.phase(ctx, taskID, "Đang lưu ảnh")
	ready := func(txctx context.Context) error {
		return s.opts.Media.Publish(txctx, []media.Artifact{{
			Path: path, File: out, ContentType: "image/png", Filename: "anh-mau.png",
		}}, func(commitCtx context.Context) error {
			return s.opts.Store.UpdatePromptSample(commitCtx, row)
		})
	}
	if s.concludeSample(ctx, f, row, nil, true, ready) {
		s.pruneSamples(ctx, promptID)
	}
}

// draw gọi provider dưới pacer ảnh dùng chung; trả whether the paid call was
// sent (cost honest) and the output receipt.
func (s *Service) draw(ctx context.Context, taskID string, res DrawerResolution, req imagegen.Request) (bool, imagegen.Receipt, error) {
	if res.Drawer == nil {
		return false, imagegen.Receipt{}, trialUnavailable("Chưa sẵn sàng vẽ ảnh")
	}
	var receipt imagegen.Receipt
	sent := false
	original := req.OnPhase
	req.OnPhase = func(phase string) {
		if phase == "sending" {
			sent = true
			s.phase(ctx, taskID, "Đang vẽ ảnh")
		}
		if original != nil {
			original(phase)
		}
	}
	err := s.withSlot(ctx, func(slotCtx context.Context) error {
		var drawErr error
		receipt, drawErr = res.Drawer.Draw(slotCtx, req)
		return drawErr
	})
	return sent, receipt, err
}

// concludeCover chốt trạng thái cuối của một lần vẽ ảnh bìa dưới fence của
// flight. Huỷ đã nhận thì không bao giờ ghi ready (ảnh vừa vẽ bị bỏ); kết quả
// ready chỉ sống khi transaction bind media + ghi hàng thành công. Trả true
// khi hàng đã thành ready.
func (s *Service) concludeCover(ctx context.Context, f *flight, row library.CoverTrial, err error, sent bool, ready func(context.Context) error) bool {
	f.mu.Lock()
	defer func() {
		f.settled = true
		f.mu.Unlock()
	}()
	if f.cancelled {
		s.coverTerminal(ctx, row, context.Canceled, sent)
		return false
	}
	if err != nil {
		s.coverTerminal(ctx, row, err, sent)
		return false
	}
	saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), terminalTimeout)
	defer cancel()
	if publishErr := ready(saveCtx); publishErr != nil {
		if errors.Is(publishErr, library.ErrNotFound) {
			return false // template/lần vẽ đã bị xoá: bỏ ảnh, không tạo tham chiếu mồ côi.
		}
		s.coverTerminal(ctx, row, publishErr, sent)
		return false
	}
	return true
}

// concludeSample là bản ảnh mẫu của concludeCover.
func (s *Service) concludeSample(ctx context.Context, f *flight, row library.PromptSample, err error, sent bool, ready func(context.Context) error) bool {
	f.mu.Lock()
	defer func() {
		f.settled = true
		f.mu.Unlock()
	}()
	if f.cancelled {
		s.sampleTerminal(ctx, row, context.Canceled, sent)
		return false
	}
	if err != nil {
		s.sampleTerminal(ctx, row, err, sent)
		return false
	}
	saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), terminalTimeout)
	defer cancel()
	if publishErr := ready(saveCtx); publishErr != nil {
		if errors.Is(publishErr, library.ErrNotFound) {
			return false // prompt/ảnh mẫu đã bị xoá.
		}
		s.sampleTerminal(ctx, row, publishErr, sent)
		return false
	}
	return true
}

// coverTerminal ghi cancelled/failed với cost thật: đã gửi lời gọi trả phí thì
// 1 ảnh, chưa gửi hoặc lỗi chắc chắn chưa tiêu phí thì 0. Không tự thử lại.
func (s *Service) coverTerminal(ctx context.Context, row library.CoverTrial, err error, sent bool) {
	now := s.now()
	row.CostImages = 0
	if sent && !imagegen.IsUnpaid(err) {
		row.CostImages = 1
	}
	row.FinishedAt = &now
	if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
		row.State = library.TrialCancelled
		row.Error = nil
	} else {
		row.State = library.TrialFailed
		row.Error = trialError(err)
	}
	s.persistCover(ctx, row)
}

func (s *Service) sampleTerminal(ctx context.Context, row library.PromptSample, err error, sent bool) {
	now := s.now()
	row.CostImages = 0
	if sent && !imagegen.IsUnpaid(err) {
		row.CostImages = 1
	}
	row.FinishedAt = &now
	if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
		row.State = library.TrialCancelled
		row.Error = nil
	} else {
		row.State = library.TrialFailed
		row.Error = trialError(err)
	}
	s.persistSample(ctx, row)
}

func (s *Service) persistCover(ctx context.Context, row library.CoverTrial) {
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), terminalTimeout)
	defer cancel()
	_ = s.opts.Store.UpdateCoverTrial(writeCtx, row)
}

func (s *Service) persistSample(ctx context.Context, row library.PromptSample) {
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), terminalTimeout)
	defer cancel()
	_ = s.opts.Store.UpdatePromptSample(writeCtx, row)
}

// trialError phân loại lỗi vẽ thành lỗi nghiệp vụ hiển thị được.
func trialError(err error) *library.TrialError {
	var le *library.Error
	if errors.As(err, &le) {
		return &library.TrialError{Code: le.Code, Message: le.Message, Recovery: le.Recovery, RetryAt: le.RetryAt}
	}
	var drawErr *imagegen.DrawError
	if errors.As(err, &drawErr) {
		return &library.TrialError{Code: drawErr.Code, Message: drawErr.Message, Recovery: drawErr.Recovery, RetryAt: drawErr.RetryAt}
	}
	var capErr *imagegen.CapError
	if errors.As(err, &capErr) {
		return &library.TrialError{
			Code:     library.CodeQuota,
			Message:  "Đã hết hạn mức ảnh trả phí hôm nay",
			Recovery: refillRecovery(&capErr.RetryAt),
			RetryAt:  &capErr.RetryAt,
		}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &library.TrialError{Code: "timeout", Message: "Lần vẽ thử quá thời gian chờ", Recovery: "Thử lại"}
	}
	return &library.TrialError{Code: "internal_error", Message: "Không vẽ được ảnh thử", Recovery: "Thử lại"}
}

// pruneCover hạ lịch sử template về 12 hàng, nhả media của hàng bị bỏ. Dùng
// ctx tách khỏi lần vẽ để huỷ muộn không bỏ dở bước dọn.
func (s *Service) pruneCover(ctx context.Context, templateID string) {
	pruneCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), terminalTimeout)
	defer cancel()
	removed, err := s.opts.Store.CoverTrialsBeyond(pruneCtx, templateID, trialKeep)
	if err != nil || len(removed) == 0 {
		return
	}
	paths := make([]string, 0, len(removed))
	ids := make([]string, 0, len(removed))
	for _, row := range removed {
		ids = append(ids, row.ID)
		if row.MediaPath != "" {
			paths = append(paths, row.MediaPath)
		}
	}
	if len(paths) == 0 {
		_ = s.opts.Store.DeleteCoverTrials(pruneCtx, ids)
		return
	}
	_ = s.opts.Media.Unbind(pruneCtx, paths, func(txctx context.Context) error {
		return s.opts.Store.DeleteCoverTrials(txctx, ids)
	})
}

// pruneSamples hạ lịch sử prompt về 12 hàng, nhả media của hàng bị bỏ.
func (s *Service) pruneSamples(ctx context.Context, promptID string) {
	pruneCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), terminalTimeout)
	defer cancel()
	removed, err := s.opts.Store.PromptSamplesBeyond(pruneCtx, promptID, trialKeep)
	if err != nil || len(removed) == 0 {
		return
	}
	paths := make([]string, 0, len(removed))
	ids := make([]string, 0, len(removed))
	for _, row := range removed {
		ids = append(ids, row.ID)
		if row.MediaPath != "" {
			paths = append(paths, row.MediaPath)
		}
	}
	if len(paths) == 0 {
		_ = s.opts.Store.DeletePromptSamples(pruneCtx, ids)
		return
	}
	_ = s.opts.Media.Unbind(pruneCtx, paths, func(txctx context.Context) error {
		return s.opts.Store.DeletePromptSamples(txctx, ids)
	})
}

// outputPath tạo tệp đích riêng cho một lần vẽ (không đè lần trước).
func outputPath(id string) string {
	return filepath.Join(os.TempDir(), "viderse-trial-"+id+".png")
}

func coverTrialPath(templateID, trialID string) string {
	return "library/cover-trials/" + templateID + "/" + trialID + ".png"
}

func promptSamplePath(promptID, sampleID string) string {
	safe := promptID
	if len(safe) > 80 {
		safe = safe[:80]
	}
	return "library/prompt-samples/" + safe + "/" + sampleID + ".png"
}

// pngSize đọc kích thước thật của ảnh đã vẽ (không đoán theo canvas yêu cầu).
func pngSize(file string) (int, int, error) {
	f, err := os.Open(file)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()
	config, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0, fmt.Errorf("ảnh thử không đọc được: %w", err)
	}
	return config.Width, config.Height, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func newUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
