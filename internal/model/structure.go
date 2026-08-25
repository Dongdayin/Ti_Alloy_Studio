package model

import (
	"math"
	"sort"
)

type Structure struct {
	Cell       Mat3           `json:"cell"`
	Positions  []Vec3         `json:"positions"`
	Species    []string       `json:"species"`
	SiteLabels []string       `json:"site_labels,omitempty"`
	PBC        [3]bool        `json:"pbc"`
	Meta       map[string]any `json:"meta,omitempty"`
}

func (s Structure) NAtoms() int     { return len(s.Species) }
func (s Structure) Volume() float64 { return math.Abs(Determinant(s.Cell)) }
func (s Structure) Fractional(wrap bool) []Vec3 {
	out := make([]Vec3, len(s.Positions))
	for i, p := range s.Positions {
		f := CartToFrac(p, s.Cell)
		if wrap {
			for a := 0; a < 3; a++ {
				if s.PBC[a] {
					f[a] = Wrap01(f[a])
				}
			}
		}
		out[i] = f
	}
	return out
}
func (s Structure) Repeat(nx, ny, nz int) Structure {
	if nx < 1 || ny < 1 || nz < 1 {
		return s
	}
	frac := s.Fractional(true)
	out := Structure{Cell: s.Cell, PBC: s.PBC, Meta: cloneMeta(s.Meta)}
	out.Cell[0] = VScale(out.Cell[0], float64(nx))
	out.Cell[1] = VScale(out.Cell[1], float64(ny))
	out.Cell[2] = VScale(out.Cell[2], float64(nz))
	out.Positions = make([]Vec3, 0, len(frac)*nx*ny*nz)
	out.Species = make([]string, 0, cap(out.Positions))
	for ix := 0; ix < nx; ix++ {
		for iy := 0; iy < ny; iy++ {
			for iz := 0; iz < nz; iz++ {
				shift := Vec3{float64(ix), float64(iy), float64(iz)}
				for i, f := range frac {
					cart := FracToCart(VAdd(f, shift), s.Cell)
					out.Positions = append(out.Positions, cart)
					out.Species = append(out.Species, s.Species[i])
					if len(s.SiteLabels) == len(s.Species) {
						out.SiteLabels = append(out.SiteLabels, s.SiteLabels[i])
					}
				}
			}
		}
	}
	out.Meta["repeat"] = []int{nx, ny, nz}
	return out
}
func (s Structure) MinimumDistance() float64 {
	if s.NAtoms() < 2 {
		return math.Inf(1)
	}
	frac := s.Fractional(false)
	best := math.Inf(1)
	for i := 0; i < len(frac)-1; i++ {
		for j := i + 1; j < len(frac); j++ {
			d := VSub(frac[j], frac[i])
			for a := 0; a < 3; a++ {
				if s.PBC[a] {
					d[a] -= math.Round(d[a])
				}
			}
			dist := Norm(FracToCart(d, s.Cell))
			if dist < best {
				best = dist
			}
		}
	}
	return best
}

// ShortestPeriodicTranslation returns the shortest non-zero lattice
// translation connecting a defect to one of its periodic images. It searches
// the nearest image shell in lattice coordinates; non-periodic axes are held
// at zero. The value is a geometric diagnostic, not a convergence criterion.
func ShortestPeriodicTranslation(s Structure) float64 {
	best := math.Inf(1)
	for i := -1; i <= 1; i++ {
		if !s.PBC[0] && i != 0 {
			continue
		}
		for j := -1; j <= 1; j++ {
			if !s.PBC[1] && j != 0 {
				continue
			}
			for k := -1; k <= 1; k++ {
				if !s.PBC[2] && k != 0 {
					continue
				}
				if i == 0 && j == 0 && k == 0 {
					continue
				}
				v := VAdd(VAdd(VScale(s.Cell[0], float64(i)), VScale(s.Cell[1], float64(j))), VScale(s.Cell[2], float64(k)))
				d := Norm(v)
				if d > 0 && d < best {
					best = d
				}
			}
		}
	}
	return best
}

func (s Structure) SpeciesCounts() map[string]int {
	m := map[string]int{}
	for _, e := range s.Species {
		m[e]++
	}
	return m
}
func (s Structure) Elements() []string {
	m := s.SpeciesCounts()
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
func cloneMeta(in map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range in {
		out[k] = v
	}
	return out
}

func BuildAlphaTi(a, c float64) Structure {
	cell := Mat3{{a, 0, 0}, {-0.5 * a, 0.5 * math.Sqrt(3) * a, 0}, {0, 0, c}}
	frac := []Vec3{{0, 0, 0}, {2.0 / 3.0, 1.0 / 3.0, 0.5}}
	pos := []Vec3{FracToCart(frac[0], cell), FracToCart(frac[1], cell)}
	s := Structure{Cell: cell, Positions: pos, Species: []string{"Ti", "Ti"}, PBC: [3]bool{true, true, true}, Meta: map[string]any{"material": "Ti", "phase": "alpha", "bravais": "hcp", "a": a, "c": c, "c_over_a": c / a}}
	s.Meta["reference_nearest_neighbor_angstrom"] = s.MinimumDistance()
	return s
}

// BuildBetaTi returns the conventional cubic BCC cell. This is the default
// user-facing representation because its a/b/c axes are orthogonal and are
// therefore unambiguous in VESTA/VASP inspection. Algorithms that explicitly
// require a primitive BCC basis use BuildBetaTiPrimitive instead.
func BuildBetaTi(a float64) Structure {
	cell := Mat3{{a, 0, 0}, {0, a, 0}, {0, 0, a}}
	frac := []Vec3{{0, 0, 0}, {0.5, 0.5, 0.5}}
	s := Structure{
		Cell:      cell,
		Positions: []Vec3{FracToCart(frac[0], cell), FracToCart(frac[1], cell)},
		Species:   []string{"Ti", "Ti"},
		PBC:       [3]bool{true, true, true},
		Meta:      map[string]any{"material": "Ti", "phase": "beta", "bravais": "bcc", "cell_setting": "conventional", "a": a},
	}
	s.Meta["reference_nearest_neighbor_angstrom"] = s.MinimumDistance()
	return s
}

func BuildBetaTiPrimitive(a float64) Structure {
	cell := Mat3{{-0.5 * a, 0.5 * a, 0.5 * a}, {0.5 * a, -0.5 * a, 0.5 * a}, {0.5 * a, 0.5 * a, -0.5 * a}}
	s := Structure{Cell: cell, Positions: []Vec3{{0, 0, 0}}, Species: []string{"Ti"}, PBC: [3]bool{true, true, true}, Meta: map[string]any{"material": "Ti", "phase": "beta", "bravais": "bcc", "cell_setting": "primitive", "a": a}}
	// A one-atom primitive cell has no explicit atom pair, so use the exact BCC
	// nearest-neighbour distance sqrt(3)*a/2 as the parent reference scale.
	s.Meta["reference_nearest_neighbor_angstrom"] = math.Sqrt(3) * a / 2
	return s
}
