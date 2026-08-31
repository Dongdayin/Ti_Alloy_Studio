package app

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"tialloystudio/internal/model"
)

type CalculationPackageRequest struct {
	Target            string  `json:"target"`
	WorkflowPreset    string  `json:"workflow_preset,omitempty"`
	VASPKPoints       string  `json:"vasp_kpoints,omitempty"`
	VASPENCUTeV       int     `json:"vasp_encut_ev,omitempty"`
	VASPISMEAR        int     `json:"vasp_ismear,omitempty"`
	VASPSigma         float64 `json:"vasp_sigma,omitempty"`
	VASPEDIFF         string  `json:"vasp_ediff,omitempty"`
	LAMMPSPairStyle   string  `json:"lammps_pair_style,omitempty"`
	LAMMPSPairCoeff   string  `json:"lammps_pair_coeff,omitempty"`
	LAMMPSRunSteps    int     `json:"lammps_run_steps,omitempty"`
	GPUMDEnsemble     string  `json:"gpumd_ensemble,omitempty"`
	GPUMDTemperatureK float64 `json:"gpumd_temperature_k,omitempty"`
	GPUMDRunSteps     int     `json:"gpumd_run_steps,omitempty"`
}

func (s *State) ExportCalculationPackage(target string) (filename, mime string, content []byte, err error) {
	return s.ExportCalculationPackageWithOptions(CalculationPackageRequest{Target: target})
}

func (s *State) ExportCalculationPackageWithOptions(req CalculationPackageRequest) (filename, mime string, content []byte, err error) {
	req, err = normalizeCalculationPackageRequest(req)
	if err != nil {
		return "", "", nil, err
	}
	s.mu.RLock()
	cur := cloneBuildResponse(s.Current)
	buildReq := s.CurrentRequest
	s.mu.RUnlock()
	if cur.Structure.NAtoms() == 0 {
		return "", "", nil, fmt.Errorf("no active model")
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	if err = writeZipText(zw, "README.txt", calculationPackageREADME(req)); err != nil {
		return "", "", nil, err
	}
	manifest := map[string]any{
		"package_kind":      "calculation_input_preparation",
		"target":            req.Target,
		"workflow_preset":   req.WorkflowPreset,
		"module":            cur.Module,
		"phase":             buildReq.Phase,
		"atom_count":        cur.Structure.NAtoms(),
		"structure_sha256":  structureSHA256(cur.Structure),
		"scientific_state":  "not_relaxed",
		"calculation_state": "not_calculated",
		"generated_by":      "Ti Alloy Studio",
		"template_options": map[string]any{
			"vasp_kpoints":        req.VASPKPoints,
			"vasp_encut_ev":       req.VASPENCUTeV,
			"vasp_ismear":         req.VASPISMEAR,
			"vasp_sigma":          req.VASPSigma,
			"vasp_ediff":          req.VASPEDIFF,
			"lammps_pair_style":   req.LAMMPSPairStyle,
			"lammps_pair_coeff":   req.LAMMPSPairCoeff,
			"lammps_run_steps":    req.LAMMPSRunSteps,
			"gpumd_ensemble":      req.GPUMDEnsemble,
			"gpumd_temperature_k": req.GPUMDTemperatureK,
			"gpumd_run_steps":     req.GPUMDRunSteps,
		},
		"notes": []string{
			"This package contains input structures and editable templates only.",
			"No external solver was run by Ti Alloy Studio for this package.",
			"Add licensed binaries, potentials, pseudopotentials, and convergence settings outside Ti Alloy Studio before production use.",
		},
	}
	manifestBytes, e := json.MarshalIndent(manifest, "", "  ")
	if e != nil {
		return "", "", nil, e
	}
	if err = writeZipText(zw, "manifest.json", string(manifestBytes)+"\n"); err != nil {
		return "", "", nil, err
	}
	if req.Target == "vasp" || req.Target == "all" {
		if err = writeVASPInputPackage(zw, cur.Structure, req); err != nil {
			return "", "", nil, err
		}
	}
	if req.Target == "lammps" || req.Target == "all" {
		if err = writeLAMMPSInputPackage(zw, cur.Structure, req); err != nil {
			return "", "", nil, err
		}
	}
	if req.Target == "gpumd" || req.Target == "all" {
		if err = writeGPUMDInputPackage(zw, cur.Structure, req); err != nil {
			return "", "", nil, err
		}
	}
	if err = zw.Close(); err != nil {
		return "", "", nil, err
	}
	name := fmt.Sprintf("TiAlloyStudio-Phase3-R2-%s-%s-Inputs.zip", strings.ToUpper(req.Target), req.WorkflowPreset)
	return name, "application/zip", buf.Bytes(), nil
}

func normalizeCalculationPackageRequest(req CalculationPackageRequest) (CalculationPackageRequest, error) {
	req.Target = strings.ToLower(singleLine(req.Target))
	if req.Target == "" {
		req.Target = "vasp"
	}
	if req.Target != "vasp" && req.Target != "lammps" && req.Target != "gpumd" && req.Target != "all" {
		return CalculationPackageRequest{}, fmt.Errorf("unsupported calculation package target %q", req.Target)
	}
	req.WorkflowPreset = strings.ToLower(singleLine(req.WorkflowPreset))
	if req.WorkflowPreset == "" {
		req.WorkflowPreset = "structure_only"
	}
	if !validWorkflowPreset(req.WorkflowPreset) {
		return CalculationPackageRequest{}, fmt.Errorf("unsupported calculation workflow preset %q", req.WorkflowPreset)
	}
	kpoints, err := normalizeKPoints(req.VASPKPoints)
	if err != nil {
		return CalculationPackageRequest{}, err
	}
	req.VASPKPoints = kpoints
	if req.VASPENCUTeV < 0 {
		return CalculationPackageRequest{}, fmt.Errorf("vasp_encut_ev must be non-negative")
	}
	req.VASPEDIFF = singleLine(req.VASPEDIFF)
	if req.VASPEDIFF == "" {
		req.VASPEDIFF = "1e-5"
	}
	if req.VASPSigma <= 0 {
		req.VASPSigma = 0.2
	}
	req.LAMMPSPairStyle = singleLine(req.LAMMPSPairStyle)
	if req.LAMMPSPairStyle == "" {
		req.LAMMPSPairStyle = "<choose-potential-style>"
	}
	req.LAMMPSPairCoeff = singleLine(req.LAMMPSPairCoeff)
	if req.LAMMPSPairCoeff == "" {
		req.LAMMPSPairCoeff = "<choose-potential-file-and-elements>"
	}
	if req.LAMMPSRunSteps < 0 {
		return CalculationPackageRequest{}, fmt.Errorf("lammps_run_steps must be non-negative")
	}
	req.GPUMDEnsemble = strings.ToLower(singleLine(req.GPUMDEnsemble))
	if req.GPUMDEnsemble == "" {
		req.GPUMDEnsemble = "nvt"
	}
	if req.GPUMDEnsemble != "nve" && req.GPUMDEnsemble != "nvt" && req.GPUMDEnsemble != "npt" {
		return CalculationPackageRequest{}, fmt.Errorf("unsupported gpumd_ensemble %q", req.GPUMDEnsemble)
	}
	if req.GPUMDTemperatureK <= 0 {
		req.GPUMDTemperatureK = 300
	}
	if req.GPUMDRunSteps < 0 {
		return CalculationPackageRequest{}, fmt.Errorf("gpumd_run_steps must be non-negative")
	}
	return req, nil
}

func validWorkflowPreset(preset string) bool {
	switch preset {
	case "structure_only", "relaxation", "static", "md_seed", "defect", "interface", "neb_seed", "nep_labeling_seed":
		return true
	default:
		return false
	}
}

func normalizeKPoints(raw string) (string, error) {
	raw = singleLine(raw)
	if raw == "" {
		return "3 3 3", nil
	}
	fields := strings.Fields(raw)
	if len(fields) != 3 {
		return "", fmt.Errorf("vasp_kpoints must contain three positive integers")
	}
	values := make([]string, 3)
	for i, field := range fields {
		n, err := strconv.Atoi(field)
		if err != nil || n < 1 {
			return "", fmt.Errorf("vasp_kpoints must contain three positive integers")
		}
		values[i] = strconv.Itoa(n)
	}
	return strings.Join(values, " "), nil
}

func singleLine(s string) string {
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.TrimSpace(s)
}

func calculationPackageREADME(req CalculationPackageRequest) string {
	return fmt.Sprintf("Ti Alloy Studio Phase 3 R2 calculation-input package\r\nTarget: %s\r\nWorkflow preset: %s\r\nState: not_relaxed / not_calculated\r\n\r\nThis package prepares structures and editable solver templates from the current titanium-alloy model. It is not a finished calculation and contains no solver results.\r\n\r\nBefore production use, add the required licensed executables, potentials or pseudopotentials, numerical settings, and cluster/job scripts for your own environment.\r\n", req.Target, req.WorkflowPreset)
}

func writeVASPInputPackage(zw *zip.Writer, s model.Structure, req CalculationPackageRequest) error {
	if err := writeZipText(zw, "vasp/POSCAR", model.ExportPOSCAR(s, "Ti Alloy Studio Phase 3 input geometry")); err != nil {
		return err
	}
	if err := writeZipText(zw, "vasp/INCAR.template", vaspINCARTemplate(req)); err != nil {
		return err
	}
	return writeZipText(zw, "vasp/KPOINTS.template", fmt.Sprintf("KPOINTS template generated by Ti Alloy Studio\n0\nGamma\n%s\n0 0 0\n", req.VASPKPoints))
}

func vaspINCARTemplate(req CalculationPackageRequest) string {
	encut := "<set-for-your-POTCAR>"
	if req.VASPENCUTeV > 0 {
		encut = strconv.Itoa(req.VASPENCUTeV)
	}
	ibrion, nsw, isif := "-1", "0", "2"
	switch req.WorkflowPreset {
	case "relaxation", "defect", "interface":
		ibrion, nsw, isif = "2", "100", "2"
	case "neb_seed":
		ibrion, nsw, isif = "3", "0", "2"
	}
	return fmt.Sprintf("SYSTEM = Ti Alloy Studio geometry input\n# Workflow preset: %s\n# Template only: no VASP calculation was run by Ti Alloy Studio.\nENCUT = %s\nISMEAR = %d\nSIGMA = %.6g\nEDIFF = %s\nIBRION = %s\nNSW = %s\nISIF = %s\nLWAVE = .FALSE.\nLCHARG = .FALSE.\n# POTCAR is not included. Add licensed PAW datasets outside Ti Alloy Studio.\n", req.WorkflowPreset, encut, req.VASPISMEAR, req.VASPSigma, req.VASPEDIFF, ibrion, nsw, isif)
}

func writeLAMMPSInputPackage(zw *zip.Writer, s model.Structure, req CalculationPackageRequest) error {
	if err := writeZipText(zw, "lammps/model.data", model.ExportLAMMPS(s)); err != nil {
		return err
	}
	return writeZipText(zw, "lammps/in.lammps.template", lammpsInputTemplate(req))
}

func lammpsInputTemplate(req CalculationPackageRequest) string {
	var b strings.Builder
	fmt.Fprintf(&b, "units metal\natom_style atomic\nboundary p p p\nread_data model.data\n\n")
	fmt.Fprintf(&b, "pair_style %s\npair_coeff %s\n\n", req.LAMMPSPairStyle, req.LAMMPSPairCoeff)
	fmt.Fprintf(&b, "neighbor 2.0 bin\nneigh_modify delay 5\nthermo 100\n\n")
	switch req.WorkflowPreset {
	case "relaxation", "defect", "interface":
		fmt.Fprintf(&b, "min_style cg\nminimize 1.0e-10 1.0e-10 1000 10000\n")
	case "md_seed":
		fmt.Fprintf(&b, "velocity all create 300.0 4928459 mom yes rot yes dist gaussian\nfix 1 all nvt temp 300.0 300.0 0.1\n")
	default:
		fmt.Fprintf(&b, "# Add minimization, dynamics, or loading commands only after selecting a validated potential.\n")
	}
	if req.LAMMPSRunSteps > 0 {
		fmt.Fprintf(&b, "run %d\n", req.LAMMPSRunSteps)
	}
	return b.String()
}

func writeGPUMDInputPackage(zw *zip.Writer, s model.Structure, req CalculationPackageRequest) error {
	if err := writeZipText(zw, "gpumd/model.extxyz", model.ExportExtXYZ(s)); err != nil {
		return err
	}
	return writeZipText(zw, "gpumd/run.in.template", gpumdRunTemplate(req))
}

func gpumdRunTemplate(req CalculationPackageRequest) string {
	var b strings.Builder
	fmt.Fprintf(&b, "potential <your-nep-potential.txt>\n")
	fmt.Fprintf(&b, "# Workflow preset: %s\n", req.WorkflowPreset)
	fmt.Fprintf(&b, "# Template only: no GPUMD calculation was run by Ti Alloy Studio.\n")
	switch req.GPUMDEnsemble {
	case "nve":
		fmt.Fprintf(&b, "ensemble nve\n")
	case "npt":
		fmt.Fprintf(&b, "ensemble npt %.6g %.6g 100 0 0 1000\n", req.GPUMDTemperatureK, req.GPUMDTemperatureK)
	default:
		fmt.Fprintf(&b, "ensemble nvt %.6g %.6g 100\n", req.GPUMDTemperatureK, req.GPUMDTemperatureK)
	}
	if req.GPUMDRunSteps > 0 {
		fmt.Fprintf(&b, "run %d\n", req.GPUMDRunSteps)
	} else {
		fmt.Fprintf(&b, "# Add run steps after choosing a validated potential and target workflow.\n")
	}
	return b.String()
}
