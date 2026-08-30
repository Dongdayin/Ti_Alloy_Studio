package app

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"

	"tialloystudio/internal/engines"
	"tialloystudio/internal/model"
)

type BuildRequest struct {
	Module                 string             `json:"module"`
	AlloyMode              string             `json:"alloy_mode,omitempty"`
	Phase                  string             `json:"phase"`
	NX                     int                `json:"nx"`
	NY                     int                `json:"ny"`
	NZ                     int                `json:"nz"`
	TargetX                float64            `json:"target_x"`
	TargetY                float64            `json:"target_y"`
	TargetZ                float64            `json:"target_z"`
	AAlpha                 float64            `json:"a_alpha"`
	CAlpha                 float64            `json:"c_alpha"`
	ABeta                  float64            `json:"a_beta"`
	CompositionWt          map[string]float64 `json:"composition_wt"`
	Seed                   int64              `json:"seed"`
	SQSBackend             string             `json:"sqs_backend"`
	SQSSteps               int                `json:"sqs_steps"`
	SQSShells              int                `json:"sqs_shells"`
	ATATDistro             string             `json:"atat_distro"`
	ATATPairCutoff         float64            `json:"atat_pair_cutoff_angstrom"`
	ATATTripletCutoff      float64            `json:"atat_triplet_cutoff_angstrom"`
	ATATRunSeconds         int                `json:"atat_run_seconds"`
	SiteID                 int                `json:"site_id"`
	NewSpecies             string             `json:"new_species"`
	SurfacePreset          string             `json:"surface_preset"`
	Vacuum                 float64            `json:"vacuum"`
	InterfaceMaxRepeat     int                `json:"interface_max_repeat"`
	InterfaceCandidate     int                `json:"interface_candidate"`
	InterfaceDistance      float64            `json:"interface_distance"`
	EOSRatios              []float64          `json:"eos_ratios"`
	EOSIndex               int                `json:"eos_index"`
	GSFEPreset             string             `json:"gsfe_preset"`
	GSFESteps              int                `json:"gsfe_steps"`
	GSFEIndex              int                `json:"gsfe_index"`
	OperationKind          string             `json:"operation_kind,omitempty"`
	OrientationPreset      string             `json:"orientation_preset,omitempty"`
	SlipSystem             string             `json:"slip_system,omitempty"`
	BurgersVector          string             `json:"burgers_vector,omitempty"`
	LineDirection          string             `json:"line_direction,omitempty"`
	DislocationCharacter   string             `json:"dislocation_character,omitempty"`
	DislocationArrangement string             `json:"dislocation_arrangement,omitempty"`
	GBType                 string             `json:"gb_type,omitempty"`
	GBAxis                 string             `json:"gb_axis,omitempty"`
	GBNormal               string             `json:"gb_normal,omitempty"`
	Grain1Orientation      string             `json:"grain_1_orientation,omitempty"`
	Grain2Orientation      string             `json:"grain_2_orientation,omitempty"`
	GBAngleDeg             float64            `json:"gb_angle_deg,omitempty"`
	OverlapCutoff          float64            `json:"overlap_cutoff_angstrom,omitempty"`
	TwinSystem             string             `json:"twin_system,omitempty"`
	ClusterSpec            string             `json:"cluster_spec,omitempty"`
	PrecipitateSpec        string             `json:"precipitate_spec,omitempty"`
	CrackSpec              string             `json:"crack_spec,omitempty"`
	IndenterSpec           string             `json:"indenter_spec,omitempty"`
	GrainCount             int                `json:"grain_count,omitempty"`
	SeriesCount            int                `json:"series_count,omitempty"`
	DatasetKind            string             `json:"dataset_kind,omitempty"`
}

type BuildResponse struct {
	Module     string                       `json:"module"`
	Structure  model.Structure              `json:"structure"`
	Validation model.ValidationReport       `json:"validation"`
	Allocation *model.CompositionAllocation `json:"allocation,omitempty"`
	SQS        *model.SQSQuality            `json:"sqs,omitempty"`
	ATAT       *engines.ATATQuality         `json:"atat,omitempty"`
	Analysis   map[string]any               `json:"analysis,omitempty"`
	Series     map[string]any               `json:"series,omitempty"`
	Engines    []engines.Report             `json:"engines,omitempty"`
}

type State struct {
	mu             sync.RWMutex
	Current        BuildResponse
	CurrentRequest BuildRequest
}

func NewState() *State { return &State{} }

func defaults(req *BuildRequest) {
	req.Module = strings.ToLower(strings.TrimSpace(req.Module))
	if req.Module == "" {
		req.Module = "random"
	}
	req.AlloyMode = strings.ToLower(strings.TrimSpace(req.AlloyMode))
	if req.AlloyMode == "" {
		switch req.Module {
		case "crystal", "random", "sqs":
			req.AlloyMode = req.Module
		default:
			req.AlloyMode = "crystal"
		}
	}
	if req.AlloyMode == "pure" {
		req.AlloyMode = "crystal"
	}
	if req.Phase == "" {
		req.Phase = "alpha"
	}
	req.Phase = strings.ToLower(req.Phase)
	if req.NX < 1 {
		req.NX = 2
	}
	if req.NY < 1 {
		req.NY = 2
	}
	if req.NZ < 1 {
		req.NZ = 2
	}
	if req.AAlpha <= 0 {
		req.AAlpha = 2.951
	}
	if req.CAlpha <= 0 {
		req.CAlpha = 4.684
	}
	if req.ABeta <= 0 {
		req.ABeta = 3.306
	}
	if req.CompositionWt == nil || len(req.CompositionWt) == 0 {
		req.CompositionWt = map[string]float64{"Al": 6, "V": 4}
	}
	if req.Seed == 0 {
		req.Seed = 20260825
	}
	if req.SQSBackend == "" {
		req.SQSBackend = "native"
	}
	req.SQSBackend = strings.ToLower(strings.TrimSpace(req.SQSBackend))
	if req.SQSBackend == "preview" {
		req.SQSBackend = "native"
	}
	if req.SQSSteps < 1 {
		req.SQSSteps = 500
	}
	if req.SQSShells < 1 {
		req.SQSShells = 2
	}
	if req.ATATRunSeconds < 1 {
		req.ATATRunSeconds = 30
	}
	if req.NewSpecies == "" {
		req.NewSpecies = "Al"
	}
	if req.SurfacePreset == "" {
		if req.Phase == "beta" {
			req.SurfacePreset = "100"
		} else {
			req.SurfacePreset = "basal_0001"
		}
	}
	if req.Vacuum <= 0 {
		req.Vacuum = 15
	}
	if req.InterfaceMaxRepeat < 1 {
		req.InterfaceMaxRepeat = 8
	}
	if req.InterfaceDistance <= 0 {
		req.InterfaceDistance = 2.5
	}
	if len(req.EOSRatios) == 0 {
		req.EOSRatios = []float64{.94, .96, .98, 1, 1.02, 1.04, 1.06}
	}
	if req.GSFESteps < 1 {
		req.GSFESteps = 10
	}
	if req.GSFEPreset == "" {
		if req.Phase == "beta" {
			req.GSFEPreset = "110_111"
		} else {
			req.GSFEPreset = "basal_a"
		}
	}
	req.OperationKind = strings.ToLower(strings.TrimSpace(req.OperationKind))
	req.OrientationPreset = strings.ToLower(strings.TrimSpace(req.OrientationPreset))
	req.SlipSystem = strings.ToLower(strings.TrimSpace(req.SlipSystem))
	if req.SlipSystem == "" {
		if req.Phase == "beta" {
			req.SlipSystem = "beta_110_111"
		} else {
			req.SlipSystem = "alpha_basal_a"
		}
	}
	req.DislocationCharacter = strings.ToLower(strings.TrimSpace(req.DislocationCharacter))
	if req.DislocationCharacter == "" {
		line := strings.ToLower(strings.TrimSpace(req.LineDirection))
		if line == "screw" || line == "edge" || line == "mixed" {
			req.DislocationCharacter = line
		} else {
			req.DislocationCharacter = "screw"
		}
	}
	req.DislocationArrangement = strings.ToLower(strings.TrimSpace(req.DislocationArrangement))
	if req.DislocationArrangement == "" {
		req.DislocationArrangement = "single"
	}
	req.GBType = strings.ToLower(strings.TrimSpace(req.GBType))
	if req.GBType == "" {
		req.GBType = "tilt"
	}
	if req.GBAxis == "" {
		req.GBAxis = "[001]"
	}
	if req.GBNormal == "" {
		req.GBNormal = "[100]"
	}
	req.Grain1Orientation = strings.TrimSpace(req.Grain1Orientation)
	req.Grain2Orientation = strings.TrimSpace(req.Grain2Orientation)
	if req.GBAngleDeg == 0 {
		req.GBAngleDeg = 10
	}
	if req.OverlapCutoff <= 0 {
		req.OverlapCutoff = 1.2
	}
	if req.TwinSystem == "" {
		req.TwinSystem = "alpha_10-12"
	}
	if req.GrainCount < 1 {
		req.GrainCount = 4
	}
	if req.SeriesCount < 1 {
		req.SeriesCount = 4
	}
	req.DatasetKind = strings.ToLower(strings.TrimSpace(req.DatasetKind))
	if req.DatasetKind == "" {
		req.DatasetKind = "nep"
	}
}

func buildBase(req BuildRequest) (model.Structure, error) {
	switch req.Phase {
	case "alpha":
		return model.BuildAlphaTi(req.AAlpha, req.CAlpha), nil
	case "beta":
		return model.BuildBetaTi(req.ABeta), nil
	default:
		return model.Structure{}, fmt.Errorf("unsupported phase %q", req.Phase)
	}
}

func buildHost(req BuildRequest) (model.Structure, error) {
	base, err := buildBase(req)
	if err != nil {
		return model.Structure{}, err
	}
	nx, ny, nz := req.NX, req.NY, req.NZ
	if req.TargetX > 0 {
		nx = int(math.Ceil(req.TargetX / model.Norm(base.Cell[0])))
	}
	if req.TargetY > 0 {
		ny = int(math.Ceil(req.TargetY / model.Norm(base.Cell[1])))
	}
	if req.TargetZ > 0 {
		nz = int(math.Ceil(req.TargetZ / model.Norm(base.Cell[2])))
	}
	if nx < 1 {
		nx = 1
	}
	if ny < 1 {
		ny = 1
	}
	if nz < 1 {
		nz = 1
	}
	out := base.Repeat(nx, ny, nz)
	if req.TargetX > 0 || req.TargetY > 0 || req.TargetZ > 0 {
		out.Meta["target_size_angstrom"] = []float64{req.TargetX, req.TargetY, req.TargetZ}
	}
	return out, nil
}

func allocationFor(host model.Structure, req BuildRequest) (*model.CompositionAllocation, error) {
	if host.NAtoms() < 1 {
		return nil, errors.New("no substitutable lattice sites are available")
	}
	t, err := model.FromWeightPercent(req.CompositionWt, "Ti")
	if err != nil {
		return nil, err
	}
	a := model.AllocateIntegerCounts(t, host.NAtoms(), true)
	return &a, nil
}

func applyTitaniumAlloyMode(host model.Structure, req BuildRequest, mode string, out *BuildResponse, allowATAT bool) error {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" || mode == "crystal" || mode == "pure" {
		out.Structure = host
		if out.Structure.Meta == nil {
			out.Structure.Meta = map[string]any{}
		}
		out.Structure.Meta["titanium_alloy_mode"] = "crystal"
		return nil
	}

	alloc, err := allocationFor(host, req)
	if err != nil {
		return err
	}
	out.Allocation = alloc
	out.Analysis["seed"] = req.Seed
	out.Analysis["composition_resolution_at_percent"] = alloc.ResolutionAtPercent
	out.Analysis["rms_atomic_percent_error"] = alloc.RMSAtomicPercentError
	out.Analysis["rms_weight_percent_error"] = alloc.RMSWeightPercentError

	switch mode {
	case "random":
		out.Structure = model.RandomSubstitution(host, *alloc, req.Seed)
		out.Structure.Meta["titanium_alloy_mode"] = "random"
		return nil

	case "sqs":
		switch req.SQSBackend {
		case "native":
			r, err := model.GenerateSQS(host, *alloc, req.Seed, req.SQSShells, req.SQSSteps, 1e-5)
			if err != nil {
				return err
			}
			out.Structure = r.Structure
			out.Structure.Meta["titanium_alloy_mode"] = "sqs"
			out.Structure.Meta["sqs_engine"] = "TiModelCore pair/triplet correlation SQS"
			out.Structure.Meta["sqs_backend"] = "native"
			out.Structure.Meta["sqs_scope"] = "selected neighbor-shell pair probabilities and closed triplet probability geometries"
			out.SQS = &r.Quality
			out.Analysis["initial_objective"] = r.InitialObjective
			out.Analysis["objective"] = r.Quality.Objective
			out.Analysis["max_abs_pair_error"] = r.Quality.MaxAbsPairError
			out.Analysis["max_abs_triplet_error"] = r.Quality.MaxAbsTripletError
			out.Analysis["engine"] = "TiModelCore pair/triplet correlation SQS"
			out.Analysis["scope"] = "pair and triplet occupation-probability residuals for bounded internal geometries; not claimed equivalent to ATAT basis-cluster correlations"
			out.Analysis["verification_status"] = r.Quality.VerificationStatus
			out.Series["convergence"] = r.Convergence
			out.Series["triplet_correlations"] = r.Quality.TripletClusters
			return nil

		case "atat":
			if !allowATAT {
				return errors.New("ATAT SQS is only available for a base SQS alloy model; operation models use the bundled SQS generator")
			}
			if req.ATATPairCutoff <= 0 {
				return errors.New("ATAT pair cutoff must be explicitly specified in angstrom; Ti Alloy Studio does not guess an SQS cluster cutoff")
			}
			parent, err := buildBase(req)
			if err != nil {
				return err
			}
			r, err := engines.RunATATSQS(parent, alloc.ActualAtomicPercent, engines.ATATOptions{
				Distro:        req.ATATDistro,
				TotalSites:    alloc.TotalSites,
				PairCutoff:    req.ATATPairCutoff,
				TripletCutoff: req.ATATTripletCutoff,
				RunSeconds:    req.ATATRunSeconds,
			})
			if err != nil {
				return err
			}
			if r.Structure.NAtoms() != alloc.TotalSites {
				return fmt.Errorf("ATAT bestsqs atom count %d does not match requested integer site count %d", r.Structure.NAtoms(), alloc.TotalSites)
			}
			out.Structure = r.Structure
			out.ATAT = &r.Quality
			out.Analysis["engine"] = "ATAT mcsqs"
			out.Analysis["command"] = r.Command
			out.Analysis["distro"] = r.Distro
			out.Analysis["return_code"] = r.ReturnCode
			out.Analysis["rms_correlation_difference"] = r.Quality.RMSDifference
			out.Analysis["max_abs_correlation_difference"] = r.Quality.MaxAbsDifference
			out.Analysis["pair_cutoff_angstrom"] = req.ATATPairCutoff
			out.Analysis["triplet_cutoff_angstrom"] = req.ATATTripletCutoff
			out.Analysis["run_seconds"] = req.ATATRunSeconds
			out.Series["correlations"] = r.Quality.Clusters
			return nil

		default:
			return fmt.Errorf("unsupported SQS backend %q; choose \"native\" or \"atat\"", req.SQSBackend)
		}
	default:
		return fmt.Errorf("unsupported titanium alloy mode %q", mode)
	}
}

func clampIndex(i, n int) int {
	if n <= 0 || i < 0 {
		return 0
	}
	if i >= n {
		return n - 1
	}
	return i
}

func buildConfiguredHost(req BuildRequest, out *BuildResponse) error {
	host, err := buildHost(req)
	if err != nil {
		return err
	}
	return applyTitaniumAlloyMode(host, req, req.AlloyMode, out, false)
}

func parseColonCountRegion(spec, fallbackElement string, fallbackCount int) (element string, count int, region string) {
	element = fallbackElement
	if strings.TrimSpace(element) == "" {
		element = "Al"
	}
	count = fallbackCount
	region = "center"
	parts := strings.Split(strings.TrimSpace(spec), ":")
	if len(parts) > 0 && strings.TrimSpace(parts[0]) != "" {
		element = strings.TrimSpace(parts[0])
	}
	if len(parts) > 1 {
		var parsed int
		if _, err := fmt.Sscanf(strings.TrimSpace(parts[1]), "%d", &parsed); err == nil && parsed > 0 {
			count = parsed
		}
	}
	if len(parts) > 2 && strings.TrimSpace(parts[2]) != "" {
		region = strings.TrimSpace(parts[2])
	}
	if count < 1 {
		count = 1
	}
	return element, count, region
}

func parseValueFromSpec(spec, key string, fallback float64) float64 {
	spec = strings.ReplaceAll(spec, ";", ",")
	for _, field := range strings.Split(spec, ",") {
		field = strings.TrimSpace(field)
		if !strings.Contains(field, "=") {
			continue
		}
		parts := strings.SplitN(field, "=", 2)
		if strings.EqualFold(strings.TrimSpace(parts[0]), key) {
			var v float64
			if _, err := fmt.Sscanf(strings.TrimSpace(parts[1]), "%f", &v); err == nil {
				return v
			}
		}
	}
	return fallback
}

func parseStringFromSpec(spec, key, fallback string) string {
	spec = strings.ReplaceAll(spec, ";", ",")
	for _, field := range strings.Split(spec, ",") {
		field = strings.TrimSpace(field)
		if !strings.Contains(field, "=") {
			continue
		}
		parts := strings.SplitN(field, "=", 2)
		if strings.EqualFold(strings.TrimSpace(parts[0]), key) {
			if v := strings.TrimSpace(parts[1]); v != "" {
				return v
			}
		}
	}
	return fallback
}

func parsedVectorOrZero(spec string) model.Vec3 {
	v, ok := model.VectorFromSpec(spec)
	if !ok {
		return model.Vec3{}
	}
	return v
}

func appDefaultString(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return strings.TrimSpace(v)
}

type phase2SeriesEntry struct {
	Kind      string
	Index     int
	Lambda    float64
	Shift     model.Vec3
	Structure model.Structure
}

func phase2SeriesForRequest(req BuildRequest) ([]phase2SeriesEntry, error) {
	defaults(&req)
	host, err := buildHost(req)
	if err != nil {
		return nil, err
	}
	var baseOut BuildResponse
	baseOut.Analysis = map[string]any{}
	baseOut.Series = map[string]any{}
	if err = applyTitaniumAlloyMode(host, req, req.AlloyMode, &baseOut, false); err != nil {
		return nil, err
	}
	host = baseOut.Structure
	switch req.Module {
	case "stacking_fault", "gamma_surface":
		series, err := model.GenerateFaultSeries(host, model.FaultSeriesOptions{
			Preset:     req.GSFEPreset,
			Steps:      req.SeriesCount,
			Cut:        0.5,
			NormalAxis: 2,
		})
		if err != nil {
			return nil, err
		}
		out := make([]phase2SeriesEntry, 0, len(series.Points))
		for _, p := range series.Points {
			out = append(out, phase2SeriesEntry{Kind: "stacking_fault", Index: p.Index, Lambda: p.Lambda, Shift: p.Shift, Structure: p.Structure})
		}
		return out, nil
	case "neb":
		neb, err := model.GenerateNEBSeries(host, model.NEBOptions{MovingSite: clampIndex(req.SiteID, host.NAtoms()), Images: req.SeriesCount})
		if err != nil {
			return nil, err
		}
		out := make([]phase2SeriesEntry, 0, len(neb.Points))
		for _, p := range neb.Points {
			out = append(out, phase2SeriesEntry{Kind: "neb", Index: p.Index, Lambda: p.Lambda, Structure: p.Structure})
		}
		return out, nil
	case "training_set":
		neb, err := model.GenerateNEBSeries(host, model.NEBOptions{MovingSite: 0, Images: req.SeriesCount})
		if err != nil {
			return nil, err
		}
		structures := []model.Structure{host}
		for _, p := range neb.Points {
			structures = append(structures, p.Structure)
		}
		dataset := model.BuildTrainingSet(structures, model.DatasetOptions{Kind: req.DatasetKind, Name: "TiAlloyStudio-phase2"})
		out := make([]phase2SeriesEntry, 0, len(dataset.Structures))
		for i, s := range dataset.Structures {
			out = append(out, phase2SeriesEntry{Kind: "training_set", Index: i, Lambda: -1, Structure: s})
		}
		return out, nil
	default:
		return []phase2SeriesEntry{{Kind: req.Module, Index: 0, Lambda: -1, Structure: host}}, nil
	}
}

func phase2BatchModule(module string) bool {
	switch module {
	case "stacking_fault", "gamma_surface", "neb", "training_set":
		return true
	default:
		return false
	}
}

func exportPhase2SeriesArchive(module, format string, entries []phase2SeriesEntry) (filename, mime string, content []byte, err error) {
	if len(entries) == 0 {
		return "", "", nil, errors.New("no structures in series")
	}
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" || format == "vasp" {
		format = "poscar"
	}
	if format != "poscar" && format != "extxyz" {
		return "", "", nil, fmt.Errorf("unsupported Phase 2 series format %q", format)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	var manifest strings.Builder
	manifest.WriteString("index,kind,lambda,shift_x_angstrom,shift_y_angstrom,shift_z_angstrom,atoms,pbc,path\n")
	for _, entry := range entries {
		dir := fmt.Sprintf("%s_%03d", strings.ReplaceAll(entry.Kind, " ", "_"), entry.Index)
		if entry.Lambda >= 0 {
			dir = fmt.Sprintf("%s_%03d_lambda%.5f", strings.ReplaceAll(entry.Kind, " ", "_"), entry.Index, entry.Lambda)
		}
		path := dir + "/POSCAR"
		payload := model.ExportPOSCAR(entry.Structure, fmt.Sprintf("Ti Alloy Studio %s geometry %03d", entry.Kind, entry.Index))
		if format == "extxyz" {
			path = dir + "/model.extxyz"
			payload = model.ExportExtXYZ(entry.Structure)
		}
		f, e := zw.Create(path)
		if e != nil {
			return "", "", nil, e
		}
		if _, e = f.Write([]byte(payload)); e != nil {
			return "", "", nil, e
		}
		pbc := fmt.Sprintf("%t/%t/%t", entry.Structure.PBC[0], entry.Structure.PBC[1], entry.Structure.PBC[2])
		fmt.Fprintf(&manifest, "%d,%s,%.10g,%.12g,%.12g,%.12g,%d,%s,%s\n",
			entry.Index, entry.Kind, entry.Lambda, entry.Shift[0], entry.Shift[1], entry.Shift[2], entry.Structure.NAtoms(), pbc, path)
	}
	mf, e := zw.Create("manifest.csv")
	if e != nil {
		return "", "", nil, e
	}
	if _, e = mf.Write([]byte(manifest.String())); e != nil {
		return "", "", nil, e
	}
	readme, e := zw.Create("README.txt")
	if e != nil {
		return "", "", nil, e
	}
	fmt.Fprintf(readme, "Ti Alloy Studio Phase 2 geometry series\r\nModule: %s\r\nStructures: %d\r\nAll structures are initial geometry candidates with not_relaxed and not_calculated metadata.\r\n", module, len(entries))
	if e = zw.Close(); e != nil {
		return "", "", nil, e
	}
	label := "POSCAR"
	if format == "extxyz" {
		label = "extXYZ"
	}
	return "TiAlloyStudio-Phase2-Geometry-Series-" + label + ".zip", "application/zip", buf.Bytes(), nil
}

func (s *State) Build(in BuildRequest) (BuildResponse, error) {
	req := in
	defaults(&req)
	var out BuildResponse
	out.Module = strings.ToLower(req.Module)
	out.Analysis = map[string]any{}
	out.Series = map[string]any{}
	switch out.Module {
	case "crystal", "random", "sqs":
		host, err := buildHost(req)
		if err != nil {
			return out, err
		}
		if err := applyTitaniumAlloyMode(host, req, out.Module, &out, true); err != nil {
			return out, err
		}

	case "vacancy":
		host, err := buildHost(req)
		if err != nil {
			return out, err
		}
		if err := applyTitaniumAlloyMode(host, req, req.AlloyMode, &out, false); err != nil {
			return out, err
		}
		idx := clampIndex(req.SiteID, out.Structure.NAtoms())
		out.Structure, err = model.CreateVacancy(out.Structure, idx)
		if err != nil {
			return out, err
		}
		out.Structure.Meta["operation"] = "vacancy"
		out.Structure.Meta["titanium_alloy_mode"] = req.AlloyMode
		out.Analysis["site_id"] = idx

	case "substitution":
		host, err := buildHost(req)
		if err != nil {
			return out, err
		}
		if err := applyTitaniumAlloyMode(host, req, req.AlloyMode, &out, false); err != nil {
			return out, err
		}
		idx := clampIndex(req.SiteID, out.Structure.NAtoms())
		out.Structure, err = model.CreateSubstitution(out.Structure, idx, req.NewSpecies)
		if err != nil {
			return out, err
		}
		out.Structure.Meta["operation"] = "substitution"
		out.Structure.Meta["titanium_alloy_mode"] = req.AlloyMode
		out.Analysis["site_id"] = idx
		out.Analysis["new_species"] = req.NewSpecies

	case "surface":
		var surf model.SurfaceModel
		if req.Phase == "beta" {
			surf = model.BetaSurface100(req.ABeta, [2]int{req.NX, req.NY}, req.NZ, req.Vacuum)
		} else {
			surf = model.AlphaSurface(req.SurfacePreset, req.AAlpha, req.CAlpha, [2]int{req.NX, req.NY}, req.NZ, req.Vacuum)
		}
		if err := applyTitaniumAlloyMode(surf.Structure, req, req.AlloyMode, &out, false); err != nil {
			return out, err
		}
		out.Structure.Meta["operation"] = "surface"
		out.Structure.Meta["model_kind"] = "titanium_alloy_surface"
		out.Analysis["plane"] = surf.Plane
		out.Analysis["area_angstrom2"] = surf.Area
		out.Analysis["thickness_angstrom"] = surf.Thickness
		out.Analysis["vacuum_angstrom"] = surf.Vacuum

	case "interface":
		g := model.BurgersGeometry(req.AAlpha, req.CAlpha, req.ABeta)
		cands := model.SearchBurgersMatches(g, req.InterfaceMaxRepeat, 32)
		if len(cands) == 0 {
			return out, errors.New("no Burgers interface candidate found")
		}
		ci := clampIndex(req.InterfaceCandidate, len(cands))
		m := model.BuildBurgersInterface(g, cands[ci], req.AAlpha, req.CAlpha, req.ABeta, req.NZ, req.NZ, req.InterfaceDistance, req.Vacuum)
		if err := applyTitaniumAlloyMode(m.Structure, req, req.AlloyMode, &out, false); err != nil {
			return out, err
		}
		out.Structure.Meta["operation"] = "interface"
		out.Structure.Meta["model_kind"] = "titanium_alloy_alpha_beta_interface"
		out.Analysis["candidate"] = m.Candidate
		out.Analysis["normal_error_deg"] = g.NormalErrorDeg
		out.Analysis["direction_error_deg"] = g.DirectionErrorDeg
		out.Analysis["alpha_atoms"] = m.AlphaAtoms
		out.Analysis["beta_atoms"] = m.BetaAtoms
		out.Series["candidates"] = cands

	case "dislocation":
		if err := buildConfiguredHost(req, &out); err != nil {
			return out, err
		}
		m, err := model.BuildDislocation(out.Structure, req.Phase, model.DislocationOptions{
			SlipSystem:    req.SlipSystem,
			BurgersVector: parsedVectorOrZero(req.BurgersVector),
			LineDirection: parsedVectorOrZero(req.LineDirection),
			Character:     req.DislocationCharacter,
			Arrangement:   req.DislocationArrangement,
			CoreRadius:    parseValueFromSpec(req.OperationKind, "core_radius", 0),
		})
		if err != nil {
			return out, err
		}
		out.Structure = m.Structure
		out.Analysis["slip_system"] = m.SlipSystem.Preset
		out.Analysis["slip_plane"] = m.SlipSystem.Plane
		out.Analysis["slip_direction"] = m.SlipSystem.Direction
		out.Analysis["burgers_vector"] = m.SlipSystem.BurgersVector
		out.Analysis["line_direction"] = m.SlipSystem.LineDirection
		out.Analysis["slip_plane_normal"] = m.SlipSystem.SlipPlaneNormal
		out.Analysis["burgers_dot_plane_normal"] = model.Dot(m.SlipSystem.BurgersVector, m.SlipSystem.SlipPlaneNormal)
		out.Analysis["periodic_image_distance_angstrom"] = m.PeriodicImageDistance
		out.Analysis["dislocation_core_count"] = m.Structure.Meta["dislocation_core_count"]
		out.Analysis["viewer_helpers"] = map[string]any{
			"burgers_vector":    m.SlipSystem.BurgersVector,
			"line_direction":    m.SlipSystem.LineDirection,
			"slip_plane_normal": m.SlipSystem.SlipPlaneNormal,
		}
		out.Analysis["core_region"] = "unrelaxed initial geometry"

	case "grain_boundary":
		if err := buildConfiguredHost(req, &out); err != nil {
			return out, err
		}
		gb, err := model.BuildGrainBoundary(out.Structure, model.GrainBoundaryOptions{
			Type:               req.GBType,
			Axis:               req.GBAxis,
			AngleDeg:           req.GBAngleDeg,
			Normal:             req.GBNormal,
			Periodic:           !strings.Contains(strings.ToLower(req.SurfacePreset), "vacuum"),
			OverlapCutoff:      req.OverlapCutoff,
			TranslationVariant: req.InterfaceCandidate,
			Grain1Orientation:  req.Grain1Orientation,
			Grain2Orientation:  req.Grain2Orientation,
		})
		if err != nil {
			return out, err
		}
		out.Structure = gb.Structure
		out.Analysis["grain_boundary_type"] = gb.Type
		out.Analysis["gb_axis"] = req.GBAxis
		out.Analysis["gb_normal"] = req.GBNormal
		out.Analysis["grain_1_orientation"] = gb.Grain1Orientation
		out.Analysis["grain_2_orientation"] = gb.Grain2Orientation
		out.Analysis["misorientation_angle_deg"] = gb.MisorientationAngleDeg
		out.Analysis["gb_plane_normal"] = gb.GBPlaneNormal
		out.Analysis["in_plane_periodic_matching_mismatch_percent"] = gb.InPlaneMismatchPercent
		out.Analysis["removed_overlap_atom_count"] = gb.RemovedOverlapAtomCount
		out.Analysis["interface_count"] = gb.InterfaceCount
		out.Analysis["translation_candidate_index"] = gb.TranslationCandidateIndex

	case "stacking_fault", "gamma_surface":
		if err := buildConfiguredHost(req, &out); err != nil {
			return out, err
		}
		series, err := model.GenerateFaultSeries(out.Structure, model.FaultSeriesOptions{
			Preset:     req.GSFEPreset,
			Steps:      req.SeriesCount,
			Cut:        0.5,
			NormalAxis: 2,
		})
		if err != nil {
			return out, err
		}
		idx := clampIndex(req.GSFEIndex, len(series.Points))
		out.Structure = series.Points[idx].Structure
		lambdas := make([]float64, len(series.Points))
		for i, p := range series.Points {
			lambdas[i] = p.Lambda
		}
		out.Series["lambda"] = lambdas
		out.Analysis["series_point_count"] = len(series.Points)
		out.Analysis["selected_index"] = idx
		out.Analysis["area_angstrom2"] = series.Area
		out.Analysis["fault_count"] = series.FaultCount
		out.Analysis["plane"] = series.Plane
		out.Analysis["direction"] = series.Direction
		out.Analysis["path_angstrom"] = series.Path
		out.Analysis["plane_normal"] = series.PlaneNormal
		out.Analysis["geometry_series"] = "stacking fault and gamma-surface displacement structures"

	case "twin":
		if err := buildConfiguredHost(req, &out); err != nil {
			return out, err
		}
		twin, err := model.BuildTwin(out.Structure, model.TwinOptions{TwinSystem: req.TwinSystem, ShearFraction: parseValueFromSpec(req.OperationKind, "shear", 0)})
		if err != nil {
			return out, err
		}
		out.Structure = twin.Structure
		out.Analysis["twin_system"] = twin.TwinSystem
		out.Analysis["shear_fraction"] = twin.ShearFraction
		out.Analysis["geometry_operation"] = "mirror/shear initial geometry"

	case "local_chemistry", "sro", "cluster", "precipitate":
		if err := buildConfiguredHost(req, &out); err != nil {
			return out, err
		}
		element, count, region := parseColonCountRegion(req.ClusterSpec, req.NewSpecies, req.SeriesCount)
		kind := "solute_cluster"
		if out.Module == "sro" {
			kind = "sro"
		}
		if out.Module == "precipitate" || strings.TrimSpace(req.PrecipitateSpec) != "" {
			kind = "precipitate_inclusion"
			if strings.TrimSpace(req.PrecipitateSpec) != "" {
				element, count, region = parseColonCountRegion(req.PrecipitateSpec, element, count)
			}
		}
		chem, err := model.ApplyLocalChemistry(out.Structure, model.LocalChemistryOptions{Kind: kind, TargetElement: element, ClusterSize: count, Seed: req.Seed, Region: region})
		if err != nil {
			return out, err
		}
		out.Structure = chem.Structure
		out.Analysis["local_chemistry_kind"] = chem.Kind
		out.Analysis["target_element"] = chem.TargetElement
		out.Analysis["cluster_size"] = chem.ClusterSize
		out.Analysis["random_seed"] = chem.Seed
		out.Analysis["region_inside"] = chem.RegionInside
		out.Analysis["region_outside"] = chem.RegionOutside
		out.Analysis["nearest_neighbor_pair_counts"] = chem.PairCounts
		out.Analysis["warren_cowley"] = chem.WarrenCowley

	case "crack":
		if err := buildConfiguredHost(req, &out); err != nil {
			return out, err
		}
		crack, err := model.BuildCrack(out.Structure, model.CrackOptions{
			Plane:   parseStringFromSpec(req.CrackSpec, "plane", appDefaultString(req.GBNormal, "(010)")),
			Front:   parseStringFromSpec(req.CrackSpec, "front", appDefaultString(req.GBAxis, "[001]")),
			Length:  parseValueFromSpec(req.CrackSpec, "length", 0),
			Opening: parseValueFromSpec(req.CrackSpec, "opening", 0),
			Vacuum:  req.Vacuum,
		})
		if err != nil {
			return out, err
		}
		out.Structure = crack.Structure
		out.Analysis["crack_plane"] = crack.Plane
		out.Analysis["crack_front"] = crack.Front
		out.Analysis["removed_atom_count"] = crack.RemovedAtomCount
		out.Analysis["initial_crack_geometry"] = "notch/crack seed only"

	case "nanoindentation":
		if err := buildConfiguredHost(req, &out); err != nil {
			return out, err
		}
		indent, err := model.BuildNanoindentation(out.Structure, model.IndenterOptions{
			Radius: parseValueFromSpec(req.IndenterSpec, "radius", 0),
			Depth:  parseValueFromSpec(req.IndenterSpec, "depth", 0),
		})
		if err != nil {
			return out, err
		}
		out.Structure = indent.Structure
		out.Analysis["indenter_radius_angstrom"] = indent.IndenterRadius
		out.Analysis["indentation_depth_angstrom"] = indent.Depth
		out.Analysis["indenter_center"] = indent.IndenterCenter
		out.Analysis["geometry_operation"] = "substrate with spherical indenter reference"

	case "polycrystal":
		if err := buildConfiguredHost(req, &out); err != nil {
			return out, err
		}
		poly, err := model.BuildPolycrystal(out.Structure, model.PolycrystalOptions{GrainCount: req.GrainCount, Seed: req.Seed})
		if err != nil {
			return out, err
		}
		out.Structure = poly.Structure
		out.Analysis["grain_count"] = req.GrainCount
		out.Analysis["grain_atom_counts"] = poly.GrainAtomCounts
		out.Analysis["orientation_seed"] = req.Seed
		out.Series["grain_orientations"] = poly.Orientations

	case "neb":
		if err := buildConfiguredHost(req, &out); err != nil {
			return out, err
		}
		neb, err := model.GenerateNEBSeries(out.Structure, model.NEBOptions{MovingSite: clampIndex(req.SiteID, out.Structure.NAtoms()), Images: req.SeriesCount})
		if err != nil {
			return out, err
		}
		idx := clampIndex(req.GSFEIndex, len(neb.Points))
		out.Structure = neb.Points[idx].Structure
		lambdas := make([]float64, len(neb.Points))
		for i, p := range neb.Points {
			lambdas[i] = p.Lambda
		}
		out.Series["lambda"] = lambdas
		out.Analysis["series_point_count"] = len(neb.Points)
		out.Analysis["selected_index"] = idx
		out.Analysis["moving_site"] = clampIndex(req.SiteID, neb.Reference.NAtoms())
		out.Analysis["geometry_series"] = "NEB initial/final/interpolated structures"

	case "training_set":
		if err := buildConfiguredHost(req, &out); err != nil {
			return out, err
		}
		series, err := model.GenerateNEBSeries(out.Structure, model.NEBOptions{MovingSite: 0, Images: req.SeriesCount})
		if err != nil {
			return out, err
		}
		structures := []model.Structure{out.Structure}
		for _, p := range series.Points {
			structures = append(structures, p.Structure)
		}
		dataset := model.BuildTrainingSet(structures, model.DatasetOptions{Kind: req.DatasetKind, Name: "TiAlloyStudio-phase2"})
		out.Structure = dataset.Structures[0]
		out.Analysis["dataset_kind"] = dataset.Kind
		out.Analysis["dataset_name"] = dataset.Name
		out.Analysis["configuration_count"] = len(dataset.Structures)
		out.Series["configuration_indices"] = make([]int, len(dataset.Structures))
		for i := range dataset.Structures {
			out.Series["configuration_indices"].([]int)[i] = i
		}

	case "eos":
		host, err := buildHost(req)
		if err != nil {
			return out, err
		}
		series := model.GenerateEOS(host, req.EOSRatios)
		idx := clampIndex(req.EOSIndex, len(series.Points))
		out.Structure = series.Points[idx].Structure
		ratios := make([]float64, len(series.Points))
		volumes := make([]float64, len(series.Points))
		for i, p := range series.Points {
			ratios[i] = p.VolumeRatio
			volumes[i] = p.Structure.Volume()
		}
		out.Series["volume_ratios"] = ratios
		out.Series["volumes_angstrom3"] = volumes
		out.Analysis["selected_index"] = idx
		out.Analysis["selected_volume_ratio"] = series.Points[idx].VolumeRatio

	case "gsfe":
		var series model.GSFESeries
		if req.Phase == "beta" {
			series = model.BetaGSFE(req.ABeta, [3]int{req.NX, req.NY, req.NZ}, req.GSFESteps, .5)
		} else {
			series = model.AlphaGSFE(req.GSFEPreset, req.AAlpha, req.CAlpha, [3]int{req.NX, req.NY, req.NZ}, req.GSFESteps, .5)
		}
		if len(series.Points) == 0 {
			return out, errors.New("GSFE generation failed")
		}
		idx := clampIndex(req.GSFEIndex, len(series.Points))
		out.Structure = series.Points[idx].Structure
		lambdas := make([]float64, len(series.Points))
		for i, p := range series.Points {
			lambdas[i] = p.Lambda
		}
		out.Series["lambda"] = lambdas
		out.Analysis["area_angstrom2"] = series.Area
		out.Analysis["fault_count"] = series.FaultCount
		out.Analysis["plane"] = series.Plane
		out.Analysis["direction"] = series.Direction
		out.Analysis["path_angstrom"] = series.Path
		out.Analysis["plane_normal"] = series.PlaneNormal
		out.Analysis["selected_index"] = idx

	default:
		return out, fmt.Errorf("unsupported module %q", out.Module)
	}

	out.Validation = model.ValidateStructure(out.Structure)
	moduleValidation(&out)
	out.Engines = engines.CrossCheck(out.Structure)
	s.mu.Lock()
	s.Current = out
	s.CurrentRequest = req
	s.mu.Unlock()
	return out, nil
}

func (s *State) ExportBatch(format string) (filename, mime string, content []byte, err error) {
	s.mu.RLock()
	cur := s.Current
	req := s.CurrentRequest
	s.mu.RUnlock()
	if cur.Structure.NAtoms() == 0 {
		return "", "", nil, errors.New("no active model")
	}
	normalizedFormat := strings.ToLower(strings.TrimSpace(format))
	if normalizedFormat == "vasp" {
		normalizedFormat = "poscar"
	}
	if normalizedFormat != "poscar" && !(phase2BatchModule(cur.Module) && normalizedFormat == "extxyz") {
		return "", "", nil, fmt.Errorf("unsupported batch format %q", format)
	}
	if phase2BatchModule(cur.Module) {
		entries, e := phase2SeriesForRequest(req)
		if e != nil {
			return "", "", nil, e
		}
		return exportPhase2SeriesArchive(cur.Module, normalizedFormat, entries)
	}
	switch cur.Module {
	case "eos":
		host, e := buildHost(req)
		if e != nil {
			return "", "", nil, e
		}
		series := model.GenerateEOS(host, req.EOSRatios)
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		var manifest strings.Builder
		manifest.WriteString("index,volume_ratio,linear_scale,volume_angstrom3,path\n")
		for _, pt := range series.Points {
			dir := fmt.Sprintf("EOS_%03d_V%.5f", pt.Index, pt.VolumeRatio)
			path := dir + "/POSCAR"
			f, e := zw.Create(path)
			if e != nil {
				return "", "", nil, e
			}
			if _, e = f.Write([]byte(model.ExportPOSCAR(pt.Structure, fmt.Sprintf("Ti Alloy Studio EOS V/V0=%.6f", pt.VolumeRatio)))); e != nil {
				return "", "", nil, e
			}
			fmt.Fprintf(&manifest, "%d,%.10g,%.10g,%.12g,%s\n", pt.Index, pt.VolumeRatio, pt.LinearScale, pt.Structure.Volume(), path)
		}
		mf, e := zw.Create("manifest.csv")
		if e != nil {
			return "", "", nil, e
		}
		if _, e = mf.Write([]byte(manifest.String())); e != nil {
			return "", "", nil, e
		}
		readme, e := zw.Create("README.txt")
		if e != nil {
			return "", "", nil, e
		}
		fmt.Fprintf(readme, "Ti Alloy Studio EOS batch\r\nPoints: %d\r\nEach directory contains one POSCAR at the listed V/V0. Use the same DFT settings for all points before fitting E(V).\r\n", len(series.Points))
		if e = zw.Close(); e != nil {
			return "", "", nil, e
		}
		return "TiAlloyStudio-EOS-POSCAR.zip", "application/zip", buf.Bytes(), nil

	case "gsfe":
		var series model.GSFESeries
		if req.Phase == "beta" {
			series = model.BetaGSFE(req.ABeta, [3]int{req.NX, req.NY, req.NZ}, req.GSFESteps, .5)
		} else {
			series = model.AlphaGSFE(req.GSFEPreset, req.AAlpha, req.CAlpha, [3]int{req.NX, req.NY, req.NZ}, req.GSFESteps, .5)
		}
		if len(series.Points) == 0 {
			return "", "", nil, errors.New("GSFE generation failed")
		}
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		var manifest strings.Builder
		manifest.WriteString("index,lambda,shift_x_angstrom,shift_y_angstrom,shift_z_angstrom,area_angstrom2,fault_count,path\n")
		for _, pt := range series.Points {
			dir := fmt.Sprintf("GSFE_%03d_lambda%.5f", pt.Index, pt.Lambda)
			path := dir + "/POSCAR"
			f, e := zw.Create(path)
			if e != nil {
				return "", "", nil, e
			}
			if _, e = f.Write([]byte(model.ExportPOSCAR(pt.Structure, fmt.Sprintf("Ti Alloy Studio GSFE lambda=%.6f", pt.Lambda)))); e != nil {
				return "", "", nil, e
			}
			fmt.Fprintf(&manifest, "%d,%.10g,%.12g,%.12g,%.12g,%.12g,%d,%s\n", pt.Index, pt.Lambda, pt.Shift[0], pt.Shift[1], pt.Shift[2], series.Area, series.FaultCount, path)
		}
		mf, e := zw.Create("manifest.csv")
		if e != nil {
			return "", "", nil, e
		}
		if _, e = mf.Write([]byte(manifest.String())); e != nil {
			return "", "", nil, e
		}
		readme, e := zw.Create("README.txt")
		if e != nil {
			return "", "", nil, e
		}
		fmt.Fprintf(readme, "Ti Alloy Studio GSFE batch\r\nSlip plane: %s\r\nDirection: %s\r\nArea: %.10g Angstrom^2\r\nFault count: %d\r\nPoints: %d\r\nUse identical DFT settings and consistent relaxation constraints for every displacement point.\r\n", series.Plane, series.Direction, series.Area, series.FaultCount, len(series.Points))
		if e = zw.Close(); e != nil {
			return "", "", nil, e
		}
		return "TiAlloyStudio-GSFE-POSCAR.zip", "application/zip", buf.Bytes(), nil

	default:
		return "", "", nil, fmt.Errorf("batch export is not available for module %q", cur.Module)
	}
}

func (s *State) Export(format string) (filename, mime, content string, err error) {
	s.mu.RLock()
	cur := s.Current
	s.mu.RUnlock()
	if cur.Structure.NAtoms() == 0 {
		return "", "", "", errors.New("no active model")
	}
	return exportStructure(cur.Structure, format)
}

// ExportRevision exports an immutable historical snapshot without changing
// the active revision or appending project history.
func (s *State) ExportRevision(id, format string) (filename, mime, content string, err error) {
	if strings.TrimSpace(id) == "" {
		return s.Export(format)
	}
	record, err := s.revisionSnapshot(id)
	if err != nil {
		return "", "", "", err
	}
	return exportStructure(record.Structure, format)
}

func exportStructure(structure model.Structure, format string) (filename, mime, content string, err error) {
	if structure.NAtoms() == 0 {
		return "", "", "", errors.New("revision has no structure snapshot")
	}
	switch strings.ToLower(format) {
	case "poscar", "vasp":
		return "POSCAR", "text/plain", model.ExportPOSCAR(structure, "Ti Alloy Studio"), nil
	case "xyz":
		return "model.xyz", "chemical/x-xyz", model.ExportXYZ(structure), nil
	case "extxyz", "gpumd":
		return "model.extxyz", "chemical/x-xyz", model.ExportExtXYZ(structure), nil
	case "lammps", "data":
		return "model.data", "text/plain", model.ExportLAMMPS(structure), nil
	case "cif":
		return "model.cif", "chemical/x-cif", model.ExportCIF(structure), nil
	default:
		return "", "", "", fmt.Errorf("unsupported export format %q", format)
	}
}

func addCheck(r *model.ValidationReport, name, status, message string, value float64) {
	r.Checks = append(r.Checks, model.Check{Name: name, Status: status, Message: message, Value: value})
	if status == "FAIL" {
		r.Status = "FAIL"
	} else if status == "WARN" && r.Status == "PASS" {
		r.Status = "WARN"
	}
}

func countSiteLabel(s model.Structure, label string) int {
	n := 0
	for _, got := range s.SiteLabels {
		if got == label {
			n++
		}
	}
	return n
}

func moduleValidation(out *BuildResponse) {
	switch out.Module {
	case "random", "sqs":
		if out.Allocation != nil {
			total := 0
			for _, n := range out.Allocation.Counts {
				total += n
			}
			if total == out.Structure.NAtoms() {
				addCheck(&out.Validation, "composition_integer_conservation", "PASS", "Integer species counts exactly conserve all lattice sites", float64(total))
			} else {
				addCheck(&out.Validation, "composition_integer_conservation", "FAIL", "Integer species counts do not conserve lattice sites", float64(total))
			}
			addCheck(&out.Validation, "composition_resolution", "PASS",
				fmt.Sprintf("One atomic site corresponds to %.6g at.%%. This is a composition-resolution metric, not a physical convergence criterion.", out.Allocation.ResolutionAtPercent),
				out.Allocation.ResolutionAtPercent)
		}
		if out.Module == "sqs" {
			if out.ATAT != nil {
				addCheck(&out.Validation, "sqs_atat_correlations", "PASS",
					"ATAT bestcorr.out was parsed. RMS/max correlation differences are reported as convergence diagnostics; Ti Alloy Studio applies no universal acceptance threshold.",
					out.ATAT.MaxAbsDifference)
			} else if out.SQS != nil {
				addCheck(&out.Validation, "sqs_native_pair_triplet_correlations", "PASS",
					"TiModelCore pair/triplet probability optimization completed. Pair/triplet residuals are diagnostics only; converge cell size, selected geometry set, and the target property. The result is not labeled ATAT-verified.",
					math.Max(out.SQS.MaxAbsPairError, out.SQS.MaxAbsTripletError))
			}
		}

	case "surface":
		p := out.Structure.PBC
		if p[0] && p[1] && !p[2] {
			addCheck(&out.Validation, "surface_pbc", "PASS", "Surface is periodic in-plane and non-periodic along the slab normal", 0)
		} else {
			addCheck(&out.Validation, "surface_pbc", "FAIL", "Surface PBC must be in-plane only", 0)
		}
		if v, ok := out.Analysis["vacuum_angstrom"].(float64); ok {
			if v > 0 {
				addCheck(&out.Validation, "surface_vacuum", "PASS",
					"Vacuum thickness is positive and explicitly recorded. No universal vacuum-convergence threshold is imposed; test the target surface quantity versus vacuum thickness.",
					v)
			} else {
				addCheck(&out.Validation, "surface_vacuum", "FAIL", "Vacuum thickness must be positive", v)
			}
		}

	case "interface":
		ne, _ := out.Analysis["normal_error_deg"].(float64)
		de, _ := out.Analysis["direction_error_deg"].(float64)
		mx := math.Max(math.Abs(ne), math.Abs(de))
		if mx < 1e-6 {
			addCheck(&out.Validation, "burgers_orientation_relation", "PASS", "Burgers OR plane normal and in-plane direction are satisfied", mx)
		} else {
			addCheck(&out.Validation, "burgers_orientation_relation", "FAIL", "Burgers OR angular relation is not satisfied", mx)
		}
		if c, ok := out.Analysis["candidate"].(model.InterfaceCandidate); ok {
			mis := math.Max(math.Abs(c.MismatchXPercent), math.Abs(c.MismatchYPercent))
			addCheck(&out.Validation, "interface_natural_mismatch", "PASS",
				"Unstrained repeat mismatch is reported as a geometric screening metric; it is not automatically the physical interface strain.",
				mis)
			addCheck(&out.Validation, "interface_imposed_strain", "PASS",
				"Balanced coherent-cell prestrain is reported without a universal acceptance threshold. Compare candidate cells and verify relaxed stress/energy and size convergence.",
				c.MaxImposedStrainPercent)
		}

	case "dislocation":
		dot, _ := out.Analysis["burgers_dot_plane_normal"].(float64)
		if math.Abs(dot) < 1e-8 {
			addCheck(&out.Validation, "dislocation_slip_geometry", "PASS", "Burgers vector lies in the selected slip plane", dot)
		} else {
			addCheck(&out.Validation, "dislocation_slip_geometry", "FAIL", "Burgers vector is not in the selected slip plane", dot)
		}
		cores := countSiteLabel(out.Structure, "dislocation_core")
		if cores > 0 {
			addCheck(&out.Validation, "dislocation_core_labels", "PASS", "Core-neighborhood atoms are labeled for visual inspection; the core is not relaxed.", float64(cores))
		} else {
			addCheck(&out.Validation, "dislocation_core_labels", "WARN", "No atom fell inside the requested core label radius; increase model size or core radius.", 0)
		}
		if d, ok := out.Analysis["periodic_image_distance_angstrom"].(float64); ok && d > 0 {
			addCheck(&out.Validation, "dislocation_periodic_image_distance", "PASS", "Nearest periodic image distance is reported as a size diagnostic, not as a convergence claim.", d)
		}

	case "grain_boundary":
		if countSiteLabel(out.Structure, "grain_1") > 0 && countSiteLabel(out.Structure, "grain_2") > 0 {
			addCheck(&out.Validation, "grain_boundary_region_labels", "PASS", "Both grain regions are labeled", 2)
		} else {
			addCheck(&out.Validation, "grain_boundary_region_labels", "FAIL", "Grain-boundary model must contain grain_1 and grain_2 labels", 0)
		}
		if c, ok := out.Analysis["interface_count"].(int); ok && c > 0 {
			addCheck(&out.Validation, "grain_boundary_interface_count", "PASS", "Interface count is recorded for the chosen topology", float64(c))
		}
		if m, ok := out.Analysis["in_plane_periodic_matching_mismatch_percent"].(float64); ok && m >= 0 {
			addCheck(&out.Validation, "grain_boundary_periodic_matching", "PASS", "In-plane periodic matching mismatch is reported as a geometric diagnostic.", m)
		}

	case "stacking_fault", "gamma_surface":
		area, _ := out.Analysis["area_angstrom2"].(float64)
		fc, _ := out.Analysis["fault_count"].(int)
		path, _ := out.Analysis["path_angstrom"].(model.Vec3)
		normal, _ := out.Analysis["plane_normal"].(model.Vec3)
		dot := math.Abs(model.Dot(path, normal))
		if area > 0 && fc > 0 && dot < 1e-8 {
			addCheck(&out.Validation, "fault_displacement_geometry", "PASS", "Displacement path lies in the selected plane and fault geometry diagnostics are recorded.", dot)
		} else {
			addCheck(&out.Validation, "fault_displacement_geometry", "FAIL", "Fault displacement direction, area, or count is invalid", dot)
		}
		if n, ok := out.Analysis["series_point_count"].(int); ok && n > 1 {
			addCheck(&out.Validation, "fault_series_count", "PASS", "A geometry series was generated for batch export.", float64(n))
		}

	case "twin":
		if countSiteLabel(out.Structure, "parent") > 0 && countSiteLabel(out.Structure, "twin") > 0 {
			addCheck(&out.Validation, "twin_region_labels", "PASS", "Parent and twinned regions are labeled for inspection.", 2)
		} else {
			addCheck(&out.Validation, "twin_region_labels", "FAIL", "Twin model must label parent and twin regions.", 0)
		}

	case "local_chemistry", "sro", "cluster", "precipitate":
		if n, ok := out.Analysis["cluster_size"].(int); ok && n > 0 {
			addCheck(&out.Validation, "local_chemistry_region_size", "PASS", "Target local-chemistry region size is recorded.", float64(n))
		}
		if out.Analysis["warren_cowley"] != nil {
			addCheck(&out.Validation, "local_chemistry_pair_statistics", "PASS", "Nearest-neighbor pair counts and Warren-Cowley diagnostics are recorded.", 1)
		}

	case "crack":
		if n, ok := out.Analysis["removed_atom_count"].(int); ok && n > 0 {
			addCheck(&out.Validation, "crack_notch_atoms_removed", "PASS", "Crack/notch seed removed atoms and labeled nearby crack-surface atoms.", float64(n))
		} else {
			addCheck(&out.Validation, "crack_notch_atoms_removed", "WARN", "No atom was removed for the requested crack seed; increase crack length/opening or model size.", 0)
		}

	case "nanoindentation":
		if countSiteLabel(out.Structure, "near_indenter") > 0 {
			addCheck(&out.Validation, "indentation_region_labels", "PASS", "Substrate atoms near the indenter reference are labeled.", float64(countSiteLabel(out.Structure, "near_indenter")))
		} else {
			addCheck(&out.Validation, "indentation_region_labels", "WARN", "No atoms were labeled near the indenter reference; increase depth or radius.", 0)
		}

	case "polycrystal":
		if n, ok := out.Analysis["grain_count"].(int); ok && n > 0 {
			addCheck(&out.Validation, "polycrystal_grain_count", "PASS", "Voronoi grain count and atom-count distribution are recorded.", float64(n))
		}

	case "neb":
		if n, ok := out.Analysis["series_point_count"].(int); ok && n > 1 {
			addCheck(&out.Validation, "neb_geometry_series_count", "PASS", "Initial, final and interpolated NEB geometry images were generated.", float64(n))
		}

	case "training_set":
		if n, ok := out.Analysis["configuration_count"].(int); ok && n > 0 {
			addCheck(&out.Validation, "training_set_configuration_count", "PASS", "Training-set configuration count is recorded with geometry-only labels.", float64(n))
		}

	case "eos":
		ratios, ok := out.Series["volume_ratios"].([]float64)
		hasRef := false
		positive := true
		for _, v := range ratios {
			if math.Abs(v-1) < 1e-10 {
				hasRef = true
			}
			if v <= 0 {
				positive = false
			}
		}
		if hasRef {
			addCheck(&out.Validation, "eos_reference_ratio", "PASS", "EOS series contains the V/V0 = 1 reference configuration", 1)
		} else {
			addCheck(&out.Validation, "eos_reference_ratio", "FAIL", "EOS series must contain the V/V0 = 1 reference configuration", 0)
		}
		if ok && positive {
			addCheck(&out.Validation, "eos_positive_volumes", "PASS", "All EOS volume ratios are positive", float64(len(ratios)))
		} else {
			addCheck(&out.Validation, "eos_positive_volumes", "FAIL", "EOS volume ratios contain an invalid value", 0)
		}

	case "gsfe":
		area, _ := out.Analysis["area_angstrom2"].(float64)
		fc, _ := out.Analysis["fault_count"].(int)
		path, _ := out.Analysis["path_angstrom"].(model.Vec3)
		normal, _ := out.Analysis["plane_normal"].(model.Vec3)
		dot := math.Abs(model.Dot(path, normal))
		if area > 0 && (fc == 1 || fc == 2) && dot < 1e-8 {
			addCheck(&out.Validation, "gsfe_slip_geometry", "PASS", "Displacement lies in the fault plane and area/fault count are valid", dot)
		} else {
			addCheck(&out.Validation, "gsfe_slip_geometry", "FAIL", "GSFE displacement geometry, area, or fault count is invalid", dot)
		}
	}
}
