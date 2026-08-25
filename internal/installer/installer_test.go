package installer

import (
	"strings"
	"testing"
)

func TestOfflineEngineInstallScriptReportsRealStages(t *testing.T) {
	s := OfflineEngineInstallScriptWithProgress(`C:\TiAlloyStudio\engines`, `C:\Temp\bundle`, `C:\Temp\progress.txt`)
	for _, needle := range []string{
		"progress.txt",
		"Report 32 'Installing private Python runtime'",
		"Report 48 'Installing bundled scientific Python packages'",
		"Report 70 'Validating ASE, spglib, pymatgen and AtomMan'",
		"Report 80 'Installing bundled Atomsk'",
	} {
		if !strings.Contains(s, needle) {
			t.Fatalf("progress-enabled engine script missing %q", needle)
		}
	}
}

func TestLegacyOfflineEngineInstallScriptStillWorksWithoutProgress(t *testing.T) {
	s := OfflineEngineInstallScript(`C:\TiAlloyStudio\engines`, `C:\Temp\bundle`)
	if strings.Contains(s, "Set-Content -LiteralPath ''") {
		t.Fatal("legacy script should not emit progress writes to an empty path")
	}
	if !strings.Contains(s, "pip install") || !strings.Contains(s, "atomsk.exe") {
		t.Fatal("legacy installer lost scientific engine steps")
	}
}
