# Steering

## 1. Purpose and authority
This file establishes the repository's governing development principles. Its north star is:

1. **Ordinary development work is tracked through the persisted development-cycle mechanism.**
2. **All work is performed on the main branch only.**
3. **Contributors coordinate through direct main-branch collaboration, GitHub MCP, and verifiable GitHub Actions evidence.**

The detailed lifecycle rules, registry contract, standard and micro cycle workflows, phase machine, plan and handoff formats, verification loop, documentation audits, and completion gates are defined in [docs/dev-life-cycle/README.md](docs/dev-life-cycle/README.md). That document and its registry/schema/templates are the operational specification for the cycle mechanism. Reusable engineering standards are maintained centrally in [docs/standards/README.md](docs/standards/README.md); project-specific policies and strategies are maintained in their owning docs/project documents.

**Governance-maintenance exemption:** Changes confined to steering.md; docs/dev-life-cycle/README.md; docs/standards/**; docs/project/decisions.md, docs/project/testing-strategy.md, docs/project/performance-strategy.md, and docs/project/git-workflow.md; README.md and TEMPLATE_VERSION; scripts/validate-steering.py and scripts/cleanup-workflow-runs.py; tests/test_cleanup_workflow_runs.py; and the steering-validation and workflow-history-cleanup workflows, when made to amend or repair governance itself, are applied directly and are exempt from creating, selecting, resuming, or completing a development cycle. This exemption does not cover project requirements, architecture, acceptance criteria, benchmark workload definitions, implementation, or unrelated documentation; those remain subject to the ordinary development process. It does not waive applicable branch, security, access, or repository controls.

**User authority and policy changes:** User instructions govern the requested outcome. A user may explicitly authorize a change to this steering policy or lifecycle specification. When the request is to amend these governance files, apply the governance-maintenance exemption above rather than requiring a cycle to change the rules that define cycles. For ordinary work, follow the lifecycle selection and priority rules in the lifecycle README. If project sources disagree, identify the conflict and resolve it explicitly; do not silently weaken unrelated requirements.

Apply these rules alongside the project's established requirements. Project-specific requirements and constraints must be established from project evidence. Never invent project-specific facts or silently import another project's commands, paths, architecture, platforms, dependencies, or quality requirements.

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
- **Enforce approval-to-artifact governance.** For a material project decision, integrate the approved outcome into its owning specification and record its provenance in docs/project/decisions.md. Publish and fetch both back, verify their exact revisions, and update the active handoff only with the resulting progress and next action. If recording or verification fails, stop and report the approved-but-unrecorded decision as a blocker. Governance-policy changes are applied directly to their owning governance files and are not entered in the project decision ledger.

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

Ordinary development tasks must be managed through the persisted development-cycle mechanism. Standard and eligible micro workflows require a persisted registry record, main-only work, and truthful verification. Multiple cycles and contributors may be active concurrently.

The registry at `docs/dev-life-cycle/cycle-index.json` is authoritative for cycle identity, ordering, workflow, status, and current phase. Standard cycles require their plan and handoff files; micro cycles record their objective, eligibility rationale, acceptance condition, verification evidence, and outcome directly in the registry. Contributors coordinate through ordinary Git collaboration, review, and conflict resolution.

The governance-maintenance exemption in Section 1 applies to changes confined to the listed governance files. Apply those changes directly without creating or advancing a development cycle.

The lifecycle README is the single source of truth for cycle selection, prioritization, phase transitions, plan/handoff maintenance, progress checkpoints, verification, documentation audits, and completion gates.

Do not invent an alternative tracker, treat chat history as durable state, or mark a cycle complete without satisfying its documented gates. If lifecycle records are invalid or inconsistent, follow the lifecycle README's recovery rules.

**Cancellation and blockers:** Standard cycles may be paused, blocked, cancelled, or completed. A blocked cycle must include a concrete blocker and the action/owner needed to unblock it. Cancelled cycles are terminal and retain their record. Never delete or reuse a cycle record. Micro cycles that become blocked, cancelled, or need preemption must first be promoted to standard with plan/handoff records.

**Urgent-work preemption:** A user explicitly marks a request urgent or directs it to take priority. Checkpoint the active cycle and pause it before starting urgent work. Urgent work starts next, ahead of paused/pending cycles. Once it completes, resume the paused cycle before pending work. Ordinary requests do not preempt unless urgency is explicit.

**Progress checkpoints:** While actively working, persist a handoff/registry checkpoint after every material decision or investigation and at least every 15 minutes during uninterrupted work. Before an extended pause, waiting period, context switch, or session end, checkpoint and update the cycle status as appropriate. A checkpoint is evidence of state, not an ownership claim.

## 6. Project documentation

Maintain documentation by subject ownership; do not use one document as a catch-all or move substantive content into history merely to reduce file size.

- **Requirements** ([docs/project/requirements.md](docs/project/requirements.md)) own product purpose, capabilities, observable behavior, domain rules, constraints, quality expectations, and acceptance criteria. Keep them implementation-neutral unless a technology or implementation constraint is itself an explicit requirement.
- **Architecture** ([docs/project/architecture.md](docs/project/architecture.md)) owns technical design: components, responsibilities, boundaries, interfaces, algorithms, data/storage representations, lifecycle, and trade-offs. It must satisfy requirements, not redefine them.
- **Coding standard** ([docs/standards/coding.md](docs/standards/coding.md)) owns reusable implementation conventions, code organization, naming, quality, and maintainability within the approved architecture. It must not introduce product or architecture decisions. The canonical home for all reusable engineering standards is docs/standards/; do not maintain duplicate copies under docs/project/.
- **Testing standard and strategy** ([docs/standards/testing.md](docs/standards/testing.md) and [docs/project/testing-strategy.md](docs/project/testing-strategy.md)) define reusable test-quality expectations and this project's actual test ownership, tiers, gates, and evidence.
- **Performance standard and strategy** ([docs/standards/performance-engineering.md](docs/standards/performance-engineering.md) and [docs/project/performance-strategy.md](docs/project/performance-strategy.md)) define reusable performance-engineering expectations and this project's workloads, metrics, targets, and evidence policy.
- **Git workflow standard and policy** ([docs/standards/git-workflow.md](docs/standards/git-workflow.md) and [docs/project/git-workflow.md](docs/project/git-workflow.md)) define reusable repository-workflow expectations and this repository's mandatory main-only operating model, GitHub MCP protocol, CI evidence, and run retention.
- **Commands** ([docs/project/commands.md](docs/project/commands.md)) owns verified setup, build, test, lint, and validation commands, including prerequisites and expected outcomes. Never list unverified commands as project guidance.
- **Project decisions** (docs/project/decisions.md) own the durable decision register and provenance. Requirements and architecture remain the curated normative specifications.
- **Cycle plans and handoffs** capture temporary execution scope, progress, verification evidence, blockers, and next steps. They link to durable decisions rather than duplicating the decision register.

Keep information in its authoritative owner and prefer concise cross-references over duplicating details. Curate approved outcomes into the relevant requirements or architecture topic; do not append raw checkpoints or handoff narratives to those specifications. The decision ledger records concise outcomes and provenance, not a second specification. Clearly distinguish approved decisions, proposals, unresolved questions, verified implementation facts, and historical provenance. Do not present intended design as implemented behavior or test evidence. When project specifications change, reconcile only the documents affected by that change.

## 7. Communication

Communicate status accurately and concisely. Distinguish verified facts from assumptions, proposals, unknowns, and blockers. State what was changed, what verification actually ran and against which revision, what remains unverified, and the next required action.

Do not imply that work is complete because code was written, a commit was created, a plan was filled in, or a CI run was merely started. The cycle registry and its completion gates determine ordinary cycle status.

## 8. Active-work checkpoints

Cycle records and handoffs record progress. Contributors coordinate through ordinary Git collaboration.

While actively working, update the current handoff or micro-cycle registry after every material decision, completed investigation, implementation checkpoint, blocker/next-action change, or verification result, and at least every 15 minutes during uninterrupted work. Each checkpoint records the exact phase, established/changed state, remaining work, verification performed or not performed, and one next action. Do not create cosmetic activity solely to refresh a timestamp.

Before an extended pause, while waiting for a response, on context switch, or before ending a session, checkpoint the work and mark a standard cycle `paused` or `blocked` as appropriate. A blocked cycle records its concrete blocker and unblocking action.

Preserve cycle history and checkpoint the actual state before another contributor continues. Resolve overlapping edits through ordinary Git collaboration.
