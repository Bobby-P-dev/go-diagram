package workspace

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestWorkspaceModuleBoundaries(t *testing.T) {
	err := filepath.WalkDir(".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		source, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		layer := strings.Split(filepath.ToSlash(path), "/")[0]
		for _, imported := range source.Imports {
			importPath, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				return err
			}
			for _, prefix := range []string{
				"github.com/Bobby-P-dev/go-diagram.git/src/",
				"github.com/Bobby-P-dev/go-diagram.git/internal/modules/diagram",
				"github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign",
			} {
				if strings.HasPrefix(importPath, prefix) {
					t.Errorf("%s imports %s; use module ports", path, importPath)
				}
			}
			infrastructure := strings.HasPrefix(importPath, "database/sql") || strings.HasPrefix(importPath, "github.com/redis/") || strings.HasPrefix(importPath, "github.com/Bobby-P-dev/go-diagram.git/internal/modules/workspace/repository")
			if (layer == "controller" || layer == "service") && infrastructure {
				t.Errorf("%s directly imports infrastructure %s", path, importPath)
			}
			if layer == "service" && (importPath == "net/http" || strings.Contains(importPath, "/controller") || strings.Contains(importPath, "/authcontext")) {
				t.Errorf("%s imports transport %s", path, importPath)
			}
			if layer == "repository" && (strings.HasSuffix(importPath, "/service") || strings.HasSuffix(importPath, "/controller")) {
				t.Errorf("%s imports upper layer %s", path, importPath)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
