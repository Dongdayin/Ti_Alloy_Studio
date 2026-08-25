package model

import "math"

type GSFEDiagnostics struct {
	PointCount                 int     `json:"point_count"`
	AtomCountConsistent        bool    `json:"atom_count_consistent"`
	CellConsistent             bool    `json:"cell_consistent"`
	PBCConsistent              bool    `json:"pbc_consistent"`
	LambdaMonotonic            bool    `json:"lambda_monotonic"`
	LambdaStart                float64 `json:"lambda_start"`
	LambdaEnd                  float64 `json:"lambda_end"`
	MinimumDistanceAngstrom    float64 `json:"minimum_distance_angstrom"`
	FaultSeparationAngstrom    float64 `json:"fault_separation_angstrom"`
	EndpointEquivalent         bool    `json:"endpoint_equivalent"`
}

func matClose(a, b Mat3, tol float64) bool {
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			scale := math.Max(1, math.Max(math.Abs(a[i][j]), math.Abs(b[i][j])))
			if math.Abs(a[i][j]-b[i][j]) > tol*scale {
				return false
			}
		}
	}
	return true
}

func samePeriodicPosition(a, b Vec3, pbc [3]bool, tol float64) bool {
	d := VSub(a, b)
	for k := 0; k < 3; k++ {
		if pbc[k] {
			d[k] -= math.Round(d[k])
		}
		if math.Abs(d[k]) > tol {
			return false
		}
	}
	return true
}

func structuresEquivalentModuloPBC(a, b Structure, tol float64) bool {
	if a.NAtoms() != b.NAtoms() || a.PBC != b.PBC || !matClose(a.Cell, b.Cell, tol) {
		return false
	}
	if len(a.Species) != len(b.Species) {
		return false
	}
	af := a.Fractional(true)
	bf := b.Fractional(true)
	used := make([]bool, len(bf))
	for i, fa := range af {
		found := -1
		for j, fb := range bf {
			if used[j] || a.Species[i] != b.Species[j] {
				continue
			}
			if samePeriodicPosition(fa, fb, a.PBC, tol) {
				found = j
				break
			}
		}
		if found < 0 {
			return false
		}
		used[found] = true
	}
	return true
}

func AnalyzeGSFESeries(series GSFESeries) GSFEDiagnostics {
	d := GSFEDiagnostics{
		PointCount:              len(series.Points),
		AtomCountConsistent:     true,
		CellConsistent:          true,
		PBCConsistent:           true,
		LambdaMonotonic:         true,
		MinimumDistanceAngstrom: math.Inf(1),
	}
	if len(series.Points) == 0 {
		d.AtomCountConsistent = false
		d.CellConsistent = false
		d.PBCConsistent = false
		d.LambdaMonotonic = false
		d.MinimumDistanceAngstrom = math.NaN()
		return d
	}

	d.LambdaStart = series.Points[0].Lambda
	d.LambdaEnd = series.Points[len(series.Points)-1].Lambda
	reference := series.Reference
	for i, p := range series.Points {
		if p.Structure.NAtoms() != reference.NAtoms() {
			d.AtomCountConsistent = false
		}
		if !matClose(p.Structure.Cell, reference.Cell, 1e-12) {
			d.CellConsistent = false
		}
		if p.Structure.PBC != reference.PBC {
			d.PBCConsistent = false
		}
		if i > 0 && p.Lambda <= series.Points[i-1].Lambda {
			d.LambdaMonotonic = false
		}
		md := p.Structure.MinimumDistance()
		if md < d.MinimumDistanceAngstrom {
			d.MinimumDistanceAngstrom = md
		}
	}

	if series.FaultCount > 0 && series.NormalAxis >= 0 && series.NormalAxis < 3 {
		periodNormal := math.Abs(Dot(reference.Cell[series.NormalAxis], Unit(series.PlaneNormal)))
		d.FaultSeparationAngstrom = periodNormal / float64(series.FaultCount)
	}
	d.EndpointEquivalent = structuresEquivalentModuloPBC(
		series.Points[0].Structure,
		series.Points[len(series.Points)-1].Structure,
		1e-8,
	)
	return d
}
