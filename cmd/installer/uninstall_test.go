package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCleanupHelperPathLivesOutsideInstallDirectory(t *testing.T) {
	installDir := `C:\Users\researcher\AppData\Local\Programs\TiAlloyStudio`
	helper := cleanupHelperPath(`C:\Users\researcher\AppData\Local\Temp`, 12345)
	if strings.HasPrefix(strings.ToLower(helper), strings.ToLower(installDir)) {
		t.Fatalf("cleanup helper must live outside install directory: %s", helper)
	}
	if filepath.Base(helper) != "TiAlloyStudio-uninstall-helper-12345.exe" {
		t.Fatalf("unexpected helper filename: %s", helper)
	}
}

func TestCleanupHelperArgsCarryInstallDirAndParentPID(t *testing.T) {
	dir := `D:\TiAlloyStudio`
	args := cleanupHelperArgs(dir, 4242, true)
	joined := strings.Join(args, " ")
	for _, want := range []string{"--cleanup-install-dir", dir, "--parent-pid", "4242", "--quiet"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("cleanup helper args missing %q: %v", want, args)
		}
	}
}

func TestCleanupModeRecognizesExplicitTarget(t *testing.T) {
	if !isCleanupMode(`D:\TiAlloyStudio`) {
		t.Fatal("non-empty cleanup target must enable stage-2 cleanup mode")
	}
	if isCleanupMode("") {
		t.Fatal("empty cleanup target must not enable stage-2 cleanup mode")
	}
}
