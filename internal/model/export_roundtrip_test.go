package model

import (
	"math"
	"reflect"
	"testing"
)

func gram(cell Mat3) Mat3 {
	var g Mat3
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			g[i][j] = Dot(cell[i], cell[j])
		}
	}
	return g
}

func matricesClose(a, b Mat3, tol float64) bool {
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			if math.Abs(a[i][j]-b[i][j]) > tol*math.Max(1, math.Max(math.Abs(a[i][j]), math.Abs(b[i][j]))) {
				return false
			}
		}
	}
	return true
}

func roundTripFixture(t *testing.T) Structure {
	t.Helper()
	host := BuildAlphaTi(2.951, 4.684).Repeat(2, 2, 2)
	target, err := FromWeightPercent(map[string]float64{"Al": 6, "V": 4}, "Ti")
	if err != nil {
		t.Fatal(err)
	}
	return RandomSubstitution(host, AllocateIntegerCounts(target, host.NAtoms(), true), 20260825)
}

func TestPOSCARRoundTripPreservesCellCountsAndCoordinates(t *testing.T) {
	original := roundTripFixture(t)
	parsed, err := ParsePOSCAR(ExportPOSCAR(original, "roundtrip"))
	if err != nil {
		t.Fatal(err)
	}
	if parsed.NAtoms() != original.NAtoms() || !reflect.DeepEqual(parsed.SpeciesCounts(), original.SpeciesCounts()) {
		t.Fatalf("species mismatch: got %#v want %#v", parsed.SpeciesCounts(), original.SpeciesCounts())
	}
	if !matricesClose(parsed.Cell, original.Cell, 1e-12) {
		t.Fatalf("cell changed: got %#v want %#v", parsed.Cell, original.Cell)
	}
	if math.Abs(parsed.Volume()-original.Volume()) > 1e-10 {
		t.Fatalf("volume changed: %.12g vs %.12g", parsed.Volume(), original.Volume())
	}
}

func TestExtXYZRoundTripPreservesCellPBCAndSpecies(t *testing.T) {
	original := roundTripFixture(t)
	parsed, err := ParseExtXYZ(ExportExtXYZ(original))
	if err != nil {
		t.Fatal(err)
	}
	if parsed.PBC != original.PBC {
		t.Fatalf("PBC mismatch: got %v want %v", parsed.PBC, original.PBC)
	}
	if !matricesClose(parsed.Cell, original.Cell, 1e-12) {
		t.Fatalf("cell mismatch: got %#v want %#v", parsed.Cell, original.Cell)
	}
	if !reflect.DeepEqual(parsed.Species, original.Species) {
		t.Fatal("extxyz species sequence changed")
	}
}

func TestLAMMPSAtomicDataRoundTripPreservesMetricAndTypeMapping(t *testing.T) {
	original := roundTripFixture(t)
	parsed, err := ParseLAMMPSAtomicData(ExportLAMMPS(original))
	if err != nil {
		t.Fatal(err)
	}
	if parsed.NAtoms() != original.NAtoms() || !reflect.DeepEqual(parsed.SpeciesCounts(), original.SpeciesCounts()) {
		t.Fatalf("species mismatch: got %#v want %#v", parsed.SpeciesCounts(), original.SpeciesCounts())
	}
	// LAMMPS restricted triclinic form rotates the basis but must preserve the
	// lattice metric tensor and volume.
	if !matricesClose(gram(parsed.Cell), gram(original.Cell), 1e-10) {
		t.Fatalf("cell metric changed: got %#v want %#v", gram(parsed.Cell), gram(original.Cell))
	}
	if math.Abs(parsed.Volume()-original.Volume()) > 1e-9 {
		t.Fatalf("volume changed: %.12g vs %.12g", parsed.Volume(), original.Volume())
	}
}
