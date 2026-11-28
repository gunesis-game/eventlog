# Development commands

## Go toolchain

- Module: `github.com/gunesis-game/eventlog`.
- The `go.mod` language directive is `go 1.27`, and its toolchain directive is `go1.27.4`. The project policy is to use the latest stable released Go version, not retain older compatibility floors.
- One external module dependency is declared: `github.com/santhosh-tekuri/jsonschema/v6` (with its indirect dependencies) for embedded JSON Schema validation, an approved exception to the standard-library-only constraint (D-046).

## Commands

| Command | Purpose |
|---|---|
| `go test -count=1 ./...` | Full test suite: unit, integration, recovery, emulator-backed web-storage client tests, and subprocess process-termination tests |
| `go test -count=1 ./internal/<package>/...` | Fast feedback for one internal package; see [testing-strategy.md](testing-strategy.md#suite-locations) for which packages a change affects |
| `bash scripts/go-test-verbose.sh <label> [-race]` | The full suite in verbose mode, as CI runs it; reports passed, failed, and skipped counts with the names of skipped and failed tests as a GitHub Actions notice |
| `go test -race -count=1 ./...` | Full suite under the race detector (requires cgo; CI runs it on Linux) |
| `go vet ./...` | Static analysis |
| `go build ./...` | Build all packages |
| `gofmt -l .` | List files needing formatting; must print nothing (CI checks on Linux) |
| `go test -run '^$' -bench . -benchtime=3x ./...` | Run the benchmark smoke set ([benchmarks.md](benchmarks.md#smoke-suite)); not run in CI |

## CI

| Workflow | Trigger | What it runs |
|---|---|---|
| [Go verification](../../.github/workflows/go.yml) | Push to `main`, manual dispatch | Linux and Windows jobs with Go `1.27.4`: gofmt (Linux), tests through `scripts/go-test-verbose.sh`, race tests (Linux), build, vet. A separate job applies [workflow-run retention](../standards/git-workflow.md#workflow-run-retention-and-scheduling) |
| [Workflow history cleanup](../../.github/workflows/cleanup-workflow-history.yml) | Manual dispatch only | Applies retention to its own history and, when both inputs are given, to the named workflow. The run ID input stands in for the current run if it is not in the fetched list; it is not a request to delete that run |

There is no benchmark workflow, no scheduled workflow, and no `workflow_run` trigger. Each test step's counts and skipped tests appear as a notice on the run's summary page and as check-run annotations, readable without the job log. Do not claim CI results until the run for the exact commit SHA has been inspected.

## Observed results

On 2026-11-27, on windows/amd64 with Go 1.27.4 (local machine), against the working tree committed as the HTTP/2 transport (D-055) with batched Reader reads:

- `go test -count=2 -cpu 1,4 ./...`, `go test -count=1 ./...`, `go test -race -count=1 ./...`, `go vet ./...`, `gofmt -l .`, and the benchmark smoke set passed in every package.
- `GOOS=linux go vet ./...` and Linux test-binary builds of every package passed (cross-compiled; not executed on Linux locally).
- `go doc -all .` output is unchanged, so the public API is unchanged.

CI: commit `9996f247817772d6e2fedcfe14f04e46857df647` (the Google services emulator) passed Go verification run 46081913499 on `ubuntu-latest` (gofmt; tests: passed 202, failed 0, skipped 0; race tests: passed 202, failed 0, skipped 0; build, vet) and `windows-latest` (tests: passed 202, failed 0, skipped 0; build, vet; gofmt and race skipped by design), with no warning annotations. Later commits have not been recorded here.

Update this document when commands, prerequisites, suite locations, or CI triggers change. Do not claim tests or checks that were not actually run.
