# Git Workflow Standard

## Main only

**This repository uses one branch: `main`. There are no other branches, ever.** All work, by people and agents alike, is committed directly to `main`. Do not create, push, check out, or propose any other branch. Do not open pull requests and do not create merge commits. This rule is D-043 in the [decision register](../project/decisions.md) and holds in every document; nothing in this or any other document permits an exception.

If a protection or a tool blocks a direct commit to `main`, stop and report it. Do not work around it with a branch or a pull request.

## Purpose

This standard defines the repository collaboration and evidence workflow for AI-assisted and human development. The [project policy](#project-policy) at the end records this repository's remaining choices. Never bypass protections, security controls, or failing checks.

## Repository interaction

When GitHub MCP is available for the repository, agents use it to inspect repository state, read and write files, publish commits, and retrieve commit, workflow-run, job, log, and artifact evidence. Do not substitute assumptions, chat recollection, or an unverified local view for repository state. If the connector is unavailable or lacks permission, report the limitation; do not claim the protocol was followed.

GitHub MCP is the agent's interaction and evidence interface; GitHub Actions is the CI execution environment. A workflow trigger or successful write response does not establish that checks passed.

## Implementation and verification loop

For each meaningful implementation iteration:

1. Select one small, coherent change and understand its dependencies.
2. Add or update focused tests and documentation as appropriate.
3. Implement and inspect the diff for scope, correctness, compatibility, and accidental changes.
4. Commit to `main` and push.
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
- Label every reported result with one status, naming the commit:
  - **CI verified at `<sha>`:** the required CI run for that exact commit completed and passed.
  - **Local only at `<sha>`:** the commands passed on a named local platform; no CI run for that commit was observed. Say "working tree" if the changes were uncommitted.
  - **CI pending at `<sha>`:** pushed, and the run has not finished.
  - **Failed at `<sha>`:** a required check failed; name it.
  - **Not verified:** nothing was run.
- A result for one commit is never evidence for another. When citing a recorded result, give its status and commit, and say that later commits are not covered.

## Workflow-run retention and scheduling

- Do not configure scheduled workflow runs. Use event-driven triggers and manual dispatch.
- Every workflow cleans run history at startup with the shared script `scripts/cleanup-workflow-runs.py`, from a job that checks out the default branch, never the triggering revision.
- **Repository-wide sweep:** each cleanup deletes every run whose conclusion is `cancelled` or `skipped`, across all workflows and regardless of age. It fetches every page of run history before deleting, so deletions cannot shift offset pagination and hide runs.
- **Per-workflow retention:** for the workflow being cleaned, keep the four most recent completed runs whose conclusion is `success` or `failure`, and delete older ones and every other completed conclusion.
- Never delete an active run. Each active run, including the current one, reserves one of the four slots until it completes.
- A retired workflow's remaining runs are cleaned through the manual inputs of `Workflow history cleanup` (a workflow ID plus any of its run IDs).

## Exceptions

An exception to this standard states its scope, rationale, risk, and compensating controls and needs an approved decision. The [main-only rule](#main-only) has no exceptions.

## Project policy

Approved as D-043 in the [decision register](../project/decisions.md).

- **Main only.** See [Main only](#main-only).
- **Commits.** One coherent change per commit, with a concise message describing it. Do not force-push, rewrite shared history, or amend published commits; roll back with a new corrective commit.
- **Ruleset target for D-043.** The active default-branch ruleset should contain only `creation`, `deletion`, and `non_fast_forward`. These prevent additional branches, branch deletion, and force-pushes while allowing ordinary direct commits to `main`. It must not contain `update` or `pull_request`, and its bypass-actor list must be empty. This enforces main-only without requiring anyone to bypass a protection.
- **Observed ruleset (verified 2026-11-28 through the public GitHub API at `79c638534778935e89933583ccf89f2478662328`).** The active ruleset "default branch" (ID 28287256) applies to all branch names, but currently also contains `update` and `pull_request`. Its bypass list contains an OrganizationAdmin entry, RepositoryRole ID 5, and Integration ID 1144995; the API reports that the connected identity can bypass. The actor identities have not been resolved. These settings conflict with D-043 and the ruleset target above; they are not permission to bypass protections or use pull requests. Until the ruleset is aligned, an agent must stop and report if a direct push is refused.
- **CI.** CI checks cannot block a direct push; they run after it lands. Verification before committing is the committer's responsibility ([Implementation and verification loop](#implementation-and-verification-loop)). The workflows and their triggers are listed in [commands.md](../project/commands.md#ci).
- **Signing and authorship.** Signed commits are not required by the observed ruleset, and current commits are unsigned. D-043 does not decide a signing or authorship requirement.
