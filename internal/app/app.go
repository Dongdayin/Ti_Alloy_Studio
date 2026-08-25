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
	Module             string             `json:"module"`
	Phase              string             `json:"phase"`
	NX                 int                `json:"nx"`
	NY                 int                `json:"ny"`
	NZ                 int                `json:"nz"`
	TargetX            float64            `json:"target_x"`
	TargetY            float64            `json:"target_y"`
	TargetZ            float64            `json:"target_z"`
	AAlpha             float64            `json:"a_alpha"`
	CAlpha             float64            `json:"c_alpha"`
	ABeta              float64            `json:"a_beta"`
	CompositionWt      map[string]float64 `json:"composition_wt"`
	Seed               int64              `json:"seed"`
	SQSBackend         string             `json:"sqs_backend"`
	SQSSteps           int                `json:"sqs_steps"`
	SQSShells          int                `json:"sqs_shells"`
	ATATDistro         string             `json:"atat_distro"`
	ATATPairCutoff     float64            `json:"atat_pair_cutoff_angstrom"`
	ATATTripletCutoff  float64            `json:"atat_triplet_cutoff_angstrom"`
	ATATRunSeconds     int                `json:"atat_run_seconds"`
	SiteID             int                `json:"site_id"`
	NewSpecies         string             `json:"new_species"`
	SurfacePreset      string             `json:"surface_preset"`
	Vacuum             float64            `json:"vacuum"`
	InterfaceMaxRepeat int                `json:"interface_max_repeat"`
	InterfaceCandidate int                `json:"interface_candidate"`
	InterfaceDistance  float64            `json:"interface_distance"`
	EOSRatios          []float64          `json:"eos_ratios"`
	EOSIndex           int                `json:"eos_index"`
	GSFEPreset         string             `json:"gsfe_preset"`
	GSFESteps          int                `json:"gsfe_steps"`
	GSFEIndex          int                `json:"gsfe_index"`
}

type BuildResponse struct {
	Module     string                       `json:"module"`
	Structure  model.Structure              `json:"structure"`
	Validation model.ValidationReport       `json:"validation"`
	Allocation *model.CompositionAllocation `json:"allocation,omitempty"`
	SQS        *model.SQSQuality             `json:"sqs,omitempty"`
	ATAT       *engines.ATATQuality          `json:"atat,omitempty"`
	Analysis   map[string]any                `json:"analysis,omitempty"`
	Series     map[string]any                `json:"series,omitempty"`
	Engines    []engines.Report             `json:"engines,omitempty"`
}

type State struct {
	mu             sync.RWMutex
	Current        BuildResponse
	CurrentRequest BuildRequest
}

func NewState() *State { return &State{} }

func defaults(req *BuildRequest) {
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
		req.SQSBackend = "atat"
	}
	req.SQSBackend = strings.ToLower(strings.TrimSpace(req.SQSBackend))
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

func clampIndex(i, n int) int {
	if n <= 0 || i < 0 {
		return 0
	}
	if i >= n {
		return n - 1
	}
	return i
}

func (s *State) Build(in BuildRequest) (BuildResponse, error) {
	req := in
	defaults(&req)
	var out BuildResponse
	out.Module = strings.ToLower(req.Module)
	out.Analysis = map[string]any{}
	out.Series = map[string]any{}
	if out.Module == "" {
		out.Module = "random"
	}

	switch out.Module {
	case "crystal":
		host, err := buildHost(req)
		if err != nil {
			return out, err
		}
		out.Structure = host

	case "random":
		host, err := buildHost(req)
		if err != nil {
			return out, err
		}
		alloc, err := allocationFor(host, req)
		if err != nil {
			return out, err
		}
		out.Allocation = alloc
		out.Structure = model.RandomSubstitution(host, *alloc, req.Seed)
		out.Analysis["seed"] = req.Seed
		out.Analysis["composition_resolution_at_percent"] = alloc.ResolutionAtPercent
		out.Analysis["rms_atomic_percent_error"] = alloc.RMSAtomicPercentError
		out.Analysis["rms_weight_percent_error"] = alloc.RMSWeightPercentError

	case "sqs":
		host, err := buildHost(req)
		if err != nil {
			return out, err
		}
		alloc, err := allocationFor(host, req)
		if err != nil {
			return out, err
		}
		out.Allocation = alloc
		switch req.SQSBackend {
		case "preview":
			r, err := model.GenerateSQS(host, *alloc, req.Seed, req.SQSShells, req.SQSSteps, 1e-5)
			if err != nil {
				return out, err
			}
			out.Structure = r.Structure
			out.Structure.Meta["sqs_engine"] = "preview pair-statistics annealer"
			out.Structure.Meta["sqs_backend"] = "preview"
			out.SQS = &r.Quality
			out.Analysis["initial_objective"] = r.InitialObjective
			out.Analysis["objective"] = r.Quality.Objective
			out.Analysis["max_abs_pair_error"] = r.Quality.MaxAbsPairError
			out.Analysis["engine"] = "Preview only — " + r.Engine
			out.Series["convergence"] = r.Convergence

		case "atat":
			if req.ATATPairCutoff <= 0 {
				return out, errors.New("ATAT pair cutoff must be explicitly specified in angstrom; Ti Alloy Studio does not guess an SQS cluster cutoff")
			}
			parent, err := buildBase(req)
			if err != nil {
				return out, err
			}
			r, err := engines.RunATATSQS(parent, alloc.ActualAtomicPercent, engines.ATATOptions{
				Distro:        req.ATATDistro,
				TotalSites:    alloc.TotalSites,
				PairCutoff:    req.ATATPairCutoff,
				TripletCutoff: req.ATATTripletCutoff,
				RunSeconds:    req.ATATRunSeconds,
			})
			if err != nil {
				return out, err
			}
			if r.Structure.NAtoms() != alloc.TotalSites {
				return out, fmt.Errorf("ATAT bestsqs atom count %d does not match requested integer site count %d", r.Structure.NAtoms(), alloc.TotalSites)
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

		default:
			return out, fmt.Errorf("unsupported SQS backend %q; choose \"atat\" or explicit \"preview\"", req.SQSBackend)
		}

	case "vacancy":
		host, err := buildHost(req)
		if err != nil {
			return out, err
		}
		idx := clampIndex(req.SiteID, host.NAtoms())
		out.Structure, err = model.CreateVacancy(host, idx)
		if err != nil {
			return out, err
		}
		out.Analysis["site_id"] = idx

	case "substitution":
		host, err := buildHost(req)
		if err != nil {
			return out, err
		}
		idx := clampIndex(req.SiteID, host.NAtoms())
		out.Structure, err = model.CreateSubstitution(host, idx, req.NewSpecies)
		if err != nil {
			return out, err
		}
		out.Analysis["site_id"] = idx
		out.Analysis["new_species"] = req.NewSpecies

	case "surface":
		var surf model.SurfaceModel
		if req.Phase == "beta" {
			surf = model.BetaSurface100(req.ABeta, [2]int{req.NX, req.NY}, req.NZ, req.Vacuum)
		} else {
			surf = model.AlphaSurface(req.SurfacePreset, req.AAlpha, req.CAlpha, [2]int{req.NX, req.NY}, req.NZ, req.Vacuum)
		}
		out.Structure = surf.Structure
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
		out.Structure = m.Structure
		out.Analysis["candidate"] = m.Candidate
		out.Analysis["normal_error_deg"] = g.NormalErrorDeg
		out.Analysis["direction_error_deg"] = g.DirectionErrorDeg
		out.Analysis["alpha_atoms"] = m.AlphaAtoms
		out.Analysis["beta_atoms"] = m.BetaAtoms
		out.Series["candidates"] = cands

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
	if strings.ToLower(format) != "poscar" && strings.ToLower(format) != "vasp" {
		return "", "", nil, fmt.Errorf("unsupported batch format %q", format)
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
	switch strings.ToLower(format) {
	case "poscar", "vasp":
		return "POSCAR", "text/plain", model.ExportPOSCAR(cur.Structure, "Ti Alloy Studio"), nil
	case "xyz":
		return "model.xyz", "chemical/x-xyz", model.ExportXYZ(cur.Structure), nil
	case "extxyz", "gpumd":
		return "model.extxyz", "chemical/x-xyz", model.ExportExtXYZ(cur.Structure), nil
	case "lammps", "data":
		return "model.data", "text/plain", model.ExportLAMMPS(cur.Structure), nil
	case "cif":
		return "model.cif", "chemical/x-cif", model.ExportCIF(cur.Structure), nil
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
				addCheck(&out.Validation, "sqs_preview_backend", "WARN",
					"Internal pair-statistics annealer is a preview/fallback only, not ATAT mcsqs. Use ATAT and perform cell-size/correlation convergence for research SQS models.",
					out.SQS.MaxAbsPairError)
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
