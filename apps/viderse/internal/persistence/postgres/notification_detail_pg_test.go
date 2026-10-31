package postgres_test

import (
 "context"
 "fmt"
 "testing"
 "github.com/datnt1998/verse/apps/viderse/internal/activity"
 "github.com/datnt1998/verse/apps/viderse/internal/domain"
 "github.com/datnt1998/verse/apps/viderse/internal/persistence/postgres"
)
func TestTransitionNotificationDetails(t *testing.T) {
 ctx:=context.Background();pool:=provision(t);store:=postgres.NewCloneStore(pool)
 var a,b string
 for i,p:=range []*string{&a,&b} { if err:=pool.QueryRow(ctx,`INSERT INTO owner_channels(id,name) VALUES(gen_random_uuid(),$1) RETURNING id::text`,fmt.Sprintf("Kênh %d",i+1)).Scan(p);err!=nil{t.Fatal(err)} }
 newJob:=func(ch string)string{var id string;if err:=pool.QueryRow(ctx,`INSERT INTO clone_jobs(id,channel_id,source_key,source_snapshot,status,stage_group) VALUES(gen_random_uuid(),$1,gen_random_uuid()::text,'{"title":"Video"}','running','working') RETURNING id::text`,ch).Scan(&id);err!=nil{t.Fatal(err)};return id}
 for _,channel:=range []string{a,a,b} {id:=newJob(channel);if _,err:=pool.Exec(ctx,`INSERT INTO clone_steps(job_id,step,status,fence,attempt) VALUES($1,'images','running',1,1)`,id);err!=nil{t.Fatal(err)};if err:=store.ApplyStepWaiting(ctx,id,domain.CloneImages,1,1,nil,&domain.CloneError{Code:domain.CloneErrAwaitingReview});err!=nil{t.Fatal(err)}}
 fail:=newJob(a);if _,err:=pool.Exec(ctx,`INSERT INTO clone_steps(job_id,step,status,error) VALUES($1,'images','failed','{"code":"tool_missing","message":"Cần đăng nhập","recovery":"Mở Cài đặt → Kết nối AI; đăng nhập ChatGPT"}')`,fail);err!=nil{t.Fatal(err)};if err:=store.SetJobStatus(ctx,fail,domain.CloneFailed);err!=nil{t.Fatal(err)}
 ready:=newJob(b);for _,step:=range domain.CloneSteps {output:=`{}`;if step==domain.CloneBuild {output=`{"data":{"durationSec":2292}}`};if step==domain.CloneExport {output=`{"data":{"size":180355072}}`};if _,err:=pool.Exec(ctx,`INSERT INTO clone_steps(job_id,step,status,output_manifest) VALUES($1,$2,'succeeded',$3::jsonb)`,ready,step,output);err!=nil{t.Fatal(err)}};if err:=store.SetJobStatus(ctx,ready,domain.CloneSucceeded);err!=nil{t.Fatal(err)}
 notes,err:=activity.New(postgres.NewActivityStore(pool),nil).Notifications(ctx);if err!=nil{t.Fatal(err)}
 found:=map[string]activity.Notification{};for _,n:=range notes{found[n.Kind]=n}
 d:=found["review_ready"].Detail;if d==nil||len(d.Channels)!=2 {t.Fatalf("review detail=%+v",d)};counts:=map[string]int{};for _,ch:=range d.Channels {counts[ch.ChannelID]=ch.Count};if counts[a]!=2||counts[b]!=1{t.Fatalf("channel counts=%v",counts)}
 d=found["needs_attention"].Detail;if d==nil||d.Cause!="Cần đăng nhập"||d.Recovery==nil||d.Recovery.Label!="Đăng nhập lại"||d.Recovery.Href!="#/settings?section=connect"{t.Fatalf("attention detail=%+v",d)}
 d=found["ready_to_post"].Detail;if d==nil||d.MP4==nil||d.MP4.DurationSec==nil||*d.MP4.DurationSec!=2292||d.MP4.Bytes==nil||*d.MP4.Bytes!=180355072{t.Fatalf("ready detail=%+v",d)}
 unread,err:=postgres.NewActivityStore(pool).ListUnreadNotifications(ctx,100);if err!=nil{t.Fatal(err)}
 if len(unread)!=3 {t.Fatalf("unread=%d",len(unread))};for _,n:=range unread{if n.Detail==nil{t.Fatalf("unread detail missing for %s",n.Kind)}}
}
