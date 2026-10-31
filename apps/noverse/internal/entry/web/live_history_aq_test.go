package web

import (
 "fmt"
 "net/http"
 "testing"
 "time"

 "github.com/datnt1998/verse/apps/noverse/internal/domain"
 "github.com/datnt1998/verse/apps/noverse/internal/host"
)

func TestLiveHistoryPagesDispatchRounds(t *testing.T) {
 tg := newTestGateway(t)
 cookie := tg.login("admin", "admin-pass")
 lib, err := tg.g.lib(t.Context(), nil); if err != nil { t.Fatal(err) }
 st, ok := lib.ResolveStory(t.Context(), nil, "_", "oldbook"); if !ok { t.Fatal("story missing") }
 facts, err := tg.g.openFactStoreInternal(st); if err != nil { t.Fatal(err) }
 for n:=1; n<=25; n++ {
  at := time.Unix(int64(n*10),0)
  for _, ev := range []host.Event{{ID:fmt.Sprintf("t%d",n),Category:"TOOL",Agent:"writer",Tool:"draft_chapter",Time:at.Add(time.Second),FinishedAt:at.Add(2*time.Second),Failed:n==25,Status:map[bool]string{true:"error",false:"ok"}[n==25],Args:`{"chapter":1}`,Result:"raw"}, {ID:fmt.Sprintf("d%d",n),Category:"DISPATCH",Agent:"writer",Time:at,FinishedAt:at.Add(3*time.Second)}} {
   if _,err := facts.Runtime.AppendQueue(domain.RuntimeQueueItem{Payload:ev}); err != nil { t.Fatal(err) }
  }
 }
 path := "/api/projects/_/stories/oldbook/live-history"
 rec:=tg.do(http.MethodGet,path,cookie,""); if rec.Code != 200 { t.Fatalf("history: %d %s",rec.Code,rec.Body) }
 first:=decodeJSONBody(t,rec); rounds:=first["rounds"].([]any); if len(rounds)!=20 || rounds[0].(map[string]any)["round"]!=float64(6) { t.Fatalf("newest: %v",rounds) }
 for i, item := range rounds { r:=item.(map[string]any); if r["round"] != float64(i+6) || r["roundId"]=="" { t.Fatalf("out of order: %v", rounds) } }
 last:=rounds[19].(map[string]any)["operations"].([]any)[0].(map[string]any); if last["status"]!="error" || last["error"]=="" { t.Fatalf("error: %v",last) }
 rec=tg.do(http.MethodGet,path+"?beforeRound=6&limit=20",cookie,""); if rec.Code!=200 { t.Fatal(rec.Body.String()) }; second:=decodeJSONBody(t,rec); older:=second["rounds"].([]any); if len(older)!=5 { t.Fatalf("older: %v",second) }; for i,item:=range older { if item.(map[string]any)["round"]!=float64(i+1) { t.Fatalf("older order: %v", older) } }
}

func TestLiveHistoryArchitectRole(t *testing.T) {
 at:=time.Now()
 rounds,err:=liveHistoryRounds([]domain.RuntimeQueueItem{{Payload:host.Event{ID:"d",Category:"DISPATCH",Agent:"architect_long",Time:at}}, {Payload:host.Event{ID:"t",Category:"TOOL",Agent:"architect_long",Time:at.Add(time.Second),Tool:"save_book"}}})
 if err!=nil || len(rounds)!=1 || rounds[0].Role!="architect" || len(rounds[0].Operations)!=1 || rounds[0].Operations[0].Role!="architect" { t.Fatalf("architect role: %+v %v",rounds,err) }
}
