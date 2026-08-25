# Ti Alloy Studio — R3 Offline

Windows-first titanium-alloy atomistic modelling, validation, visualization and export workbench.

The `r3-offline` branch builds a **fully offline Windows x64 installer**. Release construction downloads and pins the managed scientific engines on the GitHub Windows runner, proves that the Python wheelhouse can be installed with `--no-index`, then embeds the resulting engine bundle into the final installer.

End users install one package and do not configure Python, Conda, WSL, Atomsk or PATH.

Bundled scientific cross-check engines for R3:

- CPython 3.11.9 x64 private runtime
- ASE 3.29.0
- spglib 2.7.0
- pymatgen-core 2026.7.31
- AtomMan 1.4.11
- Atomsk 0.13.1 Windows

The native TiModelCore remains the deterministic primary modelling engine. Mature third-party engines provide independent read-back and structural validation.
