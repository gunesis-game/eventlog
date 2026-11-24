# Persisted development lifecycle

This directory contains the durable execution state for ordinary development work. Use of this mechanism is mandatory for ordinary project work; the narrowly scoped governance-maintenance exemption is defined below.

The mechanism is deliberately project-agnostic: project-specific objectives, architecture, commands, quality gates, branch-protection details, and deployment constraints belong in the repository's own documentation and configuration. The repository-wide main-only and single-worker requirements are fixed by `steering.md`.

## Governance-maintenance exemption

Changes whose scope is limited to `steering.md` and/or files under `docs/dev-life-cycle/`, and whose purpose is to amend or repair the steering policy or development-cycle mechanism itself, do not create or resume a cycle and do not claim the execution lock. This explicit self-maintenance exemption prevents recursive cycles and avoids making the lifecycle mechanism depend on a mechanism that it is currently modifying.

Keep such changes strictly limited to those governance files. If ordinary project files are also changed, that ordinary work must use the normal cycle process. The exemption from cycle tracking and the lock does not waive other repository controls.

## Authoritative files

- `cycle-index.json` is the single authoritative registry for cycle identity, order, workflow, status, and current phase.
- `cycle-index.schema.json` defines the registry's structural JSON contract.
- `execution-lock.json` is the repository-wide authoritative exclusive execution lock for standard and micro cycles; `execution-lock.schema.json` defines its structural contract.
- `templates/PLAN.md` is copied into each standard cycle's plan.
- `templates/HANDOFF.md` is copied into each standard cycle's handoff.

Standard-cycle structure:

```text
docs/dev-life-cycle/
  README.md
  cycle-index.json
  cycle-index.schema.json
  execution-lock.json
  templates/
    PLAN.md
    HANDOFF.md
  <cycle-uuid>/
    plan/
      PLAN.md
    handoff/
      HANDOFF.md
```

Micro-cycles have no per-cycle plan or handoff directory. Their required compact record is stored directly in `cycle-index.json`. Do not store project-wide architecture or permanent requirements in cycle plans or handoffs.

## Registry contract

The registry uses schema version 3. Template adoption begins at schema v3; this document intentionally contains no historical schema migration procedure. Upstream template-version adoption and schema migration guidance belongs in the template repository's release/change notes. A new repository starts with:

```json
{
  "schema_version": 3,
  "docs_reviewed_through_execution_order": 0,
  "cycles": []
}
```

Each cycle record contains:

| Field | Meaning |
|---|---|
| `id` | Unique UUID. It names the cycle directory for standard cycles. |
| `sequence` | Immutable, positive creation-order number. |
| `execution_order` | Immutable, positive number assigned when the cycle starts; null while pending. |
| `workflow` | `standard` or `micro`. |
| `description` | Concise objective for the cycle. |
| `status` | `pending`, `in-progress`, `paused`, `blocked`, `cancelled`, or `complete`; micro cycles may only be in-progress or complete. |
| `created_at` | UTC ISO-8601 creation time, or null if historically unknown. |
| `started_at` | UTC ISO-8601 first-start time; null until first started. |
| `completed_at` | UTC ISO-8601 verified completion time; null until complete. |
| `current_phase` | Standard phase, or `implement` / `complete` for micro cycles. |
| `status_reason` | Non-null, concise blocker or cancellation reason for `blocked`/`cancelled`; null otherwise. |
| `eligibility_reason` | For micro cycles, the concise reason the task meets every micro eligibility rule; null for standard cycles. |
| `acceptance_criteria` | For micro cycles, the single observable condition that defines completion; null for standard cycles. |
| `verification_evidence` | For completed micro cycles, the focused check performed and its observed result; null until completion and for standard cycles. |
| `outcome` | For completed micro cycles, the actual result; null until completion and for standard cycles. |

### Cross-record invariants

JSON Schema validates record shape. The lifecycle controller must additionally validate:

1. Every cycle ID is a unique UUID. Every standard cycle has its plan and handoff files; micro cycles have neither and do not need empty directories.
2. Every `sequence` is a unique positive integer. Sequence values are never changed or reused.
3. Every non-null `execution_order` is a unique positive integer. Pending standard cycles have null execution order; once assigned, it never changes.
4. At most one cycle has status `in-progress`; any number of standard cycles may be `paused` or `blocked`. Blocked cycles do not hold the execution lock.
5. A pending standard cycle has phase `assess`, null execution_order, null started_at, null completed_at, and null status_reason.
6. An in-progress standard cycle has a non-null execution_order and started_at, a phase other than `complete`, and null completed_at.
7. A paused standard cycle has a non-null execution_order and started_at, a resumable phase other than `complete`, and null completed_at/status_reason. Pausing never clears or changes its execution_order, ID, sequence, or timestamps; its checkpoint and pause context belong in the handoff.
8. A blocked standard cycle has non-null execution_order and started_at, a resumable phase, null completed_at, and a non-empty status_reason; it must release the lock. Unblock by updating the reason/checkpoint and explicitly resuming it through the normal claim protocol.
9. A cancelled standard cycle is terminal, has a non-empty status_reason, null completed_at, and retains either both execution_order/started_at (if started) or neither (if cancelled while pending). Cancelled history is never deleted or resumed.
10. A complete standard cycle has phase `complete`, non-null execution_order, started_at, and completed_at.
11. A micro cycle is never pending, paused, blocked, or cancelled. An in-progress micro cycle has phase `implement`, non-null execution_order and started_at, and null completed_at, verification_evidence, and outcome/status_reason. A complete micro cycle has phase `complete`, non-null execution_order, started_at, completed_at, verification_evidence, and outcome, with status_reason null. If a micro cycle is blocked, cancelled, or preempted, promote that same cycle to standard and create its plan/handoff first; retain the micro evidence as historical context.
12. Every micro cycle has a non-empty eligibility_reason and acceptance_criteria. Standard cycles set the four micro-only evidence fields to null.
13. Timestamp ordering is consistent where values are known: created_at is not after started_at; started_at is not after completed_at.
14. `schema_version` is supported. Unknown versions or invalid records must be surfaced, not silently rewritten.
15. Completed and cancelled records remain in the registry. Never delete history to make the registry look tidy.

The array order among pending standard cycles determines their default priority. Paused cycles retain their original execution_order and resume ahead of ordinary pending work, ordered by their original execution_order, unless the user explicitly directs a different priority. Blocked cycles are skipped until explicitly unblocked. Cancelled cycles are terminal. Reordering pending cycles requires an explicit reason and must not alter sequence or execution_order. Micro cycles start immediately and receive the next unused execution_order.

## Template version adoption

This template starts new product repositories at schema v3. Do not carry a historical migration section into derived repositories. If adopting a newer template, compare the upstream `TEMPLATE_VERSION`, review its release/change notes, and apply any schema migration deliberately while preserving cycle IDs, sequence/execution history, timestamps, and current lock ownership. The template README describes the adoption procedure.

## Choosing the workflow

Use a **micro-cycle only when every condition below is true**:

- The change is trivial, tightly bounded, and low risk.
- It is cosmetic or editorial only, such as correcting a typo, spelling, punctuation, or unambiguous formatting defect.
- It does not change runtime behavior, program logic, user-visible product behavior, architecture, interfaces, APIs, data, security posture, configuration, dependencies, migrations, or operational procedures.
- It does not introduce or alter a requirement, design decision, or substantive technical explanation.
- One concise acceptance condition and one focused verification check are sufficient.
- The task can be completed in one short implementation pass without unresolved design choices.

Examples that may qualify: correcting an obvious typo in a comment or documentation sentence without changing meaning; fixing whitespace or formatting in a non-executable text file.

**Use a standard cycle whenever any condition is not met or eligibility is uncertain.** Features, defects, refactors, behavior changes, tests, configuration changes, dependency updates, migrations, security changes, substantive documentation work, and multi-step investigations always use a standard cycle. Do not split a larger task into micro-cycles to avoid standard planning.

If a micro-cycle expands, reveals ambiguity or risk, or fails its narrow acceptance condition, stop the abbreviated path and promote the same cycle to standard: change `workflow` to `standard`, create its plan and handoff from the templates, populate the discovered context and remaining scope, set the appropriate standard phase (normally `define`), and continue under the standard phase machine. Preserve the cycle ID, sequence, execution_order, and history; do not create a parallel cycle for the same objective.

## Creating a standard cycle

Create a standard cycle for any ordinary development task not eligible for the micro path.

1. Generate a candidate UUID and determine the next unused sequence number in memory; do not write repository files yet.
2. Claim the repository-wide execution lock for that UUID with workflow `standard` before making any project or lifecycle changes.
3. Add a registry record with workflow `standard`, status `pending`, execution_order null, current_phase `assess`, and the four micro-only evidence fields set to null.
4. Create the UUID directory with `plan/PLAN.md` and `handoff/HANDOFF.md` from the templates.
5. Describe the objective, scope, non-goals, acceptance criteria, and next action in the plan.
6. Keep the handoff as the single current checkpoint for that cycle.
7. When the pending cycle first starts, assign the next unused execution_order and started_at, change status to `in-progress`, and preserve both values permanently.

## Creating a micro-cycle

A micro-cycle is still a complete, persisted development cycle. It omits per-cycle plan and handoff files only because its narrow scope is fully captured by the registry fields.

1. Confirm every micro eligibility condition. If uncertain, choose a standard cycle instead.
2. Allocate a candidate UUID, sequence, and execution_order in memory; do not write repository files yet.
3. Claim the repository-wide execution lock for that UUID with workflow `micro`.
4. Append a registry record with workflow `micro`, status `in-progress`, current_phase `implement`, created_at and started_at set, completed_at null, a concise eligibility_reason, and one acceptance_criteria statement. Set verification_evidence and outcome to null.
5. Only after that in-progress record is persisted may the bounded change begin.
6. Perform the smallest focused verification that proves the acceptance condition. Inspect the exact diff and confirm no unrelated or semantic changes slipped in.
7. Update the same registry record to status `complete`, current_phase `complete`, set completed_at, and record concise verification_evidence and outcome. Do not claim checks that were not actually run.
8. Release the execution lock. The registry record is the durable micro-cycle record; do not create empty plan/handoff files or a narrative substitute.

The micro path compresses administrative phase transitions into one persisted in-progress record and one completion update. It does not skip assessment, scope control, implementation review, verification, or reconciliation; those checks are performed as one disciplined short pass and evidenced in the registry.

## Resuming, prioritizing, and preempting work

At the start of every ordinary development task:

1. **Read `execution-lock.json` first, before reading other repository files or performing any task analysis, including read-only analysis or review.** This first check is mandatory even when the request appears small or belongs to a different cycle.
2. If the lock is `active`, stop immediately. Do not inspect the registry, plans, source, tests, or other project content; do not perform task analysis. Wait for the owner to release it or obtain explicit user/repository-owner confirmation that a terminated worker is gone. Never clear an active claim based on elapsed time or assumption.
3. If the lock is `released`, continue with read-only discovery: read `steering.md` and this README, parse and validate `cycle-index.json` against its JSON Schema and cross-record invariants, and inspect the relevant cycle plan/handoff and repository state. Read-only discovery does not authorize implementation or registry/project-file changes.
4. **Check whether the user has explicitly identified the new request as urgent or directed that it take priority over existing work.** If so, the urgent request takes priority over pending cycles and may preempt the currently in-progress cycle. Do not force it to wait behind unrelated pending work.
5. If urgent preemption is requested and a different cycle is in progress, first create a durable checkpoint in that cycle's handoff (standard) or registry record (micro), recording its exact phase, completed work, outstanding work, verification state, and next action. Then release its execution lock with accurate release evidence and set its status to `paused`. Preserve its ID, sequence, execution_order, timestamps, history, and checkpoint. For a standard cycle, retain its phase and plan/handoff. For a micro cycle, promote that same cycle to standard, create its plan/handoff, preserve its micro evidence as historical context, clear micro-only fields as required for a standard record, and then mark it paused. Never have two cycles in progress.
6. Select the urgent request's cycle next. If no urgent preemption applies and one cycle is in progress, resume it. Otherwise, resume the paused standard cycle with the lowest execution_order before selecting ordinary pending work. If no cycle is in progress or paused, select the first pending standard cycle in registry order, skipping blocked and cancelled cycles. A blocked cycle resumes only after the blocker is explicitly cleared and its status/checkpoint is updated. If none is eligible, assess the request and choose standard or micro workflow using the eligibility rules above.
7. Once the selected cycle ID and workflow are known, claim the lock **before changing the registry or any project file**. Persist the active lock as a commit on `main`, then push that commit to the shared `origin/main`. Do not begin implementation, update the registry, or make any other repository write until the claim commit is confirmed present on the shared remote. When starting a pending standard cycle, assign its next execution_order and started_at and mark it in-progress only after the published claim is confirmed. A preempting urgent request receives the next execution_order when it starts; do not rewrite prior execution-order history.
8. For a standard cycle, read its plan and handoff. For a micro cycle, use the registry's objective, eligibility_reason, acceptance_criteria, and persisted phase as the checkpoint.
9. Verify all persisted state against the actual repository, branch, working tree, implementation, tests, and CI; records are checkpoints, not unquestionable truth.
10. Continue from the persisted phase. Never restart at assessment simply because the session changed.

An urgent request does not automatically cancel or discard existing work. Preemption checkpoints and marks the active standard cycle `paused`; after the urgent cycle completes and releases the lock, the paused cycle is resumed before ordinary pending work. Cancelling is a separate explicit user/owner decision; record the reason and retain the terminal record. Blocking is for work that cannot proceed but is expected to resume; record the blocker and unblock action in status_reason plus the handoff. Pending-cycle order remains the default when no explicit urgency/preemption instruction applies.

If the registry, lock, or required standard-cycle records are invalid, contradictory, or missing, stop lifecycle mutation, report the exact inconsistency, and repair it deliberately. Do not infer intended state from chat or folder names.

## Pause, block, unblock, and cancel

- **Pause** means work is intentionally deferred. Checkpoint in the standard handoff, set status `paused`, retain the phase and historical order/timestamps, and release the lock.
- **Block** means work cannot proceed until an external dependency, decision, access, or other impediment is resolved. Set status `blocked`, retain phase/order/start timestamp, set `status_reason` to the concise blocker and unblocking action, update the handoff, and release the lock. A blocked cycle is not selected automatically.
- **Unblock** only after the stated blocker is resolved or explicitly changed. Update `status_reason` to null, document evidence/checkpoint, and resume through the ordinary lock-claim procedure; status becomes `in-progress`. Do not hold the lock while blocked.
- **Cancel** is an explicit terminal decision, not a synonym for pause or block. Set status `cancelled`, provide a `status_reason`, leave `completed_at` null, and preserve the record. A pending cycle remains assess with no execution order or start timestamp; a started cycle keeps both. Never resume a cancelled cycle; if the work is needed again, create a new cycle.
- **Micro-cycle interruption:** before pausing, blocking, cancelling, or urgent preemption, promote the same micro cycle to standard, create its plan and handoff, retain the compact micro record as context, and then update its status. Micro cycles may not have interrupted states.

The lock and registry must agree: when released, no cycle may be `in-progress`; when active, it must match the sole in-progress cycle's ID and workflow. The validator enforces this.

## Exclusive execution: one worker at a time

Development work is strictly serialized across the entire repository, not merely within an individual cycle. **At most one agent or human contributor may actively perform development work at any time.** Do not run parallel implementation agents, delegate simultaneous edits, or allow another contributor to modify files, lifecycle records, or commits while a worker owns execution.

`execution-lock.json` is the single authoritative repository-wide lock for both standard and micro cycles. Its contract is:

- `schema_version`: currently `1`.
- `status`: `released` or `active`.
- `cycle_id`: UUID of the cycle holding the lock, null when released.
- `workflow`: `standard` or `micro` while active, null when released.
- `worker_id`: agent/contributor identifier while active, null when released.
- `claimed_at`: UTC ISO-8601 claim time while active, null when released.
- `released_at`: null while active; UTC ISO-8601 release time after a normal release.
- `release_evidence`: null while active; concise handoff/release evidence after a normal release.

A new repository starts with a released lock and null owner fields:

```json
{
  "schema_version": 1,
  "status": "released",
  "cycle_id": null,
  "workflow": null,
  "worker_id": null,
  "claimed_at": null,
  "released_at": null,
  "release_evidence": null
}
```

### Publishing a claim (required)

A claim is not effective merely because it exists in a local working tree or an unpushed commit. The claim must be visible on the shared remote before any worker starts development work.

1. Confirm the lock is released and determine the selected cycle ID and workflow using the start/resume procedure above.
2. On `main`, change the lock to `active` and fill in the cycle ID, workflow, worker ID, and claim timestamp. Do not modify the registry or any other project file in this claim commit.
3. Commit the lock change and push it to `origin/main` using a normal, non-force push. Do not use another branch, force-push, rewrite history, or bypass repository protections.
4. Confirm that the shared `origin/main` contains the exact claim commit and that its lock still names this worker and cycle. Only then may the worker write to the registry or project files and begin implementation.
5. If the push is rejected, treat the claim attempt as lost to a competing writer. Stop; do not rebase the claim commit, force-push, retry the stale update, or proceed using the local active lock. Fetch the latest `main` and re-read the lock before any further action. If it is active, follow the active-lock stop rule. If it is released, restart the entire claim attempt from the latest remote state with a fresh claim commit.

This relies on Git's shared branch update being a compare-and-swap through normal fast-forward push semantics: when two workers build claims from the same released `main` tip, only one can advance the remote branch; the other must be rejected as non-fast-forward. Never use a file-only or local-only claim as evidence of exclusive ownership. If the repository's hosting or push workflow cannot provide this shared-ref race protection, stop and report that the lock cannot safely coordinate separate checkouts until an atomic claim mechanism is provided.

Before stopping or handing off, persist the current cycle checkpoint, then set status to released, clear the owner fields, and record release time/evidence. Commit and push that release to `origin/main`; do not report the lock as released until the shared remote confirms it. The lock is the authority; standard-cycle handoffs may describe the same checkpoint but do not supersede the lock.

If a worker terminates unexpectedly and the lock remains active, stop. Obtain explicit confirmation from the user or responsible repository owner that the previous worker is no longer working before releasing the lock. Never clear or overwrite another worker's claim based on assumption, elapsed time, or lack of a visible session. The explicit preemption procedure above is the authorized handoff path when the user directs urgent work to take priority; it still requires a durable checkpoint and serialized ownership transfer.

This rule blocks all other ordinary development work, including work on another cycle and read-only task analysis/review, while a worker owns the lock. Governance-maintenance changes covered by the self-maintenance exemption do not claim or require this lock.

## Standard-cycle phase machine

The registry persists one phase per standard cycle:

| Phase | Purpose | Exit condition |
|---|---|---|
| `assess` | Understand request and actual repository state; inspect relevant code, docs, configuration, tests, automation, and constraints. | Objective, current state, risks, unknowns, and next action are known. |
| `define` | Specify the next bounded change, design choice, acceptance criteria, and verification. | The next implementation step is actionable. |
| `implement` | Make coherent changes, tests, and relevant focused documentation updates. | A reviewable implementation checkpoint exists. |
| `verify` | Inspect diff and run appropriate local and project CI checks against the exact revision. | Technical acceptance is evidenced, or a justified backward transition is needed. |
| `reconcile` | Audit implementation, acceptance criteria, documentation, plan, and handoff for consistency. | All cycle requirements and documentation gates are satisfied. |
| `complete` | Terminal phase. | All completion gates pass and registry status/timestamp are updated consistently. |

Permitted transitions:

- assess → define, or directly to implement if an approved plan already makes the next task actionable.
- define → implement.
- implement → verify.
- verify → implement for a defect or failed check; verify → define only for an evidenced architectural contradiction or missing requirement; otherwise verify → reconcile.
- reconcile → implement for a code/test defect; reconcile → define for a genuine design gap; otherwise reconcile → complete.
- complete is terminal.

Do not move backwards because of uncertainty, habit, or a preference to re-read. Record the concrete evidence that justifies a backward transition.

When a standard-cycle transition occurs, update the registry's current_phase and the plan/handoff checkpoint in the same work unit. The registry wins if prose disagrees; reconcile the prose before continuing. Once a phase exit condition is met, continue into the next phase in the same session whenever tools and time permit. Do not stop after assessment or planning when implementation is actionable.

## Plan and handoff discipline

Every standard plan must include objective, problem, scope/non-goals, relevant project context, dependencies/risks, ordered tasks, testable acceptance criteria, verification approach, current phase, next action, and phase-exit evidence.

Every standard handoff must be updated before stopping, including when work is interrupted or unsuccessfully preempted. It records actual repository state, phase and exit evidence, completed changes, decisions, verification and failures, unresolved risks/blockers, documentation status, next concrete action, and uncommitted/unpushed work. Keep one current handoff per cycle; do not make a transcript or create multiple handoff files. Exclusive ownership is recorded only in `execution-lock.json`.

## Verification feedback loop

Use the repository's own documented checks, tooling, and collaboration policy. The lifecycle does not prescribe a language, CI vendor, platform matrix, or command. The repository's steering policy requires all work to remain on the `main` branch; do not create or switch branches. If direct work on `main` is blocked by repository protections or permissions, stop and report the blocker rather than using another branch or bypassing controls.

For each meaningful standard-cycle implementation checkpoint, inspect the diff, commit using the established workflow, identify CI runs for that exact revision, wait for completion where possible, and inspect relevant jobs, steps, logs, test results, and artifacts. Diagnose failures, correct the implementation, and repeat against the new exact revision. A commit is a checkpoint, not proof of correctness.

For micro cycles, run the single focused verification check that proves the stated acceptance condition and inspect the exact diff. If that is insufficient to establish correctness, promote the work to a standard cycle.

If verification fails after a commit to main, do not leave the unverified revision as the assumed good state: diagnose, fix forward with a new reviewed commit, or use a documented/repository-approved revert/rollback procedure. Never force-push or rewrite history; clearly disclose the exposure window and rollback result. If local tooling is unavailable, use remote CI as the execution environment when available. If CI is absent, broken, not triggered, or inaccessible, investigate repair or another project-approved verification route. Report checks truthfully; never infer success from another revision or incomplete run.

## Documentation audit policy

Every standard cycle considers documentation in its plan, updates relevant focused docs during implementation, and performs a final consistency audit before completion. Every micro cycle confirms that its cosmetic-only change does not alter meaning and records that review in its outcome. Promote only durable, approved decisions into long-lived project documentation; do not turn cycle notes into permanent doctrine.

At least once every three completed implementation cycles (standard and micro count; governance-only changes do not count), or sooner after a major architecture/contract change, perform a dedicated historical documentation audit. The audit is a lifecycle gate within the selected cycle, not a separate cycle. Review completed standard plans and handoffs and micro-cycle registry outcomes since the previous audit boundary alongside current implementation and maintained docs. Consolidate durable decisions, resolve contradictions, remove stale guidance, and check links and terminology. Record the reviewed execution-order range and outcome; only then advance the registry watermark.

## Completion

A standard cycle can be marked complete only after all acceptance criteria are verified or an explicitly accepted exception is recorded, the final diff is reviewed, required checks have actual results, documentation gates pass, and plan/handoff reflect the real outcome. Then set current_phase and status to complete and record completed_at.

A micro cycle can be marked complete only after its narrow acceptance condition is met, the exact diff is reviewed, focused verification is recorded, and outcome is truthfully documented in the registry.

Before any worker stops or hands off, update the current cycle checkpoint and release the execution lock. Blocked, paused, cancelled, and complete cycles must not hold an active lock. A session ending, a commit, a completed plan, or a green partial CI run does not complete a cycle. After the active cycle is fully complete, or is safely paused through the explicit preemption procedure, and the lock is released, the next cycle selected by the priority rules may start.

## Template setup and product documentation

The first-use bootstrap (replacing the root README from `README.template.md`, filling `docs/project/`, choosing product licensing, and updating `TEMPLATE_VERSION`) is setup work, not a development cycle. Complete it before the first ordinary implementation cycle. Product requirements, architecture, coding standards, and verified commands belong in `docs/project/requirements.md`, `architecture.md`, `coding-standards.md`, and `commands.md`. Coding standards must align with and defer structural and naming decisions to `architecture.md`; cycle plans and handoffs are temporary checkpoints only.


## Branch-policy adaptation

This template defaults to direct work on `main`. A derived repository that requires pull requests or another branch policy must make an explicit governance change to `steering.md` and reconcile the lock-claim protocol, serialized ownership, review, and verification rules. Do not merely change the branch instruction or bypass branch protection; the claim mechanism must remain safe across collaborators and branches.


## Validator

Run `python3 scripts/validate-steering.py` locally. The included GitHub Actions workflow runs the same standard-library validator when lifecycle files or the validator change, on pushes, pull requests, or manual dispatch. It checks registry/lock structure, statuses, uniqueness, timestamp relationships, cycle/lock consistency, and micro-cycle rules. JSON Schemas remain the structural contract; the validator covers cross-record invariants that JSON Schema cannot express. A green validator is necessary but does not replace cycle-file existence checks, implementation review, tests, or lifecycle gates.

## Active-work checkpoints and liveness

An active execution lock is an ownership record, not evidence that useful progress is occurring. The lock does not expire automatically. Elapsed time, a quiet conversation, a missing heartbeat, or apparent inactivity does not authorize another worker to take over.

While actively working under the lock:

- Update the current standard-cycle handoff, or the micro-cycle registry record, after every material decision, completed investigation, implementation checkpoint, blocker/next-action change, or verification result.
- During uninterrupted work, persist a progress checkpoint at least every 15 minutes. Each checkpoint states the exact phase, what changed or was established, remaining work, verification performed or not performed, and one next action.
- Do not create empty or cosmetic commits merely to refresh a timestamp. A checkpoint must record meaningful state.

Before an extended pause, while waiting for a user/external response, on context switch, or before ending a session:

1. Persist a truthful checkpoint with the exact current phase, completed work, outstanding work, verification state, and next action.
2. Set a standard cycle to `paused` when work is voluntarily deferred, or `blocked` when a concrete external dependency prevents progress. A blocked cycle records the blocker and unblocking action in `status_reason` and the handoff.
3. Release the execution lock through the normal persisted procedure and verify the released lock on the shared repository.
4. Do not hold an active lock while waiting for a user response or while no development work is being performed.

If meaningful progress cannot be made, state that plainly and record the investigation, blocker, or decision needed. Do not imply progress from elapsed time or conversational activity.

The handoff's last-progress checkpoint timestamp must reflect the actual latest persisted checkpoint. Keep the handoff, registry phase/status, and execution-lock state consistent. A stale handoff heading does not override the registry or lock; reconcile it immediately when discovered.

### Recovery of an abandoned active lock

If the lock remains active after the prior worker has terminated, do not infer termination from age or silence. Obtain explicit confirmation from the user or responsible repository owner that the previous worker is no longer working. Then preserve the cycle ID, sequence, execution order, phase, and history; record the confirmation and recovery rationale in the handoff; set the cycle to `paused` or `blocked` as appropriate; release and publish the old lock; fetch and verify both lock and registry; only then may a new worker claim the cycle. If the prior handoff is unavailable, record that limitation and the evidence used; never fabricate a checkpoint.


## Mandatory approval-to-artifact gate (template v1.1.0)

The current standard-cycle handoff is the authoritative decision ledger. Record every material user-approved requirement, architecture, interface, or implementation decision there; do not create a parallel tracker.

Decision states:
- **Proposed:** user approval is outstanding.
- **Approved:** user explicitly approved it. This is transient only and must not remain in a persisted checkpoint.
- **Recorded:** the canonical artifact has been updated, reviewed for consistency, published to the shared repository, fetched back, and verified at the stated revision. The handoff row names the canonical path and verified commit.

After approval, perform these steps in the same work unit:
1. Identify the canonical artifact using the document-ownership rules in `steering.md`.
2. Update it with the approved decision and rationale. Distinguish intended design from implemented behavior.
3. Review the exact diff for contradictions, compatibility, public API scope and accidental changes.
4. Publish the update, fetch it back from the shared branch, and verify the exact content/revision.
5. Update and publish the handoff decision register as `Recorded`, naming the canonical artifact and verified revision; fetch back and verify the handoff too.

Do not ask the next independent design question, begin dependent implementation, or advance phase until the decision is Recorded. If any step fails, stop. Persist the decision as Approved in the working checkpoint, identify the exact failure and next action, and do not claim it is recorded. Chat acknowledgement, local edits, an unpushed commit, or a write response without fetch-back verification are not recording evidence.
