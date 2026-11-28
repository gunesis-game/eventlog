# eventlog

## AI session routing — classify mode first

Before following development instructions, determine the intended mode from the session's objective:

- **Governance Mode:** The objective is to create, review, amend, repair, or improve the repository's governance or AI operating rules—such as steering, lifecycle policy, governance validators/workflows, generic engineering standards, or governance context-routing instructions. Follow the governance entry path and the narrowly scoped maintenance exemption. Audit only AI operating governance: process rules, gates, context routing, evidence, verification, and recovery. Product requirements, product architecture, feature correctness, and implementation documentation are out of scope unless directly needed to correct a governance rule. Do not create, select, resume, or advance an operational development cycle for work that remains within governance scope.
- **Operational Mode:** The objective is ordinary product work—requirements, product architecture, implementation, tests, product defects, behavior, or substantive product documentation. Follow `steering.md` and the development lifecycle. Reconcile persisted cycle state before substantive work.

Classify by **session intent and scope**, not merely by the files touched. Editing a product document to repair governance routing does not automatically make the session Operational; changing product requirements or architecture as actual project work does.

If intent is mixed or uncertain, clarify or split the work before proceeding. If a Governance Mode session discovers work outside the exemption, stop before undertaking it, identify the scope crossing, and route that work through Operational Mode. Do not silently expand the exemption.

Only after classification, load the relevant documents for that mode. References are conditional routing links, not instructions to recursively read every linked document.

---

## Overview

A lightweight, disk-backed append-only log written in Go. Designed as the persistent storage layer for event-sourced systems, it sequentially writes immutable events to disk so applications can reliably rebuild in-memory state after a restart and transition seamlessly into tailing live events.

## Getting started

<Prerequisites and setup instructions.>

## Usage

<Core workflows or examples.>

## Development

<Commands for install, run, test, lint, and build. Link to docs/project/commands.md.>

## Documentation

### Project specifications
- [Requirements](docs/project/requirements.md)
- [Architecture](docs/project/architecture.md)
- [Acceptance strategy](docs/project/acceptance.md)
- [Benchmark strategy](docs/project/benchmarks.md)
- [Project decisions](docs/project/decisions.md)

### Engineering standards
- [Engineering standards index](docs/standards/README.md)
- [Coding standard](docs/standards/coding.md)
- [Testing standard](docs/standards/testing.md)
- [Performance engineering standard](docs/standards/performance-engineering.md)
- [Git workflow standard](docs/standards/git-workflow.md)

### Project policies
- [Testing strategy](docs/project/testing-strategy.md)
- [Performance strategy](docs/project/performance-strategy.md)
- [Git workflow policy](docs/project/git-workflow.md)
- [Development commands](docs/project/commands.md)

### Governance
- [Steering](steering.md)
- [Development lifecycle](docs/dev-life-cycle/README.md)

## License

<Replace with this product's licensing terms.>
