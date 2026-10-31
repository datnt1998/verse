package insight

import (
 "context"
 "testing"
 "time"
)
type recapStore struct{}
func (recapStore) Get(context.Context,string,any)(bool,error){return false,nil}
func (recapStore) Set(context.Context,string,any)error{return nil}
func (recapStore) ReadRecap(_ context.Context,from,to time.Time)(Recap,error){r:=Recap{From:from,To:to,NeedsYou:[]Need{{State:"review"}}};r.Summarize();return r,nil}
func TestRecapNarratorCannotAlterCounts(t *testing.T){
 s:=Service{Store:recapStore{},Narrate:func(context.Context,Recap)(string,error){return "9 video chờ duyệt.",nil}}
 r,err:=s.Recap(context.Background(),nil);if err!=nil{t.Fatal(err)}
 if r.Summary!="Trong lúc bạn vắng: 1 video chờ duyệt."{t.Fatal(r.Summary)}
}
