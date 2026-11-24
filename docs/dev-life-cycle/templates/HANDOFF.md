# Development Cycle Handoff

## Cycle objective and status

- **Cycle UUID:** <UUID>
- **Objective:** <One-line objective>
- **Registry status:** <pending | in-progress | paused | blocked | cancelled | complete>
- **Persisted phase:** <assess | define | implement | verify | reconcile | complete>
- **Last updated:** <UTC ISO-8601 timestamp>

## Current repository state

- **Branch/revision:** <Observed branch and revision, if available>
- **Working tree:** <Clean or exact outstanding changes>
- **Collaboration state:** <Committed, pushed, awaiting review, etc.>
- **Relevant environment:** <Only details that affect continuation>

## Current checkpoint

- **Completed since the previous checkpoint:** <Actual work completed>
- **Current state:** <What is established now>
- **Next action:** <One concrete action>
- **Acceptance condition:** <How success will be established>

## Decisions and constraints

- **Durable decision register:** [docs/project/decisions.md](../../../project/decisions.md)
- **Current specifications:** [Requirements](../../../project/requirements.md) · [Architecture](../../../project/architecture.md)
- **Pending decisions:** <Decision IDs and status, or None>
- **Verified facts:** <Facts established by inspection or execution>
- **Assumptions:** <Provisional assumptions requiring confirmation>
- **Unknowns:** <Information not established>
- **Constraints:** <Relevant project constraints>

The decision register and project specifications are authoritative for durable project decisions. This handoff records only the cycle's current progress and continuation state; do not copy the decision register or project specification into this file.

## Verification evidence

| Check | Revision | Result | Evidence / limitation |
|---|---|---|---|
| <Check> | <Exact revision> | <Passed / failed / not run> | <Log, report, reason, or limitation> |

## Open issues, risks, and blockers

- **Status reason:** <required if blocked/cancelled>
- <Issue/risk/blocker, impact, and required action>

## Persisted phase checkpoint

- **Current phase:** <Must match cycle-index.json>
- **Exit evidence:** <Evidence already satisfying this phase, or what remains>
- **Next concrete action:** <Single specific next action>

## Uncommitted or unpushed work

- <Exact changes and preservation/recovery notes, or None>

## Continuation instructions

<What the next contributor/agent should inspect or do first. Validate this handoff against the current repository before relying on it.>
