package engines

import (
	"math"
	"strings"
	"testing"

	"tialloystudio/internal/model"
)

func TestATATRndStrUsesPartialOccupanciesOnEveryParentSite(t *testing.T) {
	host := model.BuildAlphaTi(2.951, 4.684)
	target, err := model.FromWeightPercent(map[string]float64{"Al": 6, "V": 4}, "Ti")
	if err != nil {
		t.Fatal(err)
	}
	text, err := BuildATATRndStr(host, target.AtomicPercent)
	if err != nil {
		t.Fatalf("BuildATATRndStr failed: %v", err)
	}
	if strings.Count(text, "Ti=") != host.NAtoms() || strings.Count(text, "Al=") != host.NAtoms() || strings.Count(text, "V=") != host.NAtoms() {
		t.Fatalf("each parent-lattice site must carry the same Ti/Al/V partial occupancies:\n%s", text)
	}
	if !strings.Contains(text, "Ti=0.8620443991") || !strings.Contains(text, "Al=0.1019548469") || !strings.Contains(text, "V=0.03600075402") {
		t.Fatalf("rndstr.in does not contain normalized Ti-6Al-4V atomic fractions:\n%s", text)
	}
}

func TestATATRndStrRejectsNonNormalizedOrUnknownOccupancies(t *testing.T) {
	host := model.BuildAlphaTi(2.951, 4.684)
	if _, err := BuildATATRndStr(host, map[string]float64{"Ti": 80, "Al": 10}); err == nil {
		t.Fatal("expected error for atomic percentages that do not sum to 100")
	}
	if _, err := BuildATATRndStr(host, map[string]float64{"Ti": 90, "Xx": 10}); err == nil {
		t.Fatal("expected error for unknown element")
	}
}

func TestParseATATBestCorrComputesRMSAndMaximumDifference(t *testing.T) {
	text := `2 2.90 0.125000 0.120000 0.005000
2 4.70 -0.250000 -0.230000 -0.020000
3 3.10 0.010000 0.000000 0.010000
`
	q, err := ParseATATBestCorr(text)
	if err != nil {
		t.Fatalf("ParseATATBestCorr failed: %v", err)
	}
	if len(q.Clusters) != 3 {
		t.Fatalf("clusters = %d, want 3", len(q.Clusters))
	}
	wantRMS := math.Sqrt((0.005*0.005 + 0.020*0.020 + 0.010*0.010) / 3)
	if math.Abs(q.RMSDifference-wantRMS) > 1e-14 {
		t.Fatalf("RMS = %.16g, want %.16g", q.RMSDifference, wantRMS)
	}
	if math.Abs(q.MaxAbsDifference-0.020) > 1e-14 {
		t.Fatalf("max abs difference = %.16g, want 0.02", q.MaxAbsDifference)
	}
}

func TestParseATATStructureRoundTripShape(t *testing.T) {
	text := `2.951 0 0
-1.4755 2.555873 0
0 0 4.684
1 0 0
0 1 0
0 0 1
0 0 0 Ti
0.6666666667 0.3333333333 0.5 Al
`
	s, err := ParseATATStructure(text)
	if err != nil {
		t.Fatalf("ParseATATStructure failed: %v", err)
	}
	if s.NAtoms() != 2 || s.Species[0] != "Ti" || s.Species[1] != "Al" {
		t.Fatalf("unexpected parsed structure: %#v", s.Species)
	}
	if math.Abs(s.Volume()-model.BuildAlphaTi(2.951, 4.684).Volume()) > 1e-5 {
		t.Fatalf("parsed ATAT structure volume is inconsistent: %g", s.Volume())
	}
}

func TestParseWSLDistroListIgnoresBlankAndNulCharacters(t *testing.T) {
	raw := "Ubuntu-24.04\x00\r\nUbuntu\x00\r\n\r\n"
	got := ParseWSLDistros(raw)
	if len(got) != 2 || got[0] != "Ubuntu-24.04" || got[1] != "Ubuntu" {
		t.Fatalf("ParseWSLDistros = %#v", got)
	}
}
