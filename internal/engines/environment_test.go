package engines

import (
	"strings"
	"testing"
)

func TestParseToolPathLines(t *testing.T) {
	raw := "python3|/usr/bin/python3\natomsk|/home/user/bin/atomsk\nmcsqs|\ncorrdump|/usr/local/bin/corrdump\n"
	got := parseToolPathLines(raw)
	if got["python3"] != "/usr/bin/python3" {
		t.Fatalf("python3 path = %q", got["python3"])
	}
	if got["atomsk"] != "/home/user/bin/atomsk" {
		t.Fatalf("atomsk path = %q", got["atomsk"])
	}
	if got["mcsqs"] != "" {
		t.Fatalf("mcsqs should be unavailable, got %q", got["mcsqs"])
	}
}

func TestWSLProbeAlwaysPrintsOneLinePerTool(t *testing.T) {
	script := buildWSLProbeScript([]string{"atomsk", "mcsqs", "lmp_mpi"})
	if strings.Count(script, "printf '%s|%s\\n'") != 3 {
		t.Fatalf("probe must emit one newline-terminated record per tool: %q", script)
	}
	if !strings.Contains(script, "find \"$HOME\"") {
		t.Fatalf("probe should fall back to a bounded HOME search for tools outside PATH: %q", script)
	}
}

func TestChooseWSLDistroHonorsRequestedName(t *testing.T) {
	distros := []string{"Ubuntu-24.04", "Ubuntu"}
	if got := chooseWSLDistro(distros, "Ubuntu"); got != "Ubuntu" {
		t.Fatalf("requested distro not selected: %q", got)
	}
	if got := chooseWSLDistro(distros, ""); got != "Ubuntu-24.04" {
		t.Fatalf("default distro = %q", got)
	}
	if got := chooseWSLDistro(distros, "Debian"); got != "" {
		t.Fatalf("unknown requested distro should not silently fall back: %q", got)
	}
}
