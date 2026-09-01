# Ti Alloy Studio — titanium alloy modeler

Ti Alloy Studio 0.4.2 Phase 3 R3 is a Windows-first titanium-alloy structure modeling, validation, visualization, revision, export, and calculation-input preparation workbench.

This release is deliberately modeling-only. It creates titanium-alloy structure files for later use in VASP, LAMMPS, GPUMD, NEP training, and other workflows, but it does not run those calculations. A normal user installs one offline package and does not configure Python, Conda, WSL, Atomsk, ATAT, or `PATH`.

## Main workflow

1. Set the Ti alloy type: Ti single crystal, random Ti alloy, or SQS Ti alloy.
2. Choose the crystal structure, enter lattice constants, and set the supercell size.
3. Generate and inspect the base model.
4. Optionally add a defect, surface, α/β interface, dislocation, grain boundary, stacking-fault/gamma-surface geometry series, twin, local-chemistry region, crack seed, nanoindentation reference, polycrystal, NEB initial/final path, or training-configuration set. If an operation section is not selected, it is skipped.
5. Export POSCAR, XYZ, extended XYZ, LAMMPS data, or CIF as many times as needed. Save-as export reports the actual file path, byte count, and SHA-256.
6. For series workflows, export a ZIP package containing POSCAR files plus a geometry manifest.
7. Select any historical structure record without rebuilding it.
8. Restore that structure's recipe and generate an edited child.
9. Save all structure records as one `.tias-project` package and reopen it on another computer with the same software release.
10. Phase 3 can export parameterized calculation-input preparation packages for VASP, LAMMPS, and GPUMD/NEP from the active model.

Interactive builds default to fast validation: Ti Alloy Studio runs internal structural checks and skips the slower Atomsk/ASE cross-check. The advanced validation selector can be switched to deep validation for release, delivery, or troubleshooting runs.

Each successful revision stores its explicit parent, normalized recipe, exact structure, validation and engine evidence, scientific state, and structure SHA-256. Export SHA-256 hashes are recorded when a user actually exports that format, so large-model generation does not waste time rendering unused POSCAR/XYZ/extXYZ/LAMMPS/CIF text. A failed build, derivation, or project import leaves the active revision and history unchanged.

## Offline package

The `r3-offline` branch builds a fully offline Windows x64 installer. Release construction prepares and verifies a relocatable application-private engine bundle. End-user installation extracts verified files only; it does not run a Python installer or download packages.

Bundled components are:

- native TiModelCore structure modeling and deterministic pair/triplet-probability SQS;
- CPython 3.11.9 x64 private runtime;
- ASE 3.29.0;
- spglib 2.7.0;
- pymatgen-core 2026.7.31;
- AtomMan 1.4.11;
- NumPy and SciPy from the pinned offline wheelhouse;
- Atomsk 0.13.1 for independent structure checks.

The capability panel reports bundled modeling functions and export formats. External solvers and WSL tools are not required for modeling or export.

Phase 2 includes a bundled Tachyon-style structure renderer. It uses the local browser canvas to approximate ray-traced molecular figure lighting with spherical highlights, projected shadows, and ambient-occlusion-style depth cues, so PNG export remains available from the same offline application package. It does not require a local Tachyon executable.

## Scientific scope

Generated structures are labeled `not_relaxed` and `not_calculated`. The software does not infer energy, equilibrium, convergence, stability, or a relaxed defect/interface state. Native SQS results report selected pair and closed-triplet probability residuals and are labeled `not_atat_verified`; they are not claimed equivalent to ATAT basis-cluster correlations.

POSCAR, LAMMPS data, and extended-XYZ exports are file formats, not evidence that the corresponding solver was run.

Training configuration sets are seed structures for later external labeling. The requested configuration count is the exact number of exported structures. The first structure is the selected base geometry; additional structures use deterministic small strain and rattle perturbations and remain `not_calculated`.

Phase 3 calculation packages contain structures, workflow presets, user-selected template parameters, and editable input templates only. They do not include VASP/LAMMPS/GPUMD outputs, potentials, pseudopotentials, or calculated labels.

## Development verification

```powershell
go test ./...
go vet ./...
```

The Windows release gate additionally verifies installation to a path containing spaces, the private Python/Atomsk payload, native and mature-engine smoke tests, revision edit plus historical re-export, `.tias-project` save/reopen, preservation of a pre-existing system Python, uninstall registration, real uninstall, and release SHA-256 files.

See `docs/TiAlloyStudio-Manual.docx` for the Phase 1 scientific methods and the interface guide. See `docs/PHASE2_MODELING.md` and `docs/PHASE3_COMPUTATIONAL_WORKSTATION.md` for the current modeling and calculation-preparation scopes.
