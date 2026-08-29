package studio

import (
	"strings"
	"testing"
)

func TestScientificSmoke(t *testing.T) {
	r, err := ScientificSmoke()
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != "PASS" {
		t.Fatalf("status=%s checks=%v", r.Status, r.Checks)
	}
	if r.Atoms != 192 || r.Counts["Ti"] != 165 || r.Counts["Al"] != 20 || r.Counts["V"] != 7 {
		t.Fatalf("smoke=%+v", r)
	}
	if r.SeriesPoints != 5 {
		t.Fatalf("series points=%d", r.SeriesPoints)
	}
	checks := strings.Join(r.Checks, "\n")
	for _, want := range []string{"stacking-fault geometry series", "Phase 2 dislocation/grain-boundary/local-chemistry geometry", "revision edit/export/project round trip"} {
		if !strings.Contains(checks, want) {
			t.Fatalf("release smoke omitted %q: %v", want, r.Checks)
		}
	}
	if r.Phase2Models < 3 {
		t.Fatalf("phase2 models=%d, want at least 3", r.Phase2Models)
	}
	if strings.Contains(checks, "GSFE") {
		t.Fatalf("release smoke still uses project-calculation wording: %v", r.Checks)
	}
	if !strings.Contains(checks, "revision edit/export/project round trip") {
		t.Fatalf("release smoke omitted the portable revision workflow: %v", r.Checks)
	}
}

func TestScientificSmokeDoesNotRequireOptionalManagedEngines(t *testing.T) {
	r, err := ScientificSmoke()
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != "PASS" {
		t.Fatalf("status=%s", r.Status)
	}
}
