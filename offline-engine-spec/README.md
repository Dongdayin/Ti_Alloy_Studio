# Ti Alloy Studio R3 offline engine policy

The end-user installer must not access the network.

Bundled release inputs:
- CPython 3.11.9 Windows x64 full installer
- Atomsk 0.13.1 Windows archive
- a fully resolved Windows CPython 3.11 wheelhouse
- pinned top-level packages: ASE 3.29.0, spglib 2.7.0, pymatgen-core 2026.7.31, AtomMan 1.4.11

`scripts/fetch_offline_engines.ps1` is a BUILD-TIME script. It may access the internet only on the release build machine. It creates `internal/installer/payload/engine-bundle.zip` and verifies that the resulting wheelhouse can install using `pip --no-index`.

The end-user installer extracts and installs only from that embedded bundle.
