package audio

import (
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestHostedCastPreservesEngineVoiceWithoutProjectForeignKey(t *testing.T) {
	projectVoiceID := pgtype.UUID{Bytes: [16]byte{1}, Valid: true}
	rows := []postgres.CastRole{
		{RoleKey: "dan", Payload: []byte(`{"name":"Dẫn truyện","voiceId":"Adam"}`)},
		{RoleKey: "nv", VoiceID: projectVoiceID, Payload: []byte(`{"name":"Nhân vật","voiceId":"old-name"}`)},
		{RoleKey: "unset", Payload: []byte(`{"name":"Chưa chọn","voiceId":null}`)},
	}
	cast, _, err := castFromRows(rows, []postgres.Voice{{ID: projectVoiceID, LegacyID: "project-sample"}})
	if err != nil {
		t.Fatal(err)
	}
	got := make(map[string]*string, len(cast.Speakers))
	for _, speaker := range cast.Speakers {
		got[speaker.ID] = speaker.VoiceID
	}
	if got["dan"] == nil || *got["dan"] != "Adam" {
		t.Fatalf("engine preset lost on reload: %v", got["dan"])
	}
	if got["nv"] == nil || *got["nv"] != "project-sample" {
		t.Fatalf("project voice foreign key must override stale payload: %v", got["nv"])
	}
	if got["unset"] != nil {
		t.Fatalf("unselected voice acquired an identity: %v", got["unset"])
	}
}
