package web

import (
 "testing"
 "time"

 "github.com/datnt1998/verse/apps/noverse/internal/domain"
 "github.com/datnt1998/verse/apps/noverse/internal/host"
 "github.com/datnt1998/verse/apps/noverse/internal/store"
)

func TestLiveSnapshotReplayDeduplicatesFinishedOperation(t *testing.T) {
 runtime:=store.NewStore(t.TempDir()).Runtime
 at:=time.Now()
 for _,ev:=range []host.Event{{ID:"tool",Time:at,Category:"TOOL",Agent:"writer",Tool:"draft_chapter",Status:"running"},{ID:"tool",Time:at,FinishedAt:at.Add(time.Second),Category:"TOOL",Agent:"writer",Tool:"draft_chapter",Status:"ok"}} {
  if _,err:=runtime.AppendQueue(domain.RuntimeQueueItem{Payload:ev}); err!=nil { t.Fatal(err) }
 }
 if err:=runtime.AppendTaskLog(liveTranscriptLog,domain.RuntimeTaskLogEntry{Summary:"A"}); err!=nil { t.Fatal(err) }
 b:=newBroadcaster(); if err:=restoreStreamHistory(&historyFakeHost{fakeHost:newFakeHost(),runtime:runtime},b); err!=nil { t.Fatal(err) }
 replay,stream,ch:=b.subscribe(); defer b.unsubscribe(ch)
 if len(replay)!=1 || replay[0].data.(host.Event).Status!="ok" { t.Fatalf("duplicate operation: %+v",replay) }
 b.emit(message{kind:"delta",data:"B"}); live:=<-ch
 if live.kind!="delta" || stream+live.data.(string)!="AB" { t.Fatalf("replayed delta: snapshot %q live %+v",stream,live) }
 runtime2:=store.NewStore(t.TempDir()).Runtime
 start:=host.Event{ID:"active",Time:at,Category:"TOOL",Agent:"writer",Tool:"draft_chapter",Status:"running"}
 if _,err:=runtime2.AppendQueue(domain.RuntimeQueueItem{Payload:start}); err!=nil { t.Fatal(err) }
 if err:=runtime2.AppendTaskLog(liveTranscriptLog,domain.RuntimeTaskLogEntry{Summary:"A"}); err!=nil { t.Fatal(err) }
 b2:=newBroadcaster(); if err:=restoreStreamHistory(&historyFakeHost{fakeHost:newFakeHost(),runtime:runtime2},b2); err!=nil { t.Fatal(err) }
 snapshot,text,updates:=b2.subscribe(); defer b2.unsubscribe(updates)
 finish:=start; finish.Status="ok"; finish.FinishedAt=at.Add(time.Second)
 b2.emit(message{kind:"event",data:finish}); b2.emit(message{kind:"delta",data:"B"})
 calls:=map[string]host.Event{}
 for _,m:=range snapshot { ev:=m.data.(host.Event); calls[ev.ID]=ev }
 update:=<-updates; ev:=update.data.(host.Event); calls[ev.ID]=ev
 delta:=<-updates
 if len(calls)!=1 || calls["active"].Status!="ok" || delta.kind!="delta" || text+delta.data.(string)!="AB" { t.Fatalf("snapshot/live mismatch: calls=%+v text=%q delta=%+v",calls,text,delta) }
}
