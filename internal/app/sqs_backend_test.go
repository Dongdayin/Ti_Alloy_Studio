package app

import (
	"strings"
	"testing"
)

func TestSQSDefaultsToBuiltInPairSQSWithoutATAT(t *testing.T) {
	state := NewState()
	res, err := state.Build(BuildRequest{
		Module: "sqs",
		Phase:  "alpha",
		NX:     2,
		NY:     2,
		NZ:     2,
		Seed:   17,
	})
	if err != nil {
		t.Fatalf("default SQS must work without WSL/ATAT: %v", err)
	}
	if res.SQS == nil {
		t.Fatal("default native SQS must return pair-correlation quality metrics")
	}
	if got := strings.ToLower(res.Structure.Meta["sqs_backend"].(string)); got != "native" {
		t.Fatalf("default SQS backend=%q, want native", got)
	}
	engine, _ := res.Analysis["engine"].(string)
	if !strings.Contains(strings.ToLower(engine), "timodelcore") || !strings.Contains(strings.ToLower(engine), "pair-sqs") {
		t.Fatalf("default SQS must identify TiModelCore pair-SQS scope, engine=%q", engine)
	}
}

func TestLegacyPreviewAliasWorksWithoutATAT(t *testing.T) {
	state := NewState()
	res, err := state.Build(BuildRequest{
		Module:     "sqs",
		Phase:      "alpha",
		NX:         2,
		NY:         2,
		NZ:         2,
		SQSBackend: "preview",
		SQSSteps:   20,
		SQSShells:  1,
		Seed:       17,
	})
	if err != nil {
		t.Fatalf("legacy preview alias must remain compatible: %v", err)
	}
	if got := strings.ToLower(res.Structure.Meta["sqs_backend"].(string)); got != "native" {
		t.Fatalf("legacy alias must normalize to native provenance, got %q", got)
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
