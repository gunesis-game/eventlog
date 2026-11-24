# Engineering Standards

This directory is the single canonical home for the repository's reusable engineering standards. Keep all cross-project standards together here.

| Standard | Governs | Project-specific companion |
|---|---|---|
| [Coding Standard](coding.md) | Code organization, naming, implementation conventions, maintainability, and architectural alignment | The project's architecture and language/framework conventions |
| [Testing Standard](testing.md) | Test design, test levels, architecture-aware selection, feedback tiers, gates, and evidence | [Testing strategy](../project/testing-strategy.md) |
| [Performance Engineering Standard](performance-engineering.md) | Workloads, metrics, baselines, targets, benchmark quality, regression analysis, and evidence | [Performance strategy](../project/performance-strategy.md) |
| [Git Workflow Standard](git-workflow.md) | Repository collaboration, GitHub MCP use, commit/integration workflow, CI evidence, and recovery | [Git workflow policy](../project/git-workflow.md) |

## Ownership

- Standards define reusable minimum expectations and belong only in this directory.
- Project documentation defines the actual architecture, language/tooling choices, project-specific strategies, and approved exceptions.
- Do not create a second copy of a standard under docs/project/. Link to the canonical standard instead.
- When adding a new engineering standard, add it to this index and update steering, lifecycle, validator, README, and adoption references as applicable.
