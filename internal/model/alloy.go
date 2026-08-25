package model

import (
	"math/rand"
	"sort"
)

func RandomSubstitution(host Structure, alloc CompositionAllocation, seed int64) Structure {
	n := host.NAtoms()
	ids := rand.New(rand.NewSource(seed)).Perm(n)
	species := make([]string, n)
	assigned := map[string][]int{}
	cur := 0
	// Preserve target element order deterministically by common alloy ordering.
	// Any element absent from this list is handled in sorted lexical order below.
	order := []string{"Ti", "Al", "V", "Mo", "Nb", "Zr", "Sn", "Fe", "Cr", "Mn", "Si", "Ta", "W", "Ni", "Cu", "Co", "Hf", "O", "N", "C", "H", "B", "Ru"}
	seen := map[string]bool{}
	assign := func(e string, count int) {
		for k := 0; k < count; k++ {
			site := ids[cur]
			species[site] = e
			assigned[e] = append(assigned[e], site)
			cur++
		}
	}
	for _, e := range order {
		c, ok := alloc.Counts[e]
		if !ok {
			continue
		}
		seen[e] = true
		assign(e, c)
	}
	extra := make([]string, 0)
	for e := range alloc.Counts {
		if !seen[e] {
			extra = append(extra, e)
		}
	}
	sort.Strings(extra)
	for _, e := range extra {
		assign(e, alloc.Counts[e])
	}
	for e := range assigned {
		sort.Ints(assigned[e])
	}
	changed := make([]int, 0)
	for i, e := range species {
		if i >= len(host.Species) || e != host.Species[i] {
			changed = append(changed, i)
		}
	}

	out := host
	out.Species = species
	out.Meta = cloneMeta(host.Meta)
	out.Meta["model_kind"] = "random_substitutional_alloy"
	out.Meta["random_seed"] = seed
	out.Meta["composition_counts"] = alloc.Counts
	out.Meta["assigned_site_ids_by_species"] = assigned
	out.Meta["substituted_site_ids"] = changed
	return out
}
