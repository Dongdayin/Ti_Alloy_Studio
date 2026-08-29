package model

import (
	"math"
	"strings"
	"testing"
)

func requireNoCalculatedQuantities(t *testing.T, s Structure) {
	t.Helper()
	for key := range s.Meta {
		lower := strings.ToLower(key)
		for _, forbidden := range []string{"energy", "force", "stress", "gamma_value", "stable_fault"} {
			if strings.Contains(lower, forbidden) {
				t.Fatalf("geometry-only model exposes calculated quantity key %q", key)
			}
		}
	}
	if got := s.Meta["scientific_state"]; got != "not_relaxed" {
		t.Fatalf("scientific_state = %v, want not_relaxed", got)
	}
	if got := s.Meta["calculation_state"]; got != "not_calculated" {
		t.Fatalf("calculation_state = %v, want not_calculated", got)
	}
}

func countLabel(labels []string, want string) int {
	n := 0
	for _, label := range labels {
		if label == want {
			n++
		}
	}
	return n
}

func TestDislocationModelRecordsSlipGeometryAndCoreLabels(t *testing.T) {
	host := BuildAlphaTi(2.951, 4.684).Repeat(6, 6, 4)
	model, err := BuildDislocation(host, "alpha", DislocationOptions{
		SlipSystem:  "alpha_prismatic_a",
		Character:   "edge",
		Arrangement: "dipole",
		CoreRadius:  3.2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if model.Structure.NAtoms() != host.NAtoms() {
		t.Fatalf("dislocation atom count = %d, want unchanged %d", model.Structure.NAtoms(), host.NAtoms())
	}
	if got := model.Structure.Meta["model_kind"]; got != "dislocation" {
		t.Fatalf("model_kind = %v, want dislocation", got)
	}
	if model.SlipSystem.Phase != "alpha" || model.SlipSystem.Plane == "" || model.SlipSystem.Direction == "" {
		t.Fatalf("slip system was not recorded: %+v", model.SlipSystem)
	}
	if dot := Dot(model.SlipSystem.BurgersVector, model.SlipSystem.SlipPlaneNormal); math.Abs(dot) > 1e-8 {
		t.Fatalf("Burgers vector is not in slip plane: b dot n = %.12g", dot)
	}
	if countLabel(model.Structure.SiteLabels, "dislocation_core") == 0 {
		t.Fatal("dislocation model did not label the core region")
	}
	if model.PeriodicImageDistance <= 0 {
		t.Fatalf("periodic image distance = %.6g, want positive diagnostic", model.PeriodicImageDistance)
	}
	requireNoCalculatedQuantities(t, model.Structure)
}

func TestGrainBoundaryModelLabelsBothGrainsAndRecordsMismatch(t *testing.T) {
	host := BuildBetaTi(3.306).Repeat(5, 5, 5)
	gb, err := BuildGrainBoundary(host, GrainBoundaryOptions{
		Type:               "tilt",
		Axis:               "[001]",
		AngleDeg:           12.5,
		Normal:             "[100]",
		Periodic:           true,
		OverlapCutoff:      1.6,
		TranslationVariant: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if countLabel(gb.Structure.SiteLabels, "grain_1") == 0 || countLabel(gb.Structure.SiteLabels, "grain_2") == 0 {
		t.Fatalf("grain labels are incomplete: %v", gb.Structure.SiteLabels)
	}
	if gb.MisorientationAngleDeg != 12.5 {
		t.Fatalf("misorientation = %.6g, want 12.5", gb.MisorientationAngleDeg)
	}
	if gb.InterfaceCount != 2 {
		t.Fatalf("interface count = %d, want 2 for periodic bicrystal", gb.InterfaceCount)
	}
	if gb.InPlaneMismatchPercent < 0 {
		t.Fatalf("mismatch = %.6g, want non-negative", gb.InPlaneMismatchPercent)
	}
	if _, ok := gb.Structure.Meta["removed_overlap_atom_count"].(int); !ok {
		t.Fatal("removed_overlap_atom_count diagnostic missing")
	}
	requireNoCalculatedQuantities(t, gb.Structure)
}

func TestFaultSeriesAndTwinModelsAreGeometryOnly(t *testing.T) {
	host := BuildAlphaTi(2.951, 4.684).Repeat(4, 4, 6)
	series, err := GenerateFaultSeries(host, FaultSeriesOptions{
		Preset:     "alpha_basal_a",
		Steps:      4,
		Cut:        0.5,
		NormalAxis: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(series.Points) != 5 {
		t.Fatalf("fault series length = %d, want 5", len(series.Points))
	}
	wantLambda := []float64{0, 0.25, 0.5, 0.75, 1}
	for i, point := range series.Points {
		if math.Abs(point.Lambda-wantLambda[i]) > 1e-12 {
			t.Fatalf("lambda[%d] = %.12g, want %.12g", i, point.Lambda, wantLambda[i])
		}
		if Dot(point.Shift, series.PlaneNormal) > 1e-8 {
			t.Fatalf("fault shift %d is not in plane: %+v normal %+v", i, point.Shift, series.PlaneNormal)
		}
		requireNoCalculatedQuantities(t, point.Structure)
	}
	if series.Area <= 0 || series.FaultCount == 0 {
		t.Fatalf("invalid fault diagnostics: area=%.6g faults=%d", series.Area, series.FaultCount)
	}

	twin, err := BuildTwin(host, TwinOptions{TwinSystem: "alpha_10-12", ShearFraction: 0.18})
	if err != nil {
		t.Fatal(err)
	}
	if countLabel(twin.Structure.SiteLabels, "parent") == 0 || countLabel(twin.Structure.SiteLabels, "twin") == 0 {
		t.Fatal("twin model did not label parent and twin regions")
	}
	if twin.Structure.Meta["model_kind"] != "twin" {
		t.Fatalf("model_kind = %v, want twin", twin.Structure.Meta["model_kind"])
	}
	requireNoCalculatedQuantities(t, twin.Structure)
}

func TestLocalChemistryRecordsRegionsPairsSROAndSeed(t *testing.T) {
	host := Structure{
		Cell:      Mat3{{8, 0, 0}, {0, 8, 0}, {0, 0, 8}},
		PBC:       [3]bool{true, true, true},
		Meta:      map[string]any{"phase": "alpha", "bravais": "hcp"},
		Positions: []Vec3{{0, 0, 0}, {1, 0, 0}, {2, 0, 0}, {3, 0, 0}, {4, 0, 0}, {5, 0, 0}, {6, 0, 0}, {7, 0, 0}},
		Species:   []string{"Ti", "Ti", "Ti", "Ti", "Ti", "Ti", "Ti", "Ti"},
	}
	chem, err := ApplyLocalChemistry(host, LocalChemistryOptions{
		Kind:          "solute_cluster",
		TargetElement: "Al",
		ClusterSize:   3,
		Seed:          77,
		Region:        "center",
	})
	if err != nil {
		t.Fatal(err)
	}
	if chem.Structure.SpeciesCounts()["Al"] != 3 {
		t.Fatalf("Al count = %d, want 3", chem.Structure.SpeciesCounts()["Al"])
	}
	if chem.ClusterSize != 3 || chem.Seed != 77 {
		t.Fatalf("cluster diagnostics = size %d seed %d, want 3 and 77", chem.ClusterSize, chem.Seed)
	}
	if len(chem.PairCounts) == 0 {
		t.Fatal("nearest-neighbor pair diagnostics missing")
	}
	if _, ok := chem.WarrenCowley["Al-Ti"]; !ok {
		t.Fatalf("Warren-Cowley diagnostic missing Al-Ti pair: %v", chem.WarrenCowley)
	}
	if chem.RegionInside["Al"] == 0 {
		t.Fatalf("inside-region composition missing target solute: %v", chem.RegionInside)
	}
	requireNoCalculatedQuantities(t, chem.Structure)
}

func TestMechanicalAndDatasetBuildersEmitLabeledGeometryOnlyStructures(t *testing.T) {
	host := BuildBetaTi(3.306).Repeat(4, 4, 4)

	crack, err := BuildCrack(host, CrackOptions{Plane: "(010)", Front: "[001]", Length: 6, Opening: 1.2, Vacuum: 10})
	if err != nil {
		t.Fatal(err)
	}
	if countLabel(crack.Structure.SiteLabels, "crack_surface") == 0 || crack.RemovedAtomCount == 0 {
		t.Fatal("crack model did not create a labeled crack/notch geometry")
	}
	requireNoCalculatedQuantities(t, crack.Structure)

	indent, err := BuildNanoindentation(host, IndenterOptions{Radius: 8, Depth: 1.5})
	if err != nil {
		t.Fatal(err)
	}
	if indent.Structure.Meta["model_kind"] != "nanoindentation" || indent.IndenterRadius <= 0 {
		t.Fatalf("invalid indentation diagnostics: %+v", indent)
	}
	requireNoCalculatedQuantities(t, indent.Structure)

	poly, err := BuildPolycrystal(host, PolycrystalOptions{GrainCount: 4, Seed: 13})
	if err != nil {
		t.Fatal(err)
	}
	if len(poly.GrainAtomCounts) != 4 {
		t.Fatalf("grain count diagnostics = %d, want 4", len(poly.GrainAtomCounts))
	}
	if countLabel(poly.Structure.SiteLabels, "grain_0") == 0 {
		t.Fatal("polycrystal model did not label grain membership")
	}
	requireNoCalculatedQuantities(t, poly.Structure)

	neb, err := GenerateNEBSeries(host, NEBOptions{MovingSite: 0, FinalShift: Vec3{0.5, 0, 0}, Images: 3})
	if err != nil {
		t.Fatal(err)
	}
	if len(neb.Points) != 5 {
		t.Fatalf("NEB series length = %d, want initial + 3 images + final", len(neb.Points))
	}
	for _, point := range neb.Points {
		requireNoCalculatedQuantities(t, point.Structure)
	}

	dataset := BuildTrainingSet([]Structure{crack.Structure, indent.Structure, poly.Structure}, DatasetOptions{Kind: "nep", Name: "phase2"})
	if dataset.Kind != "nep" || len(dataset.Structures) != 3 {
		t.Fatalf("dataset = kind %q structures %d, want nep/3", dataset.Kind, len(dataset.Structures))
	}
	for _, s := range dataset.Structures {
		requireNoCalculatedQuantities(t, s)
	}
}
