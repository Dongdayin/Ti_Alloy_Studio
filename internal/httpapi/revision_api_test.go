package httpapi

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"tialloystudio/internal/app"
)

func TestRevisionAPISelectDeriveAndExportHistoricalSnapshot(t *testing.T) {
	state := app.NewState()
	h := NewHandler(state)
	postJSON(t, h, "/api/build", `{"module":"crystal","phase":"alpha","nx":2,"ny":2,"nz":2}`, http.StatusOK)
	first := state.ProjectManifest("").History[0]
	postJSON(t, h, "/api/build", `{"module":"crystal","phase":"beta","nx":2,"ny":2,"nz":2}`, http.StatusOK)
	secondID := state.ProjectManifest("").ActiveRevisionID

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/export?format=xyz&revision_id="+first.ID, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("historical export status=%d body=%s", w.Code, w.Body.String())
	}
	sum := sha256.Sum256(w.Body.Bytes())
	if got := hex.EncodeToString(sum[:]); got != first.ExportSHA256["xyz"] {
		t.Fatalf("historical export hash=%s want=%s", got, first.ExportSHA256["xyz"])
	}
	afterExport := state.ProjectManifest("")
	if len(afterExport.History) != 2 || afterExport.ActiveRevisionID != secondID {
		t.Fatal("historical export mutated project state")
	}

	selectBody := `{"revision_id":"` + first.ID + `"}`
	selected := postJSON(t, h, "/api/project/select", selectBody, http.StatusOK)
	if !bytes.Contains(selected, []byte(`"active_revision_id":"`+first.ID+`"`)) {
		t.Fatalf("select response does not identify active revision: %s", selected)
	}
	deriveBody := `{"parent_revision_id":"` + first.ID + `","operation":"substitution","site_id":0,"new_species":"Nb"}`
	derived := postJSON(t, h, "/api/project/derive", deriveBody, http.StatusOK)
	if !bytes.Contains(derived, []byte(`"parent_id":"`+first.ID+`"`)) || !bytes.Contains(derived, []byte(`"scientific_state":"not_relaxed"`)) {
		t.Fatalf("derive response missing lineage/scientific state: %s", derived)
	}
}

func TestProjectArchiveAPIUsesSinglePortablePackage(t *testing.T) {
	state := app.NewState()
	h := NewHandler(state)
	postJSON(t, h, "/api/build", `{"module":"crystal","phase":"alpha"}`, http.StatusOK)

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/project/export?name=portable", nil))
	if w.Code != http.StatusOK || w.Header().Get("Content-Type") != "application/vnd.tialloystudio.project+zip" {
		t.Fatalf("project export status/type=%d/%q body=%s", w.Code, w.Header().Get("Content-Type"), w.Body.String())
	}
	data := append([]byte(nil), w.Body.Bytes()...)

	restored := app.NewState()
	restoreHandler := NewHandler(restored)
	w = httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/project/import", bytes.NewReader(data))
	r.Header.Set("Content-Type", "application/vnd.tialloystudio.project+zip")
	restoreHandler.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("project import status=%d body=%s", w.Code, w.Body.String())
	}
	if restored.ProjectManifest("").Name != "portable" || len(restored.ProjectManifest("").History) != 1 {
		t.Fatalf("restored project=%+v", restored.ProjectManifest(""))
	}
}

func TestCapabilitiesAPIUsesBundledCatalogWithoutAutomaticConnectorProbe(t *testing.T) {
	h := NewHandler(app.NewState())
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/capabilities", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("capabilities status=%d body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, want := range []string{`"id":"native_modeling"`, `"category":"export_format"`, `"status":"NOT_CONFIGURED"`} {
		if !bytes.Contains([]byte(body), []byte(want)) {
			t.Fatalf("capabilities response missing %s: %s", want, body)
		}
	}
	if bytes.Contains([]byte(body), []byte(`"wsl_distros"`)) || bytes.Contains([]byte(body), []byte(`"selected_distro"`)) {
		t.Fatalf("default capabilities leaked a WSL probe report: %s", body)
	}

	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/connectors", nil))
	if w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte(`"probe_performed":false`)) {
		t.Fatalf("unrequested connector response=%d %s", w.Code, w.Body.String())
	}
}

func postJSON(t *testing.T, h http.Handler, path, body string, wantStatus int) []byte {
	t.Helper()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	r.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(w, r)
	if w.Code != wantStatus {
		t.Fatalf("POST %s status=%d want=%d body=%s", path, w.Code, wantStatus, w.Body.String())
	}
	var valid any
	if err := json.Unmarshal(w.Body.Bytes(), &valid); err != nil {
		t.Fatalf("POST %s returned invalid JSON: %v", path, err)
	}
	return w.Body.Bytes()
}
