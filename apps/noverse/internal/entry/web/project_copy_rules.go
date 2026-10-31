package web

import (
 "context"
 "os"
 "path/filepath"
 "strings"

 "github.com/datnt1998/verse/apps/noverse/internal/library"
 "github.com/datnt1998/verse/platform/identity"
)

type projectRulesSnapshot struct { style string; files map[string]string }

func projectConfigTarget(ref library.ProjectRef) configTarget {
 return configTarget{scope:"project", project:ref.ID, dir:ref.Dir, path:configFileIn(ref.Dir), hosted:ref.Dir == ""}
}

func (g *Gateway) readProjectRules(ctx context.Context, user identity.User, ref library.ProjectRef) (projectRulesSnapshot, error) {
 out := projectRulesSnapshot{files:map[string]string{}}
 if ref.Legacy { return out, nil }
 target := projectConfigTarget(ref)
 cfg, _, err := g.loadTargetConfig(ctx, target, user); if err != nil { return out, err }; out.style = cfg.Style
 if target.hosted {
  rows, err := g.hostedRuleFiles(ctx, user, target); if err != nil { return out, err }
  for _, row := range rows { if validRuleName(row.Name) && !strings.HasPrefix(row.Name, ".") { out.files[row.Name] = row.Text } }
  return out, nil
 }
 entries, err := os.ReadDir(rulesDirIn(ref.Dir)); if os.IsNotExist(err) { return out,nil }; if err != nil { return out,err }
 for _, entry := range entries {
  if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !validRuleName(entry.Name()) || strings.HasPrefix(entry.Name(), ".") { continue }
  data, err := os.ReadFile(filepath.Join(rulesDirIn(ref.Dir),entry.Name())); if err != nil { return out,err }; out.files[entry.Name()] = string(data)
 }
 return out,nil
}

func (g *Gateway) writeProjectRules(ctx context.Context, user identity.User, ref library.ProjectRef, snapshot projectRulesSnapshot) error {
 target := projectConfigTarget(ref)
 cfg, _, err := g.loadTargetConfig(ctx,target,user); if err != nil { return err }; cfg.Style = snapshot.style
 if err := g.saveTargetConfig(ctx,target,user,cfg,""); err != nil { return err }
 for name, text := range snapshot.files {
  if target.hosted { _,err = g.hostedTextDocuments().Mutate(ctx,textActor(user),ref.ID,"","rules",name,text,"",false,0) } else { err = os.WriteFile(filepath.Join(rulesDirIn(ref.Dir),name),[]byte(text),0644) }
  if err != nil { return err }
 }
 return nil
}
