package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"

	"tialloystudio/internal/app"
)

type api struct{ state *app.State }

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]any{"error": err.Error()})
}

func NewHandler(state *app.State) http.Handler {
	a := &api{state: state}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/info", a.info)
	mux.HandleFunc("/api/build", a.build)
	mux.HandleFunc("/api/export", a.export)
	mux.HandleFunc("/api/export-batch", a.exportBatch)
	mux.HandleFunc("/api/environment", a.environment)
	mux.HandleFunc("/api/project", a.project)
	mux.HandleFunc("/api/project/export", a.projectExport)
	mux.HandleFunc("/api/project/import", a.projectImport)
	return mux
}

func (a *api) info(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"name":     "Ti Alloy Studio",
		"version":  "0.1.4-phase1",
		"engine":   "TiModelCore Native + bundled Atomsk/ASE/spglib/pymatgen/AtomMan cross-check + WSL ATAT adapter",
		"platform": "Windows x64 standalone offline with optional WSL scientific tools",
	})
}

func (a *api) build(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	var req app.BuildRequest
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid build request: %w", err))
		return
	}
	res, err := a.state.BuildTracked(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (a *api) export(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	name, mime, content, err := a.state.Export(r.URL.Query().Get("format"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(content))
}

func (a *api) exportBatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	name, mime, content, err := a.state.ExportBatch(r.URL.Query().Get("format"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

func (a *api) environment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, app.EnvironmentReport())
}

func (a *api) project(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, a.state.ProjectManifest(r.URL.Query().Get("name")))
}

func (a *api) projectExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	manifest := a.state.ProjectManifest(r.URL.Query().Get("name"))
	b, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="project.json"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(b)
}

func (a *api) projectImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20))
	dec.DisallowUnknownFields()
	var manifest app.ProjectManifest
	if err := dec.Decode(&manifest); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid project manifest: %w", err))
		return
	}
	res, err := a.state.ImportProject(manifest)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}
