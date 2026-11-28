# Cycle 2 — Documentation context-efficiency review

## Identity and status
- **Cycle ID:** 53e5d274-7a2a-4096-9068-dee0f853d049
- **Sequence:** 2
- **Execution order:** 2
- **Workflow:** standard
- **Status / phase:** in-progress / assess
- **Created:** 2026-11-28T10:06:34Z
- **Started:** 2026-11-28T10:07:22Z

## Objective
Refine eventlog's Markdown documentation architecture so AI agents can enter from a concise landing page, follow a deliberate hierarchy, load documents only when relevant, avoid redundant/duplicated information, and retain complete authoritative detail in small standalone documents or conditional leaf references where detail cannot be compressed.

## Problem and context
The current repository contains substantial Markdown documents, including large architecture, lifecycle, and decision-ledger documents. The user's goal is to reduce context loading and token cost without losing important content, breaking repository governance, or creating recursive reference chains that imply every linked document must be read.

## Scope

### In scope
- Inventory all Markdown files and inbound/outbound reference paths from the entry point.
- Identify duplicated content, document overlap, unclear ownership, oversized mixed-purpose documents, references that cause unnecessary recursive reading, and missing routing.
- Propose a hierarchy and explicit reference-loading semantics (e.g. must-read, read-when, reference, background), with conditional leaf documents for dense material.
- Define each document's standalone framing and canonical ownership.
- Evaluate keep, shorten, split, merge, relocate, retire, or cross-link decisions per document.
- Test the proposed hierarchy against representative AI tasks and estimate context-loading behavior qualitatively or with token measurements where practical.
- Preserve governance and all authoritative decisions/content unless consolidation or removal is approved and verified.

### Out of scope
- Product implementation or changes to runtime behavior.
- Unapproved removal or rewriting of authoritative requirements, decisions, technical detail, or governance.
- Changes to lifecycle rules, architecture, requirements, or decisions beyond those necessary for the approved documentation restructuring.

## Constraints and risks
- Follow `steering.md`, lifecycle process, and main-only Git workflow.
- Each fact or policy must have one canonical owner; references point to it rather than duplicate.
- A link is navigation, not an instruction to recursively load its target or all descendants. Explicit conditions govern when to read.
- Documents should be standalone, small where possible, and state purpose, authority, scope, prerequisites, and conditional next references.
- Essential detailed content may remain a long leaf if it is only loaded when its task requires it.
- Risks: loss of nuance through over-compression; reference cycles or unclear load triggers; accidental governance weakening; stale links; and confusing authoritative docs with indexes/checkpoints.

## Applicable standards and strategies
- Coding standard: `docs/standards/coding.md` (document edits do not change code, but inspect any relevant formatting conventions).
- Testing strategy: `docs/project/testing-strategy.md` and `docs/standards/testing.md`; select link/reference integrity checks and governance validation.
- Performance strategy: `docs/project/performance-strategy.md` and `docs/standards/performance-engineering.md`; token/context-load measurements are relevant if practical.
- Git workflow: `docs/project/git-workflow.md` and `docs/standards/git-workflow.md`; main-only direct commits, publication, fetch-back, and evidence.

## Authoritative-state reconciliation
- **Repository revision inspected:** initial checkpoint `9996f247817772d6e2fedcfe14f04e46857df647`; verify current main after lifecycle setup.
- **Canonical requirements/architecture/decisions:** to inventory and review before restructuring.
- **Candidate items:** initially hypotheses only; no file changes to project documentation yet.

## Design and review approach
1. Inventory Markdown files, their purpose/size, links, and read-path from README.
2. Inspect documents fully, including detailed topics and unresolved/open questions; verify ownership and decision state against canonical docs.
3. Produce a proposed hierarchy and per-file disposition before substantive changes; ask user for approval.
4. Implement approved changes in small related groups, fetch back, review each diff, and verify links/validator.
5. Simulate common tasks and audit context paths, duplication, completeness, standalone framing, governance compliance.

## Architecture impact and verification
- **Changed components:** documentation architecture and cross-references only.
- **Affected dependants:** repository entry points, steering, lifecycle routing, project documentation, standards indexes, research.
- **Tests:** link integrity and lifecycle validator where applicable; manual path walkthroughs from common AI tasks.
- **Acceptance:** concise entry; explicit non-recursive routing; each doc standalone and authoritative boundary clear; no unnecessary duplicate content; important detailed material remains accessible through conditional leaf paths; all links resolve; no governance rule weakened or lost.

## Ordered tasks
- [x] Complete Markdown/reference inventory and document size map.
- [x] Analyze duplication, authority, and reference-loading paths (initial analysis).
- [x] Propose hierarchy and per-document dispositions for user approval.
- [ ] Apply approved restructure incrementally.
- [ ] Audit links, document ownership, completeness, and governance.
- [ ] Simulate representative AI task context-loading paths and record findings.

## Acceptance criteria
- [ ] AI entry document is short and routes without requiring recursive reading.
- [ ] Every document has a clear purpose, scope, authority, and when-to-read guidance.
- [ ] Canonical facts are not duplicated; overlapping text is consolidated.
- [ ] Detailed critical information is retained as conditional leaves when it cannot be compressed safely.
- [ ] References are intentional, non-recursive in their loading semantics, and task-conditional.
- [ ] Governance, decisions, and project requirements remain intact and authoritative.
- [ ] Links and lifecycle validation are checked and reported.

## Persisted phase checkpoint
- **Current phase:** assess
- **Phase exit evidence:** complete inventory, document-by-document analysis, and proposed hierarchy/dispositions presented to user; no structural changes before approval.
- **Next action:** inventory Markdown paths, sizes, and cross-reference relationships.
- **Last updated:** 2026-11-28T10:07:22Z

## Progress
Cycle initialized and started. Assessment inventory and initial structural analysis completed; disposition proposal is prepared for user review.

## Completion review
- [ ] User-approved hierarchy/dispositions implemented
- [ ] Diff reviewed
- [ ] Link and governance validation passed
- [ ] Context-load simulations completed
- [ ] Documentation and handoff reconciled
- [ ] Cycle registry completed with verification evidence


## Assessment checkpoint — 2026-11-28T10:09:41Z

### Markdown inventory
- 31 Markdown files discovered from the main recursive Git tree; inventory covers root entry, steering, project specifications/strategies, generic standards, research, lifecycle protocol/templates, and current/historical cycle records.
- Large active documents: architecture 70,968 bytes; lifecycle README 36,103; decisions 33,355; benchmarks 23,672; acceptance 16,648; steering 17,957; requirements 15,703; testing strategy 8,333; cross-device notification research 14,252.
- Current cycle plan/handoff are administrative records; prior implementation cycle also has archived plan/handoff/history Markdown, including very large historical files. These should not be treated as default AI context.

### Initial findings (not final dispositions)
1. README is a short project entry but its Documentation lists nearly every project spec, policy, standard, steering and lifecycle doc side-by-side. This is an index, but it does not provide task-driven read rules or prevent the link-list from being treated as an invitation to read everything.
2. Steering is ~18 KB and combines non-bypassable operating gates, reconciliation protocol, broad operating principles, main-only workflow, cycle procedure, documentation ownership, approval gate, and communication. Its overlap with lifecycle README is the highest-priority duplication to inspect and resolve. Essential governing gates may remain at the entry boundary; operational mechanics should have one owner.
3. Lifecycle README is ~36 KB, serving as protocol, phase machine, template adoption, validator contract, decision gate, verification loop, completion procedure, and documentation audit. It likely needs a compact router/overview plus conditional protocol leaves, but any splitting must preserve its validation and mandatory rules.
4. Architecture (~71 KB) is an overloaded mixed document: system boundary/API, package layout, identity/fields, append/recovery, transactions, Reader, runtime, storage/compaction, portability, multi-device, Drive/Firebase details, and open design questions. It is a strong candidate for topic leaves, while the top-level architecture should retain overview, boundaries, component map, and critical invariants.
5. Requirements (~15.7 KB) contains externally observable behavior and constraints but references architecture, acceptance, benchmarks, and decisions. Maintain separation: requirement obligations vs design details vs acceptance test cases. Candidate overlap exists around acceptance expectations and quality constraints.
6. Acceptance (~16.6 KB) is an 8-layer test strategy plus execution/evidence guidance and post-implementation performance evidence. Testing strategy (~8.3 KB) separately defines suite tiers, architecture ownership, quality policy, CI/evidence and exclusions. These should have distinct authority, with clear routing and no duplicated test policy.
7. Benchmarks (~23.7 KB) includes workloads, measurement, layer-specific benchmark scenarios, execution strategy, smoke suite, artifacts/retention, run-history retention, and Define work. It is likely a conditional leaf for performance tasks; project performance strategy and generic performance standard should route to it rather than re-state details.
8. Generic engineering standards have an index and canonical docs; project-specific companion strategies exist. The separation is conceptually sound, but README, steering, lifecycle, and standards index reference both tiers. Need avoid requiring both standard and companion when task relevance can select.
9. Research docs (four) link to requirements and architecture. Their contents are evidence/background, not default context; approved conclusions belong only in requirements/architecture. Keep research as conditional leaves and verify source links/conclusion ownership.
10. Decisions ledger (~33 KB) is a durable register, but many decision rows are detailed and specifications themselves contain decision text. Keep decision provenance and status, while investigate whether ledger entries can be concise and specification links can avoid duplicated descriptions. Do not remove history or authority without approval.
11. Lifecycle cycle plan/handoff/history are temporal context, not permanent product specification. Entry routing should explicitly load the current active cycle only when work/progress/resumption requires it; historical cycle plans/handoffs and history docs should not be loaded by default.

### Proposed hierarchy (draft for approval)
- Tier 0: README — short purpose and task router; avoid direct catalog of all docs as mandatory reading.
- Tier 1: steering — compact mandatory agent operating gates, canonical ownership, how to select task context; links to lifecycle for procedural detail.
- Tier 2: lifecycle entry — concise cycle-selection/state/reconciliation overview; detailed cycle creation, pause/resume, phase machine, validator, approval, verification, and completion as conditional protocol leaves.
- Tier 3: project index/task routing — canonical links to Requirements, Architecture overview, Acceptance, Testing Strategy, Performance Strategy, Git Workflow, Commands, Decisions, and Standards; selection rules (task type -> must-read docs).
- Tier 4: topic leaves — architecture by subsystem; acceptance scenarios; benchmark workloads/methodology; project-specific strategies; research; detailed standards; decision provenance. Read only on explicit relevance.

### Draft reference labels
- MUST READ: only when the stated task requires the content; global mandatory reads must be very few.
- READ WHEN: explicit task triggers (e.g., code change -> applicable architecture + requirements + coding/testing; performance task -> strategy and benchmark leaf).
- REFERENCE: authoritative detail to consult to resolve a question.
- BACKGROUND: research/history, not instruction or specification.

### Remaining assessment / approval boundary
- These are initial findings and proposed dispositions, not approved changes. No documentation restructuring has been made. Next: confirm this hierarchy and proceed to Define with file-by-file keep/split/merge/relocate/retire proposals, or revise the draft before defining work.
