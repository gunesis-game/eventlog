# Git Workflow Standard

## Purpose and authority

This standard defines the minimum repository collaboration and evidence workflow for AI-assisted and human development. It applies throughout the lifecycle. The project-specific policy at `docs/project/git-workflow.md` defines the actual branch/integration model, commit conventions, review requirements, protections, and repository procedures.

Git workflow design belongs in Define; executing the approved workflow belongs in Implement and subsequent verification/reconciliation.

## Define the repository workflow

Before implementation begins, the project must explicitly decide and document:

- Branch and integration model (for example, direct commits to a protected main branch or topic branches integrated through pull requests).
- Required review, approval, status checks, branch protections, and merge policy.
- Commit granularity, message conventions, signing requirements, and authorship expectations.
- When changes are committed, pushed, reviewed, merged, and considered an implementation checkpoint.
- How conflicts, failed checks, rollback, and recovery are handled.
- Required GitHub MCP operations for repository interaction and evidence retrieval.
- GitHub Actions or other CI workflows, triggers, suite selection, and required gates.
- How exact commit SHAs connect source changes to verification evidence.

The selected model must respect repository permissions and protections. Never bypass branch protection, required review, security controls, or failing checks. Do not assume direct-to-main or PR-based work is universally correct; select and approve the model that fits the repository.

## GitHub MCP repository protocol

When GitHub MCP is available for the repository, agents must use it as the required interface for repository work throughout the lifecycle, including:

- Inspecting repository structure, source, tests, documentation, configuration, branches, commits, and current state.
- Reading, creating, and updating repository files.
- Publishing commits or changes through the approved repository workflow.
- Fetching back and verifying specifications, decisions, plans, handoffs, and other approved artifacts.
- Inspecting exact commits, workflow runs, statuses, jobs, steps, logs, and artifacts.
- Diagnosing failed checks and confirming corrective commits and reruns.

Use GitHub MCP for repository state and evidence retrieval; do not substitute assumptions, chat recollection, or an unverified local view. If the required connector is unavailable or lacks permission, stop and report the limitation; do not silently claim the protocol was followed.

GitHub MCP and GitHub Actions have different responsibilities: MCP is the agent's repository interaction and evidence interface; Actions is a CI execution environment. A workflow trigger or successful write response does not establish that checks passed.

## Implementation and verification loop

Follow the project-approved branch and integration model. For each meaningful implementation iteration:

1. Select one small, coherent change and understand its dependencies.
2. Add or update focused tests and documentation as appropriate.
3. Implement and inspect the diff for scope, correctness, compatibility, and accidental changes.
4. Commit and publish using the approved Git workflow.
5. Identify required CI workflows for the exact commit SHA; wait for completion where possible.
6. Inspect statuses and relevant jobs, steps, logs, test reports, and artifacts through GitHub MCP.
7. Diagnose failures, correct the cause, and repeat against the new exact revision before proceeding.

Do not accumulate known failures and continue as though feedback were green. A commit is a reviewable checkpoint, not proof of correctness.

## Evidence integrity

- Record the full commit SHA for every material verification result.
- Confirm the workflow actually tested that SHA; do not transfer results from another revision.
- Distinguish success, failure, skipped, cancelled, not triggered, inaccessible, and not run.
- A missing or incomplete required result is not a pass.
- Preserve links or identifiers for runs, jobs, logs, reports, and artifacts where available.
- Never claim that tests, CI, review, publication, or deployment succeeded unless the result was observed.

## Workflow-run retention and scheduling

- Do not configure scheduled workflow runs. Use event-driven triggers (such as push and pull request) and manual dispatch where justified.
- Every workflow that can run must clean its own run history at startup using the shared retention script. This explicitly includes `Initialize Repository`, `Steering state validation`, and `Workflow history cleanup`. Every cleanup invocation also performs a repository-wide sweep across all workflows.
- **Repository-wide deletion:** always delete every run whose conclusion is `cancelled` or `skipped`, regardless of age or originating workflow. Paginate through the complete repository Actions run history. Fetch all pages before deleting so deletions cannot shift offset pagination and cause records to be missed.
- **Per-workflow retention:** after the repository-wide sweep, retain the four most recent completed success/failure runs, combined, for each individual workflow (that is, only runs whose conclusion is `success` or `failure`). Delete older success/failure runs and delete any other completed conclusion that is not success or failure. Skipped and cancelled runs do not consume retention slots because they are deleted globally.
- Never attempt to delete an active run (queued, in progress, or otherwise not completed). Each active run reserves one of the four per-workflow retention slots until it completes, so a workflow cannot exceed four retained success/failure runs when those active runs finish.
- `Initialize Repository` cleans its own workflow history at startup. `Steering state validation` uses a separate trusted cleanup job to clean its own workflow history at startup; its validation job remains read-only, and pull-request runs do not receive write permissions. `Workflow history cleanup` cleans its own workflow history at startup and uses trusted `workflow_run` completion events for both `Initialize Repository` and `Steering state validation`, so a skipped run is swept after it completes and the exact completed source run also receives per-workflow retention. All three workflows use the same retention policy. The privileged completion handler checks out only the default branch and never checks out or executes pull-request code with write permissions.

## Adaptation and exceptions

The project-specific policy may choose a different branching or review model, provided it is explicit, approved, compatible with repository controls, and reconciles all related verification rules. Any exception to the standard must state its scope, rationale, risk, and compensating controls. Changes to branch policy, required checks, or evidence requirements are material governance decisions.
