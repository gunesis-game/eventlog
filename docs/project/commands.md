# Development commands

## Go toolchain

- Module: `github.com/gunesis-game/eventlog`.
- The `go.mod` language directive is `go 1.18`, the minimum language level required by the approved generic public API. No external module dependencies are declared.
- CI currently resolves `stable`; the verified run used Go 1.27.4 on both `ubuntu-latest` (linux/amd64) and `windows-latest` (windows/amd64). The module directive is not a pin to that CI toolchain version.

## Verified commands

The following commands completed successfully on both supported CI operating systems at commit `4fbf2837906b0e25c635ef00837cc711c525f374`:

| Command | Purpose | Observed result |
|---|---|---|
| `go test ./...` | Compile packages and run available tests | Success; no test files exist yet |
| `go build ./...` | Build all packages | Success |
| `go vet ./...` | Static analysis | Success |

The commands are executed by [Go verification](../../.github/workflows/go.yml) on pushes to `main` and manual dispatch, with Linux and Windows matrix jobs. This initial run verifies module/package scaffolding only; it is not evidence of acceptance-test coverage or runtime behavior.

Update this document when commands, prerequisites, suite locations, or CI triggers change. Do not claim tests or checks that were not actually run.
