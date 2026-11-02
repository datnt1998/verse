package audio

import (
	"encoding/json"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/datnt1998/verse/apps/noverse/internal/domain"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
)

// The mobile queue contract (GenPickEmpty) reads Job.NeedsScript/Step: a
// preparation row must report the compose step with no invented segment/ETA
// numbers, and a prepared render row must report render with the real count.
func TestPGJobViewPreparationContract(t *testing.T) {
	prepInput, _ := json.Marshal(pgTTSInput{
		Kind: JobKindCreate, Chapter: 3, Volume: 1, Quality: "standard",
		Segments: []pgTTSSegment{}, NeedsScript: true,
	})
	prep := pgJobView(postgres.Job{Kind: postgres.JobTTS, State: postgres.JobQueued, Input: prepInput})
	if !prep.NeedsScript || prep.Step != "compose" {
		t.Fatalf("preparation row = %+v, want needsScript + compose", prep)
	}
	if prep.Segments != 0 || prep.SegmentsDone != 0 || prep.Percent != 0 {
		t.Fatalf("preparation row invented progress: %+v", prep)
	}

	renderInput, _ := json.Marshal(pgTTSInput{
		Kind: JobKindCreate, Chapter: 3, Volume: 1, Quality: "standard",
		Segments: []pgTTSSegment{{ID: "s0001", Text: "Một."}, {ID: "s0002", Text: "Hai."}},
	})
	render := pgJobView(postgres.Job{Kind: postgres.JobTTS, State: postgres.JobQueued, Input: renderInput})
	if render.NeedsScript || render.Step != "render" || render.Segments != 2 {
		t.Fatalf("render row = %+v, want render with 2 segments", render)
	}

	done := pgJobView(postgres.Job{Kind: postgres.JobTTS, State: postgres.JobCompleted, Input: renderInput})
	if done.State != JobDone || done.Step != "" || done.Percent != 100 {
		t.Fatalf("completed row = %+v", done)
	}
}

// A prepared script/render input binds to the exact accepted chapter revision.
// The queue may read the prose before the revision pointer moves, and the
// pointer may move while the provider composes; publishing either way would
// stamp a revision number on different text, which the native ACK drops (and
// the locked script would look current). The fence must refuse both.
func TestPreparationSourceFence(t *testing.T) {
	id := pgtype.UUID{Bytes: [16]byte{1}, Valid: true}
	other := pgtype.UUID{Bytes: [16]byte{2}, Valid: true}
	text := "Chương một.\n\nNội dung."
	sha := domain.ChapterContentSHA256(text)
	ref := postgres.ChapterRef{ID: id, Number: 1, CurrentRevision: 4, Committed: true}

	cases := []struct {
		name   string
		ref    postgres.ChapterRef
		source postgres.ChapterSource
		ok     bool
		sha    string
		want   bool
	}{
		{"same source", ref, postgres.ChapterSource{ID: id, Revision: 4, SHA256: sha}, true, sha, true},
		{"same prose, different line endings", ref,
			postgres.ChapterSource{ID: id, Revision: 4, SHA256: sha}, true,
			domain.ChapterContentSHA256("Chương một.\r\n\r\nNội dung."), true},
		{"revision advanced during compose", ref,
			postgres.ChapterSource{ID: id, Revision: 5, SHA256: domain.ChapterContentSHA256("Bản mới.")},
			true, sha, false},
		{"prose read before the revision moved",
			postgres.ChapterRef{ID: id, Number: 1, CurrentRevision: 5, Committed: true},
			postgres.ChapterSource{ID: id, Revision: 5, SHA256: domain.ChapterContentSHA256("Bản mới.")},
			true, sha, false},
		{"revision pointer without its row", ref, postgres.ChapterSource{}, false, sha, false},
		{"chapter re-created under another id", ref,
			postgres.ChapterSource{ID: other, Revision: 4, SHA256: sha}, true, sha, false},
		{"uncommitted ref", postgres.ChapterRef{Number: 1},
			postgres.ChapterSource{ID: id, Revision: 4, SHA256: sha}, true, sha, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := preparationSourceMatches(tc.ref, tc.source, tc.ok, tc.sha); got != tc.want {
				t.Fatalf("preparationSourceMatches() = %v, want %v", got, tc.want)
			}
		})
	}
}
