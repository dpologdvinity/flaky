# flaky v0.1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build an offline single-binary CLI that ranks intermittent JUnit failures and bisects their first-parent introduction statistically.

**Architecture:** A stdlib CLI parses arguments, runs commands in temporary copies, parses JUnit, aggregates observations, and renders stable text/JSON. Bisect samples detached temporary worktrees using the same runner and Wilson classifier.

**Tech Stack:** Go 1.23+, stdlib only, Git CLI for bisect, GoReleaser and GitHub Actions for distribution.

**Spec:** `docs/SPEC.md`.

## Global Constraints

- Module: `github.com/dpologdvinity/flaky`; license: MIT © 2026 Kaitlyn Bassford.
- No external Go dependencies; check proxy.golang.org before pinning any new one.
- All test commands in tests are fake scripts writing JUnit; never run a real project.
- Bisect fixtures use `t.TempDir()`, fixed author/dates, and `GIT_CONFIG_GLOBAL=/dev/null`.
- Never modify the user's repo; clean worktrees and child processes on cancellation.
- Conventional commits, one per task; never push or tag.

## Review Focus

- Malformed and duplicate JUnit IDs: parser must fail, not silently count them.
- Missing cases across runs: denominator must omit them and output must show missing.
- Symlink in source tree: workspace copy must preserve it without following outside source.
- Interrupted child process: cleanup must wait for the process and remove temp paths.
- Inconclusive Wilson result: bisect must exit 3 without claiming a first commit.

---

### Task 1: Repository contract
**Files:** `go.mod`, `LICENSE`, `.git/info/exclude`, `docs/SPEC.md`, `docs/PLAN.md`.
**Interfaces:** Consumes none. Produces module `github.com/dpologdvinity/flaky` and fixed source contract.
- [ ] Verify branch is `main`, source repo is empty, and required local excludes exist.
- [ ] Add module, license, spec, and plan; run `go mod tidy` and inspect `git status`.
- [ ] Commit `docs: define flaky v0.1 contract and plan`.

### Task 2: CLI argument contract
**Files:** `cmd/flaky/main.go`, `internal/flaky/cli.go`, `internal/flaky/cli_test.go`.
**Interfaces:** Produces `func Main(ctx context.Context, args []string, out, errOut io.Writer) int` and `Options` with mode, counts, paths, command, test ID.
- [ ] Write table test for scan, bisect, help, bad counts and missing `--`.
- [ ] Run `go test ./internal/flaky -run TestCLI`, expect failure.
- [ ] Implement `Main` parsing and dispatch with usage errors; run focused test, expect pass.
- [ ] Commit `feat: define CLI and exit contract`.

### Task 3: JUnit parser
**Files:** `internal/flaky/junit.go`, `internal/flaky/junit_test.go`.
**Interfaces:** Produces `func ParseJUnit(r io.Reader) ([]Case, error)` and `Case{ID, Status, Message, Output string}`.
- [ ] Test root variants, classname/file identity, skip, failure/error, malformed XML and duplicate IDs.
- [ ] Run `go test ./internal/flaky -run TestParseJUnit`, expect failure.
- [ ] Implement streaming XML parse with `encoding/xml`; run focused test, expect pass.
- [ ] Commit `feat: parse JUnit test cases`.

### Task 4: Statistics and ranking
**Files:** `internal/flaky/stats.go`, `internal/flaky/stats_test.go`.
**Interfaces:** Produces `func Summarize(runs [][]Case) []Summary`, `func Wilson(f, n int) (float64,float64)`, `func Classify(f,p,n int) string`.
- [ ] Test mixed, fail-only, missing, skipped-only, ordering, and 0/30 Wilson stable.
- [ ] Run focused tests, expect failure.
- [ ] Implement aggregation and formulas from spec; run focused tests, expect pass.
- [ ] Commit `feat: rank tests and classify failure rates`.

### Task 5: Temporary workspace copies
**Files:** `internal/flaky/workspace.go`, `internal/flaky/workspace_test.go`.
**Interfaces:** Produces `func CopyWorkspace(src string) (string, func(), error)`; excludes `.git`, preserves symlinks.
- [ ] Test copy contents, symlinks, exclusion and cleanup in `t.TempDir()`.
- [ ] Run focused test, expect failure.
- [ ] Implement recursive copy; run focused test, expect pass.
- [ ] Commit `feat: isolate test workspaces`.

### Task 6: Runner adapters
**Files:** `internal/flaky/runner.go`, `internal/flaky/runner_test.go`.
**Interfaces:** Produces `func Run(ctx context.Context, source string, command []string, junit string) ([]Case,error)`; consumes `CopyWorkspace` and `ParseJUnit`.
- [ ] Test fake command report, nonzero exit with XML, absent report, and pytest/gotestsum argument placement.
- [ ] Run focused tests, expect failure.
- [ ] Implement runner detection, env injection, command execution and cleanup; run focused tests, expect pass.
- [ ] Commit `feat: run commands and locate JUnit reports`.

### Task 7: Parallel scan
**Files:** `internal/flaky/scan.go`, `internal/flaky/scan_test.go`.
**Interfaces:** Produces `func Scan(ctx context.Context, source string, command []string, junit string, n, parallel int) ([]Summary,error)`.
- [ ] Test seeded fake scripts produce deterministic ordering and separate workspaces with `--parallel`.
- [ ] Run focused test, expect failure.
- [ ] Implement bounded worker pool and ordered aggregation; run focused test, expect pass.
- [ ] Commit `feat: scan tests in parallel`.

### Task 8: Scan rendering
**Files:** `internal/flaky/render.go`, `internal/flaky/render_test.go`.
**Interfaces:** Produces `func RenderScan(w io.Writer, command []string, runs int, tests []Summary, jsonMode bool) error`.
- [ ] Test exact table, detail block, JSON null rate, and missing count.
- [ ] Run focused test, expect failure.
- [ ] Implement stable text/JSON; run focused test, expect pass.
- [ ] Commit `feat: render ranked scan reports`.

### Task 9: Bisect git lifecycle
**Files:** `internal/flaky/git.go`, `internal/flaky/git_test.go`.
**Interfaces:** Produces `func History(ctx context.Context, dir string) ([]Commit,error)` and `func WithWorktree(ctx context.Context, dir, hash string, fn func(string) error) error`.
- [ ] Build fixed-date fixture repo; test clean check, first-parent order, temp worktree cleanup on success/error.
- [ ] Run focused test, expect failure.
- [ ] Implement git commands without touching source checkout; run focused test, expect pass.
- [ ] Commit `feat: sample git history in disposable worktrees`.

### Task 10: Bisect algorithm
**Files:** `internal/flaky/bisect.go`, `internal/flaky/bisect_test.go`.
**Interfaces:** Produces `func Bisect(ctx context.Context, dir, id string, command []string, junit string, runs int) (BisectResult,error)`; consumes `History`, `WithWorktree`, `Run`, `Classify`.
- [ ] Test fixed-date fixture history with stable then seeded failing scripts; test absent ID and inconclusive result.
- [ ] Run focused test, expect failure.
- [ ] Implement endpoint checks and cached binary search; run focused test, expect pass.
- [ ] Commit `feat: bisect intermittent test introduction`.

### Task 11: Bisect output and signal behavior
**Files:** `internal/flaky/render.go`, `internal/flaky/cli.go`, `cmd/flaky/main.go`, tests beside them.
**Interfaces:** Produces `func RenderBisect(w io.Writer, result BisectResult, jsonMode bool) error`; wires both modes through `Main`.
- [ ] Test exact bisect text/JSON and cancellation exit 130 with no surviving temp worktree.
- [ ] Run focused tests, expect failure.
- [ ] Implement rendering, dispatch and `signal.NotifyContext`; run focused tests, expect pass.
- [ ] Commit `feat: expose bisect results and cancellation`.

### Task 12: Docs, CI and release
**Files:** `README.md`, `.github/workflows/ci.yml`, `.goreleaser.yaml`.
**Interfaces:** User installation and runner recipes; no Go API.
- [ ] Document fake-command usage, runner setup, statistical limits, name check and comparison with pytest-flakefinder/CI vendors.
- [ ] Add CI with gofmt, vet, test and GoReleaser config; run `gofmt -l .`, `go vet ./...`, `go test ./...`.
- [ ] Commit `docs: add usage and release automation`.

## Self-review

Spec coverage: CLI, parser, stats, isolation, adapters, scan, output, bisect,
cleanup, docs and release map to Tasks 1–12. Type scan: `Case`, `Summary`,
`Commit`, `BisectResult`, `Main`, `Run` and renderer consumers match their
producer task. Review Focus cases are in Tasks 3, 4, 5, 11 and 10. No external
dependency or separate state store is planned. Each task has a focused failure,
implementation, verification and commit step; Task 1 and Task 12 are document
and configuration gates with command checks instead of unit tests.
