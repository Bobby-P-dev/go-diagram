#!/usr/bin/env python3
"""Validate the backend OpenAPI contract and its parity with the Go router.

This check is intentionally offline and dependency-light. It is the local/CI
gate behind `make lint-spec`.

Checks:
  * the document parses as YAML and declares OpenAPI 3.0.x;
  * every operation has operationId, summary, tags, and responses;
  * operationIds are unique and match the documented tag list;
  * all local $ref pointers resolve;
  * 3.1-only keywords are not used in a 3.0 document;
  * every registered Go route exists in the spec, and vice versa.
"""

from __future__ import annotations

import re
import sys
from pathlib import Path
from typing import Any

try:
    import yaml
except ImportError:  # pragma: no cover - CI installs PyYAML explicitly
    sys.exit("PyYAML is required: pip install pyyaml")

ROOT = Path(__file__).resolve().parents[1]
SPEC_PATH = ROOT / "docs" / "api" / "openapi.yaml"
ROUTES_PATH = ROOT / "internal" / "app" / "routes.go"

HTTP_METHODS = {"get", "put", "post", "delete", "options", "head", "patch", "trace"}
PATH_LEVEL_KEYS = {"$ref", "summary", "description", "servers", "parameters"}
EXPECTED_OPERATIONS = 38

MISSING = object()
errors: list[str] = []


def add(message: str) -> None:
    errors.append(message)


def load_yaml(path: Path) -> Any:
    try:
        return yaml.safe_load(path.read_text(encoding="utf-8"))
    except OSError as exc:
        sys.exit(f"FAIL: cannot read {path}: {exc}")
    except yaml.YAMLError as exc:
        sys.exit(f"FAIL: cannot parse {path}: {exc}")


def resolve_ref(spec: Any, ref: str) -> Any:
    if not ref.startswith("#/"):
        return MISSING
    node = spec
    for raw_part in ref[2:].split("/"):
        part = raw_part.replace("~1", "/").replace("~0", "~")
        if isinstance(node, dict) and part in node:
            node = node[part]
        elif isinstance(node, list):
            try:
                node = node[int(part)]
            except (ValueError, IndexError):
                return MISSING
        else:
            return MISSING
    return node


def check_refs_and_keywords(spec: Any) -> None:
    def walk(node: Any, pointer: str) -> None:
        if isinstance(node, dict):
            for key, value in node.items():
                here = f"{pointer}/{key}"
                if key == "$ref":
                    if not isinstance(value, str):
                        add(f"{here}: $ref must be a string")
                    elif not value.startswith("#/"):
                        add(f"{here}: external $ref is not supported: {value}")
                    elif resolve_ref(spec, value) is MISSING:
                        add(f"{here}: unresolved $ref {value}")
                elif key == "const":
                    add(f"{here}: 'const' is OpenAPI 3.1-only; use enum")
                else:
                    walk(value, here)
        elif isinstance(node, list):
            for index, value in enumerate(node):
                walk(value, f"{pointer}/{index}")

    walk(spec, "#")


def collect_operations(spec: dict[str, Any]) -> dict[tuple[str, str], dict[str, Any]]:
    operations: dict[tuple[str, str], dict[str, Any]] = {}
    paths = spec.get("paths")
    if not isinstance(paths, dict) or not paths:
        add("paths must be a non-empty object")
        return operations

    for path, item in paths.items():
        if not isinstance(path, str) or not path.startswith("/"):
            add(f"path must start with '/': {path!r}")
            continue
        if not isinstance(item, dict):
            add(f"{path}: path item must be an object")
            continue
        for method, operation in item.items():
            if method in PATH_LEVEL_KEYS:
                continue
            if method not in HTTP_METHODS:
                add(f"{path}: unexpected key under path item: {method!r}")
                continue
            if not isinstance(operation, dict):
                add(f"{method.upper()} {path}: operation must be an object")
                continue
            operations[(method.upper(), path)] = operation

    return operations


def check_operations(spec: dict[str, Any], operations: dict[tuple[str, str], dict[str, Any]]) -> None:
    declared_tags = {
        tag["name"]
        for tag in spec.get("tags", [])
        if isinstance(tag, dict) and isinstance(tag.get("name"), str)
    }
    seen_ids: dict[str, str] = {}

    for (method, path), operation in operations.items():
        where = f"{method} {path}"

        operation_id = operation.get("operationId")
        if not isinstance(operation_id, str) or not operation_id.strip():
            add(f"{where}: missing operationId")
        elif operation_id in seen_ids:
            add(f"{where}: duplicate operationId {operation_id!r} (also used by {seen_ids[operation_id]})")
        else:
            seen_ids[operation_id] = where

        if not isinstance(operation.get("summary"), str) or not operation["summary"].strip():
            add(f"{where}: missing summary")

        tags = operation.get("tags")
        if not isinstance(tags, list) or not tags:
            add(f"{where}: missing tags")
        else:
            for tag in tags:
                if tag not in declared_tags:
                    add(f"{where}: tag {tag!r} is not declared in the top-level tags list")

        responses = operation.get("responses")
        if not isinstance(responses, dict) or not responses:
            add(f"{where}: missing responses")
            continue
        for code, response in responses.items():
            response_where = f"{where} response {code}"
            if not re.fullmatch(r"[1-5][0-9]{2}", str(code)):
                add(f"{response_where}: response code must be a 3-digit HTTP status")
            if not isinstance(response, dict):
                add(f"{response_where}: response must be an object")
                continue
            ref = response.get("$ref")
            if ref is not None:
                target = resolve_ref(spec, ref) if isinstance(ref, str) else MISSING
                if not isinstance(target, dict) or "description" not in target:
                    add(f"{response_where}: referenced response must define a description")
                continue
            if "description" not in response:
                add(f"{response_where}: missing description")
            if "schema" in response:
                add(f"{response_where}: schema must be nested under content, not on the response object")

    if len(operations) != EXPECTED_OPERATIONS:
        add(
            f"expected {EXPECTED_OPERATIONS} operations, found {len(operations)}; "
            "update EXPECTED_OPERATIONS only when the route contract intentionally changes"
        )


def parse_go_routes(path: Path) -> set[tuple[str, str]]:
    try:
        text = path.read_text(encoding="utf-8")
    except OSError as exc:
        add(f"cannot read {path}: {exc}")
        return set()

    routes: set[tuple[str, str]] = set()
    pattern = re.compile(r'mux\.HandleFunc\("([A-Z]+)\s+([^"]+)"')
    for method, route in pattern.findall(text):
        # Go 1.22 exact-root pattern is written /{$}; the contract exposes it as /.
        routes.add((method, route.replace("/{$}", "/")))
    return routes


def main() -> int:
    if not SPEC_PATH.exists():
        sys.exit(f"FAIL: missing {SPEC_PATH}")
    spec = load_yaml(SPEC_PATH)
    if not isinstance(spec, dict):
        sys.exit(f"FAIL: {SPEC_PATH} must contain a YAML object")

    version = spec.get("openapi")
    if not isinstance(version, str) or not version.startswith("3.0."):
        add(f"openapi must be a 3.0.x version, found {version!r}")

    info = spec.get("info")
    if not isinstance(info, dict) or not info.get("title") or not info.get("version"):
        add("info.title and info.version are required")

    check_refs_and_keywords(spec)
    operations = collect_operations(spec)
    check_operations(spec, operations)

    go_routes = parse_go_routes(ROUTES_PATH)
    spec_routes = set(operations)

    for method, path in sorted(go_routes - spec_routes):
        add(f"registered in internal/app/routes.go but missing from OpenAPI: {method} {path}")
    for method, path in sorted(spec_routes - go_routes):
        add(f"documented in OpenAPI but not registered in internal/app/routes.go: {method} {path}")

    schemas = spec.get("components", {}).get("schemas", {})
    schema_count = len(schemas) if isinstance(schemas, dict) else 0

    if errors:
        print("OpenAPI contract check FAILED:", file=sys.stderr)
        for message in errors:
            print(f"  - {message}", file=sys.stderr)
        return 1

    print("OpenAPI contract OK")
    print(f"  file:       {SPEC_PATH.relative_to(ROOT)}")
    print(f"  version:    {version}")
    print(f"  paths:      {len(spec['paths'])}")
    print(f"  operations: {len(operations)}")
    print(f"  schemas:    {schema_count}")
    print(f"  routes:     {len(go_routes)} (matched against internal/app/routes.go)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
