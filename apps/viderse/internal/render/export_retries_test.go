package render

import (
	"context"
	"net/url"
	"strings"
	"testing"

	"github.com/datnt1998/verse/apps/viderse/internal/tools"
)

func TestSupervisorRetriesKeepPublishedHTTPIntent(t *testing.T) {
	s, runner, _ := fakeSupervisor(t, "exit")
	original := runner.run
	ids := []string{}
	tokens := []string{}
	runner.run = func(ctx context.Context, n int, args []string, spec tools.Spec) error {
		u, err := url.Parse(args[len(args)-1])
		if err != nil {
			return err
		}
		fragment := strings.SplitN(u.Fragment, "?", 2)
		ids = append(ids, strings.TrimPrefix(fragment[0], "/render/"))
		query, _ := url.ParseQuery(fragment[1])
		tokens = append(tokens, query.Get("token"))
		return original(ctx, n, args, spec)
	}
	_, err := s.LaunchExport(context.Background(), "project", "job", nil)
	if err == nil || len(ids) != 3 {
		t.Fatalf("unexpected restart outcome: starts%d err%v", len(ids), err)
	}
	for _, id := range ids {
		if id != ids[0] {
			t.Fatal("HTTP stateUrl was stranded on an abandoned restart request")
		}
	}
	if tokens[0] == tokens[1] || tokens[1] == tokens[2] || tokens[0] == tokens[2] {
		t.Fatal("restart reused an expired attempt credential")
	}
	requests, err := s.Store.ListExports()
	if err != nil {
		t.Fatal(err)
	}
	if len(requests) != 1 || requests[0].ID != ids[0] {
		t.Fatal("one HTTP export intent created multiple tracking records")
	}
}
