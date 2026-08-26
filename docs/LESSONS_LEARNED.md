# Lessons learned

## 2026-08-27 — Do not use a registered CPython installer for an app-local runtime

- Trigger: CPython 3.11.9 was already installed for the current Windows user.
- Symptom: the official CPython installer returned exit code 0 for a different `TargetDir`, but created no private runtime there; Ti Alloy Studio then failed because `engines\python\python.exe` was absent.
- Root cause: the CPython bootstrapper entered maintenance mode for the registered same-version installation instead of deploying a second application-local copy. The clean CI runner did not contain this state.
- Fix: build the science environment into the official CPython embeddable distribution during release construction. The end-user installer now only extracts the verified private runtime and Atomsk.
- Regression gates: unit tests verify real nested extraction and reject path traversal; Windows release CI first installs the same CPython version for the current user, then proves Ti Alloy Studio installs, passes engine smoke tests, and leaves that existing Python byte-identical.
- Prevention: an application-private runtime must be a relocatable release artifact. A successful third-party installer exit code is never evidence that the requested target files were deployed.
