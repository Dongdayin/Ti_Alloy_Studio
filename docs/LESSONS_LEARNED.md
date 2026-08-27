# Lessons learned

## 2026-08-27 — Do not use a registered CPython installer for an app-local runtime

- Trigger: CPython 3.11.9 was already installed for the current Windows user.
- Symptom: the official CPython installer returned exit code 0 for a different `TargetDir`, but created no private runtime there; Ti Alloy Studio then failed because `engines\python\python.exe` was absent.
- Root cause: the CPython bootstrapper entered maintenance mode for the registered same-version installation instead of deploying a second application-local copy. The clean CI runner did not contain this state.
- Fix: build the science environment into the official CPython embeddable distribution during release construction. The end-user installer now only extracts the verified private runtime and Atomsk.
- Regression gates: unit tests verify real nested extraction and reject path traversal; Windows release CI first installs the same CPython version for the current user, then proves Ti Alloy Studio installs, passes engine smoke tests, and leaves that existing Python byte-identical.
- Prevention: an application-private runtime must be a relocatable release artifact. A successful third-party installer exit code is never evidence that the requested target files were deployed.

## 2026-08-27 — Do not present optional connector discovery as application readiness

- Trigger: the environment panel automatically probed a selected WSL distribution and listed Atomsk, ATAT, GPUMD, LAMMPS, NEP, Python and VASP commands.
- Symptom: optional tools appeared as `UNAVAILABLE` even though the bundled Windows Python and Atomsk engines were available and the modeling workflow was complete. Non-interactive WSL shell startup could also miss paths configured by an interactive shell.
- Root cause: bundled modeling capabilities and legacy external connector discovery shared one status list and one automatic refresh path.
- Fix: the default capability catalog now contains only built-in modeling functions, installed private engines and export formats. External connectors are collapsed, `NOT_CONFIGURED`, non-required, and probed only after an explicit user action.
- Regression gate: API and web tests verify that `/api/capabilities` contains no WSL probe report and that the interface does not call the legacy environment probe at startup.

## 2026-08-27 — Export history requires immutable structure snapshots

- Trigger: a user exported one model, changed parameters, and needed to export both the previous and edited structures or continue on another computer.
- Symptom: project history contained recipes and hashes but no exact structures; import rebuilt only the latest recipe and appended a duplicate record. Export always used a single mutable current structure.
- Root cause: history was treated as an audit list rather than a selectable revision graph.
- Fix: every successful build now stores an exact immutable structure and explicit parent. Export accepts a revision ID. `.tias-project` schema 2 stores each record and structure separately with SHA-256 verification and replaces state only after complete validation.
- Regression gate: tests cover selection without mutation, branch lineage, derivation from an exact snapshot, historical re-export hashes, tamper rejection, noncanonical archive paths, schema 1 migration without duplication, and portable archive round trips.

## 2026-08-27 — Fixed desktop minimum widths can hide the core workflow

- Trigger: the application was used in a narrow window.
- Symptom: the inspector and export area remained visible while the model controls were pushed outside the viewport, making later edits appear impossible.
- Root cause: `body` enforced a 1024/940 px minimum width while the page disabled normal document scrolling.
- Fix: widths below 1100 px use four accessible workspace tabs—Model, Structure, Validation and Export—with normal vertical scrolling and no fixed body minimum.
- Regression gate: web tests reject the former minimum-width declarations and require the narrow-screen tabs, active revision label and revision controls.

## 2026-08-27 — Seeded optimization is not deterministic if floating residuals iterate over maps

- Trigger: the new native pair/triplet SQS deterministic replay test ran on the Windows release builder.
- Symptom: identical inputs and random seed occasionally produced different optimized species arrangements or evidence, although single local runs often passed.
- Root cause: residual squares were accumulated by ranging over Go maps. Random map iteration changed floating-point summation at the last bits, which could change simulated-annealing acceptance decisions.
- Fix: pair and triplet correlation keys are sorted once and all objective accumulation follows that stable order.
- Regression gate: the deterministic replay test is run hundreds of times locally before release, remains part of the normal Go suite, and is executed again inside the Windows installer build.
