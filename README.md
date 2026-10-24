# flaky

`flaky` repeats a test command, reads its JUnit XML, and ranks intermittent failures. It runs offline as one Go binary. The table shows failure rate, observed pass/fail counts, missing cases, and output from a failing and passing run.

```sh
flaky -n 20 -- pytest tests/
flaky -n 30 --parallel 4 -- pytest tests/
flaky bisect pkg::test_a --runs 30 -- pytest tests/
```

Commands run in disposable copies of the current directory; parallel workers never share a copy. Bisect requires a clean git tree and samples first-parent history in detached temporary worktrees. `flaky` does not reset or check out your branch. The command itself can still write outside its disposable cwd if you give it absolute paths.

## Install

```sh
go install github.com/dpologdvinity/flaky/cmd/flaky@latest
```

Until a release is published, build from this checkout with `go build -o flaky ./cmd/flaky`. Release archives are configured for Linux, macOS, and Windows with GoReleaser.

## JUnit setup

`flaky` passes a unique absolute report path as `FLAKY_JUNIT_XML` on every run. Use `--junit RELPATH` when your command writes a fixed relative file. It must stay within the disposable workspace.

| Runner | Example | How `flaky` finds XML |
|---|---|---|
| pytest | `flaky -- pytest tests/` | Appends `--junitxml=PATH`. |
| gotestsum | `flaky -- gotestsum -- ./...` | Inserts `--junitfile=PATH` before gotestsum's `--`. |
| Jest + jest-junit | `flaky -- jest` | Sets `JEST_JUNIT_OUTPUT_DIR` and `JEST_JUNIT_OUTPUT_NAME`; install and enable the `jest-junit` reporter in the project. |
| cargo-nextest | `flaky --junit target/nextest/default/junit.xml -- cargo nextest run` | Configure `[profile.default.junit] path = "junit.xml"` in `.config/nextest.toml`; nextest writes under `target/nextest/default/`. |
| Other JUnit runner | `flaky --junit reports/junit.xml -- ./test-script` | Reads the given relative path. |

A failing command is normal if valid XML exists. A missing or malformed report is an error. Test IDs use `classname::name`, falling back to `file::name` or `name`. Use the ID printed by a scan for `bisect`.

## Reading results

`failure rate = failures / (passes + failures)`. Skips and cases absent from a run do not enter the denominator. A test is flaky only if both a pass and a failure were seen. Flaky tests sort by failure rate first; consistently failing tests follow them. `--json` emits the stable fields described in [the spec](docs/SPEC.md).

Bisect runs each sampled commit `--runs` times (30 by default). A two-sided 90% Wilson interval classifies a commit stable if its upper failure bound is below 10%, flaky if its lower bound is above 10% and both pass/fail were observed, or inconclusive otherwise. Inconclusive sampling exits 3 instead of claiming an origin. This threshold can miss rare flakes; more runs improve power. Bisect searches first-parent history and assumes a stable-to-flaky transition.

Exit codes: 0 complete, 1 test ID absent during bisect, 2 usage or execution error, 3 inconclusive bisect, 130 interrupted. See [the spec](docs/SPEC.md) for exact output and edge cases.

## How it differs

`pytest-flakefinder` repeats pytest tests to expose flakiness. `flaky` consumes JUnit XML across runners, ranks per-test rates, shows differing pass/fail output, and searches git history with repeated statistical samples. CI services such as BuildPulse and Trunk Flaky Tests collect and analyze CI results over time; `flaky` works locally and offline with no account or service. It does not provide dashboards, retention, or CI aggregation.

## Name availability

Checked 2026-10-23: `github.com/dpologdvinity/flaky` returned 404; Homebrew core had no `flaky` formula at the checked path. The npm package `flaky` **already exists** (v0.0.9, unrelated UUID package), so this project does not claim that npm name or publish an npm wrapper. Availability can change.

MIT © 2026 Kaitlyn Bassford.
