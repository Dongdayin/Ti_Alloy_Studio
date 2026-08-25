package engines

import "testing"

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
