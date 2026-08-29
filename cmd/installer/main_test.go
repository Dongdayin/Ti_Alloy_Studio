package main

import (
	"strings"
	"testing"
)

func TestTwoStageHelperDoesNotUseLegacyBatchLauncher(t *testing.T) {
	helper := cleanupHelperPath(`C:\Temp`, 99)
	if !strings.HasSuffix(strings.ToLower(helper), `tialloystudio-uninstall-helper-99.exe`) {
		t.Fatalf("unexpected helper path: %s", helper)
	}
	args := cleanupHelperArgs(`D:\TiAlloyStudio`, 321, true)
	joined := strings.Join(args, " ")
	if strings.Contains(strings.ToLower(joined), ".cmd") || strings.Contains(strings.ToLower(joined), " start ") {
		t.Fatalf("two-stage helper must not use legacy batch/start launcher: %v", args)
	}
}

func TestInstallerDisplayVersionStartsPhase2(t *testing.T) {
	if releaseVersion != "0.3.0-phase2-r2" {
		t.Fatalf("installer release version=%q", releaseVersion)
	}
}
