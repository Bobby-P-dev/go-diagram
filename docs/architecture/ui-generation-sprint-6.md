# Sprint 6: prompt-driven UI generation

## Step 2: static UI templates removed

- UI generation no longer depends on a template repository, template service,
  template DTO, or a template constructor argument in the composition root.
- Sync, streaming, and async entry points reject nonempty `template_id` with
  HTTP 400 before starting generation, streaming, project creation, or queueing.
  Supplying both a prompt and a template ID is also rejected, not silently ignored.
- The legacy `GET /api/ui-design/templates` route temporarily remains as a
  deprecated compatibility endpoint returning `data: []` without a service or
  database call. The frontend still has callers for this endpoint.
- A blank prompt without a template ID still creates a blank canvas through
  the existing synchronous/streaming workflow.
- The hardcoded section-to-HTML generator is removed. Editing a section-only
  legacy frame without HTML now fails explicitly with instructions to regenerate
  rather than silently inserting a canned layout. Stored generated HTML is preserved.
- Existing database tables and stored projects are not deleted or migrated.
  The old `ui_templates` table is unused by the UI Design runtime.
- Diagram templates, explicit foundation presets, generated Vue `<template>`
  wrappers, security sanitization, and other domains are unchanged by this step.

Tests cover empty-list compatibility, template rejection in all three HTTP
entry points and direct service/dispatcher calls, preservation of generated HTML,
and failure without mutation for section-only legacy frames.

## Step 3: API contract review & deprecation coverage

- **Contract Parity Verified**: All 38 OpenAPI endpoints, 31 paths, and 34 schemas match `internal/app/routes.go` exactly.
- **Deprecation Documented**:
  - `GET /api/ui-design/templates` marked deprecated in `docs/api/openapi.yaml`, returning an empty list (`[]`) with `maxItems: 0`.
  - `template_id` marked deprecated in `CreateUIDesignRequest`, rejecting non-empty strings with HTTP 400 (`ErrStaticUITemplateUnsupported`).
- **Frontend Consumers Checked**:
  - Diagram templates (`/api/templates`) remain 100% functional and separate.
  - Frontend UI Design consumers calling `getUiTemplates()` receive empty lists gracefully without breaking canvas components.
- **Verification Suite**:
  - `make lint-spec`: 38/38 operations passed.
  - `make verify`: All unit tests & vet passed 100%.
  - `make test-race`: Race detector completely clean.

Sprint 6 is complete. Ready to proceed to **Sprint 7: Single-Pass Bespoke UI Engine (v0/Tailwind System)**.
