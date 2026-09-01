package app

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	"tialloystudio/internal/engines"
	"tialloystudio/internal/model"
)

type ExternalRunRecord struct {
	Tool         string            `json:"tool"`
	Command      string            `json:"command"`
	Distro       string            `json:"distro,omitempty"`
	Executable   string            `json:"executable,omitempty"`
	Version      string            `json:"version,omitempty"`
	ReturnCode   int               `json:"return_code"`
	Stdout       string            `json:"stdout,omitempty"`
	Stderr       string            `json:"stderr,omitempty"`
	WorkDir      string            `json:"work_dir,omitempty"`
	InputSHA256  map[string]string `json:"input_sha256,omitempty"`
	OutputSHA256 map[string]string `json:"output_sha256,omitempty"`
}

type BuildRecord struct {
	ID              string                       `json:"id"`
	ParentID        string                       `json:"parent_id,omitempty"`
	CreatedAt       string                       `json:"created_at"`
	Module          string                       `json:"module"`
	Request         BuildRequest                 `json:"request"`
	Structure       model.Structure              `json:"structure"`
	StructureSHA256 string                       `json:"structure_sha256"`
	ExportSHA256    map[string]string            `json:"export_sha256"`
	Validation      model.ValidationReport       `json:"validation"`
	Allocation      *model.CompositionAllocation `json:"allocation,omitempty"`
	SQS             *model.SQSQuality            `json:"sqs,omitempty"`
	ATAT            *engines.ATATQuality         `json:"atat,omitempty"`
	Analysis        map[string]any               `json:"analysis,omitempty"`
	Series          map[string]any               `json:"series,omitempty"`
	Engines         []engines.Report             `json:"engines,omitempty"`
	ExternalRuns    []ExternalRunRecord          `json:"external_runs,omitempty"`
	ScientificState string                       `json:"scientific_state"`
}

type ProjectManifest struct {
	SchemaVersion    int           `json:"schema_version"`
	ProjectID        string        `json:"project_uuid"`
	Name             string        `json:"name"`
	CreatedAt        string        `json:"created_at"`
	UpdatedAt        string        `json:"updated_at"`
	ActiveRevisionID string        `json:"active_revision_id,omitempty"`
	History          []BuildRecord `json:"history"`
}

type projectState struct {
	mu       sync.Mutex
	manifest ProjectManifest
}

type DeriveRequest struct {
	Operation  string `json:"operation"`
	SiteID     int    `json:"site_id"`
	NewSpecies string `json:"new_species,omitempty"`
}

var projectRegistry sync.Map

func timestampUTC() string { return time.Now().UTC().Format(time.RFC3339Nano) }

func newRecordID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err == nil {
		b[6] = (b[6] & 0x0f) | 0x40
		b[8] = (b[8] & 0x3f) | 0x80
		return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
			b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
	}
	return fmt.Sprintf("fallback-%d", time.Now().UTC().UnixNano())
}

func stateProject(s *State) *projectState {
	if v, ok := projectRegistry.Load(s); ok {
		return v.(*projectState)
	}
	now := timestampUTC()
	p := &projectState{manifest: ProjectManifest{
		SchemaVersion: 2,
		ProjectID:     newRecordID(),
		Name:          "Untitled Project",
		CreatedAt:     now,
		UpdatedAt:     now,
		History:       []BuildRecord{},
	}}
	actual, _ := projectRegistry.LoadOrStore(s, p)
	return actual.(*projectState)
}

func sha256Bytes(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
func sha256Text(text string) string { return sha256Bytes([]byte(text)) }

func structureSHA256(s model.Structure) string {
	b, _ := json.Marshal(s)
	return sha256Bytes(b)
}

func asInt(v any) int {
	switch x := v.(type) {
	case int:
		return x
	case int32:
		return int(x)
	case int64:
		return int(x)
	case float64:
		return int(x)
	default:
		return 0
	}
}

func hashFileIfPresent(path string) (string, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return sha256Bytes(b), true
}

func matchingATATWorkDir(out BuildResponse) string {
	if out.Module != "sqs" || out.Allocation == nil {
		return ""
	}
	entries, err := os.ReadDir(os.TempDir())
	if err != nil {
		return ""
	}
	wantCounts := out.Structure.SpeciesCounts()
	var best string
	var bestTime time.Time
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "TiAlloyStudio-ATAT-") {
			continue
		}
		dir := filepath.Join(os.TempDir(), entry.Name())
		info, err := entry.Info()
		if err != nil || time.Since(info.ModTime()) > 30*time.Minute {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, "bestsqs.out"))
		if err != nil {
			continue
		}
		parsed, err := engines.ParseATATStructure(string(b))
		if err != nil || parsed.NAtoms() != out.Structure.NAtoms() || !reflect.DeepEqual(parsed.SpeciesCounts(), wantCounts) {
			continue
		}
		if best == "" || info.ModTime().After(bestTime) {
			best, bestTime = dir, info.ModTime()
		}
	}
	return best
}

func attachATATEvidence(out BuildResponse, r *ExternalRunRecord) {
	if !strings.Contains(strings.ToLower(r.Tool), "atat") {
		return
	}
	dir := matchingATATWorkDir(out)
	if dir == "" {
		return
	}
	r.WorkDir = dir
	if b, err := os.ReadFile(filepath.Join(dir, "mcsqs.stdout")); err == nil {
		r.Stdout = string(b)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "mcsqs.stderr")); err == nil {
		r.Stderr = string(b)
	}
	r.InputSHA256 = map[string]string{}
	r.OutputSHA256 = map[string]string{}
	if h, ok := hashFileIfPresent(filepath.Join(dir, "rndstr.in")); ok {
		r.InputSHA256["rndstr.in"] = h
	}
	for _, name := range []string{"bestsqs.out", "bestcorr.out", "mcsqs.stdout", "mcsqs.stderr"} {
		if h, ok := hashFileIfPresent(filepath.Join(dir, name)); ok {
			r.OutputSHA256[name] = h
		}
	}
	distro, _ := out.Analysis["distro"].(string)
	env := engines.DetectEnvironment(distro)
	for _, tool := range env.Tools {
		if tool.Name == "mcsqs" && tool.Status == "AVAILABLE" {
			r.Executable = tool.Path
			if tool.Version != "" {
				r.Version = tool.Version
			} else {
				r.Version = "not reported by mcsqs environment probe"
			}
			break
		}
	}
}

func externalRuns(out BuildResponse) []ExternalRunRecord {
	cmd, _ := out.Analysis["command"].(string)
	if strings.TrimSpace(cmd) == "" {
		return nil
	}
	tool := "external"
	if e, _ := out.Analysis["engine"].(string); e != "" {
		tool = e
	}
	r := ExternalRunRecord{
		Tool:       tool,
		Command:    cmd,
		ReturnCode: asInt(out.Analysis["return_code"]),
	}
	r.Distro, _ = out.Analysis["distro"].(string)
	r.Stdout, _ = out.Analysis["stdout"].(string)
	r.Stderr, _ = out.Analysis["stderr"].(string)
	r.WorkDir, _ = out.Analysis["work_dir"].(string)
	attachATATEvidence(out, &r)
	return []ExternalRunRecord{r}
}

func cloneManifest(m ProjectManifest) ProjectManifest {
	b, _ := json.Marshal(m)
	var out ProjectManifest
	_ = json.Unmarshal(b, &out)
	return out
}

func recordTrackedBuild(s *State, req BuildRequest, out BuildResponse) {
	p := stateProject(s)
	p.mu.Lock()
	defer p.mu.Unlock()
	parent := p.manifest.ActiveRevisionID
	now := timestampUTC()
	p.manifest.History = append(p.manifest.History, BuildRecord{
		ID:              newRecordID(),
		ParentID:        parent,
		CreatedAt:       now,
		Module:          out.Module,
		Request:         req,
		Structure:       out.Structure,
		StructureSHA256: structureSHA256(out.Structure),
		ExportSHA256:    map[string]string{},
		Validation:      out.Validation,
		Allocation:      out.Allocation,
		SQS:             out.SQS,
		ATAT:            out.ATAT,
		Analysis:        out.Analysis,
		Series:          out.Series,
		Engines:         out.Engines,
		ExternalRuns:    externalRuns(out),
		ScientificState: "not_relaxed",
	})
	p.manifest.ActiveRevisionID = p.manifest.History[len(p.manifest.History)-1].ID
	p.manifest.UpdatedAt = now
}

func responseFromRecord(r BuildRecord) BuildResponse {
	return BuildResponse{
		Module: r.Module, Structure: r.Structure, Validation: r.Validation,
		Allocation: r.Allocation, SQS: r.SQS, ATAT: r.ATAT,
		Analysis: r.Analysis, Series: r.Series, Engines: r.Engines,
	}
}

// SelectRevision makes an immutable historical snapshot active without
// rebuilding it or appending to project history.
func (s *State) SelectRevision(id string) (BuildResponse, error) {
	p := stateProject(s)
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, record := range p.manifest.History {
		if record.ID != id {
			continue
		}
		// JSON round-tripping provides a deep copy across the project boundary,
		// including maps and slices nested in the structure and evidence.
		b, _ := json.Marshal(record)
		var cloned BuildRecord
		if err := json.Unmarshal(b, &cloned); err != nil {
			return BuildResponse{}, fmt.Errorf("clone revision %q: %w", id, err)
		}
		out := responseFromRecord(cloned)
		s.mu.Lock()
		s.Current = out
		s.CurrentRequest = cloned.Request
		s.mu.Unlock()
		p.manifest.ActiveRevisionID = id
		p.manifest.UpdatedAt = timestampUTC()
		return out, nil
	}
	return BuildResponse{}, fmt.Errorf("revision %q not found", id)
}

func cloneBuildResponse(in BuildResponse) BuildResponse {
	b, _ := json.Marshal(in)
	var out BuildResponse
	_ = json.Unmarshal(b, &out)
	return out
}

// BuildChild creates a successful new revision under the caller-selected
// parent. Any build failure restores the previously active in-memory model and
// leaves project history untouched.
func (s *State) BuildChild(parentID string, req BuildRequest) (BuildResponse, error) {
	p := stateProject(s)
	p.mu.Lock()
	found := false
	for _, record := range p.manifest.History {
		if record.ID == parentID {
			found = true
			break
		}
	}
	p.mu.Unlock()
	if !found {
		return BuildResponse{}, fmt.Errorf("parent revision %q not found", parentID)
	}

	s.mu.RLock()
	previous := cloneBuildResponse(s.Current)
	previousRequest := s.CurrentRequest
	s.mu.RUnlock()
	out, err := s.BuildUser(req)
	if err != nil {
		s.mu.Lock()
		s.Current = previous
		s.CurrentRequest = previousRequest
		s.mu.Unlock()
		return BuildResponse{}, err
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.manifest.History) == 0 || p.manifest.History[len(p.manifest.History)-1].ID != p.manifest.ActiveRevisionID {
		return BuildResponse{}, errors.New("new child revision was not recorded")
	}
	p.manifest.History[len(p.manifest.History)-1].ParentID = parentID
	return out, nil
}

func (s *State) revisionSnapshot(id string) (BuildRecord, error) {
	p := stateProject(s)
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, record := range p.manifest.History {
		if record.ID != id {
			continue
		}
		b, _ := json.Marshal(record)
		var cloned BuildRecord
		if err := json.Unmarshal(b, &cloned); err != nil {
			return BuildRecord{}, fmt.Errorf("clone revision %q: %w", id, err)
		}
		return cloned, nil
	}
	return BuildRecord{}, fmt.Errorf("revision %q not found", id)
}

func (s *State) rememberActiveExportHash(format, content string) {
	p := stateProject(s)
	p.mu.Lock()
	id := p.manifest.ActiveRevisionID
	p.mu.Unlock()
	if id != "" {
		s.rememberRevisionExportHash(id, format, content)
	}
}

func (s *State) rememberRevisionExportHash(id, format, content string) {
	key, err := canonicalExportFormat(format)
	if err != nil {
		return
	}
	p := stateProject(s)
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := range p.manifest.History {
		if p.manifest.History[i].ID != id {
			continue
		}
		if p.manifest.History[i].ExportSHA256 == nil {
			p.manifest.History[i].ExportSHA256 = map[string]string{}
		}
		p.manifest.History[i].ExportSHA256[key] = sha256Text(content)
		p.manifest.UpdatedAt = timestampUTC()
		return
	}
}

// DeriveRevision applies a local defect operation to the exact selected
// structure snapshot. It intentionally does not reconstruct a pristine host.
func (s *State) DeriveRevision(parentID string, change DeriveRequest) (BuildResponse, error) {
	parent, err := s.revisionSnapshot(parentID)
	if err != nil {
		return BuildResponse{}, err
	}
	operation := strings.ToLower(strings.TrimSpace(change.Operation))
	var structure model.Structure
	switch operation {
	case "vacancy":
		structure, err = model.CreateVacancy(parent.Structure, change.SiteID)
	case "substitution":
		structure, err = model.CreateSubstitution(parent.Structure, change.SiteID, strings.TrimSpace(change.NewSpecies))
	default:
		return BuildResponse{}, fmt.Errorf("unsupported derivation %q; choose vacancy or substitution", change.Operation)
	}
	if err != nil {
		return BuildResponse{}, fmt.Errorf("derive %s from revision %q: %w", operation, parentID, err)
	}

	req := parent.Request
	req.Module = operation
	req.SiteID = change.SiteID
	req.NewSpecies = strings.TrimSpace(change.NewSpecies)
	out := BuildResponse{
		Module: operation, Structure: structure,
		Analysis: map[string]any{"site_id": change.SiteID, "derived_from_revision": parentID},
		Series:   map[string]any{},
	}
	if operation == "substitution" {
		out.Analysis["new_species"] = req.NewSpecies
	}
	req.ValidationMode = normalizeValidationMode(req.ValidationMode)
	finalizeValidation(&out, req.ValidationMode)

	s.mu.Lock()
	s.Current = cloneBuildResponse(out)
	s.CurrentRequest = req
	s.mu.Unlock()
	recordTrackedBuild(s, req, out)
	p := stateProject(s)
	p.mu.Lock()
	p.manifest.History[len(p.manifest.History)-1].ParentID = parentID
	p.mu.Unlock()
	return cloneBuildResponse(out), nil
}

func gsfeSeriesForRequest(req BuildRequest) model.GSFESeries {
	if req.Phase == "beta" {
		return model.BetaGSFE(req.ABeta, [3]int{req.NX, req.NY, req.NZ}, req.GSFESteps, 0.5)
	}
	return model.AlphaGSFE(req.GSFEPreset, req.AAlpha, req.CAlpha, [3]int{req.NX, req.NY, req.NZ}, req.GSFESteps, 0.5)
}

func enrichTrackedDiagnostics(req BuildRequest, out *BuildResponse) {
	if out.Module != "gsfe" {
		return
	}
	d := model.AnalyzeGSFESeries(gsfeSeriesForRequest(req))
	out.Analysis["series_point_count"] = d.PointCount
	out.Analysis["series_atom_count_consistent"] = d.AtomCountConsistent
	out.Analysis["series_cell_consistent"] = d.CellConsistent
	out.Analysis["series_pbc_consistent"] = d.PBCConsistent
	out.Analysis["series_lambda_monotonic"] = d.LambdaMonotonic
	out.Analysis["series_minimum_distance_angstrom"] = d.MinimumDistanceAngstrom
	out.Analysis["fault_separation_angstrom"] = d.FaultSeparationAngstrom
	out.Analysis["endpoint_lattice_equivalent"] = d.EndpointEquivalent

	if d.AtomCountConsistent && d.CellConsistent && d.PBCConsistent {
		addCheck(&out.Validation, "gsfe_series_topology", "PASS", "All GSFE points preserve atom count, simulation cell and periodic-boundary topology", float64(d.PointCount))
	} else {
		addCheck(&out.Validation, "gsfe_series_topology", "FAIL", "Atom count, cell or PBC changed within the GSFE series", float64(d.PointCount))
	}
	if d.LambdaMonotonic && math.Abs(d.LambdaStart) < 1e-12 && math.Abs(d.LambdaEnd-1) < 1e-12 {
		addCheck(&out.Validation, "gsfe_lambda_path", "PASS", "GSFE displacement parameter is monotonic and spans λ = 0 to 1", d.LambdaEnd)
	} else {
		addCheck(&out.Validation, "gsfe_lambda_path", "FAIL", "GSFE displacement parameter must monotonically span λ = 0 to 1", d.LambdaEnd)
	}
	if d.EndpointEquivalent {
		addCheck(&out.Validation, "gsfe_endpoint_equivalence", "PASS", "Preset full-slip endpoint is lattice-equivalent to the reference modulo PBC", 1)
	} else {
		addCheck(&out.Validation, "gsfe_endpoint_equivalence", "FAIL", "Preset full-slip endpoint is not lattice-equivalent to the reference; inspect slip path or supercell", 0)
	}
	if !math.IsNaN(d.MinimumDistanceAngstrom) && !math.IsInf(d.MinimumDistanceAngstrom, 0) && d.MinimumDistanceAngstrom > 1e-5 {
		addCheck(&out.Validation, "gsfe_series_minimum_distance", "PASS", "Minimum interatomic distance over the complete rigid-shift series is finite and non-overlapping", d.MinimumDistanceAngstrom)
	} else {
		addCheck(&out.Validation, "gsfe_series_minimum_distance", "FAIL", "At least one GSFE displacement contains duplicate or numerically overlapping atoms", d.MinimumDistanceAngstrom)
	}
	if d.FaultSeparationAngstrom > 0 {
		addCheck(&out.Validation, "gsfe_fault_separation", "PASS", "Geometric separation between periodic fault images is reported for thickness-convergence studies; no universal converged thickness is imposed", d.FaultSeparationAngstrom)
	}
}

// BuildTracked is the user-facing build path. The pure Build method remains
// useful for low-level scientific tests; GUI/API builds use this method so a
// successful generation always appends a reproducibility record.
func (s *State) BuildTracked(req BuildRequest) (BuildResponse, error) {
	out, err := s.Build(req)
	if err != nil {
		return out, err
	}
	s.mu.RLock()
	normalized := s.CurrentRequest
	s.mu.RUnlock()
	enrichTrackedDiagnostics(normalized, &out)
	// Replace the pure-build current response with the enriched user-facing
	// response before provenance hashing/serialization.
	s.mu.Lock()
	s.Current = out
	s.CurrentRequest = normalized
	s.mu.Unlock()
	recordTrackedBuild(s, normalized, out)
	return out, nil
}

func (s *State) ProjectManifest(name string) ProjectManifest {
	p := stateProject(s)
	p.mu.Lock()
	defer p.mu.Unlock()
	if strings.TrimSpace(name) != "" && strings.TrimSpace(name) != p.manifest.Name {
		p.manifest.Name = strings.TrimSpace(name)
		p.manifest.UpdatedAt = timestampUTC()
	}
	return cloneManifest(p.manifest)
}

func (s *State) ImportProject(m ProjectManifest) (BuildResponse, error) {
	if m.SchemaVersion != 1 {
		return BuildResponse{}, fmt.Errorf("unsupported legacy project schema version %d; use ImportProjectArchive for schema 2", m.SchemaVersion)
	}
	if strings.TrimSpace(m.ProjectID) == "" {
		return BuildResponse{}, errors.New("project_uuid is required")
	}
	if len(m.History) == 0 {
		return BuildResponse{}, errors.New("project history is empty; no model request can be restored")
	}
	seen := map[string]bool{}
	temp := NewState()
	defer projectRegistry.Delete(temp)
	rebuilt := make([]BuildRecord, 0, len(m.History))
	for i, legacy := range m.History {
		if strings.TrimSpace(legacy.ID) == "" || seen[legacy.ID] {
			return BuildResponse{}, fmt.Errorf("legacy revision %d has duplicate or empty id %q", i, legacy.ID)
		}
		if legacy.ParentID != "" && !seen[legacy.ParentID] {
			return BuildResponse{}, fmt.Errorf("legacy revision %q has unknown or forward parent %q", legacy.ID, legacy.ParentID)
		}
		out, err := temp.BuildUser(legacy.Request)
		if err != nil {
			return BuildResponse{}, fmt.Errorf("rebuild legacy revision %q: %w", legacy.ID, err)
		}
		gotHash := structureSHA256(out.Structure)
		if legacy.StructureSHA256 == "" || gotHash != legacy.StructureSHA256 {
			return BuildResponse{}, fmt.Errorf("legacy revision %q structure SHA-256 mismatch: recorded %q rebuilt %q", legacy.ID, legacy.StructureSHA256, gotHash)
		}
		tm := temp.ProjectManifest("")
		record := tm.History[len(tm.History)-1]
		record.ID = legacy.ID
		record.ParentID = legacy.ParentID
		if strings.TrimSpace(legacy.CreatedAt) != "" {
			record.CreatedAt = legacy.CreatedAt
		}
		rebuilt = append(rebuilt, record)
		seen[record.ID] = true
	}
	name := strings.TrimSpace(m.Name)
	if name == "" {
		name = "Imported Project"
	}
	created := m.CreatedAt
	if strings.TrimSpace(created) == "" {
		created = timestampUTC()
	}
	active := rebuilt[len(rebuilt)-1]
	converted := ProjectManifest{
		SchemaVersion: 2, ProjectID: m.ProjectID, Name: name,
		CreatedAt: created, UpdatedAt: timestampUTC(),
		ActiveRevisionID: active.ID, History: rebuilt,
	}
	out := responseFromRecord(active)
	projectRegistry.Store(s, &projectState{manifest: cloneManifest(converted)})
	s.mu.Lock()
	s.Current = cloneBuildResponse(out)
	s.CurrentRequest = active.Request
	s.mu.Unlock()
	return cloneBuildResponse(out), nil
}
