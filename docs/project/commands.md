# Development commands

## Go toolchain

- Module: `github.com/gunesis-game/eventlog`.
- The `go.mod` language directive is `go 1.27`, and its toolchain directive is `go1.27.4`. The project policy is to use the latest stable released Go version, not retain older compatibility floors.
- One external module dependency is declared: `github.com/santhosh-tekuri/jsonschema/v6` (with its indirect dependencies) for embedded JSON Schema validation, an approved exception to the standard-library-only constraint (D-046).
- CI explicitly pins Go `1.27.4` on both `ubuntu-latest` (linux/amd64) and `windows-latest` (windows/amd64).

## Commands

| Command | Purpose |
|---|---|
| `go test -count=1 ./...` | Full test suite: unit, integration, recovery, and subprocess process-termination tests |
| `go test -race -count=1 ./...` | Full suite under the race detector (requires cgo; CI runs it on Linux) |
| `go vet ./...` | Static analysis |
| `go build ./...` | Build all packages |
| `gofmt -l .` | List files needing formatting; must print nothing (CI checks on Linux) |
| `go test -run '^$' -bench . -benchtime=3x ./...` | Run the representative benchmark smoke set |

## Observed results

On 2026-11-27, on windows/amd64 with Go 1.27.4 (local machine), against the working tree committed as the Cycle 1 implementation commit:

- `go test -count=1 ./...`, `go test -count=5 ./...`, `go test -race -count=1 ./...`, `go vet ./...`, and `gofmt -l .` passed.
- `GOOS=linux go vet ./...` and a Linux test-binary build passed. These were cross-compiled; tests were not executed on Linux locally.

CI evidence for commit `9f3d6df41c2a10b40debf47181f7ff5da8d45622`: Go verification run 45964555490 succeeded on `ubuntu-latest` (gofmt, `go test`, `go test -race`, build, vet) and `windows-latest` (`go test`, build, vet; gofmt and race steps skipped by design). Steering state validation run 45964555461 succeeded. The commands run in [Go verification](../../.github/workflows/go.yml) on pushes to `main` and on manual dispatch, as Linux and Windows matrix jobs. Do not claim CI results until the run for the exact commit SHA has been inspected.

Update this document when commands, prerequisites, suite locations, or CI triggers change. Do not claim tests or checks that were not actually run.
