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
- [ ] Complete Markdown/reference inventory and document size map.
- [ ] Analyze duplication, authority, and reference-loading paths.
- [ ] Propose hierarchy and per-document dispositions for user approval.
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
Cycle initialized and started. Assessment has not yet started beyond prerequisite governance/lifecycle review.

## Completion review
- [ ] User-approved hierarchy/dispositions implemented
- [ ] Diff reviewed
- [ ] Link and governance validation passed
- [ ] Context-load simulations completed
- [ ] Documentation and handoff reconciled
- [ ] Cycle registry completed with verification evidence
