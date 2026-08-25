package app

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"tialloystudio/internal/engines"
	"tialloystudio/internal/model"
)

type ExternalRunRecord struct {
	Tool       string `json:"tool"`
	Command    string `json:"command"`
	Distro     string `json:"distro,omitempty"`
	ReturnCode int    `json:"return_code"`
	Stdout     string `json:"stdout,omitempty"`
	Stderr     string `json:"stderr,omitempty"`
	WorkDir    string `json:"work_dir,omitempty"`
}

type BuildRecord struct {
	ID              string                 `json:"id"`
	ParentID        string                 `json:"parent_id,omitempty"`
	CreatedAt       string                 `json:"created_at"`
	Module          string                 `json:"module"`
	Request         BuildRequest           `json:"request"`
	StructureSHA256 string                 `json:"structure_sha256"`
	ExportSHA256    map[string]string      `json:"export_sha256"`
	Validation      model.ValidationReport `json:"validation"`
	Engines         []engines.Report       `json:"engines,omitempty"`
	ExternalRuns    []ExternalRunRecord    `json:"external_runs,omitempty"`
}

type ProjectManifest struct {
	SchemaVersion int           `json:"schema_version"`
	ProjectID     string        `json:"project_uuid"`
	Name          string        `json:"name"`
	CreatedAt     string        `json:"created_at"`
	UpdatedAt     string        `json:"updated_at"`
	History       []BuildRecord `json:"history"`
}

type projectState struct {
	mu       sync.Mutex
	manifest ProjectManifest
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
		SchemaVersion: 1,
		ProjectID:     newRecordID(),
		Name:          "Untitled Project",
		CreatedAt:     now,
		UpdatedAt:     now,
		History:       []BuildRecord{},
	}}
	actual, _ := projectRegistry.LoadOrStore(s, p)
	return actual.(*projectState)
}

func sha256Text(text string) string {
	h := sha256.Sum256([]byte(text))
	return hex.EncodeToString(h[:])
}

func structureSHA256(s model.Structure) string {
	b, _ := json.Marshal(s)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func exportHashes(s model.Structure) map[string]string {
	return map[string]string{
		"poscar": sha256Text(model.ExportPOSCAR(s, "Ti Alloy Studio provenance")),
		"lammps": sha256Text(model.ExportLAMMPS(s)),
		"extxyz": sha256Text(model.ExportExtXYZ(s)),
		"xyz":    sha256Text(model.ExportXYZ(s)),
		"cif":    sha256Text(model.ExportCIF(s)),
	}
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
	parent := ""
	if n := len(p.manifest.History); n > 0 {
		parent = p.manifest.History[n-1].ID
	}
	now := timestampUTC()
	p.manifest.History = append(p.manifest.History, BuildRecord{
		ID:              newRecordID(),
		ParentID:        parent,
		CreatedAt:       now,
		Module:          out.Module,
		Request:         req,
		StructureSHA256: structureSHA256(out.Structure),
		ExportSHA256:    exportHashes(out.Structure),
		Validation:      out.Validation,
		Engines:         out.Engines,
		ExternalRuns:    externalRuns(out),
	})
	p.manifest.UpdatedAt = now
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
		return BuildResponse{}, fmt.Errorf("unsupported project schema version %d", m.SchemaVersion)
	}
	if strings.TrimSpace(m.ProjectID) == "" {
		return BuildResponse{}, errors.New("project_uuid is required")
	}
	if len(m.History) == 0 {
		return BuildResponse{}, errors.New("project history is empty; no model request can be restored")
	}
	if strings.TrimSpace(m.Name) == "" {
		m.Name = "Imported Project"
	}
	if strings.TrimSpace(m.CreatedAt) == "" {
		m.CreatedAt = timestampUTC()
	}
	m.UpdatedAt = timestampUTC()
	projectRegistry.Store(s, &projectState{manifest: cloneManifest(m)})
	latest := m.History[len(m.History)-1].Request
	return s.BuildTracked(latest)
}
