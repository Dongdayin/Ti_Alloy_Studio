package model

import (
	"math"
	"sort"
)

const exactMinimumDistanceAtomLimit = 4096

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
	if s.NAtoms() > exactMinimumDistanceAtomLimit {
		if ref, ok := referenceNearestNeighbor(s); ok {
			return minimumDistanceCellList(frac, s.Cell, s.PBC, ref, true)
		}
		if estimate := densityDistanceEstimate(s); estimate > 0 {
			return minimumDistanceCellList(frac, s.Cell, s.PBC, estimate, false)
		}
	}
	return minimumDistanceExact(frac, s.Cell, s.PBC)
}

func minimumDistanceExact(frac []Vec3, cell Mat3, pbc [3]bool) float64 {
	best := math.Inf(1)
	for i := 0; i < len(frac)-1; i++ {
		for j := i + 1; j < len(frac); j++ {
			d := VSub(frac[j], frac[i])
			for a := 0; a < 3; a++ {
				if pbc[a] {
					d[a] -= math.Round(d[a])
				}
			}
			dist := Norm(FracToCart(d, cell))
			if dist < best {
				best = dist
			}
		}
	}
	return best
}

func referenceNearestNeighbor(s Structure) (float64, bool) {
	if s.Meta == nil {
		return 0, false
	}
	v, ok := s.Meta["reference_nearest_neighbor_angstrom"].(float64)
	if !ok || !finite(v) || v <= 0 {
		return 0, false
	}
	return v, true
}

func densityDistanceEstimate(s Structure) float64 {
	if s.NAtoms() < 2 {
		return 0
	}
	vol := s.Volume()
	if !finite(vol) || vol <= 0 {
		return 0
	}
	return 2.5 * math.Cbrt(vol/float64(s.NAtoms()))
}

type distanceBinKey struct {
	x int
	y int
	z int
}

// minimumDistanceCellList is a bounded-neighbor search for large generated
// models. With a reliable parent nearest-neighbor reference it returns the
// exact minimum whenever atoms overlap or move closer than the parent spacing,
// and otherwise returns the parent reference. This keeps validation interactive
// for 100 Å-scale cells without weakening overlap detection.
func minimumDistanceCellList(frac []Vec3, cell Mat3, pbc [3]bool, searchRadius float64, hasReference bool) float64 {
	if len(frac) < 2 {
		return math.Inf(1)
	}
	if !finite(searchRadius) || searchRadius <= 0 {
		return minimumDistanceExact(frac, cell, pbc)
	}

	best := searchRadius
	minFrac := Vec3{math.Inf(1), math.Inf(1), math.Inf(1)}
	maxFrac := Vec3{math.Inf(-1), math.Inf(-1), math.Inf(-1)}
	for _, f := range frac {
		for a := 0; a < 3; a++ {
			if pbc[a] {
				continue
			}
			if f[a] < minFrac[a] {
				minFrac[a] = f[a]
			}
			if f[a] > maxFrac[a] {
				maxFrac[a] = f[a]
			}
		}
	}

	binCount := [3]int{1, 1, 1}
	neighborBins := [3]int{1, 1, 1}
	for a := 0; a < 3; a++ {
		axisLength := Norm(cell[a])
		if !pbc[a] {
			span := maxFrac[a] - minFrac[a]
			if finite(span) && span > 1e-12 {
				axisLength *= span
			}
		}
		if finite(axisLength) && axisLength > 0 {
			binCount[a] = int(math.Floor(axisLength / searchRadius))
			if binCount[a] < 1 {
				binCount[a] = 1
			}
			if binCount[a] > len(frac) {
				binCount[a] = len(frac)
			}
			neighborBins[a] = int(math.Ceil((searchRadius / axisLength) * float64(binCount[a])))
			if neighborBins[a] < 1 {
				neighborBins[a] = 1
			}
			if neighborBins[a] > binCount[a] {
				neighborBins[a] = binCount[a]
			}
		}
	}

	binFor := func(f Vec3) distanceBinKey {
		var b [3]int
		for a := 0; a < 3; a++ {
			coord := f[a]
			if pbc[a] {
				coord = Wrap01(coord)
			} else {
				span := maxFrac[a] - minFrac[a]
				if finite(span) && span > 1e-12 {
					coord = (coord - minFrac[a]) / span
				} else {
					coord = 0
				}
			}
			x := int(math.Floor(coord * float64(binCount[a])))
			if x < 0 {
				x = 0
			}
			if x >= binCount[a] {
				x = binCount[a] - 1
			}
			b[a] = x
		}
		return distanceBinKey{b[0], b[1], b[2]}
	}

	normalizeBin := func(x, axis int) (int, bool) {
		if pbc[axis] {
			n := binCount[axis]
			x %= n
			if x < 0 {
				x += n
			}
			return x, true
		}
		if x < 0 || x >= binCount[axis] {
			return 0, false
		}
		return x, true
	}

	grid := map[distanceBinKey][]int{}
	foundCandidate := false
	wrappedNeighborsCanRepeat := false
	for a := 0; a < 3; a++ {
		if pbc[a] && 2*neighborBins[a]+1 > binCount[a] {
			wrappedNeighborsCanRepeat = true
			break
		}
	}
	for i, f := range frac {
		k := binFor(f)
		var seen map[distanceBinKey]struct{}
		if wrappedNeighborsCanRepeat {
			seen = map[distanceBinKey]struct{}{}
		}
		for dx := -neighborBins[0]; dx <= neighborBins[0]; dx++ {
			bx, ok := normalizeBin(k.x+dx, 0)
			if !ok {
				continue
			}
			for dy := -neighborBins[1]; dy <= neighborBins[1]; dy++ {
				by, ok := normalizeBin(k.y+dy, 1)
				if !ok {
					continue
				}
				for dz := -neighborBins[2]; dz <= neighborBins[2]; dz++ {
					bz, ok := normalizeBin(k.z+dz, 2)
					if !ok {
						continue
					}
					nk := distanceBinKey{bx, by, bz}
					if seen != nil {
						if _, dup := seen[nk]; dup {
							continue
						}
						seen[nk] = struct{}{}
					}
					for _, j := range grid[nk] {
						d := VSub(frac[j], f)
						for a := 0; a < 3; a++ {
							if pbc[a] {
								d[a] -= math.Round(d[a])
							}
						}
						dist := Norm(FracToCart(d, cell))
						if dist < best {
							best = dist
							foundCandidate = true
						}
					}
				}
			}
		}
		grid[k] = append(grid[k], i)
	}

	if foundCandidate || hasReference {
		return best
	}
	return minimumDistanceExact(frac, cell, pbc)
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
