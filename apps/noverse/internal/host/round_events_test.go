package host

import (
 "encoding/json"
 "testing"

 "github.com/datnt1998/verse/apps/noverse/internal/store"
 "github.com/datnt1998/verse/core/agentcore"
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

func TestObserverOperationEventsCarryRoundIdentity(t *testing.T) {
	var live []string
	o:=testObserver(&[]Event{}); o.store=store.NewStore(t.TempDir())
	o.emitEv=func(ev Event) { if raw,err:=json.Marshal(ev); err==nil { live=append(live,string(raw)) } }
	o.dispatchStart("writer","Viết","", "Viết chương 1")
	check:=func(raw string, round string, status string) {
		var got map[string]any; if err:=json.Unmarshal([]byte(raw),&got); err!=nil { t.Fatal(err) }
		if got["roundId"]!=round || got["role"]!="writer" || got["id"]=="" || got["roundTitle"]!="Viết chương 1" || got["roundStartedAt"]==nil { t.Fatalf("round payload: %v",raw) }
		if status!="" && got["status"]!=status { t.Fatalf("status %q: %v",status,raw) }
	}
	queue,err:=o.store.Runtime.LoadQueue(); if err!=nil || len(queue)!=1 { t.Fatalf("queue %v",err) }
	raw,_:=json.Marshal(queue[0].Payload); var start map[string]any; _=json.Unmarshal(raw,&start); round,_:=start["roundId"].(string)
	if round=="" || start["role"]!="writer" || start["id"]!=round { t.Fatalf("start round fields: %v",start) }
	o.handleToolUpdate(agentcore.Event{Type:agentcore.EventToolExecUpdate,Progress:&agentcore.ProgressPayload{Kind:agentcore.ProgressToolStart,Agent:"writer",Tool:"draft_chapter",Args:json.RawMessage(`{}`)}})
	o.handleToolUpdate(agentcore.Event{Type:agentcore.EventToolExecUpdate,Progress:&agentcore.ProgressPayload{Kind:agentcore.ProgressToolEnd,Agent:"writer",Tool:"draft_chapter",Message:"ok"}})
	if len(live)!=3 { t.Fatalf("live events: %d",len(live)) }
	check(live[0],round,""); check(live[1],round,"running"); check(live[2],round,"ok")
}
