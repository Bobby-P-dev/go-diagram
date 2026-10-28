# Sprint 9: Offline Generation Pipeline Regression Benchmarks

## Scope and Status

Backend offline verification completed on 2026-10-28. These are service-level
regression tests using a mock LLM and an in-memory project gateway, not HTTP,
live-provider, PostgreSQL, browser-rendering, or production acceptance tests.
The authored payloads below are test fixtures only; they are not production
templates and do not demonstrate that an LLM will generate the same designs.

## Step 1: Diverse Domain Benchmark Test Suite

Implemented a comprehensive suite of offline benchmark tests covering realistic, multi-domain visual UI requirements (`internal/modules/uidesign/service/ui_benchmark_test.go`):

1. **Benchmark 1: Artisanal Bakery / Culinary Catalog (Mobile Viewport)**
   - **Scenario**: E-commerce / Food & Beverage catalog (`Katalog Toko Roti & Kue Tradisional Oma`).
   - **Device & Dimensions**: `mobile` viewport (375x812px).
   - **Typography & Foundation**: `calcom` warm editorial palette, Playfair Display heading paired with Plus Jakarta Sans body.
   - **Verification**: Fixture HTML, including its Indonesian copy, prices, and semantic `<footer>`, survives the pipeline unchanged. This checks preservation, not generated copy quality or visual aesthetics.

2. **Benchmark 2: SaaS Telemetry & Cluster Health Dashboard (Web Viewport)**
   - **Scenario**: Developer infrastructure & cloud monitoring (`SaaS Cloud Telemetry Dashboard`).
   - **Device & Dimensions**: `web` viewport (1024x720px).
   - **Layout & Visuals**: Asymmetric Bento Grid (12-column span: 8+4), dark obsidian surfaces (`#090A0F`), inline `<svg>` area sparkline with gradient fill, `font-mono` tabular numbers (JetBrains Mono).
   - **Verification**: Non-destructive SVG rendering, dark theme preservation, monospace telemetry tags.

3. **Benchmark 3: Minimalist Authentication Screen (Form Viewport)**
   - **Scenario**: Single focused login view (`buat halaman login minimalis`).
   - **Foundation**: `ramp` clean precision minimalism.
   - **Preservation Verification**: The mock returns a focused auth section without marketing content. The pipeline must retain the fixture's section count and HTML; this does not prove that a live model avoids hallucination.

4. **Benchmark 4: Contemporary Architecture & Studio Portfolio (Desktop Viewport)**
   - **Scenario**: Creative studio and spatial design showcase (`Atelier Studio Arsitektur`).
   - **Device & Dimensions**: `desktop` viewport (1100x740px).
   - **Visuals**: Asymmetric Swiss grid, monochrome high-contrast dark theme, project docket indexing (`01`, `02`), Space Grotesk display typography paired with DM Sans.
   - **Verification**: Semantic `<footer>` preserved, desktop dimensions verified.

## Step 2: Service Pipeline, Streaming, Repair, and Persistence

Each of the four scenarios runs through both `GenerateUIDesign` and
`GenerateUIDesignStream` (8 successful-path subtests). Assertions verify:

- Valid output uses exactly one mock JSON or streaming call and receives the user's prompt.
- Streaming stages are `analyzing` then `compiling`, progress increases, and exactly one final `complete` event carries the returned response.
- A single identified `ui_frame` preserves fixture HTML, theme tokens, section IDs and targeting, and device dimensions.
- Preview HTML, exported HTML, canonical HTML, and canonical Vue/exported Vue stay consistent.
- The in-memory project belongs to the expected user, its graph matches the response, and version 1 snapshots the same nodes and edges.
- User and assistant messages are persisted, structural validation does not fail, and visual review remains honestly `not_evaluated`.

Five additional streaming subtests cover malformed JSON, missing HTML, provider
failure, pre-canceled context, and successful repair. They assert bounded call
counts, error propagation, no completion or persisted artifacts on these failed
generation paths, and normal persistence after successful repair. They do not
simulate database-write failures or rollback of partial persistence.

`BenchmarkUIDesignOfflinePipeline` also measures local processing time and
allocations for each scenario. It excludes real inference, network, database,
and rendering costs; a one-iteration smoke run is not a performance baseline.

## Step 3: Verification Suite

- All benchmark scenarios use mocks and require no API key or live provider.
- Verification commands executed and passed:
  - `make lint-spec`: 38/38 OpenAPI operations synchronized.
  - `make verify`: `go vet` and all package unit tests passed.
  - `make test-race`: Race detector completely clean across all internal modules.
  - `git diff --check`: 0 whitespace issues.

Focused reproduction commands:

```sh
go test ./internal/modules/uidesign/service -run '^TestBenchmarkUIDesign' -v -count=1
go test ./internal/modules/uidesign/service -run '^$' -bench '^BenchmarkUIDesignOfflinePipeline$' -benchtime=1x -benchmem
```

## Remaining Acceptance Checks

- Run the same prompts against the configured real provider with credentials supplied locally; inspect prompt adherence, repair behavior, and actual latency.
- Render outputs in the frontend at mobile and desktop sizes; inspect overflow, typography, accessibility, and preview/export parity visually.
- Verify HTTP streaming and durable project/version/message storage with the running application and database before deployment.
- Do not treat passing offline tests as proof of visual parity with DesainPakeAI or v0, or as production readiness.

Sprint Markdown documentation stays under `docs/architecture/`.
