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

<Describe the observed problem or opportunity. Link to relevant project documentation, code, issues, and the project decision register. Separate verified facts from assumptions and unknowns.>

## Scope

### In scope
- <Specific deliverables>

### Out of scope
- <Explicit non-goals and deferred work>

## Constraints, dependencies, and risks

- **Constraints:** <Established project constraints>
- **Dependencies:** <Required work, systems, or decisions>
- **Risks:** <Risk and mitigation>
- **Open decisions:** <Decision IDs from docs/project/decisions.md, owner, and when needed>

## Design and implementation approach

<Describe the bounded approach and relevant alternatives/trade-offs. Do not duplicate permanent requirements, decisions, or architecture documentation here.>

## Ordered tasks

- [ ] <Task 1>
- [ ] <Task 2>
- [ ] <Task 3>

## Acceptance criteria

- [ ] <Observable, testable outcome>
- [ ] <Relevant regression/compatibility condition>
- [ ] <Documentation or operational condition, or explain why none applies>

## Verification strategy

- **Local checks:** <Project-specific commands/tools, or unavailable>
- **CI/integration checks:** <Project-specific workflows and required evidence>
- **Manual review:** <Relevant behavior, risks, interfaces, or user-facing checks>

## Persisted phase checkpoint

- **Current phase:** assess
- **Phase exit evidence:** <What must be true to leave this phase>
- **Next concrete action:** <One specific action>
- **Last updated:** <UTC ISO-8601 timestamp>

## Progress

<Keep this section concise. Record completed work, current state, blockers, and the next action. The handoff is the detailed current checkpoint; the project decision register is maintained separately.>

## Completion review

- [ ] Acceptance criteria verified or exceptions explicitly accepted
- [ ] Final diff reviewed
- [ ] Required verification results recorded
- [ ] Documentation reconciled
- [ ] Handoff updated
- [ ] Registry status, phase, and completion timestamp updated consistently
