package model

import (
	"math"
	"testing"
)

func TestAnalyzeGSFESeriesChecksWholeDisplacementSequence(t *testing.T) {
	series := AlphaGSFE("basal_a", 2.951, 4.684, [3]int{3, 3, 6}, 12, 0.5)
	d := AnalyzeGSFESeries(series)
	if d.PointCount != 13 {
		t.Fatalf("point count = %d, want 13", d.PointCount)
	}
	if !d.AtomCountConsistent || !d.CellConsistent || !d.PBCConsistent {
		t.Fatalf("series topology changed: %+v", d)
	}
	if !d.LambdaMonotonic || math.Abs(d.LambdaStart) > 1e-15 || math.Abs(d.LambdaEnd-1) > 1e-15 {
		t.Fatalf("invalid lambda sequence: %+v", d)
	}
	if d.MinimumDistanceAngstrom <= 0 || math.IsNaN(d.MinimumDistanceAngstrom) || math.IsInf(d.MinimumDistanceAngstrom, 0) {
		t.Fatalf("invalid minimum distance: %+v", d)
	}
	if d.FaultSeparationAngstrom <= 0 {
		t.Fatalf("fault separation not reported: %+v", d)
	}
	if !d.EndpointEquivalent {
		t.Fatalf("basal <a> path endpoint should be lattice-equivalent: %+v", d)
	}
}

func TestAnalyzeGSFESeriesDetectsCellMutation(t *testing.T) {
	series := AlphaGSFE("prismatic_a", 2.951, 4.684, [3]int{2, 2, 4}, 6, 0.5)
	series.Points[3].Structure.Cell[0][0] += 0.1
	d := AnalyzeGSFESeries(series)
	if d.CellConsistent {
		t.Fatal("cell mutation across GSFE points was not detected")
	}
}
