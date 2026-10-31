package web

import (
 "net/http"
 "strings"
 "testing"

 "github.com/datnt1998/verse/apps/noverse/internal/persistence/postgres"
)

func TestHostedProjectCopyRulesSnapshot(t *testing.T) {
 f:=provisionHostedGateway(t,"copy-source","story")
 actor:=&postgres.Actor{Username:f.admin.Username,GlobalRole:string(f.admin.Role)}
 docs:=f.repo.TextDocuments()
 if _,err:=docs.Mutate(t.Context(),actor,f.project,"","rules","a.md","Luật gốc","",false,0); err!=nil { t.Fatal(err) }
 if _,err:=docs.Mutate(t.Context(),actor,f.project,"","rules","Luật riêng.MD","Giữ giọng","",false,0); err!=nil { t.Fatal(err) }
 if err:=f.repo.ConfigDocuments().Put(t.Context(),actor,f.project,"",[]byte(`{"style":"trữ tình"}`),""); err!=nil { t.Fatal(err) }
 w:=callProjects(f.g,f.admin,http.MethodPost,"/api/projects",`{"slug":"copy-target","name":"Bản sao","copyRulesFrom":"copy-source"}`)
 if w.Code!=201 { t.Fatalf("create: %d %s",w.Code,w.Body) }
 rows,err:=docs.List(t.Context(),actor,"copy-target","","rules"); if err!=nil || len(rows)!=2 { t.Fatalf("rules: %+v %v",rows,err) }; texts:=map[string]string{}; for _,row:=range rows { texts[row.Name]=row.Text }; if texts["a.md"]!="Luật gốc" || texts["Luật riêng.MD"]!="Giữ giọng" { t.Fatalf("rules: %+v",texts) }
 cfg,_,err:=f.repo.ConfigDocuments().Get(t.Context(),actor,"copy-target",""); if err!=nil || !strings.Contains(string(cfg),"trữ tình") { t.Fatalf("style: %s %v",cfg,err) }
 if _,err:=docs.Mutate(t.Context(),actor,"copy-target","","rules","a.md","Luật mới","",false,0); err!=nil { t.Fatal(err) }
 if err:=f.repo.ConfigDocuments().Put(t.Context(),actor,"copy-target","",[]byte(`{"style":"khác"}`),""); err!=nil { t.Fatal(err) }
 original,_,err:=f.repo.ConfigDocuments().Get(t.Context(),actor,f.project,""); if err!=nil || !strings.Contains(string(original),"trữ tình") { t.Fatalf("source style changed: %s %v",original,err) }
 rows,err=docs.List(t.Context(),actor,f.project,"","rules"); if err!=nil || len(rows)!=2 { t.Fatalf("source changed: %+v %v",rows,err) }; for _,row:=range rows { if row.Name=="a.md" && row.Text!="Luật gốc" { t.Fatalf("source changed: %+v",row) } }
 w=callProjects(f.g,f.outsider,http.MethodPost,"/api/projects",`{"slug":"illegal","name":"Cấm","copyRulesFrom":"copy-source"}`); if w.Code!=403 && w.Code!=404 { t.Fatalf("unauthorized: %d %s",w.Code,w.Body) }
}
