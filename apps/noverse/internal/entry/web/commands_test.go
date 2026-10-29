package web

import (
	"encoding/json"
	"net/http"
	"testing"
)

type commandSpecJSON struct {
	Name        string   `json:"name"`
	Aliases     []string `json:"aliases"`
	Usage       string   `json:"usage"`
	Group       string   `json:"group"`
	AutoExecute bool     `json:"autoExecute"`
	NeedsIdle   bool     `json:"needsIdle"`
	Only        string   `json:"only"`
	Params      []struct {
		Key      string `json:"key"`
		Token    string `json:"token"`
		Kind     string `json:"kind"`
		Required bool   `json:"required"`
		Prefix   string `json:"prefix"`
		Flag     string `json:"flag"`
		Options  []struct {
			Value string `json:"value"`
			Note  string `json:"note"`
		} `json:"options"`
	} `json:"params"`
}

func TestCommandsEndpoint(t *testing.T) {
	tg := newTestGateway(t)
	cookie := tg.login("reader", "reader-pass")

	w := tg.do(http.MethodGet, "/api/commands", cookie, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content type = %q", ct)
	}
	var specs []commandSpecJSON
	if err := json.Unmarshal(w.Body.Bytes(), &specs); err != nil {
		t.Fatalf("body %q: %v", w.Body.String(), err)
	}
	if len(specs) != 14 {
		t.Fatalf("got %d commands, want 14", len(specs))
	}
	if specs[0].Name != "help" || specs[13].Name != "export" {
		t.Fatalf("thứ tự = %q…%q", specs[0].Name, specs[13].Name)
	}
	byName := make(map[string]commandSpecJSON, len(specs))
	for _, spec := range specs {
		byName[spec.Name] = spec
		if spec.Usage == "" || spec.Group == "" {
			t.Fatalf("spec %q missing usage/group", spec.Name)
		}
		if spec.Aliases == nil || spec.Params == nil {
			t.Fatalf("spec %q aliases/params must be []", spec.Name)
		}
	}
	cocreate := byName["cocreate"]
	if len(cocreate.Aliases) != 1 || cocreate.Aliases[0] != "plan" || cocreate.Only != "running" {
		t.Fatalf("cocreate = %+v", cocreate)
	}
	if start := byName["start"]; start.Only != "welcome" {
		t.Fatalf("start = %+v", start)
	}
	review := byName["review"]
	if len(review.Params) != 1 || review.Params[0].Kind != "enum" || !review.Params[0].Required {
		t.Fatalf("review params = %+v", review.Params)
	}
	if len(review.Params[0].Options) != 2 || review.Params[0].Options[0].Value != "on" {
		t.Fatalf("review options = %+v", review.Params[0].Options)
	}
	export := byName["export"]
	if len(export.Params) != 7 {
		t.Fatalf("export params = %+v", export.Params)
	}
	if export.Params[1].Kind != "num" || export.Params[1].Prefix != "from=" || export.Params[1].Required {
		t.Fatalf("export from = %+v", export.Params[1])
	}
	if export.Params[3].Prefix != "format=" || export.Params[3].Kind != "enum" || len(export.Params[3].Options) != 3 {
		t.Fatalf("export format = %+v", export.Params[3])
	}
	if export.Params[4].Flag != "--overwrite" || export.Params[5].Flag != "--keep" || export.Params[6].Flag != "doc-so" {
		t.Fatalf("export flags = %+v", export.Params[4:])
	}
}

func TestCommandsEndpointRequiresAuth(t *testing.T) {
	tg := newTestGateway(t)
	w := tg.do(http.MethodGet, "/api/commands", "", "")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
}
