package httpapi

import (
	"bytes"
	"encoding/json"
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
