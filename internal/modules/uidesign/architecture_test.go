package uidesign

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestUIDesignModuleDoesNotImportLegacyLayers(t *testing.T) {
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
		for _, imported := range source.Imports {
			importPath, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				return err
			}
			if strings.HasPrefix(importPath, "github.com/Bobby-P-dev/go-diagram.git/src/") {
				t.Errorf("%s imports legacy layer %s; dependencies must point to uidesign module, not src/", path, importPath)
			}
			if strings.HasPrefix(importPath, "github.com/Bobby-P-dev/go-diagram.git/internal/modules/diagram") {
				t.Errorf("%s directly imports diagram module %s; cross-domain communication must use ports or platform", path, importPath)
			}
			if strings.HasPrefix(filepath.ToSlash(path), "service/") {
				if importPath == "net/http" || importPath == "database/sql/driver" || strings.HasPrefix(importPath, "github.com/redis/") || strings.HasPrefix(importPath, "github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign/repository") || strings.HasPrefix(importPath, "github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign/integration") {
					t.Errorf("%s imports infrastructure %s; use service ports instead", path, importPath)
				}
			}
			if strings.HasPrefix(filepath.ToSlash(path), "controller/") {
				if strings.HasPrefix(importPath, "database/sql") || strings.HasPrefix(importPath, "github.com/redis/") || strings.HasPrefix(importPath, "github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign/repository") || strings.HasPrefix(importPath, "github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign/integration") {
					t.Errorf("%s imports repository/infrastructure %s; controller must only access service and DTOs", path, importPath)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
