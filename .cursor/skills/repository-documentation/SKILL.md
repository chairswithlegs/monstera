---
name: repository-documentation
description: Creates and updates documentation in the repository. Places high-level overview in README.md, ADRs and architecture in docs/architecture, user guides in docs. Uses lowercase filenames except README.md; writes in Markdown. Use when the user asks to create or update documentation.
---

# Repository Documentation

## Where to put docs

| Content | Location | Example |
|--------|----------|---------|
| High-level overview, getting started, repo summary | **README.md** (repo root) | Project intro, install, quick start |
| ADRs, architecture decisions, design docs | **docs/architecture/** | `docs/architecture/01-auth-strategy.md` |
| User guides, how-tos, specs, conventions | **docs/** | `docs/contributing.md`, `docs/api-overview.md` |

## Filenames

- **README.md** is uppercase (only this file).
- All other docs use **lowercase** names: `contributing.md`, `api-overview.md`, `01-auth-strategy.md`.

## Format

- Write all documentation in **Markdown** (`.md`).
- Use clear headings, lists, and code blocks where appropriate.

## Quick decision flow

1. **Overview or “main” doc for the repo?** → Update or create `README.md` at repo root.
2. **Saving or resolving open product/architecture question?** → Update `docs/open_questions.md`
2. **Architecture decision or design rationale?** → `docs/architecture/<filename>.md` (lowercase).
3. **User-facing guide, spec, or reference?** → `docs/<filename>.md` (lowercase).

Create `docs/architecture` if it does not exist when adding architecture or ADR docs.
