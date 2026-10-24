# Contributing to flaky

Thank you for contributing to `flaky`. This document outlines guidelines for local development, testing, style standards, and pull request conventions.

---

## 1. Prerequisites

Ensure you have the following installed locally:
- **Go**: 1.23 or newer (`go version`)
- **Git**: 2.30 or newer (`git --version`)
- **golangci-lint**: v1.60 or newer (`golangci-lint --version`)
- **GoReleaser**: v2 or newer (`goreleaser --version`)
- **actionlint**: optional for workflow linting (`actionlint --version`)

---

## 2. Local Development Workflow

### Building the Binary
To build the CLI executable:
```bash
go build -o flaky ./cmd/flaky
```

### Running Tests
Run unit and integration tests across all packages:
```bash
go test -v ./...
```

Run tests with Go's race detector enabled:
```bash
go test -race ./...
```

Inspect statement coverage:
```bash
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
```

### Running Linters and Formatting
Verify formatting and static analysis:
```bash
# Check formatting
test -z "$(gofmt -l .)"

# Run go vet
go vet ./...

# Run static analysis
golangci-lint run
```

### Validating Release Configurations & Workflows
Ensure GoReleaser and GitHub Action configurations remain valid:
```bash
goreleaser check
goreleaser build --snapshot --clean
actionlint .github/workflows/*.yml
```

---

## 3. Engineering Guidelines

1. **Stdlib-Only Policy**:
   - `flaky` uses only the Go standard library. Do not add external Go module dependencies without explicit project owner consensus.
2. **Defensive Error Handling**:
   - Never ignore error return values silently. Explicitly check errors or assign `_` with clear intent.
   - Wrap contextual errors using `fmt.Errorf("...: %w", err)`.
3. **Isolation & Safety**:
   - Test commands and bisect operations must never mutate the host repository.
   - Ephemeral workspaces and child processes must always be safely cleaned up, including on cancellation or signal interrupts.
4. **Agent Harness Hygiene**:
   - Do not commit agent-specific files (`CLAUDE.md`, `AGENTS.md`, `.claude/`, `.codex/`, `superpowers/`, etc.) into git history. Maintain local ignores in `.git/info/exclude`.

---

## 4. Git Commit & Pull Request Conventions

- We adhere to the **Conventional Commits** specification:
  - `feat:` new features or capabilities
  - `fix:` bug fixes or error handling corrections
  - `test:` test suite additions or modifications
  - `docs:` documentation updates
  - `ci:` GitHub Actions and CI/CD changes
  - `chore:` maintenance or configuration updates
- Keep commits atomic and logically separated.
- Ensure all tests and linters pass before opening a pull request.
