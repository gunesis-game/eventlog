# Coding Standards

## Purpose

These coding standards establish consistent expectations for code organization, naming, implementation, and maintainability. They apply across the project and must be interpreted according to the language, framework, and tooling used by the project. They own implementation conventions and code quality—not product requirements or architectural decisions.

## Documentation Authority

- Product capabilities, observable behavior, domain rules, constraints, quality expectations, and acceptance criteria belong in `requirements.md`.
- Technical structure, boundaries, responsibilities, interfaces, algorithms, storage/data representation, and design trade-offs belong in `architecture.md`.
- These standards govern implementation quality and consistency within the approved architecture; they do not create or redefine either requirements or architecture.
- Cycle plans and handoffs record temporary scope, progress, decisions awaiting approval, and next actions. They are not durable sources of product requirements or architecture.
- Keep details in their owning document. Use concise references instead of duplicating full content. If a durable decision changes, update its owner and reconcile dependent documents.
- Clearly label proposals, approved decisions, verified implementation facts, and unresolved questions. Never present a proposal as approved or intended design as implemented/verified behavior.

## Architectural Alignment

Code organization, naming, and implementation must align with the architecture defined in [`architecture.md`](architecture.md).

- Architectural decisions determine structural boundaries, responsibilities, dependencies, and placement of code units.
- Coding standards govern consistency and quality of implementation within those boundaries; they must not introduce architectural decisions independently.
- New code must follow the established architecture unless an architectural change is intentionally approved and documented.
- Implementation must not disguise an architectural change as an ordinary code change. Where implementation requires a deviation from established architecture, the architectural documentation must be addressed accordingly.
- Language and framework conventions remain applicable, provided they do not conflict with the project's documented architecture or standards.

## Code Structure and File Ownership

### Principle

Code must be organized into clearly named, cohesive, maintainable units. Each file must have an identifiable primary code unit and responsibility.

### Rules

1. **One primary code unit per file.** A file must not contain multiple independent classes, types, modules, or equivalent primary code units.
2. **Filename alignment.** The file name must correspond to and clearly identify its primary code unit. Where the language supports classes, the filename must match the class name using the language's established convention.
3. **No class splitting.** A class must not be distributed across multiple files unless the language, framework, or tooling explicitly requires or conventionally supports that mechanism. Any such exception must be documented.
4. **No class aggregation.** Multiple independent classes must not be combined in one file for convenience.
5. **Cohesive ownership.** Methods, functions, and related implementation details must remain with the code unit responsible for their behavior.
6. **Single responsibility.** Each code unit must have a clear responsibility. Independent or unrelated responsibilities must be separated into distinct code units and files.
7. **Language-aware application.** These rules must be interpreted according to the language and ecosystem. In languages without classes, apply the same principles to the equivalent primary code unit (such as a module, function, script, or named type).
8. **Explicit exceptions.** Deviations must be justified by language, framework, tooling, or architectural requirements—not convenience alone.

## Naming

- Names must clearly communicate the responsibility or purpose of the code unit or member.
- File and primary code-unit names must correspond, following the language's conventional casing and naming rules.
- Naming conventions must be applied consistently within the project and must respect the language and framework idioms.
- Avoid vague or catch-all names such as `Utils`, `Helpers`, `Common`, or `Misc` unless their scope and purpose are clearly justified by the architecture.
- Methods and functions should use names that communicate the action or behavior they provide. Use verbs where appropriate.
- Avoid context-free names such as `DoWork`, `Process`, `Handle`, `Execute`, `GetData`, or `Check` when a more specific name can express intent.
- When a code unit is renamed, update its filename and all relevant references, imports, registrations, and documentation.
- Naming and placement decisions must be consistent with `architecture.md`.

## Class and Code-Unit Responsibility

- Each class or equivalent code unit must have a coherent, clearly defined responsibility.
- A code unit must not become a container for unrelated functionality.
- Responsibilities should be separated when doing so improves cohesion, reduces coupling, or clarifies ownership.
- Do not create additional code units solely to meet arbitrary size or line-count targets. Split based on responsibility, cohesion, coupling, and clarity.
- Avoid oversized units that accumulate unrelated responsibilities or become difficult to understand, test, or maintain.

## Methods and Functions

- Methods and functions must have a clear, focused purpose.
- Keep behavior close to the code unit that owns it, consistent with the architecture.
- Avoid unnecessarily long methods that combine unrelated steps or responsibilities.
- Prefer clear, direct control flow over clever or unnecessarily complex implementations.
- Avoid hidden side effects; behavior and dependencies should be understandable from the implementation and its interface.
- Extract methods or functions when doing so meaningfully improves clarity, reuse, or testability—not merely to reduce line count.

## Callback Functions

- Avoid inline callback arrow functions (for example, `items.map(item => ...)`) when a named function or method can be used instead.
- Prefer named functions or methods for callbacks to improve readability, reuse, debugging, and testability.
- Keep callback bodies simple and declarative. If a callback contains branching, multiple operations, or meaningful business logic, extract it into a named function.
- Use inline arrow callbacks only when they are genuinely trivial and the surrounding code remains clear; do not use them as a substitute for named, reusable behavior.
- Apply this standard in languages and frameworks that support callback-style arrow functions, while respecting documented language-specific requirements.

## Braces and Block Formatting

- In languages that use braces to define blocks, always use braces for function, method, control-flow, and other block statements.
- Prefer explicit braces even when the language allows a single-line statement without braces.
- Do not place an entire block or an empty block on the same line as its opening brace. Use a multi-line format, including for empty blocks.

**Preferred:**
```
name() {
}
```

**Avoid:**
```
name() {}
```

- Keep the opening brace on the declaration or statement line, and place the closing brace on its own line.
- Apply the convention consistently, while respecting unavoidable language or formatter requirements.

## Exceptions and Language-Specific Conventions

- Language, framework, and tooling requirements may necessitate supporting declarations or special file structures.
- Exceptions must be narrowly scoped, idiomatic, and consistent with the architectural intent.
- Exceptions must not be used as a loophole to combine unrelated code units or split a code unit without a justified requirement.
- Where a language or framework imposes a required structure, follow that structure and document any meaningful deviation from the baseline standards.

## Consistency and Maintenance

- Follow established project conventions rather than introducing competing styles without a clear need.
- Changes must preserve or improve clarity, cohesion, and maintainability.
- When standards or architectural conventions need to change, update the governing documentation instead of relying on undocumented local practices.
