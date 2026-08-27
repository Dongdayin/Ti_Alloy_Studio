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
	if r.GSFEPoints != 11 {
		t.Fatalf("gsfe points=%d", r.GSFEPoints)
	}
	if !strings.Contains(strings.Join(r.Checks, "\n"), "revision edit/export/project round trip") {
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
