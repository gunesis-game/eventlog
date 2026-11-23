undefined

## Design checkpoint — standalone and embedded JSON Schemas approved — 2026-11-23

- User approved Option C: JSON Schema files will be stored as standalone, versioned repository files and embedded into the Go package at build time.
- Standalone schema files are the canonical source; the embedded copies are derived from those same files using Go `//go:embed`, avoiding duplicate schema definitions and drift.
- Schema layout is versioned (v1), with record/value schemas organized under a dedicated schemas directory. Tooling can inspect and validate persisted records without compiling the Go library; runtime/package code can access the same schema bytes through embedding.
- Cycle 1 remains in Define. No implementation, build, or Go tests have started.
