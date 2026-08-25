package model

import (
	"errors"
	"math"
	"sort"
)

var AtomicWeights = map[string]float64{
	"Ti": 47.867, "Al": 26.9815384, "V": 50.9415, "Mo": 95.95, "Nb": 92.90637, "Zr": 91.224,
	"Sn": 118.710, "Fe": 55.845, "Cr": 51.9961, "Mn": 54.938043, "Si": 28.085, "O": 15.999,
	"N": 14.007, "C": 12.011, "H": 1.008, "Ta": 180.94788, "W": 183.84, "Ni": 58.6934, "Cu": 63.546,
	"Co": 58.933194, "B": 10.81, "Hf": 178.49, "Ru": 101.07,
}

type CompositionTarget struct {
	Elements      []string           `json:"elements"`
	WeightPercent map[string]float64 `json:"weight_percent"`
	AtomicPercent map[string]float64 `json:"atomic_percent"`
}

type CompositionAllocation struct {
	TotalSites            int                `json:"total_sites"`
	Counts                map[string]int     `json:"counts"`
	IdealCounts           map[string]float64 `json:"ideal_counts"`
	TargetAtomicPercent   map[string]float64 `json:"target_atomic_percent"`
	ActualAtomicPercent   map[string]float64 `json:"actual_atomic_percent"`
	TargetWeightPercent   map[string]float64 `json:"target_weight_percent"`
	ActualWeightPercent   map[string]float64 `json:"actual_weight_percent"`
	AtomicPercentError    map[string]float64 `json:"atomic_percent_error"`
	WeightPercentError    map[string]float64 `json:"weight_percent_error"`
	RMSAtomicPercentError float64            `json:"rms_atomic_percent_error"`
	RMSWeightPercentError float64            `json:"rms_weight_percent_error"`
	MaxAtomicPercentError float64            `json:"max_atomic_percent_error"`
	MaxWeightPercentError float64            `json:"max_weight_percent_error"`
	ResolutionAtPercent   float64            `json:"resolution_at_percent"`
	MinimumSitesOneAtom   int                `json:"minimum_sites_one_atom"`
	Objective             float64            `json:"objective"`
}

func FromWeightPercent(values map[string]float64, balance string) (CompositionTarget, error) {
	if len(values) == 0 && balance == "" {
		return CompositionTarget{}, errors.New("composition is empty")
	}
	weights := map[string]float64{}
	total := 0.0
	for e, v := range values {
		if v < 0 || math.IsNaN(v) || math.IsInf(v, 0) {
			return CompositionTarget{}, errors.New("invalid weight percent")
		}
		if _, ok := AtomicWeights[e]; !ok {
			return CompositionTarget{}, errors.New("unknown element " + e)
		}
		weights[e] = v
		total += v
	}
	if balance != "" {
		if _, ok := AtomicWeights[balance]; !ok {
			return CompositionTarget{}, errors.New("unknown balance element")
		}
		if total >= 100 {
			return CompositionTarget{}, errors.New("explicit weight percent must be <100")
		}
		weights[balance] = 100 - total
	} else if math.Abs(total-100) > 1e-8 {
		return CompositionTarget{}, errors.New("weight percent must sum to 100")
	}
	elems := make([]string, 0, len(weights))
	for e := range weights {
		elems = append(elems, e)
	}
	sort.Strings(elems)
	molTotal := 0.0
	mol := map[string]float64{}
	for e, w := range weights {
		mol[e] = w / AtomicWeights[e]
		molTotal += mol[e]
	}
	atomic := map[string]float64{}
	for e := range weights {
		atomic[e] = 100 * mol[e] / molTotal
	}
	return CompositionTarget{Elements: elems, WeightPercent: weights, AtomicPercent: atomic}, nil
}

func actualPercents(counts map[string]int, target CompositionTarget) (map[string]float64, map[string]float64) {
	n := 0
	for _, v := range counts {
		n += v
	}
	at := map[string]float64{}
	wt := map[string]float64{}
	mass := 0.0
	for _, e := range target.Elements {
		at[e] = 100 * float64(counts[e]) / float64(n)
		mass += float64(counts[e]) * AtomicWeights[e]
	}
	for _, e := range target.Elements {
		wt[e] = 100 * float64(counts[e]) * AtomicWeights[e] / mass
	}
	return at, wt
}
func objectiveCounts(counts map[string]int, target CompositionTarget) (float64, map[string]float64, map[string]float64, map[string]float64, map[string]float64, float64, float64, float64, float64) {
	at, wt := actualPercents(counts, target)
	ae := map[string]float64{}
	we := map[string]float64{}
	sa, sw := 0.0, 0.0
	ma, mw := 0.0, 0.0
	for _, e := range target.Elements {
		ae[e] = at[e] - target.AtomicPercent[e]
		we[e] = wt[e] - target.WeightPercent[e]
		sa += ae[e] * ae[e]
		sw += we[e] * we[e]
		ma = math.Max(ma, math.Abs(ae[e]))
		mw = math.Max(mw, math.Abs(we[e]))
	}
	k := float64(len(target.Elements))
	ra := math.Sqrt(sa / k)
	rw := math.Sqrt(sw / k)
	obj := 0.5*math.Pow(ra/100, 2) + 0.5*math.Pow(rw/100, 2)
	return obj, at, wt, ae, we, ra, rw, ma, mw
}
func AllocateIntegerCounts(target CompositionTarget, total int, optimize bool) CompositionAllocation {
	if total < 1 {
		total = 1
	}
	ideal := map[string]float64{}
	counts := map[string]int{}
	rems := map[string]float64{}
	used := 0
	for _, e := range target.Elements {
		x := float64(total) * target.AtomicPercent[e] / 100
		ideal[e] = x
		c := int(math.Floor(x))
		counts[e] = c
		rems[e] = x - float64(c)
		used += c
	}
	ranked := append([]string(nil), target.Elements...)
	sort.SliceStable(ranked, func(i, j int) bool { return rems[ranked[i]] > rems[ranked[j]] })
	for i := 0; i < total-used; i++ {
		counts[ranked[i]]++
	}
	if optimize {
		for {
			cur, _, _, _, _, _, _, _, _ := objectiveCounts(counts, target)
			best := cur
			var recv, donor string
			for _, r := range target.Elements {
				for _, d := range target.Elements {
					if r == d || counts[d] == 0 {
						continue
					}
					cand := map[string]int{}
					for k, v := range counts {
						cand[k] = v
					}
					cand[r]++
					cand[d]--
					obj, _, _, _, _, _, _, _, _ := objectiveCounts(cand, target)
					if obj < best-1e-18 {
						best = obj
						recv = r
						donor = d
					}
				}
			}
			if recv == "" {
				break
			}
			counts[recv]++
			counts[donor]--
		}
	}
	obj, at, wt, ae, we, ra, rw, ma, mw := objectiveCounts(counts, target)
	minSites := 1
	for _, e := range target.Elements {
		f := target.AtomicPercent[e] / 100
		if f > 0 {
			m := int(math.Ceil(1 / f))
			if m > minSites {
				minSites = m
			}
		}
	}
	return CompositionAllocation{TotalSites: total, Counts: counts, IdealCounts: ideal, TargetAtomicPercent: target.AtomicPercent, ActualAtomicPercent: at, TargetWeightPercent: target.WeightPercent, ActualWeightPercent: wt, AtomicPercentError: ae, WeightPercentError: we, RMSAtomicPercentError: ra, RMSWeightPercentError: rw, MaxAtomicPercentError: ma, MaxWeightPercentError: mw, ResolutionAtPercent: 100 / float64(total), MinimumSitesOneAtom: minSites, Objective: obj}
}
