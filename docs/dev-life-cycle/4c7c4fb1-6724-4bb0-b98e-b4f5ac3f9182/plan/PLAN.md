# Development Cycle Plan

## Registry identity

- **Cycle UUID:** 4c7c4fb1-6724-4bb0-b98e-b4f5ac3f9182
- **Sequence:** 1
- **Execution order:** 1
- **Status:** in-progress
- **Status reason:** null
- **Current phase:** define
- **Created:** 2026-11-22T16:00:00+02:00

## Objective

Define eventlog's initial product requirements and core design decisions before implementation.

## Problem and context

The repository describes eventlog as a lightweight disk-backed append-only Go log intended for event-oriented applications. The scaffold proposes persistence, ordered replay, and transition into live tailing. Requirements, architecture, commands, and README content remain placeholders. No implementation or tests were found during initial assessment.

## Confirmed product direction

- **Primary scope:** General-purpose embedded event storage; independent of domain semantics and event-sourcing frameworks.
- **Ownership:** One process owns a log at a time. Multiple goroutines may concurrently use the library; writes are coordinated internally.
- **Public event API:** Consumers implement a library-owned Event interface returning eventlog.Fields.
- **Persistence allowlist:** Consumer event structs remain unrestricted. Only the explicitly returned Fields are persisted; eventlog does not inspect, serialize, or redact the full event struct.
- **Fields:** Heterogeneous primitive values.
- **Field construction:** Generic FieldOf selector constrains selected values to supported primitive types and derives the field name from a typed selector rather than a manually supplied string.
- **Named primitive types:** Supported when their underlying type is in the primitive allowlist (e.g. type UserID string, type AccountBalance int64, type IsActive bool). Exact allowlist remains to be finalized.
- **API direction:**
  ```go
  type Event interface {
      Fields() Fields
  }

  type Fields []Field

  func FieldOf[T any, V Primitive](
      selector func(*T) *V,
      event *T,
  ) Field
  ```
  This is the agreed API direction; implementation details, including selector name resolution, remain to be validated.
- **Serialization:** Built-in JSON persistence for v1; no custom codecs.
- **Record framing:** Newline-delimited JSON (NDJSON), one JSON event record per line, terminated by a newline.
- **Redaction:** The earlier redaction feature is superseded by the explicit Event/Fields allowlist contract. Consumers decide which fields to expose; eventlog persists only those fields.

## Scope

### In scope
- Establish product goals, workload assumptions, guarantees, and non-goals.
- Define append, replay/read, recovery, and tailing semantics.
- Define record representation, durability, concurrency, and API behaviour.
- Record approved requirements and design decisions in maintained project documentation.

### Out of scope
- Implementing the Go library in this cycle.
- Distributed or multi-process writer coordination.
- Adding dependencies or tooling without clear need.
- Full event-sourcing frameworks, databases, or brokers.
- Introducing domain-specific assumptions into the storage layer.

## Constraints, dependencies, and risks

- **Constraints:** Follow steering.md and docs/dev-life-cycle/README.md. Work on main, serialized, cycle-tracked, and verified. No project-specific Go commands are yet verified.
- **Dependencies:** User confirmation of critical design trade-offs.
- **Risks:** Overdesign; unclear durability promises; unsafe recovery; concurrency/tailing contract ambiguity; portability and performance targets.

## Open decisions

- Exact implementation of the generic selector and field-name discovery (Go has no built-in nameof; likely reflection-based selector resolution).
- Exact primitive allowlist and handling of nullable values.
- Public API error behaviour and validation of Fields.
- Record envelope/schema metadata and format versioning.
- Sequence numbering, record identity, and ordering guarantees.
- Durability/sync defaults and guarantees.
- Corruption and incomplete-tail recovery behaviour.
- Reader/tailer interface, cancellation, and replay-to-live transition semantics.
- Log rotation/segmentation and size limits (whether required initially).
- Portability expectations and performance goals.

## Proposed baseline (not yet approved)

- Library-owned Event contract returning Fields; FieldOf uses generic primitive constraints and selector-derived names. JSON is built-in; no custom codec in initial scope.
- NDJSON framing, one complete JSON record per newline-terminated line.
- Strict sequence ordering.
- Sequential replay and a tail mode.
- Safe recovery from incomplete trailing records.
- Configurable durability/sync policy with documented guarantees.
- Standard-library-only dependencies.
- Cross-platform Go where practical.

## Design and implementation approach

Confirm product requirements and trade-offs in discussion before documenting them as approved. Keep initial implementation scope minimal. Do not begin implementation until critical choices are resolved and recorded.

## Ordered tasks

- [x] Establish primary workload direction and ownership model.
- [x] Select typed generic events as public API direction.
- [x] Define Event interface and Fields/FieldOf API direction.
- [x] Replace redaction design with explicit allowlist Event contract.
- [x] Select built-in JSON serialization and NDJSON record framing.
- [ ] Finalize primitive allowlist and API edge cases.
- [ ] Decide record envelope/versioning, durability, recovery, concurrency and tailing behaviour.
- [ ] Update requirements and architecture docs with confirmed decisions.
- [ ] Review consistency; close this cycle and prepare implementation-ready scope.

## Acceptance criteria

- [ ] Confirmed requirements are separated from proposals and open questions.
- [ ] Core design decisions are explicit, justified, and captured in architecture documentation.
- [ ] Initial API and operational guarantees are understandable and testable.
- [ ] No implementation begins while critical design choices remain unresolved.
- [ ] Focused documentation review completed; cycle checkpoint and lifecycle state reconciled.

## Verification strategy

- **Local checks:** Inspect available project validation and documentation format; select appropriate checks before using them.
- **CI/integration checks:** Existing lifecycle validator workflow is available; no Go implementation/test CI established yet.
- **Manual review:** Check requirements, architecture, coding standards, README and commands docs for consistency and avoid stating proposals as facts.

## Persisted phase checkpoint

- **Current phase:** define
- **Assess exit evidence:** Repository scaffold and governance were inspected; purpose, current state, risks, unknowns, and decision areas identified sufficiently to define. User confirmed general-purpose embedded storage, single-process ownership, and an explicit Event/Fields persistence allowlist contract.
- **Next concrete action:** Resolve the remaining record envelope, durability, recovery, and reader/tailer decisions before promoting the approved design to project documentation.
- **Last updated:** 2026-11-22T16:00:00Z

## Progress and decisions

- Cycle 1 resumed; execution lock claim confirmed on shared main.
- Confirmed general-purpose embedded event storage and single-process ownership, with concurrent goroutine access coordinated internally.
- Confirmed consumer-defined Event structs expose an explicit Fields allowlist; only those fields are persisted.
- Confirmed generic FieldOf selector API direction, primitive-only selected values, and named primitive types when their underlying type is allowed.
- Confirmed built-in JSON serialization and NDJSON framing (one JSON record per newline-terminated line).
- The earlier redaction design is superseded; no full-struct inspection or automatic redaction is required.
- No implementation, build, or Go tests have started.

## Completion review

- [ ] Acceptance criteria verified or exceptions explicitly accepted
- [ ] Final diff reviewed
- [ ] Required verification results recorded
- [ ] Documentation reconciled
- [ ] Handoff updated
- [ ] Registry status, phase, and completion timestamp updated consistently
