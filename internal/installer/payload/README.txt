Ti Alloy Studio 0.2.0 Phase 1 R12 Offline

This release is intended to be a single-install, pure Windows x64 titanium-alloy atomistic modeling workstation.

R3 OFFLINE policy:
- End-user installation does not download Python, wheels, or Atomsk.
- The installer contains engine-bundle.zip prepared at release-build time.
- CPython, ASE, spglib, pymatgen-core, AtomMan and Atomsk are installed privately under the Ti Alloy Studio installation directory.
- No system Python/Conda/PATH modification is required.
- Installation is accepted only after TiModelCore and all bundled mature-engine cross-checks pass.
- Normal modeling, validation, project save/open, revision editing and structure export do not invoke WSL or locally installed VASP/LAMMPS/GPUMD/ATAT tools.
- Portable projects use one .tias-project package containing immutable structure snapshots, lineage and SHA-256 verification data.
- VASP POSCAR, LAMMPS data and extended-XYZ buttons export input structures only; Ti Alloy Studio does not run those production solvers.

See TiAlloyStudio-Manual.docx for scientific methods, model validation and operation.
