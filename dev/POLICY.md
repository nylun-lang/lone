# Dev Directory Policy & Collaboration Guide

**Status**: Active  
**Effective Date**: 2026-11-04  
**Applies To**: Human Contributors & AI Assistants (Pair Programming)  

---

## 1. Purpose & Core Principles

The `dev/` directory is the central engine for Lone's language design, architectural evolution, exploratory research, and historical accounting. It separates **ideation**, **decision-making**, **living specifications**, and **historical records** into clear, non-overlapping pillars.

### The 4 Working Principles:
1. **P1 - SPEC is Ground Truth**: `dev/SPEC.md` strictly reflects what is *actually implemented and verified* in the active compiler on `main`/`reimpl`. It is never a wish-list.
2. **P2 - Traceable Lifecycle**: Features progress predictably:
   $$\text{Research} \longrightarrow \text{RFC Draft} \longrightarrow \text{Decision (ADR / Acceptance)} \longrightarrow \text{Implementation} \longrightarrow \text{SPEC \& Docs}$$
3. **P3 - Immutable Decisions & Archives**: Accepted ADRs and historical archives (`dev/archive/`) are frozen records. When a design changes, a new ADR supersedes the old one rather than mutating history.
4. **P4 - Continuous Session Accounting**: Every engineering session and milestone must leave a traceable report in `dev/sessions/` capturing what was built, rationale, test results, and next steps.

---

## 2. Directory Taxonomy & Responsibilities

```
dev/
├── POLICY.md                  # This policy document
├── SPEC.md                    # The living, authoritative Lone Language Specification
├── adr/                       # Architectural Decision Records (ADRs)
├── rfcs/                      # Formal Request for Comments & Language Proposals
│   ├── drafts/                # Work-in-progress proposals under active discussion
│   ├── accepted/              # Approved proposals ready for or undergoing implementation
│   └── rejected/              # Closed or superseded proposals with documented rationale
├── research/                  # Investigative studies, exploratory spikes, and measurements
│   ├── exploration/           # Syntax sketches, semantics experiments, prior-art surveys
│   ├── audits/                # Language consistency audits, compiler bottleneck analyses
│   └── benchmarks/            # Performance profiling, allocation tests, Wasm size metrics
├── sessions/                  # Daily session logs and task milestone reports
│   ├── YYYY-MM-DD/            # Granular milestone reports for that date
│   └── YYYY-MM-DD.md          # Daily roll-up summary & report index
├── internals/                 # Deep-dive guides to current compiler subsystems & IRs
└── archive/                   # Frozen historical documents
    ├── legacy/                # Initial 2026-08 drafts and specs
    └── rfcs-prev0.5.0/        # Superseded pre-v0.5.0 RFCs and sketches
```

---

## 3. Subdirectory Policies & Workflows

### 3.1 `dev/SPEC.md` (Living Language Specification)
- **Role**: Single source-of-truth reference for the current language version (`v0.5.0-beta`).
- **Update Rule**: Updated **only** when a feature is implemented, tested, and passing in `pkg2/` and `test2/`.
- **Content**: Lexical rules, syntax definitions, type system invariants, standard library contracts, and memory model.

---

### 3.2 `dev/adr/` (Architectural Decision Records)
- **Role**: Documents critical, cross-cutting architectural choices (e.g. Free-List WebAssembly heap layout, Pratt parser adoption, two-tier linear memory partition).
- **Naming**: `ADR-NNN-<kebab-slug>.md` (e.g., `ADR-001-two-tier-linear-memory.md`).
- **Mutability**: Immutable once accepted. If a decision changes, create a new ADR that marks the previous one as `Superseded by ADR-NNN`.
- **Required Sections**:
  - `Status`: `Proposed` | `Accepted` | `Superseded by ADR-NNN`
  - `Context & Problem Statement`: Why are we making this decision?
  - `Decision`: What is the chosen path?
  - `Consequences`: Trade-offs, benefits, and migration requirements.

---

### 3.3 `dev/rfcs/` (Request for Comments)
Formal language and compiler proposals. All RFCs follow a structured lifecycle:

1. **`rfcs/drafts/`**:
   - Where new proposals are drafted.
   - Naming: `RFC-YYYYMMDD-<title>.md` or `NNNN-<title>.md`.
   - Used for interactive review between user and AI.
2. **`rfcs/accepted/`**:
   - Moved here once the user and AI reach consensus to implement.
   - Implementation work targets accepted RFCs.
3. **`rfcs/rejected/`**:
   - Proposals that were evaluated and rejected (or abandoned).
   - **Must** include a "Rejection Rationale" section explaining why the idea was not adopted (to avoid recurring debates).

---

### 3.4 `dev/research/` (Exploration & Audits)
Informal, exploratory, or analytical investigations that precede or support formal RFCs:
- **`research/exploration/`**:
  - Syntax sketches (e.g. pattern matching syntax options, trait dispatch variations).
  - Comparative analysis with other languages (Zig, Rust, Go, TypeScript).
- **`research/audits/`**:
  - Systematic code reviews, consistency audits across parser/analyzer/compiler, naming convention checks.
- **`research/benchmarks/`**:
  - Memory allocation measurements, compile-time performance profiles, Wasm text size comparisons.

---

### 3.5 `dev/sessions/` (Work Session Reports & Logs)
Maintains continuous project memory and transparency across pair-programming turns:
- **Organization**:
  - `dev/sessions/YYYY-MM-DD/`: Granular reports written at the completion of significant tasks or features (e.g. `2026-11-04_webassembly_linear_memory_and_freelist_allocator.md`).
  - `dev/sessions/YYYY-MM-DD.md`: Daily summary cataloging all accomplishments, commits, and links to detailed reports for that day.
- **Required Structure for Session Reports**:
  1. `Date & Scope`
  2. `Summary of Request`
  3. `Changes Implemented` (files modified/added)
  4. `Verification & Test Output` (exact command outputs)
  5. `Next Steps / Open Considerations`

---

### 3.6 `dev/internals/` (Compiler Subsystem Guides)
Technical documentation explaining *how the compiler works today*:
- Topics: Type system IR lowering, Pratt parser precedence table, datastack frame prologue/epilogue, Free-List coalescing algorithm.
- Updated whenever compiler internals or IR structures are refactored so that developers and AI assistants always have accurate architectural context.

---

### 3.7 `dev/archive/` (Historical Reference)
- Strictly read-only.
- Holds superseded RFCs, legacy drafts, and retired iterations.
- Never edit existing documents in `archive/`.

---

## 4. Collaborative Lifecycle (User $\longleftrightarrow$ AI Interaction)

When developing a new feature or architectural refactor, follow this 5-stage pipeline:

```
[1. Exploration]       dev/research/exploration/ (sketches & spikes)
       │
       ▼
[2. Proposal]          dev/rfcs/drafts/ (formal RFC authoring)
       │
       ▼
[3. Consensus]         User aligns -> Move to dev/rfcs/accepted/ (or rejected/)
       │                              Create dev/adr/ if architecturally significant
       ▼
[4. Implementation]    Code in pkg/, tests in test/, verify go test ./...
       │
       ▼
[5. Documentation]     - Update dev/SPEC.md
                       - Update dev/internals/ (if IR/subsystems changed)
                       - Write report in dev/sessions/YYYY-MM-DD/
                       - Update daily roll-up dev/sessions/YYYY-MM-DD.md
                       - Sync docs/docs/content/ for release versions (see docs/README.md & docs/WRITING_DOC_REQUIREMENT.md)
```

---

## 5. Agent Rules of Engagement

1. **Fact Checking**:
   - Look up current language rules in `dev/SPEC.md`.
   - Look up compiler internal mechanics in `dev/internals/`.
   - Never treat files in `dev/research/exploration/` or `dev/archive/` as active language rules.
2. **Writing Discipline**:
   - Write session reports only in `dev/sessions/YYYY-MM-DD/` and update `dev/sessions/YYYY-MM-DD.md`.
   - Place architectural proposals in `dev/rfcs/drafts/`. Never unilaterally move an RFC to `accepted` without explicit user confirmation.
   - Do not litter the root of `dev/` with ad-hoc files; always place files in the designated subfolder.
3. **Commit Integrity**:
   - Format all commit messages as:
     ```
     [Ai-assisted] <type>(<scope>): <description>
     ```
   - Keep `dev/SPEC.md`, tests, and implementation synchronized in commits.
