package model

import "math"

type Check struct {
	Name    string  `json:"name"`
	Status  string  `json:"status"`
	Message string  `json:"message"`
	Value   float64 `json:"value,omitempty"`
}
type ValidationReport struct {
	Status string  `json:"status"`
	Checks []Check `json:"checks"`
}

func ValidateStructure(s Structure) ValidationReport {
	r := ValidationReport{Status: "PASS"}
	add := func(n, st, msg string, v float64) {
		r.Checks = append(r.Checks, Check{n, st, msg, v})
		if st == "FAIL" {
			r.Status = "FAIL"
		} else if st == "WARN" && r.Status == "PASS" {
			r.Status = "WARN"
		}
	}
	vol := s.Volume()
	if vol < 1e-10 || math.IsNaN(vol) {
		add("cell_volume", "FAIL", "Cell is singular or invalid", vol)
	} else {
		add("cell_volume", "PASS", "Positive cell volume", vol)
	}
	if s.NAtoms() == 0 {
		add("atom_count", "FAIL", "No atoms", 0)
	} else {
		add("atom_count", "PASS", "Atom count is positive", float64(s.NAtoms()))
	}
	d := s.MinimumDistance()
	if d < 1e-5 {
		add("minimum_distance", "FAIL", "Duplicate/overlapping atomic sites", d)
	} else if d < 1.5 {
		add("minimum_distance", "WARN", "Unusually short interatomic distance; inspect model", d)
	} else {
		add("minimum_distance", "PASS", "No geometric overlap detected", d)
	}
	if len(s.Species) != len(s.Positions) {
		add("species_positions", "FAIL", "Species/position count mismatch", 0)
	} else {
		add("species_positions", "PASS", "Species and position counts match", 0)
	}
	if len(s.SiteLabels) != 0 && len(s.SiteLabels) != len(s.Species) {
		add("site_labels", "FAIL", "Site-label count must be zero or equal to atom count", float64(len(s.SiteLabels)))
	} else if len(s.SiteLabels) == len(s.Species) && len(s.SiteLabels) > 0 {
		add("site_labels", "PASS", "Per-site phase/group labels are complete", float64(len(s.SiteLabels)))
	}
	return r
}
