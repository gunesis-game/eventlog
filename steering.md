# Steering

## 0. Session-mode classification (Gate 0)

**Classify the session's intent and scope before reading lifecycle state, selecting/creating/resuming a cycle, or beginning substantive work.** Do not treat operational lifecycle rules as universally applicable to governance-maintenance sessions.

- **Governance Mode:** Work whose objective is to create, review, amend, repair, or improve repository governance/AI operating rules: steering, lifecycle, governance rules, generic engineering standards, project-strategy or decision-ledger templates, template release/adoption rules, and governance context-routing instructions. Use the narrow governance-maintenance exemption. Do not create, select, resume, or advance an operational cycle while work remains within this scope.
- **Operational Mode:** Ordinary product work, including product requirements, architecture, behavior, features, defects, implementation, tests, and substantive product documentation. The development-cycle mechanism is mandatory; reconcile persisted state before substantive work.
- **Classification is based on the session objective and scope, not simply the file being edited.** Editing a requirements/architecture document as part of governance-routing repair does not itself change the mode; actually changing product requirements or architecture as project work is Operational.
- **Mixed or uncertain scope:** Clarify or split before proceeding. Governance Mode must stop before taking on operational work, identify the scope crossing, and route that work through Operational Mode. The exemption must never be silently broadened.
- Load only the documents needed for the classified mode and task. A link is a conditional reference, not a recursive instruction to load all linked documents.

README.md establishes this first gate. This section is authoritative for the mode distinction and its scope boundary.

## 1. Purpose and authority
This file establishes the repository's governing development principles. Its north star is:

1. **Development work is tracked through the persisted development-cycle mechanism, subject to the explicit governance-maintenance exemption below.**
2. **All work is performed on the `main` branch only.**
3. **Development work is coordinated through persisted cycle records and ordinary Git collaboration.**

The detailed lifecycle rules, registry contract, standard and micro cycle workflows, phase machine, plan and handoff formats, verification loop, documentation audits, and completion gates are defined in [docs/dev-life-cycle/README.md](docs/dev-life-cycle/README.md). That document and its registry/schema/templates are the operational specification for the cycle mechanism.

**Governance-maintenance exemption:** Changes limited to session-mode governance routing in the root `README.md`, steering/lifecycle governance, generic engineering standards under `docs/standards/`, project-strategy templates under `docs/project/` (testing, performance, and Git workflow), the project decision-ledger template, the validator/workflow, or template release metadata/changelog—and intended to amend or repair those governance or adoption rules—are exempt from creating, selecting, resuming, or completing a development cycle. Root `README.md` is included only when changing its AI session classification/governance routing; ordinary product README content remains Operational work. Keep such changes strictly limited to those governance/template files. If ordinary product requirements, architecture, implementation, tests, or operational files are also changed, that ordinary work must use the normal cycle process. The exemption from cycle tracking does not waive other repository controls.

**User authority and policy changes:** User instructions govern the requested outcome. A user may explicitly authorize a change to this steering policy or lifecycle specification. When the request is to amend these governance files, apply the governance-maintenance exemption above rather than requiring a cycle to change the rules that define cycles. For ordinary work, follow the lifecycle selection and priority rules in the lifecycle README. If project sources disagree, identify the conflict and resolve it explicitly; do not silently weaken unrelated requirements.

Apply these rules alongside the project's established requirements. Project-specific requirements and constraints must be established from project evidence. Never invent project-specific facts or silently import another project's commands, paths, architecture, platforms, dependencies, or quality requirements.

## 1A. Non-bypassable operational gate (Operational Mode only)

The written lifecycle is an execution control, not optional guidance in Operational Mode. Apply it before ordinary project work and after every user approval:

1. Before ordinary work, follow the lifecycle start/resume protocol. Do not conduct substantive untracked design, implementation, or investigation outside an active persisted cycle.
2. Read the active cycle handoff as a progress checkpoint and read `docs/project/decisions.md` for durable decisions. Chat history or memory is not a substitute for persisted state.
3. When the user approves a material decision, follow the [decision approval-to-artifact gate](docs/dev-life-cycle/README.md#decision-approval-to-artifact-gate): record it in the owning specification and the decision ledger, publish, fetch back, and verify before any dependent work. If recording or verification fails, stop dependent work and leave the decision Proposed.
4. Re-read the current handoff and decision ledger before continuing after a context switch or tool failure.

### Reconcile authoritative project state before reporting remaining work

Before describing an item as open, proposing the next action, or recommending a phase transition, derive the current state from the repository's authoritative records (requirements, architecture, and the decision ledger, then the cycle plan and handoff as checkpoints), not from a stale plan, handoff, unresolved-items list, or chat history alone. The full protocol is defined once, in the [lifecycle README](docs/dev-life-cycle/README.md#reconcile-authoritative-project-state-before-reporting-remaining-work); follow it from there rather than from a copy.

Governance-maintenance work may use the narrow exemption in Section 1, but still requires review, publication, and verification.

## 2. Establish project context before acting

Discover the project before changing it:

- Inspect the repository's structure, relevant source, documentation, configuration, tests, automation, and current Git state.
- Read the instructions and documentation relevant to the requested area. Find existing conventions and decisions before introducing new ones.
- Establish the project's purpose, users, supported behavior, architecture, technology choices, constraints, quality expectations, and current status from available evidence.
- Distinguish verified facts from assumptions, proposals, and unknowns.
- Identify relevant dependencies, interfaces, compatibility expectations, and risks.

Never assume project-specific facts. Discover them from the repository, its documentation, configuration, implementation, tests, and automation. If information cannot be established, identify it as unknown rather than inventing an answer.

## 3. Operating principles

- **Understand before changing.** Trace existing behavior and dependencies before editing.
- **Preserve intent.** Maintain established behavior, public contracts, compatibility, and project conventions unless the requested change requires otherwise.
- **Keep changes focused.** Prefer the smallest coherent implementation; avoid unrelated refactoring, speculative abstractions, and unnecessary dependencies.
- **Make decisions explicit and durable.** Explain consequential trade-offs and record durable project decisions in the appropriate project documentation.
- **Enforce the approval-to-artifact gate.** An approval is not permission to move to dependent work. Follow the [decision approval-to-artifact gate](docs/dev-life-cycle/README.md#decision-approval-to-artifact-gate) first.
- **Keep evidence honest.** Never claim that a change, test, build, CI run, review, or deployment succeeded unless it was performed and its result observed.
- **Control scope.** Do not expand the task or introduce tools, infrastructure, or policy changes without a clear need and appropriate authorization.
- **Bound governance maintenance.** Governance reviews evaluate the AI's operating rules and workflow—not product-specific requirements, architecture, features, or technical documentation. Review product documents only when a governance rule explicitly depends on them or they must be changed to correct that rule. Close governance maintenance when its scoped changes are published and verified; do not turn it into a product audit or recurring historical-document sweep.
- **Protect existing work.** Inspect the working tree before editing. Preserve user changes; never overwrite or discard unrelated work.
- **Keep project guidance accurate.** Documentation must describe actual, verified behavior and remain consistent with implementation.

## 4. Main branch only

The repository has one working branch: `main`. This is an absolute, permanent project policy, not a default or an adaptable preference. All development, governance, documentation, verification, and maintenance work stays on `main`.

- Start and remain on `main` for every task.
- Never create, switch to, publish, or work on another branch, including feature, fix, task, release, or temporary branches.
- Do not use pull requests or merge commits from another branch as a development workflow. Changes are made and committed directly on `main`.
- Do not use another branch as a workaround for repository permissions, branch protection, or a failing check.
- Before making changes, verify the current branch is `main` and inspect its working tree and upstream state.
- Keep all commits on `main`, following applicable commit and signing requirements.
- Never force-push, rewrite shared history, bypass required protections, or disable checks.
- If the checkout is not on `main`, or repository protections or permissions prevent the required direct-main workflow, stop and report the exact blocker. Do not create or switch branches to get around it.

No project document, workflow, contributor instruction, or later project-level policy may authorize work on another branch or merging from another branch. This rule does not authorize bypassing repository security controls; if those controls prevent direct work on `main`, stop.

## 5. Development-cycle mechanism

Ordinary development tasks must use the persisted development-cycle mechanism defined in [docs/dev-life-cycle/README.md](docs/dev-life/README.md). This is mandatory in Operational Mode and is subject only to the governance-maintenance exemption in Section 1.

The lifecycle README, `cycle-index.json`, its schema, and the required plan/handoff or micro-cycle record are authoritative for cycle state and mechanics. The registry is the source of truth for cycle identity, ordering, workflow, status, and phase; plans and handoffs are checkpoints, not competing authorities.

Do not invent an alternative tracker, treat chat history as durable state, or mark a cycle complete without satisfying its documented gates. If lifecycle records are invalid, inconsistent, or missing, stop and follow the lifecycle README's recovery rules.

Steering establishes the mandatory gate and authority. **Delegate** cycle selection, prioritization, transitions, cancellation/blocker handling, checkpoint timing, task-scoped documentation review, bounded audit triggers, verification, and completion mechanics to the lifecycle README rather than duplicating them here.

## 6. Project documentation

Project requirements, architecture, engineering standards, project-specific strategies, operational guidance, and durable decisions belong in maintained documentation—not in this generic steering file or temporary cycle records. The [engineering standards index](docs/standards/README.md) identifies the canonical reusable standards in `docs/standards/`, which define minimum expectations. The project-specific strategies in `docs/project/` select and configure how those standards apply to this repository. Consult [requirements](docs/project/requirements.md), [architecture](docs/project/architecture.md), [coding standard](docs/standards/coding.md), [testing strategy](docs/project/testing-strategy.md), [performance strategy](docs/project/performance-strategy.md), and [Git workflow](docs/project/git-workflow.md).

### Document ownership and boundaries

Each piece of durable project information has one canonical home. Before creating or editing project documentation, classify the information by purpose; do not duplicate it across documents for convenience. Link or reference the canonical source rather than copying its text.

- **Project decisions — what was approved and when:** `docs/project/decisions.md` records concise decision outcomes, states, specification links, and verified provenance. Integrate outcomes into requirements or architecture by topic; do not copy checkpoint text into specifications or duplicate specification content in the ledger.
- **Requirements — what must be true:** Project goals and scope, stakeholder needs, required capabilities and observable behavior, domain/business rules, quality attributes, mandatory constraints, and acceptance criteria. Requirements should be implementation-neutral except where a technology or approach is an explicit constraint. Keep confirmed requirements distinct from assumptions, proposals, and open questions. Use stable requirement IDs where useful.
- **Architecture — how the solution is constructed:** The verified system structure, components and responsibilities, boundaries, interfaces, dependencies, data ownership and flow, significant runtime interactions, infrastructure/integrations, and consequential design decisions and trade-offs. Describe the current implemented architecture; clearly label proposed or future design. Architecture may reference requirement IDs to explain its response but must not copy, redefine, or own requirement statements or acceptance criteria.
- **Coding standard — how implementation is written:** `docs/standards/coding.md` defines reusable code organization, naming, style, maintainability, and implementation-quality expectations. It must align with the project's architecture and must not independently prescribe structural design decisions owned by architecture.
- **Testing standard and strategy — how correctness is verified:** `docs/standards/testing.md` defines generic test-design and evidence rules; `docs/project/testing-strategy.md` defines this project's suites, architecture-to-test map, feedback tiers, triggers, environments, and gates.
- **Performance standard and strategy — how performance is measured:** `docs/standards/performance-engineering.md` defines generic performance-engineering rules; `docs/project/performance-strategy.md` defines this project's workloads, metrics, baselines, targets, environments, and retention.
- **Git workflow standard and policy — how repository collaboration is performed:** `docs/standards/git-workflow.md` defines generic workflow and evidence rules; `docs/project/git-workflow.md` defines this repository's branch/integration model, review, commit, CI, and recovery procedures.
- **Commands and operational documentation — how to perform project-specific tasks:** Verified setup, run, test, build, lint, deployment, maintenance, and troubleshooting procedures, in their appropriate operational documents. Do not include unverified commands.
- **Cycle plans and handoffs — temporary execution context:** Task scope, progress, checkpoints, verification evidence, and next actions. Link to decision IDs; do not duplicate durable decisions.

### Separation rules

- Generic standards define minimum engineering expectations; project strategies select applicable practices and specify actual suites, workflows, commands, workloads, and gates. A strategy may justify inapplicability, but must not silently omit a standard's required assessment.
- Requirements define obligations and outcomes, not technical mechanisms. Do not write an implementation choice as a requirement unless it is an explicit, justified constraint.
- Architecture records technical realization and rationale, not a second version of product requirements.
- Requirements and architecture may be linked by IDs or references. Traceability must not become copied text or parallel maintenance.
- A change to architecture does not automatically imply a requirement change; a change in implementation that preserves required behavior may leave requirements unchanged.
- A requirement change does not automatically determine the architecture; consider the architectural response separately.
- Keep architecture factual and evidence-based. Mark proposals, plans, assumptions, and unknowns explicitly instead of presenting them as implemented.
- If a statement seems to belong in both documents, split it into its obligation (requirements) and technical response (architecture), and link them. If it cannot be meaningfully split, choose one canonical owner and reference it from the other document.
- When reviewing documentation changes, check for overlap, contradictory statements, stale descriptions, and incorrect ownership; remove duplication instead of adding another copy.

Update the existing authoritative documentation when behavior or decisions change. Avoid parallel sources of truth, duplicated explanations, and instructions that contradict implementation.

### Approval-to-artifact gate

Every material user-approved requirement, architecture, interface, or implementation decision follows the [decision approval-to-artifact gate](docs/dev-life-cycle/README.md#decision-approval-to-artifact-gate) in the lifecycle README, which is the single source of truth for the steps and the Proposed/Recorded states. The document-ownership rules above decide which specification owns each kind of decision. `docs/project/decisions.md` is the durable ledger; the handoff is a progress checkpoint only.

## 7. Communication

Communicate status accurately and concisely. Distinguish verified facts from assumptions, proposals, unknowns, and blockers. State what was changed, what verification actually ran and against which revision, what remains unverified, and the next required action.

Do not imply that work is complete because code was written, a commit was created, a plan was filled in, or a CI run was merely started. The cycle registry and its completion gates determine ordinary cycle status.
