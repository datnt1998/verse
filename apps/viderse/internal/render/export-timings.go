package render

import "math"

// ExportTimings is a cumulative browser snapshot, not a sum of independent
// stages: rendering overlaps encoding, and EncoderWaitMS is residual backpressure.
// Preparation excludes the headless claim, score gate and export preflight.
type ExportTimings struct {
	PrepareMS     float64 `json:"prepareMs"`
	RenderMS      float64 `json:"renderMs"`
	CaptureMS     float64 `json:"captureMs"`
	EncoderWaitMS float64 `json:"encoderWaitMs"`
	FinalizeMS    float64 `json:"finalizeMs"`
	UploadMS      float64 `json:"uploadMs"`
	PublishMS     float64 `json:"publishMs"`
	Frames        int64   `json:"frames"`
}

func (t ExportTimings) validate() error {
	if t.Frames < 0 {
		return requestError("invalid_request", "Số khung hình đã đo không hợp lệ")
	}
	for _, value := range [...]float64{t.PrepareMS, t.RenderMS, t.CaptureMS, t.EncoderWaitMS, t.FinalizeMS, t.UploadMS, t.PublishMS} {
		if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
			return requestError("invalid_request", "Thời gian xuất đã đo không hợp lệ")
		}
	}
	return nil
}

// ExportPostprocessTimings measures the server work after browser publication.
// Source preparation includes metadata and original-file acquisition; replacement
// finalization includes output hashing, publication and lease/workspace cleanup.
type ExportPostprocessTimings struct {
	TotalMS               int64 `json:"totalMs"`
	SourcePrepareMS       int64 `json:"sourcePrepareMs"`
	CompressMS            int64 `json:"compressMs"`
	ReplacementFinalizeMS int64 `json:"replacementFinalizeMs"`
	OriginalAcquireMS     int64 `json:"originalAcquireMs"`
	CompressionAttempted  bool  `json:"compressionAttempted"`
	CompressionApplied    bool  `json:"compressionApplied"`
}
