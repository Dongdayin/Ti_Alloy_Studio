package app

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestPhase3CalculationPackagePreparesInputsWithoutSolverResults(t *testing.T) {
	st := NewState()
	_, err := st.BuildUser(BuildRequest{
		Module:         "random",
		Phase:          "alpha",
		NX:             3,
		NY:             3,
		NZ:             3,
		CompositionWt:  map[string]float64{"Al": 6, "V": 4},
		Seed:           31,
		ValidationMode: "fast",
	})
	if err != nil {
		t.Fatal(err)
	}

	name, mime, data, err := st.ExportCalculationPackage("vasp")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(name, ".zip") || mime != "application/zip" {
		t.Fatalf("package identity = %q %q, want zip/application", name, mime)
	}
	if !strings.Contains(name, "Phase3-R4") {
		t.Fatalf("package name = %q, want Phase3-R4 release marker", name)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	contents := map[string]string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		contents[f.Name] = string(body)
	}
	for _, required := range []string{"manifest.json", "README.txt", "vasp/POSCAR", "vasp/INCAR.template", "vasp/KPOINTS.template"} {
		if _, ok := contents[required]; !ok {
			t.Fatalf("calculation package missing %s; files=%v", required, keys(contents))
		}
	}
	requireContains(t, contents["README.txt"], "Phase 3 R4")
	allText := strings.ToLower(strings.Join(mapValues(contents), "\n"))
	for _, forbidden := range []string{"outcar", "oszicar", "final_energy", "forces=", "stress="} {
		if strings.Contains(allText, forbidden) {
			t.Fatalf("calculation input package contains solver-result field %q:\n%s", forbidden, allText)
		}
	}
	if !strings.Contains(allText, "not_calculated") {
		t.Fatalf("calculation package did not record not_calculated semantics:\n%s", allText)
	}
}

func TestPhase3CalculationPackageAppliesWorkflowPresetsAndUserSettings(t *testing.T) {
	st := NewState()
	_, err := st.BuildUser(BuildRequest{
		Module:         "random",
		Phase:          "alpha",
		NX:             3,
		NY:             3,
		NZ:             3,
		CompositionWt:  map[string]float64{"Al": 6, "V": 4},
		Seed:           31,
		ValidationMode: "fast",
	})
	if err != nil {
		t.Fatal(err)
	}

	_, _, data, err := st.ExportCalculationPackageWithOptions(CalculationPackageRequest{
		Target:            "all",
		WorkflowPreset:    "relaxation",
		VASPKPoints:       "4 4 4",
		VASPENCUTeV:       520,
		VASPISMEAR:        1,
		VASPSigma:         0.2,
		VASPEDIFF:         "1e-5",
		LAMMPSPairStyle:   "eam/alloy",
		LAMMPSPairCoeff:   "* * TiAlV.eam.alloy Ti Al V",
		LAMMPSRunSteps:    2000,
		GPUMDEnsemble:     "nvt",
		GPUMDTemperatureK: 300,
		GPUMDRunSteps:     5000,
	})
	if err != nil {
		t.Fatal(err)
	}
	contents := readZipTextMembers(t, data)

	requireContains(t, contents["manifest.json"], `"workflow_preset": "relaxation"`)
	requireContains(t, contents["manifest.json"], `"vasp_kpoints": "4 4 4"`)
	requireContains(t, contents["manifest.json"], `"calculation_state": "not_calculated"`)
	requireContains(t, contents["vasp/INCAR.template"], "ENCUT = 520")
	requireContains(t, contents["vasp/INCAR.template"], "ISMEAR = 1")
	requireContains(t, contents["vasp/INCAR.template"], "SIGMA = 0.2")
	requireContains(t, contents["vasp/INCAR.template"], "EDIFF = 1e-5")
	requireContains(t, contents["vasp/KPOINTS.template"], "4 4 4")
	requireContains(t, contents["lammps/in.lammps.template"], "pair_style eam/alloy")
	requireContains(t, contents["lammps/in.lammps.template"], "pair_coeff * * TiAlV.eam.alloy Ti Al V")
	requireContains(t, contents["lammps/in.lammps.template"], "run 2000")
	requireContains(t, contents["gpumd/run.in.template"], "ensemble nvt 300 300")
	requireContains(t, contents["gpumd/run.in.template"], "run 5000")

	allText := strings.ToLower(strings.Join(mapValues(contents), "\n"))
	for _, forbidden := range []string{"outcar", "oszicar", "final_energy", "forces=", "stress="} {
		requireNotContains(t, allText, forbidden)
	}
}

func TestPhase3CalculationPackageRejectsUnknownTarget(t *testing.T) {
	st := NewState()
	if _, _, _, err := st.ExportCalculationPackage("unknown_solver"); err == nil {
		t.Fatal("unknown calculation package target was accepted")
	}
}

func readZipTextMembers(t *testing.T, data []byte) map[string]string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	contents := map[string]string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		contents[f.Name] = string(body)
	}
	return contents
}

func requireContains(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Fatalf("text missing %q:\n%s", want, got)
	}
}

func requireNotContains(t *testing.T, got, forbidden string) {
	t.Helper()
	if strings.Contains(got, forbidden) {
		t.Fatalf("text contains forbidden %q:\n%s", forbidden, got)
	}
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func mapValues(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	return out
}
