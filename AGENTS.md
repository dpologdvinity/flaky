# AGENTS.md

Canonical instructions for coding agents (Claude Code, Codex, others) working in this repo.
`CLAUDE.md` only imports this file.

## Project

`flaky` is an offline, single-binary Go CLI that reruns a test command, ranks intermittent
JUnit failures with Wilson intervals, and bisects first-parent history to find where a test
became flaky.

- `cmd/flaky/main.go` — thin entry point; wires signals and calls `flaky.Main`.
- `internal/flaky/` — the single package holding all logic (CLI, workspaces, runner, JUnit,
  stats, git/bisect, render). `process_unix.go` / `process_windows.go` are build-tagged.
- `docs/SPEC.md` — the behavioral contract (CLI, JSON output, exit codes). Change it together
  with behavior. `ARCHITECTURE.md` describes component boundaries.

## Commands

```bash
go build -o flaky ./cmd/flaky        # build
go test -race ./...                  # tests (CI adds -v and coverage)
test -z "$(gofmt -l .)"              # format check
go vet ./...
golangci-lint run                    # CI pins v1.61.0
goreleaser check                     # release config
goreleaser release --snapshot --clean  # local snapshot incl. .deb packages
actionlint .github/workflows/*.yml
shellcheck install.sh
lefthook install                     # once per clone: pre-commit runs gitleaks, gofmt, golangci-lint, typos
```

Run tests, vet and lint before declaring Go changes done.

## Conventions

- **Stdlib only.** No external Go module dependencies without owner sign-off.
- **Never touch the host repo.** Scans run in temporary copies; bisect uses detached temporary
  worktrees. Child processes and temp paths must be cleaned up on cancellation.
- **Tests never run real projects.** They use fake scripts that write JUnit, `t.TempDir()`
  fixtures, and `GIT_CONFIG_GLOBAL=/dev/null` for git fixtures.
- Check every error; wrap with `fmt.Errorf("...: %w", err)`.
- Bug fixes start with a failing regression test.

## Commits and releases

- Conventional Commits (`feat:`, `fix:`, `test:`, `docs:`, `ci:`, `chore:`), small and atomic.
- Pushing a `v*` tag runs `.github/workflows/release.yml`: GoReleaser builds archives and `.deb`
  packages, attests and verifies them, then updates the Homebrew tap. Published tags are immutable.
- Required env vars: none for development. Releases use the `HOMEBREW_TAP_GITHUB_TOKEN` repo
  secret (the tap step is skipped without it).

## Agent setup

- `.claude/settings.json` — project-scoped Claude Code plugins: `gopls-lsp` (needs `gopls` on
  `PATH`), `commit-commands`, and `ponytail` (minimal-diff mode; marketplace declared inline).
  Personal overrides go in the git-ignored `.claude/settings.local.json`.
- `.codex/config.toml` — enables the same `ponytail` plugin for Codex.
- `.agents/skills/` — repo skills, each with Codex `agents/openai.yaml` metadata:
  `surgical-patch` (narrow bug fixes with regression proof),
  `verify-and-stop` (validation without scope creep). `.claude/skills/*` are relative symlinks
  to these, so both agents share one copy.
- Workflow: v0.1 was specified in `docs/SPEC.md`, planned as TDD checkbox tasks in
  `docs/PLAN.md` (superpowers `executing-plans` format), then executed task by task with one
  commit per task.
