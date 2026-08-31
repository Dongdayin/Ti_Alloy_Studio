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

func TestPhase3CalculationPackageRejectsUnknownTarget(t *testing.T) {
	st := NewState()
	if _, _, _, err := st.ExportCalculationPackage("unknown_solver"); err == nil {
		t.Fatal("unknown calculation package target was accepted")
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
