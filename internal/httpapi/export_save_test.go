package httpapi

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"tialloystudio/internal/app"
)

func TestExportSaveAsWritesChosenFileAndReturnsReadableDetails(t *testing.T) {
	state := app.NewState()
	if _, err := state.BuildUser(app.BuildRequest{Module: "crystal", Phase: "alpha", NX: 1, NY: 1, NZ: 1}); err != nil {
		t.Fatal(err)
	}
	before := state.ProjectManifest("")
	selectedPath := filepath.Join(t.TempDir(), "alpha-ti.xyz")
	var openedPath string
	h := newHandlerWithNativeHooks(state, nativeHooks{
		saveFile: func(req saveFileRequest) (string, bool, error) {
			if req.Format != "xyz" {
				t.Fatalf("save dialog format=%q", req.Format)
			}
			if req.SuggestedName == "" {
				t.Fatal("save dialog did not receive a suggested file name")
			}
			return selectedPath, false, nil
		},
		openFolder: func(path string) error {
			openedPath = path
			return nil
		},
	})

	w := httptest.NewRecorder()
	body := `{"format":"xyz","revision_id":"` + before.ActiveRevisionID + `","suggested_name":"alpha-ti.xyz"}`
	r := httptest.NewRequest(http.MethodPost, "/api/export/save", bytes.NewBufferString(body))
	r.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("save export status=%d body=%s", w.Code, w.Body.String())
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
	if !out.Saved || out.Path != selectedPath || out.Filename != "alpha-ti.xyz" || out.Bytes == 0 {
		t.Fatalf("unexpected save response: %+v", out)
	}
	data, err := os.ReadFile(selectedPath)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	if out.SHA256 != hex.EncodeToString(sum[:]) {
		t.Fatalf("response sha=%s actual=%s", out.SHA256, hex.EncodeToString(sum[:]))
	}
	after := state.ProjectManifest("")
	if len(after.History) != len(before.History) || after.ActiveRevisionID != before.ActiveRevisionID {
		t.Fatal("save-as export mutated project revision state")
	}

	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPost, "/api/open-folder", bytes.NewBufferString(`{"path":"`+filepath.ToSlash(selectedPath)+`"}`))
	r.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK || openedPath != selectedPath {
		t.Fatalf("open saved folder status=%d opened=%q body=%s", w.Code, openedPath, w.Body.String())
	}

	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPost, "/api/open-folder", bytes.NewBufferString(`{"path":"`+filepath.ToSlash(filepath.Join(filepath.Dir(selectedPath), "other.xyz"))+`"}`))
	r.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("open-folder accepted unsaved path: status=%d body=%s", w.Code, w.Body.String())
	}
}
