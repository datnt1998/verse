package web

import (
 "os"
 "path/filepath"
 "strings"
 "testing"
)

func TestProjectCopyRulesSnapshot(t *testing.T) {
 tg := newTestGateway(t)
 admin := tg.login("admin", "admin-pass")
 source := filepath.Join(tg.root, "proj", ".ainovel")
 if err := os.MkdirAll(filepath.Join(source, "rules"), 0755); err != nil { t.Fatal(err) }
 if err := os.WriteFile(filepath.Join(source, "rules", "a.md"), []byte("Luật gốc"), 0644); err != nil { t.Fatal(err) }
 if err := os.WriteFile(filepath.Join(source, "config.json"), []byte(`{"style":"trữ tình"}`), 0644); err != nil { t.Fatal(err) }
 w := tg.do("POST", "/api/projects", admin, `{"slug":"copy","name":"Bản sao","copyRulesFrom":"proj"}`)
 if w.Code != 201 { t.Fatalf("copy: %d %s", w.Code, w.Body) }
 dest := filepath.Join(tg.root, "copy", ".ainovel")
 b, err := os.ReadFile(filepath.Join(dest, "rules", "a.md")); if err != nil || string(b) != "Luật gốc" { t.Fatalf("rule: %q %v", b, err) }
 b, err = os.ReadFile(filepath.Join(dest, "config.json")); if err != nil || !strings.Contains(string(b), "trữ tình") { t.Fatalf("style: %q %v", b, err) }
 if err := os.WriteFile(filepath.Join(dest, "rules", "a.md"), []byte("Mới"), 0644); err != nil { t.Fatal(err) }
 if err := os.WriteFile(filepath.Join(dest, "config.json"), []byte(`{"style":"khác"}`), 0644); err != nil { t.Fatal(err) }
 b, _ = os.ReadFile(filepath.Join(source, "config.json")); if !strings.Contains(string(b), "trữ tình") { t.Fatalf("source style changed: %q", b) }
 list := decodeJSONBody(t, tg.do("GET", "/api/projects", admin, ""))["projects"].([]any)
 found := false
 for _, item := range list { p := item.(map[string]any); if p["id"] == "proj" { found = p["ruleCount"] == float64(1) && p["style"] == "trữ tình" } }
 if !found { t.Fatalf("source summary missing rule count/style: %v", list) }
 if got:=tg.do("POST", "/api/projects", admin, `{"slug":"missing","copyRulesFrom":"unknown"}`); got.Code!=400 { t.Fatalf("missing source: %d %s", got.Code, got.Body) }
 reader := tg.login("reader", "reader-pass")
 if got:=tg.do("POST", "/api/projects", reader, `{"slug":"forbidden","copyRulesFrom":"proj"}`); got.Code!=403 && got.Code!=404 { t.Fatalf("forbidden: %d %s", got.Code, got.Body) }
 b, _ = os.ReadFile(filepath.Join(source, "rules", "a.md")); if string(b) != "Luật gốc" { t.Fatalf("source changed: %q", b) }
}
