# Sprint 7: Single-Pass Bespoke UI Engine (v0 / DesainPakeAI Standard)

## Step 1: System Prompt Engineering (v0-Grade Tokens & Anti-Slop)

- Engineered a single unified prompt (`internal/modules/uidesign/service/ui_synthesis_prompt.go`) combining UX architecture, responsive design tokens, and self-contained static HTML implementation.
- Emphasized strict prompt fidelity: user request, content, domain, language, device, theme, and explicit foundation take precedence over canned aesthetic archetypes.
- Integrated anti-slop visual rules directly into prompt guidelines:
  - No generic purple-indigo gradients or full-page blurred glow blobs.
  - Realistic human copywriting in the prompt's native language; no placeholder buzzwords.
  - Bento grids and asymmetric layouts applied purposefully when appropriate.
  - Scoped design tokens and typography pairings (Plus Jakarta Sans, Instrument Serif, DM Sans, Space Grotesk).

## Step 2: Single-Pass Synthesis in `ui_synthesis.go`

- Replaced the two-stage pipeline (Stage 1 JSON blueprint + Stage 2 HTML craft) with a direct, single-pass generation.
- Eliminated `craftBespokeHTMLStream`, `bespokeHTMLCraftPrompt`, and `extractStage2HTML`.
- The LLM now generates the complete `raw_html` alongside sections, theme, and design metadata in a single JSON payload.
- Bounded repair loop: if the output is malformed or missing HTML, exactly one repair attempt is made (`synthesis output invalid after one repair`).
- Removes one mandatory model round-trip on successful Go compiler requests. Actual latency, token usage, and visual quality have not been benchmarked; no percentage improvement is claimed.
- Streaming forwards the initial JSON deltas unchanged. A repair is non-streaming, as before; consumers must treat the final result as authoritative rather than parse partial deltas as HTML.
- Repair receives the validation error and previous output. Cancellation is checked before and after each model call; the current LLM interface cannot cancel an in-flight HTTP request through this context.

## Step 3: Frontend Contract Alignment & Offline Verification

- Verified contract alignment with `UiFrameNode.vue` and `ArtifactPreview.vue`:
  - `raw_html` is populated directly in frame node data.
  - `code_export["html"]` and `code_export["vue"]` mirror the canonical implementation.
  - Existing theme metadata remains available; tests verify requested mode and accent. Frontend source was inspected read-only; no browser rendering test was performed.
- Added comprehensive unit tests in `internal/modules/uidesign/service/ui_single_pass_test.go`:
  - `TestSinglePassSynthesisCompleteOutput`: verified single-call completion and JSON stream forwarding.
  - `TestSinglePassSynthesisRepair`: verified bounded repair on missing HTML and malformed JSON.
  - `TestSinglePassSynthesisCancellationAndProviderError`: verified context cancellation and failure propagation.
  - `TestSinglePassCompileFrontendContract`: verified device viewports and serialization parity for Vue Flow consumers.
- All verification commands (`make lint-spec`, `make verify`, `make test-race`, `git diff --check`) pass offline.

## Scope and next review

- Changes apply to the Go UI compiler, including callers that use it. The separate Python orchestration pipeline is unchanged.
- No canned HTML template, frontend edit, deployment, container restart, database migration, commit, or push is introduced.
- Existing HTML sanitization and compiler post-processing are retained, including their layout risks. Non-destructive sanitization belongs to Sprint 8; visual benchmark cases belong to Sprint 9.
- v0 and DesainPakeAI are quality references, not verified descriptions of those products' internal architectures. Mock tests establish call counts and compatibility, not equivalent design quality.
