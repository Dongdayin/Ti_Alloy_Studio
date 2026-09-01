package main

import (
	"errors"
	"os"
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

func TestInstallerDisplayVersionStartsPhase3(t *testing.T) {
	if releaseVersion != "0.4.3-phase3-r4" {
		t.Fatalf("installer release version=%q", releaseVersion)
	}
}

func TestPrepareCleanEngineDirReportsRemoveFailure(t *testing.T) {
	err := prepareCleanEngineDirWith(`D:\TiAlloyStudio\engines`,
		func(string) error { return errors.New("file is locked") },
		func(string) (os.FileInfo, error) { return nil, os.ErrNotExist },
	)
	if err == nil {
		t.Fatal("old engine cleanup failure was ignored")
	}
	msg := err.Error()
	for _, want := range []string{"old private engine directory", "file is locked", "close Ti Alloy Studio"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("cleanup error %q does not contain %q", msg, want)
		}
	}
}
