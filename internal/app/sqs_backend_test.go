package app

import (
	"strings"
	"testing"
)

func TestSQSDefaultsToATATAndRequiresExplicitPairCutoff(t *testing.T) {
	state := NewState()
	_, err := state.Build(BuildRequest{
		Module: "sqs",
		Phase:  "alpha",
		NX:     2,
		NY:     2,
		NZ:     2,
	})
	if err == nil {
		t.Fatal("expected ATAT SQS to reject missing pair cutoff")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "pair cutoff") {
		t.Fatalf("expected explicit pair-cutoff diagnostic, got %v", err)
	}
}

func TestSQSPreviewMustBeExplicitAndReportsPreviewEngine(t *testing.T) {
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
		t.Fatalf("explicit preview SQS failed: %v", err)
	}
	engine, _ := res.Analysis["engine"].(string)
	if !strings.Contains(strings.ToLower(engine), "preview") {
		t.Fatalf("preview result must be unmistakably labeled, engine=%q", engine)
	}
	if res.Structure.Meta["sqs_engine"] == "ATAT mcsqs" {
		t.Fatal("preview SQS was mislabeled as ATAT")
	}
}
