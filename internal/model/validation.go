package model

import (
	"fmt"
	"math"
)

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

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

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

	det := Determinant(s.Cell)
	vol := math.Abs(det)
	if !finite(det) || vol < 1e-10 {
		add("cell_volume", "FAIL", "Cell is singular or non-finite", vol)
	} else {
		add("cell_volume", "PASS", "Cell determinant is finite and non-zero", vol)
		if det < 0 {
			add("cell_handedness", "WARN", "Cell basis is left-handed. This is not automatically unphysical, but verify orientation conventions before export or comparison.", det)
		} else {
			add("cell_handedness", "PASS", "Cell basis is right-handed", det)
		}
	}

	if s.NAtoms() == 0 {
		add("atom_count", "FAIL", "No atoms", 0)
	} else {
		add("atom_count", "PASS", "Atom count is positive", float64(s.NAtoms()))
		if finite(vol) && vol > 0 {
			add("volume_per_atom", "PASS", "Volume per atom is reported as a diagnostic; convergence and physical plausibility depend on the material state.", vol/float64(s.NAtoms()))
		}
	}

	allFinite := true
	for _, p := range s.Positions {
		for _, x := range p {
			if !finite(x) {
				allFinite = false
				break
			}
		}
		if !allFinite {
			break
		}
	}
	if allFinite {
		add("finite_coordinates", "PASS", "All Cartesian coordinates are finite", 0)
	} else {
		add("finite_coordinates", "FAIL", "At least one Cartesian coordinate is NaN or infinite", 0)
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

	if s.NAtoms() >= 2 && allFinite && finite(vol) && vol > 0 {
		d := s.MinimumDistance()
		if !finite(d) || d < 1e-5 {
			add("minimum_distance", "FAIL", "Duplicate or numerically overlapping atomic sites detected", d)
		} else {
			add("minimum_distance", "PASS", "Minimum interatomic distance is reported without applying a universal absolute bond-length cutoff", d)
			if ref, ok := s.Meta["reference_nearest_neighbor_angstrom"].(float64); ok && finite(ref) && ref > 0 {
				ratio := d / ref
				add("minimum_distance_ratio_to_parent", "PASS", fmt.Sprintf("d_min/d_ref = %.6g. This parent-relative value is a screening diagnostic, not a universal pass/fail criterion.", ratio), ratio)
			}
		}
	}

	anglesOK := true
	for i := 0; i < 3; i++ {
		for j := i + 1; j < 3; j++ {
			ni, nj := Norm(s.Cell[i]), Norm(s.Cell[j])
			if ni == 0 || nj == 0 || !finite(ni) || !finite(nj) {
				anglesOK = false
				continue
			}
			c := Clamp(Dot(s.Cell[i], s.Cell[j])/(ni*nj), -1, 1)
			a := math.Acos(c) * 180 / math.Pi
			if !finite(a) || a <= 1e-8 || a >= 180-1e-8 {
				anglesOK = false
			}
		}
	}
	if anglesOK {
		add("cell_angles", "PASS", "Cell-vector angles are finite and non-degenerate", 0)
	} else {
		add("cell_angles", "FAIL", "Cell contains a degenerate or non-finite lattice-vector angle", 0)
	}

	if d, ok := s.Meta["defect_periodic_image_distance_angstrom"].(float64); ok && finite(d) && d > 0 {
		add("defect_periodic_image_distance", "PASS", "Shortest defect-to-periodic-image separation is reported for convergence studies; Ti Alloy Studio does not impose a universal converged defect-cell size.", d)
	}
	return r
}
