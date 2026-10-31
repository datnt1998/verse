package domain

import (
	"encoding/json"
	"time"
)

type CloneStep string

const (
	CloneDownload   CloneStep = "download"
	CloneBackdrop   CloneStep = "backdrop"
	CloneTranscript CloneStep = "transcript"
	CloneMetadata   CloneStep = "metadata"
	CloneScenes     CloneStep = "scenes"
	CloneImages     CloneStep = "images"
	CloneBuild      CloneStep = "build"
	CloneScore      CloneStep = "score"
	CloneExport     CloneStep = "export"
)

var CloneSteps = [...]CloneStep{CloneDownload, CloneBackdrop, CloneTranscript, CloneMetadata, CloneScenes, CloneImages, CloneBuild, CloneScore, CloneExport}

type CloneStatus string

const (
	ClonePending   CloneStatus = "pending"
	CloneRunning   CloneStatus = "running"
	CloneWaiting   CloneStatus = "waiting"
	CloneSucceeded CloneStatus = "succeeded"
	CloneFailed    CloneStatus = "failed"
	CloneCancelled CloneStatus = "cancelled"
	CloneStale     CloneStatus = "stale"
)

type ClonePriority string

const (
	ClonePick  ClonePriority = "pick"
	CloneBatch ClonePriority = "batch"
)

type CloneError struct {
	Code          string     `json:"code"`
	Message       string     `json:"message"`
	Recovery      *string    `json:"recovery"`
	RetryAt       *time.Time `json:"retryAt"`
	RetryAtSource string     `json:"retryAtSource,omitempty"`
}

func (e *CloneError) Error() string { return e.Message }

type CloneSourceSnapshot struct {
	ID           string    `json:"id"`
	URL          string    `json:"url"`
	Title        string    `json:"title"`
	ChannelID    *string   `json:"channelId"`
	ChannelTitle *string   `json:"channelTitle"`
	ThumbnailURL *string   `json:"thumbnailUrl"`
	DurationSec  *float64  `json:"durationSec"`
	ScannedAt    time.Time `json:"scannedAt"`
}
type CloneProgress struct {
	UnitsDone  int64  `json:"unitsDone"`
	UnitsTotal *int64 `json:"unitsTotal"`
	Phase      string `json:"phase"`
}
type CloneArtifact struct {
	ID          string    `json:"id"`
	Path        string    `json:"path"`
	SHA256      string    `json:"sha256"`
	Bytes       int64     `json:"bytes"`
	Step        CloneStep `json:"step"`
	ContentType string    `json:"contentType"`
	Filename    string    `json:"filename"`
}
type CloneStepOutput struct {
	Artifacts []CloneArtifact `json:"artifacts"`
	// Step-specific structured manifests have intentionally heterogeneous values.
	Data map[string]any `json:"data,omitempty"`
}
type CloneJob struct {
	ID                string              `json:"id"`
	ChannelID         string              `json:"channelId"`
	SourceKey         string              `json:"sourceKey"`
	Source            CloneSourceSnapshot `json:"source"`
	TemplateID        string              `json:"templateId"`
	TemplateSnapshot  json.RawMessage     `json:"templateSnapshot"`
	ModelRefs         json.RawMessage     `json:"modelRefs"`
	NoImages          bool                `json:"noImages"`
	Generation        int                 `json:"generation"`
	SupersededAt      *time.Time          `json:"supersededAt"`
	PausedAt          *time.Time          `json:"pausedAt"`
	CancelRequestedAt *time.Time          `json:"cancelRequestedAt"`
	Priority          ClonePriority       `json:"priority"`
	Origin            string              `json:"origin"`
	Status            CloneStatus         `json:"status"`
	StageGroup        string              `json:"stageGroup"`
	ActiveStep        CloneStep           `json:"activeStep"`
	Fence             int64               `json:"-"`
	Version           int64               `json:"version"`
	PostedAt          *time.Time          `json:"postedAt"`
	CreatedAt         time.Time           `json:"createdAt"`
	UpdatedAt         time.Time           `json:"updatedAt"`
}
type CloneStepRecord struct {
	JobID        string           `json:"jobId"`
	Step         CloneStep        `json:"step"`
	Status       CloneStatus      `json:"status"`
	Attempt      int              `json:"attempt"`
	Progress     CloneProgress    `json:"progress"`
	Checkpoint   json.RawMessage  `json:"checkpoint"`
	Output       *CloneStepOutput `json:"output"`
	Error        *CloneError      `json:"error"`
	Fence        int64            `json:"-"`
	ProcessGroup *int             `json:"-"`
	StartedAt    *time.Time       `json:"startedAt"`
	FinishedAt   *time.Time       `json:"finishedAt"`
	UpdatedAt    time.Time        `json:"updatedAt"`
}
type CloneQueueItem struct {
	ID            string              `json:"id"`
	ChannelID     string              `json:"channelId"`
	SourceVideoID string              `json:"sourceVideoId"`
	SourceKey     string              `json:"sourceKey"`
	Source        CloneSourceSnapshot `json:"source"`
	Priority      ClonePriority       `json:"priority"`
	Origin        string              `json:"origin"`
	Position      float64             `json:"position"`
	State         string              `json:"state"`
	NoImages      bool                `json:"noImages"`
	JobID         *string             `json:"jobId"`
	Reason        *string             `json:"reason"`
	CreatedAt     time.Time           `json:"createdAt"`
	StartedAt     *time.Time          `json:"startedAt"`
}
type CloneAgentView struct {
	ID     string            `json:"id"`
	State  string            `json:"state"`
	Now    *string           `json:"now"`
	Metric *CloneAgentMetric `json:"metric"`
}
type CloneAgentMetric struct {
	Label string `json:"label"`
	Value string `json:"value"`
}
type CloneJobView struct {
	CloneJob
	Subline   string            `json:"subline"`
	AgentView []CloneAgentView  `json:"agentView"`
	Steps     []CloneStepRecord `json:"steps"`
}

const (
	CloneEventCreated  = "clone.job.created"
	CloneEventUpdated  = "clone.job.updated"
	CloneEventDeleted  = "clone.job.deleted"
	CloneEventProgress = "clone.step.progress"
)
