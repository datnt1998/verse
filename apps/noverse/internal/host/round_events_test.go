package host

import (
 "encoding/json"
 "testing"

 "github.com/datnt1998/verse/apps/noverse/internal/store"
)

func TestObserverDispatchEventsAreDurableBeforeNotification(t *testing.T) {
 var seen []Event
 o:=testObserver(&seen); o.store=store.NewStore(t.TempDir())
 o.emitEv=func(ev Event) {
  queue,err:=o.store.Runtime.LoadQueue(); if err!=nil || len(queue)==0 { t.Fatalf("event %s announced before persistence: %v",ev.ID,err) }
  raw,err:=json.Marshal(queue[len(queue)-1].Payload); if err!=nil { t.Fatal(err) }; var saved Event; if err:=json.Unmarshal(raw,&saved); err!=nil { t.Fatal(err) }
  if saved.ID!=ev.ID || saved.FinishedAt.IsZero()!=ev.FinishedAt.IsZero() { t.Fatalf("event announced before durable update: %+v vs %+v",ev,saved) }
 }
 o.dispatchStart("writer","Viết","")
 o.dispatchFinish("writer",nil)
}
