package app

import (
	"strings"
	"testing"
)

func TestBuiltInPairSQSWorksWithoutATAT(t *testing.T) {
	state := NewState()
	res, err := state.Build(BuildRequest{
		Module:     "sqs",
		Phase:      "alpha",
		NX:         2,
		NY:         2,
		NZ:         2,
		SQSBackend: "preview", // compatibility key for the built-in TiModelCore pair-SQS backend
		SQSSteps:   20,
		SQSShells:  1,
		Seed:       17,
	})
	if err != nil {
		t.Fatalf("built-in pair-SQS must work without WSL/ATAT: %v", err)
	}
	if res.SQS == nil {
		t.Fatal("built-in pair-SQS must return pair-correlation quality metrics")
	}
	engine, _ := res.Analysis["engine"].(string)
	if strings.Contains(strings.ToLower(engine), "atat") {
		t.Fatalf("built-in pair-SQS was mislabeled as ATAT, engine=%q", engine)
	}
}

func TestExplicitATATRequiresExplicitPairCutoff(t *testing.T) {
	state := NewState()
	_, err := state.Build(BuildRequest{
		Module:     "sqs",
		Phase:      "alpha",
		NX:         2,
		NY:         2,
		NZ:         2,
		SQSBackend: "atat",
	})
	if err == nil {
		t.Fatal("expected explicit ATAT SQS to reject missing pair cutoff")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "pair cutoff") {
		t.Fatalf("expected explicit pair-cutoff diagnostic, got %v", err)
	}
}
