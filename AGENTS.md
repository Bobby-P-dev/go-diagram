# Project instructions for Codex

This repository is the Go backend of the Visual Workspace project. Related frontend, AI orchestration, agent definitions, and skills are in the shared project directory one level above this repository. Inspect the current code before relying on historical paths or framework descriptions in the role definitions.

## Project subagents

Use the project agents in `.codex/agents/` when delegation is useful for the user's task. Delegate focused work with explicit scope and file ownership. Small, straightforward changes can be completed by the primary agent. Wait for delegated results, review their changes, and report consolidated validation to the user.

| Agent | Use for | Relevant shared skills |
| --- | --- | --- |
| `architect` | System boundaries, architecture, database schema, and API contracts | `backend-development`, `frontend-development` |
| `backend` | Go services, HTTP routes, PostgreSQL, and snapshots | `backend-development` |
| `frontend` | Vue components, stores, canvas interactions, and responsive UI | `frontend-development`, `canvas-ui-builder` |
| `ai-engineer` | LLM integrations, prompts, structured output, and targeted node mutations | `ai-engineering`, `diagram-generation`, `ui-generation` |
| `qa` | Relevant regression checks, API behavior, database consistency, and canvas/export validation | Skills appropriate to the code under test |
| `ui-reviewer` | Visual quality, accessibility, design foundations, and generated UI review | `ui-review`, `ui-ux-pro-max` |

Read the relevant `SKILL.md` under `../.agents/skills/` before specialized work. All agents inherit the current session's model, reasoning effort, tools, and permissions; roles do not grant access to sibling repositories. Respect available permissions when a task touches another project directory.

Keep diagram and UI Design domain logic modular. Preserve unrelated changes. Coordinate edits so two agents do not modify the same file concurrently. Use validation appropriate to the actual change and report any checks that could not run.

## Maintaining agent definitions

The six `.codex/agents/*.toml` files preserve the roles from `../.agents/agents/*.md` and add Codex context. The original Markdown definitions are retained for Antigravity. Changes to either format are not automatically synchronized; update the matching definition when the role changes.

Example request: "Use architect to check the API contract, backend to implement the change, and qa to verify the affected behavior."
