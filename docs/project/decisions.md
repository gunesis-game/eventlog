# Project decisions

This is the durable register of approved project decisions. It records outcomes and provenance; it is not a substitute for the current specification.

- [Requirements](requirements.md) define product behavior and externally relevant constraints.
- [Architecture](architecture.md) defines the approved technical design.
- Development-cycle plans and handoffs track temporary scope and progress. They link here rather than duplicating this register.
- A decision is **Recorded** only after its owning specification has been published and its exact revision verified. Proposed decisions remain explicitly proposed until approved.

| ID | Decision | State | Specification | Verified revision |
|---|---|---|---|---|
| D-001 | TEvent is a concrete type constrained by Event; TChannel is constrained by Channel; the factory accepts func(chan Event) TChannel for heterogeneous notifications | Recorded | docs/project/architecture.md | 5e3dd94b988454a31068f75f5b163ec288909e67 |
| D-002 | Channel exposes the receive-only Notifications() <-chan Event accessor | Recorded | docs/project/requirements.md | b580b7c083dea60be61910d402f71d9d969fe590 |
| D-003 | Channel exposes one shared Errors() <-chan error stream with capacity one and retained first-fatal-error, best-effort delivery semantics | Recorded | docs/project/requirements.md | 4b7bc62c6cf851bf62a86bd7a11286c761063f02 |
| D-004 | A fatal Reader error terminates that Channel Reader and closes its streams after error reporting; safe recoverable errors may continue | Recorded | docs/project/architecture.md | d4fe4e197b07385e95159d7090e8110f4805b68f |
| D-005 | Concrete decoder registration derives from TEvent; historical replay waits asynchronously for every decoder required by committed Channel history; no consumer registration or activation API is added | Recorded | docs/project/architecture.md | d4fe4e197b07385e95159d7090e8110f4805b68f |
| D-006 | If a required decoder has not registered, the Channel Reader waits asynchronously without an arbitrary timeout or fatal error caused by absence alone; replay resumes when it registers | Recorded | docs/project/architecture.md | 4ea67d0ca45cc0bc2de7f46717844fbed42d4bb5 |
| D-007 | Both TEvent and TChannel arguments to PrepareLog must be pointer types | Recorded | docs/project/architecture.md | 98918b4a4529f004fc14e267627571d462521214 |
| D-008 | PrepareLog enforces pointer-only generic arguments using runtime reflection and returns an error for non-pointer arguments without changing the generic API | Recorded | docs/project/architecture.md | cef7d96797081f049782815f84b908ba2decfce7 |
| D-009 | Compatible preparations reuse live Event and Channel instances; incompatible type/schema reuse fails explicitly; Channel runtime reuse follows logical Channel identity | Recorded | docs/project/architecture.md | 0b070a42cfa3e924a6744077259dd9ed6a3c90ae |
| D-010 | Event type/schema registration, Event instance identity, and Channel runtime identity are separate registry concerns; Channel runtimes are not keyed only by concrete Go type | Recorded | docs/project/architecture.md | f3e7b8ea2dfc742fa297d19eadc20b5422ff8291 |
| D-011 | Channel runtime resolution uses the factory candidate's canonical identity, reusing an existing Channel pointer/runtime or adopting the candidate for a new logical Channel | Recorded | docs/project/architecture.md | 7bd42a01d58a795b59a57fad06b523de330c6a1f |
| D-012 | The Channel factory is constructor-only and safe to discard; runtime startup follows candidate adoption | Recorded | docs/project/architecture.md | f75de6313f4d4cc2015d11accfdbce413cfc8c72 |
| D-013 | Channel runtime lookup and candidate adoption are atomic by canonical Channel identity, producing one adopted pointer/runtime/Reader | Recorded | docs/project/architecture.md | 9c808cc9fd3c236885ec9f37110cd91cd0297541 |
| D-014 | Concurrent preparations for different Event types on one logical Channel converge on one Channel pointer/runtime/Reader; required historical decoders are awaited asynchronously | Recorded | docs/project/architecture.md | 92d5b462d99a55763cbe5a217292e52a37e4e7e1 |
| D-015 | Successful Event decoder registrations persist for the Log runtime lifetime and are keyed by concrete Event type plus exposed schema | Recorded | docs/project/architecture.md | 3258af3313da4a40f43e7e3a7dd698bb4e5f1de1 |
| D-016 | Re-registering the same Event type/schema reuses its decoder; a different schema fails with a typed conflict without mutating existing registrations or Readers | Recorded | docs/project/architecture.md | aa3f317df4daaba2ce40bed7530309493a9339a4 |
| D-017 | Typed direct-field selectors preserve named primitive types and distinguish nullable nil from a pointer to zero | Recorded | docs/project/architecture.md | 37da85f14975b31ae9810d0f22454537bb0e6c09 |
| D-018 | Historical Event reconstruction uses fresh concrete pointers; notification snapshots are immutable and isolated from live and persistence storage | Recorded | docs/project/architecture.md | 37da85f14975b31ae9810d0f22454537bb0e6c09 |
| D-019 | A prepared Event's identity is immutable; detected identity mutation is rejected and reported through the existing Channel error stream | Recorded | docs/project/architecture.md | 37da85f14975b31ae9810d0f22454537bb0e6c09 |
| D-020 | Detected selected-field changes become immutable snapshots in an internal eventlog-owned queue; persistence workers process them without changing the consumer API | Recorded | docs/project/architecture.md | 37da85f14975b31ae9810d0f22454537bb0e6c09 |
| D-021 | Each logical Event preserves FIFO snapshot order; queued snapshots are retained; only consecutive unchanged observations are suppressed; A-to-B-to-A remains distinct | Recorded | docs/project/architecture.md | 37da85f14975b31ae9810d0f22454537bb0e6c09 |
| D-022 | The internal queue is bounded; capacity pressure delays monitoring rather than consumer assignments; already-enqueued snapshots remain retained | Recorded | docs/project/architecture.md | 37da85f14975b31ae9810d0f22454537bb0e6c09 |
| D-023 | One Log-owned bounded ingress queue dispatches by logical Event identity; each Event persists serially/FIFO while different Events may persist concurrently; global sequence is assigned at commit | Recorded | docs/project/architecture.md | e3ef12c0edf141d2468898655a2708c6433cb654 |
| D-024 | Event and Channel identity fields accept [16]byte and named types with that underlying type; the UUID payload is the 16 bytes in array-index order; nullable and other underlying types are rejected | Recorded | docs/project/architecture.md | aa3fede134f317487585548056fa878486d6b3da |

## Decision maintenance

When a user approves a material project decision:
1. Update the owning specification (requirements.md or architecture.md) by integrating the outcome into the relevant topic.
2. Add or update its concise record here, including the specification path and verified commit revision.
3. Fetch both files back and verify their exact content and revisions before introducing another independent decision.

Do not append raw chat transcripts, progress checkpoints, or handoff prose to the specifications. Do not copy the same decision text into both specifications; use a concise cross-reference when the other document is affected.
