# Development Cycle Handoff

## Cycle objective and status

- **Cycle UUID:** <UUID>
- **Objective:** <One-line objective>
- **Registry status:** <pending | in-progress | paused | blocked | cancelled | complete>
- **Persisted phase:** <assess | define | implement | verify | reconcile | complete>
- **Last updated:** <UTC ISO-8601 timestamp from the system clock>
- **Last progress checkpoint:** <UTC ISO-8601 timestamp from the system clock; update at each checkpoint event: a material decision or investigation result, a phase transition, and before pausing or stopping>

## Current repository state

- **Branch/revision:** <Observed branch and full commit SHA>
- **Working tree:** <Clean or exact outstanding changes>
- **Collaboration state:** <Committed, pushed, awaiting review, etc.>
- **Relevant environment:** <Only details that affect continuation>
- **Applicable coding standard:** <Path, verified revision and relevant conventions>
- **Approved Git workflow:** <Strategy path, decision ID and integration state>

## Completed work

- <Actual changes completed; link to files or review artifacts>

## Context and constraints

- **Verified facts:** <Facts established by inspection or execution>
- **Assumptions:** <Provisional assumptions requiring confirmation>
- **Unknowns:** <Information not established>
- **Constraints:** <Relevant project constraints>

**Durable decisions:** <Relevant decision IDs in docs/project/decisions.md; do not duplicate decision outcomes here.>

## Architecture impact and feedback selection

- **Changed components:** <Components>
- **Affected dependants/consumers:** <Direct and downstream dependants>
- **Affected contracts/boundaries:** <Interfaces and integrations>
- **Fast-feedback suites selected:** <Suites and architectural impact rationale>
- **Integration suites selected:** <Suites and trigger rationale>
- **Full-verification requirement:** <Required complete suite and applicable performance checks>

## Verification evidence

| Check / workflow / suite | Exact commit SHA | Run / job / command reference | Result | Evidence / limitation |
|---|---|---|---|---|
| <Check> | <Full SHA> | <Run/job/log/report> | <Passed / failed / skipped / cancelled / not triggered / inaccessible / not run> | <Observed details> |

A triggered workflow, commit response, skipped check, or result for another SHA is not a pass. Record failures and corrective revisions explicitly.

## Performance evidence

| Workload / benchmark | Exact commit SHA | Environment / baseline | Metrics and result | Evidence / limitation |
|---|---|---|---|---|
| <Workload or approved N/A> | <Full SHA> | <Environment and baseline> | <Observed result> | <Report or rationale> |

## Documentation state

- **Updated:** <Relevant docs updated>
- **Still required:** <Remaining doc work>
- **Audit boundary:** <If this cycle performs historical documentation consolidation>

## Remaining-work reconciliation

- **Last reconciled repository revision:** <Full commit SHA>
- **Requirements and architecture sections reviewed:** <Paths and headings; include detailed topics and unresolved/open-questions lists>
- **Decision entries checked:** <Decision IDs, states, owning specification paths, and verified revisions>
- **Stale plan/handoff entries corrected:** <Items corrected, or None>
- **Material contradictions:** <Contradictions and resolution action, or None>

| Remaining item | Classification (unresolved / explicitly deferred / contradictory) | Specification / decision references | Evidence and next action |
|---|---|---|---|
| <Item> | <Classification> | <Paths and IDs> | <Evidence and action> |

Only list work as unresolved when the current specification and recorded decisions support that classification. Do not duplicate decision outcomes; reference their IDs.

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

<What the next contributor/agent should inspect or do first. Validate this handoff against the current repository and exact commit SHA before relying on it. Contributors coordinate through the approved Git workflow.>
