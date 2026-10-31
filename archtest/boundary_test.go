// Package archtest enforces the import boundaries between core, platform and apps.
package archtest

import (
	"bufio"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// violation is one forbidden import edge, as module-relative package paths.
type violation struct{ from, to string }

func (v violation) String() string { return v.from + " → " + v.to }

// scan holds the import edges of every package directory in one module.
type scan struct {
	packages  map[string]bool            // module-relative package dirs
	edges     map[string]map[string]bool // importer → module-relative imports, all files
	prodEdges map[string]map[string]bool // importer → module-relative imports, non-test files
}

// Imports are read from source with every build constraint ignored, so files for
// other platforms and build tags are checked too. Test files count as imports of
// their directory.
func scanModule(t *testing.T, root string) scan {
	t.Helper()
	modPath := modulePath(t, filepath.Join(root, "go.mod"))
	s := scan{packages: map[string]bool{}, edges: map[string]map[string]bool{}, prodEdges: map[string]map[string]bool{}}
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if path != root && (name == "testdata" || name == "node_modules" || name == "vendor" ||
				strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")) {
				return filepath.SkipDir
			}
			if path != root {
				if _, err := os.Stat(filepath.Join(path, "go.mod")); err == nil {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		dir, err := filepath.Rel(root, filepath.Dir(path))
		if err != nil {
			return err
		}
		pkg := filepath.ToSlash(dir)
		s.packages[pkg] = true
		for _, imp := range f.Imports {
			target, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
			rel, ok := strings.CutPrefix(target, modPath+"/")
			if !ok {
				if target == modPath {
					rel = "."
				} else {
					continue // standard library or another module
				}
			}
			addEdge(s.edges, pkg, rel)
			if !strings.HasSuffix(path, "_test.go") {
				addEdge(s.prodEdges, pkg, rel)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan %s: %v", root, err)
	}
	return s
}

func addEdge(edges map[string]map[string]bool, from, to string) {
	if edges[from] == nil {
		edges[from] = map[string]bool{}
	}
	edges[from][to] = true
}

func modulePath(t *testing.T, goMod string) string {
	t.Helper()
	f, err := os.Open(goMod)
	if err != nil {
		t.Fatalf("open go.mod: %v", err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(sc.Text()), "module "); ok {
			return strings.Trim(strings.TrimSpace(rest), `"`)
		}
	}
	t.Fatalf("%s has no module line", goMod)
	return ""
}

// layer classifies a module-relative package path: "core", "platform",
// "apps/<name>", or "" for anything else in the module.
func layer(pkg string) string {
	first, rest, _ := strings.Cut(pkg, "/")
	switch first {
	case "core", "platform":
		return first
	case "apps":
		if name, _, _ := strings.Cut(rest, "/"); name != "" {
			return "apps/" + name
		}
	}
	return ""
}

// allowed reports whether a package in layer from may import a module package in
// layer to. Standard library and third-party imports never reach this check.
func allowed(from, to string) bool {
	switch {
	case from == "core":
		return to == "core"
	case from == "platform":
		return to == "core" || to == "platform"
	case strings.HasPrefix(from, "apps/"):
		return !strings.HasPrefix(to, "apps/") || to == from
	}
	return true
}

func (s scan) violations() []violation {
	var out []violation
	for from, imports := range s.edges {
		for to := range imports {
			if !allowed(layer(from), layer(to)) {
				out = append(out, violation{from, to})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out
}

// blobTransportIssues keeps the shared byte transport cutover complete: apps
// must consume platform directly rather than retaining private copies or shims.
func (s scan) blobTransportIssues() []string {
	var issues []string
	for _, required := range []string{"platform/blobstore", "platform/blobhttp"} {
		if !s.packages[required] {
			issues = append(issues, "missing shared blob transport package: "+required)
		}
	}
	obsolete := func(pkg string) bool {
		return pkg == "apps/noverse/internal/blobstore" || strings.HasPrefix(pkg, "apps/noverse/internal/blobstore/") ||
			pkg == "apps/noverse/internal/s3test" || strings.HasPrefix(pkg, "apps/noverse/internal/s3test/")
	}
	for pkg := range s.packages {
		if obsolete(pkg) {
			issues = append(issues, "obsolete blob transport package: "+pkg)
		}
	}
	for from, imports := range s.edges {
		for to := range imports {
			if obsolete(to) {
				issues = append(issues, "obsolete blob transport import: "+(violation{from, to}).String())
			}
		}
	}
	sort.Strings(issues)
	return issues
}

// layeredApps are the apps whose internal packages follow the layered layout:
// cmd composes; web and persistence adapt; business packages hold the rules;
// domain holds shared value types.
var layeredApps = []string{"viderse"}

// appLayer classifies a package of a layered app: "cmd", "web", "persistence",
// "bootstrap", "domain", "business", or "" for a package outside the app.
func appLayer(app, pkg string) string {
	rest, ok := strings.CutPrefix(pkg, "apps/"+app+"/")
	if !ok {
		return ""
	}
	under := func(dir string) bool { return rest == dir || strings.HasPrefix(rest, dir+"/") }
	switch {
	case under("cmd"):
		return "cmd"
	case under("internal/web"):
		return "web"
	case under("internal/persistence"):
		return "persistence"
	case rest == "internal/bootstrap":
		return "bootstrap"
	case rest == "internal/domain":
		return "domain"
	case strings.HasPrefix(rest, "internal/"):
		return "business"
	}
	return ""
}

// appLayerAllowed lists the layers each layer of a layered app may import.
// Business packages reach storage, HTTP and composition only through
// interfaces they declare, so they never import web, persistence, bootstrap or
// cmd. Nothing imports cmd.
var appLayerAllowed = map[string][]string{
	"cmd":         {"web", "persistence", "bootstrap", "domain", "business"},
	"web":         {"web", "domain", "business"},
	"persistence": {"persistence", "domain", "business"},
	"bootstrap":   {"bootstrap", "domain"},
	"business":    {"business", "domain"},
	"domain":      {"domain"},
}

// appLayerViolations checks production imports only: a test may wire a real
// store into the package it tests.
func (s scan) appLayerViolations(app string) []violation {
	var out []violation
	for from, imports := range s.prodEdges {
		fromLayer := appLayer(app, from)
		if fromLayer == "" {
			continue
		}
		for to := range imports {
			toLayer := appLayer(app, to)
			if toLayer == "" || from == to {
				continue
			}
			if !slices.Contains(appLayerAllowed[fromLayer], toLayer) {
				out = append(out, violation{from, to})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out
}

func TestImportBoundaries(t *testing.T) {
	s := scanModule(t, "..")
	core := 0
	for pkg := range s.packages {
		if layer(pkg) == "core" {
			core++
		}
	}
	if core == 0 {
		t.Fatal("found no core packages; the scan did not start at the repository root")
	}
	for _, v := range s.violations() {
		t.Errorf("forbidden import: %s", v)
	}
}

func TestLayeredAppImports(t *testing.T) {
	s := scanModule(t, "..")
	for _, app := range layeredApps {
		found := false
		for pkg := range s.packages {
			if appLayer(app, pkg) == "business" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("found no business packages in apps/%s", app)
		}
		for _, v := range s.appLayerViolations(app) {
			t.Errorf("forbidden import in apps/%s: %s", app, v)
		}
	}
}

// The planted module holds exactly one violation per rule next to allowed edges,
// so a checker that stops seeing violations fails here.
func TestImportBoundariesCatchPlantedViolations(t *testing.T) {
	got := scanModule(t, filepath.Join("testdata", "violation")).violations()
	want := []violation{
		{"apps/b/z", "apps/a/y"},
		{"core/x", "apps/a/y"},
		{"platform/p", "apps/a/y"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("planted violations = %v, want %v", got, want)
	}
}

// The planted layered app holds one violation per forbidden direction next to
// allowed edges and a test file that imports persistence.
func TestLayeredAppImportsCatchPlantedViolations(t *testing.T) {
	got := scanModule(t, filepath.Join("testdata", "violation")).appLayerViolations("viderse")
	want := []violation{
		{"apps/viderse/internal/domain", "apps/viderse/internal/status"},
		{"apps/viderse/internal/status", "apps/viderse/internal/persistence/store"},
		{"apps/viderse/internal/status", "apps/viderse/internal/web"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("planted layer violations = %v, want %v", got, want)
	}
}
