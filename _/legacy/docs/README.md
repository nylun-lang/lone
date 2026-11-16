# How to Write and Maintain Documentation (`docs/`)

Welcome to the **Lone Documentation** authoring guide. This document explains the architecture, directory structure, versioning mechanisms, authoring standards, and step-by-step workflows for writing and maintaining Lone's official documentation website.

---

## 1. Overview & Technology Stack

The Lone documentation site is hosted under the `docs/` directory and is built using **[VitePress](https://vitepress.dev/)**, a modern, Vue 3-powered static site generator known for high performance, local instant search, and markdown-centered authoring.

Key components of the documentation stack:
- **VitePress v1.0.0-alpha.28**: Renders Markdown files into a static single-page application.
- **Dynamic Versioning & Sidebar Generator (`scripts/gen-sidebar.js`)**: Automatically discovers semantic version directories, designates the highest version as `latest`, syncs directory trees, and generates navigation menus and sidebars.
- **Local Search Engine**: Client-side full-text index without external dependencies.
- **Syntax Highlighting**: Supports `lone`, `wat`, `bash`, `yaml`, `json`, `c`, and `go` code blocks.

> [!IMPORTANT]
> **Mandatory Content Standard**: Before writing or auditing documentation for any version, ensure the content adheres to the 4-Pillar Standard defined in [`WRITING_DOC_REQUIREMENT.md`](./WRITING_DOC_REQUIREMENT.md) (Getting Started, Language Reference, Advanced & Quirks, Debugging with `lone debug`).

---

## 2. Directory Architecture

```
docs/
├── README.md                      # This authoring & maintenance guide
├── WRITING_DOC_REQUIREMENT.md     # Mandatory 4-Pillar documentation requirements specification
├── package.json                   # VitePress dependencies and lifecycle scripts
├── bun.lock                       # Lockfile
├── scripts/
│   └── gen-sidebar.js             # Versioning and dynamic sidebar generator
└── docs/                          # VitePress project root
    ├── index.md                   # Home landing page (hero, feature grid, CTA)
    ├── public/                    # Static assets (logo.svg, images, favicon)
    ├── .vitepress/
    │   ├── config.ts              # VitePress configuration (nav, theme, search)
    │   ├── sidebar.generated.json # Auto-generated sidebar & version nav
    │   └── dist/                  # Static production build output (gitignored)
    └── content/                   # Versioned documentation content
        ├── 0.1.0/                 # Historical v0.1.0 release docs
        ├── 0.2.0/                 # Historical v0.2.0 release docs
        ├── 0.4.0/                 # Historical v0.4.0 release docs
        ├── 0.5.0-beta/            # Historical v0.5.0-beta release docs
        ├── 0.6.0/                 # Current active version docs
        └── latest/                # Mirrored copy of highest semver (DO NOT EDIT DIRECTLY)
```

---

## 3. Dynamic Versioning & Sidebar Generator

The documentation uses an automated versioning engine located at [`docs/scripts/gen-sidebar.js`](./scripts/gen-sidebar.js). It runs automatically before `npm run dev` and `npm run build` via `predev` and `prebuild` npm lifecycle hooks.

### 3.1 Version Discovery & Ordering
1. The script recursively scans `docs/docs/content/` for directories with valid semantic version names (`<major>.<minor>.<patch>[-<tag>]`).
2. Directory `latest/` is explicitly excluded from version discovery.
3. Versions are parsed and sorted in **descending order** (e.g. `0.6.0` > `0.5.0-beta` > `0.4.0` > `0.2.0` > `0.1.0`).
4. The highest semantic version is designated as `latest`.

### 3.2 Automated `latest/` Synchronization
```
docs/docs/content/0.6.0/  ──────(auto-sync via gen-sidebar.js)──────>  docs/docs/content/latest/
```

> [!WARNING]
> **NEVER EDIT `docs/docs/content/latest/` DIRECTLY!**  
> Any changes made directly in `content/latest/` will be **silently overwritten** whenever `npm run dev`, `npm run build`, or `node scripts/gen-sidebar.js` is executed.  
> **Always make edits in the corresponding semantic version directory** (e.g. `docs/docs/content/0.6.0/`).

### 3.3 Section Titles and Ordering
Files are grouped into sidebar sections based on their parent subfolder inside the version directory:

| Subfolder | Sidebar Section Label | Purpose |
| :--- | :--- | :--- |
| `guide/` | **Getting Started & Guides** | Installation, quickstart, tutorials, concepts. |
| `language/` | **Language Reference** | Types, variables, functions, control flow, traits, memory. |
| `toolchain/` | **Compiler & Toolchain** | CLI reference, debugger, WASI/Wasm runtime, adapters. |
| `falone/` | **Falone Compiler & Adapters** | Target lowerings and host adapters. |
| `runtime/` | **WebAssembly Runtime** | Memory layout, shadow stack, syscalls. |
| `spec/` | **Language Specification** | Formal specification mirrors. |

The sidebar sections are sorted according to `SECTION_ORDER`:
```javascript
const SECTION_ORDER = ['guide', 'language', 'toolchain', 'falone', 'runtime', 'spec'];
```

---

## 4. Document Authoring Standard

Every documentation page is a Markdown (`.md`) file located within a section folder under a version directory (e.g. `docs/docs/content/0.6.0/language/variables-and-types.md`).

### 4.1 Required YAML Frontmatter
Every documentation page should begin with YAML frontmatter:

```yaml
---
title: "Formatted String Interpolation"
order: 15
---
```

- `title`: Human-readable document title surfaced in the sidebar, navigation, and browser tab.
- `order`: Non-negative integer determining page placement within its sidebar section (lower numbers appear first).

#### Fallback Rules
If frontmatter is omitted:
1. The script searches for the first `# Heading 1` in the document.
2. If no `# Heading 1` exists, the script derives the title from the kebab-case filename (e.g. `control-flow.md` $\rightarrow$ `Control Flow`).
3. Documents without an `order` field receive a default order of `999` and are sorted alphabetically at the end of the section.

### 4.2 Heading Hierarchy
Follow strict heading levels to ensure the right-hand table of contents (TOC) renders properly:
- `# Document Title`: Exactly one top-level heading per file.
- `## Major Section`: Main topics and conceptual boundaries.
- `### Subsection`: Detailed mechanics, rules, and sub-topics.
- `#### Minor Heading`: Optional field lists or specific code breakdowns.
- **Do not skip levels** (e.g. do not jump directly from `#` to `###`).

### 4.3 Lone Code Block Conventions
All Lone source code examples must use the fenced `lone` language tag:

````markdown
```lone
-- Import standard utilities
import .println from :std:io

pub type User = struct {
    ro(pub) name: str
    pub age: i32

    pub fn __init__(name: str, age: i32) User {
        return User { name: name, age: age }
    }
}

pub fn main() void {
    let u = User::("Alice", 30)
    println(f"User: ${u.name}, Age: ${u.age}")
}
```
````

#### Rules for Code Samples:
1. **Accurate Syntax**: Always use the active syntax for that version (e.g. for v0.6.0+: `ro(pub)` for asymmetric fields, `interface` data contracts, canonical `__init__` constructors, `must`/`try` error handling).
2. **Comments**: Use `--` double dashes for single-line comments.
3. **Completeness**: Provide runnable or self-explanatory snippets rather than ambiguous pseudo-code.
4. **Formatting**: Use 4-space indentation for blocks, structs, and functions.

### 4.4 VitePress Containers & Alerts
Use VitePress custom containers to emphasize important notes, tips, warnings, and caveats:

```markdown
::: info
This feature is available starting in Lone v0.6.0.
:::

::: tip Best Practice
Prefer using `f"..."` string interpolation over manual string concatenation.
:::

::: warning Breaking Change
Return-type-only overloading is strictly prohibited and triggers a compile-time error.
:::

::: danger Memory Safety
Direct raw pointer operations (`ptr<T>`) bypass compiler safety guarantees.
:::
```

### 4.5 Math & Formulas
Inline math can be written with LaTeX expressions:
```markdown
$$\text{Offset}(field_i) = \sum_{k=0}^{i-1} \text{Size}(field_k)$$
```

---

## 5. Step-by-Step Authoring Workflows

### Workflow A: Editing an Existing Page
1. Locate the file in the **active semantic version directory** (e.g. `docs/docs/content/0.6.0/language/functions.md`).
2. Make your edits and save.
3. Run the development server to verify changes in real time:
   ```bash
   cd docs
   npm run dev
   ```
4. Open `http://localhost:5173` in your browser. Changes hot-reload instantly.

---

### Workflow B: Adding a New Page to the Active Version
1. Identify the appropriate section directory (e.g. `docs/docs/content/0.6.0/language/`).
2. Create the Markdown file with a descriptive kebab-case filename:
   ```bash
   touch docs/docs/content/0.6.0/language/pattern-matching.md
   ```
3. Add the YAML frontmatter with `title` and appropriate `order`:
   ```markdown
   ---
   title: "Pattern Matching & Exhaustiveness"
   order: 8
   ---

   # Pattern Matching & Exhaustiveness
   ...
   ```
4. Verify sidebar placement:
   ```bash
   cd docs
   npm run gen-sidebar
   ```
   Check that `docs/docs/.vitepress/sidebar.generated.json` includes the new file in the correct order.

---

### Workflow C: Creating a New Version Documentation Suite (e.g. `v0.7.0`)
When a new minor or major release branch is prepared:

1. **Branch the Version Directory**:
   Copy the previous active version as the starting template:
   ```bash
   cp -r docs/docs/content/0.6.0 docs/docs/content/0.7.0
   ```
2. **Update New Version Pages**:
   - Update `guide/introduction.md` and `guide/getting-started.md` to reference `v0.7.0`.
   - Add documentation for newly accepted RFCs (e.g. `@fa:docs`, `@fa:impl_label`, `@fa:fn_overload`).
   - Remove or mark deprecated syntax according to release changes.
3. **Run Sidebar Generator**:
   ```bash
   cd docs
   npm run gen-sidebar
   ```
   The generator will:
   - Detect `0.7.0` as the highest semver.
   - Designate `0.7.0` as `latest`.
   - Synchronize `content/0.7.0/` to `content/latest/`.
   - Add `v0.7.0` to the version switcher dropdown in the navbar.
4. **Verify Full Static Build**:
   ```bash
   npm run build
   ```

---

### Workflow D: Translating Accepted RFCs to Documentation
When an RFC moves to `dev/rfcs/accepted/`:
- **RFC Focus**: Explains *why* the language feature was designed, its grammar rules, compiler lowering, and memory impact.
- **Docs Focus**: Explains *how* an application developer uses the feature with concise explanations, practical code examples, and best practices.

Conversion guide:
1. Extract the syntax rules and code examples from the RFC's **Detailed Specification** section.
2. Formulate user-facing guides in the appropriate section (`language/`, `toolchain/`, or `guide/`).
3. Include VitePress callouts for caveats, restrictions, and compiler diagnostic errors.
4. Link to related concepts across the language reference.

---

## 6. CLI Commands & Development Scripts

Run all documentation commands from within the `docs/` directory:

| Command | Description |
| :--- | :--- |
| `npm run dev` | Runs `gen-sidebar.js` and starts the VitePress local dev server at `http://localhost:5173`. Supports instant hot-reloading. |
| `npm run build` | Runs `gen-sidebar.js` and compiles the documentation into static HTML under `docs/docs/.vitepress/dist/`. |
| `npm run serve` | Previews the static production build locally at `http://localhost:4173`. |
| `npm run gen-sidebar` | Executes `scripts/gen-sidebar.js` standalone to regenerate the sidebar and sync `latest/`. |

*(Note: `bun run <command>` can also be used interchangeably).*

---

## 7. Pre-Commit Quality Checklist

Before submitting a pull request or committing changes to `docs/`:

- [ ] **Content Standard Compliance**: Documentation covers all 4 pillars mandated in [`WRITING_DOC_REQUIREMENT.md`](./WRITING_DOC_REQUIREMENT.md) (Getting Started, Language Reference, Advanced Quirks, `lone debug`).
- [ ] **Single Source of Truth**: All edits are made in `docs/docs/content/<sem-ver>/`, **NEVER** in `docs/docs/content/latest/`.
- [ ] **Frontmatter Present**: All added or updated pages include `title` and `order`.
- [ ] **Code Syntax Accuracy**: All ` ```lone ```` code snippets conform to the active version's grammar and invariants.
- [ ] **Zero Dead Links**: All internal links use clean URL paths without `.md` extensions (e.g. `/content/latest/language/functions`).
- [ ] **Build Verification**: `cd docs && npm run build` completes with exit code 0.
- [ ] **License Consistency**: The Apache 2.0 license notice is preserved in the footer.
