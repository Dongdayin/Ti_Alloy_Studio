package app

import (
	"fmt"
	"math"
	"strings"

	"tialloystudio/internal/engines"
	"tialloystudio/internal/model"
)

// BuildUser is the GUI/API build path. It preserves BuildTracked behavior for
// all modules, while making the physically explicit interface topology part of
// the tracked result. Legacy requests default to a fully periodic bicrystal.
func (s *State) BuildUser(req BuildRequest) (BuildResponse, error) {
	if strings.ToLower(strings.TrimSpace(req.Module)) != "interface" {
		return s.BuildTracked(req)
	}

	out, err := s.Build(req)
	if err != nil {
		return out, err
	}
	s.mu.RLock()
	normalized := s.CurrentRequest
	s.mu.RUnlock()

	topology := strings.ToLower(strings.TrimSpace(req.SurfacePreset))
	if topology == "interface_single_slab" {
		out.Analysis["interface_topology"] = "single_interface_slab"
		out.Analysis["interface_count"] = 1
		out.Analysis["vacuum_angstrom"] = normalized.Vacuum
		addCheck(&out.Validation, "interface_topology", "WARN",
			"Single-interface slab contains one alpha/beta interface plus two free surfaces separated by vacuum. Use only when that surface/interface topology is intentional; it is not a bulk periodic bicrystal.", 1)
		if out.Structure.PBC == [3]bool{true, true, false} {
			addCheck(&out.Validation, "interface_slab_pbc", "PASS", "Single-interface slab is periodic only in the interface plane", 0)
		} else {
			addCheck(&out.Validation, "interface_slab_pbc", "FAIL", "Single-interface slab PBC topology is inconsistent", 0)
		}
	} else {
		periodic, metrics, e := model.PeriodicizeBurgersInterface(out.Structure, normalized.InterfaceDistance)
		if e != nil {
			return out, fmt.Errorf("periodicize Burgers interface: %w", e)
		}
		out.Structure = periodic
		out.Analysis["interface_topology"] = "periodic_bicrystal"
		out.Analysis["interface_count"] = metrics.InterfaceCount
		out.Analysis["internal_gap_angstrom"] = metrics.InternalGapAngstrom
		out.Analysis["boundary_gap_angstrom"] = metrics.BoundaryGapAngstrom
		out.Analysis["alpha_plane_span_angstrom"] = metrics.AlphaPlaneSpanAngstrom
		out.Analysis["beta_plane_span_angstrom"] = metrics.BetaPlaneSpanAngstrom
		out.Analysis["interface_equivalence_assumed"] = false

		// The topology changed after the low-level slab construction; validation
		// and independent-engine cross-checks must therefore be recomputed on the
		// actual structure that will be shown/exported.
		out.Validation = model.ValidateStructure(out.Structure)
		moduleValidation(&out)
		out.Engines = engines.CrossCheck(out.Structure)
		addCheck(&out.Validation, "interface_topology", "PASS", "Fully periodic alpha/beta bicrystal with two interfaces and no vacuum/free surface", float64(metrics.InterfaceCount))
		if out.Structure.PBC == [3]bool{true, true, true} {
			addCheck(&out.Validation, "interface_periodic_pbc", "PASS", "Periodic bicrystal is periodic in all three lattice directions", 3)
		} else {
			addCheck(&out.Validation, "interface_periodic_pbc", "FAIL", "Periodic bicrystal must be periodic in all three lattice directions", 0)
		}
		gapDelta := math.Abs(metrics.InternalGapAngstrom - metrics.BoundaryGapAngstrom)
		if gapDelta < 1e-8 {
			addCheck(&out.Validation, "interface_gap_symmetry", "PASS", "Both periodic alpha/beta boundaries use the requested initial geometric gap", gapDelta)
		} else {
			addCheck(&out.Validation, "interface_gap_symmetry", "WARN", "The two initial interface gaps differ; inspect terminations before relaxation", gapDelta)
		}
		addCheck(&out.Validation, "alpha_region_span", "PASS", "Alpha-region normal span is reported for thickness-convergence studies; no universal converged thickness is imposed.", metrics.AlphaPlaneSpanAngstrom)
		addCheck(&out.Validation, "beta_region_span", "PASS", "Beta-region normal span is reported for thickness-convergence studies; no universal converged thickness is imposed.", metrics.BetaPlaneSpanAngstrom)
		addCheck(&out.Validation, "interface_pair_equivalence", "WARN", "The two periodic interfaces share the Burgers orientation relation but atomic terminations are not assumed equivalent. Do not normalize an interface excess energy by 2A unless equivalence has been demonstrated; otherwise treat the cell excess as the contribution of the interface pair.", float64(metrics.InterfaceCount))
	}

	enrichTrackedDiagnostics(normalized, &out)
	s.mu.Lock()
	s.Current = out
	s.CurrentRequest = normalized
	s.mu.Unlock()
	recordTrackedBuild(s, normalized, out)
	return out, nil
}
