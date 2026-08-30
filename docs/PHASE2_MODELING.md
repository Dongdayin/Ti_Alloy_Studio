# Ti Alloy Studio Phase 2 modeling scope

Phase 2 keeps Ti Alloy Studio as a titanium-alloy modeling workstation. It does not run VASP, LAMMPS, GPUMD, NEB, EOS fitting, force-field fitting, or other solvers.

## Workflow

1. Define the Ti alloy system: Ti single crystal, random Ti alloy, or SQS Ti alloy.
2. Select α-Ti HCP or β-Ti BCC and enter lattice constants.
3. Set the supercell or target box size.
4. Choose one optional modeling operation. Leaving an operation page unused means the base model is generated only.
5. Inspect geometry, validation diagnostics, region labels, and composition.
6. Export one structure or a geometry-series ZIP package.

## Added Phase 2 model classes

- Dislocation initial geometries: first-pass α basal/prismatic/pyramidal and β `{110}<111>` / `{112}<111>` presets.
- Dislocation precision controls: optional user-entered Burgers vector and line direction override the preset geometry, and the 3D viewer draws helper arrows for Burgers vector, line direction, and slip-plane normal.
- Grain-boundary bicrystals: tilt, twist, and general-bicrystal UI fields with GB axis, GB normal, optional grain orientation matrices, topology, overlap-removal, and mismatch diagnostics.
- Stacking-fault and gamma-surface geometry series: displacement structures only.
- Twin geometries: first-pass α twin presets with parent/twin labels.
- Local chemistry: SRO-like distributions, segregation, solute clusters, vacancy-solute style clusters, and simplified precipitate inclusions as composition/label operations.
- Mechanical seed geometries: crack/notch, nanoindentation substrate plus indenter reference, and Voronoi polycrystal labels.
- NEB geometry sets: initial/final and interpolated structures only.
- Training configuration sets: extXYZ/POSCAR-ready structures tagged for later external labeling.

## Scientific semantics

Every Phase 2 output is an initial structure candidate. The structure metadata must include:

- `scientific_state = not_relaxed`
- `calculation_state = not_calculated`

Validation reports geometry and bookkeeping diagnostics only: Burgers vector relations, line directions, plane normals, region labels, mismatch estimates, atom counts, pair statistics, seeds, and PBC/topology.

The program must not write artificial total energies, atomic forces, stresses, stable-fault labels, convergence decisions, or equilibrium claims. External project calculations can consume these files later, but their results are outside this software stage.

## Export behavior

Single-structure export uses the native Save As dialog and returns the saved path, byte count, and SHA-256.

Series workflows export a ZIP package with:

- `manifest.csv`
- one POSCAR per structure, or extXYZ files for training configuration sets when selected
- `README.txt`

The manifest records only geometry-series fields such as index, kind, λ, shift vector, atom count, PBC, and path. Training extXYZ headers keep `scientific_state=not_relaxed` and `calculation_state=not_calculated`; they do not contain artificial energy, force, or stress values.

## Tachyon direction

The built-in Tachyon-style renderer is a bundled canvas renderer and requires no external binary. A real Tachyon executable can be considered later only after license attribution, Windows packaging, offline installation, and CI smoke tests are complete.
