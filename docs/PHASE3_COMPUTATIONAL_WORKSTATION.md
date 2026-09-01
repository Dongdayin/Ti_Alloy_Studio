# Ti Alloy Studio Phase 3 computational workstation scope

Phase 3 starts from the accepted Phase 2 modeling workstation and adds calculation-workflow preparation around the generated titanium-alloy structures.

## Boundary

Phase 3 does not make Ti Alloy Studio a solver. It must not bundle proprietary VASP binaries, choose undocumented potentials automatically, run production LAMMPS/GPUMD/VASP calculations without explicit configuration, or create artificial energy/force/stress labels.

Every structure entering a Phase 3 package remains:

- `scientific_state = not_relaxed`
- `calculation_state = not_calculated`

## Implemented slices

R1 exported a calculation-input package ZIP from the active model.

Supported targets:

- VASP: `POSCAR`, `INCAR.template`, `KPOINTS.template`
- LAMMPS: `model.data`, `in.lammps.template`
- GPUMD / NEP: `model.extxyz`, `run.in.template`
- All formats in one package

R2 makes these packages parameterized. The user can select a workflow preset and set template-level parameters before saving:

- workflow preset: structure only, relaxation, static single point, MD seed, defect, interface, NEB seed, or NEP labeling seed
- VASP: KPOINTS, ENCUT, ISMEAR, SIGMA, EDIFF
- LAMMPS: pair style, pair coefficient line, optional run steps
- GPUMD / NEP: ensemble, temperature, optional run steps

Every package includes:

- `manifest.json`
- `README.txt`
- one or more solver-specific input folders

The manifest records the package target, workflow preset, source model module, phase, atom count, structure SHA-256, template parameters, and `not_calculated` semantics.

R3 improves large-model generation latency. A successful build still records the exact structure snapshot and its structure SHA-256 immediately, but POSCAR, XYZ, extended XYZ, LAMMPS data and CIF hashes are computed only when the user exports that specific format. This preserves export traceability while avoiding five unused full-text export renders during every model-generation click.

## Next Phase 3 increments

1. Add user-facing workflow presets: relaxation, static single point, elastic constants, MD seed, defect workflow, interface workflow, NEP labeling seed.
2. Add potential/pseudopotential registry records without bundling licensed files.
3. Add local job folder management: prepared, submitted, running, finished, failed.
4. Add import of completed external results with evidence checks before marking any quantity calculated.
5. Add report generation that separates model validation from solver result validation.
