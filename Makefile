.PHONY: test verify test-race lint-spec

# Offline suite: no LLM calls, no API key required.
test:
	go test ./... -count=1

# Validate the OpenAPI contract and its parity with the Go router.
lint-spec:
	python3 scripts/check_openapi.py

# Race detector for packages that carry session and async-job state.
test-race:
	go test -race -count=1 -timeout 60s ./internal/modules/identity/... ./internal/modules/sharing/... ./internal/modules/uidesign/... ./internal/modules/workspace/... ./internal/modules/diagram/... ./internal/platform/... ./internal/integration/... ./internal/app/...

verify:
	go vet ./...
	go test ./... -count=1
