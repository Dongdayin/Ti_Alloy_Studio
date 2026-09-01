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
	type rec struct {
		d    float64
		i, j int
	}
	var all []rec
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
				all = append(all, rec{d: distance, i: i, j: j})
			}
		}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].d < all[j].d })
	groups := [][]rec{}
	for _, item := range all {
		if len(groups) == 0 {
			groups = append(groups, []rec{item})
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
			groups = append(groups, []rec{item})
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
	maxSite := -1
	for _, shell := range shells {
		for _, pair := range shell.pairs {
			pairShellIndex[pair] = shell.index
			maxSite = max(maxSite, pair[1])
		}
	}
	bySignature := map[[3]int][][3]int{}
	for i := 0; i <= maxSite-2; i++ {
		for j := i + 1; j <= maxSite-1; j++ {
			sij, ok := pairShellIndex[[2]int{i, j}]
			if !ok {
				continue
			}
			for k := j + 1; k <= maxSite; k++ {
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
