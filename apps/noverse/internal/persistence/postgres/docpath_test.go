package postgres_test

import (
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
)

// TestDocumentPathIsInverseOfResolve pins that every concrete file path the
// forward document mapping accepts maps back to the exact same path.
func TestDocumentPathIsInverseOfResolve(t *testing.T) {
	paths := []string{
		// every concrete state/outline/stream path the parser knows
		"meta/book.json", "meta/progress.json", "meta/run.json", "meta/user_rules.json",
		"meta/user_rules_pin.json", "meta/user_rules_cache.json", "meta/user_rules_runtime.json",
		"meta/lesson_rules.json", "meta/ledger.json", "meta/pending_commit.json",
		"meta/pending_revision.json", "meta/last_commit.json", "meta/last_review.json",
		"meta/usage.json", "meta/simulation_profile.json", "meta/project_format.json",
		"meta/author_revision_style.json", "meta/foundation_audit.json", "meta/style_rules.json",
		"meta/cast_ledger.json", "meta/outline_feedback_resolution.json",
		"world_rules.json", "foreshadow_ledger.json", "relationship_state.json",
		"address_terms.json", "characters.json", "timeline.json", "meta/state_changes.json",
		"outline.json", "layered_outline.json", "meta/compass.json", "meta/story_retention.json",
		"premise.md",
		"meta/decisions.jsonl", "meta/rule_violations.jsonl", "timeline.jsonl",
		"meta/state_changes.jsonl", "relationship_log.jsonl", "meta/rewrites.jsonl",
		"meta/outline_feedback.jsonl", "meta/runtime/queue.jsonl",
		// keyed streams and sessions
		"meta/checkpoints.jsonl", "meta/sessions/cocreate.jsonl",
		"meta/sessions/agents/writer-ch01.jsonl", "meta/sessions/agents/architect_long-001.jsonl",
		"meta/sessions/agents/writer-ch/01.jsonl",
		"meta/runtime/tasks/build-1.log", "meta/runtime/tasks/a/b-2.log",
		// chapter-scoped and artifacts
		"chapters/01.md", "chapters/103.md",
		"drafts/01.draft.md", "drafts/02.plan.json",
		"summaries/01.json", "summaries/arc-1.json",
		"meta/snapshots/v01a02.json",
		"meta/chapter_records/000001.json",
		// typed review rows rendered through their file-backend paths
		"reviews/01.json", "reviews/05-global.json",
		// markdown views
		"book.md", "characters.md", "world_rules.md", "timeline.md",
		"foreshadow_ledger.md", "relationship_state.md", "outline.md",
		"layered_outline.md", "nao/index.md", "nao/nhan-vat/thu-thu.md",
	}
	for _, p := range paths {
		ref, err := postgres.ResolveDocument(p)
		if err != nil {
			t.Fatalf("ResolveDocument(%q): %v", p, err)
		}
		back, ok := postgres.DocumentPath(ref)
		if !ok {
			t.Fatalf("DocumentPath(ResolveDocument(%q)): no path for %+v", p, ref)
		}
		if back != p {
			t.Fatalf("DocumentPath(ResolveDocument(%q)) = %q", p, back)
		}
	}
}

// TestDocumentPathRejectsUnproducibleIdentities pins that the inverse refuses
// identities resolveDoc could never produce, instead of writing a wrong file.
func TestDocumentPathRejectsUnproducibleIdentities(t *testing.T) {
	cases := []postgres.DocumentRef{
		{Class: postgres.DocClassState, Kind: "no_such_kind"},
		{Class: postgres.DocClassArtifact, Kind: "snapshot", Key: "vbad"},
		{Class: postgres.DocClassArtifact, Kind: "snapshot", Key: "1a2"},
		{Class: postgres.DocClassArtifact, Kind: "chapter_plan", Key: "002"},
		{Class: postgres.DocClassArtifact, Kind: "chapter_text", Key: "abc"},
		{Class: postgres.DocClassArtifact, Kind: "markdown_view", Key: "notes.txt"},
		{Class: postgres.DocClassArtifact, Kind: "markdown_view", Key: "meta/format.json"},
		{Class: postgres.DocClassAgentSession, Agent: "writer", Key: "ch-01"},
		{Class: postgres.DocClassAgentSession, Agent: "writer", Key: ""},
		{Class: postgres.DocClassAgentSession, Agent: "", Key: "ch01"},
		{Class: postgres.DocClassStream, Kind: "decisions", Key: "extra"},
		{Class: postgres.DocClassChapterDraft, Chapter: 0},
		{Class: postgres.DocClassChapterRecord, Chapter: -1},
		{Class: postgres.DocClassReview, Chapter: 0, Key: "05"},
		{Class: postgres.DocClassReview, Chapter: 5, Key: ""},
		{Class: postgres.DocClassReview, Chapter: 5, Key: "global"},
		{Class: postgres.DocClassReview, Chapter: 5, Key: "05-global-r1"},
	}
	for _, ref := range cases {
		if rel, ok := postgres.DocumentPath(ref); ok {
			t.Fatalf("DocumentPath(%+v) = %q, expected rejection", ref, rel)
		}
	}
}
