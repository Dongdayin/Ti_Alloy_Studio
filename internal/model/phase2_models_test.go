package model

import (
	"math"
	"strings"
	"testing"
	"time"
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

func TestDislocationModelUsesCustomVectorsAndCreatesPeriodicCoreSets(t *testing.T) {
	host := BuildBetaTi(3.306).Repeat(6, 6, 6)
	disl, err := BuildDislocation(host, "beta", DislocationOptions{
		SlipSystem:    "beta_110_111",
		BurgersVector: Vec3{0, 0, 2.5},
		LineDirection: Vec3{0, 1, 0},
		Character:     "mixed",
		Arrangement:   "quadrupole",
		CoreRadius:    2.4,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := disl.SlipSystem.BurgersVector; math.Abs(got[2]-2.5) > 1e-12 || math.Abs(got[0]) > 1e-12 || math.Abs(got[1]) > 1e-12 {
		t.Fatalf("custom Burgers vector was not used: %+v", got)
	}
	if got := disl.SlipSystem.LineDirection; math.Abs(got[1]-1) > 1e-12 || math.Abs(got[0]) > 1e-12 || math.Abs(got[2]) > 1e-12 {
		t.Fatalf("custom line direction was not normalized and used: %+v", got)
	}
	if got, ok := disl.Structure.Meta["dislocation_core_count"].(int); !ok || got != 4 {
		t.Fatalf("quadrupole core count = %v, want 4", disl.Structure.Meta["dislocation_core_count"])
	}
	if countLabel(disl.Structure.SiteLabels, "dislocation_core") == 0 {
		t.Fatal("quadrupole dislocation did not label any core atoms")
	}
	requireNoCalculatedQuantities(t, disl.Structure)
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
	if math.Abs(gb.MisorientationAngleDeg-12.5) > 1e-9 {
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

func TestGrainBoundaryUsesRequestedAxisNormalAndOrientationMatrices(t *testing.T) {
	host := BuildBetaTi(3.306).Repeat(5, 5, 5)
	gb, err := BuildGrainBoundary(host, GrainBoundaryOptions{
		Type:              "general",
		Axis:              "[010]",
		Normal:            "[010]",
		AngleDeg:          20,
		Periodic:          false,
		OverlapCutoff:     0.5,
		Grain1Orientation: "1 0 0; 0 1 0; 0 0 1",
		Grain2Orientation: "0 -1 0; 1 0 0; 0 0 1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(gb.GBPlaneNormal[1]-1) > 1e-12 || math.Abs(gb.GBPlaneNormal[0]) > 1e-12 || math.Abs(gb.GBPlaneNormal[2]) > 1e-12 {
		t.Fatalf("GB normal did not use requested [010]: %+v", gb.GBPlaneNormal)
	}
	if gb.InterfaceCount != 1 || gb.Structure.PBC[1] {
		t.Fatalf("vacuum bicrystal topology did not follow the requested GB normal: interface_count=%d pbc=%v", gb.InterfaceCount, gb.Structure.PBC)
	}
	if gb.InPlaneMismatchPercent <= 0 {
		t.Fatalf("orientation mismatch diagnostic was not computed: %.6g", gb.InPlaneMismatchPercent)
	}
	if math.Abs(gb.Grain2Orientation[0][1]+1) > 1e-12 || math.Abs(gb.Grain2Orientation[1][0]-1) > 1e-12 {
		t.Fatalf("grain 2 orientation matrix not preserved: %+v", gb.Grain2Orientation)
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

	candidates := GenerateTrainingCandidates(host, TrainingCandidateOptions{Count: 4, Seed: 17})
	if len(candidates) != 4 {
		t.Fatalf("training candidates = %d, want 4", len(candidates))
	}
	if candidates[0].NAtoms() != host.NAtoms() || candidates[3].NAtoms() != host.NAtoms() {
		t.Fatalf("training candidates changed atom count: host=%d first=%d last=%d", host.NAtoms(), candidates[0].NAtoms(), candidates[3].NAtoms())
	}
	if candidates[0].Meta["training_variant"] != "base" || candidates[3].Meta["training_variant"] == "neb_image" {
		t.Fatalf("training candidate semantics not explicit: first=%v last=%v", candidates[0].Meta["training_variant"], candidates[3].Meta["training_variant"])
	}
	for _, s := range candidates {
		requireNoCalculatedQuantities(t, s)
	}

	indexed := GenerateTrainingCandidate(host, TrainingCandidateOptions{Count: 4, Seed: 17}, 3)
	if indexed.NAtoms() != candidates[3].NAtoms() {
		t.Fatalf("indexed training candidate atoms = %d, want %d", indexed.NAtoms(), candidates[3].NAtoms())
	}
	if indexed.Meta["training_candidate_index"] != 3 || indexed.Meta["training_variant"] != candidates[3].Meta["training_variant"] {
		t.Fatalf("indexed training candidate metadata = %v, want index 3 variant %v", indexed.Meta, candidates[3].Meta["training_variant"])
	}
	for site := range indexed.Positions {
		if indexed.Positions[site] != candidates[3].Positions[site] {
			t.Fatalf("indexed training candidate site %d position = %+v, want %+v", site, indexed.Positions[site], candidates[3].Positions[site])
		}
	}
	requireNoCalculatedQuantities(t, indexed)

	dataset := BuildTrainingSet([]Structure{crack.Structure, indent.Structure, poly.Structure}, DatasetOptions{Kind: "nep", Name: "phase2"})
	if dataset.Kind != "nep" || len(dataset.Structures) != 3 {
		t.Fatalf("dataset = kind %q structures %d, want nep/3", dataset.Kind, len(dataset.Structures))
	}
	for _, s := range dataset.Structures {
		requireNoCalculatedQuantities(t, s)
	}
}

func TestLargeCrackUsesScaledVisibleDefaultWithoutStalling(t *testing.T) {
	host := BuildAlphaTi(2.951, 4.684).Repeat(34, 40, 22)
	type result struct {
		crack CrackModel
		err   error
	}
	done := make(chan result, 1)
	go func() {
		crack, err := BuildCrack(host, CrackOptions{Plane: "(010)", Front: "[001]"})
		done <- result{crack: crack, err: err}
	}()

	var got result
	select {
	case got = <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("large crack generation stalled; nearest-neighbor scale should not be recomputed for every atom")
	}
	if got.err != nil {
		t.Fatal(got.err)
	}
	min, max := bounds(host)
	spanX := max[0] - min[0]
	if got.crack.LengthAngstrom < 0.25*spanX || got.crack.LengthAngstrom > 0.45*spanX {
		t.Fatalf("default crack length = %.6g Å, want a visible fraction of %.6g Å", got.crack.LengthAngstrom, spanX)
	}
	if got.crack.OpeningAngstrom < 1.5*host.MinimumDistance() {
		t.Fatalf("default crack opening = %.6g Å, too small to create a visible notch", got.crack.OpeningAngstrom)
	}
	if got.crack.RemovedAtomCount < host.NAtoms()/200 {
		t.Fatalf("removed atoms = %d of %d, crack seed is too small to inspect", got.crack.RemovedAtomCount, host.NAtoms())
	}
	if countLabel(got.crack.Structure.SiteLabels, "crack_surface") == 0 {
		t.Fatal("large crack model did not label crack-surface atoms")
	}
	requireNoCalculatedQuantities(t, got.crack.Structure)
}
