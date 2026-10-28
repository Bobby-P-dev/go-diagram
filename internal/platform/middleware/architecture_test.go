package middleware

import (
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestMiddlewareHasNoDomainOrGlobalConfigImports(test *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		test.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		source, err := parser.ParseFile(token.NewFileSet(), entry.Name(), nil, parser.ImportsOnly)
		if err != nil {
			test.Fatal(err)
		}
		for _, imported := range source.Imports {
			importPath, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				test.Fatal(err)
			}
			if strings.Contains(importPath, "/src/") || strings.Contains(importPath, "/internal/modules/") || strings.HasSuffix(importPath, "/config") || importPath == "database/sql" || strings.Contains(importPath, "go-redis") {
				test.Errorf("%s imports %s: middleware must remain domain-independent", entry.Name(), importPath)
			}
		}
	}
}
