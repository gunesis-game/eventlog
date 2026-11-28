# Git Workflow Standard

## Purpose

This standard defines the minimum repository collaboration and evidence workflow for AI-assisted and human development. The [project policy](#project-policy) at the end records the choices this repository has made.

## Define the repository workflow

Before implementation begins, the project must explicitly decide and document:

- Branch and integration model (for example, direct commits to a protected main branch or topic branches integrated through pull requests).
- Required review, approval, status checks, branch protections, and merge policy.
- Commit granularity, message conventions, signing requirements, and authorship expectations.
- How conflicts, failed checks, rollback, and recovery are handled.
- CI workflows, triggers, suite selection, and required gates.
- How exact commit SHAs connect source changes to verification evidence.

The selected model must respect repository permissions and protections. Never bypass branch protection, required review, security controls, or failing checks.

## Repository interaction

When GitHub MCP is available for the repository, agents use it to inspect repository state, read and write files, publish commits, and retrieve commit, workflow-run, job, log, and artifact evidence. Do not substitute assumptions, chat recollection, or an unverified local view for repository state. If the connector is unavailable or lacks permission, report the limitation; do not claim the protocol was followed.

GitHub MCP is the agent's interaction and evidence interface; GitHub Actions is the CI execution environment. A workflow trigger or successful write response does not establish that checks passed.

## Implementation and verification loop

For each meaningful implementation iteration:

1. Select one small, coherent change and understand its dependencies.
2. Add or update focused tests and documentation as appropriate.
3. Implement and inspect the diff for scope, correctness, compatibility, and accidental changes.
4. Commit and publish using the approved workflow.
5. Identify the CI workflows for the exact commit SHA and wait for completion where possible.
6. Inspect statuses and the relevant jobs, steps, logs, test reports, and artifacts.
7. Diagnose failures, correct the cause, and repeat against the new revision before proceeding.

Do not accumulate known failures and continue as though feedback were green. A commit is a reviewable checkpoint, not proof of correctness.

## Evidence integrity

- Record the full commit SHA for every material verification result.
- Confirm the workflow actually tested that SHA; do not transfer results from another revision.
- Distinguish success, failure, skipped, cancelled, not triggered, inaccessible, and not run. A missing or incomplete required result is not a pass.
- Preserve links or identifiers for runs, jobs, logs, reports, and artifacts where available.
- Never claim that tests, CI, review, publication, or deployment succeeded unless the result was observed.

## Workflow-run retention and scheduling

- Do not configure scheduled workflow runs. Use event-driven triggers and manual dispatch.
- Every workflow cleans run history at startup with the shared script `scripts/cleanup-workflow-runs.py`, from a job that checks out the default branch, never the triggering revision.
- **Repository-wide sweep:** each cleanup deletes every run whose conclusion is `cancelled` or `skipped`, across all workflows and regardless of age. It fetches every page of run history before deleting, so deletions cannot shift offset pagination and hide runs.
- **Per-workflow retention:** for the workflow being cleaned, keep the four most recent completed runs whose conclusion is `success` or `failure`, and delete older ones and every other completed conclusion.
- Never delete an active run. Each active run, including the current one, reserves one of the four slots until it completes.
- A retired workflow's remaining runs are cleaned through the manual inputs of `Workflow history cleanup` (a workflow ID plus any of its run IDs).

## Adaptation and exceptions

A project may choose a different branching or review model if it is explicit, approved, and compatible with repository controls. An exception to this standard states its scope, rationale, risk, and compensating controls. Changes to branch policy, required checks, or evidence requirements are material decisions.

## Project policy

Approved as D-043 in the [decision register](../project/decisions.md).

- **Main only.** All changes are committed directly to `main`. No other branches, pull requests, or merge commits. If protections block a direct-main operation, stop and report it rather than working around it.
- **Commits.** One coherent change per commit, with a concise message describing it. Do not force-push, rewrite shared history, or amend published commits; roll back with a new corrective commit.
- **Not yet verified.** Repository branch-protection and ruleset settings, and signing or authorship requirements, have not been checked. Do not assume they are absent or enabled.
- **CI.** The workflows and their triggers are listed in [commands.md](../project/commands.md#ci).
