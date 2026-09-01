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

func TestFixedCompositionSQSEvaluatorMatchesFullQualityCalculation(t *testing.T) {
	species := []string{"Ti", "Al", "Ti", "V"}
	shells := []pairShell{{
		index:    1,
		distance: 1,
		pairs: [][2]int{
			{0, 1}, {0, 2}, {0, 3},
			{1, 2}, {1, 3}, {2, 3},
		},
	}}
	triplets := []tripletCluster{{
		signature: [3]int{1, 1, 1},
		triplets:  [][3]int{{0, 1, 2}, {0, 1, 3}, {0, 2, 3}, {1, 2, 3}},
	}}

	got := newFixedCompositionSQSEvaluator(species, shells, triplets).quality(species)
	if len(got.Shells) != 1 || len(got.TripletClusters) != 1 {
		t.Fatalf("quality dimensions = %d shells %d triplets, want 1/1", len(got.Shells), len(got.TripletClusters))
	}
	pairs := got.Shells[0]
	for key, want := range map[PairKey]float64{
		"Al-Al": 0,
		"Al-Ti": 2.0 / 6.0,
		"Al-V":  1.0 / 6.0,
		"Ti-Ti": 1.0 / 6.0,
		"Ti-V":  2.0 / 6.0,
		"V-V":   0,
	} {
		if math.Abs(pairs.Observed[key]-want) > 1e-12 {
			t.Fatalf("observed pair %s = %.15g, want %.15g", key, pairs.Observed[key], want)
		}
	}
	for key, want := range map[PairKey]float64{
		"Al-Al": 0.25 * 0.25,
		"Al-Ti": 2 * 0.25 * 0.5,
		"Al-V":  2 * 0.25 * 0.25,
		"Ti-Ti": 0.5 * 0.5,
		"Ti-V":  2 * 0.5 * 0.25,
		"V-V":   0.25 * 0.25,
	} {
		if math.Abs(pairs.Target[key]-want) > 1e-12 {
			t.Fatalf("target pair %s = %.15g, want %.15g", key, pairs.Target[key], want)
		}
	}
	triplet := got.TripletClusters[0]
	for key, want := range map[TripletKey]float64{
		"Al-Ti-Ti": 1.0 / 4.0,
		"Al-Ti-V":  2.0 / 4.0,
		"Ti-Ti-V":  1.0 / 4.0,
	} {
		if math.Abs(triplet.Observed[key]-want) > 1e-12 {
			t.Fatalf("observed triplet %s = %.15g, want %.15g", key, triplet.Observed[key], want)
		}
	}
	if math.Abs(triplet.Target["Al-Ti-V"]-6*0.25*0.5*0.25) > 1e-12 {
		t.Fatalf("target triplet Al-Ti-V = %.15g", triplet.Target["Al-Ti-V"])
	}
}
