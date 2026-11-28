# Project Git Workflow

## Purpose and status

This document is the project-specific implementation of the [Git Workflow Standard](../standards/git-workflow.md). The repository's main-only policy is mandatory and overrides any generic examples in the standard. **This workflow is approved as the project baseline (D-043).** Repository-protection settings and signing/authorship requirements remain explicit verification follow-ups; this approval does not assume their state or waive any controls.

## Branch and integration model

- Working branch: main only.
- Integration mechanism: direct commits to main.
- Branch creation, switching, publishing, and work on any other branch are prohibited.
- Pull requests and merge commits from another branch are not part of this repository's development or governance workflow.
- All changes, including governance, documentation, verification and maintenance, are committed directly to main.
- Never bypass branch protection, required checks, access controls, or security controls. If they prevent the required direct-main operation, stop and report the blocker; do not use another branch or a pull request as a workaround.
- Repository branch-protection and ruleset settings have not been verified in this document. Do not assume they are absent or enabled.

## Commit policy

- Commit one coherent, reviewable change at a time.
- Use concise commit messages that describe the change. A more specific message convention and signing/authorship requirements have not been approved here.
- Do not force-push, rewrite shared history, or amend published commits.
- Fetch back every changed artifact after publication and verify its exact content and commit revision.

## Repository interaction and evidence

- GitHub MCP is the required interface for repository inspection, file reads and writes, commits, fetch-back verification, and retrieval of commit and Actions evidence whenever it is available.
- If GitHub MCP is unavailable or lacks permission, stop and report the limitation; do not claim repository work or evidence was completed through it.
- GitHub Actions is the CI execution environment; a successful write or workflow trigger is not proof that checks passed.
- Current workflow files:
  - [Go verification](../../.github/workflows/go.yml) runs formatting (Linux), tests (Linux and Windows), race tests (Linux), build, and vet on pushes to `main` and manual dispatch. Its startup cleanup job applies the shared run-retention policy.
  - [Workflow history cleanup](../../.github/workflows/cleanup-workflow-history.yml) is manual-dispatch only. It cleans its own workflow history and, when both inputs are supplied, applies the per-workflow retention policy to the selected workflow. The supplied run ID is used as a current-run/retention-slot placeholder if that run is not found in the fetched list; it is not a request to delete that specific run.
- No separate steering-validation workflow is present in the current tree. No `workflow_run` trigger is configured for automatic cleanup of a completed source workflow.
- Go verification is configured in `.github/workflows/go.yml`; do not describe project Go verification as absent.
- The one-time initialization workflow is not retained in this repository because initialization is complete. Its historical Actions runs are still subject to the same four-run retention rule; use the cleanup workflow's manual historical-target inputs when cleaning a retired workflow.
- No scheduled workflow runs are permitted. Use event-driven triggers and manual dispatch only where justified.
- Associate verification with the exact commit SHA. Inspect the workflow run, status, jobs, steps, logs, and artifacts as available.

## Workflow-run retention

Every ongoing workflow must clean its own history at startup using the shared scripts/cleanup-workflow-runs.py policy. The cleanup workflow also applies retention to the exact completed source run after the validation workflow finishes.

1. Repository-wide cleanup deletes every cancelled and skipped run across all workflows, regardless of age.
2. For each workflow, retain only the four latest completed success or failure runs combined.
3. Delete older success/failure runs and every other completed conclusion.
4. Never delete active runs. Each active run reserves one of the four retention slots until it completes.
5. Fetch all pages of repository run history before deleting repository-wide candidates so offset pagination cannot shift and hide runs.
6. The trusted workflow_run handler checks out the default branch, not the triggering revision, and never executes pull-request code with write permissions.

## Failure and recovery

- Treat failed required checks as blockers. Diagnose the cause, correct it on main, and verify the new exact commit.
- A skipped, cancelled, inaccessible, or missing required result is not a pass.
- Preserve user changes and unrelated repository state. Never discard or overwrite work to make a check green.
- If publication fails or permissions prevent direct-main work, stop and report the exact failure.
- Rollback through a new corrective commit on main; never rewrite shared history.

## Define approval checklist

- [x] Main-only direct-commit policy is explicit and consistent with steering.md.
- [x] GitHub MCP is the required repository interaction and evidence interface.
- [x] Governance validation and cleanup workflow paths are documented.
- [ ] Repository protection/ruleset settings and signing/authorship requirements are verified (operational follow-up; this does not block approval of the documented workflow baseline).
- [ ] Project test/build/lint workflows are defined when those suites are implemented (implementation follow-up).
- [x] Workflow-run retention and trusted cleanup behavior are explicit.
