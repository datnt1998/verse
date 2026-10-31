package web

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/datnt1998/verse/apps/noverse/internal/errs"
	"github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
)

func TestHostedConfigRepositoryCASAndMembership(t *testing.T) {
	f := provisionHostedGateway(t, "config-cas", "story")
	docs := f.repo.ConfigDocuments()
	ctx := context.Background()
	actor := &postgres.Actor{Username: f.owner.Username}
	for _, story := range []string{"", f.story} {
		t.Run("scope-"+story, func(t *testing.T) {
			initial, err := docs.Version(ctx, actor, f.project, story)
			if err != nil {
				t.Fatal(err)
			}
			start := make(chan struct{})
			results := make(chan error, 2)
			var wg sync.WaitGroup
			for _, body := range []string{`{"style":"xianxia"}`, `{"style":"nu-tan-bao-thu"}`} {
				wg.Add(1)
				go func(body string) {
					defer wg.Done()
					<-start
					results <- docs.Put(ctx, actor, f.project, story, []byte(body), initial)
				}(body)
			}
			close(start)
			wg.Wait()
			close(results)
			wins, conflicts := 0, 0
			for err := range results {
				if err == nil {
					wins++
				} else if errors.Is(err, errs.ErrStorageConflict) {
					conflicts++
				} else {
					t.Fatal(err)
				}
			}
			if wins != 1 || conflicts != 1 {
				t.Fatalf("CAS winners=%d conflicts=%d", wins, conflicts)
			}
			outsider := &postgres.Actor{Username: f.outsider.Username}
			if _, _, err := docs.Get(ctx, outsider, f.project, story); !errors.Is(err, errs.ErrStorageNotFound) {
				t.Fatalf("outsider read=%v", err)
			}
			if err := docs.Put(ctx, outsider, f.project, story, []byte(`{}`), ""); !errors.Is(err, errs.ErrStorageNotFound) {
				t.Fatalf("outsider write=%v", err)
			}
		})
	}
}
