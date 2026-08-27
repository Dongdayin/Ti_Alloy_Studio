# Ti Alloy Studio standalone modeling, revisions, and responsive UI design

Date: 2026-08-27  
Branch: `r3-offline`  
Status: approved in chat; written specification awaiting final user review

## 1. Purpose

Ti Alloy Studio will be a self-contained Windows modeling application. A user must be able to install one offline package on a Windows 10 or Windows 11 x64 computer and complete the modeling workflow without WSL, a system Python, ATAT, LAMMPS, GPUMD, VASP, or other preinstalled scientific software.

The complete workflow is:

```text
install -> create model -> validate -> inspect -> save revision -> export
        -> select any revision -> edit as a child revision -> validate -> export again
        -> save project -> close -> reopen -> restore exact selected revision
```

This phase is a model builder and exporter. It does not run VASP, LAMMPS, GPUMD, NEP training, or production molecular-dynamics or electronic-structure calculations.

## 2. Confirmed defects

### 2.1 Misleading environment report

The current environment endpoint combines required bundled engines with optional WSL and solver probes. Optional tools are rendered as prominent `UNAVAILABLE` cards even though the corresponding export formats do not require those tools.

The WSL probe also runs non-interactive `bash -lc`. On the verified Ubuntu-24.04 machine, scientific tool paths are configured by the interactive shell, so the UI reports false negatives for tools that are present. This is a detector defect, but the primary product defect is that a standalone modeler presents optional external integrations as required capabilities.

### 2.2 Revision history cannot restore an arbitrary model

`BuildRecord` stores a normalized request and structure/export hashes, but not the exact structure or full response. Parent assignment always chooses the most recent record. Project import rebuilds only the latest request and appends another record. Therefore the project cannot select an arbitrary prior revision, fork from it, or guarantee exact restoration after an algorithm changes.

### 2.3 Narrow windows hide the modeling controls

The current stylesheet forces a minimum body width of 1024 px and hides overflow. In a narrow Edge application window, the modeling sidebar can be outside the reachable viewport. Export remains visible while the controls needed to change the model are inaccessible.

### 2.4 Export itself is not the lock

The export endpoint reads the current structure without mutating it, and browser downloads use object URLs. Repeated export is already supported by the backend. The observed inability to modify after an export is caused by inaccessible controls and insufficient revision-selection/editing behavior, not by the file conversion functions.

## 3. Goals and non-goals

### Goals

1. Make every Phase 1 modeling and export path work without WSL or external executables.
2. Present bundled capabilities, export formats, and optional external connectors as separate concepts.
3. Allow any saved revision to be selected, restored exactly, edited as a child, validated, and exported repeatedly.
4. Preserve old revisions; never silently overwrite or reinterpret them.
5. Make the complete workflow usable in wide and narrow windows.
6. Preserve scientific semantics: generated structures remain `not_relaxed` and `not_calculated` unless solver evidence exists.
7. Provide an offline SQS path with explicit correlation evidence and no false claim of ATAT equivalence.
8. Publish one offline Windows x64 installer and verify it on a clean machine without WSL and without scientific software on `PATH`.

### Non-goals

1. Running VASP, LAMMPS, GPUMD, or NEP calculations.
2. Bundling proprietary VASP binaries or potentials.
3. Automatically choosing force fields, pseudopotentials, convergence thresholds, or relaxation settings.
4. General freehand atom-coordinate editing in the first delivery of this redesign.
5. Claiming energy, equilibrium, convergence, stability, or relaxed geometry from an unrelaxed generated structure.
6. Claiming the existing pair-statistics SQS engine is equivalent to the complete ATAT cluster basis.

## 4. Architecture

The application keeps the existing layered direction and adds two explicit domain services.

```text
responsive web UI
    |
HTTP application API
    |-- Revision service ------ project repository / project archive
    |-- Modeling service ------ TiModelCore structure builders
    |-- Capability catalog ---- bundled engine verification
    `-- Export service -------- POSCAR / XYZ / extXYZ / LAMMPS data / CIF
```

External solver discovery is not part of application startup or the core modeling request. A legacy external-connector probe may remain behind an explicit advanced action, but its result cannot disable modeling or export.

### 4.1 Capability catalog

The environment report is replaced in the normal UI by a capability catalog with three categories:

- `built_in`: native model builders, private Python, ASE, spglib, pymatgen, AtomMan, NumPy, SciPy, and private Atomsk.
- `export_format`: VASP POSCAR, XYZ, GPUMD-compatible extXYZ, LAMMPS atomic data, CIF, EOS ZIP, and GSFE ZIP.
- `external_connector`: optional compatibility probes, hidden in a collapsed advanced panel and never required.

Each capability has an identifier, category, status, version, path when useful, and a plain-language message. Required built-in capabilities use `READY` or `FAILED`. Optional connectors use `NOT_CONFIGURED`, not `UNAVAILABLE`.

The normal capability request must not start WSL. The advanced probe is on demand, bounded, and clearly labeled as diagnostic only.

### 4.2 Revision service

The revision service owns the active revision and project lineage. A revision contains:

- immutable revision ID;
- optional parent revision ID selected by the user;
- creation timestamp and application version;
- normalized `BuildRequest`;
- exact `model.Structure` snapshot;
- response analysis, series, allocation, SQS evidence, validation, and engine cross-checks;
- structure and export SHA-256 values;
- scientific state (`not_relaxed`, `not_calculated`);
- display summary: module, phase, atom count, composition, and validation status.

Creating a model appends a root or child revision. Selecting a revision changes the active read-only snapshot. `Edit recipe as new revision` restores its request into the controls, records the selected revision as the pending parent, and creates a new child only after a successful build. `Derive from this structure` applies a supported operation to the exact selected snapshot instead of rebuilding a default host. The first delivery supports vacancy and species substitution as derived operations, using the selected atom ID and an explicit new species where required. Export never appends, selects, deletes, or mutates a revision.

Failed builds do not alter the active revision or history.

### 4.3 Project persistence

Project schema version 2 stores exact revision snapshots. The user-facing project archive is a ZIP-based `.tias-project` file containing:

```text
manifest.json
revisions/<revision-id>/record.json
revisions/<revision-id>/structure.json
```

Every structure file is checked against the SHA-256 in its record during import. Import is transactional: parsing, schema validation, path validation, hash verification, and reconstruction complete in temporary state before replacing the active project.

Schema version 1 `project.json` remains importable. Because it has no snapshots, each request is rebuilt in order and checked against its recorded structure hash. A mismatch fails import with the affected revision ID; it is not silently accepted. A successful schema-1 migration creates schema-2 snapshots without adding a duplicate build.

The project summary endpoint returns lightweight revision summaries. A separate revision endpoint returns one selected record and structure, avoiding full-project payloads during ordinary UI refreshes.

## 5. Modeling and SQS semantics

All existing crystal, random alloy, defect, surface, alpha/beta interface, EOS-series, and GSFE-series builders remain local.

The first implementation supports two explicit edit paths:

1. `Edit recipe as new revision` restores and changes normalized generator parameters.
2. `Derive from this structure` applies vacancy or species substitution directly to the selected immutable structure and records the selected revision as its parent.

Surface, interface, EOS, and GSFE remain explicit modeling generators. General freehand coordinate dragging and an unrestricted operation stack are deferred; the interface must not imply they exist.

### 5.1 Bundled SQS path

The current native pair-statistics annealer remains available and is labeled `pair-correlation SQS`, with its selected shell scope, objective, errors, seed, and convergence history visible.

This redesign adds a bounded internal cluster-correlation layer for the requested binary and ternary alloy cases. It must:

1. enumerate selected periodic pair and triplet clusters from explicit user cutoffs;
2. compute the random-alloy target and candidate correlation vectors;
3. optimize species assignments with a deterministic seed while conserving integer composition;
4. report every selected cluster, multiplicity, target, observed value, error, RMS error, and maximum absolute error;
5. preserve the best structure and convergence history;
6. cross-validate cluster enumeration and correlation values against committed, provenance-labeled ATAT reference fixtures for small binary and Ti-Al-V cases.

The result may be labeled `correlation_evidence_calculated`. It may be labeled `verified_sqs` only when the selected runtime acceptance criterion is explicit, all required cluster evidence is present, and the criterion is met. No universal cutoff or acceptance tolerance is invented. Until ATAT reference cross-validation passes, the UI must retain the narrower `pair-correlation SQS` label.

The legacy WSL ATAT adapter remains an optional comparison route and does not participate in standalone release acceptance.

## 6. API and data flow

The API is extended without breaking the existing build and export routes.

- `POST /api/build`: accepts a build request plus optional `parent_revision_id`; successful response includes the new revision ID.
- `GET /api/project`: returns project identity, active revision ID, and lightweight revision summaries.
- `GET /api/project/revision?id=...`: returns the exact selected revision record and structure.
- `POST /api/project/select`: selects an existing revision without creating history.
- `POST /api/project/edit`: selects a revision as the pending parent and returns its normalized request for control restoration.
- `POST /api/project/derive`: accepts a source revision ID plus a validated vacancy or substitution operation and creates a child from the exact source snapshot.
- `GET /api/project/export`: exports the schema-2 project archive.
- `POST /api/project/import`: accepts schema-2 archives and legacy schema-1 JSON with transactional validation.
- `GET /api/capabilities`: returns built-in and export-format capabilities without probing WSL.
- `GET /api/connectors?probe=true`: performs the optional external diagnostic only after an explicit advanced action.

All mutating routes require POST, reject unknown fields, apply size limits, and return clear errors. Export routes accept an optional revision ID; omission means the active revision.

## 7. Responsive interface

### 7.1 Wide layout

At viewport widths of 1100 px and above:

- left: modeling modules, parameters, and revision history;
- center: three-dimensional structure, selection, charts, and tables;
- right: model summary, validation, built-in capabilities, and export.

### 7.2 Narrow layout

Below 1100 px, the three columns become four reachable workspace tabs:

1. `Model` — module navigation and parameters;
2. `Structure` — viewer and plots;
3. `Validation` — model information, checks, and built-in capabilities;
4. `Export` — active revision summary and export buttons.

The body has no fixed minimum width. The active page scrolls vertically; controls are never hidden by `overflow: hidden`. A persistent compact bar shows project name, active revision number, dirty/editing state, and validation status.

### 7.3 History interaction

The history list displays revision number, module, phase, atom count, timestamp, and validation state. Selecting a card previews that exact immutable revision. `Edit as new revision` restores its controls and identifies it as the parent. `Build new revision` is the only action that adds history.

The export panel states `Exporting revision #N` and permits repeated POSCAR, XYZ, extXYZ, LAMMPS data, and CIF downloads. Export completion does not navigate away, reset controls, or change history.

Common parameters are visible. Scientific or infrequently changed parameters remain in labeled advanced sections. Error messages state the failed step, invalid field or revision, and corrective action.

## 8. Error handling and safety

- A failed build leaves the previous active revision unchanged.
- A failed project import leaves the existing project unchanged and reports the schema, revision, file, or hash error.
- Project archive extraction rejects absolute paths, drive-qualified paths, and `..` traversal.
- Export uses the selected immutable snapshot and verifies the stored structure hash before serialization.
- Optional connector failures never disable built-in modeling or export.
- Missing private engines are installation failures with an installer diagnostic, not normal UI warnings.
- Original project files and old revisions are never overwritten.
- The application does not delete or modify a system Python, WSL distribution, solver installation, potential library, or licensed executable.

## 9. Packaging

The single offline installer contains:

- Ti Alloy Studio native executable and uninstaller;
- private CPython runtime;
- pinned private Python scientific packages;
- private Atomsk executable;
- built-in modeling and SQS code;
- manual, license notices, and project migration documentation.

It does not contain VASP, LAMMPS, GPUMD, NEP, ATAT, WSL, solver potentials, pseudopotentials, or third-party licensed research data. These are not required for modeling or export.

The application version and installer version use one release constant rather than separate stale strings.

## 10. Testing and acceptance

### Unit tests

- revision selection, explicit parent lineage, immutable snapshots, vacancy/substitution derivation from a selected snapshot, and failed-build rollback;
- schema-2 archive round-trip and exact structure/hash equality;
- schema-1 migration without duplicate history;
- archive path traversal, malformed JSON, missing snapshots, mismatched hashes, and size limits;
- export of active and explicitly selected revisions without state mutation;
- capability categories and absence of automatic WSL probing;
- responsive UI markers, accessible navigation, and repeated download behavior;
- pair and triplet cluster enumeration, correlation vectors, composition conservation, deterministic seeds, and quality evidence.

### Integration tests

- build revision A, export it twice, edit A, build child B, export B, reselect A, and prove A exports byte-identically;
- create a branch from a non-latest revision and verify the explicit parent ID;
- derive a vacancy and substitution from a non-latest revision and prove the untouched source snapshot remains byte-identical;
- save a project, import into a new state, select every revision, and prove exact structure/hash equality;
- import a legacy project and verify migration without an extra build;
- run the complete API flow at wide and narrow viewport dimensions.

### Scientific tests

- integer species counts conserve lattice sites;
- SQS correlation evidence is reproducible for a fixed seed;
- bundled cluster correlations match committed ATAT reference fixtures within declared numerical tolerances before any `verified_sqs` label is enabled;
- generated structures remain `not_relaxed` and `not_calculated`;
- every exported structure parses back and is physically equivalent to its selected revision.

### Windows release gate

On a clean Windows x64 runner with no WSL distribution and no external scientific executable on `PATH`:

1. install from one offline package into a path containing spaces;
2. verify all bundled engines and capability catalog entries;
3. create and validate a random alloy and bundled SQS model;
4. export revision A twice;
5. edit A and create child revision B;
6. export B and re-export A;
7. save, close, reopen, import, and verify exact revision restoration;
8. exercise narrow and wide UI flows;
9. uninstall and verify removal of application files and registration;
10. record SHA-256 values for installer, application, private engine bundle, manual, and acceptance report.

No release is accepted solely because compilation or packaging succeeded.

## 11. Delivery order

1. Revision domain and schema-2 persistence.
2. Revision-aware build, select, edit, and export APIs.
3. Responsive UI and history workflow.
4. Capability catalog and removal of mandatory external-tool scanning.
5. Bundled SQS correlation evidence and ATAT reference validation.
6. Documentation, migration guidance, installer rebuild, clean-machine acceptance, and local installation verification.

Each step is independently tested. The installer is replaced only after all release gates pass.
