package llm

import (
	"context"
	"testing"
	"time"

	litellm "github.com/datnt1998/noverse/internal/llm/wire"
)

type closeTrackedStream struct {
	litellm.Stream
	closed chan struct{}
}

func (s *closeTrackedStream) Close() error { close(s.closed); return s.Stream.Close() }

type cancelReplayProvider struct {
	captureProvider
	closed chan struct{}
}

func (p *cancelReplayProvider) Stream(context.Context, *litellm.Request) (litellm.Stream, error) {
	events := make([]litellm.Event, 1000)
	for i := range events {
		events[i] = litellm.ContentDelta{Text: "x"}
	}
	return &closeTrackedStream{Stream: &staticStream{events: events}, closed: p.closed}, nil
}
func TestModelCancelUnblocksFullEventBuffer(t *testing.T) {
	p := &cancelReplayProvider{closed: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m := NewClientModel("m", mustClient(t, p))
	events, err := m.GenerateStream(ctx, []Message{UserMsg("run")}, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-events
	cancel()
	select {
	case <-p.closed:
	case <-time.After(time.Second):
		t.Error("provider stream stayed open after cancel with a full event buffer")
	}
	for range events {
	}
}
func TestModelStreamSnapshotsDoNotMutate(t *testing.T) {
	p := &captureStreamProvider{events: []litellm.Event{litellm.ContentDelta{Text: "a"}, litellm.ContentDelta{Text: "b"}, litellm.DoneEvent{Provider: "capture", Model: "m", FinishReason: litellm.FinishReasonStop}}}
	m := NewClientModel("m", mustClient(t, p))
	events, err := m.GenerateStream(context.Background(), []Message{UserMsg("run")}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var first *Message
	for ev := range events {
		if first == nil && ev.Type == StreamEventTextDelta {
			snapshot := ev.Message
			first = &snapshot
		}
	}
	if first == nil || first.TextContent() != "a" {
		t.Fatalf("first snapshot mutated: %+v", first)
	}
}
