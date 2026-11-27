# Cycle 1 — Current handoff

## Cycle identity and status
- **Cycle ID:** 4c7c4fb1-6724-4bb0-b98e-b4f5ac3f9182
- **Sequence / execution order:** 1 / 1
- **Workflow:** standard
- **Registry status / phase:** in-progress / implement
- **Checkpoint date:** 2026-11-27

## Objective
Implement eventlog according to the approved requirements, architecture, acceptance criteria, and public API.

## Authoritative project records
- [Requirements](../../../project/requirements.md)
- [Architecture](../../../project/architecture.md)
- [Decision register](../../../project/decisions.md)
- [Acceptance matrix](../../../project/acceptance.md)
- [Testing strategy](../../../project/testing-strategy.md)
- [Performance strategy](../../../project/performance-strategy.md) and [benchmarks](../../../project/benchmarks.md)
- [Git workflow](../../../project/git-workflow.md)
- [Plan](../plan/PLAN.md)

## Decisions this session
D-044–D-048 are published and verified (revision `b7e34b2`). D-049–D-052 (transactions replace polling; shared ordered queue and group commit; prefix-rule rejection with rollback; typed `ChannelError` with pause-until-received) were approved by the user on 2026-11-27 and integrated into the specifications. They were published to `main` and fetched back from `origin/main`; the ledger cites the verified revision `dbdd652`.

D-053 (package layout) was approved on 2026-11-27 and integrated into architecture.md; it was published and fetched back from `origin/main` at revision `128ac54`.

D-054 (file-system boundary, Phase 1) was approved on 2026-11-27 and integrated into architecture.md; its ledger row cites a local commit and is **not yet published**. Phase 2 (Google Drive backend) is approved in principle: `WithStorageEndpoint(GoogleDrive{FolderID, CredentialsFile})`, service-account authentication (browser sign-in possibly later), small 1 MiB segments, and best-effort single-process ownership. The user questioned deferring Drive callbacks; the explanation that callbacks do not affect switching between local disk and Drive awaits their confirmation. Phase 2 decisions are not yet recorded. The user suggested server-sent events to avoid opening and closing connections; research on operating-system limits (docs/research/os-connection-limits.md) led to D-055: HTTP/2 with prior knowledge over one connection, with SSE proposed as the Phase 2 channel for server-to-storage events. D-055 is recorded in a local commit and not yet published.

User rules recorded for this work: no new public package functions (new behaviour goes on the Event/Channel interfaces); refactors remove everything made redundant, with no future-proofing.

## Current repository checkpoint
- **Branch:** main. `origin/main` is at `a18219b`; four local commits are unpushed.
- `internal/storage` performs file operations only through `internal/filesystem` (client, localhost server, OS backend), over one HTTP/2 connection; Readers read occurrences in batches.
- Code is organized as the public root package plus `internal/identity`, `internal/fieldschema`, `internal/record`, and `internal/storage`; the public API is unchanged (`go doc` identical).
- The polling change manager is removed. Consumers change state only through transactions.

## Verification
- Local windows/amd64, Go 1.27.4: `go test -count=3`, `go test -race -count=2`, `go vet`, `gofmt -l .`, benchmark smoke set, and the steering validator passed.
- Linux: cross-compiled vet and test-binary build passed; tests not executed locally.
- Exact-SHA CI: restructure `ffee9c0` passed Go verification run 45990909613 on Linux (gofmt, test, race, build, vet) and Windows (test, build, vet), and Steering state validation run 45990909648.

## Unresolved risks
- Memory and startup replay cost scale with total history size.
- The full benchmark catalogue is not implemented.

## Immediate next action
1. With user approval, push to `main`, fetch back D-054 and D-055, and inspect exact-SHA CI.
2. Confirm callback handling with the user, record the Phase 2 decisions, and implement the Google Drive backend.
3. Implement the full benchmark catalogue and capture baselines; complete the coding-standard review.
4. Re-run CI on the final SHA and assess the Implement exit gate.

## Uncommitted or unpushed work
The D-054 and D-055 decision commits, the Phase 1 commit, and the HTTP/2 commit are unpushed.
