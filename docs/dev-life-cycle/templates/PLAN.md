# Development Cycle Plan

## Registry identity

- **Cycle UUID:** <UUID>
- **Sequence:** <sequence>
- **Execution order:** <assigned when first started>
- **Status:** pending | in-progress | paused | blocked | cancelled | complete
- **Status reason:** null unless blocked/cancelled; record the blocker or cancellation rationale in registry and handoff
- **Current phase:** assess
- **Created:** <UTC ISO-8601 timestamp or null if unknown>

## Objective

<One concise statement of the outcome this cycle must deliver.>

## Problem and context

<Describe the observed problem or opportunity. Link to relevant project documentation, code, issues, and decisions. Separate verified facts from assumptions and unknowns.>

## Scope

### In scope
- <Specific deliverables>

### Out of scope
- <Explicit non-goals and deferred work>

## Constraints, dependencies, and risks

- **Constraints:** <Established project constraints>
- **Dependencies:** <Required work, systems, or decisions>
- **Risks:** <Risk and mitigation>
- **Open decisions:** <Decision owner and when it is needed>

## Applicable engineering standards and strategies

- **Coding standard:** <Path and verified revision; note relevant conventions or justified exceptions>

- **Testing strategy:** <Path, verified revision, decision ID; identify fast/integration/full suites relevant to this cycle>
- **Performance strategy:** <Path, verified revision, decision ID; identify applicable workloads/checks or approved inapplicability>
- **Git workflow:** <Path, verified revision, decision ID; branch/integration and evidence model>

## Authoritative-state reconciliation

Complete this when assessing or redefining the remaining work, and refresh it before recommending a phase transition.

- **Repository revision inspected:** <Full commit SHA>
- **Canonical requirements sections reviewed:** <Paths and headings, or N/A with reason>
- **Canonical architecture sections reviewed:** <Paths and headings, or N/A with reason>
- **Decision entries checked:** <Decision IDs, states, owning specification paths, and verified revisions>
- **Unresolved/open-questions lists checked against detailed specifications:** <Yes/no and findings>

| Candidate item | Classification (resolved / unresolved / explicitly deferred / contradictory) | Canonical specification / decision IDs | Evidence or conflict; required action |
|---|---|---|---|
| <Item> | <Classification> | <Paths and IDs> | <Evidence and next action> |

Do not copy decision outcomes here. Link to the owning specification and decision IDs. A stale plan entry or unresolved-items list is not evidence that work remains open.

## Design and implementation approach

<Describe the bounded approach and relevant alternatives/trade-offs. Do not duplicate permanent architecture documentation here.>

## Architecture impact and test selection

- **Changed components:** <Components>
- **Affected dependants/consumers:** <Direct and downstream dependants>
- **Affected contracts/boundaries:** <Interfaces and integrations>
- **Fast-feedback suites:** <Focused tests selected from testing-strategy.md and why>
- **Integration suites:** <Affected boundary/system tests and trigger rationale>
- **Full-verification gate:** <Complete required suite and workflows required before leaving Implement>

## Ordered tasks

- [ ] <Task 1>
- [ ] <Task 2>
- [ ] <Task 3>

## Acceptance criteria

- [ ] <Observable, testable outcome>
- [ ] <Relevant regression/compatibility condition>
- [ ] <Documentation or operational condition, or explain why none applies>

## Verification strategy

- **Local checks:** <Verified project-specific commands/tools, or unavailable>
- **Fast feedback:** <Suites, trigger and expected evidence>
- **Integration feedback:** <Suites and trigger>
- **Full verification:** <All required suites and checks; mandatory implementation exit gate>
- **Performance:** <Applicable workloads, benchmark commands, targets or informational-only policy>
- **CI evidence:** <Workflow names, exact-SHA procedure, required jobs/reports/artifacts>
- **Manual review:** <Relevant behavior, risks, interfaces, or user-facing checks>

## Git workflow

- **Branch/integration model:** <Approved project model>
- **Commit/publish checkpoints:** <Approved procedure>
- **Review/status requirements:** <Required checks and approvals>
- **Failure/recovery:** <How failed checks or publication are handled>

## Persisted phase checkpoint

- **Current phase:** assess
- **Phase exit evidence:** <What must be true to leave this phase>
- **Next concrete action:** <One specific action>
- **Last updated:** <UTC ISO-8601 timestamp>

## Progress

<Update completed tasks, progress, and scope changes. Link durable decisions by ID in docs/project/decisions.md. Keep the registry phase synchronized with this checkpoint.>

## Completion review

- [ ] Acceptance criteria verified or exceptions explicitly accepted
- [ ] Full required verification gate passed for the final implementation SHA
- [ ] Applicable performance checks completed and recorded
- [ ] Final diff reviewed
- [ ] Required workflow results and exact revisions recorded
- [ ] Documentation reconciled
- [ ] Handoff updated
- [ ] Registry status, phase, and completion timestamp updated consistently
