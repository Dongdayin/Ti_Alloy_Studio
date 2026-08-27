package httpapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"tialloystudio/internal/app"
	"tialloystudio/internal/engines"
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
	mux.HandleFunc("/api/capabilities", a.capabilities)
	mux.HandleFunc("/api/connectors", a.connectors)
	mux.HandleFunc("/api/project", a.project)
	mux.HandleFunc("/api/project/export", a.projectExport)
	mux.HandleFunc("/api/project/import", a.projectImport)
	mux.HandleFunc("/api/project/revision", a.projectRevision)
	mux.HandleFunc("/api/project/select", a.projectSelect)
	mux.HandleFunc("/api/project/edit", a.projectEdit)
	mux.HandleFunc("/api/project/derive", a.projectDerive)
	return mux
}

func (a *api) info(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"name":     "Ti Alloy Studio",
		"version":  "0.2.0-phase1-r11",
		"engine":   "TiModelCore Native + bundled Atomsk/ASE/spglib/pymatgen/AtomMan validation",
		"platform": "Windows x64 standalone offline structure modeling; no WSL or local solver required",
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
	res, err := a.state.BuildUser(req)
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
	name, mime, content, err := a.state.ExportRevision(r.URL.Query().Get("revision_id"), r.URL.Query().Get("format"))
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
	writeJSON(w, http.StatusOK, engines.DetectEnvironment(r.URL.Query().Get("distro")))
}

func (a *api) capabilities(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, engines.DetectCapabilities())
}

func (a *api) connectors(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if strings.EqualFold(r.URL.Query().Get("probe"), "true") {
		writeJSON(w, http.StatusOK, map[string]any{
			"probe_performed": true,
			"report":          engines.DetectEnvironment(r.URL.Query().Get("distro")),
		})
		return
	}
	connectors := []engines.Capability{}
	for _, capability := range engines.DetectCapabilities().Capabilities {
		if capability.Category == "external_connector" {
			connectors = append(connectors, capability)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"probe_performed": false, "connectors": connectors})
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
	b, err := a.state.ExportProjectArchive(r.URL.Query().Get("name"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", "application/vnd.tialloystudio.project+zip")
	w.Header().Set("Content-Disposition", `attachment; filename="project.tias-project"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(b)
}

func (a *api) projectImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	limited := http.MaxBytesReader(w, r.Body, 256<<20)
	data, err := io.ReadAll(limited)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("read project package: %w", err))
		return
	}
	var res app.BuildResponse
	if strings.Contains(strings.ToLower(r.Header.Get("Content-Type")), "json") {
		var manifest app.ProjectManifest
		dec := json.NewDecoder(strings.NewReader(string(data)))
		dec.DisallowUnknownFields()
		if err = dec.Decode(&manifest); err == nil {
			res, err = a.state.ImportProject(manifest)
		}
	} else {
		res, err = a.state.ImportProjectArchive(data)
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (a *api) projectRevision(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	m := a.state.ProjectManifest("")
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		writeJSON(w, http.StatusOK, m)
		return
	}
	for _, record := range m.History {
		if record.ID == id {
			writeJSON(w, http.StatusOK, record)
			return
		}
	}
	writeError(w, http.StatusNotFound, fmt.Errorf("revision %q not found", id))
}

func decodeStrictJSON(w http.ResponseWriter, r *http.Request, out any) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("invalid request: %w", err)
	}
	return nil
}

func (a *api) projectSelect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		RevisionID string `json:"revision_id"`
	}
	if err := decodeStrictJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if _, err := a.state.SelectRevision(req.RevisionID); err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, a.state.ProjectManifest(""))
}

func (a *api) projectEdit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ParentRevisionID string           `json:"parent_revision_id"`
		Request          app.BuildRequest `json:"request"`
	}
	if err := decodeStrictJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if _, err := a.state.BuildChild(req.ParentRevisionID, req.Request); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, a.state.ProjectManifest(""))
}

func (a *api) projectDerive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ParentRevisionID string `json:"parent_revision_id"`
		Operation        string `json:"operation"`
		SiteID           int    `json:"site_id"`
		NewSpecies       string `json:"new_species,omitempty"`
	}
	if err := decodeStrictJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if _, err := a.state.DeriveRevision(req.ParentRevisionID, app.DeriveRequest{Operation: req.Operation, SiteID: req.SiteID, NewSpecies: req.NewSpecies}); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, a.state.ProjectManifest(""))
}
