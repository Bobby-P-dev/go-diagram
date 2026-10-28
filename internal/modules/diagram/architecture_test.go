package diagram

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestDiagramModuleBoundaries(t *testing.T) {
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
				"github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign",
				"github.com/Bobby-P-dev/go-diagram.git/internal/modules/workspace",
				"github.com/Bobby-P-dev/go-diagram.git/internal/modules/sharing",
			} {
				if strings.HasPrefix(importPath, prefix) {
					t.Errorf("%s imports %s; module must remain isolated", path, importPath)
				}
			}
			if layer == "repository" && (strings.HasSuffix(importPath, "/service") || strings.HasSuffix(importPath, "/controller")) {
				t.Errorf("%s imports upper layer %s", path, importPath)
			}
			if layer == "service" && (strings.HasSuffix(importPath, "/controller") || strings.HasSuffix(importPath, "/repository") || strings.HasSuffix(importPath, "/config") || importPath == "database/sql" || importPath == "net/http" || strings.Contains(importPath, "go-redis")) {
				t.Errorf("%s imports forbidden dependency %s", path, importPath)
			}
			if layer == "controller" && (strings.HasSuffix(importPath, "/repository") || strings.HasSuffix(importPath, "/config") || importPath == "database/sql" || strings.Contains(importPath, "go-redis")) {
				t.Errorf("%s imports persistence or configuration %s", path, importPath)
			}
			if layer == "dto" && (strings.HasSuffix(importPath, "/service") || strings.HasSuffix(importPath, "/controller") || strings.HasSuffix(importPath, "/repository")) {
				t.Errorf("%s imports inner layer %s", path, importPath)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
