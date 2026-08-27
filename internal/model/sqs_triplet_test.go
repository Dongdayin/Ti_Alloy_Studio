package model

import (
	"math"
	"reflect"
	"testing"
)

func TestNativeSQSReportsDeterministicPairAndTripletCorrelationEvidence(t *testing.T) {
	host := BuildAlphaTi(2.951, 4.684).Repeat(2, 2, 2)
	target, err := FromWeightPercent(map[string]float64{"Al": 10}, "Ti")
	if err != nil {
		t.Fatal(err)
	}
	alloc := AllocateIntegerCounts(target, host.NAtoms(), true)
	first, err := GenerateSQS(host, alloc, 41, 2, 60, 1e-5)
	if err != nil {
		t.Fatal(err)
	}
	second, err := GenerateSQS(host, alloc, 41, 2, 60, 1e-5)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first.Structure.Species, second.Structure.Species) || !reflect.DeepEqual(first.Quality, second.Quality) {
		t.Fatal("same seed and settings did not reproduce the same SQS evidence")
	}
	if len(first.Quality.Shells) != 2 || len(first.Quality.TripletClusters) == 0 {
		t.Fatalf("pair/triplet evidence incomplete: pairs=%d triplets=%d", len(first.Quality.Shells), len(first.Quality.TripletClusters))
	}
	for _, cluster := range first.Quality.TripletClusters {
		observed, target := 0.0, 0.0
		for _, value := range cluster.Observed {
			observed += value
		}
		for _, value := range cluster.Target {
			target += value
		}
		if math.Abs(observed-1) > 1e-12 || math.Abs(target-1) > 1e-12 {
			t.Fatalf("triplet probabilities do not normalize: observed=%g target=%g", observed, target)
		}
	}
	if first.Quality.Method != "pair_triplet_correlation_sqs" || first.Quality.VerificationStatus != "not_atat_verified" {
		t.Fatalf("scientific labeling = method %q verification %q", first.Quality.Method, first.Quality.VerificationStatus)
	}
	if len(first.Convergence) != 61 {
		t.Fatalf("bounded optimization trace length=%d want 61", len(first.Convergence))
	}
}
