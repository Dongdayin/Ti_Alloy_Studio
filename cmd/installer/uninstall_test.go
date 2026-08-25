package main

import (
	"strings"
	"testing"
)

func TestUninstallCleanupScriptDeletesWholeInstallDirectoryAsynchronously(t *testing.T) {
	dir := `C:\Users\researcher\AppData\Local\Programs\TiAlloyStudio`
	script := uninstallCleanupScript(dir)
	for _, want := range []string{
		`ping -n 2 127.0.0.1`,
		`rmdir /s /q "C:\Users\researcher\AppData\Local\Programs\TiAlloyStudio"`,
		`if not exist "C:\Users\researcher\AppData\Local\Programs\TiAlloyStudio" goto removed`,
		`del /f /q "%~f0"`,
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("cleanup script missing %q:\n%s", want, script)
		}
	}
}

func TestUninstallCleanupScriptDoesNotDeletePayloadSynchronously(t *testing.T) {
	script := uninstallCleanupScript(`D:\TiAlloyStudio`)
	if strings.Contains(script, "Uninstall.exe") {
		t.Fatalf("cleanup should remove the install directory generically after parent exit, not special-case a running uninstaller: %s", script)
	}
}
