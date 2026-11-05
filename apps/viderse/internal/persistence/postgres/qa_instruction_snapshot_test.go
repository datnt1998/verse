package postgres_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/channels"
	"github.com/datnt1998/verse/apps/viderse/internal/domain"
	"github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
)

func TestQAContractAdmittedJobNoteSurvivesChannelChangeAndDeletion(t *testing.T) {
	pool := provision(t)
	ctx := context.Background()
	seedCloneChannel(t, pool)
	channelStore := postgres.NewChannelStore(pool)
	patchQA := func(value string) {
		t.Helper()
		ok, err := channelStore.PatchChannel(ctx, chOne, channels.Patch{
			AgentInstructions: channels.AgentInstructionPatch{Set: true, Key: "qa", Value: value},
		})
		if err != nil || !ok {
			t.Fatalf("patch QA: found=%v err=%v", ok, err)
		}
	}
	initial := domain.AgentInstructions{"qa": "Giữ đúng nhân vật trong ảnh"}
	patchQA(initial["qa"])
	jobs := postgres.NewCloneStore(pool)
	old, err := jobs.StartJob(ctx, admissionInput("qa-before-change", fixtureTemplate, false, 1, `{}`))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(old.Job.Instructions, initial) {
		t.Fatalf("initial snapshot=%+v", old.Job.Instructions)
	}
	execChannelSQL(t, pool, `UPDATE clone_jobs SET status='running' WHERE id=$1`, old.Job.ID)
	changed := domain.AgentInstructions{"qa": "Kiểm tra nhạc nền và giọng đọc"}
	patchQA(changed["qa"])
	next, err := jobs.StartJob(ctx, admissionInput("qa-after-change", fixtureTemplate, false, 1, `{}`))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(next.Job.Instructions, changed) {
		t.Fatalf("new admission ignored QA change: %+v", next.Job.Instructions)
	}
	patchQA("")
	channel, found, err := channelStore.GetChannel(ctx, chOne)
	if err != nil || !found || channel.AgentInstructions.InstructionFor("qa") != "" {
		t.Fatalf("QA deletion not persisted: found=%v err=%v", found, err)
	}
	// Fresh repositories force database readback for both running and queued jobs.
	for _, want := range []struct {
		id           string
		instructions domain.AgentInstructions
	}{{old.Job.ID, initial}, {next.Job.ID, changed}} {
		persisted, err := postgres.NewCloneStore(pool).GetJob(ctx, want.id)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(persisted.Instructions, want.instructions) {
			t.Fatalf("job=%s snapshot changed: got=%+v want=%+v", want.id, persisted.Instructions, want.instructions)
		}
	}
}
