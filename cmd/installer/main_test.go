package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestCleanupCommandSpecUsesBoundedCmdShellWithoutStart(t *testing.T) {
	script := `C:\Temp\TiAlloyStudio-remove.cmd`
	exe, args := cleanupCommandSpec(script)

	if exe != "cmd.exe" {
		t.Fatalf("cleanup executable = %q, want cmd.exe", exe)
	}
	want := []string{"/D", "/Q", "/C", script}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("cleanup args = %#v, want %#v", args, want)
	}
	for _, arg := range args {
		if strings.EqualFold(arg, "start") {
			t.Fatalf("cleanup launcher must not use START because START + .cmd can leave an orphan command shell: %#v", args)
		}
	}
}
