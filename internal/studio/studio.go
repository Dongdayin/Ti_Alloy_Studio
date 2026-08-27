package studio

import (
	"errors"
	"fmt"
	"math"

	"tialloystudio/internal/app"
)

type SmokeResult struct {
	Status     string         `json:"status"`
	Atoms      int            `json:"atoms"`
	Counts     map[string]int `json:"counts"`
	GSFEPoints int            `json:"gsfe_points"`
	Checks     []string       `json:"checks"`
}

func ScientificSmoke() (SmokeResult, error) {
	st := app.NewState()
	r, err := st.Build(app.BuildRequest{Module: "random", Phase: "alpha", NX: 4, NY: 4, NZ: 6, CompositionWt: map[string]float64{"Al": 6, "V": 4}, Seed: 20260825})
	if err != nil {
		return SmokeResult{}, err
	}
	out := SmokeResult{Status: "PASS", Atoms: r.Structure.NAtoms(), Counts: r.Structure.SpeciesCounts()}
	if r.Validation.Status == "FAIL" {
		out.Status = "FAIL"
		return out, fmt.Errorf("random model validation failed")
	}
	out.Checks = append(out.Checks, "TC4 192-site allocation and structure validation")
	if r.Allocation == nil || r.Allocation.Counts["Ti"] != 165 || r.Allocation.Counts["Al"] != 20 || r.Allocation.Counts["V"] != 7 {
		out.Status = "FAIL"
		return out, fmt.Errorf("TC4 integer allocation mismatch")
	}

	// The smoke test exercises the bundled pair/triplet probability backend so
	// it remains runnable on fresh installations without WSL or ATAT.
	s, err := st.Build(app.BuildRequest{Module: "sqs", Phase: "alpha", NX: 4, NY: 4, NZ: 6, CompositionWt: map[string]float64{"Al": 6, "V": 4}, Seed: 7, SQSBackend: "preview", SQSSteps: 150, SQSShells: 2})
	if err != nil {
		return out, err
	}
	if s.SQS == nil || math.IsNaN(s.SQS.Objective) || math.IsInf(s.SQS.Objective, 0) {
		out.Status = "FAIL"
		return out, fmt.Errorf("SQS preview quality invalid")
	}
	out.Checks = append(out.Checks, "bundled SQS pair/triplet probability quality")

	g, err := st.Build(app.BuildRequest{Module: "gsfe", Phase: "alpha", GSFEPreset: "basal_a", NX: 2, NY: 2, NZ: 6, GSFESteps: 10})
	if err != nil {
		return out, err
	}
	lam, ok := g.Series["lambda"].([]float64)
	if !ok {
		out.Status = "FAIL"
		return out, fmt.Errorf("GSFE lambda series missing")
	}
	out.GSFEPoints = len(lam)
	out.Checks = append(out.Checks, "alpha-Ti basal GSFE geometry")

	i, err := st.Build(app.BuildRequest{Module: "interface", InterfaceMaxRepeat: 6, InterfaceDistance: 2.5, Vacuum: 10, NZ: 2})
	if err != nil {
		return out, err
	}
	if i.Validation.Status == "FAIL" {
		out.Status = "FAIL"
		return out, fmt.Errorf("interface validation failed")
	}
	out.Checks = append(out.Checks, "Burgers alpha/beta interface construction")

	revisions := app.NewState()
	if _, err = revisions.BuildTracked(app.BuildRequest{Module: "crystal", Phase: "alpha", NX: 2, NY: 2, NZ: 2}); err != nil {
		return out, fmt.Errorf("revision smoke root build: %w", err)
	}
	root := revisions.ProjectManifest("").ActiveRevisionID
	_, _, rootXYZ, err := revisions.ExportRevision(root, "xyz")
	if err != nil {
		return out, fmt.Errorf("revision smoke root export: %w", err)
	}
	if _, err = revisions.BuildChild(root, app.BuildRequest{Module: "crystal", Phase: "beta", NX: 3, NY: 2, NZ: 2}); err != nil {
		return out, fmt.Errorf("revision smoke edit: %w", err)
	}
	child := revisions.ProjectManifest("").ActiveRevisionID
	_, _, childXYZ, err := revisions.ExportRevision(child, "xyz")
	if err != nil {
		return out, fmt.Errorf("revision smoke child export: %w", err)
	}
	if childXYZ == rootXYZ {
		return out, errors.New("revision smoke child export did not preserve a distinct edited structure")
	}
	project, err := revisions.ExportProjectArchive("smoke path with spaces")
	if err != nil {
		return out, fmt.Errorf("revision smoke project export: %w", err)
	}
	reopened := app.NewState()
	if _, err = reopened.ImportProjectArchive(project); err != nil {
		return out, fmt.Errorf("revision smoke project reopen: %w", err)
	}
	_, _, reopenedRootXYZ, err := reopened.ExportRevision(root, "xyz")
	if err != nil {
		return out, fmt.Errorf("revision smoke reopened root export: %w", err)
	}
	if reopenedRootXYZ != rootXYZ || len(reopened.ProjectManifest("").History) != 2 {
		return out, errors.New("revision smoke project round trip changed history or root export")
	}
	out.Checks = append(out.Checks, "revision edit/export/project round trip")
	return out, nil
}
