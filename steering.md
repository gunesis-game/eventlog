# Steering

## 1. Purpose and authority
This file establishes the repository's governing development principles. Its north star is:

1. **Development work is tracked through the persisted development-cycle mechanism, subject to the explicit governance-maintenance exemption below.**
2. **All work is performed on the `main` branch only.**
3. **Contributors may work concurrently; there is no repository-wide execution lock or single-worker restriction.**

The detailed lifecycle rules, registry contract, standard and micro cycle workflows, phase machine, plan and handoff formats, verification loop, documentation audits, and completion gates are defined in [docs/dev-life-cycle/README.md](docs/dev-life-cycle/README.md). That document and its registry/schema/templates are the operational specification for the cycle mechanism.

**Governance-maintenance exemption:** Changes whose scope is limited to `steering.md` and/or the development-cycle mechanism under `docs/dev-life-cycle/`, and whose purpose is to amend those governance or lifecycle rules, are exempt from creating, selecting, resuming, or completing a development cycle. This exemption applies only to those governance files and does not authorize unrelated project changes. It does not, by itself, waive other repository controls such as branch, security, or access requirements. If a change also includes ordinary project work, that work must be tracked through the normal cycle mechanism.

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
- **Enforce approval-to-artifact governance.** After every material user approval, update the canonical artifact, publish it, fetch it back and verify its exact revision, then mark the decision Recorded in the current handoff before introducing another independent decision. If recording or verification fails, stop and report the approved-but-unrecorded decision as a blocker.
- **Keep evidence honest.** Never claim that a change, test, build, CI run, review, or deployment succeeded unless it was performed and its result observed.
- **Control scope.** Do not expand the task or introduce tools, infrastructure, or policy changes without a clear need and appropriate authorization.
- **Protect existing work.** Inspect the working tree before editing. Preserve user changes; never overwrite or discard unrelated work.
- **Keep project guidance accurate.** Documentation must describe actual, verified behavior and remain consistent with implementation.

## 4. Main branch only

The `main` branch is the default working branch and policy for this template. Product teams may adopt another branch/review model only through an explicit governance change in their derived repository: document the policy, update this section and reconcile the review and verification rules to that model. Do not silently fork the mechanism or bypass branch protections.

- Start development on `main` and keep the working branch set to `main` throughout the work.
- Do not create, switch to, or work on feature, fix, task, release, or other branches.
- Do not use a branch as a workaround for repository permissions, branch protection, or a failing check.
- Before making changes, verify the current branch is `main` and inspect its working tree and upstream state.
- Keep commits on `main`, following the repository's applicable commit and signing requirements.
- Never force-push, rewrite shared history, bypass required protections, or disable checks.
- If `main` is unavailable, the checkout is on another branch, or repository protections prevent the required direct-main workflow, stop and report the exact blocker.

The main-only rule does not authorize bypassing repository security, review, or protection controls.

## 5. Development-cycle mechanism

Ordinary development tasks must be managed through the persisted development-cycle mechanism. Standard and eligible micro workflows require a persisted registry record, main-only work, and truthful verification. There is deliberately no repository-wide execution lock, worker claim, heartbeat, lease, or single-worker restriction. Multiple cycles and contributors may be active concurrently.

The registry at `docs/dev-life-cycle/cycle-index.json` is authoritative for cycle identity, ordering, workflow, status, and current phase. Standard cycles require their plan and handoff files; micro cycles record their objective, eligibility rationale, acceptance condition, verification evidence, and outcome directly in the registry. Contributors coordinate through ordinary Git collaboration, review, and conflict resolution; no contributor has exclusive ownership of the repository.

The governance-maintenance exemption in Section 1 applies when changing `steering.md` or the lifecycle mechanism itself. Do not create a cycle for such a self-maintenance change.

The lifecycle README is the single source of truth for cycle selection, prioritization, phase transitions, plan/handoff maintenance, progress checkpoints, verification, documentation audits, and completion gates.

Do not invent an alternative tracker, treat chat history as durable state, or mark a cycle complete without satisfying its documented gates. If lifecycle records are invalid or inconsistent, follow the lifecycle README's recovery rules.

**Cancellation and blockers:** Standard cycles may be paused, blocked, cancelled, or completed. A blocked cycle must include a concrete blocker and the action/owner needed to unblock it. Cancelled cycles are terminal and retain their record. Never delete or reuse a cycle record. Micro cycles that become blocked, cancelled, or need preemption must first be promoted to standard with plan/handoff records.

**Urgent-work preemption:** A user explicitly marks a request urgent or directs it to take priority. Checkpoint the active cycle and pause it before starting urgent work. Urgent work starts next, ahead of paused/pending cycles. Once it completes, resume the paused cycle before pending work. Ordinary requests do not preempt unless urgency is explicit.

**Progress checkpoints:** While actively working, persist a handoff/registry checkpoint after every material decision or investigation and at least every 15 minutes during uninterrupted work. Before an extended pause, waiting period, context switch, or session end, checkpoint and update the cycle status as appropriate. A checkpoint is evidence of state, not an ownership claim.

## 6. Project documentation

Maintain documentation by subject ownership; do not use one document as a catch-all or move substantive content into history merely to reduce file size.

- **Requirements** ([docs/project/requirements.md](docs/project/requirements.md)) own product purpose, capabilities, observable behavior, domain rules, constraints, quality expectations, and acceptance criteria. Keep them implementation-neutral unless a technology or implementation constraint is itself an explicit requirement.
- **Architecture** ([docs/project/architecture.md](docs/project/architecture.md)) owns technical design: components, responsibilities, boundaries, interfaces, algorithms, data/storage representations, lifecycle, and trade-offs. It must satisfy requirements, not redefine them.
- **Coding standards** ([docs/project/coding-standards.md](docs/project/coding-standards.md)) own implementation conventions, code organization, naming, quality, and maintainability within the approved architecture. They must not introduce product or architecture decisions.
- **Commands** ([docs/project/commands.md](docs/project/commands.md)) owns verified setup, build, test, lint, and validation commands, including prerequisites and expected outcomes. Never list unverified commands as project guidance.
- **Cycle plans and handoffs** capture temporary execution scope, progress, checkpoints, proposals awaiting decisions, verification evidence, and next steps. They are not the sole home of durable requirements or approved design.

Keep information in its authoritative owner and prefer concise cross-references over duplicating details. Clearly distinguish approved decisions, proposals, unresolved questions, verified implementation facts, and historical provenance. Do not present intended design as implemented behavior or test evidence. When requirements or architecture change, update the owning document and reconcile affected documents and cycle records. Preserve substantive approved design in active architecture documentation; archives are provenance, not a substitute for the current specification.

## 7. Communication

Communicate status accurately and concisely. Distinguish verified facts from assumptions, proposals, unknowns, and blockers. State what was changed, what verification actually ran and against which revision, what remains unverified, and the next required action.

Do not imply that work is complete because code was written, a commit was created, a plan was filled in, or a CI run was merely started. The cycle registry and its completion gates determine ordinary cycle status.

## 8. Active-work checkpoints

Cycle records and handoffs record progress; they do not grant exclusive ownership. Contributors may work concurrently and coordinate through ordinary Git collaboration.

While actively working, update the current handoff or micro-cycle registry after every material decision, completed investigation, implementation checkpoint, blocker/next-action change, or verification result, and at least every 15 minutes during uninterrupted work. Each checkpoint records the exact phase, established/changed state, remaining work, verification performed or not performed, and one next action. Do not create cosmetic activity solely to refresh a timestamp.

Before an extended pause, while waiting for a response, on context switch, or before ending a session, checkpoint the work and mark a standard cycle `paused` or `blocked` as appropriate. Do not hold the lock while waiting for a user or external response or while no development work is occurring. A blocked cycle records its concrete blocker and unblocking action.

Preserve cycle history and checkpoint the actual state before another contributor continues. Resolve overlapping edits through ordinary Git collaboration; do not assume that a cycle record grants exclusive ownership.
