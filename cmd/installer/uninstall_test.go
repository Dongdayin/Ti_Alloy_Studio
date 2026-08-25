package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveInstalledPayloadExceptSelf(t *testing.T) {
	dir := t.TempDir()
	self := filepath.Join(dir, "Uninstall.exe")
	mustWrite := func(path string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite(self)
	mustWrite(filepath.Join(dir, "TiAlloyStudio.exe"))
	mustWrite(filepath.Join(dir, "TiAlloyStudio-Manual.docx"))
	mustWrite(filepath.Join(dir, "engines", "python", "python.exe"))
	mustWrite(filepath.Join(dir, "engines", "atomsk", "atomsk.exe"))

	if err := removeInstalledPayloadExceptSelf(dir, self); err != nil {
		t.Fatalf("remove payload: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "Uninstall.exe" {
		t.Fatalf("expected only Uninstall.exe to remain, got %#v", entries)
	}
}
