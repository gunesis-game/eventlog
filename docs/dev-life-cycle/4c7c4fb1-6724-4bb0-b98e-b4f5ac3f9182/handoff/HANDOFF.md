# Development Cycle Handoff

## Cycle objective and status

- **Cycle UUID:** 4c7c4fb1-6724-4bb0-b98e-b4f5ac3f9182
- **Objective:** Define eventlog's initial product requirements and core design decisions before implementation.
- **Registry status:** in-progress
- **Persisted phase:** define
- **Last updated:** 2026-11-22T14:39:08Z

## Current repository state

- **Branch/revision:** main; sequential remote commits through GitHub contents API.
- **Working tree:** No local checkout used; no working-tree status available.
- **Collaboration state:** Cycle 1 resumed. Execution lock active and verified on shared main for ChatGPT.
- **Relevant environment:** No Go source or test suite discovered; project-specific commands remain unverified.

## Completed work

- Inspected repository metadata and tree, README, steering policy, lifecycle README, lock and registry schemas/state, plan/handoff templates, requirements, architecture, coding standards, commands documentation, and workflows.
- Initial assessment checkpoint persisted; cycle paused while awaiting product direction.
- User confirmed general-purpose embedded event storage (Option C).
- User confirmed single-process ownership; concurrent goroutines may be coordinated inside the library.
- Confirmed typed generic event API direction.
- Confirmed consumer structs remain unrestricted and may contain runtime-only or sensitive fields, but eventlog must persist only the explicitly returned field set.
- Replaced previous redaction approach with an explicit allowlist persistence contract.
- Agreed consumers implement a library-owned `Event` interface returning `eventlog.Fields`.
- `Fields` will hold heterogeneous primitive values, with generic `FieldOf` constraining values.
- Preferred field-name API uses a typed selector (e.g. `FieldOf(func(v *UserCreated) *string { return &v.Name }, e)`) to derive names rather than manually supplied strings. This exact implementation is not finalized; reflection may be needed.
- Initial serialization is built-in JSON; custom codecs/encoders/decoders are not in v1 scope.
- Cycle 1 is active on main under lifecycle lock; no implementation started.

## Confirmed decisions and constraints

- General-purpose embedded storage, independent of event/domain semantics.
- Single process owns a log at a time; library may coordinate concurrent goroutines.
- Consumer-defined struct types remain unconstrained; only their Event method's returned Fields are persisted.
- Library Event interface and Fields container form the persistence allowlist.
- Generic FieldOf constructor limits each value to approved primitive types.
- Avoid string field names by deriving the name through a selector.
- Built-in JSON serialization; no custom codec in initial scope.
- Earlier redaction feature is superseded; security responsibility remains with consumers to expose only intended fields.

## Proposed / open

- Exact Go API and generic selector implementation, including reflection behavior.
- Primitive type set and handling of named types, pointers, nullability, time values, and nested data.
- On-disk framing, versioning, checksums, sequence and ordering.
- Durability/sync defaults and guarantees.
- Incomplete-tail and corruption recovery.
- Replay/tailing API and replay-to-live behavior.
- Segmentation/rotation, limits, portability and performance goals.

## Verification evidence

- Cycle lock claim and resume previously fetched back and confirmed.
- Plan updated with current Event/Fields/selector/JSON direction in commit `885c6d3ccca35593e63dbd2e1493b57fd74577e9`.
- Build/tests not run: implementation has not started.

## Documentation state

- Updated cycle plan.
- Handoff updated in this checkpoint.
- Requirements and architecture documents will be reconciled once remaining decisions are confirmed.
- No implementation or Go validation performed.

## Persisted phase checkpoint

- **Current phase:** define
- **Assess exit evidence:** Repository governance and initial state reviewed; general-purpose scope, single-process ownership, and explicit typed persistence allowlist confirmed.
- **Next concrete action:** Validate feasibility and ergonomics of the generic field selector and settle the supported primitive representation, then proceed to operational guarantees.

## Uncommitted or unpushed work

- None known; remote contents API used.

## Continuation instructions

Execution lock remains active for cycle 1. Continue the define phase and synchronize registry, plan, and handoff for each checkpoint. Do not begin implementation until design decisions and acceptance criteria are ready. Before stopping, checkpoint, release the lock on main, and verify the lock release.
