# Contributing to Monstera

Thanks for your interest in contributing to Monstera! This guide covers the essentials for getting started.

## Development setup

### Prerequisites

- **Go 1.26+**
- **Node.js 20+** and npm (for the Next.js UI)
- **Docker** and **Docker Compose**

### Getting started

Run everything in Docker:

```bash
make start
```

Or, for local development (dependencies in Docker, server and UI running natively):

```bash
make start-dev
```

The API server runs on `http://localhost:8080` and the UI on `http://localhost:3000`.

## Running tests and linting

```bash
make test             # Unit tests
make test-integration # Integration tests (requires dependencies running)
make lint             # golangci-lint
make lint-fix         # Auto-fix lint issues
go fmt ./...          # Format code
```

All tests must pass and the linter must be clean before submitting a PR.

## Submitting changes

1. Fork the repo and create a feature branch from `main`.
2. Make your changes, ensuring tests pass and the linter is clean.
3. Write clear commit messages that explain *why*, not just *what*.
4. Open a pull request against `main` with a description of the change and how to test it.
5. Changes should be targetted in scope. Avoid sprawling PRs that are difficult to review.

For the sake of reviewer sanity, low-effort or poorly documented changes may be rejected without review.

## Project structure

See the [documentation index](docs/README.md) for architecture docs, and the [tech stack](docs/tech_stack.md) for a full list of technologies and libraries used.

## A note on AI contributions

AI-contributions are permitted. That said, they are subject to all the expectations described above.