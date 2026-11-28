# Persisted development lifecycle

## Session-mode gate — precedes lifecycle entry

The AI must classify the session's objective and scope before reading the registry, selecting/resuming a cycle, creating a cycle, or doing substantive work. This is Gate 0, established by the repository root README and `steering.md`.


- **Governance Mode:** governance/AI operating rules maintenance within the defined exemption. Do not use the operational cycle mechanism. The exemption applies only while scope stays within its boundaries.
- **Operational Mode:** ordinary product requirements, architecture, implementation, tests, defects, behavior, or substantive product documentation. The lifecycle is mandatory.
- **Mode depends on intent and scope, not the filename being edited.** A governance-routing repair may touch a product README without becoming product work; substantive product changes are Operational.
- **Mixed or uncertain scope:** clarify or split before proceeding. Stop at a scope crossing; do not expand the exemption silently.
- References are conditional; do not recursively load all linked documents.

This gate must be completed before the lifecycle protocol below is invoked.


This directory contains the durable execution state for ordinary development work. Use of this mechanism is mandatory for ordinary project work; the narrowly scoped governance-maintenance exemption is defined below.

The mechanism is deliberately project-agnostic: project-specific objectives, architecture, commands, quality gates, branch-protection details, and deployment constraints belong in the repository's own documentation and configuration. This template uses `main` as its working branch. It assumes a single maintainer and one agent working at a time, so at most one cycle is `in-progress`.

## Governance-maintenance exemption

Changes limited to session-mode governance routing in the root `README.md`, steering/lifecycle governance, generic engineering standards under `docs/standards/`, project-strategy templates under `docs/project/` (testing, performance, and Git workflow), the project decision-ledger template, the validator/workflow, or template release metadata/changelog—and intended to amend or repair those governance or adoption rules—are exempt from creating, selecting, resuming, or completing a development cycle. This explicit self-maintenance exemption prevents recursive cycles while allowing the standards and their lifecycle integration to be maintained together. Root `README.md` is included only when changing its AI session classification/governance routing instructions; ordinary product README content remains Operational work.

Keep such changes strictly limited to those governance/template files. If ordinary product requirements, architecture, implementation, tests, or operational files are also changed, that ordinary work must use the normal cycle process. The exemption from cycle tracking does not waive other repository controls.

## Authoritative files

- `cycle-index.json` is the single authoritative registry for cycle identity, order, workflow, status, and current phase.
- `cycle-index.schema.json` defines the registry's structural JSON contract.
- `templates/PLAN.md` is copied into each standard cycle's plan.
- `templates/HANDOFF.md` is copied into each standard cycle's handoff.

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

Micro-cycles have no per-cycle plan or handoff directory. Their required compact record is stored directly in `cycle-index.json`. Do not store project-wide architecture or permanent requirements in cycle plans or handoffs.

Historical or archived plans and handoffs are reference evidence only. Do not load them by default or treat their recorded next actions as current instructions. Read them only when the current task, state reconciliation, or scheduled documentation audit requires them; confirm any candidate action against the authoritative registry and current specifications before acting.

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

Take every timestamp from the system clock (for example `date -u +%Y-%m-%dT%H:%M:%SZ`); never estimate or invent one.

### Cross-record invariants

JSON Schema validates record shape. The lifecycle controller must additionally validate:

1. Every cycle ID is a unique UUID. Every standard cycle has its plan and handoff files; micro cycles have neither and do not need empty directories.
2. Every `sequence` is a unique positive integer. Sequence values are never changed or reused.
3. Every non-null `execution_order` is a unique positive integer. Pending standard cycles have null execution order; once assigned, it never changes.
4. At most one cycle has status `in-progress`, because one agent works at a time. Any number of cycles may be `pending`, `paused`, or `blocked`; each cycle's progress and next action are tracked independently in its plan/handoff or compact micro record. Starting or resuming a different cycle first requires moving the in-progress cycle to `paused` (or `blocked`, `cancelled`, or `complete`). A session that ends abruptly simply leaves its cycle `in-progress`, and the next session resumes it.
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
15. Completed and cancelled development-cycle records remain in the registry. Never delete cycle history to make the registry look tidy. This rule applies only to lifecycle records; GitHub Actions cancelled workflow runs are deleted under the Git Workflow Standard.

The array order among pending standard cycles determines their default priority. Paused cycles retain their original execution_order and resume ahead of ordinary pending work, ordered by their original execution_order, unless the user explicitly directs a different priority. Blocked cycles are skipped until explicitly unblocked. Cancelled development cycles are terminal and their registry records are retained; this does not apply to GitHub Actions workflow-run history, which follows the Git Workflow Standard. Reordering pending cycles requires an explicit reason and must not alter sequence or execution_order. Micro cycles start immediately, after pausing any in-progress standard cycle, and receive the next unused execution_order.

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
2. Add the registry record and create its plan/handoff before beginning work.
3. Add a registry record with workflow `standard`, status `pending`, execution_order null, current_phase `assess`, and the four micro-only evidence fields set to null.
4. Create the UUID directory with `plan/PLAN.md` and `handoff/HANDOFF.md` from the templates.
5. Describe the objective, scope, non-goals, acceptance criteria, and next action in the plan.
6. Keep the handoff as the single current progress checkpoint for that cycle.
7. When the pending cycle first starts and no other cycle is in-progress, assign the next unused execution_order and started_at, change status to `in-progress`, and preserve both values permanently.

## Creating a micro-cycle

A micro-cycle is still a complete, persisted development cycle. It omits per-cycle plan and handoff files only because its narrow scope is fully captured by the registry fields.

1. Confirm every micro eligibility condition. If uncertain, choose a standard cycle instead.
2. Allocate a candidate UUID, sequence, and execution_order in memory; do not write repository files yet. If a standard cycle is in-progress, first checkpoint it in its handoff and mark it `paused`; it resumes when the micro cycle completes.
3. Append the registry record before beginning work.
4. Append a registry record with workflow `micro`, status `in-progress`, current_phase `implement`, created_at and started_at set, completed_at null, a concise eligibility_reason, and one acceptance_criteria statement. Set verification_evidence and outcome to null.
5. Only after that in-progress record is persisted may the bounded change begin.
6. Perform the smallest focused verification that proves the acceptance condition. Inspect the exact diff and confirm no unrelated or semantic changes slipped in.
7. Update the same registry record to status `complete`, current_phase `complete`, set completed_at, and record concise verification_evidence and outcome. Do not claim checks that were not actually run.
8. The registry record is the durable micro-cycle record; do not create empty plan/handoff files or a narrative substitute.

The micro path compresses administrative phase transitions into one persisted in-progress record and one completion update. It does not skip assessment, scope control, implementation review, verification, or reconciliation; those checks are performed as one disciplined short pass and evidenced in the registry.

## Resuming, prioritizing, and preempting work

At the start of every ordinary development task:

1. Read `steering.md` and this README, then parse and validate `cycle-index.json` against its JSON Schema and cross-record invariants.
2. Inspect relevant cycle plans/handoffs and repository state. Read-only discovery does not authorize implementation or registry/project-file changes outside the selected cycle.
3. Determine the work from the user's current request, not from the queue. If the request continues the in-progress cycle, or matches a paused or pending cycle, select that cycle. If it is new work, assess it, choose the standard or micro workflow, and create a new cycle. If it is unclear whether the request continues an existing cycle, ask the user one short question. The request does not need to be marked urgent: the user's current request takes priority over paused and pending cycles.
4. At most one cycle may be in-progress. If the selected cycle is not the in-progress cycle, checkpoint the in-progress cycle's exact phase, completed work, outstanding work, verification state, and next action in its handoff (standard) or registry record (micro), and mark it `paused`. If it is a micro cycle, promote that same cycle to standard and create its plan/handoff first. Block, cancel, or complete it instead only when the user so decides.
5. If the user did not name specific work (for example "continue" or "what is next"), resume the in-progress cycle; otherwise the paused standard cycle with the lowest execution_order; otherwise the first pending standard cycle. If none is eligible, ask what to work on.
6. For standard cycles, read the plan and handoff. For micro cycles, use the registry objective, eligibility_reason, acceptance_criteria, and persisted phase as the checkpoint.
7. Verify persisted state against the actual repository, branch, working tree, implementation, tests, and CI; records are checkpoints, not unquestionable truth.
8. Continue from the persisted phase. Never restart at assessment simply because the session changed.

A new request does not cancel or discard existing work. Switching checkpoints and pauses the active cycle; once the new work completes, paused cycles resume (lowest execution_order first) ahead of ordinary pending work unless the user directs otherwise. Cancelling is a separate explicit user decision; record the reason and retain the terminal record. Blocking is for work that cannot proceed but is expected to resume; record the blocker and unblocking action in status_reason plus the handoff. Pending-cycle order is the default only when the user has not named the work to do.

If the registry or required standard-cycle records are invalid, contradictory, or missing, stop lifecycle mutation, report the exact inconsistency, and repair it deliberately. Do not infer intended state from chat or folder names.

## Pause, block, unblock, and cancel

- **Pause** means work is intentionally deferred. Checkpoint in the standard handoff, set status `paused`, retain the phase and historical order/timestamps, 
- **Block** means work cannot proceed until an external dependency, decision, access, or other impediment is resolved. Set status `blocked`, retain phase/order/start timestamp, set `status_reason` to the concise blocker and unblocking action, update the handoff. A blocked cycle is not selected automatically.
- **Unblock** only after the stated blocker is resolved or explicitly changed. Update `status_reason` to null, document evidence/checkpoint, and resume; status becomes `in-progress`.
- **Cancel** is an explicit terminal decision, not a synonym for pause or block. Set status `cancelled`, provide a `status_reason`, leave `completed_at` null, and preserve the record. A pending cycle remains assess with no execution order or start timestamp; a started cycle keeps both. Never resume a cancelled cycle; if the work is needed again, create a new cycle.
- **Micro-cycle interruption:** before pausing, blocking, cancelling, or switching away, promote the same micro cycle to standard, create its plan and handoff, retain the compact micro record as context, and then update its status. Micro cycles may not have interrupted states.

## Mandatory agent operating protocol

The lifecycle mechanism is mandatory for the whole ordinary task, including design discussion and investigation—not only for code edits. A task must not be advanced through chat while its cycle and handoff remain unmaintained.

**Before starting or resuming ordinary work:**
- Validate the registry and select/resume the correct standard or eligible micro cycle.
- For standard work, read and update the cycle's plan and handoff before substantive work. The handoff is the progress checkpoint.
- If no applicable cycle exists, create and persist one using the prescribed workflow before proceeding. Do not conduct an extended design conversation first and attempt to reconstruct its state later.

**After each material user approval:**
- Treat the approval as a hard stop on design progression, and follow the [decision approval-to-artifact gate](#decision-approval-to-artifact-gate) below before any dependent work.

**At every session or context resumption:**
- Re-read the registry and current handoff. Reconcile the handoff against the actual repository revision.
- Treat chat history as explanatory context only. It cannot establish cycle status, decision approval, artifact recording, or verification.
- Resume from the persisted phase and first resolve any approved-but-unrecorded decision before doing other work.

## Reconcile authoritative project state before reporting remaining work

Before describing an item as open, proposing the next action, or recommending a phase transition, derive the current state from the repository's authoritative records—not from a stale plan, handoff, unresolved-items list, or chat history alone:

1. Read the complete relevant current requirements and architecture sections, including both the detailed topic and any unresolved/open-questions section. Do not inspect only headings, summaries, or an unresolved-items list.
2. Read the relevant entries in `docs/project/decisions.md`. Confirm each entry's state, owning specification, and verified revision. The ledger records approval and provenance; the owning specification defines the current requirement or technical design.
3. Read the active cycle plan and handoff as progress checkpoints, then compare their claimed remaining work with the current specifications and decision entries.
4. Classify every candidate item as **resolved**, **unresolved**, **explicitly deferred**, or **contradictory**:
   - **Resolved:** the approved outcome is recorded, integrated into its owning specification, and no current specification statement conflicts with it.
   - **Unresolved:** no approved outcome settles the question, and the current specification or decision record identifies it as open.
   - **Explicitly deferred:** a recorded decision explicitly defers the question or work and identifies the boundary or condition for revisiting it.
   - **Contradictory:** current specifications, decision provenance, or other authoritative statements disagree, or an unresolved-items list still labels a settled topic open.
5. A decision marked `Recorded` is not sufficient by itself: verify its outcome in the owning specification and check for conflicting current statements. Plans and handoffs are checkpoints, not authorities for project requirements or technical design. If they list already-resolved work, correct them.
6. Do not advance dependent work or recommend a phase transition while a material contradiction remains. Reconcile the canonical specification and decision ledger where the decision itself needs correction; update the plan and handoff to reflect the verified state. Report the evidence and decision IDs supporting each classification.

An unresolved-items list is an index of questions, not proof that each listed question is still unresolved. Revalidate it against the detailed specification and recorded decisions whenever using it. Do not silently choose between conflicting statements.

The governance-maintenance exemption permits repair of this lifecycle mechanism without creating a recursive cycle. It still requires review, publication, fetch-back, and verification.


## Standard-cycle phase machine

The registry persists one phase per standard cycle:

| Phase | Purpose | Exit condition |
|---|---|---|
| `assess` | Understand request and actual repository state; inspect relevant code, docs, configuration, tests, automation, and constraints. | Objective, current state, risks, unknowns, and next action are known. |
| `define` | Specify the next bounded change, design choice, acceptance criteria, and verification. Approve the project testing strategy, performance strategy, and Git workflow before implementation. | The implementation plan is actionable; architecture-aware test selection, full-verification gate, performance approach/applicability, and Git/CI workflow are explicitly approved and recorded. |
| `implement` | Execute small code/test/documentation iterations using the approved Git workflow and architecture-aware fast feedback; before exiting, run the complete required verification suite and applicable performance checks against the final implementation revision. | All required tests, integration/regression/platform checks, quality gates, applicable performance checks, and coding-standard review have observed results for the exact final SHA, or an explicit approved exception is recorded. Only then may the cycle transition to Verify. |
| `verify` | Independently assess the complete verification evidence produced at the Implement exit gate, confirm exact-SHA coverage and acceptance criteria, and perform any additional acceptance/manual validation required by the approved strategy. | Evidence is complete and trustworthy for the final revision; acceptance criteria are satisfied and no required check remains unresolved. |
| `reconcile` | Audit implementation, acceptance criteria, documentation, plan, and handoff for consistency. | All cycle requirements and documentation gates are satisfied. |
| `complete` | Terminal phase. | All completion gates pass and registry status/timestamp are updated consistently. |

Permitted transitions:

- assess → define, or directly to implement if an approved plan already makes the next task actionable.
- define → implement.
- implement → verify only after the complete implementation-exit gate has passed for the final implementation SHA.
- verify → implement for a defect or failed check; verify → define only for an evidenced architectural contradiction or missing requirement; otherwise verify → reconcile.
- reconcile → implement for a code/test defect; reconcile → define for a genuine design gap; otherwise reconcile → complete.
- complete is terminal.

Do not move backwards because of uncertainty, habit, or a preference to re-read. Record the concrete evidence that justifies a backward transition.

When a standard-cycle transition occurs, update the registry's current_phase and the plan/handoff checkpoint in the same work unit. The registry wins if prose disagrees; reconcile the prose before continuing. Once a phase exit condition is met, continue into the next phase in the same session whenever tools and time permit. Do not stop after assessment or planning when implementation is actionable.

## Plan and handoff discipline

Every standard plan must include objective, problem, scope/non-goals, relevant project context, dependencies/risks, ordered tasks, testable acceptance criteria, verification approach, current phase, next action, and phase-exit evidence. It must identify the applicable coding standard revision, the approved testing, performance, and Git workflow strategy revisions/decision IDs, and the relevant test tier(s) for the planned change. Implementation review must check conformance with the coding standard and explicitly record justified exceptions.

Every standard handoff must be updated before stopping, including when work is interrupted or unsuccessfully preempted. It records actual repository state, phase and exit evidence, completed changes, links to relevant decision IDs, verification and failures, unresolved risks/blockers, documentation status, next concrete action, and uncommitted/unpushed work. Keep one current handoff per cycle; do not make a transcript or create multiple handoff files. Cycle progress is recorded in the registry and current handoff; contributors coordinate through ordinary Git collaboration.

### Decision approval-to-artifact gate

The durable project decision ledger is `docs/project/decisions.md`. It is the single register for material requirements, architecture, interface, and implementation decisions. Cycle plans and handoffs link to decision IDs and record progress; they do not maintain a parallel decision register.

Use these states in the project decision ledger:

- **Proposed:** The decision and rationale have been presented; user approval is outstanding.
- **Recorded:** The user approved the outcome, the owning specification was updated, and both the specification and decision ledger were published, fetched back, and verified at their stated revisions.

Approval is transient, not a persisted ledger state. If approval has been given but recording is incomplete, leave the ledger entry Proposed and record the blocker and next action in the current handoff.

On approval, immediately perform the following in the same work unit:
1. Identify the owning specification using the document-ownership rules in steering.md: requirements go in `requirements.md`; technical realization and design rationale in `architecture.md`; implementation conventions in `docs/standards/coding.md`; verified procedures in operational documentation.
2. Integrate the approved outcome into the relevant topic of that owning specification.
3. Review the exact diff for consistency with other specifications, public contracts, and scope.
4. Publish the specification and update docs/project/decisions.md with the concise outcome, specification path, and verified revision.
5. Fetch both files from the shared target branch and verify exact contents and revisions.
6. Update the handoff with progress, verification, and the next action only.

Only after these steps may dependent work continue. If any step fails, stop and report the decision that remains Proposed.

A decision is `Recorded` only when its canonical artifact is present on the shared repository revision and has been fetched and checked. A chat acknowledgement, local edit, unpushed commit, or successful write response alone is not evidence of recording.

## Verification feedback loop

Load the project-approved `docs/project/git-workflow.md`, `testing-strategy.md`, and `performance-strategy.md` as applicable to the task. The generic engineering standards are catalogued in [docs/standards/README.md](../standards/README.md); consult the standards that apply to the changed area, using the task-routing table below rather than routinely loading all standards. Applicable standards remain mandatory—the routing table limits context loading, not compliance. Load additional documents when cross-cutting impact, risk, unfamiliar interfaces, security implications, or verification needs require them.

### Task-based document routing

| Task type | Required context |
|---|---|
| Requirements or user-visible behavior | Requirements, relevant decisions, affected acceptance criteria |
| Architecture or interface changes | Architecture, relevant requirements, applicable decisions |
| Implementation or bug fixes | Relevant requirements and architecture, coding standard, applicable project strategy |
| Tests or test infrastructure | Testing standard, testing strategy, relevant requirements, affected implementation |
| Performance work | Performance engineering standard, performance strategy, relevant architecture and workloads |
| Git, CI, or workflow changes | Git workflow standard, project Git policy, affected workflow/configuration |
| Documentation-only changes | Documents being edited, owning specification, and directly affected references |
| Cross-cutting changes | All relevant standards and strategies for the affected areas |

Combine rows when work spans multiple categories. The standards index remains the canonical catalogue; applicable standards and project strategies must be followed in full.

GitHub MCP is the required agent interface for repository inspection, file changes, publication, fetch-back, and CI evidence retrieval whenever available; GitHub Actions is an execution environment, not the repository interaction protocol. Follow the repository's approved branch and integration model and never bypass protections.

For each meaningful implementation iteration, select focused tests from the architecture-to-test map: changed component tests, directly affected dependants, and relevant interface/contract tests. Expand to affected integration and end-to-end tests when boundaries or cross-component behavior change. Shared foundations, public interfaces, and cross-cutting changes require tests for all affected downstream consumers. Select by architectural impact, not file paths alone. If impact is uncertain, broaden the selection.

Inspect the diff, commit and publish using the approved Git workflow, identify required CI runs for that exact SHA, and inspect their statuses, jobs, steps, logs, reports, and artifacts through GitHub MCP. Wait for completion where possible. A failed required fast check blocks further progression until diagnosed and resolved or explicitly excepted. A skipped, cancelled, missing, or inaccessible required result is not a pass. Repeat against the new exact revision after corrections. A commit is a checkpoint, not proof of correctness.

**Implementation exit gate:** before leaving Implement, run the complete required verification suite defined by the approved testing strategy, including all required test levels, integration/regression/platform checks and applicable quality gates. Run applicable performance checks according to the performance strategy. All required checks must have observed results for the final implementation SHA. Fast feedback does not substitute for this full gate. Record skipped or unavailable checks and do not treat them as successful; unresolved required failures prevent implementation exit unless an explicit, approved exception records the risk and compensating evidence.

For micro cycles, run the single focused verification check that proves the stated acceptance condition and inspect the exact diff. If that is insufficient to establish correctness, promote the work to a standard cycle.

If verification fails after a commit to main, do not leave the unverified revision as the assumed good state: diagnose, fix forward with a new reviewed commit, or use a documented/repository-approved revert/rollback procedure. Never force-push or rewrite history; clearly disclose the exposure window and rollback result. If local tooling is unavailable, use remote CI as the execution environment when available. If CI is absent, broken, not triggered, or inaccessible, investigate repair or another project-approved verification route. Report checks truthfully; never infer success from another revision or incomplete run.

## Documentation audit policy

Every standard cycle considers documentation in its plan, updates relevant focused docs during implementation, and performs a final consistency audit before completion. Every micro cycle confirms that its cosmetic-only change does not alter meaning and records that review in its outcome. Promote only durable, approved decisions into long-lived project documentation; do not turn cycle notes into permanent doctrine.

At least once every three completed implementation cycles (standard and micro count; governance-only changes do not count), or sooner after a major architecture/contract change, perform a dedicated historical documentation audit. The audit is a lifecycle gate within the selected cycle, not a separate cycle. Review completed standard plans and handoffs and micro-cycle registry outcomes since the previous audit boundary alongside current implementation and maintained docs. Consolidate durable decisions, resolve contradictions, remove stale guidance, and check links and terminology. Record the reviewed execution-order range and outcome; only then advance the registry watermark.

## Completion

A standard cycle can be marked complete only after all acceptance criteria are verified or an explicitly accepted exception is recorded, the final diff is reviewed, required checks have actual results, documentation gates pass, and plan/handoff reflect the real outcome. Then set current_phase and status to complete and record completed_at.

A micro cycle can be marked complete only after its narrow acceptance condition is met, the exact diff is reviewed, focused verification is recorded, and outcome is truthfully documented in the registry.

Before stopping or handing off, update the current cycle checkpoint. A session ending, a commit, a completed plan, or a green partial CI run does not complete a cycle. After a cycle is fully complete or safely paused through the explicit switching procedure, another eligible cycle may proceed.

## Template setup and product documentation

The first-use bootstrap (replacing the root README from `README.template.md`, filling `docs/project/`, choosing product licensing, and updating `TEMPLATE_VERSION`) is setup work, not a development cycle. Complete it before the first ordinary implementation cycle. Product requirements, architecture, and verified commands belong in `docs/project/requirements.md`, `architecture.md`, and `commands.md`. The generic coding standard belongs in `docs/standards/coding.md`; keep all generic engineering standards together in `docs/standards/`. Coding standards must align with and defer structural and naming decisions to `architecture.md`; cycle plans and handoffs are temporary checkpoints only.


## Branch-policy adaptation

This template defaults to direct work on `main`. A derived repository that requires pull requests or another branch policy must make an explicit governance change to `steering.md` and reconcile its review and verification rules. Do not merely change the branch instruction or bypass branch protection; reconcile the collaboration, review, and verification rules so they remain safe across concurrent contributors and the selected integration model.



