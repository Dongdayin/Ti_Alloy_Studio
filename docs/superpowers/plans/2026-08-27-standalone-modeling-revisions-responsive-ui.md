# Standalone Modeling Revisions and Responsive UI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn Ti Alloy Studio into a self-contained Windows structure-modeling application with immutable editable revisions, repeatable revision-aware export, a compact capability view, and a responsive interface that remains usable on narrow windows.

**Architecture:** Keep GUI, HTTP application services, modeling domain logic, persistence, and optional external connectors separated. Store every successful build as an immutable revision containing the exact structure snapshot and its scientific evidence; project import/export operates transactionally on those revisions. The default interface exposes only bundled modeling capabilities, while legacy external connectors are collapsed and probed only on demand.

**Tech Stack:** Go 1.23, standard-library HTTP/JSON/ZIP, embedded HTML/CSS/JavaScript, existing native modeling core, bundled Python/ASE/spglib/pymatgen/AtomMan and Atomsk, GitHub Actions Windows packaging.

**Spec:** `docs/superpowers/specs/2026-08-27-standalone-modeling-revisions-responsive-ui-design.md`

## Global Constraints

- Preserve scientific state labels: generated structures are not relaxed, converged, stable, or energy-validated unless independent evidence exists.
- Do not depend on WSL, system Python, PATH, LAMMPS, GPUMD, NEP, VASP, or ATAT for the normal modeling workflow.
- Do not bundle proprietary or unnecessary production solvers.
- Never mutate an existing revision or overwrite the user's original project data.
- A failed build, derivation, import, or export must leave active revision and history unchanged.
- Implement each behavior test-first and commit coherent passing slices.
- Do not replace the previously accepted installer until the redesigned release passes clean-machine installation and workflow acceptance.

---

## Task 1: Immutable Revision Domain and Exact Structure Snapshots

**Files:**
- Modify: `internal/app/provenance.go`
- Modify: `internal/app/user_build.go`
- Modify: `internal/app/app.go`
- Test: `internal/app/provenance_test.go`
- Add test: `internal/app/revision_test.go`

- [ ] Add failing tests proving each successful build stores an exact structure snapshot, explicit parent ID, normalized request, hashes, validation, engine evidence, scientific state, and creation time.
- [ ] Add failing tests proving revision selection is read-only and a failed child build leaves active revision/history unchanged.
- [ ] Add failing tests proving `EditRevision` restores a selected revision's request and creates a new child only after a successful rebuild.
- [ ] Add failing tests proving `DeriveRevision` applies supported vacancy/substitution operations to the selected snapshot rather than rebuilding a default host.
- [ ] Introduce a versioned `Revision` model and a project state containing ordered immutable revisions plus `ActiveRevisionID`.
- [ ] Implement deep-copy boundaries for structures, request metadata, validation, engine results, and allocation evidence so callers cannot alias stored state.
- [ ] Implement select, edit, and derive application services with explicit parent IDs and transactional state updates.
- [ ] Run `go test ./internal/app -run 'Revision|Provenance|Tracked'` and commit the passing slice.

## Task 2: Transactional Project Archive Schema v2 and Schema v1 Migration

**Files:**
- Modify: `internal/app/provenance.go`
- Add: `internal/app/project_archive.go`
- Test: `internal/app/provenance_test.go`
- Add test: `internal/app/project_archive_test.go`

- [ ] Add failing round-trip tests for `.tias-project` ZIP layout containing `manifest.json`, `revisions/<id>/record.json`, and `revisions/<id>/structure.json`.
- [ ] Add failing tests for hash mismatch, missing member, duplicate revision ID, invalid parent, invalid active revision, path traversal, and truncated ZIP; every failure must preserve current project state.
- [ ] Add a schema v1 migration fixture and failing test proving requests are rebuilt once, recorded hashes are verified, parent order is retained, and import does not append a duplicate latest build.
- [ ] Implement deterministic schema v2 serialization, per-member SHA-256 verification, bounded ZIP reading, safe member names, and atomic in-memory replacement.
- [ ] Implement schema v1 migration with precise errors when historical snapshots cannot be reproduced.
- [ ] Run `go test ./internal/app -run 'Project|Archive|Import|Migration'` and commit the passing slice.

## Task 3: Revision-Aware HTTP APIs and Repeatable Export

**Files:**
- Modify: `internal/httpapi/httpapi.go`
- Modify: `internal/app/app.go`
- Test: `internal/httpapi/interface_api_test.go`
- Add test: `internal/httpapi/revision_api_test.go`

- [ ] Add failing API tests for revision list/detail, select, edit, derive, project save/open, and invalid revision IDs.
- [ ] Add failing tests proving export accepts an explicit revision ID and exports the selected immutable snapshot repeatedly without changing project state.
- [ ] Add failing tests for build/derive errors returning understandable typed JSON errors while retaining prior active revision.
- [ ] Add routes `/api/project/revision`, `/api/project/select`, `/api/project/edit`, `/api/project/derive`, and revision-aware `/api/export`.
- [ ] Return a single consistent project/revision view model after every successful mutation.
- [ ] Keep existing endpoints compatible where practical by defaulting omitted revision ID to the active revision.
- [ ] Run `go test ./internal/httpapi` and commit the passing slice.

## Task 4: Bundled Modeling Capability Catalog and On-Demand Legacy Connectors

**Files:**
- Modify: `internal/engines/environment.go`
- Modify: `internal/engines/managed_packages.go`
- Modify: `internal/httpapi/httpapi.go`
- Test: `internal/engines/environment_test.go`
- Test: `internal/httpapi/interface_api_test.go`

- [ ] Add failing tests for capability categories `built_in`, `export_format`, and `external_connector` with stable `AVAILABLE`, `SUPPORTED`, and `NOT_CONFIGURED` states.
- [ ] Add failing tests proving the default capability request does not start WSL or search local solver paths.
- [ ] Add failing tests proving an explicit connector probe remains available for diagnostics and clearly identifies results as optional/non-required.
- [ ] Implement `/api/capabilities` from bundled application facts and `/api/connectors?probe=true` for legacy diagnostics.
- [ ] Remove automatic WSL probing from startup/refresh paths while retaining connector code behind the explicit endpoint.
- [ ] Ensure the catalog lists bundled Python packages and Atomsk without duplicating a misleading WSL Atomsk failure.
- [ ] Run `go test ./internal/engines ./internal/httpapi -run 'Capability|Environment|Connector'` and commit the passing slice.

## Task 5: Responsive Modeling, Revision, Validation, and Export Interface

**Files:**
- Modify: `internal/webapp/static/index.html`
- Modify: `internal/webapp/static/style.css`
- Modify: `internal/webapp/static/app.js`
- Modify: `internal/webapp/static/project.js`
- Test: `internal/webapp/webapp_test.go`

- [ ] Add failing static/behavior tests requiring no fixed 1024 px body width, keyboard-accessible narrow-screen tabs, visible build controls, and an explicit active revision label beside export.
- [ ] Add failing tests for revision history controls: select/view, edit recipe as new revision, derive from selected structure, and repeated exports.
- [ ] Replace the current narrow overflowing layout with a three-column desktop grid at 1100 px and four accessible panels below 1100 px: Model, Structure, Validation, Export.
- [ ] Keep build controls in normal document flow; prevent fixed export panels from covering them.
- [ ] Render compact capability cards for bundled modeling functions and export formats; place optional connectors in a collapsed advanced section with an explicit probe action.
- [ ] Add revision cards showing ID, parent, time, module, composition/atom count, scientific state, and active status.
- [ ] Wire select/edit/derive/export actions to the revision APIs, display transactional errors, and preserve the currently usable model on failure.
- [ ] Run `go test ./internal/webapp ./internal/httpapi` and commit the passing slice.

## Task 6: Bounded Internal SQS Correlation Evidence

**Files:**
- Modify: `internal/model/sqs.go`
- Modify: `internal/model/models.go`
- Modify: `internal/app/user_build.go`
- Test: `internal/model/phase1_acceptance_test.go`
- Test: `internal/app/sqs_backend_test.go`
- Add: `internal/model/testdata/atat/README.md`
- Add fixture files under: `internal/model/testdata/atat/`

- [ ] Add failing deterministic tests for pair and triplet cluster enumeration, target random-alloy correlations, achieved correlations, residuals, seed replay, and bounded termination.
- [ ] Add failing tests proving ordinary native results are labeled `pair_triplet_correlation_sqs` and never `verified_sqs` without an explicit committed validation criterion.
- [ ] Add small redistribution-safe ATAT reference fixtures containing inputs, selected correlations, provenance, and expected comparison bounds.
- [ ] Add failing cross-validation tests against those fixtures and document exactly what is compared and what is not claimed.
- [ ] Extend the internal optimizer and report model while preserving current deterministic builds and scientific state semantics.
- [ ] Expose correlation evidence in revision records and the validation UI.
- [ ] Run `go test ./internal/model ./internal/app -run 'SQS|Correlation|ATAT'` and commit the passing slice.

## Task 7: Documentation, Versioning, and Offline Windows Release Acceptance

**Files:**
- Modify: `README.md`
- Modify: `docs/LESSONS_LEARNED.md`
- Modify: `internal/installer/payload/README.txt`
- Modify: `internal/installer/payload/THIRD_PARTY_NOTICES.txt` when component facts change
- Modify: `.github/workflows/build-r3-offline.yml`
- Modify: `.github/workflows/phase1-go-test.yml`
- Modify version declarations found by `rg '0\.1\.8|phase1-r7'`
- Test: `internal/installer/installer_test.go`

- [ ] Update user documentation for the modeling-only scope, revision workflow, repeatable export, project schema/migration, capability statuses, and precise scientific limitations.
- [ ] Record confirmed root causes and regression rules for misleading WSL capability status, fixed-width UI overflow, and state-less historical export.
- [ ] Add release tests asserting all runtime executables and Python packages are installer-managed and no normal-path WSL/system dependency is required.
- [ ] Extend Windows CI acceptance to install in a clean path containing spaces, launch with no WSL/external solvers, build a model, export revision A, edit into revision B, re-export both, save/reopen, verify hashes, and uninstall cleanly.
- [ ] Bump the application/installer version consistently and retain the previously accepted installer until this workflow is green.
- [ ] Run fresh `go test ./...` and `go vet ./...` locally.
- [ ] Push the release commit, wait for the Windows workflow, download the produced artifact, verify its SHA-256, and report the run ID and exact acceptance results.
- [ ] Commit final documentation/release-gate changes only after the source suite passes.

## Final Verification Checklist

- [ ] Search the plan and implementation for `TODO`, `TBD`, placeholder handlers, swallowed errors, and fake success states.
- [ ] Verify every design-spec requirement is represented by a test or an explicit documented limitation.
- [ ] Verify API JSON types, project schema types, UI field names, and hash algorithms agree end to end.
- [ ] Verify a failed operation cannot mutate active revision, history, original project archive, or exported files.
- [ ] Verify desktop and narrow layouts visually at representative widths, including the user's narrow-window scenario.
- [ ] Use `superpowers:verification-before-completion` before making any completion claim.
