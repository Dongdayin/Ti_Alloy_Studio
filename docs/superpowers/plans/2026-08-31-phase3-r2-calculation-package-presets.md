# Phase 3 R2 Calculation Package Presets Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Upgrade Phase 3 calculation-input ZIP export from fixed templates into parameterized, user-controlled VASP/LAMMPS/GPUMD preparation packages without running solvers or fabricating results.

**Architecture:** Keep solver preparation in `internal/app` behind a request model, expose it through strict JSON in `internal/httpapi`, and keep the web UI as a thin parameter collector. The generated package remains a ZIP with structures, templates, `manifest.json`, and explicit `not_relaxed` / `not_calculated` semantics.

**Tech Stack:** Go native core, Go HTTP API, embedded HTML/CSS/JavaScript UI, ZIP text artifacts.

**Spec:** `docs/PHASE3_COMPUTATIONAL_WORKSTATION.md`

## Global Constraints

- Ti Alloy Studio remains modeling/input-preparation software, not a VASP/LAMMPS/GPUMD solver.
- Do not bundle licensed VASP binaries, pseudopotentials, LAMMPS potentials, GPUMD/NEP potentials, or any proprietary input database.
- Do not create artificial energy, force, stress, convergence, or stability fields.
- Every exported structure remains `scientific_state = not_relaxed` and `calculation_state = not_calculated`.
- User-facing defaults must work without WSL, system Python, PATH tools, Atomsk, ATAT, LAMMPS, VASP, GPUMD, or Tachyon.

---

### Task 1: Parameterized app-level calculation package request

**Files:**
- Modify: `internal/app/phase3_workflow_test.go`
- Modify: `internal/app/app.go`
- Create: `internal/app/calculation_package.go`

**Interfaces:**
- Consumes: active `State.Current`, active `State.CurrentRequest`, `model.ExportPOSCAR`, `model.ExportLAMMPS`, `model.ExportExtXYZ`
- Produces: `type CalculationPackageRequest struct`, `func (s *State) ExportCalculationPackageWithOptions(req CalculationPackageRequest) (filename, mime string, content []byte, err error)`, existing `func (s *State) ExportCalculationPackage(target string)` remains as a compatibility wrapper

- [ ] **Step 1: Write the failing app test**

```go
func TestPhase3CalculationPackageAppliesWorkflowPresetsAndUserSettings(t *testing.T) {
	st := NewState()
	_, err := st.BuildUser(BuildRequest{
		Module: "random", Phase: "alpha", NX: 3, NY: 3, NZ: 3,
		CompositionWt: map[string]float64{"Al": 6, "V": 4},
		Seed: 31, ValidationMode: "fast",
	})
	if err != nil { t.Fatal(err) }

	_, _, data, err := st.ExportCalculationPackageWithOptions(CalculationPackageRequest{
		Target: "all", WorkflowPreset: "relaxation",
		VASPKPoints: "4 4 4", VASPENCUTeV: 520, VASPISMEAR: 1, VASPSigma: 0.2, VASPEDIFF: "1e-5",
		LAMMPSPairStyle: "eam/alloy", LAMMPSPairCoeff: "* * TiAlV.eam.alloy Ti Al V", LAMMPSRunSteps: 2000,
		GPUMDEnsemble: "nvt", GPUMDTemperatureK: 300, GPUMDRunSteps: 5000,
	})
	if err != nil { t.Fatal(err) }
	contents := readZipTextMembers(t, data)
	requireContains(t, contents["manifest.json"], `"workflow_preset": "relaxation"`)
	requireContains(t, contents["vasp/INCAR.template"], "ENCUT = 520")
	requireContains(t, contents["vasp/KPOINTS.template"], "4 4 4")
	requireContains(t, contents["lammps/in.lammps.template"], "pair_style eam/alloy")
	requireContains(t, contents["lammps/in.lammps.template"], "run 2000")
	requireContains(t, contents["gpumd/run.in.template"], "ensemble nvt 300 300")
	requireContains(t, contents["gpumd/run.in.template"], "run 5000")
	requireNotContains(t, strings.ToLower(strings.Join(mapValues(contents), "\n")), "final_energy")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app -run TestPhase3CalculationPackageAppliesWorkflowPresetsAndUserSettings -count=1`

Expected: FAIL because `CalculationPackageRequest` and `ExportCalculationPackageWithOptions` do not exist yet.

- [ ] **Step 3: Implement app-level request normalization and ZIP writers**

Create `internal/app/calculation_package.go` with normalized defaults:

```go
type CalculationPackageRequest struct {
	Target string `json:"target"`
	WorkflowPreset string `json:"workflow_preset,omitempty"`
	VASPKPoints string `json:"vasp_kpoints,omitempty"`
	VASPENCUTeV int `json:"vasp_encut_ev,omitempty"`
	VASPISMEAR int `json:"vasp_ismear,omitempty"`
	VASPSigma float64 `json:"vasp_sigma,omitempty"`
	VASPEDIFF string `json:"vasp_ediff,omitempty"`
	LAMMPSPairStyle string `json:"lammps_pair_style,omitempty"`
	LAMMPSPairCoeff string `json:"lammps_pair_coeff,omitempty"`
	LAMMPSRunSteps int `json:"lammps_run_steps,omitempty"`
	GPUMDEnsemble string `json:"gpumd_ensemble,omitempty"`
	GPUMDTemperatureK float64 `json:"gpumd_temperature_k,omitempty"`
	GPUMDRunSteps int `json:"gpumd_run_steps,omitempty"`
}
```

Move existing fixed-template package code out of `app.go`, keep `ExportCalculationPackage(target string)` as:

```go
return s.ExportCalculationPackageWithOptions(CalculationPackageRequest{Target: target})
```

- [ ] **Step 4: Run app tests**

Run: `go test ./internal/app -run "TestPhase3CalculationPackage" -count=1`

Expected: PASS.

### Task 2: HTTP API strict request support

**Files:**
- Modify: `internal/httpapi/phase2_batch_export_test.go`
- Modify: `internal/httpapi/httpapi.go`

**Interfaces:**
- Consumes: `app.CalculationPackageRequest`
- Produces: `/api/calculation-package/save` accepts target plus preset fields; `/api/calculation-package` accepts equivalent query fields for browser download

- [ ] **Step 1: Write the failing API test**

```go
r := httptest.NewRequest(http.MethodPost, "/api/calculation-package/save", bytes.NewBufferString(`{
  "target":"vasp",
  "workflow_preset":"static",
  "vasp_kpoints":"5 5 3",
  "vasp_encut_ev":450,
  "vasp_ismear":1,
  "vasp_sigma":0.15,
  "vasp_ediff":"1e-6"
}`))
```

Assert the saved ZIP contains `vasp/INCAR.template`, `ENCUT = 450`, `EDIFF = 1e-6`, `vasp/KPOINTS.template`, and `5 5 3`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/httpapi -run TestCalculationPackageSaveAsAcceptsPhase3PresetOptions -count=1`

Expected: FAIL because the strict decoder rejects the new fields or the app method is not wired.

- [ ] **Step 3: Wire API to `ExportCalculationPackageWithOptions`**

Decode POST into:

```go
var req struct {
	app.CalculationPackageRequest
	SuggestedName string `json:"suggested_name"`
}
```

For GET query export, build the same request from query values with integer/float parsing and pass it to the app service.

- [ ] **Step 4: Run API tests**

Run: `go test ./internal/httpapi -run "TestCalculationPackage" -count=1`

Expected: PASS.

### Task 3: UI controls for calculation-input preparation

**Files:**
- Modify: `internal/webapp/phase2_ui_test.go`
- Modify: `internal/webapp/static/index.html`
- Modify: `internal/webapp/static/app.js`
- Modify: `internal/webapp/static/style.css`

**Interfaces:**
- Consumes: `/api/calculation-package/save`
- Produces: user controls for workflow preset, VASP KPOINTS/ENCUT/ISMEAR/SIGMA/EDIFF, LAMMPS pair style/coeff/run steps, GPUMD ensemble/temperature/run steps

- [ ] **Step 1: Write the failing UI test**

Check `index.html` contains ids:

```text
calculationWorkflowPreset
vaspKpoints
vaspEncut
lammpsPairStyle
lammpsPairCoeff
gpumdEnsemble
gpumdRunSteps
```

Check `app.js` sends JSON keys:

```text
workflow_preset
vasp_kpoints
vasp_encut_ev
lammps_pair_style
gpumd_ensemble
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/webapp -run TestWorkbenchExposesPhase2PrecisionControlsAndViewerHelpers -count=1`

Expected: FAIL because the controls do not exist.

- [ ] **Step 3: Add UI controls and request payload**

Extend the existing `calculationPackage` details block. Keep labels short and Chinese-first. Send values in `saveCalculationPackage()`.

- [ ] **Step 4: Run UI test and JS syntax check**

Run:

```powershell
go test ./internal/webapp -run TestWorkbenchExposesPhase2PrecisionControlsAndViewerHelpers -count=1
node --check internal\webapp\static\app.js
```

Expected: PASS.

### Task 4: Documentation and release identity

**Files:**
- Modify: `README.md`
- Modify: `docs/PHASE3_COMPUTATIONAL_WORKSTATION.md`
- Modify: `cmd/installer/main.go`
- Modify: `cmd/installer/main_test.go`
- Modify: `internal/httpapi/httpapi.go`
- Modify: `internal/httpapi/revision_api_test.go`

**Interfaces:**
- Consumes: Phase 3 R2 behavior from Tasks 1-3
- Produces: version `0.4.1-phase3-r2`, docs explaining parameterized input packages and solver boundary

- [ ] **Step 1: Update version tests first**

Set expected version in tests to `0.4.1-phase3-r2`.

- [ ] **Step 2: Run focused version tests to verify failure**

Run: `go test ./cmd/installer ./internal/httpapi -run "Version|Info" -count=1`

Expected: FAIL because production version is still `0.4.0-phase3-r1`.

- [ ] **Step 3: Update version constants and docs**

Set installer/API version to `0.4.1-phase3-r2`. Update README and Phase 3 document to mention parameterized package presets.

- [ ] **Step 4: Run focused version/docs-adjacent tests**

Run: `go test ./cmd/installer ./internal/httpapi -run "Version|Info|CalculationPackage" -count=1`

Expected: PASS.

### Task 5: Full verification, package, install smoke, commit, and push

**Files:**
- No new feature files; verify all changed files.

**Interfaces:**
- Consumes: completed Tasks 1-4
- Produces: pushed `r3-offline` commit and refreshed Phase 3 CI result

- [ ] **Step 1: Run full local verification**

Run:

```powershell
go test ./... -count=1
go vet ./...
node --check internal\webapp\static\app.js
```

Expected: PASS.

- [ ] **Step 2: Build offline installer**

Run: `.\scripts\build_r3_offline.ps1`

Expected: creates `dist\TiAlloyStudio-Setup-x64-Offline.exe` and verifies bundled engines.

- [ ] **Step 3: Install and smoke test**

Install to `E:\Edownload\TiAlloy\TiAlloyStudio`, run `--smoke-test-file`, run `--engine-smoke-file`, launch the app visibly, and verify `/api/info` shows `0.4.1-phase3-r2`.

- [ ] **Step 4: Commit and push source changes only**

Stage only source/tests/docs/workflow files touched by R2, not `build/`, `dist/`, or generated installer payload files.

- [ ] **Step 5: Check remote CI**

Confirm `Phase 3 Go Scientific Core` and `Build Phase 3 Offline Windows Installer` are successful for the pushed commit.
