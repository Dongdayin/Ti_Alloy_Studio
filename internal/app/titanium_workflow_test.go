package app

import "testing"

func TestOperationsApplyToConfiguredTitaniumAlloyBase(t *testing.T) {
	req := BuildRequest{
		Module:        "vacancy",
		AlloyMode:     "random",
		Phase:         "alpha",
		NX:            4,
		NY:            4,
		NZ:            6,
		CompositionWt: map[string]float64{"Al": 6, "V": 4},
		Seed:          44,
		SiteID:        0,
	}
	res, err := NewState().Build(req)
	if err != nil {
		t.Fatal(err)
	}
	counts := res.Structure.SpeciesCounts()
	if counts["Al"] == 0 || counts["V"] == 0 {
		t.Fatalf("vacancy operation did not start from the configured Ti alloy base: counts=%v", counts)
	}
	if res.Structure.NAtoms() != 191 {
		t.Fatalf("vacancy atom count = %d, want alloy base minus one atom", res.Structure.NAtoms())
	}
}

func TestSurfaceCanBeGeneratedFromConfiguredTitaniumAlloy(t *testing.T) {
	res, err := NewState().Build(BuildRequest{
		Module:        "surface",
		AlloyMode:     "random",
		Phase:         "alpha",
		NX:            4,
		NY:            4,
		NZ:            4,
		CompositionWt: map[string]float64{"Al": 6, "V": 4},
		Seed:          77,
		SurfacePreset: "basal_0001",
		Vacuum:        12,
	})
	if err != nil {
		t.Fatal(err)
	}
	counts := res.Structure.SpeciesCounts()
	if counts["Al"] == 0 || counts["V"] == 0 {
		t.Fatalf("surface operation did not start from the configured Ti alloy base: counts=%v", counts)
	}
	if res.Structure.PBC[2] {
		t.Fatal("surface model should remain non-periodic along the slab normal")
	}
}
