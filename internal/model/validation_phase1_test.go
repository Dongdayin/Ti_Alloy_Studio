package model

import (
	"math"
	"testing"
)

func TestValidationDoesNotUseUniversalAbsoluteShortBondThreshold(t *testing.T) {
	s := Structure{
		Cell:      Mat3{{3, 0, 0}, {0, 3, 0}, {0, 0, 3}},
		Positions: []Vec3{{0, 0, 0}, {1, 0, 0}},
		Species:   []string{"Ti", "Ti"},
		PBC:       [3]bool{true, true, true},
		Meta:      map[string]any{"reference_nearest_neighbor_angstrom": 1.0},
	}
	r := ValidateStructure(s)
	for _, c := range r.Checks {
		if c.Name == "minimum_distance" && c.Status == "WARN" {
			t.Fatalf("1.0 Å was warned solely by an absolute threshold: %+v", c)
		}
	}
	if r.Status == "FAIL" {
		t.Fatalf("valid finite structure failed validation: %+v", r)
	}
}

func TestReferenceNearestNeighborMetadataTracksParentCrystal(t *testing.T) {
	a := BuildAlphaTi(2.951, 4.684)
	v, ok := a.Meta["reference_nearest_neighbor_angstrom"].(float64)
	if !ok || v <= 0 {
		t.Fatalf("alpha-Ti reference nearest-neighbor metadata missing: %#v", a.Meta)
	}
	if math.Abs(v-a.MinimumDistance()) > 1e-12 {
		t.Fatalf("reference nn = %.12g, actual parent nn = %.12g", v, a.MinimumDistance())
	}
	b := BuildBetaTi(3.306)
	vb, ok := b.Meta["reference_nearest_neighbor_angstrom"].(float64)
	if !ok || math.Abs(vb-b.MinimumDistance()) > 1e-12 {
		t.Fatalf("beta-Ti reference nearest-neighbor metadata invalid: %#v", b.Meta)
	}
}

func TestDefectPeriodicImageDistanceIsRecorded(t *testing.T) {
	host := BuildAlphaTi(2.951, 4.684).Repeat(4, 4, 3)
	want := ShortestPeriodicTranslation(host)
	if math.IsNaN(want) || math.IsInf(want, 0) || want <= 0 {
		t.Fatalf("invalid periodic translation distance: %g", want)
	}
	v, err := CreateVacancy(host, 0)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := v.Meta["defect_periodic_image_distance_angstrom"].(float64)
	if !ok || math.Abs(got-want) > 1e-12 {
		t.Fatalf("vacancy image distance = %v, want %.12g", v.Meta["defect_periodic_image_distance_angstrom"], want)
	}
	s, err := CreateSubstitution(host, 0, "Al")
	if err != nil {
		t.Fatal(err)
	}
	got, ok = s.Meta["defect_periodic_image_distance_angstrom"].(float64)
	if !ok || math.Abs(got-want) > 1e-12 {
		t.Fatalf("substitution image distance = %v, want %.12g", s.Meta["defect_periodic_image_distance_angstrom"], want)
	}
}
