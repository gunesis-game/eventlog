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

On 2026-11-27, on windows/amd64 with Go 1.27.4 (local machine), against the working tree committed as the package-layout restructure (D-053):

- `go test -count=2 -cpu 1,4 ./...`, `go test -race -count=1 ./...`, `go vet ./...`, `gofmt -l .`, and the benchmark smoke set passed in every package.
- `GOOS=linux go vet ./...` and Linux test-binary builds of every package passed (cross-compiled; not executed on Linux locally).
- `go doc -all .` output is identical to the previous commit, so the public API is unchanged.

The previous implementation commit `69dbed5aa068f58f4199eaf95cb61e240030a954` passed Go verification run 45985063315 on Linux and Windows. The restructure commit has no CI result yet. The commands run in [Go verification](../../.github/workflows/go.yml) on pushes to `main` and on manual dispatch, as Linux and Windows matrix jobs. Do not claim CI results until the run for the exact commit SHA has been inspected.

Update this document when commands, prerequisites, suite locations, or CI triggers change. Do not claim tests or checks that were not actually run.
