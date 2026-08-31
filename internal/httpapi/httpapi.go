package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"tialloystudio/internal/app"
	"tialloystudio/internal/engines"
)

type saveFileRequest struct {
	Format        string `json:"format"`
	SuggestedName string `json:"suggested_name"`
	MIME          string `json:"mime,omitempty"`
}

type nativeHooks struct {
	saveFile   func(saveFileRequest) (path string, cancelled bool, err error)
	openFolder func(path string) error
}

type api struct {
	state      *app.State
	hooks      nativeHooks
	savedMu    sync.Mutex
	savedFiles map[string]bool
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]any{"error": err.Error()})
}

func NewHandler(state *app.State) http.Handler {
	return newHandlerWithNativeHooks(state, nativeHooks{})
}

func newHandlerWithNativeHooks(state *app.State, hooks nativeHooks) http.Handler {
	if hooks.saveFile == nil {
		hooks.saveFile = nativeSaveFile
	}
	if hooks.openFolder == nil {
		hooks.openFolder = nativeOpenFolder
	}
	a := &api{state: state, hooks: hooks, savedFiles: map[string]bool{}}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/info", a.info)
	mux.HandleFunc("/api/build", a.build)
	mux.HandleFunc("/api/export", a.export)
	mux.HandleFunc("/api/export/save", a.exportSave)
	mux.HandleFunc("/api/export-batch", a.exportBatch)
	mux.HandleFunc("/api/export-batch/save", a.exportBatchSave)
	mux.HandleFunc("/api/calculation-package", a.calculationPackage)
	mux.HandleFunc("/api/calculation-package/save", a.calculationPackageSave)
	mux.HandleFunc("/api/environment", a.environment)
	mux.HandleFunc("/api/capabilities", a.capabilities)
	mux.HandleFunc("/api/connectors", a.connectors)
	mux.HandleFunc("/api/project", a.project)
	mux.HandleFunc("/api/project/save", a.projectSave)
	mux.HandleFunc("/api/project/export", a.projectExport)
	mux.HandleFunc("/api/project/import", a.projectImport)
	mux.HandleFunc("/api/project/revision", a.projectRevision)
	mux.HandleFunc("/api/project/select", a.projectSelect)
	mux.HandleFunc("/api/project/edit", a.projectEdit)
	mux.HandleFunc("/api/project/derive", a.projectDerive)
	mux.HandleFunc("/api/open-folder", a.openFolder)
	return mux
}

func (a *api) info(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"name":     "Ti Alloy Studio",
		"version":  "0.4.1-phase3-r2",
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

func (a *api) exportSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		RevisionID    string `json:"revision_id"`
		Format        string `json:"format"`
		SuggestedName string `json:"suggested_name"`
	}
	if err := decodeStrictJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	name, mime, content, err := a.state.ExportRevision(req.RevisionID, req.Format)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	suggested := strings.TrimSpace(req.SuggestedName)
	if suggested == "" {
		suggested = name
	}
	a.saveBytes(w, saveFileRequest{Format: req.Format, SuggestedName: suggested, MIME: mime}, []byte(content))
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

func (a *api) exportBatchSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Format        string `json:"format"`
		SuggestedName string `json:"suggested_name"`
	}
	if err := decodeStrictJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	name, mime, content, err := a.state.ExportBatch(req.Format)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	suggested := strings.TrimSpace(req.SuggestedName)
	if suggested == "" {
		suggested = name
	}
	a.saveBytes(w, saveFileRequest{Format: "zip", SuggestedName: suggested, MIME: mime}, content)
}

func (a *api) calculationPackage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	req, err := calculationPackageRequestFromQuery(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	name, mime, content, err := a.state.ExportCalculationPackageWithOptions(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

func (a *api) calculationPackageSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		app.CalculationPackageRequest
		SuggestedName string `json:"suggested_name"`
	}
	if err := decodeStrictJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	name, mime, content, err := a.state.ExportCalculationPackageWithOptions(req.CalculationPackageRequest)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	suggested := strings.TrimSpace(req.SuggestedName)
	if suggested == "" {
		suggested = name
	}
	a.saveBytes(w, saveFileRequest{Format: "zip", SuggestedName: suggested, MIME: mime}, content)
}

func calculationPackageRequestFromQuery(r *http.Request) (app.CalculationPackageRequest, error) {
	q := r.URL.Query()
	req := app.CalculationPackageRequest{
		Target:          q.Get("target"),
		WorkflowPreset:  q.Get("workflow_preset"),
		VASPKPoints:     q.Get("vasp_kpoints"),
		VASPEDIFF:       q.Get("vasp_ediff"),
		LAMMPSPairStyle: q.Get("lammps_pair_style"),
		LAMMPSPairCoeff: q.Get("lammps_pair_coeff"),
		GPUMDEnsemble:   q.Get("gpumd_ensemble"),
	}
	var err error
	if req.VASPENCUTeV, err = optionalQueryInt(q.Get("vasp_encut_ev"), "vasp_encut_ev"); err != nil {
		return app.CalculationPackageRequest{}, err
	}
	if req.VASPISMEAR, err = optionalQueryInt(q.Get("vasp_ismear"), "vasp_ismear"); err != nil {
		return app.CalculationPackageRequest{}, err
	}
	if req.LAMMPSRunSteps, err = optionalQueryInt(q.Get("lammps_run_steps"), "lammps_run_steps"); err != nil {
		return app.CalculationPackageRequest{}, err
	}
	if req.GPUMDRunSteps, err = optionalQueryInt(q.Get("gpumd_run_steps"), "gpumd_run_steps"); err != nil {
		return app.CalculationPackageRequest{}, err
	}
	if req.VASPSigma, err = optionalQueryFloat(q.Get("vasp_sigma"), "vasp_sigma"); err != nil {
		return app.CalculationPackageRequest{}, err
	}
	if req.GPUMDTemperatureK, err = optionalQueryFloat(q.Get("gpumd_temperature_k"), "gpumd_temperature_k"); err != nil {
		return app.CalculationPackageRequest{}, err
	}
	return req, nil
}

func optionalQueryInt(raw, field string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", field)
	}
	return value, nil
}

func optionalQueryFloat(raw, field string) (float64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be a number", field)
	}
	return value, nil
}

func (a *api) projectSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Name          string `json:"name"`
		SuggestedName string `json:"suggested_name"`
	}
	if err := decodeStrictJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	data, err := a.state.ExportProjectArchive(req.Name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	suggested := strings.TrimSpace(req.SuggestedName)
	if suggested == "" {
		suggested = "TiAlloyStudio-project.tias-project"
	}
	a.saveBytes(w, saveFileRequest{Format: "tias-project", SuggestedName: suggested, MIME: "application/vnd.tialloystudio.project+zip"}, data)
}

func (a *api) openFolder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Path string `json:"path"`
	}
	if err := decodeStrictJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	path, key, err := normalizedSavedPath(req.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	a.savedMu.Lock()
	allowed := a.savedFiles[key]
	a.savedMu.Unlock()
	if !allowed {
		writeError(w, http.StatusForbidden, fmt.Errorf("the requested path was not created by this Ti Alloy Studio session"))
		return
	}
	if err := a.hooks.openFolder(path); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("open export folder: %w", err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"opened": true, "path": path})
}

func (a *api) saveBytes(w http.ResponseWriter, req saveFileRequest, data []byte) {
	path, cancelled, err := a.hooks.saveFile(req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("choose save path: %w", err))
		return
	}
	if cancelled || strings.TrimSpace(path) == "" {
		writeJSON(w, http.StatusOK, map[string]any{"saved": false, "cancelled": true})
		return
	}
	path, key, err := normalizedSavedPath(path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("write export file: %w", err))
		return
	}
	sum := sha256.Sum256(data)
	a.savedMu.Lock()
	a.savedFiles[key] = true
	a.savedMu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{
		"saved":    true,
		"path":     path,
		"filename": filepath.Base(path),
		"bytes":    int64(len(data)),
		"sha256":   hex.EncodeToString(sum[:]),
	})
}

func normalizedSavedPath(path string) (displayPath, key string, err error) {
	if strings.TrimSpace(path) == "" {
		return "", "", errors.New("path is required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", "", err
	}
	displayPath = filepath.Clean(abs)
	key = displayPath
	if runtime.GOOS == "windows" {
		key = strings.ToLower(key)
	}
	return displayPath, key, nil
}

func nativeOpenFolder(path string) error {
	if runtime.GOOS == "windows" {
		return exec.Command("explorer.exe", "/select,"+path).Start()
	}
	return exec.Command("xdg-open", filepath.Dir(path)).Start()
}

func nativeSaveFile(req saveFileRequest) (string, bool, error) {
	if runtime.GOOS != "windows" {
		return "", false, errors.New("native save dialog is only available in the Windows desktop build")
	}
	script := `
Add-Type -AssemblyName System.Windows.Forms
$dialog = New-Object System.Windows.Forms.SaveFileDialog
$dialog.FileName = ` + psSingleQuote(req.SuggestedName) + `
$dialog.Filter = ` + psSingleQuote(saveDialogFilter(req.Format)) + `
if ($dialog.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK) {
  [Console]::OutputEncoding = [System.Text.UTF8Encoding]::new()
  Write-Output $dialog.FileName
}
`
	out, err := exec.Command("powershell.exe", "-NoProfile", "-STA", "-ExecutionPolicy", "Bypass", "-Command", script).Output()
	if err != nil {
		return "", false, err
	}
	path := strings.TrimSpace(string(out))
	return path, path == "", nil
}

func psSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func saveDialogFilter(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "poscar", "vasp":
		return "VASP POSCAR|POSCAR;*.vasp;*.poscar|All files|*.*"
	case "xyz":
		return "XYZ structure|*.xyz|All files|*.*"
	case "extxyz", "gpumd":
		return "Extended XYZ|*.extxyz;*.xyz|All files|*.*"
	case "lammps", "data":
		return "LAMMPS data|*.data;*.lmp|All files|*.*"
	case "cif":
		return "CIF structure|*.cif|All files|*.*"
	case "tias-project":
		return "Ti Alloy Studio project|*.tias-project|All files|*.*"
	case "zip":
		return "Ti Alloy Studio series package|*.zip|All files|*.*"
	default:
		return "All files|*.*"
	}
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
