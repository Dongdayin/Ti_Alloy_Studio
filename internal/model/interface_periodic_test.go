package model

import (
	"math"
	"testing"
)

func TestPeriodicBurgersInterfaceHasTwoPeriodicInterfaces(t *testing.T) {
	g := BurgersGeometry(2.951, 4.684, 3.306)
	cands := SearchBurgersMatches(g, 8, 4)
	if len(cands) == 0 {
		t.Fatal("no Burgers candidate")
	}
	slab := BuildBurgersInterface(g, cands[0], 2.951, 4.684, 3.306, 3, 3, 2.5, 10)
	p, m, err := PeriodicizeBurgersInterface(slab.Structure, 2.5)
	if err != nil {
		t.Fatal(err)
	}
	if p.PBC != [3]bool{true, true, true} {
		t.Fatalf("periodic bicrystal PBC = %#v", p.PBC)
	}
	if m.InterfaceCount != 2 {
		t.Fatalf("interface count = %d", m.InterfaceCount)
	}
	if math.Abs(m.InternalGapAngstrom-2.5) > 1e-8 {
		t.Fatalf("internal gap = %.12g", m.InternalGapAngstrom)
	}
	if math.Abs(m.BoundaryGapAngstrom-2.5) > 1e-12 {
		t.Fatalf("boundary gap = %.12g", m.BoundaryGapAngstrom)
	}
	if got, _ := p.Meta["interface_topology"].(string); got != "periodic_bicrystal" {
		t.Fatalf("topology = %q", got)
	}
	if p.NAtoms() != slab.Structure.NAtoms() {
		t.Fatalf("atom count changed: %d -> %d", slab.Structure.NAtoms(), p.NAtoms())
	}
}
