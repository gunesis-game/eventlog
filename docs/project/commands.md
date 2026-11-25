# Development commands

## Go toolchain

- Module: `github.com/gunesis-game/eventlog`.
- The `go.mod` language directive is `go 1.27`, and its toolchain directive is `go1.27.4`. The project policy is to use the latest stable released Go version, not retain older compatibility floors. No external module dependencies are declared.
- CI explicitly pins Go `1.27.4` on both `ubuntu-latest` (linux/amd64) and `windows-latest` (windows/amd64), matching the current latest stable release as of this update.

## Verified commands

The following commands completed successfully on both supported CI operating systems at commit `06a824de578880cdaf50078e1d9dca6a56a51a5261` (workflow run 45518701043):

| Command | Purpose | Observed result |
|---|---|---|
| `go test ./...` | Compile packages and run available tests | Success; no test files exist yet |
| `go build ./...` | Build all packages | Success |
| `go vet ./...` | Static analysis | Success |

The commands are executed by [Go verification](../../.github/workflows/go.yml) on pushes to `main` and manual dispatch, with Linux and Windows matrix jobs. The workflow also invokes the shared workflow-history cleanup at startup and restricts its jobs to `main`, as required by repository governance. This run verifies module/package scaffolding only; it is not evidence of acceptance-test coverage or runtime behavior.

Update this document when commands, prerequisites, suite locations, or CI triggers change. Do not claim tests or checks that were not actually run.
