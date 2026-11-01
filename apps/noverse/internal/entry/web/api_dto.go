//go:generate go run ./gentypes

package web

import (
	"reflect"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/ledger"
	"github.com/datnt1998/verse/apps/noverse/internal/lessons"
	"github.com/datnt1998/verse/apps/noverse/internal/rules"
	"github.com/datnt1998/verse/apps/noverse/internal/voice"
)

// api_dto.go is the input of the TypeScript generator (gentypes):
// web/src/api/types.gen.ts is derived from the Go types named here, never edited
// by hand.
//
// Every APIDTO entry is the type its handler marshals, so the generated file is
// checked against the real write side. The mock/<file>.json goldens replay the
// same contract end to end.

// APIDTO is a response root rendered into web/src/api/types.gen.ts. The order of
// this slice is the order of the generated declarations.
type APIDTO struct {
	// TS is the exported TypeScript name of the response.
	TS    string
	Value any
}

// APIDTOs lists every response the frontend types come from. A response that is
// not listed here does not exist for the generator: add it when the FE starts
// reading it.
var APIDTOs = []APIDTO{
	{TS: "ReviewEntry", Value: reviewEntryAPI{}},
	{TS: "Quality", Value: qualityResponse{}},
	{TS: "LedgerCheckResult", Value: ledger.Result{}},
	{TS: "LedgerSuggestion", Value: ledger.Proposal{}},
	{TS: "ProjectLedger", Value: projectLedgerResponse{}},
	{TS: "ProjectUniverse", Value: universeResponse{}},
	{TS: "LessonsResponse", Value: lessonsResponse{}},
	{TS: "ViolationsResponse", Value: violationsResponse{}},
	{TS: "RuleViolationsResponse", Value: ruleViolationsResponse{}},
	{TS: "TtsStatus", Value: ttsStatusResponse{}},
	{TS: "TtsStatusError", Value: ttsStatusError{}},
	{TS: "TtsMachine", Value: ttsMachine{}},
	{TS: "TtsCapabilities", Value: ttsCapabilities{}},
}

// APITSTypeNames pins the TypeScript name of every nested Go struct. The
// generator fails on a reachable type that is missing here, so a new DTO cannot
// be published under an accidental name.
var APITSTypeNames = map[reflect.Type]string{
	reflect.TypeOf(domain.GoalCheck{}):        "GoalCheck",
	reflect.TypeOf(domain.RecheckResult{}):    "RecheckResult",
	reflect.TypeOf(reviewItemAPI{}):           "ReviewItem",
	reflect.TypeOf(reviewDecisionAPI{}):       "ReviewDecision",
	reflect.TypeOf(reviewPackAPI{}):           "ReviewPack",
	reflect.TypeOf(reviewRoundAPI{}):          "ReviewRound",
	reflect.TypeOf(domain.ConsistencyIssue{}): "ConsistencyIssue",
	reflect.TypeOf(domain.DimensionScore{}):   "DimensionScore",

	reflect.TypeOf(qualitySectionAPI{}): "QualitySection",
	reflect.TypeOf(qualityRewriteAPI{}): "QualityRewrite",
	reflect.TypeOf(qualityChapterAPI{}): "QualityChapter",
	reflect.TypeOf(qualityCellAPI{}):    "QualityCell",
	reflect.TypeOf(qualityIssueAPI{}):   "QualityIssue",

	reflect.TypeOf(ledger.Axis{}):     "LedgerAxis",
	reflect.TypeOf(ledger.Story{}):    "LedgerStory",
	reflect.TypeOf(ledger.Check{}):    "LedgerCheck",
	reflect.TypeOf(ledger.Cell{}):     "LedgerCell",
	reflect.TypeOf(ledgerCandidate{}): "LedgerCandidate",

	reflect.TypeOf(universeEntity{}):    "ProjectUniverseEntity",
	reflect.TypeOf(universeStoryRef{}):  "UniverseStoryRef",
	reflect.TypeOf(universeCounts{}):    "UniverseCounts",
	reflect.TypeOf(universeCheck{}):     "UniverseCheckItem",
	reflect.TypeOf(universeKeptCheck{}): "UniverseKeptItem",

	reflect.TypeOf(lessons.Lesson{}):   "Lesson",
	reflect.TypeOf(lessons.Evidence{}): "LessonEvidence",
	reflect.TypeOf(lessons.Row{}):      "ViolationRow",
	reflect.TypeOf(lessonPackAPI{}):    "LessonPack",
	reflect.TypeOf(lessonStatsAPI{}):   "LessonStats",

	reflect.TypeOf(ruleViolationChapter{}): "RuleViolationChapter",
	reflect.TypeOf(rules.Violation{}):      "RuleViolation",
}

// APIEnumField names one Go string field that only accepts a closed value set.
// Owner is the struct the field belongs to; fields inherited from an
// embedded struct keep their declaring struct as Owner.
type APIEnumField struct {
	Owner reflect.Type
	Name  string
}

// APIEnum turns every listed string field into a closed TypeScript union. Values
// are the JSON strings the backend writes — where Go has constants they are used
// here so a renamed constant breaks the build, not the frontend.
type APIEnum struct {
	TS     string
	Values []string
	Fields []APIEnumField
}

// APIEnums is the complete enum table. "Normalised by the backend" fields are
// listed.
var APIEnums = []APIEnum{
	{
		// Các trạng thái của thẻ Máy đọc (10-audio.md §8.1): đã thu gọn về sáu
		// từ — `offline` là máy chủ suy ra, `provisioning` gồm cả engine
		// `installing`/`stopped` của bản cá nhân.
		TS: "TtsStatusState",
		Values: []string{
			voice.ProvisionStateMissing, voice.ProvisionStateProvisioning,
			voice.ProvisionStateStarting, voice.ProvisionStateReady,
			voice.ProvisionStateError, voice.ProvisionStateOffline,
		},
		Fields: []APIEnumField{{reflect.TypeOf(ttsStatusResponse{}), "State"}},
	},
	{
		TS:     "ReviewLevel",
		Values: []string{reviewLevelError, reviewLevelWarning},
		Fields: []APIEnumField{
			{reflect.TypeOf(reviewItemAPI{}), "Level"},
			{reflect.TypeOf(lessons.Row{}), "Level"},
			{reflect.TypeOf(rules.Violation{}), "Severity"},
		},
	},
	{
		TS:     "ReviewScope",
		Values: []string{string(domain.ScopeChapter), string(domain.ScopeArc), string(domain.ScopeGlobal)},
		Fields: []APIEnumField{{reflect.TypeOf(domain.ReviewEntry{}), "Scope"}},
	},
	{
		// The engine's full verdict vocabulary, legacy values included: host.verdictLabel
		// (internal/host/host.go) still labels "revise"/"reject" from older stored reviews.
		TS:     "ReviewVerdict",
		Values: []string{"accept", "polish", "rewrite", "revise", "reject"},
		Fields: []APIEnumField{
			{reflect.TypeOf(domain.ReviewEntry{}), "Verdict"},
			{reflect.TypeOf(reviewRoundAPI{}), "Verdict"},
		},
	},
	{
		TS:     "ContractStatus",
		Values: []string{"met", "partial", "missed"},
		Fields: []APIEnumField{{reflect.TypeOf(domain.ReviewEntry{}), "ContractStatus"}},
	},
	{
		TS:     "GoalCheckStatus",
		Values: []string{"pass", "fail", "unknown"},
		Fields: []APIEnumField{{reflect.TypeOf(domain.GoalCheck{}), "Status"}},
	},
	{
		TS:     "RecheckStatus",
		Values: []string{"fixed", "unfixed", "unknown"},
		Fields: []APIEnumField{{reflect.TypeOf(domain.RecheckResult{}), "Status"}},
	},
	{
		TS:     "IssueSeverity",
		Values: []string{"critical", "error", "warning"},
		Fields: []APIEnumField{{reflect.TypeOf(domain.ConsistencyIssue{}), "Severity"}},
	},
	{
		TS:     "ReviewDecisionState",
		Values: []string{"kept"},
		Fields: []APIEnumField{{reflect.TypeOf(reviewDecisionAPI{}), "State"}},
	},
	{
		TS:     "QualityChapterStatus",
		Values: []string{"approved", "rewritten", "rewriting", "previous"},
		Fields: []APIEnumField{{reflect.TypeOf(qualityChapterAPI{}), "Status"}},
	},
	{
		TS:     "QualityRewriteCause",
		Values: []string{"review", "user"},
		Fields: []APIEnumField{{reflect.TypeOf(qualityRewriteAPI{}), "Cause"}},
	},
	{
		TS:     "QualityIssueState",
		Values: []string{"open", "fixed", "kept"},
		Fields: []APIEnumField{{reflect.TypeOf(qualityIssueAPI{}), "State"}},
	},
	{
		TS:     "LedgerCheckLevel",
		Values: []string{"error", "warning", "ok"},
		Fields: []APIEnumField{{reflect.TypeOf(ledger.Check{}), "Level"}},
	},
	{
		TS:     "LedgerCellMark",
		Values: []string{"dup", "overused"},
		Fields: []APIEnumField{{reflect.TypeOf(ledger.Cell{}), "Mark"}},
	},
	{
		TS:     "UniverseEntityType",
		Values: []string{"char", "place", "obj", "ev", "fs", "rule"},
		Fields: []APIEnumField{{reflect.TypeOf(universeEntity{}), "Type"}},
	},
	{
		// Gói S: loại vai của nhân vật (04 › Avatar nhân vật › Màu theo loại vai). Dùng
		// chung cho vũ trụ chung; các DTO nhân vật khác là kiểu tay ở web/src/api/types.ts.
		TS: "CharacterKind",
		Values: []string{
			string(domain.CharacterKindLead), string(domain.CharacterKindAntagonist),
			string(domain.CharacterKindAlly), string(domain.CharacterKindSupport),
		},
		Fields: []APIEnumField{{reflect.TypeOf(universeEntity{}), "Kind"}},
	},
	{
		TS:     "LessonState",
		Values: []string{"pending", "kept", "dropped"},
		Fields: []APIEnumField{{reflect.TypeOf(lessons.Lesson{}), "State"}},
	},
	{
		TS:     "LessonScope",
		Values: []string{"pack", "all"},
		Fields: []APIEnumField{{reflect.TypeOf(lessons.Lesson{}), "Scope"}},
	},
	{
		TS:     "LessonOrigin",
		Values: []string{"review", "user"},
		Fields: []APIEnumField{{reflect.TypeOf(lessons.Lesson{}), "Origin"}},
	},
	{
		TS:     "ViolationAlertScope",
		Values: []string{"all"},
		Fields: []APIEnumField{{reflect.TypeOf(lessons.Row{}), "AlertScope"}},
	},
}

// APIFieldType replaces the JSON type inferred from Go for one field. Used where
// the Go type is wider than the wire (`any`) or where nil is a meaningful value
// the field tag does not express.
type APIFieldType struct {
	Owner reflect.Type
	Name  string
	TS    string
}

var APIFieldTypes = []APIFieldType{
	// Violation.Limit/Actual are ints for fatigue_words (the only rule the FE
	// reads) and nil for the forbidden_* rules.
	{reflect.TypeOf(rules.Violation{}), "Limit", "number"},
	{reflect.TypeOf(rules.Violation{}), "Actual", "number"},
	// checks: null = chưa có vòng chấm nào chạy lược đồ có `rule`; [] = đã chấm và sạch.
	{reflect.TypeOf(universeResponse{}), "Checks", "UniverseCheckItem[] | null"},
	// counts is keyed by lesson state, not by an arbitrary string.
	{reflect.TypeOf(lessonsResponse{}), "Counts", "Record<LessonState, number>"},
}

// projectLedgerResponse is the project ledger screen payload
// (GET /api/projects/{p}/ledger).
type projectLedgerResponse struct {
	Axes              []ledger.Axis    `json:"axes"`
	Stories           []ledger.Story   `json:"stories"`
	ChannelMedianDay7 *int             `json:"channelMedianDay7"` // luôn null: chưa có nguồn trung vị
	Candidate         *ledgerCandidate `json:"candidate"`
}

// LessonsResponse is GET /api/system/lessons.
type lessonsResponse struct {
	Lessons []lessons.Lesson `json:"lessons"`
	Counts  map[string]int   `json:"counts"`
	Stats   lessonStatsAPI   `json:"stats"`
	Packs   []lessonPackAPI  `json:"packs"`
}

// ViolationsResponse is GET /api/system/violations.
type violationsResponse struct {
	Rows     []lessons.Row `json:"rows"`
	Preamble string        `json:"preamble"`
	PackName string        `json:"packName"`
	Stories  int           `json:"stories"`
}

// ruleViolationChapter is one chapter section of GET S/rules/violations.
type ruleViolationChapter struct {
	Chapter    int               `json:"chapter"`
	At         string            `json:"at"`
	Violations []rules.Violation `json:"violations"`
}

// RuleViolationsResponse is GET S/rules/violations.
type ruleViolationsResponse struct {
	Chapters []ruleViolationChapter `json:"chapters"`
}
