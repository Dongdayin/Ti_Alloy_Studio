package model

import (
	"reflect"
	"testing"
)

func TestRandomSubstitutionRecordsAssignedAndChangedSiteIDs(t *testing.T) {
	host := BuildAlphaTi(2.951, 4.684).Repeat(4, 4, 6)
	target, err := FromWeightPercent(map[string]float64{"Al": 6, "V": 4}, "Ti")
	if err != nil {
		t.Fatal(err)
	}
	alloc := AllocateIntegerCounts(target, host.NAtoms(), true)
	a := RandomSubstitution(host, alloc, 1234)
	b := RandomSubstitution(host, alloc, 1234)
	idsA, ok := a.Meta["assigned_site_ids_by_species"].(map[string][]int)
	if !ok {
		t.Fatalf("assigned site metadata missing: %#v", a.Meta)
	}
	idsB, ok := b.Meta["assigned_site_ids_by_species"].(map[string][]int)
	if !ok || !reflect.DeepEqual(idsA, idsB) {
		t.Fatalf("same seed did not reproduce assigned site IDs: %#v vs %#v", idsA, idsB)
	}
	for e, want := range alloc.Counts {
		if len(idsA[e]) != want {
			t.Fatalf("%s assigned site count = %d, want %d", e, len(idsA[e]), want)
		}
	}
	changed, ok := a.Meta["substituted_site_ids"].([]int)
	if !ok {
		t.Fatalf("substituted site IDs missing: %#v", a.Meta)
	}
	if len(changed) != alloc.Counts["Al"]+alloc.Counts["V"] {
		t.Fatalf("changed site count = %d, want %d", len(changed), alloc.Counts["Al"]+alloc.Counts["V"])
	}
}
