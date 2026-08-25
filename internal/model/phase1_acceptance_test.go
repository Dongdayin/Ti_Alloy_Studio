package model

import (
	"math"
	"reflect"
	"testing"
)

func closeTo(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

func TestTi64WeightToAtomicPercent(t *testing.T) {
	target, err := FromWeightPercent(map[string]float64{"Al": 6, "V": 4}, "Ti")
	if err != nil {
		t.Fatalf("FromWeightPercent failed: %v", err)
	}
	want := map[string]float64{
		"Ti": 86.20443990754129,
		"Al": 10.195484690016237,
		"V":  3.6000754024424784,
	}
	for e, w := range want {
		if !closeTo(target.AtomicPercent[e], w, 1e-10) {
			t.Fatalf("%s at.%% = %.12g, want %.12g", e, target.AtomicPercent[e], w)
		}
	}
}

func TestIntegerAllocationConservesSitesAndDoesNotWorsenHamiltonObjective(t *testing.T) {
	target, err := FromWeightPercent(map[string]float64{"Al": 6, "V": 4}, "Ti")
	if err != nil {
		t.Fatal(err)
	}
	hamilton := AllocateIntegerCounts(target, 96, false)
	optimized := AllocateIntegerCounts(target, 96, true)
	for name, a := range map[string]CompositionAllocation{"hamilton": hamilton, "optimized": optimized} {
		total := 0
		for _, n := range a.Counts {
			if n < 0 {
				t.Fatalf("%s produced a negative species count", name)
			}
			total += n
		}
		if total != 96 {
			t.Fatalf("%s count total = %d, want 96", name, total)
		}
	}
	if optimized.Objective > hamilton.Objective+1e-15 {
		t.Fatalf("local optimization worsened objective: %.16g > %.16g", optimized.Objective, hamilton.Objective)
	}
}

func TestRandomSubstitutionSeedIsReproducible(t *testing.T) {
	host := BuildAlphaTi(2.951, 4.684).Repeat(4, 4, 3)
	target, err := FromWeightPercent(map[string]float64{"Al": 6, "V": 4}, "Ti")
	if err != nil {
		t.Fatal(err)
	}
	alloc := AllocateIntegerCounts(target, host.NAtoms(), true)
	a := RandomSubstitution(host, alloc, 20260825)
	b := RandomSubstitution(host, alloc, 20260825)
	if !reflect.DeepEqual(a.Species, b.Species) {
		t.Fatal("same host/composition/seed did not reproduce identical species assignment")
	}
	if !reflect.DeepEqual(a.SpeciesCounts(), alloc.Counts) {
		t.Fatalf("assigned counts = %#v, allocation = %#v", a.SpeciesCounts(), alloc.Counts)
	}
}

func TestSupercellAtomCountAndVolumeScale(t *testing.T) {
	base := BuildAlphaTi(2.951, 4.684)
	s := base.Repeat(3, 4, 5)
	if s.NAtoms() != base.NAtoms()*3*4*5 {
		t.Fatalf("NAtoms = %d", s.NAtoms())
	}
	wantVol := base.Volume() * 3 * 4 * 5
	if !closeTo(s.Volume(), wantVol, 1e-10*wantVol) {
		t.Fatalf("volume = %.12g, want %.12g", s.Volume(), wantVol)
	}
}

func TestEOSUsesRequestedVolumeRatios(t *testing.T) {
	base := BuildAlphaTi(2.951, 4.684).Repeat(2, 2, 2)
	ratios := []float64{0.94, 1.0, 1.06}
	series := GenerateEOS(base, ratios)
	if len(series.Points) != len(ratios) {
		t.Fatalf("points = %d, want %d", len(series.Points), len(ratios))
	}
	for i, r := range ratios {
		got := series.Points[i].Structure.Volume() / base.Volume()
		if !closeTo(got, r, 2e-12) {
			t.Fatalf("point %d V/V0 = %.12g, want %.12g", i, got, r)
		}
		if !closeTo(series.Points[i].LinearScale, math.Cbrt(r), 1e-14) {
			t.Fatalf("point %d linear scale mismatch", i)
		}
	}
}

func TestGSFEBasalGeometryAndFaultCount(t *testing.T) {
	series := AlphaGSFE("basal_a", 2.951, 4.684, [3]int{3, 3, 4}, 10, 0.5)
	if len(series.Points) != 11 {
		t.Fatalf("GSFE point count = %d, want 11", len(series.Points))
	}
	if series.Area <= 0 {
		t.Fatalf("fault area = %g", series.Area)
	}
	if series.FaultCount != 2 {
		t.Fatalf("fault_count = %d, want 2 for periodic normal topology", series.FaultCount)
	}
	if math.Abs(Dot(series.Path, series.PlaneNormal)) > 1e-10 {
		t.Fatalf("GSFE displacement not in plane: |b.n| = %g", math.Abs(Dot(series.Path, series.PlaneNormal)))
	}
	for i, p := range series.Points {
		wantLambda := float64(i) / 10.0
		if !closeTo(p.Lambda, wantLambda, 1e-15) {
			t.Fatalf("lambda[%d] = %.16g, want %.16g", i, p.Lambda, wantLambda)
		}
		if p.Structure.NAtoms() != series.Reference.NAtoms() {
			t.Fatalf("atom count changed at GSFE point %d", i)
		}
		if !closeTo(p.Structure.Volume(), series.Reference.Volume(), 1e-10) {
			t.Fatalf("cell volume changed at GSFE point %d", i)
		}
	}
}

func TestIntegerSupercellConservesDeterminantAtomCount(t *testing.T) {
	base := BuildBetaTiPrimitive(3.306)
	transform := [3][3]int{{0, 1, 0}, {-3, -1, 0}, {1, 1, 2}}
	s, err := IntegerSupercell(base, transform)
	if err != nil {
		t.Fatalf("IntegerSupercell failed: %v", err)
	}
	det := transform[0][0]*(transform[1][1]*transform[2][2]-transform[1][2]*transform[2][1]) - transform[0][1]*(transform[1][0]*transform[2][2]-transform[1][2]*transform[2][0]) + transform[0][2]*(transform[1][0]*transform[2][1]-transform[1][1]*transform[2][0])
	want := base.NAtoms() * int(math.Abs(float64(det)))
	if s.NAtoms() != want {
		t.Fatalf("NAtoms = %d, want %d", s.NAtoms(), want)
	}
}
