package httpapi

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tialloystudio/internal/app"
)

func TestBatchExportSaveAsWritesPhase2SeriesArchiveAndReportsPath(t *testing.T) {
	state := app.NewState()
	if _, err := state.BuildUser(app.BuildRequest{Module: "stacking_fault", Phase: "alpha", NX: 3, NY: 3, NZ: 4, SeriesCount: 3}); err != nil {
		t.Fatal(err)
	}
	selectedPath := filepath.Join(t.TempDir(), "fault-series.zip")
	h := newHandlerWithNativeHooks(state, nativeHooks{
		saveFile: func(req saveFileRequest) (string, bool, error) {
			if req.Format != "zip" {
				t.Fatalf("save dialog format=%q, want zip", req.Format)
			}
			if !strings.HasSuffix(req.SuggestedName, ".zip") {
				t.Fatalf("suggested name = %q, want zip", req.SuggestedName)
			}
			return selectedPath, false, nil
		},
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/export-batch/save", bytes.NewBufferString(`{"format":"poscar","suggested_name":"fault-series.zip"}`))
	r.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("save batch status=%d body=%s", w.Code, w.Body.String())
	}
	var out struct {
		Saved    bool   `json:"saved"`
		Path     string `json:"path"`
		Filename string `json:"filename"`
		Bytes    int64  `json:"bytes"`
		SHA256   string `json:"sha256"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if !out.Saved || out.Path != selectedPath || out.Filename != "fault-series.zip" || out.Bytes == 0 || out.SHA256 == "" {
		t.Fatalf("unexpected save response: %+v", out)
	}
	data, err := os.ReadFile(selectedPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("manifest.csv")) {
		t.Fatal("saved archive does not contain manifest.csv")
	}
}

func TestCalculationPackageSaveAsWritesPhase3InputArchiveAndReportsPath(t *testing.T) {
	state := app.NewState()
	if _, err := state.BuildUser(app.BuildRequest{Module: "random", Phase: "beta", NX: 3, NY: 3, NZ: 3, CompositionWt: map[string]float64{"Mo": 8}, Seed: 7}); err != nil {
		t.Fatal(err)
	}
	selectedPath := filepath.Join(t.TempDir(), "phase3-inputs.zip")
	h := newHandlerWithNativeHooks(state, nativeHooks{
		saveFile: func(req saveFileRequest) (string, bool, error) {
			if req.Format != "zip" {
				t.Fatalf("save dialog format=%q, want zip", req.Format)
			}
			if !strings.Contains(req.SuggestedName, "Phase3") {
				t.Fatalf("suggested name = %q, want Phase3 calculation package name", req.SuggestedName)
			}
			return selectedPath, false, nil
		},
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/calculation-package/save", bytes.NewBufferString(`{"target":"all"}`))
	r.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("save calculation package status=%d body=%s", w.Code, w.Body.String())
	}
	var out struct {
		Saved  bool   `json:"saved"`
		Path   string `json:"path"`
		Bytes  int64  `json:"bytes"`
		SHA256 string `json:"sha256"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if !out.Saved || out.Path != selectedPath || out.Bytes == 0 || out.SHA256 == "" {
		t.Fatalf("unexpected save response: %+v", out)
	}
	data, err := os.ReadFile(selectedPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range [][]byte{[]byte("manifest.json"), []byte("vasp/POSCAR"), []byte("lammps/model.data"), []byte("gpumd/model.extxyz")} {
		if !bytes.Contains(data, want) {
			t.Fatalf("saved calculation package missing %s", want)
		}
	}
}

func TestCalculationPackageSaveAsAcceptsPhase3PresetOptions(t *testing.T) {
	state := app.NewState()
	if _, err := state.BuildUser(app.BuildRequest{Module: "random", Phase: "alpha", NX: 3, NY: 3, NZ: 3, CompositionWt: map[string]float64{"Al": 6, "V": 4}, Seed: 17}); err != nil {
		t.Fatal(err)
	}
	selectedPath := filepath.Join(t.TempDir(), "phase3-static-v.vasp.zip")
	h := newHandlerWithNativeHooks(state, nativeHooks{
		saveFile: func(req saveFileRequest) (string, bool, error) {
			if !strings.Contains(req.SuggestedName, "Phase3") {
				t.Fatalf("suggested name = %q, want Phase3 package name", req.SuggestedName)
			}
			return selectedPath, false, nil
		},
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/calculation-package/save", bytes.NewBufferString(`{
		"target":"vasp",
		"workflow_preset":"static",
		"vasp_kpoints":"5 5 3",
		"vasp_encut_ev":450,
		"vasp_ismear":1,
		"vasp_sigma":0.15,
		"vasp_ediff":"1e-6"
	}`))
	r.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("save calculation package status=%d body=%s", w.Code, w.Body.String())
	}
	data, err := os.ReadFile(selectedPath)
	if err != nil {
		t.Fatal(err)
	}
	contents := readHTTPZipTextMembers(t, data)
	for _, check := range []struct {
		name string
		want string
	}{
		{"manifest.json", `"workflow_preset": "static"`},
		{"manifest.json", `"vasp_kpoints": "5 5 3"`},
		{"vasp/INCAR.template", "ENCUT = 450"},
		{"vasp/INCAR.template", "EDIFF = 1e-6"},
		{"vasp/KPOINTS.template", "5 5 3"},
	} {
		if !strings.Contains(contents[check.name], check.want) {
			t.Fatalf("%s missing %q:\n%s", check.name, check.want, contents[check.name])
		}
	}
}

func readHTTPZipTextMembers(t *testing.T, data []byte) map[string]string {
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
