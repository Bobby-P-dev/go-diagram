# Meta Model API (Muse Spark)

The Go client and Python worker use the existing OpenAI-compatible transport.
Keep `AI_PROVIDER=openai`; no new service or API route is required.

## Configuration

Set these values in the existing local `.env` (never commit the real key):

```dotenv
AI_PROVIDER=openai
OPEN_AI_BASE_URL=https://api.meta.ai/v1
OPEN_AI_MODEL=muse-spark-1.3
MODEL_API_KEY=replace-with-your-meta-key
AI_MAX_TOKENS=12000
```

Keep the application's database, Redis, internal service key, and other settings.
For production, put these values in the environment file used by your deployment.
Changing source or environment files does not update already-running containers.
Rebuild/recreate them only during your planned deployment; these checks do not do so.

Clear or remove old provider values before switching. Go, Python, and Compose
resolve keys in this order, taking the first nonempty value:

1. `OPENAI_API_KEY`
2. `OPEN_AI_API_KEY`
3. `AI_API_KEY`
4. `MODEL_API_KEY`
5. `META_API_KEY`

Model precedence is `OPENAI_MODEL`, `OPEN_AI_MODEL`, `AI_MODEL`, `META_MODEL`.
Base URL precedence is `OPENAI_BASE_URL`, `OPEN_AI_BASE_URL`, `AI_BASE_URL`,
`META_BASE_URL`. Compose passes the resolved configuration to the backend,
orchestrator, and worker so they do not use different providers accidentally.
Other OpenAI-compatible providers still work by changing the same settings.

## JSON output and repair

The existing Go JSON calls use `response_format: {"type":"json_object"}`;
text calls omit `response_format`. This is JSON mode, not schema-constrained
structured output. Domain validators and the existing bounded UI synthesis and
implementation repair loops remain in place. This change does not add tool calls
or `json_schema` output contracts.

SSE content deltas and standard JSON responses are supported. Mock tests cover
the request path, bearer authentication, model, history, token budget, streaming
callbacks, markdown cleanup, and non-retryable provider errors.

Meta reasoning tokens count toward the output budget. `AI_MAX_TOKENS` configures
the Go client, not every CrewAI agent: Python agents keep their existing individual
budgets and reasoning effort settings. Adjust those separately if live checks
show truncated or empty visible output. The example budget is a starting point,
not a guarantee for all generated designs.

## Offline verification

```sh
go test ./internal/platform/config ./internal/platform/llm ./internal/modules/uidesign/service -count=1
ai-orchestration/.venv/bin/python -m unittest discover -s ai-orchestration/tests
make lint-spec
make verify
make test-race
```

Configuration tests isolate environment variables and disable dotenv loading.
Transport tests use a local mock server and dummy credentials. Existing service
tests exercise actual bounded repair behavior with a fake LLM. These checks do
not establish live Meta compatibility, account access, latency, rate limits, or
output quality. Verify those later with your own key and an explicitly authorized
small live request.

## Provider references

- https://dev.meta.ai/docs/api-reference/chat-completions/schemas
- https://dev.meta.ai/docs/structured-output
- https://dev.meta.ai/docs/reasoning
