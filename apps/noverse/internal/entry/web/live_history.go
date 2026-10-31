package web

import (
 "encoding/json"
 "net/http"
 "sort"
 "strconv"
 "time"

 "github.com/datnt1998/verse/apps/noverse/internal/domain"
 "github.com/datnt1998/verse/apps/noverse/internal/host"
 "github.com/datnt1998/verse/apps/noverse/internal/library"
 "github.com/datnt1998/verse/platform/identity"
)

type liveOperation struct {
 ID string `json:"id"`; Role string `json:"role"`; Round int `json:"round"`; RoundID string `json:"roundId,omitempty"`; StartedAt time.Time `json:"startedAt"`; DurationMs int64 `json:"durationMs"`; Status string `json:"status"`; Error string `json:"error"`; Key string `json:"key"`; Params string `json:"params"`; Result string `json:"result"`
}
type liveRound struct { Round int `json:"round"`; RoundID string `json:"roundId"`; Role string `json:"role"`; Title string `json:"title,omitempty"`; StartedAt time.Time `json:"startedAt"`; Operations []liveOperation `json:"operations"` }

func liveRole(role string) string { return host.NormalizeRole(role) }

func liveHistoryRounds(queue []domain.RuntimeQueueItem) ([]liveRound,error) {
 rounds:=[]liveRound{}; active:=map[string]int{}; events:=[]host.Event{}; indexes:=map[string]int{}
 for _, item := range queue {
  raw,err:=json.Marshal(item.Payload); if err!=nil { return nil,err }; var ev host.Event; if err=json.Unmarshal(raw,&ev); err!=nil { return nil,err }
  if ev.ID=="" || (ev.Category!="DISPATCH" && ev.Category!="TOOL") { continue }
  if idx,ok:=indexes[ev.ID]; ok { if ev.RoundTitle=="" { ev.RoundTitle=events[idx].RoundTitle }; if ev.RoundID=="" { ev.RoundID=events[idx].RoundID }; if ev.RoundStartedAt==nil { ev.RoundStartedAt=events[idx].RoundStartedAt }; events[idx]=ev } else { indexes[ev.ID]=len(events); events=append(events,ev) }
 }
 sort.SliceStable(events,func(i,j int) bool { return events[i].Time.Before(events[j].Time) })
 for _,ev:=range events {
  if ev.Category=="DISPATCH" {
   idx:=len(rounds); active[ev.Agent]=idx+1
   rounds=append(rounds,liveRound{Round:idx+1,RoundID:ev.ID,Role:liveRole(ev.Agent),Title:ev.RoundTitle,StartedAt:ev.Time,Operations:[]liveOperation{}}); continue
  }
  if ev.Tool=="" { continue }
  r:=active[ev.Agent]-1; if r<0 || r>=len(rounds) { continue }
  status:="running"; if ev.Failed || ev.Status=="error" { status="error" } else if !ev.FinishedAt.IsZero() || ev.Status=="ok" || ev.Status=="done" { status="done" }
  message:=""; if status=="error" { message=host.OperationErrorMessage(ev) }
  duration:=ev.DurationMs; if duration==0 { duration=ev.Duration.Milliseconds() }
  rounds[r].Operations=append(rounds[r].Operations,liveOperation{ID:ev.ID,Role:liveRole(ev.Agent),Round:r+1,RoundID:rounds[r].RoundID,StartedAt:ev.Time,DurationMs:duration,Status:status,Error:message,Key:ev.Tool,Params:ev.Args,Result:ev.Result})
 }
 return rounds,nil
}
func (g *Gateway) serveLiveHistory(w http.ResponseWriter,r *http.Request,user identity.User,st library.Story) {
 if r.Method!=http.MethodGet { jsonError(w,404,"không tìm thấy"); return }
 limit:=20; before:=0
 for _,param:=range []struct{name string; dest *int; max int}{{"limit",&limit,100},{"beforeRound",&before,0}} {
  if value:=r.URL.Query().Get(param.name); value!="" { n,err:=strconv.Atoi(value); if err!=nil || n<1 || (param.max>0 && n>param.max) { jsonError(w,400,"Mốc lịch sử không hợp lệ"); return }; *param.dest=n }
 }
 facts,err:=g.openFactStore(st,&user); if err!=nil { libError(w,err); return }; queue,err:=facts.Runtime.LoadQueue(); if err!=nil { jsonError(w,500,"Không đọc được lịch sử luồng viết"); return }
 rounds,err:=liveHistoryRounds(queue); if err!=nil { jsonError(w,500,"Không đọc được lịch sử luồng viết"); return }
 end:=len(rounds); if before>0 && before<=end { end=before-1 }; start:=end-limit; if start<0 { start=0 }
 var next *int; if start>0 { n:=rounds[start].Round; next=&n }
 writeJSON(w,200,map[string]any{"rounds":rounds[start:end],"nextBeforeRound":next})
}
