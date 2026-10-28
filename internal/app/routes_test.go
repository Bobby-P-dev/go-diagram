package app_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestOpenAPIContractCoverage(t *testing.T) {
	content, err := os.ReadFile("../../docs/api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}

	pathPattern := regexp.MustCompile(`^  (/[^ ]*):$`)
	methodPattern := regexp.MustCompile(`^    (get|post|put|patch|delete|options|head|trace):$`)
	documented := make(map[string]bool)
	currentPath := ""
	inPaths := false
	for _, line := range strings.Split(string(content), "\n") {
		if line == "paths:" {
			inPaths = true
			continue
		}
		if !inPaths {
			continue
		}
		if line != "" && line[0] != ' ' && line[0] != '#' {
			break
		}
		if match := pathPattern.FindStringSubmatch(line); match != nil {
			currentPath = match[1]
		}
		if match := methodPattern.FindStringSubmatch(line); match != nil {
			operation := strings.ToUpper(match[1]) + " " + currentPath
			if documented[operation] {
				t.Errorf("duplicate documented operation: %s", operation)
			}
			documented[operation] = true
		}
	}
	if len(documented) == 0 {
		t.Fatal("no OpenAPI operations found; use two-space YAML indentation and unquoted path keys")
	}

	router, err := parser.ParseFile(token.NewFileSet(), "routes.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	registered := make(map[string]bool)
	ast.Inspect(router, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "HandleFunc" {
			return true
		}
		if len(call.Args) != 2 {
			t.Error("expected a route pattern and handler")
			return true
		}
		literal, ok := call.Args[0].(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			t.Error("route pattern must be a string literal for contract coverage")
			return true
		}
		operation, err := strconv.Unquote(literal.Value)
		if err != nil {
			t.Error(err)
			return true
		}
		if operation == "GET /{$}" {
			operation = "GET /"
		}
		registered[operation] = true
		return true
	})
	if len(registered) == 0 {
		t.Fatal("no registered routes found")
	}
	for operation := range registered {
		if !documented[operation] {
			t.Errorf("registered operation missing from OpenAPI: %s", operation)
		}
	}
	for operation := range documented {
		if !registered[operation] {
			t.Errorf("documented operation missing from router: %s", operation)
		}
	}
}
