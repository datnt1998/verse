package postgres

import (
 "encoding/json"
 "testing"
)
func TestSafetyRedrawNeedsRealCheckpointTransition(t *testing.T) {
 old:=json.RawMessage(`{"items":[{"id":"scene-1","status":"running","inFlight":{"generationId":"g1"}}]}`)
 next:=json.RawMessage(`{"items":[{"id":"scene-1","status":"running","modest":true,"inFlight":{"generationId":"g1"}}]}`)
 if got:=imageRedrawTransitions(old,next);len(got)!=1 || got[0]!="scene-1" {t.Fatalf("got %v",got)}
 if got:=imageRedrawTransitions(next,next);len(got)!=0 {t.Fatalf("duplicate %v",got)}
 if got:=imageRedrawTransitions(nil,next);len(got)!=0 {t.Fatalf("initial snapshot %v",got)}
}
