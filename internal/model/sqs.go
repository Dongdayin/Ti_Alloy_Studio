package model

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"sort"
)

type PairKey string
type TripletKey string

type PairShellQuality struct {
	ShellIndex   int                 `json:"shell_index"`
	Distance     float64             `json:"distance_angstrom"`
	PairCount    int                 `json:"pair_count"`
	Observed     map[PairKey]float64 `json:"observed_pair_probabilities"`
	Target       map[PairKey]float64 `json:"target_pair_probabilities"`
	Errors       map[PairKey]float64 `json:"pair_errors"`
	RMS          float64             `json:"rms_pair_error"`
	MaxAbs       float64             `json:"max_abs_pair_error"`
	WarrenCowley map[PairKey]float64 `json:"warren_cowley"`
}

type TripletClusterQuality struct {
	ClusterIndex   int                    `json:"cluster_index"`
	ShellSignature [3]int                 `json:"shell_signature"`
	TripletCount   int                    `json:"triplet_count"`
	Observed       map[TripletKey]float64 `json:"observed_triplet_probabilities"`
	Target         map[TripletKey]float64 `json:"target_triplet_probabilities"`
	Errors         map[TripletKey]float64 `json:"triplet_errors"`
	RMS            float64                `json:"rms_triplet_error"`
	MaxAbs         float64                `json:"max_abs_triplet_error"`
}

type SQSQuality struct {
	Method             string                  `json:"method"`
	VerificationStatus string                  `json:"verification_status"`
	Shells             []PairShellQuality      `json:"shells"`
	TripletClusters    []TripletClusterQuality `json:"triplet_clusters"`
	Objective          float64                 `json:"objective"`
	MaxAbsPairError    float64                 `json:"max_abs_pair_error"`
	MaxAbsTripletError float64                 `json:"max_abs_triplet_error"`
}

type SQSResult struct {
	Structure        Structure  `json:"structure"`
	Quality          SQSQuality `json:"quality"`
	InitialObjective float64    `json:"initial_objective"`
	Convergence      []float64  `json:"convergence"`
	Seed             int64      `json:"seed"`
	Steps            int        `json:"steps"`
	Engine           string     `json:"engine"`
}

type pairShell struct {
	index    int
	distance float64
	pairs    [][2]int
}

type pairShellRec struct {
	d    float64
	i, j int
}

type pairShellTemplate struct {
	d      float64
	from   int
	to     int
	offset [3]int
}

type tripletCluster struct {
	signature [3]int
	triplets  [][3]int
}

func pairKey(a, b string) PairKey {
	if a > b {
		a, b = b, a
	}
	return PairKey(a + "-" + b)
}

func tripletKey(a, b, c string) TripletKey {
	items := []string{a, b, c}
	sort.Strings(items)
	return TripletKey(items[0] + "-" + items[1] + "-" + items[2])
}

func buildPairShells(s Structure, nShells int, tol float64) ([]pairShell, error) {
	if nShells < 1 {
		return nil, errors.New("n_shells must be >=1")
	}
	if s.NAtoms() > exactMinimumDistanceAtomLimit {
		if shells, ok, err := buildPairShellsFromRepeat(s, nShells, tol); ok || err != nil {
			return shells, err
		}
		if shells, ok, err := buildPairShellsCellList(s, nShells, tol); ok || err != nil {
			return shells, err
		}
	}
	return buildPairShellsExact(s, nShells, tol)
}

func repeatCountsFromMeta(s Structure) ([3]int, bool) {
	var out [3]int
	if s.Meta == nil {
		return out, false
	}
	raw, ok := s.Meta["repeat"]
	if !ok {
		return out, false
	}
	switch v := raw.(type) {
	case []int:
		if len(v) != 3 {
			return out, false
		}
		copy(out[:], v)
	case []float64:
		if len(v) != 3 {
			return out, false
		}
		for i := 0; i < 3; i++ {
			out[i] = int(math.Round(v[i]))
		}
	case []any:
		if len(v) != 3 {
			return out, false
		}
		for i := 0; i < 3; i++ {
			switch x := v[i].(type) {
			case float64:
				out[i] = int(math.Round(x))
			case int:
				out[i] = x
			default:
				return out, false
			}
		}
	default:
		return out, false
	}
	if out[0] < 1 || out[1] < 1 || out[2] < 1 {
		return out, false
	}
	return out, true
}

func buildPairShellsFromRepeat(s Structure, nShells int, tol float64) ([]pairShell, bool, error) {
	repeat, ok := repeatCountsFromMeta(s)
	if !ok {
		return nil, false, nil
	}
	repeatProduct := repeat[0] * repeat[1] * repeat[2]
	if repeatProduct < 1 || s.NAtoms()%repeatProduct != 0 {
		return nil, false, nil
	}
	nBasis := s.NAtoms() / repeatProduct
	if nBasis < 1 || nBasis > 64 || len(s.Positions) < nBasis {
		return nil, false, nil
	}

	baseCell := s.Cell
	baseCell[0] = VScale(baseCell[0], 1/float64(repeat[0]))
	baseCell[1] = VScale(baseCell[1], 1/float64(repeat[1]))
	baseCell[2] = VScale(baseCell[2], 1/float64(repeat[2]))
	baseFrac := make([]Vec3, nBasis)
	for i := 0; i < nBasis; i++ {
		f := CartToFrac(s.Positions[i], baseCell)
		for axis := 0; axis < 3; axis++ {
			if s.PBC[axis] {
				f[axis] = Wrap01(f[axis])
			}
		}
		baseFrac[i] = f
	}

	maxOffset := nShells + 2
	if maxOffset < 3 {
		maxOffset = 3
	}
	if maxOffset > 6 {
		maxOffset = 6
	}
	templates := []pairShellTemplate{}
	for from := 0; from < nBasis; from++ {
		for to := 0; to < nBasis; to++ {
			for dx := -maxOffset; dx <= maxOffset; dx++ {
				for dy := -maxOffset; dy <= maxOffset; dy++ {
					for dz := -maxOffset; dz <= maxOffset; dz++ {
						if from == to && dx == 0 && dy == 0 && dz == 0 {
							continue
						}
						delta := VSub(VAdd(baseFrac[to], Vec3{float64(dx), float64(dy), float64(dz)}), baseFrac[from])
						distance := Norm(FracToCart(delta, baseCell))
						if distance > 1e-10 {
							templates = append(templates, pairShellTemplate{
								d: distance, from: from, to: to, offset: [3]int{dx, dy, dz},
							})
						}
					}
				}
			}
		}
	}
	templateGroups, err := pairShellTemplateGroups(templates, nShells, tol)
	if err != nil {
		return nil, true, err
	}

	shells := make([]pairShell, nShells)
	for shellIndex := 0; shellIndex < nShells; shellIndex++ {
		group := templateGroups[shellIndex]
		mean := 0.0
		pairs := make([][2]int, 0, len(group)*repeatProduct/2)
		for _, template := range group {
			mean += template.d
			for ix := 0; ix < repeat[0]; ix++ {
				for iy := 0; iy < repeat[1]; iy++ {
					for iz := 0; iz < repeat[2]; iz++ {
						tx, okX := shiftedCellIndex(ix, template.offset[0], repeat[0], s.PBC[0])
						ty, okY := shiftedCellIndex(iy, template.offset[1], repeat[1], s.PBC[1])
						tz, okZ := shiftedCellIndex(iz, template.offset[2], repeat[2], s.PBC[2])
						if !okX || !okY || !okZ {
							continue
						}
						i := repeatSiteIndex(ix, iy, iz, repeat, nBasis, template.from)
						j := repeatSiteIndex(tx, ty, tz, repeat, nBasis, template.to)
						if i < j {
							pairs = append(pairs, [2]int{i, j})
						}
					}
				}
			}
		}
		shells[shellIndex] = pairShell{
			index:    shellIndex + 1,
			distance: mean / float64(len(group)),
			pairs:    pairs,
		}
	}
	return shells, true, nil
}

func shiftedCellIndex(i, shift, n int, periodic bool) (int, bool) {
	out := i + shift
	if periodic {
		out %= n
		if out < 0 {
			out += n
		}
		return out, true
	}
	if out < 0 || out >= n {
		return 0, false
	}
	return out, true
}

func repeatSiteIndex(ix, iy, iz int, repeat [3]int, nBasis int, basis int) int {
	return (((ix*repeat[1])+iy)*repeat[2]+iz)*nBasis + basis
}

func pairShellTemplateGroups(templates []pairShellTemplate, nShells int, tol float64) ([][]pairShellTemplate, error) {
	if len(templates) == 0 {
		return nil, errors.New("not enough neighbor shells")
	}
	sort.Slice(templates, func(i, j int) bool { return templates[i].d < templates[j].d })
	groups := [][]pairShellTemplate{}
	for _, item := range templates {
		if len(groups) == 0 {
			groups = append(groups, []pairShellTemplate{item})
			continue
		}
		group := groups[len(groups)-1]
		mean := 0.0
		for _, existing := range group {
			mean += existing.d
		}
		mean /= float64(len(group))
		if math.Abs(item.d-mean) <= tol {
			groups[len(groups)-1] = append(groups[len(groups)-1], item)
		} else {
			groups = append(groups, []pairShellTemplate{item})
		}
		if len(groups) > nShells && item.d-groups[nShells-1][0].d > tol {
			break
		}
	}
	if len(groups) < nShells {
		return nil, errors.New("not enough neighbor shells")
	}
	return groups[:nShells], nil
}

func buildPairShellsExact(s Structure, nShells int, tol float64) ([]pairShell, error) {
	var all []pairShellRec
	frac := s.Fractional(false)
	for i := 0; i < len(frac)-1; i++ {
		for j := i + 1; j < len(frac); j++ {
			d := VSub(frac[j], frac[i])
			for axis := 0; axis < 3; axis++ {
				if s.PBC[axis] {
					d[axis] -= math.Round(d[axis])
				}
			}
			distance := Norm(FracToCart(d, s.Cell))
			if distance > 1e-10 {
				all = append(all, pairShellRec{d: distance, i: i, j: j})
			}
		}
	}
	return pairShellsFromRecords(all, nShells, tol)
}

func buildPairShellsCellList(s Structure, nShells int, tol float64) ([]pairShell, bool, error) {
	frac := s.Fractional(false)
	base, ok := referenceNearestNeighbor(s)
	if !ok {
		base = densityDistanceEstimate(s) / 2.5
	}
	if !finite(base) || base <= 0 {
		return nil, false, nil
	}

	radius := base * math.Max(1.12, 1.02+0.10*float64(nShells))
	var lastErr error
	for attempt := 0; attempt < 6; attempt++ {
		all := pairRecordsWithinRadius(frac, s.Cell, s.PBC, radius+tol)
		shells, err := pairShellsFromRecords(all, nShells, tol)
		if err == nil {
			return shells, true, nil
		}
		lastErr = err
		radius *= 1.45
	}
	if lastErr == nil {
		lastErr = errors.New("not enough neighbor shells")
	}
	return nil, true, lastErr
}

func pairRecordsWithinRadius(frac []Vec3, cell Mat3, pbc [3]bool, radius float64) []pairShellRec {
	if len(frac) < 2 || !finite(radius) || radius <= 0 {
		return nil
	}
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
			binCount[a] = int(math.Floor(axisLength / radius))
			if binCount[a] < 1 {
				binCount[a] = 1
			}
			if binCount[a] > len(frac) {
				binCount[a] = len(frac)
			}
			neighborBins[a] = int(math.Ceil((radius / axisLength) * float64(binCount[a])))
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

	wrappedNeighborsCanRepeat := false
	for a := 0; a < 3; a++ {
		if pbc[a] && 2*neighborBins[a]+1 > binCount[a] {
			wrappedNeighborsCanRepeat = true
			break
		}
	}

	grid := map[distanceBinKey][]int{}
	out := make([]pairShellRec, 0, len(frac)*8)
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
						for axis := 0; axis < 3; axis++ {
							if pbc[axis] {
								d[axis] -= math.Round(d[axis])
							}
						}
						distance := Norm(FracToCart(d, cell))
						if distance > 1e-10 && distance <= radius {
							out = append(out, pairShellRec{d: distance, i: j, j: i})
						}
					}
				}
			}
		}
		grid[k] = append(grid[k], i)
	}
	return out
}

func pairShellsFromRecords(all []pairShellRec, nShells int, tol float64) ([]pairShell, error) {
	if len(all) == 0 {
		return nil, errors.New("not enough neighbor shells")
	}
	sort.Slice(all, func(i, j int) bool { return all[i].d < all[j].d })
	groups := [][]pairShellRec{}
	for _, item := range all {
		if len(groups) == 0 {
			groups = append(groups, []pairShellRec{item})
			continue
		}
		group := groups[len(groups)-1]
		mean := 0.0
		for _, existing := range group {
			mean += existing.d
		}
		mean /= float64(len(group))
		if math.Abs(item.d-mean) <= tol {
			groups[len(groups)-1] = append(groups[len(groups)-1], item)
		} else {
			groups = append(groups, []pairShellRec{item})
		}
		if len(groups) > nShells && item.d-groups[nShells-1][0].d > tol {
			break
		}
	}
	if len(groups) < nShells {
		return nil, errors.New("not enough neighbor shells")
	}
	out := make([]pairShell, nShells)
	for index := 0; index < nShells; index++ {
		mean := 0.0
		for _, item := range groups[index] {
			mean += item.d
			out[index].pairs = append(out[index].pairs, [2]int{item.i, item.j})
		}
		out[index].index = index + 1
		out[index].distance = mean / float64(len(groups[index]))
	}
	return out, nil
}

func buildTripletClusters(shells []pairShell) []tripletCluster {
	pairShellIndex := map[[2]int]int{}
	adjacency := map[int][]int{}
	for _, shell := range shells {
		for _, pair := range shell.pairs {
			i, j := pair[0], pair[1]
			if i > j {
				i, j = j, i
			}
			pairShellIndex[[2]int{i, j}] = shell.index
			adjacency[i] = append(adjacency[i], j)
			adjacency[j] = append(adjacency[j], i)
		}
	}
	bySignature := map[[3]int][][3]int{}
	sites := make([]int, 0, len(adjacency))
	for site := range adjacency {
		sites = append(sites, site)
		sort.Ints(adjacency[site])
	}
	sort.Ints(sites)
	for _, i := range sites {
		neighbors := adjacency[i]
		for aj, j := range neighbors {
			if j <= i {
				continue
			}
			sij, ok := pairShellIndex[[2]int{i, j}]
			if !ok {
				continue
			}
			for _, k := range neighbors[aj+1:] {
				if k <= j {
					continue
				}
				sik, okIK := pairShellIndex[[2]int{i, k}]
				sjk, okJK := pairShellIndex[[2]int{j, k}]
				if !okIK || !okJK {
					continue
				}
				signatureSlice := []int{sij, sik, sjk}
				sort.Ints(signatureSlice)
				signature := [3]int{signatureSlice[0], signatureSlice[1], signatureSlice[2]}
				bySignature[signature] = append(bySignature[signature], [3]int{i, j, k})
			}
		}
	}
	signatures := make([][3]int, 0, len(bySignature))
	for signature := range bySignature {
		signatures = append(signatures, signature)
	}
	sort.Slice(signatures, func(i, j int) bool {
		for axis := 0; axis < 3; axis++ {
			if signatures[i][axis] != signatures[j][axis] {
				return signatures[i][axis] < signatures[j][axis]
			}
		}
		return false
	})
	out := make([]tripletCluster, 0, len(signatures))
	for _, signature := range signatures {
		out = append(out, tripletCluster{signature: signature, triplets: bySignature[signature]})
	}
	return out
}

func concentrationData(species []string) ([]string, map[string]float64) {
	counts := map[string]int{}
	for _, element := range species {
		counts[element]++
	}
	elements := make([]string, 0, len(counts))
	concentrations := map[string]float64{}
	for element, count := range counts {
		elements = append(elements, element)
		concentrations[element] = float64(count) / float64(len(species))
	}
	sort.Strings(elements)
	return elements, concentrations
}

func pairTargets(elements []string, concentrations map[string]float64) map[PairKey]float64 {
	targets := map[PairKey]float64{}
	for i, a := range elements {
		for _, b := range elements[i:] {
			if a == b {
				targets[pairKey(a, b)] = concentrations[a] * concentrations[b]
			} else {
				targets[pairKey(a, b)] = 2 * concentrations[a] * concentrations[b]
			}
		}
	}
	return targets
}

func tripletTargets(elements []string, concentrations map[string]float64) map[TripletKey]float64 {
	targets := map[TripletKey]float64{}
	for i, a := range elements {
		for j := i; j < len(elements); j++ {
			b := elements[j]
			for k := j; k < len(elements); k++ {
				c := elements[k]
				multiplicity := 6.0
				if a == c {
					multiplicity = 1
				} else if a == b || b == c {
					multiplicity = 3
				}
				targets[tripletKey(a, b, c)] = multiplicity * concentrations[a] * concentrations[b] * concentrations[c]
			}
		}
	}
	return targets
}

func sortedPairKeys(values map[PairKey]float64) []PairKey {
	keys := make([]PairKey, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}

func sortedTripletKeys(values map[TripletKey]float64) []TripletKey {
	keys := make([]TripletKey, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}

type fixedCompositionSQSEvaluator struct {
	shells         []pairShell
	triplets       []tripletCluster
	elements       []string
	concentrations map[string]float64
	pairTarget     map[PairKey]float64
	tripletTarget  map[TripletKey]float64
	pairKeys       []PairKey
	tripletKeys    []TripletKey
}

func newFixedCompositionSQSEvaluator(species []string, shells []pairShell, triplets []tripletCluster) fixedCompositionSQSEvaluator {
	elements, concentrations := concentrationData(species)
	pairTarget := pairTargets(elements, concentrations)
	tripletTarget := tripletTargets(elements, concentrations)
	return fixedCompositionSQSEvaluator{
		shells:         shells,
		triplets:       triplets,
		elements:       elements,
		concentrations: concentrations,
		pairTarget:     pairTarget,
		tripletTarget:  tripletTarget,
		pairKeys:       sortedPairKeys(pairTarget),
		tripletKeys:    sortedTripletKeys(tripletTarget),
	}
}

func sqsQuality(species []string, shells []pairShell, triplets []tripletCluster) SQSQuality {
	return newFixedCompositionSQSEvaluator(species, shells, triplets).quality(species)
}

func (e fixedCompositionSQSEvaluator) quality(species []string) SQSQuality {
	out := SQSQuality{Method: "pair_triplet_correlation_sqs", VerificationStatus: "not_atat_verified"}
	sumSquared, errorCount := 0.0, 0
	for _, shell := range e.shells {
		observedCount := map[PairKey]int{}
		directed := map[string]map[string]int{}
		for _, element := range e.elements {
			directed[element] = map[string]int{}
		}
		for _, pair := range shell.pairs {
			a, b := species[pair[0]], species[pair[1]]
			observedCount[pairKey(a, b)]++
			directed[a][b]++
			directed[b][a]++
		}
		observed, residuals, warrenCowley := map[PairKey]float64{}, map[PairKey]float64{}, map[PairKey]float64{}
		shellSquared, shellMax := 0.0, 0.0
		for _, key := range e.pairKeys {
			target := e.pairTarget[key]
			observed[key] = float64(observedCount[key]) / float64(len(shell.pairs))
			residuals[key] = observed[key] - target
			shellSquared += residuals[key] * residuals[key]
			shellMax = math.Max(shellMax, math.Abs(residuals[key]))
			sumSquared += residuals[key] * residuals[key]
			errorCount++
		}
		for _, center := range e.elements {
			total := 0
			for _, neighbor := range e.elements {
				total += directed[center][neighbor]
			}
			for _, neighbor := range e.elements {
				if total > 0 && e.concentrations[neighbor] > 0 {
					warrenCowley[PairKey(center+"->"+neighbor)] = 1 - (float64(directed[center][neighbor])/float64(total))/e.concentrations[neighbor]
				}
			}
		}
		out.Shells = append(out.Shells, PairShellQuality{
			ShellIndex: shell.index, Distance: shell.distance, PairCount: len(shell.pairs),
			Observed: observed, Target: e.pairTarget, Errors: residuals,
			RMS: math.Sqrt(shellSquared / float64(len(e.pairTarget))), MaxAbs: shellMax, WarrenCowley: warrenCowley,
		})
		out.MaxAbsPairError = math.Max(out.MaxAbsPairError, shellMax)
	}
	for index, cluster := range e.triplets {
		counts := map[TripletKey]int{}
		for _, sites := range cluster.triplets {
			counts[tripletKey(species[sites[0]], species[sites[1]], species[sites[2]])]++
		}
		observed, residuals := map[TripletKey]float64{}, map[TripletKey]float64{}
		clusterSquared, clusterMax := 0.0, 0.0
		for _, key := range e.tripletKeys {
			target := e.tripletTarget[key]
			observed[key] = float64(counts[key]) / float64(len(cluster.triplets))
			residuals[key] = observed[key] - target
			clusterSquared += residuals[key] * residuals[key]
			clusterMax = math.Max(clusterMax, math.Abs(residuals[key]))
			sumSquared += residuals[key] * residuals[key]
			errorCount++
		}
		out.TripletClusters = append(out.TripletClusters, TripletClusterQuality{
			ClusterIndex: index + 1, ShellSignature: cluster.signature, TripletCount: len(cluster.triplets),
			Observed: observed, Target: e.tripletTarget, Errors: residuals,
			RMS: math.Sqrt(clusterSquared / float64(len(e.tripletTarget))), MaxAbs: clusterMax,
		})
		out.MaxAbsTripletError = math.Max(out.MaxAbsTripletError, clusterMax)
	}
	if errorCount > 0 {
		out.Objective = math.Sqrt(sumSquared / float64(errorCount))
	}
	return out
}

func GenerateSQS(host Structure, alloc CompositionAllocation, seed int64, nShells, steps int, tol float64) (SQSResult, error) {
	if steps < 0 {
		return SQSResult{}, errors.New("steps must be non-negative")
	}
	shells, err := buildPairShells(host, nShells, tol)
	if err != nil {
		return SQSResult{}, err
	}
	triplets := buildTripletClusters(shells)
	initial := RandomSubstitution(host, alloc, seed)
	current := append([]string(nil), initial.Species...)
	evaluator := newFixedCompositionSQSEvaluator(current, shells, triplets)
	quality := evaluator.quality(current)
	initialObjective := quality.Objective
	best := append([]string(nil), current...)
	bestQuality := quality
	convergence := []float64{quality.Objective}
	rng := rand.New(rand.NewSource(seed + 7919))
	startTemperature, endTemperature := 0.02, 1e-4
	for step := 0; step < steps; step++ {
		i, j := rng.Intn(len(current)), rng.Intn(len(current))
		if i == j || current[i] == current[j] {
			convergence = append(convergence, bestQuality.Objective)
			continue
		}
		current[i], current[j] = current[j], current[i]
		candidate := evaluator.quality(current)
		delta := candidate.Objective - quality.Objective
		fraction := float64(step) / math.Max(1, float64(steps-1))
		temperature := startTemperature * math.Pow(endTemperature/startTemperature, fraction)
		accept := delta <= 0 || rng.Float64() < math.Exp(-delta/temperature)
		if accept {
			quality = candidate
			if candidate.Objective < bestQuality.Objective-1e-15 {
				bestQuality = candidate
				best = append([]string(nil), current...)
			}
		} else {
			current[i], current[j] = current[j], current[i]
		}
		convergence = append(convergence, bestQuality.Objective)
	}
	out := host
	out.Species = best
	out.Meta = cloneMeta(host.Meta)
	out.Meta["model_kind"] = "sqs"
	out.Meta["sqs_method"] = bestQuality.Method
	out.Meta["sqs_verification_status"] = bestQuality.VerificationStatus
	out.Meta["sqs_objective"] = bestQuality.Objective
	return SQSResult{
		Structure: out, Quality: bestQuality, InitialObjective: initialObjective,
		Convergence: convergence, Seed: seed, Steps: steps,
		Engine: fmt.Sprintf("TiModelCore pair/triplet probability annealer (%d pair shells, %d triplet geometries)", len(shells), len(triplets)),
	}, nil
}
