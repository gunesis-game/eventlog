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

D-054 (file-system boundary, Phase 1) was approved on 2026-11-27 and integrated into architecture.md; it is published and fetched back at revision `3ef0a0b`. Phase 2 (Google Drive backend) is approved in principle: `WithStorageEndpoint(GoogleDrive{FolderID, CredentialsFile})`, service-account authentication (browser sign-in possibly later), small 1 MiB segments, and best-effort single-process ownership. The user questioned deferring Drive callbacks; the explanation that callbacks do not affect switching between local disk and Drive awaits their confirmation. Phase 2 decisions are not yet recorded. The user suggested server-sent events to avoid opening and closing connections; research on operating-system limits (docs/research/os-connection-limits.md) led to D-055: HTTP/2 with prior knowledge over one connection, with SSE proposed as the Phase 2 channel for server-to-storage events. D-055 is published and fetched back at revision `7a9eb85`.

Multi-device scope (2026-11-27): the user clarified that single-process ownership is per device and large distribution was always the plan. Approved and recorded locally (not yet published): D-056 (many devices, concurrent writers, same Channels, no central coordinator), D-057 (causal order), D-058 (conflicts reported, deterministic convergence), D-059 (remote changes applied to live instances through the transaction queue). D-060, the per-device-log design in architecture.md, is **Proposed** and awaits approval, with three open points: where per-device local state lives, version vectors versus interval tree clocks, and record-format versioning. Research: docs/research/causal-ordering-and-conflicts.md. Google Drive change notifications need either a public HTTPS endpoint (webhooks) or the Workspace Events API with Pub/Sub StreamingPull (gRPC); polling is ruled out by the user; Drive has no WebRTC support.

Google Drive and notifications (2026-11-27), recorded locally and not yet published: D-061 (OAuth user sign-in; account in configuration; device state directory), D-062 (fixed `eventlog` folder with permanent log identity), D-063 (Firebase Realtime Database board over SSE, authenticated through Firebase Authentication with the Google ID token, in the file-system layer), D-064 (per-Channel version vectors; v1 format changed in place). D-060 remains Proposed with open points: Firebase project details source, security rules installation, and free-plan bounds. Research: docs/research/cross-device-notifications.md.

Drive access (2026-11-27), recorded locally and not yet published: a feasibility check showed `drive.file` cannot read other accounts' files in a shared folder, matching the user's earlier pulsyflux-queue project. D-065: separate accounts with the restricted `drive` scope, Testing status, test users, and weekly re-approval; owner account in configuration; folder found by tag and ownership. Research: docs/research/drive-access-model.md.

D-060 open points settled (2026-11-27), recorded locally and not yet published: D-066 (Firebase details as two descriptor fields for the owner's project; fixed rules with board access following Drive sharing through a members list synced by owner devices; `*SignInRequired` from `PrepareLog` and new ChannelError kinds `SignInRequired` and `NotificationsUnavailable`; SSH port forward for headless sign-in; free-plan bounds accepted). D-060 itself remains Proposed until the user approves the whole design.

Review of D-060 (2026-11-27), recorded locally and not yet published: D-067 (positions in other devices' logs by sequence; compaction on Drive with replaced files trashed); D-068 (storage isolation; `WithStorage(Local{Directory})` or `WithStorage(Web{SettingsFile, DeviceStateDirectory})`; generic public error names; storage refusing writes gives `WriteNotPermitted`). `WithStorageDirectory` is renamed in code to `WithStorage(Local{...})`. Ownership transfer check (2026-11-27), recorded locally and not yet published: D-069, the Drive backend hands every file to the owner (offer by the writer, API acceptance by owner devices); verified end to end and the check program removed. Remaining review gaps: history missing from another device's log (now limited to files not yet accepted, or removed by the owner), and Drive sharing types other than named people.

User rules recorded for this work: no new public package functions (new behaviour goes on the Event/Channel interfaces); refactors remove everything made redundant, with no future-proofing.

## Current repository checkpoint
- **Branch:** main, in sync with `origin/main`. Latest implementation commit: `932616509ac3da6b7f5492019ab0e6e8f3fffce1`.
- `internal/storage` performs file operations only through `internal/filesystem` (client, localhost server, OS backend), over one HTTP/2 connection; Readers read occurrences in batches.
- Code is organized as the public root package plus `internal/identity`, `internal/fieldschema`, `internal/record`, and `internal/storage`; the public API is unchanged (`go doc` identical).
- The polling change manager is removed. Consumers change state only through transactions.

## Verification
- Local windows/amd64, Go 1.27.4: `go test -count=3`, `go test -race -count=2`, `go vet`, `gofmt -l .`, benchmark smoke set, and the steering validator passed.
- Linux: cross-compiled vet and test-binary build passed; tests not executed locally.
- Exact-SHA CI: `9326165` passed Go verification run 46002096517 on Linux (gofmt, test, race, build, vet) and Windows (test, build, vet), and Steering state validation run 46002096856.

## Unresolved risks
- Memory and startup replay cost scale with total history size.
- The full benchmark catalogue is not implemented.

## Immediate next action
1. Get approval for D-060 and settle its open points; then plan and implement the multi-device design and the Google Drive backend in reviewable increments.
2. Implement the full benchmark catalogue and capture baselines; complete the coding-standard review.
3. Re-run CI on the final SHA and assess the Implement exit gate.

## Uncommitted or unpushed work
None besides this checkpoint commit.
