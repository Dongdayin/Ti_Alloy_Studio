package model

import (
	"errors"
	"math"
)

type PeriodicInterfaceMetrics struct {
	InterfaceCount        int     `json:"interface_count"`
	InternalGapAngstrom   float64 `json:"internal_gap_angstrom"`
	BoundaryGapAngstrom   float64 `json:"boundary_gap_angstrom"`
	AlphaPlaneSpanAngstrom float64 `json:"alpha_plane_span_angstrom"`
	BetaPlaneSpanAngstrom  float64 `json:"beta_plane_span_angstrom"`
}

// PeriodicizeBurgersInterface converts the single-interface/vacuum construction
// into a fully periodic alpha/beta bicrystal. The existing internal alpha->beta
// interface spacing is preserved and a second equivalent periodic-boundary gap
// is created. No free surface or vacuum remains in the returned structure.
func PeriodicizeBurgersInterface(s Structure, requestedGap float64) (Structure, PeriodicInterfaceMetrics, error) {
	if s.NAtoms() == 0 || len(s.SiteLabels) != s.NAtoms() {
		return Structure{}, PeriodicInterfaceMetrics{}, errors.New("interface structure requires alpha/beta site labels")
	}
	if requestedGap <= 0 || math.IsNaN(requestedGap) || math.IsInf(requestedGap, 0) {
		return Structure{}, PeriodicInterfaceMetrics{}, errors.New("periodic interface gap must be positive")
	}
	normal := Unit(Cross(s.Cell[0], s.Cell[1]))
	if Norm(normal) == 0 {
		return Structure{}, PeriodicInterfaceMetrics{}, errors.New("interface in-plane cell is singular")
	}
	minA, maxA := math.Inf(1), math.Inf(-1)
	minB, maxB := math.Inf(1), math.Inf(-1)
	for i, p := range s.Positions {
		z := Dot(p, normal)
		switch s.SiteLabels[i] {
		case "alpha":
			minA = math.Min(minA, z)
			maxA = math.Max(maxA, z)
		case "beta":
			minB = math.Min(minB, z)
			maxB = math.Max(maxB, z)
		}
	}
	if math.IsInf(minA, 0) || math.IsInf(minB, 0) {
		return Structure{}, PeriodicInterfaceMetrics{}, errors.New("interface must contain both alpha and beta atoms")
	}
	internalGap := minB - maxA
	if internalGap <= 0 {
		return Structure{}, PeriodicInterfaceMetrics{}, errors.New("alpha and beta regions overlap before periodicization")
	}

	out := s
	out.Positions = append([]Vec3(nil), s.Positions...)
	shift := VScale(normal, -minA)
	for i := range out.Positions {
		out.Positions[i] = VAdd(out.Positions[i], shift)
	}
	betaMaxShifted := maxB - minA
	cellLength := betaMaxShifted + requestedGap
	out.Cell[2] = VScale(normal, cellLength)
	out.PBC = [3]bool{true, true, true}
	out.Meta = cloneMeta(s.Meta)
	out.Meta["model_kind"] = "alpha_beta_periodic_bicrystal"
	out.Meta["interface_topology"] = "periodic_bicrystal"
	out.Meta["interface_count"] = 2
	out.Meta["vacuum_angstrom"] = 0.0
	out.Meta["periodic_boundary_interface_gap_angstrom"] = requestedGap
	out.Meta["internal_interface_gap_angstrom"] = internalGap

	metrics := PeriodicInterfaceMetrics{
		InterfaceCount:         2,
		InternalGapAngstrom:    internalGap,
		BoundaryGapAngstrom:    requestedGap,
		AlphaPlaneSpanAngstrom: maxA - minA,
		BetaPlaneSpanAngstrom:  maxB - minB,
	}
	return out, metrics, nil
}
