# Steering

## 1. Purpose and authority
This file establishes the repository's governing development principles. Its north star is:

1. **Development work is tracked through the persisted development-cycle mechanism, subject to the explicit governance-maintenance exemption below.**
2. **All work is performed on the `main` branch only.**
3. **Development work is coordinated through persisted cycle records and ordinary Git collaboration.**

The detailed lifecycle rules, registry contract, standard and micro cycle workflows, phase machine, plan and handoff formats, verification loop, documentation audits, and completion gates are defined in [docs/dev-life-cycle/README.md](docs/dev-life-cycle/README.md). That document and its registry/schema/templates are the operational specification for the cycle mechanism.

**Governance-maintenance exemption:** Changes limited to steering/lifecycle governance, generic engineering standards under `docs/standards/`, project-strategy templates under `docs/project/` (testing, performance, and Git workflow), the project decision-ledger template, the validator/workflow, or template release metadata/changelog—and intended to amend or repair those governance or adoption rules—are exempt from creating, selecting, resuming, or completing a development cycle. This exemption does not authorize unrelated project changes. It does not, by itself, waive other repository controls such as branch, security, or access requirements. If a change also includes ordinary project work, that work must be tracked through the normal cycle mechanism.

**User authority and policy changes:** User instructions govern the requested outcome. A user may explicitly authorize a change to this steering policy or lifecycle specification. When the request is to amend these governance files, apply the governance-maintenance exemption above rather than requiring a cycle to change the rules that define cycles. For ordinary work, follow the lifecycle selection and priority rules in the lifecycle README. If project sources disagree, identify the conflict and resolve it explicitly; do not silently weaken unrelated requirements.

Apply these rules alongside the project's established requirements. Project-specific requirements and constraints must be established from project evidence. Never invent project-specific facts or silently import another project's commands, paths, architecture, platforms, dependencies, or quality requirements.

## 1A. Non-bypassable operating gate

The written lifecycle is an execution control, not optional guidance. Apply it before ordinary project work and after every user approval:

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

Ordinary development tasks must be managed through the persisted development-cycle mechanism. Standard and eligible micro workflows require a persisted registry record, main-only work, and truthful verification. This template assumes a single maintainer and one agent working at a time: at most one cycle is `in-progress`, and every other open cycle is pending, paused, or blocked.

The registry at `docs/dev-life-cycle/cycle-index.json` is authoritative for cycle identity, ordering, workflow, status, and current phase. Standard cycles require their plan and handoff files; micro cycles record their objective, eligibility rationale, acceptance condition, verification evidence, and outcome directly in the registry. Progress is coordinated through the persisted registry, handoffs, and ordinary Git history.

The governance-maintenance exemption in Section 1 applies when changing `steering.md` or the lifecycle mechanism itself. Do not create a cycle for such a self-maintenance change.

The lifecycle README is the single source of truth for cycle selection, prioritization, phase transitions, plan/handoff maintenance, progress checkpoints, verification, documentation audits, and completion gates.

Do not invent an alternative tracker, treat chat history as durable state, or mark a cycle complete without satisfying its documented gates. If lifecycle records are invalid or inconsistent, follow the lifecycle README's recovery rules.

**Cancellation and blockers:** Standard cycles may be paused, blocked, cancelled, or completed. A blocked cycle must include a concrete blocker and the action/owner needed to unblock it. Cancelled development cycles are terminal and retain their registry records; never delete or reuse a cycle record. This applies to lifecycle records, not GitHub Actions workflow runs, whose cancelled history is deleted under the Git Workflow Standard. Micro cycles that become blocked, cancelled, or need preemption must first be promoted to standard with plan/handoff records.

**Current-request priority:** The user's current request determines which cycle is worked on; it does not need to be marked urgent. If it differs from the in-progress cycle, checkpoint and pause that cycle first, then start or resume the cycle for the new request. When that work completes, resume paused cycles (lowest execution order first) before pending work, unless the user directs otherwise. If it is unclear whether a request continues an open cycle, ask one short question rather than guessing.

**Progress checkpoints:** Persist a handoff/registry checkpoint at each of these events: a material decision or investigation result, a phase transition, and before any pause, context switch, waiting period, or session end. Do not checkpoint on a timer. Take every timestamp from the system clock rather than estimating it. A checkpoint is evidence of state, not an ownership claim.

## 6. Project documentation

Project requirements, architecture, engineering standards, project-specific strategies, operational guidance, and durable decisions belong in maintained documentation—not in this generic steering file or temporary cycle records. The generic standards in `docs/standards/` define reusable minimum expectations. The project-specific strategies in `docs/project/` select and configure how those standards apply to this repository. Consult [requirements](docs/project/requirements.md), [architecture](docs/project/architecture.md), [coding standard](docs/standards/coding.md), [testing strategy](docs/project/testing-strategy.md), [performance strategy](docs/project/performance-strategy.md), and [Git workflow](docs/project/git-workflow.md).

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
