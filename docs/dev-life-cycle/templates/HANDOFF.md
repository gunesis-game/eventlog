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

## Completed work

- <Actual changes completed; link to files or review artifacts>

## Decisions and constraints

- **Verified facts:** <Facts established by inspection or execution>
- **Decisions:** <Decision and rationale>
- **Assumptions:** <Provisional assumptions requiring confirmation>
- **Unknowns:** <Information not established>
- **Constraints:** <Relevant project constraints>

## Verification evidence

| Check | Revision | Result | Evidence / limitation |
|---|---|---|---|
| <Check> | <Exact revision> | <Passed / failed / not run> | <Log, report, reason, or limitation> |

## Documentation state

- **Updated:** <Relevant docs updated>
- **Still required:** <Remaining doc work>
- **Audit boundary:** <If this cycle performs historical documentation consolidation>

## Open issues, risks, and blockers

- **Status reason:** <required if blocked/cancelled>

- <Issue/risk/blocker, impact, and required action>

## Persisted phase checkpoint

- **Current phase:** <Must match cycle-index.json>
- **Exit evidence:** <Evidence already satisfying this phase, or what remains>
- **Next concrete action:** <Single specific next action>
- **Acceptance condition for next action:** <How success will be established>

## Uncommitted or unpushed work

- <Exact changes and preservation/recovery notes, or "None">

## Continuation instructions

<What the next contributor/agent should inspect or do first. Validate this handoff against the current repository before relying on it. The repository-wide execution claim is managed only by docs/dev-life-cycle/execution-lock.json.>
