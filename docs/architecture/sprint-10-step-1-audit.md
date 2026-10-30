# Sprint 10 Step 1: Deep Read-Only Architectural Audit & Migration Mapping

**Date**: 2026-10-28  
**Scope**: Read-only inspection of Go Modular Monolith backend, Vue frontend (`frontend-diagram`), and React starter (`frontend-react`).  
**Objective**: Identify reusable assets, data models, limitations, and architectural gaps against the Connected Multi-Artifact Product Workspace vision.

---

## 1. Executive Summary

The existing system possesses mature individual generation engines:
1. **Diagram Engine**: Flowchart, ERD, and architecture diagrams via `internal/modules/diagram`, supporting targeted node mutations (`targeted_node_ids`), version snapshotting, and rollback.
2. **UI Design Engine**: Single-pass prompt engine with anti-slop visual tokens, AST tokenizer sanitizer, and surgical localized patching via `TargetedPatcher`.
3. **Identity & Sharing**: Token-based access credentials with fine-grained permissions, share tokens, and fork capabilities.

However, the current system was designed around a **1 Project = 1 Canvas / 1 Mode** paradigm. In the user's vision of an **End-to-End Connected Product Workspace** (Brainstorming → PRD → Architecture/ERD/Flowchart → Wireframe → UI Design), a Project must become a collaborative container housing **multiple conversations and multiple interlinked artifacts**.

---

## 2. Current Backend Architecture & Data Models

### 2.1 Database Schema (`internal/platform/database/init.sql`)
- **`projects` Table**:
  - Columns: `id`, `user_id`, `title`, `diagram_type`, `project_mode` (`"diagram"` | `"ui_design"`), `is_pinned`, `metadata`, `current_nodes`, `current_edges`, `created_at`, `updated_at`.
  - *Limitation*: The project itself holds only a single set of `current_nodes` and `current_edges`. It cannot currently hold both an ERD diagram and a UI frame simultaneously without treating them as one shared canvas or overwriting each other.
- **`chat_messages` Table**:
  - Columns: `id`, `project_id`, `role`, `content`, `target_node_ids`, `metadata`, `created_at`.
  - *Limitation*: All messages attach directly to `project_id`. There is no concept of multiple chat threads (e.g., "PRD Brainstorm Chat", "ERD Modeling Chat", "UI Polish Chat") within the same project.
- **`diagram_versions` Table**:
  - Columns: `id`, `project_id`, `version_number`, `change_summary`, `nodes`, `edges`, `trigger_message_id`, `created_at`.
  - *Limitation*: Snapshots are project-wide rather than artifact-scoped.
- **`access_credentials` Table**:
  - Token-based credentials (`credential_key`), role-based access control (`user` / `admin`), and generation capabilities (`can_generate_diagram`, `can_generate_ui`). Already isolates projects by `user_id`.

### 2.2 Reusable Backend Subsystems (Gold Assets)
- **`TargetedPatcher` & `PatchEngine` (`internal/modules/uidesign/service/`)**:
  - Capable of surgical localized mutations on specific node/component IDs without rewriting the entire canvas or losing state.
- **Version Snapshotting & Rollback (`internal/modules/diagram/service/versions.go`)**:
  - Predictable sequential versioning and rollback routines.
- **SSE Streaming Pipeline (`internal/modules/uidesign/service/generation_service.go`)**:
  - Well-tested stage events (`status: analyzing`, `status: compiling`, `progress`, `complete`).

---

## 3. Frontend Audit: Vue (`frontend-diagram`) vs React Starter (`frontend-react`)

### 3.1 Vue Application (`frontend-diagram`)
- **State Management**: `diagramStore.js` (Pinia) coordinates nodes, edges, selection, active project, and version history.
- **API Client**: `useDiagramApi.js` provides 30+ endpoints covering diagram CRUD, UI design streaming, async jobs, comments, and sharing.
- **Visual Engines**:
  - `@vue-flow/core` with custom node types: `input`, `output`, `decision`, `database` (ERD with columns), `actor`, `usecase`, `ui_frame`.
  - Dagre layout engine for automated graph topology.
  - `useCanvasExport.js` for PNG 2x retina and TSX export.
- *Limitation*: Heavily coupled to a single canvas state. Navigating between project modes replaces the active graph.

### 3.2 React Application (`frontend-react`)
- **Installed Stack**:
  - React 19 + TypeScript (Strict).
  - `@xyflow/react` v12 (Direct standard successor to Vue Flow, with identical concepts of nodes, edges, handles, and custom node renderers).
  - Tailwind CSS v3 + Radix UI headless primitives + Lucide React + Sonner toasts.
  - Agent Skills: `antislop`, `antislop-ui`, `antislop-layoutmobile`, `impeccable`, `react-best-practices`, `composition-patterns`.
- **Validation**: Zero lint errors (`oxlint`), clean production build (`tsc -b && vite build`), 0 npm vulnerabilities.

---

## 4. Architectural Gaps & Required Evolution

| Dimension | Current State | Required Target State (Sprint 10–17) |
| :--- | :--- | :--- |
| **Project Boundary** | 1 Project = 1 Mode, 1 Canvas | 1 Project = Container of multiple Chats & multiple Artifacts |
| **Conversations** | 1 flat message stream per project | Multiple threads per project (`stage_mode`, `active_artifact_id`) |
| **Artifact Types** | Nodes & Edges JSON only | Rich Artifacts: Document (PRD), Graph (ERD, Flowchart), Frame (Wireframe, UI Design) |
| **Context Memory** | Chat history only | Explicit Artifact Graph (Child artifact declares parent context IDs) |
| **Revisions** | Global prompt or node-target prompt | Multi-tier: Targeted prompt, Canvas drag/edit, Opt-in cross-artifact sync |
| **Sync Strategy** | None (Independent) | Change notification banner: *"PRD updated to v2. Sync ERD?"* |

---

## 5. Decision on Supplemental Libraries

- **Recommendation**: Defer installing `motion` (Framer Motion) and `shiki` (Syntax Highlighter) until their respective feature sprints (Sprint 11 and Sprint 13/14).
- **Rationale**:
  - Keeping the package bundle minimal during Sprint 10 ensures clean diffs and zero unneeded runtime overhead.
  - The current stack (`@xyflow/react`, `radix-ui`, `lucide-react`, `tailwindcss`, `sonner`) already provides all necessary building blocks for the multi-chat and workspace navigation shell.

---

## 6. Step 1 Sign-Off & Next Step (Sprint 10 Step 2)

Step 1 audit is complete. All existing assets have been cataloged with zero destructive modifications.  
**Ready for Step 2**: Design the Database Schema & API Contracts for `Project`, `ChatThread`, `Artifact`, and `ContextLink`.
