# Persisted development lifecycle

This directory contains the durable execution state for ordinary development work. Use of this mechanism is mandatory for ordinary project work; the narrowly scoped governance-maintenance exemption is defined below.

The mechanism is deliberately project-agnostic: project-specific objectives, architecture, commands, quality gates, repository access and protection details, and deployment constraints belong in the repository's own documentation and configuration. This repository uses main as its working branch.

## Governance maintenance

The governance-maintenance exemption is defined in steering.md. Apply governance changes directly to their owning governance files; do not create or advance a development cycle for that work. This lifecycle specification does not define a second exemption policy.

## Authoritative files

- `cycle-index.json` is the single authoritative registry for cycle identity, order, workflow, status, and current phase.
- `cycle-index.schema.json` defines the registry's structural JSON contract.
- `templates/PLAN.md` is copied into each standard cycle's plan.
- `templates/HANDOFF.md` is copied into each standard cycle's handoff.
- `docs/project/decisions.md` is the durable project decision register; cycle records link to it and do not duplicate it.

Standard-cycle structure:

```text
docs/dev-life-cycle/
  README.md
  cycle-index.json
  cycle-index.schema.json
  templates/
    PLAN.md
    HANDOFF.md
  <cycle-uuid>/
    plan/
      PLAN.md
    handoff/
      HANDOFF.md
```

Micro-cycles have no per-cycle plan or handoff directory. Their required compact record is stored directly in `cycle-index.json`. Do not store project-wide architecture, requirements, or the durable decision register in cycle plans or handoffs.

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
4. Any number of cycles may be `in-progress`, `paused`, or `blocked`; each cycle's progress and next action are tracked independently in its plan/handoff or compact micro record.
5. A pending standard cycle has phase `assess`, null execution_order, null started_at, null completed_at, and null status_reason.
6. An in-progress standard cycle has a non-null execution_order and started_at, a phase other than `complete`, and null completed_at.
7. A paused standard cycle has a non-null execution_order and started_at, a resumable phase other than `complete`, and null completed_at/status_reason. Pausing never clears or changes its execution_order, ID, sequence, or timestamps; its checkpoint and pause context belong in the handoff.
8. A blocked standard cycle has non-null execution_order and started_at, a resumable phase, null completed_at, and a non-empty status_reason. Unblock by updating the reason/checkpoint and explicitly resuming it.
9. A cancelled standard cycle is terminal, has a non-empty status_reason, null completed_at, and retains either both execution_order/started_at (if started) or neither (if cancelled while pending). Cancelled history is never deleted or resumed.
10. A complete standard cycle has phase `complete`, non-null execution_order, started_at, and completed_at.
11. A micro cycle is never pending, paused, blocked, or cancelled. An in-progress micro cycle has phase `implement`, non-null execution_order and started_at, and null completed_at, verification_evidence, and outcome/status_reason. A complete micro cycle has phase `complete`, non-null execution_order, started_at, completed_at, verification_evidence, and outcome, with status_reason null. If a micro cycle is blocked, cancelled, or preempted, promote that same cycle to standard and create its plan/handoff first; retain the micro evidence as historical context.
12. Every micro cycle has a non-empty eligibility_reason and acceptance_criteria. Standard cycles set the four micro-only evidence fields to null.
13. Timestamp ordering is consistent where values are known: created_at is not after started_at; started_at is not after completed_at.
14. `schema_version` is supported. Unknown versions or invalid records must be surfaced, not silently rewritten.
15. Completed and cancelled records remain in the registry. Never delete history to make the registry look tidy.

The array order among pending standard cycles determines their default priority. Paused cycles retain their original execution_order and resume ahead of ordinary pending work, ordered by their original execution_order, unless the user explicitly directs a different priority. Blocked cycles are skipped until explicitly unblocked. Cancelled cycles are terminal. Reordering pending cycles requires an explicit reason and must not alter sequence or execution_order. Micro cycles start immediately and receive the next unused execution_order.

## Template version adoption

This template starts new product repositories at schema v3. Do not carry a historical migration section into derived repositories. If adopting a newer template, compare the upstream `TEMPLATE_VERSION`, review its release/change notes, and apply any schema migration deliberately while preserving cycle IDs, sequence/execution history, and timestamps. The template README describes the adoption procedure.

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
2. Add a registry record with workflow `standard`, status `pending`, execution_order null, current_phase `assess`, and the four micro-only evidence fields set to null.
4. Create the UUID directory with `plan/PLAN.md` and `handoff/HANDOFF.md` from the templates.
5. Describe the objective, scope, non-goals, acceptance criteria, and next action in the plan.
6. Keep the handoff as the single current checkpoint for that cycle.
7. When the pending cycle first starts, assign the next unused execution_order and started_at, change status to `in-progress`, and preserve both values permanently.

## Creating a micro-cycle

A micro-cycle is still a complete, persisted development cycle. It omits per-cycle plan and handoff files only because its narrow scope is fully captured by the registry fields.

1. Confirm every micro eligibility condition. If uncertain, choose a standard cycle instead.
2. Allocate a candidate UUID, sequence, and execution_order in memory; do not write repository files yet.
3. Append a registry record with workflow `micro`, status `in-progress`, current_phase `implement`, created_at and started_at set, completed_at null, a concise eligibility_reason, and one acceptance_criteria statement. Set verification_evidence and outcome to null.
5. Only after that in-progress record is persisted may the bounded change begin.
6. Perform the smallest focused verification that proves the acceptance condition. Inspect the exact diff and confirm no unrelated or semantic changes slipped in.
7. Update the same registry record to status `complete`, current_phase `complete`, set completed_at, and record concise verification_evidence and outcome. Do not claim checks that were not actually run.
8. The registry record is the durable micro-cycle record; do not create empty plan/handoff files or a narrative substitute.

The micro path compresses administrative phase transitions into one persisted in-progress record and one completion update. It does not skip assessment, scope control, implementation review, verification, or reconciliation; those checks are performed as one disciplined short pass and evidenced in the registry.

## Resuming, prioritizing, and preempting work

At the start of every ordinary development task:

1. Read `steering.md` and this README, then parse and validate `cycle-index.json` against its JSON Schema and cross-record invariants.
2. Inspect relevant cycle plans/handoffs and repository state. Read-only discovery does not authorize implementation or registry/project-file changes outside the selected cycle.
3. Check whether the user explicitly identified the request as urgent or directed that it take priority. Urgent work may preempt the currently active cycle.
4. If urgent preemption is requested and a different cycle is in progress, checkpoint that cycle's exact phase, completed work, outstanding work, verification state, and next action in its handoff (standard) or registry record (micro). Mark it `paused`. If it is a micro cycle, promote that same cycle to standard and create its plan/handoff first.
5. Select the urgent request's cycle next. Otherwise resume the relevant in-progress cycle when continuing existing work; if beginning new work, resume the paused standard cycle with the lowest execution_order before selecting ordinary pending work. If none is eligible, assess the request and choose standard or micro workflow.
6. For standard cycles, read the plan and handoff. For micro cycles, use the registry objective, eligibility_reason, acceptance_criteria, and persisted phase as the checkpoint.
7. Verify persisted state against the actual repository, branch, working tree, implementation, tests, and CI; records are checkpoints, not unquestionable truth.
8. Continue from the persisted phase. Never restart at assessment simply because the session changed.

Urgent work does not automatically cancel or discard existing work. Preemption checkpoints and pauses the active cycle; after urgent work completes, resume the paused cycle before ordinary pending work. Cancelling is a separate explicit user/owner decision; record the reason and retain the terminal record. Blocking is for work that cannot proceed but is expected to resume; record the blocker and unblocking action in status_reason plus the handoff. Pending-cycle order remains the default when no explicit urgency/preemption instruction applies.

If the registry or required standard-cycle records are invalid, contradictory, or missing, stop lifecycle mutation, report the exact inconsistency, and repair it deliberately. Do not infer intended state from chat or folder names.

## Pause, block, unblock, and cancel

- **Pause** means work is intentionally deferred. Checkpoint in the standard handoff, set status `paused`, retain the phase and historical order/timestamps.
- **Block** means work cannot proceed until an external dependency, decision, access, or other impediment is resolved. Set status `blocked`, retain phase/order/start timestamp, set `status_reason` to the concise blocker and unblocking action, update the handoff. A blocked cycle is not selected automatically.
- **Unblock** only after the stated blocker is resolved or explicitly changed. Update `status_reason` to null, document evidence/checkpoint, and resume; status becomes `in-progress`.
- **Cancel** is an explicit terminal decision, not a synonym for pause or block. Set status `cancelled`, provide a `status_reason`, leave `completed_at` null, and preserve the record. A pending cycle remains assess with no execution order or start timestamp; a started cycle keeps both. Never resume a cancelled cycle; if the work is needed again, create a new cycle.
- **Micro-cycle interruption:** before pausing, blocking, cancelling, or urgent preemption, promote the same micro cycle to standard, create its plan and handoff, retain the compact micro record as context, and then update its status. Micro cycles may not have interrupted states.

## Mandatory agent operating protocol

The lifecycle mechanism is mandatory for the whole ordinary task, including design discussion and investigation—not only for code edits. A task must not be advanced through chat while its cycle and handoff remain unmaintained.

**Before starting or resuming ordinary work:**
- Validate the registry and select/resume the correct standard or eligible micro cycle.
- For standard work, read and update the cycle's plan and handoff before substantive work. The handoff is the current progress record; the project decision ledger remains separate.
- If no applicable cycle exists, create and persist one using the prescribed workflow before proceeding. Do not conduct an extended design conversation first and attempt to reconstruct its state later.

**After each material user approval:**
- Treat the approval as a hard stop on design progression.
- Integrate the approved outcome into its owning project specification and record its provenance in docs/project/decisions.md. Do not copy the decision text into both requirements and architecture.
- Publish and fetch the specification and decision ledger. Verify their exact contents and revisions, and verify that the recorded revision is present in the shared `main` branch.
- Update the current handoff only with the resulting progress, verification, blockers, and next action. The handoff does not maintain a second decision register.
- Only after the specification and decision ledger are verified may the next independent design question, dependent implementation, or phase transition begin.
- If any write or verification fails, do not continue the design. Leave the decision Proposed in the ledger, record the exact blocker and recovery action in the handoff, and report it.

**At every session or context resumption:**
- Re-read the registry and current handoff. Reconcile the handoff against the actual repository revision.
- Treat chat history as explanatory context only. It cannot establish cycle status, decision approval, artifact recording, or verification.
- Resume from the persisted phase and first resolve any approved-but-unrecorded project decision using docs/project/decisions.md before doing other work.

The governance-maintenance exemption in steering.md permits direct maintenance of the governance mechanism without creating or advancing a development cycle. Governance changes remain subject to review, publication, fetch-back, and verification.


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

Every standard handoff must be updated before stopping, including when work is interrupted or unsuccessfully preempted. It records actual repository state, phase and exit evidence, completed changes, links to relevant decision IDs, verification and failures, unresolved risks/blockers, documentation status, next concrete action, and uncommitted/unpushed work. Keep one current handoff per cycle; do not make a transcript or create multiple handoff files. Cycle progress is recorded in the registry and current handoff; contributors coordinate through ordinary Git collaboration.

### Decision approval-to-artifact gate

The durable project decision ledger is docs/project/decisions.md. It is the single register for material
requirements, architecture, interface, and implementation decisions. Cycle plans and handoffs link to
decision IDs and record progress; they do not maintain a parallel decision register.

Use these states in the project decision ledger:

- **Proposed:** The decision and rationale have been presented; user approval is outstanding.
- **Recorded:** The user approved the outcome, the owning specification was updated, and both the
  specification and decision ledger were published, fetched back, and verified at their stated revisions.

Approval is transient working state, not a persisted ledger state. Do not write Approved as a durable
status. If approval has been given but recording is incomplete, leave the ledger entry Proposed and
record the exact blocker and next action in the current handoff.

On approval, immediately perform the following in the same work unit:
1. Identify the owning specification using the document-ownership rules in steering.md.
2. Integrate the approved outcome into the relevant topic in requirements.md or architecture.md.
3. Review the exact diff for consistency with the other specification, existing public contracts, and scope.
4. Publish the specification and update docs/project/decisions.md with the concise decision outcome,
   specification path, and verified revision.
5. Fetch both files from the shared target branch and verify their exact contents and revisions.
6. Update the handoff with progress, verification, and the next action only.

Only after these steps may the agent ask the next independent design question, start dependent
implementation, or advance the phase. If any step fails, stop and report the approved-but-unrecorded
decision. A user approval is not implementation authorization to bypass this gate.

## Verification feedback loop

Use the repository's own documented checks, tooling, and collaboration policy. The lifecycle does not prescribe a language, CI vendor, platform matrix, or command. The repository's steering policy requires all work to remain on the `main` branch; do not create or switch branches. If direct work on `main` is blocked by repository protections or permissions, stop and report the blocker rather than using another branch or bypassing controls.

For each meaningful standard-cycle implementation checkpoint, inspect the diff, commit using the established workflow, identify CI runs for that exact revision, wait for completion where possible, and inspect relevant jobs, steps, logs, test results, and artifacts. Diagnose failures, correct the implementation, and repeat against the new exact revision. A commit is a checkpoint, not proof of correctness.

For micro cycles, run the single focused verification check that proves the stated acceptance condition and inspect the exact diff. If that is insufficient to establish correctness, promote the work to a standard cycle.

If verification fails after a commit to main, do not leave the unverified revision as the assumed good state: diagnose, fix forward with a new reviewed commit, or use a documented/repository-approved revert/rollback procedure. Never force-push or rewrite history; clearly disclose the exposure window and rollback result. If local tooling is unavailable, use remote CI as the execution environment when available. If CI is absent, broken, not triggered, or inaccessible, investigate repair or another project-approved verification route. Report checks truthfully; never infer success from another revision or incomplete run.

## Documentation audit policy

Every standard cycle considers documentation in its plan, updates relevant focused docs during implementation, and performs a final consistency audit before completion. Every micro cycle confirms that its cosmetic-only change does not alter meaning and records that review in its outcome. Promote only durable, approved outcomes into the relevant project specification and decision ledger. Curate the result; do not copy checkpoint or handoff prose into project documentation.

At least once every three completed implementation cycles (standard and micro count; governance-only changes do not count), or sooner after a major architecture/contract change, perform a dedicated historical documentation audit. The audit is a lifecycle gate within the selected cycle, not a separate cycle. Review completed standard plans and handoffs and micro-cycle registry outcomes since the previous audit boundary alongside current implementation and maintained docs. Consolidate durable decisions, resolve contradictions, remove stale guidance, and check links and terminology. Record the reviewed execution-order range and outcome; only then advance the registry watermark.

## Completion

A standard cycle can be marked complete only after all acceptance criteria are verified or an explicitly accepted exception is recorded, the final diff is reviewed, required checks have actual results, documentation gates pass, and plan/handoff reflect the real outcome. Then set current_phase and status to complete and record completed_at.

A micro cycle can be marked complete only after its narrow acceptance condition is met, the exact diff is reviewed, focused verification is recorded, and outcome is truthfully documented in the registry.

Before stopping or handing off, update the current cycle checkpoint. A session ending, a commit, a completed plan, or a green partial CI run does not complete a cycle. After a cycle is fully complete or safely paused through the explicit preemption procedure, another eligible cycle may proceed.

## Template setup and product documentation

The first-use bootstrap (replacing the root README from `README.template.md`, filling `docs/project/`, choosing product licensing, and updating `TEMPLATE_VERSION`) is setup work, not a development cycle. Complete it before the first ordinary implementation cycle. Product requirements, architecture, coding standards, verified commands, and durable project decisions belong in docs/project/requirements.md, architecture.md, coding-standards.md, commands.md, and decisions.md. Coding standards must align with and defer structural and naming decisions to architecture.md; cycle plans and handoffs are temporary progress records.



## Validator

Run `python3 scripts/validate-steering.py` locally. The included GitHub Actions workflow runs the same standard-library validator when lifecycle files, the project decision ledger, or the validator change, on pushes to `main` or manual dispatch. It checks registry structure, statuses, uniqueness, timestamp relationships, micro-cycle rules, and the single project decision register. Decisions may be Proposed or Recorded; an Approved-but-unrecorded decision fails validation. Recorded decisions must identify an existing specification and a verified commit/revision. JSON Schemas remain the structural contract; the validator covers cross-record and decision-ledger invariants that JSON Schema cannot express. A green validator is necessary but does not replace cycle-file existence checks, implementation review, tests, or lifecycle gates.
