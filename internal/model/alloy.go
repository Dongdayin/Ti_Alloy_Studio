package model

import (
	"math/rand"
)

func RandomSubstitution(host Structure, alloc CompositionAllocation, seed int64) Structure {
	n := host.NAtoms()
	ids := rand.New(rand.NewSource(seed)).Perm(n)
	species := make([]string, n)
	cur := 0
	elems := append([]string(nil), host.Elements()...)
	_ = elems
	// preserve target element order deterministically by common alloy ordering
	order := []string{"Ti", "Al", "V", "Mo", "Nb", "Zr", "Sn", "Fe", "Cr", "Mn", "Si", "Ta", "W", "Ni", "Cu", "Co", "Hf", "O", "N", "C", "H", "B", "Ru"}
	seen := map[string]bool{}
	for _, e := range order {
		c, ok := alloc.Counts[e]
		if !ok {
			continue
		}
		seen[e] = true
		for k := 0; k < c; k++ {
			species[ids[cur]] = e
			cur++
		}
	}
	for e, c := range alloc.Counts {
		if seen[e] {
			continue
		}
		for k := 0; k < c; k++ {
			species[ids[cur]] = e
			cur++
		}
	}
	out := host
	out.Species = species
	out.Meta = cloneMeta(host.Meta)
	out.Meta["model_kind"] = "random_substitutional_alloy"
	out.Meta["random_seed"] = seed
	out.Meta["composition_counts"] = alloc.Counts
	return out
}
