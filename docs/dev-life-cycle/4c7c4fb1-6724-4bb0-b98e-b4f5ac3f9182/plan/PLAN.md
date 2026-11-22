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

- **Primary scope:** General-purpose embedded event storage (Option C); the library should not depend on domain-specific event meanings or an event-sourcing framework.
- **Ownership:** One process owns a log at a time. Multiple goroutines may use the library, but writes will be coordinated internally.
- **Public event API direction:** Consumers implement a library-owned `Event` interface whose method returns `eventlog.Fields`. Eventlog persists only this explicit representation, not the consumer's full struct.
- **Persistable fields:** `Fields` contains heterogeneous primitive values; a generic `FieldOf` constructor constrains each value to supported primitive types.
- **Field names:** Consumers prefer type-safe field selectors (e.g. `FieldOf(func(v *UserCreated) *string { return &v.Name }, e`) to derive field identity rather than hand-written string names. The selector may require reflection internally; feasibility and implementation details remain to be validated.
- **Serialization:** Built-in JSON persistence for initial scope; consumers are not expected to supply custom encoders/decoders.
- **Redaction:** The earlier explicit redaction requirement is superseded by the Event allowlist contract. Consumers explicitly expose only persisted fields; eventlog does not inspect or redact their original structs.
- **Redaction requirement:** Consumers keep their own event structs intact, including sensitive fields. Eventlog must offer an easy, type-safe field-selector redaction API (not string field names); it must redact an internal copy before serialization so the caller's original event is unchanged and sensitive values are never persisted.
- **Design intent:** Keep it reusable and small, with a simple reliability-focused API.

These are confirmed product direction decisions, not yet detailed API or file-format decisions.


## Scope

### In scope
- Establish product goals, workload assumptions, guarantees, and non-goals.
- Define append, replay/read, recovery, and tailing semantics.
- Compare storage framing/encoding, durability, concurrency, and API options.
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

- Exact implementation of the generic selector and field-name discovery (Go has no built-in `nameof`; likely reflection-based selector resolution).
- Event contract and `Fields` concrete representation; selector API implementation and reflection trade-offs remain to be finalized.
- Public generic API shape and error behavior.
- On-disk framing, metadata, checksums, and format/version strategy.
- Sequence numbering, record identity, and ordering guarantees.
- Durability defaults and configurable sync policy.
- Corruption and incomplete-tail recovery behavior.
- Reader/tailer interface, cancellation, and replay-to-live transition semantics.
- Log rotation/segmentation and size limits (whether required initially).
- Portability expectations and performance goals.

## Proposed baseline (not yet approved)

- Library-owned `Event` contract returning `Fields`; `FieldOf` uses generic primitive constraints and selector-derived names. JSON is built-in; no custom codec in initial scope.
- Append-only binary record format with explicit framing.
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
- [ ] Define `Event` interface, `Fields` representation, and `FieldOf` generic selector contract.
- [x] Replace redaction design with explicit allowlist `Event` contract.
- [ ] Decide framing, durability, recovery, concurrency and tailing behavior.
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
- **Next concrete action:** Define codec contract options and API shape, then confirm encoding approach.
- **Last updated:** 2026-11-22T14:39:08Z


## Progress and decisions

- Cycle 1 resumed; execution lock claim confirmed on shared main.
- Confirmed: general-purpose embedded event storage (Option C).
- Confirmed: single process owns a log; in-process concurrent access can be coordinated by the library.
- Confirmed: consumers implement a library-defined Event interface returning Fields; generic FieldOf selectors identify fields without strings. Built-in JSON serialization; no custom codecs planned for v1.
- Proposals are recorded as proposals pending approval; no implementation performed.


## Completion review

- [ ] Acceptance criteria verified or exceptions explicitly accepted
- [ ] Final diff reviewed
- [ ] Required verification results recorded
- [ ] Documentation reconciled
- [ ] Handoff updated
- [ ] Registry status, phase, and completion timestamp updated consistently
